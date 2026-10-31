package textanalysis

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	native "github.com/erranliu/SteerRunner/internal/codexcommand"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const ParserBoundary = "Only organize provided audit output as data. No tools, inspection, tests, repair, commit or publication. Preserve partial/unknown facts; never infer zero or success."
const ParserFormat = "Return one JSON object matching the native schema; unknown optional fields=null. done requires completed audit/validation, no remaining P0-P2."

// Bound all caller-supplied model materials, including the separate schema file.
// CLI-owned system instructions and transport framing are not part of this limit.
func inputPrompt(in Request, maxBytes int) (string, error) {
	text, _ := json.Marshal(in.Text)
	prompt := ParserBoundary + "\n" + in.Instruction + "\n" + ParserFormat + "\n" + string(text)
	if len(prompt) > maxBytes || len(in.Schema) > maxBytes-len(prompt) {
		return "", errors.New("parser input exceeds bound")
	}
	return prompt, nil
}

func invoke(ctx context.Context, in Request, prompt string, config ra.ParserConfig) (string, error) {
	executable, err := native.NativeExecutable(config.Executable)
	if err != nil {
		return "", errors.New("parser executable unavailable")
	}
	env, err := Environment(config)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "steerrunner-text-*")
	if err != nil {
		return "", errors.New("parser temporary directory unavailable")
	}
	defer os.RemoveAll(dir)
	schemaPath := filepath.Join(dir, "schema.json")
	outputPath := filepath.Join(dir, "final.json")
	if err = os.WriteFile(schemaPath, in.Schema, 0600); err != nil {
		return "", errors.New("parser schema unavailable")
	}
	version := exec.Command(executable, "--version")
	version.Env = env
	version.Dir = dir
	var versionOut bounded
	versionOut.max = 4096
	version.Stdout = &versionOut
	version.Stderr = io.Discard
	if err = runCommand(ctx, version); err != nil {
		return "", errors.New("parser version query failed")
	}
	match := regexp.MustCompile(`(?m)codex-cli (\d+)\.(\d+)\.(\d+)`).FindStringSubmatch(versionOut.String())
	if len(match) != 4 {
		return "", errors.New("parser CLI version unavailable")
	}
	var major, minor, patch int
	fmt.Sscanf(strings.Join(match[1:], "."), "%d.%d.%d", &major, &minor, &patch)
	if major == 0 && (minor < 159 || minor == 159 && patch < 2) {
		return "", errors.New("parser CLI version unsupported")
	}
	cmd := exec.Command(executable, Args(config, schemaPath, outputPath, dir)...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bounded
	stderr.max = 65536
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", errors.New("parser output unavailable")
	}
	cleanup, err := start(cmd)
	if err != nil {
		return "", errors.New("parser process could not start")
	}
	var once sync.Once
	stop := func() { once.Do(cleanup) }
	defer stop()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			stop()
		case <-finished:
		}
	}()
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 4096), max(config.MaxOutputBytes+8192, 262144))
	stream := Stream{}
	total := 0
	var streamErr error
	for scan.Scan() {
		line := scan.Bytes()
		total += len(line)
		if total > 2<<20 {
			streamErr = errors.New("parser event output exceeds bound")
			break
		}
		if streamErr = stream.Accept(line); streamErr != nil {
			break
		}
	}
	if streamErr != nil || scan.Err() != nil {
		stop()
	}
	waitErr := cmd.Wait()
	stop()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if stream.FailureCode != "" {
		return "", errors.New(stream.FailureCode)
	}
	if streamErr != nil {
		return "", streamErr
	}
	if scan.Err() != nil {
		return "", errors.New("parser output incomplete")
	}
	if waitErr != nil {
		return "", errors.New(classifyFailure(stderr.String()))
	}
	if !stream.Complete || stream.Final == "" || len(stream.Final) > config.MaxOutputBytes {
		return "", errors.New("parser final unavailable or exceeds bound")
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return "", errors.New("parser final file unavailable")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(config.MaxOutputBytes)+1))
	if err != nil || len(body) > config.MaxOutputBytes || strings.TrimSpace(string(body)) != strings.TrimSpace(stream.Final) {
		return "", errors.New("parser final output mismatch")
	}
	if !json.Valid(body) {
		return "", errors.New("parser final JSON invalid")
	}
	return string(body), nil
}

