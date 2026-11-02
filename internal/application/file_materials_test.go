package application

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestContentFileMaterialSnapshot(t *testing.T) {
	for _, mutation := range []string{"none", "changed", "missing", "location"} {
		t.Run(mutation, func(t *testing.T) {
			body := "private original JSON\r\nnever inline this body"
			r := &runstate.Record{ID: "files", InputDigest: "fixed", Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "generate", Version: 1},
				Generate: &contracts.GenerateInput{Instruction: "Read the supplied file", Materials: []contracts.Document{{Kind: "file", Path: "roadmap-inputs/graph.json", Content: &body}}}}}
			fixed, err := freezeContent(context.Background(), t.TempDir(), r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
			if err != nil {
				t.Fatal(err)
			}
			original, _ := os.ReadFile(fixed.Materials[0].SnapshotPath)
			if string(original) != body || !filepath.IsAbs(fixed.Materials[0].ExecutionLocation) {
				t.Fatal("file bytes or readable location were not frozen")
			}
			switch mutation {
			case "changed":
				err = os.WriteFile(fixed.Materials[0].SnapshotPath, []byte("tampered"), 0600)
			case "missing":
				err = os.Remove(fixed.Materials[0].SnapshotPath)
			case "location":
				fixed.Materials[0].ExecutionLocation = filepath.Join(t.TempDir(), "replacement")
			}
			if err != nil {
				t.Fatal(err)
			}
			prompt, err := contentPrompt(fixed, "")
			if mutation != "none" {
				if err == nil {
					t.Fatal("changed original material accepted")
				}
				if mutation == "missing" {
					if _, err := os.Stat(fixed.Materials[0].SnapshotPath); !os.IsNotExist(err) {
						t.Fatal("missing file was recreated")
					}
				}
				return
			}
			if err != nil || strings.Contains(prompt, "private original") || !strings.Contains(prompt, `"file":`) || !strings.Contains(prompt, fixed.Materials[0].Digest) {
				t.Fatal("file body inlined or reference missing", err, prompt)
			}
			// The persisted aggregate resumes the same file, not caller data.
			raw, _ := json.Marshal(fixed)
			var restored runstate.Record
			if err := json.Unmarshal(raw, &restored); err != nil || verifyFrozenMaterials(&restored) != nil {
				t.Fatal("original file snapshot could not be restored", err)
			}
		})
	}
}

func TestContentFileMaterialRejectsDirectoryRedirection(t *testing.T) {
	makeAlias := func(alias, target string) {
		t.Helper()
		if runtime.GOOS == "windows" {
			if output, err := exec.Command("cmd", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
				t.Fatal(err, string(output))
			}
		} else if err := os.Symlink(target, alias); err != nil {
			t.Fatal(err)
		}
	}
	body := "original private input"
	dir, outside := t.TempDir(), t.TempDir()
	parent := filepath.Join(dir, "inputs", "files", "roadmap-inputs")
	if err := os.MkdirAll(filepath.Dir(parent), 0700); err != nil {
		t.Fatal(err)
	}
	makeAlias(parent, outside)
	r := &runstate.Record{ID: "files", InputDigest: "fixed", Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "generate", Version: 1},
		Generate: &contracts.GenerateInput{Instruction: "Read", Materials: []contracts.Document{{Kind: "file", Path: "roadmap-inputs/source.md", Content: &body}}}}}
	if _, err := freezeContent(context.Background(), dir, r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"})); err == nil {
		t.Fatal("redirected input directory accepted")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("file material was written through a directory alias", entries, err)
	}
	// A same-content replacement through a junction must also fail on recovery.
	dir = t.TempDir()
	fixed, err := freezeContent(context.Background(), dir, r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
	if err != nil {
		t.Fatal(err)
	}
	parent = filepath.Dir(fixed.Materials[0].SnapshotPath)
	if err := os.Rename(parent, parent+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "source.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	makeAlias(parent, outside)
	if verifyFrozenMaterials(fixed) == nil {
		t.Fatal("redirected same-content input accepted on recovery")
	}
}
