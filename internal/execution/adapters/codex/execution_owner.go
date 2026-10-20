package codexworker

// The local execution host owns native connections, Jobs and authentication.
// It has no Runner database, workflow cursor or authorization state.
import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

const ExecutionOwnerProtocol = "steerrunner-execution-owner/v1"
const ExecutionOwnerConfigFile = "codex-owner.json"

type ExecutionOwnerIdentity struct {
	RetainsParser        bool              `json:"retains_parser,omitempty"`
	WorkspaceCheckpoints bool              `json:"workspace_checkpoints,omitempty"`
	Protocol             string            `json:"protocol"`
	Directory            string            `json:"directory"`
	Process              b.ProcessIdentity `json:"process"`
	RuntimeSHA256        string            `json:"runtime_sha256"`
	ExitPublicKey        string            `json:"exit_public_key"`
}

type ownerConfig struct {
	Token                                                                                                   string
	RuntimeSHA256                                                                                           string
	Snapshot                                                                                                b.LifecycleSnapshot
	ProfileData                                                                                             []b.ImplementationDataEnvelope
	ProfileID, AcceptedConfigVersion, RepositoryPath, RepositoryURL, RunDirectory, RemoteName, TargetBranch string
	AttemptNumber                                                                                           int
}
type ownerReady struct {
	Identity ExecutionOwnerIdentity
	Endpoint string
}
type ownerQuery struct {
	Token, Method string
	Request       b.LifecycleRequest
}
type ownerResponse struct {
	Receipt b.LifecycleReceipt
	Error   string
	Pending bool
}
type ownerOperation struct {
	Parser   OwnedParser
	Digest   string
	Response ownerResponse
	Done     chan struct{}
}

type ownerExit struct {
	Identity ExecutionOwnerIdentity
	Digest   string
	Response ownerResponse
}

func ownerRequestDigest(request b.LifecycleRequest) string {
	return digestValue(struct {
		ID, Action, Key, Payload string
		Identity                 *b.LifecycleOperationIdentity
	}{request.Operation.ID, request.Operation.Action, request.Operation.IdempotencyKey, request.Operation.PayloadDigest, request.Operation.LifecycleIdentity})
}

func validateOwnerRequest(request b.LifecycleRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	hash := sha256.Sum256(request.Operation.Request)
	original, err := b.CandidateForOperation(request.Operation)
	if err != nil || hex.EncodeToString(hash[:]) != request.Operation.PayloadDigest || !reflect.DeepEqual(original.Input, request.Input) {
		return errors.New("native input differs from the original operation payload")
	}
	return nil
}

func sameOwnerExecution(original, incoming b.LifecycleSnapshot) bool {
	a, z := original.Ref, incoming.Ref
	a.External, z.External = b.ProcessIdentity{}, b.ProcessIdentity{}
	a.Generation, z.Generation = "", ""
	a.Incomplete, z.Incomplete = false, false
	return a == z && original.Workspace == incoming.Workspace &&
		original.ConfigurationDigest == incoming.ConfigurationDigest && original.ResourceGeneration == incoming.ResourceGeneration &&
		original.Session.ProjectID == incoming.Session.ProjectID && original.Session.WorkflowRunID == incoming.Session.WorkflowRunID &&
		original.Session.ExecutionID == incoming.Session.ExecutionID && original.Session.AttemptID == incoming.Session.AttemptID &&
		original.Session.AttemptNumber == incoming.Session.AttemptNumber && original.Session.Agent == incoming.Session.Agent && original.Session.Target == incoming.Session.Target
}

func readOwnerExit(identity ExecutionOwnerIdentity) (ownerExit, error) {
	var exit ownerExit
	var signed signedRemoteExit
	key, err := hex.DecodeString(identity.ExitPublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return exit, errors.New("original owner exit key is unavailable")
	}
	raw, err := os.ReadFile(filepath.Join(identity.Directory, "owner-stop.json"))
	if err != nil {
		return exit, err
	}
	if len(raw) > 1<<20 || b.DecodeStrictJSON(raw, &signed, "invalid owner exit") != nil || !ed25519.Verify(key, signed.Body, signed.Signature) || b.DecodeStrictJSON(signed.Body, &exit, "invalid owner exit body") != nil || exit.Identity != identity || !exit.Response.Receipt.State.WritersAbsent || !exit.Response.Receipt.State.Stopped || exit.Response.Error != "" {
		return exit, errors.New("original owner tree/authentication exit is unverified")
	}
	return exit, nil
}

