package codexexec

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

const good = `{"outcome":"done","summary":"audit and validation completed; no repairs or remaining issues","question":null,"blocker":null,"fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`

func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Println("codex-cli 0.159.2")
			os.Exit(0)
		case "--parser-child":
			for {
				time.Sleep(time.Second)
			}
		case "--no-daemon":
			if strings.Contains(strings.Join(os.Args, " "), "app-server") {
				configurationFixture()
				os.Exit(0)
			}
			parserHelper()
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func configurationFixture() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			ID     int
			Method string
		}
		_ = json.Unmarshal(scanner.Bytes(), &request)
		if request.ID == 0 {
			continue
		}
		var result any
		if request.Method == "initialize" {
			home := os.Getenv("CODEX_HOME")
			if home == "" {
				base, _ := os.UserHomeDir()
				home = filepath.Join(base, ".codex")
			}
			result = map[string]any{"userAgent": "parser-fixture", "codexHome": home, "platformOs": "windows"}
		}
		if request.Method == "config/read" {
			result = map[string]any{"config": map[string]any{"mcp_servers": map[string]any{"fixture_external": map[string]any{"enabled": true}}}, "layers": []any{map[string]any{"name": map[string]any{"type": "system"}, "config": map[string]any{"mcp_servers": map[string]any{"fixture_external": map[string]any{"url": "https://unused.invalid"}}}}}}
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"id": request.ID, "result": result})
	}
}
func parserHelper() {
	if !strings.Contains(strings.Join(os.Args, " "), "mcp_servers.fixture_external.enabled=false") {
		os.Exit(4)
	}
	input, _ := io.ReadAll(os.Stdin)
	text := string(input)
	if strings.Contains(text, "CASE_HANG") {
		child := exec.Command(os.Args[0], "--parser-child")
		_ = child.Start()
		for {
			time.Sleep(time.Second)
		}
	}
	if strings.Contains(text, "CASE_EXIT") {
		os.Exit(3)
	}
	if strings.Contains(text, "CASE_MODEL") {
		fmt.Fprintln(os.Stderr, "model_not_found: unsupported model")
		os.Exit(3)
	}
	fmt.Println(`{"type":"thread.started","thread_id":"parser-session"}`)
	fmt.Println(`{"type":"turn.started"}`)
	if strings.Contains(text, "CASE_EXTERNAL") {
		fmt.Println(`{"type":"item.started","item":{"type":"mcp_tool_call","id":"external"}}`)
		for {
			time.Sleep(time.Second)
		}
	}
	if strings.Contains(text, "CASE_TOOL") {
		fmt.Println(`{"type":"item.started","item":{"type":"command_execution","id":"tool"}}`)
		for {
			time.Sleep(time.Second)
		}
	}
	body := good
	if strings.Contains(text, "CASE_BAD") {
		body = `{"outcome":"done","summary":"missing repairs"}`
	}
	if strings.Contains(text, "CASE_HUGE") {
		body = strings.Repeat("x", 70000)
	}
	if !strings.Contains(text, "CASE_NOFINAL") {
		event, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"type": "agent_message", "id": "final", "text": body}})
		fmt.Println(string(event))
		for i, arg := range os.Args {
			if arg == "--output-last-message" {
				_ = os.WriteFile(os.Args[i+1], []byte(body), 0600)
			}
		}
	}
	fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`)
}

func fixtureRequest(t *testing.T, scenario string) b.LifecycleRequest {
	t.Helper()
	cfg := ra.DefaultConfig()
	cfg.CodexParser.Enabled = true
	cfg.CodexParser.Executable = os.Args[0]
	final := "CASE_" + scenario
	pack := Package{Config: ra.FrozenConfig{Config: cfg, FileDigest: "frozen"}, Source: ra.Source{CommandID: "audit", TurnID: "audit-turn", AttemptID: "attempt", InputVersion: 1, FinalDigest: ra.Digest([]byte(final))}, ImplementationResultID: "implementation", Final: final}
	pack = FreezePackage(pack)
	payload, _ := json.Marshal(pack)
	owner, _ := process.NewServiceFacts().CurrentIdentity()
	snapshot := b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt"}, Session: b.SessionContext{WorkflowRunID: "run", AttemptID: "attempt"}, ResourceGeneration: "executor-generation", ConfigurationDigest: "config"}
	input := &b.ParseInput{ResultID: "result", Payload: string(payload), Generation: "generation-" + scenario + "-" + process.New(0).NewProcessGeneration(), Deadline: time.Now().Add(30 * time.Second), Owner: owner}
	intent, err := operation.New("run", snapshot, b.LifecycleCandidate{Action: b.LifecycleParseResult, Input: b.LifecycleInput{Parse: input}}, "operation-"+scenario, "key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return intent.Request
}

func TestRAParserProcess(t *testing.T) {
	for _, scenario := range []string{"GOOD", "BAD", "TOOL", "EXTERNAL", "EXIT", "MODEL", "NOFINAL", "HUGE", "HANG"} {
		t.Run(scenario, func(t *testing.T) {
			request := fixtureRequest(t, scenario)
			root := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan b.ProcessIdentity, 1)
			task := Start(ctx, root, request, func(r b.LifecycleReceipt) error {
				if r.ValidateFor(request) != nil {
					t.Error("invalid root receipt")
				}
				started <- *r.Process
				return nil
			})
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("root not started")
			}
			if scenario == "HANG" {
				cancel()
			}
			select {
			case <-task.Done():
			case <-time.After(15 * time.Second):
				t.Fatal("owned parser did not settle")
			}
			receipt, ok := task.Result()
			if !ok || !receipt.State.Stopped || !receipt.State.WritersAbsent {
				t.Fatal(receipt)
			}
			if receipt.ParseEvidence.Status == "completed" && scenario != "GOOD" {
				t.Fatal("invalid output accepted", scenario)
			}
			if scenario == "GOOD" && (receipt.ParseEvidence.Status != "completed" || receipt.ParseEvidence.SessionID != "parser-session") {
				t.Fatal(receipt.ParseEvidence)
			}
			if scenario == "TOOL" && (receipt.ParseEvidence.Status != "policy_violation" || receipt.ParseEvidence.ToolEvents != 1) {
				t.Fatal(receipt.ParseEvidence)
			}
			if scenario == "MODEL" && receipt.ParseEvidence.Diagnostic != "parser_model_unavailable" {
				t.Fatal(receipt.ParseEvidence)
			}
			if scenario == "EXTERNAL" && receipt.ParseEvidence.Status != "side_effect_unverified" {
				t.Fatal("external side effect was treated as safe fallback", receipt.ParseEvidence)
			}
			saved, err := ReadReceipt(root, request)
			if err != nil || saved.ParseEvidence.Status != receipt.ParseEvidence.Status {
				t.Fatal(err)
			}
		})
	}
}

func TestRAParserIdentityCommitFailure(t *testing.T) {
	request := fixtureRequest(t, "GOOD")
	root := t.TempDir()
	task := Start(context.Background(), root, request, func(b.LifecycleReceipt) error { return fmt.Errorf("commit failed") })
	select {
	case <-task.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("launch failure hung")
	}
	receipt, _ := task.Result()
	if !receipt.State.WritersAbsent || receipt.ParseEvidence.Status == "completed" {
		t.Fatal(receipt)
	}
	if _, err := os.Stat(filepath.Join(Directory(root, request), "final.json")); !os.IsNotExist(err) {
		t.Fatal("child ran before durable identity commit")
	}
}

func TestRAParserEncodedReceiptBound(t *testing.T) {
	request := fixtureRequest(t, "GOOD")
	var pack Package
	if err := json.Unmarshal([]byte(request.Input.Parse.Payload), &pack); err != nil {
		t.Fatal(err)
	}
	pack.Final = strings.Repeat(`\`, 20<<10)
	pack.Source.FinalDigest = ra.Digest([]byte(pack.Final))
	pack = FreezePackage(pack)
	payload, _ := json.Marshal(pack)
	if len(payload) > pack.Config.Config.CodexParser.MaxInputBytes {
		t.Fatal("fixture must fit the configured input bound")
	}
	input := *request.Input.Parse
	input.Payload = string(payload)
	intent, err := operation.New("run", request.Snapshot, b.LifecycleCandidate{Action: b.LifecycleParseResult, Input: b.LifecycleInput{Parse: &input}}, "encoded-operation", "encoded-key", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request = intent.Request
	receipt := b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true}, ParseEvidence: &b.ParseEvidence{Status: "failed"}}
	wire, _ := json.Marshal(receipt)
	if len(wire) <= 262144 {
		t.Fatal("fixture does not exercise encoded receipt overhead")
	}
	root := t.TempDir()
	if err := process.New(0).WriteExecutionFile(filepath.Join(Directory(root, request), "receipt.json"), wire); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReceipt(root, request); err != nil {
		t.Fatal("valid bounded input could not recover its original receipt", err)
	}
	if err := os.WriteFile(filepath.Join(Directory(root, request), "receipt.json"), make([]byte, (2<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReceipt(root, request); err == nil {
		t.Fatal("unbounded journal accepted")
	}
}

func TestRAParserPolicyAndEvents(t *testing.T) {
	args := strings.Join(Args(ra.DefaultConfig().CodexParser, "schema", "out", "work"), " ")
	for _, want := range []string{"--ignore-user-config", "--ignore-rules", "--sandbox read-only", "windows.sandbox=\"mxc\"", "approval_policy=\"never\"", "features.shell_tool=false", "features.apps=false", "features.hooks=false", "--output-schema"} {
		if !strings.Contains(args, want) {
			t.Fatal(want)
		}
	}
	for _, feature := range []string{"browser_use", "computer_use", "workspace_dependencies", "plugins", "multi_agent", "image_generation"} {
		if !strings.Contains(args, "features."+feature+"=false") {
			t.Fatal("external tool surface was not disabled", feature)
		}
	}
	if !strings.Contains(args, `default_permissions=":read-only"`) {
		t.Fatal("managed permission profile could select a write-capable default")
	}
	if !strings.Contains(args, "features.code_mode_host=true") {
		t.Fatal("required native model runtime was disabled")
	}
	startup := Stream{SessionID: "startup"}
	if startup.Accept([]byte(`{"type":"item.completed","item":{"type":"error","message":"runtime unavailable"}}`)) == nil || startup.FailureCode == "" || startup.Complete {
		t.Fatal("native startup error not classified")
	}
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("STEERRUNNER_WORKER_AUTH_BROKER", "secret")
	t.Setenv("ProgramData", "parser-platform-reference")
	env, err := Environment(ra.DefaultConfig().CodexParser)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(env, "\n"), "secret") {
		t.Fatal("business credentials inherited")
	}
	if !strings.Contains(strings.Join(env, "\n"), "ProgramData=parser-platform-reference") {
		t.Fatal("native MXC platform folder reference was removed")
	}
	config := ra.DefaultConfig().CodexParser
	config.CodexHomeEnv = "STEERRUNNER_PARSER_TEST_HOME"
	t.Setenv(config.CodexHomeEnv, "")
	if _, err := Environment(config); err == nil {
		t.Fatal("invalid explicit home silently fell back")
	}
	t.Setenv(config.CodexHomeEnv, t.TempDir())
	if _, err := Environment(config); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"file_change", "mcp_tool_call", "todo_list", "new_unknown_tool"} {
		s := Stream{SessionID: "s", Started: true}
		if s.Accept([]byte(`{"type":"item.started","item":{"type":"`+kind+`"}}`)) == nil || s.ToolEvents != 1 {
			t.Fatal(kind)
		}
		if s.SideEffectsUnverified != (kind != "todo_list") {
			t.Fatal("external/unknown tool cannot authorize fallback", kind)
		}
	}
}
