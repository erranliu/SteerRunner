package application

import (
	"context"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
	"strings"
	"time"
)

// advance holds the dispatch gate while attaching the original observation.
// Publish the observer and its handle only after its facts commit successfully.
func (e *Engine) attachObservation(id string, w *worker, p b.LifecycleObservePort, r *runstate.Record) bool {
	if w.observer != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	receipt, observer, err := p.ObserveLifecycle(ctx, r.Snapshot.Clone())
	cancel()
	if err != nil {
		if observer != nil {
			_ = observer.Close(context.Background())
		}
		e.block(id, "observation_unverified", err, true)
		return false
	}
	if !e.applyObservation(id, r, receipt) {
		if observer != nil {
			_ = observer.Close(context.Background())
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
		return e.finish(id, *receipt.TurnEvidence)
	}
	return false
}

func (e *Engine) acceptEvent(id string, event b.LifecycleEvent) error {
	f := event.Fact
	if f.Kind != b.EventQuestion && f.Kind != b.EventCompleted && f.Kind != b.EventStarted && f.Kind != b.EventGap && len(event.Implementation) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.runs[id].Clone()
	if f.SessionID != "" && f.SessionID != r.Snapshot.Session.SessionID {
		return reliable.ErrStale
	}
	if f.StartedTurnID != "" && f.StartedTurnID != r.Snapshot.Session.CurrentTurnID && f.StartedTurnID != r.Snapshot.Session.PendingTurnID {
		return reliable.ErrStale
	}
	if r.SeenNative == nil {
		r.SeenNative = map[string]string{}
	}
	key := r.Snapshot.Session.AttemptID + ":" + f.Source + ":" + f.SourceEventID
	digest := reliable.Digest(canonical(event.Fact))
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
	} else if f.Kind == b.EventGap {
		r.Diagnostic = "Original observation reported a result gap"
		r.Status = "reconciling"
	} else if f.Kind == b.EventStarted && r.Snapshot.Session.PendingCommandID != "" {
		r.Snapshot.Session = reliable.RecordTurnStart(r.Snapshot.Session, f.StartedTurnID)
		r.NativeStarted = true
		r.Phase = "observing"
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
	return e.commitLocked(next, nil, "observation_attached") == nil
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
