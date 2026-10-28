package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const InputContract = "steerrunner-model-input/v1"
const RunnerInputBytes = 1000

type InputOwner string

const CallerInput InputOwner = "caller"
const RunnerInput InputOwner = "runner"

// Closed source names are internal compile-time choices, not a registration
// API. A new instruction source must be declared here and in owner discovery.
func validInputSource(source string, owner InputOwner) bool {
	switch source {
	case "workflow.body":
		return owner == CallerInput || owner == RunnerInput
	case "project_policy.git", "project_policy.command", "project_policy.testing":
		return owner == RunnerInput
	case "caller.direct", "caller.message", "caller.instruction", "caller.material", "caller.schema", "caller.feedback", "caller.answer":
		return owner == CallerInput
	case "materials", "source_facts", "result_format", "commit", "success", "feedback.label", "publication.target", "progress", "result_file", "content.label", "content.material_frame", "content.repository", "content.schema_label", "native.developer", "native.tool_description", "native.initialization", "native.header", "native.bootstrap", "native.exception", "environment.task", "parser.boundary", "parser.instruction", "parser.format", "parser.evidence", "native.answer_frame", "legacy.wrapper", "legacy.content.label", "legacy.content.frame", "legacy.content.wrapper", "legacy.parser":
		return owner == RunnerInput
	}
	return false
}

// InputPart is an internal value, not a caller template or runtime registry.
type InputPart struct {
	Source string
	Owner  InputOwner
	Text   string
}
type InputFragment struct {
	Source string     `json:"source"`
	Owner  InputOwner `json:"owner"`
	Bytes  int        `json:"bytes"`
}
type InputBudget struct {
	Contract         string          `json:"contract"`
	SHA256           string          `json:"sha256"`
	ProvenanceSHA256 string          `json:"provenance_sha256"`
	CallerBytes      int             `json:"caller_bytes"`
	RunnerBytes      int             `json:"runner_bytes"`
	TotalBytes       int             `json:"total_bytes"`
	Fragments        []InputFragment `json:"fragments"`
}
type InputBudgetError struct {
	CallerBytes, RunnerBytes, TotalBytes int
	Sources                              []string
}

func (e *InputBudgetError) Error() string {
	return fmt.Sprintf("input_budget_exceeded: caller=%d/unlimited runner=%d/%d total=%d/unlimited UTF-8 bytes; sources=%s; candidate not sent", e.CallerBytes, e.RunnerBytes, RunnerInputBytes, e.TotalBytes, strings.Join(e.Sources, ","))
}
func (b *InputBudget) Clone() *InputBudget {
	if b == nil {
		return nil
	}
	c := *b
	c.Fragments = append([]InputFragment(nil), b.Fragments...)
	return &c
}
func inputHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func provenanceHash(b *InputBudget) string {
	raw, _ := json.Marshal(b.Fragments)
	return inputHash(string(raw))
}

// DescribeInput also supports already-frozen legacy text. It does not grant
// dispatch: CheckInput must run after native framing/normalization is complete.
func DescribeInput(parts ...InputPart) (string, *InputBudget, error) {
	b := &InputBudget{Contract: InputContract}
	var out strings.Builder
	for _, p := range parts {
		if !validInputSource(p.Source, p.Owner) || !utf8.ValidString(p.Text) {
			return "", nil, errors.New("input provenance missing or invalid UTF-8; candidate not sent")
		}
		if p.Text == "" {
			continue
		}
		out.WriteString(p.Text)
		size := len(p.Text)
		b.Fragments = append(b.Fragments, InputFragment{p.Source, p.Owner, size})
		if p.Owner == CallerInput {
			b.CallerBytes += size
		} else {
			b.RunnerBytes += size
		}
	}
	text := out.String()
	b.TotalBytes = len(text)
	b.SHA256 = inputHash(text)
	b.ProvenanceSHA256 = provenanceHash(b)
	return text, b, nil
}
func FreezeInput(parts ...InputPart) (string, *InputBudget, error) {
	text, b, err := DescribeInput(parts...)
	if err != nil {
		return text, b, err
	}
	return text, b, CheckInput(text, b)
}
func InputParts(text string, b *InputBudget) ([]InputPart, error) {
	if b == nil || b.Contract != InputContract || b.SHA256 != inputHash(text) || b.ProvenanceSHA256 != provenanceHash(b) || !utf8.ValidString(text) {
		return nil, errors.New("frozen input text/provenance changed or unavailable; candidate not sent")
	}
	parts := []InputPart{}
	offset, caller, runner := 0, 0, 0
	for _, p := range b.Fragments {
		if !validInputSource(p.Source, p.Owner) || p.Bytes <= 0 || p.Bytes > len(text)-offset {
			return nil, errors.New("invalid frozen input fragment; candidate not sent")
		}
		value := text[offset : offset+p.Bytes]
		if !utf8.ValidString(value) {
			return nil, errors.New("invalid frozen UTF-8 fragment")
		}
		parts = append(parts, InputPart{p.Source, p.Owner, value})
		offset += p.Bytes
		if p.Owner == CallerInput {
			caller += p.Bytes
		} else {
			runner += p.Bytes
		}
	}
	if offset != len(text) || caller != b.CallerBytes || runner != b.RunnerBytes || offset != b.TotalBytes {
		return nil, errors.New("frozen input accounting changed; candidate not sent")
	}
	return parts, nil
}
func CheckInput(text string, b *InputBudget) error {
	if _, err := InputParts(text, b); err != nil {
		return err
	}
	if b.RunnerBytes > RunnerInputBytes {
		sources := []string{}
		for _, p := range b.Fragments {
			sources = append(sources, p.Source)
		}
		return &InputBudgetError{b.CallerBytes, b.RunnerBytes, b.TotalBytes, sources}
	}
	return nil
}

// NativeInputParts mirrors the existing terminal's CRLF and trailing newline
// behavior while retaining ownership. It never summarizes or truncates prose.
func NativeInputParts(parts []InputPart) []InputPart {
	out := make([]InputPart, 0, len(parts))
	for _, p := range parts {
		if p.Text != "" {
			out = append(out, p)
		}
	}
	for i := range out {
		out[i].Text = strings.ReplaceAll(out[i].Text, "\r\n", "\n")
		if i+1 < len(out) && strings.HasSuffix(out[i].Text, "\r") && strings.HasPrefix(out[i+1].Text, "\n") {
			out[i].Text = strings.TrimSuffix(out[i].Text, "\r")
		}
	}
	for i := len(out) - 1; i >= 0; i-- {
		out[i].Text = strings.TrimRight(out[i].Text, "\r\n")
		if out[i].Text != "" {
			break
		}
	}
	return out
}
