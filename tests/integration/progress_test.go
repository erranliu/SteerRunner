//go:build windows

package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestSRProgressFile(t *testing.T) {
	testCount.Add(1)
	dir, req := setupRun(t)
	hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")
	if err := os.WriteFile(hold, []byte("hold"), 0600); err != nil {
		t.Fatal(err)
	}
	service := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	ids := []string{}
	paths := []string{}
	evidence := []contracts.ProgressFileView{}
	read := func(id string) contracts.ProgressFileView {
		response := call(t, dir, []string{"progress-file", "--run-id", id}, "", 0)
		var v contracts.ProgressFileView
		raw, _ := json.Marshal(response.Data)
		if json.Unmarshal(raw, &v) != nil || v.RunID != id {
			t.Fatal(response)
		}
		if len(evidence) == 0 || evidence[len(evidence)-1].RunID != v.RunID || evidence[len(evidence)-1].Version != v.Version || evidence[len(evidence)-1].CollectionStatus != v.CollectionStatus {
			evidence = append(evidence, v)
		}
		return v
	}
	wait := func(id string, match func(contracts.ProgressFileView) bool) contracts.ProgressFileView {
		t.Helper()
		until := time.Now().Add(20 * time.Second)
		for time.Now().Before(until) {
			v := read(id)
			if match(v) {
				return v
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("progress timeout", read(id))
		return contracts.ProgressFileView{}
	}
	for i := 0; i < 2; i++ {
		var input contracts.RunRequest
		_ = json.Unmarshal(req.Input, &input)
		input.ProgressFile = &contracts.ProgressFileInput{InitialContentBase64: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("seed-%d", i)))}
		req.RequestID = fmt.Sprintf("progress-%d", i)
		req.Input, _ = json.Marshal(input)
		raw, _ := json.Marshal(req)
		accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		id := accepted.Data.(map[string]any)["run_id"].(string)
		ids = append(ids, id)
		waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.Workspace != "" })
		wait(id, func(v contracts.ProgressFileView) bool { return v.Version == 1 })
		run := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Execution.TurnID != "" })
		paths = append(paths, filepath.Join(run.Execution.Workspace, ".steerrunner", "progress", id, "progress.bin"))
		replay := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
		if replay.Data.(map[string]any)["run_id"] != id {
			t.Fatal("replay")
		}
	}
	if paths[0] == paths[1] {
		t.Fatal("shared progress path")
	}
	defer func() {
		for _, id := range ids {
			v := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
			if v.StopStatus != "stopped" {
				raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "cleanup-progress-" + id, Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, v.Revision))})
				call(t, dir, []string{"stop", "--run-id", id, "--input", "-"}, string(raw), 0)
				waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
			}
		}
	}()
	before := runView(t, call(t, dir, []string{"inspect", "--run-id", ids[0]}, "", 0))
	nativeBefore := countNative(t, "turn/start")
	data := []byte{0, 255, 'v', 'i', 'o', 'l', 'e', 't'}
	for i := 0; i < 2; i++ {
		tmp := paths[i] + ".tmp"
		_ = os.WriteFile(tmp, append(data, byte(i)), 0600)
		replaceProgressProducerFile(t, tmp, paths[i])
		v := wait(ids[i], func(v contracts.ProgressFileView) bool { return v.Version == 2 })
		decoded, _ := base64.StdEncoding.DecodeString(*v.ContentBase64)
		if !bytes.Equal(decoded, append(data, byte(i))) {
			t.Fatal(v)
		}
	}
	_ = os.WriteFile(paths[0], append(data, 0), 0600)
	time.Sleep(800 * time.Millisecond)
	if read(ids[0]).Version != 2 {
		t.Fatal("same bytes incremented")
	}
	after := runView(t, call(t, dir, []string{"inspect", "--run-id", ids[0]}, "", 0))
	if before.Revision != after.Revision {
		t.Fatal("file changed business revision", before.Revision, after.Revision)
	}
	_ = os.Remove(paths[0])
	last := wait(ids[0], func(v contracts.ProgressFileView) bool { return v.CollectionStatus == "unavailable" })
	if last.Version != 2 || last.ContentBase64 == nil {
		t.Fatal("lost last snapshot")
	}
	_ = os.WriteFile(paths[1], []byte("peer-still-live"), 0600)
	wait(ids[1], func(v contracts.ProgressFileView) bool { return v.Version == 3 })
	// Service replacement retains original inputs and original file. No resume.
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("progress-restart", service.InstanceID), 0)
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(paths[0], []byte("offline-latest"), 0600)
	service = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	v := wait(ids[0], func(v contracts.ProgressFileView) bool { return v.Version == 3 })
	decoded, _ := base64.StdEncoding.DecodeString(*v.ContentBase64)
	if string(decoded) != "offline-latest" || countNative(t, "turn/start") != nativeBefore {
		t.Fatal("restart reinitialized/replayed", v)
	}
	current := runView(t, call(t, dir, []string{"inspect", "--run-id", ids[0]}, "", 0))
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	wait(ids[0], func(v contracts.ProgressFileView) bool { return v.CollectionStatus == "unavailable" && v.Version == 3 })
	raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: "progress-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, current.Revision))})
	call(t, dir, []string{"stop", "--run-id", ids[0], "--input", "-"}, string(raw), 0)
	waitRun(t, dir, ids[0], func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	wait(ids[0], func(v contracts.ProgressFileView) bool { return v.CollectionStatus == "closed" })
	_ = os.WriteFile(paths[1], []byte("peer-after-stop"), 0600)
	wait(ids[1], func(v contracts.ProgressFileView) bool { return v.Version == 4 })
	if read(ids[0]).Version != 3 {
		t.Fatal("terminal snapshot lost")
	}
	closed := read(ids[0])
	if err := os.WriteFile(paths[0], []byte("written-after-finalization"), 0600); err != nil {
		t.Fatal(err)
	}
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("progress-closed-restart", service.InstanceID), 0)
	if err := waitOwnedService(service); err != nil {
		t.Fatal(err)
	}
	service = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, service)
	if err := os.WriteFile(paths[1], []byte("peer-after-closed-restart"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(ids[1], func(v contracts.ProgressFileView) bool { return v.Version == 5 })
	// The active peer proves that this replacement observer has actually run.
	// Continue sampling beyond its first tick to reject a late terminal restart.
	for until := time.Now().Add(time.Second); time.Now().Before(until); time.Sleep(100 * time.Millisecond) {
		if got := read(ids[0]); !reflect.DeepEqual(got, closed) {
			t.Fatal("service restart recollected a finalized source", closed, got)
		}
	}
	if countNative(t, "turn/start") != nativeBefore {
		t.Fatal("closed observation restart dispatched another business input")
	}
	calls, _ := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	for _, p := range paths {
		if !bytes.Contains(calls, []byte(stringsForJSON(p))) {
			t.Fatal("execution environment missing path", p)
		}
	}
	if *evidenceFlag != "" {
		raw, _ := json.MarshalIndent(map[string]any{"run_ids": ids, "paths": paths, "snapshots": evidence, "revision_before": before.Revision, "revision_after": after.Revision, "native_turns_before_restart": nativeBefore, "native_turns_after_restart": countNative(t, "turn/start"), "raw_execution_calls": string(calls)}, "", "  ")
		if err := os.WriteFile(filepath.Join(*evidenceFlag, "progress-public-evidence.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func stringsForJSON(s string) string { raw, _ := json.Marshal(s); return string(raw[1 : len(raw)-1]) }

// Windows may reject replacement while the destination read handle is open.
// This is producer coordination, with the same complete temporary file; no
// Runner read, lifecycle, delivery deadline or content assertion is retried.
func replaceProgressProducerFile(t *testing.T, from, to string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		err := os.Rename(from, to)
		if err == nil {
			return
		}
		if (!errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32))) || !time.Now().Before(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
