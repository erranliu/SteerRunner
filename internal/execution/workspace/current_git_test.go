package workspace

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR2GitRejectsConcurrentChange(t *testing.T) {
	for _, change := range []string{"dirty", "commit", "header_like_rename"} {
		t.Run(change, func(t *testing.T) {
			start := func(cmd *exec.Cmd) (b.StopHandle, error) {
				if err := cmd.Start(); err != nil {
					return nil, err
				}
				return testProcess{cmd.Process}, nil
			}
			g := Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 10 * time.Second, Start: start}
			repo := filepath.Join(t.TempDir(), "repo")
			prepareGitFixture(t, g, repo)
			ctx := context.Background()
			identity, err := ResolveRepository(ctx, g, repo)
			if err != nil {
				t.Fatal(err)
			}
			head, err := g.Output(ctx, repo, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			branch, err := g.Output(ctx, repo, "branch", "--show-current")
			if err != nil {
				t.Fatal(err)
			}
			intent := &b.WorkspaceIntent{Path: repo, Branch: strings.TrimSpace(string(branch)), BaseCommit: strings.TrimSpace(string(head))}
			if change == "header_like_rename" {
				name := "# branch.oid " + strings.Repeat("b", 40)
				if err := os.WriteFile(filepath.Join(repo, name), []byte("file"), 0600); err != nil {
					t.Fatal(err)
				}
				for _, args := range [][]string{{"add", "--", name}, {"-c", "user.name=Fixture", "-c", "user.email=f@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "name"}, {"mv", "--", name, "renamed"}} {
					if _, err := g.Output(ctx, repo, args...); err != nil {
						t.Fatal(err)
					}
				}
				current, err := g.Output(ctx, repo, "rev-parse", "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				got, clean, err := CurrentGit(ctx, g, identity, intent)
				if err != nil || clean || got != strings.TrimSpace(string(current)) {
					t.Fatal("pathname spoofed current commit", got, clean, err)
				}
				return
			}
			count := 0
			observed := g
			observed.Start = func(cmd *exec.Cmd) (b.StopHandle, error) {
				for _, a := range cmd.Args {
					if a == "--porcelain=v2" {
						count++
						if count == 2 {
							if change == "dirty" {
								if err := os.WriteFile(filepath.Join(repo, "concurrent-untracked"), []byte("changed"), 0600); err != nil {
									t.Fatal(err)
								}
							} else {
								if _, err := g.Output(ctx, repo, "-c", "user.name=Fixture", "-c", "user.email=f@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "concurrent"); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
				}
				return start(cmd)
			}
			if _, clean, err := CurrentGit(ctx, observed, identity, intent); err == nil || clean {
				t.Fatal("concurrent change reported verified", clean, err)
			}
		})
	}
}
