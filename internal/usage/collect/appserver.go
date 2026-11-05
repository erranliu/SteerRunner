package collect

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

const appServerUsageFormat = "codex/thread-token-usage-updated.v1"

type appServerTurn struct {
	Command string
	Turn    string
}

func appServerTurns(r *runstate.Record) ([]appServerTurn, error) {
	s := r.Snapshot.Session
	if !journalID(r.ID) || s.ExecutionID != r.ID || s.WorkflowRunID != r.ID || s.AttemptID == "" ||
		s.AttemptID != r.Snapshot.Ref.AttemptID || s.Agent.Name != "codex" {
		return nil, errors.New("IDENTITY_UNVERIFIED")
	}
	if s.SessionID == "" {
		return nil, errors.New("SESSION_UNBOUND")
	}
	if !codex.ValidNativeID(s.SessionID) {
		return nil, errors.New("IDENTITY_UNVERIFIED")
	}
	var turns []appServerTurn
	seen := map[string]string{}
	add := func(command, turn string) bool {
		if !journalID(command) || !codex.ValidNativeID(turn) {
			return false
		}
		if original, ok := seen[command]; ok {
			return original == turn
		}
		seen[command] = turn
		turns = append(turns, appServerTurn{Command: command, Turn: turn})
		return true
	}
	for _, result := range r.Workflow.StageResults {
		if result.AttemptID == s.AttemptID && result.SessionID == s.SessionID && !add(result.CommandID, result.TurnID) {
			return nil, errors.New("IDENTITY_UNVERIFIED")
		}
	}
	if s.CurrentTurnID != "" && !add(s.CurrentCommandID, s.CurrentTurnID) {
		return nil, errors.New("IDENTITY_UNVERIFIED")
	}
	if len(turns) == 0 {
		return nil, errors.New("SESSION_UNBOUND")
	}
	return turns, nil
}

func journalID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 32 && err == nil
}

type appServerTokenCounts struct {
	Total     *int64 `json:"totalTokens"`
	Input     *int64 `json:"inputTokens"`
	Cached    *int64 `json:"cachedInputTokens"`
	Written   *int64 `json:"cacheWriteInputTokens"`
	Output    *int64 `json:"outputTokens"`
	Reasoning *int64 `json:"reasoningOutputTokens"`
}

func (c appServerTokenCounts) tokens() (api.Tokens, bool) {
	t := api.Tokens{Total: c.Total, InputTotal: c.Input, CacheRead: c.Cached, CacheWrite: c.Written, Output: c.Output, Reasoning: c.Reasoning}
	if c.Total == nil || c.Input == nil || c.Output == nil || usage.Validate(t) != nil {
		return api.Tokens{}, false
	}
	if c.Cached != nil && c.Written != nil {
		t.InputUncached = api.Number(*c.Input - *c.Cached - *c.Written)
	}
	return t, true
}

// Every notification reports a cumulative thread position and the last response.
// Repeated notifications share a position. A gap in cumulative positions does
// not establish the tokens of the missing responses.
func (c appServerTokenCounts) includes(previous, last appServerTokenCounts) (valid, contiguous bool) {
	contiguous = true
	currentFields := []*int64{c.Total, c.Input, c.Cached, c.Written, c.Output, c.Reasoning}
	previousFields := []*int64{previous.Total, previous.Input, previous.Cached, previous.Written, previous.Output, previous.Reasoning}
	lastFields := []*int64{last.Total, last.Input, last.Cached, last.Written, last.Output, last.Reasoning}
	for i, current := range currentFields {
		if current == nil || lastFields[i] == nil || previous.Total != nil && previousFields[i] == nil {
			continue
		}
		delta := *current
		if previousFields[i] != nil {
			delta -= *previousFields[i]
		}
		if delta < *lastFields[i] {
			return false, false
		}
		contiguous = contiguous && delta == *lastFields[i]
	}
	return true, contiguous
}

