package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
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
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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

type terminalQueryHost struct {
	b.ManagedProcessHost
	delay time.Duration
	calls atomic.Int32
}

func (h *terminalQueryHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	h.calls.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(h.delay):
	}
	return h.ManagedProcessHost.OutputCommand(ctx, c)
}

func originalQueryFixture(t *testing.T) (*Lifecycle, *terminalQueryHost, b.LifecycleSnapshot, native.Terminal) {
	t.Helper()
	l, host, s := frozenFixture(t)
	s = frozenWorkspaceFixture(t, l, host, s)
	wrapped := &terminalQueryHost{ManagedProcessHost: host}
	setup := l.setup.Clone()
	setup.ProcessHost = wrapped
	port, err := capabilityFixtureLifecycle(setup) // read the original frozen terminal through native RPC
	if err != nil {
		t.Fatal(err)
	}
	terminal := native.Terminal{Handle: s.Ref.External.Terminal, RuntimeID: s.Ref.External.RuntimeID, IncarnationID: s.Ref.External.Incarnation, WorktreeID: s.Ref.External.WorktreeID, WorktreePath: s.Workspace.Path, Branch: s.Workspace.Branch, AgentIdentity: s.Ref.Agent.Name, Connected: true, Status: "running"}
	port.nativeAccess = true
	return port, wrapped, s, terminal
}

// Fixed work and equal 20ms service time on each transport. This measures the
// original-terminal read path, not model execution or end-to-end Run capacity.
// The same test is run on the baseline and modified production implementation.
func TestOriginalTerminalConcurrency(t *testing.T) {
	l, host, s, terminal := originalQueryFixture(t)
	host.delay = 20 * time.Millisecond
	var rpcCalls atomic.Int32
	terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
		rpcCalls.Add(1)
		time.Sleep(20 * time.Millisecond)
		writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": terminal})
	})
	for _, concurrency := range []int{1, 5, 10, 20} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			const rounds, reads = 3, 2
			beforeCLI, beforeRPC := host.calls.Load(), rpcCalls.Load()
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
							if _, err := l.original(ctx, s); err != nil {
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
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			p95 := samples[(len(samples)*95+99)/100-1]
			t.Logf("callers=%d reads=%d elapsed_ms=%.3f pair_p95_ms=%.3f reads_per_sec=%.2f cli_calls=%d rpc_calls=%d", concurrency, concurrency*rounds*reads, float64(elapsed)/float64(time.Millisecond), float64(p95)/float64(time.Millisecond), float64(concurrency*rounds*reads)/elapsed.Seconds(), host.calls.Load()-beforeCLI, rpcCalls.Load()-beforeRPC)
		})
	}
}
