package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

type agentFixture struct {
	pages                           []native.AgentPage
	detail                          native.AgentItem
	create                          native.AgentCreate
	creates, closes, reads, details int
	lost                            bool
	closeRaw                        json.RawMessage
	send                            func(native.AgentSend) error
}

func (f *agentFixture) SupportsCreate(context.Context, string, string) error { return nil }
func (f *agentFixture) Create(_ context.Context, c native.AgentCreate) (json.RawMessage, error) {
	f.creates++
	f.create = c
	if f.lost {
		return nil, errors.New("response lost")
	}
	return json.RawMessage(`{"ok":true}`), nil
}
func (f *agentFixture) Close(context.Context, string) (json.RawMessage, error) {
	f.closes++
	if f.lost {
		return nil, errors.New("close response lost")
	}
	if f.closeRaw != nil {
		return f.closeRaw, nil
	}
	return json.RawMessage(`{"ok":true}`), nil
}
func (f *agentFixture) Send(_ context.Context, request native.AgentSend) (json.RawMessage, error) {
	if f.send == nil {
		return nil, errors.New("unexpected structured follow-up")
	}
	return nil, f.send(request)
}
func (f *agentFixture) History(_ context.Context, session string, before *native.AgentCursor, _ int) (native.AgentPage, error) {
	f.reads++
	i := 0
	if before != nil {
		i = 1
	}
	return f.pages[i], nil
}
func (f *agentFixture) Item(context.Context, string, string) (native.AgentItem, error) {
	f.details++
	return f.detail, nil
}

func structuredFixture(t *testing.T) (*Lifecycle, b.LifecycleSnapshot, *agentFixture) {
	t.Helper()
	l, _, s := structuredFixtureOn(t, "")
	s.Ref.Protocol = b.ProtocolOrcaAgentSessionV1
	s.Ref.External = b.ProcessIdentity{Kind: "orca_agent_session", RuntimeID: "original-runtime", WorktreeID: "worktree", ExecutionHost: "local", Executable: "orca.exe"}
	s.Workspace = b.WorkspaceIntent{Path: "original-workspace", Branch: "original-branch", BaseCommit: strings.Repeat("e", 40)}
	s.Session.PendingCommandID = strings.Repeat("c", 32)
	s.PendingTurn = &b.TurnInput{CommandID: s.Session.PendingCommandID, Prompt: "one\r\n\r\n中文😀 \"quote\" \\path\n```\n<&>\n```  \r\n"}
	l.setup.Snapshot = s.Clone()
	state := agentSessionState{RuntimeID: s.Ref.External.RuntimeID, Worktree: native.Worktree{ID: "worktree", InstanceID: "original-instance", Path: s.Workspace.Path}, SessionID: agentSessionID(s), CommandID: s.PendingTurn.CommandID, InputDigest: digest([]byte(s.PendingTurn.Prompt)), MessageID: "1793680000000-00112233445566778899aabbccddeeff", CreateID: "1793680000000-112233445566778899aabbccddeeff00"}
	state.BodyFingerprint, _ = native.AgentFingerprint("agentSession.send", state.SessionID, map[string]any{"body": agentBody(s.PendingTurn.Prompt)})
	var err error
	s.Implementation, err = l.agentEnvelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	user := native.AgentItem{ItemID: "orca:" + state.MessageID, Sequence: 2, Body: mustAgentJSON(agentBody(s.PendingTurn.Prompt))}
	turn := native.AgentItem{ItemID: "original-turn-item", Sequence: 3, Body: mustAgentJSON(map[string]any{"kind": "turn", "turnId": "native-turn", "state": "completed", "outcome": "success", "userItemId": user.ItemID, "startedAt": 10, "completedAt": 20})}
	assistant := func(id string, seq int64, text string) native.AgentItem {
		i := native.AgentItem{ItemID: id, Sequence: seq, Body: mustAgentJSON(native.AgentMessage{Kind: "message", Role: "assistant", Blocks: []native.AgentTextBlock{{Type: "text", Text: text}}})}
		i.TurnScope.Kind = "turn"
		i.TurnScope.TurnItemID = turn.ItemID
		return i
	}
	long := strings.Repeat("complete result 中文😀\n", 300)
	first, last := assistant("commentary", 5, "tool preamble"), assistant("final", 9, long)
	page := native.AgentPage{SessionID: state.SessionID, Epoch: "original-epoch", Submissions: []native.AgentSubmission{{ClientMessageID: state.MessageID, PayloadFingerprint: state.BodyFingerprint, DispatchState: "accepted", ProviderItemID: "provider-user-item"}}, Items: []native.AgentItem{last}}
	page.HasOlder = true
	page.Window.NextCursor = &native.AgentCursor{Epoch: page.Epoch, Sequence: 9}
	older := native.AgentPage{SessionID: state.SessionID, Epoch: page.Epoch, Items: []native.AgentItem{user, turn, first, {ItemID: "tool", Sequence: 6, Body: json.RawMessage(`{"kind":"tool-call","status":"completed"}`)}}}
	fake := &agentFixture{pages: []native.AgentPage{page, older}}
	l.agentSessions = func(runtime string) agentSessionPort {
		if runtime != state.RuntimeID {
			t.Fatal("runtime changed")
		}
		return fake
	}
	return l, s, fake
}
func mustAgentJSON(v any) json.RawMessage { raw, _ := json.Marshal(v); return raw }

