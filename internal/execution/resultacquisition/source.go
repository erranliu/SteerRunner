// Package resultacquisition interprets existing evidence. It has no dispatch,
// persistence, repository or task capabilities.
package resultacquisition

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Source struct {
	SessionID    string `json:"session_id,omitempty"`
	CommandID    string `json:"command_id"`
	TurnID       string `json:"turn_id"`
	AttemptID    string `json:"attempt_id"`
	InputVersion uint64 `json:"input_version"`
	FinalDigest  string `json:"final_digest"`
}

type Attempt struct {
	Method      string `json:"method"`
	Source      Source `json:"source"`
	CommandID   string `json:"command_id,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
	Status      string `json:"status"`
	Diagnostic  string `json:"diagnostic,omitempty"`
}

type Result struct {
	ID        string        `json:"id"`
	Contract  string        `json:"contract"`
	Source    Source        `json:"source"`
	Method    string        `json:"method"`
	Status    string        `json:"status"`
	Value     *Report       `json:"value,omitempty"`
	Missing   []string      `json:"missing,omitempty"`
	Conflicts []string      `json:"conflicts,omitempty"`
	Attempts  []Attempt     `json:"attempts,omitempty"`
	Fields    []FieldSource `json:"fields,omitempty"`
}

type FieldSource struct {
	Field   string `json:"field"`
	Source  Source `json:"source"`
	Locator string `json:"locator"`
	Method  string `json:"method"`
}

func Digest(b []byte) string { d := sha256.Sum256(b); return hex.EncodeToString(d[:]) }

// RecognizeSuccessOutput examines only an already verified native final. It
// never establishes native success, source eligibility or publication facts.
func RecognizeSuccessOutput(final, declaration string, frozenPrompt ...string) bool {
	for _, prompt := range frozenPrompt {
		if prompt != "" && strings.Contains(strings.ReplaceAll(final, "\r\n", "\n"), strings.ReplaceAll(prompt, "\r\n", "\n")) {
			return false
		}
	}
	if declaration == "" || strings.TrimSpace(declaration) != declaration || strings.ContainsAny(declaration, "\r\n") {
		return false
	}
	lines := strings.Split(strings.ReplaceAll(final, "\r\n", "\n"), "\n")
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if last < 0 || lines[last] != declaration {
		return false
	}
	fence := ""
	for _, line := range lines[:last+1] {
		t := strings.TrimLeft(line, " ")
		// Four spaces and tabs are code, never fence delimiters.
		if len(line)-len(t) > 3 || strings.HasPrefix(line, "\t") {
			continue
		}
		if fence == "" {
			if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
				c := t[0]
				n := 0
				for n < len(t) && t[n] == c {
					n++
				}
				fence = t[:n]
			}
		} else if strings.HasPrefix(t, fence) && strings.Trim(t[len(fence):], string(fence[0])+" \t") == "" {
			fence = ""
		}
	}
	return fence == ""
}
