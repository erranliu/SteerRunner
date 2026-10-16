package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"strings"
	"time"
)

var errRemoteReadinessPending = errors.New("remote preparation receipt not yet available")

func (l *Lifecycle) checkRemoteReadiness(s State) error {
	if !strings.Contains(strings.ToLower(l.setup.RepositoryURL), "github.com") {
		return nil
	}
	if l.profile.RuntimeExecutable == "" {
		return errors.New("remote_sandbox_authentication_unsupported: execution-host runtime is not configured; zero business input")
	}
	r := s.RemoteReadiness
	if r != nil && l.profile.RuntimeSHA256 != "" && r.RuntimeSHA256 != l.profile.RuntimeSHA256 {
		return errors.New("remote_runtime_build_mismatch; zero business input")
	}
	if r == nil || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.Contract != codex.PolicyContract || r.ConfigurationDigest != l.profile.RuntimeConfigurationDigest || r.Repository != l.setup.RepositoryURL || !codex.ValidNativeID(r.SessionID) || r.Process.PID <= 0 || !r.ExpiresAt.After(l.clock().Add(5*time.Second)) || r.CheckedAt.After(l.clock().Add(5*time.Second)) || !r.ExpiresAt.After(r.CheckedAt) {
		return errors.New("remote_sandbox_readiness_unverified_or_expired; zero business input")
	}
	return nil
}

// Captured only while the original launch is unconfirmed and before the first
// business input. Later terminal output can never replace the saved receipt.
func (l *Lifecycle) readRemoteReadiness(ctx context.Context, s b.LifecycleSnapshot) (codex.RemoteReadiness, error) {
	var ready codex.RemoteReadiness
	result, err := l.client.MetaCall(ctx, "terminal", "read", "--terminal", s.Ref.External.Terminal, "--limit", "1000", "--json")
	if err != nil {
		return ready, err
	}
	var value struct {
		Terminal struct {
			Handle string
			Tail   []string
		}
	}
	if json.Unmarshal(result.Payload, &value) != nil || result.RuntimeID != s.Ref.External.RuntimeID || value.Terminal.Handle != s.Ref.External.Terminal {
		return ready, errors.New("remote readiness original terminal changed")
	}
	found := 0
	for _, line := range value.Terminal.Tail {
		line = strings.TrimSpace(line)
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
	if err = l.checkRemoteReadiness(State{RemoteReadiness: &ready}); err != nil {
		return ready, err
	}
	return ready, nil
}
