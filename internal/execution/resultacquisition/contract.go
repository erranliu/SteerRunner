package resultacquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

var ErrConflict = errors.New("conflicting result facts")

type Blocker struct {
	Category string `json:"category"`
	Action   string `json:"action"`
	Error    string `json:"error"`
	Known    string `json:"known,omitempty"`
	Unknown  string `json:"unknown,omitempty"`
}

type Outputs struct {
	PRURL  string `json:"pr_url,omitempty"`
	Branch string `json:"branch,omitempty"`
}

type FixedIssue struct {
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Repair   string `json:"repair"`
	Evidence string `json:"evidence"`
}
type RemainingIssue struct {
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Reason   string `json:"reason"`
}
type Report struct {
	reportBody
	FixedCount      *int             `json:"fixed_count,omitempty"`
	FixedIssues     []FixedIssue     `json:"fixed_issues"`
	RemainingIssues []RemainingIssue `json:"remaining_issues"`
}

type reportBody struct {
	Outcome  string   `json:"outcome"`
	Outputs  Outputs  `json:"outputs,omitempty"`
	Summary  string   `json:"summary"`
	Question string   `json:"question,omitempty"`
	Blocker  *Blocker `json:"blocker,omitempty"`
}

// Completion constructs a program-derived candidate. External model/agent
// JSON still passes DecodeReport; an existing final is retained at its source.
func Completion(summary string) Report {
	return Report{reportBody: reportBody{Outcome: "done", Summary: summary}}
}

const ResultContract = `Return one JSON object:
{"outcome":"done|needs_input|blocked","summary":"result"}
For needs_input add question. For blocked add blocker: {"category":"failure|unknown_external|permission|prerequisite","action":"action","error":"error","known":"optional","unknown":"optional"}.`

func ResultInstruction(contract string) string {
	switch contract {
	case "audit":
		return ResultContract + `
Include fixed_count (nonnegative integer), fixed_issues [{"severity":"P0|P1|P2","issue":"issue","repair":"actual repair","evidence":"existing location/change evidence"}], and remaining_issues [{"severity":"P0|P1|P2","issue":"issue","reason":"reason"}]. Explicitly use 0 and [] only when known. done requires completed audit and necessary validation, no remaining P0-P2 issues. Preserve incomplete work and partial statistics. Never invent missing facts.`
	case "publication":
		return ResultContract + `
For done include outputs: {"pr_url":"actual PR URL"}.`
	default:
		return ResultContract
	}
}

// Reject duplicate keys at every depth before decoding. encoding/json normally
// accepts last-wins objects and case-insensitive struct fields, which is
// unsuitable for a first-wins result.
type resultFields map[string]resultFields

var reportFields = resultFields{
	"outcome": nil, "summary": nil, "question": nil,
	"outputs": {"pr_url": nil, "branch": nil},
	"blocker": {"category": nil, "action": nil, "error": nil, "known": nil, "unknown": nil},
}

func uniqueJSON(dec *json.Decoder, fields resultFields) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			// Match the struct decoder's aliases only for fields consumed here.
			// Distinct keys in ignored metadata remain valid JSON.
			var child resultFields
			for field, nested := range fields {
				if strings.EqualFold(name, field) {
					name, child = field, nested
					break
				}
			}
			if !ok || seen[name] {
				return errors.New("duplicate or invalid result key")
			}
			seen[name] = true
			if err := uniqueJSON(dec, child); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueJSON(dec, fields); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = dec.Token()
	return err
}

