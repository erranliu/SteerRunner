package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
	raw, err := os.ReadFile("testdata/completion-zero-incarnation.json")
	if err != nil {
		t.Fatal(err)
	}
	var captured struct {
		Prompt   string                  `json:"prompt"`
		State    native.AgentState       `json:"state"`
		Metadata native.StateObservation `json:"metadata_observation"`
	}
	if err = json.Unmarshal(raw, &captured); err != nil || captured.Metadata.Incarnation != 0 || captured.State.MainAgent == nil {
		t.Fatal("captured completion fixture invalid", err)
	}
	for _, mode := range []string{"original", "restored", "thin-preview", "duplicate", "altered", "ready-only", "working", "authority-conflict", "stale-metadata", "interrupted", "other-process", "query-error-conflict", "query-error-identity", "zero-incarnation", "zero-incarnation-restored", "zero-incarnation-started", "multiline-zero-incarnation", "multiline-generation-one", "missing-incarnation", "null-incarnation", "multiline-missing-incarnation", "multiline-null-incarnation"} {
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
			multiline := strings.HasPrefix(mode, "multiline-")
			var input b.TurnInput
			if multiline {
				input = b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: captured.Prompt}
				s.PendingTurn = &input
				r, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
				host.screen = true
				l.queryNative = func(context.Context, b.ProcessIdentity, string) (native.StateSample, error) {
					return native.StateSample{AgentState: base, ReceivedAt: l.clock(), Kind: native.SampleQuery}, nil
				}
			} else {
				s, input = prepared(t, l, s, strings.Repeat("c", 32))
			}
			host.unstarted = mode != "zero-incarnation-started"
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
			if err != nil || !r.State.InputAccepted || r.State.TurnStarted != !host.unstarted {
				t.Fatal("fixture must capture the original start receipt", r, err)
			}
			bind(&s, r)
			if mode == "zero-incarnation-started" {
				// The ordinary text+Enter path has an original start receipt and
				// can retain the observed working root on the legal initial generation.
				working := fixtureState(11, input.Prompt, "working")
				feed.events = []native.AgentState{working}
				feed.current = working
				r, _, err = l.observeOnce(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				bind(&s, r)
			}
			if mode == "restored" || mode == "zero-incarnation-restored" {
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
			if strings.HasPrefix(mode, "zero-incarnation") || mode == "multiline-zero-incarnation" {
				meta.Incarnation = 0
			}
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
			if multiline {
				// Replay the real canonical done shape, with only identity/time
				// normalization documented in the fixture. The single-field
				// generation=1 case is an explicit controlled counterfactual.
				end = captured.State
				root := *captured.State.MainAgent
				end.MainAgent = &root
				meta = captured.Metadata
				if mode == "multiline-generation-one" {
					meta.Incarnation = 1
				}
				end.MetadataObservation = &meta
				working := fixtureState(11, input.Prompt, "working")
				working.MainAgent.StateStartedAt = end.TurnStartedAt
				working.UpdatedAt = end.TurnStartedAt
				working.StateStartedAt = end.TurnStartedAt
				working.TurnStartedAt = end.TurnStartedAt
				working.Observation.ObservedAt = end.TurnStartedAt
				working.Observation.Incarnation = meta.Incarnation
				done := end
				done.Observation = &meta
				done.MetadataObservation = nil
				done.Prompt = preview(input.Prompt)
				feed.events = []native.AgentState{working, done}
			}
			missing := strings.Contains(mode, "missing-incarnation") || strings.Contains(mode, "null-incarnation")
			if missing {
				wire, e := json.Marshal(meta)
				if e != nil {
					t.Fatal(e)
				}
				var fields map[string]json.RawMessage
				if e = json.Unmarshal(wire, &fields); e != nil {
					t.Fatal(e)
				}
				delete(fields, "incarnation")
				if strings.Contains(mode, "null-incarnation") {
					fields["incarnation"] = json.RawMessage("null")
				}
				wire, e = json.Marshal(fields)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(wire, &meta); e != nil {
					t.Fatal(e)
				}
			}
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
			inputQueries := 0
			l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, baseline int64, before native.AgentState) (native.AgentState, error) {
				inputQueries++
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
			positive := mode == "original" || mode == "restored" || mode == "thin-preview" || strings.HasPrefix(mode, "zero-incarnation") || mode == "multiline-zero-incarnation" || mode == "multiline-generation-one"
			if positive {
				if !r.State.TurnStarted || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != end.LastAssistantMessage {
					t.Fatal("original completion did not establish missed start", r)
				}
				bind(&s, r)
				saved, e := l.state(s)
				if e != nil || mode != "zero-incarnation-started" && turnFor(&saved, input.CommandID).Native.WorkingAt != 0 {
					t.Fatal("invented streamed start", e)
				}
				if mode == "zero-incarnation-started" {
					queried, e := native.QueryOriginalSample(context.Background(), "", saved.Process, end.PaneKey)
					if e != nil {
						t.Fatal(e)
					}
					checked, e := native.QueryOriginalInput(context.Background(), "", saved.Process, end.PaneKey, turnFor(&saved, input.CommandID).Native.BaseRootAt, queried.AgentState)
					if e != nil || !shortOriginalInputVerified(checked, saved.Process, turnFor(&saved, input.CommandID)) {
						t.Fatal("probe rejected existing observed-root proof", e)
					}
				}
			} else if r.State.TurnStarted || r.TurnEvidence != nil {
				t.Fatal("unverified input authorized start/completion", mode, r)
			}
			if mode == "zero-incarnation" || mode == "zero-incarnation-restored" || mode == "multiline-zero-incarnation" {
				if inputQueries != 1 {
					t.Fatal("explicit zero did not use the original fenced query", inputQueries)
				}
				t.Logf("RECOVERED accepted=%t started=%t final=%t input_queries=%d", r.State.InputAccepted, r.State.TurnStarted, r.TurnEvidence.FinalVerified, inputQueries)
			}
			if missing {
				// An absent/null counter must remain incomplete, including after
				// the RPC and implementation snapshot JSON round trips.
				if inputQueries != 0 || !strings.Contains(r.Diagnostic, "no verified start receipt") {
					t.Fatal("incomplete observation authorized recovery", inputQueries, r.Diagnostic)
				}
				bind(&s, r)
				l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
				again, _, e := l.observeOnce(context.Background(), s)
				if e != nil || again.State.TurnStarted || again.TurnEvidence != nil || again.Diagnostic != r.Diagnostic || inputQueries != 0 {
					t.Fatal("incomplete observation was promoted on retry", again, e, inputQueries)
				}
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
			wantSends := 1
			if multiline {
				wantSends = 2
			}
			if sends != wantSends || host.nativeSends != 1 {
				t.Fatal("recovery sent another input", sends, host.nativeSends)
			}
		})
	}
}
