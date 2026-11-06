package orcatransport

import (
	"crypto/ed25519"
	"crypto/rand"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixedControlFixture struct {
	setup codex.RemoteControlSetup
	ready codex.RemoteReadiness
}

func fixtureFixedControl(t *testing.T) (*frozenRecord, *fakeHost, b.LifecycleSnapshot, *fixedControlFixture) {
	t.Helper()
	l, f, s := frozenFixture(t)
	f.nativePath = filepath.Join(t.TempDir(), "local-owned-workspace")
	s = frozenWorkspaceFixture(t, l, f, s)
	l.profile.RuntimeExecutable, l.profile.RuntimeSHA256, l.profile.RuntimeConfigurationDigest = "C:/runtime/helper.exe", strings.Repeat("b", 64), strings.Repeat("a", 64)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c := &fixedControlFixture{ready: codex.RemoteReadiness{ExitPublicKey: public, Owner: b.ProcessIdentity{Kind: "os", PID: 124, Created: "owner-creation", Executable: "helper"}, NativeTransport: codex.RemoteNativeTransport, Verified: true, Contract: codex.PolicyContract, Workspace: s.Workspace.Path, Repository: "https://github.com/erranliu/SteerlineTest.git", SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", RuntimeSHA256: l.profile.RuntimeSHA256, ConfigurationDigest: l.profile.RuntimeConfigurationDigest, Process: b.ProcessIdentity{Kind: "os", PID: 123, Created: "native-process", Executable: "codex"}, CheckedAt: time.Now().UTC()}}
	l.setup.RepositoryURL = c.ready.Repository
	l.probeProcess = func(*b.ProcessIdentity) b.ProcessProbeFacts { return b.ProcessProbeFacts{State: "absent"} }
	return l, f, s, c
}
func prepareFixedControl(t *testing.T, l *frozenRecord, s b.LifecycleSnapshot, c *fixedControlFixture) b.LifecycleSnapshot {
	t.Helper()
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	p := state.Process
	p.Kind = "orca_native_control"
	p.Terminal = "control"
	p.Incarnation = "control-inc"
	c.setup = l.controlSetup(s, state)
	state.Control = &ControlTerminal{Process: p, Ready: &c.setup}
	state.RemoteReadiness = &c.ready
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
