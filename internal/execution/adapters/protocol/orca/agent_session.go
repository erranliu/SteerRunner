package orca

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// AgentSessionClient contains only the public native conversation operations
// consumed by the structured execution adapter, independent of Runner state.
type AgentSessionClient struct {
	call func(context.Context, string, any, bool) (json.RawMessage, error)
}

type agentSessionConnection interface {
	requestWithCapabilities(context.Context, string, any, []string, bool) (string, error)
	next(context.Context, string) (json.RawMessage, error)
	close() error
}

// callAgentSession owns one verified connection and never retries a request.
// The transport still consumes mutation authorization at its actual write.
func callAgentSession(ctx context.Context, conn agentSessionConnection, advertised, capabilities []string, method string, params any, mutation bool) (json.RawMessage, error) {
	defer conn.close()
	switch method {
	case "agentSession.createSupport", "agentSession.create":
		if err := agentCreationCapabilities(advertised); err != nil {
			return nil, err
		}
	case "agentSession.send":
		if err := agentSendCapabilities(advertised); err != nil {
			return nil, err
		}
	}
	id, err := conn.requestWithCapabilities(ctx, method, params, capabilities, mutation)
	if err != nil {
		return nil, err
	}
	return conn.next(ctx, id)
}

func agentSessionCapabilities() []string {
	return []string{
		"agent-session.structured.v1", "agent-session.host-authority.v1",
		"agent-session.session-boundary.v1", "agentSession.create.message.v1", "agentSession.create.options-mode.v1",
		"agent-session.send-selection.v1", "agent-session.permission-modes.v1",
		"agent-session.permission-own.v1", "agent-session.turn-item.v1",
		"agent-session.item-detail.v1", "agent-session.current-requests.v1",
	}
}

func remoteAgentSessionCapabilities() []string {
	return append(agentSessionCapabilities(), "agent-session.structured.client-launch-mode.v1", "agent-session.structured.registered-agents.v1")
}

func NewLocalAgentSessionClient(runtime string) AgentSessionClient {
	return newLocalAgentSessionClient(runtime, agentSessionCapabilities())
}

func NewLocalPiAgentSessionClient(runtime string) AgentSessionClient {
	return newLocalAgentSessionClient(runtime, remoteAgentSessionCapabilities())
}

func newLocalAgentSessionClient(runtime string, capabilities []string) AgentSessionClient {
	return AgentSessionClient{call: func(ctx context.Context, method string, params any, mutation bool) (json.RawMessage, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		connection, err := openLocalState(ctx, runtime)
		if err != nil {
			return nil, err
		}
		return callAgentSession(ctx, connection, connection.capabilities, capabilities, method, params, mutation)
	}}
}

// NewRemoteAgentSessionClient uses the existing explicit pairing and pins the
// original host key and runtime on every unary connection. It never retries a
// mutation, including when a write succeeded but the response was lost.
func NewRemoteAgentSessionClient(environment, runtime, expectedHost string) AgentSessionClient {
	return AgentSessionClient{call: func(ctx context.Context, method string, params any, mutation bool) (json.RawMessage, error) {
		if environment == "" || runtime == "" || expectedHost == "" {
			return nil, errors.New("incomplete original remote Orca identity")
		}
		pairing, err := resolveStatePairing(environment)
		if err != nil {
			return nil, err
		}
		key, err := base64.StdEncoding.DecodeString(pairing.PublicKey)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(key)
		if "sha256:"+hex.EncodeToString(digest[:]) != expectedHost {
			return nil, stateIdentityError("original paired Orca host changed")
		}
		capabilities := remoteAgentSessionCapabilities()
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		connection, err := openStateSocketWithCapabilities(ctx, pairing, runtime, capabilities)
		if err != nil {
			return nil, err
		}
		return callAgentSession(ctx, connection, connection.capabilities, capabilities, method, params, mutation)
	}}
}

func agentCreationCapabilities(advertised []string) error {
	for _, required := range []string{"agent-session.structured.v1", "agent-session.host-authority.v1", "agent-session.session-boundary.v1", "agentSession.create.message.v1", "agentSession.create.options-mode.v1", "agent-session.send-selection.v1", "agent-session.permission-modes.v1", "agent-session.permission-own.v1", "agent-session.turn-item.v1", "agent-session.item-detail.v1"} {
		if !slices.Contains(advertised, required) {
			return fmt.Errorf("original Orca runtime lacks required structured capability %s; no input sent", required)
		}
	}
	return nil
}

