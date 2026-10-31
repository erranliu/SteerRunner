package codexworker

import (
	"context"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestRetiredOwnerPortRejectsBusinessWorkWithoutLaunchingOwner(t *testing.T) {
	launched := false
	port := NewOriginalOwnerStopLifecycle(b.LifecycleSetup{LaunchExecutionOwner: func(context.Context, string) error {
		launched = true
		return nil
	}})
	for _, action := range []b.LifecycleAction{b.LifecyclePrepareWorkspace, b.LifecyclePrepareSession, b.LifecycleStartTurn, b.LifecycleDeliverFeedback} {
		request := b.LifecycleRequest{}
		request.Operation.Action = string(action)
		receipt, observer, err := port.Execute(context.Background(), request)
		if err == nil || observer != nil || receipt.Disposition != b.OperationRejected || launched {
			t.Fatal("retired owner port allowed business work", action, receipt, err)
		}
		if _, _, err := port.Reconcile(context.Background(), request); err == nil || launched {
			t.Fatal("retired owner port reconciled business work", action, err)
		}
	}
}

func TestExplicitPermissionsPreserveReadOnlyAndFrozenFullAccess(t *testing.T) {
	full, err := NativePermissions("windows", "local", "codex", "full_access")
	if err != nil {
		t.Fatal(err)
	}
	read, err := NativePermissions("windows", "local", "codex", "read_only")
	if err != nil || read.Contract != ReadOnlyPolicyContract || read.WindowsSandbox != "mxc" {
		t.Fatal(read, err)
	}
	for _, mode := range []string{"", "sandbox", "workspace-write"} {
		if _, err := NativePermissions("windows", "local", "codex", mode); err == nil {
			t.Fatal("retired mode accepted", mode)
		}
	}
	if _, err := NativePermissions("windows", "local", "codex"); err == nil {
		t.Fatal("implicit mode accepted")
	}
	if joined := strings.Join(full.Args(), " "); strings.Contains(joined, "sandbox_workspace_write") || strings.Contains(joined, "windows.sandbox") || full.Contract != PolicyContract {
		t.Fatal(full)
	}
	id := PolicyResourceIdentity{ProjectID: "p", ExecutionID: "e", AttemptID: "a", Generation: "g", ConfigDigest: "d"}
	legacy := append(full.Args(), legacyWorkspaceArgs()...)
	for _, tc := range []struct {
		contract string
		args     []string
		valid    bool
	}{
		{PolicyContract, full.Args(), true}, {ReadOnlyPolicyContract, legacy, false},
		{ReadOnlyPolicyContract, read.Args(), true}, {PolicyContract, read.Args(), false},
		{PolicyContract, legacy, false}, {ReadOnlyPolicyContract, full.Args(), false},
		{ReadOnlyPolicyContract, append(legacy, "--config", "sandbox_mode=workspace-write"), false},
	} {
		envelope, err := NewPolicyResourceEnvelope(id, PolicyResourcePayload{NativeVersion: PolicyCodexVersion, Contract: tc.contract, Settings: b.LaunchCommand{Executable: "original.exe", Args: tc.args}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = DecodePolicyResourceEnvelope(envelope, id)
		if (err == nil) != tc.valid {
			t.Fatal(tc.contract, tc.args, err)
		}
	}
}
