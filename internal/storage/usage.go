package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/usage"
	"github.com/erranliu/SteerRunner/internal/usage/api"
)

// InitUsage is additive: it never changes the Run codec or business revision.
func (s *Store) InitUsage() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS usage_events(id TEXT PRIMARY KEY,run_id TEXT NOT NULL,body BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS usage_events_run ON usage_events(run_id);
CREATE TABLE IF NOT EXISTS usage_snapshots(run_id TEXT NOT NULL,revision INTEGER NOT NULL,body BLOB NOT NULL,events BLOB NOT NULL,PRIMARY KEY(run_id,revision));
CREATE TABLE IF NOT EXISTS usage_journal(id TEXT PRIMARY KEY,body BLOB NOT NULL);`)
	return err
}

// UsageRuns decodes only the read-only evidence projection. Unknown historical
// business configuration is not interpreted, migrated or allowed to execute.
func (s *Store) UsageRuns(ctx context.Context) ([]*runstate.Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM runs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []*runstate.Record{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var evidence struct {
			ID            string
			Revision      uint64
			Status        string    `json:"status"`
			StopStatus    string    `json:"stop_status"`
			CreatedAt     time.Time `json:"created_at"`
			LaunchCommand string    `json:"launch_command"`
			Snapshot      b.LifecycleSnapshot
			Profile       []b.ImplementationDataEnvelope
			Attempts      []runstate.Attempt
		}
		if err = json.Unmarshal(raw, &evidence); err != nil {
			return nil, err
		}
		runs = append(runs, &runstate.Record{ID: evidence.ID, Revision: evidence.Revision, Status: evidence.Status, StopStatus: evidence.StopStatus, CreatedAt: evidence.CreatedAt, LaunchCommand: evidence.LaunchCommand, Snapshot: evidence.Snapshot, Profile: evidence.Profile, Attempts: evidence.Attempts})
	}
	return runs, rows.Err()
}

func (s *Store) EnsureUsage(ctx context.Context, run string, observed time.Time) error {
	// Publishing a query target precedes observation of its effective policy and
	// binding. It must not claim collection is enabled and waiting normally yet.
	snapshot := usage.Project(api.Snapshot{RunID: run, ObservedAt: observed, AccessCleanupStatus: "not_required", Issues: []api.Issue{{Category: "usage", Code: "COLLECTION_NOT_OBSERVED", Retryable: true}}, Coverage: api.Coverage{SourceSummaries: []api.SourceSummary{}}}, nil)
	snapshot.Revision = 1
	raw, _ := json.Marshal(snapshot)
	_, err := s.db.ExecContext(ctx, `INSERT INTO usage_snapshots(run_id,revision,body,events) SELECT ?,1,?,'[]' WHERE NOT EXISTS(SELECT 1 FROM usage_snapshots WHERE run_id=?)`, run, raw, run)
	return err
}

func (s *Store) SetUsageCleanup(ctx context.Context, run, status string) error {
	snapshot, err := s.GetRunUsage(ctx, run, 0)
	if err != nil {
		return err
	}
	snapshot.AccessCleanupStatus = status
	issues := []api.Issue{}
	for _, issue := range snapshot.Issues {
		if issue.Category != "access_cleanup" {
			issues = append(issues, issue)
		}
	}
	if status == "pending" {
		issues = append(issues, api.Issue{Category: "access_cleanup", Code: "ACCESS_CLEANUP_PENDING", Retryable: true})
	}
	snapshot.Issues = issues
	_, err = s.CommitUsage(ctx, snapshot, nil)
	return err
}

func (s *Store) GetRunUsage(ctx context.Context, run string, revision uint64) (api.Snapshot, error) {
	var result api.Snapshot
	var raw []byte
	query := `SELECT body FROM usage_snapshots WHERE run_id=? ORDER BY revision DESC LIMIT 1`
	args := []any{run}
	if revision != 0 {
		query = `SELECT body FROM usage_snapshots WHERE run_id=? AND revision=?`
		args = append(args, revision)
	}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, api.ErrSnapshotNotFound
	}
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
}

func usageCursor(run string, revision uint64, index int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d:%d", usage.Digest(run), revision, index)))
}
func (s *Store) ListRunUsageEvents(ctx context.Context, run string, revision uint64, after string, limit int) (api.Page, error) {
	page := api.Page{RunID: run, Revision: revision, Events: []api.Event{}}
	if revision == 0 {
		return page, api.ErrSnapshotNotFound
	}
	if limit < 1 || limit > 500 {
		return page, api.ErrCursor
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT events FROM usage_snapshots WHERE run_id=? AND revision=?`, run, revision).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return page, api.ErrSnapshotNotFound
	}
	if err != nil {
		return page, err
	}
	var events []api.Event
	if err = json.Unmarshal(raw, &events); err != nil {
		return page, err
	}
	index := 0
	if after != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(after)
		if e != nil {
			return page, api.ErrCursor
		}
		prefix := fmt.Sprintf("%s:%d:", usage.Digest(run), revision)
		if !strings.HasPrefix(string(decoded), prefix) {
			return page, api.ErrCursor
		}
		index, e = strconv.Atoi(strings.TrimPrefix(string(decoded), prefix))
		if e != nil || index < 0 || index > len(events) || usageCursor(run, revision, index) != after {
			return page, api.ErrCursor
		}
	}

	end := index + limit
	if end > len(events) {
		end = len(events)
	}
	page.Events = events[index:end]
	if end < len(events) {
		page.NextCursor = usageCursor(run, revision, end)
	}
	return page, nil
}

