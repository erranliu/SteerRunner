package orcatransport

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
)

type TerminalPreparation struct {
	InsideWriteVerified       bool      `json:"inside_write_verified"`
	ProviderEnvironmentAbsent bool      `json:"provider_environment_absent"`
	OutsideWriteDenied        bool      `json:"outside_write_denied"`
	Contract                  string    `json:"contract"`
	AccessMode                string    `json:"access_mode"`
	AuthenticationExpiresAt   time.Time `json:"authentication_expires_at"`
	Verified                  bool      `json:"verified"`
}

// TerminalStartupFailure is positive evidence from a helper which returned
// before starting the native agent. It cannot qualify a missing/changed owner.
type TerminalStartupFailure struct {
	Version              int           `json:"version"`
	Owner                TerminalOwner `json:"owner"`
	NoWorkerStarted      bool          `json:"no_worker_started"`
	AuthenticationClosed bool          `json:"authentication_closed"`
	ClosedAt             time.Time     `json:"closed_at"`
}

type TerminalExit struct {
	AuthenticationClosed *bool         `json:"authentication_closed,omitempty"`
	Version              int           `json:"version"`
	Owner                TerminalOwner `json:"owner"`
	WritersAbsent        bool          `json:"writers_absent"`
	ClosedAt             time.Time     `json:"closed_at"`
}

// One bounded native receipt. This is extension evidence, never a Codex DTO or
// an Orca status alias. The original accepted UUID remains delivery authority.
type TerminalReceipt struct {
	ReasoningEffort  string              `json:"reasoning_effort,omitempty"`
	ActiveCommandID  string              `json:"active_command_id,omitempty"`
	NativeSessionID  string              `json:"native_session_id,omitempty"`
	NativeInputSeq   int                 `json:"native_input_seq,omitempty"`
	NativeReplySeq   int                 `json:"native_reply_seq,omitempty"`
	NativeEndSeq     int                 `json:"native_end_seq,omitempty"`
	Version          int                 `json:"version"`
	Source           string              `json:"source"`
	SourceSHA256     string              `json:"source_sha256"`
	RunID            string              `json:"run_id"`
	SessionID        string              `json:"session_id"`
	InstanceID       string              `json:"instance_id"`
	PID              int                 `json:"pid"`
	ProcessStartedAt time.Time           `json:"process_started_at"`
	Mode             string              `json:"mode"`
	Tools            []string            `json:"tools"`
	Model            string              `json:"model"`
	Preparation      TerminalPreparation `json:"preparation"`
	Stage            string              `json:"stage"`
	Failure          string              `json:"failure,omitempty"`
	CommandID        string              `json:"command_id,omitempty"`
	InputSHA256      string              `json:"input_sha256,omitempty"`
	WireSHA256       string              `json:"wire_sha256,omitempty"`
	NativeTurnIndex  int                 `json:"native_turn_index,omitempty"`
	Outcome          string              `json:"outcome,omitempty"`
	NativeInputID    string              `json:"native_input_id,omitempty"`
	NativeReplyID    string              `json:"native_reply_id,omitempty"`
	StopReason       string              `json:"stop_reason,omitempty"`
	Reply            string              `json:"reply,omitempty"`
	ReplySHA256      string              `json:"reply_sha256,omitempty"`
	ErrorKind        string              `json:"error_kind,omitempty"`
}

func readTerminalFile(directory, name string, value any) error {
	// Read only a fixed leaf in the original Run directory. Never follow a
	// replaced directory/file or parse arbitrary native session transcripts.
	real, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(directory), real) {
		return errors.New("native receipt directory redirected")
	}
	file := filepath.Join(directory, name+".json")
	info, err := os.Lstat(file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return errors.New("native receipt is redirected or oversized")
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(raw) > 65536 {
		return errors.New("native receipt read bound exceeded")
	}
	return b.DecodeStrictJSON(raw, value, "trailing native receipt")
}

