package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRCLIContract(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
		exit  int
	}{
		{"version", []string{"version", "--json"}, "", 0},
		{"unknown_command", []string{"unknown", "--json"}, "", 2},
		{"last_message_requires_run", []string{"last-message", "--json"}, "", 2},
		{"native_viewer_requires_identity", []string{"open-native-session", "--run-id", "a", "--json"}, "", 2},
		{"native_viewer_no_input", []string{"open-native-session", "--run-id", "a", "--attempt-id", "attempt", "--session-id", "session", "--generation", "gen", "--input", "-"}, "{}", 2},
		{"native_options_not_for_inspect", []string{"inspect", "--run-id", "a", "--session-id", "session"}, "", 2},
		{"last_message_no_write", []string{"last-message", "--run-id", "a", "--input", "-", "--json"}, "{}", 2},
		{"last_message_no_request_selector", []string{"last-message", "--request-id", "a", "--json"}, "", 2},
		{"last_message_no_refresh_pr", []string{"last-message", "--run-id", "a", "--refresh-pr", "--json"}, "", 2},
		{"unknown_option", []string{"version", "--unknown", "value", "--json"}, "", 2},
		{"duplicate_option", []string{"service", "start", "--state-dir", "C:/one", "--state-dir", "C:/two", "--json"}, "", 2},
		{"protocol", []string{"version", "--protocol", "steerrunner-cli/v99", "--json"}, "", 3},
		{"selector", []string{"inspect", "--run-id", "a", "--request-id", "b", "--json"}, "", 2},
		{"missing_input", []string{"stop", "--run-id", "a", "--json"}, "", 2},
		{"reply_schema", []string{"reply", "--run-id", "a", "--input", "-", "--json"}, `{"protocol":"steerrunner-cli/v1","request_id":"one","input":{"expected_revision":1,"kind":"answer","text":"value"}}`, 2},
		{"invalid_utf8", []string{"submit", "--input", "-", "--json"}, string([]byte{0xff}), 2},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var output, diagnostics bytes.Buffer
			exit := Run(item.args, strings.NewReader(item.input), &output, &diagnostics)
			if exit != item.exit {
				t.Fatalf("exit=%d wanted=%d output=%s stderr=%s", exit, item.exit, output.Bytes(), diagnostics.Bytes())
			}
			var response contracts.Response
			if err := contracts.DecodeStrict(output.Bytes(), &response); err != nil {
				t.Fatalf("not one JSON response: %s", output.Bytes())
			}
			if diagnostics.Len() != 0 {
				t.Fatalf("unexpected diagnostic content: %s", diagnostics.Bytes())
			}
			if response.OK != (exit == 0) {
				t.Fatalf("exit/envelope disagreement: %s", output.Bytes())
			}
		})
	}
}

func TestSRExpectedInstanceDoesNotStartService(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	var output, diagnostics bytes.Buffer
	exit := Run([]string{"environment-check", "start", "--state-dir", dir, "--expected-instance-id", "A", "--input", "-"}, strings.NewReader(`{"protocol":"steerrunner-cli/v1","request_id":"check-original","input":{}}`), &output, &diagnostics)
	if exit != 4 || !strings.Contains(output.String(), "service_unavailable") {
		t.Fatalf("exit=%d output=%s", exit, &output)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("bound call started/created replacement service: %v", err)
	}
	parsed, fault := parse([]string{"inspect", "--run-id", "run", "--expected-instance-id", "A"})
	if fault != nil || parsed.opts.ExpectedInstanceID != "A" {
		t.Fatal(parsed, fault)
	}
	for _, command := range [][]string{{"service", "start"}, {"version"}, {"service", "status"}} {
		if _, fault := parse(append(command, "--expected-instance-id", "A")); fault == nil {
			t.Fatal("unsupported binding ignored", command)
		}
	}
}
