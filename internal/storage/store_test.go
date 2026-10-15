package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func TestSRShutdownReceipt(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	input := contracts.Request{Protocol: contracts.Protocol, RequestID: "stop-once", Input: json.RawMessage(`{"expected_instance_id":"old"}`)}
	response, act, err := store.AcceptStop(input, "old")
	if err != nil || !response.OK || !act {
		t.Fatalf("accept: %v %+v", err, response)
	}
	firstID := response.Data.(map[string]any)["command_id"]
	store.Close()
	store, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ReconcileStops("new"); err != nil {
		t.Fatal(err)
	}
	response, act, err = store.AcceptStop(input, "new")
	if err != nil || !response.OK || act || response.Data.(map[string]any)["command_id"] != firstID || response.Data.(map[string]any)["action_status"] != "completed" {
		t.Fatalf("old retry acted on new instance: %v %+v act=%v", err, response, act)
	}
	input.Input = json.RawMessage(`{"expected_instance_id":"new"}`)
	response, act, err = store.AcceptStop(input, "new")
	if err != nil || response.Error == nil || response.Error.Code != "request_conflict" || act {
		t.Fatal("changed request replaced original receipt")
	}
	input.RequestID = "new-request-old-instance"
	input.Input = json.RawMessage(`{"expected_instance_id":"old"}`)
	response, act, err = store.AcceptStop(input, "new")
	if err != nil || response.Error == nil || response.Error.Code != "stale_instance" || act {
		t.Fatal("stale target accepted")
	}
	value, err := store.Command("request_id", "stop-once")
	if err != nil || value.ActionStatus != "completed" {
		t.Fatal("original receipt cannot be queried")
	}
}

func TestSRUnsupportedStorage(t *testing.T) {
	dir := t.TempDir()
	before := []byte("not a Runner database")
	path := filepath.Join(dir, "runner.db")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(dir)
	if err == nil {
		store.Close()
		t.Fatal("foreign state accepted")
	}
	var fault *contracts.Fault
	if !errors.As(err, &fault) || fault.Exit != 3 || fault.Detail.Code != "unsupported_schema" {
		t.Fatalf("wrong storage rejection: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("foreign database changed")
	}
}
