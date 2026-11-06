package livetest

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

func TestStructuredPipeForwardsOriginalAndDropsOnlyRealReceipt(t *testing.T) {
	root := t.TempDir()
	endpoint := fmt.Sprintf(`\\.\pipe\sr-structured-source-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	defer func() { listener.Close(); wg.Wait() }()
	var mu sync.Mutex
	creates, closes := 0, 0
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					raw, err := readAuditFrame(reader)
					if err != nil {
						return
					}
					var request struct{ ID, Method, AuthToken string }
					if json.Unmarshal(raw, &request) != nil {
						return
					}
					if request.AuthToken != "secret-original-token" {
						return
					}
					result := any(map[string]any{"ok": true})
					switch request.Method {
					case "status.get":
						result = map[string]any{"capabilities": []string{"agent-session.structured.v1", "agent-session.host-authority.v1", "agent-session.session-boundary.v1", "agentSession.create.message.v1", "agentSession.create.options-mode.v1", "agent-session.send-selection.v1", "agent-session.permission-modes.v1", "agent-session.permission-own.v1", "agent-session.turn-item.v1", "agent-session.item-detail.v1", "agent-session.current-requests.v1"}}
					case "agentSession.create":
						mu.Lock()
						creates++
						mu.Unlock()
					case "agentSession.close":
						mu.Lock()
						closes++
						mu.Unlock()
					}
					response, _ := json.Marshal(map[string]any{"id": request.ID, "ok": true, "_meta": map[string]string{"runtimeId": "runtime"}, "result": result})
					if _, err := conn.Write(append(response, '\n')); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	body, _ := json.Marshal(map[string]any{"runtimeId": "runtime", "authToken": "secret-original-token", "transports": []any{map[string]string{"kind": "named-pipe", "endpoint": endpoint}}})
	if err := os.WriteFile(filepath.Join(root, "orca-runtime.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	proxyRoot := t.TempDir()
	journal := filepath.Join(t.TempDir(), "calls.jsonl")
	audit, err := StartStructuredPipeAudit(proxyRoot, root, journal)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := audit.Close(); err != nil {
			t.Error(err)
		}
	}()
	t.Setenv("ORCA_USER_DATA_PATH", proxyRoot)
	client := native.NewLocalAgentSessionClient("runtime")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	create := native.AgentCreate{Envelope: native.AgentMutationEnvelope{SessionID: "original-session", ClientOperationID: "original-create"}, Worktree: "id:worktree", Agent: "codex", Options: map[string]string{"mode": "full_access"}, FirstMessage: native.AgentFirstMessage{ClientMessageID: "original-message", Body: native.AgentMessage{Kind: "message", Role: "user", Blocks: []native.AgentTextBlock{{Type: "text", Text: "original input"}}}}}
	create.Envelope.PayloadFingerprint, err = native.AgentFingerprint("agentSession.create", create.Envelope.SessionID, map[string]any{"worktree": create.Worktree, "agent": create.Agent, "options": create.Options, "firstMessage": create.FirstMessage})
	if err != nil {
		t.Fatal(err)
	}
	if err = audit.DropNext("agentSession.create"); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Create(ctx, create); err == nil {
		t.Fatal("actual response loss was not injected")
	}
	rows, err := audit.Calls("original-session")
	if err != nil || len(rows) != 1 || !rows[0].Dropped || len(rows[0].Response) == 0 {
		t.Fatal(rows, err)
	}
	sum := sha256.Sum256([]byte(create.FirstMessage.Body.Blocks[0].Text))
	run := StructuredRun{Binding: StructuredBinding{SessionID: create.Envelope.SessionID, CreateID: create.Envelope.ClientOperationID, MessageID: create.FirstMessage.ClientMessageID, InputDigest: hex.EncodeToString(sum[:])}}
	if err := VerifyStructuredInputCalls(rows, run); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStructuredInputCalls(append(rows, rows[0]), run); err == nil {
		t.Fatal("idempotent duplicate wire dispatch was hidden")
	}
	// A new client emulates service replacement. Only a read/close follows the
	// lost create; the test recorder must not replay it or synthesize success.
	next := native.NewLocalAgentSessionClient("runtime")
	if _, err = next.Close(ctx, "original-session"); err != nil {
		t.Fatal(err)
	}
	rows, err = audit.Calls("original-session")
	if err != nil || len(rows) != 2 || rows[1].Method != "agentSession.close" {
		t.Fatal(rows, err)
	}
	mu.Lock()
	actualCreates, actualCloses := creates, closes
	mu.Unlock()
	if actualCreates != 1 || actualCloses != 1 {
		t.Fatal(actualCreates, actualCloses)
	}
	raw, err := os.ReadFile(journal)
	if err != nil || strings.Contains(string(raw), "secret-original-token") || strings.Contains(string(raw), "authToken") {
		t.Fatal("credentials leaked to evidence", err)
	}
	// Failed independent history reads cannot become native facts.
	audit.BlockHistory(true)
	if _, err := next.History(ctx, "original-session", nil, 200); err == nil {
		t.Fatal("history gap hidden")
	}
	audit.BlockHistory(false)
	original, err := os.ReadFile(filepath.Join(root, "orca-runtime.json"))
	if err != nil || string(original) != string(body) {
		t.Fatal("original bootstrap changed", err)
	}
}
