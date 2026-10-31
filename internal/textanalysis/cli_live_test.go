//go:build live && windows

package textanalysis

import (
	"context"
	"flag"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"testing"
)

var liveCLI = flag.String("parser-cli", "", "existing native Codex executable")
var liveModel = flag.String("parser-model", "", "model already available under the existing login")

func TestCLIParserLive(t *testing.T) {
	if *liveCLI == "" {
		t.Fatal("-parser-cli must name the existing native executable")
	}
	c := ra.DefaultConfig().CodexParser
	c.Executable = *liveCLI
	c.Model = *liveModel
	c.ReasoningEffort = "low"
	t.Logf("configured model=%q", c.Model)
	var a Analyzer
	body, err := a.Analyze(context.Background(), Request{Text: "Audit and necessary validation completed. Zero repairs. No remaining P0-P2 issues. Both lists are explicitly empty.", Instruction: "Organize only the given audit output.", Schema: ra.ParserSchema()}, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ra.NormalizeParserReport(body); err != nil {
		t.Fatal("audit candidate invalid", err)
	}
	t.Log("existing-login CLI analysis returned a valid audit candidate")
}
