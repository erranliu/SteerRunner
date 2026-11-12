package host

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func TestLostSubmitRejectionIsInspectable(t *testing.T) {
	s, binding := bindingServer(t)
	store, err := storage.Open(binding.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var messages bytes.Buffer
	s.logger = log.New(&messages, "", 0)
	s.engine, err = application.New(t.Context(), binding.StateDir, store, s.logger)
	if err != nil {
		t.Fatal(err)
	}
	request := contracts.Request{
		Protocol: contracts.Protocol, RequestID: contracts.NewID(),
		Input: json.RawMessage(`{
			"title":"Submit receipt regression",
			"repository":{"url":"https://example.com/repo.git","remote":"origin","base_branch":"main","base_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			"documents":{"spec":{"kind":"inline","path":"spec.md","content":"spec"},"plan":{"kind":"inline","path":"plan.md","content":"plan"}},
			"execution":{"adapter":"codex-app-server","target":{"kind":"local"},"settings":{"access_mode":"full_access"}},
			"workflow":{"id":"execute","version":1,"prompt_overrides":{"execute":"run the check"}},
			"reuse_worktree_from_run_id":"absent"
		}`),
	}
	wire := boundWire(s, &binding, "submit")
	wire.Request = &request
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverConn.Close()
		s.handle(serverConn)
	}()
	if err := json.NewEncoder(clientConn).Encode(wire); err != nil {
		clientConn.Close()
		t.Fatal(err)
	}
	// Drop the response regardless of whether admission is already finished.
	clientConn.Close()
	<-done
	if !strings.Contains(messages.String(), "request="+request.RequestID+" outcome=run_not_found elapsed=") {
		t.Fatal(messages.String())
	}
	inspect := boundWire(s, &binding, "inspect")
	inspect.Options.RequestID = request.RequestID
	response, _ := s.dispatch(inspect)
	if !response.OK {
		t.Fatal(response)
	}
	view := response.Data.(map[string]any)["command"].(contracts.CommandView)
	if view.RequestID != request.RequestID || view.ReceiptStatus != "rejected" || view.ActionStatus != "rejected" || view.RunID != nil || view.Error == nil || view.Error.Code != "run_not_found" {
		t.Fatal(view)
	}
	replay, _ := s.dispatch(wire)
	if replay.Error == nil || *replay.Error != *view.Error {
		t.Fatal(replay)
	}
	request.Input = json.RawMessage(strings.Replace(string(request.Input), `"absent"`, `"different"`, 1))
	conflict, _ := s.dispatch(wire)
	if conflict.Error == nil || conflict.Error.Code != "request_conflict" {
		t.Fatal(conflict)
	}
	if runs, err := store.Runs(); err != nil || len(runs) != 0 {
		t.Fatal(runs, err)
	}
}

func TestInterruptedSubmitIsRejectedOnRecovery(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	request := contracts.Request{Protocol: contracts.Protocol, RequestID: contracts.NewID(), Input: json.RawMessage(`{}`)}
	digest := storage.RequestDigest("submit", "", request)
	command, err := store.BeginSubmit(request, digest)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	view, err := store.Command("request_id", request.RequestID)
	if err != nil || view.ReceiptStatus != "pending" || view.ActionStatus != "preparing" || view.RunID != nil || view.CommandID != command.ID {
		t.Fatal(view, err)
	}
	if err := store.ReconcileSubmissions(); err != nil {
		t.Fatal(err)
	}
	engine, err := application.New(context.Background(), dir, store, log.Default())
	if err != nil {
		t.Fatal(err)
	}
	result := engine.Handle("inspect", "", request.RequestID, "", "", 0, nil)
	if !result.OK {
		t.Fatal(result)
	}
	view = result.Data.(map[string]any)["command"].(contracts.CommandView)
	if view.ReceiptStatus != "rejected" || view.ActionStatus != "rejected" || view.RunID != nil || view.Error == nil || view.Error.Code != "invalid_state" || view.CommandID != command.ID {
		t.Fatal(view)
	}
	replay := engine.Handle("submit", "", "", "", "", 0, &request)
	if replay.Error == nil || *replay.Error != *view.Error {
		t.Fatal(replay)
	}
}
