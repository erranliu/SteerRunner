package codexworker

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type parserConfigSink struct{ bytes.Buffer }

func (*parserConfigSink) Close() error { return nil }

func TestRAParserConfigurationProjection(t *testing.T) {
	for _, source := range []string{"user", "system", "cloud"} {
		t.Run(source, func(t *testing.T) {
			input := &parserConfigSink{}
			client := &HistoryClient{input: input, lines: make(chan []byte, 1), done: make(chan struct{})}
			body, _ := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"config": map[string]any{"mcp_servers": map[string]any{"z": map[string]any{"env": map[string]string{"TOKEN": "private-value"}}, "a": map[string]any{}}}, "layers": []any{map[string]any{"name": map[string]any{"type": source}, "config": map[string]any{"mcp_servers": map[string]any{"a": map[string]any{}, "z": map[string]any{}}, "hooks": map[string]any{"SessionStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "private-command"}}}}}}}}}})
			client.lines <- body
			names, unsafe, err := client.ParserConfiguration(context.Background(), "cwd")
			wantNames := "a,z"
			if source == "user" {
				wantNames = ""
			}
			if err != nil || strings.Join(names, ",") != wantNames || unsafe != (source != "user") {
				t.Fatal(names, unsafe, err)
			}
			if !strings.Contains(input.String(), "config/read") || strings.Contains(input.String(), "thread/") || strings.Contains(strings.Join(names, ","), "private") {
				t.Fatal("metadata query created a session or leaked values")
			}
		})
	}
}
