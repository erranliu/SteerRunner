//go:build windows && live

package native

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
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
		if err != nil || state.RemoteOwnership == nil {
			continue
		}
		var witness struct{ Original json.RawMessage }
		raw, e := os.ReadFile(filepath.Join(s.m.Root, "evidence", "readiness-withheld.json"))
		if e != nil || json.Unmarshal(raw, &witness) != nil {
			continue
		}
		var response struct {
			Result struct{ Terminal struct{ Tail []string } }
		}
		if json.Unmarshal(witness.Original, &response) != nil {
			t.Fatal("original native readiness response malformed")
		}
		for _, line := range response.Result.Terminal.Tail {
			if strings.HasPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix) {
				raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(line), codex.RemoteReadyPrefix))
				if e != nil || json.Unmarshal(raw, &window.NativeReadiness) != nil {
					t.Fatal("original native readiness malformed")
				}
			}
		}
		if window.NativeReadiness.Process.PID > 0 {
			break
		}
	}
	state, err := livetest.InitializationResources(window.Snapshot)
	s.check("initialization-without-readiness", err == nil && state.RemoteOwnership != nil && state.RemoteReadiness == nil && !state.LaunchConfirmed && len(state.Turns) == 0 && window.NativeReadiness.Process.PID > 0, "initialization-window.json: real native started; original Runner has early ownership but no business readiness")
	save(t, "initialization-window.json", window)
	s.r.FaultsHit++
	s.check("fault-hit", true, "readiness-withheld.json: real readiness exists, only caller response hides it")
	v := s.view()
	s.bind(v)
	s.process.Kind = "orca_terminal"
	s.process.ExecutionHost = s.m.Execution.Target.Environment
	keys, creates := s.calls()
	s.check("zero-business-input", len(keys) == 1 && creates == 1 && !v.Execution.TurnStarted && v.PR == nil, "native-calls.jsonl: original helper launch only; zero business sends")
	if s.m.CaseID == "restart-before-readiness" {
		before := s.instance
		s.restart(false)
		// Allow several normal polling rounds; a replacement must not initialize
		// or send anything until explicitly authorized. Stop requires no resume.
		for i := 0; i < 3; i++ {
			s.noMoreInputs(1)
			time.Sleep(time.Second)
		}
		after := s.initializationSnapshot()
		v = s.view()
		save(t, "initialization-restart.json", map[string]any{"before": before, "after": s.instance, "snapshot": after, "view": v})
		s.check("initialization-restart-zero-input", before.InstanceID != s.instance.InstanceID && v.ResumeRequired && !v.NewInputsAllowed && after.Ref.External == window.Snapshot.Ref.External, "initialization-restart.json and native-calls.jsonl: replacement service retains original resources and zero input")
	}
	if err = revokeOriginalRun(s.m, s.dir, s.r.RunID, "stop-original-initialization"); err != nil {
		t.Fatal(err)
	}
	final := waitStopped(t, s.m, s.dir, s.r.RunID)
	s.r.Cleanup.NativeStopped = true
	s.originalIdentity(final)
	s.noMoreInputs(1)
	s.check("no-duplicate-inputs", true, "native-calls.jsonl: one original workspace/helper launch, zero business input across stop/restart")
	path, _ := codex.RemoteExitPath(codex.RemoteReadiness{ExitPublicKey: state.RemoteOwnership.ExitPublicKey})
	exit, err := native.ReadStopArtifact(context.Background(), s.m.Execution.Target.Environment, s.process.RuntimeID, s.process.WorktreeID, path)
	if err != nil {
		t.Fatal("original signed initialization exit unavailable", err)
	}
	if err = codex.VerifyRemoteOwnershipExit(exit, *state.RemoteOwnership); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(*evidenceFlag, "initialization-exit.json"), exit, 0600); err != nil {
		t.Fatal(err)
	}
	s.r.AuditComplete = s.completeNativeReceipts(keys)
	s.check("native-receipts-complete", s.r.AuditComplete, "native-resource-audit.json: original business terminal and all owned resources accounted")
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
	s.check("offline-settlement", verified.ResourcesSettled && len(verified.Runs) == 1 && verified.Runs[0].RunID == s.r.RunID && verified.Runs[0].ResourcesSettled, "offline-settlement.json: public verifier confirms original native resources")
	save(t, "pr-list.json", []any{})
	s.stage("cleanup")
}
