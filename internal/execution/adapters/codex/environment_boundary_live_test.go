//go:build windows && live

package codexworker

import (
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Directed native qualification only: user checks do not repeat boundary
// canaries. Uses the released SA authentication owner with a real valid login.
func TestEnvironmentCheckNativeBoundary(t *testing.T) {
	if *authenticationNative == "" {
		t.Fatal("explicit -sandbox-auth-native is required")
	}
	for _, mode := range []string{"sandbox", "full_access"} {
		t.Run(mode, func(t *testing.T) {
			root, err := os.MkdirTemp("C:/srt", "ecb-")
			if err != nil {
				t.Fatal(err)
			}
			t.Log("preserved native boundary evidence:", root)
			git := workspace.Git{Command: b.LaunchCommand{Executable: "git"}, Timeout: 30 * time.Second, Start: process.New(30 * time.Second).StartManaged}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			repo := filepath.Join(root, "r")
			work := filepath.Join(root, "w")
			for _, args := range [][]string{{"init", "--quiet", "-b", "main", repo}, {"-C", repo, "-c", "user.name=SteerRunner", "-c", "user.email=check@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "--quiet", "-m", "boundary"}, {"-C", repo, "remote", "add", "origin", "https://github.com/erranliu/SteerlineTest.git"}, {"-C", repo, "worktree", "add", "--quiet", "-b", "boundary", work}} {
				if _, err := git.Output(ctx, root, args...); err != nil {
					t.Fatal(err)
				}
			}
			r, err := workspace.ResolveRepository(ctx, git, repo)
			if err != nil {
				t.Fatal(err)
			}
			roots, err := workspace.ResolveLinkedResourcesAt(ctx, git, r, work)
			if err != nil {
				t.Fatal(err)
			}
			args, err := LinkedWorktreeWriteArgs(roots.Objects, roots.Refs, roots.Logs, roots.Admin)
			if err != nil {
				t.Fatal(err)
			}
			settings, _ := json.Marshal(contracts.CodexSettings{Executable: *authenticationNative, AccessMode: mode})
			s, err := NewEnvironmentSession(EnvironmentSetup{CheckID: strings.Repeat("c", 32), Nonce: strings.Repeat("d", 32), Directory: root, Workspace: work, Input: contracts.CheckInput{Repository: contracts.Repository{URL: "https://github.com/erranliu/SteerlineTest.git", Remote: "origin"}, Execution: contracts.Execution{Settings: settings}}, LinkedWriteArgs: args})
			if err != nil {
				t.Fatal(err)
			}
			binding, err := s.Prepare(ctx)
			defer func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer stopCancel()
				if !s.Stop(stopCtx) {
					t.Error("original writer tree was not settled")
				}
			}()
			if err != nil {
				t.Fatalf("same-mode preparation: %s", EnvironmentDiagnostic(err))
			}
			outside := filepath.Join(root, "outside-canary.txt")
			_, writeErr := s.exec(ctx, `[IO.File]::WriteAllText(`+psLiteral(outside)+`,'canary'); $facts['canary']='written'`)
			_, statErr := os.Stat(outside)
			passed := mode == "sandbox" && writeErr != nil && os.IsNotExist(statErr) || mode == "full_access" && writeErr == nil && statErr == nil
			fact := map[string]any{"mode": mode, "policy_contract": PolicyContract, "process": binding.Process, "session_id": binding.SessionID, "generation": binding.Generation, "valid_authentication_preparation": true, "outside_write_succeeded": writeErr == nil, "outside_file_exists": statErr == nil, "passed": passed}
			body, _ := json.MarshalIndent(fact, "", "  ")
			if err := os.WriteFile(filepath.Join(root, "boundary-result.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			if !passed {
				t.Fatal("native boundary differs from requested access mode", fact)
			}
		})
	}
}
