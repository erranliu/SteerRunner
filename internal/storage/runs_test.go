package storage

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"testing"
)

func TestSRRunAtomicCommit(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	request := contracts.Request{Protocol: contracts.Protocol, RequestID: "fixed-request", Input: json.RawMessage(`{"value":9007199254740993}`)}
	command := NewCommand("submit", "run", request)
	r := &runstate.Record{ID: "run", Revision: 1, CommandStates: map[string]string{command.ID: "pending"}}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'injected commit failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(r, 0, &command, "run_accepted"); err == nil {
		t.Fatal("fault did not reject commit")
	}
	for _, table := range []string{"runs", "commands", "events"} {
		var count int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial %s after rollback: %d %v", table, count, err)
		}
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_event`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(r, 0, &command, "run_accepted"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(r, 0, &command, "run_accepted"); err == nil {
		t.Fatal("duplicate commit succeeded")
	}
	page, err := s.Events("run", "", 1)
	if err != nil || len(page.Events) != 1 {
		t.Fatal("duplicate or missing accepted event", err)
	}
	if _, err := s.Events("another", page.NextCursor, 1); err == nil {
		t.Fatal("cross-run cursor accepted")
	}
	request.Input = json.RawMessage(`{"value":9007199254740992}`)
	if RequestDigest("submit", "run", request) == command.Digest {
		t.Fatal("fingerprint rounded distinct integers")
	}
}
