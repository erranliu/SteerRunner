package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeViewerDoesNotStartService(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent-runner")
	args := []string{"open-native-session", "--state-dir", dir, "--run-id", "run", "--attempt-id", "attempt", "--session-id", "session", "--generation", "generation", "--json"}
	parsed, fault := parse(args)
	if fault != nil || parsed.opts.NativeSession.RunID != "run" {
		t.Fatal(parsed, fault)
	}
	var out, diagnostic bytes.Buffer
	if exit := Run(args, strings.NewReader(""), &out, &diagnostic); exit != 4 {
		t.Fatal("viewer started service", exit, out.String())
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("viewer created Runner state", err)
	}
}
