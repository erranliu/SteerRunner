package application

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"reflect"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// This finite byte channel owns no business authority, native session or queue.
type runtimeFileChannel struct {
	state      *runstate.Progress
	initialize func(context.Context, []byte) error
	read       func(context.Context) ([]byte, error)
	close      func() error
}

type runtimeFileSample struct {
	state *runstate.Progress
	data  []byte
	err   error
}

func fileState(r *runstate.Record, result bool) *runstate.Progress {
	p := r.Progress
	if result {
		p = nil
		if r.ResultFile != nil {
			p = &r.ResultFile.Progress
		}
	}
	if p != nil {
		copy := *p
		copy.Directories = map[string]string{}
		for k, v := range p.Directories {
			copy.Directories[k] = v
		}
		return &copy
	}
	contract, status := contracts.ProgressContract, "initializing"
	if result {
		contract, status = resultContract(r), "pending"
	}
	return &runstate.Progress{Workspace: r.Snapshot.Workspace.Path, Binding: progressBinding(r), View: contracts.ProgressFileView{Contract: contract, RunID: r.ID, Encoding: "base64", CollectionStatus: status}}
}

func filesNeedPreparation(r *runstate.Record) bool {
	return r.Request.ProgressFile != nil && (r.Progress == nil || !r.Progress.InitializationAttempted && !r.Progress.Initialized) ||
		r.Request.ResultFile != nil && (r.ResultFile == nil || !r.ResultFile.InitializationAttempted && !r.ResultFile.Initialized && !r.ResultFile.Sealed)
}

func fileInputDispatched(r *runstate.Record) bool {
	if r.NativeStarted || r.Snapshot.Session.CurrentTurnID != "" {
		return true
	}
	for _, intent := range r.Intents {
		if intent.Request.Operation.Action == "start_turn" || intent.Request.Operation.Action == "submit_input" {
			return true
		}
	}
	return false
}

func requiresResultInitialization(r *runstate.Record) bool {
	return r.Request.ResultFile != nil && r.Request.ResultFile.RequireInitialization
}

func resultInitializationDiagnostic(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "initialization_timeout"
	case err != nil && err.Error() == "initialization_seed_mismatch":
		return err.Error()
	default:
		return progressDiagnostic(err)
	}
}

// Claim both file preparations before I/O, under the same gate as stop.
// A failed metadata save consumes this service's attempt but permits no write.
func (e *Engine) prepareRuntimeFiles(original *runstate.Record) {
	e.mu.Lock()
	w := e.workers[original.ID]
	e.mu.Unlock()
	if w == nil {
		return
	}
	w.gate.Lock()
	e.mu.Lock()
	r := e.runs[original.ID]
	if w.filesPreparationStarted || !r.Allowed || r.ResumeRequired || r.PendingStop != "" || progressTerminal(r) || r.Snapshot.Workspace.Path == "" || e.ctx.Err() != nil || progressBinding(r) != progressBinding(original) {
		e.mu.Unlock()
		w.gate.Unlock()
		return
	}
	w.filesPreparationStarted = true
	next := r.Clone()
	channels := []bool{}
	if r.Request.ProgressFile != nil && (r.Progress == nil || !r.Progress.InitializationAttempted && !r.Progress.Initialized) {
		p := fileState(r, false)
		p.InitializationAttempted = true
		p.InitializationWriteStatus = "not_started"
		next.Progress = p
		channels = append(channels, false)
	}
	if r.Request.ResultFile != nil && (r.ResultFile == nil || !r.ResultFile.InitializationAttempted && !r.ResultFile.Initialized && !r.ResultFile.Sealed) {
		p := fileState(r, true)
		p.InitializationAttempted = true
		p.InitializationWriteStatus = "not_started"
		next.ResultFile = &runstate.ResultFile{Progress: *p}
		channels = append(channels, true)
	}
	if len(channels) == 0 {
		e.mu.Unlock()
		w.gate.Unlock()
		return
	}
	err := e.store.SaveProgress(next)
	if err == nil {
		e.runs[r.ID] = next
	}
	// Existing v6 Runs may have no attempt marker after a failed file save.
	// Once business was dispatched only read the original path, never seed it.
	write := err == nil && !fileInputDispatched(r)
	e.mu.Unlock()
	w.gate.Unlock()
	if !write {
		return
	}
	for _, result := range channels {
		e.mu.Lock()
		current := e.runs[original.ID]
		allowed := current.Allowed && !current.ResumeRequired && current.PendingStop == "" && !progressTerminal(current) && progressBinding(current) == progressBinding(original)
		e.mu.Unlock()
		if !allowed {
			break
		}
		var ctx context.Context
		var cancel context.CancelFunc
		if result && requiresResultInitialization(next) {
			ensurePreparationDeadline(next, time.Now().UTC())
			ctx, cancel = context.WithDeadline(e.ctx, next.PreparationDeadline)
		} else {
			ctx, cancel = context.WithTimeout(e.ctx, 4*time.Second)
		}
		sample := e.sampleRuntimeFile(ctx, next.Clone(), result, true)
		cancel()
		e.mu.Lock()
		e.publishFilePreparationLocked(next, sample, result)
		e.mu.Unlock()
	}
}

