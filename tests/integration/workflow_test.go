//go:build windows

package integration

import (
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSRInputFreeze(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	var spec contracts.RunRequest
	_ = json.Unmarshal(request.Input, &spec)
	legacySpec := "Legacy inline content must not be sent to the agent."
	spec.Documents.Spec = contracts.Document{Kind: "inline", Path: "spec.md", Content: &legacySpec}
	spec.Documents.Plan = contracts.Document{Kind: "git", Path: "plan.md", Commit: spec.Repository.BaseRevision}
	request.Input, _ = json.Marshal(spec)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	acceptedRaw := append([]byte(nil), raw...)
	var responses [3]contracts.Response
	var exits [3]int
	var failures [3]error
	var pending sync.WaitGroup
	for i := range responses {
		pending.Add(1)
		go func(i int) {
			defer pending.Done()
			responses[i], exits[i], _, failures[i] = invoke(runnerEXE, []string{"submit", "--state-dir", dir, "--input", "-", "--json"}, string(raw))
		}(i)
	}
	pending.Wait()
	id := ""
	for i, response := range responses {
		if failures[i] != nil || exits[i] != 0 || !response.OK {
			t.Fatalf("concurrent submit %d: %v %d %+v", i, failures[i], exits[i], response)
		}
		current := response.Data.(map[string]any)["run_id"].(string)
		if id != "" && id != current {
			t.Fatal("same request accepted multiple runs")
		}
		id = current
	}
	_ = os.WriteFile(filepath.Join(spec.Repository.LocalPath, "plan.md"), []byte("Changed after acceptance"), 0600)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
	if current.Materials[1].Commit != spec.Repository.BaseRevision {
		t.Fatal("pinned source commit was lost")
	}
	for _, material := range current.Materials {
		if material.Kind != "worktree" || material.ExecutionLocation != material.Path || material.SnapshotPath != "" || material.Digest != "" {
			t.Fatalf("development material was copied or snapshotted: %+v", material)
		}
	}
	calls, err := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	implementationSeen := false
	for _, line := range strings.Split(string(calls), "\n") {
		var call struct{ Method, Prompt string }
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatal(err)
		}
		if call.Method != "turn/start" {
			continue
		}
		if strings.Contains(call.Prompt, legacySpec) || strings.Contains(call.Prompt, "steerrunner-inputs") || strings.Contains(call.Prompt, "prompt:spec.md") {
			t.Fatal("native input contains a document body or material snapshot", call.Prompt)
		}
		implementationSeen = implementationSeen || strings.Contains(call.Prompt, "\nSpec: spec.md\nPlan: plan.md")
	}
	if !implementationSeen {
		t.Fatal("native implementation input lost its worktree document paths")
	}
	spec.Title = "changed request"
	request.Input, _ = json.Marshal(spec)
	raw, _ = json.Marshal(request)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 5)
	if response.Error.Code != "request_conflict" {
		t.Fatal("changed body reused original request ID")
	}
	if countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 3 {
		t.Fatal("concurrent submit replayed native work")
	}
	if err := os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), []byte(`{"bad":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if same := call(t, dir, []string{"submit", "--input", "-"}, string(acceptedRaw), 0); same.Data.(map[string]any)["run_id"] != id {
		t.Fatal("config change re-froze duplicate request")
	}
	request.RequestID = "invalid-parser-config-new-request"
	newRaw, _ := json.Marshal(request)
	if rejected := call(t, dir, []string{"submit", "--input", "-"}, string(newRaw), 2); rejected.Error.Code != "invalid_input" {
		t.Fatal(rejected)
	}
	if countNative(t, "turn/start") != 3 {
		t.Fatal("bad new config dispatched work")
	}

}

func TestSRPermissionQualification(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	t.Setenv("SR_FIXTURE_VERSION", "codex-cli unsupported")
	if err := os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	accepted := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	id := accepted.Data.(map[string]any)["run_id"].(string)
	view := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "blocked" })
	if view.Phase != "permission_resources_unverified" || !view.ResumeRequired || view.NewInputsAllowed || countNative(t, "thread/start") != 0 || countNative(t, "turn/start") != 0 {
		t.Fatalf("unqualified native executable crossed the input boundary: %+v", view)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
func waitRun(t *testing.T, dir, id string, match func(contracts.RunView) bool) contracts.RunView {
	return waitRunFor(t, dir, id, match, 60*time.Second)
}
func waitRunFor(t *testing.T, dir, id string, match func(contracts.RunView) bool, budget time.Duration) contracts.RunView {
	t.Helper()
	until := time.Now().Add(budget)
	var current contracts.RunView
	for time.Now().Before(until) {
		response := call(t, dir, []string{"inspect", "--run-id", id}, "", 0)
		raw, _ := json.Marshal(response.Data.(map[string]any)["run"])
		if err := json.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
		if match(current) {
			return current
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Run did not reach expected state: %+v", current)
	return current
}
func setupRun(t *testing.T) (string, contracts.Request) {
	t.Helper()
	// Local receive-pack on Git for Windows has a short GIT_DIR buffer. Keep
	// this controlled bare-remote fixture independent of the Go test name.
	root, err := os.MkdirTemp(os.TempDir(), "sr-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(root) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(root), "sr-") {
			t.Error("refuse cleanup outside the fixture's temporary root")
			return
		}
		if t.Failed() {
			// Keep original journals and resource identities when settlement or
			// an assertion failed. Removing them can strand a still-live owner
			// and destroys the evidence needed for exact original cleanup.
			t.Logf("preserved failed fixture: %s", root)
			return
		}
		// Windows can retain a just-exited image briefly (including scanning).
		// Native owner exit is checked separately; never suppress a live owner.
		until := time.Now().Add(3 * time.Second)
		for {
			err := os.RemoveAll(root)
			if err == nil {
				break
			}
			if time.Now().After(until) {
				t.Error(err)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	config := filepath.Join(root, "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	source := filepath.Join(root, "source")
	bare := filepath.Join(root, "remote.git")
	native := filepath.Join(root, "native")
	_ = os.MkdirAll(source, 0700)
	_ = os.MkdirAll(native, 0700)
	git(t, root, "init", "--bare", bare)
	git(t, source, "init", "-b", "main")
	_ = os.WriteFile(filepath.Join(source, "spec.md"), []byte("Spec: controlled task"), 0600)
	_ = os.WriteFile(filepath.Join(source, "plan.md"), []byte("Plan: controlled task"), 0600)
	git(t, source, "add", ".")
	git(t, source, "-c", "user.name=Runner Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "fixture")
	revision := git(t, source, "rev-parse", "HEAD")
	git(t, root, "config", "--global", "url."+filepath.ToSlash(bare)+".insteadOf", "https://controlled.invalid/owner/repo.git")
	git(t, source, "remote", "add", "origin", "https://controlled.invalid/owner/repo.git")
	git(t, source, "push", "origin", "main")
	provider := filepath.Join(root, "provider.json")
	t.Setenv("STEERRUNNER_FIXTURE_STATE", native)
	t.Setenv("CODEX_HOME", native)
	t.Setenv("STEERRUNNER_FIXTURE_PROVIDER", provider)
	t.Cleanup(func() {
		if *evidenceFlag == "" {
			return
		}
		if body, err := os.ReadFile(filepath.Join(native, "calls.jsonl")); err == nil {
			_ = os.WriteFile(filepath.Join(*evidenceFlag, strings.ReplaceAll(t.Name(), "/", "-")+"-native-calls.jsonl"), body, 0600)
		}
	})
	request := contracts.RunRequest{Title: "独立Runner controlled", Repository: contracts.Repository{URL: "https://controlled.invalid/owner/repo.git", LocalPath: source, Remote: "origin", BaseBranch: "main", BaseRevision: revision}, Documents: contracts.Documents{Spec: contracts.Document{Path: "spec.md"}, Plan: contracts.Document{Path: "plan.md"}}, Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{}`)}, Publication: contracts.Publication{Provider: "command", Repository: "owner/repo", BaseBranch: "main"}}
	request.Execution.Settings, _ = json.Marshal(contracts.CodexSettings{Executable: helperEXE})
	request.Publication.Settings, _ = json.Marshal(contracts.CommandProviderSettings{BaseURL: "https://controlled.invalid", Query: contracts.QuerySettings{Executable: helperEXE, Args: []string{"--query", provider}}})
	raw, _ := json.Marshal(request)
	return filepath.Join(root, "runner"), contracts.Request{Protocol: contracts.Protocol, RequestID: "original-submit", Input: raw}
}
func TestSRRunWorkflow(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	input, _ := json.Marshal(request)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(input), 0)
	id := response.Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
	if current.PR == nil || current.PR.HeadBranch != current.Execution.Branch || current.Execution.SessionID == "" || len(current.Materials) != 2 {
		t.Fatalf("incomplete public view: %+v", current)
	}
	var observed time.Time
	for i := 0; i < 2; i++ {
		response := call(t, dir, []string{"last-message", "--run-id", id}, "", 0)
		raw, _ := json.Marshal(response.Data)
		var view contracts.LastMessageView
		if err := json.Unmarshal(raw, &view); err != nil || view.RunID != id || view.AttemptID != current.Execution.AttemptID || view.Message == nil || view.Message.Text == "" || view.Message.SessionID != current.Execution.SessionID || view.Message.ObservedAt.IsZero() {
			t.Fatal("public message missing or wrong identity", string(raw), err)
		}
		if i == 0 {
			observed = view.Message.ObservedAt
		} else if !observed.Equal(view.Message.ObservedAt) {
			t.Fatal("query advanced observation time")
		}
	}
	unchanged := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
	if unchanged.Revision != current.Revision || countNative(t, "turn/start") != 3 {
		t.Fatal("read query mutated execution")
	}
	changes := call(t, dir, []string{"events", "--run-id", id}, "", 0)
	changesRaw, _ := json.Marshal(changes.Data)
	var facts contracts.EventPage
	_ = json.Unmarshal(changesRaw, &facts)
	readable := 0
	for _, event := range facts.Events {
		if event.Transition != nil {
			readable++
			if event.Transition.To.Stage == "" || event.Transition.To.Status == "" {
				t.Fatal("missing workflow state")
			}
		}
	}
	if readable < 3 {
		t.Fatal("workflow changes missing from CLI events")
	}
	retry := call(t, dir, []string{"submit", "--input", "-"}, string(input), 0)
	if retry.Data.(map[string]any)["run_id"] != id {
		t.Fatal("retry created another run")
	}
	page := call(t, dir, []string{"events", "--run-id", id, "--limit", "2"}, "", 0)
	if len(page.Data.(map[string]any)["events"].([]any)) != 2 || !page.Data.(map[string]any)["has_more"].(bool) {
		t.Fatal("event pagination missing")
	}
	feedback := contracts.Request{Protocol: contracts.Protocol, RequestID: "feedback-once", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d,"kind":"feedback","text":"Update the original result"}`, current.Revision))}
	body, _ := json.Marshal(feedback)
	response = call(t, dir, []string{"reply", "--run-id", id, "--input", "-"}, string(body), 5)
	if response.Error.Code != "invalid_state" || countNative(t, "turn/start") != 3 {
		t.Fatal("milestone acquired a feedback transition")
	}

}

func TestSRStorageLogsEvents(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	_ = os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "missing-final"), []byte("no final item"), 0600)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Phase == "final_unavailable" })
	if current.Execution.FinalAvailable || current.Execution.NativeOutcome != "succeeded" {
		t.Fatal("native outcome or missing final was guessed", current.Execution)
	}
	time.Sleep(1200 * time.Millisecond)
	if countNative(t, "turn/start") != 1 {
		t.Fatal("missing original final advanced workflow")
	}
	if _, err := os.Stat(filepath.Join(dir, "logs", "service.log")); err != nil {
		t.Fatal(err)
	}
	first := call(t, dir, []string{"events", "--run-id", id, "--limit", "1"}, "", 0).Data.(map[string]any)
	second := call(t, dir, []string{"events", "--run-id", id, "--limit", "1", "--after", first["next_cursor"].(string)}, "", 0).Data.(map[string]any)
	if first["events"].([]any)[0].(map[string]any)["event_id"] == second["events"].([]any)[0].(map[string]any)["event_id"] {
		t.Fatal("cursor replayed previous event")
	}
	repeat := call(t, dir, []string{"events", "--run-id", id, "--limit", "1"}, "", 0).Data.(map[string]any)
	if first["events"].([]any)[0].(map[string]any)["event_id"] != repeat["events"].([]any)[0].(map[string]any)["event_id"] {
		t.Fatal("event identity changed")
	}
}

func TestSRPublicationIdentity(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	t.Setenv("STEERRUNNER_STAGE_PUBLISH_PLAIN", "1")
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
	if countNative(t, "turn/start") != 4 {
		t.Fatal("publication formatting did not use one source-bound correction")
	}
	provider := os.Getenv("STEERRUNNER_FIXTURE_PROVIDER")
	body, _ := os.ReadFile(provider)
	var fact map[string]any
	_ = json.Unmarshal(body, &fact)
	fact["head_branch"] = "foreign-branch"
	wrong, _ := json.Marshal(fact)
	_ = os.WriteFile(provider, wrong, 0600)
	response := call(t, dir, []string{"inspect", "--run-id", id, "--refresh-pr"}, "", 0)
	var view struct {
		Run contracts.RunView `json:"run"`
	}
	raw, _ = json.Marshal(response.Data)
	_ = json.Unmarshal(raw, &view)
	if view.Run.PR == nil || view.Run.PR.QueryError == "" || view.Run.PR.HeadBranch != current.PR.HeadBranch || !view.Run.PR.CheckedAt.Equal(current.PR.CheckedAt) {
		t.Fatal("foreign PR refresh replaced bound fact", view.Run.PR)
	}
	_ = os.WriteFile(provider, body, 0600)
	response = call(t, dir, []string{"inspect", "--run-id", id, "--refresh-pr"}, "", 0)
	raw, _ = json.Marshal(response.Data)
	view.Run = contracts.RunView{}
	_ = json.Unmarshal(raw, &view)
	if view.Run.PR.QueryError != "" || view.Run.PR.URL != current.PR.URL {
		t.Fatal("original PR did not refresh")
	}
	if countNative(t, "turn/start") != 4 {
		t.Fatal("read-only PR refresh dispatched another input")
	}
}

func TestSRPublicationBeforeWorkflowReady(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	t.Setenv("STEERRUNNER_STAGE_PUBLISH_UNKNOWN", "1")
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool {
		return r.PR != nil && r.Step == "publish.execute" && r.Status == "reconciling"
	})
	if current.Status == "pr_ready" {
		t.Fatal("fixture did not retain incomplete workflow")
	}
	provider := os.Getenv("STEERRUNNER_FIXTURE_PROVIDER")
	body, _ := os.ReadFile(provider)
	var fact map[string]any
	_ = json.Unmarshal(body, &fact)
	fact["state"], fact["merged"], fact["merged_at"], fact["merge_commit"] = "merged", true, time.Now().UTC().Format(time.RFC3339Nano), current.PR.HeadRevision
	body, _ = json.Marshal(fact)
	_ = os.WriteFile(provider, body, 0600)
	response := call(t, dir, []string{"inspect", "--run-id", id, "--refresh-pr"}, "", 0)
	var refreshed struct {
		Run contracts.RunView `json:"run"`
	}
	raw, _ = json.Marshal(response.Data)
	_ = json.Unmarshal(raw, &refreshed)
	if refreshed.Run.PR == nil || !refreshed.Run.PR.Merged || refreshed.Run.Status == "pr_ready" {
		t.Fatal("merged PR was gated on workflow readiness", refreshed.Run)
	}
	writeCommand(t, dir, "stop", id, "terminal-stop", refreshed.Run.Revision)
	stopped := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" })
	if stopped.NewInputsAllowed {
		t.Fatal("terminal stop did not revoke input")
	}
	queries, turns := countNative(t, "provider/query"), countNative(t, "turn/start")
	time.Sleep(1500 * time.Millisecond)
	if countNative(t, "provider/query") != queries || countNative(t, "turn/start") != turns {
		t.Fatal("terminal stop continued provider queries or native inputs")
	}
}

func countNative(t *testing.T, method string) int {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			t.Fatal("invalid native evidence")
		}
		if entry["method"] == method {
			count++
		}
	}
	return count
}
func writeCommand(t *testing.T, dir, command, id, requestID string, revision uint64) contracts.Response {
	t.Helper()
	request := contracts.Request{Protocol: contracts.Protocol, RequestID: requestID, Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, revision))}
	raw, _ := json.Marshal(request)
	return call(t, dir, []string{command, "--run-id", id, "--input", "-"}, string(raw), 0)
}
func TestSRNativeUnknownAndStop(t *testing.T) {
	testCount.Add(1)
	dir, request := setupRun(t)
	_ = os.WriteFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "drop-next-start"), []byte("drop exactly one native receipt"), 0600)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	response := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0)
	id := response.Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool {
		return r.Status == "reconciling" && r.Phase == "operation_outcome_unknown"
	})
	first := countNative(t, "turn/start")
	if first != 1 {
		t.Fatal("missing original dispatch")
	}
	time.Sleep(1500 * time.Millisecond)
	if countNative(t, "turn/start") != first {
		t.Fatal("unknown native input was resent")
	}
	current = waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Revision > 0 })
	writeCommand(t, dir, "stop", id, "revoke-unknown", current.Revision)
	stopped := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" })
	if stopped.NewInputsAllowed {
		t.Fatal("stop did not revoke input authorization")
	}
	time.Sleep(1100 * time.Millisecond)
	if countNative(t, "turn/start") != 1 {
		t.Fatal("stop generated a later input")
	}
}
func TestSRRestartPreservesAuthorization(t *testing.T) {
	testCount.Add(1)
	selectedRunner := runnerEXE
	defer func() { runnerEXE = selectedRunner }()
	if *baselineFlag != "" {
		runnerEXE = *baselineFlag
	}
	originalRunner := runnerEXE
	candidate := buildCompatibleCandidate(t)
	for _, mode := range []string{"graceful", "kill"} {
		t.Run(mode, func(t *testing.T) {
			dir, request := setupRun(t)
			t.Setenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER", "1")
			var input contracts.RunRequest
			json.Unmarshal(request.Input, &input)
			input.Publication.Settings, _ = json.Marshal(contracts.CommandProviderSettings{BaseURL: "https://controlled.invalid", Query: contracts.QuerySettings{Executable: helperEXE, Args: []string{"--query", os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "{pr_url}"}}})
			request.Input, _ = json.Marshal(input)
			hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")
			if err := os.WriteFile(hold, []byte("complete while Runner is offline"), 0600); err != nil {
				t.Fatal(err)
			}
			info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, info)
			raw, _ := json.Marshal(request)
			id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
			active := waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.Execution.TurnStarted && r.Execution.TurnID != "" }, 5*time.Minute)
			var owner struct {
				Identity struct{ Process b.ProcessIdentity }
			}
			body, err := os.ReadFile(filepath.Join(dir, "runs", id, "app-server", "owner", "owner-ready.json"))
			if err != nil {
				t.Fatal(err)
			}
			if json.Unmarshal(body, &owner) != nil {
				t.Fatal("invalid owner identity")
			}
			var native codex.ProcessReceipt
			body, err = os.ReadFile(filepath.Join(dir, "runs", id, "app-server", "process.json"))
			if err != nil {
				t.Fatal(err)
			}
			json.Unmarshal(body, &native)
			if mode == "graceful" {
				call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("service-stop", info.InstanceID), 0)
				err = waitOwnedService(info)
			} else {
				err = killOwnedService(info)
			}
			if err != nil {
				t.Fatal(err)
			}
			if process.InspectProcess(&owner.Identity.Process) != "matching process still exists" || process.InspectProcess(&native.Identity) != "matching process still exists" {
				t.Fatal("service exit ended the original owner/native tree")
			}
			if err = os.Remove(hold); err != nil {
				t.Fatal(err)
			}
			// Completion publication follows the original owner's bounded Git
			// checkpoint, which may take 30 seconds before the journal is final.
			deadline := time.Now().Add(time.Minute)
			completed := false
			for time.Now().Before(deadline) {
				files, _ := filepath.Glob(filepath.Join(dir, "runs", id, "turns", "*", "appserver.jsonl"))
				for _, file := range files {
					outcome, e := codex.ReadAppServerOutcome(file, active.Execution.SessionID, active.Execution.TurnID)
					if e == nil && outcome.Completed && outcome.FinalVerified {
						completed = true
					}
				}
				if completed {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if !completed || countNative(t, "turn/start") != 1 {
				t.Fatal("offline owner did not save the original result, or sent a successor without the service")
			}
			if mode == "graceful" {
				runnerEXE = candidate
				t.Cleanup(func() { runnerEXE = originalRunner })
			}
			preflight := call(t, dir, []string{"service", "verify-takeover", "--candidate", runnerEXE}, "", 0)
			body, _ = json.Marshal(preflight.Data)
			var report contracts.TakeoverReport
			json.Unmarshal(body, &report)
			if !report.Compatible {
				t.Fatalf("same-contract takeover rejected: %+v", report)
			}
			next := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			t.Logf("original=%+v candidate=%+v owner=%+v native=%+v", info, next, owner.Identity.Process, native.Identity)
			if mode == "graceful" && (next.ExecutableSHA256 == info.ExecutableSHA256 || next.ResourceDigest == info.ResourceDigest || next.Version == info.Version) {
				t.Fatal("candidate did not change build, resource digest and product version")
			}
			cleanupService(t, dir, next)
			ready := waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" }, 5*time.Minute)
			if ready.ResumeRequired || !ready.NewInputsAllowed || ready.Execution.SessionID != active.Execution.SessionID || countNative(t, "turn/start") != 3 || countNative(t, "thread/start") != 1 {
				t.Fatal("takeover required resume, replayed an input or replaced the original session")
			}
			revokeWithFreshRevision(t, dir, id, ready.Revision)
			stopped := waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" }, 5*time.Minute)
			call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("after-explicit-stop", next.InstanceID), 0)
			if err = waitOwnedService(next); err != nil {
				t.Fatal(err)
			}
			last := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
			cleanupService(t, dir, last)
			paused := waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" }, 5*time.Minute)
			if paused.NewInputsAllowed || !paused.ResumeRequired || paused.Revision != stopped.Revision || countNative(t, "turn/start") != 3 {
				t.Fatal("restart changed explicit stopped state or dispatched another input")
			}
		})
	}
	t.Run("creation_receipt", testGenerationCreationReceipt)
	t.Run("pending_stop", testGenerationPendingStop)
}