// The verification key comes from the original durable binding, not a
// rediscovered readiness file. The private key never leaves the native owner.
func VerifyExecutionOwnerStopped(snapshot b.LifecycleSnapshot) bool {
	for _, envelope := range snapshot.Implementation {
		if envelope.ImplementationKey != ExecutionBindingCodecKey || envelope.CodecVersion != 2 {
			continue
		}
		binding, err := DecodeAppServerBindingEnvelope(envelope, AppServerBindingIdentity{ProjectID: snapshot.Session.ProjectID, ExecutionID: snapshot.Session.ExecutionID, AttemptID: snapshot.Session.AttemptID, Generation: snapshot.ResourceGeneration, ConfigDigest: snapshot.ConfigurationDigest})
		if err != nil || binding.Owner == nil {
			return false
		}
		exit, err := readOwnerExit(*binding.Owner)
		return err == nil && ownerExitMatchesSnapshot(exit, snapshot) && process.InspectProcess(&snapshot.Ref.External) == "process absent"
	}
	return false
}

func ownerExitMatchesSnapshot(exit ownerExit, snapshot b.LifecycleSnapshot) bool {
	op := exit.Response.Receipt.Operation
	id := op.LifecycleIdentity
	return exit.Response.Receipt.Disposition == b.OperationConfirmed && op.Action == string(b.LifecycleStopExecution) &&
		op.RunID == snapshot.Session.WorkflowRunID && op.AttemptID == snapshot.Session.AttemptID &&
		op.Ref != nil && *op.Ref == snapshot.Ref && id != nil && id.Ref == snapshot.Ref &&
		id.ProjectID == snapshot.Session.ProjectID && id.ExecutionID == snapshot.Session.ExecutionID &&
		id.ConfigurationDigest == snapshot.ConfigurationDigest && id.ResourceGeneration == snapshot.ResourceGeneration && id.InputBindingDigest != ""
}

func RuntimeDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ServeExecutionOwner is reached through the existing independently verified
// service bootstrap, using a private per-execution directory and pinned image.
// OwnedParser retains only the original bounded acquisition process, never Run
// authorization or workflow state. Its existing started/final journals are used.
type OwnedParser interface {
	Cancel()
	Done() <-chan struct{}
	Result() (b.LifecycleReceipt, bool)
}
type StartOwnedParser func(context.Context, string, b.LifecycleRequest) OwnedParser

