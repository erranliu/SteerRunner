package codexworker

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

var authenticationNative = flag.String("sandbox-auth-native", "", "explicit Codex 0.159.2 tool-boundary diagnostic; no model or real credential")
var authenticationHostBroker = flag.Bool("sandbox-auth-host-broker", false, "execution-host login through repository broker; read-only actual sandbox tools, no model")

func TestAuthenticationNativeToolBoundary(t *testing.T) {
	if *authenticationNative == "" {
		t.Skip("explicit native sandbox diagnostic")
	}
	out, err := exec.Command(*authenticationNative, "--version").Output()
	if err != nil || VerifyPolicyVersion(string(out)) != nil {
		t.Fatal("wrong diagnostic Codex version")
	}
	root, err := os.MkdirTemp("C:/srt", "auth-boundary-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved native diagnostic: %s", root)
	// Match the installed Runner's 59-byte state root and 26-byte cache key.
	// The former objects/.sr-auth-*.sock layout is 124 bytes in this fixture.
	state := filepath.Join(root, strings.Repeat("x", 58-len(root)))
	repo := filepath.Join(state, "r", strings.Repeat("r", 26))
	work := filepath.Join(root, "worker")
	server := filepath.Join(root, "server")
	evidence := filepath.Join(root, "evidence")
	for _, dir := range []string{repo, server, evidence} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git fixture: %v %s", err, out)
		}
	}
	git("init", "-b", "main")
	if err = os.WriteFile(filepath.Join(repo, "sample"), []byte("probe"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Diagnostic", "-c", "user.email=diagnostic@example.invalid", "commit", "-m", "sandbox boundary fixture")
	git("remote", "add", "origin", "https://github.com/erranliu/SteerlineTest.git")
	git("worktree", "add", "-b", "diagnostic", work)
	roots, err := LinkedWorktreeWriteArgs(filepath.Join(repo, ".git", "objects"), filepath.Join(repo, ".git", "refs"), filepath.Join(repo, ".git", "logs"), filepath.Join(repo, ".git", "worktrees", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := prepareWorkerRuntime(work, roots)
	if err != nil {
		t.Fatal(err)
	}
	a := testAuthentication(t)
	if *authenticationHostBroker {
		a, err = newAuthentication(nil, "https://github.com/erranliu/SteerlineTest.git")
		if err != nil {
			t.Fatal(err)
		}
	}
	r.Auth = a
	revocation := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/installation/token" {
			t.Error("unexpected controlled cleanup")
		}
		w.WriteHeader(204)
	}))
	t.Cleanup(revocation.Close)
	a.endpoint = revocation.URL
	host := process.New(30 * time.Second)
	canary := a.token
	backend := Backend{Config: Config{Executable: *authenticationNative, Workspace: work, RunDir: root, RunID: strings.Repeat("a", 32), ServerDir: server, Sandbox: "workspace-write", WindowsSandbox: "mxc", WorkerRuntime: true, Authentication: a, Environment: func(env []string) []string { return env }}, Process: ProcessPort{Start: host.StartManaged, Inspect: host.InspectProcess, NewGeneration: func() string { return "diagnostic-generation" }, WriteAtomic: func(path string, data []byte) error { return os.WriteFile(path, data, 0600) }}}
	if *authenticationHostBroker {
		backend.Config.AuthenticationTransport, err = PlanAuthenticationTransport(state, work, a.remoteURL, PolicyResourceIdentity{ProjectID: repo, ExecutionID: backend.Config.RunID, AttemptID: "diagnostic-attempt", Generation: "diagnostic-generation", ConfigDigest: "diagnostic-config"})
		if err != nil {
			t.Fatal(err)
		}
		if len(filepath.Join(repo, ".git", "objects", ".sr-auth-12345678.sock")) != 124 || len(filepath.Join(backend.Config.AuthenticationTransport.Directory, "s")) != 90 {
			t.Fatal("wrong long-path fixture")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	h, binding, probeErr := backend.PrepareSession(ctx, Binding{}, b.TurnInput{CommandID: strings.Repeat("b", 32), Prompt: "business input must not be sent", LinkedWorktreeArgs: roots})
	if h == nil || binding.Process == nil {
		t.Fatalf("failed preparation lost owned process handle: %v", probeErr)
	}
	handle := h.process
	h.owned = false
	t.Cleanup(func() {
		h.Close()
		defer handle.Close()
		handle.Kill()
		a.Close()
		identity := handle.Identity()
		deadline := time.Now().Add(10 * time.Second)
		for host.InspectProcess(&identity) != "process absent" && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		mode := "controlled-invalid-canary"
		if *authenticationHostBroker {
			mode = "host-gh-repository-broker"
		}
		record, _ := json.Marshal(map[string]any{"process": identity, "process_state": host.InspectProcess(&identity), "revocation_confirmed": a.Close(), "business_inputs": 0, "credential": mode})
		_ = os.WriteFile(filepath.Join(evidence, "cleanup.json"), record, 0600)
		if !a.Close() {
			t.Error("controlled token revocation unverified")
		}
		if p := backend.Config.AuthenticationTransport; p != nil {
			if _, err := os.Lstat(p.Directory); !os.IsNotExist(err) {
				t.Error("authentication socket directory remains", err)
			}
		}
		if host.InspectProcess(&identity) != "process absent" {
			t.Error("native process exit unverified")
		}
	})
	if !*authenticationHostBroker && (!isPreparationBlocked(probeErr) || !strings.Contains(probeErr.Error(), "github_identity") || binding.SessionID != "") {
		t.Fatalf("invalid canary must reach GitHub identity only after actual sandbox/environment/Git/network gates, with no thread/input: %v", probeErr)
	}
	if *authenticationHostBroker {
		if probeErr != nil || binding.SessionID == "" {
			t.Fatalf("real broker readiness: %v", probeErr)
		}
		var result struct {
			ExitCode int    `json:"exitCode"`
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
		}
		toolRoots, _ := WritableRoots(backend.toolWriteArgs(roots))
		policy, _ := TurnStartParams(work, "probe", "probe", "probe", "workspace-write", "", "", toolRoots)
		// The additional grant must not permit writes to the shared parent.
		outside := filepath.Join(filepath.Dir(backend.Config.AuthenticationTransport.Directory), "outside-canary")
		err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{r.Shell, "-NoProfile", "-NonInteractive", "-Command", `[IO.File]::WriteAllText(` + psLiteral(outside) + `,'unexpected')`}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 20000}, &result)
		if _, statErr := os.Stat(outside); err != nil || result.ExitCode == 0 || !os.IsNotExist(statErr) {
			t.Fatal("authentication parent boundary not enforced", err, result.ExitCode, statErr)
		}
		// Query only. No worker business input, Git push, or PR mutation occurs.
		err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{r.Shell, "-NoProfile", "-NonInteractive", "-Command", `gh pr list --repo erranliu/SteerlineTest --head sandbox-readiness-nonexistent --state all --json url,state`}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 20000}, &result)
		if err != nil || result.ExitCode != 0 {
			t.Errorf("actual gh GraphQL query failed: exit=%d error=%v stderr=%s", result.ExitCode, err, result.Stderr)
		}
		_ = os.WriteFile(filepath.Join(evidence, "graphql-query.json"), encode(map[string]any{"exit_code": result.ExitCode, "read_only": true, "stdout": result.Stdout}), 0600)
		// Explicit --head makes gh skip Git pushing; --dry-run omits the PR
		// mutation. Exercise its real repository/feature/existing-PR queries.
		err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{r.Shell, "-NoProfile", "-NonInteractive", "-Command", `gh pr create --repo erranliu/SteerlineTest --head sandbox-readiness-nonexistent --base main --title 'Read-only query compatibility' --body 'No publication: explicit head and dry run' --dry-run`}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 20000}, &result)
		if err != nil || result.ExitCode != 0 {
			t.Errorf("actual gh create dry-run queries failed: exit=%d error=%v stderr=%s", result.ExitCode, err, result.Stderr)
		}
		_ = os.WriteFile(filepath.Join(evidence, "graphql-create-dry-run.json"), encode(map[string]any{"exit_code": result.ExitCode, "read_only": true, "stdout": result.Stdout}), 0600)
	} else if _, err := os.Stat(filepath.Join(server, "thread.json")); !os.IsNotExist(err) {
		t.Error("failed readiness created native business thread")
	}
	// Never count this controlled invalid token as real authenticated publication.
	for _, dir := range []string{server, filepath.Join(root, "runs"), r.Temp} {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				body, err := os.ReadFile(path)
				if err == nil && canary != "" && strings.Contains(string(body), canary) {
					t.Error("canary entered persisted native output")
				}
			}
			return nil
		})
	}
}
