package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
)

func TestProjectPoliciesFreezeFixedCommitAndRecovery(t *testing.T) {
	f := newBaseRefreshFixture(t, "origin", "main")
	slots := workflow.PolicySlots()
	write := func(root, path, text string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	const body = "secret policy body never inline"
	write(f.writer, slots[0].Path, body)
	write(f.writer, slots[2].Path, "") // An empty readable file is configured.
	write(f.writer, "runner-policies/command.md", "wrong location")
	write(f.writer, "SteerlineFlow/environment.md", "independent environment requirements")
	f.pinned = f.commit("policies")
	f.git(f.writer, "push", f.remoteName, "HEAD:refs/heads/"+f.branch)
	f.request.Repository.BaseRevision = f.pinned
	f.input.Input = canonical(f.request)
	// The caller checkout deliberately disagrees with the pinned source.
	write(f.source, slots[1].Path, "untracked caller file")
	r := f.freeze()
	want := &workflow.ProjectPolicies{Commit: f.pinned, Files: []workflow.PolicyFile{
		{PolicySlot: slots[0], Exists: true, SHA256: ra.Digest([]byte(body))},
		{PolicySlot: slots[1]},
		{PolicySlot: slots[2], Exists: true, SHA256: ra.Digest(nil)},
	}}
	if !reflect.DeepEqual(r.Workflow.ProjectPolicies, want) {
		t.Fatalf("wrong observations: %+v", r.Workflow.ProjectPolicies)
	}
	if strings.Contains(string(canonical(r.Workflow.ProjectPolicies)), body) {
		t.Fatal("stored policy body")
	}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	first := r.Snapshot.PendingTurn.Prompt
	if strings.Contains(first, body) || strings.Contains(first, slots[1].Path) || strings.Contains(first, "environment.md") {
		t.Fatal("inline body, implicit slot or environment instructions", first)
	}
	for _, index := range []int{0, 2} {
		if !strings.Contains(first, fmt.Sprintf(workflow.PolicyReadTemplate, slots[index].Path)) {
			t.Fatal("missing selected file")
		}
	}
	// Advance the repository and mutate checkout files after acceptance.
	write(f.writer, slots[0].Path, "replacement")
	write(f.writer, slots[1].Path, "newly configured")
	f.commit("changed policies")
	if err := os.Remove(filepath.Join(f.source, filepath.FromSlash(slots[1].Path))); err != nil {
		t.Fatal(err)
	}
	restored := r.Clone()
	if err := validateWorkflow(restored); err != nil {
		t.Fatal(err)
	}
	if err := queueInput(restored, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	if restored.Snapshot.PendingTurn.Prompt != first || !reflect.DeepEqual(restored.Workflow.ProjectPolicies, want) {
		t.Fatal("recovery or later input re-read current project policies")
	}
	// A fresh read of the same accepted commit also stays fixed.
	if err := freezeProjectPolicies(context.Background(), restored); err != nil || !reflect.DeepEqual(restored.Workflow.ProjectPolicies, want) {
		t.Fatal("fixed commit observations changed", err)
	}
}

func TestProjectPoliciesUnreadableFailsAtomically(t *testing.T) {
	path := workflow.PolicySlots()[0].Path
	for _, mode := range []string{"lookup", "blob", "directory", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			r := &runstate.Record{}
			err := readProjectPolicies(r, func(args ...string) ([]byte, error) {
				if args[0] == "cat-file" || mode == "lookup" {
					return nil, errors.New("denied")
				}
				header := "100644 blob deadbeef"
				if mode == "directory" {
					header = "040000 tree deadbeef"
				}
				if mode == "symlink" {
					header = "120000 blob deadbeef"
				}
				return []byte(header + "\t" + path + "\x00"), nil
			})
			if err == nil || !strings.Contains(err.Error(), path) || r.Workflow.ProjectPolicies != nil {
				t.Fatal("unreadable file accepted or partial facts committed", err)
			}
		})
	}
}

func TestProjectPoliciesBudgetAndFrozenInput(t *testing.T) {
	for _, adapter := range []string{"codex-app-server", "orca-terminal"} {
		t.Run(adapter, func(t *testing.T) {
			r := budgetRecord()
			r.Request.Execution.Adapter = adapter
			base := r.Clone()
			if err := queueInput(base, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
				t.Fatal(err)
			}
			r.Workflow.ProjectPolicies = &workflow.ProjectPolicies{}
			extra := ""
			for _, slot := range workflow.PolicySlots() {
				r.Workflow.ProjectPolicies.Files = append(r.Workflow.ProjectPolicies.Files, workflow.PolicyFile{PolicySlot: slot, Exists: true})
				extra += fmt.Sprintf(workflow.PolicyReadTemplate, slot.Path)
			}
			if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
				t.Fatal(err)
			}
			if r.Snapshot.PendingTurn.Budget.RunnerBytes != base.Snapshot.PendingTurn.Budget.RunnerBytes+len(extra) {
				t.Fatal("references bypassed budget")
			}
			frozen := r.Workflow.StageInputs[0]
			if err := queueInput(r, frozen, ""); err != nil || r.Snapshot.PendingTurn.Prompt != frozen.Prompt {
				t.Fatal("frozen input gained duplicate references", err)
			}
			// This body fits alone; policy references make it exceed the existing limit.
			r.PromptBodies[workflow.StepImplement] = strings.Repeat("b", m.RunnerInputBytes-base.Snapshot.PendingTurn.Budget.RunnerBytes)
			r.Snapshot.PendingTurn = nil
			if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err == nil || r.Snapshot.PendingTurn != nil {
				t.Fatal("policy overflow queued")
			}
		})
	}
}

