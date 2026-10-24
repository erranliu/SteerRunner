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
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestSRFreezeOrcaAgent(t *testing.T) {
	for _, agent := range []string{"", "codex", "pi", "dsh"} {
		t.Run("agent="+agent, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			executable, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			settings := contracts.OrcaSettings{Agent: agent}
			settings.Transport.Executable = executable
			f.request.Execution = contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: canonical(settings)}
			f.input.Input = canonical(f.request)
			r := f.freeze()
			want := settings.AgentName()
			if r.Snapshot.Ref.Agent.Name != want || r.Snapshot.Session.Agent.Name != want || r.View().Execution.Agent != want {
				t.Fatal("frozen agent or public projection was lost")
			}
			if (want == "codex") != (r.LaunchCommand != "") {
				t.Fatal("Codex launch defaults leaked into another agent", r.LaunchCommand)
			}
			if want == "codex" && !strings.Contains(r.LaunchCommand, "SetEnvironmentVariable('STEERLINE_PROGRESS_FILE',$null)") {
				t.Fatal("disabled progress channel inherited another Run's environment")
			}
			if r.InputDigest != reliable.Digest(canonical(r.Request)) {
				t.Fatal("frozen request digest is inconsistent")
			}
			if agent == "" {
				settings.Transport.WaitTimeout = "30s"
				settings.AgentLaunch.Executable = "codex"
				if !bytes.Equal(r.Request.Execution.Settings, canonical(settings)) {
					t.Fatal("legacy frozen settings bytes changed", string(r.Request.Execution.Settings))
				}
			}
			if want != "codex" {
				assertOrcaAgentBlocked(t, f.state, r)
			}
		})
	}
	t.Run("remote-native-and-legacy-readonly", func(t *testing.T) {
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
		if len(r.Profile) != 1 || r.Profile[0].CodecVersion != orca.NativeRemoteProfileVersion || strings.Contains(r.LaunchCommand, "steerrunner-agent") || !strings.Contains(r.LaunchCommand, `model_reasoning_effort="high"`) {
			t.Fatal("remote profile is not native", r.LaunchCommand, r.Profile)
		}
		profile, err := orca.DecodeProfile(r.Profile[0].Payload)
		if err != nil || profile.RuntimeExecutable != "" || profile.RuntimeSHA256 != "" || profile.Terminal != nil {
			t.Fatal(profile, err)
		}
		// An old remote profile without readiness is still legacy, never inferred native.
		envelope := r.Profile[0]
		envelope.CodecVersion = orca.ProfileVersion
		envelope, err = b.NewImplementationDataEnvelope(envelope, envelope.Payload)
		if err != nil {
			t.Fatal(err)
		}
		r.Profile[0] = envelope
		store, err := storage.Open(f.state)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		r.Revision = 1
		if err = store.Commit(r, 0, nil, "legacy-fixture"); err != nil {
			t.Fatal(err)
		}
		before, _ := store.Runs()
		saved := canonical(before)
		e, err := New(context.Background(), f.state, store, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if e.advance(r.ID, e.workers[r.ID]) || e.workers[r.ID].port != nil {
			t.Fatal("legacy Run entered adapter execution")
		}
		resume := contracts.Request{Protocol: contracts.Protocol, RequestID: "legacy-resume", Input: canonical(contracts.RevisionInput{ExpectedRevision: r.Revision})}
		response := e.Handle("resume", r.ID, "", "", "", 0, false, &resume)
		if response.Error == nil || response.Error.Code != "unsupported_capability" {
			t.Fatal(response)
		}
		e.Start()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err = e.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		after, _ := store.Runs()
		if !bytes.Equal(saved, canonical(after)) {
			t.Fatal("service load/shutdown rewrote legacy evidence")
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
