package storage

import (
	"database/sql"
	"errors"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func qualifyRecoverySource(db *sql.DB) error {
	if err := validateReadableFormat(db, true); err != nil {
		return err
	}
	var integrity string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&integrity); err != nil || integrity != "ok" {
		return errors.New("original recovery database integrity is unverified")
	}
	s := &Store{db: db}
	if _, err := s.Runs(); err != nil {
		return contracts.Fail(3, "unsupported_schema", "Original Run contains unsupported durable fields; no format upgrade performed.")
	}
	if _, err := s.Checks(); err != nil {
		return contracts.Fail(3, "unsupported_schema", "Original Check contains unsupported durable fields; no format upgrade performed.")
	}
	// Qualify the existing tables/columns as well as the aggregate JSON. No
	// reconstruction or normalization is needed for this semantic-only upgrade.
	for _, query := range []string{
		`SELECT request_id,command_id,command,instance_id,digest,response,action_status,run_id,check_id FROM commands LIMIT 0`,
		`SELECT seq,run_id,event_id,body FROM events LIMIT 0`,
		`SELECT id,revision,body FROM runs LIMIT 0`,
		`SELECT id,revision,body FROM checks LIMIT 0`,
	} {
		rows, err := db.Query(query)
		if err != nil {
			return err
		}
		if err = rows.Close(); err != nil {
			return err
		}
	}
	return nil
}
