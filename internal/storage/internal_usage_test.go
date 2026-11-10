package storage

import (
	"context"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestInternalUsageSharesLedgerWithoutChangingMainSession(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	main, err := s.CommitUsage(ctx, summary(), []api.Event{event("main")})
	if err != nil {
		t.Fatal(err)
	}
	call := api.Event{ID: "parser-call", RunID: main.RunID, Scope: "runner_internal", CallID: "call-1", CallSource: "llm_parser", SourceRef: "parser-source", SourceResultID: "audit-result", AttemptID: "attempt", Generation: "generation", CommandID: "audit-command", StepID: "audit.code.parse", ConfiguredModel: "parser-model", CoverageStatus: "pending"}
	pending, err := s.CommitUsage(ctx, main, []api.Event{call})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pending.Tokens, main.Tokens) || pending.Coverage.CountedResponses != main.Coverage.CountedResponses || pending.ModelCallCoverage.InternalPending != 1 {
		t.Fatal("pending parser changed main usage", pending)
	}
	call.CoverageStatus = "covered"
	call.SessionID = "parser-session"
	call.ResponseID = "parser-final"
	call.Tokens = api.Tokens{InputTotal: api.Number(20), Output: api.Number(3), Total: api.Number(23)}
	observed, err := s.CommitUsage(ctx, pending, []api.Event{call, call})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(observed.Tokens, main.Tokens) || observed.ModelCallCoverage.InternalCovered != 1 || observed.ModelCallCoverage.InternalPending != 0 || observed.ModelCallCoverage.InternalRecorded != 1 {
		t.Fatal("parser double counted or entered main total", observed)
	}
	old, err := s.ListRunUsageEvents(ctx, main.RunID, pending.Revision, "", 500)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range old.Events {
		if e.ID == call.ID && e.CoverageStatus != "pending" {
			t.Fatal("immutable snapshot overwritten")
		}
	}
	unknown := call
	unknown.ID = "unknown-call"
	unknown.CallID = "call-2"
	unknown.SessionID = ""
	unknown.ResponseID = ""
	unknown.Tokens = api.Tokens{}
	unknown.CoverageStatus = "uncovered"
	final, err := s.CommitUsage(ctx, observed, []api.Event{unknown})
	if err != nil || final.ModelCallCoverage.InternalUncovered != 1 || !reflect.DeepEqual(final.Tokens, main.Tokens) {
		t.Fatal("unknown parser represented as zero or lost", final, err)
	}
	page, err := s.ListRunUsageEvents(ctx, main.RunID, final.Revision, "", 500)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range page.Events {
		if e.ID == unknown.ID && (e.Decision != "uncovered" || e.Tokens.Total != nil) {
			t.Fatal("unknown counted", e)
		}
	}
}
