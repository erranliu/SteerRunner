package application

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestFirstInputReusesOnlyFreshBoundGitPaths(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fresh, bound bool
	}{
		{name: "fresh and bound", fresh: true, bound: true},
		{name: "recovered with original binding", bound: true},
		{name: "fresh without binding", fresh: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, w, _ := operationEngine(t)
			r := e.runs["run"].Clone()
			r.Request.Execution.Adapter = "codex-app-server"
			state, _ := r.Workflow.CurrentState()
			r.PromptBodies = map[string]string{state.PromptKey: state.Prompt}
			// No Git repository is present. Any second Git resolution must fail.
			r.Snapshot.Workspace.Path = t.TempDir()
			root := t.TempDir()
			r.LinkedGitPaths = []string{
				filepath.Join(root, "objects"), filepath.Join(root, "refs"),
				filepath.Join(root, "logs"), filepath.Join(root, "worktrees", "run"),
			}
			if tc.bound {
				// prepareRun starts after advance has validated the envelope.
				s := r.Snapshot
				identity := codex.PolicyResourceIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}
				permissions, err := codex.NativePermissions("windows", "local", "codex", "full_access")
				if err != nil {
					t.Fatal(err)
				}
				binding, err := codex.NewPolicyResourceEnvelope(identity, codex.PolicyResourcePayload{
					NativeVersion: codex.PolicyCodexVersion, Contract: permissions.Contract,
					Settings: b.LaunchCommand{Executable: "fixture.exe", Args: permissions.Args()},
				})
				if err != nil {
					t.Fatal(err)
				}
				r.Snapshot.PolicyResources = &binding
			}
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			w.freshWorkspace = tc.fresh
			advanced := e.prepareRun(r.ID, w, w.port, r.Clone())
			saved := e.runs[r.ID]
			if tc.fresh && tc.bound {
				if !advanced || saved.Snapshot.PendingTurn == nil ||
					!slices.Equal(saved.Snapshot.PendingTurn.LinkedWorktreePaths, r.LinkedGitPaths) {
					t.Fatalf("the first input did not reuse its frozen write roots: %s / %s", saved.Phase, saved.Diagnostic)
				}
			} else if advanced || saved.Snapshot.PendingTurn != nil || saved.Phase != "workspace_unverified" {
				t.Fatalf("an existing or unbound workspace bypassed path resolution: %s / %s", saved.Phase, saved.Diagnostic)
			}
		})
	}
}

func TestContentFirstInputUsesReadOnlyWorkspace(t *testing.T) {
	for _, workflowID := range []string{"generate", "conversation"} {
		t.Run(workflowID, func(t *testing.T) {
			e, w := contentEngine(t, workflowID)
			r := e.runs["run"].Clone()
			s := &r.Snapshot
			identity := codex.PolicyResourceIdentity{ProjectID: s.Session.ProjectID, ExecutionID: s.Session.ExecutionID, AttemptID: s.Session.AttemptID, Generation: s.ResourceGeneration, ConfigDigest: s.ConfigurationDigest}
			permissions, err := codex.NativePermissions("windows", "local", "codex", "read_only")
			if err != nil {
				t.Fatal(err)
			}
			binding, err := codex.NewPolicyResourceEnvelope(identity, codex.PolicyResourcePayload{
				NativeVersion: codex.PolicyCodexVersion, Contract: permissions.Contract,
				Settings: b.LaunchCommand{Executable: "fixture.exe", Args: permissions.Args(), WorkingDirectory: s.Workspace.Path},
			})
			if err != nil {
				t.Fatal(err)
			}
			s.PolicyResources = &binding
			if err := e.commitLocked(r, nil, "permission_resources_bound"); err != nil {
				t.Fatal(err)
			}
			p := &fileBusinessPort{}
			w.port = p
			e.advance(r.ID, w)
			got := e.runs[r.ID]
			if w.startupWorkspaceChecked || p.executes != 1 || p.requests[0].Operation.Action != string(b.LifecycleStartTurn) {
				t.Fatalf("first content turn did not dispatch through its read-only workspace: phase=%q diagnostic=%q sends=%d", got.Phase, got.Diagnostic, p.executes)
			}
		})
	}
}

