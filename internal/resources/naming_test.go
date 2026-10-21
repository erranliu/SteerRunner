package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestRunNamePrefixPublishedContract(t *testing.T) {
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(ConfigurationSchemas(), &schemas); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemas["run_input"])))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("https://runner.invalid/naming", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://runner.invalid/naming")
	if err != nil {
		t.Fatal(err)
	}
	for _, workflow := range []string{"development", "generate", "conversation"} {
		for _, tc := range []struct {
			name   string
			prefix any
			valid  bool
		}{
			{"omitted", nil, true}, {"task-candidate", "sl-103-c2", true},
			{"generic", "acme-feature-7", true}, {"max", strings.Repeat("a", 48), true},
			{"empty", "", false}, {"null", json.RawMessage("null"), false},
			{"long", strings.Repeat("a", 49), false}, {"path", "../outside", false},
			{"option", "--activate", false}, {"space", "two words", false},
			{"uppercase", "Name", false}, {"unicode", "任务", false},
			{"trailing-hyphen", "name-", false}, {"double-hyphen", "name--x", false},
		} {
			t.Run(workflow+"/"+tc.name, func(t *testing.T) {
				input := map[string]any{
					"title": "Naming contract", "workflow": map[string]any{"id": workflow, "version": 1},
					"execution": map[string]any{"adapter": "codex-app-server", "target": map[string]any{"kind": "local"}, "settings": map[string]any{}},
				}
				if workflow == "development" {
					input["workflow"] = map[string]any{"id": workflow, "version": 2}
					input["repository"] = map[string]any{"url": "https://example.com/repo.git", "base_revision": strings.Repeat("a", 40), "remote": "origin", "base_branch": "main"}
					input["documents"] = map[string]any{"spec": map[string]any{"kind": "inline", "path": "spec.md", "content": "spec"}, "plan": map[string]any{"kind": "inline", "path": "plan.md", "content": "plan"}}
					input["publication"] = map[string]any{"provider": "gitea", "repository": "owner/repo", "base_branch": "main", "settings": map[string]any{"base_url": "https://example.com"}}
				} else {
					input["execution"].(map[string]any)["settings"] = map[string]any{"access_mode": "read_only"}
					content := map[string]any{"instruction": "Reply", "materials": []any{}}
					if workflow == "conversation" {
						content["directory"] = map[string]any{"path": t.TempDir(), "access": "read_only"}
					}
					input[workflow] = content
				}
				if tc.prefix != nil {
					input["name_prefix"] = tc.prefix
				}
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				var decoded any
				if err := json.Unmarshal(raw, &decoded); err != nil {
					t.Fatal(err)
				}
				if accepted := schema.Validate(decoded) == nil; accepted != tc.valid {
					t.Fatalf("published schema accepted=%v, want %v: %v", accepted, tc.valid, schema.Validate(decoded))
				}
				if accepted := contracts.ValidateRun(raw) == nil; accepted != tc.valid {
					t.Fatalf("production admission accepted=%v, want %v: %v", accepted, tc.valid, contracts.ValidateRun(raw))
				}
			})
		}
	}
}
