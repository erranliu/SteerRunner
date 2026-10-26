# SteerRunner L2–L4 测试清单

2026-10-26 固定交付包增量：普通 L1 framework 包含 `tests/framework/delivery-test.ps1` 的文件清单、组件来源和证据绑定反例。`scripts/test-delivery.ps1` 为独立的受控交付入口，复用 installation/client/viewer/`TestSRProgressFile`，固定包内被测文件，不新增 L2–L4 场景或真机资格。调用方测试进度 0/6 的历史缺口继续保留；范围及命令见[测试指南](testing.md#固定交付包验证2026-10-26)。

2026-10-26 原终端核验增量：L1 自动包含 `TestOriginalTerminalRPC*`、`TestQueryBoundTerminal*`、`TestOriginalTerminalConcurrency` 及 `TestOrcaNativeStateStream/terminal-query*`，覆盖生产构造器的 IPC 查询、加密远端协议、身份/两次观察核验、握手/读取取消与共享 CLI 锁隔离。并发比较只衡量受控读取路径，不授予真实 Run 容量或 L3 资格；范围及原始前后对比见[专项记录](acceptance/original-terminal-rpc.md)。

提交前审计补充 `TestTerminalQueryCancellationCannotHideKnownIdentityConflict` 和 `TestOriginalTerminalRPCStopAndRecoveryKeepOriginalIdentity`，覆盖身份冲突与取消同时出现的错误优先级，以及停止前、手动恢复前后 RPC 核验的原 incarnation 约束。

2026-10-23 SessionStart 空预览增量：既有 L1 包入口自动包含 `TestOrcaNativeInputQueryRecoversCompactedWorkingAndFinal`、`TestOrcaNativeInputProofRejectsUnboundHistory`、`TestOrcaNativeInputCachedBindingCannotCrossBoundary`、`TestOrcaNativeInputQueryCannotHideLaterNotification`、`TestOrcaNativeInputPagesRequireExactlyOneOriginalUser`、`TestOrcaNativeInputQueryRejectsIdentityChangeDuringRead`。覆盖原输入完整核验、重复同文拒收、working/重建/final、缓存边界、分页与读取竞争；不新增模型或 PR case。现场验证仅为原 Run 的只读补证，不冒充生产部署或新远端发布资格。

提交前审计补充 `TestOrcaNativeInputCannotCombineDifferentMainRecords`、`TestOrcaNativeInputCachedBindingHandlesFeedAndStaleSamples`、`TestOrcaNativeInputIdentityErrorRemainsDurable`，并扩展原分页和 proof 反例。覆盖两路 main/agent/outcome/marker 不一致、找到正文后仍须验证游标、旧样本不撤销新绑定、更换 feed 立即失效，以及读取前后确定冲突不会因补查失败丢失。均使用隔离协议夹具，随原 L1 入口自动执行。

2026-10-23 薄壳专项：`scripts/test-client.ps1` 覆盖两个 skill 的共享 helper、生产纯校验与受控调用；`scripts/test-client-package.ps1` 验证隔离安装/定位/冲突/摘要；`scripts/test-viewer.ps1` 验证独立 EXE 的原生四列窗口及文件故障。`tests/integration/display_test.go` 的两个 `TestSRDisplaySnapshot*` 使用显式同版 EXE 验证非薄壳提交、心跳、多来源及服务换代；Viewer 自身在独立 module 内测试。以上为 focused 专项，不改变通用 L2–L4 场景计数，也不自动调用真实模型。固定构建、真实本机/远端闭环及未验范围见[薄壳验收](acceptance/skill-shell.md)。

2026-10-23 Orca 断线空预览补正：现有 L1 包入口自动包含 `TestOrcaCompletionQueryRecoversEmptyPromptAfterDisconnect`、`TestOrcaCompletionQueryRetainedStartRejectsForeignOrMissingEvidence`、`TestOrcaCompletionQueryReconnectObserverCommitsFinal` 和扩展的 `TestOriginalAgentQueryRetainedStampRequiresSameMainRecord`。验证原可信 working 起点与独立查询保留标记一致时恢复，正文延迟/adapter 重建可收尾，完成事件及诊断解除完整，发送次数固定为 1；缺证据、异源/历史冲突、截断正文不成为 final。此为确定性受控回归，不新增模型/PR case；当前结果见[执行可见性验收](acceptance/execution-observability.md#断线后空输入预览恢复2026-10-23)。

2026-10-21 用量/费用增量：L1 自动包含 `TestUsage*`、`TestCost*`、`TestRunUsage*`，覆盖三格式、去重/快照事务、授权引用/epoch/未知回收、费用解耦和真实 EXE 查询。`TestUsageExistingNativeEvidence` 需要显式旧 state-dir，默认跳过；已核对本机 Pi 200,432、DSH 318,722 token。远端现场 opt-in `TestUsageRemoteCodexLive`、`TestUsageRemoteCodexCleanupLive`、`TestRunUsageRemotePublicLive` 默认跳过，不成为默认模型/PR case；后者用真实 EXE 和隔离 state-dir 验证原 Codex Run 的自动授权、11 响应/225,453 token、分页/冻结算术价表和撤销。没有远端 helper 或新增模型调用，远端 Pi/DSH 生产绑定仍不支持，见[专属记录](acceptance/run-usage-and-cost.md)。

同日一致性复审增加：首次未观察/预算耗尽状态、缺字段与溢出、持久元数据冲突、新输入重开观察窗口、稳定本机身份/损坏 checkpoint、stop 事务许可隔离、取消时保存已知结果、子 group/repo 级联保护、按 Run/epoch 清理归属、六组合 capability 与费用节点分离、DSH 毫秒时间/seq 下界和缺计价判别字段反例。精确测试名称见 [Plan 的复审映射](spec/run-usage-and-cost.plan.md)。

本机 Codex 增补：`TestRunUsageLocalCodexPublicLive` 默认 skip，显式读取已停止的真实 orca-terminal Run，验证完整启动/业务来源、独立原始记录控制和、公开 EXE 汇总/分页/费用、正常停启与强制重读去重及关闭后的历史查询；新现场为 2 响应/39,667 token。`TestUsageLocalCodexHomeRequiresKnownRoot`、`TestUsagePreparedLocalCodexInitialization`、`TestUsageRecordedCodexIDsUseOriginalTurnAndWireDigest` 的确定性正反例进入 L1。`TestUsageLocalAcceptanceRepoCleanupLive` 也默认 skip，仅凭原登记回执撤销本批未使用 seed 项目，不是生产功能或通用删除工具。

本机 Pi/DSH 增补：`TestRunUsageLocalPiPublicLive`、`TestRunUsageLocalDSHPublicLive` 默认 skip，复用原 pr_ready Run 的只读证据；隔离服务逐响应核对 Run/attempt/generation/command/native session 及 token。独立控制和为 18 响应/200,432、21 响应/318,722；合成估价、坏/关闭价表不影响用量、采集服务停启/强制重读去重、关闭后的历史及 lease=0/许可撤销均有具体断言。原业务服务/终端不动，不增加模型调用或扩大远端 gate。

2026-10-22 提交前一致性复审增加 `TestUsageFinalReplyBelongsToCommand`、`TestUsageConflictEvidenceIsAtomicAndDurable`、`TestCostTokenOnlyScopeAndCompletionAreExplicit`；扩展 epoch 创建意图、四类清单完整性和三查询完整实例绑定/停止围栏断言。整合 main 后重新执行的包、集成和现场结果在专项验收中单列，不继承旧构建资格。

PR #47 合并前复审增加 `TestUsageCrossRunConflictSurvivesCachedObservation`，覆盖后认领 Run 在缓存发布后仍保留冲突、旧快照不变及费用无完整总额；`TestUsagePartialFieldsStillValidateKnownRelations` 补齐 nullable 总量下界与跨输入输出溢出反例。native-usage/4 的最终全包、四项集成及四组合现场资格另记于同一专项验收。

本文件维护各项测试的独有验证目标、固定预期、并行条件、实现状态和当前缺口。
分层定义、设计及清单维护规则、运行入口和证据要求见[测试指南](testing.md)。
已实现项对应仓库注册；规划项使用文档编号，不计入已实现或通过数量。

以下清单于 **2026-10-25** 对照[执行目录](../tests/catalog.json)和
[唯一 case 定义](../internal/testkit/livetest/cases.json)核对。入口支持
`L1,Controlled,L2,L3,L4` 和 `-MaxParallel 1..8`；L4 独占执行，L2/L3 有界并行。
已注册 L2 两 case、L3 二十九场景三十九 case、L4 四 case；原 12 个可靠性 case 全部保留在 L3。
以下区分实现状态与实际受控/真机验收，旧构建的通过结果不作为新构建资格。
本轮实际结果、配置来源和失败历史见[测试功能验收](acceptance/test-levels.md)。
配置引用／角色及模式等要求直接随唯一 case 注册发布；本次只调整配置消费，不增删业务 case。
缺配置项记 unavailable，完整请求不缩小，完整级别通过独立计算；规则及本轮证据见[独立配置验收](acceptance/independent-configuration.md)。
新增项采用本机原生查询、远程订阅加补查；协议差异及实际结果见[专属验收](acceptance/orca-local-remote.md)；下文已有真机通过
记录仍绑定原构建，不继承为新构建资格。
workflow/2 不再保留反馈资格门，旧原生记录不作为本次构建通过。

## R2 定向受控增量

`TestSRRunEvidenceR2` 已登记到 controlled_scenarios；隔离 state-dir、两个原开发 Run、Git 工作区和 native fixture。验证程序报告/原始日志、追加/幂等/跨 Run 隔离、缺失/越界/超限、实际 Git 干净/修改/未跟踪/新 commit，以及零新 Agent 调用、停止并移除工作区后仍读旧报告。workspace/application 定向反例另验证 junction 越界、来源不明及观察中变化。此为 R2 专属最小验证，不增加真机矩阵。

## 服务换代与近期 PR 回归增量（2026-10-20）

Controlled 新增 `TestSRContentGenerationRecovery`，独立 state-dir/原生夹具验证活动生成正常停服、活动对话强杀、原问题跨代回答及启动前原 stop；固定原进程、session/turn、历史及零重发断言。R2 原场景增加删除 producer 文件后实际服务重启、原 collect 回执重放及不可变读取。两项都不创建真实 PR，不依赖其它场景的 Run。

L1 补充报告 64 条/16 MiB 总量、单文件 1 MiB 和发现 256 项的包含式临界值；真实 SQLite 事务失败回滚及重开数据库后的原请求去重；确定性采集中增长、文件替换、读取失败；Check 退役认证/未知 operation 接管拒绝及 v3 原字节/格式保留；只读准备查询取消后授权与锁恢复。

专属入口 `scripts/test-generation-recovery.ps1` 先定向规则、再完整四项恢复/R2受控场景。提供 `-NativeExecutable <现有 Codex 0.159.2 EXE> -Model <可用模型>` 时追加两个 focused native 场景：`local-codex.generation-recovery` 用真实 Codex 验证活动期 graceful/kill/兼容新 EXE/启动 stop，保留原进程/session/turn，关闭后离线 verify-stopped；`local-codex.parser-generation-recovery` 用真实 parser、受控 business/provider 验证 parser 正常/强杀离线完成与显式 stop。后者不是全真实开发交付资格。注册见 `tests/catalog.json.focused_native_scenarios`，不改变通用 manifest driver。两个原生场景串行消费模型；各 subcase 独立现场，不操作用户安装或真实 PR。

当前通过与阻断按 [本轮验收](acceptance/pr-test-gaps-20261020.md) 登记，未运行项不计作通过。

`TestSRResourceCreationRecovery` 使用实际独立 Runner 创建执行器和受控 Orca 公开协议夹具，覆盖创建响应返回前正常停服/强杀、响应已落盘但尚未提交服务数据库、上游响应丢失、原执行器崩溃、原终端身份回执缺失六个窗口。成功恢复必须保留原 worktree、terminal handle/incarnation；后三项保持 unknown。所有窗口固定创建一次、零替换终端、零业务发送，不按名称认领资源，也不凭唯一 inventory 猜测原终端。单元测试另覆盖远端路由、初始终端及 native-control 创建、异源/损坏/丢失回执、写入失败、原创建启动期限和迟到回执后的原意图停止。它不替代真实 Orca 后端资格；本轮审查结果见 [PR #38 复验](acceptance/pr38-review-20261020.md)。

## L2：最小核心 smoke

| 场景 ID / Case ID | 独有验证目标与目标范围 | 当前入口、预期及缺口 | 并行条件 |
| --- | --- | --- | --- |
| `local-codex.result-acquisition` / `default` | 默认认证解析、显式模型/只读权限与取消、development 实际交付三个内部 case | `-Level L2 -Scenario local-codex.result-acquisition`；预期 `merged`，内部 case_count=3；本机 Codex 0.159.2。原生只读权限、行为约定及工具事后拒收分别取证 | 独立短路径现场、parser Job/会话、Canary 与本次测试 PR；不修改 SteerLine |
| `local-codex.smoke` / `default` | 最小开发交付：冻结 Spec/Plan → 接受任务 → 执行 → 发布并核验 `pr_ready` → driver 合入核验 → 停止及清理 | `-Level L2`；预期 `merged`。workflow/2 只执行本列交付路径；不需要反馈资格文件。历史构建结果不自动转为当前构建通过 | 每次独立现场/state-dir/Run/本机 worker/分支/PR/认证 capability；无共享可变构建输出；本轮 L2 串行验收，实际并行资格单独登记 |

前置为 L1、完整 Controlled、本机原生 Codex 资格、SteerlineTest 仓库及执行主机认证。
L2 另有同权限、无模型的 Git 提交前置：loose/packed head 均须无 stderr 错误，
跨仓库写入须被拒绝。业务 smoke 使用生产默认提示词及真实 Git/gh 命令，不装载测试
发布器；审计实际命令输出中的权限拒绝，退出码 0 不能掩盖 `packed-refs.lock` 错误。
默认 broker 收尾还要求可信拒绝计数为 0，缺计数或 HTTP 401/403 均不能通过。
每次独立现场、Run、base/head 和 PR；执行步骤及收尾见[测试指南的完整 smoke 契约](testing.md#当前完整-smoke-契约)。
目标 L2 不包含额外反馈轮、恢复或审计流程；driver 的真实合并可保留为交付核验手段，
不把合并变成 Runner 的生产交付能力，保留 `merged` 预期和原 PR/分支/现场证据。
workflow/2 的 L2 直接使用无反馈交付路径，不再依赖反馈资格文件。独立
`local-codex.feedback` 改为验证发布后反馈被拒绝、原 Turn/PR head 不变；
不以该场景或历史 workflow/1 资格启用返工。
本机当前认证版本的实际结果见[沙箱认证验收](acceptance/sandbox-authentication.md)，
通过结论须绑定对应源码、构建和配置。

## L3：不同流程与核心可靠性

以下为已注册的二十七场景、三十七 case，含原远端流程、12 个核心可靠性 case、三个本机 Codex 流程及新增本机 Orca 流程。全部保留在
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
更大模型/平台容量及跨批次真实收尾的并发资格按构建单列。新远端使用原生 Orca，参数生效、单仓库 capability、完整 Check 与实际双模型来源证明不属于其保证；不沿用旧 helper 通过记录。参见[原生边界](spec/orca-remote-native.spec.md)。

宿主槽锁由协调器持有，只覆盖存活的正常调度批次。协调器异常退出后可能遗留后台
worker/服务，须先对账原现场并收尾，再启动独占测试；当前没有孤儿资源自动接管资格。

| 场景 ID | Case ID | 独有验证目标 | 固定预期 | 除通用条件外的并行条件与待验证项 |
| --- | --- | --- | --- | --- |
| `orca-remote.initialization-stop` | `before-readiness`、`restart-before-readiness` | 原生已启动但 caller 未确认时停止；初始化换代零自动输入与替换会话 | `stopped`、完整原 Orca 清单和 `orca_terminal` scope；零 helper/文件部署、零业务输入 | 新独立工作区/服务/状态目录，可并行；来源及结果见[验收](acceptance/initialization-stop.md) |
| `orca-local.smoke` | `default` | 无 pair 的真实本机 IPC 查询、原请求开始回执/新主代理 done、原生 final 及真实 PR 交付/停止 | `merged` | 独立现场/工作区/会话；当前查询式实现的真机结果单列，不要求修改 Orca |
| `environment-check.orca-local-sandbox` | `complete` | 本机 Orca helper、MXC、认证与既有十二项工具检查 | `checked`，owned PR 关闭不合入、引用/工作区删除、writer 停止 | 独立 Check 资源；已接线，真机结果单列于专属验收 |
| `environment-check.orca-local-full-access` | `complete` | 本机 Orca 的显式 full_access 准备及工具/资源闭环 | 同上，实际 full_access | 不借用 sandbox 通过；已接线，真机结果单列 |
| `orca-local.codex` | `default` | 本机 Orca terminal 的开发、业务问答、发布后反馈拒绝、原终端/agent 身份、真实 PR 与清理 | `merged`，权限审计及 Run/服务停止 | 复用 development-feedback；含业务 reply、丢 submit 响应对账、兼容换代保留授权和活动 stop；实际结果见[通用接入验收](acceptance/orca-generic-agent.md) |
| `orca-local.pi` | `default` | Pi 的同一 terminal 开发、业务问答及发布后反馈拒绝，不能继承 Codex 原生证据 | 同上 | A01 扩展、双权限工具及原生权限审计已接线；包含业务 reply、换代和活动写者停止；实际结果见[通用接入验收](acceptance/orca-generic-agent.md) |
| `orca-local.dsh` | `default` | DSH 的同一 terminal 开发、业务问答及发布后反馈拒绝，排除 SessionStart/子事件假完成 | 同上 | A02 原 TUI 固定 profile、原生持久事实、业务 reply 及 H01 已接线；不改 Orca，结果见[通用接入验收](acceptance/orca-generic-agent.md) |
| `orca-local.pi-full-access` | `default` | Pi 显式 full_access 的完整交付与同会话恢复、活动写者停止 | `merged`，Run/服务停止 | 独立双权限实测，不借 sandbox 结果 |
| `orca-local.mixed` | `default` | 同一服务的 Pi/DSH：独立输入/原生会话/回执/PR，停止 Pi 后 DSH 的原生 owner 保持可用 | 两个原 PR、隔离验证及 Run/服务停止 | 冻结 peer_execution；只验证一个混合组合 |
| `orca-remote.multirun` | `default` | 同服务接受两个请求，独立原终端/工作区/分支/事件/PR；停止一方后另一方原生 owner 保持可用 | 两个原 PR，Run/服务停止，离线核验两项 orca_terminal scope | `orca-remote-codex-full-access`；不部署 helper，不声明远端 broker 或完整 OS 树证明 |
| `orca-remote.smoke` | `default` | 远端适配器的工作区创建、原生会话与真实交付；与 L2 的区别在远端契约 | `merged` | 不操作共享 runtime；独立 worktree/会话/PR，按当前 workflow/2 直接交付 |
| `orca-remote.full-access` | `default` | 显式 full_access 下完整远端开发闭环、发布后反馈拒绝及冻结参数传递核对 | `merged`，Run 原终端及服务停止 | 场景固定 full_access，独立工作区/会话/PR；不能借 sandbox 的结果代替 |
| `orca-remote.caller-exit` | `accepted-response-lost` | 接受回执丢失后结束调用方，原服务继续；按原 request_id 找回唯一 Run，同 ID/正文重试不重复输入 | `merged` | 只结束本现场调用方及 submit 进程；核验服务和其它现场仍运行 |
| `orca-remote.connection-reconnect` | `active-disconnect` | 原服务不换代的活动回合断连，恢复后自动重接原 Attempt/会话，无需 resume、零重发 | `merged` | 专属网络故障范围；验证本现场双通路中断且其它现场通信不受影响 |
| `orca-remote.connection-reconnect` | `pre-send-disconnect` | 实施/报告结束，发布 `start_turn` 已提交但尚未发出；仅阻断 Runner 直连状态通道，原 CLI 身份读取仍成功；自动重试后保留原 operation/Attempt/session，无需 resume 且发布一次 | `merged` | 本机测试 CLI 转发器在原发布终端的首次发送前身份读取处设门；仅阻断现场 Runner EXE，保留 CLI/诊断通路。证据须含门前快照、阻断后返回的原读取、完整建连超时后的再次读取、阻断期零新输入/授权保留及单次 PR。已实现并注册，真机及并行资格尚未执行 |
| `orca-remote.service-restart` | `graceful-active` | sandbox 活动回合正常停服换代，原授权保持，原结果自动对账、零重发，无需 resume | `merged` | 只停本现场服务并确认旧写者退出；验证与其它现场恢复流程互不干扰 |
| `orca-remote.service-restart` | `kill-active` | sandbox 活动回合异常终止服务后，原授权和会话保持，旧写者失效，自动继续 | `merged` | 精确匹配本现场服务 PID/创建时间；同一 handle/incarnation 补读原回执，零重发业务；验证重启不影响其它服务 |
| `orca-remote.service-restart` | `graceful-active-full-access` / `kill-active-full-access` | full_access 分别验证正常/强杀换代后的原授权、Attempt/generation、零原输入重发和自动完成交付 | `merged` | 独立固定完整权限配置；不能代替 sandbox case 的资格 |
| `orca-remote.service-restart` | `graceful-boundary` | 实施已完成但证据尚未交付时正常停服；迟到完成事实按原授权继续对账和后继阶段 | `merged` | 专属网络/完成屏障，加本现场正常服务换代；验证屏障与网络恢复不会释放其它 case |
| `orca-remote.service-restart` | `kill-boundary` | 同一切换点异常终止服务，完成事实与新实例对账后不丢失原结果、不重复阶段输入 | `merged` | 专属网络/完成屏障，加精确服务终止；验证旧/新写者和回执归属隔离 |
| `orca-remote.stop-revocation` | `active` | 活动回合 stop 撤销后继输入授权，并确认原生执行真正停止 | `stopped`，无 PR | 只停止本现场原生资源；核验原认证 broker/socket、业务写者及固定控制终端关闭，撤销证明与关闭证明分开；验证其它 Run 授权及执行不受影响 |
| `orca-remote.stop-revocation` | `boundary` | 完成事实尚未交付时 stop，随后交付迟到事实也不能触发报告或发布 | `stopped`，无 PR | 专属网络/完成屏障；验证迟到事实及释放动作仅关联本 Run |
| `orca-remote.stop-revocation` | `unreachable` | 暂时不可达时先撤销授权，未知停止不假报 stopped；撤销跨重启保持，反馈/旧 revision 不能绕过，恢复后独立收尾 | `revoked-unverified`，无 PR | 专属网络故障与服务换代；验证整个故障窗、watchdog 和资源清理不会干扰其它 case |
| `orca-remote.publication-unknown` | `completion-gap` | PR 已存在但 Runner 缺完成证据；找回原 final/PR/head，零重复发布 | `merged` | 专属发布屏障与网络范围；实际 PR 列表、发布调用计数仅按本现场分支核验 |
| `orca-remote.publication-unknown` | `query-unavailable` | 原完成证据恢复后 PR 查询仍失败，不能用旧快照假报 ready；查询恢复后核验同一 PR | `merged` | 另隔离本现场查询故障开关及 provider journal；验证不阻断其它现场查询 |
| `local-codex.feedback` | `default` | 明确提交一次发布后反馈并验证 `invalid_state` 拒绝；原 revision、Turn、PR head 不变，再核验合入及完整停止 | `merged` | 本机原生进程及 capability 按 Run 隔离；已与 audited 在上限 2 下共同通过，具体构建见验收 |
| `local-codex.audited` | `default` | 两实际不同模型完成实施与同轮代码审计修复并切回执行模型发布；仅代码审计，记录修复数量/内容和 pre_head/post_head 来源，真实 PR 核验 | `pr_ready`，PR 保留未合入，Run/服务停止 | 显式执行/审计模型；逐 turn 权限及实际模型历史核验；已与 feedback 在上限 2 下共同通过，具体构建见验收 |
| `local-codex.multirun` | `default` | 同一真实服务接受两个不同请求，校验请求/命令查询、不同会话/工作区/分支/PR 与全部事件页归属；停止第一个 Run 后第二个原生 owner 保持可用，第一 Run 仍停止且 head 不变 | 两 Run 均达 `pr_ready`，PR 保留未合入，最后均停止 | case 内共享服务、独立 Run 资源；批次其它 case 独立服务；受控变体还在活动回合停止一个 Run 后释放另一个的原始完成事实 |
| `environment-check.local-sandbox` | `complete` | 本机 sandbox 的原生权限、认证、Agent Git/PR 工具及 Check 原对象收尾 | `checked`，Check passed/completed；owned PR 关闭、不合入，owned 分支/工作区删除，writer 停止 | 独立 Check/session/workspace/base/head/PR；本轮串行真机通过，PR #556 关闭并收尾，见[独立配置验收](acceptance/independent-configuration.md)；并行资格另验 |
| `environment-check.local-full-access` | `complete` | 显式 full_access 的实际权限与认证/工具/资源闭环，验证模式生效及其独立能力契约 | 同上；实际模式必须为 `full_access` | 独立 Check 资源；不能借 sandbox 结果代替本模式；本轮缺该模式配置，记 unavailable，历史结果见[环境检查验收](acceptance/environment-check.md) |

远端共同前置为 L1、完整 Controlled、指定 Orca CLI、已配对 environment、远端同名
Git 项目和执行主机认证。12 个可靠性 case 另需新测试配置中对应角色、harness.admin_host 和故障取证工具。
`active-disconnect`、`pre-send-disconnect`、两个 publication case、两个 restart boundary case 以及 stop 的
`boundary/unreachable` 需管理员网络宿主，其余无需该宿主。网络故障限各现场专属
Runner/helper/Node；共享管理员宿主的队列回执归属、恢复动作与 watchdog 也需并发验证。
历史 36/36 属于[稳定性验收](acceptance/l3-execution-stability.md)中冻结的受测构建，
其后主线整合和当前认证版本的适用边界按各自验收记录判断；该三轮结果不是并行验收证据。

## L3：新增流程与相关受控覆盖对照

以下文档编号对应实际注册及受控覆盖，用于追踪实现来源；不作为额外 case 重复计数。

| 文档编号 / 内容 | 独有验证目标与目标判据 | 并行条件 | 当前状态与缺口 |
| --- | --- | --- | --- |
| L3-P01 发布后反馈边界 | 初次 `pr_ready` 后提交反馈，必须以 `invalid_state` 拒绝；原 revision、Turn、PR head 与 native 输入数不变，再核验合入及完整停止 | 独立本机现场及 worker；输入、commit/PR 身份和认证 capability 均按本 Run 归属 | 已注册 `local-codex.feedback/default`；不产生反馈资格文件，也不启用返工 |
| L3-P02 两模型审计通过 | 实施后执行一次代码审计及 P0–P2 修复；核实审计模型及原工作树来源，发布切回执行模型，实际 PR 核验后停止 | 独立本机现场；冻结执行/审计模型；模型切换不影响其它 session，容量满足两种模型 | 已注册 `local-codex.audited/default`，读取 `local-codex-audited` 的 `workflow.audit_model`；原生远端因缺实际模型/工作树证明而明确 unsupported，保留原定向诊断入口 |
| L3-P03 审计修复与阻断的受控覆盖对账 | 同一次正式审计内修复并验证，核对 fixed_count/fixed_issues/remaining_issues 与 pre_head/post_head；遗留 P0–P2、验证未完成或来源失效时不发布；无自动返修/重审 | 使用隔离服务和原生协议夹具，固定审计结果、turn 及调用计数；不要求真实模型稳定产生 finding | 已纳入默认 Controlled：`TestSRWorkflowDiscoveryAndSelection`、`TestSRResultAcquisition`；含零/正数修复、一次补报及 parser 分支，不新增必跑真机 finding case |
| L3-P04 同一服务内多个 Run 隔离 | 在同一服务内接受不同请求，核验 request/command/event/原生会话/工作区/PR 不串线；停止一个 Run 不撤销其它 Run 的授权 | case 内共享服务/state-dir，各 Run 独立工作区/会话/分支/PR；case 之间仍独立服务；冻结并发上限，允许契约内资源排队 | 已注册 `local-codex.multirun/default` 与原生 `orca-remote.multirun/default`；远端不承诺 capability 撤销，并将 `TestSRMultipleRunsIsolation` 纳入默认 Controlled；受控完成屏障和 PR provider 按原会话/工作区隔离 |

L3-P03 的已有证据入口是[审计路由测试](../internal/execution/workflow/audit_test.go)、
[审计生命周期测试](../internal/application/workflow_test.go)和
[EXE 受控审计测试](../tests/integration/audit_test.go)。后者含
`TestSRWorkflowDiscoveryAndSelection`、`TestSRResultAcquisition`，可通过
`test-foundation.ps1 -Pattern '^(TestSRWorkflowDiscoveryAndSelection|TestSRResultAcquisition)$'`
定向运行；仍须等待证据目录的 `result.json` 并核对零退出、精确场景数、无 skip。
[当前审计定义](workflows/development-audited.md)要求一次审计内修复与来源核验，
受控测试覆盖修复和故障分支；live 核对两实际模型、同会话和真实 PR，不强迫真实模型制造 finding。
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

当前共三十三场景、四十三 case：L2 两项、L3 三十七项（含原 12 项核心可靠性）、L4 四项。
Controlled 默认前置二十三个顶层测试，审计子变体和并行调度的框架检查另在原始日志登记。
L3-P01/P02/P04 与 L4-P01–P04 已绑定可执行项；L3-P03 是已有受控覆盖，不重复计算。

实现与资格分别登记：调度器检查可以证明队列上限、L4 互斥、worker 失联及构建变化拒绝，
不能代替真机后端并发资格。新远端使用执行主机既有登录，完整远端 Check 为 unsupported；不再以 helper 准备作为普通 Run 前置。历史构建未配置或准备
失败时仍阻断。保留历史失败和 unsupported，不使用旧 36/36 冒充当前全 L3 通过。

本机 smoke 直接执行 workflow/2 的最小交付链，不读取反馈资格文件。独立 feedback case 验证发布后输入拒绝。
两模型与多 Run 真实 PR 均显式保留未合入，仍要求原资源和服务停止。
跨 case 重复 root/base/Run/session/workspace/branch/PR、缺失身份或构建摘要变化一律不通过。

## 调整建议审计与实施顺序

2026-10-16 审计保留了 L3 核心可靠性和独有断言设计，修正了仅凭复合故障归 L4、遗漏
同服务多 Run 隔离、强迫模型制造 finding 的问题。实现保留指南与清单分离，不改写历史资格。

| 顺序 | 实现状态与验收门槛 |
| --- | --- |
| 1 | 审计发现/修复/预算及多 Run 受控测试已纳入默认前置；远端认证/同权限检查接线已补，真机资格按新场地证据判断 |
| 2 | 已实现 `-MaxParallel`、独立 worker/记录、跨批次槽锁和 L4 独占；框架检查与实际后端并发资格分别登记 |
| 3 | 已注册本机反馈边界、两模型审计及多 Run；L2 直接执行最小交付链，L3 feedback 验证发布后反馈拒绝 |
| 4 | L4 已落实四个有界受控契约；物理损坏、长期内存压力及整机极端耗尽等未实现条件仍是明确缺口，不计当前覆盖 |

2026-10-19：远端完整 Check 已从当前注册清单退役，Controlled 验证 unsupported；远端普通 Run 只走 Orca 原生链路。停止证据限原终端、参数只承诺传递，历史 helper 资格不代表本次构建，详见 [原生验收](acceptance/orca-remote-native.md)。

## 内容流程专项（2026-10-19）

`tests/catalog.json.controlled_scenarios` 新增 `TestSRContentGenerate`、
`TestSRContentConversation`、`TestSRContentQuestionAndUnknown`，分别覆盖单轮结果八个集成分支、
原会话多轮/中断/重启/关闭，以及原生问答和未知发送。无真实模型/Git/PR，独立 state-dir。

2026-10-22：坏 JSON 与 Schema 不匹配保留在 L1 `TestContentGenerateResults`，移除这两个重复的完整集成现场；其余原生结果、准备失败和资源结算断言保留。

2026-10-20：Generate 的已完成 text/JSON 与 Conversation 空闲态追加同 EXE 的公开
verify-takeover，断言原 inspect 不变、零额外 turn/start、原 submit 绑定完整。
L1 `TestContentFrozenTakeoverContracts` 从正式 freeze 产生零值配置记录，覆盖两种内容流程、
可变配置不参与接管、记录不变及未知 codec/workflow/version/材料/parser 反例；
`TestServiceGenerationTakeoverRejectsUnknownCodecAndKeepsOriginalFacts` 保留 development 配置严格检查。
L3 专项 `TestSRContentGenerateTakeoverNative` 由现有 foundation live 入口显式选择，
使用新 C:/srt/gt-*、真实 Codex、一个生成回合、原 content_result、显式 stop、verify-stopped
和离线同 EXE verify-takeover，核对原 Run/input/request 与数据库字节不变。与多轮用例独立，
不需要开发专用配置。结果见[本次验收](acceptance/content-takeover.md)。

真实 L3 专属入口为 `scripts/test-content-workflows.ps1 -NativeExecutable <原生0.159.2 EXE>`，
注册在 `focused_native_scenarios`，不混入 development 的 Git/PR manifest driver。入口先运行
定向规则与三个受控场景，再以 `-tags live` 的 `TestSRContentNative` 在新 `C:/srt/cw-*` 现场
通过公开 CLI 验证 JSON 生成、五个串行原轮（含中断）、空闲重启后原 thread 继续及结算。
模型可通过 `-Model` 显式选择；省略 `-NativeExecutable` 仅运行受控测试，不声称真实通过。
原生 opt-in 仅编译 live 时存在；默认不执行真实会话。证据及本构建结论见
[内容流程专属验收](acceptance/content-workflows.md)。


## 运行时单文件汇报专项（2026-10-23）

沿用户收敛边界：固定原执行主机/worktree/文件，不以防恶意链接替换作为前置。五项均已在 tests/catalog.json 与唯一 cases.json 注册，case=default、L3、driver=progress-file、预期业务 stopped（不创建 PR）。本专项 2026-10-23 登记时总计 33 场景/43 case，其中 L3 27 场景/37 case；当前全集见 2026-10-25 等待重试增量。

| 场景 ID | 独有断言 | 前置与隔离 |
| --- | --- | --- |
| local-codex.progress-file | 公开 EXE 提交；真实工具读取注入环境；violet/copper 任意非 JSON 字节完整回传、原子替换、重复内容不增版本 | local-codex 配置；全新 state/workspace/native owner；执行脚本由原 native 工具运行 |
| orca-local.progress-file | Orca 本地真实 IPC 文件接线、原终端启动环境、同原生准备及退出证明 | orca-local-codex 配置及本次构建 helper；独立 worktree/terminal，不改共享 runtime |
| orca-remote.progress-file | 原配对远端真实写入/读取；本机不存在源文件；原子替换后继续采集，native final 仍未核实 | orca-remote-codex、已有远端登录及固定仓库；独立远端 worktree/terminal，无新增远端 helper |
| local-codex.progress-multirun | 同一服务两 Run 交错更新，路径/内容/版本/身份隔离；停第一项后第二项原 session/turn 保持并继续写，业务 revision 不因汇报改变 | case 内共享 state/service，各 Run 独立工作区/owner；与其它 case 独立服务 |
| local-codex.progress-recovery | 原生工具仍等待期间读取故障保留最后快照；有限不可读恢复；停服期间原工具继续写，新服务保留文件/版本、原 session/turn/operation 数且零额外输入；终态仍可查询 | 只对本 case 文件短暂 rename/restore，原脚本 gate 有绝对期限；只换代本 case 服务 |

所有 case 独立 root/base/state/Run/native 资源，允许并行；marker 仅在本 case 原工作区 .sr-runtime-stage 下。每次 CLI、采集、gate、轮询、收尾均有界。保存公开 submit/快照/故障/恢复/收尾响应及原持久执行记录；explicit stop + offline verify-stopped 证明实际资源结算，保留 owned 工作区/分支/原记录便于审计。实际并行资格与当前通过结果见[验收](acceptance/runtime-progress.md)，注册不代表通过。

Controlled 新增 TestSRProgressFile，默认顶层数从 22 增至 23，覆盖真实 EXE 的结果等待、读取失败、多 Run、停止、服务换代、公开终态查询及零业务重发。L1 覆盖 64KiB、非 JSON/非 UTF-8、缺失、错误绑定、路径/schema、旧业务 clone 不覆盖新快照及精确远端 wire 长度/EOF。受控通过不能代替三条真实后端。

准确入口（自动 L1 + 完整 Controlled 前置）：

```powershell
./scripts/test.ps1 -Level L3 -Scenario local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config .evidence/progress-test-config.json -MaxParallel 5 -OutDir .evidence/progress-l3-new
./scripts/verify-progress-parallel.ps1 -Report .evidence/progress-l3-new
```

配置复用已有三项配置键；不增加调用方项目测试入口、不解析 Plan、不解释测试字段/历史。真实链路由测试自有 foreground producer 驱动并在其结束前停止原 Run。

每项首次真实写入后持续 120 秒，通过公开接口确认快照稳定、原 native 会话仍等待且无 final；窗口独立，不依赖其它场景的就绪或结果。并行核验脚本要求五项全部通过且 Runner/driver 构建一致、原生资源不冲突，并要求每一项窗口与至少另一通过项实际重叠。只设置 MaxParallel 或借用旧构建窗口不能通过。

稳定快照不要求传输永不出错。观察窗口只允许 `transport_unavailable` / `read_failed` 暂态，逐次保存原始诊断并核对身份、版本、内容、时间、长度及摘要完全保留；同一次异常须在 20 秒内恢复，窗口末尾必须恢复 `observing`，总窗口最多 140 秒。绑定异常、终态、内容变化、持续不可达或超时恢复仍失败。窗口记录包含异常采样数、恢复预算及末尾健康证明，v3 并行验证同时核对这些字段；原 r13 瞬时异常失败和旧版本并行报告原值保留。

有界预算：本机 Codex 首份内容最多 5 分钟，Orca 最多 8 分钟（含冷工作区、控制/执行 helper、原 TUI 准备及共享 CLI 排队）；产品 Orca 阶段准备期限为持久 6 分钟，不因重试/服务重启续期，其它 adapter 为 3 分钟。后续内容不超过相应等待；原 Run 停止最多 Orca 180 秒／Codex 120 秒（含下一次原身份只读对账）、服务退出最多 60 秒，受独立 case 的既有 15 分钟总预算约束。产品文件采集的 2 秒预算不变，不把测试放宽当作实际通过；原 3 分钟/45 秒及整阶段准备超时的失败记录保留。

## Orca CLI 等待与读取重试（2026-10-25）

| ID / Case | 层级 / 入口 | 独有目标与固定预期 | 隔离 / 串行要求 | 实现与资格 |
| --- | --- | --- | --- | --- |
| TestSROrcaCLIWaitRetry | Controlled / test-foundation.ps1 精确 Pattern | 真服务/CLI/SQLite；>15 秒排队仍 queue/0 calls，释放后原 operation 自动继续；正常/强杀重启保留 deadline/身份；stop 后零查询/创建；暂态 backoff 自动继续 | 独立 fixture/cache key，无真实 Orca 文件、模型或 PR；六个内部变体 | 已实现；当前证据见专项验收 |
| orca-local.cli-wait-retry/default | L3 / cli-wait-retry driver | 原生本机 status 真实竞争后原 Run 创建一次；另一原排队 Run stop 后零新查询/资源/输入；转发 journal 无重复业务输入，精确停止与离线结算 | 新 C:/srt fixture；共享用户 CLI cache，独占测试 lease；故障 key 私有，不注入生产锁 | 已注册/实现；本机资格单列 |
| orca-remote.cli-wait-retry/default | L3 / 同一 driver | 原生远端 repo list 在固定 environment/runtime 中恢复，保留原 operation/Attempt；同样验证取消、唯一创建/输入及资源结算 | 同上；原环境目标单独冻结，不以本机结果替代 | 已注册/实现；远端资格单列 |

L1 同时核验 FIFO 取消/跨进程互斥、空输出队列错误、两项读取分类和有界策略、UTC 预算/次数/next time 恢复、时间倒退、持久失败、真实并发变化与 stop 优先、公开 schema/事件对象和 null。创建/发送/停止没有新增重试；完整前置和原生通过不能互相替代。[Spec](spec/orca-cli-wait-retry.spec.md) / [Plan](spec/orca-cli-wait-retry.plan.md) / [验收](acceptance/orca-cli-wait-retry.md)。当前全集 35 场景/45 case，其中 L3 29 场景/39 case；Controlled 24 个顶层。


## result-file 本地与受控增量（2026-10-26）

新增 Controlled `TestSRResultFile`，当前 Controlled 注册 25 项。本轮只选取 `TestSRProgressFile|TestSRResultFile`：保留原 progress 回归；result 验证四种启用组合、实际环境清理、运行中 pending、结束后任意字节/原子替换、停止后封存及服务重启后不刷新。result 仅结束时采集，版本只有 0/1，不包含持续观察、交错采样或观察重开测试。

定向单元覆盖封存失败/预算/原绑定、初始化未知只读对账、resume 不刷新、旧业务/progress/result 保存保护及持久化失败。完整 content/resource recovery 长流程不作为本功能默认回归；受控和真实远端资格分别登记，真实远端和部署留待后续。详见 [验收记录](acceptance/runtime-result-file.md)。

## 固定版本代码调查与多文档生成（2026-10-27）

新增 Controlled `TestSRContentRepository`（登记后共 26 项）和 focused native `TestSRContentRepositoryNative`。
受控场景覆盖固定 commit/脏工作树隔离、完整双文档、同请求重放、完成/运行中/启动结果未知时重启、停止后重启不重发。
原 generate/conversation/question/unknown 回归分别保持独立场景。
定向单元覆盖输入及公开 Schema、旧 commit 与后续提交隔离、文件/总量限制、链接/危险路径、Git 环境覆盖、快照漂移、正文/JSON/Schema/缺 final/1 MiB 边界。
真实 Codex 仅核验只读工具读固定文件、双文档、完成结果重启与重放、停止及结算；未把受控运行中故障注入算作真实资格。
补充 application 整包复验：30 分钟预算下完整通过 141 个顶层测试、含子用例 333 项，0 失败/跳过，包内耗时 451.112 秒；未宣称完整仓库全集通过，详见上述功能的验收记录。
[Spec](spec/generate-repository.spec.md) / [Plan](spec/generate-repository.plan.md) / [接入](workflows/generate.md#固定仓库版本调查) / [验证](acceptance/generate-repository.md)。
