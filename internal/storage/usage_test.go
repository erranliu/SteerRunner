package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/cost"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

func TestUsageStopFencesLateObservation(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	permit := api.CollectionPermit{Instance: "i", PolicyDigest: "p", Active: true}
	if err := s.SetUsagePermit(ctx, permit); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitObservedUsage(ctx, summary(), []api.Event{event("early")}, 1, nil, permit); err != nil {
		t.Fatal(err)
	}
	input := contracts.Request{Protocol: contracts.Protocol, RequestID: "stop", Input: json.RawMessage(`{"expected_instance_id":"i"}`)}
	response, act, err := s.AcceptStop(input, "i")
	if err != nil || !response.OK || !act {
		t.Fatal(response, act, err)
	}
	if !errors.Is(s.CheckUsagePermit(ctx, "i", "p"), api.ErrCollectionRevoked) {
		t.Fatal("accepted stop did not revoke permit")
	}
	if _, err = s.CommitObservedUsage(ctx, summary(), []api.Event{event("late")}, 1, map[string]any{"late-checkpoint": true}, permit); !errors.Is(err, api.ErrCollectionRevoked) {
		t.Fatal("late commit escaped stop", err)
	}
	snapshot, err := s.GetRunUsage(ctx, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.ListRunUsageEvents(ctx, "r", snapshot.Revision, "", 10)
	if err != nil || len(page.Events) != 1 {
		t.Fatal(page, err)
	}
	var checkpoint bool
	if s.UsageJournal(ctx, "late-checkpoint", &checkpoint) == nil {
		t.Fatal("late checkpoint committed")
	}
	permit.Instance = "replacement"
	if err = s.SetUsagePermit(ctx, permit); err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeUsagePermit(ctx, "i"); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckUsagePermit(ctx, "replacement", "p"); err != nil {
		t.Fatal("old cleanup revoked new service", err)
	}
	old := permit
	old.Instance = "i"
	if _, err = s.CommitObservedUsage(ctx, summary(), []api.Event{event("stale")}, 1, nil, old); !errors.Is(err, api.ErrCollectionRevoked) {
		t.Fatal("old instance committed", err)
	}
}

func TestUsageInitialProjectionDoesNotInventEnabledWait(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	if err := s.EnsureUsage(ctx, "r", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetRunUsage(ctx, "r", 0)
	if err != nil || v.Status != "unavailable" || v.Tokens != nil || v.Coverage.Finalized {
		t.Fatal(v, err)
	}
	found := false
	for _, issue := range v.Issues {
		found = found || issue.Code == "COLLECTION_NOT_OBSERVED"
	}
	if !found {
		t.Fatal(v.Issues)
	}
	before := v
	if err = s.EnsureUsage(ctx, "r", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	v, err = s.GetRunUsage(ctx, "r", 0)
	if err != nil || v.Revision != before.Revision || !v.ObservedAt.Equal(before.ObservedAt) {
		t.Fatal("query target publication refreshed snapshot", v, err)
	}
}

func usageStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.InitUsage(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"r", "other"} {
		if err = s.Commit(&runstate.Record{ID: id, Revision: 1}, 0, nil); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func event(id string) api.Event {
	return api.Event{ID: usage.Identity("host", "account", "pi", "session", id), RunID: "r", SourceRef: "source", SessionID: "session", ResponseID: id, Tokens: api.Tokens{InputTotal: api.Number(10), InputUncached: api.Number(10), CacheRead: api.Number(0), CacheWrite: api.Number(0), Output: api.Number(2), Total: api.Number(12)}, ObservedAt: time.Now().UTC(), MetadataBasis: "observed", Decision: "counted"}
}
func summary() api.Snapshot {
	return api.Snapshot{RunID: "r", AccessCleanupStatus: "not_required", Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}}
}
func TestUsageSnapshotRevision(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	first, err := s.CommitUsage(ctx, summary(), []api.Event{event("a"), event("b")})
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.CommitUsage(ctx, summary(), []api.Event{event("a")})
	if err != nil || same.Revision != first.Revision {
		t.Fatal("replay created revision", same, err)
	}
	page, err := s.ListRunUsageEvents(ctx, "r", first.Revision, "", 1)
	if err != nil || len(page.Events) != 1 || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	conflict := event("a")
	conflict.Tokens.Total = api.Number(13)
	conflict.Tokens.Output = api.Number(3)
	second, err := s.CommitUsage(ctx, summary(), []api.Event{conflict})
	if err != nil || second.Revision <= first.Revision || second.Status != "partial" || *second.Tokens.Total != 12 {
		t.Fatal(second, err)
	}
	old, err := s.GetRunUsage(ctx, "r", first.Revision)
	if err != nil || *old.Tokens.Total != 24 {
		t.Fatal("historical summary mutated", old, err)
	}
	oldPage, err := s.ListRunUsageEvents(ctx, "r", first.Revision, page.NextCursor, 1)
	if err != nil || oldPage.Events[0].Decision != "counted" {
		t.Fatal("historical page mutated", oldPage, err)
	}
	if _, err = s.ListRunUsageEvents(ctx, "r", second.Revision, page.NextCursor, 1); !errors.Is(err, api.ErrCursor) {
		t.Fatal("cursor crossed revision", err)
	}
	if _, err = s.GetRunUsage(ctx, "r", 999); !errors.Is(err, api.ErrSnapshotNotFound) {
		t.Fatal(err)
	}
	runs, err := s.Runs()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Revision != 1 {
			t.Fatal("usage changed business revision")
		}
	}
}
func TestUsageIsolationAndConflict(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	e := event("a")
	model := "m1"
	e.Model = &model
	v1, err := s.CommitUsage(ctx, summary(), []api.Event{e})
	if err != nil {
		t.Fatal(err)
	}
	model2 := "m2"
	e.Model = &model2
	v2, err := s.CommitUsage(ctx, summary(), []api.Event{e})
	if err != nil || v2.Status != "available" || *v2.Tokens.Total != 12 || v2.Revision == v1.Revision {
		t.Fatal(v2, err)
	}
	page, _ := s.ListRunUsageEvents(ctx, "r", v2.Revision, "", 10)
	if len(page.Events[0].MetadataIssues) != 1 {
		t.Fatal("metadata dispute lost")
	}
	e.RunID = "other"
	other := summary()
	other.RunID = "other"
	_, err = s.CommitUsage(ctx, other, []api.Event{e})
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.GetRunUsage(ctx, "r", 0)
	if err != nil || after.Status != "unavailable" || after.Tokens != nil {
		t.Fatal("last trustworthy event was not quarantined", after, err)
	}
}

func TestUsageConflictEvidenceIsAtomicAndDurable(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	first := event("conflict")
	first.EvidenceRef = "original-record-digest"
	v1, err := s.CommitUsage(ctx, summary(), []api.Event{first})
	if err != nil {
		t.Fatal(err)
	}
	conflict := first
	conflict.EvidenceRef = "conflicting-record-digest"
	conflict.Tokens.Output, conflict.Tokens.Total = api.Number(3), api.Number(13)
	// A later journal failure must roll back both the conflict audit and the
	// event quarantine, not leave one side of the evidence committed.
	if _, err = s.CommitUsage(ctx, summary(), []api.Event{conflict}, map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("invalid checkpoint committed")
	}
	keys, err := s.UsageJournalKeys(ctx, "event-conflict:")
	if err != nil || len(keys) != 0 {
		t.Fatal("conflict evidence escaped rollback", keys, err)
	}
	unchanged, err := s.GetRunUsage(ctx, "r", 0)
	if err != nil || unchanged.Revision != v1.Revision || unchanged.Status != "available" {
		t.Fatal("quarantine escaped rollback", unchanged, err)
	}
	v2, err := s.CommitUsage(ctx, summary(), []api.Event{conflict})
	if err != nil || v2.Status != "unavailable" || v2.Tokens != nil {
		t.Fatal(v2, err)
	}
	keys, err = s.UsageJournalKeys(ctx, "event-conflict:")
	if err != nil || len(keys) != 1 {
		t.Fatal("both source claims must survive quarantine", keys, err)
	}
	var proof struct{ Prior, Observed api.Event }
	if err = s.UsageJournal(ctx, keys[0], &proof); err != nil || proof.Prior.EvidenceRef != first.EvidenceRef || proof.Observed.EvidenceRef != conflict.EvidenceRef || *proof.Prior.Tokens.Total != 12 || *proof.Observed.Tokens.Total != 13 {
		t.Fatal("source claims were lost", proof, err)
	}
	conflict.ObservedAt = conflict.ObservedAt.Add(time.Hour)
	again, err := s.CommitUsage(ctx, summary(), []api.Event{conflict})
	if err != nil || again.Revision != v2.Revision {
		t.Fatal("conflict replay changed revision", again, err)
	}
	keys, err = s.UsageJournalKeys(ctx, "event-conflict:")
	if err != nil || len(keys) != 1 {
		t.Fatal("conflict replay duplicated evidence", keys, err)
	}
	old, err := s.ListRunUsageEvents(ctx, "r", v1.Revision, "", 10)
	if err != nil || old.Events[0].Decision != "counted" || old.Events[0].EvidenceRef != first.EvidenceRef {
		t.Fatal("historical evidence was rewritten", old, err)
	}
}

func TestUsageCrossRunConflictSurvivesCachedObservation(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	first := event("shared")
	if _, err := s.CommitUsage(ctx, summary(), []api.Event{first}); err != nil {
		t.Fatal(err)
	}
	other := summary()
	other.RunID = "other"
	valid := event("independent")
	valid.RunID, valid.SourceRef = "other", "other-source"
	provider, model := "fixture-provider", "fixture-model"
	now := time.Now().UTC()
	valid.Provider, valid.Model, valid.CallTime, valid.TimeBasis = &provider, &model, &now, "request"
	before, err := s.CommitUsage(ctx, other, []api.Event{valid})
	if err != nil {
		t.Fatal(err)
	}
	claim := first
	claim.RunID, claim.SourceRef = "other", "other-source"
	conflicted, err := s.CommitUsage(ctx, other, []api.Event{claim})
	if err != nil || conflicted.Status != "partial" || conflicted.Coverage.Finalized {
		t.Fatal(conflicted, err)
	}
	// A finalized collector checkpoint can publish coverage without re-reading
	// events, including after a service restart. The rejected claimant has no
	// row in usage_events; its durable conflict must still qualify this Run.
	again, err := s.CommitUsage(ctx, other, nil)
	if err != nil || again.Status != "partial" || again.Coverage.Finalized || again.Revision != conflicted.Revision {
		t.Fatal("cached observation erased the ownership conflict", again, err)
	}
	found := false
	for _, issue := range again.Issues {
		found = found || issue.Code == "USAGE_CONFLICT" && issue.SourceRef == claim.SourceRef
	}
	if !found {
		t.Fatal("claimant source lost its conflict reason", again.Issues)
	}
	policy := cost.Policy{Schema: "run-pricing-policy/v1", ID: "fixture", Version: "1", Source: "synthetic", Currency: "USD", Enabled: true, Rules: []cost.Rule{{Provider: provider, Model: model, From: now.Add(-time.Hour), Until: now.Add(time.Hour), Rates: cost.Rates{Input: "1", CacheRead: "0", CacheWrite: "0", Output: "2"}}}}
	estimate, err := cost.EstimateRun(ctx, s, "other", again.Revision, policy)
	if err != nil || estimate.Status != "partial" || estimate.Total != nil || estimate.Covered == nil {
		t.Fatal("ownership conflict allowed a complete cost estimate", estimate, err)
	}
	old, err := s.GetRunUsage(ctx, "other", before.Revision)
	if err != nil || old.Status != "available" || !old.Coverage.Finalized {
		t.Fatal("conflict rewrote historical coverage", old, err)
	}
}

func TestUsageTransactionRollback(t *testing.T) {
	s := usageStore(t)
	ctx := context.Background()
	_, err := s.CommitUsage(ctx, summary(), []api.Event{event("ok"), {ID: "invalid"}}, map[string]any{"checkpoint": true})
	if err == nil {
		t.Fatal("invalid write accepted")
	}
	if _, err = s.GetRunUsage(ctx, "r", 0); !errors.Is(err, api.ErrSnapshotNotFound) {
		t.Fatal("partial snapshot escaped transaction", err)
	}
	var count int
	if err = s.db.QueryRow(`SELECT count(*) FROM usage_events`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial events escaped", count, err)
	}
}

func TestUsageReadOnlyLifecycleProjection(t *testing.T) {
	s := usageStore(t)
	now := time.Now().UTC()
	r := &runstate.Record{ID: "r", Revision: 2, Status: "blocked", StopStatus: "stopped", CreatedAt: now, LaunchCommand: "literal original launch"}
	if err := s.Commit(r, 1, nil); err != nil {
		t.Fatal(err)
	}
	runs, err := s.UsageRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.ID == "r" && (r.StopStatus != "stopped" || !r.CreatedAt.Equal(now) || r.LaunchCommand != "literal original launch") {
			t.Fatalf("lifecycle projection dropped facts: %+v", r)
		}
	}
}

func TestUsageStructuredProjectionRetainsOnlyOriginalInputAndEndFacts(t *testing.T) {
	s := usageStore(t)
	r := &runstate.Record{ID: "structured", Revision: 1, Status: "reconciling", StopStatus: "unknown", Allowed: true, PendingStop: "must-not-project", Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{Protocol: b.ProtocolOrcaAgentSessionV1}}, Result: &b.TurnEvidence{CommandID: "command", SessionID: "native-session", TurnID: "native-turn", Completed: true, LastMessage: "must-not-project-full-result"}}
	r.Workflow.StageInputs = []workflow.StageInput{{CommandID: "command", AttemptID: "attempt", Prompt: "one\r\n\r\n中文😀  \n", Model: "must-not-project-model-policy"}}
	r.Workflow.StageResults = []workflow.StageResult{{CommandID: "command", AttemptID: "attempt", SessionID: "native-session", TurnID: "native-turn", Final: "must-not-project-result-body"}}
	if err := s.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := s.UsageRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var projected *runstate.Record
	for _, row := range rows {
		if row.ID == r.ID {
			projected = row
		}
	}
	if projected == nil || len(projected.Workflow.StageInputs) != 1 || projected.Workflow.StageInputs[0].CommandID != "command" || projected.Workflow.StageInputs[0].AttemptID != "attempt" || projected.Workflow.StageInputs[0].Prompt != r.Workflow.StageInputs[0].Prompt || len(projected.Workflow.StageResults) != 1 || projected.Workflow.StageResults[0].SessionID != "native-session" || projected.Result == nil || !projected.Result.Completed || projected.Result.TurnID != "native-turn" {
		t.Fatal("structured usage lost frozen input or completed native turn", projected)
	}
	if projected.Allowed || projected.PendingStop != "" || projected.Workflow.Config != nil || projected.Workflow.StageInputs[0].Model != "" || projected.Workflow.StageResults[0].Final != "" || projected.Result.LastMessage != "" {
		t.Fatal("read projection gained authority/config/result body", projected)
	}
	// Unknown historical workflow formats remain entirely uninterpreted.
	var raw map[string]any
	body, _ := json.Marshal(r)
	_ = json.Unmarshal(body, &raw)
	raw["id"] = "legacy-unknown"
	raw["snapshot"].(map[string]any)["Ref"].(map[string]any)["protocol"] = string(b.ProtocolOrcaTerminalV3)
	raw["workflow"] = "unknown-historical-format"
	raw["result"] = "unknown-historical-result"
	body, _ = json.Marshal(raw)
	if _, err := s.db.Exec(`INSERT INTO runs(id,revision,body) VALUES(?,?,?)`, "legacy-unknown", 1, body); err != nil {
		t.Fatal(err)
	}
	rows, err = s.UsageRuns(context.Background())
	if err != nil {
		t.Fatal("structured projection interpreted legacy business bytes", err)
	}
	for _, row := range rows {
		if row.ID == "legacy-unknown" && (len(row.Workflow.StageInputs) != 0 || row.Result != nil) {
			t.Fatal("legacy projection changed")
		}
	}
}