func DecodeReport(body, contract string) (Report, error) {
	var value Report
	if len(body) == 0 || len(body) > 128*1024 {
		return value, errors.New("result missing or exceeds bound")
	}
	// A complete code fence is presentation. Do not extract JSON from prose
	// or select one of several objects, which could change the intended outcome.
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	for _, prefix := range []string{"```json\n", "```\n"} {
		if strings.HasPrefix(body, prefix) && strings.HasSuffix(body, "\n```") {
			body = strings.TrimSuffix(strings.TrimPrefix(body, prefix), "\n```")
			break
		}
	}
	check := json.NewDecoder(strings.NewReader(body))
	fields := reportFields
	if contract == "audit" {
		fields = resultFields{}
		for k, v := range reportFields {
			fields[k] = v
		}
		fields["fixed_count"] = nil
		fields["fixed_issues"] = resultFields{"severity": nil, "issue": nil, "repair": nil, "evidence": nil}
		fields["remaining_issues"] = resultFields{"severity": nil, "issue": nil, "reason": nil}
	}
	if !exactContractFields([]byte(body), fields) {
		return value, errors.New("unknown, aliased or null result field")
	}
	if err := uniqueJSON(check, fields); err != nil {
		return value, err
	}
	if _, err := check.Token(); err != io.EOF {
		return value, errors.New("trailing result content")
	}
	dec := json.NewDecoder(bytes.NewBufferString(body))
	// Reject undeclared fields; a parser cannot smuggle a second conclusion.
	dec.DisallowUnknownFields()
	var target any = &value
	if err := dec.Decode(target); err != nil {
		return value, err
	}
	value.Outcome = strings.TrimSpace(value.Outcome)
	value.Summary = strings.TrimSpace(value.Summary)
	value.Question = strings.TrimSpace(value.Question)
	value.Outputs.PRURL = strings.TrimSpace(value.Outputs.PRURL)
	if strings.TrimSpace(value.Summary) == "" {
		return value, errors.New("result summary missing")
	}
	if value.Outputs.PRURL != "" {
		u, err := url.Parse(value.Outputs.PRURL)
		if err != nil || u.Host == "" || u.User != nil || u.Scheme != "https" && u.Scheme != "http" {
			return value, errors.New("invalid PR URL")
		}
	}
	switch value.Outcome {
	case "done":
		if value.Question != "" || value.Blocker != nil || contract == "publication" && value.Outputs.PRURL == "" {
			return value, errors.New("done result has inconsistent fields")
		}
	case "needs_input":
		if strings.TrimSpace(value.Question) == "" || value.Blocker != nil {
			return value, errors.New("needs_input requires its question")
		}
	case "blocked":
		if value.Question != "" || value.Blocker == nil || value.Blocker.Action == "" || value.Blocker.Error == "" {
			return value, errors.New("blocked requires action and error")
		}
		switch value.Blocker.Category {
		case "failure", "unknown_external", "permission", "prerequisite":
		default:
			return value, errors.New("unsupported blocker category")
		}
	default:
		return value, errors.New("unsupported result outcome")
	}
	if contract == "audit" {
		if value.FixedCount == nil || value.FixedIssues == nil || value.RemainingIssues == nil {
			if value.Outcome == "done" {
				return value, errors.New("audit completion requires explicit repair counts and lists")
			}
		}
		if value.FixedCount != nil && (*value.FixedCount < 0 || *value.FixedCount != len(value.FixedIssues)) {
			return value, fmt.Errorf("%w: audit count conflicts with actual repairs", ErrConflict)
		}
		seen := map[string]bool{}
		for _, issue := range value.FixedIssues {
			key := issue.Issue + "\x00" + issue.Repair + "\x00" + issue.Evidence
			if !blockingSeverity(issue.Severity) || strings.TrimSpace(issue.Issue) == "" || strings.TrimSpace(issue.Repair) == "" || strings.TrimSpace(issue.Evidence) == "" || seen[key] {
				return value, errors.New("invalid or duplicate actual repair")
			}
			seen[key] = true
		}
		for _, issue := range value.RemainingIssues {
			if !blockingSeverity(issue.Severity) || strings.TrimSpace(issue.Issue) == "" || strings.TrimSpace(issue.Reason) == "" {
				return value, errors.New("invalid remaining issue")
			}
		}
		if value.Outcome == "done" && len(value.RemainingIssues) > 0 {
			return value, fmt.Errorf("%w: done audit retains blocking issues", ErrConflict)
		}
	}
	return value, nil
}
func blockingSeverity(s string) bool { return s == "P0" || s == "P1" || s == "P2" }

func exactContractFields(data []byte, fields resultFields) bool {
	var value any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if dec.Decode(&value) != nil {
		return false
	}
	return exactContractValue(value, fields)
}
func exactContractValue(value any, fields resultFields) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			nested, ok := fields[key]
			if !ok {
				return false
			}
			if nested != nil {
				if !exactContractValue(child, nested) {
					return false
				}
			} else {
				if child == nil {
					return false
				}
				switch child.(type) {
				case map[string]any, []any:
					return false
				}
			}
		}
	case []any:
		for _, child := range v {
			if !exactContractValue(child, fields) {
				return false
			}
		}
	default:
		return false
	}
	return true
}
