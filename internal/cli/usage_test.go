package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUsageQueryDoesNotStartService(t *testing.T) {
	for _, args := range [][]string{{"run-usage", "--run-id", "r"}, {"run-usage-events", "--run-id", "r", "--snapshot-revision", "1"}, {"estimate-run-cost", "--run-id", "r", "--usage-revision", "1", "--pricing-policy", `C:\missing.json`}} {
		dir := filepath.Join(t.TempDir(), "not-created")
		args = append(args, "--state-dir", dir, "--json")
		var out, diag bytes.Buffer
		exit := Run(args, strings.NewReader(""), &out, &diag)
		if exit != 4 {
			t.Fatal(exit, out.String())
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("query created state directory")
		}
	}
}
func TestRunUsageContractArguments(t *testing.T) {
	for _, args := range [][]string{{"run-usage"}, {"run-usage-events", "--run-id", "r"}, {"run-usage", "--run-id", "r", "--snapshot-revision", "0"}, {"run-usage", "--run-id", "r", "--usage-revision", "1"}, {"inspect", "--run-id", "r", "--snapshot-revision", "1"}, {"estimate-run-cost", "--run-id", "r", "--usage-revision", "1"}, {"run-usage", "--run-id", "r", "--input", "-"}} {
		if _, err := parse(args); err == nil {
			t.Fatal("accepted malformed usage query", args)
		}
	}
}
