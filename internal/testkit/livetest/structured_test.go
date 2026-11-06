package livetest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	adapter "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func structuredTestFixture(t *testing.T) (Manifest, contracts.RunView, *runstate.Record, StructuredRun) {
	t.Helper()
	id, attempt := strings.Repeat("a", 32), strings.Repeat("b", 32)
	transport := adapter.Profile{Executable: "C:/orca.exe", WaitTimeout: "30s"}
	profile := adapter.AgentSessionProfile{Transport: transport, AccessMode: "full_access", Development: true}
	raw, _ := json.Marshal(profile)
	pe, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 2, Purpose: b.ImplementationConfigPurpose, ImplementationKey: adapter.AgentSessionProfileKey, ProjectID: "project", ProfileID: id, AcceptedConfigVersion: "accepted"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	binding := StructuredBinding{RuntimeID: "runtime", Worktree: native.Worktree{ID: "worktree", InstanceID: "instance", Path: "C:/workspace", Branch: "worker"}, SessionID: StructuredSessionID(id, attempt), CommandID: strings.Repeat("c", 32), MessageID: "original-message", CreateID: "original-create", TurnID: "native-turn", TurnItemID: "turn-item", ProviderItemID: "provider-user", Epoch: "epoch", BodyFingerprint: strings.Repeat("d", 64)}
	sum := sha256.Sum256([]byte("original input"))
	binding.InputDigest = hex.EncodeToString(sum[:])
	raw, _ = json.Marshal(binding)
	e, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{CodecVersion: 1, Purpose: b.ImplementationResourcePurpose, ImplementationKey: adapter.AgentSessionStateKey, ProjectID: "project", ExecutionID: id, AttemptID: attempt, Generation: "generation", ConfigDigest: "config"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{Backend: b.BackendOrca, Protocol: b.ProtocolOrcaAgentSessionV1, RunID: id, AttemptID: attempt, Agent: b.AgentSpec{Name: "codex"}, Target: transport.Target(), External: b.ProcessIdentity{Kind: "orca_agent_session", Executable: transport.Executable, ExecutionHost: "local", RuntimeID: binding.RuntimeID, WorktreeID: binding.Worktree.ID}}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: id, ExecutionID: id, AttemptID: attempt}, ResourceGeneration: "generation", ConfigurationDigest: "config", Workspace: b.WorkspaceIntent{Path: binding.Worktree.Path, Branch: binding.Worktree.Branch}, Implementation: []b.ImplementationDataEnvelope{e}}
	m := Manifest{Execution: contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"full_access","transport":{"executable":"C:/orca.exe"}}`)}, LocalRuntimeID: "runtime"}
	view := contracts.RunView{RunID: id, StopEvidenceScope: adapter.AgentSessionStopScope, Execution: contracts.ExecutionView{Agent: "codex", Protocol: string(b.ProtocolOrcaAgentSessionV1), SessionID: binding.SessionID, TurnID: binding.TurnID, AttemptID: attempt, Generation: "generation", Workspace: binding.Worktree.Path, Branch: binding.Worktree.Branch}}
	record := &runstate.Record{ID: id, InputDigest: "accepted", StopEvidenceScope: adapter.AgentSessionStopScope, Snapshot: s, Profile: []b.ImplementationDataEnvelope{pe}}
	run, err := ReadStructuredRun(m, view, record)
	if err != nil {
		t.Fatal(err)
	}
	return m, view, record, run
}
func TestStructuredRunRequiresOriginalEnvelopeAndPublicIdentity(t *testing.T) {
	for _, fault := range []string{"none", "missing-record", "missing-state", "duplicate-state", "changed-envelope", "runtime", "attempt", "generation", "scope", "session", "agent", "workspace", "profile", "missing-profile", "old-protocol"} {
		t.Run(fault, func(t *testing.T) {
			m, v, r, _ := structuredTestFixture(t)
			switch fault {
			case "missing-record":
				r = nil
			case "missing-state":
				r.Snapshot.Implementation = nil
			case "duplicate-state":
				r.Snapshot.Implementation = append(r.Snapshot.Implementation, r.Snapshot.Implementation[0])
			case "changed-envelope":
				r.Snapshot.Implementation[0].Generation = "other"
			case "runtime":
				m.LocalRuntimeID = "other"
			case "attempt":
				v.Execution.AttemptID = "other"
			case "generation":
				v.Execution.Generation = "other"
			case "scope":
				v.StopEvidenceScope = "orca_terminal"
			case "session":
				v.Execution.SessionID = "oas_foreign"
			case "agent":
				v.Execution.Agent = "pi"
			case "workspace":
				v.Execution.Workspace = "other"
			case "profile":
				r.Profile[0].Payload = []byte(`{}`)
			case "missing-profile":
				r.Profile = nil
			case "old-protocol":
				v.Execution.Protocol = "orca_terminal_v3"
			}
			_, err := ReadStructuredRun(m, v, r)
			if (err == nil) != (fault == "none") {
				t.Fatal(fault, err)
			}
		})
	}
}

type structuredHistoryFixture struct {
	pages   []native.AgentPage
	detail  native.AgentItem
	failure error
	reads   int
}

func (f *structuredHistoryFixture) History(_ context.Context, _ string, _ *native.AgentCursor, _ int) (native.AgentPage, error) {
	if f.failure != nil {
		return native.AgentPage{}, f.failure
	}
	n := f.reads
	f.reads++
	if n >= len(f.pages) {
		return native.AgentPage{}, errors.New("unexpected page")
	}
	return f.pages[n], nil
}
func (f *structuredHistoryFixture) Item(context.Context, string, string) (native.AgentItem, error) {
	return f.detail, f.failure
}
func structuredHistoryFixtureFor(run StructuredRun) *structuredHistoryFixture {
	raw, _ := json.Marshal(map[string]any{"kind": "message", "role": "user", "blocks": []native.AgentTextBlock{{Type: "text", Text: "original input"}}})
	user := native.AgentItem{ItemID: "orca:" + run.Binding.MessageID, Sequence: 1, Body: raw}
	raw, _ = json.Marshal(map[string]any{"kind": "turn", "turnId": run.Binding.TurnID, "userItemId": user.ItemID, "state": "running", "startedAt": 1})
	turn := native.AgentItem{ItemID: run.Binding.TurnItemID, Sequence: 2, Body: raw}
	page := native.AgentPage{SessionID: run.Binding.SessionID, Epoch: run.Binding.Epoch, Working: true, Items: []native.AgentItem{user, turn}, Submissions: []native.AgentSubmission{{ClientMessageID: run.Binding.MessageID, PayloadFingerprint: run.Binding.BodyFingerprint, ProviderItemID: run.Binding.ProviderItemID, DispatchState: "accepted"}}}
	return &structuredHistoryFixture{pages: []native.AgentPage{page}}
}
func TestStructuredHistoryRequiresCompleteOriginalInputs(t *testing.T) {
	for _, fault := range []string{"none", "paged", "detail", "query-error", "epoch", "session", "newer", "removed", "missing-user", "foreign-user", "changed-user", "missing-turn", "changed-turn", "missing-submission", "duplicate-submission", "changed-submission", "cursor", "same-cursor", "shortened-detail", "detail-order", "changed-overlap"} {
		t.Run(fault, func(t *testing.T) {
			_, _, _, run := structuredTestFixture(t)
			f := structuredHistoryFixtureFor(run)
			p := &f.pages[0]
			switch fault {
			case "query-error":
				f.failure = errors.New("host unavailable")
			case "epoch":
				p.Epoch = "other"
			case "session":
				p.SessionID = "other"
			case "newer":
				p.HasNewer = true
			case "removed":
				p.RemovedItemIDs = []string{"lost"}
			case "missing-user":
				p.Items = p.Items[1:]
			case "foreign-user":
				p.Items[0].ItemID = "foreign"
			case "changed-user":
				p.Items[0].Body = json.RawMessage(`{"kind":"message","role":"user","blocks":[{"type":"text","text":"changed"}]}`)
			case "missing-turn":
				p.Items = p.Items[:1]
			case "changed-turn":
				p.Items[1].ItemID = "foreign-turn"
			case "missing-submission":
				p.Submissions = nil
			case "duplicate-submission":
				p.Submissions = append(p.Submissions, p.Submissions[0])
			case "changed-submission":
				p.Submissions[0].ProviderItemID = "foreign"
			case "cursor":
				p.HasOlder = true
			case "same-cursor":
				p.HasOlder = true
				p.Window.NextCursor = &native.AgentCursor{Epoch: p.Epoch, Sequence: 5}
				q := *p
				f.pages = append(f.pages, q)
			case "paged":
				q := *p
				q.Items = p.Items[:1]
				p.Items = p.Items[1:]
				p.HasOlder = true
				p.Window.NextCursor = &native.AgentCursor{Epoch: p.Epoch, Sequence: 2}
				f.pages = append(f.pages, q)
			case "detail", "shortened-detail", "detail-order":
				f.detail = p.Items[0]
				p.Items[0].Shortened = true
				if fault == "shortened-detail" {
					f.detail.Shortened = true
				}
				if fault == "detail-order" {
					f.detail.Sequence++
				}
			case "changed-overlap":
				q := *p
				q.Items = append([]native.AgentItem{}, p.Items...)
				q.Items[0].Revision++
				p.HasOlder = true
				p.Window.NextCursor = &native.AgentCursor{Epoch: p.Epoch, Sequence: 2}
				f.pages = append(f.pages, q)
			}
			history, err := ReadStructuredHistory(context.Background(), f, run)
			good := fault == "none" || fault == "paged" || fault == "detail"
			if (err == nil) != good {
				t.Fatal(fault, history, err)
			}
			if good && (history.Inputs != 1 || history.CurrentTurn.TurnID != run.Binding.TurnID || !strings.HasPrefix(history.Fingerprint, "1:")) {
				t.Fatal(history)
			}
		})
	}
}
func TestStructuredMergeGuardBindsRunAttemptAndAgent(t *testing.T) {
	m, v, _, _ := structuredTestFixture(t)
	m.BaseBranch = "codex/srtest/012345abcdef/base"
	v.Status = "pr_ready"
	v.PR = &contracts.PRFact{Repository: Repository, HeadRepository: Repository, BaseBranch: m.BaseBranch, HeadBranch: v.Execution.Branch, HeadRevision: strings.Repeat("a", 40), Number: 12, URL: "https://github.com/" + Repository + "/pull/12", State: "open"}
	v.Final = &contracts.FinalView{Verified: true, Source: "orca.agent-session.turn-result.v1", ItemID: "final", SessionID: v.Execution.SessionID, AttemptID: v.Execution.AttemptID, TurnID: v.Execution.TurnID, CommandID: "command"}
	if err := CheckPR(m, v); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"run", "attempt", "agent", "protocol", "session"} {
		bad := v
		switch field {
		case "run":
			bad.RunID = "other"
		case "attempt":
			bad.Execution.AttemptID = "other"
		case "agent":
			bad.Execution.Agent = "pi"
		case "protocol":
			bad.Execution.Protocol = "orca_terminal_v3"
		case "session":
			bad.Execution.SessionID = "orca-terminal:runtime:terminal:incarnation"
		}
		if CheckPR(m, bad) == nil {
			t.Fatal(field)
		}
	}
}

func TestStructuredRunKeepsOriginalCommittedInputProof(t *testing.T) {
	for _, fault := range []string{"none", "duplicate", "request", "missing-receipt", "foreign-receipt", "missing-proof"} {
		t.Run(fault, func(t *testing.T) {
			m, view, r, run := structuredTestFixture(t)
			s := &r.Snapshot
			s.Session.WorkflowRunID = r.ID
			input := b.TurnInput{CommandID: run.Binding.CommandID, Prompt: "original input"}
			raw, _ := json.Marshal(input)
			sum := sha256.Sum256(raw)
			ref := s.Ref
			op := b.Operation{Version: 1, ID: "original-operation", Action: string(b.LifecycleStartTurn), Status: b.OperationConfirmed, Request: raw, PayloadDigest: hex.EncodeToString(sum[:]), Ref: &ref, LifecycleIdentity: &b.LifecycleOperationIdentity{ProjectID: s.Session.ProjectID, ExecutionID: r.ID, ConfigurationDigest: s.ConfigurationDigest, ResourceGeneration: s.ResourceGeneration, InputBindingDigest: "original-binding", Ref: ref}}
			receipt := b.LifecycleReceipt{Operation: op, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{InputAccepted: true, SessionID: run.Binding.SessionID, TurnID: run.Binding.TurnID}, Implementation: s.Implementation}
			op.Receipt, _ = json.Marshal(receipt)
			s.Operations = []b.Operation{op}
			switch fault {
			case "duplicate":
				s.Operations = append(s.Operations, op)
			case "request":
				s.Operations[0].Request = []byte(`{"command_id":"changed","prompt":"changed"}`)
			case "missing-receipt":
				s.Operations[0].Receipt = nil
			case "foreign-receipt":
				receipt.State.SessionID = "other"
				s.Operations[0].Receipt, _ = json.Marshal(receipt)
			case "missing-proof":
				receipt.Implementation = nil
				s.Operations[0].Receipt, _ = json.Marshal(receipt)
			}
			got, err := ReadStructuredRun(m, view, r)
			if (err == nil) != (fault == "none") {
				t.Fatal(fault, err)
			}
			if err == nil && (len(got.Inputs) != 1 || FrozenStructuredFingerprint(got) != FrozenStructuredFingerprint(run)) {
				t.Fatal("original receipt binding changed", got)
			}
		})
	}
}
