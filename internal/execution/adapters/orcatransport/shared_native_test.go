package orcatransport

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func sharedNativeFixture(t *testing.T, environment string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot) {
	t.Helper()
	legacy, host, snapshot := fixtureOn(t, environment)
	setup := legacy.setup.Clone()
	profile := legacy.profile
	profile.NativeTerminal = true
	raw, _ := json.Marshal(profile)
	prior := setup.ProfileData[0]
	prior.CodecVersion = NativeProfileVersion
	var err error
	setup.ProfileData[0], err = b.NewImplementationDataEnvelope(prior, raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := NewLifecycleWithStateFactory(setup, host.stateFactory)
	if err != nil {
		t.Fatal(err)
	}
	return port.(*Lifecycle), host, snapshot
}

func TestSharedNativeProfileAndScope(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, _, snapshot := sharedNativeFixture(t, environment)
			if l.stateVersion() != NativeStateVersion || l.StopEvidenceScope() != "orca_terminal" || l.usesControl() || l.PolicyCapabilities().ReportToolPolicy != "configured_native_launch_parameters; execution_host_existing_authentication; business_read_only_is_prompt_policy" {
				t.Fatal("candidate contract differs by location")
			}
			production, err := NewLifecycle(l.setup)
			if err != nil || !production.(*Lifecycle).nativeAccess {
				t.Fatal("production candidate constructor unavailable", err)
			}
			old := l.profile
			old.NativeTerminal = false
			if !reflect.DeepEqual(old.Target(), l.profile.Target()) {
				t.Fatal("native contract changed target routing hash")
			}
			receipt, _, err := l.Execute(context.Background(), request(snapshot, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &snapshot.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&snapshot, receipt)
			if len(snapshot.Implementation) != 1 || snapshot.Implementation[0].CodecVersion != NativeStateVersion {
				t.Fatal("candidate state codec not persisted", snapshot.Implementation)
			}
			for _, version := range []int{LocalStateVersion, StateVersion} {
				changed := snapshot.Clone()
				changed.Implementation[0].CodecVersion = version
				if _, err := l.state(changed); err == nil {
					t.Fatal("candidate accepted old state contract", version)
				}
			}
		})
	}
}

func TestSharedNativeProfileCodecIsolation(t *testing.T) {
	for _, environment := range []string{"", "paired-worker"} {
		for _, marker := range []string{"true", "false", "null"} {
			for _, version := range []int{NativeRemoteProfileVersion, FullAccessTerminalProfileVersion} {
				profile, _ := json.Marshal(map[string]any{"executable": "orca.exe", "wait_timeout": "1s", "environment": environment, "native_terminal": json.RawMessage(marker)})
				if _, err := decodeProfileCodec(profile, version); err == nil {
					t.Fatal("old profile codec acquired new contract", environment, marker, version)
				}
			}
		}
		for _, helper := range []string{"terminal", "runtime_executable", "runtime_sha256", "runtime_configuration_digest"} {
			for _, value := range []any{nil, ""} {
				profile, _ := json.Marshal(map[string]any{"executable": "orca.exe", "wait_timeout": "1s", "environment": environment, "native_terminal": true, helper: value})
				if _, err := decodeProfileCodec(profile, NativeProfileVersion); err == nil {
					t.Fatal("new native profile contains helper field", environment, helper, value)
				}
			}
		}
	}
	for _, marker := range []any{false, nil} {
		profile, _ := json.Marshal(map[string]any{"executable": "orca.exe", "wait_timeout": "1s", "native_terminal": marker})
		if _, err := decodeProfileCodec(profile, NativeProfileVersion); err == nil {
			t.Fatal("new profile omitted native contract", marker)
		}
	}
	for _, payload := range []string{
		`{"executable":"orca.exe","wait_timeout":"1s","NATIVE_TERMINAL":true}`,
		`{"executable":"orca.exe","wait_timeout":"1s","native_terminal":true,"native_terminal":null}`,
		`{"executable":"orca.exe","wait_timeout":"1s","native_terminal":true,"RUNTIME_EXECUTABLE":null}`,
	} {
		for _, version := range []int{NativeProfileVersion, FullAccessTerminalProfileVersion} {
			if _, err := decodeProfileCodec([]byte(payload), version); err == nil {
				t.Fatal("case/duplicate marker or helper bypassed codec isolation", version, payload)
			}
		}
	}
	for _, field := range []string{"control", "remote_readiness", "terminal_owner", "terminal_readiness"} {
		raw, _ := json.Marshal(map[string]any{field: map[string]any{}})
		if _, err := decodeState(raw, NativeStateVersion); err == nil {
			t.Fatal("new state codec accepted helper preparation", field)
		}
	}
}
