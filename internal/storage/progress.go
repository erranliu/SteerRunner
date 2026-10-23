package storage

import (
	"encoding/json"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

// The engine serializes this update with ordinary Commit. It deliberately
// changes neither revision nor events/commands and uses the original row.
func (s *Store) SaveProgress(r *runstate.Record) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE runs SET body=? WHERE id=? AND revision=?`, raw, r.ID, r.Revision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		err = fmt.Errorf("progress row changed")
	}
	return err
}
