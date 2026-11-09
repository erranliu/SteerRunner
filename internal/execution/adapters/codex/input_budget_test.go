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
	if err := SteerAsyncAnswer(context.Background(), nil, "t", "w", "turn", "c", string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8 async answer sent")
	}
	if err := SendAskReply(context.Background(), AskReply{Answer: b.Answer{Answers: map[string][]string{strings.Repeat("q", 1001): {"answer"}}}}); err == nil {
		t.Fatal("ask Runner framing overflow sent")
	}
}

// This checks the model-visible policy, not whether a model follows it.
func TestWorkerInstructionsAllowRepairWithoutBlindReplay(t *testing.T) {
	text := (&workerRuntime{Shell: "powershell.exe"}).instructions()
	for _, required := range []string{
		"仅暂存并提交任务改动，核实结果",
		"优先项目验收，避免重复预检",
		"命令/测试错误可修复则在本阶段授权内修复复验",
		"仅缺必要信息、授权或外部条件才等待",
		"提交/推送/发布结果未知先核实原操作，保留原分支/PR，不盲目重做",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing execution boundary: %s", required)
		}
	}
	for _, obsolete := range []string{"非零即停", "$LASTEXITCODE", "检查非空暂存diff", "串行暂存/检查/提交", "失败或未知"} {
		if strings.Contains(text, obsolete) {
			t.Fatalf("reintroduced stop/check template: %s", obsolete)
		}
	}
	facts, err := sessionInput(map[string]any{"developerInstructions": text, "dynamicTools": []any{codexapp.AskDynamicTool()}})
	if err != nil || facts.Parts[0].Text != text || facts.Parts[0].Source != "native.developer" {
		t.Fatal("session did not account for actual worker instructions", facts, err)
	}
	if InputContract().(map[string]any)["worker"] != WorkerInstructionFormat {
		t.Fatal("published policy differs from worker session policy")
	}
}
func TestInputTurnChecksFinalFrozenText(t *testing.T) {
	for _, n := range []int{1999, 2000, 2001, 2999, 3000, 3001, 10552} {
		input := b.TurnInput{CommandID: strings.Repeat("c", 32), Prompt: strings.Repeat("c", n)}
		parts, _ := input.ModelInputParts()
		_, input.Budget, _ = m.DescribeInput(parts...)
		params, err := TurnParams(context.Background(), TurnParamsInput{Worktree: "workspace", ThreadID: "00000000-0000-0000-0000-000000000001", TurnInput: input, Sandbox: "read-only"}, nil)
		if err != nil {
			t.Fatal(n, params, err)
		}
		if _, err := TurnStartParams("w", "t", "c", input.Prompt, "read-only", "", "", nil, input.Budget); err != nil {
			t.Fatal("long frozen caller input rejected", n, err)
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
