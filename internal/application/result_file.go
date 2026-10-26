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
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func resultPath(r *runstate.Record) string {
	if r.Request.ResultFile == nil {
		return ""
	}
	return filepath.Join(r.Snapshot.WorkspacePlan.Path, filepath.FromSlash(workspace.ResultRelative(r.ID)))
}

func filesNeedPreparation(r *runstate.Record) bool {
	return r.Request.ProgressFile != nil && (r.Progress == nil || !r.Progress.Initialized) ||
		r.Request.ResultFile != nil && (r.ResultFile == nil || !r.ResultFile.Initialized && !r.ResultFile.Sealed)
}

// The worker holds its existing dispatch gate. Try both independent preparations
// even if one fails; never interpret uncertain initialization as ready for input.
func (e *Engine) prepareRuntimeFiles(r *runstate.Record) bool {
	progressErr := e.prepareProgress(r.Clone())
	e.mu.Lock()
	current := e.runs[r.ID].Clone()
	e.mu.Unlock()
	resultErr := e.prepareResult(current)
	return progressErr == nil && resultErr == nil
}

// Called with Engine.mu held. Save only against the current aggregate.
func (e *Engine) saveResultLocked(id string, p *runstate.ResultFile) error {
	current := e.runs[id]
	next := current.Clone()
	next.ResultFile = p
	next = next.Clone()
	next.Progress = current.Progress
	if err := e.store.SaveProgress(next); err != nil {
		return err
	}
	e.runs[id] = next
	return nil
}

func (e *Engine) saveResultPreparationLocked(original *runstate.Record, p *runstate.ResultFile) error {
	current := e.runs[original.ID]
	if current == nil || progressBinding(current) != progressBinding(original) ||
		current.PendingStop != "" || progressTerminal(current) || e.ctx.Err() != nil ||
		current.ResultFile != nil && current.ResultFile.Sealed {
		return errors.New("binding_invalid")
	}
	return e.saveResultLocked(original.ID, p)
}

func (e *Engine) prepareResult(r *runstate.Record) (outErr error) {
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
		p := current.Clone().ResultFile
		if p == nil {
			p = &runstate.ResultFile{Progress: runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), View: contracts.ProgressFileView{Contract: contracts.ResultFileContract, RunID: r.ID, Encoding: "base64"}}}
		}
		p.View.CollectionStatus, p.View.Diagnostic = "pending", progressDiagnostic(outErr)
		_ = e.saveResultPreparationLocked(r, p)
	}()
	if r.Request.ResultFile == nil || r.Snapshot.Workspace.Path == "" {
		return nil
	}
	if r.ResultFile != nil && (r.ResultFile.Initialized || r.ResultFile.Sealed) {
		return nil
	}
	e.mu.Lock()
	current := e.runs[r.ID]
	allowed := current != nil && current.PendingStop == "" && !progressTerminal(current) && progressBinding(current) == progressBinding(r) && e.ctx.Err() == nil
	e.mu.Unlock()
	if !allowed {
		return errors.New("binding_invalid")
	}
	ctx, cancel := context.WithTimeout(e.ctx, 4*time.Second)
	defer cancel()
	p := r.Clone().ResultFile
	if p == nil || p.HostID == "" {
		attempted := p != nil && p.InitializationAttempted
		p = &runstate.ResultFile{Progress: runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), HostID: "local", View: contracts.ProgressFileView{Contract: contracts.ResultFileContract, RunID: r.ID, Encoding: "base64", CollectionStatus: "pending"}}}
		p.InitializationAttempted = attempted
		if r.Request.Execution.Adapter == "orca-terminal" {
			host, err := orca.ProgressHost(progressSetup(r))
			if err != nil {
				return errors.New("binding_invalid")
			}
			p.HostID = host
			source, err := orca.ResultSource(ctx, progressSetup(r), host, "")
			if err != nil {
				return err
			}
			p.WorkspaceInstance = source.Instance
			source.Close()
		} else {
			if err := verifyOriginalWorkspace(ctx, gitHost(), r, true); err != nil {
				return errors.New("binding_invalid")
			}
			dirs, err := workspace.ResultDirectories(p.Workspace, r.ID, true)
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
			_, err = f.WriteString("\n/.steerrunner/result/\n")
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				return errors.New("directory_unavailable")
			}
		}
		e.mu.Lock()
		err := e.saveResultPreparationLocked(r, p)
		e.mu.Unlock()
		if err != nil {
			return err
		}
	}
	if p.Binding != progressBinding(r) || p.Workspace != r.Snapshot.Workspace.Path {
		return errors.New("binding_invalid")
	}
	if !p.InitializationAttempted {
		initial, _ := base64.StdEncoding.DecodeString(r.Request.ResultFile.InitialContentBase64)
		initialize := func() error { return workspace.InitializeResult(p.Workspace, r.ID, p.Directories, initial) }
		if r.Request.Execution.Adapter == "orca-terminal" {
			source, err := orca.ResultSource(ctx, progressSetup(r), p.HostID, p.WorkspaceInstance)
			if err != nil {
				return err
			} // positively before the write RPC; retry opening is safe
			defer source.Close()
			initialize = func() error { return source.Initialize(ctx, initial) }
		}
		// Persist before the cross-system effect. After an uncertain restart only
		// read the original file; never recreate it to repair missing evidence.
		if err := ctx.Err(); err != nil {
			return err
		}
		p.InitializationAttempted = true
		e.mu.Lock()
		err := e.saveResultPreparationLocked(r, p)
		e.mu.Unlock()
		if err != nil {
			return err
		}
		_ = initialize() // uncertain response is resolved by reading the same file
	}
	r.ResultFile = p
	seed, err := e.readResult(ctx, r)
	if err != nil {
		return err
	}
	expected, _ := base64.StdEncoding.DecodeString(r.Request.ResultFile.InitialContentBase64)
	if !bytes.Equal(seed, expected) {
		return errors.New("initialization_unverified")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[r.ID].Clone().ResultFile
	next.Initialized = true
	next.View.CollectionStatus = "pending"
	next.View.Diagnostic = ""
	return e.saveResultPreparationLocked(r, next)
}
func (e *Engine) readResult(ctx context.Context, r *runstate.Record) ([]byte, error) {
	p := r.ResultFile
	if p == nil || p.Binding != progressBinding(r) || p.Workspace != r.Snapshot.Workspace.Path {
		return nil, errors.New("binding_invalid")
	}
	if r.Request.Execution.Adapter == "orca-terminal" {
		c, err := orca.ResultSource(ctx, progressSetup(r), p.HostID, p.WorkspaceInstance)
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
	return workspace.ReadResult(p.Workspace, r.ID, p.Directories)
}

func (e *Engine) HandleResultFile(id string) contracts.Response {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id]
	if r == nil {
		return contracts.Rejected("", contracts.Fail(5, "run_not_found", "Run was not found."))
	}
	v := contracts.ResultFileView{Contract: contracts.ResultFileContract, RunID: id, Encoding: "base64", CollectionStatus: "pending"}
	if r.Request.ResultFile == nil {
		v.CollectionStatus = "disabled"
	} else if r.ResultFile != nil {
		v = r.Clone().ResultFile.View
	}
	return contracts.Success("", v)
}

