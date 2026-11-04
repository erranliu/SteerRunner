package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

type AuditedModelEvidence struct {
	Choice                model.Settings
	ProviderSession, Turn string
	SourceDigest          string
}

// Audited registration is owned by a single Run and never shared with metering.
type auditedModelJournal struct {
	access.Journal
	prefix string
}

func (j auditedModelJournal) UsageJournal(ctx context.Context, key string, target any) error {
	return j.Journal.UsageJournal(ctx, j.prefix+key, target)
}

func (j auditedModelJournal) PutUsageJournal(ctx context.Context, key string, value any) error {
	return j.Journal.PutUsageJournal(ctx, j.prefix+key, value)
}

// ReadAuditedModel reads only the original local Codex turn's model and effort.
// The caller must own the Run's audited result observation. This does not use
// the usage collector's permit and never returns or persists a transcript.
func ReadAuditedModel(ctx context.Context, journal access.Journal, r *runstate.Record, value b.TurnEvidence) (out AuditedModelEvidence, result error) {
	if journal == nil || r.Request.Workflow.ID != "development-audited" || r.Request.Workflow.Version != 2 || r.Request.Execution.Target.Kind != "local" || r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.Snapshot.Ref.Agent.Name != "codex" || !value.Completed || !value.Succeeded {
		return out, errors.New("original audited model read is not authorized")
	}
	inputs := make([]b.TurnInput, 0, len(r.Workflow.StageInputs))
	for _, input := range r.Workflow.StageInputs {
		if input.AttemptID == r.Snapshot.Session.AttemptID {
			inputs = append(inputs, b.TurnInput{CommandID: input.CommandID, Prompt: input.Prompt})
		}
	}
	binding, profile, original, err := orca.StructuredUsageBinding(r.Profile, r.Snapshot, inputs)
	if err != nil || binding.RunID != r.ID || original.HostID != "local" || profile.Environment != "" || original.SessionID != value.SessionID || len(binding.Commands) == 0 {
		return out, errors.New("original audited model source is unbound")
	}
	last := binding.Commands[len(binding.Commands)-1]
	if last.ID != value.CommandID || last.NativeTurnID != value.TurnID {
		return out, errors.New("original audited model turn differs from the result")
	}
	channel, err := native.OpenUsageFiles(ctx, "", original.RuntimeID, "local")
	if err != nil {
		return out, err
	}
	defer channel.Close()
	sessions, err := channel.Sessions(ctx)
	if err != nil {
		return out, err
	}
	candidate, err := selectUsageSession(sessions, binding, &original)
	if err != nil {
		return out, err
	}
	root, file := filepath.Dir(candidate.FilePath), filepath.Base(candidate.FilePath)
	home := candidate.CodexHome
	if home == "" {
		home = localCodexHome(candidate.FilePath)
	}
	if !api.SafeRoot(root) || !filepath.IsAbs(home) || !strings.HasPrefix(parsers.NormalizePath(root), parsers.NormalizePath(home)+"/sessions/") || !strings.HasSuffix(strings.ToLower(file), "-"+strings.ToLower(binding.SessionID)+".jsonl") {
		return out, errors.New("original audited model source path is unverified")
	}
	if err := channel.VerifySource(ctx, root, file); err != nil {
		return out, err
	}
	const purpose = "orca-audited-model"
	manager := access.Manager{Store: auditedModelJournal{Journal: journal, prefix: purpose + ":" + r.ID + ":"}, Instance: r.ID, PolicyDigest: purpose}
	lease, err := manager.Acquire(ctx, channel, access.Lease{RunID: r.ID, HostID: "local", Runtime: original.RuntimeID, Root: root, File: file}, value.CommandID)
	if lease.ID != "" && lease.Instance == r.ID && lease.PolicyDigest == purpose && lease.File == file {
		defer func() {
			clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
			defer cancel()
			released, err := manager.Release(clean, channel, lease, value.CommandID, func(ctx context.Context, l access.Lease) bool { return safeFolderForHost(ctx, profile, l, true) })
			if err != nil || released.Phase != "revoked" {
				out = AuditedModelEvidence{}
				result = errors.Join(result, errors.New("original audited model registration revocation is unverified"), err)
			}
		}()
	}
	if err != nil {
		return out, fmt.Errorf("acquire original audited model registration: %w", err)
	}
	if lease.File != file {
		return out, errors.New("original audited model file selector changed")
	}
	if err := manager.ValidateRead(ctx, channel, lease); err != nil {
		return out, fmt.Errorf("validate original audited model registration: %w", err)
	}
	before, err := channel.Stat(ctx, lease.FolderID, file)
	if err != nil || before.IsDirectory || before.IsSymlink || before.Mtime == nil || before.Size < 1 || before.Size > parsers.MaxSource {
		return out, errors.New("original audited model source is outside read bounds")
	}
	raw := make([]byte, 0, int(before.Size))
	for int64(len(raw)) < before.Size {
		if err := manager.ValidateRead(ctx, channel, lease); err != nil {
			return out, err
		}
		count := min(int(before.Size)-len(raw), 512<<10)
		chunk, err := channel.ReadChunk(ctx, lease.FolderID, file, int64(len(raw)), count)
		if err != nil || len(chunk) != count {
			return out, errors.New("original audited model source read was incomplete")
		}
		raw = append(raw, chunk...)
	}
	after, err := channel.Stat(ctx, lease.FolderID, file)
	if err != nil || after.IsDirectory || after.IsSymlink || after.Mtime == nil || after.Size != before.Size || *after.Mtime != *before.Mtime {
		return out, errors.New("original audited model source changed during the read")
	}
	if err := channel.VerifySource(ctx, root, file); err != nil {
		return out, err
	}
	return auditedModelFromSource(raw, binding.SessionID, binding.CWD, value.TurnID)
}

func auditedModelFromSource(raw []byte, provider, cwd, turn string) (AuditedModelEvidence, error) {
	var header struct {
		Type    string
		Payload struct{ ID, CWD string }
	}
	line, _, ok := bytes.Cut(raw, []byte{'\n'})
	if !ok || json.Unmarshal(line, &header) != nil || header.Type != "session_meta" || header.Payload.ID != provider || parsers.NormalizePath(header.Payload.CWD) != parsers.NormalizePath(cwd) {
		return AuditedModelEvidence{}, errors.New("original audited model source header changed")
	}
	choice, err := codex.RemoteTurnModel(bytes.NewReader(raw), turn)
	if err != nil {
		return AuditedModelEvidence{}, err
	}
	return AuditedModelEvidence{Choice: choice, ProviderSession: provider, Turn: turn, SourceDigest: parsers.TextHash(string(raw))}, nil
}
