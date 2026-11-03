package orca

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentFingerprintPublicVectors(t *testing.T) {
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
			CanonicalJSON, SHA256 string
			UTF8Bytes             int
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Vectors) != 5 {
		t.Fatal("public vectors missing")
	}
	for _, v := range fixture.Vectors {
		actual, err := AgentFingerprint(v.Input.Method, v.Input.SessionID, v.Input.Fields)
		if err != nil || actual != v.SHA256 {
			t.Fatal(v.Input.Method, actual, v.SHA256, err)
		}
		canonical, err := agentCanonical(map[string]any{"method": v.Input.Method, "sessionId": v.Input.SessionID, "fields": v.Input.Fields})
		if err != nil || string(canonical) != v.CanonicalJSON || len(canonical) != v.UTF8Bytes {
			t.Fatal("canonical bytes differ", err)
		}
	}
}

func TestAgentCreationRequiresAdvertisedAtomicCapabilities(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "acceptance", "evidence", "orca-structured-agent-session-mvp-20261103", "runner-normal-native-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		ID   string
		Meta struct{ RuntimeID string } `json:"_meta"`
	}
	if err = json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	result, err := stateResponse(raw, status.ID, status.Meta.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := localRuntimeCapabilities(result)
	if err = agentCreationCapabilities(capabilities); err != nil {
		t.Fatal("verified installed runtime rejected", err)
	}
	for _, missing := range []string{"agentSession.create.message.v1", "agentSession.create.options-mode.v1", "agent-session.send-selection.v1", "agent-session.permission-own.v1"} {
		var incomplete []string
		for _, capability := range capabilities {
			if capability != missing {
				incomplete = append(incomplete, capability)
			}
		}
		if agentCreationCapabilities(incomplete) == nil {
			t.Fatal("atomic contract silently downgraded", missing)
		}
	}
	if agentCreationCapabilities(nil) == nil {
		t.Fatal("unknown runtime admitted")
	}
}

func TestAgentHistoryRequiresCompletePublicEnvelope(t *testing.T) {
	for _, response := range []string{`{"ok":false,"refusal":{"code":"denied"}}`, `{"ok":true,"page":{"sessionId":"original","epoch":"epoch","items":[]}}`, `{"ok":true,"page":{"sessionId":"foreign","epoch":"epoch","items":[],"working":false,"hasOlder":false,"hasNewer":false}}`} {
		c := AgentSessionClient{call: func(context.Context, string, any, bool) (json.RawMessage, error) {
			return json.RawMessage(response), nil
		}}
		if _, err := c.History(context.Background(), "original", nil, 200); err == nil {
			t.Fatal("incomplete/foreign envelope admitted", response)
		}
	}
}

func TestAgentCreationPreservesOneBlockAndInnerRefusal(t *testing.T) {
	text := "one\r\n\r\n中文😀 \"quote\" \\path\n```\n<&>\u2028\u2029\n```  \r\n"
	create := AgentCreate{Worktree: "id:original", Agent: "codex", Options: map[string]string{"permissionMode": "full-access"}, Envelope: AgentMutationEnvelope{SessionID: "oas_original", ClientOperationID: "1793680000000-00112233445566778899aabbccddeeff"}, FirstMessage: AgentFirstMessage{ClientMessageID: "1793680000000-112233445566778899aabbccddeeff00", Body: AgentMessage{Kind: "message", Role: "user", Blocks: []AgentTextBlock{{Type: "text", Text: text}}}, Selection: &AgentSelection{Model: "requested-model", Effort: "medium"}}}
	create.Envelope.PayloadFingerprint, _ = AgentFingerprint("agentSession.create", create.Envelope.SessionID, map[string]any{"worktree": create.Worktree, "agent": create.Agent, "options": create.Options, "firstMessage": create.FirstMessage})
	calls := 0
	c := AgentSessionClient{call: func(_ context.Context, method string, params any, mutation bool) (json.RawMessage, error) {
		calls++
		got := params.(AgentCreate)
		if method != "agentSession.create" || !mutation || got.FirstMessage.Body.Blocks[0].Text != text || got.FirstMessage.Selection.Model != "requested-model" || got.FirstMessage.Selection.Effort != "medium" || got.Options["permissionMode"] != "full-access" || got.Envelope.ExpectedRuntimeFence != nil {
			t.Fatal("atomic selection/permission/body changed", got)
		}
		return json.RawMessage(`{"ok":false,"refusal":{"code":"fixture_refusal","message":"denied"}}`), nil
	}}
	_, err := c.Create(context.Background(), create)
	var refusal *AgentSessionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "fixture_refusal" || calls != 1 {
		t.Fatal(err, calls)
	}
	create.FirstMessage.Body.Blocks[0].Text = strings.Repeat("a", 256<<10)
	if _, err = c.Create(context.Background(), create); err == nil || calls != 1 {
		t.Fatal("oversized input dispatched", err, calls)
	}
}

func TestAgentIPCDispatchCheckpointPrecedesEveryByte(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit-failed", true: "revoked"}[cancelled], func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			denied := errors.New("durable commit failed")
			checks := 0
			ctx, cancel := context.WithCancel(WithDispatchCheckpoint(context.Background(), func() error { checks++; return denied }))
			defer cancel()
			if cancelled {
				cancel()
			}
			state := localState{conn: client, reader: bufio.NewReader(client), token: "synthetic", runtime: "original"}
			if _, err := state.requestWithCapabilities(ctx, "agentSession.create", nil, nil, true); err == nil {
				t.Fatal("write authorized")
			}
			_ = server.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
			var data [1]byte
			if n, _ := server.Read(data[:]); n != 0 {
				t.Fatal("bytes escaped before checkpoint")
			}
			if !cancelled && checks != 1 {
				t.Fatal(checks)
			}
		})
	}
}
