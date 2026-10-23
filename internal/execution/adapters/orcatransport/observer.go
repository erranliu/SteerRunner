package orcatransport

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

type observer struct {
	events   chan b.LifecycleEvent
	done     chan error
	finished chan struct{}
	cancel   context.CancelFunc
	process  b.ProcessIdentity
	once     sync.Once
}

func (o *observer) Events() <-chan b.LifecycleEvent { return o.events }
func (o *observer) Done() <-chan error              { return o.done }
func (o *observer) StopHandle() b.StopHandle        { return &observationHandle{owner: o} }

// Passive cancellation cannot await a completion event which the actor is acknowledging.
func (o *observer) Close(context.Context) error { o.once.Do(o.cancel); return nil }

// Passive cancellation stays nonblocking: the actor may be acknowledging an
// event. Service shutdown separately awaits this caller-side goroutine; the
// fixed execution-host control actor stays owned by the original execution.
func (o *observer) awaitCleanup(ctx context.Context) error {
	if o.finished == nil {
		return nil
	}
	select {
	case <-o.finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type observationHandle struct{ owner *observer }

func (h *observationHandle) Identity() b.ProcessIdentity { return h.owner.process }
func (h *observationHandle) Kill()                       { _ = h.owner.Close(context.Background()) }
func (h *observationHandle) Close()                      { _ = h.owner.Close(context.Background()) }

func (l *Lifecycle) observe(s b.LifecycleSnapshot, first *b.PublicMessage) b.LifecycleObserver {
	ctx, cancel := context.WithCancel(context.Background())
	o := &observer{events: make(chan b.LifecycleEvent, 4), done: make(chan error, 1), finished: make(chan struct{}), cancel: cancel, process: s.Ref.External}
	l.feedMu.Lock()
	ownedFeed := l.feed
	l.feedMu.Unlock()
	l.observationMu.Lock()
	ownedObservation := l.nativeObservation
	l.observationMu.Unlock()
	go func() {
		var observerErr error
		defer func() {
			l.releaseStateFeed(ownedFeed)
			l.clearObservation(ownedObservation)
			close(o.events)
			if ctx.Err() == nil {
				o.done <- observerErr
			}
			close(o.done)
			close(o.finished)
		}()
		send := func(event b.LifecycleEvent) bool {
			event.Kind = "turn"
			if event.Observation != nil {
				event.Kind, event.Status, event.Title = "execution", "observing", "Worker observation available"
				if event.Observation.Diagnostic != "" {
					event.Status, event.Title = "waiting", "Worker reply unavailable"
				}
			} else if event.Fact.Kind == b.EventStarted {
				event.Status, event.Title = "started", "Worker input started"
			} else if event.Fact.Kind == b.EventCompleted {
				event.Status, event.Title = "completed", "Worker reply committed"
			} else {
				event.Kind, event.Status, event.Title = "execution", "observing", "Worker output observed"
			}
			event.Persisted = make(chan error, 1)
			event.Processed = make(chan error, 1)
			select {
			case o.events <- event:
			case <-ctx.Done():
				return false
			}
			select {
			case err := <-event.Persisted:
				if err != nil {
					observerErr = err
					return false
				}
			case <-ctx.Done():
				return false
			}
			select {
			case err := <-event.Processed:
				if err != nil {
					observerErr = err
					return false
				}
			case <-ctx.Done():
				return false
			}
			return true
		}
		if first != nil {
			if !send(b.LifecycleEvent{PublicMessage: first}) {
				return
			}
			l.acknowledgeMessage(first)
		}
		started, ended := s.Session.CurrentCommandID == command(s) && s.Session.CurrentTurnID == command(s), false
		lastDiagnostic, diagnosticSet := "", false
		for {
			before, _ := json.Marshal(s.Implementation)
			receipt, message, err := l.observeOnce(ctx, s)
			// A restored observer may acquire its subscription on a later retry.
			if ownedFeed == nil {
				l.feedMu.Lock()
				if l.feedProcess == o.process {
					ownedFeed = l.feed
				}
				l.feedMu.Unlock()
			}
			if err != nil {
				if ctx.Err() == nil && native.IsCLIQueueError(err) {
					// Retry only this read-only metadata observation. The callback
					// never ran; no input or auxiliary actor can be duplicated.
					select {
					case <-ctx.Done():
						return
					case <-time.After(250 * time.Millisecond):
						continue
					}
				}
				if ctx.Err() == nil {
					observerErr = err
				}
				return
			}
			if len(receipt.Implementation) > 0 {
				s.Implementation = receipt.Implementation
			}
			if message != nil {
				if !send(b.LifecycleEvent{PublicMessage: message}) {
					return
				}
				l.acknowledgeMessage(message)
			}
			afterDiagnostic, _ := json.Marshal(receipt.Implementation)
			if !diagnosticSet || receipt.Diagnostic != lastDiagnostic || string(before) != string(afterDiagnostic) {
				if !send(b.LifecycleEvent{Observation: &b.ObservationStatus{Diagnostic: receipt.Diagnostic}, Fact: b.EventFact{Kind: b.EventStatus, Source: "orca.terminal.observation", SourceIdentity: SessionID(s.Ref.External), SourceSegment: command(s), SourceEventID: command(s) + ":observation:" + contracts.NewID(), SessionID: SessionID(s.Ref.External), StartedTurnID: command(s)}, Implementation: receipt.Implementation}) {
					return
				}
				lastDiagnostic, diagnosticSet = receipt.Diagnostic, true
			}
			if receipt.State.TurnStarted && !started {
				source := NativeReplySource
				if state, e := l.state(s); e == nil {
					if turn := turnFor(&state, command(s)); turn != nil && turn.Recorded != nil {
						source = turn.Recorded.Source
					} else if turn := turnFor(&state, command(s)); turn != nil && turn.Extension != nil {
						source = turn.Extension.Source
					}
				}
				if !send(b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStarted, Source: source, SourceIdentity: SessionID(s.Ref.External), SourceSegment: command(s), SourceEventID: command(s) + ":started", SessionID: SessionID(s.Ref.External), StartedTurnID: command(s), InputAccepted: true, Matched: true}, Implementation: receipt.Implementation}) {
					return
				}
				started = true
			}
			if receipt.State.TurnCompleted && receipt.TurnEvidence == nil && !ended {
				if !send(b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventStatus, Source: "orca.native-main-agent", SourceIdentity: SessionID(s.Ref.External), SourceSegment: command(s), SourceEventID: command(s) + ":native-ended", SessionID: SessionID(s.Ref.External), StartedTurnID: command(s), Completed: true, Matched: true, Outcome: "unknown"}, Implementation: receipt.Implementation}) {
					return
				}
				ended = true
			}
			if evidence := receipt.TurnEvidence; evidence != nil && evidence.Completed {
				outcome := "failed"
				if evidence.Succeeded {
					outcome = "succeeded"
				}
				if evidence.NativeOutcome != "" {
					outcome = evidence.NativeOutcome
				}
				if !send(b.LifecycleEvent{Fact: b.EventFact{Kind: b.EventCompleted, Source: evidence.FinalSource, SourceIdentity: SessionID(s.Ref.External), SourceSegment: command(s), SourceEventID: evidence.FinalItemID, SessionID: SessionID(s.Ref.External), StartedTurnID: command(s), Completed: true, Matched: true, Outcome: outcome, LastMessage: evidence.LastMessage}, Implementation: receipt.Implementation}) {
					return
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return o
}
