package application

import (
	"context"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"time"
)

type nativeViewer interface {
	NativeViewerCommand(context.Context, b.LifecycleSnapshot) (b.LaunchCommand, error)
}

// OpenNativeSession opens a viewer under the original worker's dispatch gate.
// It never commits Run state, starts a service, resumes or submits an input.
func (e *Engine) OpenNativeSession(target contracts.NativeSessionTarget) contracts.Response {
	reject := func(code, message string, exit int) contracts.Response {
		return contracts.Rejected("", contracts.Fail(exit, code, message))
	}
	if target.RunID == "" || target.AttemptID == "" || target.SessionID == "" || target.Generation == "" {
		return reject("invalid_arguments", "Exact native session identities are required.", 2)
	}
	e.mu.Lock()
	w := e.workers[target.RunID]
	e.mu.Unlock()
	if w == nil {
		return reject("run_not_found", "Original Run was not found.", 5)
	}
	w.gate.Lock()
	defer w.gate.Unlock()
	if e.ctx.Err() != nil {
		return reject("service_unavailable", "Service is stopping.", 4)
	}
	e.mu.Lock()
	r := e.runs[target.RunID].Clone()
	e.mu.Unlock()
	s := r.Snapshot
	if s.Session.AttemptID != target.AttemptID || s.Session.SessionID != target.SessionID || s.ResourceGeneration != target.Generation {
		return reject("stale_revision", "Original native session target changed; refresh before opening.", 5)
	}
	if r.Request.Execution.Adapter != "codex-app-server" {
		return reject("unsupported_capability", "Native session viewing is unavailable for this adapter.", 3)
	}
	p, err := e.port(r, w)
	if err != nil {
		return reject("environment_unavailable", "Original native viewer configuration is unavailable.", 4)
	}
	viewer, ok := p.(nativeViewer)
	if !ok {
		return reject("unsupported_capability", "Adapter does not provide native viewing.", 3)
	}
	ctx, cancel := context.WithTimeout(e.ctx, 10*time.Second)
	defer cancel()
	command, err := viewer.NativeViewerCommand(ctx, s.Clone())
	if err != nil {
		return reject("environment_unavailable", "Original process, session or workspace could not be verified; no viewer opened.", 4)
	}
	open := e.openNativeConsole
	if open == nil {
		open = process.OpenNativeConsole
	}
	if err := open(ctx, command); err != nil {
		return reject("environment_unavailable", "Native viewer window could not be opened.", 4)
	}
	return contracts.Success("", contracts.NativeSessionOpened{NativeSessionTarget: target, Opened: true})
}
