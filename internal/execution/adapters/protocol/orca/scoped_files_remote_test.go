package orca

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/nacl/box"
)

func remoteWorkspaceFileFixture(t *testing.T, wrongRuntime bool, dials, files *atomic.Int32) (statePairing, func()) {
	t.Helper()
	pub, private, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dials.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		_, raw, err := conn.Read(ctx)
		var hello struct{ Type, PublicKeyB64 string }
		if err != nil || json.Unmarshal(raw, &hello) != nil || hello.Type != "e2ee_hello" {
			t.Error("missing file handshake")
			return
		}
		peerBytes, err := base64.StdEncoding.DecodeString(hello.PublicKeyB64)
		if err != nil || len(peerBytes) != 32 {
			t.Error("invalid peer key")
			return
		}
		var peer [32]byte
		copy(peer[:], peerBytes)
		socket := &stateSocket{conn: conn}
		box.Precompute(&socket.key, &peer, private)
		if conn.Write(ctx, websocket.MessageText, []byte(`{"type":"e2ee_ready"}`)) != nil {
			return
		}
		raw, err = socket.read(ctx)
		var auth struct {
			Type, DeviceToken  string
			ClientCapabilities []string
		}
		if err != nil || json.Unmarshal(raw, &auth) != nil || auth.Type != "e2ee_auth" || auth.DeviceToken != "file-token" || !slices.Equal(auth.ClientCapabilities, []string{"agent-session.session-boundary.v1"}) {
			t.Error("file authentication or capability changed")
			return
		}
		if socket.write(ctx, map[string]string{"type": "e2ee_authenticated"}) != nil {
			return
		}
		for {
			raw, err = socket.read(ctx)
			var request struct {
				ID, DeviceToken, Method string
				Params                  map[string]any
			}
			if err != nil {
				return
			}
			if json.Unmarshal(raw, &request) != nil || request.DeviceToken != "file-token" {
				t.Error("invalid file request")
				return
			}
			var result any
			switch request.Method {
			case "status.get":
				result = map[string]any{}
			case "worktree.show":
				if request.Params["worktree"] != "id:original" {
					t.Error("worktree selector changed")
					return
				}
				result = map[string]any{"worktree": Worktree{ID: "original", InstanceID: "instance", Path: "/original/workspace"}}
			case "files.readChunk", "files.writeBase64":
				files.Add(1)
				if request.Params["worktree"] != "id:original" || request.Params["relativePath"] != "ordinary/data.bin" {
					t.Error("file scope changed")
					return
				}
				if request.Method == "files.readChunk" {
					if request.Params["offset"] != float64(0) || request.Params["length"] != float64(5) {
						t.Error("read limit changed")
						return
					}
					result = map[string]any{"contentBase64": "AP8=", "bytesRead": 2, "eof": true}
				} else {
					if request.Params["expectedExecutionHostId"] != "local" || request.Params["contentBase64"] != "AP8=" {
						t.Error("initialization binding changed")
						return
					}
					result = map[string]bool{"written": true}
				}
			default:
				t.Errorf("unexpected file method %s", request.Method)
				return
			}
			runtime := "runtime"
			if wrongRuntime {
				runtime = "replacement"
			}
			if socket.write(ctx, map[string]any{"id": request.ID, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}}) != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return statePairing{Endpoint: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "file-token", PublicKey: base64.StdEncoding.EncodeToString(pub[:])}, server.Close
}

func TestWorkspaceFilesUseTheVerifiedPairingSnapshot(t *testing.T) {
	for _, scenario := range []string{"normal", "replaced-before-open", "replaced-after-resolve", "original-unavailable", "wrong-runtime", "wrong-instance"} {
		t.Run(scenario, func(t *testing.T) {
			var originalDials, originalFiles, replacementDials, replacementFiles atomic.Int32
			original, closeOriginal := remoteWorkspaceFileFixture(t, scenario == "wrong-runtime", &originalDials, &originalFiles)
			replacement, _ := remoteWorkspaceFileFixture(t, false, &replacementDials, &replacementFiles)
			dir := t.TempDir()
			t.Setenv("ORCA_USER_DATA_PATH", dir)
			save := func(pairing statePairing) {
				t.Helper()
				raw, _ := json.Marshal(map[string]any{"environments": []any{map[string]any{"id": "file-env", "preferredEndpointId": "endpoint", "endpoints": []any{map[string]string{"id": "endpoint", "endpoint": pairing.Endpoint, "deviceToken": pairing.Token, "publicKeyB64": pairing.PublicKey}}}}})
				if err := os.WriteFile(filepath.Join(dir, "orca-environments.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			save(original)
			host, err := UsageHostID("file-env")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "replaced-before-open" {
				save(replacement)
			}
			if scenario == "original-unavailable" {
				closeOriginal()
			}
			resolutions := 0
			resolve := func(environment string) (statePairing, error) {
				resolutions++
				pairing, err := resolveStatePairing(environment)
				if scenario == "replaced-after-resolve" || scenario == "original-unavailable" {
					save(replacement)
				}
				return pairing, err
			}
			binding := WorkspaceFileBinding{Environment: "file-env", RuntimeID: "runtime", ExecutionHostID: host, WorktreeID: "original", WorkspacePath: "/original/workspace", InstanceID: "instance"}
			if scenario == "wrong-instance" {
				binding.InstanceID = "other-instance"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			file, err := openWorkspaceFiles(ctx, binding, FileScope{Root: "ordinary", RelativePath: "ordinary/data.bin"}, FileLimits{ReadBytes: 4, WriteBytes: 4}, resolve)
			want := ""
			switch scenario {
			case "replaced-before-open", "wrong-instance":
				want = "binding_invalid"
			case "original-unavailable", "wrong-runtime":
				want = "transport_unavailable"
			}
			if want == "" {
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := file.Initialize(ctx, []byte{0, 255}); err != nil {
					t.Fatal(err)
				}
				if body, err := file.Read(ctx); err != nil || !bytes.Equal(body, []byte{0, 255}) {
					t.Fatal(body, err)
				}
				if originalDials.Load() != 1 || originalFiles.Load() != 2 {
					t.Fatal("original request count changed", originalDials.Load(), originalFiles.Load())
				}
			} else if err == nil || err.Error() != want || file != nil || originalFiles.Load() != 0 {
				t.Fatal("unverified file binding escaped", file, err, originalFiles.Load())
			}
			if resolutions != 1 || replacementDials.Load() != 0 || replacementFiles.Load() != 0 ||
				scenario == "replaced-before-open" && originalDials.Load() != 0 {
				t.Fatal("pairing was resolved again or replacement was contacted", resolutions, replacementDials.Load(), replacementFiles.Load(), originalDials.Load())
			}
		})
	}
}
