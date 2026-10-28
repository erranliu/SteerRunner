package orca

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestBoundComposerViewportOwnsFitAndRestore(t *testing.T) {
	for _, fault := range []string{"none", "wrong-runtime", "wrong-terminal", "wrong-incarnation", "wrong-worktree", "foreign-fit", "bad-fit-size", "bad-restore-size"} {
		t.Run(fault, func(t *testing.T) {
			fits, restores := 0, 0
			owner := ""
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "terminal.show":
					handle, inc, wt, runtime := "terminal", "incarnation", "worktree", "runtime"
					if fault == "wrong-runtime" {
						runtime = "foreign"
					}
					if fault == "wrong-terminal" {
						handle = "foreign"
					}
					if fault == "wrong-incarnation" {
						inc = "foreign"
					}
					if fault == "wrong-worktree" {
						wt = "foreign"
					}
					ipcReply(conn, req, runtime, map[string]any{"terminal": map[string]any{"handle": handle, "incarnationId": inc, "worktreeId": wt}})
				case "terminal.getDisplayMode":
					ipcReply(conn, req, "runtime", map[string]any{"mode": "auto", "isPhoneFitted": fault == "foreign-fit"})
				case "terminal.resizeForClient":
					var p struct {
						Terminal, Mode, ClientID string
						Cols, Rows               int
					}
					if json.Unmarshal(req.Params, &p) != nil || p.Terminal != "terminal" || p.ClientID == "" {
						t.Error("invalid exact fit identity")
						return
					}
					if p.Mode == "mobile-fit" {
						fits++
						owner = p.ClientID
						if p.Cols != 1024 || p.Rows != 80 {
							t.Error("unexpected dimensions")
						}
						cols := 1024
						if fault == "bad-fit-size" {
							cols--
						}
						ipcReply(conn, req, "runtime", map[string]any{"terminal": map[string]any{"handle": "terminal", "mode": "mobile-fit", "cols": cols, "rows": 80, "previousCols": 180, "previousRows": 40}})
					} else if p.Mode == "restore" {
						restores++
						if p.ClientID != owner {
							t.Error("foreign override restored")
						}
						cols := 180
						if fault == "bad-restore-size" {
							cols--
						}
						ipcReply(conn, req, "runtime", map[string]any{"terminal": map[string]any{"handle": "terminal", "mode": "desktop-fit", "cols": cols, "rows": 40}})
					} else {
						t.Error("unknown mode")
					}
				default:
					t.Error("unexpected input or RPC", req.Method)
				}
			})
			p := b.ProcessIdentity{RuntimeID: "runtime", Terminal: "terminal", Incarnation: "incarnation", WorktreeID: "worktree"}
			lease, _, err := FitBoundComposer(context.Background(), "", p, nil)
			if fault == "none" || fault == "bad-restore-size" {
				if err != nil || lease == nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("invalid fit accepted", fault)
			}
			if lease != nil {
				_, restoreErr := lease.Restore(context.Background())
				if fault == "bad-restore-size" && restoreErr == nil {
					t.Fatal("bad restore receipt accepted")
				}
				if fault == "none" && restoreErr != nil {
					t.Fatal(restoreErr)
				}
				_, _ = lease.Restore(context.Background())
			}
			if fits > 1 || restores > 1 || fault == "none" && (fits != 1 || restores != 1) {
				t.Fatal("fit/restore replayed", fits, restores)
			}
			if (fault == "wrong-runtime" || fault == "wrong-terminal" || fault == "wrong-incarnation" || fault == "wrong-worktree" || fault == "foreign-fit") && fits != 0 {
				t.Fatal("changed identity resized")
			}
		})
	}
}
