package codexworker

// EnvironmentInputRejectedError is positive evidence that the session's
// preflight returned before turn/start. It must never describe a failed or
// ambiguous turn/start RPC. Reason contains only an EnvironmentDiagnostic code.
type EnvironmentInputRejectedError struct {
	Reason string
}

func (e *EnvironmentInputRejectedError) Error() string { return e.Reason }

func rejectEnvironmentInput(err error) error {
	return &EnvironmentInputRejectedError{Reason: EnvironmentDiagnostic(err)}
}
