package orcatransport

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
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
		{`{"contract":"` + terminaltools.Contract + `","access_mode":"full_access","outside_write_verified":true}`, true},
		{`{"contract":"` + terminaltools.Contract + `","access_mode":"full_access","outside_write_verified":true,"outside_write_denied":null}`, false},
		{`{"contract":"windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never","access_mode":"full_access","outside_write_denied":false}`, true},
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

func TestLegacySandboxReceiptIsPassiveStopEvidenceOnly(t *testing.T) {
	l, _, s := fixture(t)
	l.profile.Terminal = &TerminalConfig{Agent: "pi", ReceiptVersion: 2, ExecutionID: s.Session.ExecutionID, SessionID: "original", AccessMode: "sandbox", EvidenceSHA256: strings.Repeat("a", 64), BundleSHA256: strings.Repeat("b", 64)}
	ready := TerminalReceipt{Version: 2, Source: "pi-extension-v1", SourceSHA256: strings.Repeat("a", 64), RunID: s.Session.ExecutionID, SessionID: "original", InstanceID: "load", PID: 12, ProcessStartedAt: time.Now(), Mode: "tui", Tools: []string{"read", "write", "edit", "powershell"}, ReasoningEffort: "low", Preparation: TerminalPreparation{Verified: true, Contract: "windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never", AccessMode: "sandbox", InsideWriteVerified: true, ProviderEnvironmentAbsent: true, OutsideWriteDenied: testBool(true)}}
	if err := l.validateTerminalReceipt(ready); err == nil {
		t.Fatal("legacy sandbox qualified a current execution")
	}
	l.stopOnly = true
	if err := l.validateTerminalReceipt(ready); err != nil {
		t.Fatal("original stop evidence lost", err)
	}
	if _, _, err := l.Execute(context.Background(), b.LifecycleRequest{}); err == nil {
		t.Fatal("stop-only adapter executes business work")
	}
	if _, _, err := l.Reconcile(context.Background(), b.LifecycleRequest{}); err == nil {
		t.Fatal("stop-only adapter reconciles business work")
	}
	ready.Preparation.OutsideWriteDenied = testBool(false)
	if err := l.validateTerminalReceipt(ready); err == nil {
		t.Fatal("wrong original permission evidence accepted")
	}
}