func ServeExecutionOwner(ctx context.Context, dir string, startParser StartOwnedParser) error {
	raw, err := os.ReadFile(filepath.Join(dir, ExecutionOwnerConfigFile))
	if err != nil {
		return err
	}
	var cfg ownerConfig
	if err = b.DecodeStrictJSON(raw, &cfg, "invalid local owner configuration"); err != nil {
		return err
	}
	if len(cfg.Token) != 64 || cfg.Snapshot.Session.ExecutionID == "" {
		return errors.New("incomplete execution owner binding")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	sha, err := RuntimeDigest(exe)
	if err != nil || sha != cfg.RuntimeSHA256 {
		return errors.New("execution owner image changed")
	}
	id, err := process.CurrentIdentity()
	if err != nil {
		return err
	}
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	identity := ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: dir, Process: *id, RuntimeSHA256: sha, ExitPublicKey: hex.EncodeToString(public), RetainsParser: startParser != nil, WorkspaceCheckpoints: true}
	setup := b.LifecycleSetup{Snapshot: cfg.Snapshot, ProfileData: cfg.ProfileData, ProfileID: cfg.ProfileID, AcceptedConfigVersion: cfg.AcceptedConfigVersion,
		RepositoryPath: cfg.RepositoryPath, RepositoryURL: cfg.RepositoryURL, RunDirectory: cfg.RunDirectory, RemoteName: cfg.RemoteName, TargetBranch: cfg.TargetBranch,
		AttemptNumber: cfg.AttemptNumber, ProcessHost: process.New(60 * time.Second), CommandTimeout: 60 * time.Second, GitExecutable: "git"}
	port, err := NewExecutionLifecycle(setup)
	if err != nil {
		return err
	}
	lifecycle := port.(*ExecutionLifecycle)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	if err = setup.ProcessHost.WriteExecutionFile(filepath.Join(dir, "owner-ready.json"), encode(ownerReady{Identity: identity, Endpoint: listener.Addr().String()})); err != nil {
		return err
	}
	var mu sync.Mutex
	ops := map[string]*ownerOperation{}
	var owned b.StopHandle
	var nativeMu sync.Mutex
	go func() { <-ctx.Done(); listener.Close() }()
	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			return acceptErr
		}
		go func(conn net.Conn) {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(100 * time.Second))
			var q ownerQuery
			if json.NewDecoder(io.LimitReader(conn, 16<<20)).Decode(&q) != nil || q.Token != cfg.Token {
				return
			}
			respond := func(v ownerResponse) { _ = json.NewEncoder(conn).Encode(v) }
			if err := validateOwnerRequest(q.Request); err != nil {
				respond(ownerResponse{Error: err.Error()})
				return
			}
			if !sameOwnerExecution(cfg.Snapshot, q.Request.Snapshot) {
				respond(ownerResponse{Error: "native request belongs to another original owner execution"})
				return
			}
			if err := lifecycle.validateIdentity(q.Request.Snapshot); err != nil {
				respond(ownerResponse{Error: err.Error()})
				return
			}
			action := b.LifecycleAction(q.Request.Operation.Action)
			allowed := action == b.LifecyclePrepareSession || action == b.LifecycleStartTurn || action == b.LifecycleDeliverFeedback || action == b.LifecycleStopObservation || action == b.LifecycleStopExecution || action == b.LifecycleParseResult
			if !allowed || (q.Method != "native" && q.Method != "receipt" && !(action == b.LifecycleParseResult && q.Method == "cancel_parser")) {
				respond(ownerResponse{Error: "unsupported local native action"})
				return
			}
			digest := ownerRequestDigest(q.Request)
			mu.Lock()
			if action == b.LifecycleParseResult {
				p := q.Request.Input.Parse
				if p == nil || p.Owner == nil || *p.Owner != identity.Process || startParser == nil {
					mu.Unlock()
					respond(ownerResponse{Error: "parser original owner binding is unavailable"})
					return
				}
				prior := ops[q.Request.Operation.ID]
				if prior != nil && prior.Digest != digest {
					mu.Unlock()
					respond(ownerResponse{Error: "original parser binding changed"})
					return
				}
				var task OwnedParser
				if prior != nil {
					task = prior.Parser
				}
				if task == nil && q.Method == "native" {
					task = startParser(ctx, cfg.RunDirectory, q.Request)
					ops[q.Request.Operation.ID] = &ownerOperation{Digest: digest, Parser: task}
				}
				mu.Unlock()
				if task == nil {
					respond(ownerResponse{Error: "original parser was not dispatched; no replay"})
					return
				}
				if q.Method == "cancel_parser" {
					task.Cancel()
				}
				if receipt, done := task.Result(); done {
					respond(ownerResponse{Receipt: receipt})
				} else {
					respond(ownerResponse{Pending: true})
				}
				return
			}
			prior := ops[q.Request.Operation.ID]
			if prior != nil {
				mu.Unlock()
				if prior.Digest != digest {
					respond(ownerResponse{Error: "original native operation binding changed"})
					return
				}
				select {
				case <-prior.Done:
					respond(prior.Response)
				case <-time.After(80 * time.Second):
					respond(ownerResponse{Pending: true})
				}
				return
			}
			if q.Method == "receipt" {
				mu.Unlock()
				respond(ownerResponse{Error: "original native operation receipt unavailable; no action repeated"})
				return
			}
			slot := &ownerOperation{Digest: digest, Done: make(chan struct{})}
			ops[q.Request.Operation.ID] = slot
			mu.Unlock()
			nativeMu.Lock()
			callCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			var receipt b.LifecycleReceipt
			var observer b.LifecycleObserver
			var callErr error
			if action == b.LifecycleStopExecution {
				mu.Lock()
				active := []OwnedParser{}
				for _, operation := range ops {
					if task := operation.Parser; task != nil {
						task.Cancel()
						active = append(active, task)
					}
				}
				mu.Unlock()
				for _, task := range active {
					select {
					case <-task.Done():
					case <-callCtx.Done():
						callErr = callCtx.Err()
					}
					if result, done := task.Result(); !done || !result.State.WritersAbsent {
						callErr = errors.New("original parser tree exit is unverified")
					}
				}
				receipt.Operation = q.Request.Operation
				if callErr == nil {
					receipt.State, callErr = lifecycle.ShutdownLifecycle(callCtx, q.Request.Snapshot, owned)
				}
				receipt.Disposition = b.OperationUnknown
				if callErr == nil && receipt.State.Stopped {
					receipt.Disposition = b.OperationConfirmed
				}
			} else {
				if q.Request.Input.Turn != nil {
					original := q.Request.Clone()
					lifecycle.Backend.Config.TerminalCheckpoint = func(commandID, sessionID, turnID string) error {
						return writeOwnerCheckpoint(setup, identity, key, original, commandID, sessionID, turnID)
					}
				}
				receipt, observer, callErr = lifecycle.Execute(callCtx, q.Request)
			}
			cancel()
			if handle, ok := receipt.Handle.(b.OwnedProcessHandle); ok && handle.OwnedProcess() {
				owned = receipt.Handle
			}
			if observer != nil {
				// Observe already fsyncs the native journal. Keep draining it even
				// when the service has no observation connection.
				go func() {
					for event := range observer.Events() {
						if event.Persisted != nil {
							event.Persisted <- nil
						}
						if event.Processed != nil {
							event.Processed <- nil
						}
					}
				}()
			}
			nativeMu.Unlock()
			slot.Response.Receipt = receipt
			slot.Response.Receipt.Handle = nil
			if callErr != nil {
				slot.Response.Error = callErr.Error()
			}
			if action == b.LifecycleStopExecution && receipt.State.Stopped {
				// A finite exit fact, not a second operation journal.
				body, _ := json.Marshal(ownerExit{identity, digest, slot.Response})
				if e := setup.ProcessHost.WriteExecutionFile(filepath.Join(dir, "owner-stop.json"), encode(signedRemoteExit{Body: body, Signature: ed25519.Sign(key, body)})); e != nil {
					slot.Response.Error = e.Error()
					slot.Response.Receipt.State.Stopped = false
				}
			}
			close(slot.Done)
			respond(slot.Response)
			if action == b.LifecycleStopExecution && slot.Response.Receipt.State.Stopped && slot.Response.Error == "" {
				listener.Close()
			}
		}(conn)
	}
}

