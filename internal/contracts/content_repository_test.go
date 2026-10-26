package contracts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentRepositoryAdmission(t *testing.T) {
	base := RunRequest{Title: "fixed", Workflow: Workflow{ID: "generate", Version: 1}, Execution: Execution{Adapter: "codex-app-server", Target: Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}, Generate: &GenerateInput{Instruction: "Investigate", Materials: []Document{}, Repository: &GenerateRepository{LocalPath: t.TempDir(), Commit: strings.Repeat("a", 40)}, OutputSchema: json.RawMessage(`{"type":"object"}`)}}
	for _, tc := range []struct {
		name   string
		mutate func(*RunRequest)
		valid  bool
	}{
		{"valid", func(*RunRequest) {}, true},
		{"short-ref", func(r *RunRequest) { r.Generate.Repository.Commit = "HEAD" }, false},
		{"abbreviated", func(r *RunRequest) { r.Generate.Repository.Commit = "abcdef12" }, false},
		{"relative", func(r *RunRequest) { r.Generate.Repository.LocalPath = "relative" }, false},
		{"no-schema", func(r *RunRequest) { r.Generate.OutputSchema = nil }, false},
		{"directory", func(r *RunRequest) {
			r.Generate.Directory = &DirectoryAccess{Path: filepath.Clean(t.TempDir()), Access: "read_only"}
		}, false},
		{"bad-schema", func(r *RunRequest) { r.Generate.OutputSchema = json.RawMessage(`{"type":"bad"}`) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var r RunRequest
			_ = json.Unmarshal(raw, &r)
			tc.mutate(&r)
			raw, _ = json.Marshal(r)
			if fault := ValidateRun(raw); (fault == nil) != tc.valid {
				t.Fatal(fault)
			}
		})
	}
}
