package orcatransport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Common identities/host data carry neither a legacy executor nor a decoder.
func fixtureSetup(t *testing.T, environment string) (b.LifecycleSetup, *fakeHost, Profile) {
	t.Helper()
	f := &fakeHost{root: t.TempDir(), incarnation: "original-incarnation"}
	if environment != "" {
		f.nativePath = "/srv/owned-worker"
	}
	p := Profile{Executable: "orca.exe", WaitTimeout: "1s", Environment: environment}
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Target: p.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: strings.Repeat("d", 32), AttemptID: strings.Repeat("b", 32)}, Session: b.SessionContext{ProjectID: "project", ExecutionID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), WorkflowRunID: strings.Repeat("d", 32), CorrelationID: "1", NamePrefix: "sr"}, ConfigurationDigest: "config", ResourceGeneration: "generation", WorkspacePlan: b.WorkspaceIntent{BaseCommit: strings.Repeat("e", 40)}}
	return b.LifecycleSetup{Snapshot: s, ProfileID: "profile", AcceptedConfigVersion: "accepted", ProcessHost: f, RepositoryPath: f.root, RunDirectory: t.TempDir()}, f, p
}

func fixtureProfile(t *testing.T, setup b.LifecycleSetup, key string, version int, value any) b.LifecycleSetup {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	data, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: version, Purpose: b.ImplementationConfigPurpose, ImplementationKey: key, ProjectID: setup.Snapshot.Session.ProjectID, ProfileID: setup.ProfileID, AcceptedConfigVersion: setup.AcceptedConfigVersion}, raw)
	if err != nil {
		t.Fatal(err)
	}
	setup.ProfileData = []b.ImplementationDataEnvelope{data}
	return setup
}

func frozenFixture(t *testing.T) (*frozenRecord, *fakeHost, b.LifecycleSnapshot) {
	return frozenFixtureOn(t, "")
}

func frozenFixtureOn(t *testing.T, environment string) (*frozenRecord, *fakeHost, b.LifecycleSnapshot) {
	t.Helper()
	setup, host, profile := fixtureSetup(t, environment)
	setup.Snapshot.Ref.Protocol = b.ProtocolOrcaTerminalV3
	version := ProfileVersion
	if environment != "" {
		version = NativeRemoteProfileVersion
	}
	setup = fixtureProfile(t, setup, ProfileKey, version, profile)
	r, err := newFrozenReader(setup)
	if err != nil {
		t.Fatal(err)
	}
	return r, host, setup.Snapshot
}

// Construct the real current adapter before injecting an isolated CLI seam.
// Paired host validation itself is covered by the dedicated production tests;
// ordinary workspace tests must not resolve the developer's real pairings.
func structuredFixtureOn(t *testing.T, environment string) (*Lifecycle, *fakeHost, b.LifecycleSnapshot) {
	t.Helper()
	setup, host, transport := fixtureSetup(t, environment)
	setup.Snapshot.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
	profile := AgentSessionProfile{Transport: transport, AccessMode: "full_access", Model: "requested-model", ReasoningEffort: "medium"}
	if environment != "" {
		profile.PairingHostID = "sha256:" + strings.Repeat("ab", 32)
	}
	setup = fixtureProfile(t, setup, AgentSessionProfileKey, AgentSessionVersion, profile)
	port, err := NewLifecycle(setup)
	if err != nil {
		t.Fatal(err)
	}
	l := port.(*Lifecycle)
	// Preserve the original unit seam for preparation/bootstrap as well as CLI.
	// Dedicated IPC/queue tests still exercise nativeAccess on production ports.
	l.nativeAccess = false
	l.client = native.New(native.Config{CommandTimeout: "40s", Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if err := native.CheckpointDispatch(ctx); err != nil {
			return nil, err
		}
		return host.OutputCommand(ctx, cliLaunchCommand(transport, transport.Executable, setup.RepositoryPath, timeout, args...))
	}})
	if len(host.calls) != 0 {
		t.Fatal("fixture construction dispatched external I/O")
	}
	return l, host, setup.Snapshot
}

func TestCurrentFixtureUsesIsolatedHostWithInvalidPairingStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), []byte("invalid fixture pairing store"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, environment := range []string{"", "paired-worker"} {
		t.Run(environment, func(t *testing.T) {
			l, host, _ := structuredFixtureOn(t, environment)
			if l.nativeAccess {
				t.Fatal("unit fixture enabled native bootstrap/IPC")
			}
			if _, err := l.client.MetaCallWithTimeout(context.Background(), time.Second, "status", "--json"); err != nil {
				t.Fatal("unit fixture accessed real pairing/transport", err)
			}
			want := "status --json"
			if environment != "" {
				want += " --environment " + environment
			}
			if len(host.calls) != 1 || strings.Join(host.calls[0], " ") != want {
				t.Fatal("unit call escaped the isolated host", host.calls)
			}
		})
	}
}
