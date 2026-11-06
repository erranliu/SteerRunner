package orcatransport

import (
	"context"
	"reflect"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestFrozenReaderHasNoExecutionAndRejectsMutation(t *testing.T) {
	l, host, _ := sharedNativeFixture(t, "")
	original := l.setup.Clone()
	r, err := newFrozenReader(l.setup)
	if err != nil {
		t.Fatal(err)
	}
	if _, executable := any(r).(b.LifecyclePort); executable {
		t.Fatal("offline proof became an executable port")
	}
	before := len(host.calls)
	_, _, err = r.client.CloseTerminal(context.Background(), "original-handle", "original-runtime")
	if err == nil || !strings.Contains(err.Error(), "does not permit mutation") || len(host.calls) != before {
		t.Fatal("proof reader dispatched a native mutation", err)
	}
	scope, err := StopEvidenceScope(l.setup)
	if err != nil || scope != "orca_terminal" || len(host.calls) != before {
		t.Fatal("scope classification performed native I/O", scope, err)
	}
	if !reflect.DeepEqual(original, l.setup) {
		t.Fatal("reader changed the original setup")
	}
	changed := r.setup.Snapshot.Clone()
	changed.ConfigurationDigest = "different-original-config"
	if _, err := r.resourceState(changed); err == nil {
		t.Fatal("reader accepted another resource binding")
	}
}

func TestFrozenReaderKeepsConstructorFaultPriority(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"agent", "only Codex and Pi"},
		{"host", "unsupported Orca terminal lifecycle"},
		{"profile", "invalid terminal profile codec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _, _ := frozenFixture(t)
			setup := l.setup.Clone()
			setup.ProfileData[0].CodecVersion = -1
			if tc.name == "agent" {
				setup.Snapshot.Ref.Agent.Name = "dsh"
				setup.ProcessHost = nil
			} else if tc.name == "host" {
				setup.ProcessHost = nil
			}
			_, err := newFrozenReader(setup)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal("reader constructor error priority changed", err)
			}
		})
	}
}
