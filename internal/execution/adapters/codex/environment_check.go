package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

const environmentTool = "steerrunner_environment_probe"

func SupportsEnvironmentRepository(remote string) bool {
	_, err := githubTarget(remote)
	return err == nil && strings.HasSuffix(remote, ".git")
}

type EnvironmentCall struct {
	RequestID json.RawMessage
	CallID    string
	Action    string
	Nonce     string
	Completed bool
}

type EnvironmentSetup struct {
	CheckID, Nonce, Directory, Workspace string
	Input                                contracts.CheckInput
	LinkedWriteArgs                      []string
}

type EnvironmentSession struct {
	setup   EnvironmentSetup
	backend Backend
	handle  *SessionHandle
	input   b.TurnInput
	gate    *checkWriteGate
	turn    string
}

func environmentProbeTool() any {
	return map[string]any{"type": "function", "name": environmentTool, "description": "Execute one fixed environment-check action in the original Worker tool context. Writes are journaled and authorized once by Runner.", "inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "action": map[string]any{"type": "string", "enum": []string{"permissions", "workspace", "temporary", "git_admin", "remote_read", "fetch", "base_push", "commit", "head_push", "pr_create", "pr_query", "pr_close"}}}, "required": []string{"nonce", "action"}}}
}

func NewEnvironmentSession(setup EnvironmentSetup) (*EnvironmentSession, error) {
	var settings contracts.CodexSettings
	if err := contracts.DecodeStrict(setup.Input.Execution.Settings, &settings); err != nil {
		return nil, err
	}
	permissions, err := NativePermissions("windows", "local", "codex", settings.AccessMode)
	if err != nil {
		return nil, err
	}
	if settings.Executable == "" {
		settings.Executable = "codex.exe"
	}
	if settings.ReasoningEffort == "" {
		settings.ReasoningEffort = "medium"
	}
	var config *AuthenticationConfig
	if settings.GitHubAuth != nil {
		config = &AuthenticationConfig{AppID: settings.GitHubAuth.AppID, InstallationID: settings.GitHubAuth.InstallationID, PrivateKeyEnv: settings.GitHubAuth.PrivateKeyEnv}
	}
	auth, err := newAuthentication(config, setup.Input.Repository.URL)
	if err != nil {
		return nil, err
	}
	gate := &checkWriteGate{base: "steerline-check/" + setup.CheckID + "/base", head: "steerline-check/" + setup.CheckID + "/head"}
	auth.checkGate = gate
	host := process.New(30 * time.Second)
	s := &EnvironmentSession{setup: setup, gate: gate}
	s.backend = Backend{Config: Config{Executable: settings.Executable, Sandbox: permissions.Sandbox, WindowsSandbox: permissions.WindowsSandbox, Model: settings.Model, ReasoningEffort: settings.ReasoningEffort, Workspace: setup.Workspace, RunDir: setup.Directory, RunID: setup.CheckID, ServerDir: filepath.Join(setup.Directory, "app-server"), Environment: func(env []string) []string {
		return lifecycleProcessEnvironment(env, setup.CheckID, setup.CheckID, nil)
	}, WorkerRuntime: true, Authentication: auth, RemoteName: setup.Input.Repository.Remote, EnvironmentCheck: true}, Process: ProcessPort{Start: host.StartManaged, Inspect: host.InspectProcess, NewGeneration: host.NewProcessGeneration, WriteAtomic: host.WriteExecutionFile}}
	actions := []string{}
	for _, item := range contracts.CheckManifest(setup.Input.Publication != nil) {
		if item.Required {
			actions = append(actions, item.ID)
		}
	}
	s.input = b.TurnInput{CommandID: contracts.NewID(), Prompt: fmt.Sprintf("Run environment check %s. Nonce: %s. Sequentially call %s once for each action in this exact order: %s. Stop on failure. After the last result, reply DONE. Do not perform business work.", setup.CheckID, setup.Nonce, environmentTool, strings.Join(actions, ", ")), LinkedWorktreeArgs: setup.LinkedWriteArgs}
	return s, nil
}

func (s *EnvironmentSession) Prepare(ctx context.Context) (Binding, error) {
	native, resolveErr := NativeExecutable(s.backend.Config.Executable)
	if resolveErr != nil {
		return Binding{}, errors.New("native_executable_unavailable")
	}
	s.backend.Config.Executable = native
	out, err := process.New(30*time.Second).OutputCommand(ctx, b.LaunchCommand{Executable: s.backend.Config.Executable, Args: []string{"--version"}})
	if err != nil {
		return Binding{}, errors.New("native_executable_unavailable")
	}
	if err = VerifyPolicyVersion(string(out)); err != nil {
		return Binding{}, errors.New("native_version_unsupported")
	}
	if err = os.MkdirAll(s.backend.Config.ServerDir, 0700); err != nil {
		return Binding{}, err
	}
	h, binding, err := s.backend.PrepareSession(ctx, Binding{}, s.input)
	s.handle = h
	if err != nil {
		return binding, err
	}
	// A check requires transport-level, one-use write authorization. This
	// cannot silently fall back to an unrestricted installation credential.
	if s.backend.Config.Authentication.broker == nil {
		return binding, errors.New("environment_check_write_gate_unavailable_for_authentication_source")
	}
	var state struct {
		Thread   struct{ ID, Cwd string } `json:"thread"`
		Approval string                   `json:"approvalPolicy"`
		Reviewer string                   `json:"approvalsReviewer"`
		Sandbox  json.RawMessage          `json:"sandbox"`
	}
	readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readCancel()
	if err = s.handle.client.Call(readCtx, "thread/read", map[string]any{"threadId": binding.SessionID, "includeTurns": false}, &state); err != nil {
		return binding, errors.New("native_thread_identity_unverified")
	}
	if state.Thread.ID != binding.SessionID || !sameWorkspace(state.Thread.Cwd, s.setup.Workspace) {
		return binding, errors.New("native_thread_identity_mismatch")
	}
	return binding, nil
}

func (s *EnvironmentSession) Start(ctx context.Context) (string, error) {
	if s.handle == nil || s.handle.client == nil {
		return "", errors.New("original_session_missing")
	}
	if err := s.backend.verifySandboxReadiness(ctx, s.handle, s.input); err != nil {
		return "", err
	}
	params, err := TurnParams(ctx, TurnParamsInput{Worktree: s.setup.Workspace, ThreadID: s.handle.Binding.SessionID, TurnInput: s.input, Sandbox: s.backend.Config.Sandbox, Model: s.backend.Config.Model, ReasoningEffort: s.backend.Config.ReasoningEffort}, func(context.Context, string) ([]string, error) { return s.setup.LinkedWriteArgs, nil })
	if err != nil {
		return "", err
	}
	turnCtx, turnCancel := context.WithTimeout(ctx, 30*time.Second)
	defer turnCancel()
	turn, err := StartTurn(turnCtx, s.handle.client, params)
	s.turn = turn.Turn.ID
	return s.turn, err
}

func (s *EnvironmentSession) Next(ctx context.Context) (EnvironmentCall, error) {
	for {
		select {
		case <-ctx.Done():
			return EnvironmentCall{}, ctx.Err()
		case event, ok := <-s.handle.client.Notifications():
			if !ok {
				return EnvironmentCall{}, errors.New("original_native_connection_lost")
			}
			if event.Method == "item/tool/call" {
				var p struct {
					ThreadID  string          `json:"threadId"`
					TurnID    string          `json:"turnId"`
					CallID    string          `json:"callId"`
					Tool      string          `json:"tool"`
					Arguments json.RawMessage `json:"arguments"`
				}
				if len(event.ID) == 0 || !json.Valid(event.ID) || json.Unmarshal(event.Params, &p) != nil || p.ThreadID != s.handle.Binding.SessionID || p.TurnID != s.turn || p.CallID == "" || p.Tool != environmentTool {
					return EnvironmentCall{}, errors.New("native_tool_identity_mismatch")
				}
				var args struct {
					Action string `json:"action"`
					Nonce  string `json:"nonce"`
				}
				if contracts.DecodeStrict(p.Arguments, &args) != nil || args.Nonce != s.setup.Nonce {
					return EnvironmentCall{}, errors.New("native_tool_nonce_mismatch")
				}
				return EnvironmentCall{RequestID: event.ID, CallID: p.CallID, Action: args.Action, Nonce: args.Nonce}, nil
			}
			if len(event.ID) > 0 {
				_ = s.handle.client.RejectRequest(event.ID, "Environment checks permit only fixed probes; approval and escalation are disabled.")
				return EnvironmentCall{}, errors.New("unexpected_native_approval_or_tool_request")
			}
			if event.Method == "turn/completed" {
				return decodeEnvironmentCompletion(event.Params, s.handle.Binding.SessionID, s.turn)
			}
		}
	}
}

func (s *EnvironmentSession) Reply(call EnvironmentCall, status string) error {
	return s.handle.client.ReplyRequest(call.RequestID, map[string]any{"contentItems": []map[string]string{{"type": "inputText", "text": status}}, "success": status == "passed"})
}

func (s *EnvironmentSession) exec(ctx context.Context, body string) (map[string]string, error) {
	if s.handle == nil || s.handle.client == nil {
		return nil, errors.New("original_tool_context_missing")
	}
	roots, err := WritableRoots(s.setup.LinkedWriteArgs)
	if err != nil {
		return nil, err
	}
	params, err := TurnStartParams(s.setup.Workspace, "probe", "probe", "probe", s.backend.Config.Sandbox, "", "", roots)
	if err != nil {
		return nil, err
	}
	script := `$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$script:failureStage='probe_failed'
function G { $previous=$ErrorActionPreference; $ErrorActionPreference='Continue'; try { $r=@(& git @args 2>&1); $code=$LASTEXITCODE } finally { $ErrorActionPreference=$previous }; if ($code -ne 0) { if (($r -join ' ') -match 'HTTP 401|HTTP 403|Authentication failed|access denied|Permission denied|not permitted|not authorized|returned error: 403|returned error: 401') { $script:failureStage='authorization_denied' }; throw 'git_failed' }; return @($r | Where-Object { $_ -is [string] }) }
function API { $previous=$ErrorActionPreference; $ErrorActionPreference='Continue'; try { $r=@(& gh api @args 2>&1); $code=$LASTEXITCODE } finally { $ErrorActionPreference=$previous }; if ($code -ne 0) { if (($r -join ' ') -match 'HTTP 401|HTTP 403|not authorized|Permission denied') { $script:failureStage='authorization_denied' }; throw 'provider_failed' }; return ((@($r | Where-Object { $_ -is [string] }) -join [char]10)|ConvertFrom-Json) }
try {
 $facts=@{}
 ` + body + `
 $facts | ConvertTo-Json -Compress
 exit 0
} catch { @{failure_stage=$script:failureStage} | ConvertTo-Json -Compress; exit 1 }`
	command := map[string]any{"command": []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script}, "cwd": s.setup.Workspace, "sandboxPolicy": params["sandboxPolicy"], "timeoutMs": 25000, "outputBytesCap": 8192}
	var result struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = s.handle.client.Call(callCtx, "command/exec", command, &result); err != nil {
		return nil, errors.New("native_probe_outcome_unknown")
	}
	if result.ExitCode != 0 {
		var facts map[string]string
		_ = json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &facts)
		if facts["failure_stage"] == "authorization_denied" {
			return facts, errors.New("native_probe_rejected")
		}
		return facts, errors.New("native_probe_failed")
	}
	var facts map[string]string
	if json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &facts) != nil {
		return nil, errors.New("native_probe_evidence_invalid")
	}
	return facts, nil
}

