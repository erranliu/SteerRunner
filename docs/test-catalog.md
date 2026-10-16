# SteerRunner L2–L4 测试清单

本文件维护各项测试的独有验证目标、固定预期、并行条件、实现状态和当前缺口。
分层定义、设计及清单维护规则、运行入口和证据要求见[测试指南](testing.md)。
已实现项对应仓库注册；规划项使用文档编号，不计入已实现或通过数量。

以下清单于 **2026-10-16** 对照[执行目录](../tests/catalog.json)和
[唯一 case 定义](../internal/testkit/livetest/cases.json)核对。入口支持
`L1,Controlled,L2,L3,L4` 和 `-MaxParallel 1..8`；L4 独占执行，L2/L3 有界并行。
已注册 L2 一 case、L3 二十 case、L4 四 case；原 11 个可靠性 case 全部保留在 L3。
以下区分实现状态与实际受控/真机验收，旧构建的通过结果不作为新构建资格。
本轮实际结果、配置来源和失败历史见[测试功能验收](acceptance/test-levels.md)。
PR 整合主线后保留两项本机环境检查，现补齐两项 Orca 环境检查和显式 full_access 远端开发闭环，当前为 L3 十四场景/二十 case；下文已有真机通过
记录仍绑定原构建，整合后不继承反馈资格，须以新构建重新验收。

## L2：最小核心 smoke

| 场景 ID / Case ID | 独有验证目标与目标范围 | 当前入口、预期及缺口 | 并行条件 |
| --- | --- | --- | --- |
| `local-codex.smoke` / `default` | 最小开发交付：冻结 Spec/Plan → 接受任务 → 执行 → 发布并核验 `pr_ready` → driver 合入核验 → 停止及清理 | `-Level L2`；预期 `merged`。已实现 `-FeedbackQualification` 资格门，并正式通过无反馈核心模式 1/1；无资格仍执行原完整 smoke，构建或执行配置变化后重新取资格 | 每次独立现场/state-dir/Run/本机 worker/分支/PR/认证 capability；无共享可变构建输出；本轮 L2 串行验收，实际并行资格单独登记 |

