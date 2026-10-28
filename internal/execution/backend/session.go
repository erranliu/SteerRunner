package backend

import (
	"fmt"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

type DecisionOption struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Detail   string `json:"detail,omitempty"`
	Receiver string `json:"receiver,omitempty"`
	Action   string `json:"action,omitempty"`
}

type Question struct {
	ExecutionID   string           `json:"execution_id,omitempty"`
	AttemptID     string           `json:"attempt_id,omitempty"`
	Generation    string           `json:"generation,omitempty"`
	TurnID        string           `json:"turn_id,omitempty"`
	ID            string           `json:"id"`
	Header        string           `json:"header"`
	Question      string           `json:"question"`
	Options       []DecisionOption `json:"options"`
	AllowFreeText bool             `json:"allow_free_text"`
}

const (
	QuestionRouteVersion       = ImplementationDataVersion
	QuestionRouteMaxBytes      = ImplementationDataMaxBytes
	QuestionRouteAnswerPurpose = ImplementationQuestionPurpose
)

type QuestionRouteIdentity struct {
	ProjectID     string
	WorkflowRunID string
	ExecutionID   string
	AttemptID     string
	Generation    string
	TurnID        string
	QuestionID    string
}

// SessionContext freezes center identity separately from verified native session
// and turn IDs. It contains values only, with no business mutation authority.
type SessionContext struct {
	ProjectID         string
	CorrelationID     string
	NamePrefix        string
	WorkflowRunID     string
	ExecutionID       string
	AttemptID         string
	Agent             AgentSpec
	Target            ExecutionTarget
	SessionID         string
	Generation        string
	CurrentTurnID     string
	PendingTurnID     string
	LastStoppedTurnID string
	CurrentCommandID  string
	PendingCommandID  string
	AttemptNumber     int
}

// TurnInput identifies a fixed input operation; accepting it does not prove
// that its native turn has started. The native session route is bound separately.
type TurnInput struct {
	Budget             *m.InputBudget `json:"input_budget,omitempty"`
	StepID             string         `json:"step_id,omitempty"`
	InputVersion       uint64         `json:"input_version,omitempty"`
	ReportToolPolicy   string         `json:"report_tool_policy,omitempty"`
	CommandID          string
	Prompt             string
	Purpose            string
	LinkedWorktreeArgs []string `json:"linked_worktree_args,omitempty"`
	Model              string   `json:"model,omitempty"`
	ReasoningEffort    string   `json:"reasoning_effort,omitempty"`
}

func (i TurnInput) ModelInputParts() ([]m.InputPart, error) {
	if i.Budget != nil {
		return m.InputParts(i.Prompt, i.Budget)
	}
	if i.InputVersion > 0 {
		return nil, fmt.Errorf("frozen input provenance unavailable; candidate not sent")
	}
	// Direct SDK/terminal callers provide text without workflow additions.
	return []m.InputPart{{Source: "caller.direct", Owner: m.CallerInput, Text: i.Prompt}}, nil
}

// Answer is the structured answer content for the original verified question.
// Its native route stays with the corresponding platform binding.
type Answer struct {
	Answers map[string][]string `json:"answers"`
}
