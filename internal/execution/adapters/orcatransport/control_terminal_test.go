package orcatransport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixedControlFixture struct {
	setup                  codex.RemoteControlSetup
	ready                  codex.RemoteReadiness
	response               *codex.RemoteControlResponse
	mode                   string
	creates, sends, closes int
	requestIDs             []string
	closed                 bool
	authBudget             time.Duration
	stream                 []string
	streamOldest           int
	readCursors            []int
}

func fixtureFixedControl(t *testing.T) (*Lifecycle, *fakeHost, b.LifecycleSnapshot, *fixedControlFixture) {
	t.Helper()
	l, f, s := fixture(t)
	f.nativePath = filepath.Join(t.TempDir(), "local-owned-workspace")
	f.callerFileWriter = func(path string, raw []byte) error {
		rel, err := filepath.Rel(l.setup.RunDirectory, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("foreign checkpoint")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		return os.WriteFile(path, raw, 0600)
	}
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	l.profile.RuntimeExecutable, l.profile.RuntimeSHA256, l.profile.RuntimeConfigurationDigest = "C:/runtime/helper.exe", strings.Repeat("b", 64), strings.Repeat("a", 64)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	c := &fixedControlFixture{ready: codex.RemoteReadiness{ExitPublicKey: public, Owner: b.ProcessIdentity{Kind: "os", PID: 124, Created: "owner-creation", Executable: "helper"}, NativeTransport: codex.RemoteNativeTransport, Verified: true, Contract: codex.PolicyContract, Workspace: s.Workspace.Path, Repository: "https://github.com/erranliu/SteerlineTest.git", SessionID: "01a1433a-c857-7fc0-bf8c-02a784ad118e", RuntimeSHA256: l.profile.RuntimeSHA256, ConfigurationDigest: l.profile.RuntimeConfigurationDigest, Process: b.ProcessIdentity{Kind: "os", PID: 123, Created: "native-process", Executable: "codex"}, CheckedAt: now}}
	l.setup.RepositoryURL = c.ready.Repository
	l.readStopArtifact = func(context.Context, string, string, string, string) ([]byte, error) {
		body, _ := json.Marshal(codex.RemoteExit{Readiness: c.ready, WritersAbsent: true, ClosedAt: time.Now().UTC()})
		return json.Marshal(map[string]any{"body": json.RawMessage(body), "signature": ed25519.Sign(private, body)})
	}
	original := l.client
	l.client = native.New(native.Config{CommandTimeout: "1s", Output: func(ctx context.Context, d time.Duration, args ...string) ([]byte, error) {
		arg := func(name string) string {
			for i, a := range args {
				if a == name && i+1 < len(args) {
					return args[i+1]
				}
			}
			return ""
		}
		control := len(args) > 1 && args[0] == "terminal" && (args[1] == "create" || arg("--terminal") == "control" || args[1] == "list" && c.closed)
		if !control {
			return original.Raw(ctx, d, args...)
		}
		runtime, incarnation := s.Ref.External.RuntimeID, "control-inc"
		if c.mode == "wrong-incarnation" && args[1] == "show" {
			incarnation = "foreign"
		}
		terminal := map[string]any{"handle": "control", "incarnationId": incarnation, "worktreeId": s.Ref.External.WorktreeID, "connected": true, "status": "running"}
		var result any
		switch args[1] {
		case "create":
			c.creates++
			parts := strings.Split(arg("--command"), "'")
			if len(parts) != 7 {
				t.Fatal("unexpected control command", arg("--command"))
			}
			raw, e := base64.RawURLEncoding.DecodeString(parts[5])
			if e != nil || json.Unmarshal(raw, &c.setup) != nil {
				t.Fatal("control setup")
			}
			if c.mode == "unknown-create" {
				return nil, errors.New("lost create")
			}
			result = map[string]any{"terminal": terminal}
		case "show":
			if c.closed {
				return nil, errors.New("closed")
			}
			result = map[string]any{"terminal": terminal}
		case "read":
			ready := c.setup
			if c.mode == "wrong-ready" {
				ready.RuntimeSHA256 = strings.Repeat("c", 64)
			}
			raw, _ := json.Marshal(ready)
			lines := []string{codex.RemoteControlReadyPrefix + base64.RawURLEncoding.EncodeToString(raw)}
			if c.response != nil && c.mode != "hide-response" {
				raw, _ = json.Marshal(c.response)
				lines = append(lines, codex.RemoteControlPrefix+base64.RawURLEncoding.EncodeToString(raw))
			}
			if c.stream != nil {
				lines = c.stream
			}
			cursor, _ := strconv.Atoi(arg("--cursor"))
			c.readCursors = append(c.readCursors, cursor)
			start := max(cursor, c.streamOldest)
			end := min(start+1000, c.streamOldest+len(lines))
			terminal["tail"] = lines[start-c.streamOldest : end-c.streamOldest]
			terminal["source"], terminal["oldestCursor"], terminal["truncated"], terminal["limited"] = "stream", strconv.Itoa(c.streamOldest), cursor < c.streamOldest, end < c.streamOldest+len(lines)
			terminal["nextCursor"], terminal["latestCursor"] = strconv.Itoa(end), strconv.Itoa(c.streamOldest+len(lines))
			result = map[string]any{"terminal": terminal}
		case "send":
			c.sends++
			if deadline, ok := ctx.Deadline(); ok {
				c.authBudget = time.Until(deadline)
			}
			var q codex.RemoteControlRequest
			raw, e := base64.RawURLEncoding.DecodeString(arg("--text"))
			if e != nil || json.Unmarshal(raw, &q) != nil || q.Validate(c.setup) != nil {
				t.Fatal("control request")
			}
			c.requestIDs = append(c.requestIDs, arg("--retry-request"))
			response := codex.RemoteControlResponse{ID: q.ID, Binding: q.Binding, RequestDigest: codex.RemoteControlDigest(q)}
			if q.Method == "workspace-audit" {
				response.Workspace = &codex.RemoteWorkspaceAudit{Readiness: c.ready, WritersAbsent: c.mode != "workspace-reader-live", Inspection: b.WorkspaceInspection{Repository: c.ready.Workspace, Worktree: q.Audit.Intent.Path, Branch: q.Audit.Intent.Branch, Base: q.Audit.Intent.BaseCommit, Head: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Qualified: true, InspectionScope: "working_tree", CheckedAt: time.Now().UTC()}}
				if c.mode == "workspace-reader-live" {
					response.Error = "workspace_audit_unverified"
					response.Workspace.Inspection.Qualified = false
				}
				if c.mode == "workspace-foreign-branch" {
					response.Workspace.Inspection.Branch = "foreign"
				}
				if c.mode == "workspace-dirty" {
					response.Workspace.Inspection.Dirty = true
				}
			} else if q.Method == "original-turn" {
				value := codex.RemoteTurnReceipt{ReaderWritersAbsent: true, Source: codex.RemoteTurnSource, Readiness: c.ready, InputDigest: q.InputDigest, NativeTurnID: "01a14502-5798-78f1-aaa3-2d7cd15594c8", UserItemID: "user", Started: true, Completed: true, Status: "completed", FinalItemID: "final", FinalText: "exact native final", FinalDigest: digest([]byte("exact native final"))}
				switch c.mode {
				case "reader-unverified", "query-failed":
					value.ReaderWritersAbsent = c.mode == "query-failed"
					value.InspectionFailed = true
					response.Error = "original_turn_unverified"
				case "wrong-session":
					value.Readiness.SessionID = "foreign"
				case "wrong-config":
					value.Readiness.ConfigurationDigest = "foreign"
				case "wrong-input":
					value.InputDigest = strings.Repeat("0", 64)
				case "failed":
					value.Status = "failed"
				case "in-progress":
					value.Completed = false
					value.Status = "inProgress"
					value.FinalText = ""
					value.FinalItemID = ""
					value.FinalDigest = ""
				}
				response.Turn = &value
			} else {
				t.Fatal("unsupported fixture control query", q.Method)
			}

			if c.mode == "wrong-response" {
				response.RequestDigest = "foreign"
			}
			c.response = &response
			if c.stream != nil {
				raw, _ := json.Marshal(response)
				c.stream = append(c.stream, codex.RemoteControlPrefix+base64.RawURLEncoding.EncodeToString(raw))
			}
			if c.mode == "lost-send" {
				c.mode = ""
				return nil, errors.New("lost send response")
			}
			result = map[string]any{"send": map[string]any{"handle": "control", "accepted": true, "prompt": map[string]any{"requestId": arg("--retry-request"), "processIncarnation": "control-inc"}}}
		case "close":
			c.closes++
			c.closed = true
			if c.mode == "lost-close" {
				return nil, errors.New("lost close")
			}
			result = map[string]any{"close": map[string]any{"handle": "control", "ptyKilled": true}}
		case "list":
			result = map[string]any{"terminals": []any{}, "totalCount": 0, "truncated": false, "hostScope": map[string]any{"hostIds": []string{"local"}, "omittedHostIds": []string{}}}
		default:
			t.Fatal(args)
		}
		return json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]any{"runtimeId": runtime}})
	}})
	return l, f, s, c
}