func (s *EnvironmentSession) Execute(ctx context.Context, action string, r contracts.CheckResources) (map[string]string, error) {
	body, err := s.probeScript(action, r)
	if err != nil {
		return nil, err
	}
	zero := strings.Repeat("0", len(r.BaseSHA))
	switch action {
	case "base_push":
		s.gate.arm(action, r.BaseRef, zero, r.BaseSHA, s.gate.base, s.gate.head, 0)
	case "head_push":
		s.gate.arm(action, r.HeadRef, zero, r.HeadSHA, s.gate.base, s.gate.head, 0)
	case "pr_create", "pr_close":
		s.gate.arm(action, "", "", "", s.gate.base, s.gate.head, r.PRNumber)
	}
	defer s.gate.disarm()
	return s.exec(ctx, body)
}

func (s *EnvironmentSession) probeScript(action string, r contracts.CheckResources) (string, error) {
	q := psLiteral
	remote := q(s.setup.Input.Repository.Remote)
	base := q(r.BaseSHA)
	file := ".steerrunner-check-" + s.setup.Nonce + ".txt"
	content := q(s.setup.Nonce)
	repo := s.backend.Config.Authentication.repository
	api := q("repos/" + repo + "/pulls")
	lookup := `$p=@(API -X GET ` + api + ` -f state=all -f head=` + q(strings.Split(repo, "/")[0]+":"+s.gate.head) + ` -f base=` + q(s.gate.base) + ` -f per_page=100); if ($p.Count -ne 1) { throw 'original_pr_ambiguous' }; $p=$p[0]; `
	prFacts := `if ($p.number -le 0 -or $p.html_url -cne (` + q("https://github.com/"+repo+"/pull/") + `+[string]$p.number) -or (` + strconv.Itoa(r.PRNumber) + ` -gt 0 -and $p.number -ne ` + strconv.Itoa(r.PRNumber) + `) -or $p.base.sha -cne ` + q(r.BaseSHA) + ` -or $p.base.ref -cne ` + q(s.gate.base) + ` -or $p.head.ref -cne ` + q(s.gate.head) + ` -or $p.head.sha -cne ` + q(r.HeadSHA) + ` -or $p.base.repo.full_name -ine ` + q(repo) + ` -or $p.head.repo.full_name -ine ` + q(repo) + ` -or $p.merged_at -or $p.body -cne ` + content + `) { throw 'original_pr_identity_changed' }; $facts['pr_url']=[string]$p.html_url; $facts['pr_number']=[string]$p.number; $facts['pr_closed']=[string]($p.state -eq 'closed')`
	switch action {
	case "permissions":
		want := "1"
		if s.backend.Config.Sandbox == "danger-full-access" {
			want = "0"
		}
		return `Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class SRCheckToken { [DllImport("advapi32.dll", SetLastError=true)] public static extern bool GetTokenInformation(IntPtr h,int c,out int v,int n,out int r); }'; $v=0;$n=0; $id=[Security.Principal.WindowsIdentity]::GetCurrent(); if (-not [SRCheckToken]::GetTokenInformation($id.Token,29,[ref]$v,4,[ref]$n) -or $v -ne ` + want + `) { throw 'wrong_token_boundary' }; $facts['account']=$id.Name; $facts['sandbox']=` + q(s.backend.Config.Sandbox) + `; $facts['approval_policy']='never'`, nil
	case "workspace", "temporary", "git_admin":
		path := q(filepath.Join(s.setup.Workspace, file))
		if action == "temporary" {
			path = "(Join-Path $env:TEMP " + q(file) + ")"
		}
		if action == "git_admin" {
			roots, _ := WritableRoots(s.setup.LinkedWriteArgs)
			paths := []string{}
			for _, root := range roots {
				paths = append(paths, q(filepath.Join(root, file)))
			}
			return `foreach ($path in @(` + strings.Join(paths, ",") + `)) { try { [IO.File]::WriteAllText($path,` + content + `); if ([IO.File]::ReadAllText($path) -cne ` + content + `) { throw 'readback' } } finally { if ([IO.File]::Exists($path)) { [IO.File]::Delete($path) } } }; $facts['read_write']='verified'`, nil
		}
		return `$path=` + path + `; try { [IO.File]::WriteAllText($path,` + content + `); if ([IO.File]::ReadAllText($path) -cne ` + content + `) { throw 'readback' } } finally { if ([IO.File]::Exists($path)) { [IO.File]::Delete($path) } }; $facts['read_write']='verified'`, nil
	case "remote_read":
		return `$r=@(G ls-remote --exit-code ` + remote + ` ` + q("refs/heads/"+s.setup.Input.Repository.BaseBranch) + `); if ($r.Count -ne 1) { throw 'remote_identity' }; $facts['remote_ref']=[string]$r[0]`, nil
	case "fetch":
		tracking := "refs/remotes/" + s.setup.Input.Repository.Remote + "/" + s.setup.Input.Repository.BaseBranch
		return `$null=G fetch --no-tags --no-write-fetch-head -- ` + remote + ` ` + q("+refs/heads/"+s.setup.Input.Repository.BaseBranch+":"+tracking) + `; $null=G cat-file -e (` + base + `+'^{commit}'); $null=G merge-base --is-ancestor ` + base + ` ` + q(tracking) + `; $facts['base_sha']=` + base, nil
	case "base_push":
		return `$null=G push ('--force-with-lease='+` + q(r.BaseRef) + `+':') -- ` + remote + ` (` + base + `+':'+` + q(r.BaseRef) + `); $facts['pushed']='true'`, nil
	case "commit":
		return `if ((G rev-parse HEAD) -cne ` + base + ` -or @(G status --porcelain).Count -ne 0) { throw 'workspace_changed' }; [IO.File]::WriteAllText((Join-Path (Get-Location) ` + q(file) + `),` + content + `); $null=G add -- ` + q(file) + `; $null=G -c user.name=SteerRunner -c user.email=environment-check@steerrunner.invalid -c core.hooksPath=NUL -c commit.gpgsign=false commit -m ` + q("Environment check "+s.setup.CheckID) + `; $facts['head_sha']=[string](G rev-parse HEAD)`, nil
	case "head_push":
		return `$null=G push ('--force-with-lease='+` + q(r.HeadRef) + `+':') -- ` + remote + ` (` + q(r.HeadSHA) + `+':'+` + q(r.HeadRef) + `); $facts['pushed']='true'`, nil
	case "pr_create":
		return `$p=API -X POST ` + api + ` -f title=` + q("Environment check "+s.setup.CheckID) + ` -f body=` + content + ` -f base=` + q(s.gate.base) + ` -f head=` + q(s.gate.head) + ` -F maintainer_can_modify=false; ` + prFacts, nil
	case "pr_query":
		return lookup + prFacts, nil
	case "pr_close":
		return lookup + prFacts + `; if ($p.number -ne ` + strconv.Itoa(r.PRNumber) + `) { throw 'original_pr_changed' }; $p=API -X PATCH ` + q("repos/"+repo+"/pulls/"+strconv.Itoa(r.PRNumber)) + ` -f state=closed; ` + prFacts, nil
	}
	return "", errors.New("unsupported_fixed_probe")
}