func (l *ExecutionLifecycle) ownerDirectory() string {
	return filepath.Join(l.Backend.Config.ServerDir, "owner")
}

func (l *ExecutionLifecycle) ensureOwner(ctx context.Context, request b.LifecycleRequest) error {
	dir := l.ownerDirectory()
	if _, err := os.Stat(filepath.Join(dir, ExecutionOwnerConfigFile)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if request.Snapshot.Ref.External.Kind != "" || request.Snapshot.Session.SessionID != "" && l.setup.AttemptNumber <= 1 {
		return errors.New("legacy native resources cannot be transferred to a replacement execution owner")
	}
	// A later Attempt is created only by the existing explicitly authorized
	// recovery path after original writer absence, retaining the original thread.
	if l.setup.LaunchExecutionOwner == nil {
		return errors.New("independent execution owner is unavailable")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	sha, err := RuntimeDigest(exe)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	cfg := ownerConfig{Token: l.setup.ProcessHost.NewProcessGeneration() + l.setup.ProcessHost.NewProcessGeneration(), RuntimeSHA256: sha, Snapshot: request.Snapshot.Clone(), ProfileData: l.setup.ProfileData, ProfileID: l.setup.ProfileID, AcceptedConfigVersion: l.setup.AcceptedConfigVersion,
		RepositoryPath: l.setup.RepositoryPath, RepositoryURL: l.setup.RepositoryURL, RunDirectory: l.setup.RunDirectory, RemoteName: l.setup.RemoteName, TargetBranch: l.setup.TargetBranch, AttemptNumber: l.setup.AttemptNumber}
	if err = l.setup.ProcessHost.WriteExecutionFile(filepath.Join(dir, ExecutionOwnerConfigFile), encode(cfg)); err != nil {
		return err
	}
	return l.setup.LaunchExecutionOwner(ctx, dir)
}

func (l ExecutionLifecycle) ownerCall(ctx context.Context, request b.LifecycleRequest, reconcile bool) (b.LifecycleReceipt, error) {
	method := "native"
	if reconcile {
		method = "receipt"
	}
	return l.callOwner(ctx, request, method)
}

// CancelParser only revokes the existing original acquisition; it cannot start
// an absent operation. Used after the Run's explicit stop is already durable.
func (l ExecutionLifecycle) CancelParser(ctx context.Context, request b.LifecycleRequest) (b.LifecycleReceipt, error) {
	return l.callOwner(ctx, request, "cancel_parser")
}

func ExecutionOwner(snapshot b.LifecycleSnapshot) *ExecutionOwnerIdentity {
	for _, envelope := range snapshot.Implementation {
		if envelope.ImplementationKey != ExecutionBindingCodecKey || envelope.CodecVersion != 2 {
			continue
		}
		binding, err := DecodeAppServerBindingEnvelope(envelope, AppServerBindingIdentity{ProjectID: snapshot.Session.ProjectID, ExecutionID: snapshot.Session.ExecutionID, AttemptID: snapshot.Session.AttemptID, Generation: snapshot.ResourceGeneration, ConfigDigest: snapshot.ConfigurationDigest})
		if err == nil {
			return binding.Owner
		}
	}
	return nil
}

func (l ExecutionLifecycle) callOwner(ctx context.Context, request b.LifecycleRequest, method string) (b.LifecycleReceipt, error) {
	dir := l.ownerDirectory()
	raw, err := os.ReadFile(filepath.Join(dir, ExecutionOwnerConfigFile))
	if err != nil {
		return b.LifecycleReceipt{}, err
	}
	var cfg ownerConfig
	if err = b.DecodeStrictJSON(raw, &cfg, "invalid owner configuration"); err != nil {
		return b.LifecycleReceipt{}, err
	}
	raw, err = os.ReadFile(filepath.Join(dir, "owner-ready.json"))
	if err != nil {
		return b.LifecycleReceipt{}, err
	}
	var ready ownerReady
	if err = b.DecodeStrictJSON(raw, &ready, "invalid owner readiness"); err != nil {
		return b.LifecycleReceipt{}, err
	}
	if ready.Identity.Protocol != ExecutionOwnerProtocol || ready.Identity.Directory != dir || ready.Identity.RuntimeSHA256 != cfg.RuntimeSHA256 {
		return b.LifecycleReceipt{}, errors.New("original execution owner identity is unverified")
	}
	address, _, endpointErr := net.SplitHostPort(ready.Endpoint)
	if endpointErr != nil || address != "127.0.0.1" {
		return b.LifecycleReceipt{}, errors.New("original owner endpoint is not local")
	}
	if binding, found, e := l.readBinding(request.Snapshot); e != nil {
		return b.LifecycleReceipt{}, e
	} else if found && binding.Owner != nil && *binding.Owner != ready.Identity {
		return b.LifecycleReceipt{}, errors.New("original execution owner changed")
	}
	if l.Backend.Process.Inspect(&ready.Identity.Process) != "matching process still exists" {
		if request.Operation.Action == string(b.LifecycleStopExecution) {
			binding, found, e := l.readBinding(request.Snapshot)
			if e == nil && found && binding.Owner != nil {
				exit, e := readOwnerExit(*binding.Owner)
				if e == nil && exit.Digest == ownerRequestDigest(request) {
					return exit.Response.Receipt, nil
				}
			}
		}
		return b.LifecycleReceipt{}, errors.New("original execution owner is unavailable; no substitute launched")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", ready.Endpoint)
	if err != nil {
		return b.LifecycleReceipt{}, err
	}
	defer conn.Close()
	// Detach this service immediately on shutdown. The owner's native call
	// continues under its own bounded context and retains the original receipt.
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()
	deadline := time.Now().Add(90 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	if err = json.NewEncoder(conn).Encode(ownerQuery{Token: cfg.Token, Method: method, Request: request}); err != nil {
		return b.LifecycleReceipt{}, err
	}
	var response ownerResponse
	if err = json.NewDecoder(io.LimitReader(conn, 16<<20)).Decode(&response); err != nil {
		return b.LifecycleReceipt{}, err
	}
	receipt := response.Receipt
	for i := range receipt.Implementation {
		envelope := &receipt.Implementation[i]
		if envelope.ImplementationKey != ExecutionBindingCodecKey {
			continue
		}
		binding, e := DecodeAppServerBindingEnvelope(*envelope, l.identity(request.Snapshot))
		if e != nil {
			return receipt, e
		}
		binding.Owner = &ready.Identity
		*envelope, e = NewAppServerBindingEnvelope(l.identity(request.Snapshot), binding)
		if e != nil {
			return receipt, e
		}
	}
	if response.Error != "" {
		return receipt, errors.New(response.Error)
	}
	if response.Pending {
		return receipt, errors.New("original native operation still pending")
	}
	return receipt, nil
}

func (l ExecutionLifecycle) observeOwnerReceipt(ctx context.Context, request b.LifecycleRequest, receipt b.LifecycleReceipt) (b.LifecycleReceipt, b.LifecycleObserver, error) {
	if !receipt.State.TurnStarted || receipt.Process == nil {
		return receipt, nil, nil
	}
	s := request.Snapshot.Clone()
	s.Ref.External = *receipt.Process
	s.Implementation = receipt.Implementation
	s.Session.SessionID = receipt.State.SessionID
	s.Session.Generation = receipt.State.SessionGeneration
	s.Session.CurrentCommandID = request.Input.Turn.CommandID
	s.Session.CurrentTurnID = receipt.State.TurnID
	s.Session.PendingCommandID = ""
	s.Session.PendingTurnID = ""
	observation, observer, err := l.ObserveLifecycle(ctx, s)
	if observation.TurnEvidence != nil {
		receipt.TurnEvidence = observation.TurnEvidence
	}
	return receipt, observer, err
}

// Some native notifications belong only to the owner's original connection.
// Wake the ordinary read-only reattachment when its existing journal gains a
// completion/question. No durable cursor or second event queue is introduced.
func (l ExecutionLifecycle) watchOwnerJournal(snapshot b.LifecycleSnapshot, fixed b.SessionContext, observer *Observation) {
	path := filepath.Join(l.Backend.Config.RunDir, "runs", snapshot.Session.WorkflowRunID, "turns", fixed.CurrentCommandID, "appserver.jsonl")
	known := append([]string(nil), snapshot.KnownQuestionIDs...)
	if snapshot.QuestionRoute != nil {
		known = append(known, snapshot.QuestionRoute.QuestionID)
	}
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		var size int64 = -1
		for {
			select {
			case <-observer.Finished:
				return
			case <-ticker.C:
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() == size {
				continue
			}
			outcome, err := ReadAppServerOutcome(path, fixed.SessionID, fixed.CurrentTurnID)
			questions, questionErr := PendingAskEvents(path, fixed.SessionID, fixed.CurrentTurnID, known)
			if err == nil && questionErr == nil {
				size = info.Size()
			}
			if err == nil && outcome.Completed || questionErr == nil && len(questions) > 0 {
				observer.Cancel()
				return
			}
		}
	}()
}
