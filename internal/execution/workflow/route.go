package workflow

import (
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"strings"
)

type Transition struct {
	Next                                                    *StageInput
	Cursor                                                  string
	Status, Reason, Outcome, PRCandidateURL                 string
	Report                                                  *Report
	Acquired                                                *ra.Result
	AcceptResult, RecoverOriginal, Publish, ExpireQuestions bool
}

func sourceFor(w Snapshot, input StageInput, result reliable.Result) (StageInput, StageResult, bool) {
	s, ok := w.State(input.StepID)
	if !ok {
		return StageInput{}, StageResult{}, false
	}
	if s.Kind != "report" {
		r := StageResult{CommandID: input.CommandID, AttemptID: input.AttemptID, StepID: input.StepID, InputVersion: input.Version, TurnID: result.TurnID, Succeeded: result.Succeeded, FinalVerified: result.ContentVerified, Final: result.Content, Failure: result.Failure}
		if actual := Result(w, input.CommandID); actual != nil {
			r = *actual
		}
		return input, r, true
	}
	original, source := Input(w, input.SourceCommandID), Result(w, input.SourceCommandID)
	if original == nil || source == nil || source.TurnID != input.SourceTurnID || source.InputVersion != input.SourceVersion {
		return StageInput{}, StageResult{}, false
	}
	if input.SourceResultID != "" {
		acquired := Acquired(w, input.SourceResultID)
		if acquired == nil || acquired.Source != Source(*source) {
			return StageInput{}, StageResult{}, false
		}
	}
	return *original, *source, true
}
func Source(r StageResult) ra.Source {
	return ra.Source{SessionID: r.SessionID, CommandID: r.CommandID, TurnID: r.TurnID, AttemptID: r.AttemptID, InputVersion: r.InputVersion, FinalDigest: ra.Digest([]byte(r.Final))}
}

// Route consumes normalized native evidence and frozen definition values. It
// performs no effects and has no workflow-ID or success-wording branches.
func Route(w Snapshot, input StageInput, result reliable.Result, reportPolicy string) Transition {
	return route(w, input, result, reportPolicy, nil, "")
}

// RouteParsed projects the actual original native facts and an independent
// candidate; it never creates a StageInput, StageResult, or native agent final.
func RouteParsed(w Snapshot, input StageInput, body, operationID, reportPolicy string) Transition {
	source := Result(w, input.CommandID)
	if source == nil {
		return Transition{Status: "blocked", Reason: "original parser source missing"}
	}
	result := reliable.Result{Succeeded: source.Succeeded, ContentVerified: source.FinalVerified, Content: source.Final, TurnID: source.TurnID, Failure: source.Failure}
	return route(w, input, result, reportPolicy, &body, operationID)
}

