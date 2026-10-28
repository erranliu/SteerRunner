package application

import (
	"bytes"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/execution/workflow"
)

func TestPublicationPresentationPreservesStoredSourceAndDispatchFences(t *testing.T) {
	e, _ := auditEngine(t)
	finishStage(t, e, "实施及必要验证已完成。")
	finishStage(t, e, `{"outcome":"done","summary":"audited","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`)
	original := e.runs["run"].Clone()
	if original.Workflow.StepID != workflow.StepPublish {
		t.Fatal("fixture did not reach publication")
	}
	input := *workflow.Input(original.Workflow, original.Snapshot.PendingTurn.CommandID)
	long := input.Prompt
	start := strings.Index(long, "\nSource facts:\n")
	if start < 0 {
		t.Fatal("default no longer includes complete source facts")
	}
	end := strings.IndexByte(long[start+len("\nSource facts:\n"):], '\n')
	if end < 0 {
		t.Fatal("missing fixed success instruction")
	}
	end += start + len("\nSource facts:\n")
	stored := canonical(original.Workflow.Results)
	materials := canonical(original.Request.Documents)
	short := original.Clone()
	short.Request.Workflow.OmitPublishSourceFacts = true
	input.Prompt = ""
	if err := queueInput(short, input, ""); err != nil {
		t.Fatal(err)
	}
	if short.Snapshot.PendingTurn.Prompt != long[:start]+long[end:] {
		t.Fatal("short rendering changed bytes outside the visible facts block")
	}
	short = short.Clone()
	if !short.Request.Workflow.OmitPublishSourceFacts || !bytes.Equal(stored, canonical(short.Workflow.Results)) || !bytes.Equal(materials, canonical(short.Request.Documents)) {
		t.Fatal("presentation removed complete persisted facts")
	}
	queued := short.Workflow.StageInputs[len(short.Workflow.StageInputs)-1]
	if err := e.checkDispatchSource(short, queued); err != nil {
		t.Fatal("valid full source rejected", err)
	}
	for _, kind := range []string{"current-source", "input-binding", "acquired-result"} {
		bad := short.Clone()
		candidate := queued
		switch kind {
		case "current-source":
			bad.Workflow.ImplementationResultID = "another-source"
		case "input-binding":
			candidate.ImplementationResultID = "another-source"
		case "acquired-result":
			bad.Workflow.Results = nil
		}
		if e.checkDispatchSource(bad, candidate) == nil {
			t.Fatal("short presentation weakened dispatch fence", kind)
		}
	}
}