前置为 L1、完整 Controlled、本机原生 Codex 资格、SteerlineTest 仓库及执行主机认证。
L2 另有同权限、无模型的 Git 提交前置：loose/packed head 均须无 stderr 错误，
跨仓库写入须被拒绝。业务 smoke 使用生产默认提示词及真实 Git/gh 命令，不装载测试
发布器；审计实际命令输出中的权限拒绝，退出码 0 不能掩盖 `packed-refs.lock` 错误。
默认 broker 收尾还要求可信拒绝计数为 0，缺计数或 HTTP 401/403 均不能通过。
每次独立现场、Run、base/head 和 PR；执行步骤及收尾见[测试指南的完整 smoke 契约](testing.md#当前完整-smoke-契约)。
目标 L2 不包含额外反馈轮、恢复或审计流程；driver 的真实合并可保留为交付核验手段，
不把合并变成 Runner 的生产交付能力，保留 `merged` 预期和原 PR/分支/现场证据。
独立 `local-codex.feedback` 正式通过后，向 L2 传该批次的
`local-codex.feedback-default-r1-case.json` 路径，native driver 再校验原场景、资格文件摘要、
同 Runner/driver 构建和反馈/合入/收尾事实。不能用旧构建资格或伪造缺项结果删除反馈覆盖。
本机当前认证版本的实际结果见[沙箱认证验收](acceptance/sandbox-authentication.md)，
通过结论须绑定对应源码、构建和配置。

## L3：不同流程与核心可靠性

以下为已注册的十三场景、十九 case，含原远端流程、11 个核心可靠性 case、三个本机流程和本机/Orca 各两种权限环境检查。全部保留在
L3，使用 `-Level L3 -Scenario <场景 ID>` 运行；选择场景即运行它的全部 case。

**通用并行条件：** 每 case/repeat 独立现场、冻结材料、state-dir、服务实例和所属 Run、远端
worktree/terminal/session、base/head、PR、控制门、journal 和证据目录；构建使用独立输出
或一致且只读的冻结产物。可以共享只读安装和已配对 environment，但必须验证并发容量，
每项只能停止/清理自己的资源，不能切断共享 runtime 或整机网络。模型和平台限流需要
明确并发上限，资源等待不能误记为故障命中。并行验收须证明各项身份、输入/发布计数、
故障窗口与清理互不干扰，不能只看最终全部 passed。
环境检查使用独立 Check，运行中核验 session/workspace 和 owned base/head/PR，收尾
后保留身份；不要求创建 development Run，不用已删除的 owned 资源冒充未执行。

上述隔离适用于 case 之间。L3-P04 已实现同一服务的多 Run，允许 case 内多个 Run
共享一个服务/state-dir；该服务与其它 case 独立，不能以“每 case 独立服务”跳过产品
内部的隔离验证。现场生成器把 Runner 构建输出放在现场专属路径，协调器核验同批
Runner/driver 摘要及跨 case 的 Run/session/workspace/branch/PR 身份；后端实际资格
仍按[本轮验收](acceptance/test-levels.md)登记，不能从独立输出本身推导并发通过。

**通用当前缺口：** 已实现有界并行及 worker 结果、摘要与跨 case 身份校验；框架级测试
验证 L3 重叠、L4 独占、worker 无结果及构建变化拒绝。本机反馈与两模型审计已在并发上限
2 的正式批次共同通过；这只登记该构建、配置与后端组合。其它组合、远端共享 environment、
更大模型/平台容量及跨批次真实收尾的并发资格仍待验证。当前认证版本
的远端可信认证准备/同沙箱检查有能力缺口，见[沙箱认证验收](acceptance/sandbox-authentication.md)，
因此不能沿用旧构建通过结果声明当前版本 L3 已通过。下表各项均适用这些缺口。

宿主槽锁由协调器持有，只覆盖存活的正常调度批次。协调器异常退出后可能遗留后台
worker/服务，须先对账原现场并收尾，再启动独占测试；当前没有孤儿资源自动接管资格。

| 场景 ID | Case ID | 独有验证目标 | 固定预期 | 除通用条件外的并行条件与待验证项 |
| --- | --- | --- | --- | --- |
| `orca-remote.smoke` | `default` | 远端适配器的工作区创建、原生会话、一次原会话反馈与真实交付；与 L2 的区别在远端契约 | `merged` | 不操作共享 runtime；验证并发 worktree/会话及原 PR 反馈的隔离 |
| `orca-remote.full-access` | `default` | 显式 full_access 下完整远端开发闭环、原会话反馈及实际权限审计 | `merged`，Run/服务及审计终端停止 | 场景固定 full_access，独立工作区/会话/PR；不能借 sandbox 的结果代替 |
| `orca-remote.caller-exit` | `accepted-response-lost` | 接受回执丢失后结束调用方，原服务继续；按原 request_id 找回唯一 Run，同 ID/正文重试不重复输入 | `merged` | 只结束本现场调用方及 submit 进程；核验服务和其它现场仍运行 |
| `orca-remote.connection-reconnect` | `active-disconnect` | 原服务不换代的活动回合断连，恢复后自动重接原 Attempt/会话，无需 resume、零重发 | `merged` | 专属网络故障范围；验证本现场双通路中断且其它现场通信不受影响 |
| `orca-remote.service-restart` | `graceful-active` | 活动回合正常停服换代，原结果可对账，显式 resume 前零新输入 | `merged` | 只停本现场服务并确认旧写者退出；验证与其它现场恢复流程互不干扰 |
| `orca-remote.service-restart` | `kill-active` | 活动回合异常终止服务后持久状态可恢复，旧写者失效，显式恢复原会话 | `merged` | 精确匹配本现场服务 PID/创建时间；验证异常退出及重启不影响其它服务 |
| `orca-remote.service-restart` | `graceful-boundary` | 实施已完成但证据尚未交付时正常停服；迟到完成事实不得越过换代后的授权门槛 | `merged` | 专属网络/完成屏障，加本现场正常服务换代；验证屏障与网络恢复不会释放其它 case |
| `orca-remote.service-restart` | `kill-boundary` | 同一切换点异常终止服务，完成事实与新实例对账后不丢失原结果、不重复阶段输入 | `merged` | 专属网络/完成屏障，加精确服务终止；验证旧/新写者和回执归属隔离 |
| `orca-remote.stop-revocation` | `active` | 活动回合 stop 撤销后继输入授权，并确认原生执行真正停止 | `stopped`，无 PR | 只停止本现场原生资源；验证其它 Run 授权及执行不受影响 |
| `orca-remote.stop-revocation` | `boundary` | 完成事实尚未交付时 stop，随后交付迟到事实也不能触发报告或发布 | `stopped`，无 PR | 专属网络/完成屏障；验证迟到事实及释放动作仅关联本 Run |
| `orca-remote.stop-revocation` | `unreachable` | 暂时不可达时先撤销授权，未知停止不假报 stopped；撤销跨重启保持，反馈/旧 revision 不能绕过，恢复后独立收尾 | `revoked-unverified`，无 PR | 专属网络故障与服务换代；验证整个故障窗、watchdog 和资源清理不会干扰其它 case |
| `orca-remote.publication-unknown` | `completion-gap` | PR 已存在但 Runner 缺完成证据；找回原 final/PR/head，零重复发布 | `merged` | 专属发布屏障与网络范围；实际 PR 列表、发布调用计数仅按本现场分支核验 |
| `orca-remote.publication-unknown` | `query-unavailable` | 原完成证据恢复后 PR 查询仍失败，不能用旧快照假报 ready；查询恢复后核验同一 PR | `merged` | 另隔离本现场查询故障开关及 provider journal；验证不阻断其它现场查询 |
| `local-codex.feedback` | `default` | 明确发送一次反馈，取得更新的 UTC/new commit，复用原 session/Attempt/工作区/分支/PR，再验证合入及完整停止 | `merged` | 本机原生进程及 capability 按 Run 隔离；已与 audited 在上限 2 下共同通过，具体构建见验收 |
| `local-codex.audited` | `default` | 两实际不同模型完成 Spec/代码两项审计并切回执行模型发布；同会话、同实施报告来源、真实 PR 核验 | `pr_ready`，PR 保留未合入，Run/服务停止 | 显式执行/审计模型；逐 turn 权限及实际模型历史核验；已与 feedback 在上限 2 下共同通过，具体构建见验收 |
| `local-codex.multirun` | `default` | 同一真实服务接受两个不同请求，校验请求/命令查询、不同会话/工作区/分支/PR 与全部事件页归属；停止第一个 Run 后第二个接受真实反馈并更新同一 PR，第一 Run 仍停止且 head 不变 | 两 Run 均达 `pr_ready`，PR 保留未合入，最后均停止 | case 内共享服务、独立 Run 资源；批次其它 case 独立服务；受控变体还在活动回合停止一个 Run 后释放另一个的原始完成事实 |
| `environment-check.local-sandbox` | `complete` | 本机 sandbox 的原生权限、认证、Agent Git/PR 工具及 Check 原对象收尾 | `checked`，Check passed/completed；owned PR 关闭、不合入，owned 分支/工作区删除，writer 停止 | 独立 Check/session/workspace/base/head/PR，沿用主线环境检查判据；原真机记录见[环境检查验收](acceptance/environment-check.md)，整合后未重跑真机/并行资格 |
| `environment-check.local-full-access` | `complete` | 显式 full_access 的实际权限与认证/工具/资源闭环，验证模式生效及其独立能力契约 | 同上；实际模式必须为 `full_access` | 独立 Check 资源；不能借 sandbox 结果代替本模式；来源及整合后缺口同上 |
| `environment-check.orca-sandbox` | `complete` | 执行主机认证、同 app-server 的 MXC 预检、实际 Agent 的 Git/PR 全部十二项与原对象收尾 | `checked`，Check passed/completed；owned PR 关闭不合入、owned 引用/目录删除、writer 停止 | 安装可信 `steerrunner-agent`；原 runtime/terminal incarnation/native session 与工具调用绑定；新场地独立验证，不能借本机结果 |
| `environment-check.orca-full-access` | `complete` | 显式 full_access 的远端同权限预检及实际工具闭环；不能回退或借用 MXC 证据 | 同上；实际模式必须为 `full_access` | 与 sandbox 分开冻结和判定；检查会话、引用、PR、目录和认证 capability 独立 |

远端共同前置为 L1、完整 Controlled、指定 Orca CLI、已配对 environment、远端同名
Git 项目和执行主机认证。11 个可靠性 case 另需稳定性 v2 配置和故障取证工具。
`active-disconnect`、两个 publication case、两个 restart boundary case 以及 stop 的
`boundary/unreachable` 需管理员网络宿主，其余无需该宿主。网络故障限各现场专属
Runner/helper/Node；共享管理员宿主的队列回执归属、恢复动作与 watchdog 也需并发验证。
历史 36/36 属于[稳定性验收](acceptance/l3-execution-stability.md)中冻结的受测构建，
其后主线整合和当前认证版本的适用边界按各自验收记录判断；该三轮结果不是并行验收证据。

## L3：新增流程与相关受控覆盖对照

以下文档编号对应实际注册及受控覆盖，用于追踪实现来源；不作为额外 case 重复计数。

| 文档编号 / 内容 | 独有验证目标与目标判据 | 并行条件 | 当前状态与缺口 |
| --- | --- | --- | --- |
| L3-P01 原会话反馈 | 初次 `pr_ready` 后接受反馈，产生新 commit，复用原 session/工作区/分支/PR；再次核验交付并完整停止 | 独立本机现场及 worker；输入、commit/PR 身份和认证 capability 均按本 Run 归属 | 已注册 `local-codex.feedback/default` 并复用严格反馈/合入 driver；正式通过后可为 L2 提供精确构建资格 |
| L3-P02 两模型审计通过 | 实施后依次审计 Spec 和代码；审计模型正确，发布切回执行模型，实际 PR 核验后停止 | 独立本机现场；冻结执行/审计模型；模型切换不影响其它 session，容量满足两种模型 | 已注册 `local-codex.audited/default`，配置读取 `audit_model/audit_reasoning_effort`，保留原定向诊断入口 |
| L3-P03 审计修复与阻断的受控覆盖对账 | 确定性 findings 触发原会话修复及重审，旧 pass 失效；最多两次自动修复，预算耗尽为 `blocked/audit_repair_limit` 且不发布 | 使用隔离服务和原生协议夹具，固定 findings/turn 及调用计数；不要求真实模型稳定产生 finding | 已纳入默认 Controlled：`TestSRWorkflowDiscoveryAndSelection`、`TestSRAuditedWorkflow`；不新增必跑真机 finding case |
| L3-P04 同一服务内多个 Run 隔离 | 在同一服务内接受不同请求，核验 request/command/event/原生会话/工作区/PR 不串线；停止一个 Run 不撤销其它 Run 的授权 | case 内共享服务/state-dir，各 Run 独立工作区/会话/分支/PR；case 之间仍独立服务；冻结并发上限，允许契约内资源排队 | 已注册 `local-codex.multirun/default`，并将 `TestSRMultipleRunsIsolation` 纳入默认 Controlled；受控完成屏障和 PR provider 按原会话/工作区隔离 |

L3-P03 的已有证据入口是[审计路由测试](../internal/execution/workflow/audit_test.go)、
[审计生命周期测试](../internal/application/workflow_test.go)和
[EXE 受控审计测试](../tests/integration/audit_test.go)。后者含
`TestSRWorkflowDiscoveryAndSelection`、`TestSRAuditedWorkflow`，可通过
`test-foundation.ps1 -Pattern '^(TestSRWorkflowDiscoveryAndSelection|TestSRAuditedWorkflow)$'`
定向运行；仍须等待证据目录的 `result.json` 并核对零退出、精确场景数、无 skip。
[原审计 Spec](spec/workflow-selection-audit.spec.md) §8 明确由受控测试覆盖修复/故障分支，
live 只要求两实际模型、同会话和最小真实 PR 闭环，不强迫真实模型制造 finding。
因此 L3-P03 不列为真机 finding 覆盖缺失；默认前置是否通过以当前批次 Controlled 结果为准。

## L4：受控边界与资源压力

四项均已注册，使用 `test.ps1 -Level L4`；无模型或真实 PR。测试 driver 使用真实服务、
Git、SQLite 和受控原生协议，故障只注入专属临时目录、协议历史或 SQLite 触发器。
这些是已经冻结的可执行契约，不能据此宣称已覆盖任意磁盘损坏或整机资源耗尽。

| 文档编号 / 场景 ID / Case ID | 独有验证目标与固定预期 | 并行与互斥 | 当前实现与剩余边界 |
| --- | --- | --- | --- |
| L4-P01 `controlled.permanent-evidence-loss` / `default` | 原回合缺 final 后，删除受控夹具的精确原 thread 文件并跨三次服务换代；显式 resume 也须在未决操作门槛拒绝或在原会话资格核验失败后保持阻断；原 Run/session/input_digest 保持，零新业务输入、无 PR；预期 `resume_required` 且授权关闭 | L4 调度独占；仅删除本现场精确路径的协议历史 | `TestSREdgePermanentEvidenceLoss`；接受 resume 回执不等于恢复成功，不删除真实 Codex 历史，不声称证据丢失后可恢复业务结果 |
| L4-P02 `controlled.stop-commit-race` / `default` | 原 managed worker 已停止，但 stop receipt 的 SQLite 提交被触发器拒绝；换代后仍不得假报 stopped；恢复提交能力再换代，同一 operation 意图结算为 `stopped`，零业务重发 | L4 调度独占；故障触发器及服务均限本现场 | `TestSREdgeStopCommitRace`；覆盖原生动作与持久化结果交错，多代外来回执的排列穷举尚未覆盖 |
| L4-P03 `controlled.storage-failure` / `default` | 拒绝接受事务中的 command INSERT，CLI 返回 `request_outcome_unknown`；按原 request_id 核验 `request_not_found`，零原生输入；恢复后同 ID/正文只接受一次并完成、停止；故障阶段预期 `rejected` | L4 调度独占；仅操作测试 DB 的接受触发器 | `TestSREdgeStorageFailure`；验证真实事务回滚，不等同于物理磁盘满、页损坏或任意存储恢复 |
| L4-P04 `controlled.resource-pressure` / `default` | 同一服务连续 20 次提交/原 ID 重试/交付/停止；每轮原生进程确认退出，thread=20、turn=60；首轮预热后服务 handles 增长不超过 128；预期所有 Run `stopped` | L4 调度独占全部宿主调度槽；不耗尽用户机器 | `TestSREdgeResourcePressure`；这是有界负载/资源增长检查；长期内存增长与极端耗尽需另定时长、阈值、独占专用宿主及故障契约 |

L4 判定包括固定断言、故障命中、原身份、零重复副作用和真实收尾，不允许故障发生后
任选“可恢复/阻断”结果。暂时不可达停止和普通阶段切换依然保留在 L3。
L4 的候选扩展只有证明独有覆盖、冻结可重现触发与唯一预期后才注册，不为补数量复制相似测试。

## 当前清单与资格边界

当前共十九场景、二十五 case：L2 一项、L3 二十项（含原 11 项核心可靠性）、L4 四项。
Controlled 默认前置十六个顶层测试，审计子变体和并行调度的框架检查另在原始日志登记。
L3-P01/P02/P04 与 L4-P01–P04 已绑定可执行项；L3-P03 是已有受控覆盖，不重复计算。

实现与资格分别登记：调度器检查可以证明队列上限、L4 互斥、worker 失联及构建变化拒绝，
不能代替真机后端并发资格。远端认证/同权限检查由执行主机辅助程序提供，未配置或准备
失败时仍阻断。保留历史失败和 unsupported，不使用旧 36/36 冒充当前全 L3 通过。

本机 smoke 的默认完整反馈链保留；同构建独立反馈正式通过并提供资格文件后，可启用
最小核心 driver，本轮该模式已正式通过。新两模型与多 Run 真实 PR 均显式保留未合入，仍要求原资源和服务停止。
跨 case 重复 root/base/Run/session/workspace/branch/PR、缺失身份或构建摘要变化一律不通过。

## 调整建议审计与实施顺序

2026-10-16 审计保留了 L3 核心可靠性和独有断言设计，修正了仅凭复合故障归 L4、遗漏
同服务多 Run 隔离、强迫模型制造 finding 的问题。实现保留指南与清单分离，不改写历史资格。

| 顺序 | 实现状态与验收门槛 |
| --- | --- |
| 1 | 审计发现/修复/预算及多 Run 受控测试已纳入默认前置；远端认证/同权限检查接线已补，真机资格按新场地证据判断 |
| 2 | 已实现 `-MaxParallel`、独立 worker/记录、跨批次槽锁和 L4 独占；框架检查与实际后端并发资格分别登记 |
| 3 | 已注册本机反馈、两模型审计及多 Run；L2 使用同构建反馈资格门，只有正式反馈通过才能省去 L2 反馈步骤 |
| 4 | L4 已落实四个有界受控契约；物理损坏、长期内存压力及整机极端耗尽等未实现条件仍是明确缺口，不计当前覆盖 |
