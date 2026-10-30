//go:build windows && live

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Only the acquisition model/process is real here. The business agent and PR
// provider remain controlled fixtures, so this diagnostic creates no real PR.
func TestSRNativeParserGenerationRecovery(t *testing.T) {
	if *contentNativeExe == "" {
		t.Skip("explicit native opt-in required")
	}
	testCount.Add(1)
	if !filepath.IsAbs(*contentNativeExe) || *evidenceFlag == "" {
		t.Fatal("explicit executable/evidence identity required")
	}
	home := codex.EnvironmentHome()
	for _, mode := range []string{"graceful", "kill", "stop"} {
		t.Run(mode, func(t *testing.T) {
			dir, envelope := auditedRun(t)
			t.Setenv("STEERRUNNER_AUDIT_PLAIN", "1")
			t.Setenv("STEERRUNNER_SUCCESS_OUTPUT", "1")
			t.Setenv("STEERRUNNER_RECOVERY_PARSER_HOME", home)
			config := ra.DefaultConfig()
			config.CodexParser.Enabled = true
			config.CodexParser.Executable = *contentNativeExe
			config.CodexParser.Model = *contentNativeModel
			config.CodexParser.ReasoningEffort = "low"
			config.CodexParser.CodexHomeEnv = "STEERRUNNER_RECOVERY_PARSER_HOME"
			if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), marshalEvidence(t, config), 0600); err != nil {
				t.Fatal(err)
			}
			service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, service)
			raw, _ := json.Marshal(envelope)
			id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
			waitRunFor(t, dir, id, func(v contracts.RunView) bool {
				if v.Step != workflow.StepAuditParse {
					return false
				}
				for _, op := range v.Operations {
					if op.Action == string(b.LifecycleParseResult) {
						return true
					}
				}
				return false
			}, 2*time.Minute)
			reader, err := storage.OpenReader(dir)
			if err != nil {
				t.Fatal(err)
			}
			runs, err := reader.Runs()
			reader.Close()
			if err != nil || len(runs) != 1 {
				t.Fatal("original parser Run unavailable", err)
			}
			var request b.LifecycleRequest
			for _, intent := range runs[0].Intents {
				if intent.Request.Input.Parse != nil {
					request = intent.Request.Clone()
				}
			}
			if request.Input.Parse == nil {
				t.Fatal("original parser intent unavailable")
			}
			started, err := parser.ReadStartedReceipt(dir, request)
			until := request.Input.Parse.Deadline
			for err != nil && time.Now().Before(until) {
				time.Sleep(50 * time.Millisecond)
				started, err = parser.ReadStartedReceipt(dir, request)
			}
			if err != nil || started.Process == nil || process.InspectProcess(started.Process) != "matching process still exists" {
				final, finalErr := parser.ReadReceipt(dir, request)
				r := runs[0]
				port, openErr := codex.NewExecutionLifecycle(b.LifecycleSetup{Snapshot: r.Snapshot, ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest, RunDirectory: dir, RepositoryPath: r.RepositoryPath, RepositoryURL: r.Request.Repository.URL, ProcessHost: process.New(time.Minute), LaunchExecutionOwner: func(context.Context, string) error { return nil }})
				var queryErr error
				var queried b.LifecycleReceipt
				if openErr == nil {
					queryCtx, cancelQuery := context.WithTimeout(context.Background(), 5*time.Second)
					queried, _, queryErr = port.Reconcile(queryCtx, request)
					cancelQuery()
				}
				t.Fatal("native parser did not reach live process window", err, "read-only original owner query", openErr, queryErr, queried.ParseEvidence, "final", finalErr, final.ParseEvidence)
			}
			var receipt b.LifecycleReceipt
			if mode == "stop" {
				auditControl(t, dir, id, "stop", map[string]any{})
				waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				receipt, err = parser.ReadReceipt(dir, request)
				if err != nil || !receipt.State.Stopped || !receipt.State.WritersAbsent {
					t.Fatal("explicit parser stop lacks original terminal receipt", err)
				}
			} else {
				if mode == "kill" {
					if err := killOwnedService(service); err != nil {
						t.Fatal(err)
					}
				} else {
					call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("parser-detach", service.InstanceID), 0)
					if err := waitOwnedService(service); err != nil {
						t.Fatal(err)
					}
				}
				if process.InspectProcess(started.Process) != "matching process still exists" {
					t.Fatal("service exit killed original native parser")
				}
				deadline := time.Now().Add(2 * time.Minute)
				for {
					receipt, err = parser.ReadReceipt(dir, request)
					if err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("offline original native parser did not settle", err)
					}
					time.Sleep(50 * time.Millisecond)
				}
				if receipt.ParseEvidence == nil || receipt.ParseEvidence.Status != "completed" || !receipt.State.Stopped || !receipt.State.WritersAbsent {
					t.Fatal("offline native parser failed", receipt)
				}
				next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
				cleanupService(t, dir, next)
				ready := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" || v.Status == "blocked" })
				if ready.Status != "pr_ready" || ready.ResumeRequired {
					t.Fatal("original native parser completion not adopted", ready)
				}
			}
			after, err := parser.ReadStartedReceipt(dir, request)
			wantTurns := 3
			if mode == "stop" {
				wantTurns = 2
			}
			if err != nil || !reflect.DeepEqual(started, after) || countNative(t, "turn/start") != wantTurns {
				t.Fatal("native parser or business input was replaced", err)
			}
			if mode == "stop" && process.InspectProcess(started.Process) != "process absent" {
				t.Fatal("explicit stop retained native parser")
			}
			if err := os.WriteFile(filepath.Join(*evidenceFlag, "native-parser-recovery-"+mode+".json"), marshalEvidence(t, map[string]any{"mode": mode, "started": started, "receipt": receipt, "original_deadline": request.Input.Parse.Deadline}), 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
