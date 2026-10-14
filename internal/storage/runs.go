package storage

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	RequestID, ID, RunID, Name, Digest string
	Response                           contracts.Response
	Status                             string
}

func RequestDigest(command, target string, input contracts.Request) string {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(input.Input))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	raw, _ := json.Marshal(struct {
		Protocol, Command, Target string
		Input                     any
	}{input.Protocol, command, target, value})
	return digestBytes(raw)
}
func (s *Store) Original(requestID, digest string) (contracts.Response, bool, error) {
	var previous string
	var raw []byte
	var status string
	err := s.db.QueryRow(`SELECT digest,response,action_status FROM commands WHERE request_id=?`, requestID).Scan(&previous, &raw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.Response{}, false, nil
	}
	if err != nil {
		return contracts.Response{}, false, err
	}
	if previous != digest {
		return contracts.Rejected(requestID, contracts.Fail(5, "request_conflict", "request_id belongs to different command, target or input.")), true, nil
	}
	var response contracts.Response
	if err = json.Unmarshal(raw, &response); err != nil {
		return response, true, err
	}
	if data, ok := response.Data.(map[string]any); ok {
		data["action_status"] = status
	}
	return response, true, nil
}
func NewCommand(name, target string, input contracts.Request) Command {
	id := contracts.NewID()
	response := contracts.Success(input.RequestID, map[string]any{"command_id": id, "run_id": target, "receipt_status": "accepted", "action_status": "pending"})
	return Command{RequestID: input.RequestID, ID: id, RunID: target, Name: name, Digest: RequestDigest(name, target, input), Response: response, Status: "pending"}
}

// Commit writes the aggregate, optional accepted command and events together.
// The caller must publish its in-memory value only after Commit succeeds.
func (s *Store) Commit(r *runstate.Record, expected uint64, command *Command, events ...string) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if expected == 0 {
		_, err = tx.Exec(`INSERT INTO runs(id,revision,body) VALUES(?,?,?)`, r.ID, r.Revision, raw)
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE runs SET revision=?,body=? WHERE id=? AND revision=?`, r.Revision, raw, r.ID, expected)
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				err = errors.New("run revision changed")
			}
		}
	}
	if err != nil {
		return err
	}
	if command != nil {
		body, e := json.Marshal(command.Response)
		if e != nil {
			return e
		}
		_, err = tx.Exec(`INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status,run_id) VALUES(?,?,?,?,?,?,?,?)`, command.RequestID, command.ID, command.Name, "", command.Digest, body, command.Status, command.RunID)
		if err != nil {
			return err
		}
	}
	for id, status := range r.CommandStates {
		if _, err = tx.Exec(`UPDATE commands SET action_status=? WHERE command_id=? AND run_id=?`, status, id, r.ID); err != nil {
			return err
		}
	}
	for _, kind := range events {
		event := contracts.Event{EventID: contracts.NewID(), RunID: r.ID, Revision: r.Revision, Type: kind, AttemptID: r.Snapshot.Session.AttemptID, CreatedAt: time.Now().UTC()}
		if kind == "result_committed" || kind == "final_unavailable" {
			event.Final = r.View().Final
		}
		if command != nil {
			event.CommandID = command.ID
		}
		body, _ := json.Marshal(event)
		if _, err = tx.Exec(`INSERT INTO events(run_id,event_id,body) VALUES(?,?,?)`, r.ID, event.EventID, body); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Runs() ([]*runstate.Record, error) {
	rows, err := s.db.Query(`SELECT body FROM runs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*runstate.Record
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r runstate.Record
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		result = append(result, &r)
	}
	return result, rows.Err()
}
func (s *Store) Events(run, after string, limit int) (contracts.EventPage, error) {
	page := contracts.EventPage{Events: []contracts.Event{}}
	var sequence int64
	if after != "" {
		raw, err := base64.RawURLEncoding.DecodeString(after)
		parts := strings.Split(string(raw), ":")
		if err != nil || len(parts) != 3 || parts[0] != "v1" || parts[1] != run {
			return page, contracts.Fail(2, "invalid_cursor", "Cursor does not belong to this run.")
		}
		sequence, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil || sequence < 0 {
			return page, contracts.Fail(2, "invalid_cursor", "Invalid event cursor.")
		}
		if sequence > 0 {
			var owner string
			if s.db.QueryRow(`SELECT run_id FROM events WHERE seq=?`, sequence).Scan(&owner) != nil || owner != run {
				return page, contracts.Fail(2, "invalid_cursor", "Unknown event cursor.")
			}
		}
	}
	rows, err := s.db.Query(`SELECT seq,body FROM events WHERE run_id=? AND seq>? ORDER BY seq LIMIT ?`, run, sequence, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int64
		var body []byte
		if err := rows.Scan(&seq, &body); err != nil {
			return page, err
		}
		if len(page.Events) == limit {
			page.HasMore = true
			break
		}
		var event contracts.Event
		if err := json.Unmarshal(body, &event); err != nil {
			return page, err
		}
		page.Events = append(page.Events, event)
		sequence = seq
	}
	page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("v1:%s:%d", run, sequence)))
	return page, rows.Err()
}
