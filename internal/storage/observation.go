package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// Observe reads the durable projection and bounded event page in one read-only
// transaction. No provider/native query or service startup is performed here.
func (s *Store) Observe(ctx context.Context, id, after string, limit int) (*runstate.Record, contracts.EventPage, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, contracts.EventPage{}, err
	}
	defer tx.Rollback()
	var body []byte
	var revision uint64
	if err := tx.QueryRow(`SELECT revision,body FROM runs WHERE id=?`, id).Scan(&revision, &body); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = contracts.Fail(5, "run_not_found", "Run was not found.")
		}
		return nil, contracts.EventPage{}, err
	}
	var record runstate.Record
	if err := b.DecodeStrictJSON(body, &record, "trailing durable Run data"); err != nil {
		return nil, contracts.EventPage{}, err
	}
	if record.ID != id || record.Revision != revision {
		return nil, contracts.EventPage{}, errors.New("durable Run identity/revision mismatch")
	}
	page, err := eventsFrom(tx, id, after, limit)
	if err != nil {
		return nil, page, err
	}
	for _, event := range page.Events {
		if event.RunID != id || event.Revision > revision {
			return nil, page, errors.New("event exceeds observed Run snapshot")
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, page, err
	}
	return &record, page, nil
}
