package contracts

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSRCLIContract(t *testing.T) {
	valid := `{"protocol":"steerrunner-cli/v1","request_id":"中文-request","input":{"expected_instance_id":"original"}}`
	cases := []struct {
		name, body string
		exit       int
	}{
		{"valid", valid, 0},
		{"duplicate", `{"protocol":"steerrunner-cli/v1","request_id":"one","request_id":"two","input":{}}`, 2},
		{"nested_duplicate", `{"protocol":"steerrunner-cli/v1","request_id":"one","input":{"x":1,"x":2}}`, 2},
		{"field_alias", `{"Protocol":"steerrunner-cli/v1","request_id":"one","input":{}}`, 2},
		{"unknown", `{"protocol":"steerrunner-cli/v1","request_id":"one","input":{},"secret":"do-not-echo"}`, 2},
		{"trailing", valid + `{}`, 2},
		{"unsupported", strings.Replace(valid, Protocol, "steerrunner-cli/v99", 1), 3},
		{"missing_protocol", `{"request_id":"one","input":{}}`, 2},
		{"null_protocol", `{"protocol":null,"request_id":"one","input":{}}`, 2},
		{"control_id", `{"protocol":"steerrunner-cli/v1","request_id":"one\nforged-log","input":{}}`, 2},
		{"invalid_utf8", string([]byte{0xff, '{', '}'}), 2},
		{"oversize", strings.Repeat(" ", MaxRequestBytes+1), 2},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request, fault := ReadRequest(strings.NewReader(item.body))
			if item.exit == 0 {
				if fault != nil || ValidateInput("service stop", request.Input) != nil {
					t.Fatalf("valid request rejected: %v", fault)
				}
			} else if fault == nil || fault.Exit != item.exit {
				t.Fatalf("got %v; wanted exit %d", fault, item.exit)
			}
			if fault != nil && strings.Contains(fault.Error(), "do-not-echo") {
				t.Fatal("request contents leaked")
			}
		})
	}
	t.Run("response_channels", func(t *testing.T) {
		var output bytes.Buffer
		if err := Encode(Rejected("中文", Fail(6, "request_outcome_unknown", "retry original request")), &output); err != nil {
			t.Fatal(err)
		}
		var response Response
		if err := DecodeStrict(output.Bytes(), &response); err != nil || response.RequestID == nil || *response.RequestID != "中文" || ExitCode(response) != 6 {
			t.Fatalf("invalid response: %s", output.Bytes())
		}
	})
	t.Run("settings_and_paths", func(t *testing.T) {
		body := validRun()
		if failure := ValidateRun(body); failure != nil {
			t.Fatal(failure)
		}
		var value map[string]any
		json.Unmarshal(body, &value)
		value["execution"].(map[string]any)["settings"] = map[string]any{"token": "known-secret-value"}
		invalid, _ := json.Marshal(value)
		if failure := ValidateRun(invalid); failure == nil || strings.Contains(failure.Error(), "known-secret-value") {
			t.Fatal("secret/unknown settings were not rejected safely")
		}
		if safeDocument(Document{Kind: "inline", Path: "../outside.md", Content: stringPointer("text")}) {
			t.Fatal("material traversal allowed")
		}
		var pathsOnly RunRequest
		_ = json.Unmarshal(body, &pathsOnly)
		pathsOnly.Documents = Documents{Spec: Document{Path: "docs/spec.md"}, Plan: Document{Path: "docs/plan.md"}, References: []Document{{Path: "docs/reference.md"}}}
		pathInput, _ := json.Marshal(pathsOnly)
		if failure := ValidateRun(pathInput); failure != nil {
			t.Fatal("worktree document paths require no content or independent commit", failure)
		}
		for _, name := range []string{"../outside.md", "/absolute.md", `C:\outside.md`, "docs/../spec.md"} {
			if safeDocument(Document{Path: name}) {
				t.Fatal("unsafe worktree document path accepted", name)
			}
		}
		if !safeURL("ssh://git@example.com/project.git") || safeURL("https://user:secret@example.com/project.git") {
			t.Fatal("SSH identity or secret URL handling incorrect")
		}
		for _, settings := range []string{`{"model":""}`, `{"executable":null}`, `{"reasoning_effort":"unknown"}`, `{"model":"ok","Model":"alias"}`} {
			var request RunRequest
			_ = json.Unmarshal(body, &request)
			request.Execution.Settings = json.RawMessage(settings)
			bad, _ := json.Marshal(request)
			if ValidateRun(bad) == nil {
				t.Fatalf("settings accepted: %s", settings)
			}
		}
	})
}

func stringPointer(value string) *string { return &value }

func validRun() []byte {
	value := RunRequest{
		Title: "中文任务", Repository: Repository{URL: "https://example.com/repo.git", BaseRevision: strings.Repeat("a", 40), Remote: "origin", BaseBranch: "main"},
		Documents:   Documents{Spec: Document{Kind: "inline", Path: "spec.md", Content: stringPointer("Spec")}, Plan: Document{Kind: "inline", Path: "plan.md", Content: stringPointer("Plan")}},
		Workflow:    Workflow{ID: "development", Version: 2},
		Execution:   Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{}`)},
		Publication: Publication{Provider: "gitea", Repository: "owner/repo", BaseBranch: "main", Settings: json.RawMessage(`{"base_url":"https://example.com"}`)},
	}
	body, _ := json.Marshal(value)
	return body
}
