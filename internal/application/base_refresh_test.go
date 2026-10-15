package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

type baseRefreshFixture struct {
	t                                                       *testing.T
	root, source, writer, remote, state, remoteName, branch string
	input                                                   contracts.Request
	request                                                 contracts.RunRequest
	initial, pinned                                         string
}

func newBaseRefreshFixture(t *testing.T, remoteName, branch string) *baseRefreshFixture {
	t.Helper()
	f := &baseRefreshFixture{t: t, root: t.TempDir(), remoteName: remoteName, branch: branch}
	f.source, f.writer, f.remote, f.state = filepath.Join(f.root, "source"), filepath.Join(f.root, "writer"), filepath.Join(f.root, "remote.git"), filepath.Join(f.root, "state")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(f.root, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	f.git(f.root, "init", "--quiet", "--bare", f.remote)
	f.git(f.root, "init", "--quiet", "-b", branch, f.writer)
	f.git(f.writer, "config", "user.name", "Runner Fixture")
	f.git(f.writer, "config", "user.email", "fixture@example.invalid")
	f.git(f.writer, "config", "commit.gpgsign", "false")
	f.initial = f.commit("initial")
	url := "https://github.com/owner/repo.git"
	f.git(f.root, "config", "--global", "url."+filepath.ToSlash(f.remote)+".insteadOf", url)
	f.git(f.writer, "remote", "add", remoteName, url)
	f.git(f.writer, "push", remoteName, "HEAD:refs/heads/"+branch)
	f.git(f.root, "clone", "--quiet", "--origin", remoteName, "--branch", branch, "--", url, f.source)
	f.pinned = f.commit("pinned")
	f.git(f.writer, "push", remoteName, "HEAD:refs/heads/"+branch)
	// Match the caller: fetch the new target without advancing its local branch.
	f.git(f.source, "fetch", "--no-tags", remoteName, "+refs/heads/"+branch+":refs/remotes/"+remoteName+"/"+branch)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.request = contracts.RunRequest{
		Title:       "Pinned base refresh regression",
		Repository:  contracts.Repository{URL: url, LocalPath: f.source, Remote: remoteName, BaseBranch: branch, BaseRevision: f.pinned},
		Documents:   contracts.Documents{Spec: contracts.Document{Kind: "git", Path: "spec.md", Commit: f.initial}, Plan: contracts.Document{Kind: "git", Path: "plan.md", Commit: f.initial}},
		Workflow:    contracts.Workflow{ID: "development", Version: 1},
		Execution:   contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: canonical(contracts.CodexSettings{Executable: executable})},
		Publication: contracts.Publication{Provider: "command", Repository: "owner/repo", BaseBranch: branch, Settings: canonical(contracts.CommandProviderSettings{BaseURL: "https://github.com", Query: contracts.QuerySettings{Executable: executable, Args: []string{}}})},
	}
	if _, err := providerFor(&runstate.Record{Request: f.request}); err != nil {
		t.Fatal("invalid publication fixture", err)
	}
	f.input = contracts.Request{Protocol: contracts.Protocol, RequestID: "base-refresh", Input: canonical(f.request)}
	return f
}

