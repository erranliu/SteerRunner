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
	if r.Workflow.Config != nil && inputCopy.StepID == workflow.StepReport && transition.Report != nil && transition.Report.Outcome == "done" && transition.Next != nil {
		r.Workflow.AuditReportCommandID = inputCopy.CommandID
	}
	if workflow.AuditStep(inputCopy.StepID) {
		a := workflow.AuditRecord{StepID: inputCopy.StepID, SourceCommandID: inputCopy.SourceCommandID, CommandID: inputCopy.CommandID, TurnID: result.TurnID, Failure: transition.Reason}
		if transition.Report != nil {
			a.Verdict, a.Findings = transition.Report.Verdict, transition.Report.Findings
		}
		current := []workflow.AuditRecord{}
		for _, prior := range r.Workflow.Audits {
			if prior.StepID != a.StepID {
				current = append(current, prior)
			}
		}
		r.Workflow.Audits = append(current, a)
	}
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
			} else if transition.Repair {
				r.Workflow.RepairCount++
			}
		} else {
			r.NextStage = transition.Next
			if transition.Repair {
				r.Workflow.RepairCount++
			}
			if transition.Next.StepID == workflow.StepImplement {
				r.Workflow.Audits = nil
				r.Workflow.AuditReportCommandID = ""
			}
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
	if err := validateWorkflow(r); err != nil {
		e.block(id, "workflow_configuration", err, false)
		return false
	}
	if !workflow.AuditsPassed(r.Workflow) {
		e.block(id, "audit_incomplete", errors.New("Publication lacks current audit passes"), false)
		return false
	}
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
