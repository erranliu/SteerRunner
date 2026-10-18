package application

import (
	"bytes"
	"context"
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
