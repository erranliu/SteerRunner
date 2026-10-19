package workflow

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"strings"
	"testing"
)

func plainFixture() (Snapshot, StageInput, reliable.Result) {
	p, _ := Lookup("development", 2)
	i := StageInput{CommandID: "execute", AttemptID: "attempt", StepID: StepImplement, Version: 1}
	return Snapshot{Config: &p, StepID: i.StepID, StageInputs: []StageInput{i}}, i, reliable.Result{Succeeded: true, ContentVerified: true, TurnID: "turn", Content: "实施及必要验证已完成。"}
}
func TestRASuccessOutputRouting(t *testing.T) {
	w, i, r := plainFixture()
	next := Route(w, i, r, "policy")
	if next.Next == nil || next.Next.StepID != StepPublish || next.Acquired == nil || next.Acquired.Source.CommandID != i.CommandID {
		t.Fatalf("marker did not bypass report: %+v", next)
	}
	r.Content = "implemented without declared conclusion"
	next = Route(w, i, r, "policy")
	if next.Next == nil || next.Next.StepID != StepReport || next.Next.SourceVersion != 1 || next.Next.ReportToolPolicy != "policy" {
		t.Fatal("fallback lost source/policy", next)
	}
	w.Results = append(w.Results, *next.Acquired)
	report := *next.Next
	report.CommandID = "report"
	report.Version = 2
	w.StageInputs = append(w.StageInputs, report)
	w.StageResults = append(w.StageResults, StageResult{CommandID: i.CommandID, AttemptID: i.AttemptID, StepID: i.StepID, InputVersion: 1, TurnID: r.TurnID, Succeeded: true, FinalVerified: true, Final: r.Content})
	r.Content = "bad report"
	r.TurnID = "report-turn"
	next = Route(w, report, r, "")
	if next.Next != nil || next.Status != "waiting_input" || next.Cursor != StepImplementInput {
		t.Fatal("report retried or did not wait", next)
	}
	r.Content = `{"outcome":"done","summary":"implemented"}`
	next = Route(w, report, r, "")
	if next.Next == nil || next.Next.StepID != StepPublish || next.Acquired.Source.InputVersion != 1 {
		t.Fatal("report replaced original source", next)
	}
	w.StageResults[0].Succeeded = false
	if next = Route(w, report, r, ""); next.Next != nil || next.Publish {
		t.Fatal("failed original source upgraded")
	}
}
func TestRASemantics(t *testing.T) {
	w, i, r := plainFixture()
	for _, body := range []string{`{"outcome":"needs_input","summary":"incomplete","question":"details?"}`, `{"outcome":"blocked","summary":"unfinished","blocker":{"category":"failure","action":"build","error":"failed"}}`} {
		r.Content = body
		n := Route(w, i, r, "")
		if n.Status != "waiting_input" || n.Cursor != StepImplementInput || n.Next != nil {
			t.Fatal(n)
		}
	}
	r.Content = `{"outcome":"blocked","summary":"unknown","blocker":{"category":"unknown_external","action":"publish","error":"lost response"}}`
	if n := Route(w, i, r, ""); n.Status != "reconciling" || n.Next != nil {
		t.Fatal(n)
	}
	r.Content = "实施及必要验证已完成。"
	r.Succeeded = false
	if n := Route(w, i, r, ""); !n.RecoverOriginal || n.Next != nil {
		t.Fatal(n)
	}
	r.Succeeded = true
	r.ContentVerified = false
	if n := Route(w, i, r, ""); n.AcceptResult || n.Next != nil {
		t.Fatal(n)
	}
}
func TestRADefinitionRename(t *testing.T) {
	w, i, r := plainFixture()
	p := w.Config
	p.ID = "renamed-fixture"
	state := p.States[i.StepID]
	delete(p.States, i.StepID)
	state.SuccessOutput = "fixture finished"
	state.Next = "next-fixture"
	p.States["renamed.execute"] = state
	i.StepID = "renamed.execute"
	w.StepID = i.StepID
	r.Content = "fixture finished"
	n := Route(w, i, r, "")
	if n.Next == nil || n.Next.StepID != "next-fixture" {
		t.Fatal(n)
	}
	prompt, err := RenderInput(w, i)
	if err != nil || !strings.Contains(prompt, "fixture finished") {
		t.Fatal(err, prompt)
	}
}
func TestRAFeedback(t *testing.T) {
	w, _, _ := plainFixture()
	w.StepID = StepImplementInput
	if step, err := FeedbackStep(w, "implement"); err != nil || step != StepImplement {
		t.Fatal(step, err)
	}
	if _, err := FeedbackStep(w, ""); err == nil {
		t.Fatal("resume authorized implementation")
	}
	w.StepID = StepPRReady
	if _, err := FeedbackStep(w, "implement"); err == nil {
		t.Fatal("milestone acquired feedback edge")
	}
}

