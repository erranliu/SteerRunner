package orcatransport

import (
	"encoding/json"
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
	OutsideWriteDenied        *bool     `json:"outside_write_denied,omitempty"`
	OutsideWriteVerified      *bool     `json:"outside_write_verified,omitempty"`
	Contract                  string    `json:"contract"`
	AccessMode                string    `json:"access_mode"`
	AuthenticationExpiresAt   time.Time `json:"authentication_expires_at"`
	Verified                  bool      `json:"verified"`
}

func (p *TerminalPreparation) UnmarshalJSON(raw []byte) error {
	type wire TerminalPreparation
	var value wire
	if err := b.DecodeStrictJSON(raw, &value, "trailing terminal preparation"); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if value.Contract == terminaltools.Contract {
		if _, exists := fields["outside_write_denied"]; exists {
			return errors.New("new preparation contains retired denial evidence")
		}
	} else {
		if _, exists := fields["outside_write_verified"]; exists {
			return errors.New("legacy preparation contains new evidence fields")
		}
	}
	*p = TerminalPreparation(value)
	return nil
}

// TerminalStartupFailure is positive evidence from a helper which returned
// before starting the native agent. It cannot qualify a missing/changed owner.
type TerminalStartupFailure struct {
	Version              int           `json:"version"`
	Failure              string        `json:"failure,omitempty"`
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
	sourceKind, tools := "pi-extension-v1", []string{"read", "write", "edit", "powershell"}
	if c.Agent == "dsh" {
		sourceKind, tools = "dsh-plugin-v1", []string{"powershell"}
		if r.NativeSessionID == "" {
			return errors.New("DSH native session identity missing")
		}
	}
	expectedSHA := c.EvidenceSHA256
	if expectedSHA == "" || c.BundleSHA256 == "" {
		return errors.New("original frozen terminal source hashes missing")
	}
	version := c.ReceiptVersion
	if version == 0 {
		version = 1
	} // Original frozen helpers keep their source contract.
	preparationValid := validTerminalPreparation(r.Preparation, version)
	if l.stopOnly && version <= 2 && c.AccessMode == "sandbox" {
		p := r.Preparation
		preparationValid = p.AccessMode == "sandbox" && p.Contract == "windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never" && p.OutsideWriteDenied != nil && *p.OutsideWriteDenied && p.OutsideWriteVerified == nil
	}
	if r.Version != version || r.Source != sourceKind || r.SourceSHA256 != expectedSHA || r.RunID != c.ExecutionID || r.SessionID != c.SessionID || r.InstanceID == "" || r.PID <= 0 || r.Mode != "tui" || r.ProcessStartedAt.IsZero() || !reflect.DeepEqual(r.Tools, tools) || !r.Preparation.Verified || !preparationValid || r.Preparation.AccessMode != c.AccessMode || !r.Preparation.InsideWriteVerified || !r.Preparation.ProviderEnvironmentAbsent || !l.stopOnly && c.AccessMode != "full_access" {
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
	if c == nil || len(c.BundleSHA256) != 64 {
		return false
	}
	bundle := c.BundleSHA256
	_, creationErr := strconv.ParseUint(owner.Job.Owner.Created, 16, 64)
	return len(owner.Job.Owner.Created) == 16 && creationErr == nil && owner.RuntimeSHA256 != "" && owner.RuntimeSHA256 == l.profile.RuntimeSHA256 && owner.ExtensionSHA256 == bundle && owner.RunID == c.ExecutionID && owner.SessionID == c.SessionID && owner.ConfigurationDigest == c.ConfigurationDigest && owner.AccessMode == c.AccessMode && owner.Job.Owner.Kind == "os" && owner.Job.Owner.PID > 0 && owner.Job.Owner.Created != "" && owner.Job.Owner.Executable == strings.ToLower(filepath.Clean(l.profile.RuntimeExecutable))
}

func (l *Lifecycle) startupFailureOwner() (TerminalOwner, error) {
	fact, err := l.readStartupFailure()
	return fact.Owner, err
}

var errTerminalStartupExiting = errors.New("original native terminal helper is still exiting")

func (l *Lifecycle) readStartupFailure() (TerminalStartupFailure, error) {
	var fact TerminalStartupFailure
	if err := readTerminalFile(l.profile.Terminal.StateDirectory, "startup-exit", &fact); err != nil {
		return fact, err
	}
	if fact.Version != 1 || !fact.NoWorkerStarted || !fact.AuthenticationClosed || fact.ClosedAt.IsZero() || fact.Owner.Job.Handle != 0 || fact.Owner.Job.Worker != (b.ProcessIdentity{}) || !l.terminalOwnerBinding(fact.Owner) {
		return fact, errors.New("native terminal startup exit remains unverified")
	}
	switch process.Probe(&fact.Owner.Job.Owner).State {
	case "matches":
		// The helper writes this record just before returning. Keep observing
		// the accepted launch until that exact process has actually exited.
		return fact, errTerminalStartupExiting
	case "absent":
		return fact, nil
	default:
		return fact, errors.New("native terminal startup exit remains unverified")
	}
}

// Only the bound helper's verified pre-worker exit can reject an accepted
// launch. Missing receipts and live/changed processes remain unverified.
type terminalStartupError struct{ reason string }

func (e *terminalStartupError) Error() string {
	return "Native terminal startup failed: " + e.reason + "; no business input sent"
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
		if os.IsNotExist(err) && state.TerminalReadiness == nil && state.TerminalOwner == nil && !state.LaunchConfirmed && len(state.Turns) == 0 {
			if fact, failureErr := l.readStartupFailure(); failureErr == nil {
				reason := strings.TrimSpace(fact.Failure)
				if reason == "" {
					reason = "original helper exited before starting the native worker"
				}
				return &terminalStartupError{reason: reason}
			} else if !os.IsNotExist(failureErr) && !errors.Is(failureErr, errTerminalStartupExiting) {
				return failureErr
			}
			if failure := l.terminalPreparationFailure(); failure != nil && !os.IsNotExist(failure) {
				return failure
			}
		}
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

// This native fact explains a failed initializer before a readiness receipt
// exists. It never establishes readiness, completion, or resource settlement.
func (l *Lifecycle) terminalPreparationFailure() error {
	c := l.profile.Terminal
	var failure struct {
		Version      int    `json:"version"`
		RunID        string `json:"run_id"`
		SessionID    string `json:"session_id"`
		BundleSHA256 string `json:"extension_bundle_sha256"`
		PID          int    `json:"pid"`
		Failure      string `json:"failure"`
		ProbeStage   string `json:"probe_stage"`
	}
	if err := readTerminalFile(c.StateDirectory, "preparation-failure", &failure); err != nil {
		return err
	}
	owner, err := l.readTerminalOwner()
	if err != nil {
		return err
	}
	if failure.Version != 1 || failure.RunID != c.ExecutionID || failure.SessionID != c.SessionID || failure.BundleSHA256 != owner.ExtensionSHA256 || failure.PID != owner.Job.Worker.PID {
		return errors.New("native terminal preparation failure binding changed")
	}
	switch failure.Failure {
	case "native_preparation_failed", "native_authentication_preparation_missing", "native_tool_preparation_invalid", "native_tool_preparation_failed", "native_tool_environment_audit_failed", "native_write_boundary_audit_failed":
		return fmt.Errorf("%s; original native initialization failed; no business input sent", failure.Failure)
	default:
		return errors.New("native terminal preparation failure code invalid")
	}
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

func validTerminalPreparation(p TerminalPreparation, version int) bool {
	if p.AccessMode != "full_access" {
		return false
	}
	switch version {
	case 1, 2:
		return p.Contract == "windows-terminal-tools-v1:codex-sandbox-cli-0.159.2:never" && p.OutsideWriteDenied != nil && !*p.OutsideWriteDenied && p.OutsideWriteVerified == nil
	case 3:
		return p.Contract == terminaltools.Contract && p.OutsideWriteDenied == nil && p.OutsideWriteVerified != nil && *p.OutsideWriteVerified
	}
	return false
}
