package codexworker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

type unreachableWorkspaceHost struct {
	executionbackend.ManagedProcessHost
}

func TestWorkspacePreparationWaitsForLFSSmudgeWithinOriginalBudget(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	runGit := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit(root, "init", "-q", repo)
	runGit(repo, "config", "filter.fixture-lfs.clean", "cat")
	runGit(repo, "config", "filter.fixture-lfs.required", "true")
	for name, contents := range map[string]string{
		".gitattributes": "*.bin filter=fixture-lfs -text\n",
		"asset.bin":      "fixture LFS pointer\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runGit(repo, "add", ".")
	runGit(repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	base := runGit(repo, "rev-parse", "HEAD")
	common := runGit(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")

	lifecycle := ExecutionLifecycle{setup: executionbackend.LifecycleSetup{
		ProcessHost: process.New(time.Second), GitExecutable: "git", RepositoryPath: repo,
		CommandTimeout: 2 * time.Second,
	}}
	for _, tc := range []struct {
		name, delay string
		status      executionbackend.OperationStatus
	}{
		{"timeout", "30", executionbackend.OperationRejected},
		{"complete", "3", executionbackend.OperationConfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Use a local LFS filter so the timeout reproduces an interrupted
			// checkout with a real Git index lock and no network dependency.
			runGit(repo, "config", "filter.fixture-lfs.smudge", "echo 'fixture LFS smudge started' >&2; sleep "+tc.delay+"; cat")
			intent := executionbackend.WorkspaceIntent{Path: filepath.Join(root, "worktrees", tc.name), Branch: "fixture/" + tc.name, BaseCommit: base}
			request := executionbackend.LifecycleRequest{
				Input: executionbackend.LifecycleInput{Workspace: &intent},
				Snapshot: executionbackend.LifecycleSnapshot{
					WorkspacePlan: intent,
					Session:       executionbackend.SessionContext{ProjectID: common},
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			receipt, err := lifecycle.prepareWorkspace(ctx, request, false)
			if receipt.Disposition != tc.status {
				t.Fatalf("checkout receipt = %+v; %v", receipt, err)
			}
			if tc.status == executionbackend.OperationRejected {
				if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "fixture LFS smudge started") || receipt.Workspace != nil {
					t.Fatalf("timed-out checkout lost its cause or claimed preparation: %+v; %v", receipt, err)
				}
				index := runGit(intent.Path, "rev-parse", "--path-format=absolute", "--git-path", "index")
				if _, err := os.Stat(index); !os.IsNotExist(err) {
					t.Fatalf("fixture should have no committed Git index: %v", err)
				}
				if _, err := os.Stat(index + ".lock"); err != nil {
					t.Fatalf("fixture should retain the interrupted Git index lock: %v", err)
				}
			} else if err != nil || receipt.Workspace == nil || receipt.Workspace.Head != base {
				t.Fatalf("checkout with sufficient preparation budget did not finish: %+v; %v", receipt, err)
			}

			// A lost receipt must be checked read-only, with no second checkout.
			recovered, err := lifecycle.prepareWorkspace(context.Background(), request, true)
			if recovered.Disposition != tc.status {
				t.Fatalf("recovered receipt = %+v; %v", recovered, err)
			}
			if tc.status == executionbackend.OperationRejected {
				if !errors.Is(err, workspace.ErrCheckoutIncomplete) || recovered.Workspace != nil {
					t.Fatalf("partial checkout became prepared: %+v; %v", recovered, err)
				}
			} else if err != nil || recovered.Workspace == nil || recovered.Workspace.Head != base || recovered.Workspace.Dirty {
				t.Fatalf("complete checkout could not be recovered: %+v; %v", recovered, err)
			}
		})
	}
}

func TestWorkspaceLifecyclePrecedesSessionAndRejectsRetargeting(t *testing.T) {
	lifecycle := &ExecutionLifecycle{setup: executionbackend.LifecycleSetup{ProcessHost: unreachableWorkspaceHost{}}}
	intent := executionbackend.WorkspaceIntent{Path: t.TempDir(), Branch: "codex/original", BaseCommit: "0123456789abcdef0123456789abcdef01234567"}
	snapshot := executionbackend.LifecycleSnapshot{WorkspacePlan: intent, PendingTurn: &executionbackend.TurnInput{CommandID: "0123456789abcdef0123456789abcdef", Prompt: "original input"}}
	candidate, err := reliable.Next(context.Background(), lifecycle, snapshot)
	if err != nil || candidate.Action != executionbackend.LifecyclePrepareWorkspace || candidate.Input.Workspace == nil || *candidate.Input.Workspace != intent {
		t.Fatalf("session setup preceded its durable workspace intent: %+v / %v", candidate, err)
	}
	changed := intent
	changed.Branch = "codex/other"
	request := executionbackend.LifecycleRequest{Snapshot: snapshot, Input: executionbackend.LifecycleInput{Workspace: &changed}}
	for _, reconcile := range []bool{false, true} {
		receipt, err := lifecycle.prepareWorkspace(context.Background(), request, reconcile)
		if err == nil || !strings.Contains(err.Error(), "original fixed intent") || receipt.Disposition != executionbackend.OperationRejected || receipt.Workspace != nil {
			t.Fatal("changed workspace intent reached a native Git operation")
		}
	}
	snapshot.Session.SessionID = "original-session"
	if _, err := reliable.Next(context.Background(), lifecycle, snapshot); err == nil {
		t.Fatal("a bound session planned a replacement for its missing original workspace")
	}
}