func route(w Snapshot, input StageInput, result reliable.Result, reportPolicy string, parsed *string, operationID string) Transition {
	t := Transition{AcceptResult: true}
	state, ok := w.State(input.StepID)
	if !ok {
		t.Status, t.Reason = "blocked", "state absent from frozen definition"
		return t
	}
	if !result.Succeeded {
		t.RecoverOriginal = true
		t.Outcome = "blocked"
		t.Reason = "Original turn failed or was interrupted; verify the original session: " + result.Failure
		return t
	}
	if !result.ContentVerified {
		t.AcceptResult = false
		t.Status, t.Reason = "reconciling", "original final provenance unavailable"
		return t
	}
	if err := VerifyPrerequisites(w, input, state); err != nil {
		t.Status, t.Reason = "blocked", err.Error()
		return t
	}
	original, source, ok := sourceFor(w, input, result)
	if !ok || !source.Succeeded || !source.FinalVerified || source.Failure != "" {
		t.Status, t.Reason = "reconciling", "original business source is incomplete or failed"
		return t
	}
	if state.Contract == "audit" && !AuditSource(w, original) {
		t.Status, t.Reason = "blocked", "audit implementation source changed"
		return t
	}
	if state.Contract == "publication" && !AuditsPassed(w) {
		t.Status, t.Reason = "blocked", "publication lacks current audit completion"
		return t
	}
	ref := Source(source)
	acquired := ra.Result{ID: ra.Digest([]byte(fmt.Sprintf("%s/%s/%d/%s", ref.CommandID, ref.TurnID, ref.InputVersion, state.Contract))), Contract: state.Contract, Source: ref, Method: "program", Status: "insufficient"}
	if state.Kind == "report" {
		acquired.Method = "agent_report"
	}
	if parsed != nil {
		acquired.Method = "llm_parser"
	}
	acquired.Attempts = []ra.Attempt{{Method: acquired.Method, Source: ref, CommandID: input.CommandID, Status: "insufficient"}}
	if parsed != nil {
		acquired.Attempts[0].CommandID = ""
		acquired.Attempts[0].OperationID = operationID
	}
	if current := Result(w, input.CommandID); state.Kind == "report" && current != nil && current.UnexpectedWork {
		acquired.Status = "conflict"
		acquired.Conflicts = []string{"native observation proves report performed business work"}
		t.Acquired = &acquired
		t.Status, t.Reason = "blocked", acquired.Conflicts[0]
		return t
	}
	candidate, err := ra.DecodeReport(result.Content, state.Contract)
	locator := "native.final:$"
	if parsed != nil {
		candidate, err = ra.DecodeReport(*parsed, state.Contract)
		locator = "parser:" + operationID + ":$"
	}
	semanticMarker := false
	if state.Contract == "publication" && state.Kind == "execute" {
		urls := ra.PublicationClues(result.Content, source.PRCandidates)
		if len(urls) > 1 {
			err = fmt.Errorf("%w: multiple PR candidates", ra.ErrConflict)
		} else if err != nil && len(urls) == 1 {
			locator = "native.final:pr_url_match"
			for index, clue := range source.PRCandidates {
				if clue == urls[0] && !strings.Contains(result.Content, urls[0]) {
					locator = fmt.Sprintf("native.pr_candidates[%d]", index)
					break
				}
			}
			// Querying this unique candidate settles the original publication facts;
			// merely finding the URL never completes the workflow.
			candidate, err = ra.Completion("PR verification pending: "+urls[0]), nil
			candidate.Outputs.PRURL = urls[0]
		}
	}

	if parsed == nil && err != nil && state.SuccessOutput != "" && ra.RecognizeSuccessOutput(result.Content, state.SuccessOutput, input.Prompt) {
		semanticMarker = true
		locator = "native.final:last_nonempty_body_line"
		// The declaration supplies a business conclusion, never fabricated checks.
		candidate, err = ra.Completion(state.SuccessOutput), nil
		if state.Contract == "publication" {
			urls := ra.PublicationClues(result.Content, source.PRCandidates)
			if len(urls) != 1 {
				err = fmt.Errorf("publication requires one unambiguous PR URL")
			} else {
				candidate.Outputs.PRURL = urls[0]
			}
		}
	}
	if errors.Is(err, ra.ErrConflict) {
		acquired.Status = "conflict"
		acquired.Conflicts = []string{err.Error()}
		t.Acquired = &acquired
		t.Status, t.Reason = "blocked", err.Error()
		return t
	}
	if err != nil {
		acquired.Missing = []string{err.Error()}
		t.Acquired = &acquired
		if state.Kind == "execute" && state.ReportLimit == 1 && state.Report != "" && !reportUsed(w, source.CommandID) {
			t.Next = &StageInput{StepID: state.Report, SourceCommandID: source.CommandID, SourceTurnID: source.TurnID, SourceVersion: source.InputVersion, SourceResultID: acquired.ID, ImplementationResultID: original.ImplementationResultID, AuditResultID: original.AuditResultID, ReportToolPolicy: reportPolicy}
			return t
		}
		wait(&t, state, "结果不足，请补充输入。原执行结果已保留。")
		return t
	}
	t.Report = &candidate
	t.Outcome = candidate.Outcome
	acquired.Value = &candidate
	fieldRef := ref
	if actual := Result(w, input.CommandID); actual != nil {
		fieldRef = Source(*actual)
	}
	for _, field := range []string{"outcome", "summary"} {
		acquired.Fields = append(acquired.Fields, ra.FieldSource{Field: field, Source: fieldRef, Locator: locator + "/" + field, Method: acquired.Method})
	}
	if semanticMarker {
		for i := range acquired.Fields {
			acquired.Fields[i].Locator = "native.final:last_nonempty_body_line"
		}
	}
	if candidate.Question != "" {
		acquired.Fields = append(acquired.Fields, ra.FieldSource{Field: "question", Source: fieldRef, Locator: locator + "/question", Method: acquired.Method})
	}
	if candidate.Blocker != nil {
		acquired.Fields = append(acquired.Fields, ra.FieldSource{Field: "blocker", Source: fieldRef, Locator: locator + "/blocker", Method: acquired.Method})
	}
	if candidate.Outputs.PRURL != "" {
		acquired.Fields = append(acquired.Fields, ra.FieldSource{Field: "outputs.pr_url", Source: fieldRef, Locator: prLocator(locator), Method: acquired.Method})
	}
	if candidate.FixedCount != nil {
		for _, field := range []string{"fixed_count", "fixed_issues", "remaining_issues"} {
			acquired.Fields = append(acquired.Fields, ra.FieldSource{Field: field, Source: fieldRef, Locator: locator + "/" + field, Method: acquired.Method})
		}
	}

	if candidate.Outcome == "done" {
		if err := VerifyCompletion(w, state, original, source, input); err != nil {
			acquired.Status = "unverified"
			acquired.Missing = []string{err.Error()}
			t.Acquired = &acquired
			wait(&t, state, err.Error())
			return t
		}
		if state.Contract == "audit" && candidate.FixedCount != nil && *candidate.FixedCount > 0 {
			impl := Acquired(w, input.ImplementationResultID)
			if impl == nil {
				impl = Acquired(w, original.ImplementationResultID)
			}
			pre := Result(w, impl.Source.CommandID)
			if pre.Workspace.Tree == source.Workspace.Tree {
				acquired.Status = "conflict"
				acquired.Conflicts = []string{"positive repairs require a changed Git tree"}
				t.Acquired = &acquired
				t.Status, t.Reason = "blocked", "repair count contradicts Git trees"
				return t
			}
		}
		acquired.Status = "satisfied"
		acquired.Attempts[0].Status = "satisfied"
		t.Acquired = &acquired
		t.ExpireQuestions = true
		if state.Contract == "publication" {
			acquired.Status = "unverified"
			acquired.Attempts[0].Status = "unverified"
			t.Publish = true
			t.Cursor = state.Verify
			t.PRCandidateURL = candidate.Outputs.PRURL
			return t
		}
		next := StageInput{StepID: state.Next, ImplementationResultID: input.ImplementationResultID, AuditResultID: input.AuditResultID}
		if state.Contract == "implementation" {
			next.ImplementationResultID = acquired.ID
			next.SourceCommandID = source.CommandID
			next.SourceTurnID = source.TurnID
			next.SourceVersion = source.InputVersion
		}
		if state.Contract == "audit" {
			next.AuditResultID = acquired.ID
			next.ImplementationResultID = original.ImplementationResultID
		}
		t.Next = &next
		return t
	}
	acquired.Status = "satisfied"
	acquired.Attempts[0].Status = "satisfied"
	t.Acquired = &acquired
	if candidate.Blocker != nil && candidate.Blocker.Category == "unknown_external" {
		t.Status, t.Reason = "reconciling", blockerReason(candidate.Blocker)
		return t
	}
	reason := candidate.Question
	if reason == "" {
		reason = candidate.Summary
	}
	wait(&t, state, reason)
	return t
}
func wait(t *Transition, state State, reason string) {
	t.Reason = reason
	if state.Wait != "" {
		t.Status, t.Cursor = "waiting_input", state.Wait
	} else {
		t.Status = "blocked"
	}
}
func reportUsed(w Snapshot, command string) bool {
	for _, in := range w.StageInputs {
		s, _ := w.State(in.StepID)
		if s.Kind == "report" && in.SourceCommandID == command {
			return true
		}
	}
	return false
}
func blockerReason(b *Blocker) string {
	return fmt.Sprintf("%s: %s; %s; known=%s; unknown=%s", b.Category, b.Action, b.Error, b.Known, b.Unknown)
}

func prLocator(locator string) string {
	if locator == "native.final:$" {
		return "native.final:$.outputs.pr_url"
	}
	if locator == "native.final:last_nonempty_body_line" {
		return "native.final:pr_url_match"
	}
	return locator
}
