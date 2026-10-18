package codexworker

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRemoteAuditedModelRoutingAndRestoration(t *testing.T) {
	cfg := RemoteAgentConfig{Model: "writer", ReasoningEffort: "high", AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "medium"}}
	session := "original-thread"
	for _, step := range []string{workflow.StepImplement, workflow.StepReport, workflow.StepAuditSpec, workflow.StepAuditCode, workflow.StepPublish, workflow.StepPublishReport} {
		t.Run(step, func(t *testing.T) {
			model, effort := cfg.Model, cfg.ReasoningEffort
			if workflow.AuditStep(step) {
				model, effort = cfg.AuditModel.Model, cfg.AuditModel.ReasoningEffort
			}
			text := RemoteModelPrompt(strings.Repeat("a", 32), strings.Repeat("b", 64), step, model, effort, "business\r\n")
			input := map[string]any{"threadId": session, "model": "stale-ui-choice", "effort": "low", "input": []map[string]string{{"type": "text", "text": text}}, "collaborationMode": map[string]any{"mode": "default", "settings": map[string]any{"model": "stale-ui-choice", "reasoning_effort": "low", "developer_instructions": "original-mode-instructions"}}}
			raw, _ := json.Marshal(map[string]any{"id": 3, "method": "turn/start", "params": input})
			out, err := rewriteRemoteModelRequest(raw, cfg, session)
			var got struct {
				Params struct {
					Thread        string `json:"threadId"`
					Model         string `json:"model"`
					Effort        string `json:"effort"`
					Collaboration struct {
						Mode     string `json:"mode"`
						Settings struct {
							Model        string `json:"model"`
							Effort       string `json:"reasoning_effort"`
							Instructions string `json:"developer_instructions"`
						} `json:"settings"`
					} `json:"collaborationMode"`
					Input []struct {
						Text string `json:"text"`
					} `json:"input"`
				} `json:"params"`
			}
			if err != nil || json.Unmarshal(out, &got) != nil || got.Params.Model != model || got.Params.Effort != effort || got.Params.Thread != session || got.Params.Input[0].Text != text {
				t.Fatal("model routing changed original thread/input or preferences", err, string(out))
			}
			if got.Params.Collaboration.Mode != "default" || got.Params.Collaboration.Settings.Model != model || got.Params.Collaboration.Settings.Effort != effort || got.Params.Collaboration.Settings.Instructions != "original-mode-instructions" {
				t.Fatal("native collaboration settings override frozen model or change original mode", string(out))
			}
			for _, invalid := range []string{"foreign-thread", "wrong-model", "wrong-step", "changed-body", "plain-prompt", "invalid-collaboration"} {
				copy := map[string]any{"threadId": session, "input": []map[string]string{{"type": "text", "text": text}}}
				switch invalid {
				case "foreign-thread":
					copy["threadId"] = "other"
				case "wrong-model":
					copy["input"] = []map[string]string{{"type": "text", "text": RemoteModelPrompt(strings.Repeat("a", 32), strings.Repeat("b", 64), step, "third-model", effort, "business")}}
				case "wrong-step":
					copy["input"] = []map[string]string{{"type": "text", "text": RemoteModelPrompt(strings.Repeat("a", 32), strings.Repeat("b", 64), "other", model, effort, "business")}}
				case "changed-body":
					copy["input"] = []map[string]string{{"type": "text", "text": text + "changed"}}
				case "plain-prompt":
					copy["input"] = []map[string]string{{"type": "text", "text": "use reviewer"}}
				case "invalid-collaboration":
					copy["collaborationMode"] = map[string]any{"mode": "default"}
				}
				request, _ := json.Marshal(map[string]any{"id": 3, "method": "turn/start", "params": copy})
				if _, err := rewriteRemoteModelRequest(request, cfg, session); err == nil {
					t.Fatal("invalid model control accepted", invalid)
				}
			}
		})
	}
}

func TestRemoteModelBridgeUsesOriginalNativeConnection(t *testing.T) {
	cfg := RemoteAgentConfig{Model: "writer", ReasoningEffort: "high", AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}}
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		_, body, err := c.Read(r.Context())
		if err != nil {
			return
		}
		received <- body
		_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"id":1,"result":{"accepted":true}}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token := "controlled-private-native-transport-token"
	endpoint, closeBridge, err := startRemoteModelBridge(ctx, cfg, "ws"+strings.TrimPrefix(server.URL, "http"), "original", token)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	if bad, reply, e := websocket.Dial(ctx, endpoint, nil); e == nil || reply == nil || reply.StatusCode != http.StatusForbidden {
		if bad != nil {
			bad.CloseNow()
		}
		t.Fatal("unauthenticated bridge access admitted", e)
	}
	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	text := RemoteModelPrompt(strings.Repeat("c", 32), strings.Repeat("d", 64), workflow.StepAuditCode, "reviewer", "high", "audit original work")
	raw, _ := json.Marshal(map[string]any{"id": 1, "method": "turn/start", "params": map[string]any{"threadId": "original", "input": []map[string]string{{"type": "text", "text": text}}}})
	if err = conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	_, response, err := conn.Read(ctx)
	if err != nil || !strings.Contains(string(response), `"accepted":true`) {
		t.Fatal("native response was not forwarded", err)
	}
	select {
	case body := <-received:
		if !strings.Contains(string(body), `"model":"reviewer"`) {
			t.Fatal(string(body))
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestRemoteNativeModelProofRejectsMissingAndChangedContexts(t *testing.T) {
	good := `{"type":"session_meta","payload":{}}
{"type":"turn_context","payload":{"turn_id":"original","model":"reviewer","effort":"high"}}
`
	choice, err := RemoteTurnModel(strings.NewReader(good), "original")
	if err != nil || choice.Model != "reviewer" || choice.ReasoningEffort != "high" {
		t.Fatal(choice, err)
	}
	for _, bad := range []string{strings.ReplaceAll(good, `"original"`, `"foreign"`), strings.ReplaceAll(good, `"high"`, `""`), good + `{"type":"turn_context","payload":{"turn_id":"original","model":"writer","effort":"high"}}`, "invalid"} {
		if _, err := RemoteTurnModel(strings.NewReader(bad), "original"); err == nil {
			t.Fatal("invalid native model proof accepted")
		}
	}
}