func (e *Engine) publishFilePreparationLocked(original *runstate.Record, sample runtimeFileSample, result bool) {
	r := e.runs[original.ID]
	if r == nil || e.ctx.Err() != nil || r.PendingStop != "" || !r.Allowed || r.ResumeRequired || progressTerminal(r) || progressBinding(r) != progressBinding(original) || result && r.ResultFile != nil && r.ResultFile.Sealed {
		return
	}
	p := fileState(r, result)
	if sample.state != nil {
		s := sample.state
		if p.HostID != "" && p.HostID != s.HostID || p.WorkspaceInstance != "" && p.WorkspaceInstance != s.WorkspaceInstance || len(p.Directories) != 0 && !reflect.DeepEqual(p.Directories, s.Directories) {
			return
		}
		p.HostID, p.WorkspaceInstance, p.Directories = s.HostID, s.WorkspaceInstance, s.Directories
		p.Initialized = p.Initialized || s.Initialized
	}
	if p.View.Version == 0 {
		p.View.Diagnostic = progressDiagnostic(sample.err)
		if result && requiresResultInitialization(r) {
			p.View.Diagnostic = resultInitializationDiagnostic(sample.err)
		}
		if !result {
			p.View.CollectionStatus = "observing"
			if sample.err != nil {
				p.View.CollectionStatus = "unavailable"
			}
		}
	}
	var err error
	if result {
		err = e.saveResultPreparationLocked(original, &runstate.ResultFile{Progress: *p})
	} else {
		err = e.saveProgressLocked(r, p)
	}
	if err != nil && e.logger != nil {
		e.logger.Printf("optional file preparation save unavailable run=%s result=%t", r.ID, result)
	}
}

// Preserve durable write settlement when an earlier read/preparation snapshot
// arrives after the physical writer exited. File observation has its own revision.
func preserveInitializationSettlement(next, prior *runstate.Progress) {
	if next == nil || prior == nil {
		return
	}
	rank := map[string]int{"": 0, "not_started": 1, "pending": 2, "settled": 3}
	if rank[prior.InitializationWriteStatus] > rank[next.InitializationWriteStatus] {
		next.InitializationWriteStatus = prior.InitializationWriteStatus
	}
	next.InitializationAttempted = next.InitializationAttempted || prior.InitializationAttempted
}

