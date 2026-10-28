package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

type nativeInputDiagnosticKey struct{}

type nativeInputDiagnostic struct {
	OperationID string            `json:"operation_id"`
	CommandID   string            `json:"command_id"`
	RunID       string            `json:"run_id"`
	AttemptID   string            `json:"attempt_id"`
	InputBytes  int               `json:"input_bytes"`
	InputSHA256 string            `json:"input_sha256"`
	Process     b.ProcessIdentity `json:"process"`
	Phase       string            `json:"phase"`
	Directory   string            `json:"-"`
}

// These are local diagnostics, never receipts or authorization to replay.
// Preserve the entry before any effect, and retain a result even on cancellation.
func (l *Lifecycle) beginNativeInput(ctx context.Context, r b.LifecycleRequest, phase string) (context.Context, func(b.LifecycleReceipt, error) error, error) {
	if l.profile.Environment == "" || l.profile.Terminal != nil || r.Snapshot.Ref.Agent.Name != "codex" || l.setup.RunDirectory == "" || r.Input.Turn == nil {
		return ctx, func(_ b.LifecycleReceipt, e error) error { return e }, nil
	}
	d := nativeInputDiagnostic{OperationID: r.Operation.ID, CommandID: r.Input.Turn.CommandID, RunID: r.Snapshot.Ref.RunID, AttemptID: r.Snapshot.Ref.AttemptID, InputBytes: len(r.Input.Turn.Prompt), InputSHA256: digest([]byte(r.Input.Turn.Prompt)), Process: r.Snapshot.Ref.External, Phase: phase}
	d.Directory = filepath.Join(l.setup.RunDirectory, "native-input", digest([]byte(d.OperationID)), process.New(0).NewProcessGeneration())
	started := time.Now().UTC()
	if err := writeNativeDiagnostic(filepath.Join(d.Directory, "input.intent.json"), map[string]any{"at": started, "input": d}); err != nil {
		return ctx, nil, err
	}
	finish := func(receipt b.LifecycleReceipt, cause error) error {
		message := ""
		if cause != nil {
			message = cause.Error()
		}
		err := writeNativeDiagnostic(filepath.Join(d.Directory, "input.result.json"), map[string]any{"started_at": started, "finished_at": time.Now().UTC(), "input": d, "receipt": receipt, "error": message})
		return errors.Join(cause, err)
	}
	return context.WithValue(ctx, nativeInputDiagnosticKey{}, d), finish, nil
}

func nativeInputPhase(ctx context.Context, phase string) context.Context {
	if d, ok := ctx.Value(nativeInputDiagnosticKey{}).(nativeInputDiagnostic); ok {
		d.Phase = phase
		return context.WithValue(ctx, nativeInputDiagnosticKey{}, d)
	}
	return ctx
}

func writeNativeDiagnostic(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return process.New(0).WriteExecutionFile(path, raw)
}
