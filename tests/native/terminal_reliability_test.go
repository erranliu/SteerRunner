//go:build windows && live

package native

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	terminal "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
)

func terminalInputFingerprint(t *testing.T, m livetest.Manifest, run contracts.RunView) string {
	t.Helper()
	runtime, e := m.OrcaRuntimeID()
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(strings.TrimPrefix(run.Execution.SessionID, "orca-terminal:"+runtime+":"), ":")
	if len(parts) != 2 || parts[0] == "" {
		t.Fatal("original terminal input binding missing")
	}
	raw, e := os.ReadFile(filepath.Join(m.Root, "evidence", "native-calls.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	fingerprint, e := livetest.TerminalInputFingerprint(raw, parts[0])
	if e != nil {
		t.Fatal(e)
	}
	return fingerprint
}

// The old TUI and its native owner must survive a normal service replacement.
// Compatible replacement preserves the original authorization without resume.
func restartTerminalService(t *testing.T, m livetest.Manifest, dir string, instance *string, original contracts.RunView) {
	t.Helper()
	before := terminalInputFingerprint(t, m, original)
	old := *instance
	stop := request("restart-original-service", contracts.ServiceStopInput{ExpectedInstanceID: old})
	invoke(t, m, dir, []string{"service", "stop", "--input", "-"}, &stop)
	stopped := false
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		res, err := command(m, dir, []string{"service", "status"}, nil)
		if err == nil && res.Data.(map[string]any)["status"] == "not_running" {
			stopped = true
			break
		}
	}
	if !stopped {
		t.Fatal("old service exit unconfirmed")
	}
	started := invoke(t, m, dir, []string{"service", "start"}, nil)
	next := started.Data.(map[string]any)["instance_id"].(string)
	*instance = next // cleanup must own the new service even if a later assertion fails
	if next == "" || next == old {
		t.Fatal("service generation did not change")
	}
	var recovered contracts.RunView
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		var err error
		recovered, err = inspect(m, dir, original.RunID, false)
		if err != nil {
			t.Fatal(err)
		}
		if !recovered.ResumeRequired && recovered.NewInputsAllowed {
			break
		}
	}
	if recovered.ResumeRequired || !recovered.NewInputsAllowed {
		t.Fatal("compatible replacement did not preserve original authorization")
	}
	time.Sleep(3 * time.Second)
	var err error
	recovered, err = inspect(m, dir, original.RunID, false)
	if err != nil || recovered.ResumeRequired || !recovered.NewInputsAllowed || terminalInputFingerprint(t, m, original) != before || recovered.Execution.SessionID != original.Execution.SessionID || recovered.Execution.AttemptID != original.Execution.AttemptID || recovered.Execution.Generation != original.Execution.Generation || recovered.Execution.Workspace != original.Execution.Workspace || recovered.PR == nil || recovered.PR.URL != original.PR.URL || recovered.PR.HeadRevision != original.PR.HeadRevision {
		t.Fatal("service replacement changed original resources, authorization, input history or PR", err)
	}
	save(t, "service-generation-recovery.json", map[string]any{"old_instance": old, "new_instance": next, "recovered": recovered, "native_input_fingerprint_before": before, "native_input_fingerprint_after": terminalInputFingerprint(t, m, recovered)})
}

func activeWriterScripts() (string, string) {
	identity := "$p=Get-Process -Id $PID;@{kind='os';pid=$PID;created=$p.StartTime.ToUniversalTime().ToFileTimeUtc().ToString('x16');executable=$p.Path.ToLowerInvariant()}|ConvertTo-Json -Compress"
	child := identity + " | Set-Content -LiteralPath (Join-Path $PSScriptRoot 'child.json') -Encoding UTF8;Start-Sleep -Seconds 45;Set-Content -LiteralPath (Join-Path $PSScriptRoot 'late.txt') -Value late"
	parent := "$ErrorActionPreference='Stop';$taskChild=Join-Path $PSScriptRoot 'child-writer.ps1';Start-Process -WindowStyle Hidden -WorkingDirectory $PSScriptRoot -FilePath (Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe') -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-File',('\"'+$taskChild+'\"')) | Out-Null;" + identity + " | Set-Content -LiteralPath (Join-Path $PSScriptRoot 'parent.json') -Encoding UTF8;Start-Sleep -Seconds 55"
	return parent, child
}

