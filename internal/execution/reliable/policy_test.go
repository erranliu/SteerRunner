package reliable

import (
	"context"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"reflect"
	"testing"
)

type preparationFixture struct {
	caps  b.PreparationCapabilities
	facts b.PreparationFacts
	read  int
}

func (p *preparationFixture) PolicyCapabilities() b.LifecyclePolicyCapabilities {
	return b.LifecyclePolicyCapabilities{Preparation: p.caps}
}
func (p *preparationFixture) PreparationFacts(context.Context, b.LifecycleSnapshot) (b.PreparationFacts, error) {
	p.read++
	return p.facts, nil
}
func preparationSnapshot() b.LifecycleSnapshot {
	return b.LifecycleSnapshot{WorkspacePlan: b.WorkspaceIntent{Path: "original", Branch: "fixed", BaseCommit: "base"}, PendingTurn: &b.TurnInput{CommandID: "command", Prompt: "saved"}}
}

func TestReliablePreparationFacts(t *testing.T) {
	p := &preparationFixture{caps: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismApplicable, Session: b.MechanismApplicable}}
	s := preparationSnapshot()
	for _, tc := range []struct {
		facts  b.PreparationFacts
		action b.LifecycleAction
	}{
		{b.PreparationFacts{Workspace: b.GateRequired}, b.LifecyclePrepareWorkspace},
		{b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateRequired, ResourceRequired: &b.ResourceIntent{Purpose: "native-launch", Objective: "saved-command"}}, b.LifecycleCreateResource},
		{b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateReady, Session: b.GateRequired}, b.LifecyclePrepareSession},
		{b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateReady, Session: b.GateReady}, b.LifecycleStartTurn},
	} {
		p.facts = tc.facts
		next, err := Next(context.Background(), p, s)
		if err != nil || next.Action != tc.action {
			t.Fatalf("facts=%+v next=%+v err=%v", tc.facts, next, err)
		}
		if next.Input.Turn != nil && !reflect.DeepEqual(*next.Input.Turn, *s.PendingTurn) {
			t.Fatal("changed saved input")
		}
	}
}

func TestReliablePreparationApplicability(t *testing.T) {
	s := preparationSnapshot()
	p := &preparationFixture{caps: b.PreparationCapabilities{Workspace: b.MechanismApplicable, Resource: b.MechanismNotApplicable, Session: b.MechanismApplicable}, facts: b.PreparationFacts{Workspace: b.GateReady, Resource: b.GateNotApplicable, Session: b.GateRequired}}
	next, err := Next(context.Background(), p, s)
	if err != nil || next.Action != b.LifecyclePrepareSession {
		t.Fatalf("not applicable resource was not skipped: %+v %v", next, err)
	}
	for _, tc := range []struct {
		name string
		decl b.Applicability
		fact b.PreparationGate
		want b.PreparationGate
	}{
		{"required unsupported", b.MechanismUnsupported, b.GateNotApplicable, b.GateUnsupported},
		{"unknown native evidence", b.MechanismApplicable, b.GateUnverified, b.GateUnverified},
		{"zero native evidence", b.MechanismApplicable, "", b.GateUnverified},
		{"zero declaration", "", b.GateReady, b.GateUnverified},
		{"false opt out", b.MechanismApplicable, b.GateNotApplicable, b.GateUnverified},
		{"conflicting opt out", b.MechanismNotApplicable, b.GateRequired, b.GateUnverified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p.caps.Session, p.facts.Session = tc.decl, tc.fact
			next, err := Next(context.Background(), p, s)
			var gap *GateError
			if !errors.As(err, &gap) || gap.Status != tc.want || next.Action != "" || next.Done {
				t.Fatalf("gate bypassed: %+v %v", next, err)
			}
		})
	}
}

func TestReliableRecoveryAndAuthorization(t *testing.T) {
	e := b.LifecycleRecoveryEvidence{SessionAvailable: true, ResourceAvailable: true}
	if !CanContinueOriginal(e) {
		t.Fatal("original live resource rejected")
	}
	e.ProcessAbsent = true
	if CanContinueOriginal(e) {
		t.Fatal("lost resource authorized restoration")
	}
}

