# Spec：本机 Orca Codex execute/1 结构化执行与 DSH 退役

日期：2026-11-03。状态：用户已确认本 PR 范围及停止证据限制；已实现窄范围，验收和未核实项见[实施记录](../acceptance/orca-structured-agent-session-mvp.md)。代码基线 `457ffaea20b6227a0bdaec9e56f5b9e7f54da337`。对应 [Plan](orca-structured-agent-session.plan.md)、[本轮设计审计](../acceptance/orca-structured-agent-session-mvp-design-audit.md)。[P0 现场](../acceptance/orca-structured-agent-session.md)和[阻断再调查](../acceptance/orca-structured-agent-session-blocker-review.md)保留调查事实；本文件取代其先前“必须等全部组合与完整停止取证才能迁移”的实施门禁。

## 1. 本 PR 的交付边界

**目标：本机 Orca Codex 的新 execute/1 Run 使用原生结构化会话，多行正文只成为一条用户消息，取得原回合完整结果；停止证据不足时明确未核实。同时移除 DSH 当前执行和专用历史支持。**

| 项目 | 本 PR |
| --- | --- |
| 迁移组合 | adapter=orca-terminal、target.kind=local 且 environment 为空、agent=codex（含省略默认）、workflow=execute/1；仅新 Run |
| 执行机制 | Orca 创建 native session 并管理 Codex；首条正文通过 create.firstMessage 的一个 text block 提交 |
| 工作流 | 保持 execute/1 的单次执行、原输入预算/材料、完整结果、禁止追加业务输入及无自动发布 |
| 停止限制 | 原会话 close 最多一次；无充分独立结算证据则 stop_status=unknown，resources_settled=false 并给诊断，离线同样不报通过 |
| DSH | 撤下新任务准入、配置/能力声明、启动/扩展/打包/测试注册、专用历史/用量解析及恢复/离线适配 |
| 保持现状 | Orca development/audited、Pi、远端 Codex；公共 terminal；非 Orca codex-app-server、parser 及其 owner/签名/停止/配置 |

**不修改或要求修改 Orca。** 只消费已核验安装版本的现有公开 CLI/RPC；无 fork、定制包、注入、扩展、上游新增字段/API/事件或未来版本前置。无绕过 Orca 的 provider 连接、私有数据库/transcript 读取和 terminal fallback。

用户已接受该首组合的停止可核验限制；这不降低其它组合的既有停止或 audited 实际模型要求。此前 B02 不再作为本 PR 必须取得完整 resources_settled 的前置，B03 audited 不在本 PR。历史现场没有因此追认通过。

## 2. 配置、选择及持久边界

公开 adapter 和 CLI/API 请求形状保持，不新增公开 backend 或“发送方式”开关。在既有 freeze 入口按上表组合选择并冻结内部协议 `orca_agent_session_v1`；工厂、恢复、inspect 和停止都按原冻结协议选择。失败不改走旧链，旧 Run 不按新 EXE 的组合规则重新选链。其它 Codex/Pi 组合仍走原冻结实现。

新链路的 profile/state 使用 `orca.agent-session-profile` v1、`orca.agent-session-state` v1，沿既有 opaque ImplementationDataEnvelope 和 SessionContext 保存身份；不复制全量 Orca journal、不建第二状态库。opaque payload 只保存 target/runtime、原 worktree ID/instance/path/base、预生成 sessionId、原操作/消息/turn/provider 引用、必要 cursor 和实际派发状态。冻结输入与完整结果沿已有 StageInput/原 operation/ContentResult 或消息存储保存，不在限额 64 KiB 的 opaque state 再复制正文/结果，不为此新建 blob 库。native fence 是 lease 观察值，不是进程 incarnation，也不是 strict CAS 成功证明。

只为新协议补必要资源表达、严格解码/clone/持久化/准入/恢复与停止消费者；不得把 sessionId 塞入 terminal handle 或伪造 OS PID。新资源已可能创建时，不能因 Ref.External.Kind/terminal 为空走“无资源即 stopped”的快捷路径。

