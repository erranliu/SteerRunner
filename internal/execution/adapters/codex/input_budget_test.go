package codexworker

import (
	"context"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/codexapp"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"strings"
	"testing"
)

func TestInputSessionAggregateAndZeroOversizeSend(t *testing.T) {
	text := fmt.Sprintf(WorkerInstructionFormat, `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`)
	facts, err := sessionInput(map[string]any{"developerInstructions": text, "dynamicTools": []any{codexapp.AskDynamicTool()}})
	if err != nil || facts.Budget.RunnerBytes != len(text)+len(codexapp.AskToolDescription) {
		t.Fatal(facts, err)
	}
	t.Logf("worker plus ask description Runner=%d", facts.Budget.RunnerBytes)
	params := map[string]any{"developerInstructions": strings.Repeat("d", 700), "dynamicTools": []any{map[string]any{"description": strings.Repeat("t", 301)}}}
	if _, err := StartThread(context.Background(), nil, params, "workspace"); err == nil {
		t.Fatal("aggregate session limit bypassed")
	}
	params["dynamicTools"] = []map[string]any{{"description": strings.Repeat("t", 301)}}
	if _, err := StartThread(context.Background(), nil, params, "workspace"); err == nil {
		t.Fatal("alternate Go container bypassed session accounting")
	}
	if err := SteerAsyncAnswer(context.Background(), nil, "t", "w", "turn", "c", strings.Repeat("c", 2001)); err == nil {
		t.Fatal("async oversize sent")
	}
	if err := SendAskReply(context.Background(), AskReply{Answer: b.Answer{Answers: map[string][]string{"q": {strings.Repeat("c", 2001)}}}}); err == nil {
		t.Fatal("ask oversize sent")
	}
}
func TestInputTurnChecksFinalFrozenText(t *testing.T) {
	for _, n := range []int{2000, 2001} {
		input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: strings.Repeat("c", n)}
		parts, _ := input.ModelInputParts()
		_, input.Budget, _ = m.DescribeInput(parts...)
		params, err := TurnParams(context.Background(), TurnParamsInput{Worktree: "workspace", ThreadID: "00000000-0000-0000-0000-000000000001", TurnInput: input, Sandbox: "read-only"}, nil)
		if (err != nil) != (n == 2001) {
			t.Fatal(n, params, err)
		}
	}
	text, budget, _ := m.FreezeInput(m.InputPart{Source: "caller.direct", Owner: m.CallerInput, Text: "frozen"})
	if _, err := TurnStartParams("w", "t", "c", text+"!", "read-only", "", "", nil, budget); err == nil {
		t.Fatal("append after freeze sent")
	}
	if _, err := StartTurn(context.Background(), nil, map[string]any{"input": []map[string]string{{"type": "text", "text": text + "!"}}}, budget); err == nil {
		t.Fatal("final send bypassed frozen check")
	}
	if _, err := TurnStartParams("w", "t", "c", strings.Repeat("r", 1001), "read-only", "", "", nil); err == nil {
		t.Fatal("bootstrap bypass")
	}
}