func TestStructuredRunningActivityUsesOriginalTurn(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "foreign"}[foreign], func(t *testing.T) {
			l, s, f := structuredFixture(t)
			s.Session.Generation = "before-native-create"
			var turn map[string]any
			_ = json.Unmarshal(f.pages[1].Items[1].Body, &turn)
			turn["state"], turn["completedAt"] = "running", 0
			f.pages[1].Items[1].Body = mustAgentJSON(turn)
			if foreign {
				f.pages[0].Items[0].TurnScope.TurnItemID = "foreign-turn"
			}
			receipt, message, err := l.readStructuredActivity(context.Background(), s)
			if foreign {
				if err == nil || message != nil {
					t.Fatal("foreign native activity exposed", message, err)
				}
				return
			}
			if err != nil || receipt.TurnEvidence != nil || message == nil || message.SessionID != f.pages[0].SessionID || message.TurnID != "native-turn" || message.AttemptID != s.Session.AttemptID || message.Generation != receipt.State.SessionGeneration || message.Generation == s.Session.Generation || message.Kind != "assistant" || message.ObservedAt.IsZero() || message.Text != strings.Repeat("complete result 中文😀\n", 300) {
				t.Fatal("running activity lost original identity or became completion", receipt, message, err)
			}
		})
	}
}

func TestStructuredObserverProjectsAndDeduplicatesActivity(t *testing.T) {
	l, s, f := structuredFixture(t)
	var turn map[string]any
	_ = json.Unmarshal(f.pages[1].Items[1].Body, &turn)
	turn["state"], turn["completedAt"] = "running", 0
	f.pages[1].Items[1].Body = mustAgentJSON(turn)
	o := l.observeStructured(s)
	defer o.Close(context.Background())
	for _, kind := range []string{"started", "message"} {
		select {
		case event := <-o.Events():
			if event.Persisted == nil {
				t.Fatal("activity bypassed consumer acknowledgement")
			}
			if kind == "started" && event.Fact.Kind != b.EventStarted || kind == "message" && (event.PublicMessage == nil || len(event.Implementation) != 0 || event.Observation != nil || event.Fact.Completed) {
				t.Fatal("activity confused with native fact", event)
			}
			event.Persisted <- nil
		case <-time.After(3 * time.Second):
			t.Fatal("native activity not observed")
		}
	}
	select {
	case event := <-o.Events():
		t.Fatal("identical native activity repeated", event)
	case <-time.After(1200 * time.Millisecond):
	}
}

