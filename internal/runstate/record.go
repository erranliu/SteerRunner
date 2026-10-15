// Package runstate holds the service's sole durable execution aggregate.
// It contains no caller Task, DAG, delivery or storage capabilities.
package runstate

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"time"
)

type Material struct {
	contracts.MaterialView
	Content string `json:"content"`
}
type Intent struct {
	Request b.LifecycleRequest  `json:"request"`
	Receipt *b.LifecycleReceipt `json:"receipt,omitempty"`
	Mode    string              `json:"mode"`
}

func (i Intent) NeedsReconciliation() bool {
	status := i.Request.Operation.Status
	return status == b.OperationInFlight || status == b.OperationUnknown || status == b.OperationAccepted && i.Receipt != nil && i.Receipt.State.ResourcePending
}

type Answer struct {
	Epoch      string              `json:"epoch"`
	CommandID  string              `json:"command_id"`
	QuestionID string              `json:"question_id"`
	Text       string              `json:"text"`
	Answers    map[string][]string `json:"answers,omitempty"`
}
type Attempt struct {
	Snapshot b.LifecycleSnapshot `json:"snapshot"`
	EndedAt  time.Time           `json:"ended_at"`
}
type Record struct {
	PRClues             []string                       `json:"pr_clues,omitempty"`
	PRClueProbeAt       time.Time                      `json:"pr_clue_probe_at,omitempty"`
	LastFinal           *contracts.FinalView           `json:"last_final,omitempty"`
	NativeStarted       bool                           `json:"native_started"`
	SeenNative          map[string]string              `json:"seen_native,omitempty"`
	AnswerHistory       []Answer                       `json:"answer_history,omitempty"`
	LinkedWriteArgs     []string                       `json:"linked_write_args,omitempty"`
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
	Workflow            workflow.Snapshot              `json:"workflow"`
	Intents             []Intent                       `json:"intents"`
	Attempts            []Attempt                      `json:"attempts,omitempty"`
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
	v := contracts.RunView{RunID: r.ID, Revision: r.Revision, Title: r.Request.Title, Correlation: r.Request.Correlation, InputDigest: r.InputDigest, ResourceDigest: r.ResourceDigest, Workflow: r.Request.Workflow, Step: r.Workflow.StepID, Status: r.Status, Phase: r.Phase, Diagnostic: r.Diagnostic, NewInputsAllowed: r.Allowed, ResumeRequired: r.ResumeRequired, StopStatus: r.StopStatus, Question: r.Question, PR: r.PR, Summary: r.Summary, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Materials: []contracts.MaterialView{}}
	s := r.Snapshot
	v.Final = r.LastFinal
	v.Execution = contracts.ExecutionView{AttemptID: s.Session.AttemptID, TurnStarted: r.NativeStarted, Generation: s.ResourceGeneration, Backend: string(s.Ref.Backend), Protocol: string(s.Ref.Protocol), SessionID: s.Session.SessionID, TurnID: s.Session.CurrentTurnID}
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
	v.Controls = contracts.Controls{CanReply: r.Allowed && !r.ResumeRequired && !unresolved && (r.Question != nil || r.Status == "pr_ready" || r.Status == "blocked"), CanResume: r.ResumeRequired && !unresolved, CanStop: r.StopStatus != "stopped", Reason: r.Diagnostic}
	return v
}
