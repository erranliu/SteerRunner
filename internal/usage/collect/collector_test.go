package collect

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

func TestUsageRetirementGatePreservesPersistedUsageAndSkipsNewAccess(t *testing.T) {
	s, r, _ := localFixture(t)
	ctx := context.Background()
	if err := s.EnsureUsage(ctx, r.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	checks := 0
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "i", CanObserveRun: func(id string) bool {
		checks++
		if id != r.ID {
			t.Fatal("unexpected Run", id)
		}
		return false
	}}
	authorizedTick(t, &c, ctx)
	after, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || checks != 1 || !reflect.DeepEqual(before, after) {
		t.Fatal("retired collection changed persisted usage", checks, before, after, err)
	}
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 0 {
		t.Fatal("retired collection discovered or journaled a native source", keys, err)
	}
	c.CanObserveRun = func(string) bool { return true }
	c.Tick(ctx)
	current, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || current.Tokens == nil || *current.Tokens.Total != 85 {
		t.Fatal("eligible collection regressed", current, err)
	}
}

func TestUsageCollectorPermitAndSourceIdentity(t *testing.T) {
	s, r, _ := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "i"}
	authorizedTick(t, &c, ctx)
	before, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeUsagePermit(ctx, "i"); err != nil {
		t.Fatal(err)
	}
	c.Tick(ctx)
	after, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || after.Revision != before.Revision {
		t.Fatal("revoked tick changed projection", after, err)
	}
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var cp checkpoint
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil {
		t.Fatal(err)
	}
	binding, _, _, err := orca.UsageBinding(r.Profile, r.Snapshot, orca.UsageInitialization{})
	if err != nil {
		t.Fatal(err)
	}
	binding.HostID = "different-account"
	if _, err = parseSnapshot(ctx, binding, nil, cp, false); err == nil || err.Error() != "IDENTITY_CHANGED" {
		t.Fatal("changed source reused checkpoint", err)
	}
	a, b, err := localUsageIdentity()
	if err != nil {
		t.Fatal(err)
	}
	x, y, err := localUsageIdentity()
	if err != nil || a == "" || b == "" || a != x || b != y {
		t.Fatal("unstable machine/account identity", err)
	}
	if !errors.Is(c.checkPermit(ctx), api.ErrCollectionRevoked) {
		t.Fatal("revoked permit accepted")
	}
}

func TestUsageCleanupStatusIsRunScoped(t *testing.T) {
	s, _, _ := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s}
	put := func(key string, l access.Lease) {
		t.Helper()
		if err := s.PutUsageJournal(ctx, key, l); err != nil {
			t.Fatal(err)
		}
	}
	root := access.Lease{ID: "lease:shared", RunID: "r", Runs: map[string]bool{"r": true, "shared": true}, Epoch: 1, Phase: "active", Consumers: map[string]bool{"other": true}}
	put(root.ID, root)
	put("lease:unrelated", access.Lease{ID: "lease:unrelated", RunID: "other", Epoch: 1, Phase: "releasing"})
	if got := c.cleanupStatus(ctx, "r", ""); got != "active" {
		t.Fatal(got)
	}
	root.Phase = "revoked"
	root.Consumers = nil
	put(root.ID, root)
	put("revocation:lease:shared:1", root)
	if got := c.cleanupStatus(ctx, "r", ""); got != "revoked" {
		t.Fatal("unrelated Run contaminated cleanup", got)
	}
	newEpoch := root
	newEpoch.RunID = "new"
	newEpoch.Runs = map[string]bool{"new": true}
	newEpoch.Epoch = 2
	newEpoch.Phase = "creating_group"
	put(root.ID, newEpoch)
	if got := c.cleanupStatus(ctx, "r", ""); got != "revoked" {
		t.Fatal("new epoch erased old revocation", got)
	}
	if got := c.cleanupStatus(ctx, "new", ""); got != "pending" {
		t.Fatal(got)
	}
	if got := c.cleanupStatus(ctx, "never-authorized", ""); got != "pending" {
		t.Fatal("absence proved revocation", got)
	}
}

