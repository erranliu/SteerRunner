package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/erranliu/SteerRunner/internal/resources"
)

func TestCurrentAuditReportNeverExecutesRepair(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STEERRUNNER_FIXTURE_STATE", root)
	t.Setenv("STEERRUNNER_AUDIT_FIX", "1")
	var prompts map[string]string
	if err := json.Unmarshal(resources.Read("prompts.json"), &prompts); err != nil {
		t.Fatal(err)
	}
	result, err := auditResult(root, prompts["audit.code.report"])
	if err != nil || result != `{"outcome":"done","summary":"original audit formatted","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}` {
		t.Fatal("report entered execution branch", result, err)
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 0 {
		t.Fatal("report wrote repair files", files, err)
	}
}
