package reliable

import (
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func CanContinueOriginal(e b.LifecycleRecoveryEvidence) bool {
	return e.SessionAvailable && e.ResourceAvailable && !e.ProcessAbsent
}

var ErrContinuationUnavailable = errors.New("original session continuation is unverified")

func ValidateResultBinding(current, original b.LifecycleSnapshot) error {
	if current.Session.ProjectID != original.Session.ProjectID || current.Session.ExecutionID != original.Session.ExecutionID || current.Session.AttemptID != original.Session.AttemptID || current.ConfigurationDigest != original.ConfigurationDigest || current.ResourceGeneration != original.ResourceGeneration || b.LifecycleBindingDigest(current) != b.LifecycleBindingDigest(original) {
		return ErrStale
	}
	return nil
}
