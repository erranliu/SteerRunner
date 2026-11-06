package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"strings"
	"testing"
)

func TestExecutionConfigurationPublishedSchemas(t *testing.T) {
	source := contracts.ExecutionConfigurationSource{Version: Version, ExecutableSHA256: strings.Repeat("a", 64), ResourceDigest: Digest()}
	for _, raw := range []string{
		`{"operation":"describe","execution":{"adapter":"orca-terminal","target":{"kind":"local"},"settings":{"access_mode":"full_access","agent_launch":{"model":"chosen"}}},"workflow":{"id":"execute","version":1}}`,
		`{"operation":"derive","execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"access_mode":"full_access","model":"chosen"}},"workflow":{"id":"generate","version":1},"access_mode":"read_only"}`,
		`{"operation":"compose","selection":{"adapter":"codex-app-server","target":{"kind":"local"},"agent":"codex","model":"chosen","access_mode":"full_access"},"workflow":{"id":"development","version":2}}`,
	} {
		out, fault := ResolveExecutionConfiguration([]byte(raw), source)
		if fault != nil {
			t.Fatal(raw, fault)
		}
		body, _ := json.Marshal(out)
		if err := validateConfigurationObject(ExecutionConfiguration().(map[string]any)["response_schema"], body); err != nil {
			t.Fatal(string(body), err)
		}
		var incomplete map[string]any
		_ = json.Unmarshal(body, &incomplete)
		field := "identity"
		if out.Operation == "derive" {
			field = "source_identity"
		}
		delete(incomplete, field)
		broken, _ := json.Marshal(incomplete)
		if validateConfigurationObject(ExecutionConfiguration().(map[string]any)["response_schema"], broken) == nil {
			t.Fatal("incomplete operation response accepted", string(broken))
		}

	}
	// Explicit null preferences must not be interpreted as omitted inheritance.
	_, fault := ResolveExecutionConfiguration([]byte(`{"operation":"compose","selection":{"adapter":"codex-app-server","target":{"kind":"local"},"agent":"codex","model":null,"access_mode":"full_access"},"workflow":{"id":"development","version":2}}`), source)
	if fault == nil {
		t.Fatal("null preference accepted")
	}
}
