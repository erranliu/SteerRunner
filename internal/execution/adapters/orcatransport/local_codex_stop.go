package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const LocalCodexStopScope = "orca_local_codex_resources"
const LocalCodexStopDiagnostic = "Stopped the registered original Orca terminals and local Codex helper/root processes; all descendant processes and execution-host credentials were not verified or revoked."

// StopScope is fixed by accepted configuration, including before resources exist.
func StopScope(p Profile, agent string) string {
	if p.Environment != "" {
		return "orca_terminal"
	}
	if agent == "codex" && p.Terminal == nil && p.RuntimeExecutable != "" {
		return LocalCodexStopScope
	}
	return ""
}

func (l *Lifecycle) localCodexScope() bool { return l.StopEvidenceScope() == LocalCodexStopScope }

func (l *Lifecycle) localStopObjects(s b.LifecycleSnapshot, state State) ([]b.ProcessIdentity, []b.ProcessIdentity, error) {
	p := state.Process
	if p != s.Ref.External || p.Kind != "orca_terminal" || p.ExecutionHost != l.profile.Target().ExecutionHost || p.Executable != l.profile.Executable {
		return nil, nil, errors.New("original local Orca resource binding is incomplete")
	}
	terminals := []b.ProcessIdentity{p}
	if state.Control != nil {
		terminals = append(terminals, state.Control.Process)
	}
	for _, op := range s.Operations {
		if (op.Action == string(b.LifecycleCreateResource) || op.Action == string(b.LifecyclePrepareWorkspace)) && (op.Status == b.OperationInFlight || op.Status == b.OperationUnknown) {
			return terminals, nil, errors.New("original resource creation remains unresolved")
		}
	}
	if state.RemoteReadiness == nil && !state.LaunchAttempted && len(state.Turns) == 0 {
		return terminals, nil, nil // Positive pre-launch state, not missing evidence after launch.
	}
	ready, err := l.codexReadiness(state, s.Workspace.Path)
	if err != nil {
		return terminals, nil, err
	}
	if state.Control == nil {
		return terminals, nil, errors.New("original local Codex control identity missing")
	}
	return terminals, []b.ProcessIdentity{ready.Owner, ready.Process}, nil
}

func (l *Lifecycle) verifyLocalCodexStopped(ctx context.Context, s b.LifecycleSnapshot) error {
	state, err := l.resourceState(s)
	if err != nil {
		return err
	}
	terminals, roots, err := l.localStopObjects(s, state)
	if err != nil {
		return err
	}
	// This one inventory proves both terminals on the original runtime's local host.
	if err := VerifyTerminalsStopped(ctx, l.client, terminals); err != nil {
		return err
	}
	for _, root := range roots {
		if fact := l.probeProcess(&root); fact.State != "absent" {
			return fmt.Errorf("original local process %d is %s: %s", root.PID, fact.State, fact.Reason)
		}
	}
	return ctx.Err()
}

// First invocation only. Reconcile never calls this mutation path, even if a
// crash lost the receipt between the first and second terminal closure.
func (l *Lifecycle) stopLocalCodex(ctx context.Context, r b.LifecycleRequest, first bool) (b.LifecycleReceipt, error) {
	ctx, cancelStop := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStop()
	l.stopping.Store(true)
	receipt := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	state, err := l.resourceState(r.Snapshot)
	if err != nil {
		return receipt, err
	}
	var identityConflict error
	if first {
		terminals, roots, _ := l.localStopObjects(r.Snapshot, state)
		if len(roots) > 0 && l.probeProcess(&roots[0]).State == "matches" {
			cooperative, cancel := context.WithTimeout(ctx, 2*time.Second)
			path, e := codex.RemoteStopPath(*state.RemoteReadiness)
			if e == nil {
				raw, _ := json.Marshal(state.RemoteReadiness)
				e = l.uploadStopArtifact(cooperative, "", state.Process.RuntimeID, "id:"+state.Process.WorktreeID, path, bytes.NewReader(raw))
			}
			if e == nil {
				for cooperative.Err() == nil && l.probeProcess(&roots[0]).State == "matches" {
					if !stopPause(cooperative) {
						break
					}
				}
			}
			cancel()
		}
		for _, target := range terminals {
			if VerifyTerminalStopped(ctx, l.client, target) == nil {
				continue
			}
			t, e := InspectBoundTerminal(ctx, l.client, target)
			branchMatches := strings.TrimPrefix(t.Branch, "refs/heads/") == r.Snapshot.Workspace.Branch || len(state.Turns) == 0 && !state.LaunchConfirmed && t.Branch == ""
			pathMatches := strings.EqualFold(strings.ReplaceAll(t.WorktreePath, "\\", "/"), strings.ReplaceAll(r.Snapshot.Workspace.Path, "\\", "/"))
			if e == nil && (t.WorktreeID != target.WorktreeID || target == state.Process && (!pathMatches || !branchMatches)) {
				e = &b.BackendError{Code: b.BackendIdentityMismatch, Diagnostic: "original local stop workspace changed"}
			}
			if e == nil {
				_, e = CloseTerminalChecked(ctx, l.client, target.Terminal, target.RuntimeID)
			}
			var mismatch *b.BackendError
			if errors.As(e, &mismatch) && mismatch.Code == b.BackendIdentityMismatch {
				identityConflict = errors.Join(identityConflict, e)
			}
		}
	}
	// Bounded queries may observe a lost close response; they never repeat close.
	until := time.Now().Add(3 * time.Second)
	for {
		err = l.verifyLocalCodexStopped(ctx, r.Snapshot)
		if err == nil || !first || ctx.Err() != nil || time.Now().After(until) || identityConflict != nil {
			break
		}
		if !stopPause(ctx) {
			break
		}
	}
	if err = errors.Join(identityConflict, err); err != nil {
		return receipt, err
	}
	receipt.Disposition = b.OperationConfirmed
	receipt.State = b.LifecycleStateFacts{Stopped: true, WorkerAbsent: true}
	receipt.Diagnostic = LocalCodexStopDiagnostic
	return receipt, nil
}

func stopPause(ctx context.Context) bool {
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
