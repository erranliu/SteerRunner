package livetest

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

// StructuredPipeAudit forwards native bytes to the original authenticated host.
// It is test-only and outlives Runner service replacements. No response is made up.
type StructuredPipeAudit struct {
	Root, OriginalRoot string
	listener           net.Listener
	upstream           string
	journal            *os.File
	mu                 sync.Mutex
	drop               string
	blockHistory       bool
	rows               []StructuredCall
	conns              map[net.Conn]bool
	failure            error
	wg                 sync.WaitGroup
	closed             bool
	stop               chan struct{}
}
type StructuredCall struct {
	Method   string          `json:"method"`
	Session  string          `json:"session_id"`
	ID       string          `json:"request_id"`
	Params   json.RawMessage `json:"params"`
	Response json.RawMessage `json:"response,omitempty"`
	Dropped  bool            `json:"response_dropped,omitempty"`
}

func StartStructuredPipeAudit(root, originalRoot, journal string) (*StructuredPipeAudit, error) {
	raw, err := os.ReadFile(filepath.Join(originalRoot, "orca-runtime.json"))
	if err != nil {
		return nil, err
	}
	var metadata struct {
		RuntimeID, AuthToken string
		Transports           []struct{ Kind, Endpoint string }
		Transport            *struct{ Kind, Endpoint string }
	}
	if json.Unmarshal(raw, &metadata) != nil || metadata.RuntimeID == "" || metadata.AuthToken == "" {
		return nil, errors.New("original IPC metadata missing")
	}
	endpoint := ""
	if metadata.Transports != nil {
		for _, t := range metadata.Transports {
			if t.Kind == "named-pipe" {
				endpoint = t.Endpoint
				break
			}
		}
	} else if metadata.Transport != nil && metadata.Transport.Kind == "named-pipe" {
		endpoint = metadata.Transport.Endpoint
	}
	if endpoint == "" {
		return nil, errors.New("original named pipe missing")
	}
	proxy := fmt.Sprintf(`\\.\pipe\sr-structured-audit-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(proxy, nil)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(journal, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		listener.Close()
		return nil, err
	}
	audit := &StructuredPipeAudit{Root: root, OriginalRoot: originalRoot, listener: listener, upstream: endpoint, journal: f, conns: map[net.Conn]bool{}, stop: make(chan struct{})}
	if err = os.MkdirAll(root, 0700); err == nil {
		// A private metadata file contains credentials; evidence never receives it.
		body, _ := json.Marshal(map[string]any{"runtimeId": metadata.RuntimeID, "authToken": metadata.AuthToken, "transports": []any{map[string]string{"kind": "named-pipe", "endpoint": proxy}}})
		err = os.WriteFile(filepath.Join(root, "orca-runtime.json"), body, 0600)
	}
	if err != nil {
		listener.Close()
		f.Close()
		return nil, err
	}
	audit.wg.Add(1)
	go func() {
		defer audit.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			audit.mu.Lock()
			if audit.closed {
				audit.mu.Unlock()
				conn.Close()
				return
			}
			audit.conns[conn] = true
			audit.wg.Add(1)
			audit.mu.Unlock()
			go audit.serve(conn)
		}
	}()
	return audit, nil
}
func (a *StructuredPipeAudit) fail(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure == nil {
		a.failure = err
	}
}
func (a *StructuredPipeAudit) serve(conn net.Conn) {
	defer a.wg.Done()
	defer func() { conn.Close(); a.mu.Lock(); delete(a.conns, conn); a.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	upstream, err := winio.DialPipeContext(ctx, a.upstream)
	if err != nil {
		a.fail(err)
		return
	}
	defer upstream.Close()
	// Every production unary call has a bounded lifetime. Closing the fixture
	// closes both sides, including a response withheld after actual dispatch.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-a.stop:
		}
		upstream.Close()
	}()
	_ = upstream.SetDeadline(time.Now().Add(90 * time.Second))
	input, output := bufio.NewReader(conn), bufio.NewReader(upstream)
	for {
		raw, err := readAuditFrame(input)
		if err != nil {
			return
		}
		var request struct {
			ID, Method string
			Params     json.RawMessage
		}
		if json.Unmarshal(raw, &request) != nil {
			a.fail(errors.New("invalid forwarded request"))
			return
		}
		var params struct {
			SessionID string
			Envelope  struct{ SessionID string }
		}
		_ = json.Unmarshal(request.Params, &params)
		if params.SessionID == "" {
			params.SessionID = params.Envelope.SessionID
		}
		a.mu.Lock()
		blocked := a.blockHistory && request.Method == "agentSession.history"
		a.mu.Unlock()
		if blocked {
			return
		}
		row := StructuredCall{Method: request.Method, Session: params.SessionID, ID: request.ID, Params: request.Params}
		mutation := request.Method == "agentSession.create" || request.Method == "agentSession.send" || request.Method == "agentSession.close"
		index := -1
		if mutation {
			a.mu.Lock()
			index = len(a.rows)
			a.rows = append(a.rows, row)
			err = a.write(row)
			a.mu.Unlock()
			if err != nil {
				a.fail(err)
				return
			}
		}
		if _, err = upstream.Write(append(raw, '\n')); err != nil {
			return
		}
		for {
			response, err := readAuditFrame(output)
			if err != nil {
				return
			}
			var envelope struct {
				ID     string
				OK     bool
				Result struct{ OK bool }
			}
			if json.Unmarshal(response, &envelope) != nil {
				a.fail(errors.New("invalid forwarded response"))
				return
			}
			drop := false
			if envelope.ID == request.ID && mutation {
				row.Response = append([]byte(nil), response...)
				a.mu.Lock()
				if a.drop == request.Method && envelope.OK && envelope.Result.OK {
					a.drop = ""
					drop = true
					row.Dropped = true
				}
				a.rows[index] = row
				err = a.write(row)
				a.mu.Unlock()
				if err != nil {
					a.fail(err)
					return
				}
			}
			if drop {
				return
			}
			if _, err = conn.Write(append(response, '\n')); err != nil {
				return
			}
			if envelope.ID == request.ID {
				break
			}
		}
	}
}
func readAuditFrame(r *bufio.Reader) ([]byte, error) {
	var out []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(out)+len(part) > 4<<20 {
			return nil, errors.New("audit frame exceeds limit")
		}
		out = append(out, part...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return out[:len(out)-1], nil
	}
}
func (a *StructuredPipeAudit) write(row StructuredCall) error {
	raw, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = a.journal.Write(append(raw, '\n'))
	return err
}
func (a *StructuredPipeAudit) DropNext(method string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.drop != "" || method != "agentSession.create" && method != "agentSession.send" {
		return errors.New("invalid pending loss injection")
	}
	a.drop = method
	return nil
}
func (a *StructuredPipeAudit) Calls(session string) ([]StructuredCall, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var rows []StructuredCall
	for _, row := range a.rows {
		if session == "" || row.Session == session {
			rows = append(rows, row)
		}
	}
	return rows, a.failure
}
func (a *StructuredPipeAudit) Close() error {
	a.mu.Lock()
	a.closed = true
	close(a.stop)
	for conn := range a.conns {
		conn.Close()
	}
	a.mu.Unlock()
	a.listener.Close()
	a.wg.Wait()
	err := a.journal.Close()
	if a.failure != nil {
		return a.failure
	}
	return err
}

func (a *StructuredPipeAudit) BlockHistory(block bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blockHistory = block
}

// VerifyStructuredInputCalls compares actual local dispatches with original
// committed inputs. A server's idempotent duplicate response cannot hide a replay.
func VerifyStructuredInputCalls(rows []StructuredCall, run StructuredRun) error {
	if run.Agent != "codex" && run.Agent != "pi" {
		return errors.New("original structured agent missing")
	}
	expected := map[string]StructuredBinding{}
	for _, binding := range append(append([]StructuredBinding{}, run.Inputs...), run.Binding) {
		expected[binding.MessageID] = binding
	}
	seen := map[string]bool{}
	creates := 0
	for _, row := range rows {
		if row.Method != "agentSession.create" && row.Method != "agentSession.send" {
			continue
		}
		var envelope native.AgentMutationEnvelope
		var body native.AgentMessage
		var message string
		if row.Method == "agentSession.create" {
			var request native.AgentCreate
			if json.Unmarshal(row.Params, &request) != nil {
				return errors.New("invalid actual create request")
			}
			envelope, body, message = request.Envelope, request.FirstMessage.Body, request.FirstMessage.ClientMessageID
			creates++
			fingerprint, err := native.AgentFingerprint("agentSession.create", envelope.SessionID, map[string]any{"worktree": request.Worktree, "agent": request.Agent, "options": request.Options, "firstMessage": request.FirstMessage})
			if expected[message].CreateID != envelope.ClientOperationID || run.Binding.Worktree.ID == "" || request.Worktree != "id:"+run.Binding.Worktree.ID || request.Agent != run.Agent || err != nil || fingerprint != envelope.PayloadFingerprint {
				return errors.New("actual create differs from original target or operation")
			}
			if run.Agent == "codex" && (len(request.Options) != 1 || request.Options["permissionMode"] != "full-access") || run.Agent == "pi" && len(request.Options) != 0 {
				return errors.New("actual create permission options changed")
			}
		} else {
			var request native.AgentSend
			if json.Unmarshal(row.Params, &request) != nil {
				return errors.New("invalid actual send request")
			}
			envelope, body, message = request.Envelope, request.Body, request.Envelope.ClientOperationID
			fields := map[string]any{"body": body}
			if request.Selection != nil {
				fields["selection"] = request.Selection
			}
			fingerprint, err := native.AgentFingerprint("agentSession.send", envelope.SessionID, fields)
			if envelope.ExpectedRuntimeFence == nil || *envelope.ExpectedRuntimeFence <= 0 || err != nil || fingerprint != envelope.PayloadFingerprint {
				return errors.New("actual send envelope changed")
			}
		}
		binding, ok := expected[message]
		if !ok || seen[message] || row.Session != run.Binding.SessionID || envelope.SessionID != run.Binding.SessionID || body.ValidateInput() != nil {
			return errors.New("unexpected or repeated structured dispatch")
		}
		hash := sha256.Sum256([]byte(body.Blocks[0].Text))
		if hex.EncodeToString(hash[:]) != binding.InputDigest {
			return errors.New("actual input differs from original body")
		}
		seen[message] = true
	}
	if creates != 1 || len(seen) != len(expected) {
		return errors.New("original structured dispatch coverage incomplete")
	}
	return nil
}
