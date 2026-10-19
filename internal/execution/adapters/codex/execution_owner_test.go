package codexworker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
)

func ownerTestRequest(t *testing.T) b.LifecycleRequest {
	t.Helper()
	s := b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "run", AttemptID: "attempt", External: b.ProcessIdentity{Kind: "os", PID: 123, Created: "original", Executable: "original"}}, Session: b.SessionContext{ProjectID: "project", WorkflowRunID: "run", ExecutionID: "execution", AttemptID: "attempt"}, ConfigurationDigest: "config", ResourceGeneration: "generation"}
	i, err := operation.New("run", s, b.LifecycleCandidate{Action: b.LifecycleStopExecution, Input: b.LifecycleInput{Stop: &b.StopInput{TurnID: "turn"}}}, "stop", "stop", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return i.Request
}

func TestExecutionOwnerJournalWakesOriginalObservationWithoutNativeBroadcast(t *testing.T) {
	for _, kind := range []string{"completion", "question"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			l := ExecutionLifecycle{Backend: Backend{Config: Config{RunDir: dir}}}
			s := b.LifecycleSnapshot{Session: b.SessionContext{WorkflowRunID: "run"}}
			fixed := b.SessionContext{SessionID: "00000000-0000-0000-0000-000000000001", CurrentTurnID: "00000000-0000-0000-0000-000000000002", CurrentCommandID: "command"}
			path := filepath.Join(dir, "runs", "run", "turns", "command", "appserver.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			cancelled := make(chan struct{})
			observer := &Observation{Finished: make(chan struct{}), cancel: func() { close(cancelled) }}
			defer close(observer.Finished)
			l.watchOwnerJournal(s, fixed, observer)
			var event codexapp.Envelope
			if kind == "completion" {
				event.Method = "turn/completed"
				event.Params, _ = json.Marshal(map[string]any{"threadId": fixed.SessionID, "turn": map[string]any{"id": fixed.CurrentTurnID, "status": "completed"}})
			} else {
				event.Method, event.ID = "item/tool/call", json.RawMessage(`17`)
				event.Params, _ = json.Marshal(map[string]any{"threadId": fixed.SessionID, "turnId": fixed.CurrentTurnID, "callId": "original-call", "tool": codexapp.AskToolName, "arguments": map[string]any{"questions": []any{map[string]any{"id": "choice", "header": "Choice", "question": "Continue?", "allow_free_text": true}}}})
			}
			if err := os.WriteFile(path, encode(event), 0600); err != nil {
				t.Fatal(err)
			}
			if kind == "question" {
				if _, err := PendingAskEvents(path, fixed.SessionID, fixed.CurrentTurnID, nil); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-cancelled:
			case <-time.After(2 * time.Second):
				t.Fatal("owner-only native fact never woke read-only reattachment")
			}
		})
	}
}

