// Package runstate holds the service's sole durable execution aggregate.
// It contains no caller Task, DAG, delivery or storage capabilities.
package runstate

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"slices"
	"time"
)

type Material struct {
	contracts.MaterialView
	Content string `json:"content"`
}
type Answer struct {
	Epoch      string              `json:"epoch"`
	CommandID  string              `json:"command_id"`
	QuestionID string              `json:"question_id"`
	Text       string              `json:"text"`
	Answers    map[string][]string `json:"answers,omitempty"`
}
type ManualPause struct {
	Status, Phase, Diagnostic string
	ResumeDeadline            time.Time `json:"resume_deadline,omitempty"`
}

type Record struct {
	ManagedWorkspace    *ManagedWorkspace              `json:"managed_workspace,omitempty"`
	StopEvidenceScope   string                         `json:"stop_evidence_scope,omitempty"`
	ManualPause         *ManualPause                   `json:"manual_pause,omitempty"`
	ObservationDeadline time.Time                      `json:"observation_deadline,omitempty"`
	ObservationRetryAt  time.Time                      `json:"observation_retry_at,omitempty"`
	ObservationFailures int                            `json:"observation_failures,omitempty"`
	RepositorySource    *contracts.RepositorySource    `json:"repository_source,omitempty"`
	ResultFile          *ResultFile                    `json:"result_file,omitempty"`
	ResultFileContract  string                         `json:"result_file_contract,omitempty"`
	ResultFileEnd       *ResultFileEnd                 `json:"result_file_end,omitempty"`
	Progress            *Progress                      `json:"progress,omitempty"`
	Artifacts           []Artifact                     `json:"artifacts,omitempty"`
	Closed              bool                           `json:"closed,omitempty"`
	PendingInterrupt    string                         `json:"pending_interrupt,omitempty"`
	ContentResult       *contracts.ContentResult       `json:"content_result,omitempty"`
	History             []contracts.HistoryEntry       `json:"history,omitempty"`
	DirectoryIdentity   string                         `json:"directory_identity,omitempty"`
	ReportWork          map[string]string              `json:"observed_report_work,omitempty"`
	PRClues             []string                       `json:"pr_clues,omitempty"`
	PRClueProbeAt       time.Time                      `json:"pr_clue_probe_at,omitempty"`
	LastFinal           *contracts.FinalView           `json:"last_final,omitempty"`
	NativeStarted       bool                           `json:"native_started"`
	SeenNative          map[string]string              `json:"seen_native,omitempty"`
	AnswerHistory       []Answer                       `json:"answer_history,omitempty"`
	LinkedGitPaths      []string                       `json:"linked_git_paths,omitempty"`
	NextStage           *workflow.StageInput           `json:"next_stage,omitempty"`
	PendingPRURL        string                         `json:"pending_pr_url,omitempty"`
	NativeOwner         *b.ProcessIdentity             `json:"native_owner,omitempty"`
	NativeManaged       bool                           `json:"native_managed"`
	WorkspaceFact       *b.WorkspaceInspection         `json:"workspace_fact,omitempty"`
	PreparationDeadline time.Time                      `json:"preparation_deadline"`
	SubmitCommandID     string                         `json:"submit_command_id"`
	CommandStates       map[string]string              `json:"command_states"`
	PromptBodies        map[string]string              `json:"prompt_bodies"`
	ID                  string                         `json:"id"`
	Revision            uint64                         `json:"revision"`
	Request             contracts.RunRequest           `json:"request"`
	InputDigest         string                         `json:"input_digest"`
	ResourceDigest      string                         `json:"resource_digest"`
	Materials           []Material                     `json:"materials"`
	RepositoryPath      string                         `json:"repository_path"`
	RepositoryCommon    string                         `json:"repository_common"`
	Status              string                         `json:"status"`
	Phase               string                         `json:"phase"`
	Diagnostic          string                         `json:"diagnostic,omitempty"`
	Allowed             bool                           `json:"new_inputs_allowed"`
	ResumeRequired      bool                           `json:"resume_required"`
	StopStatus          string                         `json:"stop_status,omitempty"`
	Snapshot            b.LifecycleSnapshot            `json:"snapshot"`
	Profile             []b.ImplementationDataEnvelope `json:"profile"`
	LaunchCommand       string                         `json:"launch_command,omitempty"`
	LaunchInput         *m.InputBudget                 `json:"launch_input_budget,omitempty"`
	Workflow            workflow.Snapshot              `json:"workflow"`
	Intents             []operation.Intent             `json:"intents"`
	Result              *b.TurnEvidence                `json:"result,omitempty"`
	Question            *contracts.QuestionView        `json:"question,omitempty"`
	PendingAnswer       *Answer                        `json:"pending_answer,omitempty"`
	PendingFeedback     *Answer                        `json:"pending_feedback,omitempty"`
	PendingResume       string                         `json:"pending_resume,omitempty"`
	PendingStop         string                         `json:"pending_stop,omitempty"`
	PR                  *contracts.PRFact              `json:"pr,omitempty"`
	Summary             string                         `json:"summary,omitempty"`
	CreatedAt           time.Time                      `json:"created_at"`
	UpdatedAt           time.Time                      `json:"updated_at"`

	WorkspaceWriteLeaseID string `json:"workspace_write_lease_id,omitempty"`
}

