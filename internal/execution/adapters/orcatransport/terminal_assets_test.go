package orcatransport

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"testing"
)

// These historical bytes belong to receipt/owner fixtures, not current execution.
//go:embed extensions/pi-evidence.mjs extensions/pi-managed.mjs extensions/terminal-tools.mjs extensions/receipt.mjs
var historicalTerminalAssets embed.FS

func historicalTerminalAsset(t testing.TB, name string) []byte {
	t.Helper()
	raw, err := historicalTerminalAssets.ReadFile("extensions/" + name)
	if err != nil {
		t.Fatalf("read historical terminal asset %q: %v", name, err)
	}
	return raw
}

func historicalTerminalBundleDigest(t testing.TB) string {
	t.Helper()
	h := sha256.New()
	for _, name := range []string{"pi-evidence.mjs", "pi-managed.mjs", "terminal-tools.mjs", "receipt.mjs"} {
		h.Write([]byte(name))
		h.Write(historicalTerminalAsset(t, name))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
