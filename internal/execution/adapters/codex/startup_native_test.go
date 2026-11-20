//go:build windows && live

package codexworker

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	_ "modernc.org/sqlite"
)

var startupEvidence = flag.String("startup-evidence", "", "new directory for the isolated no-auth startup probe")

func TestNativeCodexStartupIndexReuse(t *testing.T) {
	if *hostNativeExecutable == "" || !filepath.IsAbs(*startupEvidence) {
		t.Fatal("explicit native executable and absolute new startup-evidence directory required")
	}
	root := *startupEvidence
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	seeds := filepath.Join(home, "sessions", "2026", "11", "19")
	if err := os.MkdirAll(seeds, 0700); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "isolated no-auth startup fixture", http.StatusUnauthorized)
	}))
	defer provider.Close()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(fmt.Sprintf("openai_base_url = %q\n", provider.URL+"/v1")), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 128; i++ {
		id := fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i)
		meta := map[string]any{"timestamp": "2026-11-19T00:00:00Z", "type": "session_meta", "payload": map[string]any{
			"id": id, "timestamp": "2026-11-19T00:00:00Z", "cwd": root, "originator": "codex_cli_rs", "cli_version": "0.166.0", "source": "cli", "model_provider": "openai",
			"base_instructions": map[string]string{"text": "Isolated startup fixture."},
		}}
		if err := os.WriteFile(filepath.Join(seeds, "rollout-2026-11-19T00-00-00-"+id+".jsonl"), encode(meta), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", home)
	environment := func(base []string) []string {
		var out []string
		allowed := " PATH PATHEXT SYSTEMROOT SYSTEMDRIVE COMSPEC WINDIR TEMP TMP USERPROFILE APPDATA LOCALAPPDATA PROGRAMFILES PROGRAMFILES(X86) PROGRAMDATA "
		for _, entry := range base {
			name, _, _ := strings.Cut(entry, "=")
			if strings.Contains(allowed, " "+strings.ToUpper(name)+" ") {
				out = append(out, entry)
			}
		}
		return append(out, "CODEX_HOME="+home)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	host := process.New(30 * time.Second)
	type started struct {
		handle *SessionHandle
		stop   func()
		secs   float64
		err    error
	}
	start := func(i int) started {
		server := filepath.Join(root, fmt.Sprintf("server-%d", i))
		workspace := filepath.Join(root, fmt.Sprintf("work-%d", i))
		if err := os.Mkdir(workspace, 0700); err != nil {
			return started{err: err}
		}
		var stop func()
		backend := Backend{Config: Config{Executable: *hostNativeExecutable, Workspace: workspace, RunDir: root, RunID: fmt.Sprintf("%032x", i+1), ServerDir: server, Sandbox: "danger-full-access", Environment: environment},
			Process: ProcessPort{Start: func(cmd *exec.Cmd) (b.StopHandle, error) {
				owned, err := host.StartManaged(cmd)
				if err == nil {
					var once sync.Once
					stop = func() {
						once.Do(func() {
							if !settleRemoteTree(owned) {
								t.Error("probe native process tree exit unverified")
							}
							owned.Close()
						})
					}
					t.Cleanup(stop)
				}
				return owned, err
			}, Inspect: host.InspectProcess, NewGeneration: host.NewProcessGeneration, WriteAtomic: host.WriteExecutionFile}}
		begin := time.Now()
		handle, _, err := backend.PrepareSession(ctx, Binding{Workspace: workspace}, b.TurnInput{CommandID: fmt.Sprintf("%032x", i+10), Prompt: "Isolated startup fixture; no model execution."})
		if handle != nil {
			handle.owned = false // The probe's stop closure owns and verifies the Job.
			t.Cleanup(handle.Close)
		}
		return started{handle: handle, stop: stop, secs: time.Since(begin).Seconds(), err: err}
	}
	// Two cold app-servers are requested concurrently against one empty index.
	var cold [2]started
	var workers sync.WaitGroup
	for i := range cold {
		workers.Go(func() { cold[i] = start(i) })
	}
	workers.Wait()
	for _, item := range cold {
		if item.err != nil {
			t.Fatal(item.err)
		}
	}
	if cold[0].handle.Binding.SessionID == cold[1].handle.Binding.SessionID || cold[0].handle.Binding.Endpoint == cold[1].handle.Binding.Endpoint {
		t.Fatal("independent app-server identities were merged")
	}
	// This diagnostic-only test observes its own synthetic native DB. Production
	// startup/history code uses only the public Codex protocol and never opens it.
	readIndex := func() (int, int64) {
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(home, "state_5.sqlite"))+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var count int
		var completed int64
		var status string
		if err := db.QueryRow("SELECT count(*) FROM threads").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT status,last_success_at FROM backfill_state WHERE id=1").Scan(&status, &completed); err != nil || status != "complete" {
			t.Fatal("native backfill not complete", status, err)
		}
		return count, completed
	}
	seeded, firstBackfill := readIndex()
	if seeded != 128 {
		t.Fatal("synthetic history did not backfill exactly once", seeded)
	}
	anchor := cold[0].handle
	var account struct {
		Account json.RawMessage `json:"account"`
	}
	if err := anchor.client.Call(ctx, "account/read", map[string]any{"refreshToken": false}, &account); err != nil || string(account.Account) != "null" {
		t.Fatal("no-auth probe unexpectedly has native credentials", err)
	}
	params, err := TurnStartParams(anchor.Binding.Workspace, anchor.Binding.SessionID, strings.Repeat("f", 32), "No-auth history anchor", "danger-full-access", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A loopback-only rejecting provider supplies no inference but lets Codex
	// persist a real Session/Turn for the history preservation assertion.
	turn, err := StartTurn(ctx, anchor.client, params)
	if err != nil {
		t.Fatal(err)
	}
	terminal := false
	for !terminal {
		select {
		case note, ok := <-anchor.client.Notifications():
			if !ok {
				t.Fatal("native connection ended before the no-auth turn")
			}
			if note.Method == "turn/completed" {
				var value struct {
					Turn struct{ ID, Status string } `json:"turn"`
				}
				if json.Unmarshal(note.Params, &value) == nil && value.Turn.ID == turn.Turn.ID {
					if value.Turn.Status != "failed" {
						t.Fatal("no-auth turn unexpectedly executed", value.Turn.Status)
					}
					terminal = true
				}
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, item := range cold {
		item.handle.client.Close()
		item.stop()
	}
	warm := start(2)
	if warm.err != nil {
		t.Fatal(warm.err)
	}
	indexed, warmBackfill := readIndex()
	if indexed != 129 || warmBackfill != firstBackfill {
		t.Fatal("new server rebuilt or lost the existing native index", indexed, firstBackfill, warmBackfill)
	}
	reader, err := OpenHistory(ctx, b.Command{Executable: *hostNativeExecutable}, root, home, host.StartManaged)
	if err != nil {
		t.Fatal(err)
	}
	original, err := reader.ReadThread(ctx, anchor.Binding.SessionID, anchor.Binding.Workspace)
	reader.Close()
	if err != nil || len(original.Turns) != 1 || original.Turns[0].ID != turn.Turn.ID {
		t.Fatal("original session/turn lost after server restart", original, err)
	}
	if !reader.WritersAbsent() {
		t.Fatal("history reader tree exit unverified")
	}
	for i := range 3 {
		if _, err := os.Stat(filepath.Join(root, fmt.Sprintf("server-%d", i), "sqlite")); !os.IsNotExist(err) {
			t.Fatal("new server created a separate native history index", err)
		}
	}
	warm.handle.client.Close()
	warm.stop()
	result := map[string]any{"passed": true, "seeded_threads": 128, "indexed_threads": indexed, "backfill_completed_at_unchanged": warmBackfill == firstBackfill, "cold_start_seconds": []float64{cold[0].secs, cold[1].secs}, "warm_start_seconds": warm.secs, "distinct_sessions": true, "original_turn_preserved": true, "history_reader_stopped": true, "all_owned_processes_stopped": true, "authentication": "none", "provider": "loopback rejection fixture", "model_success": false}
	if err := os.WriteFile(filepath.Join(root, "result.json"), encode(result), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("native startup/history evidence: %s", root)
}
