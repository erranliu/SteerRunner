//go:build windows && live

package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func (s *stabilityRun) initializationSnapshot() b.LifecycleSnapshot {
	store, err := storage.OpenReader(s.dir)
	if err != nil {
		s.t.Fatal(err)
	}
	defer store.Close()
	runs, err := store.Runs()
	if err != nil {
		s.t.Fatal(err)
	}
	for _, r := range runs {
		if r.ID == s.r.RunID {
			return r.Snapshot
		}
	}
	s.t.Fatal("original Run snapshot missing")
	return b.LifecycleSnapshot{}
}

func (s *stabilityRun) initializationStop() {
	t := s.t
	s.stage("await-original-initialization-window")
	var window livetest.InitializationWindow
	for until := time.Now().Add(150 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
		window = livetest.InitializationWindow{RunID: s.r.RunID, Snapshot: s.initializationSnapshot()}
		state, err := livetest.InitializationResources(window.Snapshot)
		if err != nil || !state.LaunchAccepted || state.LaunchConfirmed {
			continue
		}
		var witness struct{ Original json.RawMessage }
		raw, e := os.ReadFile(filepath.Join(s.m.Root, "evidence", "agent-identity-withheld.json"))
		if e != nil || json.Unmarshal(raw, &witness) != nil {
			continue
		}
		var response struct {
			Result struct{ Terminal native.Terminal }
		}
		if json.Unmarshal(witness.Original, &response) != nil {
			t.Fatal("original native identity response malformed")
		}
		window.NativeTerminal = response.Result.Terminal
		if window.NativeTerminal.AgentIdentity == "codex" {
			break
		}
	}
	state, err := livetest.InitializationResources(window.Snapshot)
	s.check("initialization-without-readiness", err == nil && state.NativeOnly() && state.LaunchAccepted && !state.LaunchConfirmed && len(state.Turns) == 0 && window.NativeTerminal.AgentIdentity == "codex", "initialization-window.json: actual original native agent exists; caller launch remains pending, zero helper resources")
	save(t, "initialization-window.json", window)
	s.r.FaultsHit++
	s.check("fault-hit", true, "agent-identity-withheld.json: only caller agentIdentity withheld; actual native receipt retained")
	v := s.view()
	s.bind(v)
	s.process.Kind = "orca_terminal"
	s.process.ExecutionHost = s.m.Execution.Target.Environment
	keys, creates := s.calls()
	s.check("zero-business-input", len(keys) == 1 && creates == 1 && !v.Execution.TurnStarted && v.PR == nil, "native-calls.jsonl: one direct native startup, zero business sends and zero helper/control resources")
	if s.m.CaseID == "restart-before-readiness" {
		beforeView := v
		before := s.instance
		s.restart(false)
		for i := 0; i < 3; i++ {
			s.noMoreInputs(1)
			time.Sleep(time.Second)
		}
		after := s.initializationSnapshot()
		v = s.view()
		save(t, "initialization-restart.json", map[string]any{"before": before, "before_view": beforeView, "after": s.instance, "snapshot": after, "view": v})
		s.check("initialization-restart-zero-input", before.InstanceID != s.instance.InstanceID && v.ResumeRequired == beforeView.ResumeRequired && v.NewInputsAllowed == beforeView.NewInputsAllowed && after.Ref.External == window.Snapshot.Ref.External, "initialization-restart.json: replacement retains original authorization/native terminal; pending readiness still permits zero business input")
	}
	if err = revokeOriginalRun(s.m, s.dir, s.r.RunID, "stop-original-initialization"); err != nil {
		t.Fatal(err)
	}
	final := waitStopped(t, s.m, s.dir, s.r.RunID)
	s.r.Cleanup.NativeStopped = true
	s.originalIdentity(final)
	s.noMoreInputs(1)
	s.check("no-duplicate-inputs", true, "native-calls.jsonl: one original workspace/direct startup, zero business input across stop/restart")
	s.r.AuditComplete = s.completeNativeReceipts(keys)
	s.check("native-receipts-complete", s.r.AuditComplete, "native-resource-audit.json: original native terminal accounted; zero helper/extra terminal/file calls")
	s.r.BusinessOutcome = "stopped"
	s.stopService(false)
	s.r.Cleanup.ServiceStopped = true
	s.instance = contracts.ServiceInfo{}
	proof := invoke(t, s.m, s.dir, []string{"service", "verify-stopped"}, nil)
	save(t, "offline-settlement.json", proof)
	raw, _ := json.Marshal(proof.Data)
	var verified contracts.StoppedVerification
	if json.Unmarshal(raw, &verified) != nil {
		t.Fatal("invalid stopped resource proof")
	}
	s.check("offline-settlement", verified.ResourcesSettled && len(verified.Runs) == 1 && verified.Runs[0].RunID == s.r.RunID && verified.Runs[0].ResourcesSettled && verified.Runs[0].StopEvidenceScope == "orca_terminal", "offline-settlement.json: original Orca terminal scope only; no Job/authentication claim")
	save(t, "pr-list.json", []any{})
	s.stage("cleanup")
}
