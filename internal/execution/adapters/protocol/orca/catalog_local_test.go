//go:build windows

package orca

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLocalOrcaCatalog(t *testing.T) {
	for _, mode := range []string{"probe", "seed", "missing", "empty", "in-progress", "unsupported", "changed-runtime", "wrong-id", "wrong-protocol", "metadata-runtime", "invalid", "oversize", "disconnect", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			// RPCs have their own 3 s/5 s deadlines. Also bound fixture shutdown
			// so a stuck Close/Wait preserves all goroutine stacks in Full.
			watchdog := time.AfterFunc(20*time.Second, func() {
				stack := make([]byte, 1<<20)
				stack = stack[:runtime.Stack(stack, true)]
				os.Stderr.Write(stack)
				panic("local Orca catalog fixture did not finish: " + mode)
			})
			defer watchdog.Stop()
			data := t.TempDir()
			endpoint := `\\.\pipe\sl-web-catalog-` + uuid.NewString()
			listener, err := winio.ListenPipe(endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}
			var closeOnce sync.Once
			closeListener := func() { closeOnce.Do(func() { listener.Close() }) }
			defer closeListener()
			var mu sync.Mutex
			methods := []string{}
			var connections sync.WaitGroup
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					connections.Add(1)
					go func(conn net.Conn) {
						defer connections.Done()
						defer conn.Close()
						conn.SetDeadline(time.Now().Add(5 * time.Second))
						sc := bufio.NewScanner(conn)
						enc := json.NewEncoder(conn)
						for sc.Scan() {
							var q struct {
								ID, Method, AuthToken string
								Params                map[string]any
								ClientCapabilities    []string
							}
							if json.Unmarshal(sc.Bytes(), &q) != nil {
								return
							}
							if (q.AuthToken != "fixture-token" && q.AuthToken != "rotated-token") || len(q.ClientCapabilities) != 2 {
								return
							}
							mu.Lock()
							methods = append(methods, q.Method)
							mu.Unlock()
							if mode == "disconnect" {
								return
							}
							if mode == "cancel" {
								for sc.Scan() {
								}
								return
							}
							if mode == "invalid" {
								conn.Write([]byte("invalid\n"))
								return
							}
							if mode == "oversize" {
								conn.Write([]byte(strings.Repeat("x", (1<<20)+2) + "\n"))
								return
							}
							var result any
							switch q.Method {
							case "status.get":
								caps := []string{"agent-session.structured.v1", "agent-session.structured.registered-agents.v1"}
								if mode == "unsupported" {
									caps = nil
								}
								protocol := 3
								if mode == "wrong-protocol" {
									protocol = 2
								}
								result = map[string]any{"runtimeProtocolVersion": protocol, "minCompatibleRuntimeClientVersion": 2, "appVersion": "fixture", "capabilities": caps}
							case "agentSession.agents":
								agents := []any{map[string]string{"agent": "codex"}, map[string]string{"agent": "pi"}, map[string]string{"agent": "dsh"}, map[string]string{"agent": "claude"}}
								if mode == "missing" {
									agents = []any{}
								}
								result = map[string]any{"agents": agents}
							case "agentSession.modelCatalog":
								if q.Params["waitForListing"] != true || mode == "in-progress" {
									result = map[string]any{"listingInProgress": true}
								} else {
									modelID := q.Params["agent"]
									if modelID == "pi" {
										modelID = "openrouter/vendor/family/model"
									}
									models := []any{map[string]any{"id": modelID, "efforts": []any{map[string]string{"value": "high", "label": "High"}}}}
									if mode == "empty" {
										models = []any{}
									}
									result = map[string]any{"origin": mode, "models": models}
								}
							default:
								t.Error("business method requested", q.Method)
								return
							}
							runtimeID := "fixture-runtime"
							if mode == "metadata-runtime" || mode == "changed-runtime" && q.Method != "status.get" {
								runtimeID = "other"
							}
							id := q.ID
							if mode == "wrong-id" {
								id = "other"
							}
							if enc.Encode(map[string]any{"id": id, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtimeID}}) != nil {
								return
							}
						}
					}(conn)
				}
			}()
			writeMeta := func(token string) {
				t.Helper()
				raw, _ := json.Marshal(map[string]any{"runtimeId": "fixture-runtime", "authToken": token, "transports": []any{map[string]string{"kind": "named-pipe", "endpoint": endpoint}}})
				if err := os.WriteFile(filepath.Join(data, "orca-runtime.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeMeta("fixture-token")
			t.Setenv("ORCA_USER_DATA_PATH", data)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if mode == "cancel" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			out, err := QueryCatalog(ctx, "", "models", "pi")
			switch mode {
			case "cancel", "wrong-id", "changed-runtime", "metadata-runtime", "invalid", "oversize", "disconnect":
				if err == nil {
					t.Fatal("invalid native result accepted", out)
				}
			case "wrong-protocol", "missing", "unsupported":
				if !errors.Is(err, ErrCatalogUnsupported) {
					t.Fatal(out, err)
				}
			case "in-progress":
				if err != nil || !out.Listing {
					t.Fatal(out, err)
				}
			default:
				if err != nil || out.Listing {
					t.Fatal(out, err)
				}
				if mode != "empty" && (len(out.Models) != 1 || out.Models[0].ID != "openrouter/vendor/family/model" || out.Models[0].Provider != "openrouter") {
					t.Fatal(out)
				}
				writeMeta("rotated-token")
				rotated, e := QueryCatalog(ctx, "", "models", "dsh")
				if e != nil || mode != "empty" && (len(rotated.Models) != 1 || rotated.Models[0].ID != "dsh") {
					t.Fatal("rotated registration or Agent ignored", rotated, e)
				}
			}
			closeListener()
			<-done
			connections.Wait()
			mu.Lock()
			defer mu.Unlock()
			for _, method := range methods {
				if method != "status.get" && method != "agentSession.agents" && method != "agentSession.modelCatalog" {
					t.Fatal(method)
				}
			}
		})
	}
}
