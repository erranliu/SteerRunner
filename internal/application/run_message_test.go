package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"io"
	"log"
	"testing"
)

type noMessageProbePort struct{ auditPort }

func (noMessageProbePort) VerifyManualRecovery(context.Context, b.LifecycleSnapshot) (b.LifecycleRecoveryEvidence, error) {
	return b.LifecycleRecoveryEvidence{}, errors.New("message must not probe the original session")
}

func TestDevelopmentRunMessagesRetainOriginalWorkflow(t *testing.T) {
	for _, flow := range []string{"development", "development-audited"} {
		t.Run(flow, func(t *testing.T) {
			e, w := contentEngine(t, "conversation")
			e.finish("run", contentEvidence(e, "original PR result", "succeeded", true))
			r := e.runs["run"].Clone()
			p, _ := workflow.Lookup(flow, 2)
			r.Request.Workflow = contracts.Workflow{ID: flow, Version: 2}
			r.Request.Conversation = nil
			r.Request.Execution.Settings = []byte(`{"access_mode":"full_access"}`)
			r.Workflow.Config, r.Workflow.StepID = &p, workflow.StepPRReady
			r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: "original", ReasoningEffort: "medium"}
			if p.AuditModelRequired {
				r.Workflow.AuditModel = &workflow.ModelSettings{Model: "auditor", ReasoningEffort: "high"}
			}
			r.Workflow.ImplementationResultID, r.Workflow.AuditResultID, r.Workflow.PublicationResultID = "implementation", "audit", "publication"
			r.PR = &contracts.PRFact{URL: "https://example.test/repo/pull/1", State: "open"}
			r.Status = "pr_ready"
			w.port = noMessageProbePort{}
			if err := e.commitLocked(r, nil, "published"); err != nil {
				t.Fatal(err)
			}
			session, attempt, workspace := r.Snapshot.Session.SessionID, r.Snapshot.Session.AttemptID, r.Snapshot.Workspace.Path
			for _, request := range []string{"first-message", "second-message"} {
				r = e.runs["run"]
				if !r.View().Controls.CanMessage || r.View().Controls.CanReply {
					t.Fatal("wrong milestone controls", r.View())
				}
				input := contracts.MessageInput{ExpectedRevision: r.Revision, Text: "caller-only instruction"}
				if response := contentControl(e, "message", request, input); !response.OK {
					t.Fatal(response)
				}
				if e.runs["run"].View().Final != nil {
					t.Fatal("old final presented as current message")
				}
				if !contentControl(e, "message", request, input).OK {
					t.Fatal("original receipt did not replay")
				}
				if response := contentControl(e, "message", request+"-concurrent", contracts.MessageInput{ExpectedRevision: e.runs["run"].Revision, Text: "parallel"}); response.OK {
					t.Fatal("concurrent message accepted")
				}
				if request == "first-message" {
					restarted, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(restarted.cancel)
					restarted.workers["run"] = w
					e = restarted
				}
				before := len(e.runs["run"].Workflow.StageInputs)
				if !e.feedback("run", w, w.port) {
					t.Fatal("original message did not freeze")
				}
				r = e.runs["run"]
				stage := r.Workflow.StageInputs[len(r.Workflow.StageInputs)-1]
				if stage.Prompt != input.Text || stage.Model != "original" || r.Workflow.StepID != workflow.StepPRReady || len(r.Workflow.StageInputs) != before+1 {
					t.Fatal("message gained rules or reran workflow", stage)
				}
				if err := e.checkDispatchSource(r, stage); err != nil {
					t.Fatal("message added material/source preflight", err)
				}
				outcome := "succeeded"
				if request == "second-message" {
					outcome = "unreported"
				}
				if !e.finish("run", contentEvidence(e, "message result", outcome, true)) {
					t.Fatal("message result lost")
				}
				r = e.runs["run"]
				if !r.View().Controls.CanMessage || r.Status != "pr_ready" || r.Snapshot.Session.SessionID != session || r.Snapshot.Session.AttemptID != attempt || r.Snapshot.Workspace.Path != workspace || r.Workflow.AuditResultID != "audit" || r.Workflow.ImplementationResultID != "implementation" || r.Workflow.PublicationResultID != "publication" || r.PR.URL != "https://example.test/repo/pull/1" || r.LastFinal.Text != "message result" || r.ContentResult != nil {
					t.Fatal("original run changed", r.View())
				}
			}
			if response := e.Handle("history", "run", "", "", "", 100, nil); !response.OK {
				t.Fatal("message history unavailable", response)
			}
			r = e.runs["run"].Clone()
			r.Allowed = false
			r.PendingStop = "original-stop"
			if err := e.commitLocked(r, nil, "stop"); err != nil {
				t.Fatal(err)
			}
			if r.View().Controls.CanMessage || contentControl(e, "message", "after-stop", contracts.MessageInput{ExpectedRevision: r.Revision, Text: "no"}).OK {
				t.Fatal("stop allowed new message")
			}
		})
	}
}
