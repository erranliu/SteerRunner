package application

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestReadNativeConversationUsesOriginalBinding(t *testing.T) {
	const runtime = "original-runtime"
	runID, attemptID := contracts.NewID(), contracts.NewID()
	hash := sha256.Sum256([]byte(runID + "\x00" + attemptID))
	target := contracts.NativeSessionTarget{
		RunID: runID, AttemptID: attemptID, SessionID: fmt.Sprintf("oas_%x", hash[:16]), Generation: "original-generation",
	}
	profile := orca.Profile{Executable: "orca.exe", WaitTimeout: "1s"}
	workspace := t.TempDir()
	snapshot := b.LifecycleSnapshot{
		Ref: b.ExecutionRef{
			Backend: b.BackendOrca, Protocol: b.ProtocolOrcaAgentSessionV1, Target: profile.Target(),
			RunID: runID, AttemptID: attemptID, Agent: b.AgentSpec{Name: "codex"},
			WorkspacePath: workspace,
			External: b.ProcessIdentity{Kind: "orca_agent_session", RuntimeID: runtime, WorktreeID: "worktree",
				Executable: profile.Executable, ExecutionHost: profile.Target().ExecutionHost},
		},
		Session:            b.SessionContext{ProjectID: "project", WorkflowRunID: runID, ExecutionID: runID, AttemptID: attemptID, SessionID: target.SessionID},
		ResourceGeneration: target.Generation, ConfigurationDigest: "config",
		Workspace: b.WorkspaceIntent{Path: workspace},
	}
	profileEnvelope := conversationTestEnvelope(t, b.ImplementationDataEnvelope{
		CodecVersion: orca.AgentSessionVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.AgentSessionProfileKey,
		ProjectID: snapshot.Session.ProjectID, ProfileID: runID, AcceptedConfigVersion: "input",
	}, orca.AgentSessionProfile{Transport: profile, AccessMode: "full_access"})
	snapshot.Implementation = []b.ImplementationDataEnvelope{conversationTestEnvelope(t, b.ImplementationDataEnvelope{
		CodecVersion: orca.AgentSessionVersion, Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.AgentSessionStateKey,
		ProjectID: snapshot.Session.ProjectID, ExecutionID: runID, AttemptID: attemptID,
		Generation: snapshot.ResourceGeneration, ConfigDigest: snapshot.ConfigurationDigest,
	}, map[string]any{
		"runtime_id": runtime, "session_id": target.SessionID, "epoch": "original-epoch",
		"worktree": native.Worktree{ID: "worktree", InstanceID: "instance", Path: workspace},
	})}
	record := &runstate.Record{ID: runID, InputDigest: "input", Snapshot: snapshot, Profile: []b.ImplementationDataEnvelope{profileEnvelope}}
	e := &Engine{ctx: t.Context(), logger: log.New(io.Discard, "", 0), runs: map[string]*runstate.Record{runID: record}}

	text := strings.Repeat("正文", 800)
	message := native.AgentItem{ItemID: "answer", Sequence: 2, Revision: 1,
		Body: json.RawMessage(fmt.Sprintf(`{"kind":"message","role":"assistant","blocks":[{"type":"text","text":%q}]}`, text))}
	tail := native.AgentPage{SessionID: target.SessionID, Epoch: "original-epoch", Items: []native.AgentItem{message}, HasOlder: true}
	tail.Window.NextCursor = &native.AgentCursor{Epoch: tail.Epoch, Sequence: 1}
	older := native.AgentPage{SessionID: target.SessionID, Epoch: tail.Epoch, HasNewer: true, Items: []native.AgentItem{{
		ItemID: "earlier", Sequence: 1, Body: json.RawMessage(`{"kind":"message","role":"assistant","blocks":[{"type":"text","text":"Earlier"}]}`),
	}}}
	var calls atomic.Int32
	var hold atomic.Bool
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	conversationTestPipe(t, runtime, func(method string, params json.RawMessage) any {
		calls.Add(1)
		var query struct{ SessionID, Direction, ItemID string }
		if err := json.Unmarshal(params, &query); err != nil {
			t.Errorf("invalid %s parameters: %v", method, err)
			return nil
		}
		if method != "status.get" && query.SessionID != target.SessionID {
			t.Errorf("unexpected session %q", query.SessionID)
			return nil
		}
		switch method {
		case "status.get":
			return map[string]any{}
		case "agentSession.history":
			if hold.Load() {
				close(started)
				select {
				case <-release:
				case <-t.Context().Done():
				}
			}
			if query.Direction == "before" {
				return map[string]any{"ok": true, "page": older}
			}
			return map[string]any{"ok": true, "page": tail}
		case "agentSession.item":
			if query.ItemID == message.ItemID {
				return map[string]any{"item": message}
			}
		}
		t.Errorf("unexpected native request: %s %s", method, params)
		return nil
	})

	first := requireConversationPage(t, e.ReadNativeConversation(target, "", 0))
	if len(first.Messages) != 1 || first.Messages[0].ID != message.ItemID || first.Messages[0].BodyCursor == "" || first.NextCursor == "" {
		t.Fatal("missing original page or cursor", first)
	}
	previous := requireConversationPage(t, e.ReadNativeConversation(target, first.NextCursor, 0))
	if len(previous.Messages) != 1 || previous.Messages[0].Text != "Earlier" {
		t.Fatal("wrong older page", previous)
	}
	body := requireConversationPage(t, e.ReadNativeConversation(target, first.Messages[0].BodyCursor, 0))
	if len(body.Messages) != 1 || body.Messages[0].Text != text || body.BodyOffset == nil || *body.BodyOffset != 0 {
		t.Fatal("wrong original message body")
	}

	before := calls.Load()
	wrong := target
	wrong.AttemptID = "foreign"
	requireConversationError(t, e.ReadNativeConversation(wrong, "", 0), "stale_revision")
	requireConversationError(t, e.ReadNativeConversation(target, "invalid", 0), "invalid_cursor")
	legacy := record.Clone()
	legacy.LaunchCommand = "retired-agent-launch"
	e.runs[runID] = legacy
	requireConversationError(t, e.ReadNativeConversation(target, "", 0), "environment_unavailable")
	if calls.Load() != before {
		t.Fatal("invalid or retired binding reached Orca")
	}
	e.runs[runID] = record

	hold.Store(true)
	result := make(chan contracts.Response, 1)
	go func() { result <- e.ReadNativeConversation(target, "", 0) }()
	select {
	case <-started:
	case response := <-result:
		t.Fatal("read returned before reaching Orca", response)
	case <-time.After(5 * time.Second):
		t.Fatal("read never reached Orca")
	}
	replacement := record.Clone()
	replacement.Snapshot.ResourceGeneration = "replacement-generation"
	e.mu.Lock()
	e.runs[runID] = replacement
	e.mu.Unlock()
	// Release the server while leaving cleanup safe if a preceding assertion failed.
	release <- struct{}{}
	requireConversationError(t, <-result, "stale_revision")
}

