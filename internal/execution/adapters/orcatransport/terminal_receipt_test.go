package orcatransport

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"os"
	"path/filepath"
)

func TestExtensionStateKeepsPreparationSeparateFromCodexProof(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			l, _, s := fixture(t)
			s.Ref.Agent.Name = agent
			s.Session.Agent.Name = agent
			l.setup.Snapshot = s
			l.profile.Terminal = &TerminalConfig{Agent: agent, ExecutionID: s.Session.ExecutionID, SessionID: "native-original", AccessMode: "full_access", ReceiptVersion: 3}
			file, source, tools := "pi-evidence.mjs", "pi-extension-v1", []string{"read", "write", "edit", "powershell"}
			if agent == "dsh" {
				file, source, tools = "dsh-managed.mjs", "dsh-plugin-v1", []string{"powershell"}
			}
			body, _ := terminalAssets.ReadFile("extensions/" + file)
			l.profile.Terminal.EvidenceSHA256 = fmt.Sprintf("%x", sha256.Sum256(body))
			l.profile.Terminal.BundleSHA256 = TerminalExtensionDigest()
			ready := TerminalReceipt{Version: 3, Source: source, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(body)), RunID: s.Session.ExecutionID, SessionID: "native-original", NativeSessionID: "native-original", InstanceID: "original-load", ReasoningEffort: "low", PID: 1234, ProcessStartedAt: time.Now().UTC(), Mode: "tui", Tools: tools, Stage: "ready", Preparation: TerminalPreparation{Verified: true, Contract: terminaltools.Contract, AccessMode: "full_access", InsideWriteVerified: true, ProviderEnvironmentAbsent: true, OutsideWriteVerified: testBool(true), AuthenticationExpiresAt: time.Now().Add(time.Hour)}}
			state := State{Process: s.Ref.External, TerminalReadiness: &ready, TerminalOwner: &TerminalOwner{}}
			turn := Turn{Command: strings.Repeat("a", 32), Prompt: "original\r\ninput\r\n", InputDigest: digest([]byte("original\r\ninput\r\n")), WireFormat: extensionWireCommand, Native: NativeProof{Agent: agent}}
			state.Turns = []Turn{turn}
			check := func(want bool) {
				s.Implementation, _ = l.envelope(s, state)
				_, err := l.state(s)
				if (err == nil) != want {
					t.Fatal("wrong native state decision", err)
				}
			}
			check(true) // A durable prepared input has no synthetic Orca/Codex pane.
			l.profile.Terminal.Model = "requested-model"
			if l.validateTerminalReceipt(ready) == nil {
				t.Fatal("different native model accepted")
			}
			ready.Model = "requested-model"
			if agent == "dsh" {
				ready.Model = "deepseek-official/requested-model"
			}
			if err := l.validateTerminalReceipt(ready); err != nil {
				t.Fatal(err)
			}
			l.profile.Terminal.Model = ""
			state.Turns[0].Attempted, state.Turns[0].Accepted = true, true
			check(true)
			fact := ready
			fact.Stage = "settled"
			fact.CommandID = turn.Command
			fact.WireSHA256 = digest([]byte(wirePrompt(&turn)))
			fact.InputSHA256 = digest([]byte("original\ninput"))
			fact.NativeInputID = "user-1"
			fact.NativeReplyID = "reply-1"
			fact.Outcome = "completed"
			fact.StopReason = "stop"
			fact.Reply = "full native reply"
			fact.ReplySHA256 = digest([]byte(fact.Reply))
			state.Turns[0].Extension = &fact
			state.Turns[0].Started, state.Turns[0].Settled = true, true
			state.Turns[0].Reply, state.Turns[0].ReplyDigest = fact.Reply, fact.ReplySHA256
			check(true)
			fact.InstanceID = "replacement-load"
			check(false)
			fact.InstanceID = ready.InstanceID
			fact.StopReason = "max_tokens"
			check(false)
			fact.StopReason = "stop"
			state.Turns[0].Extension = nil
			check(false)
		})
	}
}

func TestTerminalReceiptRejectsMissingUnboundedAndUnknownFields(t *testing.T) {
	dir := t.TempDir()
	if err := readTerminalFile(dir, "missing", &TerminalReceipt{}); err == nil {
		t.Fatal("missing native receipt accepted")
	}
	if e := os.WriteFile(filepath.Join(dir, "oversized.json"), []byte(strings.Repeat("x", 65537)), 0600); e != nil {
		t.Fatal(e)
	}
	if e := readTerminalFile(dir, "oversized", &TerminalReceipt{}); e == nil {
		t.Fatal("oversized native receipt accepted")
	}
	if e := os.WriteFile(filepath.Join(dir, "extra.json"), []byte(`{"version":1,"unrecognized_evidence":true}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := readTerminalFile(dir, "extra", &TerminalReceipt{}); e == nil {
		t.Fatal("unknown native evidence accepted")
	}
}

func TestManagedReadinessRequiresEffectiveThinkingForNewSources(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		l, _, s := fixture(t)
		l.profile.Terminal = &TerminalConfig{Agent: agent, ReceiptVersion: 3, ExecutionID: s.Session.ExecutionID, SessionID: "native-original", AccessMode: "full_access", ReasoningEffort: "high"}
		kind, file, tools := "pi-extension-v1", "pi-evidence.mjs", []string{"read", "write", "edit", "powershell"}
		if agent == "dsh" {
			kind, file, tools = "dsh-plugin-v1", "dsh-managed.mjs", []string{"powershell"}
		}
		source, _ := terminalAssets.ReadFile("extensions/" + file)
		l.profile.Terminal.EvidenceSHA256 = fmt.Sprintf("%x", sha256.Sum256(source))
		l.profile.Terminal.BundleSHA256 = TerminalExtensionDigest()
		ready := TerminalReceipt{Version: 3, Source: kind, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(source)), RunID: s.Session.ExecutionID, SessionID: "native-original", NativeSessionID: "native-original", InstanceID: "load", PID: 1234, ProcessStartedAt: time.Now(), Mode: "tui", Tools: tools, Stage: "ready", Preparation: TerminalPreparation{Verified: true, Contract: terminaltools.Contract, AccessMode: "full_access", InsideWriteVerified: true, ProviderEnvironmentAbsent: true, OutsideWriteVerified: testBool(true)}}
		for _, effort := range []string{"", "low", "high"} {
			ready.ReasoningEffort = effort
			e := l.validateTerminalReceipt(ready)
			if (e == nil) != (effort == "high") {
				t.Fatal(agent, effort, e)
			}
		}
		// Old frozen source retains its old receipt contract after Runner updates.
		l.profile.Terminal.ReceiptVersion = 0
		ready.Version = 1
		ready.Preparation.Contract = "windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never"
		ready.Preparation.OutsideWriteDenied = testBool(false)
		ready.Preparation.OutsideWriteVerified = nil
		ready.ReasoningEffort = ""
		if e := l.validateTerminalReceipt(ready); e != nil {
			t.Fatal(e)
		}
	}
}

func testBool(v bool) *bool { return &v }
