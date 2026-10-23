//go:build windows && live

package native

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	nativeorca "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func progressFiles(t *testing.T, m livetest.Manifest) {
	result := struct {
		scenarioResult
		ProgressVerified bool `json:"progress_verified"`
	}{scenarioResult: scenarioResult{Scenario: m.Scenario, Level: m.Level, Stage: "submit"}}
	dir := filepath.Join(m.Root, "s")
	ids := []string{}
	views := map[string]contracts.RunView{}
	samples := map[string]contracts.ProgressFileView{}
	sequence := 0
	record := func(kind string, value any) {
		sequence++
		save(t, fmt.Sprintf("progress-%03d-%s.json", sequence, kind), value)
	}
	serviceRaw := invoke(t, m, dir, []string{"service", "start"}, nil)
	var service contracts.ServiceInfo
	raw, _ := json.Marshal(serviceRaw.Data)
	_ = json.Unmarshal(raw, &service)
	// Cleanup executes even when a native prerequisite fails. Only this new
	// fixture's original Run IDs are stopped. No shared runtime is stopped.
	t.Cleanup(func() {
		all := len(ids) > 0
		for _, id := range ids {
			if err := revokeOriginalRun(m, dir, id, "progress-cleanup-"+id); err != nil {
				all = false
				t.Error(err)
				continue
			}
			stopped := false
			stopWait := 120 * time.Second
			if m.Execution.Adapter == "orca-terminal" {
				// The original close may complete near its first bounded budget;
				// allow the next readonly reconciliation to persist that proof.
				stopWait = 180 * time.Second
			}
			for until := time.Now().Add(stopWait); time.Now().Before(until); time.Sleep(300 * time.Millisecond) {
				v, err := inspect(m, dir, id, false)
				if err == nil {
					views[id] = v
					if v.StopStatus == "stopped" && !v.NewInputsAllowed {
						stopped = true
						break
					}
				}
			}
			if !stopped {
				all = false
				t.Error("original native stop unverified", id)
			}
			if stopped {
				response, err := command(m, dir, []string{"progress-file", "--run-id", id}, nil)
				record("retained-after-stop", response)
				var snapshot contracts.ProgressFileView
				raw, _ := json.Marshal(response.Data)
				_ = json.Unmarshal(raw, &snapshot)
				if err != nil || snapshot.ContentBase64 == nil || snapshot.Version < samples[id].Version {
					t.Error("last snapshot unavailable after stop", err)
					all = false
				}
			}
		}
		result.RunStopped = all
		req := request("progress-service-stop", map[string]string{"expected_instance_id": service.InstanceID})
		response, err := command(m, dir, []string{"service", "stop", "--input", "-"}, &req)
		record("service-stop", response)
		if err == nil {
			for until := time.Now().Add(60 * time.Second); time.Now().Before(until); time.Sleep(200 * time.Millisecond) {
				r, e := command(m, dir, []string{"service", "status"}, nil)
				if e == nil && r.Data.(map[string]any)["status"] == "not_running" {
					result.ServiceStopped = true
					break
				}
			}
		}
		if !result.ServiceStopped {
			t.Error("service exit unverified", err)
		} else {
			proof, err := command(m, dir, []string{"service", "verify-stopped"}, nil)
			record("offline-stop", proof)
			var settled contracts.StoppedVerification
			body, _ := json.Marshal(proof.Data)
			decodeErr := json.Unmarshal(body, &settled)
			if err != nil || decodeErr != nil || !settled.ResourcesSettled {
				t.Error("resource settlement unverified", err)
				result.RunStopped = false
			}
			for _, id := range ids {
				found := false
				for _, original := range settled.Runs {
					if original.RunID == id && original.ResourcesSettled {
						found = true
					}
				}
				if !found {
					t.Error("original Run missing from settled resources", id)
					result.RunStopped = false
				}
			}
			if m.Execution.Adapter == "codex-app-server" {
				for _, id := range ids {
					var source struct {
						Thread struct {
							ID   string `json:"id"`
							Path string `json:"path"`
						} `json:"thread"`
					}
					body, err := os.ReadFile(filepath.Join(dir, "runs", id, "app-server", "thread.json"))
					if err != nil || json.Unmarshal(body, &source) != nil || source.Thread.ID != views[id].Execution.SessionID || source.Thread.Path == "" {
						t.Error("original native history binding unavailable", id, err)
						continue
					}
					raw, err := os.ReadFile(source.Thread.Path)
					if err != nil {
						t.Error(err)
						continue
					}
					if err := os.WriteFile(filepath.Join(*evidenceFlag, "native-session-"+id+".jsonl"), raw, 0600); err != nil {
						t.Error(err)
					}
					record("native-history-source", map[string]any{"run_id": id, "source": source.Thread.Path, "session_id": source.Thread.ID, "bytes": len(raw)})
				}
			}
			// Preserve exact records and operation/input identities as original native
			// execution evidence, independently of public snapshot assertions.
			reader, err := storage.OpenReader(dir)
			if err == nil {
				runs, e := reader.Runs()
				reader.Close()
				if e == nil {
					record("original-records", runs)
				} else {
					t.Error(e)
				}
			} else {
				t.Error(err)
			}
		}
		for _, id := range ids {
			result.Runs = append(result.Runs, views[id])
		}
		result.Passed = !t.Failed() && result.ProgressVerified && result.RunStopped && result.ServiceStopped
		if result.Passed {
			result.Stage = "completed"
		}
		save(t, "scenario.json", result)
	})
	source, err := m.ExecutionRepositoryPath()
	if err != nil {
		t.Fatal(err)
	}
	count := 1
	if m.Scenario == "local-codex.progress-multirun" {
		count = 2
	}
	for i := 0; i < count; i++ {
		prompt := `Use the execution environment's native shell execution tool (exec_command or the provided powershell shell tool) to run powershell.exe -NoProfile -ExecutionPolicy Bypass -File sr-smoke/progress.ps1 exactly once in the current workspace. Do not use node_repl, child_process, GUI tools or another host's process API: those are not this execution environment. Keep waiting for this foreground process; it deliberately waits for external gates. Set the shell tool execution timeout to 600000 milliseconds if supported. If the tool returns a running session ID, poll that same session until it ends; never restart the script. Do not detach it or start it twice. Do not modify other files or publish Git/PR. Use the injected environment as-is. Do not inspect or print credentials. The frozen Spec and Plan describe this same producer-only task; there is no implementation, checker or publication task.`
		flow, err := m.RunWorkflow(map[string]string{workflow.StepImplement: prompt})
		if err != nil {
			t.Fatal(err)
		}
		input := contracts.RunRequest{Title: fmt.Sprintf("Progress byte channel %s %d", m.ID, i), Repository: contracts.Repository{URL: livetest.RemoteURL, LocalPath: source, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Path: "sr-smoke/spec.md"}, Plan: contracts.Document{Path: "sr-smoke/plan.md"}}, Workflow: flow, Execution: m.Execution, Publication: m.Publication, ProgressFile: &contracts.ProgressFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte("seed|" + m.ID))}}
		req := request(fmt.Sprintf("progress-submit-%d", i), input)
		record("submit-request", req)
		accepted := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
		record("submit-response", accepted)
		id := accepted.Data.(map[string]any)["run_id"].(string)
		ids = append(ids, id)
		if result.RunID == "" {
			result.RunID = id
		}
	}
	read := func(id string) contracts.ProgressFileView {
		response := invoke(t, m, dir, []string{"progress-file", "--run-id", id}, nil)
		var v contracts.ProgressFileView
		raw, _ := json.Marshal(response.Data)
		if json.Unmarshal(raw, &v) != nil || v.RunID != id {
			t.Fatal("wrong progress identity", response)
		}
		return v
	}
	wait := func(id, word string) contracts.ProgressFileView {
		t.Helper()
		// Includes a cold Orca workspace/control/helper/TUI preparation while
		// other independent cases share its bounded CLI queue. The stable writer
		// and finalization budgets remain separate and finite.
		budget := 5 * time.Minute
		if m.Execution.Adapter == "orca-terminal" {
			budget = 8 * time.Minute
		}
		until := time.Now().Add(budget)
		for time.Now().Before(until) {
			v := read(id)
			if v.ContentBase64 != nil {
				data, err := base64.StdEncoding.DecodeString(*v.ContentBase64)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.HasPrefix(data, []byte(word+"|")) {
					view, err := inspect(m, dir, id, false)
					if err != nil {
						t.Fatal(err)
					}
					views[id] = view
					path := filepath.Join(view.Execution.Workspace, ".steerrunner", "progress", id, "progress.bin")
					want := append([]byte(word+"|"+path), 0, 255, 17)
					if !bytes.Equal(data, want) || v.SizeBytes != len(data) || len(v.SHA256) != 64 || v.ObservedAt == nil {
						t.Fatal("injected path/raw bytes mismatch", v, string(data))
					}
					samples[id] = v
					record("snapshot", map[string]any{"response": v, "run": view, "native_final_available": view.Execution.FinalAvailable})
					return v
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
		record("timeout", read(id))
		view, _ := inspect(m, dir, id, false)
		record("timeout-run", view)
		t.Fatal("native producer did not publish", id, word)
		return contracts.ProgressFileView{}
	}
	marker := func(id string, step int) {
		t.Helper()
		relative := fmt.Sprintf(".sr-runtime-stage/progress-control/%s/%d", id, step)
		v, err := inspect(m, dir, id, false)
		if err != nil {
			t.Fatal(err)
		}
		if m.Execution.Adapter == "orca-terminal" {
			// The Run's exact terminal/worktree/runtime comes from its durable original
			// record; this fixture reader never creates or rebinds a native resource.
			reader, err := storage.OpenReader(dir)
			if err != nil {
				t.Fatal(err)
			}
			runs, err := reader.Runs()
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			var process b.ProcessIdentity
			for _, r := range runs {
				if r.ID == id {
					process = r.Snapshot.Ref.External
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err = nativeorca.UploadWorkspaceArtifact(ctx, m.Execution.Target.Environment, process.RuntimeID, "id:"+process.WorktreeID, relative, strings.NewReader("release"))
			if err != nil {
				t.Fatal(err)
			}
		} else {
			path := filepath.Join(v.Execution.Workspace, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("release"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		record("gate", map[string]any{"run_id": id, "step": step})
	}
	for _, id := range ids {
		first := wait(id, "violet")
		if first.Version < 2 {
			t.Fatal("initial snapshot missing", first)
		}
		if views[id].Execution.FinalAvailable {
			t.Fatal("producer unexpectedly finished")
		}
	}
	if len(ids) == 2 && views[ids[0]].Execution.Workspace == views[ids[1]].Execution.Workspace {
		t.Fatal("multi-Run shared workspace")
	}
	// Each independent fixture sustains observation while its own foreground
	// native producer waits. No barrier or resource is shared with other cases.
	// Actual parallel qualification is derived from these recorded intervals.
	windowStart := time.Now().UTC()
	for until := windowStart.Add(120 * time.Second); time.Now().Before(until); time.Sleep(2 * time.Second) {
		for _, id := range ids {
			current := read(id)
			if current.Version != samples[id].Version || current.ContentBase64 == nil || *current.ContentBase64 != *samples[id].ContentBase64 || current.CollectionStatus != "observing" {
				t.Fatal("waiting producer observation changed", current)
			}
		}
	}
	for _, id := range ids {
		current, err := inspect(m, dir, id, false)
		if err != nil || current.Execution.SessionID != views[id].Execution.SessionID || current.Execution.FinalAvailable {
			t.Fatal("waiting native execution changed", current, err)
		}
	}
	record("observation-window", map[string]any{"run_ids": ids, "start": windowStart, "end": time.Now().UTC(), "duration_seconds": time.Since(windowStart).Seconds()})
	for _, id := range ids {
		marker(id, 1)
		before := samples[id]
		after := wait(id, "copper")
		if after.Version != before.Version+1 {
			t.Fatal("content version did not advance once")
		}
		marker(id, 2)
		time.Sleep(900 * time.Millisecond)
		if read(id).Version != after.Version {
			t.Fatal("identical content created snapshot")
		}
	}
	if len(ids) == 1 && m.Scenario != "local-codex.progress-recovery" {
		// Passing the next producer gate proves the identical rewrite actually
		// ran. A scheduler delay cannot turn the previous timing check into a
		// false pass; only this new content may add a version.
		before := samples[ids[0]]
		marker(ids[0], 3)
		after := wait(ids[0], "silver")
		if after.Version != before.Version+1 {
			t.Fatal("identical rewrite advanced the content version")
		}
	}
	if m.Execution.Target.Kind == "orca_environment" {
		path := filepath.Join(views[ids[0]].Execution.Workspace, ".steerrunner", "progress", ids[0], "progress.bin")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("remote source unexpectedly exists locally", path, err)
		}
		record("remote-source-absent-locally", path)
	}
	if m.Scenario == "local-codex.progress-recovery" {
		id := ids[0]
		original := views[id]
		last := read(id)
		path := filepath.Join(original.Execution.Workspace, ".steerrunner", "progress", id, "progress.bin")
		if err := os.Rename(path, path+".missing"); err != nil {
			t.Fatal(err)
		}
		hit := false
		for until := time.Now().Add(5 * time.Second); time.Now().Before(until); time.Sleep(100 * time.Millisecond) {
			v := read(id)
			if v.CollectionStatus == "unavailable" {
				if v.Version != last.Version || v.ContentBase64 == nil {
					t.Fatal("fault discarded snapshot")
				}
				hit = true
				record("read-fault", v)
				break
			}
		}
		if err := os.Rename(path+".missing", path); err != nil {
			t.Fatal(err)
		}
		if !hit {
			t.Fatal("read fault not hit")
		}
		before, _ := inspect(m, dir, id, false)
		oldInstance := service.InstanceID
		req := request("progress-restart", map[string]string{"expected_instance_id": service.InstanceID})
		invoke(t, m, dir, []string{"service", "stop", "--input", "-"}, &req)
		offline := false
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); time.Sleep(100 * time.Millisecond) {
			r, e := command(m, dir, []string{"service", "status"}, nil)
			if e == nil && r.Data.(map[string]any)["status"] == "not_running" {
				record("offline-before-restart", r)
				offline = true
				break
			}
		}
		if !offline {
			t.Fatal("original service did not exit before restart")
		}
		// The original native foreground script continues while Runner is offline.
		release := filepath.Join(original.Execution.Workspace, ".sr-runtime-stage", "progress-control", id, "3")
		_ = os.WriteFile(release, []byte("release"), 0600)
		time.Sleep(700 * time.Millisecond)
		response := invoke(t, m, dir, []string{"service", "start"}, nil)
		raw, _ := json.Marshal(response.Data)
		_ = json.Unmarshal(raw, &service)
		if service.InstanceID == "" || service.InstanceID == oldInstance {
			t.Fatal("service instance did not change on restart")
		}
		after := wait(id, "silver")
		view, _ := inspect(m, dir, id, false)
		if after.Version != last.Version+1 || view.Execution.SessionID != before.Execution.SessionID || view.Execution.TurnID != before.Execution.TurnID || view.Execution.AttemptID != before.Execution.AttemptID || len(view.Operations) != len(before.Operations) || view.NewInputsAllowed != before.NewInputsAllowed || view.ResumeRequired != before.ResumeRequired {
			t.Fatal("restart replayed/rebound original execution", before, view)
		}
		record("recovery", map[string]any{"before": before, "after": view, "snapshot": after})
	}
	if len(ids) == 2 {
		before, _ := inspect(m, dir, ids[1], false)
		if err := revokeOriginalRun(m, dir, ids[0], "progress-stop-first"); err != nil {
			t.Fatal(err)
		}
		stopped := false
		for until := time.Now().Add(45 * time.Second); time.Now().Before(until); time.Sleep(200 * time.Millisecond) {
			first, err := inspect(m, dir, ids[0], false)
			if err != nil {
				t.Fatal(err)
			}
			if first.StopStatus == "stopped" && !first.NewInputsAllowed {
				views[ids[0]] = first
				record("first-stopped-before-peer-write", first)
				stopped = true
				break
			}
		}
		if !stopped {
			t.Fatal("first Run stop was not confirmed before peer update")
		}
		marker(ids[1], 3)
		wait(ids[1], "silver")
		after, _ := inspect(m, dir, ids[1], false)
		if before.Execution.SessionID != after.Execution.SessionID || before.Execution.TurnID != after.Execution.TurnID || before.Revision != after.Revision || !after.NewInputsAllowed || after.ResumeRequired {
			t.Fatal("peer execution/revision changed with progress", before, after)
		}
		record("multi-isolation", map[string]any{"first": ids[0], "peer_before": before, "peer_after": after})
	}
	result.ProgressVerified = true
	result.Stage = "verified"
}