func (l *Lifecycle) validateTerminalReceipt(r TerminalReceipt) error {
	c := l.profile.Terminal
	if c == nil || l.profile.Environment != "" {
		return errors.New("local native terminal profile missing")
	}
	sourceName, sourceKind, tools := "pi-evidence.mjs", "pi-extension-v1", []string{"read", "write", "edit", "powershell"}
	if c.Agent == "dsh" {
		sourceName, sourceKind, tools = "dsh-managed.mjs", "dsh-plugin-v1", []string{"powershell"}
		if r.NativeSessionID == "" {
			return errors.New("DSH native session identity missing")
		}
	}
	source, _ := terminalAssets.ReadFile("extensions/" + sourceName)
	expectedSHA := c.EvidenceSHA256
	if expectedSHA == "" {
		expectedSHA = fmt.Sprintf("%x", sha256.Sum256(source))
	}
	version := c.ReceiptVersion
	if version == 0 {
		version = 1
	} // Original frozen helpers keep their source contract.
	if r.Version != version || r.Source != sourceKind || r.SourceSHA256 != expectedSHA || r.RunID != c.ExecutionID || r.SessionID != c.SessionID || r.InstanceID == "" || r.PID <= 0 || r.Mode != "tui" || r.ProcessStartedAt.IsZero() || !reflect.DeepEqual(r.Tools, tools) || !r.Preparation.Verified || r.Preparation.Contract != terminaltools.Contract || r.Preparation.AccessMode != c.AccessMode || !r.Preparation.InsideWriteVerified || !r.Preparation.ProviderEnvironmentAbsent || r.Preparation.OutsideWriteDenied != (c.AccessMode == "sandbox") {
		return errors.New("native terminal receipt binding changed")
	}
	model := c.Model
	if c.Agent == "dsh" && model != "" {
		model = "deepseek-official/" + model
	}
	if model != "" && r.Model != model {
		return errors.New("native terminal model differs from frozen launch")
	}
	if version >= 2 && (r.ReasoningEffort == "" || c.ReasoningEffort != "" && r.ReasoningEffort != c.ReasoningEffort) {
		return errors.New("native terminal thinking level differs from frozen launch")
	}
	return nil
}

func (l *Lifecycle) readTerminalOwner() (TerminalOwner, error) {
	c := l.profile.Terminal
	if c == nil {
		return TerminalOwner{}, errors.New("native terminal profile missing")
	}
	var owner TerminalOwner
	if err := readTerminalFile(c.StateDirectory, "owner", &owner); err != nil {
		return owner, err
	}
	if !l.terminalOwnerBinding(owner) || owner.Job.Handle == 0 || owner.Job.Worker.Kind != "os" || owner.Job.Worker.PID <= 0 {
		return owner, errors.New("original native terminal owner differs from frozen launch")
	}
	return owner, nil
}

func (l *Lifecycle) terminalOwnerBinding(owner TerminalOwner) bool {
	c := l.profile.Terminal
	bundle := c.BundleSHA256
	if bundle == "" {
		bundle = TerminalExtensionDigest()
	}
	_, creationErr := strconv.ParseUint(owner.Job.Owner.Created, 16, 64)
	return len(owner.Job.Owner.Created) == 16 && creationErr == nil && owner.RuntimeSHA256 != "" && owner.RuntimeSHA256 == l.profile.RuntimeSHA256 && owner.ExtensionSHA256 == bundle && owner.RunID == c.ExecutionID && owner.SessionID == c.SessionID && owner.ConfigurationDigest == c.ConfigurationDigest && owner.AccessMode == c.AccessMode && owner.Job.Owner.Kind == "os" && owner.Job.Owner.PID > 0 && owner.Job.Owner.Created != "" && owner.Job.Owner.Executable == strings.ToLower(filepath.Clean(l.profile.RuntimeExecutable))
}

func (l *Lifecycle) startupFailureOwner() (TerminalOwner, error) {
	var fact TerminalStartupFailure
	if err := readTerminalFile(l.profile.Terminal.StateDirectory, "startup-exit", &fact); err != nil {
		return fact.Owner, err
	}
	if fact.Version != 1 || !fact.NoWorkerStarted || !fact.AuthenticationClosed || fact.ClosedAt.IsZero() || fact.Owner.Job.Handle != 0 || fact.Owner.Job.Worker != (b.ProcessIdentity{}) || !l.terminalOwnerBinding(fact.Owner) || process.Probe(&fact.Owner.Job.Owner).State != "absent" {
		return fact.Owner, errors.New("native terminal startup exit remains unverified")
	}
	return fact.Owner, nil
}

