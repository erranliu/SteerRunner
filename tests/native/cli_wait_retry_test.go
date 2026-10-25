//go:build windows && live

package native

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

// Reuses the native fixture, forwarding journal, stop and service driver. The
// scheduler takes an exclusive test lease for this driver's shared CLI cache.
// Its intentionally held mutex has a private cache key, never a production key.
func cliWaitRetry(t *testing.T, m livetest.Manifest) {
	sharedCache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SR_CLI_WAIT_SHARED_CACHE", sharedCache)
	t.Setenv("LOCALAPPDATA", filepath.Join(m.Root, "cli-wait-cache"))
	result := struct {
		scenarioResult
		WaitVerified   bool `json:"wait_verified"`
		CancelVerified bool `json:"cancel_verified"`
		NativeAudit    bool `json:"native_audit"`
	}{scenarioResult: scenarioResult{Scenario: m.Scenario, Level: m.Level, Stage: "queue"}}
	dir := filepath.Join(m.Root, "s")
	response := invoke(t, m, dir, []string{"service", "start"}, nil)
	var service contracts.ServiceInfo
	raw, _ := json.Marshal(response.Data)
	_ = json.Unmarshal(raw, &service)
	var ids []string
	t.Cleanup(func() {
		result.RunStopped = len(ids) == 2
		for _, id := range ids {
			if err := revokeOriginalRun(m, dir, id, "wait-cleanup-"+id); err != nil {
				t.Error(err)
				result.RunStopped = false
				continue
			}
			var final contracts.RunView
			for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); time.Sleep(300 * time.Millisecond) {
				v, err := inspect(m, dir, id, false)
				if err == nil {
					final = v
					if v.StopStatus == "stopped" && !v.NewInputsAllowed {
						break
					}
				}
			}
			if final.StopStatus != "stopped" || final.NewInputsAllowed {
				t.Error("native resource stop unverified", id)
				result.RunStopped = false
			}
			result.Runs = append(result.Runs, final)
		}
		req := request("wait-service-stop", map[string]string{"expected_instance_id": service.InstanceID})
		_, err := command(m, dir, []string{"service", "stop", "--input", "-"}, &req)
		if err == nil {
			for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(200 * time.Millisecond) {
				r, e := command(m, dir, []string{"service", "status"}, nil)
				if e == nil && r.Data.(map[string]any)["status"] == "not_running" {
					result.ServiceStopped = true
					break
				}
			}
		}
		if !result.ServiceStopped {
			t.Error("test service exit unverified", err)
		} else {
			proof, err := command(m, dir, []string{"service", "verify-stopped"}, nil)
			save(t, "wait-offline-stop.json", proof)
			var verified contracts.StoppedVerification
			body, _ := json.Marshal(proof.Data)
			_ = json.Unmarshal(body, &verified)
			if err != nil || !verified.ResourcesSettled {
				t.Error("offline native resources unverified", err)
				result.RunStopped = false
			}
		}
		result.NativeAudit = waitNativeAudit(t, m)
		result.Passed = !t.Failed() && result.RunStopped && result.ServiceStopped && result.WaitVerified && result.CancelVerified && result.NativeAudit
		if result.Passed {
			result.Stage = "completed"
		}
		save(t, "scenario.json", result)
	})
	source, err := m.ExecutionRepositoryPath()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 85*time.Second)
		locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		go func() {
			_, err := orca.SerializeCLI(ctx, func(context.Context) ([]byte, error) {
				close(locked)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return nil, nil
			})
			done <- err
		}()
		<-locked
		held := true
		// Release before the outer native-resource cleanup even on assertion failure.
		defer func() {
			if held {
				close(release)
			}
			cancel()
			<-done
		}()
		input := contracts.RunRequest{Title: fmt.Sprintf("CLI wait %s %d", m.ID, i), Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: source, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Path: "sr-smoke/spec.md"}, Plan: contracts.Document{Path: "sr-smoke/plan.md"}}, Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: m.Execution, Publication: m.Publication}
		flow, err := m.RunWorkflow(nil)
		if err != nil {
			t.Fatal(err)
		}
		input.Workflow = flow
		req := request(fmt.Sprintf("wait-submit-%d", i), input)
		accepted := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
		id := accepted.Data.(map[string]any)["run_id"].(string)
		ids = append(ids, id)
		if result.RunID == "" {
			result.RunID = id
		}
		wait := func(predicate func(contracts.RunView) bool, budget time.Duration) contracts.RunView {
			for until := time.Now().Add(budget); time.Now().Before(until); time.Sleep(150 * time.Millisecond) {
				v, err := inspect(m, dir, id, false)
				if err != nil {
					t.Fatal(err)
				}
				if predicate(v) {
					return v
				}
				if v.ResumeRequired {
					save(t, "wait-unexpected-blocker.json", v)
					t.Fatal("query blocked within its window", v.Diagnostic)
				}
			}
			t.Fatal("original Run did not reach native wait window")
			return contracts.RunView{}
		}
		queued := wait(func(v contracts.RunView) bool { return v.ExecutionWait != nil && v.ExecutionWait.Stage == "queue" }, 30*time.Second)
		save(t, fmt.Sprintf("wait-queue-%d.json", i), queued)
		if queued.ExecutionWait.CallsStarted != 0 || len(queued.Operations) != 1 || queued.ExecutionWait.OperationID != queued.Operations[0].ID {
			t.Fatal("uninvoked query has calls or another operation")
		}
		if i == 1 {
			if err := revokeOriginalRun(m, dir, id, "wait-stop-queued"); err != nil {
				t.Fatal(err)
			}
			final := waitStopped(t, m, dir, id)
			close(release)
			held = false
			time.Sleep(300 * time.Millisecond)
			latest, err := inspect(m, dir, id, false)
			if err != nil || latest.ExecutionWait != nil || latest.Execution.Workspace != "" || latest.Execution.TurnStarted || len(latest.Operations) != 1 || latest.Operations[0].ID != queued.Operations[0].ID || latest.Revision != final.Revision {
				t.Fatal("queued stop dispatched native work", latest, err)
			}
			save(t, "wait-canceled.json", latest)
			result.CancelVerified = true
			continue
		}
		time.Sleep(16 * time.Second)
		still, err := inspect(m, dir, id, false)
		if err != nil || still.ExecutionWait == nil || still.ExecutionWait.Stage != "queue" || still.ExecutionWait.CallsStarted != 0 || still.ResumeRequired || still.Revision != queued.Revision {
			t.Fatal("old queue timeout or GET mutation", still, err)
		}
		save(t, "wait-after-old-limit.json", still)
		close(release)
		held = false
		recovered := wait(func(v contracts.RunView) bool { return v.Execution.Workspace != "" && v.Execution.SessionID != "" }, 90*time.Second)
		if recovered.ExecutionWait != nil || recovered.Operations[0].ID != queued.ExecutionWait.OperationID || recovered.Execution.AttemptID != queued.Execution.AttemptID {
			t.Fatal("recovery replaced original native preparation")
		}
		save(t, "wait-recovered.json", recovered)
		if err := revokeOriginalRun(m, dir, id, "wait-stop-recovered"); err != nil {
			t.Fatal(err)
		}
		waitStopped(t, m, dir, id)
		result.WaitVerified = true
	}
}

