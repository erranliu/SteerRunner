package contracts

import (
	"encoding/json"
	"testing"
)

func TestContentAdmission(t *testing.T) {
	base := map[string]any{"title": "content", "workflow": map[string]any{"id": "generate", "version": 1}, "execution": map[string]any{"adapter": "codex-app-server", "target": map[string]any{"kind": "local"}, "settings": map[string]any{"access_mode": "read_only"}}, "generate": map[string]any{"instruction": "Return text", "materials": []any{}}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"generate", func(map[string]any) {}, true},
		{"conversation", func(v map[string]any) {
			v["workflow"].(map[string]any)["id"] = "conversation"
			delete(v, "generate")
			v["conversation"] = map[string]any{"instruction": "Discuss", "materials": []any{}, "directory": map[string]any{"path": "C:/project", "access": "read_only"}}
		}, true},
		{"no_directory", func(v map[string]any) {
			v["workflow"].(map[string]any)["id"] = "conversation"
			v["conversation"] = v["generate"]
			delete(v, "generate")
		}, false},
		{"development_requirements", func(v map[string]any) { v["workflow"] = map[string]any{"id": "development", "version": 2} }, false},
		{"fake_repository", func(v map[string]any) { v["repository"] = map[string]any{} }, false},
		{"fake_documents", func(v map[string]any) { v["documents"] = map[string]any{} }, false},
		{"publication", func(v map[string]any) { v["publication"] = map[string]any{} }, false},
		{"empty_override", func(v map[string]any) { v["workflow"].(map[string]any)["prompt_overrides"] = map[string]any{} }, false},
		{"null_auth", func(v map[string]any) {
			v["execution"].(map[string]any)["settings"].(map[string]any)["github_auth"] = nil
		}, false},
		{"mixed", func(v map[string]any) { v["conversation"] = v["generate"] }, false},
		{"wrong_version", func(v map[string]any) { v["workflow"].(map[string]any)["version"] = 2 }, false},
		{"write_permission", func(v map[string]any) {
			v["execution"].(map[string]any)["settings"].(map[string]any)["access_mode"] = "sandbox"
		}, false},
		{"external_ref", func(v map[string]any) {
			v["generate"].(map[string]any)["output_schema"] = map[string]any{"$ref": "file:///secret"}
		}, false},
		{"unknown_backend", func(v map[string]any) { v["execution"].(map[string]any)["adapter"] = "orca-terminal" }, false},
		{"git_material", func(v map[string]any) {
			v["generate"].(map[string]any)["materials"] = []any{map[string]any{"kind": "git", "path": "x", "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			tc.mutate(v)
			raw, _ = json.Marshal(v)
			if got := ValidateRun(raw); (got == nil) != tc.valid {
				t.Fatalf("gate=%v", got)
			}
		})
	}
}
