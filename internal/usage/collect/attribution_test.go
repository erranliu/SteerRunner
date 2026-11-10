package collect

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestUsageAttributionRequiresExactFrozenCommand(t *testing.T) {
	r := &runstate.Record{Workflow: workflow.Snapshot{StageInputs: []workflow.StageInput{
		{CommandID: "writer", AttemptID: "attempt", StepID: workflow.StepImplement, Model: "executor"},
		{CommandID: "audit", AttemptID: "attempt", StepID: workflow.StepAuditCode, Model: "reviewer", ReasoningEffort: "high"},
	}}}
	observed := "provider-alias"
	for _, tc := range []struct{ command, attempt, phase, want string }{
		{"writer", "attempt", "command", "executor"},
		{"audit", "attempt", "command", "reviewer"},
		{"audit", "other-attempt", "command", ""},
		{"foreign", "attempt", "command", ""},
		{"writer", "attempt", "initialization", ""},
	} {
		event := api.Event{CommandID: tc.command, AttemptID: tc.attempt, Phase: tc.phase, Model: &observed, MetadataBasis: "observed"}
		attributeUsage(r, &event)
		if event.ConfiguredModel != tc.want || event.Model == nil || *event.Model != observed || event.MetadataBasis != "observed" {
			t.Fatal("configured and observed identities mixed", tc, event)
		}
	}
	r.Workflow.StageInputs = append(r.Workflow.StageInputs, r.Workflow.StageInputs[0])
	event := api.Event{CommandID: "writer", AttemptID: "attempt"}
	attributeUsage(r, &event)
	if event.ConfiguredModel != "" {
		t.Fatal("ambiguous command was attributed", event)
	}
}

func TestUsageCachedAttributionDoesNotReopenLogsOrRewriteSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.InitUsage(); err != nil {
		t.Fatal(err)
	}
	run, command := strings.Repeat("a", 32), strings.Repeat("b", 32)
	thread, turn := "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"
	agent := b.AgentSpec{Name: "codex"}
	r := &runstate.Record{ID: run, Revision: 1, Status: "stopped", Snapshot: b.LifecycleSnapshot{
		Ref:     b.ExecutionRef{Backend: b.BackendLocal, Protocol: b.ProtocolLocalCodexAppServer, RunID: run, AttemptID: "attempt", Agent: agent},
		Session: b.SessionContext{ExecutionID: run, WorkflowRunID: run, AttemptID: "attempt", Agent: agent, SessionID: thread, CurrentCommandID: command, CurrentTurnID: turn}, ResourceGeneration: "generation",
	}, Workflow: workflow.Snapshot{StageInputs: []workflow.StageInput{{CommandID: command, AttemptID: "attempt", StepID: workflow.StepAuditCode, Model: "reviewer"}}}}
	if err := store.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}
	source := usage.Digest(run, "attempt:generation")
	summary := api.SourceSummary{SourceRef: source, Status: "verified", Finalized: true}
	turns, _ := json.Marshal([]appServerTurn{{Command: command, Turn: turn}})
	cp := checkpoint{Finalized: true, Summary: summary, Input: usage.Digest(appServerUsageFormat, thread, string(turns))}
	if err := store.PutUsageJournal(ctx, "source:"+source, cp); err != nil {
		t.Fatal(err)
	}
	old, err := store.CommitUsage(ctx, api.Snapshot{RunID: run, AccessCleanupStatus: "not_required", Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}}, []api.Event{{ID: "event", RunID: run, SourceRef: source, AttemptID: "attempt", CommandID: command, SessionID: thread, TurnID: turn, ResponseID: "response", Decision: "counted", MetadataBasis: "unknown", Tokens: api.Tokens{InputTotal: api.Number(10), Output: api.Number(2), Total: api.Number(12)}}})
	if err != nil {
		t.Fatal(err)
	}
	old, err = store.CommitUsage(ctx, old, []api.Event{{ID: "internal-event", RunID: run, Scope: "runner_internal", CallID: "parser-call", CallSource: "llm_parser", SourceResultID: "original-audit-result", CoverageStatus: "covered", SourceRef: "internal-source", AttemptID: "attempt", CommandID: command, StepID: "audit.code.parse", ConfiguredModel: "parser-model", SessionID: "parser-session", ResponseID: "parser-response", Tokens: api.Tokens{InputTotal: api.Number(20), Output: api.Number(3), Total: api.Number(23)}}})
	if err != nil {
		t.Fatal(err)
	}
	c := Collector{Store: store, StateDir: dir, Policy: api.DefaultPolicy(), Instance: "writer"}
	authorizedTick(t, &c, ctx)
	latest, err := store.GetRunUsage(ctx, run, 0)
	if err != nil || latest.Status != "available" || *latest.Tokens.Total != 12 {
		t.Fatal(latest, err)
	}
	page, err := store.ListRunUsageEvents(ctx, run, latest.Revision, "", 500)
	if err != nil || len(page.Events) != 2 || page.Events[0].ConfiguredModel != "reviewer" || page.Events[1].ConfiguredModel != "parser-model" || page.Events[1].StepID != "audit.code.parse" || page.Events[1].Decision != "counted" || latest.ModelCallCoverage.InternalCovered != 1 {
		t.Fatal(page, err)
	}
	history, err := store.ListRunUsageEvents(ctx, run, old.Revision, "", 500)
	if err != nil || history.Events[0].ConfiguredModel != "" {
		t.Fatal("immutable snapshot changed", history, err)
	}
	c.Tick(ctx)
	again, err := store.GetRunUsage(ctx, run, 0)
	if err != nil || again.Revision != latest.Revision {
		t.Fatal("repeated enrichment created a revision", again, err)
	}
}
