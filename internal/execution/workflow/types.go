package workflow

import "encoding/json"

const DevelopmentFlow = "development/2"

const (
	StepImplement      = "implement.execute"
	StepReport         = "implement.report"
	StepPublish        = "publish.execute"
	StepPublishReport  = "publish.report"
	StepPRReady        = "pr_ready"
	StepImplementInput = "implement.input"
	StepPublishVerify  = "publish.verify"
	StepAuditCode      = "audit.code.execute"
	StepAuditParse     = "audit.code.parse"
	StepAuditReport    = "audit.code.report"
	StepAuditInput     = "audit.code.input"
)

// StageInput is the frozen business binding of a native input. Native transport
// purpose remains business/readiness; it is not the business step identifier.
type StageInput struct {
	FeedbackDecisionID     string          `json:"feedback_decision_id,omitempty"`
	CommandID              string          `json:"command_id"`
	AttemptID              string          `json:"attempt_id"`
	StepID                 string          `json:"step_id"`
	Version                uint64          `json:"version"`
	Prompt                 string          `json:"prompt"`
	MaterialFacts          json.RawMessage `json:"material_facts,omitempty"`
	SourceCommandID        string          `json:"source_command_id,omitempty"`
	SourceTurnID           string          `json:"source_turn_id,omitempty"`
	ReportToolPolicy       string          `json:"report_tool_policy,omitempty"`
	Model                  string          `json:"model,omitempty"`
	ReasoningEffort        string          `json:"reasoning_effort,omitempty"`
	SourceVersion          uint64          `json:"source_version,omitempty"`
	SourceResultID         string          `json:"source_result_id,omitempty"`
	ImplementationResultID string          `json:"implementation_result_id,omitempty"`
	AuditResultID          string          `json:"audit_result_id,omitempty"`
	SuccessOutput          string          `json:"success_output,omitempty"`
}

// StageResult contains normalized business source facts. Native provenance
// is validated by reliable and retained only in the actor's persisted record.
type StageResult struct {
	SessionID                                string   `json:"session_id,omitempty"`
	PRCandidates                             []string `json:"pr_candidates,omitempty"`
	UnexpectedWork                           bool     `json:"unexpected_work,omitempty"`
	CommandID, AttemptID, StepID             string
	InputVersion                             uint64
	TurnID                                   string // opaque source correlation, never decoded
	Succeeded, FinalVerified                 bool
	Final, Failure, FinalDiagnostic, Outcome string
	Workspace                                *WorkspaceSource `json:"workspace,omitempty"`
}

func PublicationStep(step string) bool { return step == StepPublish || step == StepPublishReport }
