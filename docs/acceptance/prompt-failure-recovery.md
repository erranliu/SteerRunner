# 检查、失败处理与等待提示词精简

日期：2026-11-09（Asia/Shanghai）。源码基线 `827ad83bf0785bbcd32ac5775c311f1a981ae6a5`。本次只改固定正文、对应维护说明及回归测试，不改工作流路由、权限、项目验收或原运行。结果为合并前定向验证，不代表完整 L1/Controlled、L2/L3 或实际模型资格。

## 诊断与维护依据

先读取本仓 [AGENTS.md](../../AGENTS.md)、[测试指南](../testing.md)、[Workflow 提示词维护约定](../workflows/README.md#4-提示词与输出约定)和[输入预算 Spec](../spec/workflow-input-budget.spec.md)。原则是每项要求只注入一次，保留必要动作边界，区分执行与报告，不增加通用检查框架。旧 Spec 的“提交失败或未知不重试”和 Git 版本专属清单与本次授权冲突，已同步收窄；Git 2.55 清单在本次基线仅存在于文档，不是本次删除的已注入正文。

用户提供的 Task 101 / Candidate Run `cbb69728650c40cd87527de4b7f2e30f`、development/2、gpt-6-luna/high 现场表明：额外 PowerShell 检查把单文件字符串的 `[0]` 当作路径，实际得到字符 `e`，退出 1 后被“非零即停”引向 `implement.input / waiting_input`。这是命令编写错误，不是 N101-CONTENT/N101-SCOPE 失败。正式验收未运行、未提交、未创建 PR 的原事实保持。

仅只读查看 `C:/SteerlineInstall` 目录及 `C:/Projects/SteerlineTest` 当前 checkout 的 AGENTS、Spec、Plan、测试指南和项目约束，未修改、恢复、停止或清理该 Run，也未运行其验收。当前 checkout 仅辅助区分职责，不冒充原 Run 冻结输入的逐字复现。该项目 Plan 登记 CONTENT/SCOPE 和三项模拟进度检查，统一入口在按输入要求提交后运行；Runner 不应再强制一份提前暂存范围模板。项目的真实 UTC、仅目标文件、原 baseline、安排摘要、报告和通过条件均保留。

## 来源及实际注入路径

| 来源 / 固定内容 | 实际注入点与作用期 | 处理及业务原因 |
| --- | --- | --- |
| Runner：`WorkerInstructionFormat` | `codex/worker_runtime.go:instructions` → `codex/backend.go:PrepareSession` → `thread/start.developerInstructions`，经 `sessionInput` 计量并写 thread receipt；原会话后续 turn 继承 | 精简命令 SOP，允许本阶段授权内修复；它是本次现场的强制停止来源 |
| Runner：同一 worker 正文的 legacy remote 使用 | `codex/remote_agent.go:RunRemoteAgent` → `thread/start.developerInstructions` | 同步获得新正文；属于保留 runtime 路径，不能据此声称当前结构化 Orca 注入了此规则 |
| Runner：普通实施 / audited 实施 / audit.code | `workflow/definition.go`、`prompt.go`；`application/freeze.go:freeze` 固定 Profile/PromptBodies，`prepareStage` → `workflow.InputParts/promptParts` → `FreezeInput` | 不变。要求实施、必要验证、适用提交及审计修复；没有“普通失败即停”或临时检查模板，不为去重再加一份恢复清单 |
| Runner：发布正文 | 同一冻结/渲染路径；两开发流程共享；`PublicationInstruction` 另追加实际 repository/remote/base | 改为未知先核实。删除固定 `git branch --show-current` 写法，保留当前分支、真实 PR URL 和原目标身份核验 |
| Runner：结构化 Orca 首轮 / 后续输入 | `orcatransport/agent_session.go` 的 `AgentCreate.FirstMessage` / `AgentSend.Body`；`agentBody` 原样一块传冻结正文，Codex/Pi、local/environment 沿各自资格 | 没有 worker developerInstructions，也没有这条“非零即停”；只共享新发布正文，不增加适配器规则或权限参数。不能宣称新 worker 自修复指令覆盖它 |
| Runner：初始化 | `NativeReadinessPrompt` 经 `agentlaunch.Config.Command/InitializationParts` 用于 legacy TUI；`BootstrapInstruction` 经 `remote_agent.go` 初始化 turn | 不变。READY/no-tools 只限初始化；前者已明确 READY 后结束。当前结构化 Orca 不经过这两条初始化，不把它们当业务停止要求 |
| Runner：`AuthenticationInstruction` | 只在 `codex.InputContract()` 公开；本基线没有生产发送调用 | 不变。不是当前额外注入的认证副本，不虚报删除收益 |
| Runner：实施/发布/审计 report | `definition.go` / `prompt.go` → 原会话 `turn/start` 或 Orca send；实际来源经 `workflow/input.go` 绑定原 command/turn/version | 保留“仅报告、不补做/不重发”和失败/未知事实。report 没有补做授权，worker 自修复限于本阶段授权，不解除此边界 |
| Runner：审计 parse / 独立解析 | `definition.go:StepAuditParse` → `textanalysis/cli.go:inputPrompt`，含 `ParserBoundary/ParserFormat` 和原输出；独立只读模型 | 保留不检查、不测试、不修复、不发布及不编造。与业务自修复职责不同；模型行为约定、原生权限和程序拒收分别存在，未放宽 |
| Runner：原生问答 | `protocol/codexapp/ask.go:AskToolDescription` 与原问题的结构校验/绑定，答案由原 answer/steer 路径发送 | 保留“只问必要的人类决策、等待答案”；等待真实答案是必要边界，不能变成所有错误都等待 |
| Runner：内容流程、execute/1 与运行文件 | 只读会话 `ContentSessionInstruction`；`application/content.go`；execute 默认“操作一次，保留结果”；`freeze.go` 按阶段追加 progress/result 或 Orca 文件绑定，后续沿同会话短说明 | 不变。只读限制、一次性操作、协调写入、封存和不以文件表示成功有独立价值，不属于额外预检 |
| 调用方：覆盖正文、材料、反馈、回答 | `Request.Workflow.PromptOverrides` → `PromptBodies`；Spec/Plan/References → materials；反馈/答案按 CallerInput 计量，原冻结输入恢复不重渲染 | 不改。Runner 默认正文精简不改写调用方文字，覆盖范围、来源、预算和成功标记保持 |
| 项目：AGENTS / Spec / Plan / 策略槽位 | AGENTS 由原生代理读取；Spec/Plan 给 worktree 相对路径。非 PlanDriven 的选中槽位经 `policyParts` 追加阅读路径，正文不内联；普通 development 的 PlanDriven 实施从材料入口发现规则 | 不改。项目决定验收及命令约束，调用方负责材料可用性。不得把项目内容归因为 Runner 固定提示，也不能用 Runner 精简取消项目检查 |

首次及后续业务投递最终通过 `application/freeze.go:prepareStage` 固定实际正文、片段来源及字节；直接 Codex 由 `TurnStartParams` 发送，结构化 Orca 由 create/send 发送。显式反馈/回答不是自动重试；旧 Run 的已冻结阶段正文继续原样，当前修改不会重写现场。

## 正文差异与字节

完整 before/after、UTF-8 长度及 SHA-256 保存在 [prompt-diff.json](evidence/prompt-failure-recovery-20261109/prompt-diff.json)。不含 Go/JSON 包裹引号、文件缩进和尾换行；只计算实际正文。资源 `prompts.json.publish` 与编译默认逐字同步，不能把这份目录副本再算成同一次投递的节省。

| 唯一正文 | 修改前 | 修改后 | 差值 |
| --- | ---: | ---: | ---: |
| WorkerInstructionFormat 格式串 | 479B | 476B | -3B |
| worker 实际展开（`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`） | 541B | 538B | -3B |
| publish 默认正文（每个发布步骤） | 150B | 106B | -44B |

两类唯一格式正文合计 629→582B（-47B）；不是单次模型请求大小。worker 新增恢复语义抵消了大部分删除收益，但未增加正文长度或另一套检查模板。shell 引号转义及 AskToolDescription 继续合计受 1000B 限制，其余正文/固定后缀未改。

worker 修改前：

```text
shell=%q,login=false。临时文件使用本次Run指定临时目录并同次清理，禁写认证/Git管理根。不得输出环境/凭据、提权、登录、改认证或权限。仅精确暂存任务路径，检查非空暂存diff且仅含任务；串行暂存/检查/提交，非零即停。commit不用管道，紧接命令保存并打印其$LASTEXITCODE，再查状态。失败或未知（含认证过期）保留原分支/PR结果并停止对账；未知发布不重做。
```

worker 修改后：

```text
shell=%q,login=false。临时文件限本次Run指定临时目录，用后清理；禁写认证/Git管理根，禁输出环境/凭据、提权、登录、改认证/权限。仅暂存并提交任务改动，核实结果。优先项目验收，避免重复预检；命令/测试错误可修复则在本阶段授权内修复复验，仅缺必要信息、授权或外部条件才等待。提交/推送/发布结果未知先核实原操作，保留原分支/PR，不盲目重做。
```

发布正文差异（两开发流程共用）：

```diff
-发布当前分支并创建 PR；用 git branch --show-current 确定 head。返回真实 PR URL；结果未知时说明并停止，不重复发布。
+发布当前分支并创建 PR；返回真实 PR URL；结果未知先核实原操作，不盲目重做。
```

| 删除 / 保留 / 收窄 | 业务原因 |
| --- | --- |
| 删除强制“非空暂存 diff 且仅含任务”预检模板 | 不强制编写与正式 SCOPE 验收重复的临时检查；仍须仅暂存任务改动并核实结果 |
| 删除“串行暂存/检查/提交，非零即停” | 操作有依赖时自然顺序执行；非零是诊断事实，不能把可修复命令错误变为人类阻塞 |
| 删除 commit 管道禁令、打印 `$LASTEXITCODE`、再查状态的固定写法 | 它们是命令实现选择。保留实际结果核实，退出码不能单独证明成功，也不能用后继命令遮蔽失败 |
| 将“失败或未知停止”拆为可修复错误与未知副作用 | 允许在本阶段授权内诊断、修复、复验普通失败；未知先保留原分支/PR并核实原操作，不能盲目重复提交/推送/发布 |
| 保留 shell/login、临时目录和清理、认证/Git 管理根、凭据/权限禁令 | 防止改变身份或扩大执行边界；不授权代理自行登录、提权或修复被拒绝的权限 |
| 保留项目验收、当前分支、真实 URL、报告/解析与成功行约定 | 项目决定验收；Runner 核实来源、完成及发布身份。报告并非重新实施，声明不能覆盖真实失败或未知 |
| 等待限于必要信息、授权或外部条件 | 真实依赖必须等待；缺少可自行获取的事实、自己的只读命令错误或可修复测试失败不自动要求用户介入 |

## 独立审核、路由与验证

独立只读审核 `prompt_review` 核对了生产接线及维护规则，没有以目录样本更新作为判定。初审指出会话级规则可能越过 report 的“不补做”；已加入“本阶段授权内”并更新回归断言。复审确认生产正文、阶段授权、项目规则及注入路径无剩余实质问题；另指出发布正文附件仍保留逗号版本，已刷新为实际分号正文并核实 SHA-256。审核记录见 [independent-review.md](evidence/prompt-failure-recovery-20261109/independent-review.md)。

路由不变：`workflow.Route` 只消费已核实的 turn 结果，普通工具失败不会单独触发 input。代理在当前执行轮内修正后仍可按原成功标记完成；若原轮已明确输出“未完成”，现行定义进入 input。没有证据表明需要新增自动返工边；那会重发已结束的业务输入，并扩大本次授权与重试范围。原生结果/来源未知仍只能对账，不能用用户“继续”越过门槛。

自动测试结果及源码摘要在 [validation.json](evidence/prompt-failure-recovery-20261109/validation.json)，原 JSONL 已 gzip 保留。

| 本轮执行 | 实际结果 / 覆盖边界 |
| --- | --- |
| 六个受影响包全包：codex、workflow、resources、terminaltools、orcatransport、textanalysis | 222 个顶层测试通过，0 失败，1 个原有 helper skip（`TestServiceStopWorkerHelper`，未启用 helper 子进程模式）；六包均成功。不是完整 L1 |
| application 精确八根冻结/后继/恢复/补报/结构化选择定向 | 8/8 顶层通过，0 失败/skip，保存原冻结输入与未知对账边界 |
| 最终发布标点变更后的 workflow/resources/testkit-protocol 全包复验 | 43 个顶层通过，0 失败/skip；没有修改 fixture 或黄金样本适配正文 |
| 正式入口 `test-controlled.ps1 -Combination result-plain` | `TestSRResultAcquisition/plain-marker` 实际完成，exit 0、1/1 case 通过；包含原四项 foundation 单元前置和本次固定 EXE。summary 的 `qualification_eligible=false`、`complete_controlled_passed=false` 保持 |
| 首次默认沙箱执行 | owners 15 根失败、adapter/parser 7 根失败、application 3 根失败；临时 Git/socket/junction 权限及 sandbox 路径影响。保留三份失败日志；改用宿主权限原命令复验通过，未改产品权限或断言豁免 |

关键断言的资格分别是：

| 边界 | 自动验证 | 仍需实际模型验证 |
| --- | --- | --- |
| 自身只读检查写错后继续 | 新 `TestTerminalToolReadOnlyCheckCanBeCorrected` 真实执行错误路径读取并保留错误，随后同工作区正确读取成功、证据未改。新 worker 测试检查阶段授权、自修复、等待与未知边界确实进入会话输入计量/公开合同，且旧检查模板不再出现 | 模型是否识别自己写错、主动修正并完成验收；测试程序选择第二条命令不能代替此项 |
| 普通测试失败 | 文字合同允许在本阶段授权内修复复验；原结果/验收门没有改变 | 真实模型修复并重新执行项目已登记验收，而非提前输出未完成或伪造通过 |
| 外部操作结果未知 | `TestStructuredDevelopmentFollowUp/lost-response`、`TestStructuredLostCloseRemainsUnknownWithoutReplay`、`TestStructuredDurableCloseReceiptReconcilesWithoutReplay` 及 application 冻结未知对账用例通过，验证原输入/close 不因丢回执重发；发布文字要求核实原操作 | 模型内部 git commit/push/PR 创建丢响应时查询原结果、保持原身份且不盲目重做。协议零重放不等于验证这些模型内部动作 |

受控前置、定向和 helper skip 都不记为实际模型通过。未运行完整 L1、完整 Controlled、L2/L3 或合并后验证；验证期间没有发布 PR 或执行合并。

实际模型行为尚未验证。需要在全新独立现场，用 gpt-6-luna/high 及生产注入验证：错误只读检查后自行诊断修正并完成项目验收；普通可修复测试失败后修复复验；模拟提交/推送/PR 已生效但响应丢失时先查询原操作、业务写入不重做；缺必要授权或外部依赖时准确提问；report 只整理既有事实。单元测试中的确定性工具调用由测试程序选择，受控 peer 的固定响应也不能证明模型会自主遵循提示词。本次不恢复原 Candidate Run，不以此宣称 Task 101 已解阻。

初次工作区交付前于 2026-11-09 11:40:06 +08:00 通过 GitHub connector 读取真实 `main=676078d2731df725460eb40046cb0d8e6fd435e1`，唯一新父链提交基于本地 `827ad83b`。当时其修改路径与本次无交集。`git fetch` 因宿主 HTTPS TLS 失败未刷新本地 remote ref，采用连接器远端事实，未声称本地 origin/main 已更新或已验证新主干构建；当时未创建远端分支/PR，不能把文件无交集冒充平台合并资格。原事实见 [target-check.json](evidence/prompt-failure-recovery-20261109/target-check.json)。

随后用户授权提交 PR。创建前重新执行 `git fetch origin main` 成功，本地 origin/main 与连接器均为 `676078d2`；保留已验证分支历史，不为追平主干改写提交。本次仅补充证据目录的 `.gitattributes` 原字节保留规则及本段交付说明，产品源码和测试源码摘要未变。原测试范围及实际模型缺口保持，不补签新主干或合并后测试资格。PR 的平台合并状态在创建后查询并由交付回复记录；未部署安装版。
