package application

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
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
	next.ResultFile = current.ResultFile
	if err := e.store.SaveProgress(next); err != nil {
		return err
	}
	e.runs[r.ID] = next
	return nil
}
func (e *Engine) prepareProgress(r *runstate.Record) (outErr error) {
	defer func() {
		if outErr == nil {
			return
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.runs[r.ID]
		if current == nil {
			return
		}
		p := current.Clone().Progress
		if p == nil {
			p = &runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: r.ID, Encoding: "base64"}}
		}
		p.View.CollectionStatus, p.View.Diagnostic = "unavailable", progressDiagnostic(outErr)
		_ = e.saveProgressLocked(current, p)
	}()
	if r.Request.ProgressFile == nil || r.Snapshot.Workspace.Path == "" {
		return nil
	}
	if r.Progress != nil && r.Progress.Initialized {
		return nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, 4*time.Second)
	defer cancel()
	p := r.Progress
	if p == nil || p.HostID == "" {
		p = &runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), HostID: "local", View: contracts.ProgressFileView{Contract: contracts.ProgressContract, RunID: r.ID, Encoding: "base64", CollectionStatus: "initializing"}}
		if r.Request.Execution.Adapter == "orca-terminal" {
			host, err := orca.ProgressHost(progressSetup(r))
			if err != nil {
				return errors.New("binding_invalid")
			}
			p.HostID = host
			source, err := orca.ProgressSource(ctx, progressSetup(r), host, "")
			if err != nil {
				return err
			}
			p.WorkspaceInstance = source.Instance
			source.Close()
		} else {
			if err := verifyOriginalWorkspace(ctx, gitHost(), r, true); err != nil {
				return errors.New("binding_invalid")
			}
			dirs, err := workspace.ProgressDirectories(p.Workspace, r.ID, true)
			if err != nil {
				return err
			}
			p.Directories = dirs
			out, err := gitHost().Output(ctx, p.Workspace, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
			if err != nil {
				return errors.New("directory_unavailable")
			}
			f, err := os.OpenFile(string(bytes.TrimSpace(out)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return errors.New("directory_unavailable")
			}
			_, err = f.WriteString("\n/.steerrunner/progress/\n")
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				return errors.New("directory_unavailable")
			}
		}
		e.mu.Lock()
		err := e.saveProgressLocked(e.runs[r.ID], p)
		e.mu.Unlock()
		if err != nil {
			return err
		}
	}
	if p.Binding != progressBinding(r) || p.Workspace != r.Snapshot.Workspace.Path {
		return errors.New("binding_invalid")
	}
	if !p.InitializationAttempted {
		initial, _ := base64.StdEncoding.DecodeString(r.Request.ProgressFile.InitialContentBase64)
		initialize := func() error { return workspace.InitializeProgress(p.Workspace, r.ID, p.Directories, initial) }
		if r.Request.Execution.Adapter == "orca-terminal" {
			source, err := orca.ProgressSource(ctx, progressSetup(r), p.HostID, p.WorkspaceInstance)
			if err != nil {
				return err
			} // positively before the write RPC; retry opening is safe
			defer source.Close()
			initialize = func() error { return source.Initialize(ctx, initial) }
		}
		// Persist before the cross-system effect. After an uncertain restart only
		// read the original file; never recreate it to repair missing evidence.
		p.InitializationAttempted = true
		e.mu.Lock()
		err := e.saveProgressLocked(e.runs[r.ID], p)
		e.mu.Unlock()
		if err != nil {
			return err
		}
		_ = initialize() // uncertain response is resolved by reading the same file
	}
	r.Progress = p
	seed, err := e.readProgress(ctx, r)
	if err != nil {
		return err
	}
	expected, _ := base64.StdEncoding.DecodeString(r.Request.ProgressFile.InitialContentBase64)
	if !bytes.Equal(seed, expected) {
		return errors.New("initialization_unverified")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[r.ID].Clone().Progress
	next.Initialized = true
	next.View.CollectionStatus = "observing"
	next.View.Diagnostic = ""
	return e.saveProgressLocked(e.runs[r.ID], next)
}
func (e *Engine) readProgress(ctx context.Context, r *runstate.Record) ([]byte, error) {
	p := r.Progress
	if p == nil || p.Binding != progressBinding(r) || p.Workspace != r.Snapshot.Workspace.Path {
		return nil, errors.New("binding_invalid")
	}
	if r.Request.Execution.Adapter == "orca-terminal" {
		c, err := orca.ProgressSource(ctx, progressSetup(r), p.HostID, p.WorkspaceInstance)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		first, err := c.Read(ctx)
		if err != nil {
			return nil, err
		}
		second, err := c.Read(ctx)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(first, second) {
			return nil, errors.New("file_changing")
		}
		return second, nil
	}
	return workspace.ReadProgress(p.Workspace, r.ID, p.Directories)
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
	e.mu.Unlock()
	if r.Progress == nil || !r.Progress.Initialized {
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
	data, err := e.readProgress(ctx, r)
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.runs[id]
	if current.Progress == nil || !reflect.DeepEqual(current.Progress, r.Progress) {
		return
	}
	next := current.Clone().Progress
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
	if changed || v.CollectionStatus != status || v.Diagnostic != diagnostic {
		v.CollectionStatus = status
		v.Diagnostic = diagnostic
		if e.saveProgressLocked(current, next) != nil {
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
