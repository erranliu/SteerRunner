package application

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/execution/workspace"
)

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
		if err = e.attachWriteRoots(r); err != nil {
			t.Fatal(err)
		}
	}
	prepare()
	if len(r.LinkedWriteArgs) != 10 || r.LinkedWriteArgs[9] != r.RepositoryCommon {
		t.Fatal("new private repository lacks root-level Git lock access")
	}
	// Simulate the original storage layout without migrating its permissions.
	r.RepositoryPath = repositoryCache(f.state, f.request.Repository)
	r.RepositoryCommon = filepath.Join(r.RepositoryPath, ".git")
	r.Snapshot.WorkspacePlan.Path += "-legacy"
	r.Snapshot.WorkspacePlan.Branch += "-legacy"
	prepare()
	if len(r.LinkedWriteArgs) != 8 {
		t.Fatal("legacy shared repository gained common-directory write access")
	}
}
