package codexworker

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestPolicyResourceMatchesOriginalProfile(t *testing.T) {
	resourceID := PolicyResourceIdentity{ProjectID: "project", ExecutionID: "execution", AttemptID: "attempt", Generation: "generation", ConfigDigest: "config"}
	profileID := ProfileConfigIdentity{ProjectID: "project", ProfileID: "profile", AcceptedConfigVersion: "config"}
	for _, resourceMode := range []string{"full_access", "read_only"} {
		for _, profileMode := range []string{"full_access", "read_only"} {
			t.Run(resourceMode+"/"+profileMode, func(t *testing.T) {
				permissions, err := NativePermissions("windows", "local", "codex", resourceMode)
				if err != nil {
					t.Fatal(err)
				}
				payload := PolicyResourcePayload{NativeVersion: PolicyCodexVersion, Contract: permissions.Contract, Settings: b.LaunchCommand{Executable: "original.exe", Args: permissions.Args(), WorkingDirectory: "original"}}
				resource, err := NewPolicyResourceEnvelope(resourceID, payload)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := EncodeProfileConfig(ProfileConfig{AccessMode: profileMode, Executable: "original.exe"})
				if err != nil {
					t.Fatal(err)
				}
				profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: ProfileCodecKey, CodecVersion: ProfileVersionForMode(profileMode), Purpose: b.ImplementationConfigPurpose, ProjectID: profileID.ProjectID, ProfileID: profileID.ProfileID, AcceptedConfigVersion: profileID.AcceptedConfigVersion}, raw)
				if err != nil {
					t.Fatal(err)
				}
				profiles := []b.ImplementationDataEnvelope{profile}
				before, _ := json.Marshal([]any{resource, profiles})
				check := func(resource b.ImplementationDataEnvelope, identity PolicyResourceIdentity, profiles []b.ImplementationDataEnvelope, id ProfileConfigIdentity) error {
					return ValidatePolicyResourceForProfile(resource, identity, profiles, id)
				}
				err = check(resource, resourceID, profiles, profileID)
				if (err == nil) != (resourceMode == profileMode) {
					t.Fatalf("mode matching: %v", err)
				}
				if err != nil && err.Error() != "permission resources differ from frozen access mode" {
					t.Fatal(err)
				}
				after, _ := json.Marshal([]any{resource, profiles})
				if !bytes.Equal(before, after) {
					t.Fatal("validation changed original resources/profile")
				}
				wrongResource := resourceID
				wrongResource.AttemptID = "other"
				if err := check(resource, wrongResource, nil, profileID); err == nil || !strings.Contains(err.Error(), "resource") {
					t.Fatal("resource error lost priority over missing profile", err)
				}
				wrongProfile := profileID
				wrongProfile.AcceptedConfigVersion = "other"
				if err := check(resource, resourceID, profiles, wrongProfile); err == nil || !strings.Contains(err.Error(), "profile") {
					t.Fatal("profile error lost priority over mode matching", err)
				}
				for _, invalid := range []string{"old-version", "argv", "unknown-field"} {
					bad := payload
					switch invalid {
					case "old-version":
						bad.NativeVersion = "codex-cli 0.159.1"
					case "argv":
						bad.Settings.Args = append(append([]string(nil), bad.Settings.Args...), "--config", "sandbox_mode=workspace-write")
					}
					encoded, _ := json.Marshal(bad)
					if invalid == "unknown-field" {
						encoded = append(encoded[:len(encoded)-1], []byte(`,"unrecognized":true}`)...)
					}
					badEnvelope, err := b.NewImplementationDataEnvelope(resource, encoded)
					if err != nil {
						t.Fatal(err)
					}
					if err := check(badEnvelope, resourceID, profiles, profileID); err == nil || err.Error() == "permission resources differ from frozen access mode" {
						t.Fatal(invalid, "did not fail in original strict resource decoder", err)
					}
				}
			})
		}
	}
}