// Verify performs a separate owner query, never trusting model text or a
// successful write receipt as proof of the original remote object.
func (s *EnvironmentSession) Verify(ctx context.Context, action string, r contracts.CheckResources) (map[string]string, error) {
	q := psLiteral
	switch action {
	case "base_push", "head_push":
		ref, sha := r.BaseRef, r.BaseSHA
		if action == "head_push" {
			ref, sha = r.HeadRef, r.HeadSHA
		}
		return s.exec(ctx, `$r=@(G ls-remote --exit-code `+q(s.setup.Input.Repository.Remote)+` `+q(ref)+`); if ($r.Count -ne 1 -or ($r[0] -split '\s+')[0] -cne `+q(sha)+` -or ($r[0] -split '\s+')[1] -cne `+q(ref)+`) { throw 'remote_object_changed' }; $facts['sha']=`+q(sha))
	case "commit":
		file := ".steerrunner-check-" + s.setup.Nonce + ".txt"
		return s.exec(ctx, `if ((G rev-parse HEAD) -cne `+q(r.HeadSHA)+` -or (G rev-parse HEAD^) -cne `+q(r.BaseSHA)+`) { throw 'commit_changed' }; $names=@(G diff --name-only `+q(r.BaseSHA)+` HEAD); if ($names.Count -ne 1 -or $names[0] -cne `+q(file)+` -or (G show ('HEAD:'+`+q(file)+`)) -cne `+q(s.setup.Nonce)+`) { throw 'foreign_changes' }; $facts['sha']=`+q(r.HeadSHA))
	case "pr_create", "pr_query", "pr_close":
		body, err := s.probeScript("pr_query", r)
		if err != nil {
			return nil, err
		}
		return s.exec(ctx, body)
	default:
		return map[string]string{"owner": "same_native_tool_context"}, nil
	}
}

