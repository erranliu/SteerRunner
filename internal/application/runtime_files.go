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
		contract, status = contracts.ResultFileContract, "pending"
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

// Claim both optional preparations before I/O, under the same gate as stop.
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
		next.Progress = p
		channels = append(channels, false)
	}
	if r.Request.ResultFile != nil && (r.ResultFile == nil || !r.ResultFile.InitializationAttempted && !r.ResultFile.Initialized && !r.ResultFile.Sealed) {
		p := fileState(r, true)
		p.InitializationAttempted = true
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
		ctx, cancel := context.WithTimeout(e.ctx, 4*time.Second)
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

// A non-cooperating filesystem/transport cannot extend the channel deadline.
// The goroutine only owns private values and its channel; late results are ignored.
func (e *Engine) sampleRuntimeFile(ctx context.Context, r *runstate.Record, result, prepare bool) runtimeFileSample {
	if err := ctx.Err(); err != nil {
		return runtimeFileSample{err: err}
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
		if reader != nil {
			defer func() { e.mu.Lock(); reader.progressReadActive = false; e.mu.Unlock() }()
		}
		c, err := e.runtimeFile(ctx, r, result, prepare)
		if err != nil {
			done <- runtimeFileSample{err: err}
			return
		}
		defer c.close()
		if prepare {
			seed := r.Request.ProgressFile
			if result {
				seed = r.Request.ResultFile
			}
			data, _ := base64.StdEncoding.DecodeString(seed.InitialContentBase64)
			if err = ctx.Err(); err == nil {
				err = c.initialize(ctx, data)
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
