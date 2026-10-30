//go:build windows && live

package codexworker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

// Opt-in qualification of the model binding assembly against an actual native
// app-server. It owns only a fresh read-only directory/session a bootstrap and three tiny
// tool-free turns; it does not operate on a repository, broker, PR or Orca TUI.
func TestBoundaryNativeModelBridge(t *testing.T) {
	executable, root := os.Getenv("STEERRUNNER_BOUNDARY_NATIVE_CODEX"), os.Getenv("STEERRUNNER_BOUNDARY_NATIVE_EVIDENCE")
	if executable == "" || root == "" {
		t.Skip("explicit native executable and fresh evidence directory required")
	}
	dir := filepath.Join(root, "model-bridge")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	host := process.New(30 * time.Second)
	owned, _, endpoint, err := LaunchServer(LaunchConfig{Executable: executable, Workspace: dir, ServerDir: dir, Sandbox: "read-only", WindowsSandbox: "mxc"}, host.StartManaged, func(env []string) []string { return env })
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		owned.Kill()
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			if tree, ok := owned.(interface{ WritersAbsent() bool }); ok && tree.WritersAbsent() {
				owned.Close()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		owned.Close()
		t.Error("native model probe tree exit unverified")
	}()
	client, err := ConnectAppServer(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var catalog struct{ Data []struct{ ID, Model string } }
	if err = client.Call(ctx, "model/list", map[string]any{"limit": 100}, &catalog); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, entry := range catalog.Data {
		v := entry.Model
		if v == "" {
			v = entry.ID
		}
		if v != "" {
			ids = append(ids, v)
		}
	}
	if len(ids) < 2 {
		t.Fatal("two native models are required for routing qualification")
	}
	writer, audit := ids[0], ids[1]
	// Prefer the same low-cost, currently available model family when advertised.
	for _, id := range ids {
		if id == "gpt-6-luna" {
			audit = id
		}
		if id == "gpt-6.1-sol" {
			writer = id
		}
	}
	if writer == audit {
		audit = ids[1]
		if audit == writer {
			audit = ids[0]
		}
	}
	t.Logf("native models writer=%s audit=%s", writer, audit)
	params, err := ThreadStartParams(dir, "read-only", writer, nil)
	if err != nil {
		t.Fatal(err)
	}
	config, err := contentConfiguration(ctx, client, dir)
	if err != nil {
		t.Fatal(err)
	}
	params["config"] = config
	thread, err := StartThread(ctx, client, params, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Match the production helper: materialize the original empty thread before
	// the TUI bridge can resume its rollout on a second connection.
	bootstrapParams, err := TurnStartParams(dir, thread.Thread.ID, strings.Repeat("0", 32), "Initialize this isolated session. Reply READY only. Do not use tools or read or write files.", "read-only", writer, "low", nil)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := StartTurn(ctx, client, bootstrapParams)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapped := false
	for !bootstrapped {
		select {
		case event, ok := <-client.Notifications():
			if !ok {
				t.Fatal("bootstrap connection lost")
			}
			if len(event.ID) > 0 {
				_ = client.RejectRequest(event.ID, "The test bootstrap allows no tools or approvals.")
				t.Fatal("unexpected native bootstrap request")
			}
			if event.Method == "turn/completed" {
				if err = validateTurnCompletion(event.Params, thread.Thread.ID, bootstrap.Turn.ID); err != nil {
					t.Fatal(err)
				}
				bootstrapped = true
			}
		case <-ctx.Done():
			t.Fatal("native bootstrap timed out")
		}
	}
	t.Log("original thread bootstrap completed")
	go func() {
		for range client.Notifications() {
		}
	}()
	cfg := RemoteAgentConfig{Model: writer, ReasoningEffort: "low", AuditModel: &workflow.ModelSettings{Model: audit, ReasoningEffort: "low"}}
	cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: writer, ReasoningEffort: "low"}, cfg.AuditModel)
	const token = "boundary-native-model-bridge-private-token"
	bridge, closeBridge, err := startRemoteModelBridge(ctx, cfg, endpoint, thread.Thread.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	conn, _, err := websocket.Dial(ctx, bridge, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(16 << 20)
	send := func(v any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
			t.Fatal(err)
		}
	}
	send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]string{"name": "steerrunner_boundary_probe", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}}})
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var v struct{ ID int }
		_ = json.Unmarshal(raw, &v)
		if v.ID == 1 {
			break
		}
	}
	send(map[string]any{"method": "initialized", "params": map[string]any{}})
	send(map[string]any{"id": 100, "method": "thread/resume", "params": map[string]any{"threadId": thread.Thread.ID, "cwd": dir, "excludeTurns": true, "sandbox": "read-only", "approvalPolicy": "never", "approvalsReviewer": "user", "config": config}})
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			ID    int
			Error any
		}
		if json.Unmarshal(raw, &v) != nil {
			t.Fatal("invalid resume reply")
		}
		if v.ID == 100 {
			if v.Error != nil {
				t.Fatal("original thread resume failed", string(raw))
			}
			break
		}
	}
	t.Log("bridge resumed original thread")
	// The first native connection retains the newly created thread. The bridge
	// sends to that exact thread, so no second thread/start or replacement occurs.
	evidence := []map[string]string{}
	for n, step := range []string{workflow.StepImplement, workflow.StepAuditCode, workflow.StepPublish} {
		choice := cfg.models[step]
		text := RemoteModelPrompt(fmt.Sprintf("%032x", n+1), strings.Repeat("a", 64), step, choice.Model, choice.ReasoningEffort, "Reply READY only. Do not use tools or inspect or change any files.")
		turnParams, err := TurnStartParams(dir, thread.Thread.ID, fmt.Sprintf("%032x", n+1), text, "read-only", "stale-ui-model", "low", nil)
		if err != nil {
			t.Fatal(err)
		}
		send(map[string]any{"id": n + 2, "method": "turn/start", "params": turnParams})
		t.Logf("starting native step=%s model=%s", step, choice.Model)
		turnID := ""
		complete := false
		for !complete {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var event struct {
				ID     int
				Method string
				Result json.RawMessage
				Params json.RawMessage
				Error  any
			}
			if json.Unmarshal(raw, &event) != nil {
				t.Fatal("invalid native message")
			}
			if event.ID == n+2 {
				if event.Error != nil {
					t.Fatal("native turn rejected", string(raw))
				}
				var receipt TurnReceipt
				if json.Unmarshal(event.Result, &receipt) != nil {
					t.Fatal("native turn identity missing")
				}
				turnID = receipt.Turn.ID
				t.Logf("native turn accepted id=%s", turnID)
			}
			if event.Method == "item/started" || event.Method == "item/completed" {
				var item struct{ Item struct{ Type string } }
				_ = json.Unmarshal(event.Params, &item)
				switch item.Item.Type {
				case "userMessage", "agentMessage", "reasoning":
				default:
					t.Fatal("native probe used a tool", item.Item.Type)
				}
			}
			if event.Method == "turn/completed" {
				var done struct {
					ThreadID string `json:"threadId"`
					Turn     struct{ ID, Status string }
				}
				_ = json.Unmarshal(event.Params, &done)
				if done.ThreadID != thread.Thread.ID || turnID == "" || done.Turn.ID != turnID || done.Turn.Status != "completed" {
					t.Fatal("native turn completion mismatch", string(raw))
				}
				complete = true
				t.Logf("native turn completed id=%s", turnID)
			}
		}
		var read struct{ Thread HistoryThread }
		if err = client.Call(ctx, "thread/read", map[string]any{"threadId": thread.Thread.ID, "includeTurns": true}, &read); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(read.Thread.Path)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := RemoteTurnModel(file, turnID)
		file.Close()
		if err != nil || actual != *choice {
			t.Fatal("native model binding changed", actual, choice, err)
		}
		evidence = append(evidence, map[string]string{"step": step, "session_id": thread.Thread.ID, "turn_id": turnID, "model": actual.Model, "effort": actual.ReasoningEffort})
	}
	raw, _ := json.MarshalIndent(evidence, "", "  ")
	if err = os.WriteFile(filepath.Join(dir, "native-models.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
