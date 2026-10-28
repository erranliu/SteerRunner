package orcatransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// The existing opt-in lifecycle entry owns one fresh terminal and never replays
// a prior probe. The host records real calls without replacing responses.
func runNativeInputLiveProbe(t *testing.T) {
	root := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ROOT")
	if root == "" {
		t.Fatal("explicit mode requires a fresh probe root")
	}
	var target struct {
		Worktree, Runtime, Environment, CLI string
		Launch                              agentlaunch.Config
	}
	raw, e := os.ReadFile(filepath.Join(root, "target.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = contracts.DecodeStrict(raw, &target); e != nil {
		t.Fatal(e)
	}
	if target.Worktree == "" || target.Runtime == "" || target.Environment == "" || target.CLI == "" || target.Launch.Executable == "" || target.Launch.Model == "" {
		t.Fatal("incomplete fixed native probe target")
	}
	owner, e := os.OpenFile(filepath.Join(root, "probe-attempted"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		t.Fatal("probe root already attempted; never replay", e)
	}
	owner.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	baseHost := process.New(40 * time.Second)
	host := &segmentProbeHost{ManagedProcessHost: baseHost, root: root}
	creator := native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
		args = append(args, "--environment", target.Environment)
		return host.OutputCommand(ctx, b.LaunchCommand{Executable: target.CLI, Args: args, WorkingDirectory: root, Timeout: timeout})
	}})
	record := map[string]any{"business_inputs": 0, "completed": false, "cleaned_up": false, "launch": target.Launch, "mode": os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE")}
	defer func() {
		record["paste_calls"], record["enter_calls"] = host.pastes.Load(), host.enters.Load()
		raw, _ := json.MarshalIndent(record, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "native-result.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	}()
	census := func(ctx context.Context, name string) bool {
		result, err := creator.MetaCall(ctx, "terminal", "list", "--worktree", target.Worktree, "--json")
		record[name] = result
		var v struct {
			TotalCount *int
			Truncated  *bool
			HostScope  *struct{ HostIDs, OmittedHostIDs *[]string }
			Terminals  *[]json.RawMessage
		}
		return err == nil && result.RuntimeID == target.Runtime && !result.CallUncertain && result.ErrorCode == "" && json.Unmarshal(result.Payload, &v) == nil && v.TotalCount != nil && *v.TotalCount == 0 && v.Truncated != nil && !*v.Truncated && v.HostScope != nil && v.HostScope.HostIDs != nil && len(*v.HostScope.HostIDs) > 0 && v.HostScope.OmittedHostIDs != nil && len(*v.HostScope.OmittedHostIDs) == 0 && v.Terminals != nil && len(*v.Terminals) == 0
	}
	if !census(ctx, "before_census") {
		t.Fatal("probe requires an empty, completely listed independent worktree")
	}
	launch, e := target.Launch.Command("codex")
	if e != nil {
		t.Fatal(e)
	}
	created, e := creator.MetaCall(ctx, "terminal", "create", "--worktree", target.Worktree, "--title", "SteerRunner isolated native input probe", "--command", launch, "--json")
	var value struct{ Terminal native.Terminal }
	if e != nil || created.RuntimeID != target.Runtime || json.Unmarshal(created.Payload, &value) != nil || value.Terminal.Handle == "" {
		t.Fatal("new probe creation unverified", e)
	}
	terminal := value.Terminal
	record["terminal"] = terminal
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		proof, e := creator.CloseTerminalChecked(cleanup, terminal.Handle, target.Runtime)
		record["cleanup"] = proof
		record["cleaned_up"] = e == nil && proof.Disposition == "confirmed" && census(cleanup, "after_census")
		if record["cleaned_up"] != true {
			t.Error("owned terminal cleanup/census unverified", e)
		}
	}()
	terminal, e = creator.ShowTerminal(ctx, terminal.Handle)
	if e != nil {
		t.Fatal(e)
	}
	profile := Profile{Executable: target.CLI, Environment: target.Environment, WaitTimeout: "40s"}
	identity := b.ProcessIdentity{Kind: "orca_terminal", Executable: target.CLI, Terminal: terminal.Handle, RuntimeID: target.Runtime, Incarnation: terminal.IncarnationID, WorktreeID: terminal.WorktreeID, ExecutionHost: profile.Target().ExecutionHost}
	var ready native.AgentState
	startup, startupCancel := context.WithTimeout(ctx, 90*time.Second)
	defer startupCancel()
	for {
		raw, e := native.QueryState(startup, target.Environment, target.Runtime, "id:"+identity.WorktreeID)
		var state native.AgentState
		if e == nil {
			state, e = native.DecodeOriginalState(raw, identity)
		}
		if e == nil {
			sample, queryErr := native.QueryOriginalSample(startup, target.Environment, identity, state.PaneKey)
			e = queryErr
			state = sample.AgentState
		}
		if e == nil && state.MainAgent != nil && state.MainAgent.State == "done" && state.LastAssistantMessage == "READY" {
			ready = state
			break
		}
		select {
		case <-startup.Done():
			t.Fatal("initial READY not verified", e)
		case <-time.After(time.Second):
		}
	}
	record["ready"] = ready
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaTerminalV3, Target: profile.Target(), Agent: b.AgentSpec{Name: "codex"}, RunID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), External: identity}, Session: b.SessionContext{ProjectID: "native-input-probe", ExecutionID: strings.Repeat("a", 32), AttemptID: strings.Repeat("b", 32), WorkflowRunID: strings.Repeat("a", 32), SessionID: SessionID(identity)}, ConfigurationDigest: "native-input-probe", ResourceGeneration: "native-input-probe", Workspace: b.WorkspaceIntent{Path: terminal.WorktreePath, Branch: strings.TrimPrefix(terminal.Branch, "refs/heads/")}}
	raw, _ = json.Marshal(profile)
	data, e := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: NativeRemoteProfileVersion, Purpose: b.ImplementationConfigPurpose, ImplementationKey: ProfileKey, ProjectID: s.Session.ProjectID, ProfileID: "native", AcceptedConfigVersion: "native"}, raw)
	if e != nil {
		t.Fatal(e)
	}
	port, e := NewLifecycle(b.LifecycleSetup{Snapshot: s, ProfileData: []b.ImplementationDataEnvelope{data}, ProfileID: "native", AcceptedConfigVersion: "native", ProcessHost: host, RepositoryPath: root, RunDirectory: filepath.Join(root, "owned-journal")})
	if e != nil {
		t.Fatal(e)
	}
	l := port.(*Lifecycle)
	defer l.closeStateFeed()
	s.Implementation, e = l.envelope(s, State{Process: identity})
	if e != nil {
		t.Fatal(e)
	}
	if strings.HasPrefix(os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE"), "followup-") {
		runNativeFollowupProbe(t, ctx, l, s, target.Environment, ready, record, root)
		return
	}
	nonce := make([]byte, 16)
	if _, e := rand.Read(nonce); e != nil {
		t.Fatal(e)
	}
	marker := "SR_NATIVE_INPUT_" + hex.EncodeToString(nonce)
	normalized := "This is a no-tools delivery qualification. Do not use tools, read or modify files, execute business tests, commit, deploy or publish. Reply exactly with " + marker + ".\n\n" + strings.Repeat("\u4e2d\u6587\u539f\u59cb\u8f93\u5165 example fragment, preserve every character and blank line.\n```powershell\nWrite-Output 'fixture-only-never-execute'\n```\n\n", 100)
	prompt := strings.ReplaceAll(normalized, "\n", "\r\n")
	if e := os.WriteFile(filepath.Join(root, "original-input.txt"), []byte(prompt), 0600); e != nil {
		t.Fatal(e)
	}
	record["prompt_sha256"] = digest([]byte(prompt))
	record["logical_bytes"] = len(prompt)
	if os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE") == "segmented-shape" {
		runNativeSegmentedShape(t, ctx, creator, identity, ready, prompt, marker, record)
		return
	}
	input := b.TurnInput{CommandID: hex.EncodeToString(nonce), Prompt: prompt}
	s.PendingTurn = &input
	r, _, e := l.Execute(ctx, request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
	if e != nil {
		t.Fatal(e)
	}
	bind(&s, r)
	req := request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})
	req.Operation.ID += "-" + terminal.Handle
	r, obs, e := l.Execute(ctx, req)
	record["submission_attempts"] = 1
	if r.State.InputAccepted {
		record["business_inputs"] = 1
	}
	record["submit_receipt"] = r.Operation.Receipt
	record["initial_start_confirmed"] = r.State.TurnStarted
	if e != nil || obs == nil {
		t.Fatal("original submission unverified", e)
	}
	bind(&s, r)
	defer func() {
		obs.Close(context.Background())
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if e := obs.(*observer).awaitCleanup(cleanup); e != nil {
			t.Error(e)
		}
	}()
	diagnostics := []string{}
	for {
		select {
		case event, ok := <-obs.Events():
			if !ok {
				t.Fatal("observer ended before completion", diagnostics)
			}
			if len(event.Implementation) > 0 {
				s.Implementation = event.Implementation
			}
			if event.Observation != nil {
				diagnostics = append(diagnostics, event.Observation.Diagnostic)
			}
			completed := event.Fact.Kind == b.EventCompleted && event.Fact.Completed
			event.Persisted <- nil
			event.Processed <- nil
			if completed {
				r, _, e = l.ObserveLifecycle(ctx, s)
				if e != nil || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || strings.TrimSpace(r.TurnEvidence.LastMessage) != marker {
					t.Fatal("original final unverified", e, r.Diagnostic)
				}
				sample, err := native.QueryOriginalSample(ctx, target.Environment, identity, ready.PaneKey)
				if err != nil {
					t.Fatal(err)
				}
				verified, err := native.QueryOriginalInput(ctx, target.Environment, identity, ready.PaneKey, ready.MainAgent.StateStartedAt, sample.AgentState)
				if err != nil || verified.NativeInput == nil || verified.NativeInput.Text != nativePromptText(prompt) || !verified.NativeInput.MatchesOriginalProcess(identity) || !verified.NativeInput.MatchesOriginalQuery(verified) || verified.LastAssistantMessage != marker || verified.MainAgent.StateStartedAt <= ready.MainAgent.StateStartedAt {
					t.Fatal("independent original input/final mismatch", err)
				}
				record["original_query"] = verified
				record["verified_original_input"] = verified.NativeInput
				if host.pastes.Load() != 1 || host.enters.Load() != 1 {
					t.Fatal("expected exactly one production paste and Enter")
				}
				record["completed"] = true
				record["final"] = r.TurnEvidence
				record["diagnostics"] = diagnostics
				return
			}
		case <-ctx.Done():
			record["diagnostics"] = diagnostics
			t.Fatal("native original did not complete", ctx.Err())
		}
	}
}

