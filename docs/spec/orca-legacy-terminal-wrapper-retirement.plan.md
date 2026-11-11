# Plan：Orca 旧终端 create / send / read Go 包装器退役

日期：2026-11-11。状态：2026-11-11 已实施，合并前定向验收通过。依据 [Spec](orca-legacy-terminal-wrapper-retirement.spec.md)。调查主线 `e6cac6e17bf1b8de198c115a06308361b42d2b3c`（#142），原设计 HEAD `389f11952ce2825576b064df7d8ea460dcace8b3` 与之文件树相同，实施分支从审计主线建立。

## P0. 核对消费者并建立基线

1. 获得实施授权后刷新 main，核对两个提交的实际文件差异。工作区使用 Windows/amd64、PowerShell 7，按 [测试指南](../testing.md) 记录实际宿主、命令、退出与运行范围。
2. 全仓查找 Spec §2 的声明、引用、方法值、专属测试及 `ptrString`，检查 build tag、cmd、tests、helper、脚本及登记入口。另沿现行工作区创建/对账、历史 reader、终端清单/关闭和结构化会话确认保留项。记录 Pi probe 的 executable 调用及 stability 的 CLI、状态和 driver/helper 路径，发现额外消费者先更新清单。
3. 冻结六个原文件及其余源、共享测试、Pi/stability 脚本/工具的字节清单。以同一捕获方法记录代表历史/当前 profile、state/record/receipt 编码、全部捕获类型声明和固定 schema/capabilities/resource_digest，实施前后对比。要求类型差集恰为 `resource_create.go::TerminalCreate/TerminalCreation`、`terminal_send.go::TerminalSend/TerminalPromptReceipt/TerminalSendReceipt`、`terminal_read.go::TerminalRead/TerminalReadPage`；其余类型与所有编码/资源项目相同。样本只说明代表编码，完整字段保持同时由精确变更范围举证。
4. 删除前执行 P2 的同一 adapter 八项、livetest 两项及下列 protocol 七项。登记运行中的两个专属顶层测试、混合测试的十个 worktree 子例和七个 terminal 子例，保存原 JSON、stderr、退出与实际名字。

```powershell
go test ./internal/execution/adapters/protocol/orca -run '^(TestOrcaResourceCreateCapability|TestOrcaTerminalSendCapability|TestOrcaTerminalReadCapability|TestOrcaCloseInventoryCapability|TestBoundTerminalRejectsRuntimeAndIncarnationReplacement|TestTerminalCloseRequiresExactRuntimeHandleAndPtyProof|TestOrcaResourceQueries)$' -count=1 -timeout=5m -json
```

## P1. 仅删除已批准的包装器及专属测试

1. `resource_create.go` 删除两组 terminal 参数/回执声明，保留 worktree 原文、imports 和字段。
2. 删除 `terminal_send.go`、`terminal_read.go` 及两份同名 `_test.go`，专属断言随被测能力退役。
3. `resource_create_test.go` 只删除两个 terminal 循环和 `ptrString`。以原文件扣除这些块后的字节核对保留部分。三项 `agent-codex`、`agent-pi`、`agent-` 验证原参数、显式空 agent 与缺省差异；七项 `worktree-complete`、`worktree-bare`、`worktree-missing`、`worktree-bad-worktree`、`worktree-bad-agent`、`worktree-bad-startup`、`worktree-extension` 验证原解码、调用事实和容错。
4. 复查实际变更恰为两个混合文件和四个删文件。保留历史文档和样本中的旧名，静态零引用只按当前可编译代码与可执行入口判定。其余 Go、PowerShell、JS 和配置入口原字节保持。

## P2. 合并前最小定向验收

### P2.1 受影响组件

