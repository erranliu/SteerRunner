package contracts

import (
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"time"
)

type MaterialView struct {
	Kind              string `json:"kind"`
	Path              string `json:"path"`
	Commit            string `json:"commit,omitempty"`
	Digest            string `json:"digest"`
	SnapshotPath      string `json:"snapshot_path"`
	ExecutionLocation string `json:"execution_location,omitempty"`
}

type ExecutionView struct {
	RequestedModel           string `json:"requested_model,omitempty"`
	RequestedReasoningEffort string `json:"requested_reasoning_effort,omitempty"`
	WorkspaceScope           string `json:"workspace_scope,omitempty"`
	TurnStarted              bool   `json:"turn_started"`
	AttemptID                string `json:"attempt_id"`
	Generation               string `json:"generation"`
	Backend                  string `json:"backend"`
	Protocol                 string `json:"protocol"`
	SessionID                string `json:"session_id,omitempty"`
	TurnID                   string `json:"turn_id,omitempty"`
	NativeOutcome            string `json:"native_outcome,omitempty"`
	FinalAvailable           bool   `json:"final_available"`
	Workspace                string `json:"workspace"`
	Branch                   string `json:"branch"`
}

type QuestionOption struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}
type QuestionView struct {
	ID              string           `json:"id"`
	Text            string           `json:"text"`
	Options         []QuestionOption `json:"options,omitempty"`
	AllowFreeText   bool             `json:"allow_free_text"`
	NativeQuestions []QuestionView   `json:"native_questions,omitempty"`
}
type PRFact struct {
	URL            string     `json:"url"`
	Provider       string     `json:"provider"`
	Repository     string     `json:"repository"`
	Number         int        `json:"number"`
	BaseBranch     string     `json:"base_branch"`
	HeadRepository string     `json:"head_repository"`
	HeadBranch     string     `json:"head_branch"`
	HeadRevision   string     `json:"head_revision,omitempty"`
	State          string     `json:"state"`
	Merged         bool       `json:"merged"`
	MergedAt       *time.Time `json:"merged_at,omitempty"`
	MergeCommit    *string    `json:"merge_commit,omitempty"`
	CheckedAt      time.Time  `json:"checked_at"`
	QueryError     string     `json:"query_error,omitempty"`
}
type Controls struct {
	CanReply  bool   `json:"can_reply"`
	CanResume bool   `json:"can_resume"`
	CanStop   bool   `json:"can_stop"`
	Reason    string `json:"reason,omitempty"`
}

// FinalView keeps native outcome separate from verified original result content.
type FinalView struct {
	CommandID     string `json:"command_id"`
	AttemptID     string `json:"attempt_id"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id"`
	NativeOutcome string `json:"native_outcome"`
	Verified      bool   `json:"verified"`
	Source        string `json:"source,omitempty"`
	ItemID        string `json:"item_id,omitempty"`
	Text          string `json:"text,omitempty"`
	Diagnostic    string `json:"diagnostic,omitempty"`
}

type OperationView struct {
	ID                 string `json:"operation_id"`
	Action             string `json:"action"`
	Status             string `json:"status"`
	Diagnostic         string `json:"diagnostic,omitempty"`
	OriginalDiagnostic string `json:"original_diagnostic,omitempty"`
}
type RunView struct {
	Audit            *AuditView        `json:"audit,omitempty"`
	Operations       []OperationView   `json:"operations"`
	RunID            string            `json:"run_id"`
	Revision         uint64            `json:"revision"`
	Title            string            `json:"title"`
	Correlation      map[string]string `json:"correlation,omitempty"`
	InputDigest      string            `json:"input_digest"`
	ResourceDigest   string            `json:"resource_digest"`
	Workflow         Workflow          `json:"workflow"`
	Step             string            `json:"step"`
	Status           string            `json:"status"`
	Phase            string            `json:"phase"`
	Diagnostic       string            `json:"diagnostic,omitempty"`
	NewInputsAllowed bool              `json:"new_inputs_allowed"`
	ResumeRequired   bool              `json:"resume_required"`
	StopStatus       string            `json:"stop_status,omitempty"`
	Materials        []MaterialView    `json:"materials"`
	Execution        ExecutionView     `json:"execution"`
	Question         *QuestionView     `json:"question,omitempty"`
	Controls         Controls          `json:"controls"`
	PR               *PRFact           `json:"pr,omitempty"`
	Summary          string            `json:"summary,omitempty"`
	Final            *FinalView        `json:"final,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type AuditView struct {
	Model           string                 `json:"model"`
	ReasoningEffort string                 `json:"reasoning_effort"`
	RepairCount     uint64                 `json:"repair_count"`
	Results         []workflow.AuditRecord `json:"results"`
}
type Event struct {
	Transition *WorkflowTransition `json:"transition,omitempty"`
	EventID    string              `json:"event_id"`
	RunID      string              `json:"run_id"`
	Revision   uint64              `json:"revision"`
	Type       string              `json:"type"`
	CommandID  string              `json:"command_id,omitempty"`
	AttemptID  string              `json:"attempt_id,omitempty"`
	Final      *FinalView          `json:"final,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
}

// WorkflowTransition is captured in the same transaction as the aggregate.
type WorkflowState struct {
	Stage      string `json:"workflow_stage"`
	Status     string `json:"workflow_status"`
	Diagnostic string `json:"diagnostic,omitempty"`
}
type WorkflowTransition struct {
	From WorkflowState `json:"from"`
	To   WorkflowState `json:"to"`
}

// LastMessage is best-effort telemetry, never execution or recovery authority.
// ObservedAt is when the native notification arrived, not a query timestamp.
type LastMessage struct {
	RunID      string    `json:"run_id"`
	AttemptID  string    `json:"attempt_id"`
	SessionID  string    `json:"session_id"`
	TurnID     string    `json:"turn_id"`
	Kind       string    `json:"kind"`
	Text       string    `json:"text"`
	ObservedAt time.Time `json:"observed_at"`
}
type LastMessageView struct {
	RunID     string       `json:"run_id"`
	AttemptID string       `json:"attempt_id"`
	Message   *LastMessage `json:"message"`
}
type EventPage struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}