// The incarnation is captured at creation, never inferred from a reuse request.
type ManagedWorkspace struct {
	Identity    string               `json:"identity"`
	OriginRunID string               `json:"origin_run_id"`
	SourceRunID string               `json:"source_run_id,omitempty"`
	Local       *b.WorkspaceIdentity `json:"local,omitempty"`
}

// Immutable program bytes share the Run's existing atomic durable boundary.
type Artifact struct {
	contracts.Artifact
	Content []byte `json:"content"`
}

func (r *Record) Clone() *Record {
	raw, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	var result Record
	if err := json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return &result
}
func (r *Record) View() contracts.RunView {
	v := contracts.RunView{RunID: r.ID, Revision: r.Revision, Title: r.Request.Title, Correlation: r.Request.Correlation, InputDigest: r.InputDigest, ResourceDigest: r.ResourceDigest, Workflow: r.Request.Workflow, Step: r.Workflow.StepID, Status: r.Status, Phase: r.Phase, Diagnostic: r.Diagnostic, NewInputsAllowed: r.Allowed, ResumeRequired: r.ResumeRequired, StopStatus: r.StopStatus, StopEvidenceScope: r.StopEvidenceScope, Question: r.Question, PR: r.PR, Summary: r.Summary, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Materials: []contracts.MaterialView{}}
	for _, a := range r.Workflow.Results {
		v.ResultAcquisition = append(v.ResultAcquisition, contracts.AcquisitionView{ID: a.ID, Contract: a.Contract, Source: a.Source, Method: a.Method, Status: a.Status, Missing: a.Missing, Conflicts: a.Conflicts, Attempts: a.Attempts, Fields: a.Fields})
	}
	for _, intent := range r.Intents {
		if q := intent.PreparationQuery; q != nil {
			v.ExecutionWait = nil
			if q.Visible {
				value := q.Wait
				v.ExecutionWait = &value
			}
		}
	}
	s := r.Snapshot
	v.Final = r.LastFinal
	v.Execution = contracts.ExecutionView{AttemptID: s.Session.AttemptID, TurnStarted: r.NativeStarted, Generation: s.ResourceGeneration, Backend: string(s.Ref.Backend), Protocol: string(s.Ref.Protocol), SessionID: s.Session.SessionID, TurnID: s.Session.CurrentTurnID}
	v.Execution.Agent = s.Ref.Agent.Name
	if len(r.Workflow.StageInputs) > 0 {
		input := r.Workflow.StageInputs[len(r.Workflow.StageInputs)-1]
		v.Execution.RequestedModel, v.Execution.RequestedReasoningEffort = input.Model, input.ReasoningEffort
	}
	if r.Workflow.AuditModel != nil {
		v.Audit = &contracts.AuditView{Model: r.Workflow.AuditModel.Model, ReasoningEffort: r.Workflow.AuditModel.ReasoningEffort, Results: []workflow.AuditRecord{}}
		for _, a := range r.Workflow.Audits {
			if a.SourceResultID == r.Workflow.ImplementationResultID {
				v.Audit.Results = append(v.Audit.Results, a)
			}
		}
	}
	if r.ManagedWorkspace != nil {
		v.ReuseWorktreeFromRunID = r.ManagedWorkspace.SourceRunID
		v.WorkspaceOriginRunID = r.ManagedWorkspace.OriginRunID
		v.WorkspaceIdentity = r.ManagedWorkspace.Identity
	}
	if r.WorkspaceFact != nil {
		v.Execution.Workspace, v.Execution.Branch, v.Execution.WorkspaceScope = r.WorkspaceFact.Worktree, r.WorkspaceFact.Branch, r.WorkspaceFact.InspectionScope
	}
	if r.Result != nil {
		v.Execution.NativeOutcome = r.Result.NativeOutcome
		v.Execution.FinalAvailable = r.Result.FinalVerified && r.Result.FinalSource != "" && r.Result.FinalItemID != ""
		v.Final = &contracts.FinalView{CommandID: r.Result.CommandID, AttemptID: s.Session.AttemptID, SessionID: r.Result.SessionID, TurnID: r.Result.TurnID, NativeOutcome: r.Result.NativeOutcome, Verified: v.Execution.FinalAvailable, Source: r.Result.FinalSource, ItemID: r.Result.FinalItemID, Diagnostic: r.Result.FinalDiagnostic}
		if v.Final.Verified {
			v.Final.Text = r.Result.LastMessage
		}
		if r.LastFinal != nil && r.LastFinal.CommandID == r.Result.CommandID {
			v.Final = r.LastFinal
		}
	}
	for _, m := range r.Materials {
		v.Materials = append(v.Materials, m.MaterialView)
	}
	unresolved := false
	for _, intent := range r.Intents {
		op := intent.Request.Operation
		view := contracts.OperationView{ID: op.ID, Action: op.Action, Status: string(op.Status), Diagnostic: op.Diagnostic}
		if intent.Receipt != nil {
			view.OriginalDiagnostic = intent.Receipt.Operation.Diagnostic
		}
		v.Operations = append(v.Operations, view)
		if intent.NeedsReconciliation() {
			unresolved = true
		}
	}
	v.Controls = contracts.Controls{CanReply: r.Allowed && !r.ResumeRequired && !unresolved && (r.Question != nil || r.Status == "blocked"), CanResume: r.ResumeRequired && (!unresolved || r.ManualPause != nil) && !r.Closed && r.PendingStop == "" && r.PendingResume == "", CanStop: r.StopStatus != "stopped", Reason: r.Diagnostic}
	v.Controls.CanMessage = r.canMessage(unresolved)
	if workflow.Content(r.Request.Workflow.ID) {
		v.RepositorySource = r.RepositorySource
		v.Closed, v.ContentResult = r.Closed, r.ContentResult
		v.Controls.CanReply = r.Allowed && !r.ResumeRequired && !unresolved && !r.Closed && r.Question != nil && r.PendingAnswer == nil && r.PendingInterrupt == ""
		v.Controls.CanResume = r.ResumeRequired && (!unresolved || r.ManualPause != nil) && !r.Closed && r.PendingStop == "" && r.PendingResume == ""
		v.Controls.CanInterrupt = r.Request.Workflow.ID == "conversation" && !r.Closed && r.Allowed && !r.ResumeRequired && !unresolved && r.PendingInterrupt == "" && s.Session.CurrentTurnID != "" && s.Session.CurrentTurnID != s.Session.LastStoppedTurnID
		v.Controls.CanClose = r.Request.Workflow.ID == "conversation" && !r.Closed
	}
	if s.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 {
		settled := false
		v.ResourcesSettled = &settled
	}
	if r.ManagedConversation() {
		v.WorkspaceWrite = &contracts.WorkspaceWriteView{Owner: "runner", CanAcquire: v.Controls.CanMessage}
		if r.WorkspaceWriteLeaseID != "" {
			v.WorkspaceWrite = &contracts.WorkspaceWriteView{Owner: "steerline", LeaseID: r.WorkspaceWriteLeaseID}
			v.NewInputsAllowed = false
			v.Controls = contracts.Controls{Reason: "SteerLine holds the managed worktree write lease."}
		}
	}
	if r.Request.Workflow.ID == workflow.SingleExecution {
		v.Closed = r.Closed
		if s.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 {
			v.ContentResult = r.ContentResult
		}
		v.Controls.CanReply = false
		v.Controls.CanResume = false
		v.Controls.CanInterrupt = false
		v.Controls.CanClose = false
	}
	return v
}

