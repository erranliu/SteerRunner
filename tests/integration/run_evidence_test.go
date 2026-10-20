//go:build windows

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRRunEvidenceR2(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	v := waitRun(t, dir, id, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	request.RequestID = "r2-second-run"
	raw, _ = json.Marshal(request)
	other := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	waitRun(t, dir, other, func(v contracts.RunView) bool { return v.Status == "pr_ready" })
	before, err := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	observation := func(wantClean bool) contracts.WorkspaceGit {
		t.Helper()
		response := call(t, dir, []string{"workspace-git", "--run-id", id}, "", 0)
		var out contracts.WorkspaceGit
		raw, _ := json.Marshal(response.Data)
		if json.Unmarshal(raw, &out) != nil || !out.Verified || out.Clean == nil || *out.Clean != wantClean || out.Commit == nil || *out.Commit != git(t, v.Execution.Workspace, "rev-parse", "HEAD") || out.ObservedAt.IsZero() {
			t.Fatal(response)
		}
		return out
	}
	initial := observation(true)
	tracked := filepath.Join(v.Execution.Workspace, "spec.md")
	if err := os.WriteFile(tracked, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	observation(false)
	git(t, v.Execution.Workspace, "restore", "spec.md")
	untracked := filepath.Join(v.Execution.Workspace, "untracked-r2")
	if err := os.WriteFile(untracked, []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	observation(false)
	if err := os.Remove(untracked); err != nil {
		t.Fatal(err)
	}
	git(t, v.Execution.Workspace, "-c", "user.name=Fixture", "-c", "user.email=f@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "R2 current HEAD")
	changed := observation(true)
	if *changed.Commit == *initial.Commit {
		t.Fatal("current Git used cached HEAD")
	}
	reportDir := filepath.Join(v.Execution.Workspace, filepath.FromSlash(contracts.ArtifactDirectory), "execution-001")
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		t.Fatal(err)
	}
	report := []byte("{\"actual\":true,\"结果\":\"程序报告\"}\n")
	log := []byte{'l', 'o', 'g', 0, 255, '\n'}
	for name, data := range map[string][]byte{"report.json": report, "raw.log": log} {
		if err := os.WriteFile(filepath.Join(reportDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	discovered := call(t, dir, []string{"artifacts", "discover", "--run-id", id}, "", 0)
	var discovery contracts.ArtifactDiscovery
	raw, _ = json.Marshal(discovered.Data)
	_ = json.Unmarshal(raw, &discovery)
	if len(discovery.Candidates) != 2 || discovery.RunID != id {
		t.Fatal(discovered)
	}
	collectionInputs := map[string]string{}
	collect := func(requestID, path string, exit int) contracts.Response {
		t.Helper()
		view := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
		input, _ := json.Marshal(contracts.CollectArtifactInput{ExpectedRevision: view.Revision, RelativePath: path})
		raw, _ := json.Marshal(contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: input})
		collectionInputs[requestID] = string(raw)
		response := call(t, dir, []string{"artifacts", "collect", "--run-id", id, "--input", "-"}, string(raw), exit)
		if exit == 0 {
			replay := call(t, dir, []string{"artifacts", "collect", "--run-id", id, "--input", "-"}, string(raw), 0)
			if !reflect.DeepEqual(response, replay) {
				t.Fatal("collect replay changed identity", response, replay)
			}
		}
		return response
	}
	a := collect("r2-report", "execution-001/report.json", 0)
	artifactID := a.Data.(map[string]any)["artifact"].(map[string]any)["artifact_id"].(string)
	read := func(artifact string, want []byte) contracts.Response {
		t.Helper()
		r := call(t, dir, []string{"artifacts", "read", "--run-id", id, "--artifact-id", artifact}, "", 0)
		var out contracts.ArtifactContent
		raw, _ := json.Marshal(r.Data)
		_ = json.Unmarshal(raw, &out)
		data, err := base64.StdEncoding.DecodeString(out.ContentBase64)
		if err != nil || !bytes.Equal(data, want) || out.RunID != id || out.ArtifactID != artifact || out.SizeBytes != len(want) || out.SHA256 != fmt.Sprintf("%x", sha256.Sum256(want)) {
			t.Fatal(r)
		}
		return r
	}
	original := read(artifactID, report)
	if !reflect.DeepEqual(original, read(artifactID, report)) {
		t.Fatal("repeat read changed")
	}
	logReceipt := collect("r2-log", "execution-001/raw.log", 0)
	logID := logReceipt.Data.(map[string]any)["artifact"].(map[string]any)["artifact_id"].(string)
	read(logID, log)
	newer := []byte("{\"new\":true}")
	if err := os.WriteFile(filepath.Join(reportDir, "report.json"), newer, 0600); err != nil {
		t.Fatal(err)
	}
	b := collect("r2-new-report", "execution-001/report.json", 0)
	newID := b.Data.(map[string]any)["artifact"].(map[string]any)["artifact_id"].(string)
	if artifactID == newID {
		t.Fatal("new collection overwrote original")
	}
	read(newID, newer)
	read(artifactID, report)
	listed := call(t, dir, []string{"artifacts", "list", "--run-id", id}, "", 0)
	if len(listed.Data.(map[string]any)["artifacts"].([]any)) != 3 {
		t.Fatal(listed)
	}
	isolated := call(t, dir, []string{"artifacts", "list", "--run-id", other}, "", 0)
	if len(isolated.Data.(map[string]any)["artifacts"].([]any)) != 0 {
		t.Fatal(isolated)
	}
	call(t, dir, []string{"artifacts", "read", "--run-id", other, "--artifact-id", artifactID}, "", 5)
	call(t, dir, []string{"artifacts", "read", "--run-id", id, "--artifact-id", "unknown"}, "", 5)
	for i, path := range []string{"missing.json", "../spec.md", "C:/Windows/win.ini", "execution-001/raw.log:stream"} {
		exit := 2
		if i == 0 {
			exit = 5
		}
		response := collect("r2-reject-"+string(rune('a'+i)), path, exit)
		if response.OK {
			t.Fatal(response)
		}
	}
	if err := os.WriteFile(filepath.Join(reportDir, "large"), make([]byte, contracts.MaxArtifactBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	collect("r2-too-large", "execution-001/large", 2)
	call(t, dir, []string{"artifacts", "discover", "--run-id", id}, "", 2)
	after, err := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("evidence operations invoked Agent or resumed/replayed native work", err)
	}
	// Cross the actual CLI/IPC/service boundary after the producer file is
	// gone. The original accepted collection must replay from durable bytes.
	if err := os.Remove(filepath.Join(reportDir, "report.json")); err != nil {
		t.Fatal(err)
	}
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("r2-service-detach", info.InstanceID), 0)
	if err := waitOwnedService(info); err != nil {
		t.Fatal(err)
	}
	info = serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	replayed := call(t, dir, []string{"artifacts", "collect", "--run-id", id, "--input", "-"}, collectionInputs["r2-report"], 0)
	if !reflect.DeepEqual(a, replayed) || !reflect.DeepEqual(original, read(artifactID, report)) {
		t.Fatal("service generation changed original immutable report or collection receipt")
	}
	after, err = os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("report recovery dispatched new native work", err)
	}
	view := runView(t, call(t, dir, []string{"inspect", "--run-id", id}, "", 0))
	writeCommand(t, dir, "stop", id, "r2-stop", view.Revision)
	waitRun(t, dir, id, func(v contracts.RunView) bool { return v.StopStatus == "stopped" })
	read(artifactID, report)
	// This is the isolated fixture's exact owned workspace, not a product cleanup API.
	common := git(t, v.Execution.Workspace, "rev-parse", "--path-format=absolute", "--git-common-dir")
	git(t, filepath.Dir(common), "worktree", "remove", "--force", v.Execution.Workspace)
	unavailable := call(t, dir, []string{"workspace-git", "--run-id", id}, "", 0).Data.(map[string]any)
	if unavailable["verified"] != false || unavailable["clean"] != nil || unavailable["commit"] != nil || unavailable["diagnostic"] == "" {
		t.Fatal(unavailable)
	}
	if !reflect.DeepEqual(original, read(artifactID, report)) {
		t.Fatal("cleanup changed evidence")
	}
	call(t, dir, []string{"artifacts", "discover", "--run-id", id}, "", 4)
	if *evidenceFlag != "" {
		raw, _ := json.MarshalIndent(map[string]any{"first_commit": initial, "changed_commit": changed, "discover": discovered, "collect": a, "read": original, "list": listed, "unavailable": unavailable, "zero_agent_calls_for_evidence": true}, "", "  ")
		if err := os.WriteFile(filepath.Join(*evidenceFlag, "r2-public-evidence.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
