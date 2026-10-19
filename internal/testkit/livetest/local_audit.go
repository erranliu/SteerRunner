package livetest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

// LocalPermissionAudit uses a fresh diagnostic terminal, never the business
// terminal. Its trusted helper reads native history on the execution host.
func LocalPermissionAudit(ctx context.Context, client native.Client, process b.ProcessIdentity, runtimeExecutable, workspace, provider string) (audit codex.RemoteAudit, err error) {
	if process.ExecutionHost != "local" {
		return audit, errors.New("helper audit is local only")
	}

	input, _ := json.Marshal(map[string]string{"Workspace": workspace, "Session": provider})
	command := "& '" + strings.ReplaceAll(runtimeExecutable, "'", "''") + "' '--audit' '" + base64.RawURLEncoding.EncodeToString(input) + "'"
	r, err := client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+process.WorktreeID, "--title", "SteerRunner original native permission audit", "--command", command, "--json")
	if err != nil {
		return audit, err
	}
	var created struct{ Terminal native.Terminal }
	if json.Unmarshal(r.Payload, &created) != nil || r.RuntimeID != process.RuntimeID || created.Terminal.Handle == "" || created.Terminal.IncarnationID == "" || created.Terminal.WorktreeID != process.WorktreeID {
		return audit, errors.New("remote audit terminal creation identity unverified")
	}
	original := b.ProcessIdentity{RuntimeID: r.RuntimeID, Terminal: created.Terminal.Handle, Incarnation: created.Terminal.IncarnationID, WorktreeID: process.WorktreeID}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		closeErr := CloseAuxiliaryTerminal(closeCtx, client, original)
		audit.Inspector = &original
		audit.InspectorClosed = closeErr == nil
		err = errors.Join(err, closeErr)
	}()
	for {
		if _, err = client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, original); err != nil {
			return audit, err
		}
		r, err = client.MetaCall(ctx, "terminal", "read", "--terminal", original.Terminal, "--limit", "200", "--json")
		if err != nil {
			return audit, err
		}
		var value struct {
			Terminal struct {
				Handle string
				Tail   []string
			}
		}
		if json.Unmarshal(r.Payload, &value) != nil || r.RuntimeID != original.RuntimeID || value.Terminal.Handle != original.Terminal {
			return audit, errors.New("remote audit read identity changed")
		}
		for _, line := range value.Terminal.Tail {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SR_REMOTE_AUDIT:") {
				raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, "SR_REMOTE_AUDIT:"))
				if e != nil || json.Unmarshal(raw, &audit) != nil {
					return audit, errors.New("invalid native audit receipt")
				}
				if !audit.Readiness.Verified || audit.Native.Session != provider || audit.Readiness.SessionID != provider || !strings.EqualFold(strings.ReplaceAll(audit.Readiness.Workspace, "\\", "/"), strings.ReplaceAll(workspace, "\\", "/")) {
					return audit, errors.New("remote native audit identity changed")
				}
				return audit, nil
			}
			if strings.HasPrefix(line, "remote native audit failed:") {
				return audit, fmt.Errorf("%s", line)
			}
		}
		select {
		case <-ctx.Done():
			return audit, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
