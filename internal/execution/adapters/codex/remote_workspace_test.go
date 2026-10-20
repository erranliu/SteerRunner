package codexworker

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

func TestRemoteWorkspaceAuditUsesOriginalGitOwner(t *testing.T) {
	for _, mode := range []string{"clean", "dirty", "hidden-index", "sparse", "foreign-repository", "foreign-session", "foreign-helper", "foreign-workspace", "foreign-branch", "foreign-base", "foreign-pre-head"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", t.TempDir())
			root := t.TempDir()
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "--initial-branch=main")
			git("config", "user.name", "audit fixture")
			git("config", "user.email", "fixture@example.invalid")
			git("config", "commit.gpgsign", "false")
			if err := os.WriteFile(filepath.Join(root, "artifact.txt"), []byte("original\n"), 0600); err != nil {
				t.Fatal(err)
			}
			git("add", "artifact.txt")
			git("commit", "-m", "base")
			git("remote", "add", "origin", "https://github.com/owner/repo.git")
			base := git("rev-parse", "HEAD")
			owner, err := process.CurrentIdentity()
			if err != nil {
				t.Fatal(err)
			}
			sha, err := RuntimeBinarySHA256()
			if err != nil {
				t.Fatal(err)
			}
			setup := RemoteControlSetup{Workspace: root, RuntimeSHA256: sha, Binding: strings.Repeat("a", 64)}
			query := RemoteWorkspaceAuditQuery{Intent: b.WorkspaceIntent{Path: root, Branch: "main", BaseCommit: base}}
			cfg := RemoteAgentConfig{Executable: "unused-native.exe", RepositoryURL: "https://github.com/owner/repo.git"}
			ready := RemoteReadiness{Verified: true, Contract: PolicyContract, NativeTransport: RemoteNativeTransport, RuntimeSHA256: sha, ConfigurationDigest: RemoteConfigurationDigest(cfg), Repository: cfg.RepositoryURL, Workspace: root, SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", Process: *owner, Owner: *owner}
			dir := RemoteRuntimeDirectory(root)
			if err = os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]any{"ready.json": ready, "configuration.json": cfg} {
				raw, _ := json.Marshal(value)
				if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			session := ready.SessionID
			switch mode {
			case "dirty":
				_ = os.WriteFile(filepath.Join(root, "artifact.txt"), []byte("changed\n"), 0600)
			case "hidden-index":
				git("update-index", "--assume-unchanged", "artifact.txt")
			case "sparse":
				git("config", "core.sparseCheckout", "true")
			case "foreign-repository":
				git("remote", "set-url", "origin", "https://github.com/foreign/repo.git")
			case "foreign-session":
				session = "01a1433a-c857-7fc0-bf8c-02a784ad118f"
			case "foreign-helper":
				setup.RuntimeSHA256 = strings.Repeat("f", 64)
			case "foreign-workspace":
				query.Intent.Path = t.TempDir()
			case "foreign-branch":
				query.Intent.Branch = "foreign"
			case "foreign-base":
				query.Intent.BaseCommit = strings.Repeat("f", 40)
			case "foreign-pre-head":
				query.PreHead = strings.Repeat("f", 40)
			}
			indexBefore, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			proof, err := InspectRemoteWorkspaceAudit(ctx, setup, session, query)
			if (err == nil) != (mode == "clean") || !proof.WritersAbsent || proof.Inspection.Qualified != (mode == "clean") {
				t.Fatalf("mode=%s proof=%+v err=%v", mode, proof, err)
			}
			if mode == "clean" && ValidateRemoteWorkspaceAudit(ready, query, proof) != nil {
				t.Fatal("original complete source rejected")
			}
			indexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
			if !bytes.Equal(indexBefore, indexAfter) || git("rev-parse", "HEAD") != base || process.Probe(owner).State != "matches" {
				t.Fatal("readonly audit changed source or original business owner")
			}
		})
	}
}

func TestRemoteWorkspaceEvidenceCannotPromoteRevisionOnlyOrForeignFacts(t *testing.T) {
	root := t.TempDir()
	ready := RemoteReadiness{Workspace: root, SessionID: "original"}
	q := RemoteWorkspaceAuditQuery{Intent: b.WorkspaceIntent{Path: root, Branch: "original", BaseCommit: strings.Repeat("a", 40)}}
	for _, mode := range []string{"complete", "revision-only", "foreign-owner", "foreign-branch", "dirty", "missing-tree", "reader-live"} {
		t.Run(mode, func(t *testing.T) {
			proof := RemoteWorkspaceAudit{Readiness: ready, WritersAbsent: true, Inspection: b.WorkspaceInspection{Repository: root, Worktree: root, Branch: q.Intent.Branch, Base: q.Intent.BaseCommit, Head: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), Qualified: true, InspectionScope: "working_tree", CheckedAt: time.Now().UTC()}}
			switch mode {
			case "revision-only":
				proof.Inspection.InspectionScope = "revision"
			case "foreign-owner":
				proof.Readiness.SessionID = "foreign"
			case "foreign-branch":
				proof.Inspection.Branch = "foreign"
			case "dirty":
				proof.Inspection.Dirty = true
			case "missing-tree":
				proof.Inspection.Tree = ""
			case "reader-live":
				proof.WritersAbsent = false
			}
			if err := ValidateRemoteWorkspaceAudit(ready, q, proof); (err == nil) != (mode == "complete") {
				t.Fatal(mode, err)
			}
		})
	}
}
