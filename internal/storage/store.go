// Package storage is the service-owned persistence implementation.
package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

const Format = "steerrunner-state/v6"

type Store struct{ db *sql.DB }

func Open(dir string) (*Store, error) {
	databasePath := filepath.Join(dir, "runner.db")
	_, statErr := os.Stat(databasePath)
	fresh := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !fresh {
		return nil, statErr
	}
	if !fresh {
		file, err := os.Open(databasePath)
		if err != nil {
			return nil, err
		}
		var header [16]byte
		_, readErr := file.ReadAt(header[:], 0)
		file.Close()
		if readErr != nil || string(header[:]) != "SQLite format 3\x00" {
			return nil, contracts.Fail(3, "unsupported_schema", "State directory contains an unsupported storage format; use a fresh state/v6 --state-dir.")
		}
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { db.Close(); return nil, err }
	if _, err := db.Exec(`PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		return fail(err)
	}
	if fresh {
		if _, err := db.Exec(`
BEGIN IMMEDIATE;
CREATE TABLE metadata (format TEXT NOT NULL);
INSERT INTO metadata VALUES ('steerrunner-state/v6');
CREATE TABLE commands (
 request_id TEXT PRIMARY KEY, command_id TEXT NOT NULL UNIQUE,
 command TEXT NOT NULL, instance_id TEXT NOT NULL, digest TEXT NOT NULL,
 response BLOB NOT NULL, action_status TEXT NOT NULL, run_id TEXT
);
CREATE TABLE runs(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, body BLOB NOT NULL);
CREATE TABLE events(seq INTEGER PRIMARY KEY AUTOINCREMENT,run_id TEXT NOT NULL,event_id TEXT NOT NULL UNIQUE,body BLOB NOT NULL);
CREATE INDEX events_run ON events(run_id,seq);
PRAGMA user_version=6;
COMMIT;`); err != nil {
			return fail(err)
		}
	}
	if err := validateFormat(db); err != nil {
		return fail(err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;`); err != nil {
		return fail(err)
	}
	return &Store{db: db}, nil
}

// OpenReader never creates or migrates storage. The host must hold the writer
// lease while using this snapshot for offline resource verification.
func OpenReader(dir string) (*Store, error) {
	return openReader(dir)
}

func openReader(dir string) (*Store, error) {
	path, err := filepath.Abs(filepath.Join(dir, "runner.db"))
	if err != nil {
		return nil, err
	}
	value := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		value = "/" + value
	}
	uri := url.URL{Scheme: "file", Path: value}
	db, err := sql.Open("sqlite", uri.String()+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := validateFormat(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func validateFormat(db *sql.DB) error {
	var format string
	var version int
	var count int
	if db.QueryRow(`SELECT count(*) FROM metadata`).Scan(&count) != nil || count != 1 || db.QueryRow(`SELECT format FROM metadata`).Scan(&format) != nil || db.QueryRow(`PRAGMA user_version`).Scan(&version) != nil || (format != Format || version != 6) {
		return contracts.Fail(3, "unsupported_schema", "State directory contains an unsupported storage format; use a fresh state/v6 --state-dir.")
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func StopDigest(input contracts.Request) string {
	var value any
	_ = json.Unmarshal(input.Input, &value)
	canonical, _ := json.Marshal(struct {
		Protocol string `json:"protocol"`
		Command  string `json:"command"`
		Input    any    `json:"input"`
	}{input.Protocol, "service stop", value})
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

// AcceptStop commits the original receipt before any shutdown action. A retry
// returns that receipt, even if it targets a previous service instance.
func (s *Store) AcceptStop(input contracts.Request, instance string) (contracts.Response, bool, error) {
	digest := StopDigest(input)
	tx, err := s.db.Begin()
	if err != nil {
		return contracts.Response{}, false, err
	}
	defer tx.Rollback()
	var originalDigest string
	var original []byte
	var actionStatus string
	err = tx.QueryRow(`SELECT digest, response, action_status FROM commands WHERE request_id=?`, input.RequestID).Scan(&originalDigest, &original, &actionStatus)
	if err == nil {
		if digest != originalDigest {
			return contracts.Rejected(input.RequestID, contracts.Fail(5, "request_conflict", "request_id belongs to different input.")), false, nil
		}
		var response contracts.Response
		if json.Unmarshal(original, &response) != nil {
			return contracts.Response{}, false, errors.New("invalid persisted command receipt")
		}
		if data, ok := response.Data.(map[string]any); ok {
			data["action_status"] = actionStatus
		}
		return response, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return contracts.Response{}, false, err
	}
	var stop contracts.ServiceStopInput
	if contracts.DecodeStrict(input.Input, &stop) != nil || stop.ExpectedInstanceID != instance {
		return contracts.Rejected(input.RequestID, contracts.Fail(5, "stale_instance", "Stop target is not the current service instance.")), false, nil
	}
	commandID := contracts.NewID()
	response := contracts.Success(input.RequestID, map[string]any{
		"command_id": commandID, "run_id": nil, "receipt_status": "accepted",
		"instance_id": instance, "action_status": "pending",
	})
	body, err := json.Marshal(response)
	if err != nil {
		return contracts.Response{}, false, err
	}
	if _, err := tx.Exec(`INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status) VALUES (?,?,?,?,?,?,?)`, input.RequestID, commandID, "service stop", instance, digest, body, "pending"); err != nil {
		return contracts.Response{}, false, err
	}
	if err := revokeUsagePermit(tx, instance); err != nil {
		return contracts.Response{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return contracts.Response{}, false, err
	}
	return response, true, nil
}

func (s *Store) CompleteStop(requestID string) error {
	_, err := s.db.Exec(`UPDATE commands SET action_status='completed' WHERE request_id=?`, requestID)
	return err
}

// Only called after the new service owns the exclusive writer lock: the old
// writer has ended. This reconciles shutdown receipts, never native execution.
func (s *Store) ReconcileStops(instance string) error {
	_, err := s.db.Exec(`UPDATE commands SET action_status='completed' WHERE command='service stop' AND instance_id<>? AND action_status<>'completed'`, instance)
	return err
}

func (s *Store) Command(selector, id string) (contracts.CommandView, error) {
	var value contracts.CommandView
	column := "request_id"
	if selector == "command_id" {
		column = "command_id"
	}
	err := s.db.QueryRow(`SELECT request_id, command_id, command, action_status,run_id FROM commands WHERE `+column+`=?`, id).Scan(&value.RequestID, &value.CommandID, &value.Command, &value.ActionStatus, &value.RunID)
	if err != nil {
		return value, err
	}
	value.ReceiptStatus = "accepted"
	return value, nil
}

func digestBytes(raw []byte) string { value := sha256.Sum256(raw); return hex.EncodeToString(value[:]) }
