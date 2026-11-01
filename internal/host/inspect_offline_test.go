package host

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineInspectPreservesOriginalSnapshotWithoutStarting(t *testing.T) {
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
	profile, _ := workflow.Lookup(workflow.SingleExecution, 1)
	q := contracts.Request{Protocol: contracts.Protocol, RequestID: "original-submit", Input: json.RawMessage(`{"title":"original"}`)}
	cmd := storage.NewCommand("submit", "original-run", q)
	r := &runstate.Record{ID: "original-run", Revision: 23, InputDigest: "original-input", Status: "running", Allowed: true, Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: workflow.SingleExecution, Version: 1}}, Workflow: workflow.Snapshot{Config: &profile, StepID: "execute"}, CommandStates: map[string]string{cmd.ID: "pending"}}
	if err = store.Commit(r, 0, &cmd); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []Options{{Command: "inspect", RunID: r.ID}, {Command: "inspect", RequestID: q.RequestID}, {Command: "inspect", CommandID: cmd.ID}} {
		reply := InspectOffline(context.Background(), dir, opts)
		if !reply.OK {
			t.Fatal(reply)
		}
		raw, _ := json.Marshal(reply.Data)
		var result struct {
			Run     contracts.RunView      `json:"run"`
			Command *contracts.CommandView `json:"command"`
		}
		if json.Unmarshal(raw, &result) != nil || result.Run.RunID != r.ID || result.Run.Revision != 23 || result.Run.InputDigest != r.InputDigest || result.Run.Status != "running" {
			t.Fatal(string(raw))
		}
		if opts.RequestID != "" && (result.Command == nil || result.Command.RequestID != q.RequestID) {
			t.Fatal(string(raw))
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("offline query changed persisted state", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "service", "endpoint.json")); !os.IsNotExist(err) {
		t.Fatal("offline query published service", err)
	}
	lease, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
	if err != nil || busy {
		t.Fatal(err, busy)
	}
	reply := InspectOffline(context.Background(), dir, Options{Command: "inspect", RunID: r.ID})
	lease.Close()
	if reply.OK || reply.Error.Code != "directory_conflict" {
		t.Fatal("query ignored writer lease", reply)
	}
	reply = InspectOffline(context.Background(), dir, Options{Command: "inspect", RequestID: "never-accepted"})
	if reply.OK || reply.Error.Code != "request_not_found" {
		t.Fatal(reply)
	}
	if err := os.WriteFile(bootstrapPath(dir), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	reply = InspectOffline(context.Background(), dir, Options{Command: "inspect", RunID: r.ID})
	if reply.OK || reply.Error.Code != "startup_unverified" {
		t.Fatal(reply)
	}
}

func TestOfflineInspectRejectsInvalidSelectorsAndForeignState(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "service"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service", "writer.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	body := []byte("foreign database bytes")
	if err := os.WriteFile(filepath.Join(dir, "runner.db"), body, 0600); err != nil {
		t.Fatal(err)
	}
	for _, opts := range []Options{{Command: "inspect"}, {Command: "inspect", RunID: "r", RequestID: "q"}, {Command: "inspect", RunID: "r", RefreshPR: true}} {
		r := InspectOffline(context.Background(), dir, opts)
		if r.OK || r.Error.Code != "invalid_arguments" {
			t.Fatal(r)
		}
	}
	r := InspectOffline(context.Background(), dir, Options{Command: "inspect", RunID: "r"})
	if r.OK || r.Error.Code != "unsupported_schema" {
		t.Fatal(r)
	}
	after, err := os.ReadFile(filepath.Join(dir, "runner.db"))
	if err != nil || !bytes.Equal(body, after) {
		t.Fatal("query mutated foreign state", err)
	}
}