// Fault timing depends on positively observed OS writers, not a model promise
// or elapsed startup delay. Both parent and detached child must belong to the
// original native Job, including when the tool runs inside AppContainer.
func activeTerminalStop(t *testing.T, m livetest.Manifest, dir string, input contracts.RunRequest) {
	t.Helper()
	// A separate implementation Run supplies an active writer. workflow/2 has
	// no feedback edge after publication; do not invent one for fault timing.
	parent, child := activeWriterScripts()
	parentBytes := append([]byte{0xef, 0xbb, 0xbf}, []byte(parent)...)
	childBytes := append([]byte{0xef, 0xbb, 0xbf}, []byte(child)...)
	specText := "This Run tests stopping owned native writers. Its only work is to run the driver-supplied parent script from Plan, which starts a child and records both OS identities under sr-owned-stop. Keep it running until the driver stops the Run. There is no implementation artifact or PR to produce. Do not recreate or edit the supplied scripts."
	planText := fmt.Sprintf("The driver supplies sr-owned-stop/active-writer.ps1 (SHA256 %x) and child-writer.ps1 (SHA256 %x) after workspace creation. Wait for both files, verify their hashes, then run this short command exactly once and wait: powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-owned-stop/active-writer.ps1", sha256.Sum256(parentBytes), sha256.Sum256(childBytes))
	input.Title = "SteerRunner owned native writer stop"
	input.Documents = contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "active-stop.spec.md", Content: &specText}, Plan: contracts.Document{Kind: "inline", Path: "active-stop.plan.md", Content: &planText}}
	input.Workflow.PromptOverrides = map[string]string{"implement.execute": "Execute the active-writer Spec and Plan now. Start the fixed native PowerShell command and wait while the test driver stops this Run."}
	req := request("start-owned-native-writers", input)
	response := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
	id := response.Data.(map[string]any)["run_id"].(string)
	defer cleanupTerminalFaultRun(t, m, dir, id, "active-writer-cleanup")
	var original contracts.RunView
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(200 * time.Millisecond) {
		var err error
		original, err = inspect(m, dir, id, false)
		if err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(original.Execution.Workspace); err == nil && info.IsDir() {
			break
		}
	}
	if original.Execution.Workspace == "" {
		t.Fatal("active writer workspace unavailable")
	}
	real, err := filepath.EvalSymlinks(original.Execution.Workspace)
	if err != nil || !strings.EqualFold(filepath.Clean(real), filepath.Clean(original.Execution.Workspace)) {
		t.Fatal("active writer workspace redirected or unavailable", err)
	}
	root := filepath.Join(original.Execution.Workspace, "sr-owned-stop")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "active-writer.ps1"), parentBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "child-writer.ps1"), childBytes, 0600); err != nil {
		t.Fatal(err)
	}
	var writers [2]b.ProcessIdentity
	for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); time.Sleep(200 * time.Millisecond) {
		complete := true
		for i, name := range []string{"parent", "child"} {
			raw, err := os.ReadFile(filepath.Join(root, name+".json"))
			raw = bytesWithoutBOM(raw)
			if err != nil || json.Unmarshal(raw, &writers[i]) != nil || process.Probe(&writers[i]).State != "matches" {
				complete = false
			}
		}
		if complete {
			break
		}
	}
	for _, writer := range writers {
		if process.Probe(&writer).State != "matches" {
			t.Fatal("native parent and child did not positively start")
		}
	}
	var owner terminal.TerminalOwner
	var job b.ProcessTreeHandle
	if original.Execution.Agent != "codex" {
		raw, err := os.ReadFile(filepath.Join(dir, "terminal-agents", original.RunID, original.Execution.AttemptID, "owner.json"))
		if err != nil || json.Unmarshal(raw, &owner) != nil {
			t.Fatal("native original Job owner missing", err)
		}
		job, err = process.AttachOwnedJob(owner.Job)
		if err != nil {
			t.Fatal(err)
		}
		defer job.Close()
		membership, ok := job.(interface{ ContainsProcess(uint32) (bool, error) })
		if !ok {
			t.Fatal("native Job membership observation unavailable")
		}
		for _, writer := range writers {
			inside, err := membership.ContainsProcess(uint32(writer.PID))
			if err != nil || !inside {
				t.Fatal("started native tool writer escaped original Job", writer, err)
			}
		}
	}
	save(t, "active-writers-before-stop.json", map[string]any{"writers": writers, "owner": owner, "original_job_membership_checked": job != nil})
	if err := revokeOriginalRun(m, dir, original.RunID, "active-native-stop"); err != nil {
		t.Fatal(err)
	}
	stopped := waitStopped(t, m, dir, original.RunID)
	if job != nil && !job.WritersAbsent() {
		t.Fatal("original Job retains writers after stop")
	}
	for _, writer := range writers {
		if process.Probe(&writer).State != "absent" {
			t.Fatal("started native writer survived stop", writer)
		}
	}
	// Span the child's real write deadline. A quiet PTY alone cannot pass.
	time.Sleep(46 * time.Second)
	if _, err := os.Stat(filepath.Join(root, "late.txt")); !os.IsNotExist(err) {
		t.Fatal("stopped native writer produced late output", err)
	}
	facts := map[string]any{"run": stopped, "writers": writers, "started_writers_absent": true, "late_write_absent": true}
	if job != nil {
		facts["job_writers_absent"] = job.WritersAbsent()
		facts["owner_state"] = process.Probe(&owner.Job.Owner)
	}
	save(t, "active-writers-after-stop.json", facts)
}

