package application

import (
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strings"
	"testing"
)

func TestSROrcaFrozenMaterials(t *testing.T) {
	r := &runstate.Record{Request: contracts.RunRequest{Execution: contracts.Execution{Adapter: "orca-terminal"}}, PromptBodies: map[string]string{workflow.StepImplement: "Custom instruction"}, Materials: []runstate.Material{{MaterialView: contracts.MaterialView{Kind: "inline", Path: "spec.md", Digest: "spec-digest", SnapshotPath: `C:\unavailable\spec.md`}, Content: "冻结 Spec 中文"}, {MaterialView: contracts.MaterialView{Kind: "git", Path: "plan.md", Commit: "fixed-commit", Digest: "plan-digest", SnapshotPath: `C:\unavailable\plan.md`}, Content: "Frozen Plan"}}}
	r.Snapshot.Session = b.SessionContext{AttemptID: "attempt"}
	r.Snapshot.Ref.WorkspacePath = "/remote/execution"
	if err := materialize(r); err != nil {
		t.Fatal("remote material preparation accessed center filesystem", err)
	}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	prompt := r.Snapshot.PendingTurn.Prompt
	for _, want := range []string{"Custom instruction", "prompt:spec.md", "冻结 Spec 中文", "Frozen Plan", "fixed-commit", "spec-digest"} {
		if !strings.Contains(prompt, want) {
			t.Fatal("frozen attachment lost", want)
		}
	}
	if strings.Contains(prompt, `C:\unavailable`) {
		t.Fatal("remote input referenced center-local file")
	}
	if r.Materials[0].SnapshotPath != `C:\unavailable\spec.md` {
		t.Fatal("center snapshot provenance was overwritten")
	}
}
