package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"strings"
	"time"
)

const (
	observationReconnecting    = "observation_reconnecting"
	observationReconnectBudget = 2 * time.Minute
	observationCallBudget      = 30 * time.Second
)

func canReconnectObservation(r *runstate.Record) bool {
	s := r.Snapshot.Session
	return r.Request.Execution.Adapter == "orca-terminal" && r.NativeStarted &&
		s.SessionID != "" && s.CurrentTurnID != "" && s.CurrentTurnID != s.LastStoppedTurnID &&
		r.Status == "running" && r.Allowed && !r.ResumeRequired && !r.Closed &&
		r.PendingStop == "" && r.PendingResume == "" && r.StopStatus == "" && !unknown(r)
}

func (e *Engine) retryObservation(id string, original *runstate.Record, cause error, now time.Time) {
	e.mu.Lock()
	current := e.runs[id]
	if current.Revision != original.Revision || !canReconnectObservation(current) || e.ctx.Err() != nil {
		e.mu.Unlock()
		return
	}
	if b.IsAdapterError(cause, b.AdapterIdentityMismatch) {
		e.mu.Unlock()
		e.requireManual(id, cause.Error())
		return
	}
	defer e.mu.Unlock()
	next := current.Clone()
	if next.Phase != observationReconnecting || next.ObservationDeadline.IsZero() {
		next.ObservationDeadline = now.Add(observationReconnectBudget)
		next.ObservationFailures = 0
	}
	next.ObservationFailures++
	delay := min(time.Second<<min(next.ObservationFailures-1, 4), 15*time.Second)
	next.ObservationRetryAt = now.Add(delay)
	next.Phase, next.Diagnostic = observationReconnecting, e.redact(next, cause.Error())
	_ = e.commitLocked(next, nil, observationReconnecting)
}

// advance enters with the dispatch gate held.
// Publish the observer and its handle only after its facts commit successfully.
func (e *Engine) attachObservation(id string, w *worker, p b.LifecycleObservePort, r *runstate.Record) bool {
	if w.observer != nil || r.Closed {
		return false
	}
	now := time.Now()
	reconnect := canReconnectObservation(r)
	deadline := now.Add(observationCallBudget)
	if reconnect && r.Phase == observationReconnecting && !r.ObservationDeadline.IsZero() {
		if !now.Before(r.ObservationDeadline) {
			e.requireManual(id, "Original resource observation deadline exhausted: "+r.Diagnostic)
			return false
		}
		if now.Before(r.ObservationRetryAt) {
			return false
		}
		if r.ObservationDeadline.Before(deadline) {
			deadline = r.ObservationDeadline
		}
	} else if !reconnect && !r.ObservationDeadline.IsZero() {
		deadline = r.ObservationDeadline
	}
	ctx, cancel := context.WithDeadline(e.ctx, deadline)
	if reconnect {
		w.queryCancel = cancel
		w.gate.Unlock()
	}
	receipt, observer, err := p.ObserveLifecycle(ctx, r.Snapshot.Clone())
	cancel()
	if reconnect {
		w.gate.Lock()
		w.queryCancel = nil
		e.mu.Lock()
		current := e.runs[id]
		valid := current.Revision == r.Revision && canReconnectObservation(current) && e.ctx.Err() == nil
		e.mu.Unlock()
		if !valid {
			if observer != nil {
				_ = observer.Close(context.Background())
			}
			return false
		}
		state := receipt.State
		if err == nil && (state.SessionID != "" && state.SessionID != r.Snapshot.Session.SessionID ||
			state.TurnID != "" && state.TurnID != r.Snapshot.Session.CurrentTurnID) {
			err = &b.AdapterError{Kind: b.AdapterIdentityMismatch, Operation: "observe", Diagnostic: "Original session or turn identity changed"}
		}
		if err == nil && (state.SessionID == "" || state.TurnID == "" || !state.InputAccepted || !state.TurnStarted ||
			(observer == nil && receipt.TurnEvidence == nil)) {
			diagnostic := receipt.Diagnostic
			if diagnostic == "" {
				diagnostic = "Original session and turn could not be verified"
			}
			err = errors.New(diagnostic)
		}
	}
	if err == nil && receipt.Disposition == b.OperationUnknown && observer == nil && receipt.TurnEvidence == nil {
		err = errors.New("Original observation remains unverified: " + receipt.Diagnostic)
	}
	if err != nil {
		if observer != nil {
			_ = observer.Close(context.Background())
		}
		if reconnect {
			e.retryObservation(id, r, err, time.Now())
		} else if !time.Now().Before(deadline) {
			e.requireManual(id, "Original resource observation deadline exhausted: "+err.Error())
		} else if r.Status == "blocked" || r.Status == "waiting_input" || r.ResumeRequired {
			e.logger.Printf("original observation unavailable run=%s diagnostic=%s", id, e.redact(r, err.Error()))
		} else {
			e.mu.Lock()
			next := e.runs[id].Clone()
			if next.Revision == r.Revision {
				if next.ObservationDeadline.IsZero() {
					next.ObservationDeadline = deadline
				}
				next.Diagnostic = e.redact(next, err.Error())
				_ = e.commitLocked(next, nil, "observation_unverified")
			}
			e.mu.Unlock()
		}
		return false
	}
	if !e.applyObservation(id, r, receipt) {
		if observer != nil {
			_ = observer.Close(context.Background())
		}
		if reconnect {
			e.retryObservation(id, r, errors.New("Original observation could not be committed"), time.Now())
		}
		return false
	}
	if observer != nil {
		w.observer = observer
		if h := observer.StopHandle(); h != nil {
			e.retainHandle(w, h)
		}
	}
	if receipt.TurnEvidence != nil {
		return e.finishWithGate(id, *receipt.TurnEvidence, &w.gate)
	}
	return false
}

