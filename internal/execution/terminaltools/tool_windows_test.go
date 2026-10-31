//go:build windows

package terminaltools

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	c := Config{AccessMode: "full_access", Workspace: filepath.Join(root, "workspace"), StateDirectory: filepath.Join(root, "state"), Shell: filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")}
	for _, dir := range []string{c.Workspace, c.StateDirectory} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestFullAccessCommandHasNoCodexSandboxDependency(t *testing.T) {
	c := fixture(t)
	command, err := c.Command(filepath.Join(c.StateDirectory, "tool.ps1"))
	if err != nil || command.Executable != c.Shell || strings.Contains(strings.Join(command.Args, " "), "sandbox") {
		t.Fatal(command, err)
	}
	for _, mode := range []string{"", "sandbox", "auto", "danger-full-access"} {
		c.AccessMode = mode
		if _, err := c.Command(filepath.Join(c.StateDirectory, "tool.ps1")); err == nil {
			t.Fatal("accepted retired/implicit mode", mode)
		}
	}
}

func TestTerminalToolInputCannotExposeProviderSecrets(t *testing.T) {
	env := ToolEnvironment([]string{"PATH=tools", "OPENROUTER_API_KEY=secret", "DEEPSEEK_API_KEY=secret", "GH_TOKEN=scoped-capability", "GIT_CONFIG_COUNT=2", "OTHER_SECRET=secret", "SystemRoot=C:\\Windows"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "secret") || !strings.Contains(joined, "GH_TOKEN=scoped-capability") || !strings.Contains(joined, "GIT_CONFIG_COUNT=2") {
		t.Fatal(env)
	}
}

func TestTerminalToolFullAccessUsesOwnedJobAndCleansStaging(t *testing.T) {
	c := fixture(t)
	c.AccessMode = "full_access"
	var out, diagnostics bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := Run(ctx, c, strings.NewReader("[Console]::Write('native-tool-ok')"), &out, &diagnostics)
	if err != nil || out.String() != "native-tool-ok" {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	files, err := os.ReadDir(c.StateDirectory)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}

func TestTerminalToolCancellationRevokesOwnedChildren(t *testing.T) {
	c := fixture(t)
	c.AccessMode = "full_access"
	marker := filepath.Join(c.Workspace, "child.txt")
	// The child inherits the Job even when PowerShell detaches it. It must
	// never reach this delayed write after the original tool is cancelled.
	body := `Start-Process powershell.exe -WindowStyle Hidden -ArgumentList @('-NoProfile','-Command',"Start-Sleep -Seconds 4; [IO.File]::WriteAllText('` + marker + `','escaped')") | Out-Null; Start-Sleep -Seconds 30`
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	var out, diagnostics bytes.Buffer
	if err := Run(ctx, c, strings.NewReader(body), &out, &diagnostics); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("cancelled tool lost its uncertainty classification", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("child wrote after cancellation", err)
	}
	files, err := os.ReadDir(c.StateDirectory)
	if err != nil || len(files) != 0 {
		t.Fatal("owned job did not settle/clean", files, err)
	}
}

func TestTerminalToolStateCannotBeGrantedToTheChild(t *testing.T) {
	c := fixture(t)
	c.WriteRoots = []string{filepath.Dir(c.StateDirectory)}
	if c.Validate() == nil {
		t.Fatal("tool state became writable")
	}
}

func TestTerminalToolNativeStderrUTF8AndExitStatus(t *testing.T) {
	c := fixture(t)
	c.AccessMode = "full_access"
	var out, diagnostics bytes.Buffer
	// The real PowerShell 5 pipeline which aborted a live Git publication:
	// capturing a successful native command's stderr must not terminate it.
	body := `$items = & $PSHOME\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('native-progress');exit 0" 2>&1; $code=$LASTEXITCODE; $items | ForEach-Object {$_.ToString()}; [Console]::Write('中文'); exit $code`
	if err := Run(context.Background(), c, strings.NewReader(body), &out, &diagnostics); err != nil || !strings.Contains(out.String(), "native-progress") || !strings.Contains(out.String(), "中文") {
		t.Fatal("native stderr or UTF-8 changed command outcome", err, out.String(), diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	body = `& $PSHOME\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('native-progress');exit 0" 2>&1`
	if err := Run(context.Background(), c, strings.NewReader(body), &out, &diagnostics); err != nil {
		t.Fatal("PowerShell status flag replaced successful native exit code", err, out.String(), diagnostics.String())
	}
	for _, body := range []string{`& $PSHOME\powershell.exe -NoProfile -Command 'exit 7'`, `Get-Item -LiteralPath './missing-native-file' -ErrorAction Stop`, `throw 'native-command-failure'`} {
		out.Reset()
		diagnostics.Reset()
		if err := Run(context.Background(), c, strings.NewReader(body), &out, &diagnostics); err == nil {
			t.Fatal("failed native/cmdlet statement became successful", body)
		}
	}
}

func TestTerminalToolFinalStatusDoesNotBorrowEarlierNativeExit(t *testing.T) {
	c := fixture(t)
	c.AccessMode = "full_access"
	prefix := "& './foreign.ps1'; "
	foreign := strings.Repeat("\n", 4) + strings.Repeat(" ", len(prefix)) + `& $PSHOME\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('progress');exit 0" 2>&1`
	if e := os.WriteFile(filepath.Join(c.Workspace, "foreign.ps1"), []byte(foreign), 0600); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`& $PSHOME\powershell.exe -NoProfile -Command 'exit 0'; Get-Item -LiteralPath './missing-native-file'`, false},
		{prefix + `Get-Item missing-native-file -ErrorAction Ignore`, false},
		{`& $PSHOME\powershell.exe -NoProfile -Command 'exit 7'; if($LASTEXITCODE -eq 7){Write-Output 'handled'}`, true},
		{`& $PSHOME\powershell.exe -NoProfile -Command "[Console]::Error.WriteLine('progress');exit 0" 2>&1; Get-Item -LiteralPath './missing-native-file' -ErrorAction Ignore`, false},
	} {
		var out, diag bytes.Buffer
		err := Run(context.Background(), c, strings.NewReader(tc.body), &out, &diag)
		if (err == nil) != tc.ok {
			t.Fatalf("body=%s error=%v stdout=%s stderr=%s", tc.body, err, out.String(), diag.String())
		}
	}
}