func agentSendCapabilities(advertised []string) error {
	for _, required := range []string{"agent-session.structured.v1", "agent-session.host-authority.v1", "agent-session.session-boundary.v1", "agent-session.send-selection.v1"} {
		if !slices.Contains(advertised, required) {
			return fmt.Errorf("original Orca runtime lacks required send capability %s; no input sent", required)
		}
	}
	return nil
}

type AgentMessage struct {
	Kind   string           `json:"kind"`
	Role   string           `json:"role"`
	Blocks []AgentTextBlock `json:"blocks"`
}
type AgentTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type AgentSelection struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}
type AgentMutationEnvelope struct {
	SessionID            string `json:"sessionId"`
	ClientOperationID    string `json:"clientOperationId"`
	ExpectedRuntimeFence *int64 `json:"expectedRuntimeFence"`
	PayloadFingerprint   string `json:"payloadFingerprint"`
}
type AgentFirstMessage struct {
	ClientMessageID string          `json:"clientMessageId"`
	Body            AgentMessage    `json:"body"`
	Selection       *AgentSelection `json:"selection,omitempty"`
}
type AgentCreate struct {
	Envelope     AgentMutationEnvelope `json:"envelope"`
	Worktree     string                `json:"worktree"`
	Agent        string                `json:"agent"`
	Options      map[string]string     `json:"options"`
	FirstMessage AgentFirstMessage     `json:"firstMessage"`
}

// Send has no separate message ID: the public host uses ClientOperationID for
// both the operation and its user message. No queue/steer or unknown retry is
// requested; the lifecycle must first qualify the original session as idle.
type AgentSend struct {
	Envelope  AgentMutationEnvelope `json:"envelope"`
	Body      AgentMessage          `json:"body"`
	Selection *AgentSelection       `json:"selection,omitempty"`
}
type AgentCursor struct {
	Epoch    string `json:"epoch"`
	Sequence int64  `json:"sequence"`
}
type AgentItem struct {
	ItemID        string          `json:"itemId"`
	Revision      int64           `json:"revision"`
	Body          json.RawMessage `json:"body"`
	Sequence      int64           `json:"sequence"`
	SequenceIndex int64           `json:"sequenceIndex"`
	ObservedAt    int64           `json:"observedAt"`
	Shortened     bool            `json:"shortened"`
	TurnScope     struct {
		Kind       string `json:"kind"`
		TurnItemID string `json:"turnItemId"`
	} `json:"turnScope"`
}
type AgentSubmission struct {
	ClientMessageID    string `json:"clientMessageId"`
	PayloadFingerprint string `json:"payloadFingerprint"`
	DispatchState      string `json:"dispatchState"`
	ProviderItemID     string `json:"providerItemId"`
	Fence              int64  `json:"fence"`
}
type AgentTurn struct {
	TurnID      string `json:"turnId"`
	State       string `json:"state"`
	Outcome     string `json:"outcome"`
	UserItemID  string `json:"userItemId"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt"`
}
type AgentPage struct {
	SessionID           string            `json:"sessionId"`
	Epoch               string            `json:"epoch"`
	Fence               int64             `json:"fence"`
	Items               []AgentItem       `json:"items"`
	Submissions         []AgentSubmission `json:"submissions"`
	HasOlder            bool              `json:"hasOlder"`
	HasNewer            bool              `json:"hasNewer"`
	Working             bool              `json:"working"`
	ActionablePromptIDs []string          `json:"actionablePromptIds"`
	RemovedItemIDs      []string          `json:"removedItemIds"`
	Window              struct {
		NextCursor *AgentCursor `json:"nextCursor"`
	} `json:"window"`
}

// AgentSessionRefusal is a domain refusal inside a successful RPC envelope.
type AgentSessionRefusal struct{ Code, Message string }

func (e *AgentSessionRefusal) Error() string { return e.Code + ": " + e.Message }

