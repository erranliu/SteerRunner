// Package parsers decodes verified native formats locally. It performs no I/O.
package parsers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/klauspost/compress/zstd"
)

const MaxSource = 64 << 20
const MaxDecoded = 256 << 20
const MaxRecord = 4 << 20
const Version = "native-usage/3"

var errRecordLimit = errors.New("record budget exceeded")

type Command struct {
	ID                  string
	WireSHA256          string
	NativeTurnID        string
	InputID             string
	ReplyID             string
	InputSeq            int64
	EndSeq              int64
	Phase               string
	NotBefore, NotAfter *time.Time
}
type Binding struct {
	RunID, AttemptID, Generation, HostID, Namespace, Agent, SessionID, CWD string
	Commands                                                               []Command
}
type Result struct {
	BindingHash string
	SourceBytes int
	SourceHash  string
	Events      []api.Event
	Issues      []api.Issue
	Missing     int64
	Complete    bool
	HeaderHash  string
	Watermark   string
}

func TextHash(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
func NormalizePath(s string) string {
	for strings.Contains(s, `\\`) {
		s = strings.ReplaceAll(s, `\\`, `\`)
	}
	return strings.ToLower(path.Clean(strings.ReplaceAll(s, `\`, "/")))
}

type object map[string]json.RawMessage

func obj(raw json.RawMessage) object { var o object; _ = json.Unmarshal(raw, &o); return o }
func str(o object, k string) string  { var s string; _ = json.Unmarshal(o[k], &s); return s }
func number(o object, k string) *int64 {
	raw, ok := o[k]
	if !ok || string(raw) == "null" {
		return nil
	}
	var v int64
	if json.Unmarshal(raw, &v) != nil {
		return api.Number(-1)
	}
	return &v
}
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []object
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out strings.Builder
	for _, p := range parts {
		out.WriteString(str(p, "text"))
	}
	return out.String()
}
func timestamp(raw json.RawMessage) *time.Time {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	v = v.UTC()
	return &v
}

func dshTimestamp(raw json.RawMessage) *time.Time {
	var millis int64
	if json.Unmarshal(raw, &millis) != nil || millis <= 0 || millis > 253402300799999 {
		return nil
	}
	value := time.UnixMilli(millis).UTC()
	return &value
}
func token(o object, agent string) api.Tokens {
	var t api.Tokens
	if agent == "codex" {
		t.InputTotal = number(o, "input_tokens")
		t.CacheRead = number(o, "cached_input_tokens")
		t.CacheWrite = number(o, "cache_write_input_tokens")
		t.Output = number(o, "output_tokens")
		t.Reasoning = number(o, "reasoning_output_tokens")
		t.Total = number(o, "total_tokens")
		if t.InputTotal != nil && t.CacheRead != nil && t.CacheWrite != nil && *t.CacheWrite == 0 {
			t.InputUncached = api.Number(*t.InputTotal - *t.CacheRead)
		}
	} else {
		keys := []string{"input", "cacheRead", "cacheWrite", "output", "reasoning", "totalTokens"}
		if agent == "dsh" {
			keys = []string{"inputTokens", "cacheReadTokens", "cacheWriteTokens", "outputTokens", "reasoningTokens", "totalTokens"}
		}
		t.InputUncached = number(o, keys[0])
		t.CacheRead = number(o, keys[1])
		t.CacheWrite = number(o, keys[2])
		t.Output = number(o, keys[3])
		t.Reasoning = number(o, keys[4])
		t.Total = number(o, keys[5])
		if t.InputUncached != nil && t.CacheRead != nil && t.CacheWrite != nil {
			// Validate sums without allowing integer wraparound.
			a, b, c := *t.InputUncached, *t.CacheRead, *t.CacheWrite
			if a >= 0 && b >= 0 && c >= 0 && a <= int64(^uint64(0)>>1)-b && a+b <= int64(^uint64(0)>>1)-c {
				t.InputTotal = api.Number(a + b + c)
			} else {
				t.InputTotal = api.Number(-1)
			}
		}
	}
	return t
}
func Parse(ctx context.Context, b Binding, raw []byte, compressed bool) Result {
	r := Result{Events: []api.Event{}, Issues: []api.Issue{}, Complete: true}
	issue := func(code string) {
		r.Issues = append(r.Issues, api.Issue{Category: "usage", Code: code, Retryable: code == "LOG_TAIL_INCOMPLETE"})
		r.Complete = false
	}
	if len(raw) > MaxSource {
		issue("BUDGET_EXCEEDED")
		return r
	}
	var reader io.Reader = bytes.NewReader(raw)
	if compressed {
		decoder, err := zstd.NewReader(reader, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true), zstd.WithDecoderMaxMemory(MaxDecoded), zstd.WithDecoderMaxWindow(MaxSource))
		if err != nil {
			issue("FORMAT_UNSUPPORTED")
			return r
		}
		defer decoder.Close()
		reader = &verifiedFrames{ctx: ctx, source: raw, decoder: decoder}
	}
	scanner := bufio.NewReaderSize(io.LimitReader(reader, MaxDecoded+1), 64<<10)
	var records []object
	size := 0
	for {
		if ctx.Err() != nil {
			issue("READ_TIMEOUT")
			break
		}
		line, err := readLine(scanner)
		size += len(line)
		if size > MaxDecoded {
			issue("BUDGET_EXCEEDED")
			break
		}
		if err != nil && err != io.EOF {
			if errors.Is(err, errRecordLimit) || errors.Is(err, errDecodeBudget) {
				issue("BUDGET_EXCEEDED")
			} else if errors.Is(err, errInvalidFrame) {
				issue("FORMAT_UNSUPPORTED")
			} else {
				issue("LOG_TAIL_INCOMPLETE")
			}
			break
		}
		if len(line) == 0 && err == io.EOF {
			break
		}
		if len(line) > MaxRecord {
			issue("BUDGET_EXCEEDED")
			break
		}
		if line[len(line)-1] != '\n' {
			issue("LOG_TAIL_INCOMPLETE")
			break
		}
		var record object
		if json.Unmarshal(line, &record) != nil {
			issue("FORMAT_UNSUPPORTED")
			break
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		issue("USAGE_NOT_RECORDED")
		return r
	}
	header := records[0]
	h := header
	expectedType := "session"
	if b.Agent == "codex" {
		expectedType = "session_meta"
		h = obj(header["payload"])
	}
	if str(header, "type") != expectedType || str(h, "id") != b.SessionID || NormalizePath(str(h, "cwd")) != NormalizePath(b.CWD) || b.SessionID == "" || b.CWD == "" {
		issue("IDENTITY_CHANGED")
		return r
	}
	if b.Agent == "pi" && (number(h, "version") == nil || *number(h, "version") != 3) || b.Agent == "dsh" && (number(h, "version") == nil || *number(h, "version") != 4) {
		issue("FORMAT_UNSUPPORTED")
		return r
	}
	if string(h["isSeeded"]) == "true" {
		issue("COVERAGE_INCOMPLETE")
		return r
	}
	hraw, _ := json.Marshal(h)
	r.HeaderHash = TextHash(string(hraw))
	commands := map[string]Command{}
	for _, c := range b.Commands {
		if c.WireSHA256 != "" {
			if _, ok := commands[c.WireSHA256]; ok {
				issue("SESSION_AMBIGUOUS")
				return r
			}
			commands[c.WireSHA256] = c
		}
	}
	matched := map[string]bool{}
	responseOwners := map[string]string{}
	owners := map[string]Command{}
	turnOwners := map[string]Command{}
	turnModels := map[string]string{}
	current := Command{}
	turnID := ""
	model, provider := "", str(h, "model_provider")
	for _, v := range records[1:] {
		if ctx.Err() != nil {
			issue("READ_TIMEOUT")
			break
		}
		kind := str(v, "type")
		p := obj(v["payload"])
		message := obj(v["message"])
		data := obj(v["data"])
		var user, uid string
		var seq int64
		if n := number(v, "seq"); n != nil {
			seq = *n
		}
		switch b.Agent {
		case "codex":
			if kind == "event_msg" && str(p, "type") == "task_started" {
				turnID = str(p, "turn_id")
				delete(turnOwners, turnID)
				delete(turnModels, turnID)
				current = Command{}
			}
			if kind == "turn_context" {
				model = str(p, "model")
				turnID = str(p, "turn_id")
				turnModels[turnID] = model
			}
			if kind == "response_item" && str(p, "role") == "user" {
				user = text(p["content"])
				uid = str(p, "id")
			}
		case "pi":
			if kind == "message" && str(message, "role") == "user" {
				user = text(message["content"])
				uid = str(v, "id")
			}
		case "dsh":
			if kind == "user/message" {
				// DSH persists runtime-context snapshots as role=user. They are
				// native context updates inside the same request, not human input.
				if str(obj(data["source"]), "kind") != "runtime-context" {
					user = text(data["content"])
					uid = str(data, "id")
				}
			}
			if kind == "request/header" {
				config := obj(obj(data["header"])["config"])
				model = str(config, "model")
				provider = str(config, "provider")
			}
			if kind == "request/context" {
				if value := str(data, "model"); value != "" {
					model = value
				}
				if value := str(data, "provider"); value != "" {
					provider = value
				}
			}
		}
		if uid != "" || user != "" {
			c, ok := commands[TextHash(user)]
			current = Command{}
			if ok && commandTimeMatches(c, timestamp(v["timestamp"])) && (c.InputID == "" || c.InputID == uid) && (c.InputSeq == 0 || c.InputSeq == seq) && (c.NativeTurnID == "" || c.NativeTurnID == turnID) {
				current = c
				matched[c.ID] = true
			}
			if b.Agent == "codex" {
				turnOwners[turnID] = current
			}
			if b.Agent == "pi" {
				owners[uid] = current
			}
		}
		if b.Agent == "pi" && kind == "message" && str(message, "role") != "user" {
			current = owners[str(v, "parentId")]
			owners[str(v, "id")] = current
		}
		var usageRaw json.RawMessage
		response := ""
		owner := current
		eventTurn := turnID
		switch b.Agent {
		case "codex":
			if kind != "token_usage_record" {
				continue
			}
			usageRaw = p["usage"]
			response = str(p, "response_id")
			eventTurn = str(p, "turn_id")
			owner = turnOwners[eventTurn]
			model = turnModels[eventTurn]
			if str(p, "session_id") != "" && str(p, "session_id") != b.SessionID {
				issue("IDENTITY_CHANGED")
				continue
			}
		case "pi":
			if kind != "message" || str(message, "role") != "assistant" {
				continue
			}
			usageRaw = message["usage"]
			response = str(v, "id")
			model = str(message, "model")
			provider = str(message, "provider")
		case "dsh":
			if kind != "assistant/message" {
				continue
			}
			usageRaw = data["usage"]
			message = obj(data["message"])
			response = str(message, "id")
			if owner.InputSeq > 0 && seq <= owner.InputSeq || owner.EndSeq > 0 && seq > owner.EndSeq {
				owner = Command{}
			}
		default:
			issue("FORMAT_UNSUPPORTED")
			return r
		}
		if owner.ID == "" {
			issue("COVERAGE_INCOMPLETE")
			continue
		}
		if !commandTimeMatches(owner, timestamp(v["timestamp"])) {
			issue("COVERAGE_INCOMPLETE")
			continue
		}
		if response == "" {
			r.Missing++
			issue("USAGE_INVALID")
			continue
		}
		if len(response) > 512 || len(model) > 256 || len(provider) > 256 {
			r.Missing++
			issue("USAGE_INVALID")
			continue
		}
		t := token(obj(usageRaw), b.Agent)
		if e := usage.Validate(t); e != nil {
			r.Missing++
			issue(e.Error())
			continue
		}
		// Empty native default usage is not positive evidence of a billed response.
		if t.Total != nil && *t.Total == 0 {
			r.Missing++
			issue("USAGE_NOT_RECORDED")
			continue
		}
		if t.InputTotal == nil || t.Output == nil || t.Total == nil {
			r.Missing++
			issue("USAGE_NOT_RECORDED")
		}
		e := api.Event{ID: usage.Identity(b.HostID, b.Namespace, b.Agent, b.SessionID, response), SourceRef: usage.Digest(b.HostID, b.Namespace, b.Agent, b.SessionID), RunID: b.RunID, AttemptID: b.AttemptID, Generation: b.Generation, CommandID: owner.ID, Agent: b.Agent, SessionID: b.SessionID, TurnID: eventTurn, ResponseID: response, Tokens: t, MetadataBasis: "observed", TimeBasis: "completion", ObservedAt: time.Now().UTC(), Format: b.Agent + "/" + string(h["version"]), ParserVersion: Version, EvidenceRef: TextHash(response), MetadataIssues: []api.Issue{}, Decision: "counted"}
		e.Phase = "command"
		if owner.Phase == "initialization" {
			e.CommandID = ""
			e.Phase = "initialization"
		}
		if model != "" {
			m := model
			e.Model = &m
		}
		if provider != "" {
			p := provider
			e.Provider = &p
		}
		e.CallTime = timestamp(v["timestamp"])
		if b.Agent == "dsh" {
			e.CallTime = dshTimestamp(v["time"])
		}
		if e.CallTime == nil {
			e.TimeBasis = "unknown"
		}
		if e.Model == nil || e.Provider == nil || e.CallTime == nil {
			e.MetadataIssues = append(e.MetadataIssues, api.Issue{Category: "metadata", Code: "SOURCE_METADATA_INCOMPLETE"})
		}
		if b.Agent == "codex" {
			e.Format = "codex/token_usage_record.v1"
		}
		recordRaw, _ := json.Marshal(v)
		e.EvidenceRef = TextHash(string(recordRaw))
		r.Events = append(r.Events, e)
		responseOwners[response] = owner.ID
		r.Watermark = e.ID
	}
	for _, c := range b.Commands {
		if !matched[c.ID] {
			issue("COVERAGE_INCOMPLETE")
		}
		if c.ReplyID != "" && responseOwners[c.ReplyID] != c.ID && b.Agent != "codex" {
			issue("COVERAGE_INCOMPLETE")
		}
	}
	if len(r.Events) == 0 && len(r.Issues) == 0 {
		issue("USAGE_NOT_RECORDED")
	}
	return r
}

func commandTimeMatches(c Command, observed *time.Time) bool {
	if c.NotBefore == nil && c.NotAfter == nil {
		return true
	}
	return observed != nil && (c.NotBefore == nil || !observed.Before(*c.NotBefore)) && (c.NotAfter == nil || !observed.After(*c.NotAfter))
}
func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > MaxRecord {
			return nil, errRecordLimit
		}
		line = append(line, part...)
		if err != bufio.ErrBufferFull {
			return line, err
		}
	}
}