func TestProjectPoliciesLegacyDefinitionUnchanged(t *testing.T) {
	r := budgetRecord()
	r.Workflow.Config.PolicyReadTemplate = ""
	for id, state := range r.Workflow.Config.States {
		state.PolicySlots = nil
		r.Workflow.Config.States[id] = state
	}
	r = r.Clone()
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal("old definition invalidated", err)
	}
	if r.Workflow.ProjectPolicies != nil || strings.Contains(r.Snapshot.PendingTurn.Prompt, "runner-policies") {
		t.Fatal("old run gained policy material or instructions")
	}
}

func TestProjectPoliciesReadFailureDoesNotBecomeMissing(t *testing.T) {
	r := &runstate.Record{}
	slots := workflow.PolicySlots()
	reads := 0
	err := readProjectPolicies(r, func(args ...string) ([]byte, error) {
		if args[0] == "cat-file" {
			reads++
			return []byte("readable git policy"), nil
		}
		if args[len(args)-1] == ":(literal)"+slots[0].Path {
			return []byte("100644 blob deadbeef\t" + slots[0].Path + "\x00"), nil
		}
		return nil, errors.New("command policy lookup denied")
	})
	var fault *contracts.Fault
	if !errors.As(err, &fault) || fault.Detail.Code != "project_policy_unreadable" || reads != 1 || r.Workflow.ProjectPolicies != nil {
		t.Fatal("read failure was ignored or published partial observations", err)
	}
}

func TestProjectPoliciesAuditedStepsBudget(t *testing.T) {
	e, _ := auditEngine(t)
	r := e.runs["run"]
	r.Workflow.ProjectPolicies = &workflow.ProjectPolicies{}
	for _, slot := range workflow.PolicySlots() {
		r.Workflow.ProjectPolicies.Files = append(r.Workflow.ProjectPolicies.Files, workflow.PolicyFile{PolicySlot: slot, Exists: true})
	}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.CheckInput(r.Snapshot.PendingTurn.Prompt, r.Snapshot.PendingTurn.Budget); err != nil {
		t.Fatal(err)
	}
	finishStage(t, e, "实施及必要验证已完成。")
	r = e.runs["run"]
	if r.Snapshot.PendingTurn == nil || r.Workflow.StepID != workflow.StepAuditCode {
		t.Fatal("audit with policies unavailable", r.Diagnostic)
	}
	if err := m.CheckInput(r.Snapshot.PendingTurn.Prompt, r.Snapshot.PendingTurn.Budget); err != nil {
		t.Fatal(err)
	}
	// The Orca model bridge includes a larger native header. Its existing final
	// budget rejects this combination even though the app-server body fits.
	var budgetError *m.InputBudgetError
	if err := projectPolicyBridgeBudget(r.Snapshot.PendingTurn); !errors.As(err, &budgetError) {
		t.Fatal("model bridge failed to count policy references", err)
	}
	for _, slot := range workflow.PolicySlots() {
		if !strings.Contains(r.Snapshot.PendingTurn.Prompt, fmt.Sprintf(workflow.PolicyReadTemplate, slot.Path)) {
			t.Fatal("audit policy reference missing", slot.Slot)
		}
	}
	finishStage(t, e, `{"outcome":"done","summary":"audited and validated; no repairs","fixed_count":0,"fixed_issues":[],"remaining_issues":[]}`)
	r = e.runs["run"]
	if r.Snapshot.PendingTurn == nil || r.Workflow.StepID != workflow.StepPublish {
		t.Fatal("publication with policies unavailable", r.Diagnostic)
	}
	if err := m.CheckInput(r.Snapshot.PendingTurn.Prompt, r.Snapshot.PendingTurn.Budget); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Snapshot.PendingTurn.Prompt, workflow.PolicySlots()[2].Path) {
		t.Fatal("publication gained testing policy")
	}
}

func TestProjectPoliciesProducerGuidesRespectExistingBudget(t *testing.T) {
	r := budgetRecord()
	r.Request.ProgressFile = &contracts.ProgressFileInput{}
	r.Request.ResultFile = &contracts.ResultFileInput{}
	r.Workflow.ProjectPolicies = &workflow.ProjectPolicies{}
	for _, slot := range workflow.PolicySlots() {
		r.Workflow.ProjectPolicies.Files = append(r.Workflow.ProjectPolicies.Files, workflow.PolicyFile{PolicySlot: slot, Exists: true})
	}
	if err := queueInput(r, workflow.StageInput{StepID: workflow.StepImplement}, ""); err != nil {
		t.Fatal(err)
	}
	// The complete references, two producer guides and native model bridge
	// exceed 1000B. The final wire input must fail the existing hard limit.
	err := projectPolicyBridgeBudget(r.Snapshot.PendingTurn)
	var budgetError *m.InputBudgetError
	if !errors.As(err, &budgetError) {
		t.Fatal("combined native input bypassed budget", err)
	}
}

func projectPolicyBridgeBudget(input *b.TurnInput) error {
	parts, err := m.InputParts(input.Prompt, input.Budget)
	if err != nil {
		return err
	}
	parts = m.NativeInputParts(parts)
	body, _, err := m.DescribeInput(parts...)
	if err != nil {
		return err
	}
	wire := codex.RemoteModelPrompt(input.CommandID, input.Budget.SHA256, input.StepID, "gpt-6.1-sol", "xhigh", input.Prompt)
	parts = append([]m.InputPart{{Source: "native.header", Owner: m.RunnerInput, Text: strings.TrimSuffix(wire, body)}}, parts...)
	_, _, err = m.FreezeInput(parts...)
	return err
}
