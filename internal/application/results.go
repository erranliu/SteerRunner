package application

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	"github.com/erranliu/SteerRunner/internal/execution/publication"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"time"
)

func (e *Engine) acceptEvent(id string, event b.LifecycleEvent) error {
	if event.PublicMessage != nil {
		return e.acceptMessage(id, *event.PublicMessage)
	}
	f := event.Fact
	if f.Kind != b.EventQuestion && f.Kind != b.EventCompleted && f.Kind != b.EventStarted && f.Kind != b.EventGap && len(event.Implementation) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if f.SessionID != "" && f.SessionID != r.Snapshot.Session.SessionID {
		return reliable.ErrStale
	}
	if f.StartedTurnID != "" && f.StartedTurnID != r.Snapshot.Session.CurrentTurnID && f.StartedTurnID != r.Snapshot.Session.PendingTurnID {
		return reliable.ErrStale
	}
	if r.SeenNative == nil {
		r.SeenNative = map[string]string{}
	}
	key := r.Snapshot.Session.AttemptID + ":" + f.Source + ":" + f.SourceEventID
	digest := reliable.Digest(canonical(event.Fact))
	if f.SourceEventID != "" {
		if prior, ok := r.SeenNative[key]; ok {
			if prior != digest {
				return errors.New("conflicting duplicate native event")
			}
			return nil
		}
		r.SeenNative[key] = digest
	}
	if len(event.Implementation) > 0 {
		if err := merge(r, b.LifecycleReceipt{Implementation: event.Implementation}); err != nil {
			return err
		}
	}
	if f.Kind == b.EventQuestion {
		if f.QuestionRoute == nil {
			return errors.New("native question lacks its verified original route")
		}
		route := *f.QuestionRoute
		if err := route.ValidateIdentity(b.QuestionRouteIdentity{ProjectID: r.Snapshot.Session.ProjectID, WorkflowRunID: id, ExecutionID: id, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.Session.Generation, TurnID: r.Snapshot.Session.CurrentTurnID, QuestionID: route.QuestionID}); err != nil {
			return err
		}
		if r.PendingAnswer != nil && r.Question != nil && r.Question.ID != route.QuestionID {
			return errors.New("another native question cannot replace the original pending answer")
		}
		question := &contracts.QuestionView{ID: route.QuestionID, Text: f.QuestionText, AllowFreeText: f.AllowFreeText}
		for _, option := range f.QuestionOptions {
			question.Options = append(question.Options, contracts.QuestionOption{ID: option.ID, Label: option.Label, Detail: option.Detail})
		}
		for _, native := range f.Questions {
			q := contracts.QuestionView{ID: native.ID, Text: native.Question, AllowFreeText: native.AllowFreeText}
			for _, o := range native.Options {
				q.Options = append(q.Options, contracts.QuestionOption{ID: o.ID, Label: o.Label, Detail: o.Detail})
			}
			question.NativeQuestions = append(question.NativeQuestions, q)
		}
		if reflect.DeepEqual(r.Question, question) {
			return nil
		}
		r.Question = question
		r.Snapshot.QuestionRoute = &route
		r.Snapshot.PendingQuestions = f.Questions
		r.Status, r.Phase = "waiting_input", "question"
	} else if f.Kind == b.EventGap {
		r.Diagnostic = "Original observation reported a result gap"
		r.Status = "reconciling"
	} else if f.Kind == b.EventStarted && r.Snapshot.Session.PendingCommandID != "" {
		r.Snapshot.Session.CurrentCommandID = r.Snapshot.Session.PendingCommandID
		r.Snapshot.Session.CurrentTurnID = f.StartedTurnID
		r.Snapshot.Session.PendingCommandID = ""
		r.Snapshot.Session.PendingTurnID = ""
		r.NativeStarted = true
		r.Phase = "observing"
	} else if len(event.Implementation) == 0 {
		return nil
	}
	return e.commitLocked(r, nil, string(f.Kind))
}

