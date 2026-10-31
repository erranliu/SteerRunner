package orcatransport

import (
	"encoding/json"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"path/filepath"
	"strings"
	"testing"
)

func TestOriginalTerminalOwnerNeverInfersMissingBundleFromCurrentBuild(t *testing.T) {
	l, _, _ := fixture(t)
	c := &TerminalConfig{Agent: "pi", ExecutionID: "run", SessionID: "original", ConfigurationDigest: "configuration", AccessMode: "full_access", BundleSHA256: strings.Repeat("a", 64)}
	l.profile.Terminal = c
	l.profile.RuntimeExecutable, l.profile.RuntimeSHA256 = filepath.Join(t.TempDir(), "original-helper.exe"), strings.Repeat("b", 64)
	owner := TerminalOwner{RunID: c.ExecutionID, SessionID: c.SessionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode, RuntimeSHA256: l.profile.RuntimeSHA256, ExtensionSHA256: c.BundleSHA256}
	owner.Job.Owner = b.ProcessIdentity{Kind: "os", PID: 1, Created: "0123456789abcdef", Executable: strings.ToLower(l.profile.RuntimeExecutable)}
	if !l.terminalOwnerBinding(owner) {
		t.Fatal("original frozen bundle rejected")
	}
	c.BundleSHA256 = ""
	owner.ExtensionSHA256 = TerminalExtensionDigest()
	if l.terminalOwnerBinding(owner) {
		t.Fatal("missing original bundle silently inferred from current build")
	}
}

func TestPreparationVersionsRejectMixedFieldsIncludingNull(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"contract":"` + terminaltools.Contract + `","access_mode":"full_access","verified":true,"provider_environment_absent":true}`, true},
		{`{"contract":"` + terminaltools.Contract + `","access_mode":"full_access","outside_write_verified":true,"outside_write_denied":null}`, false},
		{`{"contract":"windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never","access_mode":"full_access","outside_write_denied":false}`, false},
		{`{"contract":"windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never","access_mode":"full_access","outside_write_denied":false,"outside_write_verified":null}`, false},
	} {
		var p TerminalPreparation
		if err := json.Unmarshal([]byte(tc.raw), &p); (err == nil) != tc.valid {
			t.Fatal(tc.raw, err)
		}
	}
	for _, raw := range []string{`{"receipt_version":3,"sandbox_executable":null}`, `{"receipt_version":3,"sandbox_executable":""}`} {
		var c TerminalConfig
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			t.Fatal("new config accepted retired field", raw)
		}
	}
}