// Logs actual public reads only; no send interception, mutation, or fabricated response.
type segmentProbeHost struct {
	b.ManagedProcessHost
	root   string
	reads  atomic.Uint64
	pastes atomic.Uint64
	enters atomic.Uint64
}

func (h *segmentProbeHost) OutputCommand(ctx context.Context, c b.LaunchCommand) ([]byte, error) {
	if len(c.Args) > 1 && c.Args[0] == "terminal" && c.Args[1] == "send" {
		for _, arg := range c.Args {
			if arg == "--text" {
				h.pastes.Add(1)
			}
			if arg == "--enter" {
				h.enters.Add(1)
			}
		}
	}
	raw, err := h.ManagedProcessHost.OutputCommand(ctx, c)
	if len(c.Args) > 1 && c.Args[0] == "terminal" && c.Args[1] == "read" {
		n := h.reads.Add(1)
		if e := os.WriteFile(filepath.Join(h.root, fmt.Sprintf("actual-public-draft-read-%02d.json", n)), raw, 0600); e != nil {
			return nil, e
		}
	}
	return raw, err
}

// Explicit two-effect shape experiment; this is not the production one-paste path.
func runNativeSegmentedShape(t *testing.T, ctx context.Context, creator native.Client, identity b.ProcessIdentity, ready native.AgentState, prompt, marker string, record map[string]any) {
	environment := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_ENVIRONMENT")
	record["submission_attempts"] = 1
	text := nativePromptText(prompt)

	initial, e := creator.MetaCall(ctx, "terminal", "read", "--terminal", identity.Terminal, "--limit", "80", "--json")
	var initialDraft struct {
		Terminal struct{ Handle, Draft string }
	}
	if e != nil || initial.CallUncertain || initial.ErrorCode != "" || json.Unmarshal(initial.Payload, &initialDraft) != nil || initial.RuntimeID != identity.RuntimeID || initialDraft.Terminal.Handle != identity.Terminal || initialDraft.Terminal.Draft != "" {
		t.Fatal("fresh composer not empty", e)
	}
	runes := []rune(text)
	split := len(runes) / 3
	chunks := []string{string(runes[:split]), string(runes[split:])}
	record["paste_attempts"] = 0
	for index, chunk := range chunks {
		record["paste_attempts"] = index + 1
		payload := "\x1b[200~" + chunk + "\x1b[201~"
		pasted, e := creator.MetaCall(ctx, "terminal", "send", "--terminal", identity.Terminal, "--text", payload, "--json")
		record[fmt.Sprintf("paste_receipt_%d", index+1)] = pasted
		var paste struct {
			Send struct {
				Handle       string
				Accepted     bool
				BytesWritten int
			}
		}
		if e != nil || pasted.CallUncertain || pasted.ErrorCode != "" || json.Unmarshal(pasted.Payload, &paste) != nil || !paste.Send.Accepted || paste.Send.Handle != identity.Terminal || pasted.RuntimeID != identity.RuntimeID || paste.Send.BytesWritten != len(payload) {
			t.Fatal("sole original chunk unverified; no retry", e)
		}
		expectedText := string(runes[:split])
		if index == 1 {
			expectedText = text
		}
		stable := 0
		last := ""
		drafts := []string{}
		for {
			raw, e := creator.MetaCall(ctx, "terminal", "read", "--terminal", identity.Terminal, "--limit", "80", "--json")
			var read struct {
				Terminal struct{ Handle, Draft, Source string }
			}
			if e != nil || raw.CallUncertain || raw.ErrorCode != "" || json.Unmarshal(raw.Payload, &read) != nil || raw.RuntimeID != identity.RuntimeID || read.Terminal.Handle != identity.Terminal || read.Terminal.Source != "screen" {
				t.Fatal("original draft read unverified", e)
			}
			draft := read.Terminal.Draft
			drafts = append(drafts, draft)
			record[fmt.Sprintf("drafts_%d", index+1)] = drafts
			if nativePasteDraftMatches(draft, expectedText) {
				if draft == last {
					stable++
				} else {
					stable = 1
				}
			} else {
				stable = 0
			}
			last = draft
			if stable >= 2 {
				if index == 1 && strings.Count(draft, "[Pasted Content ") < 2 {
					t.Fatal("segmented native shape not reproduced; do not claim qualification")
				}
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("segmented composer did not settle; no Enter")
			case <-time.After(300 * time.Millisecond):
			}
		}
	}
	record["logical_inputs"] = 1
	current, e := creator.ShowTerminal(ctx, identity.Terminal)
	if e != nil || current.IncarnationID != identity.Incarnation || current.WorktreeID != identity.WorktreeID || current.AgentIdentity != "codex" {
		t.Fatal("original terminal changed", e)
	}
	boundary, e := native.QueryOriginalSample(ctx, environment, identity, ready.PaneKey)
	if e != nil || boundary.MainAgent == nil || boundary.MainAgent.State != "done" || boundary.MainAgent.StateStartedAt != ready.MainAgent.StateStartedAt || boundary.ProviderSession != ready.ProviderSession {
		t.Fatal("pre-input boundary changed; no Enter", e)
	}
	record["enter_attempts"] = 1
	entered, e := creator.MetaCall(ctx, "terminal", "send", "--terminal", identity.Terminal, "--enter", "--json")
	record["enter_receipt"] = entered
	if e != nil {
		t.Fatal(e)
	}
	var enter struct {
		Send struct {
			Handle       string
			Accepted     bool
			BytesWritten int
		}
	}
	if entered.CallUncertain || entered.ErrorCode != "" || json.Unmarshal(entered.Payload, &enter) != nil || !enter.Send.Accepted || enter.Send.BytesWritten != 1 || enter.Send.Handle != identity.Terminal || entered.RuntimeID != identity.RuntimeID {
		t.Fatal("original submit unverified")
	}
	for {
		sample, e := native.QueryOriginalSample(ctx, environment, identity, ready.PaneKey)
		if e == nil && sample.AgentState.State == "done" && sample.AgentState.LastAssistantMessage == marker {
			state, e := native.QueryOriginalInput(ctx, environment, identity, ready.PaneKey, ready.MainAgent.StateStartedAt, sample.AgentState)
			if e == nil && state.NativeInput != nil && state.NativeInput.Text == text && state.NativeInput.MatchesOriginalProcess(identity) && state.NativeInput.MatchesOriginalQuery(state) {
				record["verified_original_input"] = state.NativeInput
				record["completed"] = true
				record["final"] = state
				return
			}
			record["last_query_error"] = fmt.Sprint(e)
		}
		select {
		case <-ctx.Done():
			t.Fatal("original staged input not completed", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