func prepareFixedControl(t *testing.T, l *Lifecycle, s b.LifecycleSnapshot, c *fixedControlFixture) b.LifecycleSnapshot {
	t.Helper()
	state, _ := l.state(s)
	intent := l.controlIntent(s, state)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecycleCreateResource, b.LifecycleInput{Resource: &intent}))
	if err != nil || r.Disposition != b.OperationConfirmed {
		t.Fatal(r, err)
	}
	bind(&s, r)
	state, err = l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	state.RemoteReadiness = &c.ready
	s.Implementation, _ = l.envelope(s, state)
	baseline := fixtureState(1, "READY", "done")
	baseline.ProviderSession.ID = c.ready.SessionID
	l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) {
		return &controlledStateFeed{initial: baseline, current: baseline}, nil
	}
	return s
}

func TestFixedControlInitializationUnknownBlocksWithoutReplacement(t *testing.T) {
	for _, mode := range []string{"unknown-create", "wrong-ready"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			c.mode = mode
			state, _ := l.state(s)
			intent := l.controlIntent(s, state)
			q := request(s, b.LifecycleCreateResource, b.LifecycleInput{Resource: &intent})
			r, _, err := l.Execute(context.Background(), q)
			if err == nil || r.Disposition == b.OperationConfirmed {
				t.Fatal("initialization accepted", r, err)
			}
			if mode == "wrong-ready" {
				bind(&s, r)
				q.Snapshot = s
			}
			_, _, _ = l.Reconcile(context.Background(), q)
			if c.creates != 1 || c.sends != 0 || f.nativeSends != 0 {
				t.Fatal("unknown initialization dispatched work", c.creates, c.sends, f.nativeSends)
			}
		})
	}
}

