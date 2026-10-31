package codexworker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteModelBridgeRejectsInvalidOriginalRequest(t *testing.T) {
	for _, id := range []json.RawMessage{json.RawMessage(`7`), json.RawMessage(`"native-request"`)} {
		for _, fault := range []string{"missing-newline", "truncated", "changed-byte", "mixed-input", "foreign-thread", "wrong-model", "new-thread", "steer"} {
			t.Run(string(id)+"/"+fault, func(t *testing.T) {
				cfg := RemoteAgentConfig{Model: "writer", ReasoningEffort: "high", AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}}
				cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort}, cfg.AuditModel)
				var forwarded atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, err := websocket.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer conn.CloseNow()
					for {
						_, body, err := conn.Read(r.Context())
						if err != nil {
							return
						}
						forwarded.Add(1)
						var req struct {
							ID     json.RawMessage `json:"id"`
							Method string          `json:"method"`
						}
						if json.Unmarshal(body, &req) != nil || req.Method != "thread/read" {
							t.Error("rejected input reached original server", string(body))
						}
						reply, _ := json.Marshal(map[string]any{"id": req.ID, "result": map[string]bool{"read": true}})
						if conn.Write(r.Context(), websocket.MessageText, reply) != nil {
							return
						}
					}
				}))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				const token = "controlled-private-native-transport-token"
				endpoint, closeBridge, err := startRemoteModelBridge(ctx, cfg, "ws"+strings.TrimPrefix(server.URL, "http"), "original", token)
				if err != nil {
					t.Fatal(err)
				}
				defer closeBridge()
				conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
				if err != nil {
					t.Fatal(err)
				}
				defer conn.CloseNow()
				text := RemoteModelPrompt(strings.Repeat("c", 32), strings.Repeat("d", 64), workflow.StepImplement, "writer", "high", "中文 first\n\nsecond")
				method, thread, want := "turn/start", "original", "remote_model_input_invalid"
				switch fault {
				case "missing-newline":
					text = strings.Replace(text, "first\n\nsecond", "firstsecond", 1)
				case "truncated":
					text = text[:len(text)-1]
				case "changed-byte":
					text = strings.Replace(text, "second", "Second", 1)
				case "mixed-input":
					text += "foreign input"
				case "foreign-thread":
					thread, want = "foreign", "remote_model_original_input_required"
				case "wrong-model":
					text = RemoteModelPrompt(strings.Repeat("c", 32), strings.Repeat("d", 64), workflow.StepImplement, "other", "high", "original")
					want = "remote_model_choice_not_frozen"
				case "new-thread":
					method, want = "thread/start", "remote_model_original_thread_only"
				case "steer":
					method, want = "turn/steer", "remote_model_original_thread_only"
				}
				raw, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": map[string]any{"threadId": thread, "input": []map[string]string{{"type": "text", "text": text}}}})
				if err = conn.Write(ctx, websocket.MessageText, raw); err != nil {
					t.Fatal(err)
				}
				_, body, err := conn.Read(ctx)
				var reply struct {
					ID    json.RawMessage `json:"id"`
					Error struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err != nil || json.Unmarshal(body, &reply) != nil || string(reply.ID) != string(id) || reply.Error.Code != -32602 || reply.Error.Message != want {
					t.Fatal("integrity rejection became a transport loss or lost identity", string(body), err)
				}
				if forwarded.Load() != 0 || strings.Contains(string(body), "中文") || strings.Contains(string(body), token) {
					t.Fatal("rejection forwarded input or leaked content")
				}
				// A query is a deterministic barrier: the same connection stays usable,
				// and only this query may reach the server after the rejection.
				if err = conn.Write(ctx, websocket.MessageText, []byte(`{"id":99,"method":"thread/read","params":{"threadId":"original"}}`)); err != nil {
					t.Fatal(err)
				}
				_, body, err = conn.Read(ctx)
				if err != nil || !strings.Contains(string(body), `"read":true`) || forwarded.Load() != 1 {
					t.Fatal("rejected request was retried or connection was lost", string(body), err, forwarded.Load())
				}
			})
		}
	}
}

func TestRemoteModelBridgeRejectsUnboundInvalidRequest(t *testing.T) {
	for _, id := range []string{"missing", "null", "true", "{}"} {
		t.Run(id, func(t *testing.T) {
			var forwarded atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				if _, _, err := conn.Read(r.Context()); err == nil {
					forwarded.Add(1)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			const token = "controlled-private-native-transport-token"
			endpoint, closeBridge, err := startRemoteModelBridge(ctx, RemoteAgentConfig{}, "ws"+strings.TrimPrefix(server.URL, "http"), "original", token)
			if err != nil {
				t.Fatal(err)
			}
			defer closeBridge()
			conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			raw := `{"method":"thread/start"}`
			if id != "missing" {
				raw = `{"id":` + id + `,"method":"thread/start"}`
			}
			if err = conn.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
				t.Fatal(err)
			}
			if _, body, err := conn.Read(ctx); err == nil || forwarded.Load() != 0 {
				t.Fatal("unbound request was answered or forwarded", string(body), err, forwarded.Load())
			}
		})
	}
}

