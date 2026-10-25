package backend

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"
)

const ExecutionWaitContract = "steerrunner-execution-wait/v1"

// PreparationQuery is a fact belonging to the original operation. Visibility
// may end before that operation settles; its budget must survive that gap.
type PreparationQuery struct {
	Wait    ExecutionWait `json:"wait"`
	Visible bool          `json:"visible"`
}

type ExecutionWait struct {
	Contract      string     `json:"contract"`
	OperationID   string     `json:"operation_id"`
	Query         string     `json:"query"`
	Stage         string     `json:"stage"`
	ReasonCode    string     `json:"reason_code"`
	StartedAt     time.Time  `json:"started_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeadlineAt    time.Time  `json:"deadline_at"`
	CallsStarted  int        `json:"calls_started"`
	MaxCalls      int        `json:"max_calls"`
	NextAttemptAt *time.Time `json:"next_attempt_at"`
	Diagnostic    string     `json:"diagnostic"`
}

func (q PreparationQuery) Validate(op Operation) error {
	w := q.Wait
	invalid := func() error {
		return errors.New("original preparation query contract or operation binding is unsupported")
	}
	if op.Action != string(LifecyclePrepareWorkspace) || w.OperationID != op.ID || w.OperationID == "" || w.Contract != ExecutionWaitContract ||
		(w.Query != "runtime_status" && w.Query != "repository_inventory") || w.MaxCalls != 5 || w.CallsStarted < 0 || w.CallsStarted > 5 ||
		w.StartedAt.IsZero() || w.UpdatedAt.IsZero() || w.DeadlineAt.IsZero() || len(w.Diagnostic) > 1024 || !utf8.ValidString(w.Diagnostic) {
		return invalid()
	}
	for _, at := range []time.Time{w.StartedAt, w.UpdatedAt, w.DeadlineAt} {
		if _, offset := at.Zone(); offset != 0 {
			return invalid()
		}
	}
	switch w.Stage {
	case "queue":
		if w.ReasonCode != "orca_cli_busy" || w.CallsStarted >= 5 {
			return invalid()
		}
	case "calling":
		if w.ReasonCode != "orca_query_running" || w.CallsStarted == 0 {
			return invalid()
		}
	case "backoff":
		if w.ReasonCode != "orca_query_transient" || w.CallsStarted == 0 || w.CallsStarted >= 5 || w.NextAttemptAt == nil {
			return invalid()
		}
	case "exhausted":
		if w.ReasonCode != "wait_budget_exhausted" && w.ReasonCode != "query_attempts_exhausted" {
			return invalid()
		}
	default:
		return invalid()
	}
	if w.Stage != "backoff" && w.NextAttemptAt != nil {
		return invalid()
	}
	return nil
}

type preparationProgressKey struct{}
type preparationRecoveryKey struct{}

func WithPreparationRecovery(ctx context.Context, authorized bool) context.Context {
	return context.WithValue(ctx, preparationRecoveryKey{}, authorized)
}
func PreparationRecovery(ctx context.Context) bool {
	return ctx.Value(preparationRecoveryKey{}) == true
}
func PreparationQueryRecorded(ctx context.Context) bool {
	_, recorded := ctx.Value(preparationRecoveryKey{}).(bool)
	return recorded
}

type PreparationProgress func(string, func(*PreparationQuery) error) (*PreparationQuery, error)

// Installed only inside the existing ReadOnlyPreparation scope. Adapters have
// no application/storage capability and cannot create an operation or window.
func WithPreparationProgress(ctx context.Context, progress PreparationProgress) context.Context {
	return context.WithValue(ctx, preparationProgressKey{}, progress)
}
func PreparationProgressFrom(ctx context.Context) PreparationProgress {
	progress, _ := ctx.Value(preparationProgressKey{}).(PreparationProgress)
	return progress
}
