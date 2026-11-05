package workflow

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	m "github.com/erranliu/SteerRunner/internal/execution/model"
)

func TestProjectPolicyStepDeclarations(t *testing.T) {
	for _, p := range Catalog() {
		for id, state := range p.States {
			want := []string{}
			if !Content(p.ID) && p.ID != SingleExecution && state.Kind == "execute" {
				want = []string{"git", "command"}
				if state.Contract != "publication" {
					want = append(want, "testing")
				}
			}
			if !reflect.DeepEqual(state.PolicySlots, want) {
				t.Fatalf("%s/%s: %v != %v", p.ID, id, state.PolicySlots, want)
			}
		}
	}
}

func TestProjectPolicyOnlyPublicTemplateAndSelectedSlots(t *testing.T) {
	p, _ := Lookup(AuditedDevelopment, 2)
	for mask := 0; mask < 8; mask++ {
		w := Snapshot{Config: &p, ProjectPolicies: &ProjectPolicies{Commit: "fixed"}}
		for i, slot := range PolicySlots() {
			w.ProjectPolicies.Files = append(w.ProjectPolicies.Files, PolicyFile{PolicySlot: slot, Exists: mask&(1<<i) != 0, SHA256: "digest-must-not-reach-prompt"})
		}
		for id, state := range p.States {
			parts := policyParts(w, state)
			want := ""
			for i, slot := range PolicySlots() {
				for _, selected := range state.PolicySlots {
					if selected == slot.Slot && mask&(1<<i) != 0 {
						want += fmt.Sprintf(PolicyReadTemplate, slot.Path)
					}
				}
			}
			got, budget, err := m.FreezeInput(parts...)
			if err != nil || got != want || budget.RunnerBytes != len(want) || budget.CallerBytes != 0 {
				t.Fatalf("mask=%d step=%s got=%q want=%q budget=%+v err=%v", mask, id, got, want, budget, err)
			}
		}
	}
}

func TestProjectPolicyRenderOverrideAndLegacy(t *testing.T) {
	p, _ := Lookup(AuditedDevelopment, 2)
	w := Snapshot{Config: &p, ProjectPolicies: &ProjectPolicies{Files: []PolicyFile{{PolicySlot: PolicySlots()[0], Exists: true}}}}
	input := StageInput{StepID: StepImplement, MaterialFacts: []byte(`{"spec":{"path":"spec.md"},"plan":{"path":"plan.md"}}`)}
	want, err := RenderInputWithBody(Snapshot{Config: &p}, input, "custom body")
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderInputWithBody(w, input, "custom body")
	if err != nil || got != want+fmt.Sprintf(PolicyReadTemplate, PolicySlots()[0].Path) {
		t.Fatal("policy text missing or hidden additions", got, err)
	}
	p.PolicyReadTemplate = "\nLegacy read %s"
	got, err = RenderInputWithBody(w, input, "custom body")
	if err != nil || !strings.HasSuffix(got, "\nLegacy read "+PolicySlots()[0].Path) {
		t.Fatal("render ignored frozen template", got, err)
	}
	w.ProjectPolicies = nil
	got, err = RenderInputWithBody(w, input, "custom body")
	if err != nil || got != want {
		t.Fatal("legacy input gained policy instructions", got, err)
	}
}

func TestSingleExecutionHasNoPublicationOrAcquisition(t *testing.T) {
	p, ok := Lookup(SingleExecution, 1)
	if !ok || p.Initial != "execute" || len(p.States) != 2 || p.RequiresResultAcquisition() || p.AuditModelRequired {
		t.Fatal(p)
	}
	s := p.States[p.Initial]
	if s.Contract != "final" || s.Next != "closed" || s.Report != "" || s.Verify != "" || s.Wait != "" || len(s.PolicySlots) != 0 {
		t.Fatal(s)
	}
	if _, ok := p.States[StepPublish]; ok {
		t.Fatal("publication leaked into single execution")
	}
	body, err := RenderInputWithBody(Snapshot{Config: &p}, StageInput{StepID: "execute"}, "call existing entry")
	if err != nil || body != "call existing entry" {
		t.Fatal(body, err)
	}
}
