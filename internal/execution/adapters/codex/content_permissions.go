package codexworker

import (
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
)

// Native read-only applies to shell/file tools. External MCP services have
// separate permissions, so content sessions explicitly disable every inherited
// service before thread creation/resume, retaining only Runner's question tool.
func contentConfiguration(ctx context.Context, client *codexapp.Client, cwd string) (map[string]any, error) {
	var response struct {
		Config struct {
			MCP map[string]json.RawMessage `json:"mcp_servers"`
		} `json:"config"`
	}
	if err := client.Call(ctx, "config/read", map[string]any{"cwd": cwd, "includeLayers": true}, &response); err != nil {
		return nil, err
	}
	servers := map[string]any{}
	for name := range response.Config.MCP {
		servers[name] = map[string]any{"enabled": false}
	}
	return map[string]any{"features.multi_agent": false, "features.multi_agent_v2": false, "features.apps": false, "features.plugins": false, "features.hooks": false, "web_search": "disabled", "mcp_servers": servers}, nil
}
