package orca

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrcaScopedFileCapability(t *testing.T) {
	var calls atomic.Int32
	var crossed atomic.Bool
	var mu sync.Mutex
	var uploaded []byte
	chunks := 0
	root := t.TempDir()
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		calls.Add(1)
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		switch req.Method {
		case "status.get":
			ipcReply(conn, req, "runtime", map[string]any{})
		case "worktree.show":
			if params["worktree"] != "id:original" {
				crossed.Store(true)
			}
			ipcReply(conn, req, "runtime", map[string]any{"worktree": Worktree{ID: "original", InstanceID: "instance", Path: root}})
		case "files.readChunk", "files.writeBase64", "files.writeBase64Chunk":
			if params["worktree"] != "id:original" || params["relativePath"] != "ordinary/data.bin" {
				crossed.Store(true)
			}
			if req.Method == "files.readChunk" {
				if params["offset"] != float64(0) || params["length"] != float64(5) {
					crossed.Store(true)
				}
				ipcReply(conn, req, "runtime", map[string]any{"contentBase64": "AP8=", "bytesRead": 2, "eof": true})
				return
			}
			if params["expectedExecutionHostId"] != "local" {
				crossed.Store(true)
			}
			if req.Method == "files.writeBase64Chunk" {
				mu.Lock()
				data, err := base64.StdEncoding.DecodeString(params["contentBase64"].(string))
				if err != nil || len(data) > 2 || params["append"] != (chunks > 0) {
					crossed.Store(true)
				}
				chunks++
				uploaded = append(uploaded, data...)
				mu.Unlock()
			} else if params["contentBase64"] != "AP8=" {
				crossed.Store(true)
			}
			ipcReply(conn, req, "runtime", map[string]any{"written": true})
		default:
			crossed.Store(true)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	binding := WorkspaceFileBinding{RuntimeID: "runtime", ExecutionHostID: "local", WorktreeID: "original", WorkspacePath: root, InstanceID: "instance"}
	scope := FileScope{Root: "ordinary", RelativePath: "ordinary/data.bin"}
	limits := FileLimits{ReadBytes: 4, WriteBytes: 4}
	file, err := OpenWorkspaceFiles(ctx, binding, scope, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Instance != "instance" || file.Initialize(ctx, []byte{0, 255}) != nil {
		t.Fatal("ordinary scoped file unavailable")
	}
	if body, err := file.Read(ctx); err != nil || !bytes.Equal(body, []byte{0, 255}) {
		t.Fatal(body, err)
	}
	before := calls.Load()
	if err := file.Initialize(ctx, make([]byte, 5)); err == nil || err.Error() != "size_exceeded" || calls.Load() != before {
		t.Fatal("write limit dispatched", err)
	}
	if err := UploadScopedFile(ctx, "", "runtime", "id:original", scope, 2, bytes.NewReader([]byte{0, 1, 2, 3, 4})); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	equal := bytes.Equal(uploaded, []byte{0, 1, 2, 3, 4}) && chunks == 3
	mu.Unlock()
	if !equal || crossed.Load() {
		t.Fatal("file scope/chunk/host parameters changed", equal, crossed.Load())
	}
	for _, relative := range []string{"../escape", "/ordinary/file", "ordinary/../escape", "ordinary/a/../../escape", "ordinary-other/file", "ordinary/a\\b", "ordinary/a:b", "ordinary/a\x00b", "ordinary/a\nb", "ordinary//file", "ordinary", "ordinary/.. /escape", "ordinary/a./file", "ordinary/a\tb"} {
		t.Run(relative, func(t *testing.T) {
			badScope := FileScope{Root: "ordinary", RelativePath: relative}
			before := calls.Load()
			if _, err := OpenWorkspaceFiles(ctx, binding, badScope, limits); err == nil || err.Error() != "binding_invalid" {
				t.Fatal("scope admitted", err)
			}
			if err := UploadScopedFile(ctx, "", "runtime", "id:original", badScope, 2, bytes.NewReader(nil)); err == nil || calls.Load() != before {
				t.Fatal("invalid scope connected", err, calls.Load(), before)
			}
		})
	}
	for _, readBytes := range []int{0, -1, int(^uint(0) >> 1)} {
		before := calls.Load()
		if _, err := OpenWorkspaceFiles(ctx, binding, scope, FileLimits{ReadBytes: readBytes, WriteBytes: 4}); err == nil || calls.Load() != before {
			t.Fatal("invalid/overflowing read budget connected", readBytes, err)
		}
	}
}
