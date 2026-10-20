package resultacquisition

import (
	"encoding/json"
	"testing"
)

func TestContentOutputFormat(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"$ref":"#/$defs/name"}},"additionalProperties":false,"$defs":{"name":{"type":"string"}}}`)
	for _, tc := range []struct {
		text string
		ok   bool
	}{{`{"name":"test"}`, true}, {`{"name":1}`, false}, {`{`, false}, {"```json\n{}\n```", false}, {`{} {}`, false}, {`null`, false}} {
		if _, err := ValidateOutput(schema, tc.text); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.text, err)
		}
	}
	for _, bad := range []string{`{"$ref":"https://example.com/x"}`, `{"$defs":{"unused":{"$ref":"file:///x"}}}`, `{"type":"wrong"}`, `{"$schema":"https://example.com/meta"}`} {
		if _, err := CompileOutputSchema(json.RawMessage(bad)); err == nil {
			t.Fatal("accepted external/invalid schema", bad)
		}
	}
}
