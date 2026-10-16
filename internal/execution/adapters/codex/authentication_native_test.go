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
	repo := filepath.Join(root, "repo")
	work := filepath.Join(root, "worker")
	server := filepath.Join(root, "server")
	evidence := filepath.Join(root, "evidence")
	for _, dir := range []string{repo, server, evidence} {
		if err = os.Mkdir(dir, 0700); err != nil {
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
		policy, _ := TurnStartParams(work, "probe", "probe", "probe", "workspace-write", "", "", []string{filepath.Join(repo, ".git", "objects"), filepath.Join(repo, ".git", "refs"), filepath.Join(repo, ".git", "logs"), filepath.Join(repo, ".git", "worktrees", "worker")})
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
	// Git 2.55 unconditionally deletes the absent AUTO_MERGE after committing.
	// Its packed-refs cleanup remains denied. Verify the commit's own exit and
	// actual Git state independently; never repair/retry the denied operation.
	{
		policy, _ := TurnStartParams(work, "probe", "probe", "probe", "workspace-write", "", "", []string{filepath.Join(repo, ".git", "objects"), filepath.Join(repo, ".git", "refs"), filepath.Join(repo, ".git", "logs"), filepath.Join(repo, ".git", "worktrees", "worker")})
		script := `$ErrorActionPreference='Continue'
$before=(& git rev-parse HEAD | Out-String).Trim()
[IO.File]::WriteAllText((Join-Path $pwd 'native-commit-probe'),'owned native Git diagnostic')
git add -- native-commit-probe
if($LASTEXITCODE -ne 0){exit 21}
git -c user.name=Diagnostic -c user.email=diagnostic@example.invalid commit -m 'native Git boundary'
$commitCode=$LASTEXITCODE
$head=(& git rev-parse HEAD | Out-String).Trim()
$parent=(& git rev-parse HEAD^ | Out-String).Trim()
$paths=@(& git diff-tree --no-commit-id --name-only -r HEAD)
$clean=[string]::IsNullOrWhiteSpace((& git status --porcelain | Out-String))
$pending=@('AUTO_MERGE','MERGE_HEAD','CHERRY_PICK_HEAD','REVERT_HEAD')|Where-Object{& git rev-parse --verify -q $_ 2>$null | Out-Null;$LASTEXITCODE -eq 0}
$configDenied=$false;$packedLockDenied=$false
try{$f=[IO.File]::Open(` + psLiteral(filepath.Join(repo, ".git", "config")) + `,[IO.FileMode]::Open,[IO.FileAccess]::Write);$f.Dispose()}catch{$configDenied=$true}
try{[IO.File]::WriteAllText(` + psLiteral(filepath.Join(repo, ".git", "packed-refs.lock")) + `,'owned diagnostic')}catch{$packedLockDenied=$true}finally{if(-not $packedLockDenied){[IO.File]::Delete(` + psLiteral(filepath.Join(repo, ".git", "packed-refs.lock")) + `)}}
git -c user.name=Diagnostic -c user.email=diagnostic@example.invalid commit -m 'must not create another commit' 1>$null 2>$null
$emptyCode=$LASTEXITCODE
$unchanged=((& git rev-parse HEAD|Out-String).Trim() -ceq $head)
$verified=$commitCode -eq 0 -and $head -cne $before -and $parent -ceq $before -and $paths.Count -eq 1 -and $paths[0] -ceq 'native-commit-probe' -and $clean -and $pending.Count -eq 0 -and $configDenied -and $packedLockDenied -and $emptyCode -ne 0 -and $unchanged
Write-Output ('SR_NATIVE_GIT_BOUNDARY:'+(@{verified=$verified;commit_exit=$commitCode;empty_commit_exit=$emptyCode;head=$head;parent=$parent;config_denied=$configDenied;packed_lock_denied=$packedLockDenied;clean=$clean;pending_count=$pending.Count}|ConvertTo-Json -Compress))
if(-not $verified){exit 22};exit 0`
		var result struct {
			ExitCode       int
			Stdout, Stderr string
		}
		err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{r.Shell, "-NoProfile", "-NonInteractive", "-Command", script}, "cwd": work, "sandboxPolicy": policy["sandboxPolicy"], "timeoutMs": 20000}, &result)
		marker := ""
		for _, line := range strings.Split(result.Stdout, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "SR_NATIVE_GIT_BOUNDARY:") {
				marker = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "SR_NATIVE_GIT_BOUNDARY:"))
			}
		}
		var proof struct{ Verified bool }
		if err != nil || result.ExitCode != 0 || json.Unmarshal([]byte(marker), &proof) != nil || !proof.Verified {
			t.Errorf("native Git commit boundary failed: exit=%d rpc=%v stdout=%s stderr=%s", result.ExitCode, err, result.Stdout, result.Stderr)
		}
		_ = os.WriteFile(filepath.Join(evidence, "native-git-boundary.json"), encode(map[string]any{"exit_code": result.ExitCode, "proof": json.RawMessage(marker), "packed_cleanup_denial": strings.Contains(result.Stdout+result.Stderr, "packed-refs.lock"), "business_inputs": 0, "external_writes": 0}), 0600)
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
