//go:build windows && live

package native

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	adapter "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

var structuredPipe *livetest.StructuredPipeAudit

func originalStructuredRecord(m livetest.Manifest, view contracts.RunView) (*runstate.Record, livetest.StructuredRun, error) {
	var empty livetest.StructuredRun
	store, err := storage.OpenReader(filepath.Join(m.Root, "s"))
	if err != nil {
		return nil, empty, err
	}
	defer store.Close()
	records, err := store.Runs()
	if err != nil {
		return nil, empty, err
	}
	for _, record := range records {
		if record.ID == view.RunID {
			run, err := livetest.ReadStructuredRun(m, view, record)
			return record, run, err
		}
	}
	return nil, empty, errors.New("original structured Run missing")
}
func structuredClient(run livetest.StructuredRun) orca.AgentSessionClient {
	if run.Profile.Transport.Environment != "" {
		return orca.NewRemoteAgentSessionClient(run.Profile.Transport.Environment, run.Binding.RuntimeID, run.Profile.PairingHostID)
	}
	if run.Agent == "pi" {
		return orca.NewLocalPiAgentSessionClient(run.Binding.RuntimeID)
	}
	return orca.NewLocalAgentSessionClient(run.Binding.RuntimeID)
}
func structuredNativeState(m livetest.Manifest, view contracts.RunView) (livetest.StructuredRun, livetest.StructuredHistory, error) {
	_, run, err := originalStructuredRecord(m, view)
	if err != nil {
		return run, livetest.StructuredHistory{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, shown, err := candidateClient(m).ShowWorktree(ctx, "id:"+run.Binding.Worktree.ID)
	if err != nil {
		return run, livetest.StructuredHistory{}, err
	}
	original := run.Binding.Worktree
	if !shown.Decoded() || result.RuntimeID != run.Binding.RuntimeID || shown.Worktree.ID != original.ID || shown.Worktree.InstanceID != original.InstanceID || shown.Worktree.Path != view.Execution.Workspace || shown.Worktree.Branch != view.Execution.Branch || shown.Worktree.BaseRef != original.BaseRef {
		return run, livetest.StructuredHistory{}, errors.New("original structured workspace changed")
	}
	history, err := livetest.ReadStructuredHistory(ctx, structuredClient(run), run)
	return run, history, err
}
func structuredStopEvidence(m livetest.Manifest, view contracts.RunView) (any, error) {
	record, run, err := originalStructuredRecord(m, view)
	if err != nil {
		return nil, err
	}
	if view.StopStatus != "stopped" || view.NewInputsAllowed || view.StopEvidenceScope != adapter.AgentSessionStopScope {
		return nil, errors.New("original structured stop not confirmed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = adapter.VerifyStopped(ctx, b.LifecycleSetup{Snapshot: record.Snapshot, ProfileData: record.Profile, ProfileID: record.ID, AcceptedConfigVersion: record.InputDigest, ProcessHost: process.New(30 * time.Second)})
	if err != nil {
		return nil, err
	}
	return map[string]any{"run_id": view.RunID, "scope": view.StopEvidenceScope, "original_session": run.Binding.SessionID, "original_close_receipt": run.Binding.CloseReceipt, "session_not_created": run.Binding.SessionNotCreated, "additional_resource_cleanup_verified": false}, nil
}
func structuredInputFingerprint(t *testing.T, m livetest.Manifest, view contracts.RunView) string {
	t.Helper()
	_, run, err := originalStructuredRecord(m, view)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := livetest.FrozenStructuredFingerprint(run)
	if view.StopStatus != "stopped" {
		_, history, readErr := structuredNativeState(m, view)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if history.Fingerprint != fingerprint {
			t.Fatal("native inputs differ from original durable identity")
		}
	}
	if structuredPipe == nil {
		return fingerprint
	}
	rows, err := structuredPipe.Calls(view.Execution.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := livetest.VerifyStructuredInputCalls(rows, run); err != nil {
		t.Fatal(err)
	}
	var inputs []livetest.StructuredCall
	for _, row := range rows {
		if row.Method == "agentSession.create" || row.Method == "agentSession.send" {
			inputs = append(inputs, livetest.StructuredCall{Method: row.Method, ID: row.ID, Params: row.Params})
		}
	}
	raw, _ := json.Marshal(inputs)
	return fingerprint + ":" + string(raw)
}
func setupStructuredPipe(t *testing.T, m livetest.Manifest) {
	t.Helper()
	if m.Execution.Adapter != "orca-terminal" || m.Execution.Target.Kind != "local" || m.Driver == "execution-stability" {
		return
	}
	original := os.Getenv("ORCA_USER_DATA_PATH")
	if original == "" {
		original = filepath.Join(os.Getenv("APPDATA"), "orca")
	}
	audit, err := livetest.StartStructuredPipeAudit(t.TempDir(), original, filepath.Join(*evidenceFlag, "structured-calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	structuredPipe = audit
	t.Cleanup(func() {
		structuredPipe = nil
		if err := audit.Close(); err != nil {
			t.Error(err)
		}
	})
}
func runnerAuditEnvironment() []string {
	if structuredPipe == nil {
		return os.Environ()
	}
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "ORCA_USER_DATA_PATH") && !strings.EqualFold(key, "SR_ORCA_AUDIT_ORIGINAL_USER_DATA") {
			env = append(env, entry)
		}
	}
	return append(env, "ORCA_USER_DATA_PATH="+structuredPipe.Root, "SR_ORCA_AUDIT_ORIGINAL_USER_DATA="+structuredPipe.OriginalRoot)
}