func (s *EnvironmentSession) Cleanup(ctx context.Context, action string, r contracts.CheckResources) (contracts.CheckResources, error) {
	if action == "cleanup_pr_close" && r.PRNumber > 0 && !r.PRClosed {
		facts, err := s.Execute(ctx, "pr_close", r)
		if err != nil || facts["pr_closed"] != "True" {
			return r, errors.New("original_pr_close_unverified")
		}
		r.PRClosed = true
	}
	for _, head := range []bool{true, false} {
		if (head && action != "cleanup_head") || (!head && action != "cleanup_base") {
			continue
		}
		ref, sha, deleted := r.BaseRef, r.BaseSHA, r.BaseDeleted
		if head {
			ref, sha, deleted = r.HeadRef, r.HeadSHA, r.HeadDeleted
		}
		owned := r.BaseOwned
		if head {
			owned = r.HeadOwned
		}
		if deleted || sha == "" || !owned {
			continue
		}
		s.gate.arm("delete", ref, sha, strings.Repeat("0", len(sha)), s.gate.base, s.gate.head, 0)
		_, err := s.exec(ctx, `$r=@(G ls-remote `+psLiteral(s.setup.Input.Repository.Remote)+` `+psLiteral(ref)+`); if ($r.Count -gt 0) { if ($r.Count -ne 1 -or ($r[0] -split '\s+')[0] -cne `+psLiteral(sha)+`) { throw 'remote_sha_changed' }; $null=G push ('--force-with-lease='+`+psLiteral(ref)+`+':'+`+psLiteral(sha)+`) -- `+psLiteral(s.setup.Input.Repository.Remote)+` (':'+`+psLiteral(ref)+`) }; if (@(G ls-remote `+psLiteral(s.setup.Input.Repository.Remote)+` `+psLiteral(ref)+`).Count -ne 0) { throw 'delete_unverified' }; $facts['deleted']='true'`)
		s.gate.disarm()
		if err != nil {
			return r, err
		}
		if head {
			r.HeadDeleted = true
		} else {
			r.BaseDeleted = true
		}
	}
	return r, nil
}

