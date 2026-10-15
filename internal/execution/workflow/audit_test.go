package workflow

import (
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"strings"
	"testing"
)

const auditPass = `{"outcome":"done","summary":"checked","verdict":"pass","findings":[]}`
const auditFail = `{"outcome":"done","summary":"fix needed","verdict":"changes_requested","findings":[{"severity":"P1","message":"evidenced defect"},{"severity":"P3","message":"optional suggestion"}]}`

func auditFixture() (Snapshot, StageInput) {
	p, _ := Lookup(AuditedDevelopment, 1)
	w := Snapshot{Config: &p, AuditReportCommandID: "report", StageInputs: []StageInput{{StepID: StepImplement, CommandID: "execute"}, {StepID: StepReport, CommandID: "report", SourceCommandID: "execute", SourceTurnID: "execution-turn"}}, StageResults: []StageResult{{StepID: StepImplement, CommandID: "execute", TurnID: "execution-turn", Succeeded: true, FinalVerified: true, Final: "implementation"}, {StepID: StepReport, CommandID: "report", TurnID: "report-turn", Succeeded: true, FinalVerified: true, Final: `{"outcome":"done","summary":"implemented"}`}}}
	return w, StageInput{StepID: StepAuditSpec, CommandID: "audit", SourceCommandID: "report", SourceTurnID: "report-turn"}
}

func TestSRAuditContractAndRouting(t *testing.T) {
	if _, err := DecodeReport(`{"outcome":"done","summary":"ordinary report","verdict":{},"Verdict":true,"findings":"ignored metadata"}`, StepReport); err != nil {
		t.Fatal("audit fields changed ordinary reports", err)
	}
	for name, body := range map[string]string{
		"missing_findings": `{"outcome":"done","summary":"x","verdict":"pass"}`,
		"null_findings":    `{"outcome":"done","summary":"x","verdict":"pass","findings":null}`,
		"contradiction":    strings.Replace(auditFail, "changes_requested", "pass", 1),
		"duplicate":        strings.Replace(auditPass, `"verdict":"pass"`, `"verdict":"pass","Verdict":"changes_requested"`, 1),
		"nested_alias":     strings.Replace(auditFail, `"severity":"P1"`, `"severity":"P1","Severity":"P3"`, 1),
		"unicode_findings": `{"outcome":"done","summary":"x","verdict":"pass","findings":[{"severity":"P1","message":"blocking defect"}],"findingſ":[]}`,
		"unicode_reverse":  `{"outcome":"done","summary":"x","verdict":"pass","finding\u017f":[{"severity":"P1","message":"blocking defect"}],"findings":[]}`,
		"unicode_nested":   `{"outcome":"done","summary":"x","verdict":"pass","findings":[{"severity":"P1","ſeverity":"P3","message":"blocking defect"}]}`,
		"empty":            strings.Replace(auditFail, "evidenced defect", "  ", 1),
		"path":             strings.Replace(auditFail, `"severity":"P1"`, `"path":"../outside","severity":"P1"`, 1),
		"incomplete":       `{"outcome":"needs_input","summary":"x","question":"clarify","verdict":null}`,
		"trailing":         auditPass + ` {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReport(body, StepAuditSpec); err == nil {
				t.Fatal("invalid audit accepted", body)
			}
			w, input := auditFixture()
			input.StepID = StepAuditCode
			w.Audits = []AuditRecord{{StepID: StepAuditSpec, SourceCommandID: "report", Verdict: "pass"}}
			if next := Route(w, input, reliable.Result{Succeeded: true, ContentVerified: true, Content: body}, ""); next.Next != nil || next.Publish || next.Status != "blocked" {
				t.Fatal("invalid audit passed publication gate", next)
			}
		})
	}
	for _, body := range []string{auditPass, auditFail, strings.Replace(auditPass, `[]`, `[{"severity":"P3","message":"advisory"}]`, 1)} {
		if _, err := DecodeReport(body, StepAuditCode); err != nil {
			t.Fatal(err)
		}
	}
	w, input := auditFixture()
	result := reliable.Result{Succeeded: true, ContentVerified: true, Content: auditPass, TurnID: "audit-turn"}
	next := Route(w, input, result, "")
	if next.Next == nil || next.Next.StepID != StepAuditCode || next.Next.SourceCommandID != "report" {
		t.Fatal(next)
	}
	input.StepID = StepAuditCode
	if next := Route(w, input, result, ""); next.Next != nil || next.Status != "blocked" {
		t.Fatal("skipped first audit", next)
	}
	w.Audits = []AuditRecord{{StepID: StepAuditSpec, SourceCommandID: "report", Verdict: "pass"}}
	if next := Route(w, input, result, ""); next.Next == nil || next.Next.StepID != StepPublish {
		t.Fatal(next)
	}
	input.StepID = StepAuditSpec
	result.Content = auditFail
	for count := uint64(0); count <= 2; count++ {
		w.RepairCount = count
		next := Route(w, input, result, "")
		if count < 2 {
			if !next.Repair || next.Next == nil || next.Next.StepID != StepImplement || len(next.Next.RepairFindings) != 1 {
				t.Fatal(next)
			}
		} else if next.Next != nil || next.Reason != "audit_repair_limit" {
			t.Fatal(next)
		}
	}
	w.RepairCount = 0
	input.SourceTurnID = "other"
	if next := Route(w, input, result, ""); next.Status != "blocked" || next.Next != nil {
		t.Fatal("foreign source passed", next)
	}
	input.SourceTurnID = "report-turn"
	result.Content, result.ContentVerified = auditPass, false
	if next := Route(w, input, result, ""); next.AcceptResult || next.Next != nil {
		t.Fatal("unverified audit passed", next)
	}
	result.ContentVerified, result.Succeeded = true, false
	if next := Route(w, input, result, ""); !next.RecoverOriginal || next.Next != nil {
		t.Fatal("failed native audit passed", next)
	}
	input.StepID = StepPublish
	if next := Route(w, input, result, ""); next.Next != nil || next.Publish {
		t.Fatal("publication skipped audits", next)
	}
	w, input = auditFixture()
	input.MaterialFacts = []byte(`{"spec":{"path":"frozen-spec"},"plan":{"path":"frozen-plan"}}`)
	prompt, err := RenderInputWithBody(w, input, "custom audit")
	if err != nil || !strings.Contains(prompt, "frozen-spec") || !strings.Contains(prompt, "report-turn") || !strings.Contains(prompt, "verdict") {
		t.Fatal(err, prompt)
	}
	w.StepID, w.StageInputs = StepAuditSpec, append(w.StageInputs, input)
	if continued := FeedbackInput(w); continued.SourceCommandID != input.SourceCommandID || continued.SourceTurnID != input.SourceTurnID {
		t.Fatal("audit answer lost source")
	}
}
