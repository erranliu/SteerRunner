package collect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestLocalAppServerUsageCollectsOriginalTurns(t *testing.T) {
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

	run, first, second := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32)
	thread := "00000000-0000-0000-0000-000000000001"
	turnOne := "00000000-0000-0000-0000-000000000002"
	turnTwo := "00000000-0000-0000-0000-000000000003"
	foreign := "00000000-0000-0000-0000-000000000004"
	agent := b.AgentSpec{Name: "codex"}
	r := &runstate.Record{ID: run, Revision: 1, Status: "pr_ready",
		Snapshot: b.LifecycleSnapshot{
			Ref: b.ExecutionRef{Backend: b.BackendLocal, Protocol: b.ProtocolLocalCodexAppServer, RunID: run, AttemptID: "attempt", Agent: agent},
			Session: b.SessionContext{ExecutionID: run, WorkflowRunID: run, AttemptID: "attempt", SessionID: thread, Agent: agent,
				CurrentCommandID: second, CurrentTurnID: turnTwo},
			ResourceGeneration: "generation",
		},
		Workflow: workflow.Snapshot{StageInputs: []workflow.StageInput{
			{CommandID: first, AttemptID: "attempt", StepID: workflow.StepImplement, Model: "executor", ReasoningEffort: "medium"},
			{CommandID: second, AttemptID: "attempt", StepID: workflow.StepAuditCode, Model: "reviewer", ReasoningEffort: "high"},
		}, StageResults: []workflow.StageResult{{CommandID: first, AttemptID: "attempt", SessionID: thread, TurnID: turnOne}}},
	}
	if err := store.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}

	counts := func(input, cached, output, reasoning int) map[string]int {
		return map[string]int{"totalTokens": input + output, "inputTokens": input, "cachedInputTokens": cached,
			"cacheWriteInputTokens": 0, "outputTokens": output, "reasoningOutputTokens": reasoning}
	}
	notification := func(sessionID, turnID string, total, last map[string]int) string {
		raw, err := json.Marshal(map[string]any{"method": "thread/tokenUsage/updated",
			"params": map[string]any{"threadId": sessionID, "turnId": turnID,
				"tokenUsage": map[string]any{"total": total, "last": last}}})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw) + "\n"
	}
	write := func(command, log string) {
		t.Helper()
		path := filepath.Join(dir, "runs", run, "turns", command, "appserver.jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(log), 0600); err != nil {
			t.Fatal(err)
		}
	}
	one, two := counts(100, 60, 20, 5), counts(80, 30, 10, 3)
	update := notification(thread, turnOne, one, one)
	write(first, update+update)
	write(second, notification(foreign, turnTwo, counts(900, 0, 0, 0), counts(900, 0, 0, 0))+
		notification(thread, turnTwo, counts(180, 90, 30, 8), two))

	c := Collector{Store: store, StateDir: dir, Policy: api.DefaultPolicy(), Instance: "writer"}
	authorizedTick(t, &c, ctx)
	snapshot, err := store.GetRunUsage(ctx, run, 0)
	if err != nil || snapshot.Status != "available" || snapshot.Tokens == nil || *snapshot.Tokens.Total != 210 ||
		*snapshot.Tokens.CacheRead != 90 || *snapshot.Tokens.InputUncached != 90 || snapshot.Coverage.CountedResponses != 2 {
		t.Fatalf("unexpected usage snapshot: %+v, %v", snapshot, err)
	}
	page, err := store.ListRunUsageEvents(ctx, run, snapshot.Revision, "", 500)
	if err != nil || len(page.Events) != 2 {
		t.Fatal(page, err)
	}
	byCommand := map[string]api.Event{}
	for _, event := range page.Events {
		byCommand[event.CommandID] = event
		if event.Model != nil || event.MetadataBasis != "unknown" {
			t.Fatal("configured choice fabricated observed model", event)
		}
	}
	if e := byCommand[first]; e.StepID != workflow.StepImplement || e.ConfiguredModel != "executor" || *e.Tokens.Total != 120 {
		t.Fatal("implementation usage attributed incorrectly", e)
	}
	if e := byCommand[second]; e.StepID != workflow.StepAuditCode || e.ConfiguredModel != "reviewer" || e.ConfiguredReasoningEffort != "high" || *e.Tokens.Total != 90 {
		t.Fatal("audit usage mixed with executor", e)
	}
	c.Tick(ctx)
	again, err := store.GetRunUsage(ctx, run, 0)
	if err != nil || again.Revision != snapshot.Revision {
		t.Fatalf("rereading duplicated usage: %+v, %v", again, err)
	}
}