func (c AgentSessionClient) SupportsCreate(ctx context.Context, worktree, agent string) error {
	if agent != "codex" && agent != "pi" {
		return errors.New("unsupported structured Orca agent")
	}
	raw, err := c.call(ctx, "agentSession.createSupport", map[string]string{"worktree": worktree, "agent": agent}, false)
	if err != nil {
		return err
	}
	var result struct {
		Supported bool `json:"supported"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Supported {
		return fmt.Errorf("original Orca host cannot create the required %s session", agent)
	}
	return nil
}

func agentDomain(raw json.RawMessage) error {
	var result struct {
		OK      *bool                          `json:"ok"`
		Refusal struct{ Code, Message string } `json:"refusal"`
	}
	if json.Unmarshal(raw, &result) != nil || result.OK == nil {
		return errors.New("invalid Orca agent result")
	}
	if !*result.OK {
		return &AgentSessionRefusal{Code: result.Refusal.Code, Message: result.Refusal.Message}
	}
	return nil
}

func (c AgentSessionClient) Create(ctx context.Context, request AgentCreate) (json.RawMessage, error) {
	if request.Envelope.SessionID == "" || request.Envelope.ClientOperationID == "" || request.FirstMessage.ClientMessageID == "" || request.Worktree == "" || request.Agent != "codex" && request.Agent != "pi" {
		return nil, errors.New("incomplete structured creation identity")
	}
	if err := request.FirstMessage.Body.ValidateInput(); err != nil {
		return nil, err
	}
	fields := map[string]any{"worktree": request.Worktree, "agent": request.Agent, "options": request.Options, "firstMessage": request.FirstMessage}
	fingerprint, err := AgentFingerprint("agentSession.create", request.Envelope.SessionID, fields)
	if err != nil || fingerprint != request.Envelope.PayloadFingerprint {
		return nil, errors.New("structured create fingerprint mismatch")
	}
	raw, err := c.call(ctx, "agentSession.create", request, true)
	if err != nil {
		return raw, err
	}
	return raw, agentDomain(raw)
}

func (c AgentSessionClient) Send(ctx context.Context, request AgentSend) (json.RawMessage, error) {
	if request.Envelope.SessionID == "" || request.Envelope.ClientOperationID == "" || request.Envelope.ExpectedRuntimeFence == nil || *request.Envelope.ExpectedRuntimeFence <= 0 {
		return nil, errors.New("structured send requires the original session, operation and positive runtime fence")
	}
	if !agentSendOperationID.MatchString(request.Envelope.ClientOperationID) {
		return nil, errors.New("structured send requires an Orca operation ID with a 13-digit timestamp and 32 lowercase hexadecimal digits")
	}
	if err := request.Body.ValidateInput(); err != nil {
		return nil, err
	}
	fields := map[string]any{"body": request.Body}
	if request.Selection != nil {
		fields["selection"] = request.Selection
	}
	fingerprint, err := AgentFingerprint("agentSession.send", request.Envelope.SessionID, fields)
	if err != nil || fingerprint != request.Envelope.PayloadFingerprint {
		return nil, errors.New("structured send fingerprint mismatch")
	}
	raw, err := c.call(ctx, "agentSession.send", request, true)
	if err != nil {
		return raw, err
	}
	// Acceptance is not a started/completed turn. Its original user/turn
	// binding must still be established through the public history reader.
	return raw, agentDomain(raw)
}

// Orca's mutation handler applies this check after parsing the public schema.
var agentSendOperationID = regexp.MustCompile(`^[0-9]{13}-[0-9a-f]{32}$`)

func (message AgentMessage) ValidateInput() error {
	if message.Kind != "message" || message.Role != "user" || len(message.Blocks) != 1 || message.Blocks[0].Type != "text" {
		return errors.New("structured input requires one user text block")
	}
	blocks, err := agentCanonical(message.Blocks)
	if err != nil || len(blocks) > 256<<10 {
		return errors.New("Orca structured input exceeds wire limit")
	}
	return nil
}

func (c AgentSessionClient) History(ctx context.Context, session string, before *AgentCursor, limit int) (AgentPage, error) {
	var result struct {
		Page AgentPage `json:"page"`
	}
	if session == "" || limit < 1 || limit > 200 {
		return result.Page, errors.New("invalid agent history bounds")
	}
	request := map[string]any{"sessionId": session, "direction": "tail", "limit": limit}
	if before != nil {
		request["direction"], request["cursor"] = "before", before
	}
	raw, err := c.call(ctx, "agentSession.history", request, false)
	if err != nil {
		return result.Page, err
	}
	if err := agentDomain(raw); err != nil {
		return result.Page, err
	}
	var presence struct {
		Page struct {
			HasOlder, HasNewer, Working *bool
			Items                       *[]json.RawMessage
		}
	}
	if json.Unmarshal(raw, &presence) != nil || presence.Page.HasOlder == nil || presence.Page.HasNewer == nil || presence.Page.Working == nil || presence.Page.Items == nil || json.Unmarshal(raw, &result) != nil || result.Page.SessionID != session || result.Page.Epoch == "" || len(result.Page.Items) > limit {
		return AgentPage{}, errors.New("agent history identity or page bounds changed")
	}
	return result.Page, nil
}

func (c AgentSessionClient) Item(ctx context.Context, session, item string) (AgentItem, error) {
	var result struct {
		Item *AgentItem `json:"item"`
	}
	raw, err := c.call(ctx, "agentSession.item", map[string]string{"sessionId": session, "itemId": item}, false)
	if err != nil {
		return AgentItem{}, err
	}
	if json.Unmarshal(raw, &result) != nil || result.Item == nil || result.Item.ItemID != item || result.Item.Shortened {
		return AgentItem{}, errors.New("complete original agent item unavailable")
	}
	return *result.Item, nil
}

func (c AgentSessionClient) Close(ctx context.Context, session string) (json.RawMessage, error) {
	if session == "" {
		return nil, errors.New("missing original agent session")
	}
	raw, err := c.call(ctx, "agentSession.close", map[string]string{"sessionId": session}, true)
	if err != nil {
		return raw, err
	}
	return raw, agentDomain(raw)
}

// AgentFingerprint follows the installed public contract's JSON.stringify
// canonicalization. Request JSON encoding itself need not use this byte layout.
func AgentFingerprint(method, session string, fields any) (string, error) {
	canonical, err := agentCanonical(map[string]any{"method": method, "sessionId": session, "fields": fields})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func agentCanonical(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var parsed any
	if err := decoder.Decode(&parsed); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	var write func(any) error
	write = func(value any) error {
		switch value := value.(type) {
		case nil:
			out.WriteString("null")
		case bool:
			out.WriteString(strconv.FormatBool(value))
		case json.Number:
			out.WriteString(string(value))
		case string:
			var encoded bytes.Buffer
			encoder := json.NewEncoder(&encoded)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(value); err != nil {
				return err
			}
			text := strings.TrimSuffix(encoded.String(), "\n")
			// encoding/json always escapes these two characters; JSON.stringify does not.
			// Decode escape tokens rather than replacing escaped literal backslashes.
			for i := 0; i < len(text); i++ {
				if text[i] == '\\' && i+1 < len(text) {
					if i+6 <= len(text) && (text[i:i+6] == `\u2028` || text[i:i+6] == `\u2029`) {
						if text[i+5] == '8' {
							out.WriteRune('\u2028')
						} else {
							out.WriteRune('\u2029')
						}
						i += 5
						continue
					}
					out.WriteByte(text[i])
					i++
					out.WriteByte(text[i])
					continue
				}
				out.WriteByte(text[i])
			}
		case []any:
			out.WriteByte('[')
			for i, x := range value {
				if i > 0 {
					out.WriteByte(',')
				}
				if err := write(x); err != nil {
					return err
				}
			}
			out.WriteByte(']')
		case map[string]any:
			keys := make([]string, 0, len(value))
			for k := range value {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
			out.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					out.WriteByte(',')
				}
				if err := write(k); err != nil {
					return err
				}
				out.WriteByte(':')
				if err := write(value[k]); err != nil {
					return err
				}
			}
			out.WriteByte('}')
		default:
			return fmt.Errorf("unsupported structured fingerprint value %T", value)
		}
		return nil
	}
	if err := write(parsed); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func utf16Less(a, b string) bool {
	x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}
