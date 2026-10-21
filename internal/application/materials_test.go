package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestDevelopmentPromptsReferenceWorktreeDocuments(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		t.Run(adapter, func(t *testing.T) {
			body := strings.Repeat("document body must not reach the prompt", 500)
			r := &runstate.Record{
				Request: contracts.RunRequest{
					Workflow:  contracts.Workflow{ID: "development", Version: 2},
					Execution: contracts.Execution{Adapter: adapter},
					Documents: contracts.Documents{
						Spec:       contracts.Document{Kind: "inline", Path: "docs/spec.md", Content: &body},
						Plan:       contracts.Document{Kind: "git", Path: "docs/plan.md", Commit: strings.Repeat("f", 40)},
						References: []contracts.Document{{Path: "docs/reference.md"}},
					},
				},
				PromptBodies: map[string]string{workflow.StepImplement: "Custom instruction"},
				// Legacy snapshots must not be read or embedded in a fresh turn.
				Materials: []runstate.Material{{MaterialView: contracts.MaterialView{SnapshotPath: `C:\unavailable\spec.md`}, Content: body}},
			}
			r.Snapshot.Session = b.SessionContext{AttemptID: "attempt"}
			r.Snapshot.Ref.WorkspacePath = "/unavailable/execution"
			bindTestWorkflow(r)
			e := &Engine{}
			for _, feedback := range []string{"", "Continue the implementation"} {
				if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, feedback); err != nil {
					t.Fatal(err)
				}
				if err := e.checkDispatchSource(r, r.Workflow.StageInputs[len(r.Workflow.StageInputs)-1]); err != nil {
					t.Fatal("dispatch accessed development documents", err)
				}
				prompt := r.Snapshot.PendingTurn.Prompt
				for _, want := range []string{"Custom instruction", "Spec: docs/spec.md", "Plan: docs/plan.md", "Reference: docs/reference.md", feedback} {
					if !strings.Contains(prompt, want) {
						t.Fatal("worktree reference or feedback lost", want)
					}
				}
				for _, unwanted := range []string{body, "prompt:", `C:\unavailable`, strings.Repeat("f", 40), "Frozen Spec"} {
					if strings.Contains(prompt, unwanted) {
						t.Fatal("prompt contains a material attachment", unwanted[:min(len(unwanted), 80)])
					}
				}
			}
		})
	}
}

func TestDevelopmentAdmissionDoesNotResolveDocumentFilesOrCommits(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	f.request.Documents = contracts.Documents{
		Spec:       contracts.Document{Kind: "git", Path: "missing/spec.md", Commit: strings.Repeat("f", 40)},
		Plan:       contracts.Document{Path: "missing/plan.md"},
		References: []contracts.Document{{Path: "missing/reference.md"}},
	}
	f.request.Publication.Settings = canonical(contracts.CommandProviderSettings{BaseURL: "https://github.com", Query: contracts.QuerySettings{Executable: os.Args[0], Args: []string{"--query"}}})
	f.input.Input = canonical(f.request)
	if failure := contracts.ValidateRun(f.input.Input); failure != nil {
		t.Fatal(failure)
	}
	r, err := freeze(context.Background(), f.state, f.input)
	if err != nil {
		t.Fatal("development admission resolved caller-owned documents", err)
	}
	if len(r.Materials) != 3 {
		t.Fatal("document paths lost")
	}
	if _, err := os.Stat(filepath.Join(f.state, "inputs")); !os.IsNotExist(err) {
		t.Fatal("development admission created material snapshots", err)
	}
}

func TestOrcaPromptLimitAppliesToRenderedPrompt(t *testing.T) {
	r := &runstate.Record{Request: contracts.RunRequest{Workflow: contracts.Workflow{ID: "development", Version: 2}, Execution: contracts.Execution{Adapter: "orca-terminal"}, Documents: contracts.Documents{Spec: contracts.Document{Path: "spec.md"}, Plan: contracts.Document{Path: "plan.md"}}}, PromptBodies: map[string]string{workflow.StepImplement: strings.Repeat("😀", 6001)}}
	bindTestWorkflow(r)
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err == nil {
		t.Fatal("oversized actual prompt was accepted")
	}
	if r.Snapshot.PendingTurn != nil || len(r.Workflow.StageInputs) != 0 {
		t.Fatal("oversized prompt queued native work")
	}
}
