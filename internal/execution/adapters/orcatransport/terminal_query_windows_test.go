package orcatransport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
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
	l, host, s := fixture(t)
	receipt, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, receipt)
	wrapped := &terminalQueryHost{ManagedProcessHost: host}
	setup := l.setup.Clone()
	setup.ProcessHost = wrapped
	port, err := NewLifecycle(setup) // exercise the production constructor
	if err != nil {
		t.Fatal(err)
	}
	terminal := native.Terminal{Handle: s.Ref.External.Terminal, RuntimeID: s.Ref.External.RuntimeID, IncarnationID: s.Ref.External.Incarnation, WorktreeID: s.Ref.External.WorktreeID, WorktreePath: s.Workspace.Path, Branch: s.Workspace.Branch, AgentIdentity: s.Ref.Agent.Name, Connected: true, Status: "running"}
	return port.(*Lifecycle), wrapped, s, terminal
}

func TestOriginalTerminalRPCIdentity(t *testing.T) {
	for _, scenario := range []string{"valid", "orphaned", "refs-prefix", "initializing", "initializing-wrong-branch", "managed-pi", "managed-dsh", "runtime", "request", "handle", "incarnation", "worktree", "workspace", "branch", "agent", "disconnected", "exited", "empty", "malformed", "rejected"} {
		t.Run(scenario, func(t *testing.T) {
			l, host, s, terminal := originalQueryFixture(t)
			valid := scenario == "valid" || scenario == "orphaned" || scenario == "refs-prefix" || scenario == "initializing" || strings.HasPrefix(scenario, "managed-")
			if strings.HasPrefix(scenario, "initializing") {
				l.setup.AgentLaunchCommand = "codex"
				terminal.Branch, terminal.AgentIdentity = "", ""
			}
			switch scenario {
			case "orphaned":
				terminal.Orphaned = true
			case "refs-prefix":
				terminal.Branch = "refs/heads/" + terminal.Branch
			case "initializing-wrong-branch":
				terminal.Branch = "foreign"
			case "managed-pi", "managed-dsh":
				l.profile.Terminal = &TerminalConfig{}
				s.Ref.Agent.Name = strings.TrimPrefix(scenario, "managed-")
				l.setup.Snapshot.Ref.Agent = s.Ref.Agent
				terminal.AgentIdentity = "gemini" // managed TUI label is not execution identity
			case "handle":
				terminal.Handle = "foreign"
			case "incarnation":
				terminal.IncarnationID = "foreign"
			case "worktree":
				terminal.WorktreeID = "foreign"
			case "workspace":
				terminal.WorktreePath = "foreign"
			case "branch":
				terminal.Branch = "foreign"
			case "agent":
				terminal.AgentIdentity = "foreign"
			case "disconnected":
				terminal.Connected = false
			case "exited":
				terminal.Status = "exited"
			}
			// Route by the original handle even when the response has been replaced.
			bound := terminal
			bound.Handle = s.Ref.External.Terminal
			terminalQueryIPC(t, bound, func(conn net.Conn, id, runtime string) {
				switch scenario {
				case "runtime":
					runtime = "foreign"
				case "request":
					id = "foreign"
				case "malformed":
					writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": "invalid"})
					return
				case "rejected":
					raw, _ := json.Marshal(map[string]any{"id": id, "ok": false, "error": map[string]string{"code": "unavailable", "message": "terminal unavailable"}, "_meta": map[string]string{"runtimeId": runtime}})
					_, _ = conn.Write(append(raw, '\n'))
					return
				}
				result := map[string]any{"terminal": terminal}
				if scenario == "empty" {
					result = map[string]any{}
				}
				writeTerminalQueryReply(conn, id, runtime, result)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := l.original(ctx, s)
			if (err == nil) != valid {
				t.Fatalf("valid=%t error=%v", valid, err)
			}
			if host.calls.Load() != 0 {
				t.Fatal("original RPC fell back to CLI", host.calls.Load())
			}
		})
	}
}

func TestOriginalTerminalRPCBypassesCLIQueue(t *testing.T) {
	l, host, s, terminal := originalQueryFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entered, release, gateDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var reads atomic.Int32
	terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
		reads.Add(1)
		writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": terminal})
	})
	go func() {
		_, err := native.SerializeCLI(ctx, func(ctx context.Context) ([]byte, error) {
			close(entered)
			select {
			case <-release:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		gateDone <- err
	}()
	defer func() {
		close(release)
		if err := <-gateDone; err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := l.original(ctx, s); results <- err }()
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if reads.Load() != 2 || host.calls.Load() != 0 {
		t.Fatal("reads waited for or entered CLI", reads.Load(), host.calls.Load())
	}
}

func TestOriginalTerminalRPCCancellation(t *testing.T) {
	for _, scenario := range []string{"already-canceled", "cancel-read", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			l, host, s, terminal := originalQueryFixture(t)
			entered := make(chan struct{}, 1)
			terminalQueryIPC(t, terminal, func(net.Conn, string, string) { entered <- struct{}{} })
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if scenario == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			if scenario == "already-canceled" {
				cancel()
			}
			result := make(chan error, 1)
			go func() { _, err := l.original(ctx, s); result <- err }()
			if scenario == "cancel-read" {
				select {
				case <-entered:
					cancel()
				case <-time.After(2 * time.Second):
					t.Fatal("read did not start")
				}
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("want %v, got %v", want, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not end the read")
			}
			if host.calls.Load() != 0 {
				t.Fatal("canceled query entered CLI")
			}
			if scenario == "already-canceled" && len(entered) != 0 {
				t.Fatal("canceled query reached server")
			}
		})
	}
}

func TestOriginalTerminalRPCChecksBothSidesOfObservation(t *testing.T) {
	l, host, feed, s, input := publicMessageFixture(t)
	l.profile.Environment = ""
	s.Ref.Target = l.profile.Target()
	l.setup.Snapshot.Ref.Target = s.Ref.Target
	l.nativeAccess = true
	terminal := native.Terminal{Handle: s.Ref.External.Terminal, RuntimeID: s.Ref.External.RuntimeID, IncarnationID: s.Ref.External.Incarnation, WorktreeID: s.Ref.External.WorktreeID, WorktreePath: s.Workspace.Path, Branch: s.Workspace.Branch, AgentIdentity: s.Ref.Agent.Name, Connected: true}
	var reads atomic.Int32
	terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
		current := terminal
		if reads.Add(1) == 2 {
			current.IncarnationID = "replacement"
		}
		writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": current})
	})
	feed.events = []native.AgentState{messageStart(input), messageProgress(input, 12, "must not publish")}
	before := host.nativeSends
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, message, err := l.observeOnce(ctx, s)
	if err == nil || message != nil || r.TurnEvidence != nil || reads.Load() != 2 || host.nativeSends != before {
		t.Fatal("replaced terminal published a result or replayed input", err, message, reads.Load(), host.nativeSends)
	}
}

