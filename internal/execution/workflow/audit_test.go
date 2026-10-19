package workflow

import (
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"reflect"
	"testing"
)

const auditPass = `{"outcome":"done","summary":"audited and validated","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`
const auditFixed = `{"outcome":"done","summary":"audited repaired and validated","fixed_count":1,"fixed_issues":[{"severity":"P1","issue":"bug","repair":"fixed branch","evidence":"code.go branch"}],"remaining_issues":[]}`

func auditFixture() (Snapshot, StageInput) {
	p, _ := Lookup(AuditedDevelopment, 2)
	pre := &WorkspaceSource{Qualified: true, Head: "pre", Tree: "pre-tree", Repository: "repo", Worktree: "tree", Branch: "branch", Base: "base"}
	post := *pre
	post.Head = "post"
	post.Tree = "post-tree"
	impl := StageResult{CommandID: "implement", AttemptID: "attempt", StepID: StepImplement, InputVersion: 1, TurnID: "implement-turn", Succeeded: true, FinalVerified: true, Final: "implemented", Workspace: pre}
	value, _ := ra.DecodeReport(`{"outcome":"done","summary":"implemented"}`, "implementation")
	acquired := ra.Result{ID: "implementation-result", Source: Source(impl), Status: "satisfied", Value: &value}
	i := StageInput{StepID: StepAuditCode, CommandID: "audit", AttemptID: "attempt", Version: 2, SourceCommandID: impl.CommandID, SourceTurnID: impl.TurnID, SourceVersion: 1, ImplementationResultID: acquired.ID}
	return Snapshot{Config: &p, StepID: i.StepID, ImplementationResultID: acquired.ID, Results: []ra.Result{acquired}, StageInputs: []StageInput{{CommandID: impl.CommandID, StepID: impl.StepID, Version: 1}, i}, StageResults: []StageResult{impl, {CommandID: i.CommandID, TurnID: "audit-turn", AttemptID: i.AttemptID, StepID: i.StepID, InputVersion: i.Version, Succeeded: true, FinalVerified: true, Final: auditFixed, Workspace: &post}}}, i
}
func TestRAAuditSources(t *testing.T) {
	w, i := auditFixture()
	r := reliable.Result{Succeeded: true, ContentVerified: true, TurnID: "audit-turn", Content: auditFixed}
	n := Route(w, i, r, "")
	if n.Next == nil || n.Next.StepID != StepPublish || n.Acquired.Source.CommandID != "audit" {
		t.Fatal(n)
	}
	w.StageResults[1].Workspace.Tree = "pre-tree"
	n = Route(w, i, r, "")
	if n.Next != nil || n.Acquired.Status != "conflict" {
		t.Fatal("empty commit supported positive repairs", n)
	}
	r.Content = auditPass
	n = Route(w, i, r, "")
	if n.Next == nil {
		t.Fatal("zero repair requires changes", n)
	}
	w.ImplementationResultID = "new-result"
	if n = Route(w, i, r, ""); n.Next != nil {
		t.Fatal("stale audit passed", n)
	}
}
func TestRAAuditPartialAndFallback(t *testing.T) {
	w, i := auditFixture()
	w.StageResults[1].Workspace.Qualified = false
	r := reliable.Result{Succeeded: true, ContentVerified: true, TurnID: "audit-turn", Content: `{"outcome":"needs_input","summary":"unfinished","question":"clarify","fixed_count":1,"fixed_issues":[{"severity":"P2","issue":"a","repair":"b","evidence":"c"}],"remaining_issues":[{"severity":"P1","issue":"d","reason":"need input"}]}`}
	n := Route(w, i, r, "")
	if n.Status != "waiting_input" || n.Acquired.Value.FixedCount == nil || *n.Acquired.Value.FixedCount != 1 || n.Next != nil {
		t.Fatal(n)
	}
	r.Content = "natural-language audit"
	n = Route(w, i, r, "")
	if n.Next == nil || n.Next.StepID != StepAuditReport || n.Next.SourceCommandID != "audit" {
		t.Fatal(n)
	}
	w.Results = append(w.Results, *n.Acquired)
	report := *n.Next
	report.CommandID = "report"
	report.Version = 3
	w.StageInputs = append(w.StageInputs, report)
	r.Content = "invalid"
	if n = Route(w, report, r, ""); n.Next != nil || n.Cursor != StepAuditInput {
		t.Fatal(n)
	}
	r.Content = auditPass
	w.StageResults = append(w.StageResults, StageResult{CommandID: "report", Workspace: &WorkspaceSource{Qualified: true, Head: "new-clean"}})
	if n = Route(w, report, r, ""); n.Next != nil || n.Acquired.Status != "unverified" {
		t.Fatal("report manufactured checkpoint", n)
	}
}

// Independent parsing must attribute a question or blocker to its own operation,
// while retaining the immutable original audit as the business source.
func TestRAParserFieldProvenance(t *testing.T) {
	for _, tc := range []struct{ field, body string }{
		{"question", `{"outcome":"needs_input","summary":"validation incomplete","question":"Provide validation evidence."}`},
		{"blocker", `{"outcome":"blocked","summary":"validation incomplete","blocker":{"category":"prerequisite","action":"validate","error":"missing evidence"}}`},
	} {
		t.Run(tc.field, func(t *testing.T) {
			w, input := auditFixture()
			before := *Result(w, input.CommandID)
			got := RouteParsed(w, input, tc.body, "parser-operation", "")
			if got.Acquired == nil || got.Acquired.Source != Source(before) || got.Cursor != StepAuditInput || got.Next != nil {
				t.Fatal(got)
			}
			found := false
			for _, field := range got.Acquired.Fields {
				if field.Field == tc.field {
					found = true
					if field.Method != "llm_parser" || field.Source != Source(before) || field.Locator != "parser:parser-operation:$/"+tc.field {
						t.Fatal(field)
					}
				}
			}
			if !found || !reflect.DeepEqual(*Result(w, input.CommandID), before) {
				t.Fatal("lost field provenance or replaced native source")
			}
		})
	}
}