```powershell
go test ./internal/execution/adapters/protocol/orca -run '^(TestOrcaResourceCreateCapability|TestOrcaCloseInventoryCapability|TestBoundTerminalRejectsRuntimeAndIncarnationReplacement|TestTerminalCloseRequiresExactRuntimeHandleAndPtyProof|TestOrcaResourceQueries)$' -count=1 -timeout=5m -json
go test ./internal/execution/adapters/orcatransport -run '^(TestWorkspaceCreationFacts|TestWorkspaceCreationCodecPreservesCheckpoint|TestWorkspaceCreationBindsCLIResult|TestStructuredWorkspaceRecoversOriginalOwnerOutput|TestCreationOwnerUnknownAndForeignEvidenceNeverRedispatch|TestFrozenReaderHasNoExecutionAndRejectsMutation|TestTerminalStoppedRequiresCompleteOriginalHost|TestLocalCodexStoppedUsesRegisteredResources)$' -count=1 -timeout=5m -json
go test ./internal/testkit/livetest -run '^(TestDiagnosticReadNeverReplaysMutation|TestAuxiliaryCloseReconcilesExactIdentityAndCompleteInventory)$' -count=1 -timeout=5m -json
```

最终 protocol 五项、adapter 八项、livetest 两项都必须实际匹配并通过。两项 livetest 分别覆盖 raw mutation 零重放和辅助关闭的原身份、清单完整性。对照前后 JSON 中 `TestOrcaResourceCreateCapability` 的子例名字和结果，最终恰为 P1 的十个 worktree 子例。检查原两项专属测试已从源码移除；Go 零匹配只说明没有执行，不能记为测试通过。报告所有失败与 skip，基线与最终记录分开。只有执行了正向回执、未知零重派、历史 reader 零 mutation 和原停止清单的相应断言，才签收对应条款。

### P2.2 Pi 与 stability 入口的静态和编译检查

```powershell
$tokens = $null
$parseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseFile(
    (Resolve-Path -LiteralPath ./scripts/probe-orca-pi.ps1).Path,
    [ref]$tokens,
    [ref]$parseErrors
)
if ($parseErrors.Count -gt 0) { throw ($parseErrors -join "`n") }

$out = '.evidence/orca-legacy-terminal-wrapper-retirement/final'
New-Item -ItemType Directory -Force -Path $out | Out-Null
go build -trimpath -o "$out/stability.exe" ./tests/helpers/stability
if ($LASTEXITCODE -ne 0) { throw 'stability helper compile failed' }
go test -tags=live -c -o "$out/native.test.exe" ./tests/native
if ($LASTEXITCODE -ne 0) { throw 'native driver compile failed' }
```

静态核对 `Invoke-ProbeOrca` 参数来源、stability 的 raw CLI/原 runtime 核对以及 helper 的状态订阅调用。比对入口、夹具和脚本原字节。上述 driver 编译形态沿 [test-native.ps1](../../scripts/test-native.ps1)，helper 沿 [new-test-fixture.ps1](../../scripts/new-test-fixture.ps1)。保存退出与产物摘要。仅解析 Pi 脚本、编译两个产物，实施验收中不启动 probe、native driver、Orca 或网络故障工具。

### P2.3 正式构建与同一通过点

```powershell
./scripts/build.ps1 -OutFile .evidence/orca-legacy-terminal-wrapper-retirement/final/steerrunner.exe
```

复核正式构建的 source_commit/source_dirty/source_digest、EXE SHA256 与最终输入，比较 P0 固定编码、声明与资源捕获。定向结果仅支持本批删除边界。完整套件、公开受控、真实 Pi/stability 和真实 Orca/模型场景另按原入口和实际需求验证。

## P3. 记录与结束

按 OTC-01–05 在 `docs/acceptance/orca-legacy-terminal-wrapper-retirement.md` 独立登记消费者、删除与保留、十项 worktree 子例、Pi/stability 实际核对、运行范围及构建/资源证据。刷新目标并核对差异；若目标推进，先核对受影响入口，按影响补验。验收完成后更新 Spec/Plan 与索引。本轮授权包括实施和验收。

独立审计确认原六文件范围、两个专属测试、十个 worktree 子例以及 Pi 和 stability 的入口。按审计结果补录辅助关闭路径与两项现有隔离测试，并将类型一致性收紧为仅允许批准的七项差异。

2026-11-11，交付前刷新 main 仍为实施基线。前后定向测试、Pi 静态解析、stability helper/native driver 编译及正式构建均通过，见[独立验收](../acceptance/orca-legacy-terminal-wrapper-retirement.md)。
