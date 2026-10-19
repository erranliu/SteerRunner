package orcatransport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"strings"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

const remoteWireCommand = "native-command-v1"
const extensionWireCommand = "terminal-extension-v1"

func wirePrompt(t *Turn) string {
	if t.WireFormat != remoteWireCommand && t.WireFormat != extensionWireCommand {
		return t.Prompt
	}
	// The TUI removes trailing line terminators. Compile that wire text before
	// hashing, so history verification compares every delivered byte.
	prompt := strings.TrimRight(strings.ReplaceAll(t.Prompt, "\r\n", "\n"), "\r\n")
	inputDigest := t.InputDigest
	if t.WireFormat == extensionWireCommand {
		inputDigest = digest([]byte(prompt))
	}
	return fmt.Sprintf("[SteerRunner command=%s input=%s]\n%s", t.Command, inputDigest, prompt)
}

type NativeInspection struct {
	NativeSettled bool               `json:"native_settled,omitempty"`
	Attempted     bool               `json:"attempted"`
	Unknown       bool               `json:"unknown"`
	Process       *b.ProcessIdentity `json:"process,omitempty"`
	Closed        bool               `json:"closed"`
	At            time.Time          `json:"at"`
}

func (l *Lifecycle) observeRecorded(ctx context.Context, state *State, turn *Turn) string {
	if !turn.Accepted {
		return "Original input has no verified acceptance; no input was resent"
	}
	release, gateErr := l.inspectionGate(ctx)
	if gateErr != nil {
		return gateErr.Error()
	}
	defer release()
	if _, err := l.settleInspections(ctx, state); err != nil {
		return "Original native history cleanup is unverified: " + err.Error()
	}
	if turn.Inspection == nil && l.clock().Before(turn.Native.LastQuery.Add(30*time.Second)) {
		// The accepted turn already has a bounded observation interval. Do not
		// spawn a history client immediately while the original model is working.
		// A stream notification is only a polling hint; history still establishes
		// completion, including fast turns whose start notification was missed.
		due := false
		if feed, err := l.stateFeed(ctx, state.Process); err == nil {
			signals, _ := feed.Drain()
			signals = append(signals, feed.Initial())
			for _, s := range signals {
				if s.PaneKey == turn.Native.Pane && s.ProviderSession.ID == state.RemoteReadiness.SessionID && trustedState(s) && s.Observation.Revision > turn.Native.BaseRevision && s.MainAgent.State == "done" && s.MainAgent.StateStartedAt > turn.Native.BaseRootAt && s.Prompt == preview(wirePrompt(turn)) && !s.SessionBoundary && !s.RestoredUnconfirmed {
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
	if turn.Inspection != nil && !turn.Inspection.Unknown && turn.Inspection.Closed && l.clock().Before(turn.Inspection.At.Add(30*time.Second)) {
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
	if ready == nil || !value.ReaderWritersAbsent || turn.WireFormat != remoteWireCommand || !turn.Accepted || value.Source != codex.RemoteTurnSource || value.InputDigest != digest([]byte(wirePrompt(turn))) || !r.Verified || r.NativeTransport != codex.RemoteNativeTransport || r.SessionID != ready.SessionID || r.ConfigurationDigest != ready.ConfigurationDigest || r.RuntimeSHA256 != ready.RuntimeSHA256 || r.Repository != ready.Repository || r.Contract != ready.Contract || !strings.EqualFold(strings.ReplaceAll(r.Workspace, "\\", "/"), strings.ReplaceAll(ready.Workspace, "\\", "/")) || r.Process.PID != ready.Process.PID || r.Process.Created != ready.Process.Created {
		return errors.New("original native receipt binding changed")
	}
	if r.Process != ready.Process || r.Owner != ready.Owner || string(r.ExitPublicKey) != string(ready.ExitPublicKey) || r.AccessMode != ready.AccessMode || !r.CheckedAt.Equal(ready.CheckedAt) || !r.ExpiresAt.Equal(ready.ExpiresAt) {
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

// This inspector starts only a read-only history client. It never resumes a
// thread, sends a model prompt, or borrows the business terminal for commands.
// An uncertain creation/closure is retained and blocks replacement inspectors.
func (l *Lifecycle) recordedTurn(ctx context.Context, state *State, turn *Turn) (*codex.RemoteTurnReceipt, error) {
	if turn.Inspection != nil && turn.Inspection.Unknown {
		return nil, errors.New("original native inspector ownership is unverified; no replacement created")
	}
	if turn.Inspection != nil && !turn.Inspection.Closed {
		return nil, errors.New("original native inspector is not settled")
	}
	query := codex.RemoteTurnQuery{Workspace: state.RemoteReadiness.Workspace, Session: state.RemoteReadiness.SessionID, InputDigest: digest([]byte(wirePrompt(turn)))}
	raw, _ := json.Marshal(query)
	command := "& '" + strings.ReplaceAll(l.profile.RuntimeExecutable, "'", "''") + "' '--original-turn' '" + base64.RawURLEncoding.EncodeToString(raw) + "'"
	attempt := &NativeInspection{Attempted: true, At: l.clock()}
	turn.Inspection = attempt
	j, journalErr := l.loadInspections(*state)
	if journalErr != nil {
		return nil, journalErr
	}
	j.Entries = append(j.Entries, inspectionEntry{ID: contracts.NewID(), Command: turn.Command, InputDigest: query.InputDigest, Inspection: *attempt})
	index := len(j.Entries) - 1
	if err := l.saveInspections(*state, j); err != nil {
		return nil, err
	}
	checkpoint := func() error { j.Entries[index].Inspection = *attempt; return l.saveInspections(*state, j) }
	result, err := l.client.MetaCall(ctx, "terminal", "create", "--worktree", "id:"+state.Process.WorktreeID, "--title", "SteerRunner original native receipt "+turn.Command, "--command", command, "--json")
	var created struct{ Terminal native.Terminal }
	if err != nil || result.CallUncertain || json.Unmarshal(result.Payload, &created) != nil || result.RuntimeID != state.Process.RuntimeID || created.Terminal.Handle == "" || created.Terminal.IncarnationID == "" || created.Terminal.WorktreeID != state.Process.WorktreeID {
		attempt.Unknown = true
		_ = checkpoint()
		return nil, errors.New("original native inspector creation unverified")
	}
	inspector := b.ProcessIdentity{Kind: "orca_original_turn_reader", RuntimeID: result.RuntimeID, Terminal: created.Terminal.Handle, Incarnation: created.Terminal.IncarnationID, WorktreeID: state.Process.WorktreeID}
	attempt.Process = &inspector
	if err := checkpoint(); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, closeErr := l.client.CloseReadonlyTerminal(closeCtx, inspector)
		attempt.Unknown = true
		return nil, errors.Join(err, closeErr)
	}
	var value *codex.RemoteTurnReceipt
	readErr := func() error {
		for {
			if _, e := l.client.InspectBoundTerminal(ctx, b.ExecutionTarget{}, inspector); e != nil {
				return e
			}
			result, e := l.client.MetaCall(ctx, "terminal", "read", "--terminal", inspector.Terminal, "--limit", "150", "--json")
			if e != nil {
				return e
			}
			var output struct {
				Terminal struct {
					Handle string
					Tail   []string
				}
			}
			if json.Unmarshal(result.Payload, &output) != nil || result.RuntimeID != inspector.RuntimeID || output.Terminal.Handle != inspector.Terminal {
				return errors.New("original native inspector read identity changed")
			}
			for _, line := range output.Terminal.Tail {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, codex.RemoteTurnPrefix) {
					body, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(line, codex.RemoteTurnPrefix))
					var candidate codex.RemoteTurnReceipt
					if e != nil || json.Unmarshal(body, &candidate) != nil {
						return errors.New("original native receipt is invalid")
					}
					if e = validateReaderExit(*state, turn, candidate); e != nil {
						return e
					}
					attempt.NativeSettled = true
					if e = checkpoint(); e != nil {
						return e
					}
					if e = validateRecordedTurn(*state, turn, candidate); e != nil {
						return e
					}
					value = &candidate
					return nil
				}
				if strings.HasPrefix(line, "remote original turn inspection failed") {
					return errors.New("original native history unavailable")
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	proof, closeErr := l.client.CloseReadonlyTerminal(closeCtx, inspector)
	attempt.Closed = closeErr == nil && proof.Disposition == "confirmed"
	attempt.Unknown = !attempt.Closed
	if err := checkpoint(); err != nil {
		return nil, errors.Join(readErr, closeErr, err)
	}
	if !attempt.Closed {
		return nil, errors.Join(readErr, errors.New("original native inspector closure unverified"))
	}
	return value, readErr
}
