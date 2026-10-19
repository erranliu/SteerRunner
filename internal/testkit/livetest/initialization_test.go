package livetest

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInitializationAgentFaultPreservesOriginalEnvelope(t *testing.T) {
	raw := []byte(`{"ok":true,"result":{"terminal":{"handle":"original","incarnationId":"original-inc","worktreeId":"original-worktree","agentIdentity":"codex","connected":true,"status":"running","agentWait":{"phase":"running"}}},"_meta":{"runtimeId":"original-runtime"}}`)
	delivered, hidden, err := WithholdNativeAgentIdentity(raw)
	if err != nil || !hidden {
		t.Fatal(string(delivered), err)
	}
	var original, actual map[string]any
	_ = json.Unmarshal(raw, &original)
	_ = json.Unmarshal(delivered, &actual)
	original["result"].(map[string]any)["terminal"].(map[string]any)["agentIdentity"] = ""
	expected, _ := json.Marshal(original)
	got, _ := json.Marshal(actual)
	if string(got) != string(expected) {
		t.Fatal("fault changed original identities", string(got))
	}
	for _, old := range []string{"powershell", "pi"} {
		source := []byte(strings.ReplaceAll(string(raw), `"codex"`, `"`+old+`"`))
		got, hidden, err := WithholdNativeAgentIdentity(source)
		if err != nil || hidden || string(got) != string(source) {
			t.Fatal("foreign native identity was changed")
		}
	}
	for _, bad := range []string{`{}`, `{"result":42}`, `{"result":{"terminal":42}}`} {
		if _, _, err := WithholdNativeAgentIdentity([]byte(bad)); err == nil {
			t.Fatal("malformed source accepted", bad)
		}
	}
	for _, change := range []string{strings.ReplaceAll(string(raw), `"connected":true`, `"connected":false`), strings.ReplaceAll(string(raw), `"status":"running"`, `"status":"exited"`)} {
		if _, hidden, _ := WithholdNativeAgentIdentity([]byte(change)); hidden {
			t.Fatal("inactive terminal became readiness witness")
		}
	}
}
