package orcatransport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativeFollowupAfterCompletedToolTurn(t *testing.T) {
	for _, fault := range []string{"none", "zero-incarnation", "draft-only", "submit-response-lost", "old-final", "changed-input"} {
		t.Run(fault, func(t *testing.T) {
			l, host, feed, s, first := publicMessageFixture(t)
			feed.events = []native.AgentState{messageStart(first)}
			r, _, err := l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			end := fixtureState(12, first.Prompt, "done")
			end.TurnStartedAt, end.LastAssistantMessage = 11000, "FIRST_DONE"
			feed.current, feed.events = end, []native.AgentState{end}
			l.clock = func() time.Time { return time.Unix(131, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified {
				t.Fatal("first completion", r.Diagnostic, err)
			}
			bind(&s, r)
			l.closeStateFeed()
			feed = &controlledStateFeed{initial: end, current: end}
			l.stateFactory = func(context.Context, string, b.ProcessIdentity) (native.StateFeed, error) { return feed, nil }
			second := b.TurnInput{CommandID: strings.Repeat("d", 32), Prompt: "后续发布输入诊断\r\n\r\n" + strings.Repeat("中文占位正文。\n", 400)}
			s.PendingTurn = &second
			r, err = l.prepareInput(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &second}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			host.screen = true
			if fault == "draft-only" {
				host.lostPasteReply = true
			}
			if fault == "submit-response-lost" {
				host.lostEnterReply = true
			}
			l.queryNative = func(context.Context, b.ProcessIdentity, string) (native.StateSample, error) {
				return native.StateSample{AgentState: end}, nil
			}
			r, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &second}))
			bind(&s, r)
			if fault == "draft-only" || fault == "submit-response-lost" {
				if err == nil || r.Disposition != b.OperationUnknown || r.State.InputAccepted {
					t.Fatal("unknown became accepted", r, err)
				}
				before := len(host.calls)
				if _, err := l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &second})); err == nil || len(host.calls) != before {
					t.Fatal("unknown second input replayed", err)
				}
				if (host.draft != "") != (fault == "draft-only") {
					t.Fatal("fixture failed to distinguish draft from applied submit")
				}
				return
			}
			if err != nil || !r.State.InputAccepted || r.State.TurnStarted {
				t.Fatal(r, err)
			}
			// The prior final must not qualify the new input, even after acceptance.
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil || r.TurnEvidence != nil {
				t.Fatal("old final qualified second input", r, err)
			}
			if fault == "old-final" {
				return
			}
			current := fixtureState(14, "", "done")
			current.ProviderSession.TranscriptPath = "original.jsonl"
			current.TurnStartedAt, current.LastAssistantMessage = 13000, "FOLLOWUP_DONE"
			meta := *current.Observation
			meta.Incarnation = 1
			if fault == "zero-incarnation" {
				meta.Incarnation = 0
			}
			current.Observation, current.MetadataObservation = nil, &meta
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			text := nativePromptText(second.Prompt)
			if fault == "changed-input" {
				text += "foreign"
			}
			thinInputRPCAt(t, state.Process, current, text, false, 13001, true)
			l.queryNative = func(ctx context.Context, p b.ProcessIdentity, pane string) (native.StateSample, error) {
				return native.QueryOriginalSample(ctx, "", p, pane)
			}
			l.queryInput = func(ctx context.Context, p b.ProcessIdentity, pane string, baseline int64, sample native.AgentState) (native.AgentState, error) {
				if baseline != 12000 {
					t.Fatal("wrong second baseline", baseline)
				}
				return native.QueryOriginalInput(ctx, "", p, pane, baseline, sample)
			}
			l.clock = func() time.Time { return time.Unix(162, 0).UTC() }
			r, _, err = l.observeOnce(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "changed-input" {
				if r.TurnEvidence != nil || r.Diagnostic == "" {
					t.Fatal("changed followup accepted", r)
				}
			} else if r.TurnEvidence == nil || !r.TurnEvidence.FinalVerified || r.TurnEvidence.LastMessage != "FOLLOWUP_DONE" {
				t.Fatal("followup not completed", r.Diagnostic)
			}
			if host.nativeSends != 2 {
				t.Fatal("wrong logical sends", host.nativeSends)
			}
			files, _ := filepath.Glob(filepath.Join(l.setup.RunDirectory, "native-input", "*", "*", "input.result.json"))
			if len(files) < 4 {
				t.Fatal("missing per-input timelines", files)
			}
			for _, path := range files {
				raw, e := os.ReadFile(path)
				var value struct {
					Input nativeInputDiagnostic `json:"input"`
				}
				if e != nil || json.Unmarshal(raw, &value) != nil || value.Input.CommandID == "" || value.Input.InputBytes == 0 || value.Input.InputSHA256 == "" {
					t.Fatal("incomplete diagnostic identity", path, e)
				}
			}
		})
	}
}
