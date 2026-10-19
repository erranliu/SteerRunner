package codexworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestFixedControlFiniteProtocol(t *testing.T) {
	setup := RemoteControlSetup{Binding: strings.Repeat("a", 64), Workspace: t.TempDir(), RuntimeSHA256: strings.Repeat("b", 64)}
	q := RemoteControlRequest{ID: strings.Repeat("c", 32), Binding: setup.Binding, Method: "original-turn", Session: "01a1433a-c857-7fc0-bf8c-02a784ad118e", InputDigest: strings.Repeat("d", 64)}
	for _, mode := range []string{"query", "revoke", "duplicate", "conflict", "method", "session", "binding", "input", "unknown-field"} {
		t.Run(mode, func(t *testing.T) {
			request := q
			switch mode {
			case "revoke":
				request.Method = "authentication-stop"
				request.InputDigest = ""
			case "method":
				request.Method = "exec"
			case "session":
				request.Session = "foreign"
			case "binding":
				request.Binding = "foreign"
			case "input":
				request.InputDigest = "missing"
			}
			encode := func(value any) string {
				raw, _ := json.Marshal(value)
				return base64.RawURLEncoding.EncodeToString(raw) + "\n"
			}
			input := encode(request)
			if mode == "duplicate" {
				input += encode(request)
			}
			if mode == "conflict" {
				request.InputDigest = strings.Repeat("e", 64)
				input += encode(request)
			}
			if mode == "unknown-field" {
				raw, _ := json.Marshal(request)
				raw = append(raw[:len(raw)-1], []byte(",\"command\":\"publish\"}")...)
				input = base64.RawURLEncoding.EncodeToString(raw) + "\n"
			}
			calls := 0
			var out bytes.Buffer
			err := serveRemoteControl(context.Background(), setup, strings.NewReader(input), &out, func(ctx context.Context, got RemoteControlRequest) RemoteControlResponse {
				calls++
				if ctx.Err() != nil {
					t.Fatal(ctx.Err())
				}
				return RemoteControlResponse{Error: "original_turn_unverified"}
			})
			valid := mode == "query" || mode == "revoke" || mode == "duplicate"
			if (err == nil) != valid {
				t.Fatal(mode, err)
			}
			want := 0
			if valid || mode == "conflict" {
				want = 1
			}
			if calls != want {
				t.Fatal("unexpected native calls", calls, want)
			}
			if mode == "duplicate" {
				lines := strings.Split(strings.TrimSpace(out.String()), "\n")
				if len(lines) != 3 || lines[1] != lines[2] {
					t.Fatal("cached response changed")
				}
			}
		})
	}
}
