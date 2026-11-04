package application

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"reflect"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
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
		current.ResultFileEnd != nil || current.ResultFile != nil && current.ResultFile.Sealed {
		return errors.New("binding_invalid")
	}
	return e.saveResultLocked(original.ID, p)
}

func (e *Engine) readResult(ctx context.Context, r *runstate.Record) ([]byte, error) {
	c, err := e.runtimeFile(ctx, r, true, false)
	if err != nil {
		return nil, err
	}
	defer c.close()
	return c.read(ctx)
}

func (e *Engine) HandleResultFile(id string) contracts.Response {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id]
	if r == nil {
		return contracts.Rejected("", contracts.Fail(5, "run_not_found", "Run was not found."))
	}
	v := contracts.ResultFileView{Contract: resultContract(r), RunID: id, Encoding: "base64", CollectionStatus: "pending"}
	if r.Request.ResultFile == nil {
		v.CollectionStatus = "disabled"
	} else if r.ResultFile != nil {
		v = r.Clone().ResultFile.View
	}
	return contracts.Success("", v)
}

func structuredAudited(r *runstate.Record) bool {
	return contracts.OrcaStructuredAudited(r.Request.Execution, r.Request.Workflow) && r.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1
}

func resultContract(r *runstate.Record) string {
	if r.ResultFileContract != contracts.ResultFileSnapshotContract || r.Request.Execution.Adapter != "orca-terminal" || r.Snapshot.Ref.Backend != b.BackendOrca || r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
		return contracts.ResultFileContract
	}
	selected, target := r.Request.Workflow, r.Request.Execution.Target
	if r.Snapshot.Ref.Agent.Name == "pi" && target.Kind == "local" && target.Environment == "" && (selected.ID == "execute" && selected.Version == 1 || selected.ID == "development" && selected.Version == 2) {
		return contracts.ResultFileSnapshotContract
	}
	if r.Snapshot.Ref.Agent.Name != "codex" {
		return contracts.ResultFileContract
	}
	if structuredAudited(r) || selected.ID == "development" && selected.Version == 2 || selected.ID == "execute" && selected.Version == 1 && target.Kind == "orca_environment" && target.Environment != "" {
		return contracts.ResultFileSnapshotContract
	}
	return contracts.ResultFileContract
}

// Called inside the original business-result or stop-receipt transaction.
// The first end owns the only collection window; later facts cannot refresh it.
func markResultSnapshotEnd(r *runstate.Record, identity string) {
	if r.Request.ResultFile == nil || resultContract(r) != contracts.ResultFileSnapshotContract || identity == "" {
		return
	}
	if r.ResultFileEnd == nil && (r.ResultFile == nil || !r.ResultFile.Sealed) {
		r.ResultFileEnd = &runstate.ResultFileEnd{ID: identity, Deadline: time.Now().UTC().Add(2 * time.Second)}
	}
}

// StopStatus alone can be set by a service shutdown that leaves Run authority
// intact. Require the original whole-execution end as well. pr_ready, one turn,
// observation loss and an accepted resume are not final-result eligibility.
func resultEndEligible(r *runstate.Record) bool {
	if resultContract(r) == contracts.ResultFileSnapshotContract {
		return r.ResultFileEnd != nil && r.ResultFileEnd.ID != "" && !r.ResultFileEnd.Deadline.IsZero()
	}
	return r.StopStatus == "stopped" && r.PendingResume == "" &&
		(r.Status == "stopped" || r.Status == "completed" || r.Closed)
}

func resultEndIdentity(r *runstate.Record) string {
	if resultContract(r) == contracts.ResultFileSnapshotContract && r.ResultFileEnd != nil {
		return reliable.Digest(canonical([]any{progressBinding(r), r.ResultFileEnd}))
	}
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
	if resultContract(r) == contracts.ResultFileSnapshotContract {
		var endCancel context.CancelFunc
		ctx, endCancel = context.WithDeadline(ctx, r.ResultFileEnd.Deadline)
		defer endCancel()
	}
	var data []byte
	var err error
	data, err = boundedResultRead(ctx, func() ([]byte, error) { return e.readResult(ctx, r) })
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
	p.View = contracts.ResultFileView{Contract: resultContract(original), RunID: original.ID, Encoding: "base64", CollectionStatus: "closed"}
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