func TestStructuredAtomicCreateAndReadOnlyRecovery(t *testing.T) {
	l, s, f := structuredFixture(t)
	f.lost = true
	r := request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: s.PendingTurn})
	receipt, observer, err := l.Execute(context.Background(), r)
	if err == nil || observer != nil || receipt.Disposition != b.OperationUnknown || f.creates != 1 {
		t.Fatal(receipt, err)
	}
	if f.create.FirstMessage.Body.Blocks[0].Text != s.PendingTurn.Prompt || len(f.create.FirstMessage.Body.Blocks) != 1 || f.create.FirstMessage.Selection.Model != "requested-model" || f.create.FirstMessage.Selection.Effort != "medium" || f.create.Options["permissionMode"] != "full-access" {
		t.Fatal("atomic input wiring changed", f.create)
	}
	// Persisted receipt may be lost as well: restore from the original intent,
	// never from a replacement identity and never call create again.
	restored := &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, agentSessions: l.agentSessions}
	receipt, observer, err = restored.Reconcile(context.Background(), r)
	if observer != nil {
		defer observer.Close(context.Background())
	}
	if err != nil || f.creates != 1 || receipt.Disposition != b.OperationConfirmed || receipt.State.TurnID != "native-turn" {
		t.Fatal(receipt, err, f.creates)
	}
	merged, _, err := reliable.ApplySessionReceipt(s, nil, receipt, false)
	if err != nil || merged.Session.CurrentTurnID != "native-turn" || merged.PendingTurn != nil {
		t.Fatal("native start did not bind before result", merged.Session, err)
	}
	evidence := receipt.TurnEvidence
	if evidence == nil || evidence.FinalSessionID != evidence.SessionID || evidence.FinalTurnID != evidence.TurnID || evidence.FinalNativeSessionID != evidence.SessionID || evidence.FinalNativeTurnID != evidence.TurnID {
		t.Fatal("atomic session final lost its execution binding or native provenance", evidence)
	}
	if evidence == nil || !evidence.FinalVerified || !evidence.Succeeded || evidence.LastMessage != "tool preamble\n\n"+strings.Repeat("complete result 中文😀\n", 300) {
		t.Fatal("incomplete original turn result", evidence)
	}
	state, err := l.agentState(merged)
	if err != nil || state.ProviderItemID == "orca:"+state.MessageID {
		t.Fatal("provider/normalized identities conflated", err)
	}
	if strings.Contains(string(merged.Implementation[0].Payload), "tool preamble") || strings.Contains(string(merged.Implementation[0].Payload), "complete result") {
		t.Fatal("result duplicated in opaque state")
	}
}

func TestStructuredRemoteStopRetainsOriginalTurnAndReusesReceipt(t *testing.T) {
	for _, version := range []int{AgentSessionVersion, AgentSessionCloseProfileVersion} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			l, s, f := structuredFixture(t)
			l.profileVersion = version
			settled := version == AgentSessionCloseProfileVersion
			l.profile.Environment = "original-remote"
			s.Ref.Target = l.profile.Target()
			s.Ref.External.ExecutionHost = s.Ref.Target.ExecutionHost
			l.setup.Snapshot = s
			var turn map[string]any
			_ = json.Unmarshal(f.pages[1].Items[1].Body, &turn)
			turn["state"], turn["outcome"] = "interrupted", "cancellation"
			f.pages[1].Items[1].Body = mustAgentJSON(turn)
			// The original accepted turn binding was committed before the user stop.
			observed, err := l.readStructured(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			s.Implementation = observed.Implementation
			state, err := l.agentState(s)
			if err != nil {
				t.Fatal(err)
			}
			state.Attempted = true
			s.Implementation, err = l.agentEnvelope(s, state)
			if err != nil {
				t.Fatal(err)
			}
			r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{TurnID: state.TurnID}})
			receipt, err := l.StopLifecycle(context.Background(), r, nil)
			if (err == nil) != settled || receipt.State.Stopped != settled || receipt.State.WritersAbsent || receipt.TurnEvidence == nil || receipt.TurnEvidence.NativeOutcome != "cancelled" || !receipt.TurnEvidence.FinalVerified || f.closes != 1 {
				t.Fatal("stop conflated business result and exit proof", receipt, err)
			}
			reads := f.reads
			raw, _ := json.Marshal(receipt)
			r.Operation.Receipt = raw
			receipt, _, err = l.Reconcile(context.Background(), r)
			if (err == nil) != settled || receipt.TurnEvidence == nil || receipt.TurnEvidence.NativeOutcome != "cancelled" || f.closes != 1 || f.reads != reads || receipt.State.Stopped != settled {
				t.Fatal("original stop result replayed or lost", receipt, err, f.reads, reads)
			}
			r.Snapshot.Implementation = receipt.Implementation
			if (l.verifyAgentSessionStopped(r.Snapshot) == nil) != settled {
				t.Fatal("offline verification changed the frozen remote scope")
			}
		})
	}
}

