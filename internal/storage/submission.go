package storage

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// BeginSubmit records the exact request identity before repository or material preparation.
// Its response remains unknown until the admission is rejected or committed with a Run.
func (s *Store) BeginSubmit(input contracts.Request, digest string) (Command, error) {
	c := Command{RequestID: input.RequestID, ID: contracts.NewID(), Name: "submit", Digest: digest, Status: "pending", admission: true}
	pending := contracts.Rejected(c.RequestID, contracts.Fail(6, "request_outcome_unknown", "Submission preparation is in progress; inspect the original request_id."))
	body, err := json.Marshal(pending)
	if err != nil {
		return c, err
	}
	_, err = s.db.Exec(`INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status) VALUES(?,?,?,?,?,?,?)`,
		c.RequestID, c.ID, c.Name, "", c.Digest, body, "preparing")
	return c, err
}

func (c *Command) AcceptSubmit(runID string) {
	c.bindRun(runID)
}

func (s *Store) RejectSubmit(c Command, response contracts.Response) error {
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return updateSubmit(s.db, c, body, "rejected", nil)
}

type submitWriter interface {
	Exec(string, ...any) (sql.Result, error)
}

func updateSubmit(writer submitWriter, c Command, body []byte, status string, runID any) error {
	result, err := writer.Exec(`UPDATE commands SET response=?,action_status=?,run_id=?
		WHERE request_id=? AND command_id=? AND command='submit' AND digest=? AND action_status='preparing' AND run_id IS NULL`,
		body, status, runID, c.RequestID, c.ID, c.Digest)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return errors.New("submit admission is no longer preparing")
	}
	return err
}

// ReconcileSubmissions runs under the exclusive writer lease before the service
// listens. A preparing command has no Run because both are committed atomically.
func (s *Store) ReconcileSubmissions() error {
	_, err := s.db.Exec(`UPDATE commands
		SET action_status='rejected', response=json_object('protocol', ?, 'ok', json('false'),
			'request_id', request_id, 'data', NULL, 'error', json_object('code', 'invalid_state',
			'message', 'Service stopped before submission preparation completed; no Run was accepted.'))
		WHERE command='submit' AND action_status='preparing' AND run_id IS NULL`, contracts.Protocol)
	return err
}
