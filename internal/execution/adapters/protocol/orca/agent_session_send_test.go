package orca

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSendPublishedFingerprints(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "acceptance", "evidence", "orca-structured-agent-session-p0-20261103", "contract-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Vectors []struct {
			Input struct {
				Method, SessionID string
				Fields            json.RawMessage
			}
			SHA256 string
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := AgentSessionClient{call: func(context.Context, string, any, bool) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"ok":true}`), nil
	}}
	for _, vector := range fixture.Vectors {
		if vector.Input.Method != "agentSession.send" {
			continue
		}
		var request AgentSend
		if err := json.Unmarshal(vector.Input.Fields, &request); err != nil {
			t.Fatal(err)
		}
		fence := int64(3)
		request.Envelope = AgentMutationEnvelope{SessionID: vector.Input.SessionID, ClientOperationID: originalAgentSend().Envelope.ClientOperationID, ExpectedRuntimeFence: &fence, PayloadFingerprint: vector.SHA256}
		if _, err := client.Send(context.Background(), request); err != nil {
			t.Fatal("installed public send fingerprint rejected", err)
		}
	}
	if calls != 2 {
		t.Fatal("published selected/default send vectors missing", calls)
	}
}

func originalAgentSend() AgentSend {
	fence := int64(3)
	request := AgentSend{
		Envelope:  AgentMutationEnvelope{SessionID: "oas_fixture", ClientOperationID: "1793680000000-00112233445566778899aabbccddeeff", ExpectedRuntimeFence: &fence},
		Body:      AgentMessage{Kind: "message", Role: "user", Blocks: []AgentTextBlock{{Type: "text", Text: "one\r\n\r\n中文😀\n```\npath\\with space\n```  \r\n"}}},
		Selection: &AgentSelection{Model: "fixture-model", Effort: "medium"},
	}
	request.Envelope.PayloadFingerprint, _ = AgentFingerprint("agentSession.send", request.Envelope.SessionID, map[string]any{"body": request.Body, "selection": request.Selection})
	return request
}

func TestAgentSendBindsBodySelectionAndOriginalFenceBeforeCall(t *testing.T) {
	for _, name := range []string{"session", "operation", "runner-operation-id", "nil-fence", "zero-fence", "negative-fence", "body", "selection", "fingerprint", "oversized", "not-user"} {
		t.Run(name, func(t *testing.T) {
			request := originalAgentSend()
			switch name {
			case "session":
				request.Envelope.SessionID = ""
			case "operation":
				request.Envelope.ClientOperationID = ""
			case "runner-operation-id":
				request.Envelope.ClientOperationID = "00112233445566778899aabbccddeeff"
			case "nil-fence":
				request.Envelope.ExpectedRuntimeFence = nil
			case "zero-fence":
				*request.Envelope.ExpectedRuntimeFence = 0
			case "negative-fence":
				*request.Envelope.ExpectedRuntimeFence = -1
			case "body":
				request.Body.Blocks[0].Text += "changed"
			case "selection":
				request.Selection.Effort = "high"
			case "fingerprint":
				request.Envelope.PayloadFingerprint = strings.Repeat("0", 64)
			case "oversized":
				request.Body.Blocks[0].Text = strings.Repeat("a", 256<<10)
			case "not-user":
				request.Body.Role = "assistant"
			}
			calls := 0
			client := AgentSessionClient{call: func(context.Context, string, any, bool) (json.RawMessage, error) {
				calls++
				return nil, errors.New("unexpected write")
			}}
			if _, err := client.Send(context.Background(), request); err == nil || calls != 0 {
				t.Fatal("unbound send reached transport", err, calls)
			}
		})
	}
}

func TestAgentSendPreservesWireShapeAndInnerRefusal(t *testing.T) {
	for _, selection := range []bool{false, true} {
		t.Run(map[bool]string{false: "host-default", true: "selected"}[selection], func(t *testing.T) {
			request := originalAgentSend()
			if !selection {
				request.Selection = nil
				request.Envelope.PayloadFingerprint, _ = AgentFingerprint("agentSession.send", request.Envelope.SessionID, map[string]any{"body": request.Body})
			}
			calls := 0
			client := AgentSessionClient{call: func(_ context.Context, method string, params any, mutation bool) (json.RawMessage, error) {
				calls++
				wire, err := json.Marshal(params)
				var shape map[string]json.RawMessage
				var got AgentSend
				if err != nil || json.Unmarshal(wire, &shape) != nil || json.Unmarshal(wire, &got) != nil {
					t.Fatal("invalid wire", err)
				}
				wantFields := 2
				if selection {
					wantFields++
				}
				if method != "agentSession.send" || !mutation || len(shape) != wantFields || string(shape["body"]) == "null" || got.Body.Blocks[0].Text != request.Body.Blocks[0].Text || *got.Envelope.ExpectedRuntimeFence != 3 || got.Envelope.ClientOperationID != request.Envelope.ClientOperationID || (got.Selection != nil) != selection {
					t.Fatal("body/identity changed or implicit queue/retry/message ID added", string(wire))
				}
				return json.RawMessage(`{"ok":false,"refusal":{"code":"fixture_refusal","message":"denied"}}`), nil
			}}
			_, err := client.Send(context.Background(), request)
			var refusal *AgentSessionRefusal
			if !errors.As(err, &refusal) || refusal.Code != "fixture_refusal" || calls != 1 {
				t.Fatal("inner refusal hidden or retried", err, calls)
			}
		})
	}
}

func TestAgentSendRequiresItsAdvertisedCapabilities(t *testing.T) {
	for _, required := range []string{"agent-session.structured.v1", "agent-session.host-authority.v1", "agent-session.session-boundary.v1", "agent-session.send-selection.v1"} {
		var incomplete []string
		for _, capability := range agentSessionCapabilities() {
			if capability != required {
				incomplete = append(incomplete, capability)
			}
		}
		if agentSendCapabilities(incomplete) == nil {
			t.Fatal("missing send capability admitted", required)
		}
	}
	if err := agentSendCapabilities(agentSessionCapabilities()); err != nil {
		t.Fatal(err)
	}
}
