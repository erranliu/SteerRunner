package orca

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Real local IPC to a private fixture, not a real Orca remote qualification.
func TestResultFileOriginalIPCBoundaries(t *testing.T) {
	for _, scenario := range []string{"good", "wrong_host", "wrong_runtime", "wrong_worktree", "wrong_instance", "wrong_path", "disconnect_show", "disconnect_read", "blocked_read", "oversize"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			run := strings.Repeat("a", 32)
			relative := ".steerrunner/result/" + run + "/result.bin"
			localPath := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(localPath, []byte("local path must never substitute RPC"), 0600); err != nil {
				t.Fatal(err)
			}
			var reads atomic.Int32
			var crossed atomic.Bool
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				switch req.Method {
				case "status.get":
					ipcReply(conn, req, "runtime", map[string]any{})
				case "worktree.show":
					if scenario == "disconnect_show" {
						_ = conn.Close()
						return
					}
					id, instance, path := "original-worktree", "original-instance", root
					if scenario == "wrong_worktree" {
						id = "foreign-worktree"
					}
					if scenario == "wrong_instance" {
						instance = "replacement-instance"
					}
					if scenario == "wrong_path" {
						path = root + "-foreign"
					}
					var params map[string]any
					_ = json.Unmarshal(req.Params, &params)
					if params["worktree"] != "id:original-worktree" {
						crossed.Store(true)
					}
					ipcReply(conn, req, "runtime", map[string]any{"worktree": map[string]any{"id": id, "instanceId": instance, "path": path}})
				case "files.readChunk":
					reads.Add(1)
					var params map[string]any
					_ = json.Unmarshal(req.Params, &params)
					requested, _ := params["relativePath"].(string)
					if params["worktree"] != "id:original-worktree" || requested != relative && requested != ".steerrunner/result/"+strings.Repeat("b", 32)+"/result.bin" {
						crossed.Store(true)
					}
					if scenario == "disconnect_read" {
						_ = conn.Close()
						return
					}
					if scenario == "blocked_read" {
						return
					}
					data := []byte{0, 255, 'a'}
					if strings.Contains(requested, strings.Repeat("b", 32)) {
						data = []byte{0, 254, 'b'}
					}
					if scenario == "oversize" {
						data = make([]byte, 65537)
					}
					ipcReply(conn, req, "runtime", map[string]any{"contentBase64": base64.StdEncoding.EncodeToString(data), "bytesRead": len(data), "eof": true})
				default:
					crossed.Store(true)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			host, runtime := "local", "runtime"
			if scenario == "wrong_host" {
				host = "foreign-host"
			}
			if scenario == "wrong_runtime" {
				runtime = "foreign-runtime"
			}
			started := time.Now()
			source, err := OpenResultFiles(ctx, "", runtime, host, "original-worktree", root, "original-instance", run)
			expected := ""
			switch scenario {
			case "wrong_host", "wrong_worktree", "wrong_instance", "wrong_path":
				expected = "binding_invalid"
			case "wrong_runtime", "disconnect_show":
				expected = "transport_unavailable"
			}
			if expected != "" {
				if err == nil || err.Error() != expected || reads.Load() != 0 {
					t.Fatal("original binding bypassed", scenario, err, reads.Load())
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer source.Close()
				readCtx, stopRead := context.WithTimeout(ctx, 50*time.Millisecond)
				data, readErr := source.Read(readCtx)
				stopRead()
				expectedRead := ""
				if scenario == "disconnect_read" || scenario == "blocked_read" {
					expectedRead = "read_failed"
				}
				if scenario == "oversize" {
					expectedRead = "size_exceeded"
				}
				if expectedRead != "" {
					if readErr == nil || readErr.Error() != expectedRead {
						t.Fatal(readErr)
					}
				} else {
					if readErr != nil || !bytes.Equal(data, []byte{0, 255, 'a'}) {
						t.Fatal("local substitution", data, readErr)
					}
					other, err := OpenResultFiles(ctx, "", "runtime", "local", "original-worktree", root, "original-instance", strings.Repeat("b", 32))
					if err != nil {
						t.Fatal(err)
					}
					defer other.Close()
					data, err = other.Read(ctx)
					if err != nil || !bytes.Equal(data, []byte{0, 254, 'b'}) {
						t.Fatal("cross-Run read", data, err)
					}
				}
			}
			if time.Since(started) > time.Second+200*time.Millisecond || crossed.Load() {
				t.Fatal("unbounded or crossed RPC")
			}
			local, _ := os.ReadFile(localPath)
			if string(local) != "local path must never substitute RPC" {
				t.Fatal("touched local substitute")
			}
		})
	}
}
