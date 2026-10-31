package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sync"
)

// ProcessPort preserves the host's managed process / Job Object policy.
type ProcessPort struct {
	Start         func(*exec.Cmd) (executionbackend.StopHandle, error)
	Inspect       func(*executionbackend.ProcessIdentity) string
	NewGeneration func() string
	WriteAtomic   func(string, []byte) error
}

type Binding struct {
	NativeVersion string `json:",omitempty"`
	Owner         *ExecutionOwnerIdentity
	Workspace     string
	Process       *executionbackend.ProcessIdentity
	Endpoint      string
	Generation    string
	SessionID     string
}

type ProcessReceipt struct {
	Endpoint   string                           `json:"endpoint"`
	Generation string                           `json:"generation"`
	Identity   executionbackend.ProcessIdentity `json:"identity"`
	Managed    bool                             `json:"managed"`
}

type Backend struct {
	Config  Config
	Process ProcessPort
}

type SessionHandle struct {
	Binding Binding
	Paths   TurnPaths
	client  *codexapp.Client
	process executionbackend.StopHandle
	owned   bool
	input   executionbackend.TurnInput
}

func (h *SessionHandle) Close() {
	if h.client != nil {
		h.client.Close()
	}
	if h.owned && h.process != nil {
		h.process.Close()
	}
}

// Detach closes the app-server connection while retaining the exact process
// handle for the center to own after it commits the session receipt.
func (h *SessionHandle) Detach() executionbackend.StopHandle {
	if h == nil {
		return nil
	}
	if h.client != nil {
		h.client.Close()
		h.client = nil
	}
	h.owned = false
	return h.process
}

func encode(v any) []byte { b, _ := json.Marshal(v); return append(b, '\n') }

