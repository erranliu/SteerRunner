package storage

import "github.com/erranliu/SteerRunner/internal/contracts"

// BindTakeoverRequests adds only existing accepted command facts, allowing a
// caller with a lost submit response to correlate before workers are started.
func (s *Store) BindTakeoverRequests(items []contracts.TakeoverItem) []contracts.TakeoverItem {
	for i := range items {
		item := &items[i]
		err := s.db.QueryRow(`SELECT request_id,digest FROM commands WHERE run_id=? AND command='submit'`, item.ID).Scan(&item.RequestID, &item.RequestDigest)
		if err != nil {
			item.Status, item.Diagnostic = "unsupported", "original accepted creation command is unavailable"
		}
	}
	return items
}
