package workflow

import (
	"strings"
	"testing"
)

func TestPublicationFactsPresentationBoundary(t *testing.T) {
	for _, step := range []string{StepPublish, StepPublishReport, StepReport, StepImplement} {
		state := developmentStates(false)[step]
		facts := map[string]any{"source": "complete-original-source", "materials": map[string]any{}}
		visible, err := renderPromptPresentation(state, facts, "fixed caller body", false)
		if err != nil {
			t.Fatal(err)
		}
		hidden, err := renderPromptPresentation(state, facts, "fixed caller body", true)
		if err != nil {
			t.Fatal(err)
		}
		if step == StepPublish {
			want := "fixed caller body\n完成后，在最终回复末尾单独写一行：本次发布已完成。"
			if hidden != want || !strings.Contains(visible, "complete-original-source") {
				t.Fatal("publication presentation changed more than visible facts", hidden)
			}
		} else if hidden != visible {
			t.Fatal("presentation flag changed another stage", step)
		}
	}
	state := developmentStates(false)[StepPublish]
	if _, err := renderPromptPresentation(state, map[string]any{"invalid": make(chan int)}, "fixed", true); err == nil {
		t.Fatal("hidden facts bypassed complete-facts serialization")
	}
}