func (b Backend) PrepareSession(ctx context.Context, fixed Binding, input executionbackend.TurnInput) (handle *SessionHandle, receipt Binding, err error) {
	parts, budgetErr := input.ModelInputParts()
	if budgetErr != nil {
		return nil, fixed, budgetErr
	}
	if _, _, budgetErr = m.FreezeInput(parts...); budgetErr != nil {
		return nil, fixed, budgetErr
	}
	receipt = fixed
	if !executionbackend.StableExecutionID(input.CommandID) || input.Prompt == "" || b.Config.Workspace == "" || b.Config.Executable == "" || fixed.SessionID != "" && !ValidNativeID(fixed.SessionID) {
		return nil, receipt, errors.New("app-server worker turn lacks its fixed workspace or command")
	}
	// Validate native settings before filesystem or process side effects.
	if err = b.Config.Validate(runtime.GOOS); err != nil {
		return nil, receipt, err
	}
	if fixed.Process == nil {
		if _, err = NativeExecutable(b.Config.Executable); err != nil {
			return nil, receipt, err
		}
	}
	var workerSetup *workerRuntime
	if b.Config.WorkerRuntime {
		workerSetup, err = prepareWorkerRuntime(b.Config.Workspace, input.LinkedWorktreeArgs)
		if err != nil {
			return nil, receipt, err
		}

	}
	paths, err := PrepareTurn(b.Config.RunDir, b.Config.RunID, input.CommandID, b.Config.ServerDir, input.Prompt, b.Process.WriteAtomic)
	if err != nil {
		return nil, receipt, err
	}

	h := &SessionHandle{Binding: fixed, Paths: paths, input: input}
	defer func() {
		receipt = h.Binding
		if err != nil {
			if b.Config.Sandbox == "read-only" && h.process != nil {
				// Preserve partial/late owned launch handles for explicit tree
				// settlement, including handshake and thread-create failures.
				h.owned = false
				handle = h
			} else {
				h.Close()
			}
		}
	}()
	if fixed.Process != nil {
		saved, readErr := b.readProcess()
		if readErr != nil || saved.Identity != *fixed.Process || saved.Endpoint != fixed.Endpoint || saved.Generation != fixed.Generation {
			return nil, receipt, errors.New("saved app-server process receipt differs from execution")
		}
		if fixed.Endpoint == "" || fixed.Generation == "" || b.Process.Inspect(fixed.Process) != "matching process still exists" {
			return nil, receipt, errors.New("original app-server process is not verifiably live")
		}
		h.process, err = attachProcess(*fixed.Process)
		if err != nil {
			return nil, receipt, err
		}
	} else {
		var endpoint string
		h.process, _, endpoint, err = LaunchServer(LaunchConfig{Executable: b.Config.Executable, Args: b.Config.Args, Profile: b.Config.Profile, Sandbox: b.Config.Sandbox, WindowsSandbox: b.Config.WindowsSandbox, Workspace: b.Config.Workspace, ServerDir: b.Config.ServerDir, Runtime: workerSetup}, b.Process.Start, b.Config.Environment)
		if err != nil {
			return nil, receipt, err
		}
		h.owned = true
		identity := h.process.Identity()
		generation := b.Process.NewGeneration()
		h.Binding.Process = &identity
		h.Binding.Endpoint = endpoint
		h.Binding.Generation = generation
		saved := ProcessReceipt{Endpoint: endpoint, Generation: generation, Identity: identity, Managed: true}
		if err = b.Process.WriteAtomic(filepath.Join(b.Config.ServerDir, "process.json"), encode(saved)); err != nil {
			return nil, receipt, fmt.Errorf("app-server started but its process receipt is not durable: %w", err)
		}
	}
	h.client, err = ConnectAppServer(ctx, h.Binding.Endpoint)
	if err != nil {
		return nil, receipt, err
	}
	if fixed.SessionID == "" {
		if err = b.verifySandboxReadiness(ctx, h, input); err != nil {
			// No thread/input was created. Retain the original owned handle so
			// the center can record and settle this process tree through W18.
			h.owned = false
			return h, receipt, preparationBlocked(err)
		}
		tools := []any{codexapp.AskDynamicTool()}
		params, paramErr := ThreadParams(b.Config.Workspace, b.Config.Sandbox, b.Config.Model, tools)
		if paramErr != nil {
			return nil, receipt, paramErr
		}
		if workerSetup != nil {
			params["developerInstructions"] = workerSetup.instructions()
		}
		if b.Config.Sandbox == "read-only" {
			config, configErr := contentConfiguration(ctx, h.client, b.Config.Workspace)
			if configErr != nil {
				return nil, receipt, configErr
			}
			params["config"] = config
			params["developerInstructions"] = ContentSessionInstruction
		}
		inputFacts, inputErr := sessionInput(params)
		if inputErr != nil {
			return nil, receipt, inputErr
		}
		if err := b.Process.WriteAtomic(filepath.Join(b.Config.ServerDir, "thread.json"), encode(ThreadReceipt{Input: inputFacts})); err != nil {
			return nil, receipt, err
		}
		response, startErr := StartThread(ctx, h.client, params, b.Config.Workspace)
		if startErr != nil {
			return nil, receipt, startErr
		}
		response.Input = inputFacts
		h.Binding.SessionID = response.Thread.ID
		if b.Config.Sandbox == "read-only" {
			want := "readOnly"
			if response.ApprovalPolicy != "never" || response.ApprovalsReviewer != "user" || response.Sandbox.Type != want {
				h.owned = false
				return h, h.Binding, errors.New("native_effective_permissions_unverified")
			}
		}
		if err = b.Process.WriteAtomic(filepath.Join(b.Config.ServerDir, "thread.json"), encode(response)); err != nil {
			return nil, receipt, err
		}
	} else {
		if err = resumeThread(ctx, h.client, fixed.SessionID, b.Config.Workspace, b.Config.Sandbox); err != nil {
			return nil, receipt, err
		}
	}
	return h, h.Binding, nil
}

// StartTurn is a separate operation: the center must persist PrepareSession's
// partial binding before calling it. No persistence callback drives the backend.
func (b Backend) StartTurn(ctx context.Context, h *SessionHandle, input executionbackend.TurnInput, linkedArgs []string) (*Observation, string, error) {
	if h == nil || h.client == nil || input.CommandID != h.input.CommandID || input.Prompt != h.input.Prompt || !slices.Equal(input.LinkedWorktreeArgs, h.input.LinkedWorktreeArgs) || input.Model != h.input.Model || input.ReasoningEffort != h.input.ReasoningEffort {
		return nil, "", errors.New("app-server turn changed its prepared request identity or input")
	}
	if input.Budget != nil && h.input.Budget != nil && !reflect.DeepEqual(input.Budget, h.input.Budget) {
		return nil, "", errors.New("prepared input provenance changed")
	}
	if err := b.verifySandboxReadiness(ctx, h, input); err != nil {
		return nil, "", preparationBlocked(err)
	}
	params, err := TurnParams(ctx, TurnParamsInput{Worktree: b.Config.Workspace, ThreadID: h.Binding.SessionID, TurnInput: input, Sandbox: b.Config.Sandbox, Model: b.Config.Model, ReasoningEffort: b.Config.ReasoningEffort}, func(context.Context, string) ([]string, error) { return append([]string(nil), linkedArgs...), nil })
	if err != nil {
		return nil, "", err
	}
	turn, err := StartTurn(ctx, h.client, params, input.Budget)
	if err != nil {
		return nil, "", err
	}
	if err = b.Process.WriteAtomic(filepath.Join(h.Paths.TurnDir, "turn-start.json"), encode(turn)); err != nil {
		return nil, turn.Turn.ID, err
	}
	journal, err := os.OpenFile(h.Paths.JournalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, turn.Turn.ID, err
	}
	var terminal func() error
	if checkpoint := b.Config.TerminalCheckpoint; checkpoint != nil {
		terminal = func() error { return checkpoint(input.CommandID, h.Binding.SessionID, turn.Turn.ID) }
	}
	observer := newObservation(h.process, h.client, journal, h.Binding.SessionID, turn.Turn.ID, nil, terminal)
	h.client = nil
	h.owned = false
	return observer, turn.Turn.ID, nil
}