// CommitUsage atomically merges stable identities, publishes immutable pages and
// advances coverage. File/network work must be complete before entering here.
func (s *Store) CommitUsage(ctx context.Context, next api.Snapshot, incoming []api.Event, journal ...map[string]any) (api.Snapshot, error) {
	return s.commitUsage(ctx, next, incoming, 0, journal...)
}
func (s *Store) CommitObservedUsage(ctx context.Context, next api.Snapshot, incoming []api.Event, runRevision uint64, journal map[string]any, permit api.CollectionPermit) (api.Snapshot, error) {
	return s.commitUsageWithPermit(ctx, next, incoming, runRevision, &permit, journal)
}
func (s *Store) commitUsage(ctx context.Context, next api.Snapshot, incoming []api.Event, runRevision uint64, journal ...map[string]any) (api.Snapshot, error) {
	return s.commitUsageWithPermit(ctx, next, incoming, runRevision, nil, journal...)
}
func (s *Store) commitUsageWithPermit(ctx context.Context, next api.Snapshot, incoming []api.Event, runRevision uint64, permit *api.CollectionPermit, journal ...map[string]any) (api.Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return next, err
	}
	defer tx.Rollback()
	if permit != nil {
		var raw []byte
		var current api.CollectionPermit
		if e := tx.QueryRowContext(ctx, `SELECT body FROM usage_journal WHERE id=?`, api.CollectionPermitKey).Scan(&raw); e != nil {
			return next, api.ErrCollectionRevoked
		}
		if json.Unmarshal(raw, &current) != nil || !current.Active || !permit.Active || current.Instance != permit.Instance || current.PolicyDigest != permit.PolicyDigest {
			return next, api.ErrCollectionRevoked
		}
	}
	var currentRevision uint64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM runs WHERE id=?`, next.RunID).Scan(&currentRevision); errors.Is(err, sql.ErrNoRows) {
		return next, contracts.Fail(5, "run_not_found", "Run was not found.")
	} else if err != nil {
		return next, err
	}
	if runRevision != 0 && runRevision != currentRevision {
		return next, contracts.Fail(5, "usage_observation_stale", "Run changed during observation; no projection was committed.")
	}
	affected := map[string]bool{next.RunID: true}
	for _, e := range incoming {
		claimedSource := e.SourceRef
		if e.ID == "" || e.RunID != next.RunID || e.SourceRef == "" || e.SessionID == "" || e.ResponseID == "" || usage.Validate(e.Tokens) != nil {
			return next, errors.New("invalid usage event")
		}
		e.Decision = "counted"
		if e.MetadataIssues == nil {
			e.MetadataIssues = []api.Issue{}
		}
		var oldraw []byte
		err = tx.QueryRowContext(ctx, `SELECT body FROM usage_events WHERE id=?`, e.ID).Scan(&oldraw)
		if err == nil {
			var old api.Event
			if err = json.Unmarshal(oldraw, &old); err != nil {
				return next, err
			}
			affected[old.RunID] = true
			merged := usage.Merge(old, e)
			if merged.Decision == "quarantined" {
				// Quarantine keeps the original public identity. Preserve both
				// normalized source claims privately in the same transaction;
				// otherwise the incoming token/evidence digest would be lost.
				identity := e
				identity.ObservedAt = time.Time{}
				canonical, _ := json.Marshal(identity)
				proof, _ := json.Marshal(struct{ Prior, Observed api.Event }{old, e})
				key := "event-conflict:" + e.ID + ":" + usage.Digest(string(canonical))
				if _, err = tx.ExecContext(ctx, `INSERT INTO usage_journal(id,body) VALUES(?,?) ON CONFLICT(id) DO NOTHING`, key, proof); err != nil {
					return next, err
				}
			}
			e = merged
		} else if !errors.Is(err, sql.ErrNoRows) {
			return next, err
		}
		raw, _ := json.Marshal(e)
		if _, err = tx.ExecContext(ctx, `INSERT INTO usage_events(id,run_id,body) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body`, e.ID, e.RunID, raw); err != nil {
			return next, err
		}
		if e.RunID != next.RunID {
			next.Issues = append(next.Issues, api.Issue{Category: "usage", Code: "USAGE_CONFLICT", SourceRef: claimedSource})
		}
	}
	for run := range affected {
		var prior api.Snapshot
		var priorRaw, priorEvents []byte
		err = tx.QueryRowContext(ctx, `SELECT body,events FROM usage_snapshots WHERE run_id=? ORDER BY revision DESC LIMIT 1`, run).Scan(&priorRaw, &priorEvents)
		if err == nil {
			if err = json.Unmarshal(priorRaw, &prior); err != nil {
				return next, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return next, err
		}
		projection := next
		// Quarantine is permanent. A rejected cross-Run claim has no event row
		// under the claimant, so a cached source observation must retain its
		// persisted conflict rather than infer complete coverage from that absence.
		if run == next.RunID {
			for _, issue := range prior.Issues {
				if issue.Category == "usage" && issue.Code == "USAGE_CONFLICT" {
					projection.Issues = append(projection.Issues, issue)
				}
			}
		}
		if run != next.RunID {
			projection = prior
			if projection.RunID == "" {
				continue
			}
		}
		rows, e := tx.QueryContext(ctx, `SELECT body FROM usage_events WHERE run_id=? ORDER BY id`, run)
		if e != nil {
			return next, e
		}
		events := []api.Event{}
		for rows.Next() {
			var raw []byte
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return next, e
			}
			var event api.Event
			if e = json.Unmarshal(raw, &event); e != nil {
				rows.Close()
				return next, e
			}
			events = append(events, event)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return next, e
		}
		projection = usage.Project(projection, events)
		sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
		eventRaw, _ := json.Marshal(events)
		if prior.RunID != "" && usage.Equivalent(prior, projection) && string(eventRaw) == string(priorEvents) {
			if run == next.RunID {
				next = prior
			}
			continue
		}
		projection.Revision = prior.Revision + 1
		projection.ObservedAt = time.Now().UTC()
		raw, _ := json.Marshal(projection)
		if _, err = tx.ExecContext(ctx, `INSERT INTO usage_snapshots(run_id,revision,body,events) VALUES(?,?,?,?)`, run, projection.Revision, raw, eventRaw); err != nil {
			return next, err
		}
		if run == next.RunID {
			next = projection
		}
	}
	for _, updates := range journal {
		for id, value := range updates {
			raw, e := json.Marshal(value)
			if e != nil {
				return next, e
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO usage_journal(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body`, id, raw); e != nil {
				return next, e
			}
		}
	}
	return next, tx.Commit()
}

