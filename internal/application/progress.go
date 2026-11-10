package application

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"reflect"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// File collection only needs frozen routing facts. It must not construct the
// business lifecycle setup, which owns a worker handle and preparation gate.
func progressSetup(r *runstate.Record) b.LifecycleSetup {
	return b.LifecycleSetup{Snapshot: r.Snapshot.Clone(), ProfileData: r.Profile, ProfileID: r.ID, AcceptedConfigVersion: r.InputDigest}
}

func progressBinding(r *runstate.Record) string {
	values := []any{r.ID, r.InputDigest, r.Snapshot.Session.ProjectID, r.Snapshot.Session.ExecutionID, r.Snapshot.Workspace.Path, r.Request.Execution}
	if r.Request.Execution.Adapter == "orca-terminal" {
		values = append(values, r.Snapshot.Ref.External, r.Snapshot.Ref.Target)
	}
	return reliable.Digest(canonical(values))
}
func (e *Engine) saveProgressLocked(r *runstate.Record, p *runstate.Progress) error {
	current := e.runs[r.ID]
	next := current.Clone()
	next.Progress = p
	next = next.Clone() // do not publish mutable preparation-local pointers
	preserveInitializationSettlement(next.Progress, current.Progress)
	next.ResultFile = current.ResultFile
	if err := e.store.SaveProgress(next); err != nil {
		return err
	}
	e.runs[r.ID] = next
	return nil
}
func (e *Engine) readProgress(ctx context.Context, r *runstate.Record) ([]byte, error) {
	c, err := e.runtimeFile(ctx, r, false, false)
	if err != nil {
		return nil, err
	}
	defer c.close()
	return c.read(ctx)
}
func (e *Engine) HandleProgress(id string) contracts.Response {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id]
	if r == nil {
		return contracts.Rejected("", contracts.Fail(5, "run_not_found", "Run was not found."))
	}
	v := contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: id, Encoding: "base64", CollectionStatus: "pending"}
	if r.Request.ProgressFile == nil {
		v.CollectionStatus = "disabled"
	} else if r.Progress != nil {
		v = r.Clone().Progress.View
	}
	return contracts.Success("", v)
}
func progressTerminal(r *runstate.Record) bool {
	return r.StopStatus == "stopped" || r.Closed || r.Status == "pr_ready" || r.Status == "completed"
}
func (e *Engine) collectProgress(ctx context.Context, id, final string) {
	e.mu.Lock()
	r := e.runs[id].Clone()
	w := e.workers[id]
	attempted := r.Progress != nil && (r.Progress.InitializationAttempted || r.Progress.Initialized) || w != nil && w.filesPreparationStarted || fileInputDispatched(r)
	e.mu.Unlock()
	if r.Request.ProgressFile == nil {
		return
	}
	if !attempted {
		if final != "" {
			e.mu.Lock()
			current := e.runs[id]
			if current.Progress == nil || !current.Progress.Initialized {
				p := current.Clone().Progress
				if p == nil {
					p = &runstate.Progress{Workspace: current.Snapshot.Workspace.Path, Binding: progressBinding(current), View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: id, Encoding: "base64"}}
				}
				p.View.CollectionStatus = progressFinalStatus(current, final)
				_ = e.saveProgressLocked(current, p)
			}
			e.mu.Unlock()
		}
		return
	}
	sample := e.sampleRuntimeFile(ctx, r, false, false)
	data, err := sample.data, sample.err
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if !reflect.DeepEqual(current.Progress, r.Progress) {
		return
	}
	next := fileState(current, false)
	if err == nil {
		view := next.View
		next = sample.state
		next.View = view
		next.InitializationAttempted = true
	}
	v := &next.View
	status, diagnostic := "observing", ""
	if err != nil {
		status = "unavailable"
		diagnostic = progressDiagnostic(err)
	}
	if final != "" {
		status = progressFinalStatus(current, final)
	}
	changed := false
	if err == nil {
		digest := reliable.Digest(data)
		if v.Version == 0 || v.SHA256 != digest {
			encoded := base64.StdEncoding.EncodeToString(data)
			now := time.Now().UTC()
			v.Version++
			v.ContentBase64 = &encoded
			v.SizeBytes = len(data)
			v.SHA256 = digest
			v.ObservedAt = &now
			changed = true
		}
	}
	if changed || v.CollectionStatus != status || v.Diagnostic != diagnostic || !reflect.DeepEqual(current.Progress, next) {
		v.CollectionStatus = status
		v.Diagnostic = diagnostic
		if e.saveProgressLocked(current, next) != nil && e.logger != nil {
			e.logger.Printf("progress commit unavailable run=%s", id)
		}
	}
}
func progressDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "binding_invalid", "workspace_unavailable", "directory_unavailable", "file_missing", "read_failed", "size_exceeded", "file_changing", "initialization_unverified", "transport_unavailable":
		return err.Error()
	}
	return "read_failed"
}
func (e *Engine) observeProgress(id string) {
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		w := e.workers[id]
		w.progressWatching = false
		if e.ctx.Err() == nil && !progressTerminal(e.runs[id]) {
			e.startProgressLocked(id, w)
		}
	}()
	timer := time.NewTicker(300 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-e.ctx.Done():
			e.finishProgress(id, "detached")
			return
		case <-timer.C:
			e.mu.Lock()
			terminal := progressTerminal(e.runs[id])
			e.mu.Unlock()
			if terminal {
				e.finishProgress(id, "closed")
				return
			}
			ctx, cancel := context.WithTimeout(e.ctx, 2*time.Second)
			e.collectProgress(ctx, id, "")
			cancel()
		}
	}
}
func (e *Engine) finishProgress(id, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		e.collectProgress(ctx, id, "")
		select {
		case <-ctx.Done():
			e.collectProgress(ctx, id, status)
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	e.collectProgress(ctx, id, status)
}

func (e *Engine) startProgressLocked(id string, w *worker) {

	if !w.progressEnabled || w.progressWatching || e.ctx.Err() != nil {
		return
	}
	// A completed final sample belongs to that terminal Run. Service reloads
	// must not reopen its source; a terminal Run missing that sample still gets
	// one bounded finalization, and an explicitly resumed Run can observe again.
	r := e.runs[id]
	if progressTerminal(r) && r.Progress != nil && r.Progress.View.CollectionStatus == "closed" {
		return
	}
	w.progressWatching = true
	e.wait.Add(1)
	go func() { defer e.wait.Done(); e.observeProgress(id) }()
}

func progressFinalStatus(r *runstate.Record, requested string) string {
	if progressTerminal(r) {
		return "closed"
	}
	return requested
}
func progressPath(r *runstate.Record) string {
	if r.Request.ProgressFile == nil {
		return ""
	}
	return filepath.Join(r.Snapshot.WorkspacePlan.Path, filepath.FromSlash(workspace.ProgressRelative(r.ID)))
}
