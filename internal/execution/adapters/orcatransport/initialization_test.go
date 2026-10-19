package orcatransport

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestInitializationOwnershipCommittedBeforeReadiness(t *testing.T) {
	l, f, s := fixtureOn(t, "paired-remote")
	l.setup.AgentLaunchCommand = "opaque-start"
	f.commandMode = true
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	l.setup.RepositoryURL = "https://github.com/erranliu/SteerlineTest.git"
	l.profile.RuntimeExecutable = "C:/runtime/helper.exe"
	l.profile.RuntimeSHA256 = strings.Repeat("a", 64)
	l.profile.RuntimeConfigurationDigest = strings.Repeat("b", 64)
	owner := codex.RemoteOwnership{ExitPublicKey: make([]byte, 32), Owner: b.ProcessIdentity{Kind: "os", PID: 2, Created: "original", Executable: l.profile.RuntimeExecutable}, RuntimeSHA256: l.profile.RuntimeSHA256, ConfigurationDigest: l.profile.RuntimeConfigurationDigest, Workspace: s.Workspace.Path, Repository: l.setup.RepositoryURL, CreatedAt: time.Now().UTC()}
	raw, _ := json.Marshal(owner)
	f.lines = []string{codex.RemoteOwnerPrefix + base64.RawURLEncoding.EncodeToString(raw)}
	input := b.LifecycleInput{Resource: &b.ResourceIntent{Purpose: "agent-start", Name: "original-terminal", Objective: l.setup.AgentLaunchCommand}}
	r, err = l.launchAgent(context.Background(), request(s, b.LifecycleCreateResource, input), false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeReceiptState(l, s, r)
	if err != nil || state.RemoteOwnership == nil || state.RemoteReadiness != nil || state.LaunchConfirmed || f.nativeSends != 0 {
		t.Fatal("early ownership was not saved without business readiness", state, err)
	}
	bind(&s, r)
	r, err = l.launchAgent(context.Background(), request(s, b.LifecycleCreateResource, input), true)
	if err != nil || r.Disposition != b.OperationAccepted || f.nativeSends != 0 {
		t.Fatal("missing readiness recreated or admitted original session", r, err)
	}
	// A stop can overtake the first ownership checkpoint. Capture from the
	// original stream into durable stop progress before any native effect.
	state.RemoteOwnership = nil
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.calls)
	q := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	captured, e := l.StopLifecycle(context.Background(), q, nil)
	var progress stopProgress
	if e != nil || json.Unmarshal(captured.Operation.Receipt, &progress) != nil || progress.Ownership == nil || progress.Phase != "not-attempted" || captured.State.Stopped {
		t.Fatal("initialization stop lost uncommitted original ownership", captured, e)
	}
	for _, call := range f.calls[before:] {
		if len(call) > 1 && (call[1] == "close" || call[1] == "send") {
			t.Fatal("stop effect preceded ownership checkpoint", call)
		}
	}
	for _, mode := range []string{"key", "owner", "config", "runtime", "workspace", "repository"} {
		changed := owner
		switch mode {
		case "key":
			changed.ExitPublicKey = nil
		case "owner":
			changed.Owner.Created = ""
		case "config":
			changed.ConfigurationDigest = "other"
		case "runtime":
			changed.RuntimeSHA256 = "other"
		case "workspace":
			changed.Workspace = "other"
		case "repository":
			changed.Repository = "other"
		}
		if l.validateRemoteOwnership(changed, s.Workspace.Path) == nil {
			t.Fatal("foreign early ownership accepted", mode)
		}
	}
}

func TestInitializationExitRequiresOriginalOwnershipAndCompleteCleanup(t *testing.T) {
	for _, mode := range []string{"complete", "live-tree", "authentication-open", "foreign-key", "business-without-ready"} {
		t.Run(mode, func(t *testing.T) {
			l, _, s := fixtureOn(t, "paired-remote")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, _ := l.state(s)
			public, private, _ := ed25519.GenerateKey(nil)
			l.profile.RuntimeSHA256 = strings.Repeat("a", 64)
			l.profile.RuntimeConfigurationDigest = strings.Repeat("b", 64)
			l.setup.RepositoryURL = "https://github.com/erranliu/SteerlineTest.git"
			owner := codex.RemoteOwnership{ExitPublicKey: public, Owner: b.ProcessIdentity{Kind: "os", PID: 2, Created: "owner", Executable: "helper.exe"}, RuntimeSHA256: l.profile.RuntimeSHA256, ConfigurationDigest: l.profile.RuntimeConfigurationDigest, Workspace: s.Workspace.Path, Repository: l.setup.RepositoryURL, CreatedAt: time.Now().UTC()}
			state.RemoteOwnership = &owner
			if mode == "business-without-ready" {
				state.LaunchConfirmed = true
			}
			proof := codex.RemoteExit{Ownership: &owner, WritersAbsent: mode != "live-tree", AuthenticationClosed: mode != "authentication-open", ClosedAt: time.Now().UTC()}
			body, _ := json.Marshal(proof)
			signature := ed25519.Sign(private, body)
			if mode == "foreign-key" {
				signature[0] ^= 1
			}
			raw, _ := json.Marshal(map[string]any{"body": json.RawMessage(body), "signature": signature})
			l.readStopArtifact = func(context.Context, string, string, string, string) ([]byte, error) { return raw, nil }
			l.client = native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				if args[0] != "terminal" || args[1] != "list" {
					t.Fatal("verification mutated resources", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]any{"runtimeId": state.Process.RuntimeID}})
			}})
			s.Implementation, err = l.envelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			if err = l.verifyStopped(context.Background(), s); (err == nil) != (mode == "complete") {
				t.Fatal("initialization settlement gate mismatch", mode, err)
			}
		})
	}
}