func (f *baseRefreshFixture) git(dir string, args ...string) string {
	f.t.Helper()
	out, err := gitHost().Output(context.Background(), dir, args...)
	if err != nil {
		f.t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func (f *baseRefreshFixture) commit(body string) string {
	f.t.Helper()
	for _, name := range []string{"spec.md", "plan.md"} {
		if err := os.WriteFile(filepath.Join(f.writer, name), []byte(body+" "+name), 0600); err != nil {
			f.t.Fatal(err)
		}
	}
	f.git(f.writer, "add", ".")
	f.git(f.writer, "commit", "--quiet", "-m", body)
	return f.git(f.writer, "rev-parse", "HEAD")
}

func (f *baseRefreshFixture) freeze() *runstate.Record {
	f.t.Helper()
	r, err := freeze(context.Background(), f.state, f.input)
	if err != nil {
		f.t.Fatal(err)
	}
	if r.Request.Repository != f.request.Repository || r.Snapshot.WorkspacePlan.BaseCommit != f.pinned || r.Snapshot.Ref.BaseCommit != f.pinned {
		f.t.Fatal("refresh changed the caller's fixed base or repository")
	}
	if len(r.Materials) != 2 {
		f.t.Fatal("frozen Spec/Plan were lost")
	}
	for _, m := range r.Materials {
		if m.Commit != f.initial || m.Content != "initial "+m.Path {
			f.t.Fatal("refresh changed frozen document provenance", m)
		}
	}
	if f.git(f.source, "rev-parse", "refs/heads/"+f.branch) != f.initial {
		f.t.Fatal("refresh advanced the caller's local branch")
	}
	if _, err := os.Stat(filepath.Join(f.state, "workspaces")); !errors.Is(err, os.ErrNotExist) {
		f.t.Fatal("freeze started execution", err)
	}
	return r
}

func TestSRFreezeBaseBranchRefresh(t *testing.T) {
	t.Run("local-branch-behind", func(t *testing.T) {
		f := newBaseRefreshFixture(t, "origin", "main")
		r := f.freeze()
		if f.git(r.RepositoryPath, "rev-parse", "refs/remotes/origin/main") != f.pinned {
			t.Fatal("cache retained the stale local branch")
		}
	})
	t.Run("warm-cache-custom-target", func(t *testing.T) {
		f := newBaseRefreshFixture(t, "upstream", "release/task-101")
		first := f.freeze()
		newHead := f.commit("advanced")
		f.git(f.writer, "push", f.remoteName, "HEAD:refs/heads/"+f.branch)
		second := f.freeze()
		if first.RepositoryPath != second.RepositoryPath || first.InputDigest != second.InputDigest {
			t.Fatal("warm refresh changed cache or frozen input identity")
		}
		if f.git(second.RepositoryPath, "rev-parse", "refs/remotes/"+f.remoteName+"/"+f.branch) != newHead {
			t.Fatal("warm cache did not refresh the configured target")
		}
	})
	for _, scenario := range []string{"off-branch", "rewound", "deleted", "offline"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBaseRefreshFixture(t, "origin", "main")
			if scenario == "off-branch" {
				f.git(f.writer, "checkout", "--quiet", "-b", "side", f.initial)
				f.request.Repository.BaseRevision = f.commit("side")
				f.git(f.writer, "push", f.remoteName, "HEAD:refs/heads/side")
				f.git(f.source, "fetch", f.remoteName, "refs/heads/side:refs/remotes/origin/side")
				f.input.Input = canonical(f.request)
			} else {
				f.freeze() // Seed an otherwise valid warm cache before the failure.
				switch scenario {
				case "rewound":
					f.git(f.writer, "push", "--force", f.remoteName, f.initial+":refs/heads/main")
				case "deleted":
					f.git(f.root, "--git-dir", f.remote, "update-ref", "-d", "refs/heads/main")
				case "offline":
					if err := os.Rename(f.remote, f.remote+".offline"); err != nil {
						t.Fatal(err)
					}
				}
			}
			r, err := freeze(context.Background(), f.state, f.input)
			want := "invalid_input"
			if scenario == "deleted" || scenario == "offline" {
				want = "environment_unavailable"
			}
			var failure *contracts.Fault
			if r != nil || !errors.As(err, &failure) || failure.Detail.Code != want {
				t.Fatalf("expected %s before Run acceptance, got record=%v error=%v", want, r != nil, err)
			}
			if want == "invalid_input" && failure.Detail.Message != "Pinned base is not in the configured base branch." {
				t.Fatalf("unexpected rejection: %s", failure.Detail.Message)
			}
			var unchanged contracts.RunRequest
			if json.Unmarshal(f.input.Input, &unchanged) != nil || unchanged.Repository != f.request.Repository {
				t.Fatal("failed refresh changed the original request")
			}
		})
	}
}