func TestStructuredHistoryFailsClosed(t *testing.T) {
	for _, scenario := range []string{"epoch", "gap", "removed", "cursor", "foreign-user", "foreign-assistant", "unknown-outcome", "failed", "cancelled", "native-cancellation", "shortened", "detail-changed", "detail-order-changed", "duplicate-scope-changed", "duplicate-order-changed", "ambiguous-order"} {
		t.Run(scenario, func(t *testing.T) {
			l, s, f := structuredFixture(t)
			switch scenario {
			case "epoch":
				f.pages[1].Epoch = "foreign"
			case "gap":
				f.pages[0].HasNewer = true
			case "removed":
				f.pages[0].RemovedItemIDs = []string{"removed-result"}
			case "cursor":
				f.pages[1].HasOlder = true
				f.pages[1].Window.NextCursor = f.pages[0].Window.NextCursor
			case "foreign-user":
				f.pages[1].Items[0].ItemID = "foreign-user"
			case "foreign-assistant":
				f.pages[0].Items[0].TurnScope.TurnItemID = "foreign-turn"
			case "unknown-outcome", "failed", "cancelled", "native-cancellation":
				var body map[string]any
				_ = json.Unmarshal(f.pages[1].Items[1].Body, &body)
				body["outcome"] = scenario
				if scenario == "failed" {
					body["outcome"] = "failure"
				}
				if scenario == "cancelled" {
					body["outcome"] = "cancellation"
				}
				if scenario == "native-cancellation" {
					body["outcome"] = "cancellation"
					body["state"] = "interrupted"
				}
				f.pages[1].Items[1].Body = mustAgentJSON(body)
			case "duplicate-scope-changed", "duplicate-order-changed":
				duplicate := f.pages[0].Items[0]
				if scenario == "duplicate-scope-changed" {
					duplicate.TurnScope.TurnItemID = "foreign-turn"
					f.pages[0].Items[0] = duplicate
					duplicate.TurnScope.TurnItemID = "original-turn-item"
				} else {
					duplicate.SequenceIndex++
				}
				f.pages[1].Items = append(f.pages[1].Items, duplicate)
			case "ambiguous-order":
				duplicate := f.pages[0].Items[0]
				duplicate.ItemID = "another-assistant-at-same-position"
				f.pages[0].Items = append(f.pages[0].Items, duplicate)
			case "shortened", "detail-changed", "detail-order-changed":
				f.detail = f.pages[0].Items[0]
				f.pages[0].Items[0].Shortened = true
				f.pages[0].Items[0].Body = json.RawMessage(`{"kind":"message","role":"assistant","blocks":[]}`)
				if scenario == "detail-changed" {
					f.detail.Revision++
				}
				if scenario == "detail-order-changed" {
					f.detail.SequenceIndex++
				}
			}
			receipt, err := l.readStructured(context.Background(), s)
			if scenario == "failed" || scenario == "cancelled" || scenario == "native-cancellation" {
				outcome := scenario
				if scenario == "native-cancellation" {
					outcome = "cancelled"
				}
				if err != nil || receipt.TurnEvidence == nil || receipt.TurnEvidence.Succeeded || receipt.TurnEvidence.NativeOutcome != outcome {
					t.Fatal(receipt, err)
				}
				return
			}
			if scenario == "shortened" {
				if err != nil || receipt.TurnEvidence == nil || f.details != 1 || !strings.Contains(receipt.TurnEvidence.LastMessage, "complete result") {
					t.Fatal(receipt, err)
				}
				return
			}
			if err == nil || receipt.TurnEvidence != nil {
				t.Fatal("unverified result promoted", receipt, err)
			}
		})
	}
}

