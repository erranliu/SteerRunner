package codexapp

import "encoding/json"

const AskToolName = "steerline_ask"
const AskToolDescription = "Ask only necessary human decisions; group related questions, offer clear options and wait for answers."
const RejectMalformedQuestion = "Runner rejected an unsupported or malformed native question."
const RejectUnauthorizedRequest = "Runner cannot authorize this native host request."
const RejectInitializationRequest = "Initialization permits no tools or approvals."

// AskDynamicTool describes the native app-server tool used for worker questions.
func AskDynamicTool() any {
	option := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "detail": map[string]any{"type": "string"}}, "required": []string{"id", "label"}}
	question := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"id": map[string]any{"type": "string"}, "header": map[string]any{"type": "string"}, "question": map[string]any{"type": "string"}, "options": map[string]any{"type": "array", "items": option}, "allow_free_text": map[string]any{"type": "boolean"}}, "required": []string{"id", "header", "question", "options", "allow_free_text"}}
	return map[string]any{"type": "function", "name": AskToolName, "description": AskToolDescription, "inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"questions": map[string]any{"type": "array", "minItems": 1, "maxItems": 3, "items": question}}, "required": []string{"questions"}}}
}

func AskToolResponse(answers map[string][]string) map[string]any {
	data, _ := json.Marshal(map[string]any{"answers": answers})
	return map[string]any{"contentItems": []map[string]string{{"type": "inputText", "text": string(data)}}, "success": true}
}
