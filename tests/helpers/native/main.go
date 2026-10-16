// Executable-only controlled protocol fixture. Production never imports it.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	protocol "github.com/erranliu/SteerRunner/internal/testkit/protocol"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type thread struct {
	ID    string           `json:"id"`
	Cwd   string           `json:"cwd"`
	Turns []map[string]any `json:"turns"`
}
type rpc struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type server struct {
	mu     sync.Mutex
	thread thread
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "--query" {
		if os.Getenv("STEERRUNNER_FIXTURE_STATE") != "" {
			_ = appendCall(map[string]any{"method": "provider/query"})
		}
		path := os.Args[2]
		if os.Getenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER") == "1" && len(os.Args) == 4 {
			uri, err := url.Parse(os.Args[3])
			if err != nil || uri.Host != "controlled.invalid" {
				os.Exit(2)
			}
			number, err := strconv.ParseUint(filepath.Base(uri.Path), 10, 32)
			if err != nil {
				os.Exit(2)
			}
			path = filepath.Join(filepath.Dir(path), fmt.Sprintf("pr-%d.json", number))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			os.Exit(2)
		}
		fmt.Print(string(body))
		return
	}
	args := strings.Join(os.Args[1:], " ")
	if strings.Contains(args, "--version") {
		version := os.Getenv("SR_FIXTURE_VERSION")
		if version == "" {
			version = "codex-cli 0.159.2"
		}
		fmt.Println(version)
		return
	}
	if strings.Contains(args, "--help") {
		fmt.Println("app-server --listen ADDRESS --stdio")
		return
	}
	s := &server{}
	if strings.Contains(args, "--stdio") {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 4096), 16<<20)
		for scanner.Scan() {
			var request rpc
			if json.Unmarshal(scanner.Bytes(), &request) != nil || len(request.ID) == 0 {
				continue
			}
			response, _ := s.call(request)
			_ = json.NewEncoder(os.Stdout).Encode(response)
		}
		return
	}
	endpoint := ""
	for i, arg := range os.Args {
		if arg == "--listen" && i+1 < len(os.Args) {
			endpoint = os.Args[i+1]
		}
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "ws" {
		os.Exit(2)
	}
	listener, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		os.Exit(2)
	}
	_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer connection.CloseNow()
		connectionContext, cancel := context.WithCancel(context.Background())
		defer cancel()
		connection.SetReadLimit(16 << 20)
		for {
			_, body, err := connection.Read(context.Background())
			if err != nil {
				return
			}
			var request rpc
			if json.Unmarshal(body, &request) != nil || len(request.ID) == 0 {
				continue
			}
			response, events := s.call(request)
			if response == nil {
				return
			}
			if connection.Write(context.Background(), websocket.MessageText, encode(response)) != nil {
				return
			}
			for _, event := range events {
				if connection.Write(context.Background(), websocket.MessageText, encode(event)) != nil {
					return
				}
			}
			if os.Getenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER") == "1" && request.Method == "turn/start" && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")) {
				go s.releaseHeld(connectionContext, connection)
			}
		}
	}))
}
func encode(v any) []byte {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return body
}
func statePath(id string) string {
	return filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), id+".json")
}
func (s *server) call(request rpc) (map[string]any, []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	response := map[string]any{"id": request.ID}
	var params map[string]json.RawMessage
	_ = json.Unmarshal(request.Params, &params)
	str := func(key string) string { var value string; _ = json.Unmarshal(params[key], &value); return value }
	fail := func(err error) (map[string]any, []map[string]any) {
		_ = appendCall(map[string]any{"method": "rpc/error", "rpc_method": request.Method, "error": err.Error()})
		response["error"] = map[string]any{"code": -32600, "message": err.Error()}
		return response, nil
	}
	switch request.Method {
	case "initialize":
		response["result"] = map[string]any{"userAgent": "controlled-app-server", "codexHome": os.Getenv("STEERRUNNER_FIXTURE_STATE"), "platformOs": "windows"}
	case "thread/start":
		sum := sha256.Sum256([]byte(str("cwd")))
		id := hex.EncodeToString(sum[:16])
		id = id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
		s.thread = thread{ID: id, Cwd: str("cwd"), Turns: []map[string]any{}}
		if err := appendCall(map[string]any{"method": "thread/start", "thread": id, "cwd": s.thread.Cwd}); err != nil {
			return fail(err)
		}
		if err := s.save(); err != nil {
			return fail(err)
		}
		response["result"] = map[string]any{"thread": s.thread}
	case "thread/resume", "thread/read":
		body, err := os.ReadFile(statePath(str("threadId")))
		if err != nil {
			return fail(err)
		}
		if err := json.Unmarshal(body, &s.thread); err != nil {
			return fail(err)
		}
		response["result"] = map[string]any{"thread": s.thread}
	case "turn/start":
		if str("threadId") != s.thread.ID {
			return fail(fmt.Errorf("wrong original thread"))
		}
		var input []struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(params["input"], &input)
		if len(input) != 1 {
			return fail(fmt.Errorf("one input required"))
		}
		prompt := input[0].Text
		if err := appendCall(map[string]any{"method": "turn/start", "thread": s.thread.ID, "cwd": s.thread.Cwd, "prompt": prompt, "model": str("model"), "effort": str("effort")}); err != nil {
			return fail(err)
		}
		audit := strings.HasPrefix(prompt, "Audit the current")
		if os.Getenv("STEERRUNNER_AUDIT_EXPECT_MODELS") == "1" {
			expectedModel, expectedEffort := "executor", "medium"
			if audit {
				expectedModel, expectedEffort = "reviewer", "high"
			}
			if str("model") != expectedModel || str("effort") != expectedEffort {
				return fail(fmt.Errorf("wrong per-turn model or effort"))
			}
		}
		content := ""
		var err error
		if audit {
			content, err = auditResult(prompt)
		} else {
			content, err = protocol.StagedBusiness(s.thread.Cwd, prompt, os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "origin")
		}
		if err != nil {
			return fail(err)
		}
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", len(s.thread.Turns)+1)
		item := map[string]any{"type": "agentMessage", "id": "final-" + id, "phase": "final_answer", "text": content}
		turn := map[string]any{"id": id, "status": "completed", "completedAt": 1, "items": []any{item}}
		missingFinal := exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "missing-final"))
		if missingFinal {
			turn["items"] = []any{}
		}
		s.thread.Turns = append(s.thread.Turns, turn)
		if err := s.save(); err != nil {
			return fail(err)
		}
		if marker := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "drop-next-start"); exists(marker) {
			_ = os.Remove(marker)
			return nil, nil
		}
		if exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")) || strings.HasPrefix(prompt, "Format the preceding publication result as JSON.") && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-publication-report")) {
			if os.Getenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER") == "1" {
				if err := os.WriteFile(statePath(s.thread.ID)+".completed", encode(s.thread), 0600); err != nil {
					return fail(err)
				}
			}
			turn["status"] = "inProgress"
			turn["items"] = []any{}
			delete(turn, "completedAt")
			_ = s.save()
			response["result"] = map[string]any{"turn": map[string]any{"id": id, "status": "inProgress"}}
			return response, []map[string]any{{"method": "turn/started", "params": map[string]any{"threadId": s.thread.ID, "turn": map[string]string{"id": id}}}}
		}
		response["result"] = map[string]any{"turn": map[string]any{"id": id, "status": "inProgress"}}
		if missingFinal {
			return response, []map[string]any{{"method": "turn/started", "params": map[string]any{"threadId": s.thread.ID, "turn": map[string]string{"id": id}}}, {"method": "turn/completed", "params": map[string]any{"threadId": s.thread.ID, "turn": turn}}}
		}
		return response, []map[string]any{{"method": "turn/started", "params": map[string]any{"threadId": s.thread.ID, "turn": map[string]string{"id": id}}}, {"method": "item/completed", "params": map[string]any{"threadId": s.thread.ID, "turnId": id, "item": item}}, {"method": "turn/completed", "params": map[string]any{"threadId": s.thread.ID, "turn": turn}}}
	case "thread/list":
		response["result"] = map[string]any{"data": []thread{}, "nextCursor": nil}
	default:
		return fail(fmt.Errorf("unsupported fixture method %s", request.Method))
	}
	return response, nil
}

