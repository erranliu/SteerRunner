//go:build windows

package integration

import (
	"database/sql"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"path/filepath"
	"testing"
)

func compatibilitySQL(t *testing.T, dir, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(statement); err != nil {
		t.Fatal(err)
	}
}

func compatibilityRecord(t *testing.T, dir string) runstate.Record {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var body []byte
	if err := db.QueryRow(`SELECT body FROM runs`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var record runstate.Record
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return record
}
