package codexworker

import (
	"context"
	"strings"
	"testing"

	executionbackend "github.com/erranliu/SteerRunner/internal/execution/backend"
)

func TestTurnParamsRejectsUnboundCommandBeforeWorkspaceProbe(t *testing.T) {
	called := false
	_, err := TurnParams(context.Background(), TurnParamsInput{
		Worktree: "C:/fixed/worktree", ThreadID: "00000000-0000-0000-0000-000000000001",
		TurnInput: executionbackend.TurnInput{CommandID: "wrong", Prompt: "continue"}, Sandbox: "danger-full-access",
	}, func(context.Context, string) ([]string, error) {
		called = true
		return nil, nil
	})
	if err == nil || called {
		t.Fatalf("unbound command reached workspace probe: %v, called=%v", err, called)
	}
}

func TestSRTurnModelBinding(t *testing.T) {
	input := TurnParamsInput{Worktree: "C:/fixed/worktree", ThreadID: "00000000-0000-0000-0000-000000000001", TurnInput: executionbackend.TurnInput{CommandID: strings.Repeat("a", 32), Prompt: "fixed", Model: "reviewer", ReasoningEffort: "high"}, Sandbox: "danger-full-access", Model: "executor", ReasoningEffort: "medium"}
	params, err := TurnParams(context.Background(), input, func(context.Context, string) ([]string, error) { return nil, nil })
	if err != nil || params["model"] != "reviewer" || params["effort"] != "high" || params["threadId"] != input.ThreadID {
		t.Fatal("per-turn model not used", err, params)
	}
	input.TurnInput.Model = ""
	input.TurnInput.ReasoningEffort = ""
	params, err = TurnParams(context.Background(), input, func(context.Context, string) ([]string, error) { return nil, nil })
	if err != nil || params["model"] != "executor" || params["effort"] != "medium" {
		t.Fatal("legacy fallback changed", err, params)
	}
}
