package codexworker

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"strings"
	"testing"
	"time"
)

func TestContentReadOnlyPermissions(t *testing.T) {
	p, err := NativePermissions("windows", "local", "codex", "read_only")
	if err != nil {
		t.Fatal(err)
	}
	if p.Sandbox != "read-only" || p.NetworkAccess || p.ApprovalPolicy != "never" || p.ApprovalsReviewer != "user" {
		t.Fatal(p)
	}
	args := strings.Join(p.Args(), " ")
	for _, s := range []string{"sandbox_mode=read-only", "features.apps=false", "features.plugins=false", "features.hooks=false", "features.multi_agent=false"} {
		if !strings.Contains(args, s) {
			t.Fatal("missing boundary", s)
		}
	}
	thread, err := ThreadStartParams("C:/fixed", "read-only", "", nil)
	if err != nil || thread["sandbox"] != "read-only" {
		t.Fatal(thread, err)
	}
	turn, err := TurnStartParams("C:/fixed", "thread", "command", "input", "read-only", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	policy := turn["sandboxPolicy"].(map[string]any)
	if policy["type"] != "readOnly" || policy["writableRoots"] != nil {
		t.Fatal(policy)
	}
}

func TestContentInterruptRejectsChangedProcess(t *testing.T) {
	run, attempt := strings.Repeat("b", 32), strings.Repeat("c", 32)
	target, agent := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}, b.AgentSpec{Name: "codex"}
	identity := b.ProcessIdentity{Kind: "os", PID: 123, Created: "original"}
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: b.BackendLocal, Protocol: b.ProtocolLocalCodexAppServer, RunID: run, AttemptID: attempt, Target: target, Agent: agent, WorkspacePath: "C:/fixed", External: identity}, Session: b.SessionContext{ProjectID: strings.Repeat("a", 32), WorkflowRunID: run, ExecutionID: run, AttemptID: attempt, Agent: agent, Target: target, CurrentTurnID: "00000000-0000-0000-0000-000000000001"}, ConfigurationDigest: "fixed", ResourceGeneration: strings.Repeat("d", 32)}
	intent, err := operation.New(run, s, b.LifecycleCandidate{Action: b.LifecycleStopObservation, Input: b.LifecycleInput{Stop: &b.StopInput{TurnID: s.Session.CurrentTurnID}}}, strings.Repeat("e", 32), strings.Repeat("f", 32), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	l := &ExecutionLifecycle{Backend: Backend{Process: ProcessPort{Inspect: func(id *b.ProcessIdentity) string {
		calls++
		if *id != identity {
			t.Fatal("interrupt changed its original process target")
		}
		return "process identity changed"
	}}}}
	receipt, observer, err := l.Execute(context.Background(), intent.Request)
	if err == nil || receipt.Disposition != b.OperationRejected || observer != nil || calls != 1 {
		t.Fatal("unverified interrupt reached native dispatch", receipt, err, calls)
	}
}
