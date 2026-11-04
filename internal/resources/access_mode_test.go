package resources

import (
	"strings"
	"testing"
)

func TestTerminalAgentCapabilitiesDoNotRequireCodex(t *testing.T) {
	workflows := map[string]bool{}
	for _, c := range Catalog().Combinations {
		if c.Agent != "pi" {
			continue
		}
		workflows[c.Workflow.ID] = true
		if c.TargetKind != "local" || c.AccessMode != "full_access" {
			t.Fatal("retired terminal mode advertised", c)
		}
		for _, r := range c.Requirements {
			if strings.HasPrefix(r, "native_codex_") {
				t.Fatal("terminal agent requires Codex sandbox", c)
			}
		}
	}
	if len(workflows) != 2 || !workflows["execute"] || !workflows["development"] {
		t.Fatal("local Pi workflows missing or unexpected", workflows)
	}
}
