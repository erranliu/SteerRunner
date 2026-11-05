package application

import (
	"database/sql"
	"errors"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// InspectPersisted reads the same strict, complete Run set used at startup,
// without constructing an execution engine. The host owns selector validation,
// the offline writer lease and the current-format read-only store lifetime.
// Run decoding errors belong to the host's persisted-projection fault boundary;
// command query errors retain the resident application's response mapping.
func InspectPersisted(store *storage.Store, runID, requestID, commandID string) (contracts.Response, error) {
	records, err := store.Runs()
	if err != nil {
		return contracts.Response{}, err
	}
	runs := make(map[string]*runstate.Record, len(records))
	for _, r := range records {
		runs[r.ID] = r
	}
	if runID == "" {
		return inspectCommand(store, runs, requestID, commandID), nil
	}
	if r := runs[runID]; r != nil {
		return inspectionView(nil, r), nil
	}
	return contracts.Rejected("", contracts.Fail(5, "run_not_found", "Run was not found.")), nil
}

// The resident caller holds Engine.mu across both command and Run projection.
func inspectCommand(store *storage.Store, runs map[string]*runstate.Record, requestID, commandID string) contracts.Response {
	selector, target := "request_id", requestID
	if commandID != "" {
		selector, target = "command_id", commandID
	}
	value, err := store.Command(selector, target)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.Rejected("", contracts.Fail(5, selector[:len(selector)-3]+"_not_found", "The original command was not found."))
	}
	if err != nil {
		return contracts.Rejected("", fault(err))
	}
	var r *runstate.Record
	if value.RunID != nil {
		r = runs[*value.RunID]
	}
	return inspectionView(&value, r)
}

func inspectionView(command *contracts.CommandView, r *runstate.Record) contracts.Response {
	if command == nil {
		return contracts.Success("", map[string]any{"command": nil, "run": r.View()})
	}
	var view *contracts.RunView
	if r != nil {
		value := r.View()
		view = &value
	}
	return contracts.Success("", map[string]any{"command": *command, "run": view})
}
