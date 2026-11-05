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
	shows, replaceAfterRead := 0, false
	l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		if args[0] == "worktree" && args[1] == "show" {
			shows++
			if replaceAfterRead && shows == 2 {
				fixture.workspace["instanceId"] = "replacement"
			}
		}
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
	fixture.workspace["instanceId"] = state.Worktree.InstanceID
	shows, replaceAfterRead = 0, true
	if fact, err := l.InspectLifecycleAuditWorkspace(context.Background(), snapshot, base); err == nil || fact.Qualified || shows != 2 {
		t.Fatal("worktree replaced after Git reads qualified", fact, err, shows)
	}
}