func TestExecutionOwnerAllowsOnlyQualifiedLaterAttemptForOriginalIdleThread(t *testing.T) {
	called := 0
	l := ExecutionLifecycle{setup: b.LifecycleSetup{AttemptNumber: 1, ProcessHost: process.New(time.Second), LaunchExecutionOwner: func(context.Context, string) error { called++; return nil }}, Backend: Backend{Config: Config{ServerDir: t.TempDir()}}}
	r := ownerTestRequest(t)
	r.Snapshot.Ref.External = b.ProcessIdentity{}
	r.Snapshot.Session.SessionID = "original-thread"
	if l.ensureOwner(context.Background(), r) == nil || called != 0 {
		t.Fatal("legacy bound thread transferred without qualified manual recovery")
	}
	l.setup.AttemptNumber = 2
	r.Snapshot.Session.AttemptNumber = 2
	if err := l.ensureOwner(context.Background(), r); err != nil || called != 1 {
		t.Fatalf("qualified original thread recovery blocked: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(l.ownerDirectory(), ExecutionOwnerConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	var cfg ownerConfig
	if json.Unmarshal(raw, &cfg) != nil || cfg.Snapshot.Session.SessionID != "original-thread" {
		t.Fatal("manual recovery replaced original thread")
	}
}

func TestExecutionOwnerServiceCancellationClosesOnlyClientConnection(t *testing.T) {
	l := ExecutionLifecycle{Backend: Backend{Config: Config{ServerDir: t.TempDir()}, Process: ProcessPort{Inspect: func(*b.ProcessIdentity) string { return "matching process still exists" }}}}
	dir := l.ownerDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for name, value := range map[string]any{ExecutionOwnerConfigFile: ownerConfig{Token: "original", RuntimeSHA256: "original-sha"}, "owner-ready.json": ownerReady{Identity: ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: dir, RuntimeSHA256: "original-sha"}, Endpoint: listener.Addr().String()}} {
		if err = os.WriteFile(filepath.Join(dir, name), encode(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	connected := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		var q ownerQuery
		if json.NewDecoder(conn).Decode(&q) != nil {
			return
		}
		close(connected)
		<-release // Native work outlives the service-side read.
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	request := ownerTestRequest(t)
	go func() { _, err := l.ownerCall(ctx, request, false); completed <- err }()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("original owner connection not established")
	}
	cancel()
	select {
	case err = <-completed:
		if err == nil {
			t.Fatal("cancelled query succeeded without a receipt")
		}
	case <-time.After(time.Second):
		t.Fatal("service detach waited for native operation completion")
	}
}

func TestExecutionOwnerRejectsInputOutsideOriginalPayload(t *testing.T) {
	r := ownerTestRequest(t)
	if err := validateOwnerRequest(r); err != nil {
		t.Fatal(err)
	}
	r.Input.Stop.TurnID = "replacement"
	if validateOwnerRequest(r) == nil {
		t.Fatal("changed input accepted with original operation digest")
	}
}

func TestExecutionOwnerRejectsForeignRunConfigurationAndWorkspace(t *testing.T) {
	r := ownerTestRequest(t)
	if !sameOwnerExecution(r.Snapshot, r.Snapshot) {
		t.Fatal("original binding rejected")
	}
	for _, field := range []string{"run", "execution", "attempt", "generation", "config", "workspace"} {
		copy := r.Snapshot.Clone()
		switch field {
		case "run":
			copy.Session.WorkflowRunID = "other"
		case "execution":
			copy.Session.ExecutionID = "other"
		case "attempt":
			copy.Session.AttemptID = "other"
		case "generation":
			copy.ResourceGeneration = "other"
		case "config":
			copy.ConfigurationDigest = "other"
		case "workspace":
			copy.Workspace.Path = "other"
		}
		if sameOwnerExecution(r.Snapshot, copy) {
			t.Fatalf("foreign %s adopted", field)
		}
	}
}

func TestExecutionOwnerExitRequiresSignatureAndCompleteOriginalBinding(t *testing.T) {
	request := ownerTestRequest(t)
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := ExecutionOwnerIdentity{Directory: t.TempDir(), ExitPublicKey: hex.EncodeToString(public)}
	exit := ownerExit{Identity: id, Digest: ownerRequestDigest(request), Response: ownerResponse{Receipt: b.LifecycleReceipt{Operation: request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true}}}}
	body, _ := json.Marshal(exit)
	path := filepath.Join(id.Directory, "owner-stop.json")
	if err = os.WriteFile(path, encode(signedRemoteExit{Body: body, Signature: ed25519.Sign(key, body)}), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := readOwnerExit(id)
	if err != nil || !ownerExitMatchesSnapshot(saved, request.Snapshot) {
		t.Fatalf("original exit rejected: %v", err)
	}
	for _, field := range []string{"project", "config", "generation", "root", "action"} {
		t.Run(field, func(t *testing.T) {
			other := request.Clone()
			switch field {
			case "project":
				other.Snapshot.Session.ProjectID = "other"
			case "config":
				other.Snapshot.ConfigurationDigest = "other"
			case "generation":
				other.Snapshot.ResourceGeneration = "other"
			case "root":
				other.Snapshot.Ref.External.Created = "other"
			case "action":
				saved.Response.Receipt.Operation.Action = "start_turn"
			}
			if ownerExitMatchesSnapshot(saved, other.Snapshot) {
				t.Fatal("foreign exit proof accepted")
			}
		})
	}
	body[len(body)-2] ^= 1
	original, _ := json.Marshal(exit)
	if err = os.WriteFile(path, encode(signedRemoteExit{Body: body, Signature: ed25519.Sign(key, original)}), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readOwnerExit(id); err == nil {
		t.Fatal("modified exit proof accepted")
	}
}
