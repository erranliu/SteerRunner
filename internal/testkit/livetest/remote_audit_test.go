package livetest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestRemotePermissionAuditBindsReceiptAndClosesOnlyInspector(t *testing.T) {
	for _, mode := range []string{"valid", "foreign-session", "foreign-workspace", "unverified", "foreign-read-runtime", "invalid-receipt"} {
		t.Run(mode, func(t *testing.T) {
			original := b.ProcessIdentity{RuntimeID: "runtime", WorktreeID: "repo::C:/worker", Terminal: "main", Incarnation: "main-inc"}
			receipt := codex.RemoteAudit{Readiness: codex.RemoteReadiness{Verified: mode != "unverified", Workspace: "C:/worker", SessionID: "provider"}}
			receipt.Native.Session = "provider"
			if mode == "foreign-session" {
				receipt.Native.Session = "other"
			}
			if mode == "foreign-workspace" {
				receipt.Readiness.Workspace = "C:/other"
			}
			body, _ := json.Marshal(receipt)
			line := "SR_REMOTE_AUDIT:" + base64.RawURLEncoding.EncodeToString(body)
			if mode == "invalid-receipt" {
				line = "SR_REMOTE_AUDIT:not-valid-json"
			}
			creates, closes := 0, 0
			client := native.New(native.Config{Output: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				runtime := "runtime"
				terminal := map[string]any{"handle": "inspector", "incarnationId": "inspector-inc", "worktreeId": original.WorktreeID, "connected": true, "status": "running", "tail": []string{line}}
				var result any
				switch args[1] {
				case "create":
					creates++
					if !strings.Contains(strings.Join(args, " "), "--audit") {
						t.Fatal("audit did not use fixed helper", args)
					}
					result = map[string]any{"terminal": terminal}
				case "show", "read", "close":
					if len(args) < 4 || args[2] != "--terminal" || args[3] != "inspector" {
						t.Fatal("touched original business terminal", args)
					}
					result = map[string]any{"terminal": terminal}
					if args[1] == "read" && mode == "foreign-read-runtime" {
						runtime = "foreign"
					}
					if args[1] == "close" {
						closes++
						result = map[string]any{"close": map[string]any{"handle": "inspector", "ptyKilled": true}}
					}
				default:
					t.Fatal("audit sent business input or unexpected operation", args)
				}
				return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
			}})
			audit, err := RemotePermissionAudit(context.Background(), client, original, "C:/frozen/helper.exe", "C:/worker", "provider")
			if (err == nil) != (mode == "valid") || creates != 1 || closes != 1 || !audit.InspectorClosed || audit.Inspector == nil || audit.Inspector.Terminal != "inspector" {
				t.Fatal("receipt accepted or inspector cleanup unconfirmed", audit, err, creates, closes)
			}
		})
	}
}
