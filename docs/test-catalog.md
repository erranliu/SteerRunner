# SteerRunner L2–L4 测试清单

核对日期：2026-11-06；基线：`dda9a827`。本文件维护注册范围、独有目标、固定预期、隔离及资格缺口；运行命令、选测与证据判定统一见[测试指南](testing.md)。[精简前记录](archive/2026-11/maintenance/test-catalog.before-consolidation.md)保存旧数量、调整建议、历史模型/耗时和原构建结论。

## 当前清单与资格边界

数量对照[执行目录](../tests/catalog.json)和[唯一 case 注册](../internal/testkit/livetest/cases.json)核对；注册不是当前构建通过。Controlled 子场景、专项内部变体、repeat 和框架检查不混入通用 case 计数。

| 范围 | 场景/测试数 | Case 数或范围 |
| --- | --- | --- |
| L1 | 按实际批次报告 | framework、`./internal/...`、当前 Pi Node 扩展、native driver 编译/零匹配保护 |
| Controlled | 29 个顶层测试 | 六个主分组；完整资格还要求规定的子场景实际覆盖 |
| L2 | 1 场景 | 1 case |
| L3 | 25 场景 | 33 case |
| L4 | 4 场景 | 4 case |
| 通用 L2–L4 合计 | 30 场景 | 38 case |
| 独立原生 L3 专项 | 3 项 | 不由 `test.ps1 -Level L3` 自动执行 |

### 当前接线与资格缺口

- 已注册 Orca feedback、smoke/multirun 与 native-development/native-active-stop 使用结构化 session、完整 history 和 `orca_agent_session` close。本机测试 IPC 转发记录 create/send/close 及丢回执；远端按原 host key、历史和操作核对输入，不冒充 wire 重复次数证明。close 后使用已取得历史、原输入、撤销和原回执结算，不要求宿主继续提供历史。实际范围见[结构化资格清理验收](archive/2026-11/implementation/acceptance/orca-structured-qualification-proof-cleanup.md)。
- 本机 Orca audited 虽有产品支持，尚无注册真机场景；execution-stability 的旧终端故障夹具未随该批迁完。其 14 个 case 保留注册和独有目标，不能继承结构化资格或旧构建 36/36。下表为注册预期，旧 scope、故障接线和当前生产停止合同必须分别核对。
- 旧 `orca-remote.native-input` 已注销，`test-native-input.ps1` 在构建/现场操作前拒绝；TUI/helper 写入探针退役，原证据只属原构建。[退役边界](archive/2026-11/implementation/acceptance/orca-reader-execution-boundary.md)保留共享创建、只读历史/usage/files/停止证明及原终端并发读取。
- DSH 专用执行/历史/用量及 `orca-local.dsh`、`orca-local.mixed` 已退役；旧 sandbox、broker/CA/socket 和跨构建活动接管不作为新配置或当前验收义务。
- 完整级别、定向、受控、真机、并行、产品支持和实际部署分别报告。案例实现、静态 availability、原生默认 skip 或历史通过均不授予当前完整资格。

## Controlled 覆盖与定向粒度

