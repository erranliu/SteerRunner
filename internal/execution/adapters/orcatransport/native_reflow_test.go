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
	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

type reflowTestViewport struct{ restore func() error }

func TestNativePreparationAcceptsLongCallerInput(t *testing.T) {
	l, _, s := fixtureOn(t, "paired-worker")
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	text := strings.Repeat("中文😀\n", 500)
	input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: text, InputVersion: 1}
	_, input.Budget, err = m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	s.PendingTurn = &input
	r, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	defer l.closeStateFeed()
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	turn := turnFor(&state, command(s))
	// Preserve the existing native trailing-newline normalization.
	want := strings.TrimRight(text, "\r\n")
	if nativeWireText(turn) != want || turn.WireBudget.CallerBytes != len(want) || turn.WireBudget.TotalBytes <= 3000 || m.CheckInput(want, turn.WireBudget) != nil {
		t.Fatal("long caller input changed or rejected", turn.WireBudget)
	}
}

func (v reflowTestViewport) Restore(context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{"restored":true}`), v.restore()
}

func TestNative833ReflowPreservesExactTextAndFences(t *testing.T) {
	for _, fault := range []string{"before-fix", "none", "inserted-wrap", "real-newline", "real-inserted-newline", "changed-byte", "trailing-blank", "lost-fit-reply", "changed-provider", "changed-terminal", "restore-failed"} {
		t.Run(fault, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/draft833.txt")
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if len([]rune(text)) != 833 {
				t.Fatal("frozen original 833 input changed")
			}
			if fault == "trailing-blank" {
				text += " "
			}
			painted := strings.Replace(text, "; coordinate writers", "; coordinate\nwriters", 1)
			if fault == "trailing-blank" {
				painted = strings.TrimSuffix(painted, " ")
			}
			if fault == "inserted-wrap" || fault == "real-inserted-newline" {
				painted = strings.Replace(painted, "隔离投递诊断", "隔离投递\n诊断", 1)
			}
			if painted == text || nativePasteDraftMatches(painted, text) || nativeDraftNeedsReflow(painted, text) != (fault != "trailing-blank") {
				t.Fatal("fixture no longer rejects the original ambiguity")
			}
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: text, InputVersion: 1}
			_, input.Budget, err = m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: text})
			if err != nil {
				t.Fatal("reflow fixture must fit the unchanged input budgets", err)
			}
			s.PendingTurn = &input
			r, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			defer l.closeStateFeed()
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			turn := turnFor(&state, command(s))
			if nativeWireText(turn) != text || turn.WireBudget.TotalBytes != len(text) {
				t.Fatal("prepared reflow input differs from frozen bytes")
			}
			base := fixtureState(10, "previous", "done")
			base.PaneKey, base.ProviderSession.ID, base.MainAgent.StateStartedAt = turn.Native.Pane, turn.Native.Provider, turn.Native.BaseRootAt
			wide, pasted, entered, fits, restores := false, 0, 0, 0, 0
			l.queryNative = func(context.Context, b.ProcessIdentity, string) (native.StateSample, error) {
				v := base
				if fault == "changed-provider" && wide {
					v.ProviderSession.ID = "another-provider"
				}
				return native.StateSample{AgentState: v}, nil
			}
			l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				if len(args) > 1 && args[0] == "terminal" && args[1] == "read" {
					draft := ""
					if pasted > 0 {
						draft = painted
						if wide && fault != "real-newline" && fault != "real-inserted-newline" {
							draft = text
						}
						if wide && fault == "changed-byte" {
							draft = strings.Replace(text, "writers", "writerx", 1)
						}
					}
					return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "runtime"}, "result": map[string]any{"terminal": map[string]any{"handle": "terminal", "source": "screen", "draft": draft}}})
				}
				if len(args) > 1 && args[0] == "terminal" && args[1] == "send" {
					size := 1
					for i, a := range args {
						if a == "--text" {
							pasted++
							if args[i+1] != "\x1b[200~"+text+"\x1b[201~" {
								t.Fatal("fix mutated original text")
							}
							size = len(args[i+1])
						}
					}
					if size == 1 {
						entered++
					}
					return json.Marshal(map[string]any{"ok": true, "_meta": map[string]string{"runtimeId": "runtime"}, "result": map[string]any{"send": map[string]any{"handle": "terminal", "accepted": true, "bytesWritten": size}}})
				}
				return host.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
			}})
			if fault != "before-fix" {
				l.fitComposer = func(context.Context, b.ProcessIdentity, native.ViewportRecorder) (composerViewport, json.RawMessage, error) {
					fits++
					wide = true
					lease := reflowTestViewport{restore: func() error {
						restores++
						wide = false
						if fault == "restore-failed" {
							return errors.New("restore receipt lost")
						}
						return nil
					}}
					if fault == "changed-terminal" {
						host.incarnation = "replacement"
					}
					if fault == "lost-fit-reply" {
						return lease, nil, errors.New("fit response lost")
					}
					return lease, json.RawMessage(`{"terminal":{"cols":1024,"rows":80}}`), nil
				}
			}
			budget := 600 * time.Millisecond
			if fault == "none" || fault == "inserted-wrap" || fault == "restore-failed" {
				budget = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			_, err = l.submitNativePaste(ctx, s, text)
			if pasted != 1 || entered > 1 || fits > 1 || restores != fits {
				t.Fatal("effect repeated or viewport leaked", pasted, entered, fits, restores, err)
			}
			if fault == "none" || fault == "inserted-wrap" {
				if err != nil || entered != 1 {
					t.Fatal("unchanged 833 input not submitted", err, entered)
				}
			} else if err == nil {
				t.Fatal("fault accepted", fault)
			}
			if fault != "none" && fault != "inserted-wrap" && fault != "restore-failed" && entered != 0 {
				t.Fatal("unverified original submitted", fault)
			}
		})
	}
}

func TestComposerReflowCannotBypassInputBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		owner  m.InputOwner
		source string
		count  int
	}{
		{"runner", m.RunnerInput, "workflow.body", 1001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			fits := 0
			l.fitComposer = func(context.Context, b.ProcessIdentity, native.ViewportRecorder) (composerViewport, json.RawMessage, error) {
				fits++
				return nil, nil, errors.New("must not fit an oversized input")
			}
			text, budget, err := m.DescribeInput(m.InputPart{Source: tc.source, Owner: tc.owner, Text: strings.Repeat("x", tc.count)})
			if err != nil {
				t.Fatal(err)
			}
			input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: text, Budget: budget, InputVersion: 1}
			s.PendingTurn = &input
			_, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
			if err == nil || fits != 0 || host.nativeSends != 0 {
				t.Fatal("oversized input reached reflow/delivery", err, fits, host.nativeSends)
			}
		})
	}
}
