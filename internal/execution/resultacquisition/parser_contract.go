package resultacquisition

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ParserSchema uses nullable optional values because native strict structured
// output requires every property. Null means missing, never a fabricated zero.
func ParserSchema() []byte {
	text := `{"type":"object","additionalProperties":false,"properties":{"outcome":{"type":"string","enum":["done","needs_input","blocked"]},"summary":{"type":"string"},"question":{"type":["string","null"]},"blocker":{"anyOf":[{"type":"null"},{"type":"object","additionalProperties":false,"properties":{"category":{"type":"string","enum":["failure","unknown_external","permission","prerequisite"]},"action":{"type":"string"},"error":{"type":"string"},"known":{"type":"string"},"unknown":{"type":"string"}},"required":["category","action","error","known","unknown"]}]},"fixed_count":{"type":["integer","null"]},"fixed_issues":{"anyOf":[{"type":"null"},{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"severity":{"type":"string","enum":["P0","P1","P2"]},"issue":{"type":"string"},"repair":{"type":"string"},"evidence":{"type":"string"}},"required":["severity","issue","repair","evidence"]}}]},"remaining_issues":{"anyOf":[{"type":"null"},{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"severity":{"type":"string","enum":["P0","P1","P2"]},"issue":{"type":"string"},"reason":{"type":"string"}},"required":["severity","issue","reason"]}}]}},"required":["outcome","summary","question","blocker","fixed_count","fixed_issues","remaining_issues"]}`
	return []byte(text)
}

func NormalizeParserReport(body string) (string, error) {
	if len(body) > 131072 {
		return "", errors.New("parser result exceeds bound")
	}
	fields := resultFields{"outcome": nil, "summary": nil, "question": nil, "blocker": {"category": nil, "action": nil, "error": nil, "known": nil, "unknown": nil}, "fixed_count": nil, "fixed_issues": {"severity": nil, "issue": nil, "repair": nil, "evidence": nil}, "remaining_issues": {"severity": nil, "issue": nil, "reason": nil}}
	dec := json.NewDecoder(strings.NewReader(body))
	if err := uniqueJSON(dec, fields); err != nil {
		return "", err
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", errors.New("trailing parser content")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &object) != nil || object == nil {
		return "", errors.New("parser object missing")
	}
	for key, value := range object {
		if _, ok := fields[key]; !ok {
			return "", errors.New("undeclared parser field")
		}
		if string(value) == "null" && key != "outcome" && key != "summary" {
			delete(object, key)
		}
	}
	normalized, _ := json.Marshal(object)
	if _, err := DecodeReport(string(normalized), "audit"); err != nil {
		return string(normalized), err
	}
	return string(normalized), nil
}
