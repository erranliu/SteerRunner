package application

import (
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	"github.com/erranliu/SteerRunner/internal/execution/publication"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"sync"
	"time"
)

func (e *Engine) finish(id string, value b.TurnEvidence) bool {
	return e.finishWithGate(id, value, nil)
}
func (e *Engine) finishWithGate(id string, value b.TurnEvidence, gate *sync.Mutex) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	owner := e.workers[id]
	e.mu.Unlock()
	result, err := reliable.NormalizeResult(r.Snapshot.Session, value)
	if err != nil {
		return false
	}
	if prior := workflow.Result(r.Workflow, value.CommandID); prior != nil {
		if prior.TurnID != result.TurnID || prior.Succeeded != result.Succeeded || prior.FinalVerified != result.ContentVerified || prior.Final != result.Content {
			e.blockRevision(id, r.Revision, "result_conflict", errors.New("conflicting native result for the same original command"), false)
		}
		return false
	}
	input := workflow.Input(r.Workflow, value.CommandID)
	if input == nil || input.AttemptID != r.Snapshot.Session.AttemptID || owner == nil || owner.port == nil {
		return false
	}
	inputCopy := *input
	state, ok := r.Workflow.State(input.StepID)
	if !ok {
		return false
	}
	if result.ContentVerified {
		// A fast/offline turn can finish before this service subscribes to its
		// notifications. Sampling its verified native final is observation too;
		// keep the existing bounded, redacted, memory-only activity projection.
		_ = e.acceptMessage(id, b.PublicMessage{AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.Session.Generation, SessionID: value.SessionID, TurnID: value.TurnID, Kind: "assistant", Text: result.Content, ObservedAt: time.Now().UTC()})
	}
	value.NativeOutcome = result.NativeOutcome
	value.FinalDiagnostic = e.redact(r, value.FinalDiagnostic)
	if unknown(r) {
		// Preserve the original terminal evidence without consuming its source or
		// authorizing a successor. After reconciliation the original history read
		// re-enters this boundary; no business/model input is generated.
		if state.Workspace != "" {
			if owner.checkpoints == nil {
				owner.checkpoints = map[string]*workflow.WorkspaceSource{}
			}
			if owner.checkpoints[inputCopy.CommandID] == nil {
				owner.checkpoints[inputCopy.CommandID] = &workflow.WorkspaceSource{Diagnostic: "original terminal had unresolved operations; no completion checkpoint"}
			}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.runs[id].Revision != r.Revision {
			return false
		}
		if r.Result != nil && reflect.DeepEqual(*r.Result, value) && r.Phase == "operation_before_result" {
			return false
		}
		r.Result = &value
		r.LastFinal = r.View().Final
		if r.LastFinal != nil {
			r.LastFinal.Text = e.redact(r, r.LastFinal.Text)
		}
		r.Status, r.Phase, r.Diagnostic = "reconciling", "operation_before_result", "Original operations must reconcile before acquiring the business result"
		_ = e.commitLocked(r, nil, "native_result_retained")
		return false
	}

	var checkpoint *workflow.WorkspaceSource
	if state.Workspace != "" && result.ContentVerified {
		// Git I/O must not hold either the actor lock or the native dispatch gate.
		// A stop/new source can commit while collection is pending.
		if gate != nil {
			gate.Unlock()
		}
		checkpoint = e.originalCheckpoint(r, owner, inputCopy)
		if gate != nil {
			gate.Lock()
		}
		if checkpoint == nil {
			e.mu.Lock()
			defer e.mu.Unlock()
			current := e.runs[id]
			if current.Revision == r.Revision && r.Allowed && !r.ResumeRequired && r.PendingStop == "" && r.Status != "blocked" && r.Status != "waiting_input" && r.Phase != "completion_checkpoint_unavailable" {
				next := r.Clone()
				next.Status, next.Phase, next.Diagnostic = "reconciling", "completion_checkpoint_unavailable", "Original owner completion checkpoint is unavailable; only original fact reads are allowed"
				_ = e.commitLocked(next, nil, "completion_checkpoint_unavailable")
			}
			return false
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current.Revision != r.Revision {
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
	raw := workflow.StageResult{SessionID: value.SessionID, PRCandidates: append([]string(nil), value.PRCandidates...), UnexpectedWork: r.ReportWork[inputCopy.CommandID] != "", CommandID: value.CommandID, AttemptID: inputCopy.AttemptID, StepID: inputCopy.StepID, InputVersion: inputCopy.Version, TurnID: value.TurnID, Succeeded: result.Succeeded, FinalVerified: result.ContentVerified, Final: result.Content, Failure: result.Failure, FinalDiagnostic: result.Diagnostic, Outcome: result.NativeOutcome, Workspace: checkpoint}
	if state.Contract == "publication" && result.ContentVerified {
		if provider, err := providerFor(r); err == nil {
			seen := map[string]bool{}
			for _, url := range r.PRClues {
				seen[url] = true
			}
			for _, url := range append(value.PRCandidates, ra.PRURLs(result.Content)...) {
				if _, err := provider.Config.PRNumber(url); err == nil && !seen[url] && len(r.PRClues) < 8 {
					r.PRClues = append(r.PRClues, url)
					seen[url] = true
				}
			}
		}
	}
	r.Workflow.StageResults = append(r.Workflow.StageResults, raw)
	transition := workflow.Route(r.Workflow, inputCopy, result, owner.port.PolicyCapabilities().ReportToolPolicy)
	if !transition.AcceptResult {
		return false
	}
	r.Result = &value
	r.LastFinal = r.View().Final
	if r.LastFinal != nil {
		r.LastFinal.Text = e.redact(r, r.LastFinal.Text)
		r.LastFinal.Diagnostic = e.redact(r, r.LastFinal.Diagnostic)
	}
	r.Snapshot.Session.LastStoppedTurnID = value.TurnID
	r.Snapshot.PendingTurn = nil
	r.NextStage = nil
	if state.Parse != "" && transition.Acquired != nil && transition.Acquired.Status == "insufficient" && transition.Next != nil && r.AcquisitionConfig.Config.CodexParser.Enabled {
		e.selectParser(r, inputCopy, state, &transition)
	}
	e.applyWorkflowTransition(r, inputCopy, state, transition)
	if err := e.commitLocked(r, nil, "result_committed"); err != nil {
		return false
	}
	e.signal(owner)
	return transition.Next != nil && r.Allowed
}

func (e *Engine) applyWorkflowTransition(r *runstate.Record, inputCopy workflow.StageInput, state workflow.State, transition workflow.Transition) {
	if transition.Acquired != nil {
		a := *transition.Acquired
		for i, prior := range r.Workflow.Results {
			if prior.ID == a.ID {
				for j := len(a.Attempts) - 1; j >= 0; j-- {
					if a.Attempts[j].OperationID != "" {
						for _, attempt := range prior.Attempts {
							if attempt.OperationID == a.Attempts[j].OperationID {
								a.Attempts = append(a.Attempts[:j], a.Attempts[j+1:]...)
								break
							}
						}
					}
				}
				a.Attempts = append(prior.Attempts, a.Attempts...)
				r.Workflow.Results = append(r.Workflow.Results[:i], r.Workflow.Results[i+1:]...)
				break
			}
		}
		r.Workflow.Results = append(r.Workflow.Results, a)
		if state.Contract == "publication" && !transition.Publish && (a.Status == "unverified" || a.Status == "conflict") {
			invalidateWorkspaceEligibility(r)
		}

		if state.Contract == "audit" && a.Value != nil {
			impl := workflow.Acquired(r.Workflow, inputCopy.ImplementationResultID)
			original := workflow.Result(r.Workflow, a.Source.CommandID)
			if impl != nil && original != nil {
				pre := workflow.Result(r.Workflow, impl.Source.CommandID)
				record := workflow.AuditRecord{StepID: original.StepID, SourceResultID: impl.ID, Source: impl.Source, ResultID: a.ID, CommandID: a.Source.CommandID, TurnID: a.Source.TurnID, FixedCount: a.Value.FixedCount, FixedIssues: a.Value.FixedIssues, RemainingIssues: a.Value.RemainingIssues}
				record.FixedIssues = append([]ra.FixedIssue{}, record.FixedIssues...)
				record.RemainingIssues = append([]ra.RemainingIssue{}, record.RemainingIssues...)
				for i := range record.FixedIssues {
					f := &record.FixedIssues[i]
					f.Issue = e.redact(r, f.Issue)
					f.Repair = e.redact(r, f.Repair)
					f.Evidence = e.redact(r, f.Evidence)
				}
				for i := range record.RemainingIssues {
					f := &record.RemainingIssues[i]
					f.Issue = e.redact(r, f.Issue)
					f.Reason = e.redact(r, f.Reason)
				}

				if pre != nil && pre.Workspace != nil && pre.Workspace.Qualified {
					record.PreHead = pre.Workspace.Head
				}
				if original.Workspace != nil && original.Workspace.Qualified {
					record.PostHead = original.Workspace.Head
				}
				if a.Status != "satisfied" || a.Value.Outcome != "done" {
					record.Failure = transition.Reason
				}
				records := []workflow.AuditRecord{}
				for _, prior := range r.Workflow.Audits {
					if prior.ResultID != a.ID {
						records = append(records, prior)
					}
				}
				r.Workflow.Audits = append(records, record)
				if a.Status == "satisfied" && a.Value.Outcome == "done" {
					r.Workflow.AuditResultID = a.ID
				}
			}
		}

		if a.Status == "satisfied" && a.Value != nil && a.Value.Outcome == "done" {
			if state.Contract == "implementation" {
				r.Workflow.ImplementationResultID = a.ID
				r.Workflow.AuditResultID = ""
				r.Workflow.Audits = nil
			}

		}
	}
	r.Diagnostic = transition.Reason
	r.Question = nil
	r.Snapshot.QuestionRoute = nil
	r.Snapshot.PendingQuestions = nil
	r.Phase = "result_committed"
	if transition.Report != nil {
		r.Summary = e.redact(r, transition.Report.Summary)
	}
	if transition.RecoverOriginal {
		r.Status = "blocked"
		r.ResumeRequired = true
		r.Allowed = false
		r.Phase = "original_session_recovery"
	}
	if transition.Cursor != "" {
		r.Workflow.StepID = transition.Cursor
	}
	if transition.Status != "" {
		r.Status = transition.Status
	}
	if transition.Status == "waiting_input" {
		r.Question = &contracts.QuestionView{ID: contracts.NewID(), Text: e.redact(r, transition.Reason), AllowFreeText: true, Source: "workflow_policy"}
		if transition.Report != nil && transition.Report.Question != "" && transition.Acquired != nil {
			r.Question.Source = transition.Acquired.Method
		}

	}
	if transition.Next != nil && r.PendingStop == "" && r.StopStatus == "" {
		if r.Allowed && !r.ResumeRequired && r.PendingStop == "" {
			if err := queueInput(r, *transition.Next, ""); err != nil {
				r.Status, r.Diagnostic = "blocked", err.Error()
			}
		} else {
			r.NextStage = transition.Next
			r.Workflow.StepID = transition.Next.StepID
			r.ResumeRequired = true
			r.Phase = "next_stage_requires_resume"
		}
	}
	if transition.Publish && r.Allowed && !r.ResumeRequired && r.PendingStop == "" && r.StopStatus == "" {
		r.PendingPRURL = transition.PRCandidateURL
		r.Workflow.PublicationResultID = transition.Acquired.ID
		r.Status, r.Phase = "reconciling", "pr_verification"
	}

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
		e.blockRevision(id, r.Revision, "workflow_configuration", err, false)
		return false
	}
	publicationResult := workflow.Acquired(r.Workflow, r.Workflow.PublicationResultID)
	if publicationResult == nil || publicationResult.Status != "unverified" && publicationResult.Status != "satisfied" || publicationResult.Value == nil || publicationResult.Value.Outcome != "done" || publicationResult.Value.Outputs.PRURL != r.PendingPRURL {
		e.blockRevision(id, r.Revision, "publication_source_unverified", errors.New("publication cursor lacks its exact accepted result"), false)
		return false
	}
	originalPublication := workflow.Input(r.Workflow, publicationResult.Source.CommandID)
	if originalPublication == nil || originalPublication.ImplementationResultID != r.Workflow.ImplementationResultID || originalPublication.AuditResultID != r.Workflow.AuditResultID {
		e.blockRevision(id, r.Revision, "publication_source_unverified", errors.New("publication prerequisites changed"), false)
		return false
	}
	if !workflow.AuditsPassed(r.Workflow) {
		e.blockRevision(id, r.Revision, "audit_incomplete", errors.New("Publication lacks current audit passes"), false)
		return false
	}
	provider, err := providerFor(r)
	if err != nil {
		e.blockRevision(id, r.Revision, "pr_configuration", err, false)
		return false
	}
	if r.PR != nil && r.PendingPRURL != r.PR.URL {
		e.blockRevision(id, r.Revision, "pr_identity_conflict", errors.New("A later result cannot replace the bound PR"), true)
		return false
	}
	if !r.Allowed || r.ResumeRequired || r.PendingStop != "" || e.ctx.Err() != nil {
		return false
	}
	if err := e.checkPublicationSource(r); err != nil {
		e.blockRevision(id, r.Revision, "source_unverified", err, false)
		return false
	}
	info, err := provider.Query(e.ctx, r.PendingPRURL)
	if err == nil {
		err = validatePR(r, info)
	}
	if err == nil && r.Workflow.RequiresWorkspace() {
		post := checkpoint(r, r.Workflow.AuditResultID)
		if post == nil || info.HeadRevision == "" || info.HeadRevision != post.Head {
			err = errors.New("PR head differs from audited revision")
		}
	}
	if err == nil {
		if sourceErr := e.checkPublicationSource(r); sourceErr != nil {
			e.blockRevision(id, r.Revision, "source_unverified", sourceErr, false)
			return false
		}
	}
	if err != nil {
		e.blockRevision(id, r.Revision, "pr_unverified", errors.New("Provider could not verify the fixed repository/base/head/PR binding"), true)
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if next.Revision != r.Revision || next.PendingPRURL != r.PendingPRURL || !next.Allowed || next.ResumeRequired || next.PendingStop != "" || next.StopStatus != "" || next.Workflow.ImplementationResultID != r.Workflow.ImplementationResultID || next.Workflow.AuditResultID != r.Workflow.AuditResultID || e.ctx.Err() != nil {
		return false
	}
	for i := range next.Workflow.Results {
		a := &next.Workflow.Results[i]
		if a.ID == next.Workflow.PublicationResultID {
			a.Status = "satisfied"
			for j := range a.Attempts {
				if a.Attempts[j].Method == "program" && a.Attempts[j].Status == "unverified" {
					a.Attempts[j].Status = "satisfied"
				}
			}
			a.Fields = append(a.Fields, ra.FieldSource{Field: "outputs.pr_url", Source: a.Source, Locator: "provider.Query(" + info.URL + "):verified_repository_base_head", Method: "program"})
		}
	}
	next.PR = prFact(next, info)
	next.PendingPRURL = ""
	next.Workflow.PRURL = info.URL
	state, _ := next.Workflow.CurrentState()
	next.Workflow.StepID = state.Next
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

// A stale verification failure must not replace a stop or a newer source.
func (e *Engine) blockRevision(id string, revision uint64, phase string, err error, uncertain bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current.Revision != revision || current.PendingStop != "" || current.StopStatus != "" {
		return
	}
	next := current.Clone()
	if phase == "source_unverified" {
		invalidateWorkspaceEligibility(next)
	}
	if phase == "result_conflict" {
		for i := range next.Workflow.Results {
			a := &next.Workflow.Results[i]
			if a.ID == next.Workflow.ImplementationResultID || a.ID == next.Workflow.AuditResultID || a.ID == next.Workflow.PublicationResultID {
				a.Status = "conflict"
				a.Conflicts = append(a.Conflicts, "conflicting original native result")
			}
		}
	}

	next.Phase = phase
	next.Diagnostic = e.redact(next, err.Error())
	next.Status = "blocked"
	if uncertain {
		next.Status = "reconciling"
	} else {
		next.Allowed = false
		next.ResumeRequired = true
	}
	if reflect.DeepEqual(next, current) {
		return
	}
	_ = e.commitLocked(next, nil, "verification_unverified")
}

func invalidateWorkspaceEligibility(r *runstate.Record) {
	for i := range r.Workflow.Results {
		result := &r.Workflow.Results[i]
		if result.ID == r.Workflow.ImplementationResultID || result.ID == r.Workflow.AuditResultID {
			result.Status = "unverified"
			result.Conflicts = append(result.Conflicts, "observed workspace/source drift invalidated eligibility")
		}
	}
}
