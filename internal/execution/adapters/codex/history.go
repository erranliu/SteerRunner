package codexworker

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	executionvalues "github.com/erranliu/SteerRunner/internal/execution/evidence"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// HistoryClient only uses the public app-server protocol. It never opens
// Codex's database or rollout files and never resumes a thread while reading it.
type HistoryClient struct {
	input         io.WriteCloser
	lines         chan []byte
	done          chan struct{}
	proc          executionbackend.StopHandle
	mu            sync.Mutex
	next          int
	err           error
	userAgent     string
	serverHome    string
	platformOS    string
	writersAbsent bool
}

func OpenHistory(ctx context.Context, command executionbackend.Command, dir, home string, start func(*exec.Cmd) (executionbackend.StopHandle, error)) (*HistoryClient, error) {
	cmd := exec.Command(command.Executable, append(append([]string{}, command.Args...), "app-server", "--stdio")...)
	cmd.Dir = dir
	if home != "" {
		cmd.Env = historyEnvironment(cmd.Environ(), home)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	proc, err := start(cmd)
	if err != nil {
		input.Close()
		output.Close()
		stderr.Close()
		return nil, err
	}
	diagnostic := make(chan string, 1)
	go func() {
		raw, _ := io.ReadAll(io.LimitReader(stderr, 8192))
		_, _ = io.Copy(io.Discard, stderr)
		diagnostic <- strings.TrimSpace(string(raw))
	}()
	c := &HistoryClient{input: input, lines: make(chan []byte, 8), done: make(chan struct{}), proc: proc}
	go func() {
		defer close(c.done)
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 64<<10), 16<<20)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case c.lines <- line:
			case <-ctx.Done():
				proc.Kill()
			}
		}
		c.err = scanner.Err()
		if waitErr := cmd.Wait(); c.err == nil {
			c.err = waitErr
		}
		// Keep the Job open through accounting. The caller may export a native
		// reader-settlement fact only after this entire tree is actually empty.
		c.writersAbsent = settleRemoteTree(proc)
		proc.Close()
	}()
	initialized, initErr := c.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "steerline_conversation", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}})
	if initErr != nil {
		c.Close()
		if detail := <-diagnostic; detail != "" {
			return nil, fmt.Errorf("%w: %s", initErr, detail)
		}
		return nil, initErr
	}
	var metadata struct {
		UserAgent  string `json:"userAgent"`
		CodexHome  string `json:"codexHome"`
		PlatformOS string `json:"platformOs"`
	}
	if err = json.Unmarshal(initialized, &metadata); err != nil || metadata.UserAgent == "" || metadata.CodexHome == "" || metadata.PlatformOS == "" || home != "" && canonicalPath(metadata.CodexHome) != canonicalPath(home) {
		c.Close()
		return nil, errors.New("Codex app-server identity, version or home is unverified")
	}
	c.userAgent, c.serverHome, c.platformOS = metadata.UserAgent, metadata.CodexHome, metadata.PlatformOS
	if _, err = io.WriteString(input, "{\"method\":\"initialized\",\"params\":{}}\n"); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *HistoryClient) Close() {
	_ = c.input.Close()
	// Let the public reader close its native stores after stdin EOF. It never
	// owns a model turn; forced termination is only the bounded fallback.
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-c.lines:
		case <-c.done:
			return
		case <-timer.C:
			c.proc.Kill()
		}
	}
}

func (c *HistoryClient) WritersAbsent() bool {
	select {
	case <-c.done:
		return c.writersAbsent
	default:
		return false
	}
}

type historyRPCError struct {
	method  string
	code    int
	message string
}

func (e *historyRPCError) Error() string {
	return fmt.Sprintf("Codex %s (%d): %s", e.method, e.code, e.message)
}

func (c *HistoryClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	request, err := json.Marshal(map[string]any{"id": c.next, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if _, err = c.input.Write(append(request, '\n')); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, fmt.Errorf("Codex app-server ended before %s: %v", method, c.err)
		case line := <-c.lines:
			var reply struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(line, &reply); err != nil {
				return nil, fmt.Errorf("invalid Codex app-server response: %w", err)
			}
			if reply.ID != c.next {
				continue
			}
			if reply.Error != nil {
				return nil, &historyRPCError{method, reply.Error.Code, reply.Error.Message}
			}
			if len(reply.Result) == 0 {
				return nil, errors.New("Codex app-server omitted result")
			}
			return reply.Result, nil
		}
	}
}