func auditResult(prompt string) (string, error) {
	root := os.Getenv("STEERRUNNER_FIXTURE_STATE")
	if exists(filepath.Join(root, "audit-unavailable")) {
		return "", fmt.Errorf("requested audit model unavailable")
	}
	if exists(filepath.Join(root, "audit-bad-json")) {
		return "not JSON", nil
	}
	if exists(filepath.Join(root, "audit-unicode-alias")) {
		return `{"outcome":"done","summary":"ambiguous audit","verdict":"pass","findings":[{"severity":"P1","message":"blocking defect"}],"findingſ":[]}`, nil
	}
	if strings.HasPrefix(prompt, "Audit the current implementation") {
		if marker := filepath.Join(root, "audit-question"); exists(marker) {
			_ = os.Remove(marker)
			return `{"outcome":"needs_input","summary":"clarify scope","question":"Is this required?"}`, nil
		}
		limit, _ := strconv.Atoi(os.Getenv("STEERRUNNER_AUDIT_FAILURES"))
		usedPath := filepath.Join(root, "audit-failures-used")
		raw, _ := os.ReadFile(usedPath)
		used, _ := strconv.Atoi(string(raw))
		if used < limit {
			if err := os.WriteFile(usedPath, []byte(strconv.Itoa(used+1)), 0600); err != nil {
				return "", err
			}
			return `{"outcome":"done","summary":"repair required","verdict":"changes_requested","findings":[{"severity":"P1","message":"controlled defect requires a repair"}]}`, nil
		}
	}
	return `{"outcome":"done","summary":"checked","verdict":"pass","findings":[]}`, nil
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func (s *server) releaseHeld(ctx context.Context, connection *websocket.Conn) {
	for exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
	s.mu.Lock()
	var complete thread
	path := statePath(s.thread.ID) + ".completed"
	body, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(body, &complete) != nil || len(complete.Turns) == 0 || complete.ID != s.thread.ID {
		s.mu.Unlock()
		return
	}
	s.thread = complete
	if err = s.save(); err != nil {
		s.mu.Unlock()
		return
	}
	_ = os.Remove(path)
	turn := complete.Turns[len(complete.Turns)-1]
	s.mu.Unlock()
	if items, ok := turn["items"].([]any); ok {
		for _, item := range items {
			_ = connection.Write(ctx, websocket.MessageText, encode(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": complete.ID, "turnId": turn["id"], "item": item}}))
		}
	}
	_ = connection.Write(ctx, websocket.MessageText, encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": complete.ID, "turn": turn}}))
}
func (s *server) save() error {
	if err := os.MkdirAll(filepath.Dir(statePath(s.thread.ID)), 0700); err != nil {
		return err
	}
	return os.WriteFile(statePath(s.thread.ID), encode(s.thread), 0600)
}
func appendCall(value any) error {
	file, err := os.OpenFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(value)
}
