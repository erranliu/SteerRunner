package reliable

import (
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"testing"
)

func TestContinuationRequiresOriginalIdleResourceOrVerifiedRestoration(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		facts                         b.LifecycleRecoveryEvidence
		unknown, needs, restore, fail bool
	}{
		{"live", b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, ResourceAvailable: true}, false, true, false, false},
		{"busy", b.LifecycleRecoveryEvidence{SessionAvailable: true, ResourceAvailable: true}, false, true, false, true},
		{"lost-writers", b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true}, false, true, false, true},
		{"idle-no-input", b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true, WritersAbsent: true}, false, false, false, false},
		{"restore", b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true, WritersAbsent: true}, false, true, true, false},
		{"unknown", b.LifecycleRecoveryEvidence{SessionAvailable: true, SessionIdle: true, SessionRebindable: true, ProcessAbsent: true, WritersAbsent: true}, true, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore, err := PlanContinuation(tc.facts, "original", tc.unknown, tc.needs)
			if restore != tc.restore || (err != nil) != tc.fail {
				t.Fatal(restore, err)
			}
		})
	}
}
