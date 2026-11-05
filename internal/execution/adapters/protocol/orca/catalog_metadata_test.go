package orca

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCatalogMetadataBoundaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORCA_USER_DATA_PATH", dir)
	for _, raw := range []string{`{}`, `{"runtimeId":"r","authToken":"t","transports":[{"kind":"named-pipe","endpoint":"\\\\remote\\pipe\\x"}]}`, `{"runtimeId":"r","authToken":"t","transports":[{"kind":"websocket","endpoint":"ws://localhost"}]}`, strings.Repeat("x", (1<<20)+1)} {
		if err := os.WriteFile(filepath.Join(dir, "orca-runtime.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := QueryCatalog(context.Background(), "", "agents", ""); err == nil {
			t.Fatal("unsafe metadata accepted")
		}
	}
	t.Setenv("ORCA_USER_DATA_PATH", "relative-directory")
	if _, err := CatalogEnvironments(context.Background()); err == nil {
		t.Fatal("relative metadata directory accepted")
	}
}
