package livetest

import (
	"strings"
	"testing"
)

func TestSandboxAuditRejectsEscalationWrongPolicyAndMissingEvidence(t *testing.T) {
	good := `{"type":"session_meta","payload":{"id":"original","cwd":"C:/worker"}}
{"type":"turn_context","payload":{"approval_policy":"never","sandbox_policy":{"type":"workspace-write","network_access":true}}}
{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"git push\"}"}}
`
	for _, c := range []struct {
		name, text string
		ok         bool
	}{
		{"good", good, true},
		{"missing", "", false},
		{"wrong-policy", strings.ReplaceAll(good, "never", "on-request"), false},
		{"wrong-session", strings.ReplaceAll(good, "original", "other"), false},
		{"keyword-is-not-request", strings.ReplaceAll(good, "git push", "echo require_escalated"), true},
		{"escalation", strings.ReplaceAll(good, `\"cmd\":\"git push\"`, `\"cmd\":\"git push\",\"sandbox_permissions\":\"require_escalated\"`), false},
		{"bypass", strings.ReplaceAll(good, "git push", "--dangerously-bypass-approvals-and-sandbox"), false},
		{"approval", good + `{"type":"event_msg","payload":{"type":"approval_requested"}}`, false},
		{"code-mode", strings.Replace(good, `"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"git push\"}"`, `"type":"custom_tool_call","name":"exec","input":"text(await tools.exec_command({cmd:'git push'}));"`, 1), true},
		{"code-mode-escalation", strings.Replace(good, `"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"git push\"}"`, `"type":"custom_tool_call","name":"exec","input":"text(await tools.exec_command({cmd:'git push',sandbox_permissions:'require_escalated'}));"`, 1), false},
		{"unknown-custom-tool", strings.Replace(good, `"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"git push\"}"`, `"type":"custom_tool_call","name":"unknown","input":"opaque"`, 1), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := AuditSandboxHistory(strings.NewReader(c.text), "original", "C:/worker")
			if (err == nil) != c.ok {
				t.Fatalf("audit: %+v %v", a, err)
			}
		})
	}
}