func TestReliableLateFacts(t *testing.T) {
	s := b.SessionContext{CurrentCommandID: "command", CurrentTurnID: "turn", SessionID: "session"}
	e := b.TurnEvidence{CommandID: "command", TurnID: "turn", SessionID: "session", Started: true, Completed: true, Succeeded: true, LastMessage: "final"}
	result, err := NormalizeResult(s, e)
	if err != nil || result.ContentVerified {
		t.Fatal("missing provenance became verified")
	}
	e.SessionID = "replacement"
	if _, err := NormalizeResult(s, e); !errors.Is(err, ErrStale) {
		t.Fatal("replacement accepted")
	}
	value := BindingSnapshot{Target: b.ExecutionTarget{ID: "original"}, SessionID: "session"}
	receipt := b.LifecycleReceipt{Target: &b.ExecutionTarget{ID: "changed"}, State: b.LifecycleStateFacts{SessionID: "replacement"}}
	got, err := MergeReceipt(value, receipt)
	if err == nil || !reflect.DeepEqual(got, value) {
		t.Fatal("invalid partial receipt leaked a mutation")
	}
}

func TestRAReportPolicyUnknown(t *testing.T) {
	input := &b.TurnInput{ReportToolPolicy: "frozen native policy"}
	if QualifyInput(input, b.LifecyclePolicyCapabilities{}) == nil {
		t.Fatal("unknown policy accepted")
	}
	if QualifyInput(input, b.LifecyclePolicyCapabilities{ReportToolPolicy: input.ReportToolPolicy}) != nil {
		t.Fatal("same frozen native policy rejected")
	}
}

// A coarse completion flag cannot supersede an explicit native failure or unknown outcome, even
// when the final contains the workflow's success declaration or a done report.
func TestRANativeFailureDominatesCompletion(t *testing.T) {
	session := b.SessionContext{CurrentCommandID: "command", CurrentTurnID: "turn", SessionID: "session"}
	for _, outcome := range []string{"failed", "cancelled", "unknown"} {
		t.Run(outcome, func(t *testing.T) {
			source := b.TurnEvidence{CommandID: "command", TurnID: "turn", SessionID: "session", Started: true, Completed: true, Succeeded: true, NativeOutcome: outcome, LastMessage: `{"outcome":"done","summary":"claimed success"}`, FinalVerified: true, FinalSource: "native", FinalItemID: "final"}
			got, err := NormalizeResult(session, source)
			if err != nil || got.Succeeded || !got.ContentVerified || got.NativeOutcome != outcome || got.Content != source.LastMessage {
				t.Fatal(got, err)
			}
		})
	}
}

func TestRAUnreportedOutcomeBoundary(t *testing.T) {
	session := b.SessionContext{CurrentCommandID: "command", CurrentTurnID: "turn", SessionID: "session"}
	for _, mode := range []string{"complete", "missing-source", "missing-item", "unverified", "empty", "blank", "failure", "not-collected", "not-started", "not-completed", "other-session"} {
		t.Run(mode, func(t *testing.T) {
			e := b.TurnEvidence{CommandID: "command", TurnID: "turn", SessionID: "session", Started: true, Completed: true, Succeeded: true, NativeOutcome: "unreported", FinalVerified: true, FinalSource: "native", FinalItemID: "original", LastMessage: "original final"}
			switch mode {
			case "missing-source":
				e.FinalSource = ""
			case "missing-item":
				e.FinalItemID = ""
			case "unverified":
				e.FinalVerified = false
			case "empty":
				e.LastMessage = ""
			case "blank":
				e.LastMessage = " \n "
			case "failure":
				e.Failure = "actual failure"
			case "not-collected":
				e.Succeeded = false
			case "not-started":
				e.Started = false
			case "not-completed":
				e.Completed = false
			case "other-session":
				e.SessionID = "other"
			}
			r, err := NormalizeResult(session, e)
			stale := mode == "not-started" || mode == "not-completed" || mode == "other-session"
			if stale {
				if !errors.Is(err, ErrStale) {
					t.Fatal("unbound result admitted", r, err)
				}
				return
			}
			if err != nil || r.NativeOutcome != "unreported" || r.Succeeded != (mode == "complete") {
				t.Fatal("unreported boundary changed", r, err)
			}
		})
	}
}