func (e *Engine) finish(id string, value b.TurnEvidence) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	result, err := reliable.NormalizeResult(r.Snapshot.Session, value)
	if err != nil {
		return false
	}
	value.NativeOutcome = result.NativeOutcome
	value.FinalDiagnostic = e.redact(r, value.FinalDiagnostic)
	if workflow.Result(r.Workflow, value.CommandID) != nil {
		return false
	}
	if result.Succeeded && !result.ContentVerified {
		if r.Result != nil && reflect.DeepEqual(*r.Result, value) {
			return false
		}
		r.Result = &value
		r.Status, r.Phase = "reconciling", "final_unavailable"
		r.Diagnostic = "Original succeeded turn lacks verified final content; only original result reads are allowed"
		_ = e.commitLocked(r, nil, "final_unavailable")
		return false
	}
	input := workflow.Input(r.Workflow, value.CommandID)
	if input == nil || input.AttemptID != r.Snapshot.Session.AttemptID {
		return false
	}
	inputCopy := *input
	if workflow.PublicationStep(inputCopy.StepID) && result.ContentVerified {
		provider, providerErr := providerFor(r)
		if providerErr == nil {
			seen := map[string]bool{}
			for _, url := range r.PRClues {
				seen[url] = true
			}
			for _, url := range value.PRCandidates {
				if _, err := provider.Config.PRNumber(url); err == nil && !seen[url] && len(r.PRClues) < 8 {
					r.PRClues = append(r.PRClues, url)
					seen[url] = true
				}
			}
		}
	}
	owner := e.workers[id]
	if owner == nil || owner.port == nil {
		return false
	}
	// Freeze the originating adapter's actual policy in the next report.
	// Workflow must not substitute its local default for Orca permissions.
	reportPolicy := owner.port.PolicyCapabilities().ReportToolPolicy
	transition := workflow.Route(r.Workflow, inputCopy, result, reportPolicy)
	if !transition.AcceptResult {
		r.Status, r.Phase, r.Diagnostic = transition.Status, "result_unverified", transition.Reason
		_ = e.commitLocked(r, nil, "result_unverified")
		return false
	}
	r.Result = &value
	r.Snapshot.Session.LastStoppedTurnID = value.TurnID
	r.LastFinal = r.View().Final
	if r.LastFinal != nil {
		r.LastFinal.Text = e.redact(r, r.LastFinal.Text)
		r.LastFinal.Diagnostic = e.redact(r, r.LastFinal.Diagnostic)
	}
	r.Snapshot.PendingTurn = nil
	r.Workflow.StageResults = append(r.Workflow.StageResults, workflow.StageResult{CommandID: value.CommandID, AttemptID: r.Snapshot.Session.AttemptID, StepID: inputCopy.StepID, InputVersion: inputCopy.Version, TurnID: value.TurnID, Succeeded: value.Succeeded, FinalVerified: value.FinalVerified, Final: value.LastMessage, Failure: value.Failure, FinalDiagnostic: value.FinalDiagnostic, Outcome: value.NativeOutcome})
	r.Workflow.ReportRetry = transition.ReportRetry
	r.Diagnostic = transition.Reason
	r.Question = nil
	r.Snapshot.QuestionRoute = nil
	r.Snapshot.PendingQuestions = nil
	r.Phase = "result_committed"
	if transition.Report != nil {
		r.Summary = transition.Report.Summary
	}
	if transition.RecoverOriginal {
		r.Status = "blocked"
		r.ResumeRequired = true
		r.Allowed = false
		r.Phase = "original_session_recovery"
	}
	if transition.Status != "" {
		r.Status = transition.Status
	}
	if transition.Status == "waiting_input" {
		r.Question = &contracts.QuestionView{ID: contracts.NewID(), Text: transition.Reason, AllowFreeText: true}
	}
	if transition.Next != nil {
		if r.Allowed && !r.ResumeRequired {
			if err := queueInput(r, *transition.Next, ""); err != nil {
				r.Status, r.Diagnostic = "blocked", err.Error()
			}
		} else {
			r.NextStage = transition.Next
			r.ResumeRequired = true
			r.Phase = "next_stage_requires_resume"
		}
	}
	if transition.Publish {
		r.PendingPRURL = transition.PRCandidateURL
		r.Status, r.Phase = "reconciling", "pr_verification"
	}
	if err := e.commitLocked(r, nil, "result_committed"); err != nil {
		return false
	}
	e.signal(e.workers[id])
	return transition.Next != nil && r.Allowed
}
func validatePR(r *runstate.Record, info evidence.PRInfo) error {
	url := r.PendingPRURL
	if r.PR != nil {
		url = r.PR.URL
	}
	return publication.ValidateWorkerPRIdentity(info, url, r.Request.Publication.Repository, r.Request.Repository.BaseBranch, r.Snapshot.Ref.WorkspaceBranch)
}
func prFact(r *runstate.Record, info evidence.PRInfo) *contracts.PRFact {
	provider, _ := providerFor(r)
	number, _ := provider.Config.PRNumber(info.URL)
	return &contracts.PRFact{URL: info.URL, Provider: r.Request.Publication.Provider, Repository: info.Repository, Number: number, BaseBranch: info.BaseBranch, HeadRepository: info.HeadRepository, HeadBranch: info.HeadBranch, HeadRevision: info.HeadRevision, State: info.State, Merged: info.Merged, MergedAt: info.MergedAt, MergeCommit: info.MergeCommit, CheckedAt: time.Now().UTC()}
}
func (e *Engine) verifyPublication(id string) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	provider, err := providerFor(r)
	if err != nil {
		e.block(id, "pr_configuration", err, false)
		return false
	}
	if r.PR != nil && r.PendingPRURL != r.PR.URL {
		e.block(id, "pr_identity_conflict", errors.New("A later result cannot replace the bound PR"), true)
		return false
	}
	info, err := provider.Query(e.ctx, r.PendingPRURL)
	if err == nil {
		err = validatePR(r, info)
	}
	if err != nil {
		e.block(id, "pr_unverified", errors.New("Provider could not verify the fixed repository/base/head/PR binding"), true)
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if next.PendingPRURL != r.PendingPRURL {
		return false
	}
	next.PR = prFact(next, info)
	next.PendingPRURL = ""
	next.Workflow.PRURL = info.URL
	next.Workflow.StepID = workflow.StepPRReady
	next.Status, next.Phase, next.Diagnostic = "pr_ready", "pr_ready", ""
	return e.commitLocked(next, nil, "pr_ready") == nil
}

// A verified PR binding is independent of workflow completion. In particular,
// formatting the publication report cannot prevent the caller from observing
// a correctly bound PR that has already merged.
func (e *Engine) probePRClues(id string) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	provider, err := providerFor(r)
	if err != nil {
		return false
	}
	verified := []evidence.PRInfo{}
	for _, url := range r.PRClues {
		info, err := provider.Query(e.ctx, url)
		if err == nil && publication.ValidateWorkerPRIdentity(info, url, r.Request.Publication.Repository, r.Request.Repository.BaseBranch, r.Snapshot.Ref.WorkspaceBranch) == nil {
			verified = append(verified, info)
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if next.PR != nil || next.Snapshot.Ref.WorkspaceBranch != r.Snapshot.Ref.WorkspaceBranch {
		return false
	}
	next.PRClueProbeAt = time.Now().UTC().Add(5 * time.Second)
	if len(verified) == 1 {
		next.PR = prFact(next, verified[0])
		next.Workflow.PRURL = verified[0].URL
		return e.commitLocked(next, nil, "pr_bound") == nil
	}
	return e.commitLocked(next, nil, "pr_clues_unverified") == nil
}
