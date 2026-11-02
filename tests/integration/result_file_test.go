//go:build windows

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

var resultFileCasesFlag = flag.String("result-file-cases", "", "Exact ResultFile case IDs, comma separated; empty runs both")

func runResultFileVariant(t *testing.T, mask int, run func(*testing.T)) bool {
	t.Helper()
	executed := false
	passed := t.Run(fmt.Sprintf("enabled-%d", mask), func(t *testing.T) {
		executed = true
		run(t)
	})
	if !executed {
		t.Fatalf("required ResultFile enable variant did not execute: %d", mask)
	}
	return passed
}

// Controlled public-EXE loop only: the native fixture is not a real remote Agent.
func TestSRResultFile(t *testing.T) {
	testCount.Add(1)
	for _, id := range selectedControlledCases("TestSRResultFile") {
		runControlledCase(t, "TestSRResultFile", id, func(t *testing.T) {
			if id == "final-seal" {
				testResultFileFinalSeal(t)
			} else {
				testOptionalFilesDoNotGateWorkflow(t)
			}
		})
	}
}

func testResultFileFinalSeal(t *testing.T) {
	dir, req := setupRun(t)
	t.Setenv("STEERLINE_PROGRESS_FILE", "C:/foreign-run/progress.bin")
	t.Setenv("STEERLINE_RESULT_FILE", "C:/foreign-run/result.bin")
	if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn"), []byte("hold"), 0600); err != nil {
		t.Fatal(err)
	}
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	type fixture struct {
		ID, Workspace, ResultPath, ProgressPath string
		Progress, Result                        bool
		Final                                   []byte
		Sealed                                  contracts.ResultFileView
	}
	fixtures := []fixture{}
	read := func(t *testing.T, command, id string) contracts.ResultFileView {
		response := call(t, dir, []string{command, "--run-id", id}, "", 0)
		var v contracts.ResultFileView
		raw, _ := json.Marshal(response.Data)
		if json.Unmarshal(raw, &v) != nil || v.RunID != id || v.Encoding != "base64" {
			t.Fatal(response)
		}
		if command == "result-file" && v.Contract != contracts.ResultFileContract {
			t.Fatal(v)
		}
		return v
	}
	waitResult := func(t *testing.T, id string) contracts.ResultFileView {
		t.Helper()
		until := time.Now().Add(20 * time.Second)
		for time.Now().Before(until) {
			v := read(t, "result-file", id)
			if v.CollectionStatus == "closed" {
				return v
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("final result timeout", read(t, "result-file", id))
		return contracts.ResultFileView{}
	}
	stop := func(t *testing.T, id string) {
		t.Helper()
		v := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
		if v.StopStatus == "stopped" {
			return
		}
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "final-result-stop-" + id, Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, v.Revision))})
		call(t, dir, []string{"stop", "--run-id", id, "--input", "-"}, string(raw), 0)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	}
	defer func() {
		for _, f := range fixtures {
			stop(t, f.ID)
		}
	}()
	base := append([]byte(nil), req.Input...)
	for i, combo := range []struct{ progress, result bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		if !runResultFileVariant(t, i, func(t *testing.T) {
			var input contracts.RunRequest
			_ = json.Unmarshal(base, &input)
			if combo.progress {
				input.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: "cHJvZ3Jlc3M="}
			}
			if combo.result {
				input.ResultFile = &contracts.ResultFileInput{InitialContentBase64: "c2VlZA=="}
			}
			req.RequestID = fmt.Sprintf("final-result-%d", i)
			req.Input, _ = json.Marshal(input)
			raw, _ := json.Marshal(req)
			response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := response.Data.(map[string]any)["run_id"].(string)
			fixtures = append(fixtures, fixture{ID: id, Progress: combo.progress, Result: combo.result})
			f := &fixtures[len(fixtures)-1]
			run := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.TurnID != "" })
			f.Workspace = run.Execution.Workspace
			resultPath := filepath.Join(f.Workspace, ".steerrunner", "result", id, "result.bin")
			progressPath := filepath.Join(f.Workspace, ".steerrunner", "progress", id, "progress.bin")
			if combo.result {
				f.ResultPath = resultPath
			}
			if combo.progress {
				f.ProgressPath = progressPath
			}
			if resultPath == progressPath || !filepath.IsAbs(resultPath) {
				t.Fatal("file path isolation")
			}
			v := read(t, "result-file", id)
			expected := "disabled"
			if combo.result {
				expected = "pending"
			}
			if v.CollectionStatus != expected || v.Version != 0 || v.ContentBase64 != nil || v.ObservedAt != nil || v.SHA256 != "" {
				t.Fatal("result was collected while running", v)
			}
			if combo.result {
				initial, err := os.ReadFile(resultPath)
				if err != nil || string(initial) != "seed" {
					t.Fatal("initialization", err, string(initial))
				}
				f.Final = []byte{0, 255, 'r', byte(i)}
				if err := os.WriteFile(resultPath+".tmp", f.Final, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(resultPath+".tmp", resultPath); err != nil {
					t.Fatal(err)
				}
			} else if _, err := os.Stat(filepath.Dir(resultPath)); !os.IsNotExist(err) {
				t.Fatal("disabled result created file", err)
			}
			if combo.progress {
				if err := os.WriteFile(progressPath, []byte("progress-live"), 0600); err != nil {
					t.Fatal(err)
				}
				until := time.Now().Add(20 * time.Second)
				for {
					p := read(t, "progress-file", id)
					if p.ContentBase64 != nil && *p.ContentBase64 == base64.StdEncoding.EncodeToString([]byte("progress-live")) {
						break
					}
					if time.Now().After(until) {
						t.Fatal("progress delivery", p)
					}
					time.Sleep(100 * time.Millisecond)
				}
			} else if read(t, "progress-file", id).CollectionStatus != "disabled" {
				t.Fatal("progress enabled unexpectedly")
			}
			if got := read(t, "result-file", id); !reflect.DeepEqual(v, got) {
				t.Fatal("running write refreshed result", v, got)
			}
			// The native process, not the parent shell, reports its actual environment.
			calls, err := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range bytes.Split(calls, []byte("\n")) {
				var event map[string]any
				if json.Unmarshal(line, &event) != nil || event["method"] != "result/environment" {
					continue
				}
				cwd, _ := event["cwd"].(string)
				if !strings.EqualFold(filepath.Clean(cwd), filepath.Clean(f.Workspace)) {
					continue
				}
				found = true
				if event["path"] != f.ResultPath || event["progress_path"] != f.ProgressPath {
					t.Fatal("inherited or crossed environment", event, f)
				}
			}
			if !found {
				t.Fatal("native environment unobserved")
			}
		}) {
			t.FailNow()
		}
	}
	nativeBefore := countNative(t, "turn/start")
	for i := range fixtures {
		f := &fixtures[i]
		if f.Result {
			call(t, dir, []string{"result-file", "--run-id", f.ID, "--expected-instance-id", "wrong-instance"}, "", 5)
			if read(t, "result-file", f.ID).CollectionStatus != "pending" {
				t.Fatal("query collected result")
			}
		}
		stop(t, f.ID)
		if !f.Result {
			continue
		}
		f.Sealed = waitResult(t, f.ID)
		v := f.Sealed
		if v.Version != 1 || v.Diagnostic != "" || v.ContentBase64 == nil || v.ObservedAt == nil {
			t.Fatal(v)
		}
		data, err := base64.StdEncoding.DecodeString(*v.ContentBase64)
		if err != nil || !bytes.Equal(data, f.Final) || v.SizeBytes != len(f.Final) || v.SHA256 != fmt.Sprintf("%x", sha256.Sum256(f.Final)) {
			t.Fatal("final bytes", v)
		}
		if err := os.WriteFile(f.ResultPath, []byte("after-seal"), 0600); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v, read(t, "result-file", f.ID)) {
			t.Fatal("query refreshed seal")
		}
	}
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("final-result-restart", service.InstanceID), 0)
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	service = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	for _, f := range fixtures {
		if f.Result && !reflect.DeepEqual(f.Sealed, read(t, "result-file", f.ID)) {
			t.Fatal("restart refreshed seal", f)
		}
	}
	if countNative(t, "turn/start") != nativeBefore {
		t.Fatal("query/restart dispatched input")
	}
	if *evidenceFlag != "" {
		raw, _ := json.MarshalIndent(map[string]any{"mode": "controlled-final-only", "runs": fixtures, "native_turns": nativeBefore, "service_instance_after_restart": service.InstanceID}, "", "  ")
		if err := os.WriteFile(filepath.Join(*evidenceFlag, "result-final-public-evidence.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func testOptionalFilesDoNotGateWorkflow(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		runResultFileVariant(t, mask, func(t *testing.T) {
			dir, req := setupRun(t)
			var input contracts.RunRequest
			if err := json.Unmarshal(req.Input, &input); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(input.Repository.LocalPath, ".steerrunner")
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			for _, channel := range []string{"progress", "result"} {
				if err := os.WriteFile(filepath.Join(root, channel), []byte("blocked optional directory"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git(t, input.Repository.LocalPath, "add", ".")
			git(t, input.Repository.LocalPath, "-c", "user.name=Runner Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Block optional file directories")
			input.Repository.BaseRevision = git(t, input.Repository.LocalPath, "rev-parse", "HEAD")
			git(t, input.Repository.LocalPath, "push", "origin", "main")
			if mask&1 != 0 {
				input.ProgressFile = &contracts.ProgressFileInput{}
			}
			if mask&2 != 0 {
				input.ResultFile = &contracts.ResultFileInput{}
			}
			req.Input, _ = json.Marshal(input)
			raw, _ := json.Marshal(req)
			service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, service)
			start := time.Now()
			response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
			id := response.Data.(map[string]any)["run_id"].(string)
			t.Cleanup(func() {
				v := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
				if v.StopStatus != "stopped" {
					stop, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "optional-cleanup-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, v.Revision))})
					call(t, dir, []string{"stop", "--run-id", id, "--input", "-"}, string(stop), 0)
					waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
				}
			})
			ready := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
			if ready.PR == nil || ready.ResumeRequired || !ready.NewInputsAllowed || countNative(t, "turn/start") != 3 {
				t.Fatal("optional failure disrupted workflow", ready)
			}
			elapsed := time.Since(start)
			stop, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "optional-failure-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, ready.Revision))})
			call(t, dir, []string{"stop", "--run-id", id, "--input", "-"}, string(stop), 0)
			waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			if mask&2 != 0 {
				var v contracts.ResultFileView
				until := time.Now().Add(5 * time.Second)
				for {
					reply := call(t, dir, []string{"result-file", "--run-id", id}, "", 0)
					body, _ := json.Marshal(reply.Data)
					_ = json.Unmarshal(body, &v)
					if v.CollectionStatus == "closed" {
						break
					}
					if time.Now().After(until) {
						t.Fatal("file fault prevented final settlement", v)
					}
					time.Sleep(100 * time.Millisecond)
				}
				if v.Version != 0 || v.Diagnostic == "" {
					t.Fatal("failed channel fabricated final bytes", v)
				}
			}
			t.Logf("enabled=%d: pr_ready=%v, business inputs=3 (including original result qualification fallback), stop confirmed", mask, elapsed)
		})
	}
}