// Persist the obligation before any directory/exclude/seed mutation. Stop keeps
// its existing deadline; only transfer waits for physical write settlement.
func (e *Engine) beginFilePreparation(r *runstate.Record, result bool) (*worker, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current, w := e.runs[r.ID], e.workers[r.ID]
	if current == nil || w == nil || !current.Allowed || current.ResumeRequired || current.PendingStop != "" || progressTerminal(current) || e.ctx.Err() != nil || progressBinding(current) != progressBinding(r) || e.workspaceSuperseded(current) {
		return nil, errors.New("binding_invalid")
	}
	p := fileState(current, result)
	if p.InitializationWriteStatus != "not_started" {
		return nil, errors.New("initialization_unverified")
	}
	p.InitializationWriteStatus = "pending"
	var err error
	if result {
		sealed := current.ResultFile != nil && current.ResultFile.Sealed
		if sealed {
			return nil, errors.New("binding_invalid")
		}
		err = e.saveResultLocked(r.ID, &runstate.ResultFile{Progress: *p})
	} else {
		err = e.saveProgressLocked(current, p)
	}
	if err != nil {
		return nil, err
	}
	w.filePreparationsActive++
	return w, nil
}

func (e *Engine) finishFilePreparation(r *runstate.Record, w *worker, result, settled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() { w.filePreparationsActive-- }()
	current := e.runs[r.ID]
	if !settled || current == nil || progressBinding(current) != progressBinding(r) || e.workspaceSuperseded(current) {
		return
	}
	p := fileState(current, result)
	p.InitializationWriteStatus = "settled"
	// Settlement remains necessary after stop/sealing; never publish late bytes.
	var err error
	if result {
		next := *current.ResultFile
		next.Progress = *p
		err = e.saveResultLocked(r.ID, &next)
	} else {
		err = e.saveProgressLocked(current, p)
	}
	if err != nil && e.logger != nil {
		e.logger.Printf("optional file write settlement unavailable run=%s result=%t", r.ID, result)
	}
}

// A non-cooperating filesystem/transport cannot extend the channel deadline.
// Late bytes are ignored, but physical write obligations survive the deadline.
func (e *Engine) sampleRuntimeFile(ctx context.Context, r *runstate.Record, result, prepare bool) runtimeFileSample {
	if err := ctx.Err(); err != nil {
		return runtimeFileSample{err: err}
	}
	var preparer *worker
	if prepare {
		var err error
		preparer, err = e.beginFilePreparation(r, result)
		if err != nil {
			return runtimeFileSample{err: err}
		}
	}
	var reader *worker
	if !result && !prepare {
		e.mu.Lock()
		reader = e.workers[r.ID]
		if reader != nil && reader.progressReadActive {
			e.mu.Unlock()
			return runtimeFileSample{err: errors.New("read_failed")}
		}
		if reader != nil {
			reader.progressReadActive = true
		}
		e.mu.Unlock()
	}
	done := make(chan runtimeFileSample, 1)
	go func() {
		settled := true
		required := prepare && result && requiresResultInitialization(r)
		var verification runtimeFileSample
		if required {
			// Publish the verdict only after the original channel is closed
			// and the writer's settlement has been persisted.
			defer func() { done <- verification }()
		}
		if preparer != nil {
			defer func() { e.finishFilePreparation(r, preparer, result, settled) }()
		}
		if reader != nil {
			defer func() { e.mu.Lock(); reader.progressReadActive = false; e.mu.Unlock() }()
		}
		c, err := e.runtimeFile(ctx, r, result, prepare)
		if err != nil {
			if required {
				verification.err = err
			} else {
				done <- runtimeFileSample{err: err}
			}
			return
		}
		defer c.close()
		if prepare {
			var seed string
			if result {
				seed = r.Request.ResultFile.InitialContentBase64
			} else {
				seed = r.Request.ProgressFile.InitialContentBase64
			}
			data, _ := base64.StdEncoding.DecodeString(seed)
			if err = ctx.Err(); err == nil {
				// A lost remote reply cannot prove the remote write has exited.
				settled = r.Request.Execution.Adapter != "orca-terminal"
				err = c.initialize(ctx, data)
				settled = settled || err == nil
			}
			if required {
				verification = runtimeFileSample{state: c.state, err: err}
				if verification.err == nil {
					var actual []byte
					actual, verification.err = c.read(ctx)
					if verification.err == nil && reliable.Digest(actual) != reliable.Digest(data) {
						verification.err = errors.New("initialization_seed_mismatch")
					}
				}
				c.state.Initialized = verification.err == nil
				return
			}
			if result && err == nil {
				c.state.Initialized = true
				done <- runtimeFileSample{state: c.state}
				return
			}
		}
		// A lost write reply or changed seed is not a business gate. Actual stable
		// bytes from the original path can establish availability independently.
		var data []byte
		readErr := ctx.Err()
		if readErr == nil {
			data, readErr = c.read(ctx)
		}
		if readErr == nil {
			c.state.Initialized = true
			err = nil
		} else if err == nil {
			err = readErr
		}
		done <- runtimeFileSample{state: c.state, data: data, err: err}
	}()
	select {
	case <-ctx.Done():
		return runtimeFileSample{err: ctx.Err()}
	case sample := <-done:
		if err := ctx.Err(); err != nil {
			return runtimeFileSample{err: err}
		}
		return sample
	}
}

