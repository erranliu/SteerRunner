package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRFullBindingCLI(t *testing.T) {
	binding := contracts.ExpectedServiceBinding{Contract: contracts.ServiceBindingContract, Protocol: contracts.Protocol, InstanceID: "original", StateDir: filepath.Join(t.TempDir(), "absent"), ExecutableSHA256: strings.Repeat("a", 64), Version: "fixed", ResourceDigest: strings.Repeat("b", 64)}
	body, _ := json.Marshal(binding)
	parsed, fault := parse([]string{"inspect", "--run-id", "run", "--expected-binding", string(body)})
	if fault != nil || parsed.opts.ExpectedBinding == nil || *parsed.opts.ExpectedBinding != binding || parsed.opts.ExpectedInstanceID != binding.InstanceID {
		t.Fatal(parsed, fault)
	}
	for _, args := range [][]string{
		{"run-usage", "--run-id", "run"},
		{"run-usage-events", "--run-id", "run", "--snapshot-revision", "1"},
		{"estimate-run-cost", "--run-id", "run", "--usage-revision", "1", "--pricing-policy", "unused.json"},
	} {
		args = append(args, "--expected-binding", string(body))
		parsed, fault := parse(args)
		if fault != nil || parsed.opts.ExpectedBinding == nil || *parsed.opts.ExpectedBinding != binding || parsed.opts.ExpectedInstanceID != binding.InstanceID {
			t.Fatal("usage query lost full instance binding", args, parsed, fault)
		}
	}
	for _, argv := range [][]string{
		{"service", "start", "--expected-binding", string(body)},
		{"inspect", "--run-id", "run", "--expected-binding", "{}"},
		{"inspect", "--run-id", "run", "--expected-binding", string(body), "--expected-instance-id", "other"},
	} {
		if _, fault := parse(argv); fault == nil {
			t.Fatal("invalid binding accepted", argv)
		}
	}
	// A complete bound environment check must not create even a bootstrap in an
	// absent namespace; input semantics cannot grant implicit service startup.
	var output, diagnostics bytes.Buffer
	code := Run([]string{"environment-check", "start", "--state-dir", binding.StateDir, "--expected-binding", string(body), "--input", "-"}, strings.NewReader(`{"protocol":"steerrunner-cli/v1","request_id":"check-original","input":{}}`), &output, &diagnostics)
	if code == 0 {
		t.Fatal("absent bound service accepted action", output.String())
	}
	if _, err := os.Stat(binding.StateDir); !os.IsNotExist(err) {
		t.Fatal("bound request implicitly created namespace", err)
	}
}