func TestRemoteModelBridgePreservesLargeNativeReplies(t *testing.T) {
	// A legitimate startup reply can be larger than the bridge's old 16 MiB
	// bound. Codex's native remote client admits messages up to 128 MiB.
	large := []byte(`{"id":1,"result":{"metadata":"` + strings.Repeat("x", (16<<20)+1) + `"}}`)
	var connections, requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		connections.Add(1)
		defer conn.CloseNow()
		for {
			_, raw, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			requests.Add(1)
			var request struct {
				Method string `json:"method"`
				Params struct {
					Model string `json:"model"`
					Input []struct {
						Text string `json:"text"`
					} `json:"input"`
				} `json:"params"`
			}
			if json.Unmarshal(raw, &request) != nil {
				t.Error("invalid original request")
				return
			}
			switch request.Method {
			case "plugin/list":
				if conn.Write(r.Context(), websocket.MessageText, large) != nil {
					return
				}
			case "turn/start":
				if request.Params.Model != "writer" || len(request.Params.Input) != 1 {
					t.Error("original model or input changed")
					return
				}
				if _, err := ParseRemoteModelPrompt(request.Params.Input[0].Text); err != nil {
					t.Error(err)
					return
				}
				if conn.Write(r.Context(), websocket.MessageText, []byte(`{"id":2,"result":{"accepted":true}}`)) != nil {
					return
				}
			default:
				t.Error("unexpected replay or request", request.Method)
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg := RemoteAgentConfig{Model: "writer", ReasoningEffort: "high"}
	cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort}, nil)
	const token = "controlled-private-native-transport-token"
	endpoint, closeBridge, err := startRemoteModelBridge(ctx, cfg, "ws"+strings.TrimPrefix(server.URL, "http"), "original", token)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(128 << 20)
	if err = conn.Write(ctx, websocket.MessageText, []byte(`{"id":1,"method":"plugin/list","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	_, body, err := conn.Read(ctx)
	if err != nil || len(body) != len(large) || sha256.Sum256(body) != sha256.Sum256(large) {
		t.Fatal("native reply was truncated or disconnected the TUI", len(body), len(large), err)
	}
	text := RemoteModelPrompt(strings.Repeat("c", 32), strings.Repeat("d", 64), workflow.StepImplement, "writer", "high", "中文 original\n\nsecond")
	raw, _ := json.Marshal(map[string]any{"id": 2, "method": "turn/start", "params": map[string]any{"threadId": "original", "input": []map[string]string{{"type": "text", "text": text}}}})
	if err = conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	_, body, err = conn.Read(ctx)
	if err != nil || string(body) != `{"id":2,"result":{"accepted":true}}` || connections.Load() != 1 || requests.Load() != 2 {
		t.Fatal("startup reply lost the original connection or replayed input", string(body), err, connections.Load(), requests.Load())
	}
}

func TestRemoteAuditedModelRoutingAndRestoration(t *testing.T) {
	cfg := RemoteAgentConfig{Model: "writer", ReasoningEffort: "high", AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "medium"}}
	cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort}, cfg.AuditModel)
	session := "original-thread"
	for _, step := range []string{workflow.StepImplement, workflow.StepReport, workflow.StepAuditReport, workflow.StepAuditCode, workflow.StepPublish, workflow.StepPublishReport} {
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
	cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort}, cfg.AuditModel)
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
	cfg.models[workflow.StepAuditCode].Model = "caller-mutated-after-bridge-start"
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

func TestRemoteModelAssemblyPreservesConfigurationAndPromptEncoding(t *testing.T) {
	cfg := RemoteAgentConfig{AuditModel: &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "medium"}, Executable: "codex.exe", Model: "writer", ReasoningEffort: "high", RepositoryURL: "https://example.test/a/b.git"}
	const wire = `{"audit_model":{"model":"reviewer","reasoning_effort":"medium"},"executable":"codex.exe","model":"writer","reasoning_effort":"high","repository_url":"https://example.test/a/b.git"}`
	cfg.models = workflow.RemoteModelBindings(workflow.ModelSettings{Model: cfg.Model, ReasoningEffort: cfg.ReasoningEffort}, cfg.AuditModel)
	raw, err := json.Marshal(cfg)
	if err != nil || string(raw) != wire || RemoteConfigurationDigest(cfg) != fmt.Sprintf("%x", sha256.Sum256([]byte(wire))) {
		t.Fatal("frozen configuration encoding/digest changed", string(raw), err)
	}
	text := RemoteModelPrompt(strings.Repeat("a", 32), strings.Repeat("b", 64), workflow.StepAuditCode, "reviewer", "medium", "business\r\n")
	v, err := ParseRemoteModelPrompt(text)
	if err != nil || v.Step != workflow.StepAuditCode || v.BodyDigest != fmt.Sprintf("%x", sha256.Sum256([]byte("business"))) {
		t.Fatal("original prompt binding changed", v, err)
	}
	cfg.AuditModel.Model = "later-default"
	if cfg.models[workflow.StepAuditCode].Model != "reviewer" {
		t.Fatal("assembled model binding aliases mutable configuration")
	}
}