完整执行集合只由 catalog 的 `controlled_scenarios` 决定；`controlled_selection` 维护六组和固定组合。各组精确成员如下，运行及子场景取证见[选测说明](testing.md#controlled-分组固定选测与耗时2026-11-02)。

| 主分组 | 顶层数 | 精确注册测试 |
| --- | --- | --- |
| foundation | 7 | `TestSRStandalonePackage`、`TestSRServiceOwnership`、`TestSRInputFreeze`、`TestSRPermissionQualification`、`TestSRStorageLogsEvents`、`TestSRNativeCatalogPublicCLI`、`TestSRExecutionConfigurationPublicCLI` |
| workflow | 5 | `TestSRRunWorkflow`、`TestSRPublicationIdentity`、`TestSRPublicationBeforeWorkflowReady`、`TestSRWorkflowDiscoveryAndSelection`、`TestSRResultAcquisition` |
| recovery | 7 | `TestSRNativeUnknownAndStop`、`TestSRRestartPreservesAuthorization`、`TestSRStabilityLostSubmitResponse`、`TestSRStabilityRevocationAcrossRestart`、`TestSRMultipleRunsIsolation`、`TestSRResourceCreationRecovery`、`TestSRManualReconciliation` |
| content | 5 | `TestSRContentGenerate`、`TestSRContentRepository`、`TestSRContentConversation`、`TestSRContentQuestionAndUnknown`、`TestSRContentGenerationRecovery` |
| files | 4 | `TestSRRunEvidenceR2`、`TestSRProgressFile`、`TestSRResultFile`、`TestSRSingleExecutionColdStop` |
| orca-cli | 1 | `TestSROrcaCLIWaitRetry` |

ResultFile 默认两类/各 enabled-0..3，ResultAcquisition 默认八项，RestartPreservesAuthorization 默认四项；完整 Controlled 不能只看父测试总数。选测即使覆盖全部注册也保持完整资格 false，不生成前置复用证明。断言、原生预算和组合并集详见[ResultFile](testing.md#resultfile-职责与定向粒度2026-11-02)、[ResultAcquisition](testing.md#resultacquisition-断言对照2026-11-02)、[Restart](testing.md#restart-职责与断言对照2026-11-02)。

R2 覆盖原报告/日志、实际 Git 状态、限额/越界/幂等/跨 Run 隔离、重启后原回执及 producer 删除后不可变读取，零新 Agent 调用。[R2 Spec](archive/2026-11/implementation/spec/run-evidence-r2.spec.md)及[验收](archive/2026-11/implementation/acceptance/run-evidence-r2.md)保存详细来源。

ResourceCreationRecovery 按 structured 契约保留七个故障窗口：正常/强杀停服、创建事实未提交、响应丢失、owner 崩溃、缺 terminal 字段、缺创建计划。只按原回执和完整 inventory 结算原 shell，未知分支零发现/收养/关闭，创建一次、零替换或业务输入；[目录边界验收](archive/2026-11/implementation/acceptance/native-catalog-boundary.md)登记当前夹具及复验。

SingleExecutionColdStop 覆盖活动 execute 离线原 request 查询、PendingStops 启动前受理、原输入一次、closed/stopped、原 stop 完成及 result-file 封存；新增同 worktree/分支四 Run 累积成果与新 Run 提交遗留修改、停止后遗留修改、并发接管唯一胜者、重复请求、活动来源拒绝、旧 stop 隔离与原 Run/结果不变断言。它属于默认 Controlled，不额外增加 L3/L4 或模型/PR case。

## L2：最小核心 smoke

| 场景 ID | Case ID → 固定预期 | 独有验证目标 |
| --- | --- | --- |
| `local-codex.smoke` | `default` → `merged` | 最小开发交付、发布后反馈拒绝、driver 合入核验及停止；生产默认提示词，无独立 parser |

前置为完整 L1/Controlled、本机原生 Codex、专属测试仓库及执行主机登录。业务使用生产默认提示词与真实 Git/gh，loose/packed refs 的无模型前置核验无权限错误且 peer canary 不变。完整流程、PR 合入及停止要求统一见[smoke 契约](testing.md#当前完整-smoke-契约)。

## L3：不同流程与核心可靠性

| 场景 ID | Case ID → 固定预期 | 独有验证目标 |
| --- | --- | --- |
| `local-codex.result-acquisition` | `default` → `parsed` | 既有 Codex 登录/CLI 的独立文本分析与取消，两个内部 case；无 development Run/服务/PR |
| `orca-local.codex` | `default` → `merged` | 本机 Codex 业务问答、发布后反馈拒绝、受理丢响应对账、同构建重启及活动停止 |
| `orca-local.pi` | `default` → `merged` | 本机 Pi 业务问答、原请求/会话与真实交付停止；不继承 Codex 证据 |
| `orca-remote.smoke` | `default` → `merged` | 配对远端工作区/会话及真实 PR 交付/合入；不继承本机资格 |
| `orca-remote.caller-exit` | `accepted-response-lost` → `merged` | 受理回执丢失后结束 caller，原服务继续；原 request_id 找回唯一 Run，幂等且零重复输入 |
| `orca-remote.connection-reconnect` | `active-disconnect` → `merged`；`pre-send-disconnect` → `merged` | 活动断连自动重接；发布发送前仅状态通道断连，原 operation/Attempt/session 保留且发布一次 |
| `orca-remote.service-restart` | `graceful-active` → `merged`；`kill-active` → `merged`；`graceful-boundary` → `merged`；`kill-boundary` → `merged` | 活动/阶段切换窗口分别正常停服与强杀；原授权和会话继续，迟到结果不丢失、零重发 |
| `orca-remote.stop-revocation` | `active` → `stopped`；`boundary` → `stopped`；`unreachable` → `revoked-unverified` | 活动/阶段切换/暂时不可达时撤销；迟到 final 不触发后继，撤销跨重启保持 |
| `orca-remote.publication-unknown` | `completion-gap` → `merged`；`query-unavailable` → `merged` | PR 已存在但完成证据丢失/查询不可用；恢复原 final/PR/head，查询恢复前不假报 ready，零重复发布 |
| `local-codex.audited` | `default` → `pr_ready` | 两实际不同模型实施/同轮代码审计修复/切回发布；pre/post HEAD 与 PR 来源；PR 不合入 |
| `local-codex.multirun` | `default` → `pr_ready` | 同一服务两请求，request/command/events/session/worktree/branch/PR 隔离；停一方另一方继续 |
| `orca-remote.multirun` | `default` → `pr_ready` | 配对远端同服务两 Run 隔离、各自原结果与停止；PR 不合入 |
| `orca-local.smoke` | `default` → `merged` | 本机 Orca IPC 路由、结构化会话、原结果与真实 PR 交付/合入及停止 |
| `orca-remote.initialization-stop` | `before-readiness` → `stopped`；`restart-before-readiness` → `stopped` | 原生已启动但 caller 未确认、或此窗口服务重启；零业务输入和替换会话，停止核对原身份 |
| `local-codex.progress-file` | `default` → `stopped` | 真实工具写入原环境路径；非 JSON 字节、原子替换、重复内容不增版本 |
| `orca-local.progress-file` | `default` → `stopped` | 本机 Orca 实际执行环境和文件读取，原工作区/绑定与停止 |
| `orca-remote.progress-file` | `default` → `stopped` | 配对远端真实文件写入/读取，本机不存在源文件；任意字节及停止 |
| `local-codex.progress-multirun` | `default` → `stopped` | 同服务两 Run 交错更新的路径/内容/版本隔离；停一方另一方继续，业务 revision 不因汇报改变 |
| `local-codex.progress-recovery` | `default` → `stopped` | 读取失败保留最后快照；原工具停服期间继续写，同会话重启零额外输入，终态可查询 |
| `orca-local.cli-wait-retry` | `default` → `stopped` | 真实 CLI cache 竞争、原操作有界排队/恢复，排队 stop 后零新资源/输入及精确结算 |
| `orca-remote.cli-wait-retry` | `default` → `stopped` | 固定 environment/runtime 的 repo list 竞争恢复，原操作/Attempt 保留；取消及唯一输入/结算 |
| `orca-local.native-development` | `default` → `pr_ready` | 本机原生候选 Runner 开发交付、反馈拒绝、原结果与未合入 PR，按当前结构化接线核对 |
| `orca-local.native-active-stop` | `default` → `stopped` | 本机真实活动会话 stop、授权撤销及绑定原 session 的 close 证明 |
| `orca-remote.native-development` | `default` → `pr_ready` | 配对远端同合同开发/反馈拒绝/原结果与未合入 PR，原 host key 保留 |
| `orca-remote.native-active-stop` | `default` → `stopped` | 配对远端真实活动 stop，原 host/session 及 close 回执；不证明主机全进程清场 |

同一 case 的不同故障窗口各有独有断言，不能因为步骤相近就合并；默认选择场景会运行其全部 case。独立 parser 不创建业务 Run，审计与多 Run 的 PR 保留未合入，其余交付/停止按各 case outcome 判断。

### 并行与故障隔离

每 case/repeat 独立 root、state-dir、服务、Run/session/worktree、base/head/PR、控制门、journal 和故障设施；case 内多 Run 允许共享服务但资源身份必须独立。相同资源身份、缺身份或构建摘要变化拒绝。调度锁只协调调度器，协调器退出不证明原资源已清理。

execution-stability 的 active-disconnect、pre-send-disconnect、两种 publication、两个 restart boundary 及 stop boundary/unreachable 需管理员网络宿主；按 registry requirements 核对。不影响共享 runtime 或其它现场，不以单次最终 passed 代替故障命中、输入/发布计数与清理隔离证明。

progress 五项验证真实源文件/绑定、任意字节/原子替换、同服务两 Run、读取失败保留和重启零额外输入；不创建 PR。原窗口要求首次真实写入后稳定 120 秒，每项与至少另一通过项重叠；仅 transport_unavailable/read_failed 暂态可在 20 秒内恢复，总窗口至多 140 秒、末尾恢复 observing，内容/版本/时间/摘要不变。`verify-progress-parallel.ps1 -Report <本批目录>` 校验同构建和实际重叠，设置 MaxParallel 不等于通过。[原专项验收](archive/2026-11/implementation/acceptance/runtime-progress.md)仍绑定旧构建，不能代替当前接线复验。

CLI wait 两项使用真实用户共享 cache 和独占测试 lease，故障互斥 key 私有；独立 state-dir 不代表 CLI cache 隔离。原 create/send 未知零重发，取消后零新资源；本机与远端分别取证。[Spec](spec/orca-cli-wait-retry.spec.md)及[验收](acceptance/orca-cli-wait-retry.md)记录七个受控变体与原生范围。

历史并行样本、远端共享 environment、其它模型/平台容量和跨批次收尾各按构建登记，不从原 feedback/审计上限 2 的样本推导全部组合通过。

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

## 独立原生 L3 专项

专项为显式 opt-in，独立于通用 L2–L4 场景计数。省略原生 EXE 或只执行受控部分不能算真实通过；默认 skip 也不能算通过。

| 专项 ID | 入口 | 独有覆盖与前置 |
| --- | --- | --- |
| `local-codex.generate-repository` | [`scripts/test-foundation.ps1`](../scripts/test-foundation.ps1) / `TestSRContentRepositoryNative` | 先通过 ContentRepository；全新只读 Git 调查固定 commit，真实代码工具、两份完整文档、请求重放/重启结果不变、停止及离线结算。无目标写入/PR。 |
| `local-codex.generation-recovery` | [`scripts/test-generation-recovery.ps1`](../scripts/test-generation-recovery.ps1) / `TestSRContentNativeGenerationRecovery` | 先通过恢复受控前置，显式原生 Codex；活动 graceful、kill、startup-stop，原进程/session/turn 与离线结算。无 Git/PR、跨构建接管或 parser owner 恢复。 |
| `local-codex.content-workflows` | [`scripts/test-content-workflows.ps1`](../scripts/test-content-workflows.ps1) / `TestSRContentNative` | 先通过内容受控前置；一次生成、同 thread 多轮/中断/空闲重启及停止/离线结算。generate-takeover 子 ID 保留，但旧 verify-takeover 只验 exit 2、原 DB 不变。 |

各专项要求显式原生 executable，参数与原子场景按对应入口和 catalog 登记。固定仓库调查、内容流程及恢复的实际范围分别见[仓库调查验收](archive/2026-11/implementation/acceptance/generate-repository.md)、[内容流程验收](archive/2026-11/implementation/acceptance/content-workflows.md)、[恢复追补历史](archive/2026-11/acceptance/pr-test-gaps-20261020.md)。

## 维护与证据入口

每个新增项先明确独有断言、固定预期、隔离/互斥和资格缺口，再同步注册、fixture、driver 及清单。未注册规划不计实现数量；受控通过不代替真机或并行资格。判定规则统一见[测试指南](testing.md#批次报告)，全部构建事实通过[独立验收目录](acceptance/)检索，中央表格不要求穷举每份报告。
