package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

type originalRemoteWorkspaceOwner struct {
	b.LifecyclePort
	calls   int
	preHead string
	fail    bool
}

func (p *originalRemoteWorkspaceOwner) InspectLifecycleAuditWorkspace(_ context.Context, s b.LifecycleSnapshot, pre string) (b.WorkspaceInspection, error) {
	p.calls++
	p.preHead = pre
	f := b.WorkspaceInspection{Repository: "remote-original-repository", Worktree: s.Workspace.Path, Branch: s.Workspace.Branch, Base: s.Workspace.BaseCommit, Head: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Qualified: true}
	if p.fail {
		return f, errors.New("original owner observation failed")
	}
	return f, nil
}
func TestLocalOrcaWorkspaceSourceRequiresOriginalOwnerWithoutFallback(t *testing.T) {
	for _, mode := range []string{"original", "missing-owner", "unsupported-owner", "owner-error"} {
		t.Run(mode, func(t *testing.T) {
			p := &originalRemoteWorkspaceOwner{fail: mode == "owner-error"}
			e := &Engine{ctx: context.Background(), workers: map[string]*worker{"run": {port: p}}}
			if mode == "missing-owner" {
				delete(e.workers, "run")
			}
			if mode == "unsupported-owner" {
				e.workers["run"].port = &readOnlyUnsupportedWorkspaceOwner{}
			}
			r := &runstate.Record{ID: "run", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}}}, RepositoryPath: "caller-cannot-open-remote-path", Snapshot: b.LifecycleSnapshot{Workspace: b.WorkspaceIntent{Path: "remote-original-workspace", Branch: "original", BaseCommit: strings.Repeat("c", 40)}}}
			f := e.inspectSource(r, strings.Repeat("d", 40))
			if f.Qualified != (mode == "original") {
				t.Fatal(mode, f)
			}
			if mode == "original" && (p.calls != 1 || p.preHead != strings.Repeat("d", 40) || f.Repository != "remote-original-repository") {
				t.Fatal("original remote source was replaced", p, f)
			}
			if mode != "original" && f.Diagnostic == "" {
				t.Fatal("missing source uncertainty")
			}
		})
	}
}

type readOnlyUnsupportedWorkspaceOwner struct{ b.LifecyclePort }

func TestNativeRemoteWorkspaceSourceNeverCallsHelperOwner(t *testing.T) {
	p := &originalRemoteWorkspaceOwner{}
	e := &Engine{ctx: context.Background(), workers: map[string]*worker{"run": {port: p}}}
	r := &runstate.Record{ID: "run", Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "orca_environment", Environment: "paired"}}}, RepositoryPath: t.TempDir()}
	got := e.inspectSource(r, "")
	if got.Qualified || got.Diagnostic == "" || p.calls != 0 {
		t.Fatal("native remote source gained helper or caller qualification", got, p.calls)
	}
}