func (s *EnvironmentSession) Stop(ctx context.Context) bool {
	s.gate.disarm()
	defer s.backend.Config.Authentication.Close()
	if s.handle == nil {
		return true
	}
	h := s.handle
	if h.process == nil {
		h.Close()
		return h.Binding.Process == nil
	}
	h.process.Kill()
	defer h.process.Close()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	defer h.Close()
	for {
		tree, ok := h.process.(b.ProcessTreeHandle)
		if ok && tree.WritersAbsent() && s.backend.Process.Inspect(h.Binding.Process) == "process absent" {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

// Keep native diagnostics finite: errors from executables/providers can contain
// arbitrary stdout, paths or credential-bearing URLs and are never published.
func EnvironmentDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	for _, stage := range []string{"native_version_unsupported", "native_executable_unavailable", "native_effective_permissions_unverified", "native_thread_identity_unverified", "native_thread_identity_mismatch", "authentication_host_gh_unavailable", "authentication_host_login_unavailable", "authentication_key_unavailable", "authentication_expired_or_unavailable", "authentication_api_socket_path_too_long", "authentication_scope_or_expiry_invalid", "sandbox_identity", "tool_environment", "git_admin_read", "git_admin_write", "remote_identity", "git_transport_authentication", "github_identity_trust", "github_identity_transport", "github_identity_rejected", "github_identity", "github_repository", "remote_git_ca_file", "remote_git_ca_chain", "remote_git_tls", "remote_git_trust", "remote_git_transport", "remote_git_rejected", "remote_git", "repository_identity_mismatch"} {
		if strings.Contains(err.Error(), stage) {
			return stage
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") {
		return "io_timeout_unverified"
	}
	return "preparation_unverified"
}

func decodeEnvironmentCompletion(raw json.RawMessage, threadID, turnID string) (EnvironmentCall, error) {
	var p struct {
		ThreadID string                      `json:"threadId"`
		Turn     struct{ ID, Status string } `json:"turn"`
	}
	if json.Unmarshal(raw, &p) != nil || p.ThreadID != threadID || p.Turn.ID != turnID || p.Turn.Status != "completed" {
		return EnvironmentCall{}, errors.New("original_native_turn_unsettled")
	}
	return EnvironmentCall{Completed: true}, nil
}
