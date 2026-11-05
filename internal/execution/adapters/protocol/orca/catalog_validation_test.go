package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestOrcaCatalogRejectsInvalidIdentities(t *testing.T) {
	for _, mode := range []string{"duplicate-agent", "empty-agent", "duplicate-model", "empty-model", "duplicate-effort", "empty-effort"} {
		t.Run(mode, func(t *testing.T) {
			methods := []string{}
			call := func(_ context.Context, method string, _ any) (json.RawMessage, error) {
				methods = append(methods, method)
				var result any
				switch method {
				case "status.get":
					result = map[string]any{"runtimeProtocolVersion": 3, "capabilities": catalogCapabilities}
				case "agentSession.agents":
					agents := []map[string]string{{"agent": "codex"}}
					if mode == "duplicate-agent" {
						agents = append(agents, map[string]string{"agent": "codex"})
					} else if mode == "empty-agent" {
						agents[0]["agent"] = ""
					}
					result = map[string]any{"agents": agents}
				case "agentSession.modelCatalog":
					model := map[string]any{"id": "one", "efforts": []map[string]string{{"value": "high", "label": "High"}}}
					models := []any{model}
					switch mode {
					case "duplicate-model":
						models = append(models, map[string]any{"id": "one", "label": "Other label"})
					case "empty-model":
						model["id"] = ""
					case "duplicate-effort":
						model["efforts"] = []map[string]string{{"value": "high"}, {"value": "high", "label": "Other label"}}
					case "empty-effort":
						model["efforts"] = []map[string]string{{"value": "", "label": "Empty"}}
					}
					result = map[string]any{"models": models}
				default:
					return nil, fmt.Errorf("unexpected business method: %s", method)
				}
				return json.Marshal(result)
			}
			_, err := readCatalog(context.Background(), call, "", "models", "codex")
			if err == nil {
				t.Fatal("invalid native identity accepted", methods)
			}
			wantCalls := 3
			if mode == "duplicate-agent" || mode == "empty-agent" {
				wantCalls = 2
			}
			if len(methods) != wantCalls {
				t.Fatal("validation did not reject at the original response", methods, err)
			}
		})
	}
}