选中组合不得生成或执行旧 helper/TUI 启动命令，包括 helper metadata 探测。显式 `agent_launch.runtime_executable/runtime_sha256`、`executable` 或非空 `args` 在该组合的新请求中明确拒绝并说明原生 Orca 管理执行器；不静默忽略、不自动改用户配置。model/effort 沿现有配置及缺省冻结，显式选择通过 firstMessage.selection 提交；full_access 用已有公开 permission 选项，缺少兑现能力则正文派发前拒绝。请求选项与实际报告分别标识，不承诺 audited 逐回合实际模型资格。其它组合的配置校验不随之收窄。

暂无全局 state/v8/user_version 变更依据，不强制换目录或新建旧格式兼容矩阵。升级先由原构建排空旧活动 Run；新协议记录不让旧二进制接管。不迁移旧 DSH 活动任务；需要退出时由原构建先处理。

## 3. 最小结构与会话所有权

| 位置 | 职责 |
| --- | --- |
| application / Workflow / reliable | 原业务输入、预算、持久意图、撤销/单 writer、结果事实和公开状态；不解析 Orca wire |
| protocol/orca | 原生 create/history/close 及完整结果确有需要的 item；类型、指纹、双层回执解码、本机 unary I/O |
| 既有 Orca adapter 包 | 新协议的窄生命周期实现，绑定 Run/operation/session/message/turn，认领开始/结果，原会话停止/只读对账 |
| 既有工厂 | 按冻结协议装配；未迁移组合继续原实现，无按错误重路由 |

复用现有本机连接、worktree 创建/固定 base/受限文件、operation/事务和实际派发 checkpoint。现有 workspace 准备中与 Agent terminal 强绑定的步骤仅在新协议路径拆开；工作区默认 shell 如被创建，按真实辅助资源登记与正常清理，不当作 Agent owner。

首 PR 不建设 Pi 映射、远端结构化 transport、新订阅体系、send/feedback/answer、队列/steer/continuation、模型路由平台、native viewer 或审计 Git 服务。只有确实新增第二个消费者时再抽取 Agent 薄映射，不先建仅有一个实现的插件层。共享 I/O 只保留实际使用的窄边界。

## 4. 原子首消息与可靠派发

1. 原 worktree 准备后，首次业务外发前冻结完整正文/预算、sessionId、clientOperationId、clientMessageId 和 operation/body 摘要；原身份随持久意图保存。
2. 首次业务动作执行一次 create.firstMessage。复用已有 lifecycle action/事务表达该原子副作用；准备步骤不先空 create，后续 StartTurn 也不得再 send 相同首消息。不为 API 名称另造业务调度动作。
3. 实际 socket write 前消费一次 CheckpointDispatch；排队未写和可能写入严格区分。撤销/持久失败零调用；同一原动作不能换 ID 重发，错误/未知不走 CLI 或 terminal fallback。
4. 使用最终冻结正文的一个 text block，LF/CRLF/空行/尾空白作为正文数据（该新组合的 execute 准入允许多行；其它组合原规则不变）。不走 LF→CR、bracketed paste、补 Enter、TUI 模型命令头或 trimEnd。沿原既有冻结规则计量；Runner 预算与 Orca blocks UTF-8 wire 上限分别检查，不扩大业务预算。
5. mutation envelope 摘要与 submission 的 body-only 摘要分开；按 P0 canonical JSON/UTF-8/SHA-256 及固定向量实现，不能直接采用 Go 默认 HTML/Unicode 转义结果。外层 RPC ok 与内层 result.ok/refusal 都检查。

create 返回 pending/accepted 不代表 Codex 已开始。预生成 sessionId 不是资源已存在的证明，但它必须在丢响应后仍足以只读查询原资源、供 stop 定向；不能因此再建另一个 session。