func TestFixedControlReceiptRecovery(t *testing.T) {
	for _, mode := range []string{"success", "lost-send", "wrong-session", "wrong-config", "wrong-input", "wrong-response", "wrong-incarnation", "failed", "in-progress"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			f.unstarted = true
			r, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			before := s.Clone()
			c.mode = mode
			l.clock = func() time.Time { return c.ready.CheckedAt.Add(31 * time.Second) }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "lost-send" {
				if r.TurnEvidence != nil {
					t.Fatal("uncertain send advanced")
				}
				l = &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, stateFactory: l.stateFactory, clock: l.clock}
				r, _, err = l.observeOnce(context.Background(), before)
				if err != nil {
					t.Fatal(err)
				}
			}
			want := mode == "success" || mode == "lost-send"
			if (r.TurnEvidence != nil) != want {
				t.Fatal("incorrect completion", r.Diagnostic, r.TurnEvidence)
			}
			if want && r.TurnEvidence.LastMessage != "exact native final" {
				t.Fatal("final replaced", r.TurnEvidence)
			}
			if c.creates != 1 || c.closes != 0 || f.nativeSends != 1 {
				t.Fatal("recovery created resources or resent business", c.creates, c.closes, f.nativeSends)
			}
		})
	}
}

func TestFixedControlRunStopAndServiceRelease(t *testing.T) {
	for _, mode := range []string{"success"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			if facts, err := l.ShutdownLifecycle(context.Background(), s, nil); err == nil || facts.Stopped || c.closes != 0 || c.sends != 0 {
				t.Fatal("service shutdown changed execution resources", err, c.closes, c.sends)
			}
			c.mode = mode
			r, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), nil)
			businessClosed := false
			for _, call := range f.calls {
				if len(call) > 3 && call[0] == "terminal" && call[1] == "close" && call[3] == s.Ref.External.Terminal {
					businessClosed = true
				}
			}
			wantCloses := 1
			if mode == "bad-auth" {
				wantCloses = 0
			}
			if !businessClosed || c.closes != wantCloses || c.creates != 1 {
				t.Fatal("stop did not close original resources", businessClosed, c.closes, c.creates)
			}
			if (err == nil && r.State.Stopped) != (mode == "success") {
				t.Fatal("revocation uncertainty overstated", r, err)
			}
			if mode == "success" {
				if _, err := l.ShutdownLifecycle(context.Background(), s, nil); err != nil {
					t.Fatal("stopped control blocked service release", err)
				}
			}
		})
	}
}

func TestFixedControlWindowsPathSpellings(t *testing.T) {
	l, _, s, c := fixtureFixedControl(t)
	c.ready.Workspace = strings.ToUpper(strings.ReplaceAll(c.ready.Workspace, "/", "\\"))
	s = prepareFixedControl(t, l, s, c)
	state, err := l.state(s)
	if err != nil {
		t.Fatal("same Windows workspace lost control binding", err)
	}
	if err := l.validateControl(l.setup.Snapshot, state, true); err != nil {
		t.Fatal(err)
	}
	foreign := *state.RemoteReadiness
	foreign.Workspace += "-other"
	state.RemoteReadiness = &foreign
	if err := l.validateControl(s, state, true); err == nil {
		t.Fatal("foreign workspace accepted")
	}
}