func (e *Engine) acceptEvent(id string, event b.LifecycleEvent) error {
	if event.PublicMessage != nil {
		return e.acceptMessage(id, *event.PublicMessage)
	}
	f := event.Fact
	if f.Kind != b.EventWorkObserved && f.Kind != b.EventQuestion && f.Kind != b.EventCompleted && f.Kind != b.EventStarted && f.Kind != b.EventGap && event.Observation == nil && len(event.Implementation) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if e.workspaceSuperseded(r) {
		return reliable.ErrStale
	}
	if f.SessionID != "" && f.SessionID != r.Snapshot.Session.SessionID {
		return reliable.ErrStale
	}
	if f.StartedTurnID != "" && f.StartedTurnID != r.Snapshot.Session.CurrentTurnID && f.StartedTurnID != r.Snapshot.Session.PendingTurnID {
		return reliable.ErrStale
	}
	if event.Observation != nil && (f.Kind != b.EventStatus || f.SessionID == "" || f.StartedTurnID == "" || f.Source == "" || f.SourceEventID == "") {
		return errors.New("observation status lacks its original session/turn/event binding")
	}
	if r.SeenNative == nil {
		r.SeenNative = map[string]string{}
	}
	key := r.Snapshot.Session.AttemptID + ":" + f.Source + ":" + f.SourceEventID
	digest := reliable.Digest(canonical(event.Fact))
	if event.Observation != nil {
		digest = reliable.Digest(canonical(struct {
			Fact        b.EventFact
			Observation b.ObservationStatus
		}{f, *event.Observation}))
	}
	if f.SourceEventID != "" {
		if prior, ok := r.SeenNative[key]; ok {
			if prior != digest {
				return errors.New("conflicting duplicate native event")
			}
			return nil
		}
		r.SeenNative[key] = digest
	}
	if len(event.Implementation) > 0 {
		if err := merge(r, b.LifecycleReceipt{Implementation: event.Implementation}); err != nil {
			return err
		}
	}
	if event.Observation != nil {
		e.applyObservationDiagnostic(r, event.Observation.Diagnostic)
	}
	if f.Kind == b.EventWorkObserved {
		input := workflow.Input(r.Workflow, r.Snapshot.Session.CurrentCommandID)
		if input != nil {
			if state, _ := r.Workflow.State(input.StepID); state.Kind == "report" {
				if r.ReportWork == nil {
					r.ReportWork = map[string]string{}
				}
				r.ReportWork[input.CommandID] = f.SourceEventID
			}
		}
	}
	if f.Kind == b.EventQuestion {
		if f.QuestionRoute == nil {
			return errors.New("native question lacks its verified original route")
		}
		route := *f.QuestionRoute
		if err := route.ValidateIdentity(b.QuestionRouteIdentity{ProjectID: r.Snapshot.Session.ProjectID, WorkflowRunID: id, ExecutionID: id, AttemptID: r.Snapshot.Session.AttemptID, Generation: r.Snapshot.Session.Generation, TurnID: r.Snapshot.Session.CurrentTurnID, QuestionID: route.QuestionID}); err != nil {
			return err
		}
		if r.PendingAnswer != nil && r.Question != nil && r.Question.ID != route.QuestionID {
			return errors.New("another native question cannot replace the original pending answer")
		}
		question := &contracts.QuestionView{ID: route.QuestionID, Text: f.QuestionText, AllowFreeText: f.AllowFreeText}
		for _, option := range f.QuestionOptions {
			question.Options = append(question.Options, contracts.QuestionOption{ID: option.ID, Label: option.Label, Detail: option.Detail})
		}
		for _, native := range f.Questions {
			q := contracts.QuestionView{ID: native.ID, Text: native.Question, AllowFreeText: native.AllowFreeText}
			for _, o := range native.Options {
				q.Options = append(q.Options, contracts.QuestionOption{ID: o.ID, Label: o.Label, Detail: o.Detail})
			}
			question.NativeQuestions = append(question.NativeQuestions, q)
		}
		if reflect.DeepEqual(r.Question, question) {
			// An unchanged public question can still carry a new source event or
			// implementation fact. Acknowledge those only after they are durable,
			// while retaining the original question route.
			if f.SourceEventID != "" || len(event.Implementation) > 0 {
				return e.commitLocked(r, nil, string(f.Kind))
			}
			return nil
		}
		r.Question = question
		r.Snapshot.QuestionRoute = &route
		r.Snapshot.PendingQuestions = f.Questions
		r.Status, r.Phase = "waiting_input", "question"
		if workflow.Content(r.Request.Workflow.ID) || r.DevelopmentMessage() || r.Request.Workflow.ID == workflow.SingleExecution {
			r.History = append(r.History, contracts.HistoryEntry{CommandID: r.Snapshot.Session.CurrentCommandID, SessionID: r.Snapshot.Session.SessionID, TurnID: r.Snapshot.Session.CurrentTurnID, Role: "question", Question: question})
			if r.Request.Workflow.ID == "generate" || r.Request.Workflow.ID == workflow.SingleExecution {
				r.ContentResult = &contracts.ContentResult{Status: "failed", Diagnostic: "human_input_required", Source: contracts.FinalView{CommandID: r.Snapshot.Session.CurrentCommandID, AttemptID: r.Snapshot.Session.AttemptID, SessionID: r.Snapshot.Session.SessionID, TurnID: r.Snapshot.Session.CurrentTurnID}}
				r.Diagnostic = "human_input_required"
				closeContent(r, "generate:question:"+route.QuestionID)
			}
		}
	} else if f.Kind == b.EventGap {
		r.Diagnostic = "Original observation reported a result gap"
		r.Status = "reconciling"
	} else if f.Kind == b.EventStarted && r.Snapshot.Session.PendingCommandID != "" {
		r.Snapshot.Session = reliable.RecordTurnStart(r.Snapshot.Session, f.StartedTurnID)
		r.NativeStarted = true
		if r.Phase != "observation_unverified" {
			r.Phase = "observing"
		}
	} else if len(event.Implementation) == 0 && f.SourceEventID == "" {
		return nil
	}
	return e.commitLocked(r, nil, string(f.Kind))
}

