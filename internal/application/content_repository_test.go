package application

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func repositoryFixture(t *testing.T) (string, string) {
	t.Helper()
	path := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := gitHost().Output(context.Background(), path, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Runner Test")
	git("config", "user.email", "runner@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("fixed-version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "code.txt")
	git("-c", "core.hooksPath=", "commit", "-m", "fixed")
	commit := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("later-version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "code.txt")
	git("-c", "core.hooksPath=", "commit", "-m", "later")
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("dirty-version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "untracked.txt"), []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	return path, commit
}

func TestContentRepositoryFixedObjectsAndRecovery(t *testing.T) {
	path, commit := repositoryFixture(t)
	material := "Roadmap R7"
	r := &runstate.Record{ID: "repo", InputDigest: "fixed", Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "generate", Version: 1}, Generate: &contracts.GenerateInput{Repository: &contracts.GenerateRepository{LocalPath: path, Commit: commit}, Instruction: "Investigate", Materials: []contracts.Document{{Kind: "inline", Path: "roadmap.txt", Content: &material}}, OutputSchema: json.RawMessage(`{"type":"object"}`)}}}
	r, err := freezeContent(context.Background(), t.TempDir(), r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(r.RepositoryPath, "code.txt"))
	if err != nil || string(data) != "fixed-version\n" || len(r.RepositorySource.Files) != 1 || r.RepositorySource.Commit != commit || r.RepositorySource.Tree == "" || r.RepositorySource.Files[0].SHA256 != reliable.Digest(data) {
		t.Fatal("fixed tree mismatch", r.RepositorySource, err)
	}
	if _, err := os.Stat(filepath.Join(r.RepositoryPath, ".git")); !os.IsNotExist(err) {
		t.Fatal("snapshot exposed Git")
	}
	prompt, err := contentPrompt(r, "")
	if err != nil || !strings.Contains(prompt, commit) || !strings.Contains(prompt, material) || strings.Contains(prompt, "dirty-version") {
		t.Fatal(prompt, err)
	}
	// Recovery uses exactly the durable snapshot, even if source checkout is gone.
	restored := r.Clone()
	restored.Request.Generate.Repository.LocalPath = filepath.Join(path, "gone")
	if verifyContentDirectory(restored) != nil {
		t.Fatal("recovery consulted original checkout")
	}
	if err := os.WriteFile(filepath.Join(path, "code.txt"), []byte("still-dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyContentDirectory(r) != nil {
		t.Fatal("caller edits affected snapshot")
	}
	if err := os.WriteFile(filepath.Join(r.RepositoryPath, "extra.txt"), []byte("injected"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyContentDirectory(r) == nil {
		t.Fatal("snapshot injection accepted")
	}
	if verifyFrozenMaterials(r) == nil {
		t.Fatal("takeover accepted changed repository snapshot")
	}
	if _, err := contentPrompt(r, ""); err == nil {
		t.Fatal("changed snapshot dispatched")
	}
	if err := os.Remove(filepath.Join(r.RepositoryPath, "extra.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.RepositoryPath, "code.txt"), []byte("wrong-version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyContentDirectory(r) == nil {
		t.Fatal("snapshot modification accepted")
	}
}

func TestContentGenerateDocumentLimits(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["documents"],"additionalProperties":false,"properties":{"documents":{"type":"array","minItems":2,"maxItems":2,"prefixItems":[{"type":"object","required":["path","content"],"properties":{"path":{"const":"R7/spec.md"},"content":{"type":"string","minLength":1}}},{"type":"object","required":["path","content"],"properties":{"path":{"const":"R7/plan.md"},"content":{"type":"string","minLength":1}}}]}}}`)
	valid := `{"documents":[{"path":"R7/spec.md","content":"# Spec\n完整正文"},{"path":"R7/plan.md","content":"# Plan\n完整正文"}]}`
	for _, tc := range []struct {
		name, text     string
		verified, want bool
	}{
		{"complete", valid, true, true}, {"missing-document", `{"documents":[{"path":"R7/spec.md","content":"x"}]}`, true, false}, {"bad-json", valid[:len(valid)-2], true, false}, {"bad-schema", `{"documents":[]}`, true, false}, {"missing-final", valid, false, false}, {"oversize", strings.Repeat("x", contracts.MaxGenerateResultBytes+1), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := contentEngine(t, "generate")
			e.runs["run"].Request.Generate.OutputSchema = schema
			if !e.finish("run", contentEvidence(e, tc.text, "succeeded", tc.verified)) {
				t.Fatal("result not consumed")
			}
			r := e.runs["run"]
			if (r.ContentResult.Status == "succeeded") != tc.want {
				t.Fatal(r.ContentResult.Status, r.ContentResult.Diagnostic)
			}
			if tc.want && string(r.ContentResult.JSON) != valid {
				t.Fatal("result body changed")
			}
			settleContentStatus(r)
			if !tc.want && r.Status == "completed" {
				t.Fatal("bad result completed")
			}
			if tc.want {
				var page map[string]any
				raw := canonical(contentHistory(r, "", 100).Data)
				if json.Unmarshal(raw, &page) != nil || !strings.Contains(string(raw), "完整正文") {
					t.Fatal("history lost full document")
				}
			}
		})
	}
	// Exact size limit is allowed for schema-free generation.
	e, _ := contentEngine(t, "generate")
	if !e.finish("run", contentEvidence(e, strings.Repeat("a", contracts.MaxGenerateResultBytes), "succeeded", true)) || e.runs["run"].ContentResult.Status != "succeeded" {
		t.Fatal("boundary rejected")
	}
}

func TestContentRepositoryResultRejectsDrift(t *testing.T) {
	e, _ := contentEngine(t, "generate")
	r := e.runs["run"]
	r.Request.Generate.Repository = &contracts.GenerateRepository{Commit: strings.Repeat("a", 40)}
	r.RepositorySource = &contracts.RepositorySource{Commit: strings.Repeat("a", 40), Files: []contracts.RepositoryFile{}}
	if err := os.WriteFile(filepath.Join(r.RepositoryPath, "injected"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !e.finish("run", contentEvidence(e, "{}", "succeeded", true)) || e.runs["run"].ContentResult.Diagnostic != "repository_snapshot_changed" {
		t.Fatal("drift succeeded")
	}
}
