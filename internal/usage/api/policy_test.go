package api

import "testing"

func TestUsageAuthorizationPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.Validate() != nil {
		t.Fatal("default invalid")
	}
	for _, root := range []string{`C:\Users\account`, `C:\Users\account\.codex`, `C:\logs\*`, `C:\sessions\2026\10\21\..\auth`, `C:\sessions\2026\10\21\`} {
		if SafeRoot(root) {
			t.Fatal("unsafe root", root)
		}
	}
	root := `C:\Users\account\.codex\sessions\2026\10\21`
	if !SafeRoot(root) {
		t.Fatal("leaf rejected")
	}
	if p.Allows("env", "host", root) {
		t.Fatal("default grants access")
	}
}
