package storage

import (
	"github.com/erranliu/SteerRunner/internal/runstate"
	"testing"
)

func TestWorkflowTransitionsAreAtomicHistoricalFacts(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := &runstate.Record{ID: "run", Revision: 1, Status: "running"}
	r.Workflow.StepID = "implement.execute"
	r.Snapshot.Session.AttemptID = "attempt"
	if err := s.Commit(r, 0, nil, "accepted"); err != nil {
		t.Fatal(err)
	}
	next := r.Clone()
	next.Revision = 2
	next.Workflow.StepID = "implement.report"
	next.Status = "blocked"
	next.Diagnostic = "HTTP 401"
	if _, err := s.db.Exec(`CREATE TRIGGER fail_change BEFORE INSERT ON events WHEN json_extract(NEW.body,'$.type')='workflow_changed' BEGIN SELECT RAISE(ABORT,'fault'); END;`); err != nil {
		t.Fatal(err)
	}
	if s.Commit(next, 1, nil, "result_committed") == nil {
		t.Fatal("partial transition accepted")
	}
	runs, err := s.Runs()
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].Revision != 1 {
		t.Fatal("aggregate escaped rollback")
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_change`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(next, 1, nil, "result_committed"); err != nil {
		t.Fatal(err)
	}
	next.Revision = 3
	next.Diagnostic = "HTTP 403"
	if err := s.Commit(next, 2, nil, "diagnostic"); err != nil {
		t.Fatal(err)
	}
	next.Revision = 4
	if err := s.Commit(next, 3, nil, "operation_committed"); err != nil {
		t.Fatal(err)
	}
	page, err := s.Events("run", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	changes := 0
	for _, event := range page.Events {
		if event.Transition == nil {
			continue
		}
		changes++
		if changes == 2 {
			v := event.Transition
			if v.From.Stage != "implement.execute" || v.To.Stage != "implement.report" || v.To.Status != "blocked" || v.To.Diagnostic != "HTTP 401" || event.AttemptID != "attempt" {
				t.Fatal("history repainted", event)
			}
		}
	}
	if changes != 3 {
		t.Fatal("missing or duplicate transitions", changes)
	}
}