func (e *Engine) applyObservation(id string, original *runstate.Record, receipt b.LifecycleReceipt) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	next := e.runs[id].Clone()
	if reliable.ValidateResultBinding(next.Snapshot, original.Snapshot) != nil {
		return false
	}
	if merge(next, receipt) != nil {
		return false
	}
	next.ObservationDeadline = time.Time{}
	next.ObservationRetryAt = time.Time{}
	next.ObservationFailures = 0
	if original.Phase == observationReconnecting {
		next.Diagnostic = ""
	}
	e.applyObservationDiagnostic(next, receipt.Diagnostic)
	if reflect.DeepEqual(next, e.runs[id]) {
		return true
	}
	return e.commitLocked(next, nil, "observation_attached") == nil
}

const observationUnverified = "observation_unverified"

func (e *Engine) applyObservationDiagnostic(r *runstate.Record, diagnostic string) {
	if r.Status != "running" {
		return
	}
	if diagnostic != "" && (r.Phase == "observing" || r.Phase == "input_accepted_waiting_start" || r.Phase == "observation_unverified") {
		r.Phase, r.Diagnostic = "observation_unverified", e.redact(r, diagnostic)
	} else if diagnostic == "" && r.Phase == "observation_unverified" {
		r.Phase, r.Diagnostic = "observing", ""
		if !r.NativeStarted {
			r.Phase = "input_accepted_waiting_start"
		}
	}
}

// Validate the complete native question set before persisting any input intent.
func nativeAnswers(questions []b.Question, text string) (map[string][]string, error) {
	answers := map[string][]string{}
	if len(questions) == 0 {
		return nil, errors.New("Original native questions are missing")
	}
	if len(questions) == 1 {
		answers[questions[0].ID] = []string{text}
	} else if contracts.DecodeStrict([]byte(text), &answers) != nil {
		return nil, errors.New("Multiple questions require unambiguous JSON keyed by original question ID")
	}
	if len(answers) != len(questions) {
		return nil, errors.New("Answer must cover exactly the original native questions")
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if q.ID == "" || seen[q.ID] {
			return nil, errors.New("Original native question identity is ambiguous")
		}
		seen[q.ID] = true
		values, ok := answers[q.ID]
		if !ok || len(values) == 0 || len(values) > 2 || len(values) > 1 && !q.AllowFreeText {
			return nil, errors.New("Answer differs from the original native question")
		}
		for i, value := range values {
			if strings.TrimSpace(value) == "" {
				return nil, errors.New("Native answer cannot be empty")
			}
			offered := false
			for _, option := range q.Options {
				if value == option.ID || value == option.Label {
					values[i] = option.Label
					offered = true
					break
				}
			}
			if !offered && !q.AllowFreeText {
				return nil, errors.New("Answer is not an offered option")
			}
		}
	}
	return answers, nil
}