func TestFixedControlStopCancelsOnlyOriginalObserver(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "foreign"}[foreign], func(t *testing.T) {
			l, _, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			cancelled := false
			finished := make(chan struct{})
			close(finished)
			identity := s.Ref.External
			if foreign {
				identity.Incarnation = "foreign"
			}
			o := &observer{process: identity, finished: finished, cancel: func() { cancelled = true }}
			r, err := l.StopLifecycle(context.Background(), request(s, b.LifecycleStopExecution, b.LifecycleInput{Stop: &b.StopInput{}}), &observationHandle{owner: o})
			if foreign {
				if err == nil || cancelled || c.sends != 0 || c.closes != 0 {
					t.Fatal("foreign observer affected", err, cancelled, c.sends, c.closes)
				}
			} else if err != nil || !cancelled || !r.State.Stopped {
				t.Fatal("original observer not cancelled", r, err, cancelled)
			}
		})
	}
}

func TestFixedControlUnverifiedReceiptBackoff(t *testing.T) {
	l, f, s, c := fixtureFixedControl(t)
	s = prepareFixedControl(t, l, s, c)
	s, input := prepared(t, l, s, strings.Repeat("c", 32))
	f.unstarted = true
	r, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	c.mode = "wrong-response"
	l.clock = func() time.Time { return c.ready.CheckedAt.Add(31 * time.Second) }
	r, _, err = l.observeOnce(context.Background(), s)
	if err != nil || r.TurnEvidence != nil {
		t.Fatal(r, err)
	}
	bind(&s, r)
	before := c.sends
	r, _, err = l.observeOnce(context.Background(), s)
	if err != nil || r.TurnEvidence != nil || c.sends != before {
		t.Fatal("unverified query retried without backoff", r, err, c.sends, before)
	}
}

func TestFixedControlTransportCheckpointFailure(t *testing.T) {
	for _, phase := range []string{"intent", "response"} {
		t.Run(phase, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			state, _ := l.state(s)
			write := f.callerFileWriter
			writes := 0
			f.callerFileWriter = func(path string, raw []byte) error {
				writes++
				if phase == "intent" || writes == 2 {
					return errors.New("checkpoint write failed")
				}
				return write(path, raw)
			}
			if err := inspectHostNativeControl(l, state); err == nil {
				t.Fatal("storage fault hidden")
			}
			if phase == "intent" {
				if c.sends != 0 {
					t.Fatal("uncommitted control input sent")
				}
				return
			}
			f.callerFileWriter = write
			l = &Lifecycle{setup: l.setup, profile: l.profile, client: l.client, clock: l.clock}
			if err := inspectHostNativeControl(l, state); err != nil {
				t.Fatal("original response not recovered", err)
			}
			if c.sends != 1 || c.creates != 1 {
				t.Fatal("commit recovery repeated effect", c.sends, c.creates)
			}
		})
	}
}

func TestFixedControlQueryFailurePreservesNativeReaderSettlement(t *testing.T) {
	for _, mode := range []string{"query-failed", "reader-unverified"} {
		t.Run(mode, func(t *testing.T) {
			l, f, s, c := fixtureFixedControl(t)
			s = prepareFixedControl(t, l, s, c)
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			f.unstarted = true
			r, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			state, _ := l.state(s)
			c.mode = mode
			if _, err = l.recordedTurn(context.Background(), &state, &state.Turns[0]); err == nil {
				t.Fatal("failed history became business success")
			}
			j, err := l.loadControlCheckpoint(state)
			if err != nil || j.Response == nil || j.Response.Turn == nil || !j.Response.Turn.InspectionFailed {
				t.Fatal("query failure discarded original resource proof", j, err)
			}
			before := c.sends
			err = l.closeControl(context.Background(), state)
			if mode == "reader-unverified" {
				if err == nil || c.sends != before {
					t.Fatal("unverified reader proof overwritten", err, c.sends)
				}
				after, _ := l.loadControlCheckpoint(state)
				if after.Request.ID != j.Request.ID {
					t.Fatal("original query lost")
				}
			} else if err != nil || c.sends != before || c.creates != 1 {
				t.Fatal("settled failed query blocked original revocation", err, c.sends, c.creates)
			}
		})
	}
}

func inspectHostNativeControl(l *Lifecycle, state State) error {
	intent := l.setup.Snapshot.WorkspacePlan
	intent.Path = state.RemoteReadiness.Workspace
	if intent.Branch == "" {
		intent.Branch = "main"
	}
	_, err := l.callControlWithAudit(context.Background(), state, "workspace-audit", "", &codex.RemoteWorkspaceAuditQuery{Intent: intent})
	return err
}
