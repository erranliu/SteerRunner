package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/klauspost/compress/zstd"
)

func TestUsageFinalReplyBelongsToCommand(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			b, _ := fixture(t, agent)
			b.Commands = []Command{
				{ID: "first", WireSHA256: TextHash("first input"), InputID: "u1", ReplyID: "r1"},
				{ID: "second", WireSHA256: TextHash("second input"), InputID: "u2", ReplyID: "r2"},
			}
			var raw string
			if agent == "pi" {
				raw = `{"type":"session","version":3,"id":"fixture-session","cwd":"C:/fixture/work"}
{"type":"message","id":"u1","message":{"role":"user","content":"first input"}}
{"type":"message","id":"r1","parentId":"u1","message":{"role":"assistant","usage":{"input":1,"cacheRead":0,"cacheWrite":0,"output":2,"totalTokens":3}}}
{"type":"message","id":"u2","parentId":"r1","message":{"role":"user","content":"second input"}}
{"type":"message","id":"r2","parentId":"u2","message":{"role":"assistant","usage":{"input":1,"cacheRead":0,"cacheWrite":0,"output":2,"totalTokens":3}}}
`
			} else {
				b.Commands[0].InputSeq, b.Commands[0].EndSeq = 1, 2
				b.Commands[1].InputSeq, b.Commands[1].EndSeq = 3, 4
				raw = `{"type":"session","version":4,"id":"fixture-session","cwd":"C:/fixture/work"}
{"type":"user/message","seq":1,"data":{"id":"u1","content":"first input"}}
{"type":"assistant/message","seq":2,"data":{"message":{"id":"r1"},"usage":{"inputTokens":1,"cacheReadTokens":0,"cacheWriteTokens":0,"outputTokens":2,"totalTokens":3}}}
{"type":"user/message","seq":3,"data":{"id":"u2","content":"second input"}}
{"type":"assistant/message","seq":4,"data":{"message":{"id":"r2"},"usage":{"inputTokens":1,"cacheReadTokens":0,"cacheWriteTokens":0,"outputTokens":2,"totalTokens":3}}}
`
			}
			baseline := Parse(context.Background(), b, []byte(raw), false)
			if !baseline.Complete || len(baseline.Events) != 2 {
				t.Fatal("valid command receipts rejected", baseline)
			}
			// Both IDs exist in the same session, but swapping them must not
			// prove either command's final response or complete Run coverage.
			b.Commands[0].ReplyID, b.Commands[1].ReplyID = "r2", "r1"
			result := Parse(context.Background(), b, []byte(raw), false)
			if result.Complete || len(result.Events) != 2 {
				t.Fatal("foreign command reply accepted or trusted usage lost", result)
			}
			for i, event := range result.Events {
				if event.ResponseID != fmt.Sprintf("r%d", i+1) || event.CommandID != b.Commands[i].ID {
					t.Fatal("response ownership changed", event)
				}
			}
			snapshot := usage.Project(api.Snapshot{RunID: b.RunID, Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}, Issues: result.Issues}, result.Events)
			if snapshot.Status != "partial" || snapshot.Coverage.Finalized || snapshot.Tokens == nil || snapshot.Tokens.Total == nil || *snapshot.Tokens.Total != 6 {
				t.Fatal("receipt gap was hidden", snapshot)
			}
		})
	}
}

func TestUsageInitializationTimeOwnership(t *testing.T) {
	b, raw := fixture(t, "codex")
	start, _ := time.Parse(time.RFC3339Nano, "2026-10-20T03:42:00Z")
	end := start.Add(3 * time.Second)
	b.Commands[1].Phase = "initialization"
	b.Commands[1].NotBefore = &start
	b.Commands[1].NotAfter = &end
	result := Parse(context.Background(), b, raw, false)
	if !result.Complete || len(result.Events) != 7 || result.Events[0].Phase != "initialization" || result.Events[0].CommandID != "" {
		t.Fatal(result)
	}
	// A matching bootstrap from an older session interval is not this Run's
	// initialization, even when every prompt byte and response ID matches.
	later := start.Add(time.Hour)
	b.Commands[1].NotBefore = &later
	result = Parse(context.Background(), b, raw, false)
	if result.Complete || len(result.Events) != 6 {
		t.Fatal("historical initialization attributed", result)
	}
	// The response must also settle inside the original ownership interval.
	b.Commands[1].NotBefore = &start
	beforeResponse := start.Add(2 * time.Second)
	b.Commands[1].NotAfter = &beforeResponse
	result = Parse(context.Background(), b, raw, false)
	if result.Complete || len(result.Events) != 6 {
		t.Fatal("response crossed initialization boundary", result)
	}
}

