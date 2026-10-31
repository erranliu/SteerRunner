package codexworker

import (
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestVerifyPolicyMinimumVersion(t *testing.T) {
	for _, tc := range []struct {
		version string
		accept  bool
	}{
		{"codex-cli 0.159.2", true},
		{"\r\ncodex-cli 0.159.2\r\n", true},
		{"codex-cli 0.159.2+build.7", true},
		{"codex-cli 0.159.3", true},
		{"codex-cli 0.159.10", true},
		{"codex-cli 0.160.0", true},
		{"codex-cli 0.163.0-alpha.10.2", true},
		{"codex-cli 1.0.0", true},
		{"codex-cli 10.0.0-rc.1+build.2", true},
		{"codex-cli 0.159.1", false},
		{"codex-cli 0.158.99", false},
		{"codex-cli 0.99.999", false},
		{"codex-cli 0.159.2-alpha.1", false},
		{"codex-cli 0.159.2-rc.1+build.2", false},
		{"codex-cli 0.0160.0", false},
		{"codex-cli 0.160.0-alpha.01", false},
		{"codex-cli 0.160", false},
		{"codex-cli 0.160.0.1", false},
		{"codex-cli 0.160.0-alpha..1", false},
		{"codex-cli 0.160.0-", false},
		{"codex-cli 0.160.0+", false},
		{"codex-cli v0.160.0", false},
		{"codex 0.160.0", false},
		{"0.160.0", false},
		{"codex-cli 0.160.0\nwarning", false},
		{"", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			err := VerifyPolicyVersion(tc.version)
			if (err == nil) != tc.accept {
				t.Fatalf("VerifyPolicyVersion(%q) = %v; accept=%v", tc.version, err, tc.accept)
			}
			if err != nil && !strings.Contains(err.Error(), ">= 0.159.2") {
				t.Fatalf("diagnostic lacks minimum version: %v", err)
			}
		})
	}
}

func TestPolicyResourceAcceptsNewerNativeVersion(t *testing.T) {
	identity := PolicyResourceIdentity{ProjectID: "project", ExecutionID: "execution", AttemptID: "attempt", Generation: "generation", ConfigDigest: "profile"}
	permissions, err := NativePermissions("windows", "local", "codex", "full_access")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{PolicyCodexVersion, "codex-cli 0.163.0-alpha.10.2", "codex-cli 0.159.1"} {
		envelope, err := NewPolicyResourceEnvelope(identity, PolicyResourcePayload{NativeVersion: version, Contract: PolicyContract, Settings: b.LaunchCommand{Executable: "codex.exe", Args: permissions.Args(), WorkingDirectory: "workspace"}})
		if err != nil {
			t.Fatal(err)
		}
		payload, err := DecodePolicyResourceEnvelope(envelope, identity)
		if version == "codex-cli 0.159.1" {
			if err == nil {
				t.Fatal("old version accepted in persisted permission resource")
			}
		} else if err != nil || payload.NativeVersion != version {
			t.Fatalf("actual version was rejected or lost: %+v, %v", payload, err)
		}
	}
}
