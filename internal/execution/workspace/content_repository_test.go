package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestContentRepositoryUnsafeAndLimits(t *testing.T) {
	g := Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: process.New(30 * time.Second).StartManaged}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := g.Output(context.Background(), dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	run("init")
	run("config", "user.name", "Runner Test")
	run("config", "user.email", "runner@example.invalid")
	write := func(input string, args ...string) string {
		t.Helper()
		out, err := g.OutputInput(context.Background(), dir, strings.NewReader(input), 16<<20, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	blob := write("original\n", "hash-object", "-w", "--stdin")
	large := write(strings.Repeat("x", 32768), "hash-object", "-w", "--stdin")
	for _, tc := range []struct{ name, tree string }{
		{"link", fmt.Sprintf("120000 blob %s\tlink\n", blob)},
		{"case-collision", fmt.Sprintf("100644 blob %s\tFile\n100644 blob %s\tfile\n", blob, blob)},
		{"reserved-git", fmt.Sprintf("100644 blob %s\t.git\n", blob)},
		{"file-count", func() string {
			var s strings.Builder
			for i := 0; i <= contracts.MaxGenerateRepositoryFiles; i++ {
				fmt.Fprintf(&s, "100644 blob %s\tf%05d\n", blob, i)
			}
			return s.String()
		}()},
		{"total-size", func() string {
			var s strings.Builder
			for i := 0; i < 2049; i++ {
				fmt.Fprintf(&s, "100644 blob %s\tf%05d\n", large, i)
			}
			return s.String()
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := write(tc.tree, "mktree")
			commit := run("commit-tree", tree, "-m", "test snapshot")
			destination := filepath.Join(t.TempDir(), "snapshot")
			if _, err := FreezeContentRepository(context.Background(), g, contracts.GenerateRepository{LocalPath: dir, Commit: commit}, destination); err == nil {
				t.Fatal("unsupported tree accepted")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("failed admission left snapshot", err)
			}
		})
	}
	// Annotated tags/tree object IDs are not commits; no implicit peeling.
	tree := write(fmt.Sprintf("100644 blob %s\tcode.txt\n", blob), "mktree")
	if _, err := FreezeContentRepository(context.Background(), g, contracts.GenerateRepository{LocalPath: dir, Commit: tree}, filepath.Join(t.TempDir(), "snapshot")); err == nil {
		t.Fatal("tree accepted as commit")
	}
	commit := run("commit-tree", tree, "-m", "fixed")
	// Ambient Git routing must not redirect a caller-bound repository.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing"))
	destination := filepath.Join(t.TempDir(), "snapshot")
	if source, err := FreezeContentRepository(context.Background(), g, contracts.GenerateRepository{LocalPath: dir, Commit: commit}, destination); err != nil || source.Commit != commit {
		t.Fatal("ambient Git routing leaked", err)
	}
}
