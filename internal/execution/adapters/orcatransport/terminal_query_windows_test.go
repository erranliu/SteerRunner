package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	winio "github.com/Microsoft/go-winio"

	"fmt"
	"net"
	"os"
	"path/filepath"

	"sort"

	"sync"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

// This fixture serves concurrent short-lived IPC connections, including the
// normal status handshake. It never connects to a user Orca instance.
func terminalQueryIPC(t *testing.T, terminal native.Terminal, serve func(net.Conn, string, string)) {
	t.Helper()
	endpoint := fmt.Sprintf(`\\.\pipe\sr-terminal-query-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var connections []net.Conn
	var workers sync.WaitGroup
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			select {
			case <-stop:
				mu.Unlock()
				_ = conn.Close()
				return
			default:
			}
			connections = append(connections, conn)
			workers.Add(1)
			mu.Unlock()
			go func() {
				defer workers.Done()
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var req struct {
						ID, Method, AuthToken string
						Params                map[string]string
					}
					if json.Unmarshal(scanner.Bytes(), &req) != nil || req.AuthToken != "test-token" {
						return
					}
					if req.Method == "status.get" {
						writeTerminalQueryReply(conn, req.ID, terminal.RuntimeID, map[string]any{})
					} else if req.Method == "terminal.show" && req.Params["terminal"] == terminal.Handle {
						serve(conn, req.ID, terminal.RuntimeID)
					} else {
						t.Errorf("unexpected RPC: %s %v", req.Method, req.Params)
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		defer func() {
			mu.Lock()
			active := append([]net.Conn(nil), connections...)
			mu.Unlock()
			for _, conn := range active {
				_ = conn.Close()
			}
			workers.Wait()
		}()
		// Match the thin-query fixture: stop Accept before listener.Close.
		// go-winio can otherwise consume Close without stopping its listener.
		close(stop)
		select {
		case <-done:
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			wake, err := winio.DialPipeContext(ctx, endpoint)
			if err != nil {
				t.Errorf("wake test RPC accept: %v", err)
				return
			}
			// Keep it connected so Accept does not retry ERROR_NO_DATA.
			defer wake.Close()
			select {
			case <-done:
			case <-ctx.Done():
				t.Error("test RPC accept did not stop", ctx.Err())
				return
			}
		}
		_ = listener.Close()
		<-done
	})
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	t.Setenv("LOCALAPPDATA", dir) // isolate the real CLI mutex from the user's cache
	raw, _ := json.Marshal(map[string]any{"runtimeId": terminal.RuntimeID, "authToken": "test-token", "transport": map[string]string{"kind": "named-pipe", "endpoint": endpoint}})
	if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func writeTerminalQueryReply(conn net.Conn, id, runtime string, result any) {
	raw, _ := json.Marshal(map[string]any{"id": id, "ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
	_, _ = conn.Write(append(raw, '\n'))
}

func originalQueryTerminal() native.Terminal {
	return native.Terminal{Handle: "terminal", RuntimeID: "runtime", IncarnationID: "original-incarnation", WorktreeID: "worktree", WorktreePath: "C:/original", Branch: "worker", AgentIdentity: "codex", Connected: true, Status: "running"}
}

// Fixed work and 20ms RPC service time. This measures the direct terminal
// capability, not the retired Lifecycle wrapper or end-to-end Run capacity.
func TestOriginalTerminalConcurrency(t *testing.T) {
	terminal := originalQueryTerminal()
	identity := native.TerminalIdentity{RuntimeID: terminal.RuntimeID, Handle: terminal.Handle, IncarnationID: terminal.IncarnationID}
	var rpcCalls atomic.Int32
	terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
		rpcCalls.Add(1)
		time.Sleep(20 * time.Millisecond)
		writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": terminal})
	})
	for _, concurrency := range []int{1, 5, 10, 20} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			const rounds, reads = 3, 2
			beforeRPC := rpcCalls.Load()
			start := make(chan struct{})
			durations := make(chan time.Duration, concurrency*rounds)
			errs := make(chan error, concurrency)
			var wg sync.WaitGroup
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					for j := 0; j < rounds; j++ {
						began := time.Now()
						for k := 0; k < reads; k++ {
							if _, err := native.QueryBoundTerminal(ctx, "", identity); err != nil {
								errs <- err
								return
							}
						}
						durations <- time.Since(began)
					}
				}()
			}
			began := time.Now()
			close(start)
			wg.Wait()
			elapsed := time.Since(began)
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			close(durations)
			var samples []time.Duration
			for d := range durations {
				samples = append(samples, d)
			}
			if len(samples) != concurrency*rounds {
				t.Fatalf("incomplete: %d", len(samples))
			}
			if got := rpcCalls.Load() - beforeRPC; got != int32(concurrency*rounds*reads) {
				t.Fatalf("incomplete RPC work: got %d want %d", got, concurrency*rounds*reads)
			}
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			p95 := samples[(len(samples)*95+99)/100-1]
			t.Logf("scope=direct-terminal-capability callers=%d reads=%d elapsed_ms=%.3f pair_p95_ms=%.3f reads_per_sec=%.2f rpc_calls=%d", concurrency, concurrency*rounds*reads, float64(elapsed)/float64(time.Millisecond), float64(p95)/float64(time.Millisecond), float64(concurrency*rounds*reads)/elapsed.Seconds(), rpcCalls.Load()-beforeRPC)
		})
	}
}

func TestOriginalTerminalQueryIdentity(t *testing.T) {
	for _, scenario := range []string{"original", "foreign-runtime", "foreign-handle", "foreign-incarnation"} {
		t.Run(scenario, func(t *testing.T) {
			terminal := originalQueryTerminal()
			var calls atomic.Int32
			terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
				calls.Add(1)
				value := terminal
				switch scenario {
				case "foreign-runtime":
					runtime = "replacement-runtime"
				case "foreign-handle":
					value.Handle = "replacement-terminal"
				case "foreign-incarnation":
					value.IncarnationID = "replacement-incarnation"
				}
				writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": value})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			value, err := native.QueryBoundTerminal(ctx, "", native.TerminalIdentity{RuntimeID: terminal.RuntimeID, Handle: terminal.Handle, IncarnationID: terminal.IncarnationID})
			if scenario == "original" {
				if err != nil || value.RuntimeID != terminal.RuntimeID || value.Handle != terminal.Handle || value.IncarnationID != terminal.IncarnationID {
					t.Fatal("original terminal was not returned", value, err)
				}
			} else if !errors.Is(err, native.ErrStateIdentityChanged) {
				t.Fatal("contrary identity was not rejected", value, err)
			}
			if calls.Load() != 1 {
				t.Fatal("query did not read exactly one original RPC response", calls.Load())
			}
		})
	}
}