func fixture(t *testing.T, agent string) (Binding, []byte) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + agent + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	commands, err := os.ReadFile("testdata/" + agent + "-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	b := Binding{RunID: "run", AttemptID: "attempt", Generation: "g", HostID: "host", Namespace: "account", Agent: agent, SessionID: "fixture-session", CWD: `C:\fixture\work`}
	if err = json.Unmarshal(commands, &b.Commands); err != nil {
		t.Fatal(err)
	}
	return b, raw
}
func TestUsageParserNativeFixtures(t *testing.T) {
	for _, tc := range []struct {
		agent        string
		count, total int64
	}{{"codex", 7, 130011}, {"pi", 1, 85}, {"dsh", 1, 6321}} {
		t.Run(tc.agent, func(t *testing.T) {
			b, raw := fixture(t, tc.agent)
			r := Parse(context.Background(), b, raw, false)
			s := usage.Project(api.Snapshot{RunID: b.RunID, Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1}, Issues: r.Issues}, r.Events)
			if !r.Complete || s.Tokens == nil || s.Coverage.CountedResponses != tc.count || s.Tokens.Total == nil || *s.Tokens.Total != tc.total {
				t.Fatalf("result=%+v snapshot=%+v", r, s)
			}
			if tc.agent == "dsh" && s.Tokens.Reasoning != nil {
				t.Fatal("invented reasoning count")
			}
		})
	}
}
func TestUsageParserConcatenatedFramesAndTail(t *testing.T) {
	b, raw := fixture(t, "dsh")
	cut := strings.Index(string(raw), "\n") + 1
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	compressed := append(encoder.EncodeAll(raw[:cut], nil), encoder.EncodeAll(raw[cut:], nil)...)
	r := Parse(context.Background(), b, compressed, true)
	if !r.Complete || len(r.Events) != 1 {
		t.Fatalf("frames: %+v", r)
	}
	r = Parse(context.Background(), b, compressed[:len(compressed)-2], true)
	if r.Complete || len(r.Events) != 0 {
		t.Fatal("truncated frame claimed complete")
	}
}

func TestUsageOwnershipDSHRuntimeContext(t *testing.T) {
	b, raw := fixture(t, "dsh")
	needle := []byte(`{"type":"assistant/message"`)
	contextRecord := []byte("{\"type\":\"user/message\",\"seq\":9,\"data\":{\"id\":\"context\",\"role\":\"user\",\"source\":{\"kind\":\"runtime-context\"},\"content\":[{\"type\":\"text\",\"text\":\"synthetic native context\"}]}}\n")
	index := strings.Index(string(raw), string(needle))
	if index < 0 {
		t.Fatal("fixture missing assistant")
	}
	changed := append(append(append([]byte{}, raw[:index]...), contextRecord...), raw[index:]...)
	result := Parse(context.Background(), b, changed, false)
	if !result.Complete || len(result.Events) != 1 {
		t.Fatal(result)
	}
}

func TestUsageDSHNativeTimeAndSequence(t *testing.T) {
	b, raw := fixture(t, "dsh")
	r := Parse(context.Background(), b, raw, false)
	if len(r.Events) != 1 || r.Events[0].CallTime == nil || r.Events[0].CallTime.UnixMilli() != 1792554364254 || r.Events[0].TimeBasis != "completion" {
		t.Fatal(r)
	}
	b.Commands[2].InputSeq = 15
	r = Parse(context.Background(), b, raw, false)
	if r.Complete || len(r.Events) != 0 {
		t.Fatal("pre-input response counted", r)
	}
}
func TestUsageBindingAndLimits(t *testing.T) {
	b, raw := fixture(t, "pi")
	for _, mutate := range []func(*Binding){func(b *Binding) { b.SessionID = "other" }, func(b *Binding) { b.CWD = `C:\other` }, func(b *Binding) { b.Commands[0].WireSHA256 = "unknown" }} {
		clone := b
		clone.Commands = append([]Command{}, b.Commands...)
		mutate(&clone)
		r := Parse(context.Background(), clone, raw, false)
		if r.Complete || len(r.Events) != 0 {
			t.Fatal("unverified ownership counted")
		}
	}
	r := Parse(context.Background(), b, append(raw, []byte(`{"type":`)...), false)
	if r.Complete || len(r.Events) != 1 {
		t.Fatal("complete prefix not retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Parse(ctx, b, raw, false).Complete {
		t.Fatal("cancel ignored")
	}
}
func TestUsageNormalization(t *testing.T) {
	for _, tokens := range []api.Tokens{{Total: api.Number(-1)}, {InputTotal: api.Number(2), Output: api.Number(3), Total: api.Number(6)}, {Output: api.Number(2), Reasoning: api.Number(3)}, {InputUncached: api.Number(9223372036854775807), CacheRead: api.Number(1), CacheWrite: api.Number(0), InputTotal: api.Number(0)}} {
		if usage.Validate(tokens) == nil {
			t.Fatalf("accepted invalid tokens %+v", tokens)
		}
	}
	o := obj(json.RawMessage(`{"input":10,"cacheRead":20,"cacheWrite":3,"output":4,"reasoning":2,"totalTokens":37}`))
	v := token(o, "pi")
	if usage.Validate(v) != nil || *v.InputTotal != 33 || *v.Total != 37 {
		t.Fatal(v)
	}
	c := token(obj(json.RawMessage(`{"input_tokens":10,"cached_input_tokens":2,"cache_write_input_tokens":3,"output_tokens":1,"total_tokens":11}`)), "codex")
	if c.InputUncached != nil {
		t.Fatal("unverified cache-write split invented")
	}
}

func TestUsageMissingPrimaryCounterIsPartial(t *testing.T) {
	b, raw := fixture(t, "pi")
	changed := strings.Replace(string(raw), `"output":10,`, "", 1)
	if changed == string(raw) {
		t.Fatal("fixture output shape changed")
	}
	result := Parse(context.Background(), b, []byte(changed), false)
	if result.Complete || result.Missing == 0 || len(result.Events) != 1 || result.Events[0].Tokens.Output != nil {
		t.Fatal(result)
	}
}
