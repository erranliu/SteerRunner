package orcatransport

import (
	"context"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"strings"
	"testing"
)

func TestWireBudgetDefaultStages(t *testing.T) {
	profile, _ := workflow.Lookup(workflow.AuditedDevelopment, 2)
	for step, state := range profile.States {
		if state.Kind != "execute" && state.Kind != "report" {
			continue
		}
		facts := map[string]any{}
		if state.Contract == "implementation" && state.Kind == "execute" {
			facts["materials"] = map[string]any{"spec": workflow.FileReference{Path: "docs/spec.md"}, "plan": workflow.FileReference{Path: "docs/plan.md"}}
		}
		if state.Kind == "report" {
			facts["source"] = strings.Repeat("a", 32)
		}
		text, err := workflow.RenderPrompt(step, facts)
		if err != nil {
			t.Fatal(step, err)
		}
		parts := []m.InputPart{{Source: "workflow.body", Owner: m.RunnerInput, Text: text}}
		if state.Contract == "publication" && state.Kind == "execute" {
			parts = append(parts, m.InputPart{Source: "publication.target", Owner: m.RunnerInput, Text: workflow.PublicationInstruction(state, []byte(`{"provider":"gitea","repository":"team/project","settings":{"base_url":"http://git.example:3000"}}`), "origin", "main")})
		}
		if state.Kind == "execute" && state.Contract == "audit" {
			parts = append(parts, m.InputPart{Source: "progress", Owner: m.RunnerInput, Text: "\n" + workflow.ContinueProgressInstruction}, m.InputPart{Source: "result_file", Owner: m.RunnerInput, Text: "\n" + workflow.ContinueResultInstruction})
		}
		text, b, err := m.DescribeInput(parts...)
		if err != nil {
			t.Fatal(err)
		}
		for _, format := range []string{"", remoteWireCommand, remoteModelWireCommand, extensionWireCommand} {
			turn := Turn{Command: strings.Repeat("c", 32), InputDigest: digest([]byte(text)), Prompt: text, Budget: b, WireFormat: format, StepID: step, Model: "gpt-6.1-sol", ReasoningEffort: "xhigh"}
			if err := freezeWireInput(&turn, 1); err != nil {
				t.Errorf("%s/%s: %v", step, format, err)
				continue
			}
			t.Logf("%s/%s Runner=%d", step, format, turn.WireBudget.RunnerBytes)
			if err := m.CheckInput(nativeWireText(&turn), turn.WireBudget); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestWireBudgetLegacyPreparedInputPersistsBeforeSend(t *testing.T) {
	l, host, s := fixture(t)
	r, _, err := l.Execute(context.Background(), request(s, b.LifecyclePrepareWorkspace, b.LifecycleInput{Workspace: &s.WorkspacePlan}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	s, input := prepared(t, l, s, strings.Repeat("c", 32))
	state, err := l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	turn := turnFor(&state, input.CommandID)
	turn.Budget, turn.WireBudget = nil, nil
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	input.InputVersion = 1
	_, input.Budget, err = m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: input.Prompt})
	if err != nil {
		t.Fatal(err)
	}
	s.PendingTurn = &input
	facts, err := l.PreparationFacts(context.Background(), s)
	if err != nil || facts.Session != b.GateRequired {
		t.Fatal("legacy final budget not made durable before effect", facts, err)
	}
	r, _, err = l.Execute(context.Background(), request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
	if err != nil {
		t.Fatal(err)
	}
	bind(&s, r)
	state, err = l.state(s)
	if err != nil {
		t.Fatal(err)
	}
	if turnFor(&state, input.CommandID).WireBudget == nil || host.nativeSends != 0 {
		t.Fatal("budget missing or preparation sent model input")
	}
	turnFor(&state, input.CommandID).Budget.ProvenanceSHA256 = "changed"
	s.Implementation, err = l.envelope(s, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.state(s); err != nil {
		t.Fatal("budget metadata blocked existing state reads/stop", err)
	}
	if _, err = l.submit(context.Background(), request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input})); err == nil || host.nativeSends != 0 {
		t.Fatal("changed budget sent input", err)
	}
}
func TestWireBudgetRejectsHeaderOverflowAndChangedBody(t *testing.T) {
	text, b, _ := m.FreezeInput(m.InputPart{Source: "workflow.body", Owner: m.RunnerInput, Text: strings.Repeat("r", 900)})
	turn := Turn{Command: strings.Repeat("c", 32), InputDigest: digest([]byte(text)), Prompt: text, Budget: b, WireFormat: remoteWireCommand}
	if err := freezeWireInput(&turn, 1); err == nil {
		t.Fatal("header budget not counted")
	}
	turn.Prompt += "extra"
	if err := freezeWireInput(&turn, 1); err == nil {
		t.Fatal("post-freeze append accepted")
	}
}
func TestTerminalToolDescriptionsBudget(t *testing.T) {
	parts, err := terminalDescriptionParts()
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := m.FreezeInput(parts...)
	if err != nil || b.RunnerBytes == 0 {
		t.Fatal(b, err)
	}
	t.Logf("DSH tool descriptions Runner=%d", b.RunnerBytes)
}
