package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"reflect"
)

func (s *Store) Checks() ([]*runstate.CheckRecord, error) {
	rows, err := s.db.Query(`SELECT body FROM checks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*runstate.CheckRecord{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r runstate.CheckRecord
		if err := b.DecodeStrictJSON(raw, &r, "trailing durable Check data"); err != nil {
			return nil, err
		}
		result = append(result, &r)
	}
	return result, rows.Err()
}

func NewCheckCommand(name, checkID string, request contracts.Request) Command {
	id := contracts.NewID()
	var target *string
	if checkID != "" {
		target = &checkID
	}
	return Command{RequestID: request.RequestID, ID: id, Name: name, Digest: RequestDigest(name, "", request), Status: "pending", Response: contracts.Success(request.RequestID, contracts.CheckReceipt{CommandID: id, CheckID: target, ReceiptStatus: "accepted", ActionStatus: "pending"})}
}

// The check, original receipt and command outcomes share one transaction.
// A rejected valid request uses the same commands table, with no fake check.
func (s *Store) CommitCheck(r *runstate.CheckRecord, expected uint64, command *Command) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	checkID := ""
	if r != nil {
		checkID = r.View.CheckID
		if r.View.Revision != expected+1 {
			return errors.New("check revision is not consecutive")
		}
		if expected > 0 {
			var raw []byte
			if err := tx.QueryRow(`SELECT body FROM checks WHERE id=? AND revision=?`, checkID, expected).Scan(&raw); err != nil {
				return err
			}
			var prior runstate.CheckRecord
			if err := json.Unmarshal(raw, &prior); err != nil {
				return err
			}
			if err := contracts.ValidateCheckUpdate(prior.View, r.View); err != nil {
				return err
			}
			if prior.Nonce != r.Nonce || prior.View.Actual.AttemptID != r.View.Actual.AttemptID || prior.View.Actual.Generation != r.View.Actual.Generation {
				return errors.New("check attempt changed")
			}
			if prior.AgentSettled && (!r.AgentSettled || prior.View.Result != r.View.Result || !reflect.DeepEqual(prior.View.Items, r.View.Items)) {
				return errors.New("settled Agent capability results cannot change during resource settlement")
			}
		}
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if expected == 0 {
			_, err = tx.Exec(`INSERT INTO checks(id,revision,body) VALUES(?,?,?)`, checkID, r.View.Revision, raw)
		} else {
			var result sql.Result
			result, err = tx.Exec(`UPDATE checks SET revision=?,body=? WHERE id=? AND revision=?`, r.View.Revision, raw, checkID, expected)
			if err == nil {
				n, _ := result.RowsAffected()
				if n != 1 {
					err = errors.New("check revision changed")
				}
			}
		}
		if err != nil {
			return err
		}
	}
	if command != nil {
		raw, err := json.Marshal(command.Response)
		if err != nil {
			return err
		}
		var target *string
		if checkID != "" {
			target = &checkID
		}
		if _, err = tx.Exec(`INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status,check_id) VALUES(?,?,?,?,?,?,?,?)`, command.RequestID, command.ID, command.Name, "", command.Digest, raw, command.Status, target); err != nil {
			return err
		}
	}
	if r != nil {
		for id, status := range r.CommandStates {
			if _, err = tx.Exec(`UPDATE commands SET action_status=? WHERE command_id=? AND check_id=?`, status, id, checkID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) CheckCommand(requestID string) (contracts.CheckCommand, error) {
	var value contracts.CheckCommand
	var raw []byte
	err := s.db.QueryRow(`SELECT request_id,command_id,command,action_status,check_id,response FROM commands WHERE request_id=? AND command IN ('environment-check start','environment-check stop')`, requestID).Scan(&value.RequestID, &value.CommandID, &value.Command, &value.ActionStatus, &value.CheckID, &raw)
	if err != nil {
		return value, err
	}
	var response contracts.Response
	if err := json.Unmarshal(raw, &response); err != nil {
		return value, err
	}
	value.ReceiptStatus = "accepted"
	value.Error = response.Error
	if !response.OK {
		value.ReceiptStatus = "rejected"
	}
	return value, nil
}
