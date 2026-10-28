package orcatransport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

// A bounded real two-turn adapter experiment. This test EXE does not qualify
// the shipped Runner CLI or development workflow; their evidence is separate.
func runNativeFollowupProbe(t *testing.T, ctx context.Context, l *Lifecycle, s b.LifecycleSnapshot, environment string, ready native.AgentState, record map[string]any, root string) {
	t.Helper()
	mode := os.Getenv("STEERRUNNER_ORCA_STATE_PROBE_MODE")
	nonce := process.New(0).NewProcessGeneration()
	firstMarker, secondMarker := "SR_FIRST_"+nonce, "SR_FOLLOWUP_"+nonce
	first := "这是隔离输入诊断。不得修改文件、运行项目测试、提交、推送或创建PR。\n请用一次 shell 工具执行无副作用命令 Write-Output 'SR_TOOL_OUTPUT'，确认输出后正常结束，仅回复 " + firstMarker + "。"
	if mode == "followup-output" {
		first = "这是隔离输入诊断。不得修改文件、运行项目测试、提交、推送或创建PR。\n请顺序调用8次 shell 工具，每次执行 1..100 | ForEach-Object { Write-Output ('SR_TOOL_OUTPUT_'+$_) }。等待所有原调用完成，再仅回复 " + firstMarker + "。"
	}
	second := "这是第一任务完成后的第二条诊断输入，模拟后续发布提示的中文、换行和长粘贴形态。\n不发布、不创建PR、不使用工具、不修改任何文件。忽略以下只读占位材料的含义。只回复唯一标记：" + secondMarker + "\n\n"
	filler := strings.Repeat("中文发布输入占位材料；保留原请求、完整正文及换行。\n\n", 150)
	second += string([]rune(filler)[:3427-utf8.RuneCountInString(second)])
	verified := []any{}
	record["verified_turns"] = verified
	for index, prompt := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("input-%d.txt", index+1)), []byte(prompt), 0600); err != nil {
			t.Fatal(err)
		}
		if index == 1 && mode == "followup-idle" {
			if err := l.waitOriginalComposer(ctx, s); err != nil {
				t.Fatal("explicit B readiness", err)
			}
			record["explicit_ready_at"] = time.Now().UTC()
		}
		input := b.TurnInput{CommandID: process.New(0).NewProcessGeneration(), Prompt: prompt}
		s.PendingTurn = &input
		r, err := l.prepareInput(ctx, request(s, b.LifecyclePrepareSession, b.LifecycleInput{Turn: &input}))
		if err != nil {
			t.Fatal("prepare original", index, err)
		}
		bind(&s, r)
		state, err := l.state(s)
		if err != nil {
			t.Fatal(err)
		}
		baseline := turnFor(&state, input.CommandID).Native.BaseRootAt
		r, err = l.submit(ctx, request(s, b.LifecycleStartTurn, b.LifecycleInput{Turn: &input}))
		record[fmt.Sprintf("submission_%d", index+1)] = r
		bind(&s, r)
		if err != nil {
			t.Fatal("sole original submit; no replay", index, err)
		}
		marker := firstMarker
		if index == 1 {
			marker = secondMarker
		}
		for {
			r, _, err = l.observeOnce(ctx, s)
			if err != nil {
				// Retry only a read-only connection timeout within the original
				// bounded probe. Never repeat prepare/submit or hide identity errors.
				if strings.Contains(err.Error(), "state streaming connection failed: timeout") && ctx.Err() == nil {
					record["observation_timeout"] = map[string]any{"at": time.Now().UTC(), "error": err.Error(), "turn": index + 1}
					continue
				}
				t.Fatal(err)
			}
			bind(&s, r)
			if r.TurnEvidence != nil && r.TurnEvidence.FinalVerified {
				if strings.TrimSpace(r.TurnEvidence.LastMessage) != marker {
					t.Fatal("wrong final", r.TurnEvidence.LastMessage)
				}
				sample, err := native.QueryOriginalSample(ctx, environment, s.Ref.External, ready.PaneKey)
				if err != nil {
					t.Fatal(err)
				}
				original, err := native.QueryOriginalInput(ctx, environment, s.Ref.External, ready.PaneKey, baseline, sample.AgentState)
				if err != nil || original.NativeInput == nil || original.NativeInput.Text != nativePromptText(prompt) || !original.NativeInput.MatchesOriginalProcess(s.Ref.External) || !original.NativeInput.MatchesOriginalQuery(original) || original.LastAssistantMessage != marker || original.ProviderSession.ID != ready.ProviderSession.ID {
					t.Fatal("independent original full input/identity/final verification", index, err)
				}
				verified = append(verified, map[string]any{"index": index + 1, "completed_at": time.Now().UTC(), "command_id": input.CommandID, "input_bytes": len(prompt), "input_sha256": digest([]byte(prompt)), "baseline": baseline, "query": original, "final": r.TurnEvidence})
				record["verified_turns"] = verified
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("original input did not complete; no replay", index, r.Diagnostic, ctx.Err())
			case <-time.After(500 * time.Millisecond):
			}
		}
		l.closeStateFeed()
	}
	record["completed"] = true
}
