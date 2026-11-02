package orca

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestOrcaTerminalReadCapability(t *testing.T) {
	for _, scenario := range []string{"raw-page", "screen", "missing-limited", "missing-truncated", "limited", "wrong-runtime", "wrong-handle", "bad-next", "bad-latest", "duplicate-next", "bad-tail", "malformed", "response-lost"} {
		t.Run(scenario, func(t *testing.T) {
			lines := []string{" first\r", "", "partial trailing line"}
			page := map[string]any{"handle": "terminal", "source": "stream", "oldestCursor": "5", "nextCursor": "9", "latestCursor": "10", "tail": lines, "truncated": true, "limited": false}
			runtime := "runtime"
			switch scenario {
			case "screen":
				page["source"] = "screen"
			case "missing-limited":
				delete(page, "limited")
			case "missing-truncated":
				delete(page, "truncated")
			case "limited":
				page["limited"] = true
			case "wrong-runtime":
				runtime = "foreign"
			case "wrong-handle":
				page["handle"] = "foreign"
			case "bad-next":
				page["nextCursor"] = 42
			case "bad-latest":
				page["latestCursor"] = []any{}
			case "bad-tail":
				page["tail"] = []any{42}
			}
			result := map[string]any{"terminal": page}
			if scenario == "malformed" {
				result["terminal"] = "not a page"
			}
			raw, _ := json.Marshal(map[string]any{"ok": true, "result": result, "_meta": map[string]string{"runtimeId": runtime}})
			lost := errors.New("read response lost")
			calls := 0
			client := New(Config{CommandTimeout: "17s", Output: func(_ context.Context, timeout time.Duration, args ...string) ([]byte, error) {
				calls++
				if timeout != 17*time.Second || !reflect.DeepEqual(args, []string{"terminal", "read", "--terminal", "terminal", "--cursor", "7", "--limit", "1000", "--json"}) {
					t.Fatal("read changed original argv or deadline", args, timeout)
				}
				if scenario == "response-lost" {
					return nil, lost
				}
				if scenario == "duplicate-next" {
					return []byte(`{"ok":true,"result":{"terminal":{"handle":"terminal","source":"stream","oldestCursor":"5","nextCursor":42,"nextCursor":"9","latestCursor":"10","tail":[" first\r","","partial trailing line"],"truncated":true,"limited":false}},"_meta":{"runtimeId":"runtime"}}`), nil
				}
				return raw, nil
			}})
			nativeResult, got, err := client.ReadTerminal(context.Background(), TerminalRead{Handle: "terminal", Cursor: 7, Limit: 1000})
			badBase := scenario == "bad-tail" || scenario == "malformed" || scenario == "response-lost"
			badCursors := scenario == "bad-next" || scenario == "bad-latest" || scenario == "duplicate-next" || badBase
			if calls != 1 || (err != nil) != badBase || got.Matches("runtime", "terminal") != (!badBase && scenario != "wrong-runtime" && scenario != "wrong-handle") || got.CursorsDecoded() == badCursors || nativeResult.CallUncertain != (scenario == "response-lost") {
				t.Fatal("read changed identity, parse scope, uncertainty or call count", calls, got, nativeResult, err)
			}
			if !badBase {
				if !reflect.DeepEqual(got.Tail, lines) || got.OldestCursor != "5" || (got.Limited == nil) != (scenario == "missing-limited") ||
					got.Limited != nil && *got.Limited != (scenario == "limited") || (got.Truncated == nil) != (scenario == "missing-truncated") || got.Truncated != nil && !*got.Truncated ||
					got.Source != map[bool]string{true: "screen", false: "stream"}[scenario == "screen"] {
					t.Fatal("read normalized raw rows or filled missing page facts", got)
				}
				if !badCursors && (got.NextCursor != "9" || got.LatestCursor != "10") {
					t.Fatal("cursor facts changed", got)
				}
			}
			if scenario == "response-lost" && !errors.Is(err, lost) {
				t.Fatal("read lost original call error", err)
			}
		})
	}
	t.Run("missing-input-zero-invocation", func(t *testing.T) {
		calls := 0
		client := New(Config{Output: func(context.Context, time.Duration, ...string) ([]byte, error) { calls++; return nil, nil }})
		for _, read := range []TerminalRead{{Handle: "terminal"}, {Limit: 1000}, {}} {
			_, page, err := client.ReadTerminal(context.Background(), read)
			if err == nil || calls != 0 || page.Matches("runtime", "terminal") {
				t.Fatal("unbounded or implicit read invoked", calls, page, err)
			}
		}
	})
	t.Run("wire-cannot-inject-parse-proof", func(t *testing.T) {
		var page TerminalReadPage
		if err := json.Unmarshal([]byte(`{"RuntimeID":"runtime","handle":"terminal","decoded":true,"cursorsDecoded":true}`), &page); err != nil {
			t.Fatal(err)
		}
		if page.Matches("runtime", "terminal") || page.CursorsDecoded() || page.RuntimeID != "" {
			t.Fatal("wire injected local parse identity", page)
		}
	})
}
