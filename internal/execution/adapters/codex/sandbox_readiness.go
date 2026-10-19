package codexworker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/execution/backend"
)

type sandboxReadiness struct {
	CommandID               string                   `json:"command_id"`
	Process                 *backend.ProcessIdentity `json:"process"`
	Workspace               string                   `json:"workspace"`
	Contract                string                   `json:"contract"`
	CheckedAt               time.Time                `json:"checked_at"`
	ExpiresAt               time.Time                `json:"expires_at"`
	Repository              string                   `json:"repository"`
	Verified                bool                     `json:"verified"`
	Stage                   string                   `json:"stage"`
	AuthenticationDirectory string                   `json:"authentication_directory,omitempty"`
}

func psLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

// Only the trusted script's finite result is persisted. No raw native command
// output, credential-helper response or environment is copied into diagnostics.
func (a *AuthenticationRuntime) readinessScript(roots []string, remoteName string, modes ...string) (string, time.Time, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.validLocked(); err != nil {
		return "", time.Time{}, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(a.token)))
	paths := []string{}
	for _, root := range roots {
		paths = append(paths, psLiteral(root))
	}
	if remoteName == "" {
		remoteName = "origin"
	}
	identityProbe := `$raw=@(& gh api installation/repositories --paginate=false 2>$null)
 if ($LASTEXITCODE -ne 0) { throw 'installation rejected' }
 $identity=($raw -join [char]10) | ConvertFrom-Json
 if ($identity.total_count -ne 1 -or $identity.repositories.Count -ne 1 -or $identity.repositories[0].full_name -ine ` + psLiteral(a.repository) + `) { throw 'scope mismatch' }`
	transportProbe := ""
	if a.broker != nil && !a.broker.installation {
		identityProbe = `$ErrorActionPreference='Continue'; $raw=@(& gh api user 2>&1); $code=$LASTEXITCODE; $ErrorActionPreference='Stop'
 if ($code -ne 0) {
  $message=($raw -join [char]10)
  if ($message -match 'x509|certificate|TLS') { $stage='github_identity_trust' }
  elseif ($message -match 'connectex|dial tcp|connection|proxyconnect') { $stage='github_identity_transport' }
  elseif ($message -match 'HTTP 401|HTTP 403') { $stage='github_identity_rejected' }
  throw 'broker identity rejected'
 }
 if ((($raw -join [char]10) | ConvertFrom-Json).login -cne ` + psLiteral(a.broker.login) + `) { throw 'broker identity mismatch' }`
		transportProbe = `if ($env:HTTPS_PROXY -cne ` + psLiteral(a.broker.address) + ` -or $env:SSL_CERT_FILE -cne ` + psLiteral(a.broker.caPath) + ` -or -not [IO.File]::Exists($env:SSL_CERT_FILE)) { throw 'transport filtered' }`
	}
	if a.broker != nil && a.broker.installation {
		transportProbe = `if ($env:HTTPS_PROXY -cne ` + psLiteral(a.broker.address) + ` -or $env:SSL_CERT_FILE -cne ` + psLiteral(a.broker.caPath) + `) { throw 'transport filtered' }`
	}
	containerValue := "1"
	if len(modes) > 0 && modes[0] == "danger-full-access" {
		containerValue = "0"
	}
	script := `$ErrorActionPreference='Stop'
$stage='tool_environment'
try {
 $stage='sandbox_identity'
 Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class SRTokenProbe { [DllImport("advapi32.dll", SetLastError=true)] public static extern bool GetTokenInformation(IntPtr h,int c,out int v,int n,out int r); }'
 $value=0; $size=0
 if (-not [SRTokenProbe]::GetTokenInformation([Security.Principal.WindowsIdentity]::GetCurrent().Token,29,[ref]$value,4,[ref]$size) -or $value -ne ` + containerValue + `) { throw 'execution token boundary differs' }
 $stage='tool_environment'
 if (-not $env:GH_TOKEN -or -not $env:GIT_CONFIG_COUNT -or $env:GH_HOST -ne 'github.com' -or $env:GH_PROMPT_DISABLED -ne '1') { throw 'filtered' }
 $sha=[Security.Cryptography.SHA256]::Create()
 try { $hash=([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($env:GH_TOKEN)))).Replace('-','').ToLowerInvariant() } finally { $sha.Dispose() }
 if ($hash -ne ` + psLiteral(hash) + `) { throw 'different credential' }
 ` + transportProbe + `
 $stage='git_admin_read'
 $head=@(& git rev-parse --verify HEAD 2>&1); if ($LASTEXITCODE -ne 0) { throw 'git read' }
 $stage='git_admin_write'
 foreach ($root in @(` + strings.Join(paths, ",") + `)) {
  if (-not [IO.Directory]::Exists($root)) { throw 'missing root' }
  $probe=Join-Path $root ('.sr-auth-'+[Guid]::NewGuid().ToString('N')+'.lock')
  try { [IO.File]::WriteAllText($probe,'probe'); if ([IO.File]::ReadAllText($probe) -ne 'probe') { throw 'readback' } } finally { if ([IO.File]::Exists($probe)) { [IO.File]::Delete($probe) } }
 }
 $stage='remote_identity'
 $remote=(& git remote get-url ` + psLiteral(remoteName) + ` 2>&1 | Out-String).Trim()
 if ($LASTEXITCODE -ne 0 -or $remote -cne ` + psLiteral(a.remoteURL) + `) { throw 'remote mismatch' }
 $stage='git_transport_authentication'
 $helpers=@(& git config --get-all credential.helper 2>$null)
 if ($LASTEXITCODE -ne 0 -or $helpers.Count -eq 0 -or $helpers[-1] -ne '') { throw 'helper boundary' }
 # The authenticated native gh/Git operations below establish connectivity.
 # An anonymous WebRequest to /meta adds an unrelated shared-IP quota gate.
 $stage='github_identity'
 ` + identityProbe + `
 $stage='github_repository'
 $raw=@(& gh api ` + psLiteral("repos/"+a.repository) + ` 2>$null)
 if ($LASTEXITCODE -ne 0 -or (($raw -join [char]10) | ConvertFrom-Json).full_name -ine ` + psLiteral(a.repository) + `) { throw 'repository unavailable' }
 $stage='remote_git'
 $ErrorActionPreference='Continue'; $refs=@(& git ls-remote --exit-code ` + psLiteral(remoteName) + ` HEAD 2>&1); $code=$LASTEXITCODE; $ErrorActionPreference='Stop'
 if ($code -ne 0 -or $refs.Count -eq 0) {
  $message=($refs -join [char]10)
  if ($message -match 'error setting certificate|CAfile|CApath') { $stage='remote_git_ca_file' }
  elseif ($message -match 'self.signed|unable to get|unknown authority') { $stage='remote_git_ca_chain' }
  elseif ($message -match 'SSL_connect|SSL_ERROR|tls') { $stage='remote_git_tls' }
  elseif ($message -match 'certificate|SSL|schannel') { $stage='remote_git_trust' }
  elseif ($message -match 'connect|proxy|resolve host') { $stage='remote_git_transport' }
  elseif ($message -match '401|403|Authentication|Username') { $stage='remote_git_rejected' }
  throw 'remote unavailable'
 }
 Write-Output '{"verified":true,"stage":"ready"}'
 exit 0
} catch {
 Write-Output ('{"verified":false,"stage":"'+$stage+'"}')
 exit 1
}`
	return script, a.expires, nil
}