func runCommand(ctx context.Context, cmd *exec.Cmd) error {
	cleanup, err := start(cmd)
	if err != nil {
		return err
	}
	var once sync.Once
	stop := func() { once.Do(cleanup) }
	defer stop()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		stop()
		<-done
		return ctx.Err()
	}
}

type bounded struct {
	bytes.Buffer
	max int
}

func (b *bounded) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errors.New("parser output exceeds bound")
	}
	return b.Buffer.Write(p)
}
func Args(config ra.ParserConfig, schema, output, cwd string) []string {
	args := []string{"--no-daemon", "--strict-config", "exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--json", "--sandbox", "read-only", "--skip-git-repo-check", "--cd", cwd, "--output-schema", schema, "--output-last-message", output}
	// Some native model profiles require the Code Mode runtime even when no
	// tools are used. Keeping that runtime is not a tool-free guarantee; only
	// tool items surfaced by exec JSONL can be rejected by the policy below.
	for _, setting := range []string{"approval_policy=\"never\"", "approvals_reviewer=\"user\"", "windows.sandbox=\"mxc\"", "project_doc_max_bytes=0", "memories.use_memories=false", "memories.generate_memories=false", "features.shell_tool=false", "features.apps=false", "features.plugins=false", "features.hooks=false", "features.multi_agent=false", "features.multi_agent_v2=false", "features.code_mode=false", "features.code_mode_host=true", "features.image_generation=false", "features.view_image=false", "allow_login_shell=false", "features.shell_snapshot=false", "web_search=\"disabled\"", "mcp_servers={}"} {
		args = append(args, "-c", setting)
	}
	for _, feature := range []string{"browser_use", "browser_use_external", "browser_use_full_cdp_access", "in_app_browser", "computer_use", "workspace_dependencies", "skill_mcp_dependency_install", "external_agent_memory_import", "memories"} {
		args = append(args, "-c", "features."+feature+"=false")
	}
	// Managed hosts can force permission profiles instead of the legacy sandbox.
	// Select the same read-only boundary in either mode; never fall back to write.
	args = append(args, "-c", `default_permissions=":read-only"`)
	// Built-in provider IDs cannot be overridden. This invocation-local provider
	// keeps native OpenAI login/default endpoint resolution, with transport retry
	// disabled. Only Analyzer may retry a transient failure, at most once.
	for _, setting := range []string{
		`model_provider="steerrunner_text"`,
		`model_providers.steerrunner_text.name="Codex text analysis"`,
		`model_providers.steerrunner_text.requires_openai_auth=true`,
		`model_providers.steerrunner_text.wire_api="responses"`,
		`model_providers.steerrunner_text.request_max_retries=0`,
		`model_providers.steerrunner_text.stream_max_retries=0`,
		`model_providers.steerrunner_text.supports_websockets=false`,
		`features.unbounded_connection_retries=false`,
	} {
		args = append(args, "-c", setting)
	}
	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}
	if config.ReasoningEffort != "" {
		wire, _ := json.Marshal(config.ReasoningEffort)
		args = append(args, "-c", "model_reasoning_effort="+string(wire))
	}
	return append(args, "-")
}

