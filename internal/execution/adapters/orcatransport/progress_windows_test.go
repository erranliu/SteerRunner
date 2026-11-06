package orcatransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResultFileRPCUsesOriginalSelectorAndIndependentPath(t *testing.T) {
	l, f, s := frozenFixture(t)
	s = frozenWorkspaceFixture(t, l, f, s)
	setup := l.setup
	setup.Snapshot = s
	var calls atomic.Int32
	var crossed atomic.Bool
	var result atomic.Bool
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		calls.Add(1)
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		switch req.Method {
		case "status.get":
			ipcReply(conn, req, "runtime", map[string]any{})
		case "worktree.show":
			ipcReply(conn, req, "runtime", map[string]any{"worktree": f.workspace})
		case "files.writeBase64", "files.readChunk":
			want := ".steerrunner/progress/" + s.Session.WorkflowRunID + "/progress.bin"
			if result.Load() {
				want = ".steerrunner/result/" + s.Session.WorkflowRunID + "/result.bin"
			}
			if params["worktree"] != "id:"+s.Ref.External.WorktreeID || params["relativePath"] != want {
				crossed.Store(true)
			}
			if req.Method == "files.writeBase64" {
				if params["contentBase64"] != "AP8=" || params["expectedExecutionHostId"] != "local" {
					crossed.Store(true)
				}
				ipcReply(conn, req, "runtime", map[string]any{})
			} else {
				if params["length"] != float64(65537) {
					crossed.Store(true)
				}
				ipcReply(conn, req, "runtime", map[string]any{"contentBase64": "AP8=", "bytesRead": 2, "eof": true})
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, isResult := range []bool{false, true} {
		result.Store(isResult)
		open := ProgressSource
		if isResult {
			open = ResultSource
		}
		source, err := open(ctx, setup, "local", f.workspace["instanceId"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if err := source.Initialize(ctx, []byte{0, 255}); err != nil {
			t.Fatal(err)
		}
		data, err := source.Read(ctx)
		_ = source.Close()
		if err != nil || !bytes.Equal(data, []byte{0, 255}) || crossed.Load() {
			t.Fatal("original Run file scope changed", data, err, crossed.Load())
		}
	}
	before := calls.Load()
	for _, run := range []string{"../other-run", strings.Repeat("a", 31), strings.Repeat("a", 31) + "/"} {
		if _, err := openRunFile(ctx, "", "runtime", "local", "worktree", "C:/same-path", "instance", run, true); err == nil {
			t.Fatal("foreign Run accepted")
		}
	}
	if _, err := openRunFile(ctx, "", "runtime", "remote", "worktree", "C:/same-path", "instance", strings.Repeat("a", 32), true); err == nil || calls.Load() != before {
		t.Fatal("invalid Run/host connected", err)
	}
}

func TestRuntimeArtifactStagingScope(t *testing.T) {
	for _, relative := range []string{"../escape", ".sr-runtime-stage/../escape", ".sr-runtime-stage/a\\b", ".sr-runtime-stage-other/file", ".sr-runtime-stage/a..b", ".sr-runtime-stage/a\nb"} {
		if err := UploadWorkspaceArtifact(context.Background(), "", "runtime", "worktree", relative, strings.NewReader("bad")); err == nil || err.Error() != "runtime artifact must stay inside its staging directory" {
			t.Fatal("old staging rejection changed", relative, err)
		}
	}
}
