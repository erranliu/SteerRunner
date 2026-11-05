package orcatransport

import (
	"context"

	"sync"

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

type observationHandle struct{ owner *observer }

func (h *observationHandle) Identity() b.ProcessIdentity { return h.owner.process }
func (h *observationHandle) Kill()                       { _ = h.owner.Close(context.Background()) }
func (h *observationHandle) Close()                      { _ = h.owner.Close(context.Background()) }
