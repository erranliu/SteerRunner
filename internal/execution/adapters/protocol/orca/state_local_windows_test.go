package orca

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

type ipcTestRequest struct {
	ID, AuthToken, Method string
	Params                json.RawMessage
}

func localIPCFixture(t *testing.T, serve func(net.Conn, ipcTestRequest)) {
	t.Helper()
	var id [12]byte
	_, _ = rand.Read(id[:])
	endpoint := `\\.\pipe\sr-orca-test-` + hex.EncodeToString(id[:])
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var connections []net.Conn
	var workers sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections = append(connections, conn)
			mu.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				scanner.Buffer(make([]byte, 4096), 4<<20)
				for scanner.Scan() {
					var request ipcTestRequest
					if json.Unmarshal(scanner.Bytes(), &request) != nil {
						return
					}
					if request.AuthToken != "local-test-secret" {
						data, _ := json.Marshal(map[string]any{"id": request.ID, "ok": false, "error": map[string]string{"code": "unauthorized", "message": "IPC authentication rejected"}, "_meta": map[string]string{"runtimeId": "runtime"}})
						_, _ = conn.Write(append(data, '\n'))
						return
					}
					serve(conn, request)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		mu.Lock()
		for _, conn := range connections {
			_ = conn.Close()
		}
		mu.Unlock()
		workers.Wait()
	})
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	// Neither a valid-looking pairing link nor an implicit environment may route IPC.
	t.Setenv("ORCA_PAIRING_CODE", "orca://pair?code=unrelated-secret")
	t.Setenv("ORCA_REMOTE_PAIRING", "unrelated-remote-secret")
	t.Setenv("ORCA_ENVIRONMENT", "other-host")
	data, _ := json.Marshal(map[string]any{"runtimeId": "runtime", "authToken": "local-test-secret", "transports": []any{map[string]string{"kind": "websocket", "endpoint": "ws://127.0.0.1:1"}, map[string]string{"kind": "named-pipe", "endpoint": endpoint}}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func ipcReply(conn net.Conn, req ipcTestRequest, runtime string, result any) {
	data, _ := json.Marshal(map[string]any{"id": req.ID, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
	_, _ = conn.Write(append(data, '\n'))
}

func TestOrcaLocalIPCQueryAndUpload(t *testing.T) {
	var mu sync.Mutex
	var uploaded []byte
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		switch req.Method {
		case "status.get":
			ipcReply(conn, req, "runtime", map[string]any{})
		case "session.tabs.list", "worktree.ps":
			_, _ = conn.Write([]byte("\n{\"_keepalive\":true}\n"))
			ipcReply(conn, req, "runtime", map[string]any{"queried": req.Method})
		case "files.writeBase64Chunk":
			var p struct {
				ContentBase64, RelativePath, ExpectedExecutionHostID string
				Append                                               bool
			}
			if json.Unmarshal(req.Params, &p) != nil || p.RelativePath != ".sr-runtime-stage/test.bin" || p.ExpectedExecutionHostID != "local" {
				return
			}
			chunk, err := base64.StdEncoding.DecodeString(p.ContentBase64)
			if err != nil {
				return
			}
			mu.Lock()
			if !p.Append {
				uploaded = nil
			}
			uploaded = append(uploaded, chunk...)
			mu.Unlock()
			ipcReply(conn, req, "runtime", map[string]any{"written": true})
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := QueryState(ctx, "", "runtime", "id:worktree"); err != nil {
		t.Fatal(err)
	}
	if _, err := QueryWorktreeAgents(ctx, "", "runtime"); err != nil {
		t.Fatal(err)
	}
	validPairing, _ := json.Marshal(map[string]any{"v": 2, "scope": "runtime", "endpoint": "ws://127.0.0.1:1", "deviceToken": "unrelated-pair-token", "publicKeyB64": base64.StdEncoding.EncodeToString(make([]byte, 32))})
	t.Setenv("ORCA_PAIRING_CODE", "orca://pair?code="+base64.RawURLEncoding.EncodeToString(validPairing))
	if _, err := QueryWorktreeAgents(ctx, "", "runtime"); err != nil {
		t.Fatal("valid remote pairing interfered with local IPC", err)
	}
	payload := bytes.Repeat([]byte("payload"), 100000)
	if err := UploadWorkspaceArtifact(ctx, "", "runtime", "id:worktree", ".sr-runtime-stage/test.bin", bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	equal := bytes.Equal(payload, uploaded)
	mu.Unlock()
	if !equal {
		t.Fatal("multi-chunk content changed")
	}
	if _, err := QueryWorktreeAgents(ctx, "missing-remote", "runtime"); err == nil {
		t.Fatal("remote fell back to local IPC")
	}
	if _, err := QueryWorktreeAgents(ctx, "", "foreign-runtime"); !errors.Is(err, ErrStateIdentityChanged) {
		t.Fatal("bootstrap identity accepted", err)
	}
	for _, path := range []string{"../escape", ".sr-runtime-stage/../escape", ".sr-runtime-stage/a\\b"} {
		if err := UploadWorkspaceArtifact(ctx, "", "runtime", "worktree", path, strings.NewReader("bad")); err == nil {
			t.Fatal("unsafe upload accepted", path)
		}
	}
}

func TestOrcaLocalIPCReadsOnlyBoundStopArtifact(t *testing.T) {
	const relative = ".sr-runtime-stage/native-stop/public-key/exit.json"
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		switch req.Method {
		case "status.get":
			ipcReply(conn, req, "runtime", map[string]any{})
		case "files.readChunk":
			var p struct {
				Worktree, RelativePath string
				Offset, Length         int
			}
			if json.Unmarshal(req.Params, &p) != nil || p.Worktree != "id:worktree" || p.RelativePath != relative || p.Offset != 0 || p.Length != 65536 {
				t.Error("wrong original resource read")
				return
			}
			ipcReply(conn, req, "runtime", map[string]any{"contentBase64": base64.StdEncoding.EncodeToString([]byte("proof")), "bytesRead": 5, "eof": true})
		default:
			t.Error("unexpected native effect", req.Method)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if raw, err := ReadStopArtifact(ctx, "", "runtime", "worktree", relative); err != nil || string(raw) != "proof" {
		t.Fatal(string(raw), err)
	}
	if _, err := ReadStopArtifact(ctx, "", "foreign-runtime", "worktree", relative); !errors.Is(err, ErrStateIdentityChanged) {
		t.Fatal("foreign runtime accepted", err)
	}
	for _, path := range []string{"../escape", ".sr-runtime-stage/native-stop/../exit.json", ".sr-runtime-stage/native-stop/key/request.json"} {
		if _, err := ReadStopArtifact(ctx, "", "runtime", "worktree", path); err == nil {
			t.Fatal("foreign path accepted", path)
		}
	}
}

func TestOrcaLocalIPCFrameFailuresAndCancellation(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "wrong-runtime", "wrong-request", "invalid", "truncated", "oversized", "cancelled", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
				if req.Method == "status.get" {
					ipcReply(conn, req, "runtime", map[string]any{})
					return
				}
				switch scenario {
				case "wrong-runtime":
					ipcReply(conn, req, "replacement", map[string]any{})
				case "wrong-request":
					req.ID = "foreign"
					ipcReply(conn, req, "runtime", map[string]any{})
				case "invalid":
					_, _ = conn.Write([]byte("{bad}\n"))
				case "truncated":
					_, _ = conn.Write([]byte(`{"id":`))
					_ = conn.Close()
				case "oversized":
					_, _ = conn.Write(bytes.Repeat([]byte("x"), (4<<20)+1))
					_ = conn.Close()
				case "cancelled": // Let the client's deadline interrupt the blocked read.
				case "legacy":
					ipcReply(conn, req, "runtime", map[string]any{"legacy": true})
				}
			})
			if scenario == "unauthorized" {
				dir, _ := stateUserDataDir()
				path := filepath.Join(dir, "orca-runtime.json")
				data, _ := os.ReadFile(path)
				var metadata map[string]any
				_ = json.Unmarshal(data, &metadata)
				metadata["authToken"] = "rejected-test-secret"
				data, _ = json.Marshal(metadata)
				_ = os.WriteFile(path, data, 0600)
			}
			if scenario == "legacy" {
				dir, _ := stateUserDataDir()
				path := filepath.Join(dir, "orca-runtime.json")
				data, _ := os.ReadFile(path)
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				m["transport"] = m["transports"].([]any)[1]
				delete(m, "transports")
				data, _ = json.Marshal(m)
				_ = os.WriteFile(path, data, 0600)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			_, err := QueryWorktreeAgents(ctx, "", "runtime")
			if scenario == "legacy" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid IPC response accepted")
			}
			if scenario == "unauthorized" {
				var rpc *StateRPCError
				if !errors.As(err, &rpc) || rpc.Code != "unauthorized" || strings.Contains(err.Error(), "test-secret") {
					t.Fatal("authentication denial was lost or exposed credentials", err)
				}
			}
			if strings.HasPrefix(scenario, "wrong-") && !errors.Is(err, ErrStateIdentityChanged) {
				t.Fatal("identity conflict became retryable", err)
			}
			if scenario == "cancelled" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			if (scenario == "truncated") != IsStateConnectionError(err) {
				t.Fatal("connection recovery classification changed", scenario, err)
			}
		})
	}
}

func TestOrcaLocalIPCConcurrentClose(t *testing.T) {
	reading := make(chan struct{})
	localIPCFixture(t, func(conn net.Conn, req ipcTestRequest) {
		if req.Method == "status.get" {
			ipcReply(conn, req, "runtime", map[string]any{})
		} else if req.Method == "worktree.ps" {
			close(reading)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := openStateConnection(ctx, "", "runtime")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := callState(ctx, conn, "worktree.ps", map[string]any{}); done <- err }()
	select {
	case <-reading:
	case <-ctx.Done():
		t.Fatal("query did not reach the pipe")
	}
	_ = conn.close()
	_ = conn.close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed query succeeded")
		}
	case <-ctx.Done():
		t.Fatal("close did not interrupt read")
	}
}