func TestUsageCorruptCheckpointCannotRebind(t *testing.T) {
	s, r, file := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "i"}
	authorizedTick(t, &c, ctx)
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	if err = s.PutUsageJournal(ctx, keys[0], "malformed checkpoint shape"); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	v, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "partial" || v.Tokens == nil || *v.Tokens.Total != 85 || v.Coverage.Finalized {
		t.Fatal(v, err)
	}
	found := false
	for _, i := range v.Issues {
		found = found || i.Code == "SOURCE_CHANGED"
	}
	if !found {
		t.Fatal("corrupt checkpoint triggered log discovery", v.Issues)
	}
	var original string
	if err = s.UsageJournal(ctx, keys[0], &original); err != nil || original != "malformed checkpoint shape" {
		t.Fatal("corrupt identity evidence replaced", err)
	}
}

func authorizedTick(t *testing.T, c *Collector, ctx context.Context) {
	t.Helper()
	if err := c.Store.SetUsagePermit(ctx, api.CollectionPermit{Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Active: true}); err != nil {
		t.Fatal(err)
	}
	c.Tick(ctx)
}

func TestUsageLocalCodexHomeRequiresKnownRoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".codex", "sessions", "2026", "10", "21", "rollout.jsonl")
	if localCodexHome(file) != filepath.Join(home, ".codex") {
		t.Fatal("default local source rejected")
	}
	configured := filepath.Join(t.TempDir(), "private-codex-home")
	t.Setenv("CODEX_HOME", configured)
	if localCodexHome(filepath.Join(configured, "sessions", "2026", "10", "21", "rollout.jsonl")) != configured {
		t.Fatal("explicit local home rejected")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	account := filepath.Join(config, "orca", "codex-accounts", "account-identity", "home")
	if localCodexHome(filepath.Join(account, "sessions", "2026", "10", "21", "rollout.jsonl")) != account {
		t.Fatal("Orca account rejected")
	}
	for _, unknown := range []string{filepath.Join(t.TempDir(), "unverified", "sessions", "2026", "10", "21", "rollout.jsonl"), filepath.Join(home, ".codex", "auth.json"), "relative/sessions/2026/10/21/log.jsonl"} {
		if localCodexHome(unknown) != "" {
			t.Fatal("untrusted root accepted", unknown)
		}
	}
}

func TestUsageAuthorizationAfterExhaustedObservation(t *testing.T) {
	s, r, file := localFixture(t)
	ctx := context.Background()
	policy := api.DefaultPolicy()
	policy.Enabled = false
	c := Collector{Store: s, Policy: policy, Instance: "first"}
	authorizedTick(t, &c, ctx)
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var cp checkpoint
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil {
		t.Fatal(err)
	}
	cp.EndedAt = time.Now().Add(-31 * time.Second)
	if err = s.PutUsageJournal(ctx, keys[0], cp); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	authorizedTick(t, &c, ctx)
	v, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "unavailable" || v.Coverage.Finalized {
		t.Fatal("exhausted observation claimed complete coverage", v, err)
	}
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil || !cp.Finalized {
		t.Fatal("observation did not stop", err)
	}
	c.Policy.Enabled = true
	authorizedTick(t, &c, ctx)
	v, err = s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "available" || !v.Coverage.Finalized || v.Tokens == nil || *v.Tokens.Total != 85 {
		t.Fatal("new permit did not repair gap", v, err)
	}
	// Complete history does not require re-reading on replacement or disable.
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	c.Instance = "replacement"
	c.Policy.Enabled = false
	authorizedTick(t, &c, ctx)
	after, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || after.Revision != v.Revision {
		t.Fatal("complete history degraded", after, err)
	}
}

