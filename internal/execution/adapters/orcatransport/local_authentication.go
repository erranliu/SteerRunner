package orcatransport

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

var errRemoteReadinessPending = errors.New("remote preparation receipt not yet available")

func (l *Lifecycle) checkLocalReadiness(s State) error {
	if l.profile.Environment != "" {
		return errors.New("helper preparation is local only")
	}
	if l.profile.Terminal != nil {
		r := s.TerminalReadiness
		if r == nil || r.Stage != "ready" {
			return errors.New("native terminal original preparation missing; zero business input")
		}
		return l.validateTerminalReceipt(*r)
	}
	if l.profile.RuntimeExecutable == "" {
		return nil
	}

	r := s.RemoteReadiness
	if r != nil && l.profile.RuntimeSHA256 != "" && r.RuntimeSHA256 != l.profile.RuntimeSHA256 {
		return errors.New("remote_runtime_build_mismatch; zero business input")
	}
	if r == nil || len(r.ExitPublicKey) != ed25519.PublicKeySize || r.Owner.PID <= 0 || r.Owner.Created == "" || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.ConfigurationDigest != l.profile.RuntimeConfigurationDigest || r.Repository != l.setup.RepositoryURL || !codex.ValidNativeID(r.SessionID) || r.Process.PID <= 0 || r.CheckedAt.After(l.clock().Add(5*time.Second)) {
		return errors.New("remote_host_readiness_unverified; zero business input")
	}
	return nil
}

// Captured only while the original launch is unconfirmed and before the first
// business input. Later terminal output can never replace the saved receipt.
func (l *Lifecycle) readLocalCodexReadiness(ctx context.Context, s b.LifecycleSnapshot) (codex.RemoteReadiness, error) {
	var ready codex.RemoteReadiness
	if l.profile.Environment != "" {
		return ready, errors.New("helper preparation is local only")
	}
	result, err := l.client.MetaCall(ctx, "terminal", "read", "--terminal", s.Ref.External.Terminal, "--cursor", "0", "--limit", "1000", "--json")
	if err != nil {
		return ready, err
	}
	var value struct {
		Terminal struct {
			Handle, Source, OldestCursor string
			Tail                         []string
			Truncated, Limited           *bool
		}
	}
	if result.CallUncertain || json.Unmarshal(result.Payload, &value) != nil || result.RuntimeID != s.Ref.External.RuntimeID || value.Terminal.Handle != s.Ref.External.Terminal {
		return ready, errors.New("remote readiness original terminal changed")
	}
	if value.Terminal.Source != "stream" || value.Terminal.OldestCursor != "0" || value.Terminal.Truncated == nil || *value.Terminal.Truncated || value.Terminal.Limited == nil || *value.Terminal.Limited {
		return ready, errors.New("remote readiness output stream incomplete or unavailable")
	}
	found := 0
	for _, line := range value.Terminal.Tail {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, codex.RemoteFailurePrefix) {
			code := strings.TrimPrefix(line, codex.RemoteFailurePrefix)
			if code == "" || len(code) > 128 || strings.Trim(code, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
				return ready, errors.New("invalid remote startup failure receipt")
			}
			return ready, errors.New("remote_agent_preparation_failed:" + code)
		}
		if !strings.HasPrefix(line, codex.RemoteReadyPrefix) {
			continue
		}
		raw, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteReadyPrefix))
		if decodeErr != nil || json.Unmarshal(raw, &ready) != nil {
			return ready, errors.New("invalid remote readiness receipt")
		}
		found++
	}
	if found == 0 {
		return ready, errRemoteReadinessPending
	}
	if found != 1 || !strings.EqualFold(strings.ReplaceAll(ready.Workspace, "\\", "/"), strings.ReplaceAll(s.Workspace.Path, "\\", "/")) {
		return ready, errors.New("remote readiness receipt missing or workspace changed")
	}
	if err = l.checkLocalReadiness(State{RemoteReadiness: &ready}); err != nil {
		return ready, err
	}
	return ready, nil
}