// StopStatus alone can be set by a service shutdown that leaves Run authority
// intact. Require the original whole-execution end as well. pr_ready, one turn,
// observation loss and an accepted resume are not final-result eligibility.
func resultEndEligible(r *runstate.Record) bool {
	return r.StopStatus == "stopped" && r.PendingResume == "" &&
		(r.Status == "stopped" || r.Status == "completed" || r.Closed)
}

func resultEndIdentity(r *runstate.Record) string {
	stopID := ""
	for _, intent := range r.Intents {
		if intent.Request.Operation.Action == "stop_execution" {
			stopID = intent.Request.Operation.ID
		}
	}
	return reliable.Digest(canonical([]any{progressBinding(r), r.Snapshot.Ref, r.Snapshot.ResourceGeneration, r.Snapshot.Session.Generation, r.Snapshot.Session.CurrentTurnID, stopID}))
}

// One physical collection attempt, including identity and stability checks.
// It is bounded by both the existing lifecycle deadline and two seconds.
func (e *Engine) sealResult(parent context.Context, id string) {
	e.mu.Lock()
	r := e.runs[id].Clone()
	if r.Request.ResultFile == nil || !resultEndEligible(r) || r.ResultFile != nil && r.ResultFile.Sealed {
		e.mu.Unlock()
		return
	}
	if w := e.workers[id]; w != nil {
		if w.resultCollectionStarted {
			e.mu.Unlock()
			return
		}
		w.resultCollectionStarted = true
	}
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	var data []byte
	var err error
	if r.ResultFile == nil || !r.ResultFile.Initialized {
		err = errors.New("initialization_unverified")
	} else {
		data, err = boundedResultRead(ctx, func() ([]byte, error) { return e.readResult(ctx, r) })
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.publishResultLocked(r, data, err); err != nil {
		e.logger.Printf("result file seal unavailable run=%s", id)
	}
}

// A blocked transport cannot extend the original stop deadline. A late physical
// read is never published; crash recovery may read again before a durable seal.
func boundedResultRead(ctx context.Context, read func() ([]byte, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type sample struct {
		data []byte
		err  error
	}
	done := make(chan sample, 1)
	go func() { data, err := read(); done <- sample{data, err} }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case value := <-done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return value.data, value.err
	}
}

// Must hold Engine.mu. First durable seal wins; neither an old result callback
// nor a whole-record business/progress save may replace it or other Run facts.
func (e *Engine) publishResultLocked(original *runstate.Record, data []byte, readErr error) error {
	current := e.runs[original.ID]
	if current == nil || current.Request.ResultFile == nil || !resultEndEligible(current) ||
		resultEndIdentity(current) != resultEndIdentity(original) || !reflect.DeepEqual(current.ResultFile, original.ResultFile) || current.ResultFile != nil && current.ResultFile.Sealed {
		return nil
	}
	p := current.Clone().ResultFile
	if p == nil {
		p = &runstate.ResultFile{Progress: runstate.Progress{Workspace: original.Snapshot.Workspace.Path, Binding: progressBinding(original)}}
	}
	p.Sealed = true
	p.View = contracts.ResultFileView{Contract: contracts.ResultFileContract, RunID: original.ID, Encoding: "base64", CollectionStatus: "closed"}
	if readErr != nil {
		p.View.Diagnostic = progressDiagnostic(readErr)
	} else {
		encoded := base64.StdEncoding.EncodeToString(data)
		now := time.Now().UTC()
		p.View.Version = 1
		p.View.ContentBase64 = &encoded
		p.View.SizeBytes = len(data)
		p.View.SHA256 = reliable.Digest(data)
		p.View.ObservedAt = &now
	}
	return e.saveResultLocked(original.ID, p)
}
