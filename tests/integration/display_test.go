//go:build windows

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/host"
	"github.com/erranliu/SteerRunner/internal/runview"
)

func TestSRDisplaySnapshotPublicService(t *testing.T) {
	testCount.Add(1)
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	dir, request := setupRun(t)
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	body, _ := json.Marshal(request)
	accepted := call(t, dir, []string{"submit", "--input", "-"}, string(body), 0)
	id := accepted.Data.(map[string]any)["run_id"].(string)
	current := waitRun(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" })
	normalized, err := host.Normalize(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, "SteerRunnerView", "snapshots", runview.Source(normalized)+".json")
	until := time.Now().Add(10 * time.Second)
	var snapshot runview.Snapshot
	for time.Now().Before(until) {
		raw, readErr := os.ReadFile(path)
		if readErr == nil && json.Unmarshal(raw, &snapshot) == nil && len(snapshot.Runs) == 1 && snapshot.Runs[0].Status == "pr_ready" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].ID != id || snapshot.Runs[0].Status != current.Status || snapshot.Runs[0].PR == nil || snapshot.Runs[0].PR.URL != current.PR.URL || snapshot.Instance != info.InstanceID {
		t.Fatal("public CLI Run missing from independently exported view", snapshot)
	}
	prior := snapshot.Sequence
	until = time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		raw, _ := os.ReadFile(path)
		_ = json.Unmarshal(raw, &snapshot)
		if snapshot.Sequence > prior {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if snapshot.Sequence <= prior {
		t.Fatal("no independent heartbeat after CLI clients exited")
	}
	if *evidenceFlag != "" {
		raw, _ := json.MarshalIndent(snapshot, "", "  ")
		_ = os.WriteFile(filepath.Join(*evidenceFlag, "display-snapshot.json"), raw, 0600)
	}
}

func TestSRDisplaySnapshotSourceIsolationAndRestart(t *testing.T) {
	testCount.Add(1)
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	dir := t.TempDir()
	first := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, first)
	alias := serviceInfo(t, call(t, strings.ToUpper(dir), []string{"service", "start"}, "", 0))
	if alias.InstanceID != first.InstanceID {
		t.Fatal("case alias created a second writer")
	}
	otherDir := t.TempDir()
	other := serviceInfo(t, call(t, otherDir, []string{"service", "start"}, "", 0))
	cleanupService(t, otherDir, other)
	read := func(dir, instance string) runview.Snapshot {
		t.Helper()
		normalized, err := host.Normalize(dir)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(local, "SteerRunnerView", "snapshots", runview.Source(normalized)+".json")
		until := time.Now().Add(10 * time.Second)
		for time.Now().Before(until) {
			body, err := os.ReadFile(path)
			var s runview.Snapshot
			if err == nil && json.Unmarshal(body, &s) == nil && s.Instance == instance && s.Sequence > 0 && s.Runs != nil {
				return s
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("no valid snapshot for original source and expected writer", dir, instance)
		return runview.Snapshot{}
	}
	a, b := read(dir, first.InstanceID), read(otherDir, other.InstanceID)
	if a.Source == b.Source || len(a.Runs) != 0 || len(b.Runs) != 0 {
		t.Fatal("distinct empty services did not publish independent complete sources", a, b)
	}
	next := restartEdgeService(t, dir, first, 1)
	c := read(dir, next.InstanceID)
	if c.Source != a.Source || c.Instance == a.Instance {
		t.Fatal("service replacement changed source identity or retained old writer", a, c)
	}
}
