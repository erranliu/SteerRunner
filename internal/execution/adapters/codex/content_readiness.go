package codexworker

import (
	"context"
	"errors"
	backend "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Same native session and permission compiler, with no Git/publication probes
// and no model turn. Never use a prior development check as this receipt.
func (b Backend) verifyContentReadiness(ctx context.Context, h *SessionHandle, input backend.TurnInput) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var account struct {
		Requires *bool `json:"requiresOpenaiAuth"`
		Account  *struct {
			Type string `json:"type"`
		} `json:"account"`
	}
	if err := h.client.Call(ctx, "account/read", map[string]any{"refreshToken": false}, &account); err != nil {
		return errors.New("content_authentication_unverified")
	}
	if account.Requires == nil || *account.Requires && (account.Account == nil || account.Account.Type == "") {
		return errors.New("content_authentication_unavailable")
	}
	params, err := TurnStartParams(b.Config.Workspace, "probe", input.CommandID, "probe", "read-only", "", "", nil)
	if err != nil {
		return err
	}
	marker := "SR_CONTENT_READY:" + input.CommandID
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := "$ErrorActionPreference='Stop'; $actual=(Get-Item -Force -LiteralPath '.').FullName; if([IO.Path]::GetFullPath($actual).TrimEnd('\\') -ine " + q(filepath.Clean(b.Config.Workspace)) + ".TrimEnd('\\')){throw 'directory_identity'}; $entries=[IO.Directory]::EnumerateFileSystemEntries($actual); $iterator=$entries.GetEnumerator(); try{[void]$iterator.MoveNext()}finally{if($iterator -is [IDisposable]){$iterator.Dispose()}}; Write-Output " + q(marker)
	var response struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
	}
	err = h.client.Call(ctx, "command/exec", map[string]any{"command": []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script}, "cwd": b.Config.Workspace, "sandboxPolicy": params["sandboxPolicy"], "timeoutMs": 10000, "outputBytesCap": 2048}, &response)
	verified := err == nil && response.ExitCode == 0 && strings.TrimSpace(response.Stdout) == marker
	fact := map[string]any{"command_id": input.CommandID, "process": h.Binding.Process, "workspace": b.Config.Workspace, "contract": PolicyContract, "scope": "read_only_directory", "authentication_verified": true, "verified": verified, "checked_at": time.Now().UTC()}
	if err := b.Process.WriteAtomic(filepath.Join(h.Paths.TurnDir, "content-readiness.json"), encode(fact)); err != nil {
		return errors.New("content_readiness_evidence_unavailable")
	}
	if !verified {
		return errors.New("native_read_only_directory_unavailable")
	}
	return nil
}
