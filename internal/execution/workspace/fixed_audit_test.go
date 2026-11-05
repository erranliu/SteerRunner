package workspace

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

type fixedAuditTestHost struct {
	b.ManagedProcessHost
	start func(*exec.Cmd) (b.StopHandle, error)
}

func (h fixedAuditTestHost) StartManaged(cmd *exec.Cmd) (b.StopHandle, error) {
	return h.start(cmd)
}

type readerWithoutTreeProof struct{ b.StopHandle }

func TestFixedAuditRejectsUnsettledReaderAndChangedHead(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires managed Git reader")
	}
	for _, mode := range []string{"reader-unverified", "head-changed"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repo")
			prepareGitFixture(t, Git{}, path)
			runGit := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", path}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v (%s)", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			const url = "https://example.invalid/owner/repo.git"
			runGit("remote", "add", "original", url)
			base := runGit("rev-parse", "HEAD")
			owner := process.New(30 * time.Second)
			headReads := 0
			host := fixedAuditTestHost{ManagedProcessHost: owner, start: func(cmd *exec.Cmd) (b.StopHandle, error) {
				prefix := []string{"git", "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}
				if !SamePath(path, cmd.Dir) || len(cmd.Args) < len(prefix) || !slices.Equal(prefix, cmd.Args[:len(prefix)]) {
					t.Fatalf("fixed Git reader changed: %q in %q", cmd.Args, cmd.Dir)
				}
				if mode == "head-changed" && slices.Equal(cmd.Args[len(prefix):], []string{"rev-parse", "HEAD"}) {
					headReads++
					if headReads == 2 {
						runGit("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "concurrent head")
					}
				}
				h, err := owner.StartManaged(cmd)
				if err == nil && mode == "reader-unverified" {
					return readerWithoutTreeProof{h}, nil
				}
				return h, err
			}}
			fact, settled, err := InspectFixedAudit(context.Background(), host, path, url, "original", b.WorkspaceIntent{Path: path, Branch: "main", BaseCommit: base}, "")
			want := "original_workspace_reader_tree_unverified"
			if mode == "head-changed" {
				want = "HEAD changed during workspace inspection"
			}
			if err == nil || err.Error() != want || fact.Qualified || fact.Head != base || fact.Tree == "" || settled != (mode == "head-changed") {
				t.Fatalf("unexpected fixed audit result: %+v settled=%v err=%v", fact, settled, err)
			}
		})
	}
}
