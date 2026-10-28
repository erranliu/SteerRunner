package codexexec

import (
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"strings"
	"testing"
)

func TestInputParserBudgetKeepsCompleteEvidence(t *testing.T) {
	final := `{"outcome":"done","summary":"audit and validation complete","fixed_count":1,"fixed_issues":[{"severity":"P1","issue":"null access","repair":"guard null","evidence":"src/task.go:42; unit check passed"}],"remaining_issues":[]}`
	p := FreezePackage(Package{Source: ra.Source{SessionID: "00000000-0000-0000-0000-000000000001", CommandID: strings.Repeat("a", 32), TurnID: "00000000-0000-0000-0000-000000000002", AttemptID: strings.Repeat("b", 32), InputVersion: 3, FinalDigest: ra.Digest([]byte(final))}, Final: final, Instruction: "Organize the original audit; retain unknown and partial facts."})
	if err := m.CheckInput(p.Prompt, p.Budget); err != nil {
		t.Fatal("valid parser path lost", err)
	}
	if p.Final != final || p.Source.FinalDigest != ra.Digest([]byte(final)) || !strings.Contains(p.Prompt, `unit check passed`) {
		t.Fatal("evidence rewritten")
	}
	t.Logf("valid repaired audit parser Runner=%d", p.Budget.RunnerBytes)
	p.Final = strings.Repeat("evidence ", 400)
	p = FreezePackage(p)
	if err := m.CheckInput(p.Prompt, p.Budget); err == nil || p.Final != strings.Repeat("evidence ", 400) {
		t.Fatal("parser silently truncated or exceeded budget", err)
	}
}
