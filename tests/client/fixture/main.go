// Controlled public-CLI fixture. Production validation is delegated unchanged
// to the supplied actual EXE; synthetic execution facts never count as live QA.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func output(v any) { _ = json.NewEncoder(os.Stdout).Encode(v) }
func value(args []string, key string) string {
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
func response(ok bool, data any, code string) {
	var fault any
	if !ok {
		fault = map[string]any{"code": code, "message": "controlled fixture"}
	}
	output(map[string]any{"protocol": "steerrunner-cli/v1", "request_id": nil, "ok": ok, "data": data, "error": fault})
}
func main() {
	args := os.Args[1:]
	name := strings.ToLower(filepath.Base(os.Args[0]))
	root := os.Getenv("SRCLI_TEST_ROOT")
	if strings.HasPrefix(name, "codex") {
		fmt.Println("codex-cli 0.159.2")
		return
	}
	if strings.HasPrefix(name, "gh") {
		if len(args) > 0 && args[0] == "api" {
			fmt.Println("main")
		}
		return
	}
	if strings.HasPrefix(name, "orca") {
		scenario := os.Getenv("SRCLI_TEST_SCENARIO")
		if scenario == "orca-auth" && len(args) > 0 && args[0] == "environment" {
			output(map[string]any{"ok": false, "error": map[string]any{"code": "authentication_failed"}})
			os.Exit(4)
		}
		if scenario == "orca-offline" && len(args) > 0 && args[0] == "status" {
			output(map[string]any{"ok": false, "error": map[string]any{"code": "unavailable"}})
			os.Exit(4)
		}
		var result any = map[string]any{"runtime": map[string]any{"state": "ready"}}
		if len(args) > 1 && args[0] == "environment" {
			result = map[string]any{"environments": []any{map[string]any{"id": "remote-one", "name": "remote"}}}
			if scenario == "orca-empty" {
				result = map[string]any{"environments": []any{}}
			}
		}
		if len(args) > 1 && args[0] == "repo" {
			result = map[string]any{"repos": []any{map[string]any{"id": "repo", "path": "C:/remote/project"}}}
			if args[1] == "show" {
				result = map[string]any{"repo": map[string]any{"remoteUrl": os.Getenv("SRCLI_TEST_URL")}}
			}
		}
		output(map[string]any{"ok": true, "result": result})
		return
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	if args[0] == "validate" || args[0] == "version" || args[0] == "capabilities" && value(args, "--state-dir") == "" {
		cmd := exec.Command(os.Getenv("SRCLI_REAL_RUNNER"), args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		return
	}
	instance := os.Getenv("SRCLI_TEST_INSTANCE")
	if instance == "" {
		instance = "original"
	}
	if raw := value(args, "--expected-binding"); raw != "" {
		var b map[string]any
		_ = json.Unmarshal([]byte(raw), &b)
		if b["instance_id"] != instance {
			response(false, nil, "stale_instance")
			os.Exit(5)
		}
	}
	if args[0] == "service" {
		response(true, map[string]any{"status": "ready", "state_dir": value(args, "--state-dir"), "instance_id": instance, "executable_sha256": strings.Repeat("a", 64), "resource_digest": strings.Repeat("b", 64), "version": "test", "protocol": "steerrunner-cli/v1"}, "")
		return
	}
	if args[0] == "capabilities" {
		response(true, map[string]any{"capability_source": map[string]any{"instance_id": instance}}, "")
		return
	}
	ledgerPath := filepath.Join(root, "ledger.json")
	ledger := map[string]json.RawMessage{}
	b, _ := os.ReadFile(ledgerPath)
	_ = json.Unmarshal(b, &ledger)
	run := map[string]any{"run_id": "fixture-run", "revision": 1, "title": "controlled", "status": "running", "question": map[string]any{"id": "question-1"}, "controls": map[string]any{"can_reply": true, "can_stop": true, "can_resume": true}}
	if os.Getenv("SRCLI_TEST_SCENARIO") == "no-question" {
		delete(run, "question")
	}
	if args[0] == "inspect" {
		id := value(args, "--request-id")
		if id != "" {
			if original, ok := ledger[id]; ok {
				var req map[string]any
				_ = json.Unmarshal(original, &req)
				response(true, map[string]any{"command": map[string]any{"request_id": id}, "run": run}, "")
				return
			}
			response(false, nil, "request_not_found")
			os.Exit(5)
		}
		response(true, run, "")
		return
	}
	if args[0] == "observe" {
		response(true, map[string]any{"run": run, "event_page": map[string]any{"next_cursor": "cursor-1", "events": []any{}, "has_more": false}}, "")
		return
	}
	if args[0] == "submit" || args[0] == "reply" || args[0] == "resume" || args[0] == "stop" {
		input, _ := io.ReadAll(os.Stdin)
		var req map[string]any
		_ = json.Unmarshal(input, &req)
		id, _ := req["request_id"].(string)
		if old, exists := ledger[id]; exists {
			if string(old) != string(input) {
				response(false, nil, "request_conflict")
				os.Exit(5)
			}
			output(map[string]any{"protocol": "steerrunner-cli/v1", "request_id": id, "ok": true, "error": nil, "data": map[string]any{"run_id": "fixture-run", "receipt_status": "accepted"}})
			return
		}
		ledger[id] = input
		b, _ := json.Marshal(ledger)
		_ = os.WriteFile(ledgerPath, b, 0600)
		f, _ := os.OpenFile(filepath.Join(root, "writes.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		fmt.Fprintln(f, args[0], id)
		f.Close()
		if os.Getenv("SRCLI_TEST_LOST_RESPONSE") == "1" {
			fmt.Print("truncated")
			return
		}
		output(map[string]any{"protocol": "steerrunner-cli/v1", "request_id": id, "ok": true, "error": nil, "data": map[string]any{"run_id": "fixture-run", "receipt_status": "accepted"}})
		return
	}
	response(false, nil, "unsupported_command")
	os.Exit(2)
}