func localFixture(t *testing.T) (*storage.Store, *runstate.Record, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.InitUsage(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "terminal-agents", "run", "generation")
	if err = os.MkdirAll(filepath.Join(root, "sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../parsers/testdata/pi.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "sessions", "date_fixture-session.jsonl")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	p := orca.Profile{Executable: "orca", WaitTimeout: "1s", Terminal: &orca.TerminalConfig{Agent: "pi", ExecutionID: "r", AttemptID: "a", Generation: "g", SessionID: "fixture-session", StateDirectory: root}}
	raw, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{Purpose: b.ImplementationConfigPurpose, ImplementationKey: orca.ProfileKey, CodecVersion: orca.ProfileVersion, ProjectID: "p", ProfileID: "profile", AcceptedConfigVersion: "1"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	state := orca.State{Turns: []orca.Turn{{Command: "command-0", Prompt: "fixture input 0", Accepted: true, Settled: true}}}
	raw, _ = json.Marshal(state)
	envelope, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{Purpose: b.ImplementationResourcePurpose, ImplementationKey: orca.StateKey, CodecVersion: orca.LocalStateVersion, ProjectID: "p", ExecutionID: "r", AttemptID: "a", Generation: "g", ConfigDigest: "config"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	r := &runstate.Record{ID: "r", Revision: 1, Status: "pr_ready", Profile: []b.ImplementationDataEnvelope{profile}, Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{RunID: "r", AttemptID: "a", WorkspacePath: `C:\fixture\work`, Agent: b.AgentSpec{Name: "pi"}}, Session: b.SessionContext{ProjectID: "p", ExecutionID: "r", AttemptID: "a"}, ResourceGeneration: "g", ConfigurationDigest: "config", Implementation: []b.ImplementationDataEnvelope{envelope}}}
	if err = s.Commit(r, 0, nil); err != nil {
		t.Fatal(err)
	}
	return s, r, file
}
func TestUsageAutomaticCollectionAndHistoricalRetention(t *testing.T) {
	s, r, file := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "instance"}
	authorizedTick(t, &c, ctx)
	v, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "available" || v.Tokens == nil || *v.Tokens.Total != 85 || !v.Coverage.Finalized {
		t.Fatal(v, err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	c.Policy.Enabled = false
	c.Instance = "replacement"
	authorizedTick(t, &c, ctx)
	again, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || again.Revision != v.Revision || again.Status != "available" {
		t.Fatal("finalized history degraded", again, err)
	}
	runs, _ := s.Runs()
	if runs[0].Revision != 1 {
		t.Fatal("collector mutated Run")
	}
}

func TestUsageReplacementRetriesExhaustedGap(t *testing.T) {
	s, r, file := localFixture(t)
	ctx := context.Background()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "original"}
	authorizedTick(t, &c, ctx)
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var cp checkpoint
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil {
		t.Fatal(err)
	}
	cp.EndedAt = time.Now().Add(-31 * time.Second)
	if err = s.PutUsageJournal(ctx, keys[0], cp); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	v, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "unavailable" || v.Coverage.Finalized {
		t.Fatal("same observation silently restarted", v, err)
	}
	replacement := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "replacement"}
	authorizedTick(t, &replacement, ctx)
	v, err = s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || v.Status != "available" || !v.Coverage.Finalized || v.Tokens == nil || *v.Tokens.Total != 85 {
		t.Fatal("new instance did not repair original gap", v, err)
	}
}
func TestUsageSnapshotPrefixChanged(t *testing.T) {
	b := parsers.Binding{}
	raw := []byte("prefix\n")
	cp := checkpoint{Bytes: len(raw), Hash: parsers.TextHash(string(raw))}
	if _, err := parseSnapshot(context.Background(), b, []byte("changed\n"), cp, false); err == nil || err.Error() != "SOURCE_CHANGED" {
		t.Fatal(err)
	}
}

func TestUsageNewAuthorizedInputInvalidatesClosedCoverage(t *testing.T) {
	s, r, _ := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "instance"}
	authorizedTick(t, &c, ctx)
	old, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || !old.Coverage.Finalized {
		t.Fatal(old, err)
	}
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var cp checkpoint
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil {
		t.Fatal(err)
	}
	cp.EndedAt = time.Now().Add(-31 * time.Second)
	if err = s.PutUsageJournal(ctx, keys[0], cp); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	r.Status = "running"
	r.Snapshot.PendingTurn = &b.TurnInput{CommandID: "new-authorized-input", Prompt: "next exact instruction"}
	r.Snapshot.Session.PendingCommandID = "new-authorized-input"
	if err = s.Commit(r, r.Revision-1, nil); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	next, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || next.Status != "partial" || next.Coverage.Finalized || next.Revision <= old.Revision || next.Tokens == nil || *next.Tokens.Total != 85 {
		t.Fatal("new input reused old finality", next, err)
	}
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil || !cp.EndedAt.IsZero() || cp.Finalized {
		t.Fatal("new input inherited old observation budget", cp, err)
	}
	history, _ := s.GetRunUsage(ctx, r.ID, old.Revision)
	if !history.Coverage.Finalized || history.Status != "available" {
		t.Fatal("history rewritten")
	}
}

func TestUsageEndedBindingWaitCannotStayPending(t *testing.T) {
	s, r, _ := localFixture(t)
	ctx := context.Background()
	r.Snapshot.Implementation = nil
	r.Revision++
	if err := s.Commit(r, r.Revision-1, nil); err != nil {
		t.Fatal(err)
	}
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "i"}
	authorizedTick(t, &c, ctx)
	before, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || before.Status != "pending" {
		t.Fatal(before, err)
	}
	keys, err := s.UsageJournalKeys(ctx, "source:")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	var cp checkpoint
	if err = s.UsageJournal(ctx, keys[0], &cp); err != nil {
		t.Fatal(err)
	}
	cp.EndedAt = time.Now().Add(-31 * time.Second)
	if err = s.PutUsageJournal(ctx, keys[0], cp); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	after, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || after.Status != "unavailable" || after.Tokens != nil || after.Coverage.Finalized {
		t.Fatal("exhausted binding wait stayed pending", after, err)
	}
}

