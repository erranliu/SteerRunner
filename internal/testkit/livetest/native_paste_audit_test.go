package livetest

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func nativePasteAuditCalls() []nativeAuditCall {
	response := func(handle string, n int) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "runtime"}, "result": map[string]any{"send": map[string]any{"handle": handle, "accepted": true, "bytesWritten": n}}})
		return raw
	}
	frame := "\x1b[200~Implement\n中文\x1b[201~"
	var calls []nativeAuditCall
	for _, c := range []struct {
		id   string
		args []string
		size int
	}{
		{"workspace", []string{"worktree", "create"}, 0},
		{"launch", []string{"terminal", "send", "--terminal", "original", "--text", "codex launch", "--enter", "--json", "--environment", "paired"}, 13},
		{"paste", []string{"terminal", "send", "--terminal", "original", "--text", frame, "--json", "--environment", "paired"}, len(frame)},
		{"enter", []string{"terminal", "send", "--terminal", "original", "--enter", "--json", "--environment", "paired"}, 1},
	} {
		calls = append(calls, nativeAuditCall{Kind: "intent", ID: c.id, Args: c.args}, nativeAuditCall{Kind: "receipt", ID: c.id, Args: c.args, Response: response("original", c.size)})
	}
	return calls
}
func nativePasteAuditJSON(calls []nativeAuditCall) []byte {
	var raw []byte
	for _, c := range calls {
		line, _ := json.Marshal(c)
		raw = append(append(raw, line...), 10)
	}
	return raw
}
func TestNativeAuditPairsOriginalPasteAndEnter(t *testing.T) {
	p := NativeReceiptAuditPolicy{NativeLaunchCommand: "codex launch", Environment: "paired"}
	calls := nativePasteAuditCalls()
	if f, e := AnalyzeNativeAudit(nativePasteAuditJSON(calls), "runtime", p); e != nil || !f.Complete || f.Creations != 1 || len(f.Prompts) != 2 {
		t.Fatalf("original split input rejected: %+v %v", f, e)
	}
	for _, n := range []int{5, 6, 7} {
		t.Run("incomplete-"+strconv.Itoa(n), func(t *testing.T) {
			f, e := AnalyzeNativeAudit(nativePasteAuditJSON(calls[:n]), "runtime", p)
			if e == nil && f.Complete {
				t.Fatal("incomplete original input qualified")
			}
		})
	}
	tests := map[string]func([]nativeAuditCall) []nativeAuditCall{
		"unmatched-enter": func(c []nativeAuditCall) []nativeAuditCall { return append(c[:4], c[6:]...) },
		"duplicate-enter": func(c []nativeAuditCall) []nativeAuditCall { x := c[6]; x.ID = "extra-enter"; return append(c, x) },
		"interleaved-paste": func(c []nativeAuditCall) []nativeAuditCall {
			x := c[4]
			x.ID = "extra-paste"
			return append(append(c[:6], x), c[6:]...)
		},
		"enter-before-paste-receipt": func(c []nativeAuditCall) []nativeAuditCall { c[5], c[6] = c[6], c[5]; return c },
		"wrong-terminal":             func(c []nativeAuditCall) []nativeAuditCall { c[6].Args[3] = "foreign"; return c },
		"wrong-environment":          func(c []nativeAuditCall) []nativeAuditCall { c[6].Args[len(c[6].Args)-1] = "other"; return c },
		"extra-flags":                func(c []nativeAuditCall) []nativeAuditCall { c[6].Args = append(c[6].Args, "--interrupt"); return c },
		"duplicate-flags":            func(c []nativeAuditCall) []nativeAuditCall { c[6].Args = append(c[6].Args, "--enter"); return c },
		"broken-frame": func(c []nativeAuditCall) []nativeAuditCall {
			c[4].Args[5] = strings.TrimSuffix(c[4].Args[5], "\x1b[201~")
			return c
		},
		"embedded-control":     func(c []nativeAuditCall) []nativeAuditCall { c[4].Args[5] = "\x1b[200~text\x1b[0m\x1b[201~"; return c },
		"failed-paste":         func(c []nativeAuditCall) []nativeAuditCall { c[5].Failed = true; return c },
		"failed-enter":         func(c []nativeAuditCall) []nativeAuditCall { c[7].Failed = true; return c },
		"receipt-args-changed": func(c []nativeAuditCall) []nativeAuditCall { c[7].Args = append(c[7].Args, "--interrupt"); return c },
	}
	for _, field := range []string{"accepted", "handle", "bytesWritten", "runtimeId"} {
		field := field
		tests["receipt-"+field] = func(c []nativeAuditCall) []nativeAuditCall {
			r := string(c[7].Response)
			switch field {
			case "accepted":
				r = strings.Replace(r, `"accepted":true`, `"accepted":false`, 1)
			case "handle":
				r = strings.Replace(r, `"handle":"original"`, `"handle":"other"`, 1)
			case "bytesWritten":
				r = strings.Replace(r, `"bytesWritten":1`, `"bytesWritten":2`, 1)
			case "runtimeId":
				r = strings.Replace(r, `"runtimeId":"runtime"`, `"runtimeId":"other"`, 1)
			}
			c[7].Response = json.RawMessage(r)
			return c
		}
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			var copy []nativeAuditCall
			if e := json.Unmarshal(nativePasteAuditJSONAsArray(calls), &copy); e != nil {
				t.Fatal(e)
			}
			f, e := AnalyzeNativeAudit(nativePasteAuditJSON(change(copy)), "runtime", p)
			if e == nil && f.Complete {
				t.Fatalf("altered native input qualified: %+v", f)
			}
		})
	}
}
func nativePasteAuditJSONAsArray(c []nativeAuditCall) []byte { raw, _ := json.Marshal(c); return raw }
