package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredCheckCommandsRejectBeforeCreatingState(t *testing.T) {
	for _, args := range [][]string{
		{"environment-check", "start"},
		{"environment-check", "inspect"},
		{"environment-check", "stop"},
		{"validate", "--kind", "environment-check", "--input", "-"},
		{"inspect", "--check-id", "old"},
	} {
		dir := filepath.Join(t.TempDir(), "absent")
		var out, diagnostics bytes.Buffer
		argv := append(append([]string{}, args...), "--state-dir", dir)
		if code := Run(argv, strings.NewReader(`{}`), &out, &diagnostics); code != 2 {
			t.Fatalf("retired command accepted: %v %d %s", args, code, out.String())
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("retired command created state", args, err)
		}
	}
}
