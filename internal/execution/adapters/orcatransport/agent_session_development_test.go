package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestStructuredDevelopmentFollowUp(t *testing.T) {
	for _, scenario := range []string{"normal", "message", "audit-role", "model-unavailable", "lost-response", "active", "reused-turn", "foreign-user"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			l, snapshot, fake := structuredFixture(t)
			l.agentDevelopment = true
			if scenario == "audit-role" {
				l.agentAudited = true
				l.setup.SelectAuditedResult = func(input b.TurnInput, messages []string) (int, error) {
					if input.Prompt == "" || len(messages) == 0 {
						t.Fatal("audited selector lost the original input")
					}
					return len(messages) - 1, nil
				}
			}
			prepared, err := l.agentState(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			prepared.Attempted = true
			snapshot.Implementation, err = l.agentEnvelope(snapshot, prepared)
			if err != nil {
				t.Fatal(err)
			}
			fake.pages[0].Fence = 5
			original, err := l.readStructured(ctx, snapshot)
			if err != nil || original.TurnEvidence == nil {
				t.Fatal("first input not qualified", err)
			}
			first := request(snapshot, b.LifecycleStartTurn, b.LifecycleInput{Turn: snapshot.PendingTurn})
			first.Operation.Request = mustAgentJSON(snapshot.PendingTurn)
			first.Operation.PayloadDigest = digest(first.Operation.Request)
			first.Operation.Status = b.OperationConfirmed
			original.Operation = first.Operation
			original.Disposition = b.OperationConfirmed
			first.Operation.Receipt = mustAgentJSON(original)
			snapshot.Operations = append(snapshot.Operations, first.Operation)
			snapshot.Implementation = original.Implementation
			snapshot.Session.LastStoppedTurnID = original.State.TurnID
			body := snapshot.PendingTurn.Prompt
			if scenario == "audit-role" {
				snapshot.PendingTurn = nil // Confirmed native acceptance consumed it.
				observed, err := l.readStructured(ctx, snapshot)
				if err != nil || observed.TurnEvidence == nil || observed.TurnEvidence.CommandID != original.TurnEvidence.CommandID {
					t.Fatal("audited result lost the committed input", observed, err)
				}
			}
			fake.pages[0].Fence = 7
			fake.pages[0].Working = scenario == "active"

			// A distinct original command can carry the same bytes.
			snapshot.PendingTurn = &b.TurnInput{CommandID: strings.Repeat("b", 32), Prompt: body}
			wantModel, wantEffort := "requested-model", "medium"
			wantFence := int64(7)
			if scenario == "message" {
				snapshot.PendingTurn.Purpose = "message"
				wantFence = 5
			}
			if scenario == "audit-role" || scenario == "model-unavailable" {
				wantModel, wantEffort = "audit-model", "high"
				snapshot.PendingTurn.Model, snapshot.PendingTurn.ReasoningEffort = wantModel, wantEffort
			}
			r := request(snapshot, b.LifecycleStartTurn, b.LifecycleInput{Turn: snapshot.PendingTurn})
			r.Operation.ID = strings.Repeat("a", 32)
			r.Operation.StartedAt = time.UnixMilli(1793680000123)
			writes := 0
			final := `{"outcome":"needs_input","summary":"original result","question":"Continue?"}`
			fake.send = func(send native.AgentSend) error {
				writes++
				if send.Envelope.ClientOperationID != "1793680000123-"+r.Operation.ID || *send.Envelope.ExpectedRuntimeFence != wantFence || send.Body.Blocks[0].Text != body || send.Selection == nil || send.Selection.Model != wantModel || send.Selection.Effort != wantEffort {
					t.Fatal("follow-up lost original wire facts", send)
				}
				if scenario == "model-unavailable" {
					return errors.New("configured audit-model is unavailable")
				}
				userID := "orca:" + send.Envelope.ClientOperationID
				if scenario == "foreign-user" {
					userID = "orca:external"
				}
				turnID := "second-turn"
				if scenario == "reused-turn" {
					turnID = original.State.TurnID
				}
				fingerprint, _ := native.AgentFingerprint("agentSession.send", send.Envelope.SessionID, map[string]any{"body": send.Body})
				if fingerprint == send.Envelope.PayloadFingerprint {
					t.Fatal("submission body fingerprint collapsed into operation envelope fingerprint")
				}
				fake.pages[0].Submissions = append(fake.pages[0].Submissions, native.AgentSubmission{ClientMessageID: send.Envelope.ClientOperationID, PayloadFingerprint: fingerprint, DispatchState: "accepted", ProviderItemID: "second-user"})
				fake.pages[0].Items = append(fake.pages[0].Items,
					native.AgentItem{ItemID: userID, Sequence: 10, Body: mustAgentJSON(send.Body)},
					native.AgentItem{ItemID: "second-turn-item", Sequence: 11, Body: json.RawMessage(`{"kind":"turn","turnId":"` + turnID + `","state":"completed","outcome":"success","userItemId":"` + userID + `","startedAt":30,"completedAt":40}`)})
				for i, text := range []string{"commentary from second turn", final} {
					item := native.AgentItem{ItemID: "second-" + text[:3], Sequence: int64(12 + i), Body: mustAgentJSON(native.AgentMessage{Kind: "message", Role: "assistant", Blocks: []native.AgentTextBlock{{Type: "text", Text: text}}})}
					item.TurnScope.Kind, item.TurnScope.TurnItemID = "turn", "second-turn-item"
					fake.pages[0].Items = append(fake.pages[0].Items, item)
				}
				if scenario == "lost-response" {
					return errors.New("lost original send response")
				}
				return nil
			}
			result, observer, err := l.Execute(ctx, r)
			if scenario == "lost-response" {
				if err == nil || result.Disposition != b.OperationUnknown || observer != nil {
					t.Fatal("lost response did not preserve the uncertain operation", result, err)
				}
				result, observer, err = l.Reconcile(ctx, r)
			}
			if observer != nil {
				defer observer.Close(ctx)
			}
			switch scenario {
			case "model-unavailable":
				if err == nil || !strings.Contains(err.Error(), "audit-model is unavailable") || writes != 1 || result.State.InputAccepted {
					t.Fatal("model failure was hidden or triggered fallback", writes, result, err)
				}
			case "active", "reused-turn", "foreign-user":
				if err == nil || result.State.InputAccepted || scenario == "active" && writes != 0 || scenario != "active" && writes != 1 {
					t.Fatal("unsafe follow-up authorized", result, writes, err)
				}
			default:
				if err != nil || writes != 1 || result.Disposition != b.OperationConfirmed || result.TurnEvidence == nil || result.TurnEvidence.TurnID != "second-turn" || result.TurnEvidence.LastMessage != final {
					t.Fatal("second result lost provenance or repeated a write", result, writes, err)
				}
			}
		})
	}
}
