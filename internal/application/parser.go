package application

import (
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	parser "github.com/erranliu/SteerRunner/internal/execution/resultacquisition/codexexec"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func (e *Engine) selectParser(r *runstate.Record, input workflow.StageInput, state workflow.State, transition *workflow.Transition) {
	source := workflow.Result(r.Workflow, input.CommandID)
	if source == nil || len(source.Final) > 65536 {
		return
	}
	for _, intent := range r.Intents {
		if intent.Request.Input.Parse != nil && intent.Request.Input.Parse.ResultID == transition.Acquired.ID {
			return
		}
	}
	parseState, ok := r.Workflow.State(state.Parse)
	if !ok || parseState.Kind != "parse" {
		return
	}
	pack := parser.FreezePackage(parser.Package{Config: r.AcquisitionConfig, Source: workflow.Source(*source), ImplementationResultID: input.ImplementationResultID, Final: source.Final, Instruction: parseState.Prompt})
	if err := m.CheckInput(pack.Prompt, pack.Budget); err != nil {
		transition.Reason = err.Error()
		transition.Acquired.Missing = append(transition.Acquired.Missing, "parser candidate not sent: "+err.Error())
		return // keep the original, already-authorized one-report fallback
	}
	pack.ReportToolPolicy = transition.Next.ReportToolPolicy
	payload, err := json.Marshal(pack)
	if err != nil || len(payload) > r.AcquisitionConfig.Config.CodexParser.MaxInputBytes {
		return
	}
	at := time.Now().UTC()
	p := &b.ParseInput{ResultID: transition.Acquired.ID, Payload: string(payload), Generation: contracts.NewID(), Deadline: at.Add(time.Duration(r.AcquisitionConfig.Config.CodexParser.TimeoutSeconds) * time.Second), Owner: e.identity}
	if owner := parser.RetainedOwner(r.Snapshot); owner != nil {
		p.Owner = owner
	}
	intent, err := operation.NewParser(r.ID, r.Snapshot, *p, contracts.NewID(), contracts.NewID(), at)
	if err != nil {
		transition.Reason = "parser intent could not be frozen"
		return
	}
	r.Intents = append(r.Intents, intent)
	r.Snapshot.Operations = append(r.Snapshot.Operations, intent.Request.Operation)
	transition.Acquired.Attempts = append(transition.Acquired.Attempts, ra.Attempt{Method: "llm_parser", Source: pack.Source, OperationID: intent.Request.Operation.ID, Status: "pending"})
	transition.Next = nil
	transition.Cursor = state.Parse
	transition.Status = "running"
}

func parserUnsettled(intent operation.Intent) bool {
	return intent.ParserUnsettled()
}
func parserResourcesSettled(r *runstate.Record) bool {
	for _, intent := range r.Intents {
		if parserUnsettled(intent) {
			return false
		}
	}
	return true
}
func parserIndex(r *runstate.Record) int {
	for i := len(r.Intents) - 1; i >= 0; i-- {
		intent := r.Intents[i]
		if intent.Request.Input.Parse == nil {
			continue
		}
		if parserUnsettled(intent) {
			return i
		}
		if a := workflow.Acquired(r.Workflow, intent.Request.Input.Parse.ResultID); a != nil && a.Status == "insufficient" && currentParser(r, intent) {
			return i
		}
	}
	return -1
}

func currentParser(r *runstate.Record, intent operation.Intent) bool {
	state, _ := r.Workflow.CurrentState()
	if state.Kind != "parse" || intent.Request.Input.Parse == nil {
		return false
	}
	a := workflow.Acquired(r.Workflow, intent.Request.Input.Parse.ResultID)
	if a == nil {
		return false
	}
	input := workflow.Input(r.Workflow, a.Source.CommandID)
	if input == nil || input.ImplementationResultID != r.Workflow.ImplementationResultID {
		return false
	}
	original, _ := r.Workflow.State(input.StepID)
	return original.Parse == r.Workflow.StepID
}

func validateParserReference(r *runstate.Record) error {
	count := 0
	for _, intent := range r.Intents {
		p := intent.Request.Input.Parse
		if p == nil {
			continue
		}
		a := workflow.Acquired(r.Workflow, p.ResultID)
		if a == nil {
			continue
		}
		input := workflow.Input(r.Workflow, a.Source.CommandID)
		source := workflow.Result(r.Workflow, a.Source.CommandID)
		if input == nil || source == nil {
			continue
		}
		if input.ImplementationResultID != r.Workflow.ImplementationResultID {
			continue
		}
		state, _ := r.Workflow.State(input.StepID)
		if state.Parse != r.Workflow.StepID {
			continue
		}
		var pack parser.Package
		if intent.Request.Validate() != nil || json.Unmarshal([]byte(p.Payload), &pack) != nil || pack.Source != a.Source || workflow.Source(*source) != a.Source || pack.ImplementationResultID != input.ImplementationResultID {
			return errors.New("parser cursor/source/frozen operation mismatch")
		}
		count++
	}
	if count != 1 || r.NextStage != nil || r.Snapshot.PendingTurn != nil {
		return errors.New("parser cursor requires exactly one original operation and no native prompt")
	}
	return nil
}

// advanceParser runs under the original worker gate. Slow source verification
// releases that gate and rechecks the original revision before dispatch/accept.
func (e *Engine) advanceParser(id string, w *worker, r *runstate.Record) (bool, bool) {
	index := parserIndex(r)
	if index < 0 {
		return false, false
	}
	intent := r.Intents[index]
	if w.parserTask != nil {
		if r.PendingStop != "" || !r.Allowed || r.ResumeRequired {
			w.parserTask.Cancel()
		}
		if receipt, done := w.parserTask.Result(); done {
			accepted := e.acceptParserReceipt(id, w, index, receipt)
			if accepted {
				w.parserTask = nil
			}
			return true, accepted
		}
		return true, false
	}
	if intent.Receipt != nil && intent.Receipt.State.Stopped && intent.Receipt.ParseEvidence != nil {
		if state, _ := r.Workflow.CurrentState(); state.Kind != "parse" || !r.Allowed || r.ResumeRequired || r.PendingStop != "" {
			return false, false
		}
		return true, e.acceptParserReceipt(id, w, index, *intent.Receipt)
	}
	if intent.Request.Operation.Status != b.OperationPending {

		if parser.Retained(intent.Request) && r.PendingStop != "" {
			if w.port == nil {
				_, _ = e.port(r, w)
			}
			if port, ok := w.port.(b.ParserOwnerPort); ok {
				w.gate.Unlock()
				_, _ = port.CancelParser(e.ctx, intent.Request)
				w.gate.Lock()
			}
		}
		resources := &parser.Resources{Root: e.dir, Prior: intent.Receipt}
		result := operation.InvokeParser(e.ctx, resources, resources, intent.Request, operation.ReconcileOriginalIntent)
		if result.Err == nil {
			receipt := result.Receipt
			if resources.RootObserved && intent.Receipt == nil {
				e.mu.Lock()
				if current := e.runs[id]; current.Revision == r.Revision {
					next := current.Clone()
					next.Intents[index].Settle(next.Snapshot.Operations, receipt, operation.ReconcileOriginalIntent, b.OperationAccepted, "", time.Now().UTC())
					next.Phase = "parser_running"
					_ = e.commitLocked(next, nil, "parser_root_bound")
				}
				e.mu.Unlock()
				return true, true
			}
			if receipt.ParseEvidence != nil {
				return true, e.acceptParserReceipt(id, w, index, receipt)
			}
		}
		return true, false
	}
	if r.PendingStop != "" {
		receipt := b.LifecycleReceipt{Operation: intent.Request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true, WorkerAbsent: true}, ParseEvidence: &b.ParseEvidence{Status: "failed", Diagnostic: "unsent parser cancelled"}}
		return true, e.acceptParserReceipt(id, w, index, receipt)
	}
	if !r.Allowed || r.ResumeRequired || r.PendingResume != "" {
		return false, false
	}
	if !time.Now().Before(intent.Request.Input.Parse.Deadline) {
		receipt := b.LifecycleReceipt{Operation: intent.Request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true, WorkerAbsent: true}, ParseEvidence: &b.ParseEvidence{Status: "failed", Diagnostic: "parser deadline expired while waiting for capacity"}}
		return true, e.acceptParserReceipt(id, w, index, receipt)
	}
	w.gate.Unlock()
	sourceErr := e.checkParserSource(r, intent)
	w.gate.Lock()
	if sourceErr != nil {
		e.blockRevision(id, r.Revision, "source_unverified", sourceErr, false)
		return true, false
	}
	e.mu.Lock()
	current := e.runs[id]
	if current.Revision != r.Revision || current.PendingStop != "" || !current.Allowed || current.ResumeRequired {
		e.mu.Unlock()
		return true, false
	}
	for _, other := range e.runs {
		for _, op := range other.Intents {
			if parserUnsettled(op) {
				e.mu.Unlock()
				return true, false
			}
		}
	}
	next := current.Clone()
	if err := next.Intents[index].MarkParserDispatch(next.Snapshot.Operations); err != nil {
		e.mu.Unlock()
		return true, false
	}
	if a := workflow.Acquired(next.Workflow, intent.Request.Input.Parse.ResultID); a != nil {
		for j := range a.Attempts {
			if a.Attempts[j].OperationID == intent.Request.Operation.ID {
				a.Attempts[j].Status = "running"
			}
		}
	}
	next.Phase = "parser_starting"
	if err := e.commitLocked(next, nil, "parser_dispatch_committed"); err != nil {
		e.mu.Unlock()
		return true, false
	}
	request := next.Intents[index].Request.Clone()
	e.mu.Unlock()
	resources := &parser.Resources{Root: e.dir, Started: func(receipt b.LifecycleReceipt) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		current := e.runs[id]
		if e.ctx.Err() != nil || !current.Allowed || current.ResumeRequired || current.PendingStop != "" || current.Intents[index].Request.Operation.ID != request.Operation.ID {
			return errors.New("parser dispatch authorization revoked")
		}
		next := current.Clone()
		next.Intents[index].Settle(next.Snapshot.Operations, receipt, operation.ExecuteNewIntent, b.OperationAccepted, "", time.Now().UTC())
		next.Phase = "parser_running"
		return e.commitLocked(next, nil, "parser_root_bound")
	}}
	if parser.Retained(request) {
		resources.Owner, _ = w.port.(b.ParserOwnerPort)
		w.gate.Unlock()
		_ = operation.InvokeParser(e.ctx, resources, resources, request, operation.ExecuteNewIntent)
		w.gate.Lock()
	} else {
		_ = operation.InvokeParser(e.ctx, resources, resources, request, operation.ExecuteNewIntent)
		w.parserTask = resources.Task
	}
	return true, false
}