func (l *Lifecycle) terminalExitVerified(owner TerminalOwner) bool {
	var fact TerminalExit
	return readTerminalFile(l.profile.Terminal.StateDirectory, "exit", &fact) == nil && fact.Version == 1 && (l.profile.Terminal.ReceiptVersion < 2 || fact.AuthenticationClosed != nil && *fact.AuthenticationClosed) && fact.WritersAbsent && !fact.ClosedAt.IsZero() && reflect.DeepEqual(fact.Owner, owner) && process.Probe(&owner.Job.Owner).State == "absent"
}

func (l *Lifecycle) pinTerminalJob(owner TerminalOwner) error {
	l.terminalMu.Lock()
	defer l.terminalMu.Unlock()
	if l.terminalJob != nil {
		if l.terminalJob.Identity() != owner.Job.Worker {
			return errors.New("native terminal Job changed")
		}
		return nil
	}
	job, err := process.AttachOwnedJob(owner.Job)
	if err != nil {
		return err
	}
	l.terminalJob = job
	return nil
}

func (l *Lifecycle) closeTerminalJob() {
	l.terminalMu.Lock()
	defer l.terminalMu.Unlock()
	if l.terminalJob != nil {
		l.terminalJob.Close()
		l.terminalJob = nil
	}
}

func (l *Lifecycle) readTerminalReadiness(state *State, requireLease bool) error {
	c := l.profile.Terminal
	if c == nil {
		return errors.New("native terminal profile missing")
	}
	var ready TerminalReceipt
	if err := readTerminalFile(c.StateDirectory, "readiness", &ready); err != nil {
		return err
	}
	if err := l.validateTerminalReceipt(ready); err != nil {
		return err
	}
	if ready.Stage != "ready" || requireLease && !ready.Preparation.AuthenticationExpiresAt.After(l.clock().Add(5*time.Second)) {
		return errors.New("native terminal preparation unverified or expired")
	}
	owner, err := l.readTerminalOwner()
	if err != nil {
		return err
	}
	if owner.Job.Worker.PID != ready.PID {
		return errors.New("original native terminal owner differs from frozen launch")
	}
	if process.Probe(&owner.Job.Owner).State != "matches" || process.Probe(&owner.Job.Worker).State != "matches" {
		return errors.New("original native terminal process creation identity changed")
	}
	if state.TerminalReadiness != nil {
		previous, current := *state.TerminalReadiness, ready
		previous.ActiveCommandID, current.ActiveCommandID = "", ""
		if !reflect.DeepEqual(state.TerminalOwner, &owner) || !reflect.DeepEqual(previous, current) {
			return errors.New("original native terminal readiness changed")
		}
	}
	if ready.ActiveCommandID != "" && turnFor(state, ready.ActiveCommandID) == nil {
		return errors.New("unassociated native input boundary; no input was sent")
	}
	state.TerminalReadiness = &ready
	state.TerminalOwner = &owner
	return l.pinTerminalJob(owner)
}

// Defaults are pinned by the original readiness too. A receipt cannot change
// native session, model, effort or preparation just because the caller omitted
// an explicit model/effort setting.
func sameTerminalInstance(a, b TerminalReceipt) bool {
	return a.InstanceID == b.InstanceID && a.PID == b.PID && a.ProcessStartedAt == b.ProcessStartedAt && a.SessionID == b.SessionID && a.NativeSessionID == b.NativeSessionID && a.Model == b.Model && a.ReasoningEffort == b.ReasoningEffort && reflect.DeepEqual(a.Preparation, b.Preparation)
}

