package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func TestFrozenBudgetMissingDoesNotGateOriginalResults(t *testing.T) {
	for _, id := range []string{"conversation", "generate", workflow.SingleExecution} {
		for _, mode := range []string{"accepted-waiting-start", "paused-resume", "stopped-late-final"} {
			t.Run(id+"/"+mode, func(t *testing.T) {
				var e *Engine
				if id == workflow.SingleExecution {
					e = singleExecutionFixture(t)
				} else {
					e, _ = contentEngine(t, id)
				}
				evidence := contentEvidence(e, "original verified final", "succeeded", true)
				r := e.runs["run"].Clone()
				r.Workflow.StageInputs[0].Budget = nil
				r.Snapshot.PendingTurn = nil // native acceptance consumed this input
				r.Status, r.Phase = "running", "observing"
				if mode == "accepted-waiting-start" {
					s := &r.Snapshot.Session
					s.PendingCommandID, s.PendingTurnID = evidence.CommandID, evidence.TurnID
					s.CurrentCommandID, s.CurrentTurnID = "", ""
					r.NativeStarted = false
					r.Phase = "input_accepted_waiting_start"
				} else if mode == "paused-resume" {
					r.Allowed, r.ResumeRequired = false, true
				}
				if err := e.commitLocked(r, nil, "missing_original_budget_fixture"); err != nil {
					t.Fatal(err)
				}
				before := r.Clone()
				restarted, err := New(context.Background(), e.dir, e.store, e.logger)
				if err != nil {
					t.Fatal("missing budget prevented restart", err)
				}
				t.Cleanup(restarted.cancel)
				e = restarted
				w := e.workers["run"]
				p := &consumedObservationPort{evidence: evidence}
				w.port = p
				if !reflect.DeepEqual(before, e.runs["run"]) {
					t.Fatal("load repaired budget or changed original facts")
				}
				commands := []string{"inspect", "events"}
				if id != workflow.SingleExecution {
					commands = append(commands, "history")
				}
				for _, command := range commands {
					if response := e.Handle(command, "run", "", "", "", 100, nil); !response.OK {
						t.Fatal("missing budget prevented read", command, response)
					}
				}
				if mode == "accepted-waiting-start" {
					err := e.acceptEvent("run", b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStarted, Source: "native", SourceEventID: "original-start", SessionID: evidence.SessionID, StartedTurnID: evidence.TurnID}})
					if err != nil || !e.runs["run"].NativeStarted {
						t.Fatal("missing budget blocked original start acknowledgement", err)
					}
				} else if mode == "paused-resume" {
					response := contentControl(e, "resume", "resume-original", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
					if id == workflow.SingleExecution {
						if response.OK || response.Error.Code != "unsupported_capability" {
							t.Fatal("single execution gained resume authority", response)
						}
					} else {
						if !response.OK {
							t.Fatal("missing budget blocked original resume", response)
						}
						e.advance("run", w)
						if e.runs["run"].Snapshot.PendingTurn != nil || e.runs["run"].NextStage != nil {
							t.Fatal("resume recreated consumed input")
						}
					}
				}
				stopCommand := ""
				if mode == "stopped-late-final" {
					response := contentControl(e, "stop", "stop-before-final", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
					if !response.OK {
						t.Fatal(response)
					}
					stopCommand = e.runs["run"].PendingStop
					e.finish("run", evidence)
					if e.runs["run"].PendingStop != stopCommand {
						t.Fatal("late final replaced original stop")
					}
					e.advance("run", w)
					if e.runs["run"].StopStatus != "stopped" || e.runs["run"].CommandStates[stopCommand] != "completed" {
						t.Fatal("missing budget prevented original stop settlement")
					}
				} else {
					e.advance("run", w)
					if p.reads != 1 {
						t.Fatal("missing budget prevented original observation", p.reads)
					}
				}
				after := e.runs["run"]
				if after.ContentResult == nil || after.ContentResult.Status != "succeeded" || after.ContentResult.Source.CommandID != evidence.CommandID || after.ContentResult.Source.TurnID != evidence.TurnID || after.LastFinal == nil || !after.LastFinal.Verified {
					t.Fatal("missing budget lost the original final", after.ContentResult)
				}
				if p.sends != 0 || len(after.Workflow.StageInputs) != 1 || after.Workflow.StageInputs[0].Budget != nil || !reflect.DeepEqual(before.Intents, after.Intents) {
					t.Fatal("original result read dispatched input or repaired original state")
				}
				rows, err := e.store.Runs()
				if err != nil || rows[0].Workflow.StageInputs[0].Budget != nil || rows[0].ContentResult == nil {
					t.Fatal("original result not durable without budget", err)
				}
			})
		}
	}
}
