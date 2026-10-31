package codexworker

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestRecoveryCompatibilityPreservesOriginalOwnerRequirements(t *testing.T) {
	root := t.TempDir()
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: ProfileCodecKey, CodecVersion: ProfileCodecVersion, Purpose: b.ImplementationConfigPurpose, ProjectID: "project", ProfileID: "run", AcceptedConfigVersion: "config"}, []byte(`{"executable":"codex.exe"}`))
	if err != nil {
		t.Fatal(err)
	}
	identity := AppServerBindingIdentity{ProjectID: "project", ExecutionID: "run", AttemptID: "attempt", Generation: "generation", ConfigDigest: "config"}
	current, err := process.NewServiceFacts().CurrentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"compatible", "legacy-capability", "missing-checkpoint", "settled", "owner-protocol", "legacy-live", "unknown-codec", "retired-authentication"} {
		t.Run(variant, func(t *testing.T) {
			owner := &ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: filepath.Join(root, "original-owner"), Process: *current, RuntimeSHA256: strings.Repeat("a", 64), ExitPublicKey: strings.Repeat("b", 64), WorkspaceCheckpoints: true}
			needs := b.RecoveryRequirements{WorkspaceCheckpoints: true}
			protocol := ExecutionOwnerProtocol
			want := ""
			switch variant {
			case "legacy-capability":
				owner.LegacyParserCapability = true
			case "missing-checkpoint":
				owner.WorkspaceCheckpoints = false
				want = "original owner lacks required completion-checkpoint capability; no live owner replaced"
			case "settled":
				owner.WorkspaceCheckpoints = false
				needs = b.RecoveryRequirements{Stopped: true}
			case "owner-protocol":
				protocol = "unsupported"
				want = "original execution owner protocol unsupported"
			case "legacy-live":
				owner = nil
				want = "legacy service owns the live native Job; independent transfer is unavailable"
			}
			binding, err := NewAppServerBindingEnvelope(identity, AppServerBinding{Owner: owner, Endpoint: "ws://127.0.0.1:1", Generation: "native"})
			if err != nil {
				t.Fatal(err)
			}
			p := profile.Clone()
			if variant == "unknown-codec" {
				binding.CodecVersion = 99
				_, err = DecodeAppServerBindingEnvelope(binding, identity)
				want = err.Error()
			}
			if variant == "retired-authentication" {
				p.Payload = []byte(`{"executable":"codex.exe","github_auth":null}`)
				p, err = b.NewImplementationDataEnvelope(p, p.Payload)
				if err != nil {
					t.Fatal(err)
				}
				_, err = DecodeProfileConfigEnvelope([]b.ImplementationDataEnvelope{p}, ProfileConfigIdentity{ProjectID: "project", ProfileID: "run", AcceptedConfigVersion: "config"})
				if err == nil {
					t.Fatal("retired profile accepted by baseline decoder")
				}
				want = err.Error()
			}
			binding, err = b.NewImplementationDataEnvelope(binding, binding.Payload)
			if err != nil {
				t.Fatal(err)
			}
			setup := b.LifecycleSetup{ProfileData: []b.ImplementationDataEnvelope{p}, ProfileID: "run", AcceptedConfigVersion: "config", Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{External: *current}, Session: b.SessionContext{ProjectID: "project", ExecutionID: "run", AttemptID: "attempt"}, ResourceGeneration: "generation", ConfigurationDigest: "config", Implementation: []b.ImplementationDataEnvelope{binding}}}
			before, _ := json.Marshal(struct {
				Snapshot b.LifecycleSnapshot
				Profile  []b.ImplementationDataEnvelope
			}{setup.Snapshot, setup.ProfileData})
			err = ValidateRecovery(setup, nil, protocol, needs)
			if want == "" && err != nil || want != "" && (err == nil || err.Error() != want) {
				t.Fatal("original compatibility boundary changed", variant, want, err)
			}
			after, _ := json.Marshal(struct {
				Snapshot b.LifecycleSnapshot
				Profile  []b.ImplementationDataEnvelope
			}{setup.Snapshot, setup.ProfileData})
			if !bytes.Equal(before, after) {
				t.Fatal("compatibility check rewrote frozen input")
			}
		})
	}
}
