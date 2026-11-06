package orcatransport

import (
	"errors"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"

	"strings"
)

const remoteWireCommand = "native-command-v1"
const extensionWireCommand = "terminal-extension-v1"
const remoteModelWireCommand = "native-model-command-v1"

func wirePrompt(t *Turn) string {
	if t.WireFormat == remoteModelWireCommand {
		return codex.RemoteModelPrompt(t.Command, t.InputDigest, t.StepID, t.Model, t.ReasoningEffort, t.Prompt)
	}
	if t.WireFormat != remoteWireCommand && t.WireFormat != extensionWireCommand {
		return t.Prompt
	}
	// The TUI removes trailing line terminators. Compile that wire text before
	// hashing, so history verification compares every delivered byte.
	prompt := nativePromptText(t.Prompt)
	inputDigest := t.InputDigest
	if t.WireFormat == extensionWireCommand {
		inputDigest = digest([]byte(prompt))
	}
	return fmt.Sprintf("[SteerRunner command=%s input=%s]\n%s", t.Command, inputDigest, prompt)
}

func nativePromptText(text string) string {
	return strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\r\n")
}

func validateReaderExit(state State, turn *Turn, value codex.RemoteTurnReceipt) error {
	ready := state.RemoteReadiness
	r := value.Readiness
	if ready == nil || !value.ReaderWritersAbsent || turn.WireFormat != remoteWireCommand && turn.WireFormat != remoteModelWireCommand || !turn.Accepted || value.Source != codex.RemoteTurnSource || value.InputDigest != digest([]byte(wirePrompt(turn))) || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.SessionID != ready.SessionID || r.ConfigurationDigest != ready.ConfigurationDigest || r.RuntimeSHA256 != ready.RuntimeSHA256 || r.Repository != ready.Repository || r.Contract != ready.Contract || !strings.EqualFold(strings.ReplaceAll(r.Workspace, "\\", "/"), strings.ReplaceAll(ready.Workspace, "\\", "/")) || r.Process.PID != ready.Process.PID || r.Process.Created != ready.Process.Created {
		return errors.New("original native receipt binding changed")
	}
	if turn.Model != "" && value.Started && (value.Model != turn.Model || value.ReasoningEffort != turn.ReasoningEffort) {
		return errors.New("original native model differs from the frozen workflow turn")
	}
	if r.Owner != ready.Owner || string(r.ExitPublicKey) != string(ready.ExitPublicKey) || r.Process != ready.Process || r.AccessMode != ready.AccessMode || !r.CheckedAt.Equal(ready.CheckedAt) {
		return errors.New("original native readiness identity changed")
	}
	return nil
}

func validateRecordedTurn(state State, turn *Turn, value codex.RemoteTurnReceipt) error {
	if err := validateReaderExit(state, turn, value); err != nil {
		return err
	}
	if value.InspectionFailed {
		return errors.New("original native history query failed after reader cleanup")
	}
	if value.Completed && !value.Started || value.Started && (!codex.ValidNativeID(value.NativeTurnID) || value.UserItemID == "") || value.FinalText != "" && (len(value.FinalText) > maxReplyBytes || value.FinalItemID == "" || value.FinalDigest != digest([]byte(value.FinalText))) {
		return errors.New("original native receipt evidence is incomplete")
	}
	if previous := turn.Recorded; previous != nil && (previous.NativeTurnID != value.NativeTurnID || previous.UserItemID != value.UserItemID || previous.Completed && (!value.Completed || previous.Status != value.Status || previous.FinalItemID != value.FinalItemID || previous.FinalDigest != value.FinalDigest)) {
		return errors.New("original native receipt changed after capture")
	}
	return nil
}

// ValidateRecordedTurn independently verifies persisted native completion and
// model evidence using the same original resource/input binding as observation.
func ValidateRecordedTurn(state State, turn *Turn) error {
	if turn == nil || turn.Recorded == nil {
		return errors.New("original native receipt missing")
	}
	return validateRecordedTurn(state, turn, *turn.Recorded)
}

// The fixed original control actor reads native history without a model input.