func TestUsageChangedCurrentProfileCannotReuseClosedBinding(t *testing.T) {
	s, r, _ := localFixture(t)
	ctx := context.Background()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "instance"}
	authorizedTick(t, &c, ctx)
	var p orca.Profile
	if err := json.Unmarshal(r.Profile[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	p.Terminal.SessionID = "changed-native-session"
	raw, _ := json.Marshal(p)
	envelope, err := b.NewImplementationDataEnvelope(r.Profile[0], raw)
	if err != nil {
		t.Fatal(err)
	}
	r.Profile[0] = envelope
	r.Revision++
	if err = s.Commit(r, r.Revision-1, nil); err != nil {
		t.Fatal(err)
	}
	authorizedTick(t, &c, ctx)
	next, err := s.GetRunUsage(ctx, r.ID, 0)
	if err != nil || next.Status != "partial" || next.Coverage.Finalized {
		t.Fatal("profile switch hid binding change", next, err)
	}
}

func TestUsageDedupAccountNamespaceMoves(t *testing.T) {
	first := codexNamespace(`C:\Users\original\orca\codex-accounts\account-identity\home`)
	moved := codexNamespace(`D:\moved\codex-accounts\account-identity\home`)
	if first != moved {
		t.Fatal("Orca account namespace depended on its installation path")
	}
}

func TestUsageEmptyTerminalListingProof(t *testing.T) {
	for _, raw := range []string{
		`{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"omittedHostIds":[]}}`,
		`{"terminals":[]}`, `{"terminals":[],"totalCount":1,"truncated":false,"hostScope":{"omittedHostIds":[]}}`,
		`{"terminals":[],"totalCount":0,"truncated":true,"hostScope":{"omittedHostIds":[]}}`,
		`{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"omittedHostIds":["local"]}}`,
		`{"terminals":[{}],"totalCount":1,"truncated":false,"hostScope":{"omittedHostIds":[]}}`,
	} {
		want := raw == `{"terminals":[],"totalCount":0,"truncated":false,"hostScope":{"omittedHostIds":[]}}`
		if emptyTerminalListing(json.RawMessage(raw)) != want {
			t.Fatal(raw)
		}
	}
}

func TestUsageRemoteProductionBindingUnsupported(t *testing.T) {
	for _, agent := range []string{"pi", "dsh"} {
		t.Run(agent, func(t *testing.T) {
			s, r, _ := localFixture(t)
			var p orca.Profile
			if err := json.Unmarshal(r.Profile[0].Payload, &p); err != nil {
				t.Fatal(err)
			}
			p.Environment = "explicit-test-environment"
			raw, _ := json.Marshal(p)
			envelope, err := b.NewImplementationDataEnvelope(r.Profile[0], raw)
			if err != nil {
				t.Fatal(err)
			}
			r.Profile[0] = envelope
			r.Snapshot.Ref.Agent.Name = agent
			r.Revision = 2
			if err = s.Commit(r, 1, nil); err != nil {
				t.Fatal(err)
			}
			c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "i"}
			authorizedTick(t, &c, context.Background())
			v, err := s.GetRunUsage(context.Background(), r.ID, 0)
			if err != nil || v.Status != "unavailable" || v.Tokens != nil {
				t.Fatal(v, err)
			}
			found := false
			for _, issue := range v.Issues {
				if issue.Code == "PRODUCTION_BINDING_UNSUPPORTED" {
					found = true
				}
			}
			if !found {
				t.Fatal(v)
			}
		})
	}
}
func TestUsageAvailabilityWithoutCost(t *testing.T) {
	s, r, _ := localFixture(t)
	p := api.DefaultPolicy()
	p.Enabled = false
	c := Collector{Store: s, Policy: p, Instance: "i"}
	authorizedTick(t, &c, context.Background())
	v, err := s.GetRunUsage(context.Background(), r.ID, 0)
	if err != nil || v.Status != "unavailable" || v.Tokens != nil || len(v.Issues) == 0 || v.Issues[0].Code != "COLLECTION_DISABLED" {
		t.Fatal(v, err)
	}
}