func TestRAPublicationProgramClue(t *testing.T) {
	w, i, r := plainFixture()
	i.StepID = StepPublish
	i.ImplementationResultID = "impl"
	r.Content = "publication result"
	r.TurnID = "publish-turn"
	implRaw := StageResult{CommandID: "impl-command", TurnID: "impl-turn", InputVersion: 1, AttemptID: "attempt", Final: "implemented", Succeeded: true, FinalVerified: true}
	implValue, _ := ra.DecodeReport(`{"outcome":"done","summary":"implemented"}`, "implementation")
	w.ImplementationResultID = "impl"
	w.Results = []ra.Result{{ID: "impl", Status: "satisfied", Source: Source(implRaw), Value: &implValue}}
	w.StageResults = []StageResult{implRaw, {CommandID: i.CommandID, AttemptID: i.AttemptID, StepID: i.StepID, InputVersion: i.Version, TurnID: r.TurnID, Succeeded: true, FinalVerified: true, Final: r.Content, PRCandidates: []string{"https://example.test/o/r/pull/1"}}}
	n := Route(w, i, r, "")
	if !n.Publish || n.Next != nil || n.PRCandidateURL != "https://example.test/o/r/pull/1" || n.Acquired.Status != "unverified" {
		t.Fatal("unique native clue did not enter verification", n)
	}
	found := false
	for _, field := range n.Acquired.Fields {
		if field.Field == "outputs.pr_url" && strings.HasPrefix(field.Locator, "native.pr_candidates[") {
			found = true
		}
	}
	if !found {
		t.Fatal("native clue was misattributed to final text")
	}
	w.StageResults[1].PRCandidates = append(w.StageResults[1].PRCandidates, "https://example.test/o/r/pull/2")
	n = Route(w, i, r, "")
	if n.Publish || n.Next != nil || n.Acquired.Status != "conflict" {
		t.Fatal("ambiguous native clues selected", n)
	}
	w.StageInputs = []StageInput{i}
	w.StageResults[1].PRCandidates = nil
	r.Content = "publication URL not available"
	w.StageResults[1].Final = r.Content
	n = Route(w, i, r, "frozen-policy")
	if n.Next == nil || n.Next.StepID != StepPublishReport || n.Next.SourceVersion != i.Version {
		t.Fatal("missing URL did not select one report", n)
	}
	w.Results = append(w.Results, *n.Acquired)
	report := *n.Next
	report.CommandID = "pub-report"
	report.Version = 2
	w.StageInputs = append(w.StageInputs, report)
	r.Content = "invalid report"
	r.TurnID = "report-turn"
	n = Route(w, report, r, "frozen-policy")
	if n.Next != nil || n.Status != "blocked" {
		t.Fatal("publication report correction was allowed", n)
	}

}

func TestRAVerificationConflictSurvivesResume(t *testing.T) {
	w, i := auditFixture()
	w.Results[0].Status = "conflict"
	if err := VerifyPrerequisites(w, i, w.Config.States[i.StepID]); err == nil {
		t.Fatal("authorization could resurrect conflicted implementation")
	}
}

