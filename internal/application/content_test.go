package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func contentEngine(t *testing.T, id string) (*Engine, *worker) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	w := &worker{wake: make(chan struct{}, 1), port: auditPort{reportPolicyPort{}}}
	e := &Engine{dir: dir, ctx: context.Background(), epoch: "epoch", logger: log.New(io.Discard, "", 0), store: store, runs: map[string]*runstate.Record{}, workers: map[string]*worker{"run": w}}
	request := contracts.RunRequest{Title: "test", Workflow: contracts.Workflow{ID: id, Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"access_mode":"read_only"}`)}}
	if id == "generate" {
		request.Generate = &contracts.GenerateInput{Instruction: "Reply", Materials: []contracts.Document{}}
	} else {
		request.Conversation = &contracts.ConversationInput{Instruction: "Discuss", Materials: []contracts.Document{}, Directory: contracts.DirectoryAccess{Path: t.TempDir(), Access: "read_only"}}
	}
	r := &runstate.Record{ID: "run", Request: request, Allowed: true, Status: "queued", InputDigest: "digest", CommandStates: map[string]string{}}
	r, err = freezeContent(dir, r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
	if err != nil {
		t.Fatal(err)
	}
	if err = queueInput(r, workflow.StageInput{StepID: "execute"}, ""); err != nil {
		t.Fatal(err)
	}
	r.Snapshot.Session.SessionID = "original-session"
	if err = e.commitLocked(r, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	return e, w
}

func TestContentMaterialSnapshot(t *testing.T) {
	material := "fixed submitted material"
	r := &runstate.Record{ID: "snapshot", InputDigest: "fixed", Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "generate", Version: 1}, Generate: &contracts.GenerateInput{Instruction: "Use the material", Materials: []contracts.Document{{Kind: "inline", Path: "source.txt", Content: &material}}}, Execution: contracts.Execution{Settings: json.RawMessage(`{"access_mode":"read_only"}`)}}}
	r, err := freezeContent(t.TempDir(), r, "attempt", "generation", canonical(codex.ProfileConfig{Executable: os.Args[0], AccessMode: "read_only"}))
	if err != nil {
		t.Fatal(err)
	}
	material = "changed after acceptance"
	prompt, err := contentPrompt(r, "")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(r.Materials[0].SnapshotPath)
	if err != nil || string(stored) != "fixed submitted material" || r.Materials[0].Digest != reliable.Digest(stored) || !strings.Contains(prompt, "fixed submitted material") || strings.Contains(prompt, "changed after acceptance") {
		t.Fatal("accepted materials were replaced", err, prompt)
	}
}

func TestContentConcurrentMessages(t *testing.T) {
	e, w := contentEngine(t, "conversation")
	e.finish("run", contentEvidence(e, "done", "succeeded", true))
	revision := e.runs["run"].Revision
	var responses [2]contracts.Response
	var wg sync.WaitGroup
	for i := range responses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			responses[i] = contentControl(e, "message", []string{"first", "second"}[i], contracts.MessageInput{ExpectedRevision: revision, Text: []string{"first body", "second body"}[i]})
		}(i)
	}
	wg.Wait()
	accepted := 0
	for _, r := range responses {
		if r.OK {
			accepted++
		} else if r.Error.Code != "stale_revision" {
			t.Fatal(r)
		}
	}
	if accepted != 1 || e.runs["run"].PendingFeedback == nil || len(e.runs["run"].Workflow.StageInputs) != 1 {
		t.Fatal("concurrent input crossed serial boundary")
	}
	prior := e.runs["run"].PendingFeedback.CommandID
	before := string(canonical(e.runs["run"]))
	restarted, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical(restarted.runs["run"])) != before {
		t.Fatal("restart changed the original accepted message or authorization")
	}
	t.Cleanup(restarted.cancel)
	if !restarted.feedback("run", w, w.port) {
		t.Fatal("original authorized message did not freeze after restart")
	}
	r := restarted.runs["run"]
	if r.PendingFeedback != nil || r.CommandStates[prior] != "running" || len(r.Workflow.StageInputs) != 2 || r.Snapshot.Session.SessionID != "original-session" || r.Workflow.StageInputs[1].FeedbackDecisionID != prior {
		t.Fatal("message lost its original identity or was duplicated")
	}
	second, err := New(context.Background(), e.dir, e.store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.cancel)
	if second.feedback("run", w, w.port) || len(second.runs["run"].Workflow.StageInputs) != 2 || string(canonical(second.runs["run"])) != string(canonical(r)) {
		t.Fatal("second restart recreated an already frozen message")
	}
	r = e.runs["run"].Clone()
	closeContent(r, "close-pending")
	if r.PendingFeedback != nil || r.CommandStates[prior] != "failed" {
		t.Fatal("closing left an unsent message pending")
	}
}

func TestContentRecoveryKeepsContractsAndRejectsForeignMessageScope(t *testing.T) {
	for _, id := range []string{"conversation", "generate"} {
		t.Run(id, func(t *testing.T) {
			e, _ := contentEngine(t, id)
			for _, phase := range []string{"queued", "result"} {
				if phase == "result" && !e.finish("run", contentEvidence(e, "original result", "succeeded", true)) {
					t.Fatal("original content result not accepted")
				}
				r := e.runs["run"]
				before := string(canonical(r))
				items := TakeoverFacts([]*runstate.Record{r}, nil, resources.RecoveryCapabilities(), workflow.Catalog())
				if len(items) != 1 || items[0].Status != "compatible" || string(canonical(r)) != before {
					t.Fatal("content contract rejected or rewritten", phase, items)
				}
				changed := r.Clone()
				changed.Profile[0].CodecVersion = 99
				if item := TakeoverFacts([]*runstate.Record{changed}, nil, resources.RecoveryCapabilities(), workflow.Catalog())[0]; item.Status != "unsupported" {
					t.Fatal("content bypassed codec validation", phase, item)
				}
			}
		})
	}
	for _, changed := range []string{"session", "legacy_epoch"} {
		t.Run(changed, func(t *testing.T) {
			e, w := contentEngine(t, "conversation")
			e.finish("run", contentEvidence(e, "done", "succeeded", true))
			if !contentControl(e, "message", "saved", contracts.MessageInput{ExpectedRevision: e.runs["run"].Revision, Text: "original body"}).OK {
				t.Fatal("message was not accepted")
			}
			r := e.runs["run"].Clone()
			if changed == "session" {
				r.Snapshot.Session.SessionID = "foreign-session"
			} else {
				r.PendingFeedback.Epoch = "previous-service-epoch"
			}
			e.runs["run"] = r
			if e.feedback("run", w, w.port) || len(e.runs["run"].Workflow.StageInputs) != 1 || e.runs["run"].Phase != "fresh_feedback_required" {
				t.Fatal("changed or unbound original message reached content input")
			}
		})
	}
}

func TestContentAdmissionTakeoverDoesNotRequireDevelopmentParser(t *testing.T) {
	for _, id := range []string{"generate", "conversation"} {
		t.Run(id, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "service"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "service", "result-acquisition.json"), []byte("invalid development parser configuration"), 0600); err != nil {
				t.Fatal(err)
			}
			material := "frozen content material"
			docs := []contracts.Document{{Kind: "inline", Path: "source.txt", Content: &material}}
			request := contracts.RunRequest{Title: "content takeover", Workflow: contracts.Workflow{ID: id, Version: 1}, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: canonical(contracts.CodexSettings{Executable: os.Args[0], AccessMode: "read_only"})}}
			if id == "generate" {
				request.Generate = &contracts.GenerateInput{Instruction: "Generate from source", Materials: docs}
			} else {
				request.Conversation = &contracts.ConversationInput{Instruction: "Discuss source", Materials: docs, Directory: contracts.DirectoryAccess{Path: t.TempDir(), Access: "read_only"}}
			}
			r, err := freeze(context.Background(), dir, contracts.Request{Protocol: contracts.Protocol, RequestID: contracts.NewID(), Input: canonical(request)})
			if err != nil {
				t.Fatal(err)
			}
			before := string(canonical(r))
			items := TakeoverFacts([]*runstate.Record{r}, nil, resources.RecoveryCapabilities(), workflow.Catalog())
			if len(items) != 1 || items[0].Status != "compatible" || string(canonical(r)) != before {
				t.Fatal("production content admission cannot be taken over unchanged", items)
			}
			if err := os.WriteFile(r.Materials[0].SnapshotPath, []byte("replaced material"), 0600); err != nil {
				t.Fatal(err)
			}
			if item := TakeoverFacts([]*runstate.Record{r}, nil, resources.RecoveryCapabilities(), workflow.Catalog())[0]; item.Status != "unsupported" {
				t.Fatal("content bypassed frozen material verification", item)
			}
		})
	}
}

type contentStopHandle struct{ identity b.ProcessIdentity }

func (h contentStopHandle) Identity() b.ProcessIdentity { return h.identity }
func (contentStopHandle) Kill()                         {}
func (contentStopHandle) Close()                        {}

type contentCleanupFailure struct{ auditPort }

func (contentCleanupFailure) ShutdownLifecycle(context.Context, b.LifecycleSnapshot, b.StopHandle) (b.LifecycleStateFacts, error) {
	return b.LifecycleStateFacts{}, errors.New("cleanup unavailable")
}

type contentInterruptReceiptPort struct {
	auditPort
	disposition b.OperationStatus
}

func (p contentInterruptReceiptPort) Execute(_ context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	return b.LifecycleReceipt{Operation: request.Operation, Disposition: p.disposition}, nil, nil
}

func TestContentInterruptRequestStatus(t *testing.T) {
	for _, tc := range []struct {
		disposition b.OperationStatus
		want        string
	}{{b.OperationRejected, "failed"}, {b.OperationUnknown, "unknown"}} {
		t.Run(tc.want, func(t *testing.T) {
			e, w := contentEngine(t, "conversation")
			_ = contentEvidence(e, "", "succeeded", true)
			response := contentControl(e, "interrupt", "interrupt-receipt", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision})
			if !response.OK {
				t.Fatal(response)
			}
			r := e.runs["run"].Clone()
			command := r.PendingInterrupt
			w.port = contentInterruptReceiptPort{disposition: tc.disposition}
			e.interruptContent("run", w, w.port, r)
			if r = e.runs["run"]; r.CommandStates[command] != tc.want || tc.want == "failed" && r.PendingInterrupt != "" || len(r.Workflow.StageInputs) != 1 {
				t.Fatal("original interrupt request has incorrect outcome", r.CommandStates)
			}
			query := e.Handle("inspect", "", "interrupt-receipt", "", "", 0, false, nil)
			if !query.OK {
				t.Fatal(query)
			}
			raw, _ := json.Marshal(query.Data)
			var view struct {
				Command struct {
					Status string `json:"action_status"`
				} `json:"command"`
			}
			_ = json.Unmarshal(raw, &view)
			if view.Command.Status != tc.want {
				t.Fatal("public original-request query lost interrupt outcome", string(raw))
			}
		})
	}
}

func TestContentCleanupFailureIndependent(t *testing.T) {
	e, w := contentEngine(t, "generate")
	e.finish("run", contentEvidence(e, "generated", "succeeded", true))
	r := e.runs["run"].Clone()
	r.Snapshot.Ref.External = b.ProcessIdentity{Kind: "os", PID: 42, Created: "original"}
	if err := e.commitLocked(r, nil, "bound"); err != nil {
		t.Fatal(err)
	}
	w.handle = contentStopHandle{r.Snapshot.Ref.External}
	w.port = contentCleanupFailure{}
	e.stopRun("run", w, w.port)
	r = e.runs["run"]
	if r.ContentResult.Status != "succeeded" || r.ContentResult.Text != "generated" || r.StopStatus != "unknown" || r.Status == "completed" {
		t.Fatal("cleanup failure concealed by result", r.View())
	}
}
func contentEvidence(e *Engine, text, outcome string, verified bool) b.TurnEvidence {
	r := e.runs["run"].Clone()
	input := r.Snapshot.PendingTurn
	r.Snapshot.Session.CurrentCommandID = input.CommandID
	r.Snapshot.Session.CurrentTurnID = "turn-" + input.CommandID
	r.Snapshot.Session.PendingCommandID = ""
	r.Snapshot.Session.PendingTurnID = ""
	e.runs["run"] = r
	return b.TurnEvidence{CommandID: input.CommandID, SessionID: r.Snapshot.Session.SessionID, TurnID: r.Snapshot.Session.CurrentTurnID, Started: true, Completed: true, Succeeded: outcome == "succeeded", NativeOutcome: outcome, LastMessage: text, FinalVerified: verified, FinalSource: "native.final", FinalItemID: "item", FinalNativeSessionID: r.Snapshot.Session.SessionID, FinalNativeTurnID: r.Snapshot.Session.CurrentTurnID}
}
func contentControl(e *Engine, command, id string, body any) contracts.Response {
	raw, _ := json.Marshal(body)
	return e.Handle(command, "run", "", "", "", 0, false, &contracts.Request{Protocol: contracts.Protocol, RequestID: id, Input: raw})
}

func TestContentGenerateResults(t *testing.T) {
	for _, tc := range []struct {
		name, text, schema, outcome, want string
		verified                          bool
	}{
		{"text", "完整内容\n第二行", "", "succeeded", "succeeded", true},
		{"json", `{"ok":true}`, `{"type":"object","required":["ok"],"properties":{"ok":{"const":true}}}`, "succeeded", "succeeded", true},
		{"invalid_json", "bad", `{"type":"object"}`, "succeeded", "failed", true},
		{"schema", `{"ok":false}`, `{"required":["name"]}`, "succeeded", "failed", true},
		{"failed", "looks good", "", "failed", "failed", true},
		{"cancelled", "looks good", "", "cancelled", "cancelled", true},
		{"no_final", "looks good", "", "succeeded", "failed", false},
		{"unknown", "looks good", "", "unknown", "failed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := contentEngine(t, "generate")
			r := e.runs["run"]
			if tc.schema != "" {
				r.Request.Generate.OutputSchema = json.RawMessage(tc.schema)
			}
			value := contentEvidence(e, tc.text, tc.outcome, tc.verified)
			if !e.finish("run", value) {
				t.Fatal("result not accepted")
			}
			r = e.runs["run"]
			if r.ContentResult.Status != tc.want || !r.Closed || r.StopStatus != "pending" || r.Status != "settling" || len(r.Workflow.StageInputs) != 1 || r.NextStage != nil || r.Snapshot.PendingTurn != nil || r.PR != nil {
				t.Fatalf("wrong result/settlement: %+v", r.View())
			}
			if tc.name == "invalid_json" || tc.name == "schema" {
				prefix := "invalid_json:"
				if tc.name == "schema" {
					prefix = "schema_mismatch:"
				}
				if !strings.HasPrefix(r.ContentResult.Diagnostic, prefix) || r.ContentResult.Text != "" || len(r.ContentResult.JSON) != 0 {
					t.Fatalf("invalid structured output was retained or failed for the wrong reason: %+v", r.ContentResult)
				}
				stored, err := e.store.Runs()
				if err != nil || len(stored) != 1 || string(canonical(stored[0].ContentResult)) != string(canonical(r.ContentResult)) {
					t.Fatal("structured output rejection was not persisted", err)
				}
			}
			if e.finish("run", value) {
				t.Fatal("duplicate completion advanced")
			}
			if len(r.Workflow.Results) != 0 {
				t.Fatal("development acquisition used")
			}
		})
	}
}

func TestContentConversationControls(t *testing.T) {
	e, w := contentEngine(t, "conversation")
	value := contentEvidence(e, "first", "succeeded", true)
	if !e.finish("run", value) {
		t.Fatal("finish")
	}
	r := e.runs["run"]
	if r.Status != "waiting_input" || !r.View().Controls.CanMessage || r.Closed {
		t.Fatal(r.View())
	}
	input := contracts.MessageInput{ExpectedRevision: r.Revision, Text: "second"}
	response := contentControl(e, "message", "message-1", input)
	if !response.OK {
		t.Fatal(response)
	}
	if duplicate := contentControl(e, "message", "message-1", input); !duplicate.OK {
		t.Fatal(duplicate)
	}
	if competing := contentControl(e, "message", "message-2", contracts.MessageInput{ExpectedRevision: e.runs["run"].Revision, Text: "concurrent"}); competing.OK {
		t.Fatal("concurrent input accepted")
	}
	if !e.feedback("run", w, w.port) {
		t.Fatal("message not queued")
	}
	if e.runs["run"].Snapshot.Session.SessionID != "original-session" || len(e.runs["run"].Workflow.StageInputs) != 2 {
		t.Fatal("new session or lost message")
	}
	value = contentEvidence(e, "", "cancelled", false)
	r = e.runs["run"]
	response = contentControl(e, "interrupt", "interrupt-1", contracts.RevisionInput{ExpectedRevision: r.Revision})
	if !response.OK {
		t.Fatal(response)
	}
	if !e.finish("run", value) {
		t.Fatal("cancel result")
	}
	r = e.runs["run"]
	if r.Closed || !r.View().Controls.CanMessage || r.CommandStates[r.PendingInterrupt] == "pending" {
		t.Fatal(r.View())
	}
	response = contentControl(e, "close", "close-1", contracts.RevisionInput{ExpectedRevision: r.Revision})
	if !response.OK {
		t.Fatal(response)
	}
	if response = contentControl(e, "message", "after-close", contracts.MessageInput{ExpectedRevision: e.runs["run"].Revision, Text: "no"}); response.OK || response.Error.Code != "session_closed" {
		t.Fatal(response)
	}
	if response = contentControl(e, "resume", "resume-closed", contracts.RevisionInput{ExpectedRevision: e.runs["run"].Revision}); response.OK {
		t.Fatal("closed resumed")
	}
	page := contentHistory(e.runs["run"], "another:0", 10)
	if page.OK {
		t.Fatal("cross-run cursor")
	}
}

func TestContentDirectoryAndUnknownResult(t *testing.T) {
	for _, outcome := range []string{"unknown", "unreported"} {
		e, _ := contentEngine(t, "conversation")
		if e.finish("run", contentEvidence(e, "looks complete", outcome, true)) || e.runs["run"].View().Controls.CanMessage || e.runs["run"].Status != "reconciling" {
			t.Fatal("unverified native outcome allowed another turn")
		}
	}
	e, _ := contentEngine(t, "conversation")
	value := contentEvidence(e, "final", "succeeded", true)
	value.FinalNativeTurnID = "foreign"
	if e.finish("run", value) || e.runs["run"].ContentResult != nil || e.runs["run"].Status != "reconciling" {
		t.Fatal("foreign final accepted")
	}
	r := e.runs["run"].Clone()
	r.Snapshot.Ref.WorkspacePath = filepath.Join(t.TempDir(), "missing")
	if verifyContentDirectory(r) == nil {
		t.Fatal("replaced directory accepted")
	}
	// A prior explicit authorization gate survives service generations.
	r = e.runs["run"].Clone()
	r.Allowed = false
	r.ResumeRequired = true
	if err := e.commitLocked(r, nil, "authorization_revoked"); err != nil {
		t.Fatal(err)
	}
	if response := contentControl(e, "message", "without-resume", contracts.MessageInput{ExpectedRevision: r.Revision, Text: "no"}); response.OK {
		t.Fatal("restart auto-authorized")
	}
	e, _ = contentEngine(t, "generate")
	value = contentEvidence(e, "final", "succeeded", true)
	r = e.runs["run"].Clone()
	if _, err := e.saveIntent("run", r, b.LifecycleCandidate{Action: b.LifecycleStartTurn, Input: b.LifecycleInput{Turn: r.Snapshot.PendingTurn}}); err != nil {
		t.Fatal(err)
	}
	if e.finish("run", value) || e.runs["run"].ContentResult != nil || len(e.runs["run"].Workflow.StageInputs) != 1 || e.runs["run"].Status != "reconciling" {
		t.Fatal("unknown operation advanced or requeued")
	}
}
