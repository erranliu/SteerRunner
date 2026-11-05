package collect

import (
	"strings"
	"testing"
)

func TestAuditedModelSourceBindsOriginalTurn(t *testing.T) {
	const header = `{"type":"session_meta","payload":{"id":"original-provider","cwd":"C:/original"}}` + "\n"
	const model = `{"type":"turn_context","payload":{"turn_id":"original-turn","model":"reviewer","effort":"high"}}` + "\n"
	const conflicting = `{"type":"turn_context","payload":{"turn_id":"original-turn","model":"writer","effort":"medium"}}` + "\n"
	for _, source := range []string{strings.ReplaceAll(header, "original-provider", "foreign") + model, strings.ReplaceAll(header, "C:/original", "C:/replacement") + model, header + strings.ReplaceAll(model, "original-turn", "foreign"), header + model + conflicting} {
		if evidence, err := auditedModelFromSource([]byte(source), "original-provider", "C:/original", "original-turn"); err == nil || evidence.SourceDigest != "" {
			t.Fatal("unbound model evidence accepted", evidence, err)
		}
	}
	evidence, err := auditedModelFromSource([]byte(header+model), "original-provider", "C:/original", "original-turn")
	if err != nil || evidence.Choice.Model != "reviewer" || evidence.Choice.ReasoningEffort != "high" || len(evidence.SourceDigest) != 64 {
		t.Fatal(evidence, err)
	}
}

func TestAuditedModelLocalFolderCensus(t *testing.T) {
	const local = `{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"hostIds":["local"],"omittedHostIds":["runtime:paired"]}}`
	if !emptyTerminalListingForHost([]byte(local), true) || emptyTerminalListing([]byte(local)) {
		t.Fatal("local-only and complete-host checks did not remain distinct")
	}
	for _, raw := range []string{strings.Replace(local, `["local"]`, `["runtime:paired"]`, 1), strings.Replace(local, `"runtime:paired"`, `"local"`, 1), strings.Replace(local, `"truncated":false`, `"truncated":true`, 1)} {
		if emptyTerminalListingForHost([]byte(raw), true) {
			t.Fatal("incomplete local inventory qualified", raw)
		}
	}
}