func (s *Store) SetUsagePermit(ctx context.Context, permit api.CollectionPermit) error {
	if permit.Instance == "" || permit.PolicyDigest == "" {
		return api.ErrCollectionRevoked
	}
	return s.PutUsageJournal(ctx, api.CollectionPermitKey, permit)
}
func (s *Store) CheckUsagePermit(ctx context.Context, instance, digest string) error {
	var current api.CollectionPermit
	if err := s.UsageJournal(ctx, api.CollectionPermitKey, &current); err != nil {
		return api.ErrCollectionRevoked
	}
	if !current.Active || current.Instance != instance || current.PolicyDigest != digest {
		return api.ErrCollectionRevoked
	}
	return ctx.Err()
}
func (s *Store) RevokeUsagePermit(ctx context.Context, instance string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = revokeUsagePermit(tx, instance); err != nil {
		return err
	}
	return tx.Commit()
}
func revokeUsagePermit(tx *sql.Tx, instance string) error {
	var table string
	if err := tx.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='usage_journal'`).Scan(&table); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var raw []byte
	if err := tx.QueryRow(`SELECT body FROM usage_journal WHERE id=?`, api.CollectionPermitKey).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var permit api.CollectionPermit
	if json.Unmarshal(raw, &permit) != nil {
		permit = api.CollectionPermit{Instance: instance}
	}
	if permit.Instance != instance {
		return nil
	}
	permit.Active = false
	raw, _ = json.Marshal(permit)
	_, err := tx.Exec(`UPDATE usage_journal SET body=? WHERE id=?`, raw, api.CollectionPermitKey)
	return err
}

// Journal contains private source checkpoints and remote ownership intents only.
func (s *Store) UsageJournal(ctx context.Context, id string, target any) error {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM usage_journal WHERE id=?`, id).Scan(&raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
func (s *Store) PutUsageJournal(ctx context.Context, id string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO usage_journal(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body`, id, raw)
	return err
}
func (s *Store) UsageJournalKeys(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM usage_journal WHERE id LIKE ? ORDER BY id`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