func TestRecoveredWorkspaceChecksOriginalGitBeforeFirstInput(t *testing.T) {
	for _, tc := range []struct {
		name, diagnostic string
		change           func(*baseRefreshFixture, string)
	}{
		{
			name: "clean_diff_with_wrong_HEAD", diagnostic: "worktree HEAD differs from pinned base",
			change: func(f *baseRefreshFixture, path string) {
				f.git(path, "-c", "user.name=Runner Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=", "commit", "--allow-empty", "-m", "move HEAD only")
				if f.git(path, "status", "--porcelain=v1") != "" {
					f.t.Fatal("fixture must move HEAD without changing the diff")
				}
			},
		},
		{
			name: "tracked_diff_at_pinned_HEAD", diagnostic: "original worktree changed before first input",
			change: func(f *baseRefreshFixture, path string) {
				if err := os.WriteFile(filepath.Join(path, "spec.md"), []byte("changed"), 0600); err != nil {
					f.t.Fatal(err)
				}
			},
		},
		{
			name: "untracked_file_at_pinned_HEAD", diagnostic: "original worktree changed before first input",
			change: func(f *baseRefreshFixture, path string) {
				if err := os.WriteFile(filepath.Join(path, "untracked-before-restart"), []byte("changed"), 0600); err != nil {
					f.t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkRecoveredWorkspaceBeforeFirstInput(t, tc.change, tc.diagnostic)
		})
	}
}

func checkRecoveredWorkspaceBeforeFirstInput(t *testing.T, change func(*baseRefreshFixture, string), diagnostic string) {
	t.Helper()
	f := newBaseRefreshFixture(t, "origin", "main")
	r := f.freeze()
	ctx := context.Background()
	r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
	intent := &r.Snapshot.Workspace
	repo := workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}
	prepared, err := workspace.Prepare(ctx, gitHost(), repo, filepath.Dir(intent.Path), intent)
	if err != nil || prepared.Inspection == nil {
		t.Fatal("fixture worktree creation failed", err)
	}
	r.WorkspaceFact = prepared.Inspection
	r.Allowed = true
	store, err := storage.Open(f.state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	first, err := New(ctx, f.state, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.cancel()
	if err := first.commitLocked(r, nil, "workspace_prepared"); err != nil {
		t.Fatal(err)
	}
	change(f, intent.Path)
	restarted, err := New(ctx, f.state, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.cancel()
	w := restarted.workers[r.ID]
	port := &operationCommitPort{}
	w.port = port
	if restarted.advance(r.ID, w) || port.executes != 0 {
		t.Fatal("recovery dispatched from an existing changed worktree")
	}
	if got := restarted.runs[r.ID]; got.Phase != "workspace_unverified" || got.Diagnostic != diagnostic {
		t.Fatalf("recovery did not check the original Git state: %s / %s", got.Phase, got.Diagnostic)
	}
}

func TestPrivateRunGitGrantDoesNotExpandLegacyCache(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	r := f.freeze()
	e := &Engine{dir: f.state, ctx: context.Background()}
	prepare := func() {
		t.Helper()
		r.Snapshot.Workspace = r.Snapshot.WorkspacePlan
		_, err := workspace.Prepare(e.ctx, gitHost(), workspace.Repository{Path: r.RepositoryPath, CommonDir: r.RepositoryCommon}, filepath.Dir(r.Snapshot.Workspace.Path), &r.Snapshot.Workspace)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.attachWriteRoots(r, false); err != nil {
			t.Fatal(err)
		}
	}
	prepare()
	if len(r.LinkedGitPaths) != 5 || r.LinkedGitPaths[4] != r.RepositoryCommon {
		t.Fatal("new private repository lacks root-level Git lock access")
	}
	// Simulate the original storage layout without migrating its permissions.
	r.RepositoryPath = repositoryCache(f.state, f.request.Repository)
	r.RepositoryCommon = filepath.Join(r.RepositoryPath, ".git")
	r.Snapshot.WorkspacePlan.Path += "-legacy"
	r.Snapshot.WorkspacePlan.Branch += "-legacy"
	prepare()
	if len(r.LinkedGitPaths) != 4 {
		t.Fatal("legacy shared repository gained common-directory write access")
	}
}
