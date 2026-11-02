package evidence

type MessagePhase string

const (
	MessagePhaseCommentary  MessagePhase = "commentary"
	MessagePhaseFinalAnswer MessagePhase = "final_answer"
	MessagePhaseUnknown     MessagePhase = "unknown"
)
