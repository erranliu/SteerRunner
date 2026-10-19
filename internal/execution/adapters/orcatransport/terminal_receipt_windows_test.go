//go:build windows

package orcatransport

import (
	"context"
	"encoding/json"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalStartupFailureCanStopWithoutInventingWorker(t *testing.T) {
	if raw := os.Getenv("SR_TEST_TERMINAL_STARTUP"); raw != "" {
		var c TerminalConfig
		if json.Unmarshal([]byte(raw), &c) != nil {
			t.Fatal("invalid child config")
		}
		if err := RunTerminalAgent(context.Background(), c, strings.NewReader(""), os.Stdout, os.Stderr); err == nil {
			t.Fatal("invalid preparation succeeded")
		}
		return
	}
	l, f, s := fixture(t)
	f.commandMode = true
	l.setup.AgentLaunchCommand = "original-helper-command"
	s.Ref.Agent.Name = "pi"
	s.Session.Agent.Name = "pi"
	l.setup.Snapshot = s
	self, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	meta, e := TerminalRuntimeMetadata()
	if e != nil {
		t.Fatal(e)
	}
	c := TerminalConfig{ReceiptVersion: 2, Agent: "pi", ExecutionID: s.Session.ExecutionID, SessionID: "original-native", StateDirectory: t.TempDir(), AuthenticationState: t.TempDir(), AccessMode: "sandbox", ConfigurationDigest: s.ConfigurationDigest, BundleSHA256: meta.BundleSHA256}
	l.profile.Terminal = &c
	l.profile.RuntimeExecutable = self
	l.profile.RuntimeSHA256 = meta.RuntimeSHA256
	r, _, e := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if e != nil {
		t.Fatal(e)
	}
	bind(&s, r)
	stop := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	if r, e := l.StopLifecycle(context.Background(), stop, nil); e == nil || r.State.Stopped {
		t.Fatal("missing owner/receipt proved absence")
	}
	raw, _ := json.Marshal(c)
	cmd := exec.Command(self, "-test.run=^TestTerminalStartupFailureCanStopWithoutInventingWorker$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "SR_TEST_TERMINAL_STARTUP="+string(raw))
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	var fact TerminalStartupFailure
	if e := readTerminalFile(c.StateDirectory, "startup-exit", &fact); e != nil {
		t.Fatal(e)
	}
	if process.Probe(&fact.Owner.Job.Owner).State != "absent" {
		t.Fatal("helper remains live")
	}
	original := fact
	for _, change := range []func(*TerminalStartupFailure){func(f *TerminalStartupFailure) { f.NoWorkerStarted = false }, func(f *TerminalStartupFailure) { f.AuthenticationClosed = false }, func(f *TerminalStartupFailure) { f.Owner.RuntimeSHA256 = "changed" }, func(f *TerminalStartupFailure) { f.Owner.Job.Owner.Created = "changed" }, func(f *TerminalStartupFailure) { f.Owner.Job.Handle = 1 }} {
		fact = original
		change(&fact)
		body, _ := json.Marshal(fact)
		os.WriteFile(filepath.Join(c.StateDirectory, "startup-exit.json"), body, 0600)
		if _, e := l.startupFailureOwner(); e == nil {
			t.Fatal("changed/incomplete startup exit accepted")
		}
	}
	raw, _ = json.Marshal(original)
	os.WriteFile(filepath.Join(c.StateDirectory, "startup-exit.json"), raw, 0600)
	r, e = l.StopLifecycle(context.Background(), stop, nil)
	if e != nil || !r.State.Stopped || !r.State.WorkerAbsent {
		t.Fatal("known pre-worker failure could not stop", r, e)
	}
	closes := 0
	for _, call := range f.calls {
		if len(call) > 1 && call[0] == "terminal" && call[1] == "close" {
			closes++
		}
	}
	if closes != 1 {
		t.Fatal("wrong close count", closes)
	}
}

func TestManagedNativeChannelRejectsMissingChangedAndLateFacts(t *testing.T) {
	l, _, snap := fixture(t)
	meta, e := TerminalRuntimeMetadata()
	if e != nil {
		t.Fatal(e)
	}
	self, _ := os.Executable()
	c := TerminalConfig{ReceiptVersion: 2, Agent: "pi", ExecutionID: snap.Session.ExecutionID, SessionID: "original-native", StateDirectory: t.TempDir(), AccessMode: "sandbox", ConfigurationDigest: snap.ConfigurationDigest, BundleSHA256: meta.BundleSHA256, EvidenceSHA256: meta.PiSHA256, Model: "", ReasoningEffort: ""}
	l.profile.Terminal = &c
	l.profile.RuntimeExecutable = self
	l.profile.RuntimeSHA256 = meta.RuntimeSHA256
	cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
	owned, e := process.New(time.Minute).StartManaged(cmd)
	if e != nil {
		t.Fatal(e)
	}
	defer owned.Close()
	defer cmd.Wait()
	defer owned.Kill()
	defer l.closeTerminalJob()
	job, e := process.DescribeOwnedJob(owned)
	if e != nil {
		t.Fatal(e)
	}
	owner := TerminalOwner{Job: job, RuntimeSHA256: meta.RuntimeSHA256, ExtensionSHA256: meta.BundleSHA256, RunID: c.ExecutionID, SessionID: c.SessionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode}
	ready := TerminalReceipt{Version: 2, Source: "pi-extension-v1", SourceSHA256: meta.PiSHA256, RunID: c.ExecutionID, SessionID: c.SessionID, InstanceID: "original-load", PID: job.Worker.PID, ProcessStartedAt: time.Now().UTC(), Mode: "tui", Tools: []string{"read", "write", "edit", "powershell"}, Model: "test/test", ReasoningEffort: "low", Stage: "ready", Preparation: TerminalPreparation{Verified: true, Contract: terminaltools.Contract, AccessMode: "sandbox", InsideWriteVerified: true, ProviderEnvironmentAbsent: true, OutsideWriteDenied: true, AuthenticationExpiresAt: time.Now().Add(time.Hour)}}
	turn := Turn{Command: strings.Repeat("c", 32), Prompt: "original input", InputDigest: digest([]byte("original input")), Attempted: true, Accepted: true, WireFormat: extensionWireCommand, Native: NativeProof{Agent: "pi"}}
	ready.ActiveCommandID = turn.Command
	fact := ready
	fact.Stage = "settled"
	fact.CommandID = turn.Command
	fact.InputSHA256 = turn.InputDigest
	fact.WireSHA256 = digest([]byte(wirePrompt(&turn)))
	fact.NativeInputID = "user-original"
	fact.NativeReplyID = "assistant-original"
	fact.Outcome = "completed"
	fact.StopReason = "stop"
	fact.Reply = "complete body"
	fact.ReplySHA256 = digest([]byte(fact.Reply))
	write := func(name string, value any) {
		t.Helper()
		raw, _ := json.Marshal(value)
		if e := process.New(0).WriteExecutionFile(filepath.Join(c.StateDirectory, name+".json"), raw); e != nil {
			t.Fatal(e)
		}
	}
	for _, kind := range []string{"missing", "oversized", "wrong-source", "wrong-session", "wrong-native-session", "changed-preparation", "wrong-command", "wrong-instance", "wrong-input", "wrong-wire", "wrong-model", "wrong-effort", "bad-final-hash", "bad-failure", "pending", "valid"} {
		t.Run(kind, func(t *testing.T) {
			write("owner", owner)
			write("readiness", ready)
			candidate := fact
			switch kind {
			case "wrong-source":
				candidate.SourceSHA256 = strings.Repeat("0", 64)
			case "wrong-native-session":
				candidate.NativeSessionID = "foreign-native-session"
			case "changed-preparation":
				candidate.Preparation.AuthenticationExpiresAt = candidate.Preparation.AuthenticationExpiresAt.Add(time.Minute)
			case "wrong-session":
				candidate.SessionID = "replacement"
			case "wrong-command":
				candidate.CommandID = strings.Repeat("d", 32)
			case "wrong-instance":
				candidate.InstanceID = "replacement"
			case "wrong-input":
				candidate.InputSHA256 = strings.Repeat("0", 64)
			case "wrong-wire":
				candidate.WireSHA256 = strings.Repeat("0", 64)
			case "wrong-model":
				candidate.Model = "changed"
			case "wrong-effort":
				candidate.ReasoningEffort = "high"
			case "bad-failure":
				candidate.Outcome = "error"
				candidate.NativeInputID = ""
			case "bad-final-hash":
				candidate.ReplySHA256 = strings.Repeat("0", 64)
			case "pending":
				candidate.Stage = "started"
			}
			write(turn.Command, candidate)
			if kind == "missing" {
				os.Remove(filepath.Join(c.StateDirectory, turn.Command+".json"))
			}
			if kind == "oversized" {
				os.WriteFile(filepath.Join(c.StateDirectory, turn.Command+".json"), []byte(strings.Repeat("x", 65537)), 0600)
			}
			state := State{Turns: []Turn{turn}}
			current := &state.Turns[0]
			diagnostic := l.observeTerminal(&state, current)
			if kind == "bad-failure" && l.terminalFailure(state, *current) != nil {
				t.Fatal("malformed failure became authoritative")
			}
			if kind == "valid" {
				if diagnostic != "" || !current.Settled || current.Reply != fact.Reply {
					t.Fatal(diagnostic, current)
				}
				candidate.Reply = "changed after settlement"
				candidate.ReplySHA256 = digest([]byte(candidate.Reply))
				write(turn.Command, candidate)
				if !strings.Contains(l.observeTerminal(&state, current), "changed") {
					t.Fatal("frozen result mutation accepted")
				}
			} else if current.Settled || kind != "pending" && kind != "missing" && diagnostic == "" {
				t.Fatal("incomplete/foreign native evidence accepted", kind, diagnostic)
			}
		})
	}
	// A native final may exist, but it cannot invent a lost Orca acceptance.
	write(turn.Command, fact)
	state := State{Turns: []Turn{turn}}
	state.Turns[0].Accepted = false
	if d := l.observeTerminal(&state, &state.Turns[0]); !strings.Contains(d, "acceptance unresolved") || state.Turns[0].Started || state.Turns[0].Settled {
		t.Fatal("native final promoted unknown delivery", d)
	}
}

func TestManagedStopWaitsForOriginalExitAndAuthenticationCleanup(t *testing.T) {
	if raw := os.Getenv("SR_TEST_TERMINAL_OWNER"); raw != "" {
		var c TerminalConfig
		if json.Unmarshal([]byte(raw), &c) != nil {
			t.Fatal("child config")
		}
		meta, e := TerminalRuntimeMetadata()
		if e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
		owned, e := process.New(time.Minute).StartManaged(cmd)
		if e != nil {
			t.Fatal(e)
		}
		defer owned.Close()
		ref, e := process.DescribeOwnedJob(owned)
		if e != nil {
			owned.Kill()
			cmd.Wait()
			t.Fatal(e)
		}
		owner := TerminalOwner{Job: ref, RuntimeSHA256: meta.RuntimeSHA256, ExtensionSHA256: meta.BundleSHA256, RunID: c.ExecutionID, SessionID: c.SessionID, ConfigurationDigest: c.ConfigurationDigest, AccessMode: c.AccessMode}
		body, _ := json.Marshal(owner)
		if e = process.New(0).WriteExecutionFile(filepath.Join(c.StateDirectory, "owner.json"), body); e != nil {
			owned.Kill()
			cmd.Wait()
			t.Fatal(e)
		}
		_ = cmd.Wait()
		owned.Kill()
		tree := owned.(b.ProcessTreeHandle)
		for until := time.Now().Add(2 * time.Second); !tree.WritersAbsent() && time.Now().Before(until); time.Sleep(10 * time.Millisecond) {
		}
		if !tree.WritersAbsent() {
			t.Fatal("child writers remain")
		}
		// This is the interval PTY close used to interrupt.
		time.Sleep(150 * time.Millisecond)
		closed := true
		body, _ = json.Marshal(TerminalExit{Version: 1, Owner: owner, WritersAbsent: true, AuthenticationClosed: &closed, ClosedAt: time.Now().UTC()})
		if e = process.New(0).WriteExecutionFile(filepath.Join(c.StateDirectory, "exit.json"), body); e != nil {
			t.Fatal(e)
		}
		return
	}
	l, f, s := fixture(t)
	f.commandMode = true
	l.setup.AgentLaunchCommand = "original-helper-command"
	s.Ref.Agent.Name = "pi"
	s.Session.Agent.Name = "pi"
	l.setup.Snapshot = s
	self, _ := os.Executable()
	meta, e := TerminalRuntimeMetadata()
	if e != nil {
		t.Fatal(e)
	}
	c := TerminalConfig{ReceiptVersion: 2, Agent: "pi", ExecutionID: s.Session.ExecutionID, SessionID: "original-native", StateDirectory: t.TempDir(), AccessMode: "sandbox", ConfigurationDigest: s.ConfigurationDigest, BundleSHA256: meta.BundleSHA256}
	l.profile.Terminal = &c
	l.profile.RuntimeExecutable = self
	l.profile.RuntimeSHA256 = meta.RuntimeSHA256
	r, _, e := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if e != nil {
		t.Fatal(e)
	}
	bind(&s, r)
	raw, _ := json.Marshal(c)
	child := exec.Command(self, "-test.run=^TestManagedStopWaitsForOriginalExitAndAuthenticationCleanup$")
	child.Env = append(os.Environ(), "SR_TEST_TERMINAL_OWNER="+string(raw))
	if e = child.Start(); e != nil {
		t.Fatal(e)
	}
	defer child.Wait()
	defer child.Process.Kill()
	defer l.closeTerminalJob()
	var owner TerminalOwner
	found := false
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); time.Sleep(20 * time.Millisecond) {
		if e = readTerminalFile(c.StateDirectory, "owner", &owner); e == nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("original owner missing", e)
	}
	r, e = l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
	if e != nil || !r.State.Stopped || !l.terminalExitVerified(owner) {
		t.Fatal("stop raced original cleanup", r, e)
	}
	var exit TerminalExit
	if e = readTerminalFile(c.StateDirectory, "exit", &exit); e != nil {
		t.Fatal(e)
	}
	state, e := l.state(s)
	if e != nil {
		t.Fatal(e)
	}
	state.TerminalOwner = &owner
	s.Implementation, e = l.envelope(s, state)
	if e != nil {
		t.Fatal(e)
	}
	l.client = native.New(native.Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) {
		return json.Marshal(map[string]any{"ok": true, "result": map[string]any{"terminals": []any{}, "truncated": false, "totalCount": 0, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}, "_meta": map[string]string{"runtimeId": s.Ref.External.RuntimeID}})
	}})
	if e = l.verifyStopped(context.Background(), s); e != nil {
		t.Fatal("original helper exit proof was not accepted", e)
	}
	no := false
	exit.AuthenticationClosed = &no
	raw, _ = json.Marshal(exit)
	os.WriteFile(filepath.Join(c.StateDirectory, "exit.json"), raw, 0600)
	if l.terminalExitVerified(owner) {
		t.Fatal("failed authentication cleanup was ignored")
	}
	if e = l.verifyStopped(context.Background(), s); e == nil {
		t.Fatal("offline verification ignored failed authentication cleanup")
	}
}
