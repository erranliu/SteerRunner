package orcatransport

import (
	"errors"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"

	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"strings"
	"time"
)

type TerminalPreparation struct {
	ProviderEnvironmentAbsent bool   `json:"provider_environment_absent"`
	Contract                  string `json:"contract"`
	AccessMode                string `json:"access_mode"`
	Verified                  bool   `json:"verified"`
}

func (p *TerminalPreparation) UnmarshalJSON(raw []byte) error {
	type wire TerminalPreparation
	var value wire
	if err := b.DecodeStrictJSON(raw, &value, "trailing terminal preparation"); err != nil {
		return err
	}
	*p = TerminalPreparation(value)
	return nil
}

// TerminalStartupFailure is positive evidence from a helper which returned
// before starting the native agent. It cannot qualify a missing/changed owner.
type TerminalStartupFailure struct {
	Version         int           `json:"version"`
	Failure         string        `json:"failure,omitempty"`
	Owner           TerminalOwner `json:"owner"`
	NoWorkerStarted bool          `json:"no_worker_started"`
	ClosedAt        time.Time     `json:"closed_at"`
}

type TerminalExit struct {
	Version       int           `json:"version"`
	Owner         TerminalOwner `json:"owner"`
	WritersAbsent bool          `json:"writers_absent"`
	ClosedAt      time.Time     `json:"closed_at"`
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
	return l.frozenRecord().validateTerminalReceipt(r)
}

func (l *Lifecycle) readTerminalOwner() (TerminalOwner, error) {
	return l.frozenRecord().readTerminalOwner()
}

func (l *Lifecycle) terminalOwnerBinding(owner TerminalOwner) bool {
	return l.frozenRecord().terminalOwnerBinding(owner)
}

var errTerminalStartupExiting = errors.New("original native terminal helper is still exiting")

// Only the bound helper's verified pre-worker exit can reject an accepted
// launch. Missing receipts and live/changed processes remain unverified.
type terminalStartupError struct{ reason string }

func (e *terminalStartupError) Error() string {
	return "Native terminal startup failed: " + e.reason + "; no business input sent"
}

func (l *Lifecycle) terminalExitVerified(owner TerminalOwner) bool {
	return l.frozenRecord().terminalExitVerified(owner)
}

// This native fact explains a failed initializer before a readiness receipt
// exists. It never establishes readiness, completion, or resource settlement.

// Defaults are pinned by the original readiness too. A receipt cannot change
// native session, model, effort or preparation just because the caller omitted
// an explicit model/effort setting.
func sameTerminalInstance(a, b TerminalReceipt) bool {
	return a.InstanceID == b.InstanceID && a.PID == b.PID && a.ProcessStartedAt == b.ProcessStartedAt && a.SessionID == b.SessionID && a.NativeSessionID == b.NativeSessionID && a.Model == b.Model && a.ReasoningEffort == b.ReasoningEffort && reflect.DeepEqual(a.Preparation, b.Preparation)
}

func (l *Lifecycle) validateTerminalTurn(state State, turn *Turn) error {
	return l.frozenRecord().validateTerminalTurn(state, turn)
}

func validTerminalPreparation(p TerminalPreparation, version int) bool {
	return version == 4 && p.AccessMode == "full_access" && p.Contract == terminaltools.Contract && p.Verified && p.ProviderEnvironmentAbsent
}
