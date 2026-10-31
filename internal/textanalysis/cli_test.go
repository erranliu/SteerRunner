package textanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
)

func TestMain(m *testing.M) {
	if data, err := os.ReadFile(filepath.Join(os.Getenv("CODEX_HOME"), "parser-fixture.json")); err == nil {
		if len(os.Args) > 1 && os.Args[1] == "--version" {
			fmt.Println("codex-cli 0.159.2")
			os.Exit(0)
		}
		mode := string(data)
		home := os.Getenv("CODEX_HOME")
		file, _ := os.OpenFile(filepath.Join(home, "calls"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		file.WriteString("call\n")
		file.Close()
		calls, _ := os.ReadFile(filepath.Join(home, "calls"))
		count := strings.Count(string(calls), "call\n")
		output := ""
		for i, arg := range os.Args {
			if arg == "--output-last-message" && i+1 < len(os.Args) {
				output = os.Args[i+1]
			}
		}
		fmt.Println(`{"type":"thread.started","thread_id":"text-only"}`)
		fmt.Println(`{"type":"turn.started"}`)
		os.WriteFile(filepath.Join(home, "started"), nil, 0600)
		os.WriteFile(filepath.Join(home, fmt.Sprintf("started-%d", count)), nil, 0600)
		if mode == "transient" || mode == "transient-once" && count == 1 {
			fmt.Println(`{"type":"turn.failed","error":{"message":"connection reset"}}`)
			os.Exit(1)
		}
		if mode == "auth" {
			fmt.Println(`{"type":"turn.failed","error":{"message":"authentication failed"}}`)
			os.Exit(1)
		}
		if mode == "wait" {
			time.Sleep(time.Minute)
			os.Exit(1)
		}
		if mode == "tool" {
			fmt.Println(`{"type":"item.started","item":{"type":"command_execution"}}`)
			os.Exit(1)
		}
		body := `{"value":"supplied"}`
		if mode == "invalid" {
			body = "not JSON"
		}
		if mode == "schema" {
			body = `{"value":1}`
		}
		if mode == "overflow" {
			body = `{"value":"` + strings.Repeat("x", 33000) + `"}`
		}
		os.WriteFile(output, []byte(body), 0600)
		event, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"type": "agent_message", "text": body}})
		fmt.Println(string(event))
		fmt.Println(`{"type":"turn.completed"}`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fixture(t *testing.T, mode string) (ra.ParserConfig, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "parser-fixture.json"), []byte(mode), 0600); err != nil {
		t.Fatal(err)
	}
	c := ra.DefaultConfig().CodexParser
	c.Executable = os.Args[0]
	return c, home
}
func request() Request {
	return Request{Text: "supplied", Instruction: "Return only the supplied value", Schema: []byte(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`)}
}

func TestCLITextAndInternalFailures(t *testing.T) {
	for _, mode := range []string{"good", "tool", "invalid", "schema", "overflow", "auth"} {
		t.Run(mode, func(t *testing.T) {
			c, home := fixture(t, mode)
			var a Analyzer
			body, err := a.Analyze(context.Background(), request(), c)
			if mode == "good" && (err != nil || body != `{"value":"supplied"}`) {
				t.Fatal(body, err)
			}
			if mode != "good" && err == nil {
				t.Fatal("bad CLI result accepted")
			}
			calls, err := os.ReadFile(filepath.Join(home, "calls"))
			if err != nil || string(calls) != "call\n" {
				t.Fatal("bad result retried", string(calls), err)
			}
		})
	}
}

func TestCLICancellationAndQueue(t *testing.T) {
	c, home := fixture(t, "wait")
	c.MaxConcurrent = 1
	var a Analyzer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := a.Analyze(ctx, request(), c); done <- err }()
	until := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(home, "started")); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatal("CLI failed before barrier", err)
		case <-until:
			t.Fatal("CLI start barrier timed out")
		case <-time.After(10 * time.Millisecond):
		}
	}
	queued, stop := context.WithCancel(context.Background())
	defer stop()
	waiting := &admissionContext{Context: queued, waiting: make(chan struct{})}
	queuedDone := make(chan error, 1)
	go func() { _, err := a.Analyze(waiting, request(), c); queuedDone <- err }()
	select {
	case <-waiting.waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("second call did not wait for capacity")
	}
	if waiting.deadlineRead.Load() {
		t.Fatal("attempt timeout began in admission queue")
	}
	stop()
	select {
	case err := <-queuedDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queue did not propagate cancellation", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("queue cancellation blocked")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled CLI returned success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancellation blocked")
	}
	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if string(calls) != "call\n" {
		t.Fatal("queued work invoked CLI")
	}
}

// Done is consulted by the capacity wait. Deadline is first consulted when
// constructing the per-attempt context, which must happen after admission.
type admissionContext struct {
	context.Context
	waiting      chan struct{}
	once         sync.Once
	deadlineRead atomic.Bool
}

func (c *admissionContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}
func (c *admissionContext) Deadline() (time.Time, bool) {
	c.deadlineRead.Store(true)
	return c.Context.Deadline()
}

func TestCLITransientRetryIsLocalAndBounded(t *testing.T) {
	for _, mode := range []string{"transient", "transient-once"} {
		t.Run(mode, func(t *testing.T) {
			c, home := fixture(t, mode)
			var a Analyzer
			body, err := a.Analyze(context.Background(), request(), c)
			if mode == "transient-once" && (err != nil || body != `{"value":"supplied"}`) {
				t.Fatal(body, err)
			}
			if mode == "transient" && (err == nil || err.Error() != "parser_transient_failure") {
				t.Fatal(err)
			}
			calls, _ := os.ReadFile(filepath.Join(home, "calls"))
			if string(calls) != "call\ncall\n" {
				t.Fatal("transient retry not bounded to two attempts", string(calls))
			}
		})
	}
}

func TestCLIInputMaterialsBound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		delta    int
		wantCall bool
	}{
		{"exact", 0, true}, {"one-byte-over", -1, false}, {"schema-not-counted", -100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, home := fixture(t, "good")
			in := request()
			in.Text = strings.Repeat("data", 4096) + "provided \"quoted\"\n汉字\\text"
			encoded, _ := json.Marshal(in.Text)
			c.MaxInputBytes = len(ParserBoundary) + 1 + len(in.Instruction) + 1 + len(ParserFormat) + 1 + len(encoded) + len(in.Schema) + tc.delta
			var a Analyzer
			_, err := a.Analyze(context.Background(), in, c)
			calls, readErr := os.ReadFile(filepath.Join(home, "calls"))
			if tc.wantCall {
				if err != nil || string(calls) != "call\n" {
					t.Fatal("exact bound rejected", err, string(calls))
				}
			} else if err == nil || !os.IsNotExist(readErr) {
				t.Fatal("oversize materials entered CLI", err, string(calls), readErr)
			}
		})
	}
}

func TestCLITimeoutStartsAfterAdmission(t *testing.T) {
	c, home := fixture(t, "wait")
	c.MaxConcurrent = 1
	var a Analyzer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	firstConfig := c
	go func() { _, err := a.Analyze(ctx, request(), firstConfig); firstDone <- err }()
	waitStarted(t, home, "started-1", firstDone)
	queued := &admissionContext{Context: context.Background(), waiting: make(chan struct{})}
	secondConfig := c
	secondConfig.TimeoutSeconds = 1 // Short test deadline; production config remains >= 10s.
	secondDone := make(chan error, 1)
	go func() { _, err := a.Analyze(queued, request(), secondConfig); secondDone <- err }()
	select {
	case <-queued.waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("second call did not queue")
	}
	if queued.deadlineRead.Load() {
		t.Fatal("queued call started attempt timeout")
	}
	cancel()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	waitStarted(t, home, "started-2", secondDone)
	if !queued.deadlineRead.Load() {
		t.Fatal("admitted call lacks attempt timeout")
	}
	select {
	case err := <-secondDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("attempt did not time out", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("attempt deadline did not stop CLI")
	}
	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if string(calls) != "call\ncall\n" {
		t.Fatal("timeout retried", string(calls))
	}
}

func TestCLIConcurrentCapacity(t *testing.T) {
	for _, tc := range []struct{ configured, admitted int }{{0, 2}, {3, 3}} {
		t.Run(fmt.Sprintf("configured-%d", tc.configured), func(t *testing.T) {
			c, home := fixture(t, "wait")
			c.MaxConcurrent = tc.configured
			var a Analyzer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, tc.admitted)
			for i := 0; i < tc.admitted; i++ {
				go func() { _, err := a.Analyze(ctx, request(), c); done <- err }()
			}
			waitStarted(t, home, fmt.Sprintf("started-%d", tc.admitted), done)
			queued := &admissionContext{Context: ctx, waiting: make(chan struct{})}
			queuedDone := make(chan error, 1)
			go func() { _, err := a.Analyze(queued, request(), c); queuedDone <- err }()
			select {
			case <-queued.waiting:
			case <-time.After(10 * time.Second):
				t.Fatal("extra call did not wait")
			}
			if queued.deadlineRead.Load() {
				t.Fatal("extra call entered a full parser")
			}
			cancel()
			for i := 0; i < tc.admitted; i++ {
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("active call did not cancel")
				}
			}
			if err := <-queuedDone; !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			calls, _ := os.ReadFile(filepath.Join(home, "calls"))
			if strings.Count(string(calls), "call\n") != tc.admitted {
				t.Fatal("wrong concurrency", string(calls))
			}
		})
	}
}

func waitStarted(t *testing.T, home, marker string, done <-chan error) {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(filepath.Join(home, marker)); err == nil {
			return
		}
		select {
		case err := <-done:
			t.Fatal("CLI ended before start barrier", err)
		case <-timer.C:
			t.Fatal("CLI start barrier timed out")
		case <-tick.C:
		}
	}
}

func TestCLIRetrySettingsUseExistingNativeAuthentication(t *testing.T) {
	args := Args(ra.DefaultConfig().CodexParser, "schema", "output", "temp")
	settings := map[string]string{}
	for i, arg := range args {
		if arg == "-c" {
			key, value, _ := strings.Cut(args[i+1], "=")
			settings[key] = value
		}
	}
	for key, want := range map[string]string{
		"model_provider": `"steerrunner_text"`,
		"model_providers.steerrunner_text.requires_openai_auth": "true",
		"model_providers.steerrunner_text.request_max_retries":  "0",
		"model_providers.steerrunner_text.stream_max_retries":   "0",
		"model_providers.steerrunner_text.supports_websockets":  "false",
		"features.unbounded_connection_retries":                 "false",
	} {
		if settings[key] != want {
			t.Fatal("native retry/login contract mismatch", key, settings[key])
		}
	}
	for key := range settings {
		if strings.HasSuffix(key, ".base_url") || strings.HasSuffix(key, ".env_key") || strings.HasSuffix(key, ".experimental_bearer_token") {
			t.Fatal("parser supplied a service or credential", key)
		}
	}
}

func TestCLIArgumentsDisableTools(t *testing.T) {
	args := strings.Join(Args(ra.DefaultConfig().CodexParser, "schema", "output", "temp"), " ")
	for _, want := range []string{"--ignore-user-config", "--ignore-rules", "--ephemeral", "features.shell_tool=false", "features.apps=false", "features.plugins=false", "features.hooks=false", "mcp_servers={}", "read-only"} {
		if !strings.Contains(args, want) {
			t.Fatal("missing internal restriction", want)
		}
	}
}
