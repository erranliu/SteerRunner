package orcatransport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestFixedWorkspaceAuditUsesFreshOriginalControlAndReconcilesPendingRead(t *testing.T) {
	for _, mode := range []string{"fresh", "lost-send", "workspace-foreign-branch", "workspace-dirty", "workspace-reader-live"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			c.mode = mode
			before := f.nativeSends
			fact, err := l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
			if mode == "fresh" {
				if err != nil || !fact.Qualified {
					t.Fatal(fact, err)
				}
			} else if err == nil {
				t.Fatal("unverified source accepted", mode, fact)
			}
			if mode == "lost-send" {
				c.mode = "success"
				fact, err = l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
				if err != nil || !fact.Qualified || c.sends != 1 {
					t.Fatal("pending original read was replaced", fact, err, c.sends)
				}
			}
			if mode == "fresh" || mode == "lost-send" {
				fact, err = l.InspectLifecycleAuditWorkspace(context.Background(), s, "")
				if err != nil || !fact.Qualified || c.sends != 2 {
					t.Fatal("new checkpoint reused stale source", fact, err, c.sends)
				}
			}
			if c.creates != 1 || f.nativeSends != before {
				t.Fatal("source query created resources or business input", c.creates, f.nativeSends)
			}
		})
	}
}

func TestStructuredWorkspaceAuditReadsOriginalChangedTree(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires managed Git reader")
	}
	l, snapshot, _ := structuredFixture(t)
	state, err := l.agentState(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(content string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		git("add", "tracked.txt")
		git("-c", "user.name=Audit", "-c", "user.email=audit@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", content)
		return git("rev-parse", "HEAD")
	}
	git("init", "-b", "original")
	const repository = "https://example.invalid/runner/original.git"
	git("remote", "add", "origin", repository)
	base := commit("implementation")
	snapshot.Workspace.Path, snapshot.Workspace.Branch, snapshot.Workspace.BaseCommit = root, "original", base
	state.RuntimeID, state.Worktree.Path, state.Worktree.Branch, state.Worktree.BaseRef = "runtime", root, "original", base
	state.Attempted = true
	snapshot.Ref.External.RuntimeID = state.RuntimeID
	snapshot.Implementation, err = l.agentEnvelope(snapshot, state)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &fakeHost{workspace: map[string]any{"id": state.Worktree.ID, "repoId": "repo", "instanceId": state.Worktree.InstanceID, "path": root, "branch": "original", "baseRef": base, "head": base}}
	l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		return fixture.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
	}})
	l.setup.ProcessHost, l.setup.RepositoryURL = process.New(10*time.Second), repository
	before, err := l.InspectLifecycleAuditWorkspace(context.Background(), snapshot, "")
	if err != nil || !before.Qualified || before.Head != base {
		t.Fatal(before, err)
	}
	audit := commit("audited fix")
	after, err := l.InspectLifecycleAuditWorkspace(context.Background(), snapshot, base)
	if err != nil || !after.Qualified || after.Head != audit || after.Tree == before.Tree {
		t.Fatal(after, err)
	}
	fixture.workspace["instanceId"] = "replacement"
	if fact, err := l.InspectLifecycleAuditWorkspace(context.Background(), snapshot, base); err == nil || fact.Qualified {
		t.Fatal("replacement worktree qualified", fact, err)
	}
}