func (b Backend) verifySandboxReadiness(ctx context.Context, h *SessionHandle, input backend.TurnInput) error {
	if b.Config.Sandbox == "read-only" {
		return b.verifyContentReadiness(ctx, h, input)
	}
	if b.Config.Authentication == nil {
		return nil
	}
	if b.Config.EnvironmentCheck {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	return b.checkSandboxReadiness(ctx, h, input, h.client.Call)
}

func (b Backend) checkSandboxReadiness(ctx context.Context, h *SessionHandle, input backend.TurnInput, call func(context.Context, string, any, any) error) error {
	roots, err := WritableRoots(input.LinkedWorktreeArgs)
	if err != nil || (len(roots) != 4 && len(roots) != 5) {
		return errors.New("sandbox_readiness_git_roots_invalid")
	}
	script, expires, err := b.Config.Authentication.readinessScript(roots, b.Config.RemoteName, b.Config.Sandbox)
	if err != nil {
		return err
	}
	toolRoots, err := WritableRoots(b.toolWriteArgs(input.LinkedWorktreeArgs))
	if err != nil {
		return err
	}
	params, err := TurnStartParams(b.Config.Workspace, "probe", "probe", "probe", b.Config.Sandbox, "", "", toolRoots)
	if err != nil {
		return err
	}
	command := map[string]any{"command": []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script}, "cwd": b.Config.Workspace, "sandboxPolicy": params["sandboxPolicy"], "timeoutMs": 25000, "outputBytesCap": 2048}
	var result struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	err = call(ctx, "command/exec", command, &result)
	fact := sandboxReadiness{CommandID: input.CommandID, Process: h.Binding.Process, Workspace: b.Config.Workspace, Contract: PolicyContract, CheckedAt: time.Now().UTC(), ExpiresAt: expires, Repository: b.Config.Authentication.repository, Stage: "native_sandbox_command"}
	if p := b.Config.AuthenticationTransport; p != nil {
		fact.AuthenticationDirectory = p.Directory
	}
	var output struct {
		Verified bool   `json:"verified"`
		Stage    string `json:"stage"`
	}
	allowed := map[string]bool{"sandbox_identity": true, "tool_environment": true, "git_admin_read": true, "git_admin_write": true, "remote_identity": true, "git_transport_authentication": true, "github_identity": true, "github_repository": true, "remote_git": true, "ready": true}
	for _, stage := range []string{"github_identity_trust", "github_identity_transport", "github_identity_rejected", "remote_git_trust", "remote_git_transport", "remote_git_rejected"} {
		allowed[stage] = true
	}
	for _, stage := range []string{"remote_git_ca_file", "remote_git_ca_chain", "remote_git_tls"} {
		allowed[stage] = true
	}
	if err == nil && json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &output) == nil && allowed[output.Stage] {
		fact.Stage = output.Stage
		fact.Verified = result.ExitCode == 0 && output.Verified && output.Stage == "ready"
	}
	if saveErr := b.Process.WriteAtomic(filepath.Join(h.Paths.TurnDir, "sandbox-readiness.json"), encode(fact)); saveErr != nil {
		return errors.New("sandbox_readiness_evidence_unavailable")
	}
	if !fact.Verified {
		return fmt.Errorf("sandbox_readiness_failed: %s; no business input sent", fact.Stage)
	}
	return nil
}
