package contracts

// NativeSessionTarget contains public identities only. Native endpoints and
// adapter bindings remain private to the resident owner.
type NativeSessionTarget struct {
	RunID      string `json:"run_id"`
	AttemptID  string `json:"attempt_id"`
	SessionID  string `json:"session_id"`
	Generation string `json:"generation"`
}

type NativeSessionOpened struct {
	NativeSessionTarget
	Opened bool `json:"opened"`
}