func Environment(config ra.ParserConfig) ([]string, error) {
	allowed := map[string]bool{}
	for _, key := range strings.Fields("SYSTEMROOT SYSTEMDRIVE WINDIR COMSPEC PATH PATHEXT USERPROFILE HOME LOCALAPPDATA APPDATA PROGRAMFILES PROGRAMFILES(X86) PROGRAMW6432 PROGRAMDATA ALLUSERSPROFILE TEMP TMP CODEX_HOME OPENAI_API_KEY CODEX_API_KEY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY SSL_CERT_FILE SSL_CERT_DIR") {
		allowed[key] = true
	}
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if allowed[strings.ToUpper(key)] {
			env = append(env, entry)
		}
	}
	if config.CodexHomeEnv != "" {
		value, ok := os.LookupEnv(config.CodexHomeEnv)
		if !ok || !filepath.IsAbs(value) {
			return nil, errors.New("native authentication home reference is missing or invalid")
		}
		info, err := os.Stat(value)
		if err != nil || !info.IsDir() {
			return nil, errors.New("native authentication home reference is unavailable")
		}
		filtered := env[:0]
		for _, entry := range env {
			key, _, _ := strings.Cut(entry, "=")
			if !strings.EqualFold(key, "CODEX_HOME") {
				filtered = append(filtered, entry)
			}
		}
		env = append(filtered, "CODEX_HOME="+value)
	}
	return env, nil
}
func classifyFailure(stderr string) string {
	s := strings.ToLower(stderr)
	for _, entry := range []struct {
		words []string
		code  string
	}{
		{[]string{"unauthorized", "authentication", "please log in", "401"}, "parser_authentication_failed"},
		{[]string{"model is not supported", "model_not_found", "model does not exist", "not available for", "unsupported model"}, "parser_model_unavailable"},
		{[]string{"rate limit", "quota", "usage limit"}, "parser_usage_limit"},
		{[]string{"configuration", "config.toml", "invalid value"}, "parser_configuration_failed"},
	} {
		for _, word := range entry.words {
			if strings.Contains(s, word) {
				return entry.code
			}
		}
	}
	if strings.Contains(s, "temporarily unavailable") || strings.Contains(s, "connection reset") {
		return "parser_transient_failure"
	}
	return "parser_native_invocation_failed"
}

type Stream struct {
	SessionID, Final  string
	FailureCode       string
	Complete, Started bool
}

func (s *Stream) Accept(line []byte) error {
	var v struct {
		Type, ThreadID string
		Thread         string `json:"thread_id"`
		Item           struct{ Type, Text, ID, Message string }
		Error          struct{ Message string }
		Message        string
	}
	if json.Unmarshal(line, &v) != nil {
		return errors.New("parser JSONL invalid")
	}
	switch v.Type {
	case "thread.started":
		if s.SessionID != "" || v.Thread == "" {
			return errors.New("parser session identity conflict")
		}
		s.SessionID = v.Thread
	case "turn.started":
		if s.Started || s.SessionID == "" {
			return errors.New("parser turn identity conflict")
		}
		s.Started = true
	case "turn.completed":
		if !s.Started || s.Complete {
			return errors.New("parser duplicate/incomplete turn")
		}
		s.Complete = true
	case "turn.failed", "error":
		s.FailureCode = classifyFailure(v.Error.Message + " " + v.Message)
		return errors.New("parser native turn failed")
	case "item.started", "item.updated", "item.completed":
		// The native CLI can emit a startup configuration error after
		// thread.started but before turn.started. It is never a result.
		if v.Item.Type == "error" {
			s.FailureCode = classifyFailure(v.Item.Message)
			return errors.New("parser native item error")
		}
		if !s.Started || s.Complete {
			return errors.New("parser item outside original turn")
		}
		switch v.Item.Type {
		case "reasoning":
		case "agent_message":
			if v.Type == "item.completed" {
				if s.Final != "" {
					return errors.New("parser ambiguous multiple final messages")
				}
				s.Final = v.Item.Text
			}
		default:
			return errors.New("parser tool event rejected")
		}
	default:
		return errors.New("parser unknown native event")
	}
	return nil
}
