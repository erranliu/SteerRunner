# Plan：Orca 无 helper 原生可行性核验 B0

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：P0–P6 已完成，独立审计确认的 P2 已修复并只读复核；见[验收](../acceptance/orca-native-capability-baseline.md)。依据 [Spec](orca-native-capability-baseline.spec.md) 和[长期路线](../../../../spec/orca-capability-layer.spec.md#6-分批实施路线)。本 Plan 只执行 B0，代码基线 `efa871cfffb950505bf2f0a325ec7751fb4be7f3`；既有长期设计文档工作树差异属于本任务，不覆盖或回退。

## P0 固定现场与实现边界

- [x] 记录当前源码、分支、Orca CLI/Codex 版本及摘要；使用会话已选定的 Orca CLI，读取版本匹配 guide/help，禁止临时换构建。
- [x] 在调用方进程屏蔽隐式 Orca 路由，明确本机/远端目标及其 user-data 来源；不改全局环境和配对配置，不把认证原文写入日志。
- [x] 使用现有 fixture/Orca 机制为两端分别分配本轮专属空 worktree，保存完整 ID/runtime；未知创建仅对账，不重建。探针内部只创建终端。
- [x] 固定本批白名单和当前生产文件摘要。仅允许测试文件、现有脚本的新 opt-in 模式与文档；记录旧 long-input/controlled 模式的前置及结果条件，后续逐项回归。

本批可在测试中读取当前 profile/State/Turn，但不修改它们的生产结构。提前确认本机无 helper 的 NewLifecycle 构造接线可用，以及当前正式本机 verifyStopped 不接受此终端范围；后者是差距，不是本批应绕过的门禁。

## P1 扩展同一个探针入口

| 文件 | 计划改动 | 保留项 |
| --- | --- | --- |
| scripts/test-native-input.ps1 | ValidateSet 增加 capability-baseline；新模式独立验证无 helper 目标和两轮结果，冻结 target/source/EXE 身份 | long-input 的 helper 前置、单轮计数；controlled 的现有 roots、矩阵和行为 |
| orcatransport/native_live_test.go | TestOrcaNativeStateLiveLifecycle 先分派新模式到 B0 实现 | 原 opt-in、原模式及默认 skip 行为 |
| 新 orcatransport/native_capability_probe_test.go | 固定目标验证、真实构造器的新双端原生探针、证据及清理校验和负例 | 不新增生产 API、独立状态机或原文匹配算法 |
| 必要的现有 *_test.go 支撑 | 复用/最小调整实际 CLI 记录器，让每轮 terminal/request 分别计数 | 不改变正式提交、回执、history、原输入及 final 判据 |

新模式复用 nativeInputProbeTarget，拒绝 RuntimeExecutable/AuditModel，要求显式 full_access、固定模型偏好和原资源身份。脚本必须在任何副作用前选择新/旧前置条件；不能为了允许 B0 而取消旧本机 helper 检查。

为实际命令和结果来源建立可核验的零旧链检查。只计业务发送时按 terminal/request 区分两轮；初始化、创建和清理独立记录。不得简单删除旧 `submit_calls == 1` 判断，必须让旧模式仍要求一轮，新模式明确要求两轮分别有效。

## P2 实现两轮原生探针

1. 创建 attempted 标记并保存目标摘要，验证实时 runtime 和完整空清单；错误或歧义时零 terminal create。
2. 用现有 Config.Command 生成无 helper Codex 启动命令，创建唯一终端，取得完整原身份；记录 READY 和 provider/pane。必要的输入就绪等待仅调用已有 waitOriginalComposer，沿原预算，不增加睡眠或独立完成逻辑。
3. 构造真实 NewLifecycle：本机使用 ProfileVersion，远端使用 NativeRemoteProfileVersion；RuntimeExecutable/Terminal 均空。State codec 由当前实现确定，不把本机编码为远端。不替换 stateFactory/queryInput/clock、不预置 Started。
4. 复用既有探针的 `operation.New`、发送前本地 intent/receipt 记录、Execute/ObserveLifecycle 和 event 确认方式执行第一轮；使用生产 QueryOriginalInput 和 NativeProof 规则独立复核原文/final。
5. 仅在第一轮已完成、原终端和 provider session 仍一致后执行第二轮，更新到第一轮结束后的基线；nonce A 不进入第二轮正文，精确核对返回的 B/A 组合。新 request/command 各唯一，任何未知立即停止后继输入。
6. 验证两轮 State/Turn 无 managed 控制和 recorded history，结合原命令/资源日志确认零旧链。测试装配、额外等待、实际取证路径分别记录，不能把它称为正式 Runner 新 Run 已通过。
7. 在唯一清理路径核对原资源并最多 close 一次，用 VerifyTerminalsStopped/原完整清单证明退出或缺席，随后核对专属 worktree 完整空清单。正常和失败分支均收尾；不调用本机 helper StopLifecycle 后吞错，不输出正式 resources_settled。

整个现场沿 Spec 的 6 分钟原总期限，异常清理上限 45 秒；脚本测试进程仍为 10 分钟。连接/查询可按现有只读规则继续，创建、业务 send 和 close 均不自动重试。未知创建无确切身份时不能清理认领；保存人工处理信息，保留现场，不重用同一 worktree 继续资格。

## P3 结果格式和必需反例

新模式继续产出 native-result.json 与 result.json，新模式记录独立 discriminator，包含两轮明细和逐目标 qualification，不把本批格式扩成公开 Run schema。

| 记录 | 必需内容 |
| --- | --- |
| 身份 | mode=capability-baseline、源码/EXE/CLI/target 摘要、runtime/worktree/terminal/incarnation/pane/provider |
| 每轮 | command/operation/request、逻辑正文及 wire 摘要、原基线、实际业务发送数、accepted、原输入查询页/归属、原生新 done、完整 final 及摘要 |
| 零旧链 | helper/control/recorded 调用证据、State/Turn 来源、实际命令和清单相互一致 |
| 收尾 | close 调用及不确定事实、完整原 host 清单、cleanup_confirmed、未解决原因；不使用正式 Run stop scope 成功字段 |
| 结论 | 两轮均有效、业务发送总数 2、text-only/bare-Enter/正文屏幕读取 0、零旧链、原资源清理确认；所有条件成立才能 passed |

计划新增的确定性测试根为 `TestOrcaCapabilityProbeTarget`、`TestOrcaCapabilityProbeEvidence`、`TestOrcaCapabilityProbeCleanup`。它们是拟新增入口，不代表当前已有或已通过。具体子例在实现前列表冻结，至少覆盖：

- 目标：有效本机/远端、带 helper、带 audit model、缺固定身份/非法 launch、非空或不完整清单；拒绝分支检查零创建/发送。
- 结果：只有一轮、重复/缺 request、错 provider/incarnation、managed/Recorded 来源、final 缺失/截断、计数缺失或一轮多发、清理未确认、错误 mode；全部拒绝通过。原输入反例仍以生产核验器测试为准，不在 reporter 复制一套原文归属算法。
- 清理：响应丢失且完整清单已缺席、仍活动、换 incarnation、缺 host/截断、缺原身份；检查最多一次 close，未知后只有查询，无新建/发送/额外关闭。

新模式通过必须由其实际测试根及 mode-specific 结果确认，不能仅凭旧根名称 PASS、默认 skip、测试进程退出码 0 或两个计数为 2 签收。脚本对 mode/摘要、每轮 request/证明、总数及清理做一致性核对。

## P4 定向检查与构建冻结

实施后运行下列命令，保存 test2json、退出码、实际命中 roots/叶例和 skip；本轮写文档不执行这些未来测试。

```powershell
go test -json ./internal/execution/adapters/orcatransport -run '^TestOrcaCapabilityProbe(Target|Evidence|Cleanup)$' -count=1 -timeout=2m
go test -json ./internal/execution/adapters/orcatransport -run '^Test(NativePromptSubmission|NativePromptWireFormats|NativePromptReceiptFaults|LocalNativeProbeOriginalQueryConstructor|LocalNativeProbeTargetAndBusinessScope|LocalNativeProbeCensusKeepsOriginalHostScope|NativeUnstartedReceiptRecoversOnlyCompletedOriginalInput)$' -count=1 -timeout=4m
go test -json ./internal/execution/adapters/protocol/orca -run '^Test(TerminalStoppedRequiresCompleteOriginalHost|TerminalsStoppedUseOneCompleteOriginalInventory|QueryBoundTerminalMissingIdentityDoesNotConnect|QueryBoundTerminalHandshakeCancellation)$' -count=1 -timeout=2m
git diff --check
```

用 PowerShell parser 检查修改后的脚本；通过测试编译证明真实入口可构造。新模式单独演练结果校验反例，不能放宽旧模式。最后检查白名单与生产摘要：任何非测试 .go、配置/codec、旧业务素材或 Orca 全局设置变化均停下审查，不能归入 B0。

同一源码固定测试 EXE，双端只改变 target/证据目录。脚本当前每次编译需保存两次摘要并确认相等；如果不同则不合并为同源资格。已有默认 L1 不运行真实模型，不能用其 skip 作真实通过。

## P5 双端新现场各一次

以下为新模式实现后的命令模板；目标文件必须来自 P0 的实际资源，不能直接运行占位路径。

```powershell
.\scripts\test-native-input.ps1 -Mode capability-baseline -Target <本机B0目标.json> -OutDir <全新本机证据目录> -Repeat 1
.\scripts\test-native-input.ps1 -Mode capability-baseline -Target <远端B0目标.json> -OutDir <全新远端证据目录> -Repeat 1
```

默认串行，两个目标各一个新终端、两个业务回合。若失败，保留证据并先确认原资源收尾；只对受影响项建立新的专属现场补验，不向旧 request/目录补发。环境条件缺失如实阻断，本机通过不放行远端或 B3，不修改真实 Orca/登录/权限来凑通过。

## P6 验收、commit 与独立审计

- [x] 新增 `docs/acceptance/orca-native-capability-baseline.md`，按 OCB0-01–07 记录实际源码/产物、命令、命中数、双端原始证据、失败/补验、业务和清理各自结论。
- [x] 能力差距表至少列原输入/完整 final/同会话后续、启动模型偏好与实际模型未核验、workspace/audited 未验、Git/PR 未验、原终端停止与本机现行 helper scope 的差别。
- [x] 更新长期路线 B0 状态及 Spec/Plan 实际状态。只有双端及清理证据齐备才能标完成；不更新生产支持或 B5 资格。
- [x] 按用户目标提交本批实现和验收 commit，不提阶段 PR；之后由独立只读审计者核对设计、实际 diff、生产不变、证据和未知零重发。
- [x] 确认的修复单独 commit，补跑受影响检查并经原审计者复核；未知创建 P2 已关闭，未验范围保留。全部批次完成后再统一提一个 PR，不合并或部署。

停止条件：OCB0-01–07 全部有直接证据，资源已确认收尾，独立审计完成且确认问题已修复。本批未完成不阻止长期路线允许的 B1/B2a 独立收益工作，但不能开始依赖 B0 通过的 B3。
