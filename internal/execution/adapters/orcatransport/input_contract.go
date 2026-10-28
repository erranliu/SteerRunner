package orcatransport

import (
	"encoding/json"
	"errors"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"regexp"
)

// Read the two literal model-visible descriptions from the very extension we
// ship. A changed declaration shape fails closed instead of publishing a copy.
func terminalDescriptionParts() ([]m.InputPart, error) {
	raw, err := terminalAssets.ReadFile("extensions/dsh-managed.mjs")
	if err != nil {
		return nil, err
	}
	matches := regexp.MustCompile(`description: ("(?:[^"\\]|\\.)*")`).FindAllSubmatch(raw, -1)
	if len(matches) != 2 {
		return nil, errors.New("DSH tool description contract changed")
	}
	parts := []m.InputPart{}
	for _, match := range matches {
		var text string
		if err := json.Unmarshal(match[1], &text); err != nil {
			return nil, err
		}
		parts = append(parts, m.InputPart{Source: "native.tool_description", Owner: m.RunnerInput, Text: text})
	}
	return parts, nil
}
func InputContract() any {
	parts, err := terminalDescriptionParts()
	if err != nil {
		panic(err)
	}
	return map[string]any{"dsh_tool_descriptions": parts, "headers": "existing native-command/terminal-extension: command+input digest; native-model: original base64 JSON command/input/body digests/step/model/effort; all header bytes are Runner", "normalization": "existing CRLF to LF and trailing CR/LF removal; physical bracketed-paste control bytes are transport", "pi": "Runner adds no model instruction or tool description; original SDK tools unchanged"}
}