func readAppServerUsage(ctx context.Context, dir string, r *runstate.Record, turns []appServerTurn) (parsers.Result, error) {
	result := parsers.Result{Complete: true}
	session := r.Snapshot.Session
	observed := time.Now().UTC()
	var prior appServerTokenCounts
	positions := map[int64]appServerTokenCounts{}
	issues := map[string]bool{}
	addIssue := func(code string, retryable bool) {
		result.Complete = false
		if !issues[code] {
			issues[code] = true
			result.Issues = append(result.Issues, api.Issue{Category: "usage", Code: code, Retryable: retryable})
		}
	}
	remaining := int64(parsers.MaxSource)
	for _, turn := range turns {
		path := filepath.Join(dir, "runs", r.ID, "turns", turn.Command, "appserver.jsonl")
		err := scanAppServerJournal(ctx, path, &remaining, func(event codexapp.Envelope) {
			if event.Method != "thread/tokenUsage/updated" || len(event.ID) != 0 {
				return
			}
			var update struct {
				ThreadID   string `json:"threadId"`
				TurnID     string `json:"turnId"`
				TokenUsage struct {
					Total appServerTokenCounts `json:"total"`
					Last  appServerTokenCounts `json:"last"`
				} `json:"tokenUsage"`
			}
			if json.Unmarshal(event.Params, &update) != nil {
				addIssue("USAGE_INVALID", false)
				return
			}
			if update.ThreadID != session.SessionID || update.TurnID != turn.Turn {
				return
			}
			total, last := update.TokenUsage.Total, update.TokenUsage.Last
			tokens, validLast := last.tokens()
			if _, validTotal := total.tokens(); !validTotal || !validLast {
				addIssue("USAGE_INVALID", false)
				return
			}
			position := *total.Total
			if seen, ok := positions[position]; ok {
				if !reflect.DeepEqual(total, seen) {
					addIssue("USAGE_CONFLICT", false)
				}
				return
			}
			valid, contiguous := total.includes(prior, last)
			if !valid {
				addIssue("USAGE_INVALID", false)
				return
			}
			if !contiguous {
				addIssue("COVERAGE_INCOMPLETE", true)
			}
			prior, positions[position] = total, total
			// This stream has no response ID, observed model, provider or call time.
			// Use its stable position for deduplication and leave pricing metadata unknown.
			response := "appserver:" + strconv.FormatInt(position, 10)
			id := usage.Identity("local", appServerUsageFormat, "codex", session.SessionID, response)
			result.Events = append(result.Events, api.Event{
				ID: id, RunID: r.ID, AttemptID: session.AttemptID, Generation: r.Snapshot.ResourceGeneration,
				CommandID: turn.Command, Agent: "codex", SessionID: session.SessionID, TurnID: turn.Turn,
				ResponseID: response, Tokens: tokens, ObservedAt: observed, Format: appServerUsageFormat,
				ParserVersion: "1", EvidenceRef: id, MetadataBasis: "unknown", TimeBasis: "unknown",
			})
			result.Watermark = id
		})
		if ctx.Err() != nil {
			return parsers.Result{}, ctx.Err()
		}
		if err != nil {
			addIssue(errorCode(err), true)
		}
		if remaining <= 0 {
			break
		}
	}
	if len(result.Events) > 0 {
		result.HeaderHash = usage.Digest(appServerUsageFormat, session.SessionID)
	} else if len(result.Issues) == 0 {
		addIssue("SESSION_UNBOUND", true)
	}
	return result, nil
}

func scanAppServerJournal(ctx context.Context, path string, remaining *int64, visit func(codexapp.Envelope)) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return errors.New("LOG_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("SOURCE_PATH_UNVERIFIED")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.New("SOURCE_CHANGED")
	}
	limit := min(info.Size(), *remaining)
	*remaining -= limit
	reader := bufio.NewReader(io.LimitReader(file, limit))
	var line []byte
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		part, more, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(line)+len(part) > parsers.MaxRecord {
			return errors.New("BUDGET_EXCEEDED")
		}
		line = append(line, part...)
		if more {
			continue
		}
		var event codexapp.Envelope
		if json.Unmarshal(line, &event) != nil {
			return errors.New("LOG_TAIL_INCOMPLETE")
		}
		visit(event)
		line = line[:0]
	}
	if limit < info.Size() || *remaining == 0 {
		return errors.New("BUDGET_EXCEEDED")
	}
	return nil
}