func TestStructuredLostCloseRemainsUnknownWithoutReplay(t *testing.T) {
	l, s, f := structuredFixture(t)
	state, _ := l.agentState(s)
	state.Attempted = true
	s.Implementation, _ = l.agentEnvelope(s, state)
	f.lost = true
	r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{TurnID: "native-turn"}})
	receipt, err := l.StopLifecycle(context.Background(), r, nil)
	if err == nil || f.closes != 1 || receipt.State.Stopped || receipt.State.WorkerAbsent || receipt.State.WritersAbsent {
		t.Fatal("close manufactured settlement", receipt, err)
	}
	restored := &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, agentSessions: l.agentSessions}
	raw, _ := json.Marshal(receipt)
	r.Operation.Receipt = raw
	receipt, _, err = restored.Reconcile(context.Background(), r)
	if err == nil || f.closes != 1 || receipt.State.Stopped || receipt.Disposition != b.OperationUnknown {
		t.Fatal("unknown close replayed", receipt, err)
	}
	// A second public stop command still cannot close the same native session.
	s.Operations = []b.Operation{r.Operation}
	next := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	next.Operation.ID = "second-stop"
	restored.stopping.Store(false)
	_, _ = restored.StopLifecycle(context.Background(), next, nil)
	if f.closes != 1 {
		t.Fatal("second stop closed again")
	}
	if err := VerifyStopped(context.Background(), l.setup); err == nil || !strings.Contains(err.Error(), "未核实") {
		t.Fatal("offline verification invented absence", err)
	}
}

func TestStructuredObserverBindsDelayedNativeStart(t *testing.T) {
	l, s, f := structuredFixture(t)
	f.pages[0].ActionablePromptIDs = []string{"native-approval"}
	o := l.observeStructured(s)
	defer o.Close(context.Background())
	for _, kind := range []b.EventKind{b.EventStarted, b.EventStatus, "", b.EventCompleted} {
		select {
		case event := <-o.Events():
			if kind == "" {
				if event.PublicMessage == nil || event.PublicMessage.TurnID != "native-turn" || event.PublicMessage.SessionID != agentSessionID(s) || event.Fact.Completed {
					t.Fatal("public native message lost identity or became completion", event)
				}
				event.Persisted <- nil
				continue
			}
			if event.Fact.Kind != kind || event.Fact.StartedTurnID != "native-turn" {
				t.Fatal(event)
			}
			if kind == b.EventStatus && (event.Observation == nil || event.Observation.Diagnostic == "" || event.Fact.SessionID != agentSessionID(s) || event.Fact.Source == "" || event.Fact.SourceEventID == "") {
				t.Fatal("diagnostic cannot pass the Engine observation identity gate", event)
			}
			event.Persisted <- nil
		case <-time.After(time.Second):
			t.Fatal("missing native event", kind)
		}
	}
}

