//go:build windows && live

package native

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"testing"
	"time"
)

// Compatible replacement preserves the original session and authorization.
func restartTerminalService(t *testing.T, m livetest.Manifest, dir string, instance *string, original contracts.RunView) {
	t.Helper()
	before := structuredInputFingerprint(t, m, original)
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
		recovered, err = inspect(m, dir, original.RunID)
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
	recovered, err = inspect(m, dir, original.RunID)
	if err != nil || recovered.ResumeRequired || !recovered.NewInputsAllowed || structuredInputFingerprint(t, m, original) != before || recovered.Execution.SessionID != original.Execution.SessionID || recovered.Execution.AttemptID != original.Execution.AttemptID || recovered.Execution.Generation != original.Execution.Generation || recovered.Execution.Workspace != original.Execution.Workspace || recovered.PR == nil || recovered.PR.URL != original.PR.URL || recovered.PR.HeadRevision != original.PR.HeadRevision {
		t.Fatal("service replacement changed original resources, authorization, input history or PR", err)
	}
	save(t, "service-generation-recovery.json", map[string]any{"old_instance": old, "new_instance": next, "recovered": recovered, "native_input_fingerprint_before": before, "native_input_fingerprint_after": structuredInputFingerprint(t, m, recovered)})
}

// The current Orca stop contract is original-session close, not OS Job exit.
func activeTerminalStop(t *testing.T, m livetest.Manifest, dir string, input contracts.RunRequest) {
	t.Helper()
	input.Title = "SteerRunner original structured active stop"
	input.Workflow.PromptOverrides = map[string]string{"implement.execute": "This is an isolated original-session stop qualification. Run Start-Sleep -Seconds 120 with the native shell tool and keep waiting. Do not modify files, commit, push, or publish. The caller will stop this Run while that exact native turn is active."}
	req := request("start-native-active-turn", input)
	response := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
	id := response.Data.(map[string]any)["run_id"].(string)
	defer cleanupTerminalFaultRun(t, m, dir, id, "active-session-cleanup")
	var original contracts.RunView
	active := false
	for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		var err error
		original, err = inspect(m, dir, id)
		if err != nil {
			t.Fatal(err)
		}
		if original.Execution.TurnStarted {
			_, history, err := structuredNativeState(m, original)
			if err == nil && history.Tail.Working && history.CurrentTurn.State == "running" {
				active = true
				save(t, "active-original-session.json", map[string]any{"run": original, "history": history})
				break
			}
		}
		if original.Status == "blocked" || original.PR != nil {
			t.Fatal("original active qualification failed", original.Diagnostic)
		}
	}
	if !active {
		t.Fatal("original native working turn not positively observed")
	}
	before := structuredInputFingerprint(t, m, original)
	if err := revokeOriginalRun(m, dir, id, "active-native-stop"); err != nil {
		t.Fatal(err)
	}
	stopped := waitStopped(t, m, dir, id)
	proof, err := structuredStopEvidence(m, stopped)
	if err != nil {
		t.Fatal(err)
	}
	if structuredInputFingerprint(t, m, stopped) != before {
		t.Fatal("new input after original session revocation")
	}
	save(t, "active-session-stopped.json", proof)
}

// Only the real native create response is lost. While original history is
// unavailable the Run waits; when it returns, reconcile the original request.
func verifyLocalTerminalFaults(t *testing.T, m livetest.Manifest, dir string, input contracts.RunRequest) {
	t.Helper()
	if structuredPipe == nil {
		t.Fatal("local structured wire audit required")
	}
	if err := structuredPipe.DropNext("agentSession.create"); err != nil {
		t.Fatal(err)
	}
	structuredPipe.BlockHistory(true)
	defer structuredPipe.BlockHistory(false)
	input.Workflow.PromptOverrides = map[string]string{"implement.execute": "Return exactly {\"outcome\":\"needs_input\",\"summary\":\"Original structured receipt recovery\",\"question\":\"Stop this qualification Run\"}. Do not call tools, modify files, commit, push, or publish."}
	req := request("native-create-response-lost", input)
	res := invoke(t, m, dir, []string{"submit", "--input", "-"}, &req)
	id := res.Data.(map[string]any)["run_id"].(string)
	defer cleanupTerminalFaultRun(t, m, dir, id, "lost-native-cleanup")
	var original contracts.RunView
	var session string
	dropped := false
	for until := time.Now().Add(3 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		rows, err := structuredPipe.Calls("")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Dropped {
				dropped = true
				session = row.Session
			}
		}
		original, err = inspect(m, dir, id)
		if err != nil {
			t.Fatal(err)
		}
		if dropped {
			break
		}
	}
	if !dropped || session == "" {
		t.Fatal("real original structured create response was not withheld")
	}
	time.Sleep(3 * time.Second)
	original, err := inspect(m, dir, id)
	if err != nil {
		t.Fatal(err)
	}
	unknown := false
	for _, op := range original.Operations {
		if op.Action == "start_turn" && op.Status == "outcome_unknown" {
			unknown = true
		}
	}
	if !unknown || original.Execution.TurnStarted || original.PR != nil {
		t.Fatal("missing original proof was promoted", original)
	}
	rows, err := structuredPipe.Calls(session)
	if err != nil || len(rows) != 1 || rows[0].Method != "agentSession.create" {
		t.Fatal("original mutation replayed while proof missing", rows, err)
	}
	save(t, "lost-structured-response-waiting.json", map[string]any{"run": original, "calls": rows})
	structuredPipe.BlockHistory(false)
	recovered := false
	for until := time.Now().Add(2 * time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		original, err = inspect(m, dir, id)
		if err != nil {
			t.Fatal(err)
		}
		if original.Status == "waiting_input" && original.Question != nil && original.Execution.TurnStarted {
			recovered = true
			break
		}
	}
	if !recovered {
		t.Fatal("original structured response could not be reconciled", original.Diagnostic)
	}
	_, history, err := structuredNativeState(m, original)
	if err != nil || history.Inputs != 1 {
		t.Fatal("original recovered input unverified", err)
	}
	rows, err = structuredPipe.Calls(session)
	if err != nil || len(rows) != 1 {
		t.Fatal("recovery replayed original mutation", rows, err)
	}
	if err = revokeOriginalRun(m, dir, id, "stop-lost-native-input"); err != nil {
		t.Fatal(err)
	}
	final := waitStopped(t, m, dir, id)
	proof, err := structuredStopEvidence(m, final)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = structuredPipe.Calls(session)
	if err != nil || len(rows) != 2 || rows[1].Method != "agentSession.close" {
		t.Fatal("original create/close counts changed", rows, err)
	}
	save(t, "lost-structured-response-recovered.json", map[string]any{"run": original, "history": history, "stopped": proof, "calls": rows, "original_request_not_replayed": true})
}

func cleanupTerminalFaultRun(t *testing.T, m livetest.Manifest, dir, id, prefix string) {
	t.Helper()
	if e := revokeOriginalRun(m, dir, id, prefix); e != nil {
		t.Error("fault Run revoke failed", e)
		return
	}
	for until := time.Now().Add(time.Minute); time.Now().Before(until); time.Sleep(time.Second) {
		v, e := inspect(m, dir, id)
		if e == nil && v.StopStatus == "stopped" && !v.NewInputsAllowed {
			return
		}
	}
	t.Error("fault Run stop remained unverified; preserve original resources", id)
}
