//go:build windows && live

package native

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var manifestFlag = flag.String("native-manifest", "", "Explicit generated native manifest")
var evidenceFlag = flag.String("evidence-dir", "", "Fresh native harness evidence directory")
var scenarios atomic.Int64

func TestMain(m *testing.M) {
	flag.Parse()
	if *evidenceFlag != "" {
		path, err := filepath.EvalSymlinks(*evidenceFlag)
		if err != nil || !strings.HasPrefix(strings.ToLower(path), `c:\steerlinedebug\`) {
			fmt.Fprintln(os.Stderr, "native evidence must stay in SteerlineDebug")
			os.Exit(2)
		}
		log, err := os.OpenFile(filepath.Join(path, "native.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Stdout, os.Stderr = log, log
		code := m.Run()
		raw, _ := json.Marshal(map[string]any{"exit_code": code, "scenario_count": scenarios.Load(), "finished_at": time.Now().UTC(), "pattern": flag.Lookup("test.run").Value.String()})
		_ = os.WriteFile(filepath.Join(path, "result.json"), raw, 0600)
		_ = log.Close()
		os.Exit(code)
	}
	os.Exit(m.Run())
}

type target struct {
	Execution contracts.Execution `json:"execution"`
}
type manifest struct {
	DebugRoot      string                `json:"debug_root"`
	RunnerExe      string                `json:"runner_exe"`
	RunnerSHA256   string                `json:"runner_sha256"`
	RepositoryPath string                `json:"repository_path"`
	ProjectRunDir  string                `json:"project_run_dir"`
	BaseBranch     string                `json:"base_branch"`
	BaseRevision   string                `json:"base_revision"`
	Publication    contracts.Publication `json:"publication"`
	Targets        map[string]target     `json:"targets"`
}

func load(t *testing.T) manifest {
	t.Helper()
	path := os.Getenv("STEERRUNNER_NATIVE_MANIFEST")
	if *manifestFlag != "" {
		path = *manifestFlag
	}
	if path == "" {
		t.Fatal("explicit STEERRUNNER_NATIVE_MANIFEST is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := contracts.DecodeStrict(raw, &m); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(m.DebugRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.ToLower(root), `c:\steerlinedebug\`) {
		t.Fatal("native validation requires a generated SteerlineDebug environment")
	}
	for _, owned := range []string{m.RunnerExe, m.RepositoryPath, m.ProjectRunDir, path} {
		resolved, err := filepath.EvalSymlinks(owned)
		if err != nil || !strings.HasPrefix(strings.ToLower(resolved), strings.ToLower(root)+string(filepath.Separator)) {
			t.Fatal("native input is outside its generated debug package", owned, err)
		}
	}
	var debug struct {
		Status string `json:"status"`
		Base   string `json:"base_branch"`
		Runner struct {
			SHA string `json:"sha256"`
		} `json:"runner"`
	}
	raw, err = os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || json.Unmarshal(raw, &debug) != nil || debug.Status != "ready" || debug.Base != m.BaseBranch || !strings.EqualFold(debug.Runner.SHA, m.RunnerSHA256) {
		t.Fatal("debug package provenance differs from native manifest")
	}
	exe, err := os.ReadFile(m.RunnerExe)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(exe)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), m.RunnerSHA256) {
		t.Fatal("native executable hash changed")
	}
	if m.Publication.Repository != "erranliu/SteerlineTest" || m.Publication.BaseBranch != m.BaseBranch || !strings.HasPrefix(m.BaseBranch, "codex/debug/") {
		t.Fatal("native PR must use this package's dedicated SteerlineTest base")
	}
	cmd := exec.Command("git", "-C", m.RepositoryPath, "config", "--get", "remote.origin.url")
	url, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(url)) != "https://github.com/erranliu/SteerlineTest.git" {
		t.Fatal("native repository remote changed")
	}
	return m
}
func invoke(t *testing.T, m manifest, dir string, args []string, input *contracts.Request) contracts.Response {
	t.Helper()
	argv := append(append([]string{}, args...), "--state-dir", dir, "--json")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.RunnerExe, argv...)
	var body []byte
	if input != nil {
		body, _ = json.Marshal(input)
		cmd.Stdin = bytes.NewReader(body)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	entry, _ := json.Marshal(map[string]any{"at": time.Now().UTC(), "argv": argv, "request": json.RawMessage(body), "response": json.RawMessage(out), "stderr": stderr.String()})
	f, e := os.OpenFile(filepath.Join(m.DebugRoot, "evidence", "runner-native-calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		_, _ = f.Write(append(entry, '\n'))
		_ = f.Close()
	}
	var response contracts.Response
	if json.Unmarshal(out, &response) != nil {
		t.Fatalf("CLI response unavailable: %v %s", err, stderr.String())
	}
	if err != nil || !response.OK {
		t.Fatalf("CLI %v: %v %+v", args, err, response.Error)
	}
	return response
}
func run(t *testing.T, key string) {
	scenarios.Add(1)
	m := load(t)
	selected, ok := m.Targets[key]
	if !ok {
		t.Fatalf("manifest target %s is required", key)
	}
	dir := filepath.Join(m.ProjectRunDir, "runner", key)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("native state directory must be new; use a new debug package for a new execution plan")
	}
	started := invoke(t, m, dir, []string{"service", "start"}, nil)
	service := started.Data.(map[string]any)
	instance := service["instance_id"].(string)
	t.Cleanup(func() {
		request := contracts.Request{Protocol: contracts.Protocol, RequestID: "native-cleanup-" + key, Input: json.RawMessage(fmt.Sprintf(`{"expected_instance_id":%q}`, instance))}
		invoke(t, m, dir, []string{"service", "stop", "--input", "-"}, &request)
	})
	spec := "Run powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sandbox-check.ps1 using the normal sandbox and login=false; preserve its actual exit code and output. Replace result.txt with exactly two lines: SteerlineTest debug result, then the exact current branch from git branch --show-current, followed by a newline. Verify and commit only result.txt locally. No push or PR until a separate publication input. Publication must invoke powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./publish.ps1 and target this environment's dedicated base."
	plan := "Read the frozen Spec, run its exact sandbox probe, update and verify result.txt and commit locally. The later report only formats the original implementation result. The separate publication input authorizes the fixed publish.ps1 command; use its exact verified PR URL. A sandbox denial may use the existing bounded automatic review route for that same command, preserving any denial."
	if selected.Execution.Adapter == "orca-terminal" {
		spec = "Use the execution host's native permissions. Replace result.txt with exactly two lines: SteerlineTest debug result, then the exact current branch from git branch --show-current, followed by a newline. Verify and commit only result.txt locally. No push or PR until a separate publication input. Publication must invoke powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./publish.ps1 and target this environment's dedicated base. Return actual command results and blockers."
		plan = "Read the attached frozen Spec and Plan, verify this worktree's repository/base/branch, update and verify result.txt and commit locally. The later report only formats the original implementation result. The separate publication input authorizes the fixed publish.ps1 command; use its verified PR URL. Preserve native permission denials or missing credentials as blockers."
	}
	request := contracts.RunRequest{Title: "SteerRunner native " + key, Repository: contracts.Repository{URL: "https://github.com/erranliu/SteerlineTest.git", LocalPath: m.RepositoryPath, Remote: "origin", BaseBranch: m.BaseBranch, BaseRevision: m.BaseRevision}, Documents: contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "native.spec.md", Content: &spec}, Plan: contracts.Document{Kind: "inline", Path: "native.plan.md", Content: &plan}}, Workflow: contracts.Workflow{ID: "development", Version: 1}, Execution: selected.Execution, Publication: m.Publication}
	raw, _ := json.Marshal(request)
	envelope := contracts.Request{Protocol: contracts.Protocol, RequestID: "native-submit-" + key, Input: raw}
	accepted := invoke(t, m, dir, []string{"submit", "--input", "-"}, &envelope)
	id := accepted.Data.(map[string]any)["run_id"].(string)
	t.Cleanup(func() {
		response := invoke(t, m, dir, []string{"inspect", "--run-id", id}, nil)
		run := response.Data.(map[string]any)["run"].(map[string]any)
		body, _ := json.Marshal(map[string]any{"expected_revision": run["revision"]})
		stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "native-run-cleanup-" + key, Input: body}
		invoke(t, m, dir, []string{"stop", "--run-id", id, "--input", "-"}, &stop)
	})
	until := time.Now().Add(12 * time.Minute)
	var view contracts.RunView
	for time.Now().Before(until) {
		response := invoke(t, m, dir, []string{"inspect", "--run-id", id}, nil)
		raw, _ := json.Marshal(response.Data.(map[string]any)["run"])
		view = contracts.RunView{}
		if json.Unmarshal(raw, &view) != nil {
			t.Fatal("invalid Run view")
		}
		if view.Status == "pr_ready" {
			break
		}
		if view.Status == "blocked" || view.Status == "waiting_input" || view.Status == "stopped" || strings.HasPrefix(view.Diagnostic, "original workspace creation has no exact saved creation receipt") {
			_ = os.WriteFile(filepath.Join(m.DebugRoot, "evidence", "runner-native-"+key+".json"), raw, 0600)
			t.Fatalf("native execution blocked: %+v", view)
		}
		time.Sleep(2 * time.Second)
	}
	raw, _ = json.MarshalIndent(view, "", "  ")
	_ = os.WriteFile(filepath.Join(m.DebugRoot, "evidence", "runner-native-"+key+".json"), raw, 0600)
	if view.Status != "pr_ready" || view.PR == nil || view.PR.QueryError != "" || view.PR.Repository != "erranliu/SteerlineTest" || view.PR.BaseBranch != m.BaseBranch || view.PR.HeadBranch != view.Execution.Branch || view.Execution.SessionID == "" || view.Final == nil || !view.Final.Verified || view.Final.Source == "" || view.Final.ItemID == "" {
		t.Fatalf("native PR/final evidence incomplete: %+v", view)
	}
	check := exec.Command("git", "-C", view.Execution.Workspace, "show", "HEAD:result.txt")
	if selected.Execution.Target.Kind == "local" {
		body, err := check.Output()
		if err != nil || strings.ReplaceAll(string(body), "\r\n", "\n") != "SteerlineTest debug result\n"+view.Execution.Branch+"\n" {
			t.Fatalf("native committed result differs: %v %q", err, body)
		}
	}
	t.Logf("native target=%s run=%s PR=%s session=%s final_source=%s", key, id, view.PR.URL, view.Execution.SessionID, view.Final.Source)
}
func TestSRNativeLocalCodex(t *testing.T) { run(t, "local-codex") }
func TestSRNativeOrcaTargets(t *testing.T) {
	for _, name := range []string{"local", "environment"} {
		t.Run(name, func(t *testing.T) { run(t, "orca-"+name) })
	}
}
