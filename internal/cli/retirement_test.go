package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSRRetiredTakeoverNeverStartsService(t *testing.T) {
	for _, args := range [][]string{{"service", "verify-takeover"}, {"service", "verify-takeover", "--candidate", os.Args[0]}, {"service", "start", "--candidate", os.Args[0]}} {
		dir := filepath.Join(t.TempDir(), "unused-state")
		args = append(args, "--state-dir", dir, "--json")
		var out, diag bytes.Buffer
		if exit := Run(args, strings.NewReader(""), &out, &diag); exit != 2 {
			t.Fatal(exit, out.String(), diag.String())
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("retired command created state or started service", err)
		}
	}
}

func TestOfflineInspectRejectsEffectsAndMissingState(t *testing.T) {
	for _, extra := range [][]string{nil, {"--refresh-pr"}, {"--expected-instance-id", "original"}, {"--expected-binding", `{}`}} {
		dir := filepath.Join(t.TempDir(), "missing")
		args := append([]string{"inspect", "--offline", "--run-id", "run", "--state-dir", dir, "--json"}, extra...)
		var out, diag bytes.Buffer
		exit := Run(args, strings.NewReader(""), &out, &diag)
		if exit == 0 || len(extra) > 0 && exit != 2 {
			t.Fatal(exit, out.String(), diag.String())
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("query created state", err)
		}
	}
}
