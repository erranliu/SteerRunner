package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
	"github.com/klauspost/compress/zstd"
)

// An independent raw-file oracle, not the production parser or normalization.
// These dedicated real sessions must contain only responses owned by receipts
// in the frozen Run. No prompt text, credentials or raw transcript is saved.
func readLocalTerminalControl(t *testing.T, file string, b parsers.Binding) localNativeControl {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil || len(raw) > 64<<20 {
		t.Fatal("native source read", err)
	}
	sum := sha256.Sum256(raw)
	control := localNativeControl{SHA256: hex.EncodeToString(sum[:]), Bytes: len(raw), Responses: map[string]api.Tokens{}}
	decoded := raw
	if b.Agent == "dsh" {
		decoder, e := zstd.NewReader(bytes.NewReader(raw), zstd.WithDecoderMaxMemory(256<<20))
		if e != nil {
			t.Fatal(e)
		}
		defer decoder.Close()
		decoded, e = io.ReadAll(io.LimitReader(decoder, (256<<20)+1))
		if e != nil || len(decoded) > 256<<20 {
			t.Fatal("native decompression", e)
		}
	}
	commands := map[string]parsers.Command{}
	matched := map[string]bool{}
	replies := map[string]string{}
	owners := map[string]string{}
	for _, c := range b.Commands {
		if c.WireSHA256 == "" || commands[c.WireSHA256].ID != "" {
			t.Fatal("ambiguous receipt")
		}
		commands[c.WireSHA256] = c
	}
	var current string
	var totals [7]int64
	reasoningKnown := true
	headerCount := 0
	scanner := bufio.NewScanner(bytes.NewReader(decoded))
	scanner.Buffer(make([]byte, 65536), 4<<20)
	textContent := func(raw json.RawMessage) string {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		var parts []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &parts) != nil {
			t.Fatal("invalid native content")
		}
		for _, p := range parts {
			s += p.Text
		}
		return s
	}
	for scanner.Scan() {
		var r struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			CWD     string `json:"cwd"`
			Version int    `json:"version"`
			Parent  string `json:"parentId"`
			Seq     int64  `json:"seq"`
			Message struct {
				Role    string                     `json:"role"`
				Content json.RawMessage            `json:"content"`
				Usage   map[string]json.RawMessage `json:"usage"`
			} `json:"message"`
			Data struct {
				ID      string          `json:"id"`
				Content json.RawMessage `json:"content"`
				Source  struct {
					Kind string `json:"kind"`
				} `json:"source"`
				Message struct {
					ID string `json:"id"`
				} `json:"message"`
				Usage map[string]json.RawMessage `json:"usage"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			t.Fatal("invalid native JSONL")
		}
		if r.Type == "session" {
			headerCount++
			wantVersion := 3
			if b.Agent == "dsh" {
				wantVersion = 4
			}
			if r.ID != b.SessionID || parsers.NormalizePath(r.CWD) != parsers.NormalizePath(b.CWD) || r.Version != wantVersion {
				t.Fatal("foreign native session header")
			}
		}
		uid, prompt := "", ""
		if b.Agent == "pi" && r.Type == "message" && r.Message.Role == "user" {
			uid, prompt = r.ID, textContent(r.Message.Content)
		}
		if b.Agent == "dsh" && r.Type == "user/message" && r.Data.Source.Kind != "runtime-context" {
			uid, prompt = r.Data.ID, textContent(r.Data.Content)
		}
		if uid != "" {
			hash := sha256.Sum256([]byte(prompt))
			c, ok := commands[hex.EncodeToString(hash[:])]
			if !ok || c.InputID != "" && c.InputID != uid || c.InputSeq != 0 && c.InputSeq != r.Seq {
				t.Fatal("unowned original user input")
			}
			current = c.ID
			matched[c.ID] = true
			owners[uid] = current
		}
		if b.Agent == "pi" && r.Type == "message" && r.Message.Role != "user" {
			current = owners[r.Parent]
			owners[r.ID] = current
		}
		response := ""
		var usage map[string]json.RawMessage
		if b.Agent == "pi" && r.Type == "message" && r.Message.Role == "assistant" {
			response, usage = r.ID, r.Message.Usage
		}
		if b.Agent == "dsh" && r.Type == "assistant/message" {
			response, usage = r.Data.Message.ID, r.Data.Usage
		}
		isAssistant := b.Agent == "pi" && r.Type == "message" && r.Message.Role == "assistant" || b.Agent == "dsh" && r.Type == "assistant/message"
		if !isAssistant {
			continue
		}
		if usage == nil {
			t.Fatal("native assistant usage missing")
		}
		if current == "" || response == "" {
			t.Fatal("unowned native usage")
		}
		if b.Agent == "dsh" {
			found := false
			for _, c := range b.Commands {
				if c.ID == current {
					found = r.Seq > c.InputSeq && (c.EndSeq == 0 || r.Seq <= c.EndSeq)
				}
			}
			if !found {
				t.Fatal("native usage outside command sequence range")
			}
		}
		keys := []string{"input", "cacheRead", "cacheWrite", "output", "reasoning", "totalTokens"}
		if b.Agent == "dsh" {
			keys = []string{"inputTokens", "cacheReadTokens", "cacheWriteTokens", "outputTokens", "reasoningTokens", "totalTokens"}
		}
		counters := map[string]*int64{}
		for _, key := range keys {
			var value *int64
			if rawValue, ok := usage[key]; ok {
				if json.Unmarshal(rawValue, &value) != nil {
					t.Fatal("invalid native counter", key)
				}
			}
			counters[key] = value
		}
		for _, i := range []int{0, 1, 2, 3, 5} {
			if counters[keys[i]] == nil || *counters[keys[i]] < 0 {
				t.Fatal("native counter unavailable", keys[i])
			}
		}
		input := *counters[keys[0]] + *counters[keys[1]] + *counters[keys[2]]
		if input+*counters[keys[3]] != *counters[keys[5]] {
			t.Fatal("native counter equation mismatch")
		}
		x := api.Tokens{InputTotal: api.Number(input), InputUncached: counters[keys[0]], CacheRead: counters[keys[1]], CacheWrite: counters[keys[2]], Output: counters[keys[3]], Reasoning: counters[keys[4]], Total: counters[keys[5]]}
		if old, ok := control.Responses[response]; ok {
			if !reflect.DeepEqual(old, x) || replies[response] != current {
				t.Fatal("conflicting native response")
			}
			continue
		}
		control.Responses[response] = x
		replies[response] = current
		totals[0] += input
		totals[1] += *x.InputUncached
		totals[2] += *x.CacheRead
		totals[3] += *x.CacheWrite
		totals[4] += *x.Output
		totals[6] += *x.Total
		if x.Reasoning == nil {
			reasoningKnown = false
		} else if *x.Reasoning < 0 {
			t.Fatal("invalid reasoning")
		} else {
			totals[5] += *x.Reasoning
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if headerCount != 1 || len(control.Responses) == 0 || len(matched) != len(b.Commands) {
		t.Fatal("incomplete native identity or command coverage", headerCount, len(matched), len(b.Commands))
	}
	for _, c := range b.Commands {
		if c.ReplyID != "" && replies[c.ReplyID] != c.ID {
			t.Fatal("final receipt response not owned", c.ID)
		}
	}
	control.Commands = replies
	control.Tokens = api.Tokens{InputTotal: api.Number(totals[0]), InputUncached: api.Number(totals[1]), CacheRead: api.Number(totals[2]), CacheWrite: api.Number(totals[3]), Output: api.Number(totals[4]), Total: api.Number(totals[6])}
	if reasoningKnown {
		control.Tokens.Reasoning = api.Number(totals[5])
	}
	return control
}