func conversationTestEnvelope(t *testing.T, envelope b.ImplementationDataEnvelope, value any) b.ImplementationDataEnvelope {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = b.NewImplementationDataEnvelope(envelope, body)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func requireConversationPage(t *testing.T, response contracts.Response) contracts.NativeConversationPage {
	t.Helper()
	page, ok := response.Data.(contracts.NativeConversationPage)
	if !response.OK || !ok {
		t.Fatalf("expected original conversation page, error: %+v, data: %+v", response.Error, response.Data)
	}
	return page
}

func requireConversationError(t *testing.T, response contracts.Response, code string) {
	t.Helper()
	if response.OK || response.Error == nil || response.Error.Code != code || response.Data != nil {
		t.Fatalf("expected %s without conversation data, got %+v", code, response)
	}
}

func conversationTestPipe(t *testing.T, runtime string, handle func(string, json.RawMessage) any) {
	t.Helper()
	endpoint := `\\.\pipe\sr-conversation-` + contracts.NewID()
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	const token = "test-token"
	metadata, err := json.Marshal(map[string]any{"runtimeId": runtime, "authToken": token,
		"transports": []map[string]string{{"kind": "named-pipe", "endpoint": endpoint}}})
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), metadata, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		listener.Close()
		<-done
	})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var request struct {
						ID, Method, AuthToken string
						Params                json.RawMessage
					}
					if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.AuthToken != token {
						t.Errorf("invalid fixture request or token: %v", err)
						return
					}
					result := handle(request.Method, request.Params)
					if err := json.NewEncoder(conn).Encode(map[string]any{"id": request.ID, "ok": true,
						"result": result, "_meta": map[string]string{"runtimeId": runtime}}); err != nil {
						t.Error(err)
						return
					}
				}
			}()
		}
	}()
}
