package storage

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"testing"
)

func TestEnvironmentCheckAtomicRevisionAndRejection(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	request := contracts.Request{Protocol: contracts.Protocol, RequestID: "start", Input: json.RawMessage(`{"value":1}`)}
	c := NewCheckCommand("environment-check start", "original", request)
	r := &runstate.CheckRecord{View: contracts.EnvironmentCheck{CheckID: "original", Revision: 1, InputDigest: "input", ResourceDigest: "resources"}, Nonce: "nonce", CommandStates: map[string]string{c.ID: "pending"}}
	if err = s.CommitCheck(r, 0, &c); err != nil {
		t.Fatal(err)
	}
	changed := r.Clone()
	changed.View.Revision = 2
	changed.View.InputDigest = "different"
	if s.CommitCheck(changed, 1, nil) == nil {
		t.Fatal("immutable digest changed")
	}
	changed = r.Clone()
	changed.View.Revision = 3
	if s.CommitCheck(changed, 1, nil) == nil {
		t.Fatal("revision skipped")
	}
	changed = r.Clone()
	changed.View.Revision = 2
	changed.CommandStates[c.ID] = "completed"
	if err = s.CommitCheck(changed, 1, nil); err != nil {
		t.Fatal(err)
	}
	command, err := s.CheckCommand("start")
	if err != nil || command.ActionStatus != "completed" || command.CheckID == nil || *command.CheckID != "original" {
		t.Fatal(command, err)
	}
	original, found, err := s.Original("start", RequestDigest(c.Name, "", request))
	if err != nil || !found || !original.OK {
		t.Fatal(original, found, err)
	}
	bad := contracts.Request{Protocol: contracts.Protocol, RequestID: "refused", Input: json.RawMessage(`{}`)}
	rejection := NewCheckCommand("environment-check stop", "", bad)
	rejection.Status = "failed"
	rejection.Response = contracts.Rejected(bad.RequestID, contracts.Fail(5, "stale_revision", "Inspect the current check."))
	if err = s.CommitCheck(nil, 0, &rejection); err != nil {
		t.Fatal(err)
	}
	command, err = s.CheckCommand("refused")
	if err != nil || command.ReceiptStatus != "rejected" || command.Error.Code != "stale_revision" || command.CheckID != nil {
		t.Fatal(command, err)
	}
	checks, err := s.Checks()
	if err != nil || len(checks) != 1 || checks[0].View.Revision != 2 || checks[0].View.InputDigest != "input" {
		t.Fatal(checks, err)
	}
}
