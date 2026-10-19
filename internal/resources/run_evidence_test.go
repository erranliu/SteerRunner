package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"testing"
	"time"
)

func TestR2PublishedEvidenceContract(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(Read("run-evidence.schema.json"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("https://runner.invalid/r2", doc); err != nil {
		t.Fatal(err)
	}
	a := contracts.Artifact{RunID: "R", ArtifactID: "A", RelativePath: "report.json", SHA256: strings.Repeat("a", 64), SizeBytes: 0, CollectedAt: time.Now().UTC(), ReadCommand: "artifacts read --run-id R --artifact-id A"}
	commit, clean := strings.Repeat("b", 40), true
	fixtures := map[string]any{
		"artifact": a, "read": contracts.ArtifactContent{Artifact: a, Encoding: "base64", ContentBase64: ""},
		"list":          contracts.ArtifactList{RunID: "R", Artifacts: []contracts.Artifact{a}},
		"discover":      contracts.ArtifactDiscovery{RunID: "R", Directory: contracts.ArtifactDirectory, Candidates: []contracts.ArtifactCandidate{{RelativePath: "report.json", SizeBytes: 0}}},
		"collect":       map[string]any{"command_id": "C", "run_id": "R", "receipt_status": "accepted", "action_status": "completed", "artifact": a},
		"workspace_git": contracts.WorkspaceGit{RunID: "R", Commit: &commit, Clean: &clean, ObservedAt: time.Now().UTC(), Verified: true},
	}
	var cap struct {
		Examples struct {
			Collect contracts.Request `json:"collect"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(Read("run-evidence.capabilities.json"), &cap); err != nil {
		t.Fatal(err)
	}
	if f := contracts.ValidateInput("artifacts collect", cap.Examples.Collect.Input); f != nil {
		t.Fatal(f)
	}
	var collect any
	_ = json.Unmarshal(cap.Examples.Collect.Input, &collect)
	fixtures["collect_input"] = collect
	for name, value := range fixtures {
		schema, err := compiler.Compile("https://runner.invalid/r2#/$defs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		var decoded any
		_ = json.Unmarshal(raw, &decoded)
		if err := schema.Validate(decoded); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	schema, err := compiler.Compile("https://runner.invalid/r2#/$defs/workspace_git")
	if err != nil {
		t.Fatal(err)
	}
	v := map[string]any{"run_id": "R", "commit": nil, "clean": nil, "verified": false, "observed_at": time.Now().UTC().Format(time.RFC3339Nano), "diagnostic": "workspace missing"}
	if err := schema.Validate(v); err != nil {
		t.Fatal(err)
	}
	v["clean"] = true
	if schema.Validate(v) == nil {
		t.Fatal("unverified clean was accepted by schema")
	}
}