func TestStructuredObserverKeepsOriginalDiagnosticBindingDuringHistoryGap(t *testing.T) {
	l, s, f := structuredFixture(t)
	verified, err := l.readStructured(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	s.Implementation = verified.Implementation
	s.Session.SessionID, s.Session.CurrentTurnID = verified.State.SessionID, verified.State.TurnID
	// A later incomplete observation does not undo the previously verified IDs
	// or authorize another mutation. Its diagnostic must still reach Engine.
	f.pages = []native.AgentPage{{SessionID: verified.State.SessionID, Epoch: "original-epoch"}}
	o := l.observeStructured(s)
	defer o.Close(context.Background())
	select {
	case event := <-o.Events():
		if event.Observation == nil || event.Observation.Diagnostic == "" || event.Fact.Kind != b.EventStatus || event.Fact.SessionID != verified.State.SessionID || event.Fact.StartedTurnID != "native-turn" {
			t.Fatal("history gap lost original diagnostic binding")
		}
		event.Persisted <- nil
	case <-time.After(time.Second):
		t.Fatal("history gap diagnostic missing")
	}
}

func TestStructuredServiceShutdownOnlyDetaches(t *testing.T) {
	l, s, f := structuredFixture(t)
	facts, err := l.ShutdownLifecycle(context.Background(), s, nil)
	if f.closes != 0 || f.creates != 0 || facts.Stopped || facts.WorkerAbsent || facts.WritersAbsent || err == nil || !strings.Contains(err.Error(), "未核实") {
		t.Fatal("service shutdown mutated native ownership or invented exit", facts, err)
	}
}

// Replay public RPC evidence so stricter completeness checks also cover the
// real Orca journal shape and ordering, without creating another live session.
func TestStructuredRecordedPublicResults(t *testing.T) {
	for _, tc := range []struct {
		name, outcome, sha256 string
		bytes                 int
	}{{"normal", "succeeded", "5ec2625fd5a37c6e3f2f69e97e9cb23fe4a54bcf8896654e88574da767bbe362", 7027}, {"large", "succeeded", "0a94d0df689cb7b14418619ad1fc6a85e2119b4791a38eba9f3b8143e9ddadbe", 13927}, {"active-after-stop", "cancelled", "", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			l, s, f := structuredFixture(t)
			name := "verified-" + tc.name + "-native-history-page-0.json"
			if tc.name == "active-after-stop" {
				name = "verified-active-native-after-stop-history-page-0.json"
			}
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "acceptance", "evidence", "orca-structured-agent-session-mvp-20261103", name))
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Result struct{ Page native.AgentPage }
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			page := wire.Result.Page
			if page.HasOlder || len(page.Submissions) != 1 {
				t.Fatal("recorded complete first-turn evidence changed")
			}
			state, _ := l.agentState(s)
			state.MessageID = page.Submissions[0].ClientMessageID
			state.BodyFingerprint = page.Submissions[0].PayloadFingerprint
			var input native.AgentMessage
			for _, item := range page.Items {
				if item.ItemID == "orca:"+state.MessageID {
					if err := json.Unmarshal(item.Body, &input); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := input.ValidateInput(); err != nil {
				t.Fatal(err)
			}
			s.PendingTurn.Prompt = input.Blocks[0].Text
			state.InputDigest = digest([]byte(s.PendingTurn.Prompt))
			s.Implementation, _ = l.agentEnvelope(s, state)
			page.SessionID = state.SessionID
			f.pages = []native.AgentPage{page}
			receipt, err := l.readStructured(context.Background(), s)
			if err != nil || receipt.TurnEvidence == nil || !receipt.TurnEvidence.FinalVerified || receipt.TurnEvidence.NativeOutcome != tc.outcome {
				t.Fatal("recorded original result rejected", err)
			}
			if tc.bytes > 0 && (len(receipt.TurnEvidence.LastMessage) != tc.bytes || digest([]byte(receipt.TurnEvidence.LastMessage)) != tc.sha256) {
				t.Fatal("recorded result truncated or reordered", len(receipt.TurnEvidence.LastMessage))
			}
		})
	}
}

func TestStructuredCloseAcknowledgementSettlesOnlySessionScope(t *testing.T) {
	l, s, f := structuredFixture(t)
	state, _ := l.agentState(s)
	state.Attempted = true
	s.Implementation, _ = l.agentEnvelope(s, state)
	r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{TurnID: "native-turn"}})
	got, err := l.StopLifecycle(context.Background(), r, nil)
	if err != nil || f.closes != 1 || got.Disposition != b.OperationConfirmed || !got.State.Stopped || !got.State.WorkerAbsent || got.State.WritersAbsent {
		t.Fatal("session close scope", got, err, f.closes)
	}
	settled, _, err := reliable.ApplySessionReceipt(s, nil, got, false)
	if err != nil {
		t.Fatal(err)
	}
	l.setup.Snapshot = settled
	if err := VerifyStopped(context.Background(), l.setup); err != nil {
		t.Fatal("offline session proof", err)
	}
	if f.reads != 0 {
		t.Fatal("offline proof queried or reopened the session")
	}
	raw, _ := json.Marshal(got)
	r.Operation.Receipt = raw
	restored := &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, agentSessions: l.agentSessions}
	again, _, err := restored.Reconcile(context.Background(), r)
	if err != nil || again.Disposition != b.OperationConfirmed || f.closes != 1 {
		t.Fatal("replayed acknowledged close", again, err, f.closes)
	}
	l.setup.Snapshot.Ref.External.RuntimeID = "foreign-runtime"
	if err := VerifyStopped(context.Background(), l.setup); err == nil {
		t.Fatal("foreign runtime accepted")
	}
}