func bytesWithoutBOM(raw []byte) []byte {
	if len(raw) >= 3 && raw[0] == 0xef && raw[1] == 0xbb && raw[2] == 0xbf {
		return raw[3:]
	}
	return raw
}

// These short, separate Runs exercise actual preparation failure and actual
// Orca-send response loss. They never continue an unknown input into business.
func verifyLocalTerminalFaults(t *testing.T, m livetest.Manifest, dir string, input contracts.RunRequest) {
	t.Helper()
	var settings contracts.OrcaSettings
	if json.Unmarshal(input.Execution.Settings, &settings) != nil {
		t.Fatal("invalid original settings")
	}
	if settings.AgentName() != "codex" {
		broken := input
		bad := settings
		bad.AgentLaunch.Executable = filepath.Join(m.Root, "bin", "missing-native-"+settings.AgentName()+".cmd")
		broken.Execution.Settings, _ = json.Marshal(bad)
		req := request("native-startup-failure", broken)
		res := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
		id := res.Data.(map[string]any)["run_id"].(string)
		func() {
			stopped := false
			defer func() {
				if !stopped {
					cleanupTerminalFaultRun(t, m, dir, id, "failed-startup-cleanup")
				}
			}()
			var view contracts.RunView
			var fact terminal.TerminalStartupFailure
			verified := false
			for until := time.Now().Add(2 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
				var e error
				view, e = inspect(m, dir, id, false)
				if e != nil {
					t.Fatal(e)
				}
				raw, e := os.ReadFile(filepath.Join(dir, "terminal-agents", id, view.Execution.AttemptID, "startup-exit.json"))
				if e == nil && json.Unmarshal(raw, &fact) == nil && process.Probe(&fact.Owner.Job.Owner).State == "absent" {
					verified = true
					break
				}
			}
			if !verified || !fact.NoWorkerStarted || !fact.AuthenticationClosed || fact.Owner.RunID != id || view.Execution.TurnStarted || view.PR != nil {
				save(t, "startup-failure-unverified.json", view)
				t.Fatal("native pre-worker failure proof unavailable")
			}
			if e := revokeOriginalRun(m, dir, id, "stop-known-startup-failure"); e != nil {
				t.Fatal(e)
			}
			final := waitStopped(t, m, dir, id)
			stopped = true
			raw, e := os.ReadFile(filepath.Join(m.Root, "evidence", "native-calls.jsonl"))
			if e != nil {
				t.Fatal(e)
			}
			runtime, _ := m.OrcaRuntimeID()
			parts := strings.Split(strings.TrimPrefix(final.Execution.SessionID, "orca-terminal:"+runtime+":"), ":")
			if len(parts) != 2 {
				t.Fatal("startup original terminal missing")
			}
			// Only the original helper launch is permitted; there was no business send.
			if fp, e := livetest.TerminalInputFingerprint(raw, parts[0]); e != nil || !strings.HasPrefix(fp, "1:") {
				t.Fatal("startup failure sent business input", fp, e)
			}
			save(t, "startup-failure.json", map[string]any{"failure": fact, "run": final, "business_inputs": 0, "original_terminal_closed": true})
		}()
	}
	marker := filepath.Join(m.Root, "evidence", "drop-next-native-send")
	if e := os.WriteFile(marker, []byte("one original business receipt"), 0600); e != nil {
		t.Fatal(e)
	}
	req := request("native-send-response-lost", input)
	res := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
	id := res.Data.(map[string]any)["run_id"].(string)
	stopped := false
	defer func() {
		if !stopped {
			cleanupTerminalFaultRun(t, m, dir, id, "lost-native-cleanup")
		}
	}()
	var withheld struct {
		CallID   string   `json:"call_id"`
		Args     []string `json:"args"`
		Response struct {
			OK   bool `json:"ok"`
			Meta struct {
				RuntimeID string `json:"runtimeId"`
			} `json:"_meta"`
			Result struct {
				Send struct {
					Handle   string `json:"handle"`
					Accepted bool   `json:"accepted"`
					Prompt   struct {
						RequestID string `json:"requestId"`
					} `json:"prompt"`
				} `json:"send"`
			} `json:"result"`
		} `json:"response"`
	}
	var view contracts.RunView
	hit := false
	for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		var e error
		view, e = inspect(m, dir, id, false)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := os.ReadFile(filepath.Join(m.Root, "evidence", "withheld-native-send.json"))
		if e == nil && json.Unmarshal(raw, &withheld) == nil {
			hit = true
			break
		}
	}
	if !hit || !withheld.Response.OK || !withheld.Response.Result.Send.Accepted || withheld.CallID == "" || withheld.Response.Result.Send.Prompt.RequestID == "" {
		save(t, "lost-native-unverified.json", view)
		t.Fatal("real native accepted response was not withheld")
	}
	before := terminalInputFingerprint(t, m, view)
	if !strings.HasPrefix(before, "2:") {
		t.Fatal("original launch plus exactly one business send required", before)
	}
	if settings.AgentName() != "codex" {
		wire := ""
		for i, arg := range withheld.Args {
			if arg == "--text" && i+1 < len(withheld.Args) {
				wire = withheld.Args[i+1]
			}
		}
		begin := strings.Index(wire, "[SteerRunner command=")
		if begin < 0 {
			t.Fatal("original native command missing")
		}
		begin += len("[SteerRunner command=")
		if len(wire) < begin+32 {
			t.Fatal("original native command truncated")
		}
		commandID := wire[begin : begin+32]
		if !b.StableExecutionID(commandID) {
			t.Fatal("invalid original command")
		}
		var final terminal.TerminalReceipt
		settled := false
		for until := time.Now().Add(40 * time.Second); time.Now().Before(until); time.Sleep(time.Second) {
			raw, e := os.ReadFile(filepath.Join(dir, "terminal-agents", id, view.Execution.AttemptID, commandID+".json"))
			if e == nil && len(raw) <= 65536 && contracts.DecodeStrict(raw, &final) == nil && final.Stage == "settled" && final.Outcome == "completed" && final.StopReason == "stop" && final.RunID == id && final.CommandID == commandID && final.NativeInputID != "" && final.NativeReplyID != "" {
				settled = true
				break
			}
		}
		if !settled {
			t.Fatal("native final did not arrive during unknown acceptance")
		}
		save(t, "late-native-final.json", final)
	}
	// Give original reconciliation time to observe even a settled extension fact.
	// Native settlement alone must never fabricate the missing Orca acceptance.
	time.Sleep(5 * time.Second)
	view, e := inspect(m, dir, id, false)
	if e != nil {
		t.Fatal(e)
	}
	unknown := false
	for _, op := range view.Operations {
		if op.Action == "start_turn" && op.Status == "outcome_unknown" {
			unknown = true
		}
	}
	if !unknown || view.PR != nil || view.Execution.TurnStarted || terminalInputFingerprint(t, m, view) != before {
		save(t, "lost-native-unverified.json", view)
		t.Fatal("unknown native acceptance was promoted/replayed")
	}
	if e := revokeOriginalRun(m, dir, id, "stop-lost-native-input"); e != nil {
		t.Fatal(e)
	}
	final := waitStopped(t, m, dir, id)
	stopped = true
	save(t, "lost-native-send-response.json", map[string]any{"withheld": withheld, "unknown_run": view, "stopped_run": final, "fingerprint_before": before, "fingerprint_after": terminalInputFingerprint(t, m, final), "original_request_not_replayed": true})
}

func cleanupTerminalFaultRun(t *testing.T, m livetest.Manifest, dir, id, prefix string) {
	t.Helper()
	if e := revokeOriginalRun(m, dir, id, prefix); e != nil {
		t.Error("fault Run revoke failed", e)
		return
	}
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		v, e := inspect(m, dir, id, false)
		if e == nil && v.StopStatus == "stopped" && !v.NewInputsAllowed {
			return
		}
	}
	t.Error("fault Run stop remained unverified; preserve original resources", id)
}
