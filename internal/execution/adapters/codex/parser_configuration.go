package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// ParserConfiguration reads public native configuration without creating or
// resuming a thread. Return only connection names and an unsafe-managed-hook
// flag; configuration values (potentially secret) are never returned or logged.
func (c *HistoryClient) ParserConfiguration(ctx context.Context, cwd string) ([]string, bool, error) {
	wire, err := c.call(ctx, "config/read", map[string]any{"cwd": cwd, "includeLayers": true})
	if err != nil {
		return nil, false, err
	}
	var response struct {
		Config struct {
			MCP   map[string]json.RawMessage `json:"mcp_servers"`
			Hooks json.RawMessage            `json:"hooks"`
		} `json:"config"`
		Layers []struct {
			Name   struct{ Type string }      `json:"name"`
			Config map[string]json.RawMessage `json:"config"`
		} `json:"layers"`
	}
	if json.Unmarshal(wire, &response) != nil || response.Layers == nil {
		return nil, false, errors.New("native parser configuration provenance unavailable")
	}
	names := []string{}
	seen := map[string]bool{}
	managedHook := false
	for _, layer := range response.Layers {
		if layer.Name.Type == "user" || layer.Name.Type == "sessionFlags" {
			continue
		}
		// exec ignores the user layer. Emitting enabled=false for a user-only
		// server would create an invalid transport-less entry in that clean stack.
		var inherited map[string]json.RawMessage
		if json.Unmarshal(layer.Config["mcp_servers"], &inherited) == nil {
			for name := range inherited {
				if _, active := response.Config.MCP[name]; active && !seen[name] {
					names = append(names, name)
					seen[name] = true
				}
			}
		}
		var value any
		if json.Unmarshal(layer.Config["hooks"], &value) == nil && hasHookHandler(value) {
			managedHook = true
		}
		var notification []json.RawMessage
		if json.Unmarshal(layer.Config["notify"], &notification) == nil && len(notification) > 0 {
			managedHook = true
		}
	}
	sort.Strings(names)
	return names, managedHook, nil
}

func hasHookHandler(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		if kind, _ := v["type"].(string); kind == "command" || kind == "http" || kind == "agent" || kind == "prompt" {
			return true
		}
		for _, child := range v {
			if hasHookHandler(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasHookHandler(child) {
				return true
			}
		}
	}
	return false
}
