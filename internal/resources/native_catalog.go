package resources

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
)

func NativeCatalog() any {
	var schemas struct {
		Request  json.RawMessage `json:"request"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(Read("native-catalog.schema.json"), &schemas); err != nil {
		panic("invalid native catalog resource")
	}
	return map[string]any{
		"contract": contracts.NativeCatalogContract, "command": "native-catalog", "query_only": true, "requires_service": false,
		"queries":        []any{map[string]string{"kind": "environments"}, map[string]string{"kind": "agents", "adapter": "orca-terminal", "target_kind": "local"}, map[string]string{"kind": "agents", "adapter": "orca-terminal", "target_kind": "orca_environment"}, map[string]string{"kind": "models", "adapter": "orca-terminal", "target_kind": "local"}, map[string]string{"kind": "models", "adapter": "orca-terminal", "target_kind": "orca_environment"}, map[string]string{"kind": "models", "adapter": "codex-app-server", "target_kind": "local"}},
		"request_schema": schemas.Request, "response_schema": schemas.Response,
		"limits": map[string]int{"input_bytes": contracts.NativeCatalogInputLimit, "output_bytes": contracts.NativeCatalogOutputLimit, "timeout_ms": 30000, "cleanup_timeout_ms": 5000},
	}
}
