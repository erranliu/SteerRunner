package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func environmentEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(context.Background(), dir, store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.Shutdown(ctx)
		_ = store.Close()
	})
	return e
}

func environmentRequest(t *testing.T, id string) contracts.Request {
	t.Helper()
	v := contracts.CheckInput{Repository: contracts.Repository{URL: "https://github.com/owner/repo.git", BaseRevision: strings.Repeat("a", 40), Remote: "origin", BaseBranch: "main", LocalPath: t.TempDir()}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"executable":"missing-codex.exe"}`)}}
	raw, _ := json.Marshal(v)
	return contracts.Request{Protocol: contracts.Protocol, RequestID: id, Input: raw}
}

func TestEnvironmentCheckAcceptedFailureOriginalCommandsAndRestart(t *testing.T) {
	e := environmentEngine(t)
	request := environmentRequest(t, "original-start")
	response := e.HandleEnvironmentCheck("environment-check start", "", "", &request)
	if !response.OK {
		t.Fatal(response)
	}
	receipt := response.Data.(contracts.CheckReceipt)
	if receipt.CheckID == nil {
		t.Fatal(receipt)
	}
	id := *receipt.CheckID
	until := time.Now().Add(10 * time.Second)
	for {
		r := e.checkSnapshot(id)
		if r.View.CleanupStatus == "completed" {
			break
		}
		if time.Now().After(until) {
			t.Fatal(r)
		}
		time.Sleep(10 * time.Millisecond)
	}
	inspection := e.HandleEnvironmentCheck("environment-check inspect", "", request.RequestID, nil)
	v := inspection.Data.(contracts.CheckInspection)
	if !inspection.OK || v.Command.Command != "environment-check start" || v.Check.CheckID != id || v.Check.Result != "failed" || v.Check.Items[0].Stage != "workspace_preparation_failed" {
		t.Fatal(inspection)
	}
	if runs, err := e.store.Runs(); err != nil || len(runs) != 0 {
		t.Fatal("check fabricated a development Run", runs, err)
	}
	retry := e.HandleEnvironmentCheck("environment-check start", "", "", &request)
	raw, _ := json.Marshal(retry.Data)
	var retried contracts.CheckReceipt
	_ = json.Unmarshal(raw, &retried)
	if !retry.OK || retried.CheckID == nil || *retried.CheckID != id {
		t.Fatal(retry)
	}
	conflict := request
	conflict.Input = json.RawMessage(`{}`)
	if r := e.HandleEnvironmentCheck("environment-check start", "", "", &conflict); r.Error == nil || r.Error.Code != "request_conflict" {
		t.Fatal(r)
	}
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "original-stop", Input: canonical(contracts.CheckStopInput{CheckID: id, ExpectedRevision: v.Check.Revision})}
	if r := e.HandleEnvironmentCheck("environment-check stop", "", "", &stop); !r.OK {
		t.Fatal(r)
	}
	for {
		inspection = e.HandleEnvironmentCheck("environment-check inspect", "", stop.RequestID, nil)
		v = inspection.Data.(contracts.CheckInspection)
		if v.Command.ActionStatus == "completed" {
			break
		}
		if time.Now().After(until) {
			t.Fatal(v)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if v.Command.Command != "environment-check stop" || v.Command.RequestID != stop.RequestID || v.Command.CommandID == receipt.CommandID {
		t.Fatal("stop query impersonated start", v)
	}
	stale := contracts.Request{Protocol: contracts.Protocol, RequestID: "stale-stop", Input: stop.Input}
	if r := e.HandleEnvironmentCheck("environment-check stop", "", "", &stale); r.Error == nil || r.Error.Code != "stale_revision" {
		t.Fatal(r)
	}
	denied := e.HandleEnvironmentCheck("environment-check inspect", "", stale.RequestID, nil).Data.(contracts.CheckInspection)
	if denied.Command.ReceiptStatus != "rejected" || denied.Command.Error.Code != "stale_revision" || denied.Check != nil {
		t.Fatal(denied)
	}
	before := e.checkSnapshot(id)
	for i := 0; i < 3; i++ {
		if !e.HandleEnvironmentCheck("environment-check inspect", id, "", nil).OK {
			t.Fatal("inspect")
		}
	}
	if e.checkSnapshot(id).View.Revision != before.View.Revision {
		t.Fatal("inspect changed state")
	}
	// A new Engine reads reports, but never replays accepted input.
	other, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer other.cancel()
	if r := other.HandleEnvironmentCheck("environment-check inspect", id, "", nil); !r.OK || r.Data.(contracts.CheckInspection).Check.InputDigest != before.View.InputDigest {
		t.Fatal(r)
	}
}

func TestEnvironmentCheckUnsupportedIsQueryableWithoutCheck(t *testing.T) {
	e := environmentEngine(t)
	request := environmentRequest(t, "unsupported")
	v := contracts.DecodeCheckInput(request.Input)
	v.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "remote"}, Settings: json.RawMessage(`{"access_mode":"full_access"}`)}
	request.Input = canonical(v)
	r := e.HandleEnvironmentCheck("environment-check start", "", "", &request)
	if r.Error == nil || r.Error.Code != "unsupported_capability" {
		t.Fatal(r)
	}
	query := e.HandleEnvironmentCheck("environment-check inspect", "", request.RequestID, nil)
	inspection := query.Data.(contracts.CheckInspection)
	if !query.OK || inspection.Check != nil || inspection.Command.Error.Code != "unsupported_capability" || inspection.Command.CheckID != nil {
		t.Fatal(query)
	}
	if len(e.checks) != 0 {
		t.Fatal("unsupported target created resources")
	}
	missing := e.HandleEnvironmentCheck("environment-check inspect", "", "unknown", nil)
	if missing.Error.Code != "request_not_found" {
		t.Fatal(missing)
	}
}

type environmentProbePort struct {
	checkPort
	t                 *testing.T
	e                 *Engine
	executes, replies int
	fail              bool
	reject            bool
	denied            bool
}

func (p *environmentProbePort) Execute(_ context.Context, action string, r contracts.CheckResources) (map[string]string, error) {
	checks, err := p.e.store.Checks()
	if err != nil || len(checks) != 1 {
		p.t.Fatal(err)
	}
	last := checks[0].Operations[len(checks[0].Operations)-1]
	if last.Action != action || last.Status != b.OperationInFlight || len(last.Request) == 0 {
		p.t.Fatal("write preceded durable original intent", last)
	}
	p.executes++
	if p.denied {
		return map[string]string{"failure_stage": "authorization_denied"}, errors.New("native_probe_rejected")
	}
	if p.reject {
		return nil, errors.New("native_probe_failed")
	}
	if p.fail {
		return nil, errors.New("native_probe_outcome_unknown")
	}
	return map[string]string{"pushed": "true"}, nil
}

func TestEnvironmentCheckExternalFailureCannotHideObservedObject(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncertain_exit", true: "authorization_denied"}[denied], func(t *testing.T) {
			e := environmentEngine(t)
			request := environmentRequest(t, "fixture")
			r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: contracts.DecodeCheckInput(request.Input), InputDigest: "input", ResourceDigest: "resource", Items: contracts.CheckManifest(false), Actual: contracts.CheckContext{AttemptID: "attempt", Generation: "generation", SessionID: "session", TurnID: "turn"}, ExecutionDeadline: time.Now().Add(time.Minute), CleanupDeadline: time.Now().Add(2 * time.Minute)}, Nonce: "nonce", Allowed: true, CommandStates: map[string]string{}}
			for i := 0; i < 6; i++ {
				r.View.Items[i].Status = "passed"
			}
			e.mu.Lock()
			err := e.commitCheckLocked(r, nil)
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			p := &environmentProbePort{t: t, e: e, reject: true, denied: denied}
			w := &checkWorker{port: p}
			_ = e.executeCheckProbe(context.Background(), "original", w, codex.EnvironmentCall{CallID: "native", Action: "base_push", Nonce: "nonce"})
			next := e.checkSnapshot("original")
			wantStatus := "unverified"
			if denied {
				wantStatus = "failed"
			}
			if next.Operations[0].Status != b.OperationUnknown || next.View.Items[6].Status != wantStatus {
				t.Fatal("command error hid possible external effect", next.View)
			}
			// Also reject a formerly saved contradictory settlement projection.
			next.Operations[0].Status = b.OperationRejected
			next.View.CleanupStatus = "completed"
			corrected := reconcileCheckOwnership(next)
			if corrected == nil || corrected.View.CleanupStatus != "residual" || corrected.Operations[0].Status != b.OperationUnknown {
				t.Fatal("observed object was declared settled")
			}
		})
	}
}

func TestEnvironmentCheckLocalOwnershipAndRecreatedPath(t *testing.T) {
	for _, scenario := range []string{"owned", "foreign_marker", "already_removed"} {
		t.Run(scenario, func(t *testing.T) {
			e := environmentEngine(t)
			root := checkDirectory(e.dir, "original")
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			marker := "original\nnonce\n"
			if scenario == "foreign_marker" {
				marker = "someone_else\nforeign\n"
			}
			if err := os.WriteFile(filepath.Join(root, ".check-owner"), []byte(marker), 0600); err != nil {
				t.Fatal(err)
			}
			r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", InputDigest: "input", ResourceDigest: "resource", CleanupDeadline: time.Now().Add(time.Minute), Resources: contracts.CheckResources{LocalRemoved: scenario == "already_removed"}}, Nonce: "nonce", DirectoryOwned: true, CommandStates: map[string]string{}}
			e.mu.Lock()
			err := e.commitCheckLocked(r, nil)
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			e.finishCheck("original", &checkWorker{})
			_, statErr := os.Stat(root)
			if scenario == "owned" && !os.IsNotExist(statErr) {
				t.Fatal("owned directory was not removed")
			}
			if scenario != "owned" && statErr != nil {
				t.Fatal("foreign or recreated directory was removed")
			}
		})
	}
}

func TestEnvironmentCheckFreshSettlementPreservesCompletedCapability(t *testing.T) {
	e := environmentEngine(t)
	id := "original"
	root := checkDirectory(e.dir, id)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".check-owner"), []byte(id+"\nnonce\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: id, InputDigest: "input", ResourceDigest: "resource", Status: "stopping", Result: "passed", CleanupStatus: "residual", CleanupDeadline: time.Now().Add(time.Minute), Resources: contracts.CheckResources{BaseDeleted: true, HeadDeleted: true, PRClosed: true, WriterStopped: true}}, AgentSettled: true, Nonce: "nonce", DirectoryOwned: true, NativeRoot: &b.ProcessIdentity{Kind: "os", PID: 123}, CommandStates: map[string]string{}, Operations: []b.Operation{{Action: "base_push", Status: b.OperationConfirmed}, {Action: "head_push", Status: b.OperationConfirmed}, {Action: "pr_create", Status: b.OperationConfirmed}}}
	e.mu.Lock()
	err := e.commitCheckLocked(r, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.finishCheck(id, &checkWorker{})
	next := e.checkSnapshot(id)
	if next.View.CleanupStatus != "completed" || next.View.Result != "passed" || !next.View.Resources.LocalRemoved {
		t.Fatal("fresh cleanup lost original settlement or rewrote capability", next.View)
	}
}
func (p *environmentProbePort) Verify(context.Context, string, contracts.CheckResources) (map[string]string, error) {
	return map[string]string{"sha": strings.Repeat("a", 40)}, nil
}
func (p *environmentProbePort) Reply(codex.EnvironmentCall, string) error { p.replies++; return nil }

func TestEnvironmentCheckDuplicateAndUnknownNativeTools(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirmed", true: "lost_response"}[unknown], func(t *testing.T) {
			e := environmentEngine(t)
			request := environmentRequest(t, "fixture")
			r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: contracts.DecodeCheckInput(request.Input), InputDigest: "input", ResourceDigest: "resource", Items: contracts.CheckManifest(false), Actual: contracts.CheckContext{AttemptID: "attempt", Generation: "generation", SessionID: "session", TurnID: "turn"}, ExecutionDeadline: time.Now().Add(time.Minute)}, Nonce: "nonce", Allowed: true, CommandStates: map[string]string{}}
			for i := 0; i < 6; i++ {
				r.View.Items[i].Status = "passed"
			}
			e.mu.Lock()
			err := e.commitCheckLocked(r, nil)
			e.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			p := &environmentProbePort{t: t, e: e, fail: unknown}
			w := &checkWorker{port: p}
			call := codex.EnvironmentCall{CallID: "first", Action: "base_push", Nonce: "nonce"}
			_ = e.executeCheckProbe(context.Background(), "original", w, call)
			call.CallID = "second-same-turn"
			_ = e.executeCheckProbe(context.Background(), "original", w, call)
			if p.executes != 1 || len(e.checkSnapshot("original").Operations) != 1 {
				t.Fatal("duplicate write dispatched", p.executes)
			}
			if unknown && e.checkSnapshot("original").View.Items[6].Status != "unverified" {
				t.Fatal("unknown became passed")
			}
			// A late request from a revoked generation may not dispatch.
			_ = e.updateCheck("original", func(r *runstate.CheckRecord) { r.Allowed = false })
			call.Action = "commit"
			_ = e.executeCheckProbe(context.Background(), "original", w, call)
			if p.executes != 1 {
				t.Fatal("revoked tool dispatched")
			}
		})
	}
}

type environmentCleanupPort struct {
	checkPort
	steps []string
}

func (p *environmentCleanupPort) Cleanup(_ context.Context, action string, r contracts.CheckResources) (contracts.CheckResources, error) {
	p.steps = append(p.steps, action)
	switch action {
	case "cleanup_pr_close":
		r.PRClosed = true
	case "cleanup_head":
		r.HeadDeleted = true
	case "cleanup_base":
		r.BaseDeleted = true
	}
	return r, nil
}
func (p *environmentCleanupPort) Stop(context.Context) bool {
	p.steps = append(p.steps, "stop_original_writer")
	return true
}

func TestEnvironmentCheckSettlementKeepsCapabilityAndOriginalIdentity(t *testing.T) {
	e := environmentEngine(t)
	request := environmentRequest(t, "cleanup-fixture")
	value := contracts.DecodeCheckInput(request.Input)
	value.Publication = &contracts.Publication{Provider: "command"}
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: value, InputDigest: "input", ResourceDigest: "resource", Items: contracts.CheckManifest(true), CleanupDeadline: time.Now().Add(time.Minute), Status: "unverified", Result: "unverified", Resources: contracts.CheckResources{BaseRef: "refs/heads/steerline-check/original/base", HeadRef: "refs/heads/steerline-check/original/head", BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), BaseOwned: true, HeadOwned: true, PRNumber: 17, PRURL: "https://github.com/owner/repo/pull/17"}}, CommandStates: map[string]string{}, Operations: []b.Operation{{Action: "base_push", Status: b.OperationConfirmed}, {Action: "head_push", Status: b.OperationConfirmed}, {Action: "pr_create", Status: b.OperationConfirmed}}}
	e.mu.Lock()
	err := e.commitCheckLocked(r, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p := &environmentCleanupPort{}
	e.finishCheck("original", &checkWorker{port: p})
	if strings.Join(p.steps, ",") != "cleanup_pr_close,cleanup_head,cleanup_base,stop_original_writer" {
		t.Fatal("settlement order changed", p.steps)
	}
	next := e.checkSnapshot("original")
	if next.View.CleanupStatus != "completed" || next.View.Result != "unverified" || next.View.Items[11].Status != "unverified" || next.View.Resources.PRNumber != 17 {
		t.Fatal("deterministic cleanup fabricated Agent capability", next.View)
	}
	for i, action := range []string{"cleanup_pr_close", "cleanup_head", "cleanup_base"} {
		if next.Operations[3+i].Action != action || next.Operations[3+i].Status != b.OperationConfirmed {
			t.Fatal("external settlement intent missing", next.Operations)
		}
	}
}

func TestEnvironmentCheckRestartReconcilesWithoutNewInput(t *testing.T) {
	e := environmentEngine(t)
	request := environmentRequest(t, "restart-fixture")
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: contracts.DecodeCheckInput(request.Input), InputDigest: "input", ResourceDigest: "resource", Items: contracts.CheckManifest(false), Status: "running", Result: "unverified", CleanupStatus: "pending", ExecutionDeadline: time.Now().Add(time.Minute), CleanupDeadline: time.Now().Add(2 * time.Minute)}, Allowed: true, CommandStates: map[string]string{}, Operations: []b.Operation{{ID: "original-push", Action: "base_push", Status: b.OperationUnknown}}}
	e.mu.Lock()
	err := e.commitCheckLocked(r, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	restarted.Start()
	until := time.Now().Add(5 * time.Second)
	for {
		next := restarted.checkSnapshot("original")
		if next.View.CleanupStatus == "residual" {
			if next.Allowed || len(next.Operations) != 1 || next.Operations[0].ID != "original-push" {
				t.Fatal("restart replayed or replaced unknown operation", next)
			}
			break
		}
		if time.Now().After(until) {
			t.Fatal(next)
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := restarted.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if restarted.ResourceSettlementError() == nil {
		t.Fatal("service settlement hid unknown Check operation")
	}
	if proof := VerifyStopped(ctx, e.dir, nil, restarted.checkSnapshot("original")); proof.ResourcesSettled {
		t.Fatal("verify-stopped ignored unresolved Check resources")
	}
}

func TestEnvironmentCheckOperationUsesFrozenExecutionHost(t *testing.T) {
	for _, remote := range []bool{false, true} {
		e := environmentEngine(t)
		value := contracts.DecodeCheckInput(environmentRequest(t, "target").Input)
		if remote {
			value.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "environment", Environment: "original-remote"}, Settings: json.RawMessage(`{"transport":{"executable":"fixed-orca.exe"},"agent_launch":{"executable":"codex.exe","runtime_executable":"runtime.exe"}}`)}
		}
		r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: value, Actual: contracts.CheckContext{AttemptID: "attempt"}}, CommandStates: map[string]string{}}
		e.mu.Lock()
		err := e.commitCheckLocked(r, nil)
		e.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		index, err := e.checkIntent("original", "prepare_workspace", "original-call", map[string]string{"identity": "original"})
		if err != nil {
			t.Fatal(err)
		}
		op := e.checkSnapshot("original").Operations[index]
		if remote {
			if op.Backend != b.BackendOrca || op.Protocol != b.ProtocolOrcaTerminalV3 || op.Target.ExecutionHost != "original-remote" || op.Target.ExternalRuntime != "original-remote" {
				t.Fatalf("remote operation mislabeled as caller-local: %+v", op)
			}
		} else if op.Backend != b.BackendLocal || op.Protocol != b.ProtocolLocalCodexAppServer || op.Target.ExecutionHost != "local" {
			t.Fatalf("local operation target changed: %+v", op)
		}
	}
}

func TestEnvironmentCheckCompletedRemoteStopKeepsConfirmedCleanup(t *testing.T) {
	e := environmentEngine(t)
	value := contracts.DecodeCheckInput(environmentRequest(t, "completed-remote").Input)
	value.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "environment", Environment: "original-remote"}, Settings: json.RawMessage(`{"transport":{"executable":"missing-orca.exe"},"agent_launch":{"executable":"codex.exe","runtime_executable":"runtime.exe"}}`)}
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Input: value, Status: "stopping", Result: "passed", CleanupStatus: "completed", CleanupDeadline: time.Now().Add(time.Minute), Resources: contracts.CheckResources{PRClosed: true, BaseDeleted: true, HeadDeleted: true, WriterStopped: true, LocalRemoved: true}}, AgentSettled: true, Endpoint: "closed-original-actor", NativeRoot: &b.ProcessIdentity{RuntimeID: "original-runtime", Terminal: "original-terminal"}, CommandStates: map[string]string{}}
	e.mu.Lock()
	err := e.commitCheckLocked(r, nil)
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.finishCheck("original", &checkWorker{})
	result := e.checkSnapshot("original")
	if result.View.CleanupStatus != "completed" || !result.View.Resources.WriterStopped || !result.View.Resources.LocalRemoved || len(result.Operations) != 0 {
		t.Fatalf("confirmed cleanup regressed after stopping a completed remote check: %+v", result)
	}
}
