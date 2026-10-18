package orca

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Explicit live opt-in, no model. All mutations stay in one owned probe worktree.
func TestOrcaLocalIPCNativeProbe(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_IPC_PROBE_ROOT")
	if root == "" {
		t.Skip("requires explicit fresh native IPC probe evidence directory")
	}
	cli, repo, base := os.Getenv("STEERRUNNER_ORCA_IPC_PROBE_CLI"), os.Getenv("STEERRUNNER_ORCA_IPC_PROBE_REPO"), os.Getenv("STEERRUNNER_ORCA_IPC_PROBE_BASE")
	if !filepath.IsAbs(root) || !filepath.IsAbs(cli) || repo == "" || len(base) != 40 {
		t.Fatal("explicit native probe identity is incomplete")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ORCA_PAIRING_CODE", "ORCA_REMOTE_PAIRING", "ORCA_ENVIRONMENT"} {
		previous, exists := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	runtime, err := LocalRuntimeID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := QueryWorktreeAgents(ctx, "", runtime); err != nil {
		t.Fatal(err)
	}
	cleanup := false
	report := map[string]any{"runtime_id": runtime, "transport": "local-ipc", "pairing_present": false}
	t.Cleanup(func() {
		report["passed"] = !t.Failed() && cleanup
		report["resources_cleaned"] = cleanup
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "probe.json"), data, 0600); err != nil {
			t.Error(err)
		}
	})
	calls := 0
	call := func(args ...string) (Envelope, error) {
		callCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		out, err := exec.CommandContext(callCtx, cli, append(args, "--json")...).CombinedOutput()
		calls++
		_ = os.WriteFile(filepath.Join(root, fmt.Sprintf("cli-%02d.json", calls)), out, 0600)
		envelope, parseErr := ParseEnvelope(out)
		if err != nil || parseErr != nil {
			return envelope, fmt.Errorf("native probe CLI %v: %v %v", args, err, parseErr)
		}
		if envelope.Meta.RuntimeID != runtime {
			return envelope, stateIdentityError("probe CLI runtime changed")
		}
		return envelope, nil
	}
	created, err := call("worktree", "create", "--repo", "id:"+repo, "--name", "sr-ipc-"+filepath.Base(root), "--base-branch", base, "--no-parent", "--setup", "skip")
	if err != nil {
		t.Fatal(err)
	}
	var made struct{ Worktree Worktree }
	if json.Unmarshal(created.Result, &made) != nil || made.Worktree.ID == "" || made.Worktree.Path == "" || made.Worktree.Head != base {
		t.Fatal("native probe creation identity is incomplete")
	}
	report["worktree_id"] = made.Worktree.ID
	t.Cleanup(func() {
		if _, err := call("terminal", "close", "--worktree", "id:"+made.Worktree.ID, "--all"); err != nil {
			t.Error(err)
			return
		}
		if _, err := call("worktree", "rm", "--worktree", "id:"+made.Worktree.ID, "--force"); err != nil {
			t.Error(err)
			return
		}
		cleanup = true
	})
	initial, err := QueryState(ctx, "", runtime, "id:"+made.Worktree.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "initial-query.json"), initial, 0600)
	terminal, err := call("terminal", "create", "--worktree", "id:"+made.Worktree.ID, "--title", "SteerRunner IPC probe")
	if err != nil {
		t.Fatal(err)
	}
	var term struct{ Terminal Terminal }
	if json.Unmarshal(terminal.Result, &term) != nil || term.Terminal.Handle == "" {
		t.Fatal("probe terminal missing")
	}
	for {
		current, err := QueryState(ctx, "", runtime, "id:"+made.Worktree.ID)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(current, []byte(term.Terminal.Handle)) {
			_ = os.WriteFile(filepath.Join(root, "updated-query.json"), current, 0600)
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	// Each query opens/closes its own authenticated connection.
	reopened, err := QueryState(ctx, "", runtime, "id:"+made.Worktree.ID)
	if err != nil || !bytes.Contains(reopened, []byte(term.Terminal.Handle)) {
		t.Fatal("reopened query lost original terminal", err)
	}
	report["query_change_verified"] = true
	report["reopened_original_terminal"] = true
	report["observation_mode"] = "native-query"
	payload := bytes.Repeat([]byte(runtime+"\n"), 12000)
	if err := UploadWorkspaceArtifact(ctx, "", runtime, "id:"+made.Worktree.ID, ".sr-runtime-stage/ipc-probe.bin", bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(made.Worktree.Path, ".sr-runtime-stage", "ipc-probe.bin"))
	if err != nil || !bytes.Equal(payload, actual) {
		t.Fatal("native upload content differs", err)
	}
	sum := sha256.Sum256(actual)
	report["upload_sha256"] = hex.EncodeToString(sum[:])
	report["upload_bytes"] = len(actual)
}
