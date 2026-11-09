package api

import (
	"strings"
	"testing"
)

func TestUsageDefaultAuthorizesVerifiedSessionAcrossDates(t *testing.T) {
	p := DefaultPolicy()
	host := "sha256:" + strings.Repeat("a", 64)
	for _, root := range []string{`C:\Users\account\.codex\sessions\2026\11\06`, `C:\Users\account\.codex\sessions\2026\11\09`} {
		if !p.Allows("paired", host, root) {
			t.Fatal("default denied verified date", root)
		}
		p.Enabled = false
		if p.Allows("paired", host, root) {
			t.Fatal("disabled collection allowed remote access")
		}
		p.Enabled = true
		for _, identity := range [][2]string{{"", host}, {"paired", "host"}, {"paired", "sha256:" + strings.Repeat("z", 64)}} {
			if p.Allows(identity[0], identity[1], root) {
				t.Fatal("unverified identity accepted", identity)
			}
		}
	}
	for _, root := range []string{`C:\Users\account`, `C:\Users\account\.codex`, `C:\logs\*`, `C:\sessions\2026\10\21\..\auth`, `C:\sessions\2026\10\21\`} {
		if SafeRoot(root) || p.Allows("paired", host, root) {
			t.Fatal("unsafe root accepted", root)
		}
	}
}
