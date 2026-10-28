package codexworker

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"sort"
)

const ContentSessionInstruction = "Read-only: use submitted directory/frozen materials; return content. No writes, commits, push or PR. Ask via steerline_ask."
const EnvironmentSessionInstruction = "Use only steerrunner_environment_probe for the fixed nonce/actions, once each in order. Stop on failure or unknown; no other tools/business work. Reply DONE after all actions."
const BootstrapInstruction = "Initialize this agent session. Reply READY only. Do not use tools or change files."
const AuthenticationInstruction = "Authentication set; no env/credential/config/header dumps, login, auth/permission changes, escalation or uncertain publication repeats."
const EnvironmentTaskFormat = "Run environment check %s. Nonce: %s. Sequentially call %s once for each action in this exact order: %s. Stop on failure. After the last result, reply DONE. Do not perform business work."

// Session fields stay in their existing native request. This checks all Runner
// authored prose in that one request; schemas remain structured metadata.
type SessionInputFacts struct {
	Budget *m.InputBudget `json:"budget"`
	Parts  []m.InputPart  `json:"parts"`
	Fields []string       `json:"fields,omitempty"`
}

func sessionInput(params map[string]any) (*SessionInputFacts, error) {
	parts := []m.InputPart{}
	fields := []string{}
	add := func(source, field, text string) {
		parts = append(parts, m.InputPart{Source: source, Owner: m.RunnerInput, Text: text})
		fields = append(fields, field)
	}
	if value, present := params["developerInstructions"]; present && value != nil {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("unaccounted native developer instructions")
		}
		add("native.developer", "developerInstructions", text)
	}
	if value, present := params["dynamicTools"]; present && value != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var tools []any
		if err = json.Unmarshal(raw, &tools); err != nil {
			return nil, err
		}
		for index, tool := range tools {
			raw, err := json.Marshal(tool)
			if err != nil {
				return nil, err
			}
			var object map[string]any
			if err = json.Unmarshal(raw, &object); err != nil {
				return nil, err
			}
			var walk func(any, string)
			walk = func(v any, path string) {
				switch x := v.(type) {
				case map[string]any:
					keys := make([]string, 0, len(x))
					for key := range x {
						keys = append(keys, key)
					}
					sort.Strings(keys)
					for _, key := range keys {
						value := x[key]
						if key == "description" {
							if text, ok := value.(string); ok {
								add("native.tool_description", fmt.Sprintf("%s[%q]", path, key), text)
								continue
							}
						}
						walk(value, fmt.Sprintf("%s[%q]", path, key))
					}
				case []any:
					for i, child := range x {
						walk(child, fmt.Sprintf("%s[%d]", path, i))
					}
				}
			}
			walk(object, fmt.Sprintf("dynamicTools[%d]", index))
		}
	}
	_, budget, err := m.FreezeInput(parts...)
	if err != nil {
		return nil, err
	}
	return &SessionInputFacts{Budget: budget, Parts: parts, Fields: fields}, nil
}
func InputContract() any {
	return map[string]any{"initialization": NativeReadinessPrompt, "bootstrap": BootstrapInstruction, "authentication": AuthenticationInstruction, "worker": WorkerInstructionFormat, "content_session": ContentSessionInstruction, "environment_session": EnvironmentSessionInstruction, "environment_task": EnvironmentTaskFormat, "environment_slots": "check ID, nonce, fixed required action IDs; Runner", "ask_tool": codexapp.AskDynamicTool(), "environment_tool": environmentProbeTool(), "answers": "caller text; native JSON question IDs/framing are Runner; same-ask/turn binding unchanged", "native_rejections": []string{codexapp.RejectMalformedQuestion, codexapp.RejectUnauthorizedRequest, codexapp.RejectEnvironmentRequest, codexapp.RejectInitializationRequest}}
}