// CanMessage reports whether the saved Run facts allow a new message.
func (r *Record) CanMessage() bool {
	return r.canMessage(slices.ContainsFunc(r.Intents, operation.Intent.NeedsReconciliation))
}

// CanAcquireWorkspaceWrite is the record-side gate before native verification.
func (r *Record) CanAcquireWorkspaceWrite() bool {
	return r.ManagedConversation() && r.CanMessage()
}

func (r *Record) canMessage(unresolved bool) bool {
	if !r.Allowed || r.ResumeRequired || unresolved || r.Closed || r.PendingFeedback != nil || r.PendingAnswer != nil {
		return false
	}
	if r.Request.Workflow.ID != "conversation" {
		return r.DevelopmentMessage() && r.Status == "pr_ready" &&
			r.PendingStop == "" && r.StopStatus == "" && r.PendingResume == "" &&
			r.Question == nil && r.Snapshot.PendingTurn == nil
	}
	if r.Workflow.StepID != "input" || r.PendingInterrupt != "" {
		return false
	}
	if !r.ManagedConversation() {
		return true
	}
	session := r.Snapshot.Session
	return r.WorkspaceWriteLeaseID == "" && r.Status == "waiting_input" &&
		r.StopStatus == "" && r.PendingStop == "" && r.PendingResume == "" &&
		r.NextStage == nil && r.Question == nil && r.ManagedWorkspace != nil && r.ManagedWorkspace.Local != nil &&
		r.Snapshot.PendingTurn == nil && session.PendingCommandID == "" && session.PendingTurnID == "" &&
		session.SessionID != "" && session.CurrentTurnID != "" && session.CurrentTurnID == session.LastStoppedTurnID
}

func (r *Record) ManagedConversation() bool {
	return workflow.ManagedConversation(r.Request.Workflow.ID, r.Request.Workflow.Version)
}

// DevelopmentMessage identifies the completed development cursor, not a new workflow.
func (r *Record) DevelopmentMessage() bool {
	return (r.Request.Workflow.ID == "development" || r.Request.Workflow.ID == "development-audited") && r.Workflow.StepID == workflow.StepPRReady
}
