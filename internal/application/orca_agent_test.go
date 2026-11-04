package application

import (
	"bytes"
	"context"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"io"
	"log"
	"os/exec"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestSRFreezeOrcaAgent(t *testing.T) {
	for _, agent := range []string{"", "codex", "pi"} {
		t.Run("agent="+agent, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			executable, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			settings := contracts.OrcaSettings{AccessMode: "full_access", Agent: agent}
			settings.Transport.Executable = executable
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			r := f.freeze()
			want := settings.AgentName()
			if r.Snapshot.Ref.Agent.Name != want || r.Snapshot.Session.Agent.Name != want || r.View().Execution.Agent != want {
				t.Fatal("frozen agent or public projection was lost")
			}
			if r.LaunchCommand != "" {
				t.Fatal("default launch should not inject a terminal command", r.LaunchCommand)
			}
			if want == "codex" && (r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 || r.StopEvidenceScope != orca.AgentSessionStopScope || r.Profile[0].CodecVersion != orca.AgentSessionCloseProfileVersion || resultContract(r) != contracts.ResultFileContract) {
				t.Fatal("default Codex development did not freeze the structured owner and contracts")
			}
			if r.InputDigest != reliable.Digest(canonical(r.Request)) {
				t.Fatal("frozen request digest is inconsistent")
			}
			if agent == "" {
				settings.Transport.WaitTimeout = "30s"
				if !bytes.Equal(r.Request.Execution.Settings, canonical(settings)) {
					t.Fatal("default settings gained an explicit launch override", string(r.Request.Execution.Settings))
				}
			}
			if want == "codex" {
				for _, override := range []contracts.AgentSettings{{Executable: "codex"}, {Args: []string{"--no-alt-screen"}}} {
					settings.AgentLaunch = override
					f.request.Execution.Settings = canonical(settings)
					f.input.Input = canonical(f.request)
					explicit := f.freeze()
					if explicit.Snapshot.Ref.Protocol != b.ProtocolOrcaTerminalV3 || !strings.Contains(explicit.LaunchCommand, "SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$null)") {
						t.Fatal("explicit launch override did not keep the original terminal path", override, explicit.LaunchCommand)
					}
				}
			}
			if want != "codex" {
				assertOrcaAgentBlocked(t, f.state, r)
			}
		})
	}
	t.Run("remote-native-and-retired-profile-rejected", func(t *testing.T) {
		f := newBaseRefreshFixture(t, "origin", "main")
		exe, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		settings := contracts.OrcaSettings{AccessMode: "full_access"}
		settings.Transport.Executable = exe
		settings.AgentLaunch.Executable = "codex"
		settings.AgentLaunch.Model = "gpt-6-luna"
		settings.AgentLaunch.ReasoningEffort = "high"
		f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "original-remote"}, Settings: canonical(settings)}
		f.input.Input = canonical(f.request)
		r := f.freeze()
		if len(r.Profile) != 1 || r.Profile[0].CodecVersion != orca.NativeProfileVersion || strings.Contains(r.LaunchCommand, "steerrunner-agent") || !strings.Contains(r.LaunchCommand, `model_reasoning_effort="high"`) {
			t.Fatal("remote profile is not native", r.LaunchCommand, r.Profile)
		}
		profile, err := orca.DecodeProfile(r.Profile[0].Payload)
		if err != nil || !profile.NativeTerminal || profile.RuntimeExecutable != "" || profile.RuntimeSHA256 != "" || profile.Terminal != nil {
			t.Fatal(profile, err)
		}
		// V7 has no synthetic read-only projection for a retired remote profile.
		r.Profile[0].CodecVersion = orca.ProfileVersion
		if _, err := orca.NewLifecycle(b.LifecycleSetup{Snapshot: r.Snapshot, ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}); err == nil {
			t.Fatal("retired remote profile admitted")
		}

	})
}

func assertOrcaAgentBlocked(t *testing.T, dir string, record *runstate.Record) {
	t.Helper()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	w := &worker{wake: make(chan struct{}, 1)}
	e := &Engine{ctx: context.Background(), dir: dir, store: store, logger: log.New(io.Discard, "", 0), runs: map[string]*runstate.Record{}, workers: map[string]*worker{record.ID: w}}
	if err := e.commitLocked(record.Clone(), nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	assertBlocked := func() {
		t.Helper()
		e.advance(record.ID, w)
		current := e.runs[record.ID]
		if current.Status != "blocked" || current.Phase != "preparation_blocked" || !strings.Contains(current.Diagnostic, "terminal_agent_evidence_unsupported") || len(current.Intents) != 0 || current.Snapshot.Ref.External.Terminal != "" || current.Snapshot.PendingTurn != nil || current.Allowed {
			t.Fatal("real scheduling path bypassed zero-input admission", current.View())
		}
	}
	assertBlocked()
	// An explicit resume may restore authorization, but cannot supply missing
	// agent facts or bypass preparation on the next scheduler pass.
	next := e.runs[record.ID].Clone()
	next.PendingResume = "resume-agent"
	next.CommandStates[next.PendingResume] = "pending"
	if err := e.commitLocked(next, nil); err != nil {
		t.Fatal(err)
	}
	e.advance(record.ID, w)
	assertBlocked()
	next = e.runs[record.ID].Clone()
	next.PendingStop = "stop-agent"
	next.CommandStates[next.PendingStop] = "pending"
	if err := e.commitLocked(next, nil); err != nil {
		t.Fatal(err)
	}
	e.advance(record.ID, w)
	if current := e.runs[record.ID]; current.StopStatus != "stopped" || current.Allowed || len(current.Intents) != 0 {
		t.Fatal("uncreated execution could not stop without native effects", current.View())
	}
}
