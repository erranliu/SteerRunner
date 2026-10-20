//go:build windows && live

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"golang.org/x/sys/windows"
)

type nativeRecoveryTool struct {
	ItemID          string            `json:"item_id"`
	NativeProcessID string            `json:"native_process_id"`
	Command         string            `json:"command"`
	Process         b.ProcessIdentity `json:"process"`
}

// Do not interpret native processId as an OS PID. Bind the actual OS shell
// independently by the unique nonce in this exact native tool command.
func nativeToolProcess(nonce string) (b.ProcessIdentity, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	script := `$taskNonce='` + strings.ReplaceAll(nonce, "'", "''") + `'; $taskCandidates=@(Get-CimInstance Win32_Process -Filter "Name='pwsh.exe' OR Name='powershell.exe'" | Where-Object {$_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($taskNonce) -and $_.CommandLine -match 'Start-Sleep'} | Select-Object ProcessId); ConvertTo-Json -InputObject @($taskCandidates) -Compress`
	raw, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return b.ProcessIdentity{}, false
	}
	var candidates []struct {
		PID int `json:"ProcessId"`
	}
	if json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), &candidates) != nil {
		return b.ProcessIdentity{}, false
	}
	for _, candidate := range candidates {
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(candidate.PID))
		if err != nil {
			continue
		}
		identity, err := process.IdentityFromHandle(handle, candidate.PID)
		windows.CloseHandle(handle)
		if err == nil && process.InspectProcess(&identity) == "matching process still exists" {
			return identity, true
		}
	}
	return b.ProcessIdentity{}, false
}

// Bind a real in-progress wait command to the original native thread/turn and
// an independently observed OS shell. A turn-start notification is insufficient.
func waitNativeRecoveryTool(t *testing.T, dir, id, nonce string, active contracts.RunView, root b.ProcessIdentity) nativeRecoveryTool {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		paths, _ := filepath.Glob(filepath.Join(dir, "runs", id, "turns", "*", "appserver.jsonl"))
		started := map[string]nativeRecoveryTool{}
		completed := map[string]bool{}
		for _, path := range paths {
			raw, _ := os.ReadFile(path)
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				var event struct {
					Method string `json:"method"`
					Params struct {
						ThreadID string `json:"threadId"`
						TurnID   string `json:"turnId"`
						Item     struct {
							ID        string `json:"id"`
							Type      string `json:"type"`
							Command   string `json:"command"`
							ProcessID string `json:"processId"`
						} `json:"item"`
					} `json:"params"`
				}
				if json.Unmarshal(line, &event) != nil || event.Params.ThreadID != active.Execution.SessionID || event.Params.TurnID != active.Execution.TurnID || event.Params.Item.Type != "commandExecution" {
					continue
				}
				item := event.Params.Item
				if event.Method == "item/completed" {
					completed[item.ID] = true
					continue
				}
				if event.Method != "item/started" || !strings.Contains(strings.ToLower(item.Command), "start-sleep") || !strings.Contains(item.Command, nonce) {
					continue
				}
				started[item.ID] = nativeRecoveryTool{ItemID: item.ID, NativeProcessID: item.ProcessID, Command: item.Command}
			}
		}
		for item, tool := range started {
			if !completed[item] && process.InspectProcess(&root) == "matching process still exists" {
				if identity, ok := nativeToolProcess(nonce); ok {
					tool.Process = identity
					return tool
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("real active wait tool and independent original shell process identity were not observed")
	return nativeRecoveryTool{}
}