## 5. 观察、结果和恢复

共同读取只用有界 unary history；确需完整 item 时补 item。复用每连接单 reader、期限/取消/分页限制和 runtime/request identity 核验；不为了本机命名管道不支持 subscribe 而新增 WS 配对或 streaming 系统。

核对原 clientMessageId、正文、submission、normalized user item、原 turn.userItemId 及结束原因。providerItemId 与 normalized user item ID 分别保存，不能假设相等。一个业务输入须对应唯一原用户消息；不得用“最新消息”、assistant 标记自述、UI 或旧 terminal history 认领新 native 结果。

原 turn 的可核验 started/ended/outcome 与完整原助手结果分别收集并沿既有 TurnEvidence/result 事务投影。完整性不足、消息被缩短/缺页、归属不明、多条不明用户输入或未知 outcome 时保持未核实；不能从最后一段预览拼造 final。已核验的公开 message 没有 commentary/final phase 字段：本版本按原 turn 的 journal 顺序取得全部完整 assistant 文本，以空行保留消息边界；FinalSource 标为原回合聚合来源，不冒充 provider final phase。短预览不能替代完整 item。显式失败/取消不映成成功，空结果沿 execute 原规则处理。

create 已可能外发、响应丢失或回执提交失败后，只读查询原 session/消息/turn；查不到不等于从未创建，不 ensure/reopen/resume/create/send 来“补恢复”。普通服务重启只接回原观察，补收原结果，绝不重发首输入；活动窗口、原查询预算和撤销不刷新。缺充分事实沿既有有界对账转人工，不增加新恢复框架。

execute/1 仍拒绝 reply/message/resume 等追加业务输入。遇到原生问题/审批只报告可用事实和当前受阻原因，允许 stop，不代答、不新增回答能力。可选 usage/progress 缺失明确未报告，不作为消息传递的新前置。

## 6. 已接受的停止能力与结果边界

停止 scope 在新 Run 冻结为 `orca_agent_session`，描述待核验的原会话资源范围，不表示已取得该范围的完整退出证明。显式 Run stop，以及正常结果后的既有自动收尾，都先沿原事务撤销业务授权，向同一原 session 发 close 最多一次；不强制 cancel→close，不实现 close 的 mutation 重试或独立停止账本。

| 事实 | 允许的表达 |
| --- | --- |
| 原业务 turn 已核验成功、完整结果已保存 | 原结果可读；既有 content_result 保留业务 outcome |
| close 请求返回 ok | 保存该请求回执/诊断；不据此设置 Stopped/WorkerAbsent/WritersAbsent=true |
| 原 close 已可能外发但结果未知 | 原 stop 操作只读对账，重复 stop/服务重启/离线查询均不补发 close |
| 无独立完整结算证明 | stop_status=unknown；公开 resources_settled=false 并说明“未核实”，不得解释为已证实仍运行或已停止 |
| 当前无可用只读接口证明退出 | 有界结束自动查询，保留人工处理出口，不永久轮询、不启动 Engine/Agent 来查证 |

沿现有结果与清理状态分离：保存完整 ContentResult 后，即使聚合 Run 仍处于 reconciling/停止未核实，也必须能读取结果；不以 cleanup unknown 覆盖业务成功/失败，不伪造整体 completed/stopped。无需增加全局状态枚举或让所有 Workflow 改用新完成模型。

正常 service stop 只脱离观察，不自动 close Agent；Run 自动收尾/显式 stop 是另外的原持久动作。late final 不恢复授权。新 scope 的离线 verify-stopped 必须路由到原协议；不能落入 OS/terminal 缺席、旧 stop 标签或空 Ref 的成功分支。缺证据时 aggregate verification 也不通过。

代价明确：此版本可能一直无法由程序证明原会话完全退出，停止请求成功也不改变这一点。保留工作区/资源线索，不自动删除现场或把升级判为已排空；原构建/用户按现有人工流程处理，不能以新目录、强制排空按钮或补发 close 隐藏限制。