func TestStructuredDurableCloseReceiptReconcilesWithoutReplay(t *testing.T) {
	l, s, f := structuredFixture(t)
	state, _ := l.agentState(s)
	state.Attempted = true
	state.CloseAttempted = true
	s.Implementation, _ = l.agentEnvelope(s, state)
	r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
	saved := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
	saved.Operation.Receipt = json.RawMessage(`{"ok":true}`)
	raw, _ := json.Marshal(saved)
	r.Operation.Receipt = raw
	got, _, err := l.Reconcile(context.Background(), r)
	if err != nil || got.Disposition != b.OperationConfirmed || !got.State.Stopped || f.closes != 0 {
		t.Fatal("durable original acknowledgement not reconciled", got, err)
	}
	saved.Operation.RunID = "foreign-run"
	raw, _ = json.Marshal(saved)
	r.Operation.Receipt = raw
	if _, _, err := l.Reconcile(context.Background(), r); err == nil {
		t.Fatal("foreign operation receipt accepted")
	}
}

func TestStructuredInvalidCloseResponseDoesNotSettle(t *testing.T) {
	for _, raw := range []string{`{}`, `{"ok":false}`, `{"ok":"true"}`, `null`} {
		t.Run(raw, func(t *testing.T) {
			l, s, f := structuredFixture(t)
			state, _ := l.agentState(s)
			state.Attempted = true
			s.Implementation, _ = l.agentEnvelope(s, state)
			f.closeRaw = json.RawMessage(raw)
			got, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
			if err == nil || got.State.Stopped || got.Disposition != b.OperationUnknown {
				t.Fatal("unconfirmed close settled", got, err)
			}
		})
	}
}

func TestStructuredStopBeforeCreateDoesNotInventCloseReceipt(t *testing.T) {
	l, s, f := structuredFixture(t)
	got, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
	if err != nil || got.Disposition != b.OperationConfirmed || !got.State.Stopped || f.closes != 0 || len(got.Operation.Receipt) != 0 {
		t.Fatal("undispatched session stop", got, err)
	}
	settled, _, err := reliable.ApplySessionReceipt(s, nil, got, false)
	if err != nil {
		t.Fatal(err)
	}
	l.setup.Snapshot = settled
	if err := VerifyStopped(context.Background(), l.setup); err != nil {
		t.Fatal(err)
	}
	state, _ := l.agentState(settled)
	state.Attempted = true
	settled.Implementation, _ = l.agentEnvelope(settled, state)
	l.setup.Snapshot = settled
	if err := VerifyStopped(context.Background(), l.setup); err == nil {
		t.Fatal("contradictory dispatch was accepted as absent")
	}
}

func TestStructuredSavedCloseRejectsChangedResource(t *testing.T) {
	for _, field := range []string{"runtime", "worktree"} {
		t.Run(field, func(t *testing.T) {
			l, s, f := structuredFixture(t)
			state, _ := l.agentState(s)
			state.Attempted = true
			state.CloseAttempted = true
			s.Implementation, _ = l.agentEnvelope(s, state)
			r := request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}})
			saved := b.LifecycleReceipt{Operation: r.Operation, Disposition: b.OperationUnknown}
			saved.Operation.Receipt = json.RawMessage(`{"ok":true}`)
			r.Operation.Receipt, _ = json.Marshal(saved)
			if field == "runtime" {
				r.Snapshot.Ref.External.RuntimeID = "replacement-runtime"
				state.RuntimeID = "replacement-runtime"
			} else {
				r.Snapshot.Ref.External.WorktreeID = "replacement-worktree"
				state.Worktree.ID = "replacement-worktree"
			}
			r.Snapshot.Implementation, _ = l.agentEnvelope(r.Snapshot, state)
			l.setup.Snapshot = r.Snapshot.Clone()
			got, _, err := l.Reconcile(context.Background(), r)
			if err == nil || got.State.Stopped || f.closes != 0 {
				t.Fatal("old close proof moved to a replacement resource", got, err)
			}
		})
	}
}