func (e *Engine) checkParserSource(r *runstate.Record, intent operation.Intent) error {
	if err := validateWorkflow(r); err != nil {
		return err
	}
	var pack parser.Package
	if json.Unmarshal([]byte(intent.Request.Input.Parse.Payload), &pack) != nil {
		return errors.New("parser frozen source package invalid")
	}
	source := workflow.Result(r.Workflow, pack.Source.CommandID)
	input := workflow.Input(r.Workflow, pack.Source.CommandID)
	acquired := workflow.Acquired(r.Workflow, intent.Request.Input.Parse.ResultID)
	if source == nil || input == nil || acquired == nil || acquired.Source != pack.Source || workflow.Source(*source) != pack.Source || pack.ImplementationResultID != r.Workflow.ImplementationResultID {
		return errors.New("parser original source changed")
	}
	state, _ := r.Workflow.State(input.StepID)
	if err := workflow.VerifyPrerequisites(r.Workflow, *input, state); err != nil {
		return err
	}
	if source.Workspace != nil && source.Workspace.Qualified && !matchesCheckpoint(e.inspectSource(r, ""), source.Workspace) {
		return errors.New("parser workspace differs from original audit checkpoint")
	}
	return nil
}

func (e *Engine) acceptParserReceipt(id string, w *worker, index int, receipt b.LifecycleReceipt) bool {
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	intent := r.Intents[index]
	if receipt.ValidateFor(intent.Request) != nil || receipt.ParseEvidence == nil {
		return false
	}
	currentSource := currentParser(r, intent)
	eligible := currentSource && r.Allowed && !r.ResumeRequired && r.PendingStop == "" && r.StopStatus == "" && e.ctx.Err() == nil
	priorFinal, same := intent.ParserReceipt(receipt)
	if priorFinal {
		if !same {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.runs[id].Revision != r.Revision {
				return false
			}
			next := r.Clone()
			if a := workflow.Acquired(next.Workflow, intent.Request.Input.Parse.ResultID); a != nil {
				a.Status = "conflict"
				a.Conflicts = append(a.Conflicts, "conflicting original parser receipt")
				if currentSource || a.ID == next.Workflow.AuditResultID {
					next.Status, next.Phase, next.Diagnostic = "blocked", "parser_receipt_conflict", "conflicting original parser receipt"
					next.Allowed = false
					next.ResumeRequired = true
				}
			}
			return e.commitLocked(next, nil, "parser_receipt_conflict") == nil
		}
		if !eligible {
			return true
		}
	}
	var sourceErr error
	if eligible {
		w.gate.Unlock()
		sourceErr = e.checkParserSource(r, intent)
		w.gate.Lock()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.runs[id].Revision != r.Revision {
		return false
	}
	next := r.Clone()
	status := b.OperationConfirmed
	if !receipt.State.Stopped || !receipt.State.WritersAbsent {
		status = b.OperationUnknown
		eligible = false
		next.Status, next.Phase, next.Diagnostic = "reconciling", "parser_resource_unverified", receipt.ParseEvidence.Diagnostic
	}
	receipt.ParseEvidence.Diagnostic = e.redact(next, receipt.ParseEvidence.Diagnostic)
	if !priorFinal {
		next.Intents[index].Settle(next.Snapshot.Operations, receipt, operation.ReconcileOriginalIntent, status, receipt.ParseEvidence.Diagnostic, time.Now().UTC())
	}
	acquired := workflow.Acquired(next.Workflow, intent.Request.Input.Parse.ResultID)
	if acquired != nil {
		for i := range acquired.Attempts {
			if acquired.Attempts[i].OperationID == intent.Request.Operation.ID {
				acquired.Attempts[i].Status = receipt.ParseEvidence.Status
				acquired.Attempts[i].Diagnostic = receipt.ParseEvidence.Diagnostic
			}
		}
	}
	if sourceErr != nil {
		invalidateWorkspaceEligibility(next)
		if acquired != nil {
			acquired.Status = "unverified"
			acquired.Conflicts = append(acquired.Conflicts, sourceErr.Error())
		}
		next.Status, next.Phase, next.Diagnostic = "blocked", "source_unverified", sourceErr.Error()
		next.Allowed = false
		next.ResumeRequired = true
	} else if receipt.ParseEvidence.Status == "side_effect_unverified" {
		if acquired != nil {
			acquired.Status = "unverified"
			acquired.Conflicts = append(acquired.Conflicts, "parser tool side effects unverified")
		}
		next.Allowed = false
		next.ResumeRequired = true
		if next.PendingStop == "" && next.StopStatus == "" {
			next.Status, next.Phase, next.Diagnostic = "blocked", "parser_side_effect_unverified", "parser tool side effects unverified"
		}
	} else if eligible && acquired != nil {
		source := workflow.Input(next.Workflow, acquired.Source.CommandID)
		if source != nil && next.Workflow.StepID != "" {
			parseState, _ := next.Workflow.CurrentState()
			if parseState.Kind == "parse" {
				state, _ := next.Workflow.State(source.StepID)
				normalized := "invalid independent parser result"
				if receipt.ParseEvidence.Status == "completed" || receipt.ParseEvidence.Status == "conflict" {
					if value, err := ra.NormalizeParserReport(receipt.ParseEvidence.Content); err == nil || errors.Is(err, ra.ErrConflict) {
						normalized = value
					}
				}
				var pack parser.Package
				_ = json.Unmarshal([]byte(intent.Request.Input.Parse.Payload), &pack)
				policy := pack.ReportToolPolicy
				transition := workflow.RouteParsed(next.Workflow, *source, normalized, intent.Request.Operation.ID, policy)
				e.applyWorkflowTransition(next, *source, state, transition)
			}
		}
	} else if currentSource && status == b.OperationConfirmed && next.PendingStop == "" && next.Status != "blocked" && next.Status != "waiting_input" {
		next.Phase = "parser_result_requires_resume"
	}
	// Complete native evidence can be committed after stop/revocation but never
	// authorizes a successor there. The original final/session remain untouched.
	if reflect.DeepEqual(next, r) {
		return true
	}
	return e.commitLocked(next, nil, "parser_receipt_committed") == nil
}

func (e *Engine) stopParserForShutdown(id string, w *worker) {
	if w.parserTask == nil {
		return
	}
	w.parserTask.Cancel()
	select {
	case <-w.parserTask.Done():
	case <-time.After(10 * time.Second):
		return
	}
	w.gate.Lock()
	defer w.gate.Unlock()
	e.mu.Lock()
	r := e.runs[id].Clone()
	e.mu.Unlock()
	index := parserIndex(r)
	if receipt, ok := w.parserTask.Result(); ok && index >= 0 {
		if e.acceptParserReceipt(id, w, index, receipt) {
			w.parserTask = nil
		}
	}
}