func (e *Engine) runtimeFile(ctx context.Context, r *runstate.Record, result, prepare bool) (*runtimeFileChannel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.openRuntimeFile != nil {
		return e.openRuntimeFile(ctx, r, result, prepare)
	}
	p := fileState(r, result)
	if p.Workspace == "" || p.Workspace != r.Snapshot.Workspace.Path || p.Binding != progressBinding(r) {
		return nil, errors.New("binding_invalid")
	}
	if r.Request.Execution.Adapter == "orca-terminal" {
		if p.HostID == "" {
			host, err := orca.ProgressHost(progressSetup(r))
			if err != nil {
				return nil, errors.New("binding_invalid")
			}
			p.HostID = host
		}
		open := orca.ProgressSource
		if result {
			open = orca.ResultSource
		}
		source, err := open(ctx, progressSetup(r), p.HostID, p.WorkspaceInstance)
		if err != nil {
			return nil, err
		}
		p.WorkspaceInstance = source.Instance
		read := func(ctx context.Context) ([]byte, error) {
			first, err := source.Read(ctx)
			if err != nil {
				return nil, err
			}
			second, err := source.Read(ctx)
			if err == nil && !bytes.Equal(first, second) {
				err = errors.New("file_changing")
			}
			return second, err
		}
		return &runtimeFileChannel{state: p, initialize: source.Initialize, read: read, close: source.Close}, nil
	}
	if p.HostID != "" && p.HostID != "local" {
		return nil, errors.New("binding_invalid")
	}
	if prepare || len(p.Directories) == 0 {
		e.mu.Lock()
		w := e.workers[r.ID]
		fresh := prepare && w != nil && w.freshWorkspace
		e.mu.Unlock()
		if err := verifyOriginalWorkspace(ctx, gitHost(), r, true, fresh); err != nil {
			return nil, errors.New("binding_invalid")
		}
		directories := workspace.ProgressDirectories
		if result {
			directories = workspace.ResultDirectories
		}
		dirs, err := directories(p.Workspace, r.ID, prepare)
		if err != nil {
			return nil, err
		}
		if len(p.Directories) != 0 && !reflect.DeepEqual(p.Directories, dirs) {
			return nil, errors.New("binding_invalid")
		}
		p.Directories = dirs
		if prepare {
			out, err := gitHost().Output(ctx, p.Workspace, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
			if err != nil {
				return nil, errors.New("directory_unavailable")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			f, err := os.OpenFile(string(bytes.TrimSpace(out)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return nil, errors.New("directory_unavailable")
			}
			kind := "progress"
			if result {
				kind = "result"
			}
			_, err = f.WriteString("\n/.steerrunner/" + kind + "/\n")
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				return nil, errors.New("directory_unavailable")
			}
		}
	}
	p.HostID = "local"
	initialize, read := workspace.InitializeProgress, workspace.ReadProgress
	if result {
		initialize, read = workspace.InitializeResult, workspace.ReadResult
	}
	return &runtimeFileChannel{state: p, initialize: func(_ context.Context, raw []byte) error { return initialize(p.Workspace, r.ID, p.Directories, raw) }, read: func(_ context.Context) ([]byte, error) { return read(p.Workspace, r.ID, p.Directories) }, close: func() error { return nil }}, nil
}
