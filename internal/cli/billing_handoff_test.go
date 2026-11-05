//go:build billingacceptance

package cli

import (
	"context"
	"encoding/json"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
	"github.com/erranliu/SteerRunner/internal/usage/parsers"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Creates only a tagged, explicit, create-only acceptance fixture. No native
// process or business input is created; SteerLine subsequently uses public CLI.
func TestUsageSteerLineFixtureHandoff(t *testing.T) {
	root := os.Getenv("STEERRUNNER_BILLING_FIXTURE")
	if root == "" || !strings.HasPrefix(strings.ToLower(filepath.Clean(root)), `c:\steerlinedebug\`) {
		t.Fatal("explicit isolated fixture root required")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal("fixture must be new", err)
	}
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(state)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.InitUsage(); err != nil {
		t.Fatal(err)
	}
	model, provider := "fixture", "synthetic"
	when := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"priced", "completion"} {
		run := &runstate.Record{ID: id, Revision: 1, Status: "stopped", StopStatus: "stopped", Closed: true, Snapshot: b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1}}}
		if err = s.Commit(run, 0, nil); err != nil {
			t.Fatal(err)
		}
		basis := "request"
		if id == "completion" {
			basis = "completion"
		}
		source := usage.Digest(id, ":")
		profile, _ := json.Marshal(run.Profile)
		inputHash := usage.Digest(orca.UsageInputDigest(run.Snapshot), usage.Digest(string(profile)), run.LaunchCommand, run.CreatedAt.UTC().Format(time.RFC3339Nano), parsers.Version)
		if err = s.PutUsageJournal(context.Background(), "source:"+source, map[string]any{"Finalized": true, "Input": inputHash, "Summary": api.SourceSummary{SourceRef: source, Status: "verified", Finalized: true}}); err != nil {
			t.Fatal(err)
		}
		event := api.Event{ID: id + "-event", RunID: id, SourceRef: source, SessionID: id + "-session", ResponseID: id + "-response", Decision: "counted", Model: &model, Provider: &provider, MetadataBasis: "observed", CallTime: &when, TimeBasis: basis, Tokens: api.Tokens{InputTotal: api.Number(100), InputUncached: api.Number(100), CacheRead: api.Number(0), CacheWrite: api.Number(0), Output: api.Number(20), Total: api.Number(120)}}
		_, err = s.CommitUsage(context.Background(), api.Snapshot{RunID: id, AccessCleanupStatus: "not_required", Coverage: api.Coverage{ExpectedSources: api.Number(1), VerifiedSources: 1, Finalized: true}}, []api.Event{event})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(root, "fixture.json"), []byte(`{"schema":"billing-acceptance/v1","runs":["priced","completion"]}`), 0600); err != nil {
		t.Fatal(err)
	}
}
