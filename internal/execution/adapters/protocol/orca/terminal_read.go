package orca

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type TerminalRead struct {
	Handle string
	Cursor uint64
	Limit  uint
}

// TerminalReadPage preserves one raw page. Cursor interpretation, retained
// windows and receipt attribution belong to the caller, as do identity fences.
type TerminalReadPage struct {
	RuntimeID                                              string `json:"-"`
	Handle, Source, OldestCursor, NextCursor, LatestCursor string
	Tail                                                   []string
	Truncated, Limited                                     *bool
	decoded, cursorsDecoded                                bool
}

func (p TerminalReadPage) Matches(runtime, handle string) bool {
	return p.decoded && runtime != "" && handle != "" && p.RuntimeID == runtime && p.Handle == handle
}

// CursorsDecoded checks the complete page's field types, not cursor presence or
// validity. Callers retain their original from-start versus paging requirements.
func (p TerminalReadPage) CursorsDecoded() bool { return p.cursorsDecoded }

func (c Client) ReadTerminal(ctx context.Context, read TerminalRead) (Result, TerminalReadPage, error) {
	if read.Handle == "" || read.Limit == 0 {
		return Result{}, TerminalReadPage{}, errors.New("Orca terminal read requires an explicit terminal and positive limit")
	}
	result, callErr := c.MetaCall(ctx, "terminal", "read", "--terminal", read.Handle, "--cursor", strconv.FormatUint(read.Cursor, 10), "--limit", strconv.FormatUint(uint64(read.Limit), 10), "--json")
	var full struct{ Terminal TerminalReadPage }
	fullErr := json.Unmarshal(result.Payload, &full)
	var base struct {
		Terminal struct {
			Handle, Source, OldestCursor string
			Tail                         []string
			Truncated, Limited           *bool
		}
	}
	err := json.Unmarshal(result.Payload, &base)
	page := full.Terminal
	page.Handle, page.Source, page.OldestCursor = base.Terminal.Handle, base.Terminal.Source, base.Terminal.OldestCursor
	page.Tail, page.Truncated, page.Limited = base.Terminal.Tail, base.Terminal.Truncated, base.Terminal.Limited
	page.RuntimeID, page.decoded, page.cursorsDecoded = result.RuntimeID, err == nil, fullErr == nil
	return result, page, errors.Join(callErr, err)
}
