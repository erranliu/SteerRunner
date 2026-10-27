package orcatransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestNativeMultilinePasteRequiresSettledOriginalDraft(t *testing.T) {
	for _, fault := range []string{"none", "unicode-collapsed", "existing-draft", "unavailable-screen", "wrong-size", "paste-response-lost", "wrong-draft", "replaced-terminal", "changed-provider", "changed-main", "enter-response-lost"} {
		t.Run(fault, func(t *testing.T) {
			l, host, s := fixtureOn(t, "paired-worker")
			r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
			if err != nil {
				t.Fatal(err)
			}
			bind(&s, r)
			s, _ = prepared(t, l, s, strings.Repeat("c", 32))
			defer l.closeStateFeed()
			state, err := l.state(s)
			if err != nil {
				t.Fatal(err)
			}
			turn := turnFor(&state, command(s))
			if turn == nil {
				t.Fatal("turn")
			}
			base := fixtureState(10, "previous", "done")
			base.PaneKey = turn.Native.Pane
			base.ProviderSession.ID = turn.Native.Provider
			base.MainAgent.StateStartedAt = turn.Native.BaseRootAt
			l.queryNative = func(context.Context, b.ProcessIdentity, string) (native.StateSample, error) {
				v := base
				if fault == "changed-provider" {
					v.ProviderSession.ID = "other"
				}
				if fault == "changed-main" {
					m := *v.MainAgent
					m.StateStartedAt++
					v.MainAgent = &m
				}
				return native.StateSample{AgentState: v}, nil
			}
			text := "One original prompt\n" + strings.Repeat("context ", 180)
			if fault == "unicode-collapsed" {
				text += "\n\u4e2d\u6587\U0001f642"
			}
			pasted, entered, reads := 0, 0, 0
			l.client = native.New(native.Config{Output: func(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				if len(args) > 1 && args[0] == "terminal" && args[1] == "read" {
					reads++
					draft := ""
					source := "screen"
					runtime := "runtime"
					if pasted > 0 {
						draft = text
						if fault == "unicode-collapsed" {
							draft = fmt.Sprintf("[Pasted Content %d chars]", utf8.RuneCountInString(text))
						}
					}
					if fault == "existing-draft" {
						draft = "human draft"
					}
					if fault == "wrong-draft" && pasted > 0 {
						draft = "partial paste"
					}
					if fault == "unavailable-screen" {
						source = "stream"
					}
					if fault == "replaced-terminal" && pasted > 0 {
						runtime = "other"
					}
					return json.Marshal(map[string]any{"ok": true, "_meta": map[string]any{"runtimeId": runtime}, "result": map[string]any{"terminal": map[string]any{"handle": "terminal", "source": source, "draft": draft}}})
				}
				if len(args) > 1 && args[0] == "terminal" && args[1] == "send" {
					size := 1
					for i, a := range args {
						if a == "--text" {
							pasted++
							payload := args[i+1]
							if payload != "\x1b[200~"+text+"\x1b[201~" {
								t.Fatal("original text changed")
							}
							size = len(payload)
						}
					}
					if size == 1 {
						entered++
					}
					if fault == "paste-response-lost" && size > 1 || fault == "enter-response-lost" && size == 1 {
						return nil, errors.New("lost response")
					}
					if fault == "wrong-size" {
						size--
					}
					return json.Marshal(map[string]any{"ok": true, "_meta": map[string]any{"runtimeId": "runtime"}, "result": map[string]any{"send": map[string]any{"handle": "terminal", "accepted": true, "bytesWritten": size}}})
				}
				return host.OutputCommand(ctx, b.LaunchCommand{Args: args, Timeout: timeout})
			}})
			budget := 2 * time.Second
			if fault == "wrong-draft" {
				budget = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			_, err = l.submitNativePaste(ctx, s, text)
			if fault == "none" || fault == "unicode-collapsed" {
				if err != nil || pasted != 1 || entered != 1 || reads != 3 {
					t.Fatal("sole staged input not verified", err, pasted, entered, reads)
				}
			} else {
				if err == nil || pasted > 1 || entered > 1 {
					t.Fatal("fault not rejected", err, pasted, entered)
				}
				if fault != "enter-response-lost" && entered != 0 {
					t.Fatal("unverified paste was submitted", fault)
				}
			}
		})
	}
}