type HistoryThread struct {
	ID    string        `json:"id"`
	Cwd   string        `json:"cwd"`
	Path  string        `json:"path,omitempty"`
	Turns []HistoryTurn `json:"turns"`
}

type HistoryTurn struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Items       []json.RawMessage `json:"items"`
	Error       json.RawMessage   `json:"error"`
	ItemsView   string            `json:"itemsView"`
	CompletedAt *int64            `json:"completedAt"`
}

// ReadThread reads the original durable thread without resuming or starting it.
func (c *HistoryClient) ReadThread(ctx context.Context, id, cwd string) (HistoryThread, error) {
	return c.readThread(ctx, id, cwd, true)
}

func (c *HistoryClient) readThread(ctx context.Context, id, cwd string, includeTurns bool) (HistoryThread, error) {
	raw, err := c.call(ctx, "thread/read", map[string]any{"threadId": id, "includeTurns": includeTurns})
	if err != nil {
		return HistoryThread{}, err
	}
	var result struct {
		Thread HistoryThread `json:"thread"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return HistoryThread{}, err
	}
	return verifiedHistoryThread(result.Thread, id, cwd)
}

func verifiedHistoryThread(thread HistoryThread, id, cwd string) (HistoryThread, error) {
	if thread.ID != id || !sameWorkspace(thread.Cwd, cwd) || len(thread.Turns) > 4096 {
		return HistoryThread{}, errors.New("Codex recovery thread/workspace or bounded history is unverified")
	}
	return thread, nil
}

func historyThreadIdle(thread HistoryThread, stoppedTurn string) bool {
	if len(thread.Turns) == 0 {
		return false
	}
	for _, turn := range thread.Turns {
		// A killed owned tree can leave its exact turn without completion timing,
		// even after native history labels it interrupted. Other native/manual
		// turns still require terminal proof; this does not adopt an old result.
		if turn.ID != "" && turn.ID == stoppedTurn && (turn.Status == "inProgress" || turn.Status == "completed" || turn.Status == "failed" || turn.Status == "interrupted") {
			continue
		}
		if turn.ID == "" || turn.CompletedAt == nil || (turn.Status != "completed" && turn.Status != "failed" && turn.Status != "interrupted") {
			return false
		}
	}
	return true
}

func canonicalPath(s string) string {
	s = filepath.Clean(s)
	if runtime.GOOS == "windows" {
		return strings.ToLower(s)
	}
	return s
}

func historyEnvironment(env []string, home string) []string {
	out := []string{}
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "CODEX_HOME") {
			out = append(out, entry)
		}
	}
	return append(out, "CODEX_HOME="+home)
}

func digestValue(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func ReaderCommand(command executionbackend.Command, profile string) executionbackend.Command {
	command.Args = append([]string(nil), command.Args...)
	if profile != "" {
		command.Args = append(command.Args, "--profile", profile)
	}
	return command
}

func EnvironmentHome() string {
	if path := os.Getenv("CODEX_HOME"); path != "" {
		return path
	}
	if path, err := os.UserHomeDir(); err == nil {
		return filepath.Join(path, ".codex")
	}
	return ""
}

type HistoryTurnFacts struct {
	Terminal      bool
	HasFinal      bool
	HasPhase      bool
	CompleteFinal bool
}

func InspectHistoryTurn(turn HistoryTurn) (HistoryTurnFacts, error) {
	facts := HistoryTurnFacts{Terminal: turn.CompletedAt != nil && (turn.Status == "completed" || turn.Status == "failed" || turn.Status == "interrupted")}
	for _, raw := range turn.Items {
		var item struct {
			Type     string  `json:"type"`
			Phase    string  `json:"phase"`
			Delivery string  `json:"delivery"`
			Text     *string `json:"text"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return facts, err
		}
		if item.Type == "agentMessage" && item.Delivery != "async" {
			facts.HasPhase = facts.HasPhase || item.Phase == string(executionvalues.MessagePhaseCommentary) || item.Phase == string(executionvalues.MessagePhaseFinalAnswer)
			if item.Phase == string(executionvalues.MessagePhaseFinalAnswer) && item.Text != nil {
				facts.HasFinal = true
				facts.CompleteFinal = turn.CompletedAt != nil
			}
		}
	}
	return facts, nil
}
