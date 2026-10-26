package contracts

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestResultFileAdmissionAndOpaqueByteBounds(t *testing.T) {
	for _, progress := range []bool{false, true} {
		for _, seed := range []string{"", base64.StdEncoding.EncodeToString([]byte{0, 255, '{'}), base64.StdEncoding.EncodeToString(make([]byte, 65536))} {
			var input map[string]any
			_ = json.Unmarshal(validRun(), &input)
			input["result_file"] = map[string]any{"initial_content_base64": seed}
			if progress {
				input["progress_file"] = map[string]any{"initial_content_base64": "cA=="}
			}
			raw, _ := json.Marshal(input)
			if fault := ValidateRun(raw); fault != nil {
				t.Fatal(fault)
			}
		}
	}
	for _, value := range []any{nil, map[string]any{}, map[string]any{"initial_content_base64": nil}, map[string]any{"initial_content_base64": "YQ==\n"}, map[string]any{"initial_content_base64": "?"}, map[string]any{"initial_content_base64": base64.StdEncoding.EncodeToString(make([]byte, 65537))}, map[string]any{"initial_content_base64": "", "path": "C:/foreign"}} {
		var input map[string]any
		_ = json.Unmarshal(validRun(), &input)
		input["result_file"] = value
		raw, _ := json.Marshal(input)
		if ValidateRun(raw) == nil {
			t.Fatal("invalid result input accepted", value)
		}
	}
	for _, r := range []RunRequest{{Execution: Execution{Adapter: "other"}, ResultFile: &ResultFileInput{}}, {Execution: Execution{Adapter: "codex-app-server"}, ResultFile: &ResultFileInput{}, Conversation: &ConversationInput{}}, {Execution: Execution{Adapter: "codex-app-server"}, ResultFile: &ResultFileInput{}, Generate: &GenerateInput{}}} {
		if fault := ValidateResultFile(r); fault == nil || fault.Detail.Code != "unsupported_capability" {
			t.Fatal(fault)
		}
	}
	if !ServiceBindingCommand("result-file") || ResultFileContract == ProgressContract || ResultFileEnvironment == ProgressEnvironment || !strings.Contains(ResultFileEnvironment, "RESULT") {
		t.Fatal("channel identity/fence missing")
	}
}
