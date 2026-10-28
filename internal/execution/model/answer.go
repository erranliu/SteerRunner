package model

import (
	"encoding/json"
	"sort"
)

// AnswerParts describes the exact JSON text used by the existing native ask
// response. Question IDs/punctuation are framing; human values are caller text.
func AnswerParts(answers map[string][]string) []InputPart {
	parts := []InputPart{{Source: "native.answer_frame", Owner: RunnerInput, Text: `{"answers":`}}
	if answers == nil {
		return append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "null}"})
	}
	parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "{"})
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		prefix := ""
		if i > 0 {
			prefix = ","
		}
		encoded, _ := json.Marshal(key)
		parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: prefix + string(encoded) + ":"})
		if answers[key] == nil {
			parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "null"})
			continue
		}
		parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "["})
		for j, value := range answers[key] {
			prefix = ""
			if j > 0 {
				prefix = ","
			}
			encoded, _ = json.Marshal(value)
			parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: prefix + `"`}, InputPart{Source: "caller.answer", Owner: CallerInput, Text: string(encoded[1 : len(encoded)-1])}, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: `"`})
		}
		parts = append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "]"})
	}
	return append(parts, InputPart{Source: "native.answer_frame", Owner: RunnerInput, Text: "}}"})
}
