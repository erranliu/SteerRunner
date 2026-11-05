package host

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func inspectionFixture(t *testing.T) (string, *runstate.Record, storage.Command) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	config, _ := workflow.Lookup(workflow.SingleExecution, 1)
	command := storage.NewCommand("submit", "original-run", contracts.Request{Protocol: contracts.Protocol, RequestID: "original-request", Input: json.RawMessage(`{}`)})
	r := &runstate.Record{ID: command.RunID, Revision: 7, Allowed: true, Status: "running", InputDigest: "original-input", Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: workflow.SingleExecution, Version: 1}}, Workflow: workflow.Snapshot{Config: &config, StepID: "execute"}, CommandStates: map[string]string{command.ID: "pending"}}
	if err := store.Commit(r, 0, &command); err != nil {
		t.Fatal(err)
	}
	return dir, r, command
}

func inspectionSQL(t *testing.T, dir, query string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func inspectionFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			rel, _ := filepath.Rel(dir, path)
			// The existing mode=ro SQLite connection may create these transient
			// WAL coordination files even though it writes no durable data.
			if rel == "runner.db-shm" {
				return nil
			}
			if rel == "runner.db-wal" {
				if len(data) != 0 {
					return errors.New("offline fixture acquired nonempty WAL data")
				}
				return nil
			}
			files[rel] = data
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestOfflineInspectionMatchesResidentAndOriginalResponseShape(t *testing.T) {
	dir, r, command := inspectionFixture(t)
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	stopRequest := contracts.Request{Protocol: contracts.Protocol, RequestID: "service-stop", Input: json.RawMessage(`{"expected_instance_id":"original-service"}`)}
	if _, _, err := store.AcceptStop(stopRequest, "original-service"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	// Preserve the old command-only shape even when its Run is absent.
	inspectionSQL(t, dir, `INSERT INTO commands(request_id,command_id,command,instance_id,digest,response,action_status,run_id) VALUES('dangling-request','dangling-command','submit','','digest','{}','unknown','absent-run')`)
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := application.New(context.Background(), dir, store, log.New(io.Discard, "", 0))
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	runView := r.View()
	commandView := contracts.CommandView{RequestID: command.RequestID, CommandID: command.ID, Command: "submit", ReceiptStatus: "accepted", ActionStatus: "pending", RunID: &r.ID}
	cases := []struct {
		name string
		opts Options
		want contracts.Response
	}{
		{"run", Options{Command: "inspect", RunID: r.ID}, contracts.Success("", map[string]any{"command": nil, "run": runView})},
		{"request", Options{Command: "inspect", RequestID: command.RequestID}, contracts.Success("", map[string]any{"command": commandView, "run": runView})},
		{"command", Options{Command: "inspect", CommandID: command.ID}, contracts.Success("", map[string]any{"command": commandView, "run": runView})},
		{"missing-run", Options{Command: "inspect", RunID: "missing"}, contracts.Rejected("", contracts.Fail(5, "run_not_found", "Run was not found."))},
		{"missing-request", Options{Command: "inspect", RequestID: "missing"}, contracts.Rejected("", contracts.Fail(5, "request_not_found", "The original command was not found."))},
		{"missing-command", Options{Command: "inspect", CommandID: "missing"}, contracts.Rejected("", contracts.Fail(5, "command_not_found", "The original command was not found."))},
	}
	for _, selector := range []string{"request_id", "command_id"} {
		for _, requestID := range []string{"service-stop", "dangling-request"} {
			value, err := store.Command("request_id", requestID)
			if err != nil {
				t.Fatal(err)
			}
			opts := Options{Command: "inspect", RequestID: requestID}
			if selector == "command_id" {
				opts.RequestID, opts.CommandID = "", value.CommandID
			}
			cases = append(cases, struct {
				name string
				opts Options
				want contracts.Response
			}{selector + "-" + requestID, opts, contracts.Success("", map[string]any{"command": value, "run": nil})})
		}
	}
	resident := make([][]byte, len(cases))
	for i, tc := range cases {
		response := engine.Handle("inspect", tc.opts.RunID, tc.opts.RequestID, tc.opts.CommandID, "", 0, nil)
		if response.OK {
			data := response.Data.(map[string]any)
			if tc.opts.RunID != "" {
				if _, ok := data["run"].(contracts.RunView); !ok || data["command"] != nil {
					t.Fatal("resident Run projection changed its value shape")
				}
			} else if _, ok := data["command"].(contracts.CommandView); !ok {
				t.Fatal("resident command projection changed its value shape")
			}
		}
		resident[i], err = json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	before := inspectionFiles(t, dir)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(InspectOffline(context.Background(), dir, tc.opts))
			want, _ := json.Marshal(tc.want)
			if err != nil || !bytes.Equal(got, resident[i]) || !bytes.Equal(got, want) {
				t.Fatalf("response changed: offline=%s resident=%s expected=%s error=%v", got, resident[i], want, err)
			}
		})
	}
	if !reflect.DeepEqual(before, inspectionFiles(t, dir)) {
		t.Fatal("offline inspect changed durable bytes or created execution files")
	}
}

func TestOfflineInspectionPreservesDecodeAndQueryFailureBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, query, code, message string
	}{
		{"unknown-field", `INSERT INTO runs VALUES('unrelated',1,'{"id":"unrelated","unsupported":true}')`, "service_unavailable", "Original persisted projection is unavailable."},
		{"malformed", `INSERT INTO runs VALUES('unrelated',1,'{')`, "service_unavailable", "Original persisted projection is unavailable."},
		{"trailing", `INSERT INTO runs VALUES('unrelated',1,'{"id":"unrelated"} {}')`, "service_unavailable", "Original persisted projection is unavailable."},
		{"runs-query", `DROP TABLE runs`, "service_unavailable", "Original persisted projection is unavailable."},
		{"command-query", `DROP TABLE commands`, "internal_error", "Cannot commit or read the requested operation."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, r, _ := inspectionFixture(t)
			inspectionSQL(t, dir, tc.query)
			before := inspectionFiles(t, dir)
			selectors := []Options{{Command: "inspect", RequestID: "never-accepted"}, {Command: "inspect", CommandID: "never-accepted"}}
			if tc.name != "command-query" {
				selectors = append(selectors, Options{Command: "inspect", RunID: r.ID})
			}
			for _, opts := range selectors {
				got := InspectOffline(context.Background(), dir, opts)
				if got.OK || got.Error.Code != tc.code || got.Error.Message != tc.message {
					t.Fatal("changed strict decoding/selector error priority", got)
				}
			}
			if !reflect.DeepEqual(before, inspectionFiles(t, dir)) {
				t.Fatal("failed offline read changed original state")
			}
		})
	}
}

func TestOfflineInspectionRejectsExecutionBindingsAndExpiredContext(t *testing.T) {
	dir, r, _ := inspectionFixture(t)
	before := inspectionFiles(t, dir)
	for _, opts := range []Options{
		{Command: "inspect", RunID: r.ID, ExpectedInstanceID: "service"},
		{Command: "inspect", RunID: r.ID, ExpectedBinding: &contracts.ExpectedServiceBinding{}},
		{Command: "stop", RunID: r.ID},
	} {
		got := InspectOffline(context.Background(), dir, opts)
		if got.OK || got.Error.Code != "invalid_arguments" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := InspectOffline(ctx, dir, Options{Command: "inspect", RunID: r.ID})
	if got.OK || got.Error.Code != "service_unavailable" || got.Error.Message != "Offline query deadline expired." {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(before, inspectionFiles(t, dir)) {
		t.Fatal("rejected offline query changed state")
	}
}