// Opt in with a directory containing previously completed real Runs. The source
// database is opened mode=ro. Only copies and usage projections enter t.TempDir;
// no Engine, helper, terminal or model is started by this verification.
func TestUsageExistingNativeEvidence(t *testing.T) {
	source := os.Getenv("SR_USAGE_EXISTING_STATE")
	if source == "" {
		t.Skip("explicit existing-state verification only")
	}
	reader, err := storage.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	runs, err := reader.UsageRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dest, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	if err = dest.InitUsage(); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{"691dc23163d59c560b734458a1bff3d5": 200432, "6dd9e9102f2d197bc419f08881d098c2": 318722}
	found := 0
	for _, run := range runs {
		want, ok := ids[run.ID]
		if !ok {
			continue
		}
		found++
		run.Revision = 1
		if err = dest.Commit(run, 0, nil); err != nil {
			t.Fatal(err)
		}
		c := Collector{Store: dest, Policy: api.DefaultPolicy(), Instance: "read-only-evidence"}
		authorizedTick(t, &c, context.Background())
		v, err := dest.GetRunUsage(context.Background(), run.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("agent=%s status=%s tokens=%+v coverage=%+v issues=%+v", run.Snapshot.Ref.Agent.Name, v.Status, v.Tokens, v.Coverage, v.Issues)
		if v.Tokens == nil || v.Tokens.Total == nil || *v.Tokens.Total != want {
			t.Fatalf("agent=%s expected total %d", run.Snapshot.Ref.Agent.Name, want)
		}
	}
	if found != len(ids) {
		t.Fatalf("expected two original Runs, found %d", found)
	}
}