// Independently read the forwarding journal: exactly one native workspace
// creation, complete mutation receipts, and no repeated business input text.
func waitNativeAudit(t *testing.T, m livetest.Manifest) bool {
	f, err := os.Open(filepath.Join(m.Root, "evidence", "native-calls.jsonl"))
	if err != nil {
		t.Error(err)
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 24<<20)
	intents, receipts, prompts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	creates, queries := 0, 0
	expectedRuntime, err := m.OrcaRuntimeID()
	if err != nil {
		t.Error(err)
		return false
	}
	for scanner.Scan() {
		var row struct {
			Kind     string `json:"kind"`
			ID       string `json:"call_id"`
			Args     []string
			Failed   bool
			Response json.RawMessage
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			t.Error("invalid native journal")
			return false
		}
		if len(row.Args) < 2 {
			continue
		}
		if row.Kind == "intent" && (row.Args[0] == "status" || row.Args[0] == "repo" && row.Args[1] == "list") {
			queries++
		}
		mutation := row.Args[0] == "worktree" && row.Args[1] == "create" || row.Args[0] == "terminal" && (row.Args[1] == "create" || row.Args[1] == "send")
		if !mutation {
			continue
		}
		if row.ID == "" {
			t.Error("native mutation has no call identity")
			return false
		}
		if row.Kind == "intent" {
			if intents[row.ID] {
				t.Error("duplicate mutation identity")
				return false
			}
			intents[row.ID] = true
			if row.Args[0] == "worktree" {
				creates++
			}
			for i, arg := range row.Args {
				if arg == "--text" && i+1 < len(row.Args) {
					text := strings.TrimPrefix(row.Args[i+1], "\x1b[200~")
					if strings.HasPrefix(text, "[SteerRunner command=") {
						if prompts[text] {
							t.Error("business input repeated")
							return false
						}
						prompts[text] = true
					}
				}
			}
		} else if row.Kind == "receipt" {
			var e struct {
				OK   bool
				Meta struct {
					RuntimeID string `json:"runtimeId"`
				} `json:"_meta"`
			}
			if row.Failed || json.Unmarshal(row.Response, &e) != nil || !e.OK || e.Meta.RuntimeID != expectedRuntime || receipts[row.ID] {
				t.Error("native mutation receipt unverified")
				return false
			}
			receipts[row.ID] = true
		}
	}
	for id := range intents {
		if !receipts[id] {
			t.Error("native mutation lacks receipt", id)
			return false
		}
	}
	if scanner.Err() != nil || creates != 1 || queries != 1 || len(intents) != len(receipts) {
		t.Error("native creation count or receipts invalid", creates, scanner.Err())
		return false
	}
	save(t, "wait-native-audit.json", map[string]any{"workspace_creations": creates, "preparation_queries": queries, "mutation_intents": len(intents), "mutation_receipts": len(receipts), "unique_business_inputs": len(prompts), "passed": true})
	return true
}
