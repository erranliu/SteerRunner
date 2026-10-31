package codexworker

import (
	"encoding/json"
	"strings"
	"testing"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestPS2RejectsAllRetiredCodexOverrides(t *testing.T) {
	for _, field := range []string{"args", "profile", "windows_sandbox", "sandbox", "sandbox_mode", "approval_policy", "approvals_reviewer"} {
		for _, value := range []any{nil, "", []string{}, map[string]any{}} {
			raw, err := json.Marshal(map[string]any{"executable": "must-not-run", field: value})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := NormalizeProfilePayload(raw, "windows", "local", "codex"); err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("retired %s=%v accepted: %v", field, value, err)
			}
		}
	}
}

func TestPS2NativeResourceRejectsApprovalChanges(t *testing.T) {
	identity := PolicyResourceIdentity{ProjectID: "project", ExecutionID: "execution", AttemptID: "attempt", Generation: "generation", ConfigDigest: "profile"}
	permissions, err := NativePermissions("windows", "local", "codex", "full_access")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]string) []string{
		func(v []string) []string { return v },
		func(v []string) []string { return append(v, "--config", "approval_policy=never") },
		func(v []string) []string { return append(v, "--sandbox", "danger-full-access") },
		func(v []string) []string { return nil },
	} {
		args := mutate(permissions.Args())
		envelope, err := NewPolicyResourceEnvelope(identity, PolicyResourcePayload{NativeVersion: PolicyCodexVersion, Contract: PolicyContract, Settings: executionbackend.LaunchCommand{Executable: "codex.exe", Args: args, WorkingDirectory: "candidate"}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = DecodePolicyResourceEnvelope(envelope, identity)
		valid := len(args) == len(permissions.Args())
		if (err == nil) != valid {
			t.Fatal("current resource accepted invalid native launch settings", args, err)
		}
	}
}

func TestPS2NativeLaunchAndInputForbidEscalation(t *testing.T) {
	for _, host := range []string{"local", "orca"} {
		permissions, err := NativePermissions("windows", host, "codex", "full_access")
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Join(permissions.Args(), " ")
		for _, setting := range []string{"sandbox_mode=danger-full-access", "approval_policy=never", "approvals_reviewer=user"} {
			if !strings.Contains(args, setting) {
				t.Fatal("missing", setting, args)
			}
		}
		for _, forbidden := range []string{"=elevated", "=unelevated", "workspace-write", "windows.sandbox=", "auto_review", "approval_policy=on-request"} {
			if strings.Contains(args, forbidden) {
				t.Fatal("unexpected fallback", args)
			}
		}
	}
	thread, err := ThreadStartParams("candidate", "danger-full-access", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	turn, err := TurnStartParams("candidate", "thread", "command", "prompt", "danger-full-access", "", "", []string{"candidate/.git/admin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, params := range []map[string]any{thread, turn} {
		if params["approvalPolicy"] != "never" || params["approvalsReviewer"] != "user" {
			t.Fatal("input changed native reviewer", params)
		}
	}
	for _, target := range [][3]string{{"linux", "local", "codex"}, {"windows", "orca_supervised", "codex"}, {"windows", "orca", "claude"}} {
		if _, err := NativePermissions(target[0], target[1], target[2]); err == nil {
			t.Fatal("misapplied native settings", target)
		}
	}
}
