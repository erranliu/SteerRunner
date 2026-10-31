// Executable-only controlled protocol fixture. Production never imports it.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	protocol "github.com/erranliu/SteerRunner/internal/testkit/protocol"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	if len(os.Args) > 1 && os.Args[1] == "--orca-create-fixture" {
		os.Exit(orcaCreationFixture(os.Args[2:]))
	}
	if len(os.Args) == 4 && os.Args[1] == "--test-config" {
		config, digest, err := livetest.ReadConfig(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		var response struct {
			Data struct {
				Catalog        resources.CapabilityCatalog `json:"capability_catalog"`
				ResourceDigest string                      `json:"resource_digest"`
			} `json:"data"`
			OK bool `json:"ok"`
		}
		var raw []byte
		if os.Args[3] == "-" {
			raw, err = io.ReadAll(io.LimitReader(os.Stdin, 16<<20))
		} else {
			raw, err = os.ReadFile(os.Args[3])
		}
		if err != nil || json.Unmarshal(raw, &response) != nil || !response.OK || response.Data.ResourceDigest != resources.Digest() {
			fmt.Fprintln(os.Stderr, "Test helper and selected Runner capability contract do not match.")
			os.Exit(2)
		}
		result := livetest.AssessConfig(config, digest, response.Data.Catalog)
		if json.NewEncoder(os.Stdout).Encode(result) != nil {
			os.Exit(1)
		}
		return
	}
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
	if len(os.Args) > 1 && os.Args[1] == "--no-daemon" && strings.Contains(args, "exec") {
		parserFixture()
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
			// The native creation has succeeded and is recorded before its RPC
			// response reaches the owner. Used only by the protocol fixture.
			for request.Method == "thread/start" && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-thread-response")) {
				time.Sleep(20 * time.Millisecond)
			}
			if connection.Write(context.Background(), websocket.MessageText, encode(response)) != nil {
				return
			}
			for _, event := range events {
				if connection.Write(context.Background(), websocket.MessageText, encode(event)) != nil {
					return
				}
			}
			if os.Getenv("STEERRUNNER_FIXTURE_MULTI_PROVIDER") == "1" && request.Method == "turn/start" && exists(statePath(s.thread.ID)+".completed") {
				go s.releaseHeld(connectionContext, connection)
			}
			if os.Getenv("STEERRUNNER_CONTENT_FIXTURE") == "1" && request.Method == "turn/start" {
				go s.releaseContent(connectionContext, connection)
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
	case "account/read":
		if err := appendCall(map[string]any{"method": "account/read"}); err != nil {
			return fail(err)
		}
		response["result"] = map[string]any{"requiresOpenaiAuth": true, "account": map[string]any{"type": "apiKey"}}
		if exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "content-no-auth")) {
			response["result"] = map[string]any{"requiresOpenaiAuth": true, "account": nil}
		}
	case "command/exec":
		result, err := contentReadinessFixture(params)
		if err != nil {
			return fail(err)
		}
		response["result"] = result
	case "initialize":
		home := os.Getenv("STEERRUNNER_FIXTURE_STATE")
		if home == "" {
			home = os.Getenv("CODEX_HOME")
		}
		if home == "" {
			base, _ := os.UserHomeDir()
			home = filepath.Join(base, ".codex")
		}
		response["result"] = map[string]any{"userAgent": "controlled-app-server", "codexHome": home, "platformOs": "windows"}
	case "config/read":
		response["result"] = map[string]any{"config": map[string]any{"mcp_servers": map[string]any{"fixture_external": map[string]any{"enabled": true}}}, "layers": []any{map[string]any{"name": map[string]any{"type": "system"}, "config": map[string]any{"mcp_servers": map[string]any{"fixture_external": map[string]any{"url": "https://unused.invalid"}}}}}}
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
		if str("sandbox") == "read-only" {
			response["result"] = map[string]any{"thread": s.thread, "approvalPolicy": "never", "approvalsReviewer": "user", "sandbox": map[string]any{"type": "readOnly"}}
		}
	case "thread/resume", "thread/read":
		body, err := os.ReadFile(statePath(str("threadId")))
		if err != nil {
			return fail(err)
		}
		if err := json.Unmarshal(body, &s.thread); err != nil {
			return fail(err)
		}
		response["result"] = map[string]any{"thread": s.thread}
		if str("sandbox") == "read-only" {
			response["result"] = map[string]any{"thread": s.thread, "approvalPolicy": "never", "approvalsReviewer": "user", "sandbox": map[string]any{"type": "readOnly"}}
		}
	case "turn/interrupt", "turn/steer":
		body, err := os.ReadFile(statePath(str("threadId")))
		if err != nil {
			return fail(err)
		}
		if json.Unmarshal(body, &s.thread) != nil || len(s.thread.Turns) == 0 {
			return fail(fmt.Errorf("original turn missing"))
		}
		turn := s.thread.Turns[len(s.thread.Turns)-1]
		id := str("turnId")
		if request.Method == "turn/steer" {
			id = str("expectedTurnId")
		}
		if turn["id"] != id || turn["status"] != "inProgress" {
			return fail(fmt.Errorf("original turn is not active"))
		}
		if err := appendCall(map[string]any{"method": request.Method, "thread": s.thread.ID, "turn": id}); err != nil {
			return fail(err)
		}
		turn["status"], turn["completedAt"] = "interrupted", 1
		if request.Method == "turn/steer" {
			turn["status"] = "completed"
			turn["items"] = append(turn["items"].([]any), map[string]any{"type": "agentMessage", "id": "answer-final", "phase": "final_answer", "text": "answered"}, map[string]any{"type": "userMessage", "id": "answer-user", "clientId": str("clientUserMessageId"), "content": json.RawMessage(params["input"])})
		}
		if err := s.save(); err != nil {
			return fail(err)
		}
		if request.Method == "turn/interrupt" && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "drop-next-interrupt")) {
			return nil, nil
		}
		response["result"] = map[string]any{"turnId": id}
		return response, nil
	case "turn/start":
		_ = appendCall(map[string]any{"method": "result/environment", "path": os.Getenv("STEERLINE_RESULT_FILE"), "progress_path": os.Getenv("STEERLINE_PROGRESS_FILE"), "cwd": s.thread.Cwd})
		if path := os.Getenv("STEERLINE_PROGRESS_FILE"); path != "" {
			_ = appendCall(map[string]any{"method": "progress/environment", "path": path, "cwd": s.thread.Cwd})
		}
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
		audit := (strings.HasPrefix(prompt, "Audit the current") || strings.HasPrefix(prompt, "审计当前代码") || strings.HasPrefix(prompt, "根据已有上下文整理指定审计") || strings.HasPrefix(prompt, "审计修复 P0–P2；") || strings.HasPrefix(prompt, "仅报告原审计；"))
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
		if os.Getenv("STEERRUNNER_CONTENT_FIXTURE") == "1" {
			body, readErr := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "content-final"))
			if readErr != nil {
				return fail(readErr)
			}
			content = string(body)
		} else if audit {
			content, err = auditResult(s.thread.Cwd, prompt)
		} else {
			content, err = protocol.StagedBusiness(s.thread.Cwd, prompt, os.Getenv("STEERRUNNER_FIXTURE_PROVIDER"), "origin")
		}
		if err != nil {
			return fail(err)
		}
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", len(s.thread.Turns)+1)
		item := map[string]any{"type": "agentMessage", "id": "final-" + id, "phase": "final_answer", "text": content}
		turn := map[string]any{"id": id, "status": "completed", "completedAt": 1, "items": []any{item}}
		if os.Getenv("STEERRUNNER_CONTENT_FIXTURE") == "1" {
			root := os.Getenv("STEERRUNNER_FIXTURE_STATE")
			if exists(filepath.Join(root, "content-failed")) {
				turn["status"] = "failed"
			}
			if exists(filepath.Join(root, "content-cancelled")) {
				turn["status"] = "interrupted"
			}
			if exists(filepath.Join(root, "content-question")) {
				item = map[string]any{"type": "agentMessage", "id": "question-" + id, "phase": "final_answer", "delivery": "async", "text": "Choose a value", "questions": []any{map[string]any{"title": "Choose a value", "options": []string{"one", "two"}}}}
				turn["items"], turn["status"] = []any{item}, "inProgress"
				delete(turn, "completedAt")
			}
		}
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
		if exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")) || audit && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-audit")) || (strings.HasPrefix(prompt, "Format the preceding publication result as JSON.") || strings.HasPrefix(prompt, "根据已有上下文整理指定发布") || strings.HasPrefix(prompt, "仅整理原发布结果与 PR URL，不重发；")) && exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-publication-report")) {
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
		if os.Getenv("STEERRUNNER_CONTENT_FIXTURE") == "1" && turn["status"] == "inProgress" {
			return response, []map[string]any{{"method": "turn/started", "params": map[string]any{"threadId": s.thread.ID, "turn": map[string]any{"id": id}}}, {"method": "item/completed", "params": map[string]any{"threadId": s.thread.ID, "turnId": id, "item": item}}}
		}
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

