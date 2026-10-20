package codexworker

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/erranliu/SteerRunner/internal/execution/model"
)

func RemoteTurnModel(source io.Reader, turnID string) (model.Settings, error) {
	var choice model.Settings
	found := false
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		var row struct {
			Type    string `json:"type"`
			Payload struct {
				Turn   string `json:"turn_id"`
				Model  string `json:"model"`
				Effort string `json:"effort"`
			} `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return choice, errors.New("native_model_history_invalid")
		}
		if row.Type != "turn_context" || row.Payload.Turn != turnID {
			continue
		}
		next := model.Settings{Model: row.Payload.Model, ReasoningEffort: row.Payload.Effort}
		if next.Model == "" || !ValidReasoningEffort(next.ReasoningEffort) || found && next != choice {
			return choice, errors.New("native_model_history_mismatch")
		}
		choice = next
		found = true
	}
	if scanner.Err() != nil || !found {
		return choice, errors.New("native_model_history_missing")
	}
	return choice, nil
}

const remoteModelPrefix = "[SteerRunner model="
const remoteModelTokenEnv = "STEERRUNNER_REMOTE_MODEL_BRIDGE_TOKEN"

type RemoteModelInput struct {
	Command         string `json:"command"`
	InputDigest     string `json:"input_digest"`
	BodyDigest      string `json:"body_digest"`
	Step            string `json:"step"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
}

func RemoteModelPrompt(command, digest, step, model, effort, prompt string) string {
	body := strings.TrimRight(strings.ReplaceAll(prompt, "\r\n", "\n"), "\r\n")
	raw, _ := json.Marshal(RemoteModelInput{Command: command, InputDigest: digest, BodyDigest: fmt.Sprintf("%x", sha256.Sum256([]byte(body))), Step: step, Model: model, ReasoningEffort: effort})
	return remoteModelPrefix + base64.RawURLEncoding.EncodeToString(raw) + "]\n" + body
}

func ParseRemoteModelPrompt(text string) (RemoteModelInput, error) {
	var v RemoteModelInput
	first, body, ok := strings.Cut(text, "\n")
	if !ok || !strings.HasPrefix(first, remoteModelPrefix) || !strings.HasSuffix(first, "]") {
		return v, errors.New("remote_model_input_missing")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(first, remoteModelPrefix), "]"))
	if err != nil {
		return v, errors.New("remote_model_input_invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&v) != nil || len(v.Command) != 32 || strings.Trim(v.Command, "0123456789abcdef") != "" || len(v.InputDigest) != 64 || strings.Trim(v.InputDigest, "0123456789abcdef") != "" || v.BodyDigest != fmt.Sprintf("%x", sha256.Sum256([]byte(body))) {
		return v, errors.New("remote_model_input_invalid")
	}
	canonical, _ := json.Marshal(v)
	if string(canonical) != string(raw) {
		return v, errors.New("remote_model_input_noncanonical")
	}
	return v, nil
}

func (c RemoteAgentConfig) validateModels() error {
	if c.AuditModel == nil {
		return nil
	}
	if c.Model == "" || c.Model == c.AuditModel.Model || strings.TrimSpace(c.AuditModel.Model) != c.AuditModel.Model || c.AuditModel.Model == "" || strings.ContainsAny(c.AuditModel.Model, "\x00\r\n\t") || !ValidReasoningEffort(c.ReasoningEffort) || !ValidReasoningEffort(c.AuditModel.ReasoningEffort) {
		return errors.New("remote_audit_models_invalid")
	}
	return nil
}

// The native TUI still owns the original thread and every input. This bridge
// supplies only the frozen workflow's per-turn preferences to that original
// server; a model suggestion in ordinary prompt prose has no effect.
func rewriteRemoteModelRequest(raw []byte, c RemoteAgentConfig, session string) ([]byte, error) {
	var rpc map[string]json.RawMessage
	if json.Unmarshal(raw, &rpc) != nil {
		return nil, errors.New("remote_model_rpc_invalid")
	}
	var method string
	_ = json.Unmarshal(rpc["method"], &method)
	switch method {
	case "thread/start", "thread/fork", "turn/steer", "thread/rollback":
		return nil, errors.New("remote_model_original_thread_only")
	}
	if method == "thread/resume" {
		var p struct {
			Thread string `json:"threadId"`
		}
		if json.Unmarshal(rpc["params"], &p) != nil || p.Thread != session {
			return nil, errors.New("remote_model_original_thread_only")
		}
	}
	if method != "turn/start" {
		return raw, nil
	}
	var p map[string]json.RawMessage
	if json.Unmarshal(rpc["params"], &p) != nil {
		return nil, errors.New("remote_model_rpc_invalid")
	}
	var thread string
	_ = json.Unmarshal(p["threadId"], &thread)
	var input []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if thread != session || json.Unmarshal(p["input"], &input) != nil || len(input) != 1 || input[0].Type != "text" {
		return nil, errors.New("remote_model_original_input_required")
	}
	v, err := ParseRemoteModelPrompt(input[0].Text)
	if err != nil {
		return nil, err
	}
	expected, known := c.models[v.Step]
	if !known {
		return nil, errors.New("remote_model_step_invalid")
	}
	if expected == nil {
		return nil, errors.New("remote_audit_model_missing")
	}
	if v.Model != expected.Model || v.ReasoningEffort != expected.ReasoningEffort {
		return nil, errors.New("remote_model_choice_not_frozen")
	}
	p["model"], _ = json.Marshal(expected.Model)
	p["effort"], _ = json.Marshal(expected.ReasoningEffort)
	// Native TUI collaboration settings take precedence over turn preferences.
	// Preserve the original mode/instructions while freezing the same model at
	// both levels, including restoration to the writer after an audit.
	if modeRaw := p["collaborationMode"]; len(modeRaw) != 0 && string(modeRaw) != "null" {
		var mode, settings map[string]json.RawMessage
		if json.Unmarshal(modeRaw, &mode) != nil || mode == nil || json.Unmarshal(mode["settings"], &settings) != nil || settings == nil {
			return nil, errors.New("remote_model_collaboration_invalid")
		}
		settings["model"], _ = json.Marshal(expected.Model)
		settings["reasoning_effort"], _ = json.Marshal(expected.ReasoningEffort)
		mode["settings"], _ = json.Marshal(settings)
		p["collaborationMode"], _ = json.Marshal(mode)
	}
	p["clientUserMessageId"], _ = json.Marshal(v.Command)
	rpc["params"], _ = json.Marshal(p)
	return json.Marshal(rpc)
}

func remoteModelToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func startRemoteModelBridge(ctx context.Context, c RemoteAgentConfig, upstream, session, token string) (string, func(), error) {
	c.models = c.models.Clone()
	if err := c.validateModels(); err != nil {
		return "", nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	if len(token) < 32 {
		listener.Close()
		return "", nil, errors.New("remote_model_transport_token_invalid")
	}
	lifetime, cancel := context.WithCancel(ctx)
	var mu sync.Mutex
	active := false
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if r.URL.RequestURI() != "/" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 || active {
			mu.Unlock()
			http.Error(w, "unavailable", http.StatusForbidden)
			return
		}
		active = true
		mu.Unlock()
		defer func() { mu.Lock(); active = false; mu.Unlock() }()
		downstream, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer downstream.CloseNow()
		downstream.SetReadLimit(2 << 20)
		upstreamConn, _, err := websocket.Dial(lifetime, upstream, nil)
		if err != nil {
			return
		}
		defer upstreamConn.CloseNow()
		upstreamConn.SetReadLimit(16 << 20)
		connection, cancelConnection := context.WithCancel(lifetime)
		defer cancelConnection()
		done := make(chan struct{}, 1)
		go func() {
			defer func() { done <- struct{}{}; cancelConnection() }()
			for {
				kind, body, e := upstreamConn.Read(connection)
				if e != nil {
					return
				}
				if downstream.Write(connection, kind, body) != nil {
					return
				}
			}
		}()
		for {
			kind, body, e := downstream.Read(connection)
			if e != nil {
				break
			}
			body, e = rewriteRemoteModelRequest(body, c, session)
			if e != nil {
				break
			}
			if upstreamConn.Write(connection, kind, body) != nil {
				break
			}
		}
		cancelConnection()
		<-done
	})
	go func() { _ = server.Serve(listener) }()
	return "ws://" + listener.Addr().String(), func() { cancel(); _ = server.Close() }, nil
}
