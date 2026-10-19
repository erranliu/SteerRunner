package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
)

type brokenInput struct{}

func (brokenInput) Read([]byte) (int, error) { return 0, errors.New("reader failed") }

func TestSRStaticConfigurationInterfaces(t *testing.T) {
	// Neither static path may normalize/create the host default directory.
	absent := filepath.Join(t.TempDir(), "missing-local-app-data")
	t.Setenv("LOCALAPPDATA", absent)
	valid := `{"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{}},"workflow":{"id":"development","version":1}}`
	text := "Frozen document"
	run := contracts.RunRequest{Title: "Validate without readiness", Repository: contracts.Repository{URL: "https://github.com/example/repository.git", Remote: "origin", BaseBranch: "main", BaseRevision: strings.Repeat("a", 40)}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"executable":"not-installed-codex.exe"}`)}, Workflow: contracts.Workflow{ID: "development", Version: 1}, Documents: contracts.Documents{Spec: contracts.Document{Kind: "inline", Path: "spec.md", Content: &text}, Plan: contracts.Document{Kind: "inline", Path: "plan.md", Content: &text}}, Publication: contracts.Publication{Provider: "gitea", Repository: "example/repository", BaseBranch: "main", Settings: json.RawMessage(`{"base_url":"https://example.invalid"}`)}}
	validRun, _ := json.Marshal(run)
	validCheck, _ := json.Marshal(contracts.CheckInput{Repository: run.Repository, Execution: run.Execution})
	tests := []struct {
		name    string
		args    []string
		reader  io.Reader
		exit    int
		code    string
		hasData bool
	}{
		{"valid", []string{"validate", "--kind", "run-configuration", "--input", "-"}, strings.NewReader(valid), 0, "", true},
		{"valid-run-no-readiness-probe", []string{"validate", "--kind", "run", "--input", "-"}, bytes.NewReader(validRun), 0, "", true},
		{"valid-check-no-workflow", []string{"validate", "--kind", "environment-check", "--input", "-"}, bytes.NewReader(validCheck), 0, "", true},
		{"unknown-workflow-priority", []string{"validate", "--kind", "run-configuration", "--input", "-"}, strings.NewReader(strings.Replace(valid, "development", "unknown", 1)), 3, "unsupported_schema", true},
		{"envelope-not-auto-unwrapped", []string{"validate", "--kind", "run", "--input", "-"}, strings.NewReader(`{"protocol":"steerrunner-cli/v1","request_id":"id","input":{}}`), 2, "invalid_input", true},
		{"duplicate", []string{"validate", "--kind", "run", "--input", "-"}, strings.NewReader(`{"title":"a","title":"b"}`), 2, "invalid_input", true},
		{"non-object", []string{"validate", "--kind", "run", "--input", "-"}, strings.NewReader(`[]`), 2, "invalid_input", true},
		{"utf8", []string{"validate", "--kind", "run", "--input", "-"}, strings.NewReader(string([]byte{255})), 2, "invalid_input", true},
		{"bounded", []string{"validate", "--kind", "run", "--input", "-"}, strings.NewReader(strings.Repeat(" ", contracts.MaxRequestBytes+1)), 2, "invalid_input", true},
		{"read-error", []string{"validate", "--kind", "run", "--input", "-"}, brokenInput{}, 1, "internal_error", false},
		{"unknown-kind", []string{"validate", "--kind", "arbitrary-schema", "--input", "-"}, strings.NewReader(valid), 2, "invalid_arguments", false},
		{"offline-option-fence", []string{"capabilities", "--offline", "--state-dir", absent}, nil, 2, "invalid_arguments", false},
		{"validate-option-fence", []string{"validate", "--kind", "run", "--input", "-", "--expected-instance-id", "id"}, strings.NewReader(valid), 2, "invalid_arguments", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output, diagnostics bytes.Buffer
			exit := Run(test.args, test.reader, &output, &diagnostics)
			var response contracts.Response
			if json.Unmarshal(output.Bytes(), &response) != nil || exit != test.exit || response.RequestID != nil || diagnostics.Len() != 0 {
				t.Fatalf("exit=%d response=%s stderr=%s", exit, output.Bytes(), diagnostics.Bytes())
			}
			if test.code != "" && (response.Error == nil || response.Error.Code != test.code) {
				t.Fatalf("fault changed: %s", output.Bytes())
			}
			if (response.Data != nil) != test.hasData {
				t.Fatalf("incorrect result vs transport failure: %s", output.Bytes())
			}
		})
	}
	var output, diagnostics bytes.Buffer
	if Run([]string{"capabilities", "--offline"}, nil, &output, &diagnostics) != 0 {
		t.Fatal(output.String())
	}
	var response struct {
		Data struct {
			Source  resources.CapabilitySource  `json:"capability_source"`
			Catalog resources.CapabilityCatalog `json:"capability_catalog"`
		} `json:"data"`
	}
	if json.Unmarshal(output.Bytes(), &response) != nil || response.Data.Source.Kind != "executable" || len(response.Data.Source.ExecutableSHA256) != 64 || response.Data.Source.InstanceID != "" || len(response.Data.Catalog.ValidationKinds) != 3 {
		t.Fatal(output.String())
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("static operation created the state root", err)
	}
}
