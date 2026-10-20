package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRecoveryUpgradePreservesWALAndOriginalRecords(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const original = `{"id":"original","revision":7,"new_inputs_allowed":true,"pending_resume":"saved-resume","command_states":{"saved-resume":"pending"}}`
	if _, err = s.db.Exec(`INSERT INTO runs VALUES('original',7,?); UPDATE metadata SET format='steerrunner-state/v3'; PRAGMA user_version=3;`, original); err != nil {
		t.Fatal(err)
	}
	// Keep a WAL reader open while closing the writer; migration must use a
	// SQLite snapshot, not a filesystem copy of the main database.
	reader, err := OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var body string
	if err = s.db.QueryRow(`SELECT body FROM runs WHERE id='original'`).Scan(&body); err != nil || body != original {
		t.Fatalf("original aggregate changed: %s %v", body, err)
	}
	if err = validateFormat(s.db); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "runner-state-v3-*.backup.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup: %v %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var format string
	var saved string
	if backup.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || format != "steerrunner-state/v3" || backup.QueryRow(`SELECT body FROM runs WHERE id='original'`).Scan(&saved) != nil || saved != original {
		t.Fatal("backup lost original schema or WAL records")
	}
}

func TestRecoveryUpgradeFailureKeepsOriginalFormat(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`UPDATE metadata SET format='steerrunner-state/v3'; PRAGMA user_version=3; CREATE TRIGGER reject_upgrade BEFORE UPDATE ON metadata BEGIN SELECT RAISE(ABORT,'injected metadata failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if upgraded, err := Open(dir); err == nil {
		upgraded.Close()
		t.Fatal("failed transaction accepted")
	}
	r, err := OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var format string
	var version int
	if r.db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || r.db.QueryRow(`PRAGMA user_version`).Scan(&version) != nil || format != "steerrunner-state/v3" || version != 3 {
		t.Fatal("partial migration")
	}
}

func TestRecoveryUpgradeRejectsUnknownDurableFieldsBeforeMarkerChange(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`INSERT INTO runs VALUES('future',1,'{"id":"future","revision":1,"future_authorization":true}'); UPDATE metadata SET format='steerrunner-state/v3'; PRAGMA user_version=3;`)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if upgraded, err := Open(dir); err == nil {
		upgraded.Close()
		t.Fatal("unknown durable semantics were upgraded")
	}
	r, err := OpenRecoveryReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var format string
	if r.db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || format != "steerrunner-state/v3" {
		t.Fatal("source marker changed")
	}
}
