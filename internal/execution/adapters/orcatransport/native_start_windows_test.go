package orcatransport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativeComposerGatePreventsPrematureInput(t *testing.T) {
	for _, mode := range []string{"ready", "busy", "permission", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			host.busy = mode == "busy"
			if mode == "permission" {
				host.waitBlocked = "permission"
			}
			if mode == "timeout" {
				host.waitFailures = 1
			}
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if mode == "ready" {
				if err != nil || !r.State.InputAccepted || host.nativeSends != 1 {
					t.Fatal(r, err, host.nativeSends)
				}
			} else if err == nil || host.nativeSends != 0 || r.State.InputAccepted {
				t.Fatal("unready composer received input", r, err, host.nativeSends)
			}
			if host.waitCalls != 1 {
				t.Fatal("original composer was not checked exactly once", host.waitCalls)
			}
		})
	}
}

func TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput(t *testing.T) {
	for _, mode := range []string{"original", "restored", "thin-preview", "duplicate", "altered", "ready-only", "working", "authority-conflict", "stale-metadata", "interrupted", "other-process", "query-error-conflict", "query-error-identity"} {
		t.Run(mode, func(t *testing.T) {
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			l.clock = func() time.Time { return time.Unix(100, 0).UTC() }
			base := fixtureState(10, "previous", "done")
			feed := &controlledStateFeed{initial: base, current: base}
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
			s, input := prepared(t, l, s, strings.Repeat("c", 32))
			host.unstarted = true
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil || !r.State.InputAccepted || r.State.TurnStarted {
				t.Fatal("fixture must capture accepted/unstarted receipt", r, err)
			}
			bind(&s, r)
			if mode == "restored" {
				l.closeStateFeed()
				port, e := NewLifecycleWithStateFactory(l.setup, l.stateFactory)
				if e != nil {
					t.Fatal(e)
				}
				l = port.(*Lifecycle)
			}
			t.Cleanup(l.closeStateFeed)
			state, e := l.state(s)
			if e != nil {
				t.Fatal(e)
			}
			end := fixtureState(12, "", "done")
			meta := *end.Observation
			meta.Incarnation = 1
			if mode == "authority-conflict" {
				meta.AuthorityID = "replacement"
			}
			if mode == "stale-metadata" {
				meta.Revision = 1
			}
			end.Observation = nil
			end.MetadataObservation = &meta
			end.TurnStartedAt = 11000
			end.ProviderSession.TranscriptPath = "original.jsonl"
			end.LastAssistantMessage = "original completed result"
			if mode == "ready-only" {
				end = base
			}
			if mode == "working" {
				end = fixtureState(12, "", "working")
			}
			if mode == "interrupted" {
				end.Interrupted = true
			}
			prompt := input.Prompt
			if mode == "altered" {
				prompt += " changed"
			}
			process := state.Process
			if mode == "other-process" {
				process.Incarnation = "replacement"
			}
			if mode == "thin-preview" {
				end.Prompt = preview(input.Prompt)
				end.MetadataObservation = nil
				end.ThinMetadata = true
				end.TurnStartedAt = 0
			}
			thinInputRPC(t, process, end, prompt, mode == "duplicate", mode != "thin-preview")
			l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
				return native.QueryOriginalSample(ctx, "", p, pane)
			}
			l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (native.AgentState, error) {
				current, err := native.QueryOriginalInput(ctx, "", p, pane, baseline, before)
				if err == nil && mode == "query-error-conflict" {
					current.ProviderSession.ID = "replaced-after-read"
					return current, errors.New("independent post-read mismatch")
				}
				if err == nil && mode == "query-error-identity" {
					return current, native.ErrStateIdentityChanged
				}
				return current, err
			}
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			positive := mode == "original" || mode == "restored" || mode == "thin-preview"
			if positive {
				if !r.State.TurnStarted || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage {
					t.Fatal("original completion did not establish missed start", r)
				}
				bind(&s, r)
				saved, e := l.state(s)
				if e != nil || turnFor(&saved, input.CommandID).Native.WorkingAt != 0 {
					t.Fatal("invented streamed start", e)
				}
			} else if r.State.TurnStarted || r.TurnEvidence != nil {
				t.Fatal("unverified input authorized start/completion", mode, r)
			}
			if strings.HasPrefix(mode, "query-error-") {
				bind(&s, r)
				saved, err := l.state(s)
				if err != nil || turnFor(&saved, input.CommandID).Native.Conflict == "" {
					t.Fatal("query failure discarded positive replacement evidence", err)
				}
				l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
				again, _, err := l.observeOnce(context.Background(), s)
				if err != nil || again.State.TurnStarted || again.TurnEvidence != nil || again.Diagnostic == "" {
					t.Fatal("later observation erased original identity conflict", again, err)
				}
			}
			sends := 0
			for _, call := range host.calls {
				if len(call) > 1 && call[0] == "terminal" && call[1] == "send" {
					sends++
				}
			}
			if sends != 1 || host.nativeSends != 1 {
				t.Fatal("recovery sent another input", sends, host.nativeSends)
			}
		})
	}
}
