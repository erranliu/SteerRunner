package reliable

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"time"
)

func CanContinueOriginal(e b.LifecycleRecoveryEvidence) bool {
	return e.SessionAvailable && e.ResourceAvailable && !e.ProcessAbsent
}

var ErrContinuationUnavailable = errors.New("original session continuation is unverified")

type RecoveryRevision struct {
	State            string
	SessionAvailable *bool
	Version          uint64
}

func RecoveryChange(prior RecoveryRevision, facts RecoveryFacts, available bool, unknown bool) RecoveryRevision {
	facts.OutcomeUncertain = facts.OutcomeUncertain || unknown
	state := string(ClassifyRecovery(facts))
	if prior.SessionAvailable != nil && (prior.State != state || *prior.SessionAvailable != available) {
		prior.Version++
	}
	prior.State = state
	prior.SessionAvailable = &available
	return prior
}

type ObservationIdentity struct {
	ExecutionID, AttemptID, Generation, CommandID, TurnID, PendingCommandID, PendingTurnID string
	RecoveryVersion                                                                        uint64
}

func CurrentObservation(current, original ObservationIdentity) bool { return current == original }
func ValidateResultBinding(current, original b.LifecycleSnapshot) error {
	if current.Session.ProjectID != original.Session.ProjectID || current.Session.ExecutionID != original.Session.ExecutionID || current.Session.AttemptID != original.Session.AttemptID || current.ConfigurationDigest != original.ConfigurationDigest || current.ResourceGeneration != original.ResourceGeneration || b.LifecycleBindingDigest(current) != b.LifecycleBindingDigest(original) {
		return ErrStale
	}
	return nil
}
func InputNotSubmitted(turn, input, session string) bool {
	for _, status := range []string{turn, input} {
		if status != "" && status != "rejected" {
			return false
		}
	}
	return session == "" || session == "confirmed" || session == "rejected"
}
func PreparationWindow(deadline, at time.Time, timeout time.Duration) time.Duration {
	remaining := deadline.Sub(at)
	if remaining <= 0 {
		return 0
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return min(remaining, timeout)
}
func QualificationTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return min(timeout, 10*time.Second)
}

func ObservationTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return min(timeout, 30*time.Second)
}

func SessionPrepared(receipt b.LifecycleReceipt, err error) bool {
	return err == nil && receipt.Disposition == b.OperationConfirmed && receipt.State.SessionID != "" && receipt.Process != nil && receipt.State.SessionGeneration != ""
}
func StartAccepted(receipt b.LifecycleReceipt, err error, observer bool) bool {
	return err == nil && receipt.Disposition == b.OperationAccepted && receipt.State.InputAccepted && receipt.State.TurnStarted && receipt.State.TurnID != "" && observer
}
func FailedStart(receipt b.LifecycleReceipt, status string, action b.LifecycleAction) bool {
	return action == b.LifecycleStartTurn && status == "rejected" && receipt.State.Stopped && receipt.State.WorkerAbsent && receipt.State.WritersAbsent
}

type RecoveryNotice string

const (
	RecoveryWritersUnverified      RecoveryNotice = "writers_unverified"
	RecoveryOperationUnknown       RecoveryNotice = "operation_unknown"
	RecoveryOriginalCompleted      RecoveryNotice = "original_completed"
	RecoveryOriginalInterrupted    RecoveryNotice = "original_interrupted"
	RecoveryObservationUnavailable RecoveryNotice = "observation_unavailable"
	RecoveryObservationAttached    RecoveryNotice = "observation_attached"
)

func RecoveryResult(dead, completed, writersAbsent, unknown bool, readErr error) RecoveryNotice {
	if dead {
		if !writersAbsent {
			return RecoveryWritersUnverified
		}
		if unknown {
			return RecoveryOperationUnknown
		}
		if completed {
			return RecoveryOriginalCompleted
		}
		return RecoveryOriginalInterrupted
	}
	if readErr != nil {
		return RecoveryObservationUnavailable
	}
	return RecoveryObservationAttached
}
