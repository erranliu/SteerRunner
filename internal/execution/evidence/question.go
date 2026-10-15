package evidence

import executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"

type DecisionRequest struct {
	InteractionKind string                                       `json:"interaction_kind,omitempty"`
	BlocksProgress  bool                                         `json:"blocks_progress"`
	FormatVersion   int                                          `json:"format_version"`
	Question        string                                       `json:"question"`
	Context         string                                       `json:"context,omitempty"`
	Options         []executionbackend.DecisionOption            `json:"options"`
	Questions       []executionbackend.Question                  `json:"questions,omitempty"`
	QuestionRoute   *executionbackend.ImplementationDataEnvelope `json:"question_route,omitempty"`
	Recommended     string                                       `json:"recommended,omitempty"`
	AllowFreeText   bool                                         `json:"allow_free_text,omitempty"`
}