## 7. DSH 执行与专用历史支持移除

本 PR 的 Orca 新任务只接受 Codex/Pi；显式 DSH/其它 agent 在新受理阶段返回 unsupported，不能按默认 Codex 解释，零 Agent 启动/输入。移除 schema/catalog/capabilities/默认值/当前示例及生产冻结中的 DSH 接线。

按消费者删除 DSH 专用 profile、dsh-managed 扩展及打包/test 注册，清除 terminal_runtime/input/usage binding 等共享文件的 DSH 生产分支。保留 Pi 受管 TUI、公共 terminal 和非 Orca 仍有调用的共享实现，不把整个共享文件一并删除。

DSH 专用历史/用量 parser、provider policy、读取夹具、恢复与离线适配一并移除，不保留多版本私有支持层。旧 DSH 记录不再保证语义读取/恢复/停止；必要时在记录级明确 unsupported，不能把 DSH 解释为 Codex/Pi、让全目录读取失败或影响非 DSH 恢复。只保留通用存储容器所需的普通错误隔离，不为 DSH 增设解码框架。

不物理清除用户 DB/文件，不自动卸载主机 DSH，不改历史 acceptance/evidence 字节或旧验收结论。旧 DSH 活动任务须由原版本先排空；新版本不接管。当前支持移除与历史证据留存分别处理。

## 8. 本 PR 验收要求

| ID | 必须证明 |
| --- | --- |
| MVP-01 | 组合选择精确、按原协议恢复；新 execute/1 无 helper/TUI 启动或业务 terminal send；其它 Codex/Pi 和非 Orca 不变 |
| MVP-02 | CRLF/LF、空行、中文/emoji、路径、引号/反斜线、代码块、尾空白正文保真；一个原动作一条用户消息、一次业务 mutation |
| MVP-03 | 原消息→原 turn→完整结果；带工具/多段输出、失败/取消、分页/缩短/错来源、双层拒绝分别处理 |
| MVP-04 | 持久失败/撤销零调用；create/close 丢响应和回执提交失败零重放；重启只读补收，无替代 session/ID |
| MVP-05 | 自动收尾/活动 stop/重复 stop/离线均不误报资源结算；结果仍可读、无资源快捷路径不漏 native session |
| MVP-06 | DSH 新任务拒绝及执行/专用历史资产撤下，旧 DSH unsupported 不污染同目录 Codex/Pi；Pi/terminal/非 Orca回归 |

真实资格限本机 Codex execute/1：至少一例多行正文、工具使用及完整结果正常收尾，一例确认原 turn 活动后的 stop；通过正式 CLI/API、工厂、冻结/持久化路径和隔离工作区执行，不造 PR、不合并或部署。停止未核实是本版本允许且须显式验收的结果，不计完整退出资格。P0 短探针不能替代正式 Runner 验收。实际场景、构建摘要、失败记录及回归范围见实施记录；停止完整资源结算仍未核实。

## 9. 后续迁移与本轮不做

| 后续目标 | 本 PR 保留边界 |
| --- | --- |
| development/2、Pi、远端结构化迁移 | 以后复用本 PR 的会话 wire/身份/原结果与可靠派发；现在不预建无消费者能力，不安装远端 Pi |
| 本机 Codex audited 与旧 helper 完全退役 | 实际逐回合模型取证仍有现有公开能力缺口；当前保留原 Workflow/实现，不降低其证明要求 |
| 完整资源结算资格 | 本 PR 不要求 Orca 补能力，不追认所有后代退出；后续仅按现有充分证据或另行确认的产品合同处理 |

本 PR 删除 DSH；未迁移 Codex/Pi 的旧实现仍有真实消费者，不是新路径的 fallback。后续逐项替换并在最后一个消费者退役时删除，不能扩展旧 helper 作为临时输入桥，也不以本 PR 宣称四组合全量迁移完成。