func TestRASuccessOutputLargeFinal(t *testing.T) {
	w, i, r := plainFixture()
	r.Content = strings.Repeat("existing output\n", 10000) + "实施及必要验证已完成。"
	n := Route(w, i, r, "")
	if n.Next == nil || n.Next.StepID != StepPublish || n.Acquired.Source.FinalDigest != ra.Digest([]byte(r.Content)) {
		t.Fatal("complete final unnecessarily requested report", n)
	}
}

func TestRASourcesReportUsesOriginalContext(t *testing.T) {
	w, i, r := plainFixture()
	r.Content = strings.Repeat("existing result output\n", 10000)
	n := Route(w, i, r, "frozen-policy")
	if n.Next == nil {
		t.Fatal(n)
	}
	w.StageResults = []StageResult{{CommandID: i.CommandID, AttemptID: i.AttemptID, StepID: i.StepID, InputVersion: i.Version, TurnID: r.TurnID, Succeeded: true, FinalVerified: true, Final: r.Content}}
	prompt, err := RenderInput(w, *n.Next)
	if err != nil || len(prompt) > 12000 || !strings.Contains(prompt, i.CommandID) || !strings.Contains(prompt, ra.Digest([]byte(r.Content))) || strings.Contains(prompt, "existing result output") {
		t.Fatal("report duplicated execution output instead of binding original context", err)
	}
	if w.StageResults[0].Final != r.Content {
		t.Fatal("source final was rewritten")
	}
}

func TestRAHistoricalOutcomeCannotBePromoted(t *testing.T) {
	for _, outcome := range []string{"unknown", "failed", "cancelled", "future-outcome", "succeeded", "unreported"} {
		t.Run(outcome, func(t *testing.T) {
			w, input, result := plainFixture()
			result.NativeOutcome = outcome
			w.StageResults = []StageResult{{CommandID: input.CommandID, AttemptID: input.AttemptID, StepID: input.StepID, InputVersion: input.Version, TurnID: result.TurnID, Succeeded: true, Outcome: outcome, FinalVerified: true, Final: result.Content}}
			// A previous version could persist both Succeeded=true and unknown.
			raw, err := json.Marshal(w)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &w); err != nil {
				t.Fatal(err)
			}
			allowed := outcome == "succeeded" || outcome == "unreported"
			parsed := `{"outcome":"done","summary":"parser claims completed"}`
			for _, next := range []Transition{Route(w, input, result, ""), RouteParsed(w, input, parsed, "original-parser", "")} {
				if (next.Next != nil) != allowed || !allowed && (next.Publish || next.Acquired != nil && next.Acquired.Status == "satisfied") {
					t.Fatal("historical native outcome was promoted", outcome, next)
				}
			}
			// A successful report may not launder the original execution outcome.
			clean := result
			clean.NativeOutcome = "succeeded"
			clean.Content = "no completion declaration"
			w.StageResults[0].Final = clean.Content
			fallback := Route(Snapshot{Config: w.Config, StepID: input.StepID, StageInputs: []StageInput{input}}, input, clean, "policy")
			if fallback.Next == nil {
				t.Fatal(fallback)
			}
			report := *fallback.Next
			report.CommandID = "report"
			report.Version = 2
			w.Results = append(w.Results, *fallback.Acquired)
			w.StageInputs = append(w.StageInputs, report)
			clean.Content = parsed
			clean.TurnID = "report-turn"
			next := Route(w, report, clean, "policy")
			if (next.Next != nil) != allowed {
				t.Fatal("report changed original native outcome", outcome, next)
			}
			// Neither a prior satisfied business record nor renewed authorization
			// can make a non-success original eligible as a publication prerequisite.
			original := w.StageResults[0]
			acquired := ra.Result{ID: "historical-result", Status: "satisfied", Source: Source(original)}
			value := ra.Completion("claimed completed")
			acquired.Value = &value
			w.Results = []ra.Result{acquired}
			w.ImplementationResultID = acquired.ID
			pub := StageInput{StepID: StepPublish, ImplementationResultID: acquired.ID}
			if err := VerifyPrerequisites(w, pub, w.Config.States[StepPublish]); (err == nil) != allowed {
				t.Fatal("historical prerequisite changed", outcome, err)
			}
		})
	}
}
