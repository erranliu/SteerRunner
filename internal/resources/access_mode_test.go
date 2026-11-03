package resources

import (
	"strings"
	"testing"
)

func TestTerminalAgentCapabilitiesDoNotRequireCodex(t *testing.T) {
	count := 0
	for _, c := range Catalog().Combinations {
		if c.Agent != "pi" {
			continue
		}
		count++
		if c.AccessMode != "full_access" {
			t.Fatal("retired terminal mode advertised", c)
		}
		for _, r := range c.Requirements {
			if strings.HasPrefix(r, "native_codex_") {
				t.Fatal("terminal agent requires Codex sandbox", c)
			}
		}
	}
	if count != 1 {
		t.Fatal("terminal capabilities missing", count)
	}
}
