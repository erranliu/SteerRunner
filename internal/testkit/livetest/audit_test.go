package livetest

import (
	"encoding/json"
	"testing"
)

func TestNativeAuditCountsImplicitTerminalCreationAndFencesReceipts(t *testing.T) {
	lines := []byte{}
	appendLine := func(v any) { raw, _ := json.Marshal(v); lines = append(lines, raw...); lines = append(lines, '\n') }
	for _, c := range []struct {
		id   string
		args []string
	}{{"workspace", []string{"worktree", "create"}}, {"launch", []string{"terminal", "send", "--text", "codex launch"}}, {"implement", []string{"terminal", "send", "--text", "implement"}}} {
		appendLine(map[string]any{"kind": "intent", "call_id": c.id, "args": c.args})
		appendLine(map[string]any{"kind": "receipt", "call_id": c.id, "args": c.args, "response": map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "original"}}})
	}
	policy := NativeReceiptAuditPolicy{NativeLaunchCommand: "codex launch", Environment: "paired"}
	if facts, err := AnalyzeNativeAudit(lines, "original", policy); err != nil || !facts.Complete {
		t.Fatal("native launch rejected", err)
	}
	for _, bad := range []string{"changed-launch", "upload", "extra-terminal"} {
		altered := append([]byte{}, lines...)
		if bad == "changed-launch" {
			policy.NativeLaunchCommand = "other codex"
		} else {
			args := []string{"file", "upload"}
			if bad == "extra-terminal" {
				args = []string{"terminal", "create"}
			}
			raw, _ := json.Marshal(nativeAuditCall{Kind: "intent", ID: bad, Args: args})
			altered = append(altered, raw...)
			altered = append(altered, '\n')
		}
		if _, err := AnalyzeNativeAudit(altered, "original", policy); err == nil {
			t.Fatal("unsafe native audit accepted", bad)
		}
		policy.NativeLaunchCommand = "codex launch"
	}
	facts, err := AnalyzeNativeAudit(lines, "original")
	if err != nil || !facts.Complete || facts.Creations != 1 || len(facts.Prompts) != 2 {
		t.Fatalf("real worktree-owned terminal audit rejected: %+v %v", facts, err)
	}
	if facts, err = AnalyzeNativeAudit(lines, "replacement"); err != nil || facts.Complete {
		t.Fatal("foreign runtime accepted")
	}
	appendLine(map[string]any{"kind": "intent", "call_id": "second-input", "args": []string{"terminal", "send", "--text", "implement"}})
	if facts, err = AnalyzeNativeAudit(lines, "original"); err != nil || facts.Complete || len(facts.Prompts) != 3 {
		t.Fatal("pending repeated mutation lost from audit")
	}
	appendLine(map[string]any{"kind": "intent", "call_id": "second-input", "args": []string{"terminal", "send", "--text", "implement"}})
	if _, err = AnalyzeNativeAudit(lines, "original"); err == nil {
		t.Fatal("duplicate intent identity accepted")
	}
}
