//go:build windows

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func waitFixtureOwnerExit(t *testing.T, dir, id string) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "runs", id, "attempts", "*", "app-server", "owner", "owner-ready.json"))
	files = append(files, filepath.Join(dir, "runs", id, "app-server", "owner", "owner-ready.json"))
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		var ready struct{ Identity codex.ExecutionOwnerIdentity }
		if err != nil || json.Unmarshal(body, &ready) != nil || ready.Identity.Process.PID <= 0 {
			t.Errorf("fixture owner exit identity unavailable: %s", path)
			continue
		}
		until := time.Now().Add(5 * time.Second)
		for {
			probe := process.Probe(&ready.Identity.Process)
			// A reused PID proves the original owner is gone. Never wait for,
			// or stop, the unrelated process now holding that PID.
			if probe.State == "absent" || probe.State == "pid-reused" {
				break
			}
			if time.Now().After(until) {
				t.Errorf("explicitly stopped fixture owner remains unverified: %+v (%s: %s)", ready.Identity.Process, probe.State, probe.Reason)
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func testFixtureOwnerExitWithReusedPID(t *testing.T) {
	identity, err := process.NewServiceFacts().CurrentIdentity()
	if err != nil || process.Probe(identity).State != "matches" {
		t.Fatal("current test process identity unavailable", err)
	}
	created, err := strconv.ParseUint(identity.Created, 16, 64)
	if err != nil || created == 0 {
		t.Fatal("current process creation identity unavailable", err)
	}
	// Store an older incarnation of our own PID. The current test process
	// stays alive, so waiting for the PID itself would fail this regression.
	identity.Created = fmt.Sprintf("%016x", created-1)
	dir := t.TempDir()
	owner := filepath.Join(dir, "runs", "original", "app-server", "owner")
	if err := os.MkdirAll(owner, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"identity": codex.ExecutionOwnerIdentity{Process: *identity}})
	if err := os.WriteFile(filepath.Join(owner, "owner-ready.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	waitFixtureOwnerExit(t, dir, "original")
}

// A real second image with different package bytes/default resource digest.
// The JSON whitespace overlay changes no workflow, permission or result contract.
func buildCompatibleCandidate(t *testing.T) string {
	t.Helper()
	// go test starts in tests/integration; the independent foundation harness
	// starts in the repository root with explicit Runner/helper image paths.
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); os.IsNotExist(err) {
		root = filepath.Clean(filepath.Join(root, "..", ".."))
	} else if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	asset := filepath.Join(root, "internal", "resources", "assets", "prompts.json")
	body, err := os.ReadFile(asset)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(dir, "prompts.json")
	if err = os.WriteFile(replacement, append(body, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{asset: replacement}})
	path := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(path, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(dir, "candidate.exe")
	cmd := exec.Command("go", "build", "-trimpath", "-overlay", path, "-ldflags", "-X github.com/erranliu/SteerRunner/internal/resources.Version=takeover-compatible", "-o", image, "./cmd/steerrunner")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("candidate build: %v %s", err, output)
	}
	return image
}

func testGenerationCreationReceipt(t *testing.T) {
	dir, request := setupRun(t)
	hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-thread-response")
	if err := os.WriteFile(hold, []byte("native create succeeded; response withheld"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(hold) })
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	// Workspace preparation precedes this native creation fault barrier.
	until := time.Now().Add(5 * time.Minute)
	for {
		calls, _ := os.ReadFile(filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "calls.jsonl"))
		if strings.Contains(string(calls), `"method":"thread/start"`) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("native creation barrier not reached")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := killOwnedService(info); err != nil {
		t.Fatal(err)
	}
	reader, err := storage.OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reader.Runs()
	reader.Close()
	if err != nil || len(runs) != 1 || runs[0].Snapshot.Session.SessionID != "" {
		t.Fatalf("creation receipt was already committed: %v", err)
	}
	if err = os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	current := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, current)
	ready := waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.Status == "pr_ready" }, 5*time.Minute)
	if ready.ResumeRequired || countNative(t, "thread/start") != 1 || countNative(t, "turn/start") != 3 {
		t.Fatal("lost create receipt replaced original session or replayed input")
	}
}

func testGenerationPendingStop(t *testing.T) {
	dir, request := setupRun(t)
	// Keep the original turn active until the service is offline.
	hold := filepath.Join(os.Getenv("STEERRUNNER_FIXTURE_STATE"), "hold-turn")
	if err := os.WriteFile(hold, []byte("hold original execution"), 0600); err != nil {
		t.Fatal(err)
	}
	info := serviceInfo(t, call(t, dir, []string{"service", "start"}, "", 0))
	cleanupService(t, dir, info)
	raw, _ := json.Marshal(request)
	id := call(t, dir, []string{"submit", "--input", "-"}, string(raw), 0).Data.(map[string]any)["run_id"].(string)
	waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.Execution.TurnStarted }, 5*time.Minute)
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("offline-for-stop", info.InstanceID), 0)
	if err := waitOwnedService(info); err != nil {
		t.Fatal(err)
	}
	reader, err := storage.OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reader.Runs()
	reader.Close()
	if err != nil || len(runs) != 1 {
		t.Fatalf("original run: %v", err)
	}
	stop := contracts.Request{Protocol: contracts.Protocol, RequestID: "caller-original-stop", Input: json.RawMessage(fmt.Sprintf(`{"expected_revision":%d}`, runs[0].Revision))}
	input := contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: id, Request: stop}}}
	if err = os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	current := serviceInfo(t, call(t, dir, []string{"service", "start", "--input", "-"}, string(input.JSON()), 0))
	cleanupService(t, dir, current)
	waitRunFor(t, dir, id, func(r contracts.RunView) bool { return r.StopStatus == "stopped" }, 5*time.Minute)
	if countNative(t, "turn/start") != 1 {
		t.Fatal("successor overtook startup cancellation")
	}
	// The accepted receipt survives a lost startup reply and a further restart,
	// even though its original expected_revision is now stale.
	call(t, dir, []string{"service", "stop", "--input", "-"}, stopRequest("retry-startup-stop", current.InstanceID), 0)
	if err = waitOwnedService(current); err != nil {
		t.Fatal(err)
	}
	last := serviceInfo(t, call(t, dir, []string{"service", "start", "--input", "-"}, string(input.JSON()), 0))
	cleanupService(t, dir, last)
	call(t, dir, []string{"inspect", "--request-id", stop.RequestID}, "", 0)
	if countNative(t, "turn/start") != 1 {
		t.Fatal("accepted startup cancellation was replayed as new business input")
	}
}
