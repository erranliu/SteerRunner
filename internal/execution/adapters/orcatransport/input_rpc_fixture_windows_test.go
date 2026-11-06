package orcatransport

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	winio "github.com/Microsoft/go-winio"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
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