func auditResult(cwd, prompt string) (string, error) {
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
	if strings.HasPrefix(prompt, "根据已有上下文整理指定审计") || strings.HasPrefix(prompt, "仅报告原审计；") {
		return `{"outcome":"done","summary":"original audit formatted","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`, nil
	}
	if exists(filepath.Join(root, "audit-question")) {
		return `{"outcome":"needs_input","summary":"unfinished","question":"Is this required?"}`, nil
	}
	if os.Getenv("STEERRUNNER_AUDIT_FIX") == "1" {
		if err := os.WriteFile(filepath.Join(cwd, "stage-result.txt"), []byte("repaired audited code\n"), 0600); err != nil {
			return "", err
		}
		for _, args := range [][]string{{"add", "--", "stage-result.txt"}, {"-c", "user.name=Audit Fixture", "-c", "user.email=audit@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "audit repair"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = cwd
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("audit fixture: %v %s", err, out)
			}
		}
		return `{"outcome":"done","summary":"audited repaired validated","fixed_count":1,"fixed_issues":[{"severity":"P1","issue":"controlled bug","repair":"repaired code","evidence":"stage-result.txt"}],"remaining_issues":[]}`, nil
	}
	if os.Getenv("STEERRUNNER_AUDIT_PLAIN") == "1" {
		return "Audited and validated. No P0-P2 issues remain; fixed zero. " + os.Getenv("STEERRUNNER_PARSER_CASE"), nil
	}
	return `{"outcome":"done","summary":"audited and validated","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`, nil
}