func TestOriginalTerminalRPCStopAndRecoveryKeepOriginalIdentity(t *testing.T) {
	for _, operation := range []string{"stop", "recovery"} {
		for _, replaced := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/replaced=%t", operation, replaced), func(t *testing.T) {
				l, host, s, terminal := originalQueryFixture(t)
				state := fixtureState(10, "", "done")
				feed := &controlledStateFeed{initial: state, current: state}
				l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
				var reads atomic.Int32
				terminalQueryIPC(t, terminal, func(conn net.Conn, id, runtime string) {
					current := terminal
					n := reads.Add(1)
					if replaced && (operation == "stop" || n == 2) {
						current.IncarnationID = "replacement"
					}
					writeTerminalQueryReply(conn, id, runtime, map[string]any{"terminal": current})
				})
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				var err error
				wantReads := int32(1)
				if operation == "stop" {
					err = l.originalForStop(ctx, s)
				} else {
					wantReads = 2
					var evidence b.LifecycleRecoveryEvidence
					evidence, err = l.VerifyManualRecovery(ctx, s)
					if evidence.SessionIdle == replaced {
						t.Fatal("replacement authorized recovery or original idle evidence lost", evidence)
					}
				}
				if (err != nil) != replaced || reads.Load() != wantReads || host.calls.Load() != 0 {
					t.Fatal("stop/recovery lost original binding or caused CLI effects", err, reads.Load(), host.calls.Load())
				}
			})
		}
	}
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
