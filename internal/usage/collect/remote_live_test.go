package collect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage/access"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
)

// Recovery uses only original journal identities and the same safety checks.
func TestUsageRemoteCodexCleanupLive(t *testing.T) {
	state := os.Getenv("SR_USAGE_LIVE_CLEANUP_STATE")
	if state == "" {
		t.Skip("explicit original isolated journal only")
	}
	s, err := storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := Collector{Store: s, Policy: api.DefaultPolicy(), Instance: "cleanup-live"}
	c.Leases = &access.Manager{Store: s, Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Permit: c.checkPermit}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c.cleanup(ctx)
	keys, err := s.UsageJournalKeys(ctx, "lease:")
	if err != nil || len(keys) == 0 {
		t.Fatal("missing original leases", err)
	}
	for _, key := range keys {
		var l access.Lease
		if err = s.UsageJournal(ctx, key, &l); err != nil {
			t.Fatal(err)
		}
		t.Logf("epoch=%d phase=%s group=%s folder=%s", l.Epoch, l.Phase, l.GroupID, l.FolderID)
		if l.Phase != "revoked" {
			t.Fatal("cleanup remains pending")
		}
	}
}

// Explicit opt-in only: reads one completed original Run into an isolated ledger.
// No Engine, model input, remote process, credential or helper is created.
func TestUsageRemoteCodexLive(t *testing.T) {
	source, id, evidence := os.Getenv("SR_USAGE_LIVE_STATE"), os.Getenv("SR_USAGE_LIVE_RUN"), os.Getenv("SR_USAGE_LIVE_EVIDENCE")
	if source == "" || id == "" || evidence == "" {
		t.Skip("requires explicit existing state, completed Run and evidence directory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	reader, err := storage.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reader.UsageRuns(ctx)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.ID != id {
			continue
		}
		if run.Status != "pr_ready" && run.StopStatus != "stopped" {
			t.Fatal("requires completed evidence")
		}
		binding, p, _, err := orca.UsageBinding(run.Profile, run.Snapshot, orca.UsageInitialization{LaunchCommand: run.LaunchCommand, CreatedAt: run.CreatedAt})
		if err != nil {
			t.Fatal(err)
		}
		if binding.Agent != "codex" || p.Environment == "" {
			t.Fatal("requires remote Codex binding")
		}
		host, err := native.UsageHostID(p.Environment)
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := currentRuntime(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		channel, err := native.OpenUsageFiles(ctx, p.Environment, runtime, host)
		if err != nil {
			t.Fatal(err)
		}
		defer channel.Close()
		groups, err := channel.Groups(ctx)
		if err != nil {
			t.Fatal(err)
		}
		folders, err := channel.Folders(ctx)
		if err != nil {
			t.Fatal(err)
		}
		sessions, err := channel.Sessions(ctx)
		if err != nil {
			t.Fatal(err)
		}
		root, file := "", ""
		for _, session := range sessions {
			if session.SessionID == binding.SessionID && session.Agent == binding.Agent && parsers.NormalizePath(session.CWD) == parsers.NormalizePath(binding.CWD) {
				if root != "" {
					t.Fatal("ambiguous locator")
				}
				root, file = filepath.Dir(session.FilePath), filepath.Base(session.FilePath)
			}
		}
		if root == "" {
			t.Fatal("original native session not found")
		}
		if err = channel.VerifySource(ctx, root, file); err != nil {
			t.Fatalf("preflight path: %v (no registration created)", err)
		}
		if err = os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		state := filepath.Join(evidence, "state")
		if err = os.MkdirAll(state, 0700); err != nil {
			t.Fatal(err)
		}
		dest, err := storage.Open(state)
		if err != nil {
			t.Fatal(err)
		}
		defer dest.Close()
		if err = dest.InitUsage(); err != nil {
			t.Fatal(err)
		}
		run.Revision = 1
		if err = dest.Commit(run, 0, nil); err != nil {
			t.Fatal(err)
		}
		c := Collector{Store: dest, Policy: api.DefaultPolicy(), Instance: "explicit-live-acceptance"}
		// Default configuration must not create any remote registration.
		authorizedTick(t, &c, ctx)
		v, err := dest.GetRunUsage(ctx, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != "unavailable" || v.Tokens != nil {
			t.Fatalf("default policy: %+v", v)
		}
		keys, err := dest.UsageJournalKeys(ctx, "lease:")
		if err != nil || len(keys) != 0 {
			t.Fatal("disabled policy created an intent", keys, err)
		}
		c.Policy.RemoteLogAccess.Mode = "temporary"
		c.Policy.RemoteLogAccess.Grants = []api.Grant{{Environment: p.Environment, HostID: host, SessionRoots: []string{root}}}
		c.Leases = &access.Manager{Store: dest, Instance: c.Instance, PolicyDigest: c.Policy.Digest(), Permit: c.checkPermit}
		defer func() {
			clean, done := context.WithTimeout(context.Background(), 45*time.Second)
			defer done()
			c.cleanup(clean)
			afterGroups, e1 := channel.Groups(clean)
			afterFolders, e2 := channel.Folders(clean)
			if e1 != nil || e2 != nil || !reflect.DeepEqual(groups, afterGroups) || !reflect.DeepEqual(folders, afterFolders) {
				t.Errorf("remote registration baseline not restored: groups=%d/%d folders=%d/%d errors=%v/%v", len(groups), len(afterGroups), len(folders), len(afterFolders), e1, e2)
			}
		}()
		authorizedTick(t, &c, ctx)
		v, err = dest.GetRunUsage(ctx, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.MarshalIndent(v, "", "  ")
		if err = os.WriteFile(filepath.Join(evidence, "usage.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		keys, _ = dest.UsageJournalKeys(ctx, "lease:")
		for _, key := range keys {
			var lease access.Lease
			if err = dest.UsageJournal(ctx, key, &lease); err != nil {
				t.Fatal(err)
			}
			t.Logf("lease epoch=%d phase=%s group=%s folder=%s unresolved=%t/%t", lease.Epoch, lease.Phase, lease.GroupID, lease.FolderID, lease.CreateUnresolved, lease.MutationUnresolved)
		}
		t.Logf("snapshot=%s", raw)
		if v.Status != "available" || !v.Coverage.Finalized || v.AccessCleanupStatus != "revoked" {
			t.Fatal("live acceptance incomplete")
		}
		revision := v.Revision
		authorizedTick(t, &c, ctx)
		v, err = dest.GetRunUsage(ctx, id, 0)
		if err != nil || v.Revision != revision {
			t.Fatal("repeated collection changed finalized usage", err)
		}
		return
	}
	t.Fatal("original Run not found")
}
