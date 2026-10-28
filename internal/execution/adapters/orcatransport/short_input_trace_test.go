package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

// Probe-only passive recording. Delegates return the original values unchanged;
// neither observations nor a diagnostic transcript can authorize completion.
type shortInputTrace struct {
	mu       sync.Mutex
	file     *os.File
	sequence int
	err      error
}

func newShortInputTrace(t *testing.T, root string) *shortInputTrace {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(root, "native-observer.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	trace := &shortInputTrace{file: f}
	t.Cleanup(func() {
		trace.mu.Lock()
		defer trace.mu.Unlock()
		if err := f.Close(); err != nil {
			t.Error(err)
		}
		if trace.err != nil {
			t.Error(trace.err)
		}
	})
	return trace
}
func (r *shortInputTrace) write(kind string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sequence++
	data, err := json.Marshal(map[string]any{"sequence": r.sequence, "at": time.Now().UTC(), "kind": kind, "value": value})
	if err == nil {
		_, err = r.file.Write(append(data, '\n'))
	}
	if err == nil {
		err = r.file.Sync()
	}
	if err != nil && r.err == nil {
		r.err = err
	}
}
func traceAgent(s native.AgentState, p b.ProcessIdentity) any {
	value := map[string]any{"state": s, "metadata_observation": s.MetadataObservation, "thin_metadata": s.ThinMetadata, "can_query_completion_input": s.CanQueryCompletionInput()}
	if s.NativeInput != nil {
		value["native_input"], value["public_pages"] = s.NativeInput, s.NativeInput.Pages
		value["matches_original_query"], value["matches_original_process"] = s.NativeInput.MatchesOriginalQuery(s), s.NativeInput.MatchesOriginalProcess(p)
	}
	return value
}
func traceSample(s native.StateSample, p b.ProcessIdentity) any {
	return map[string]any{"agent": traceAgent(s.AgentState, p), "received_at": s.ReceivedAt, "sample_kind": s.Kind}
}

type shortTraceFeed struct {
	native.StateFeed
	trace   *shortInputTrace
	process b.ProcessIdentity
	id      int
}

func (f *shortTraceFeed) Initial() native.StateSample {
	s := f.StateFeed.Initial()
	f.trace.write("feed.initial", map[string]any{"feed": f.id, "sample": traceSample(s, f.process)})
	return s
}
func (f *shortTraceFeed) Drain() ([]native.StateSample, error) {
	samples, err := f.StateFeed.Drain()
	values := []any{}
	for _, s := range samples {
		values = append(values, traceSample(s, f.process))
	}
	f.trace.write("feed.drain", map[string]any{"feed": f.id, "samples": values, "error": errorText(err)})
	return samples, err
}
func (f *shortTraceFeed) Query(ctx context.Context) (native.StateSample, error) {
	f.trace.write("feed.query.begin", map[string]any{"feed": f.id})
	s, err := f.StateFeed.Query(ctx)
	f.trace.write("feed.query.end", map[string]any{"feed": f.id, "sample": traceSample(s, f.process), "error": errorText(err), "context_error": errorText(ctx.Err())})
	return s, err
}
func (f *shortTraceFeed) Close() {
	f.trace.write("feed.close", map[string]any{"feed": f.id})
	f.StateFeed.Close()
}

func (r *shortInputTrace) attach(l *Lifecycle) {
	if fit := l.fitComposer; fit != nil {
		l.fitComposer = func(ctx context.Context, p b.ProcessIdentity, record native.ViewportRecorder) (composerViewport, json.RawMessage, error) {
			return fit(ctx, p, func(event native.ViewportRPC) error {
				r.write("viewport.rpc", event)
				if record != nil {
					return record(event)
				}
				return nil
			})
		}
	}
	factory := l.stateFactory
	var mu sync.Mutex
	feedID := 0
	l.stateFactory = func(ctx context.Context, environment string, p b.ProcessIdentity) (native.StateFeed, error) {
		r.write("feed.open.begin", map[string]any{"process": p, "environment": environment})
		f, err := factory(ctx, environment, p)
		r.write("feed.open.end", map[string]any{"process": p, "error": errorText(err), "context_error": errorText(ctx.Err())})
		if err != nil {
			return f, err
		}
		mu.Lock()
		feedID++
		id := feedID
		mu.Unlock()
		return &shortTraceFeed{StateFeed: f, trace: r, process: p, id: id}, nil
	}
	query := l.queryNative
	l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
		r.write("query.native.begin", map[string]any{"process": p, "pane": pane})
		s, err := query(ctx, p, pane)
		r.write("query.native.end", map[string]any{"sample": traceSample(s, p), "error": errorText(err), "identity_error": errors.Is(err, native.ErrStateIdentityChanged), "context_error": errorText(ctx.Err())})
		return s, err
	}
	input := l.queryInput
	l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (native.AgentState, error) {
		r.write("query.input.begin", map[string]any{"process": p, "pane": pane, "baseline": baseline, "before": traceAgent(before, p)})
		s, err := input(ctx, p, pane, baseline, before)
		r.write("query.input.end", map[string]any{"after": traceAgent(s, p), "error": errorText(err), "identity_error": errors.Is(err, native.ErrStateIdentityChanged), "context_error": errorText(ctx.Err())})
		return s, err
	}
}

func (r *shortInputTrace) event(index int, event b.LifecycleEvent, s b.LifecycleSnapshot, l *Lifecycle) {
	state, err := l.state(s)
	r.write("observer.event", map[string]any{"input_index": index, "kind": event.Kind, "fact": event.Fact, "observation": event.Observation, "public_message": event.PublicMessage, "implementation": event.Implementation, "snapshot_session": s.Session, "state": state, "state_error": errorText(err)})
}

// Called only after the sole attempted input has failed observation. Uses a
// separate read-only context before terminal cleanup, never feeds a receipt back.
func (r *shortInputTrace) failure(l *Lifecycle, s b.LifecycleSnapshot, creator native.Client, p b.ProcessIdentity, pane string, index int) {
	state, err := l.state(s)
	r.write("failure.saved", map[string]any{"input_index": index, "state": state, "error": errorText(err)})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	sample, err := native.QueryOriginalSample(ctx, l.profile.Environment, p, pane)
	r.write("failure.native", map[string]any{"sample": traceSample(sample, p), "error": errorText(err)})
	if err == nil {
		if turn := turnFor(&state, command(s)); turn != nil {
			input, e := native.QueryNativeInput(ctx, l.profile.Environment, p.RuntimeID, sample.ProviderSession.ID, sample.ProviderSession.TranscriptPath, turn.Native.BaseRootAt)
			r.write("failure.public_input", map[string]any{"input": input, "error": errorText(e)})
			if input != nil {
				r.write("failure.public_pages", input.Pages)
			}
		}
	}
	read, e := creator.MetaCall(ctx, "terminal", "read", "--terminal", p.Terminal, "--limit", "80", "--json")
	r.write("failure.screen", map[string]any{"read": read, "error": errorText(e)})
}