func parserFixture() {
	if args := strings.Join(os.Args, " "); !strings.Contains(args, "--ignore-user-config") || !strings.Contains(args, "mcp_servers={}") {
		os.Exit(4)
	}
	input, _ := io.ReadAll(os.Stdin)
	if strings.Contains(string(input), "PARSER_HANG") {
		fmt.Println(`{"type":"thread.started","thread_id":"independent-parser-fixture"}`)
		fmt.Println(`{"type":"turn.started"}`)
		for {
			time.Sleep(time.Second)
		}
	}
	fmt.Println(`{"type":"thread.started","thread_id":"independent-parser-fixture"}`)
	fmt.Println(`{"type":"turn.started"}`)
	if marker := regexp.MustCompile(`PARSER_WAIT\(([A-Za-z0-9_-]+)\)`).FindSubmatch(input); len(marker) == 2 {
		path, err := base64.RawURLEncoding.DecodeString(string(marker[1]))
		if err != nil {
			os.Exit(5)
		}
		file, err := os.OpenFile(string(path)+".starts", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(6)
		}
		_, _ = file.WriteString("start\n")
		_ = file.Close()
		for exists(string(path)) {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if strings.Contains(string(input), "PARSER_TOOL") {
		fmt.Println(`{"type":"item.started","item":{"id":"tool","type":"command_execution"}}`)
		return
	}
	body := `{"outcome":"done","summary":"audited validated no repairs","question":null,"blocker":null,"fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`
	if strings.Contains(string(input), "PARSER_BAD") {
		body = `{"outcome":"done","summary":"missing details"}`
	}
	event, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"type": "agent_message", "id": "final", "text": body}})
	fmt.Println(string(event))
	for i, arg := range os.Args {
		if arg == "--output-last-message" && i+1 < len(os.Args) {
			_ = os.WriteFile(os.Args[i+1], []byte(body), 0600)
		}
	}
	fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`)
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func (s *server) releaseHeld(ctx context.Context, connection *websocket.Conn) {
	for exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")) || exists(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-audit")) {
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
