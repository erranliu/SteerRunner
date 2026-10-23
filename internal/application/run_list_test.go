package application

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestDisplayRunsAreIndependentWhitelistCopies(t *testing.T) {
	r := &runstate.Record{ID: "direct-cli-run", Status: "running", Revision: 4, Request: contracts.RunRequest{Title: "display", Workflow: contracts.Workflow{ID: "development-audited", Version: 2, PromptOverrides: map[string]string{"audit.code": "PRIVATE PROMPT"}, AuditModel: &workflow.ModelSettings{Model: "PRIVATE MODEL"}}}, PR: &contracts.PRFact{URL: "https://github.com/o/r/pull/3", Number: 3, QueryError: "PRIVATE DIAGNOSTIC"}}
	r.Workflow.StepID = "audit.code"
	e := &Engine{runs: map[string]*runstate.Record{r.ID: r}}
	rows, err := e.DisplayRuns()
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	body, _ := json.Marshal(rows)
	if strings.Contains(string(body), "PRIVATE") || r.Revision != 4 {
		t.Fatal("private fields/side effects", string(body))
	}
	r.PR.URL = "changed"
	r.Status = "blocked"
	if rows[0].PR.URL == r.PR.URL || rows[0].Status != "running" {
		t.Fatal("snapshot aliases mutable state")
	}
}

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
