package orcatransport

import (
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"reflect"
	"testing"
)

func TestRecoveryProfileQualificationMatchesConstructorWithoutNativeCapabilities(t *testing.T) {
	for _, environment := range []string{"", "remote"} {
		t.Run(environment, func(t *testing.T) {
			l, host, _ := fixtureOn(t, environment)
			original := l.setup.Clone()
			for _, variant := range []string{"valid", "version", "identity", "target", "retired-authentication"} {
				t.Run(variant, func(t *testing.T) {
					setup := original.Clone()
					switch variant {
					case "version":
						setup.ProfileData[0].CodecVersion = 99
					case "identity":
						setup.ProfileData[0].AcceptedConfigVersion = "different"
					case "target":
						setup.Snapshot.Ref.Target.ID = "different"
					case "retired-authentication":
						var fields map[string]any
						_ = json.Unmarshal(setup.ProfileData[0].Payload, &fields)
						fields["github_auth"] = nil
						setup.ProfileData[0].Payload, _ = json.Marshal(fields)
					}
					rebuilt, rebuildErr := b.NewImplementationDataEnvelope(setup.ProfileData[0], setup.ProfileData[0].Payload)
					if rebuildErr != nil {
						t.Fatal(rebuildErr)
					}
					setup.ProfileData[0] = rebuilt
					before, encodeErr := json.Marshal(struct {
						Snapshot b.LifecycleSnapshot
						Profile  []b.ImplementationDataEnvelope
					}{setup.Snapshot, setup.ProfileData})
					if encodeErr != nil {
						t.Fatal(encodeErr)
					}
					_, oldErr := NewLifecycle(setup)
					setup.ProcessHost = nil
					err := ValidateRecovery(setup)
					if (oldErr == nil) != (err == nil) || oldErr != nil && oldErr.Error() != err.Error() {
						t.Fatal("qualification changed", oldErr, err)
					}
					setup.ProcessHost = original.ProcessHost
					after, encodeErr := json.Marshal(struct {
						Snapshot b.LifecycleSnapshot
						Profile  []b.ImplementationDataEnvelope
					}{setup.Snapshot, setup.ProfileData})
					if encodeErr != nil {
						t.Fatal(encodeErr)
					}
					if !reflect.DeepEqual(before, after) || len(host.calls) != 0 {
						t.Fatal("read-only qualification changed source or called native transport")
					}
				})
			}
		})
	}
}