func (b Backend) readProcess() (ProcessReceipt, error) {
	var saved ProcessReceipt
	raw, err := os.ReadFile(filepath.Join(b.Config.ServerDir, "process.json"))
	if err != nil {
		return saved, err
	}
	err = json.Unmarshal(raw, &saved)
	if err == nil && (saved.Endpoint == "" || saved.Generation == "") {
		err = errors.New("original app-server process receipt is missing or unreadable")
	}
	return saved, err
}

func (b Backend) ManagedProcessAbsent(fixed Binding) bool {
	managed, absent := b.ManagedProcessState(fixed)
	return managed && absent
}

func (b Backend) ManagedProcessState(fixed Binding) (managed, absent bool) {
	saved, err := b.readProcess()
	if err != nil || fixed.Process == nil || !saved.Managed || saved.Identity != *fixed.Process || saved.Endpoint != fixed.Endpoint || saved.Generation != fixed.Generation {
		return false, false
	}
	state := b.Process.Inspect(fixed.Process)
	return state == "matching process still exists" || state == "process absent", state == "process absent"
}

func (b Backend) ValidateReply(fixed Binding, generation, threadID, turnID, currentTurn, lastStopped string) error {
	if fixed.Process == nil || b.Process.Inspect(fixed.Process) != "matching process still exists" || fixed.Endpoint == "" || fixed.Generation != generation || fixed.SessionID != threadID || currentTurn != turnID || lastStopped == turnID {
		return &executionbackend.BackendError{Code: executionbackend.BackendIdentityMismatch, Diagnostic: "native answer no longer belongs to the live original app-server request"}
	}
	return nil
}

func Interrupt(ctx context.Context, endpoint, threadID, turnID string) error {
	client, err := codexapp.Open(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil)
}

func ValidNativeID(id string) bool { return nativeID.MatchString(id) }

type attachedProcess struct {
	identity executionbackend.ProcessIdentity
	process  *os.Process
}

func attachProcess(identity executionbackend.ProcessIdentity) (executionbackend.StopHandle, error) {
	process, err := os.FindProcess(identity.PID)
	return &attachedProcess{identity, process}, err
}

func (p *attachedProcess) Identity() executionbackend.ProcessIdentity { return p.identity }

func (p *attachedProcess) Kill() {
	if p.process != nil {
		_ = p.process.Kill()
	}
}

func (p *attachedProcess) Close() {}

type Observation struct {
	Process  executionbackend.StopHandle
	Events   chan ObservationEvent
	Done     chan error
	Finished chan struct{}
	cancel   context.CancelFunc
	once     sync.Once
}

func (o *Observation) Cancel() { o.once.Do(o.cancel) }

func newObservation(process executionbackend.StopHandle, client *codexapp.Client, journal *os.File, threadID, turnID string, replay []codexapp.Envelope, terminal ...func() error) *Observation {
	ctx, cancel := context.WithCancel(context.Background())
	o := &Observation{Process: process, Events: make(chan ObservationEvent, 64), Done: make(chan error, 1), Finished: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(o.Finished)
		defer close(o.Events)
		defer close(o.Done)
		defer client.Close()
		defer journal.Close()
		completed, err := Observe(ctx, client, journal, threadID, turnID, replay, func(e codexapp.Envelope) error { _, err := ParseAskCall(e, threadID, turnID); return err }, func(event ObservationEvent) bool {
			event.Persisted = make(chan error, 1)
			select {
			case o.Events <- event:
			case <-ctx.Done():
				return false
			}
			select {
			case err := <-event.Persisted:
				return err == nil
			case <-ctx.Done():
				return false
			}
		}, terminal...)
		if completed || err != nil {
			o.Done <- err
		}
	}()
	return o
}
