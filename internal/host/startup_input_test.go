package host

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestPendingStopsRetainOriginalRequestsAcrossUnknownStartup(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	input := &contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{{RunID: "run", Request: contracts.Request{Protocol: contracts.Protocol, RequestID: "original-stop", Input: json.RawMessage(`{"expected_revision":3}`)}}}}
	if err := savePendingStart(dir, input); err != nil {
		t.Fatal(err)
	}
	if err := savePendingStart(dir, nil); err != nil {
		t.Fatal(err)
	}
	saved, err := loadPendingStart(dir)
	if err != nil || len(saved.PendingStops) != 1 || saved.PendingStops[0].Request.RequestID != "original-stop" {
		t.Fatalf("lost cancellation: %+v %v", saved, err)
	}
	if err = savePendingStart(dir, &contracts.ServiceStartInput{PendingStops: []contracts.PendingStop{}}); err == nil {
		t.Fatal("unresolved stop dropped")
	}
	input.PendingStops[0].Request.Input = json.RawMessage(`{"expected_revision":4}`)
	if err = savePendingStart(dir, input); err == nil {
		t.Fatal("unresolved stop changed")
	}
}

func TestLegacyRecoveryRejectsRetiredCheckBeforeFormatUpgrade(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		t.Run(adapter, func(t *testing.T) {
			dir := t.TempDir()
			s, err := storage.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			r := runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Revision: 1, Input: contracts.CheckInput{Execution: contracts.Execution{Adapter: adapter, Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{"github_auth":null}`)}}}}
			body, _ := json.Marshal(r)
			if _, err := db.Exec(`INSERT INTO checks VALUES('original',1,?); UPDATE metadata SET format='steerrunner-state/v3'; PRAGMA user_version=3;`, body); err != nil {
				t.Fatal(err)
			}
			if qualifyLegacyRecovery(dir) == nil {
				t.Fatal("retired Check authentication adopted before semantic migration")
			}
			var format, saved string
			var version int
			if db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || db.QueryRow(`PRAGMA user_version`).Scan(&version) != nil || format != "steerrunner-state/v3" || version != 3 || db.QueryRow(`SELECT body FROM checks WHERE id='original'`).Scan(&saved) != nil || saved != string(body) {
				t.Fatal("refused migration changed original format or Check bytes")
			}
			backups, err := filepath.Glob(filepath.Join(dir, "runner-state-v3-*.backup.db"))
			if err != nil || len(backups) != 0 {
				t.Fatal("refused migration started an upgrade", backups, err)
			}
		})
	}
}

func TestLegacyRecoveryQualifiesContractsBeforeChangingFormat(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`UPDATE metadata SET format='steerrunner-state/v3'; PRAGMA user_version=3;`); err != nil {
		t.Fatal(err)
	}
	if err = qualifyLegacyRecovery(dir); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO runs VALUES('unsupported',1,'{"id":"unsupported","revision":1,"request":{"workflow":{"id":"future","version":99}}}')`); err != nil {
		t.Fatal(err)
	}
	if qualifyLegacyRecovery(dir) == nil {
		t.Fatal("unknown contract adopted before migration")
	}
	var format string
	if db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || format != "steerrunner-state/v3" {
		t.Fatal("rejected source marker changed")
	}
}
