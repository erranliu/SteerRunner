//go:build windows && live

package codexexec

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

func TestBoundaryNativeParserResources(t *testing.T) {
	executable, root := os.Getenv("STEERRUNNER_BOUNDARY_NATIVE_CODEX"), os.Getenv("STEERRUNNER_BOUNDARY_NATIVE_EVIDENCE")
	if executable == "" || root == "" {
		t.Skip("explicit native executable and fresh evidence directory required")
	}
	dir := filepath.Join(root, "parser-resources")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := ra.DefaultConfig()
	config.CodexParser.Enabled = true
	config.CodexParser.Executable = executable
	config.CodexParser.Model = os.Getenv("STEERRUNNER_BOUNDARY_NATIVE_MODEL")
	config.CodexParser.ReasoningEffort = "low"
	final := `{"outcome":"done","summary":"The provided fixture audit and validation completed; no repairs were needed and no P0-P2 issues remain.","question":null,"blocker":null,"fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`
	pack := FreezePackage(Package{Config: ra.FrozenConfig{Config: config}, Source: ra.Source{SessionID: "source-session", CommandID: "source-command", TurnID: "source-turn", AttemptID: "source-attempt", InputVersion: 1, FinalDigest: ra.Digest([]byte(final))}, ImplementationResultID: "source-implementation", Final: final})
	payload, _ := json.Marshal(pack)
	identity, err := process.NewServiceFacts().CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "boundary-parser", AttemptID: "attempt"}, Session: b.SessionContext{WorkflowRunID: "boundary-parser", AttemptID: "attempt"}, ConfigurationDigest: ra.Digest(payload), ResourceGeneration: "resource"}
	makeIntent := func(label string) operation.Intent {
		t.Helper()
		i, err := operation.NewParser("boundary-parser", snapshot, b.ParseInput{ResultID: "result-" + label, Payload: string(payload), Generation: label, Deadline: time.Now().Add(150 * time.Second), Owner: identity}, "operation-"+label, "key-"+label, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err = i.MarkParserDispatch(nil); err != nil {
			t.Fatal(err)
		}
		return i
	}
	i := makeIntent("complete")
	resources := &Resources{Root: dir, Started: func(receipt b.LifecycleReceipt) error {
		raw, _ := json.Marshal(receipt)
		return process.New(0).WriteExecutionFile(filepath.Join(dir, "root.json"), raw)
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result := operation.InvokeParser(ctx, resources, resources, i.Request, operation.ExecuteNewIntent)
	if result.Err != nil || resources.Task == nil {
		t.Fatal(result.Err)
	}
	defer func() {
		resources.Task.Cancel()
		select {
		case <-resources.Task.Done():
		case <-time.After(10 * time.Second):
			t.Error("native parser tree exit unverified")
		}
	}()
	select {
	case <-resources.Task.Done():
	case <-ctx.Done():
		t.Fatal("native parser did not settle")
	}
	receipt, done := resources.Task.Result()
	if !done || receipt.ValidateFor(i.Request) != nil || receipt.ParseEvidence == nil || receipt.ParseEvidence.Status != "completed" || !receipt.State.Stopped || !receipt.State.WritersAbsent {
		raw, _ := json.Marshal(receipt)
		t.Fatal("native parser final/settlement unverified", string(raw))
	}
	if _, err = ra.NormalizeParserReport(receipt.ParseEvidence.Content); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(Directory(dir, i.Request), "receipt.json")
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	task := resources.Task
	i.Request.Operation.Status = b.OperationUnknown
	resources.Prior = &receipt
	_ = operation.InvokeParser(ctx, resources, resources, i.Request, operation.ExecuteNewIntent)
	after, err := os.ReadFile(journal)
	if err != nil || resources.Task != task || !bytes.Equal(before, after) {
		t.Fatal("unknown original parser was replayed", err)
	}
	// A committed cancellation still reaches the resource's terminal journal,
	// without releasing a native parser root or consuming another model turn.
	canceled := makeIntent("canceled")
	cctx, ccancel := context.WithCancel(ctx)
	ccancel()
	roots := 0
	cr := &Resources{Root: dir, Started: func(b.LifecycleReceipt) error { roots++; return nil }}
	_ = operation.InvokeParser(cctx, cr, cr, canceled.Request, operation.ExecuteNewIntent)
	if cr.Task == nil {
		t.Fatal("canceled parser lost original settlement path")
	}
	select {
	case <-cr.Task.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("canceled parser did not journal settlement")
	}
	stopped, _ := cr.Task.Result()
	if roots != 0 || !stopped.State.Stopped || !stopped.State.WritersAbsent || stopped.ParseEvidence == nil || stopped.ParseEvidence.Status != "failed" {
		t.Fatal("canceled-before-dispatch parser released a root or lost settlement")
	}
	summary, _ := json.MarshalIndent(map[string]any{"session_id": receipt.ParseEvidence.SessionID, "model": receipt.ParseEvidence.Model, "status": receipt.ParseEvidence.Status, "writers_absent": receipt.State.WritersAbsent, "unknown_replayed": false, "canceled_roots": roots}, "", "  ")
	if err = os.WriteFile(filepath.Join(dir, "native-parser.json"), summary, 0600); err != nil {
		t.Fatal(err)
	}
}
