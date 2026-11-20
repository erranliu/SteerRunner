//go:build l4

package application

import "testing"

func TestDisplayRunsIgnoreRolledBackChanges(t *testing.T) {
	e, _, db := operationEngine(t)
	before, _ := e.DisplayRuns()
	failOperationCommit(t, db)
	next := e.runs["run"].Clone()
	next.Status = "uncommitted-status"
	next.Request.Title = "uncommitted-title"
	if err := e.commitLocked(next, nil, "uncommitted-change"); err == nil {
		t.Fatal("expected real SQLite transaction failure")
	}
	after, err := e.DisplayRuns()
	if err != nil || len(after) != 1 || after[0].Status != before[0].Status || after[0].Title != before[0].Title {
		t.Fatal("uncommitted display became visible", after, err)
	}
}
