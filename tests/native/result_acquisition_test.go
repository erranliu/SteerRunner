//go:build windows && live

package native

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/contracts"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/testkit/livetest"
	"github.com/erranliu/SteerRunner/internal/textanalysis"
	"testing"
)

func resultAcquisition(t *testing.T, m livetest.Manifest) {
	var settings contracts.CodexSettings
	if err := contracts.DecodeStrict(m.Execution.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	c := ra.DefaultConfig().CodexParser
	c.Executable = settings.Executable
	c.Model = settings.Model
	c.ReasoningEffort = "low"
	var analyzer textanalysis.Analyzer
	request := textanalysis.Request{Text: "The audit and necessary validation completed. Zero P0-P2 issues were repaired. No P0-P2 issues remain. The fixed and remaining lists are explicitly empty.", Instruction: "Organize only the supplied audit output.", Schema: ra.ParserSchema()}
	cases := []map[string]any{}
	t.Cleanup(func() {
		save(t, "scenario.json", map[string]any{"scenario": m.Scenario, "level": m.Level, "business_outcome": "parsed", "passed": !t.Failed() && len(cases) == 2, "analysis_invocations": 2, "permission_probe_calls": 0, "cases": cases})
	})
	body, err := analyzer.Analyze(context.Background(), request, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ra.NormalizeParserReport(body); err != nil {
		t.Fatal(err)
	}
	cases = append(cases, map[string]any{"case": "text-analysis", "passed": true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = analyzer.Analyze(ctx, request, c); err == nil {
		t.Fatal("cancelled parser returned a candidate")
	}
	cases = append(cases, map[string]any{"case": "cancellation", "passed": true})
}