func (l *Lifecycle) observeTerminal(state *State, turn *Turn) string {
	if !turn.Accepted {
		return "Original input acceptance unresolved; no input was resent"
	}
	if err := l.readTerminalReadiness(state, false); err != nil {
		return err.Error()
	}
	if state.TerminalReadiness.ActiveCommandID != turn.Command {
		return "original native input boundary changed"
	}
	var fact TerminalReceipt
	if err := readTerminalFile(l.profile.Terminal.StateDirectory, turn.Command, &fact); err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return err.Error()
	}
	if err := l.validateTerminalReceipt(fact); err != nil {
		return err.Error()
	}
	ready := state.TerminalReadiness
	if !sameTerminalInstance(fact, *ready) || fact.CommandID != turn.Command || fact.WireSHA256 != digest([]byte(wirePrompt(turn))) || fact.InputSHA256 != digest([]byte(strings.TrimRight(strings.ReplaceAll(turn.Prompt, "\r\n", "\n"), "\r\n"))) {
		return "Original native input correlation changed"
	}
	if fact.Stage == "unverified" {
		return "Original native evidence unverified: " + fact.Failure
	}
	if fact.Stage != "input_observed" && fact.Stage != "started" && fact.Stage != "settled" {
		return "Original native evidence stage unsupported"
	}
	if turn.Extension != nil && turn.Extension.Stage == "settled" {
		candidate := fact
		if turn.Settled && turn.Extension.Reply == "" {
			candidate.Reply = ""
		}
		if !reflect.DeepEqual(turn.Extension, &candidate) {
			return "Original settled native evidence changed"
		}
	}
	// Validate the complete final before retaining it. Otherwise a malformed
	// failure could escape through terminalFailure despite this diagnostic.
	if fact.Stage == "settled" {
		if fact.NativeInputID == "" || fact.ReplySHA256 != digest([]byte(fact.Reply)) || len(fact.Reply) > maxReplyBytes {
			return "Original native final evidence incomplete"
		}
		if fact.Outcome == "completed" && fact.StopReason == "stop" && (fact.Reply == "" || fact.NativeReplyID == "") {
			return "Original native final body missing"
		}
	}
	turn.Extension = &fact
	if fact.Stage == "started" || fact.Stage == "settled" {
		turn.Started = true
	}
	if fact.Stage == "settled" {
		if fact.Outcome != "completed" || fact.StopReason != "stop" {
			return "Original native turn failed: " + fact.Outcome + "/" + fact.StopReason
		}
		turn.Reply, turn.ReplyDigest, turn.Settled = fact.Reply, fact.ReplySHA256, true
		// Keep the complete body once in the existing Turn, plus its native hash
		// and provenance. Do not clone a second transcript into implementation state.
		fact.Reply = ""
	}
	return ""
}

func (l *Lifecycle) terminalFailure(state State, turn Turn) *b.TurnEvidence {
	f := turn.Extension
	if f == nil || f.Stage != "settled" || f.Outcome == "completed" && f.StopReason == "stop" {
		return nil
	}
	return &b.TurnEvidence{NativeOutcome: "failed", FinalOrigin: "native_history", FinalSource: f.Source, FinalItemID: f.NativeInputID + ":end", TurnID: turn.Command, CommandID: turn.Command, SessionID: SessionID(state.Process), Started: turn.Started, Completed: true, Succeeded: false}
}

func (l *Lifecycle) validateTerminalTurn(state State, turn *Turn) error {
	f := turn.Extension
	if f == nil {
		if turn.Started || turn.Settled {
			return errors.New("native extension start/settlement evidence missing")
		}
		return nil
	}
	if err := l.validateTerminalReceipt(*f); err != nil {
		return err
	}
	r := state.TerminalReadiness
	if r == nil || !sameTerminalInstance(*f, *r) || f.CommandID != turn.Command || f.WireSHA256 != digest([]byte(wirePrompt(turn))) || !turn.Accepted {
		return errors.New("saved native extension turn changed")
	}
	if turn.Started && f.Stage != "started" && f.Stage != "settled" || turn.Settled && (f.Stage != "settled" || f.Outcome != "completed" || f.StopReason != "stop" || f.NativeInputID == "" || f.NativeReplyID == "" || f.Reply != "" && f.Reply != turn.Reply || f.ReplySHA256 != turn.ReplyDigest) {
		return errors.New("saved native extension outcome changed")
	}
	return nil
}

func (l *Lifecycle) terminalEvidence(state State, turn Turn) *b.TurnEvidence {
	f := turn.Extension
	nativeSession := f.SessionID
	if f.NativeSessionID != "" {
		nativeSession = f.NativeSessionID
	}
	return &b.TurnEvidence{NativeOutcome: "succeeded", FinalOrigin: "native_history", FinalSource: f.Source, FinalItemID: f.NativeReplyID, FinalVerified: true, TurnID: turn.Command, CommandID: turn.Command, SessionID: SessionID(state.Process), Started: true, Completed: true, Succeeded: true, LastMessage: turn.Reply, FinalNativeSessionID: nativeSession, FinalNativeTurnID: f.NativeInputID}
}
