package orcatransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
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

func (l *Lifecycle) observeRecorded(ctx context.Context, state *State, turn *Turn) string {
	if l.profile.Environment != "" {
		return "helper history readers are local only"
	}
	if !turn.Accepted {
		return "Original input has no verified acceptance; no input was resent"
	}
	release, gateErr := l.inspectionGate(ctx)
	if gateErr != nil {
		return gateErr.Error()
	}
	defer release()
	if turn.ReceiptQueriedAt.IsZero() && l.clock().Before(turn.Native.LastQuery.Add(30*time.Second)) {
		// The accepted turn already has a bounded observation interval. Do not
		// spawn a history client immediately while the original model is working.
		// A stream notification is only a polling hint; history still establishes
		// completion, including fast turns whose start notification was missed.
		due := false
		if feed, err := l.stateFeed(ctx, state.Process); err == nil {
			signals, _ := feed.Drain()
			signals = append(signals, feed.Initial())
			for _, s := range signals {
				if s.PaneKey == turn.Native.Pane && s.ProviderSession.ID == state.RemoteReadiness.SessionID && trustedState(s.AgentState) && s.Observation.Revision > turn.Native.BaseRevision && s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt && s.Prompt == preview(wirePrompt(turn)) && !s.SessionBoundary && !s.RestoredUnconfirmed {
					due = true
				}
			}
		} else if errors.Is(err, native.ErrStateIdentityChanged) {
			return err.Error()
		}
		if !due {
			return ""
		}
	}
	if !turn.ReceiptQueriedAt.IsZero() && l.clock().Before(turn.ReceiptQueriedAt.Add(30*time.Second)) {
		return ""
	}
	inspectCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	value, err := l.recordedTurn(inspectCtx, state, turn)
	if err != nil {
		return "Original native history is unverified: " + err.Error()
	}
	if value == nil || !value.Started {
		return "Original native input not yet recorded; no input was resent"
	}
	turn.Recorded = value
	turn.Started = true
	if !value.Completed {
		return ""
	}
	if value.Status != "completed" {
		return "Original native turn ended without successful completion"
	}
	if value.FinalText == "" {
		return "Original native turn ended without a complete final_answer"
	}
	turn.Reply, turn.ReplyDigest, turn.Settled = value.FinalText, value.FinalDigest, true
	return ""
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
	if r.AuthenticationControl != ready.AuthenticationControl || r.Owner != ready.Owner || string(r.ExitPublicKey) != string(ready.ExitPublicKey) || r.Process != ready.Process || r.AccessMode != ready.AccessMode || !r.CheckedAt.Equal(ready.CheckedAt) || !r.ExpiresAt.Equal(ready.ExpiresAt) {
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
func (l *Lifecycle) recordedTurn(ctx context.Context, state *State, turn *Turn) (*codex.RemoteTurnReceipt, error) {
	if l.profile.Environment != "" {
		return nil, errors.New("helper history is local only")
	}
	turn.ReceiptQueriedAt = l.clock()
	response, err := l.callControl(ctx, *state, "original-turn", digest([]byte(wirePrompt(turn))))
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}
	if response.Turn == nil {
		return nil, errors.New("original native control receipt missing")
	}
	if err := validateRecordedTurn(*state, turn, *response.Turn); err != nil {
		return nil, err
	}
	turn.ReceiptRequest, turn.ReceiptQueriedAt = response.ID, l.clock()
	return response.Turn, nil
}
