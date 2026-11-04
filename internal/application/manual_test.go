package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

type manualPort struct {
	operationCommitPort
	unknown, lost    bool
	entered, release chan struct{}
}

func (p *manualPort) Reconcile(ctx context.Context, r b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	p.reconciles++
	p.requests = append(p.requests, r.Clone())
	if p.entered != nil {
		close(p.entered)
		select {
		case <-p.release:
		case <-ctx.Done():
			return b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}, nil, ctx.Err()
		}
	}
	disposition := b.OperationConfirmed
	if p.unknown {
		disposition = b.OperationUnknown
	}
	return b.LifecycleReceipt{Operation: r.Operation, Disposition: disposition}, nil, nil
}
func (p *manualPort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{SessionAvailable: true, ResourceAvailable: !p.lost, ProcessAbsent: p.lost, SessionIdle: true}, nil
}
func manualFixture(t *testing.T) (*Engine, *worker, *manualPort, *sql.DB) {
	t.Helper()
	e, w, db := operationEngine(t)
	p := &manualPort{}
	w.port = p
	r := e.runs["run"].Clone()
	r.Snapshot.Session.SessionID = "original-session"
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "orca_terminal", Terminal: "original-terminal"}
	if err := e.commitLocked(r, nil, "fixture_original_resource"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.saveIntent("run", r, b.LifecycleCandidate{Action: b.LifecycleCreateResource, Input: b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Objective: "original"}}}); err != nil {
		t.Fatal(err)
	}
	r = e.runs["run"].Clone()
	r.Intents[0].Request.Operation.StartedAt = time.Now().Add(-time.Minute)
	if err := e.commitLocked(r, nil, "fixture_elapsed_original_budget"); err != nil {
		t.Fatal(err)
	}
	e.advance("run", w)
	if e.runs["run"].Phase != manualIntervention || !e.runs["run"].View().Controls.CanResume || p.reconciles != 0 {
		t.Fatal("deadline did not produce manual exit")
	}
	return e, w, p, db
}
func manualResume(t *testing.T, e *Engine, request string) (contracts.Request, string) {
	t.Helper()
	input, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
	r := contracts.Request{Protocol: contracts.Protocol, RequestID: request, Input: input}
	response := e.Handle("resume", "run", "", "", "", 0, &r)
	if !response.OK {
		t.Fatal(response.Error)
	}
	return r, response.Data.(map[string]any)["command_id"].(string)
}
func TestSRManualReconciliationExplicitRetryReplayAndLostResource(t *testing.T) {
	for _, mode := range []string{"resolved", "unknown", "lost"} {
		t.Run(mode, func(t *testing.T) {
			e, w, p, _ := manualFixture(t)
			p.unknown = mode == "unknown"
			p.lost = mode == "lost"
			original := e.runs["run"].Intents[0].Request.Clone()
			request, command := manualResume(t, e, "manual-one")
			e.advance("run", w)
			r := e.runs["run"]
			want := "completed"
			if mode != "resolved" {
				want = "failed"
			}
			if r.CommandStates[command] != want || p.executes != 0 || p.reconciles != 1 || r.Snapshot.Session.AttemptID != original.Snapshot.Session.AttemptID || r.Snapshot.Ref.External != original.Snapshot.Ref.External || len(r.Intents) != 1 {
				t.Fatal("manual reconciliation recreated or repeated original", r.CommandStates, p)
			}
			if mode != "resolved" && (r.Phase != manualIntervention || !r.ResumeRequired || r.Allowed) {
				t.Fatal("failed reconciliation escaped manual pause")
			}
			replay := e.Handle("resume", "run", "", "", "", 0, &request)
			if !replay.OK || replay.Data.(map[string]any)["action_status"] != want {
				t.Fatal("original resume receipt did not replay", replay)
			}
			if mode == "unknown" {
				_, command = manualResume(t, e, "manual-two")
				e.advance("run", w)
				if p.reconciles != 2 || e.runs["run"].CommandStates[command] != "failed" {
					t.Fatal("fresh manual request cannot retry original query")
				}
			}
			for _, q := range p.requests {
				if q.Operation.ID != original.Operation.ID || string(q.Operation.Request) != string(original.Operation.Request) || q.Operation.PayloadDigest != original.Operation.PayloadDigest {
					t.Fatal("manual query replaced original operation")
				}
			}
		})
	}
}
func TestSRManualReconciliationStopWinsDuringRead(t *testing.T) {
	e, w, p, _ := manualFixture(t)
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	_, command := manualResume(t, e, "manual-stop-race")
	done := make(chan bool, 1)
	go func() { done <- e.advance("run", w) }()
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("query not entered")
	}
	input, _ := json.Marshal(contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-during-query", Input: input}
	accepted := make(chan contracts.Response, 1)
	go func() { accepted <- e.Handle("stop", "run", "", "", "", 0, &stop) }()
	select {
	case response := <-accepted:
		if !response.OK {
			t.Fatal(response.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("manual query held dispatch gate")
	}
	close(p.release)
	<-done
	r := e.runs["run"]
	if r.PendingStop == "" || r.Allowed || !r.ResumeRequired || r.CommandStates[command] == "completed" || p.executes != 0 {
		t.Fatal("late manual query overrode stop")
	}
}
func TestSRManualReconciliationCommitFailureAndRestart(t *testing.T) {
	e, w, p, db := manualFixture(t)
	request, command := manualResume(t, e, "manual-db-failure")
	// The explicit request is durable; receipt and failed-control commits fail.
	failOperationCommit(t, db)
	e.advance("run", w)
	if e.runs["run"].CommandStates[command] != "pending" || !e.runs["run"].Intents[0].NeedsReconciliation() || p.executes != 0 {
		t.Fatal("failed transaction granted authority")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_operation_event`); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	r := e.runs["run"].Clone()
	e.mu.Unlock()
	restarted, err := New(context.Background(), e.dir, e.store, e.logger)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.cancel()
	restarted.workers["run"].port = p
	restarted.advance("run", restarted.workers["run"])
	if restarted.runs["run"].CommandStates[command] != "completed" || p.executes != 0 || p.reconciles != 2 {
		t.Fatal("restart replayed effects instead of original query", r)
	}
	replay := restarted.Handle("resume", "run", "", "", "", 0, &request)
	if !replay.OK {
		t.Fatal(replay)
	}
	_ = w
}
func TestSRManualLateFactsPreserveBusinessPause(t *testing.T) {
	e, w, p, _ := manualFixture(t)
	r := e.runs["run"].Clone()
	r.ManualPause = &runstate.ManualPause{Status: "waiting_input", Phase: "business_question"}
	r.Question = &contracts.QuestionView{ID: "original-question", Text: "choose"}
	if err := e.commitLocked(r, nil, "original_question"); err != nil {
		t.Fatal(err)
	}
	_, command := manualResume(t, e, "manual-business")
	e.advance("run", w)
	r = e.runs["run"]
	if r.CommandStates[command] != "completed" || r.Status != "waiting_input" || r.Question == nil || p.executes != 0 {
		t.Fatal(errors.New("technical resume consumed business pause"))
	}
}

func TestSRManualLostResourceDuringContinuation(t *testing.T) {
	e, w, _ := operationEngine(t)
	p := &manualPort{lost: true}
	r := e.runs["run"].Clone()
	if err := e.qualifyContinuation(r, w, p, true, func(b.LifecycleRecoveryEvidence, error) error { return errors.New("original native resource is gone") }); err == nil {
		t.Fatal("lost resource authorized continuation")
	}
	r = e.runs["run"]
	if r.Phase != manualIntervention || !r.ResumeRequired || r.Allowed || !r.View().Controls.CanResume || p.executes != 0 {
		t.Fatal("resource loss lacks explicit manual exit")
	}
}

func TestSRManualExpiredControlDoesNotRenewQueryOnReload(t *testing.T) {
	e, _, p, _ := manualFixture(t)
	_, command := manualResume(t, e, "manual-expired")
	r := e.runs["run"].Clone()
	// Advance only the fixture's accepted control deadline; no native facts
	// or operation identities change and no product clock switch is introduced.
	r.ManualPause.ResumeDeadline = time.Now().Add(-time.Second)
	if err := e.commitLocked(r, nil, "fixture_control_deadline_elapsed"); err != nil {
		t.Fatal(err)
	}
	next, err := New(context.Background(), e.dir, e.store, e.logger)
	if err != nil {
		t.Fatal(err)
	}
	defer next.cancel()
	w := next.workers["run"]
	w.port = p
	next.advance("run", w)
	if next.runs["run"].CommandStates[command] != "failed" || p.reconciles != 0 || p.executes != 0 || !next.runs["run"].View().Controls.CanResume {
		t.Fatal("restart renewed an expired manual query")
	}
	_, fresh := manualResume(t, next, "manual-after-expiry")
	next.advance("run", w)
	if next.runs["run"].CommandStates[fresh] != "completed" || p.reconciles != 1 || p.executes != 0 {
		t.Fatal("new explicit request cannot reconcile after expiry")
	}
}
