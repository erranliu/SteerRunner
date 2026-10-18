//go:build windows

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOwnedJobReconnectAndStopIncludesStartedDescendant(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "child-started")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	child := filepath.Join(root, "child.ps1")
	if err := os.WriteFile(child, []byte("[IO.File]::WriteAllText("+quote(marker)+", 'started'); Start-Sleep -Seconds 60"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", "$p=Start-Process -WindowStyle Hidden -PassThru -FilePath "+quote(shell)+" -ArgumentList '-NoProfile','-NonInteractive','-File',"+quote(child)+"; Start-Sleep -Seconds 60")
	owned, err := StartManaged(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	defer cmd.Wait()
	defer owned.Kill()
	until := time.Now().Add(10 * time.Second)
	for {
		if _, err = os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("owned descendant did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}
	ref, err := DescribeOwnedJob(owned)
	if err != nil {
		t.Fatal(err)
	}
	wrong := ref
	wrong.Owner.Created = "foreign"
	if h, e := AttachOwnedJob(wrong); e == nil {
		h.Close()
		t.Fatal("adopted changed owner")
	}
	first, err := AttachOwnedJob(ref)
	if err != nil {
		t.Fatal(err)
	}
	first.Close() // Service stop releases only its duplicate.
	if Probe(&ref.Worker).State != "matches" {
		t.Fatal("observation shutdown stopped original worker")
	}
	reconnected, err := AttachOwnedJob(ref)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	owned.Close() // Original owner handle disappears; reconnect pin preserves Job.
	reconnected.Kill()
	until = time.Now().Add(3 * time.Second)
	for !reconnected.WritersAbsent() && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if !reconnected.WritersAbsent() {
		t.Fatal("started descendant remains after original Job stop")
	}
}
