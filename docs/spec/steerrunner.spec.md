# Spec：SteerRunner 产品与 CLI 契约

> Orca 停止范围增量（2026-11-03）：原 session close 的成功回执可确认 `orca_agent_session` 范围结算；额外清理没有保证，不额外阻断结果、检测、Workflow、重启或部署。PC-R06/PC-R10 的“停止/排空”按对应 scope 解释，不扩大为全进程清场。close 自身失败或未知仍保留未知。统一设计见[Orca 执行链路设计](../orca-execution.md)，实际资格见[专项验收](../acceptance/orca-session-result-settlement.md)。

维护日期：2026-11-05；本次核对代码基线：`f6a389c` 加内容结果锁/离线读取职责收口工作树差异；既往专项基线保留在各节。本文长期维护 Runner 的能力、可靠性承诺和 CLI 契约，不随某一批实施结束而归档。

## 长期产品承诺

### 定位与适用前提

SteerRunner 是面向个人、运行在本人管理和信任的本机及多台远端机器上的 Agent 执行器。它接收确定的任务、材料和执行目标，按 Workflow 执行，维护可追踪的运行状态，返回结果或对应的 PR。无法可靠继续时保留已知事实和可定位的现场，说明缺口并交由本人处理。

用户负责主机、工具、账号登录、仓库权限和可用资源；调用方负责需求规划、任务拆分、目标选择及最终业务决策。Runner 管理 Run，不接管 SteerLine 的 Task、DAG、Candidate 或 TaskDelivery，也不自动合并 PR。并行数量受主机资源、后端能力及平台限制，不承诺无限并行、固定吞吐或任务必定成功。

以下是支持路径应维护的产品边界，不代表任意平台、Agent、Workflow 组合已经通过验证。实际可用范围须同时核对当前构建的 `capabilities`、生产校验和[验收记录](../acceptance/README.md)。声明支持、已实现和已验证分别记录。

### 能力与责任

| 编号 | 长期承诺 | 范围与限制 |
| --- | --- | --- |
| PC-C01 执行目标 | 在用户选定、已配置的主机和 Agent 上执行；支持独立 CLI 和客户端接入 | 主机、工具及登录由用户维护；具体后端和支持组合见当前实现范围，不承诺自动跨机迁移任务 |
| PC-C02 任务绑定 | 保存接受时的任务、配置、材料引用及 Workflow；仓库任务固定基线、工作区、分支和结果对应关系 | 开发材料是固定基线 worktree 中的仓库相对路径，不承诺额外保存文档字节副本；内容流程沿各自材料冻结契约 |
| PC-C03 长任务与流程 | 后台执行已接受的任务，按冻结 Workflow 管理步骤、结果、等待和显式用户输入，包括已声明的审计及修复步骤 | 不依赖调用方窗口存活；不接管调用方的任意任务 DAG，不承诺无限或超出 Workflow 定义的隐式返修 |
| PC-C04 结果与 PR | 返回可归属的结果；开发流程通过 provider 核对 PR 的仓库、base/head 等身份 | Agent 给出的 URL 是候选；PR 就绪不代表已经合并、资源已停止或代码质量获得完整认证 |
| PC-C05 审计 | 按选定 Workflow 执行审计及约定修复，保留结果来源和适用的提交绑定 | 不保证发现全部缺陷；不把旧提交的审计结论推广到新代码；仍遵守路径支持限制 |
| PC-C06 执行权限 | 开发使用执行账号已有权限及 Git/gh/SSH 登录 | 必须显式 full_access；worktree 用于任务组织，不提供 OS 隔离，不承诺单仓库凭据、凭据退出失效或抵御恶意 Agent |
| PC-C07 可见性 | 服务可用且查询合法时，按公开契约查询已提交的 Run 状态、受理回执、操作结果、停止状态及已保存结果；查询失败明确报错 | 核心查询返回已提交事实，不保证外部状态已实时刷新；消息摘要、文件进度、日志采集、用量和 Viewer 实时性等可选观测尽力提供 |

开发权限采用[宿主认证](host-native-auth.spec.md)和[统一 full_access](development-full-access.spec.md)，不再创建 broker、CA、socket 或认证租期。内容流程的 read_only 和 parser 自身工具禁用机制继续保留；个人信任环境的定位不等于这些现行机制已删除，也不取消基本输入校验、凭据脱敏或精确资源身份核对。

核心状态及回执查询不适用“可选观测尽力提供”的降级约定：无法查询时返回明确错误，不能伪造成功、空结果或终态。已提交快照与外部实时事实分别表达；外部查询失败时保留的旧事实沿现有契约带观察时间和诊断，不冒充最新状态。可选通道缺失、过期或采集失败只表示可见性缺口，不证明任务失败、完成或停止；不承诺逐 token 可见、断线期间实时更新或费用统计绝对准确。

### 长期责任边界与当前支持范围

长期不承担的职责包括多租户安全、主机级安全隔离、集中凭据托管、用户任务规划和自动跨机故障迁移。这些边界服务于个人可信主机的定位。

当前仅交付 Windows/amd64、远端使用 Orca、Workflow 使用内置定义且不提供外部定义装载，是本版本的实现及支持范围，不作为永久禁止扩展的产品原则。以后增加平台、后端或 Workflow 来源时，另行明确接口、行为和验证范围，并更新当前支持声明；现有代码或本文方向描述不自动构成新组合的支持资格。具体 Workflow 已声明的有限审计、修复或其他步骤仍应按定义执行，不能以“不承诺隐式返修”为由跳过。

### 可靠性承诺

可靠性的重点是任务不串线、未知业务动作不盲重发、停止后不继续派发、没有证据不报成功。以下保证限于声明支持的机制及可核实事实；不以增加覆盖所有异常的恢复框架作为默认目标。

| 编号 | 应保持的规则 | 不作出的保证 |
| --- | --- | --- |
| PC-R01 持久受理 | 单 state-dir 单 writer；同 request_id/同正文找回原命令，改正文冲突；Run、命令和业务事件原子提交后才发布状态或派发动作 | CLI 成功退出或内存状态不能代替持久事实；不提供与外部系统的跨库事务 |
| PC-R02 副作用与未知 | 首次外发前持久保存操作身份和固定输入；可能已发生的创建、启动、业务输入及发布只核实原操作，不因超时或响应丢失重新派发 | 不承诺外部系统端到端 exactly-once；未知可转人工，不用盲目重试换取表面成功 |
| PC-R03 任务及结果归属 | 按原 Run、工作区、会话、步骤和操作的必要身份接收事实；重复事实去重，冲突或错来源不推进业务 | 不仅凭终端文字、最新 HEAD 或一句完成声明认定原任务完成；不要求所有后端具备相同原生取证机制 |
| PC-R04 退出与重启 | CLI/调用方退出不撤销已接受的执行；同构建的服务普通重启在原资源仍有效时接回原授权、游标及未完成操作，补收原结果 | 不保证资源丢失后重建 owner、Attempt、终端或 session；主执行不因重启重新授权或刷新预算；独立 parser 的例外见下文 |
| PC-R05 有界恢复 | 查询沿原操作的适用预算；自动窗口耗尽或资源丢失后进入待人工，显式 resume 仅有界核实原事实并复核继续资格 | 不无限自动恢复，不自动跨机故障迁移；活动长回合不因未知输入的短查询期限被判失败；技术恢复不解除业务等待或停止撤销 |
| PC-R06 撤销与停止 | stop 持久受理后禁止新的业务派发；按冻结的后端范围终止原资源，有证据才报告 stopped，无法核实仍为 unknown/unsupported | 已在途动作不可回滚；停止不撤销宿主登录，不证明整台主机无相关进程；服务停止、观察脱离与 Run 停止分别表达 |
| PC-R07 结果声明 | 分开表达原生执行事实、Agent 汇报、程序核验、Workflow 完成、PR 平台事实及资源结算 | 不独立认证 Agent 的全部测试/质量声明；Agent 自述不能代替 Workflow 已要求的 final、审计来源或 PR 核验 |
| PC-R08 观测容错 | progress/result 文件准备及采集故障只影响通道事实，保留可用快照和诊断，不成为新的业务门禁 | 不把观测失败解释为任务失败或资源已停止；业务意图或受理事务保存失败仍禁止派发，不能借观测容错跳过 |
| PC-R09 并行任务 | 各 Run 的工作区、会话、输入、结果和停止目标分别绑定；只操作精确归属的资源 | 不提供主机级资源配额或 OS 安全隔离；后端串行通道、排队及资源不足可能限制并发 |
| PC-R10 升级 | 原版本先排空原生资源和未完成操作/控制，再升级；兼容且已排空时允许保留目录和历史 | 不跨构建接管活动任务，不维护退役私有格式矩阵；pr_ready 或 service stop 均不能代替排空 |

当前有界人工对账、普通服务重启及升级规则见[个人执行器简化 Spec §4](personal-executor-simplification.spec.md#4-恢复与升级收缩pes-02)和[操作指南](../cli-guide.md#人工对账与资源丢失)。服务重启不等于执行主机重启后仍保证资源存活。

**独立 parser 的可靠性例外与代价：** parser 是内存文本任务，未提交的结果可能因服务中断丢失。服务重启或显式恢复后，仅在当前业务源及推进资格仍满足时，允许按当时的 parser 配置重新分析，可能重复推理和收费；不保证跨重启的总调用次数、总费用、相同模型或相同输出，也不恢复原 parser 操作或继承其预算。该取舍仅适用于独立文本分析，不允许重发原实施、审计、发布或其他业务执行。取消、结果采纳及 fallback 仍沿[parser 契约](parser-decoupling.spec.md)，不是在同一等待状态无限自动重算。

### 成功与停止的含义

| 对外事实 | 表示什么 | 不能推导什么 |
| --- | --- | --- |
| accepted | 请求及原受理命令已经持久保存 | 模型已开始、业务已完成 |
| 开发交付 / pr_ready | 开发 Workflow 的结果和 PR 身份等必要核验条件已满足 | 所有测试由 Runner 独立复验、PR 已合并、执行资源已停止 |
| generate 完整完成 | 完整原生 final 及适用的格式/schema 校验通过，结果已固定，并完成现行停止及资源结算 | 仅取得成功结果就代表全部完成；清理失败不能显示全部完成；不代表内容质量获得独立认证 |
| conversation 回合完成 | 本轮原执行及结果按契约结算，保留结果/轮次事实，等待显式 message | 整个会话已关闭或资源已停止；原回合失败/取消不因回合结束被改写为业务成功 |
| stop 已受理 | 后续业务输入授权已撤销 | 已在途动作回滚、原生进程立即结束 |
| stop_status=stopped | 冻结后端范围内的停止证据满足现行契约 | 任意派生进程或主机后台服务均结束、宿主凭据撤销 |
| resources_settled / verify-stopped | 在报告范围内取得现行资源结算事实 | 超出 scope 的主机级清场、任务质量或业务成功 |
| service 已停止 | 服务写者退出 | Run 已停止；保留的原生执行可以继续 |

Workflow 完成条件按具体定义解释，不能把 `pr_ready` 的资源边界套用到全部流程。内容结果状态与 cleanup/stop 状态分别报告；generate 成功结果已取得但清理失败时保留这两个事实，不能以结果成功覆盖未结算状态。conversation 的一轮结束也不自动关闭会话，close/stop 沿其显式控制契约。详见[内容流程契约](content-workflows.spec.md)。

当前停止保证按冻结合同分别表达：Codex/development/2/full_access 不配置 helper 时，本机与远端 Orca 均以 `orca_terminal` 核实原终端；本机显式受管 Codex 的 `orca_local_codex_resources` 核实原业务/控制终端及 helper、app-server 原根进程；直接 Codex 和 Pi/DSH 沿各自现行机制。原生合同实际模型未独立核验、workspace audit unreported，不继承 audited 资格。选择、保留例外与排空见[Orca 双端执行合同](../orca-execution.md)。PC-R06 不将这些范围扩大为主机级清场。

<a id="待实施的承诺收敛本机-orca-codex-停止"></a>
### 现行停止范围：本机 Orca Codex

状态：2026-11-01 已实施并完成专项最小验收，尚未部署或发布。见[Spec](orca-local-codex-stop-simplification.spec.md)、[Plan](orca-local-codex-stop-simplification.plan.md)和[验收](../acceptance/orca-local-codex-stop-simplification.md)。旧标题锚点仅保留链接兼容，不表示仍待实施。

本机受管 Codex 按新 scope 终止并核实登记的原业务终端、控制终端、helper 和 app-server 原根进程。保留最多 15 秒的原停止预算、最多 2 秒的协作停止、原 helper 的 Job/工具实际清理、精确身份及停止后零新业务输入。删除签名退出文件和 control Closed/pending reader 响应作为该范围结算门禁；同一判据用于显式停止、服务关闭中的核验和离线 verify-stopped，后者即使已有 stopped 也重新查询原资源。

代价是停止成功不再证明所有后代写进程退出；脱离管理的进程可由本人检查。原 stop 可能外发后只读对账，不重发 helper 请求或 close；部分清理中断可保持 unknown 转人工。正常业务及审计查询仍核验 reader/结果来源；专属 helper 退出文件产出已删除，公钥请求绑定、共享签名封装及直接 Codex owner/checkpoint 的真实签名仍保留。直接 Codex、Pi/DSH、原生远端和独立 parser 不改用此 scope。

scope 在新 Run 受理时冻结，公开为可选 `stop_evidence_scope`。state/v8、user_version=8 和新默认目录明确隔离 v7 旧承诺；不回填旧 Run、不按新 EXE 重释原停止保证。原版本先排空，旧历史仍由原版本读取，显式目录和客户端绑定不自动迁移。

确定收益是删除两类停止专属证明规则；adapter 生产代码净减少 10 行，包含格式、投影及客户端后生产代码净增加 22 行。未测得可比耗时加速，不承诺正常任务提速或全局净删。专项仅取得受影响组件、3 项公开受控及一条真实本机停止探针资格，不继承为完整 L2/L3。停止入口与业务证据的剩余耦合已在[再审计](../acceptance/orca-local-codex-stop-reaudit.md)修正，最终构建与验证以该报告为准。

### 现行实现清尾：退出证明产出与旧预算重建

状态：2026-11-01 已实施；见 [Spec](exit-proof-budget-cleanup.spec.md)、[Plan](exit-proof-budget-cleanup.plan.md)、[初轮验收](../acceptance/exit-proof-budget-cleanup.md)及[实现再审计](../acceptance/exit-proof-budget-reaudit.md)，最终构建/验证以再审计报告为准。本机 Orca Codex 不再预开、签名、写入或 Sync 专属 exit.json，也不保留其专用 verifier/读取包装。公钥形状、readiness 编码和原停止请求地址/完整匹配、Git exclude、实际 Job/reader 清理及直接 Codex owner/checkpoint 签名保持。

已冻结阶段输入的新派发只接受已有合法 Budget，不再从旧提示词、材料或反馈反推来源。仅 PendingTurn 副本缺失时，可在原命令、步骤、版本、Attempt、正文及模型等绑定一致、尚无该输入的持久外发意图时复制原 StageInput 的合法 Budget，并先提交再继续。尚未渲染草稿仍正常首次冻结，已有预算（包括旧来源标签）继续校验。

取舍：不再保证恢复已冻结却缺 Budget 的不完整现场，即使旧算法曾能补算也拒绝新候选，可能需本人处理。此限制不阻止原查询、结果观察、accepted/unknown 只读对账、stop 或离线核验，不补写原意图/结果。state/v8、CLI 和 Workflow 版本保持，不新增迁移器或恢复框架。生产代码分项净减 103/83 行；未做同条件耗时比较，不宣称任务提速。未部署、未发布，不授予完整 L2/L3 资格。

### 现行实现清理：旧规则与 Lifecycle 接口

2026-11-01 已按[Spec](dormant-lifecycle-cleanup.spec.md)、[Plan](dormant-lifecycle-cleanup.plan.md)实施并完成[专项验收](../acceptance/dormant-lifecycle-cleanup.md)。删除无生产入口的旧准备/回合/恢复模型、三类未调用 Lifecycle 方法和六个无读取能力字段。实际继续资格、受理/开始区分、原操作对账、观察、结果/预算、owner/Job/reader、权限和停止仍由现有生产入口执行。

保留 state/v8 的全部持久字段和动作/解码，以及 Intents/Snapshot.Operations 表示；公开 capabilities、资源摘要和编码未变。生产净减 688 行，收益为规则与维护面减少；未测同条件耗时，不承诺提速。该批取得受影响组件和三项公开受控资格，未部署/发布，不增加真实模型或完整 L1/L2/L3 资格。

2026-11-02 [实现再审计](../acceptance/dormant-lifecycle-reaudit.md)复核全部保留声明和实际入口，确认持久格式及原执行、恢复、停止承诺保持；完整组件组与补充恢复/停止测试通过，无新增生产修复。

### 现行实现清理：旧历史采集与 Orca Backend

2026-11-02 已按 [Spec](history-adapter-cleanup.spec.md) / [Plan](history-adapter-cleanup.plan.md) 实施并完成[专项验收](../acceptance/history-adapter-cleanup.md)。删除无生产装配的 Codex 历史分页采集/旧事件模型和未构造的 Orca Backend 包装、三个旧接口及其私有能力报告。真实 history binding 文件/编码、原 thread 恢复、完整 final、公开消息和当前 conversation Workflow 保留；共享 MessagePhase、64 KiB 原读取常量及恢复测试夹具迁址保留。

生产净减 986 行；state/v8、持久字段/动作解码、双操作表示、公开 capabilities/资源摘要和原执行/恢复/停止承诺不变。最小验收包含 490 个组件/application 顶层和两个公开受控场景；8 项 skip/3 个无测试包、两次编译遗漏与修复均单列。维护面减少不等于任务提速；未部署/发布，无新增真实模型或完整 L1/L2/L3 资格。

同日[实现再审计](../acceptance/history-adapter-reaudit.md)重新核对全部保留声明、真实消费者及共享恢复夹具，未发现新增确认缺陷；持久格式和原执行/恢复/停止承诺保持，44个补充定向顶层通过。生产/测试源码与初轮最终验收一致，最新资格以该再审计报告为准。

### 现行实现收口：单事件确认与旧环境分支

2026-11-02 已按 [Spec](event-ack-cleanup.spec.md) / [Plan](event-ack-cleanup.plan.md) 实施并完成[专项验收](../acceptance/event-ack-cleanup.md)。通用事件仅保留原 Persisted 单确认：持久事实仍在原 acceptEvent/commitLocked 返回后回复同一错误；重复/可选消息不被迫新增事务，独立 owner 的确认沿原生 journal 已落盘后的原 drain 点，不代表中心事务提交。错误/取消不自动补成功确认，原观察、消息采样、未知只读恢复与停止机制保持。

同时删除无消费者的通用事件字段、无赋值入口的 BindingSnapshot 环境分支及旧声明；真实短输入 trace 和底层 Codex 问答测试消费者保留。state/v8、持久字段/动作解码、双操作表示、公开能力/资源摘要和长期承诺均不变。生产净减105行，492个组件/application顶层及三个公开受控场景通过；skip/首轮新夹具失败单列。只减少重复协调与维护面，不宣称任务提速；未部署或发布，无新增真实模型或完整 L1/L2/L3 资格。

同日[实现再审计](../acceptance/event-ack-reaudit.md)对 worker、owner 和 Orca observe 做精确变换复核，确认原单次回复位置、三类确认语义及错误/取消处理保持；9份共享夹具和持久格式复核通过，无新增确认缺陷。生产/测试源码未再改，33个补充定向顶层通过；最新资格以该报告为准。

### 现行实现清尾：零消费者生产入口与孤立夹具

2026-11-02 已按 [Spec](orphan-entry-cleanup.spec.md) / [Plan](orphan-entry-cleanup.plan.md) 先设计审计再实施，完成[专项验收](../acceptance/orphan-entry-cleanup.md)。删除17个无实际消费者的旧生产函数及独占声明，生产净减201行；另删孤立FakeTerminal及两个无调用平台锁文件，测试支撑净减333行。真实MockAppServer/StagedBusiness/historyFixtureBinding和正式创建探针及原断言保持，未修改实际执行、恢复、单确认或停止规则。

state/v8、全部持久字段/动作/解码、双操作表示和公开能力不变；建议C及持久格式退役未实施。100个保留声明和777项其余源/配置保持，编码相同；最终496个组件/application顶层和3个公开受控场景通过，原skip及首轮沙箱失败/超时另记。收益为维护面减少，不宣称提速。未提交、部署或发布，无新增真实模型或完整L1/L2/L3资格。

同日[实现再审计](../acceptance/orphan-entry-reaudit.md)未发现产品缺陷，修复证据目录Git原字节保护遗漏；删除符号无外部入口，18个关键消费者及全部原测试/共享夹具保持，重新编码一致，25个定向顶层通过。初轮交付状态见上，后续按用户授权提交并提PR，不合并、部署或发布。

### 现行实现清尾：新意图写入与展示残留

2026-11-02 已按[Spec](new-intent-boundary-cleanup.spec.md)/[Plan](new-intent-boundary-cleanup.plan.md)先设计审计后实施并完成[专项验收](../acceptance/new-intent-boundary-cleanup.md)。仅InputValue新构造不再接受四类无生产候选旧动作，删除空PreparationFacts.Cleanup步骤；历史字段/动作/校验/解码、原Request对账、owner和停止保持。submit_input及全部保留构造仍在，不把新写入限制加到原请求恢复。

另删无消费者Display及底层事件Summary/Title，保留Kind/Status真实问答消费者、原生journal与单确认。state/v8及全部持久编码/公开能力不变，原测试和共享夹具不改。A/B生产净减11/15行，仅减少少量维护面，不宣称提速。最终498个组件/application顶层及三个公开受控场景通过；新夹具失败、既有skip和Controlled计时导入资格单列。未提交、部署或发布，无新增真实模型或完整L1/L2/L3资格。

同日[实现再审计](../acceptance/new-intent-boundary-reaudit.md)未发现产品缺陷；快进main66cc0ac的上游测试更新后，原解码/对账、共享夹具、编码及停止承诺保持，27个定向和新主线最终EXE的三个公开场景通过。原文件夹具初始化失败与复验记录另存，不更改生产或原断言。上段为初轮资格/交付状态；后续按用户授权提交并提PR，不合并或部署。

### 现行结构与分批收口：Orca 能力层

Orca 对接按[能力层长期设计](orca-capability-layer.spec.md)逐批收敛：以 Orca 外部系统为边界提供独立能力，执行适配器消费并组合；每抽出一项即接入本机/远端消费者并删除对应旧实现。目标为同一个薄 Orca adapter，仅保留明确目标及必要流程差异。原生 Codex 通过自身协议能力接入，不依赖 Orca 包。

复杂取证抽离以前先核验无 helper 原生路径的可行性；能力迁移同时检查依赖和业务语义归属。B5 先实现完整候选生产接线，经真实 Runner CLI/API 验证受理、冻结、执行、发布、持久结果、在线 stop 和离线 verify-stopped，另覆盖活动停止与未知关闭结果；B6 在已验接线上最终启用并退役旧链。资格要求零旧链调用，能力测试装配、短文本会话及现有 managed 探针均不能代替。

B0–B6 已完成能力迁移、生产接线、正式合同与保留例外同步；最终集成构建的双端开发及活动停止、真实离线核验和独立审计均通过，见[B6 验收](../acceptance/orca-native-rollout.md)。当前选定原生合同及未验证边界见[Orca 双端执行合同](../orca-execution.md)。显式 managed/audited、Pi/DSH 与旧记录仍有消费者，保持原 scope/资格，不能把首轮原生通过扩大到这些组合。

### 本机 Orca Codex execute/1 结构化执行与 DSH 退役

2026-11-03 用户确认首 PR 收窄为 [Spec](orca-structured-agent-session.spec.md) / [Plan](orca-structured-agent-session.plan.md)：仅本机 Orca Codex 新 execute/1 从 native session 创建开始使用结构化单消息正文，取得原 turn 完整结果；本 PR 同时移除 DSH 执行、专用历史/用量与恢复支持。其它 Orca Codex Workflow、Pi/远端、公共 terminal、非 Orca codex-app-server 和独立 parser 保持原合同。不修改或要求修改 Orca，不依赖未来上游能力。

用户接受 Orca 全部额外资源清理不保证。该协议 `orca_agent_session` scope 以原 close 的明确成功回执确认原会话停止，允许正常结果封存与业务收尾；不证明任意派生进程、主机资源或凭据清理，额外清理未保证不成为流程门禁。close 自身未知仍为 unknown，不能伪造回执或重放 close。其它组合按其原 scope 核验，不把 terminal/helper 证明套给结构化会话。当前统一规则和验证入口见[长期设计](../orca-execution.md)。DSH 旧记录必要时明确 unsupported，不物理删除用户数据或历史 evidence。

上述结构化执行已实现；历史[设计复审](../acceptance/orca-structured-agent-session-mvp-design-audit.md)不代表代码/当前能力已改变。最终 Codex/Pi 全量结构化及 helper/TUI 退役仍是后续方向；audited 实际模型、完整资源结算和远端 Pi 的未验/环境缺口不追认通过。

### 现行职责收口：权限资源与原完成证明

2026-11-02 已按 [Spec](native-proof-boundaries.spec.md) / [Plan](native-proof-boundaries.plan.md)先设计审计后实施，见[验收](../acceptance/native-proof-boundaries.md)。Codex adapter 统一解释原权限包与冻结 profile 的匹配，application 不再扫描原生 sandbox 参数；adapter 负责原 owner 完成证明的适用性和读取，业务侧只接收事实并保留仓库/审计来源核验。要求原证明但缺失/无效时仍等待，不能用缓存或当前 HEAD 代替。

原签名读写、Git/common-dir 实际用途、持久格式、350 个既有测试文件及执行/恢复/停止/原提交承诺保持。仅修改两个原协调函数，新增三个窄函数，生产净增15行；收益为私有知识归属，非净删或提速。448 个组件、57 个定向应用顶层及最终 EXE 三公开场景通过；8 个既有 skip 和完整应用组的基线缺 Budget 夹具失败/总时限耗尽单列。未提交或部署，无新增完整 L1/L2/L3 或真实模型资格。

同日[实现再审计](../acceptance/native-proof-reaudit.md)未发现产品缺陷；新增实际结果提交/停止先到回归，同步 main `da8155e` 测试接线后，351 个既有测试文件、原持久/执行/恢复/停止保持，33 个定向及最终 EXE 三个公开场景通过。上段为初轮状态，后续按新增授权提交提 PR；初轮基线失败/时限边界不改记通过，不自动合并或部署。

提交前进一步整合 main `efa871c` 的文件材料及夹具修复；353 个既有测试文件无本批改动，38 个定向及同最终 EXE 三公开场景通过，原缺预算测试随上游修复后定向通过。与该 main 原源码捕获相比，145494 字节编码完全相同；与初轮捕获的差异仅是上游 capabilities/resource_digest。A/B 没有改变持久格式或长期承诺，最新产物与资格见再审计报告 §4。

### 现行职责收口：内容结果校验与离线读取

2026-11-05 按两份独立 [A Spec](content-result-lock-boundary.spec.md)/[Plan](content-result-lock-boundary.plan.md)、[B Spec](offline-inspection-boundary.spec.md)/[Plan](offline-inspection-boundary.plan.md)先设计审计，再实施 A、验证后实施 B，见[专项记录](../acceptance/content-inspection-boundaries.md)。内容结果的原材料校验移出 Engine.mu，保留同 Run 派发锁，重锁复核原 revision 后仍沿原结果事务提交；停止后的原 final 可收录但不恢复授权。离线 inspect 直接读取全部严格解码的持久 Run 和原命令，与驻留查询共用投影，不再构造执行 Engine、身份和 workers。

材料/仓库/路径核验、输入与恢复门、全量读取的错误顺序、三 selector 的 null/未找到/查询错误、writer lease/bootstrap/当前格式护栏均保持。SQLite 原只读路径的临时 WAL/SHM 行为保持，DB 原字节与持久/执行文件不变。state/v8、持久声明和编解码、现行后端的执行/恢复/停止及 PC-R01–R06/R08/R09 不变；不新增后台框架、持久校验票据或跨回合缓存。收益是解除跨 Run 材料 I/O 与提交锁、离线读取与执行构造的两处依赖，不以净删或未测量提速为承诺；实际构建和验证资格以专项记录为准。

同日[实现再审计](../acceptance/content-inspection-reaudit.md)未确认生产缺陷，新增真实stop入口的原提交边界回归；46项应用定向、74项组件通过，原4个现场opt-in跳过。351个既有内部测试字节、原持久编码及执行/恢复/停止保持；Runner/受控驱动/原生夹具SHA与初轮相同，三项公开受控沿原同产物证据，不虚记本次重跑。按用户追加授权提交提PR。

### 文档分工与维护规则

- 本节是能力及可靠性承诺的长期总入口。CLI 字段/退出码、Workflow 和专项 Spec 细化机制；[可靠性交付链路](../reliable-delivery.md)解释运行路径；[操作指南](../cli-guide.md)描述如何使用；acceptance 保存具体构建的验证事实。
- 改变产品承诺时在这里修改对应 PC-C/PC-R 项，标明现行或待实施，链接受影响专项。不能仅在某次 Plan、验收报告或聊天中改变长期承诺，也不能用新的产品方向覆盖尚未修改的运行行为。
- 区分长期责任边界与当前支持范围；版本的平台、后端和定义装载限制不自动成为永久禁令，增加能力仍需独立设计及对应验证。各 Workflow 完成条件、核心状态查询和独立 parser 的例外不能被通用表述覆盖。
- 实施完成后再把待实施条款并入现行承诺，更新相关操作说明和受影响的验证引用；只为实际受影响机制补验证，不因维护本文新增全后端测试前置。
- 已完成的快速定向验证、观测不阻塞、恢复/兼容删除及问答输入收口保持结项。残余具体导入或共用 Record 不自动构成重开重构的理由，需另有重复规则、实际故障或可验证收益。
- 本文后续保留首版背景及 CLI 细节；已取代条款按下面的当前基线和专项阅读。历史验收不改写，待实施方向不计为已实现或已验证。

## 当前实现基线与历史说明

当前使用 `steerrunner-cli/v1`、state/v8、SQLite user_version=8，默认 `%LOCALAPPDATA%/SteerRunner-v8`；拒绝 v7 及更旧目录，不迁移或代替原版本收尾。verify-takeover、跨构建活动任务接管、资源重建和旧 Check 已删除。开发权限与认证见上述现行承诺；恢复/升级详见[个人执行器简化 Spec](personal-executor-simplification.spec.md) / [Plan](personal-executor-simplification.plan.md)。

当前 Workflow 与结果获取见[能力解耦及结果获取 Spec](result-acquisition.spec.md)和[Workflow 定义](../workflows/README.md)；它们取代 workflow/1 的固定报告、双审计及发布后反馈返工。下文首版目标、旧版本、旧权限/认证及恢复描述只在未被取代的范围适用；不恢复 broker/sandbox、旧 decoder 或资源重建要求。

下文首版独立化调查日期为 2026-10-14，原[实施 Plan](../archive/2026-10/spec/steerrunner.plan.md)及[专属验收](../acceptance/steerrunner.md)保留其历史范围。2026-10-15 的本机限定验收、此前取消的 Orca 验收不改写，也不否定后续构建的专项结果。当前支持及缺口从[文档索引](../README.md)和对应验收查阅，不以首版记录推导当前全部资格。

## 1. 目标、依据与首版范围

SteerRunner 接收固定 Spec、Plan、参考材料及执行配置，在指定本地或远端环境按 workflow 实现、验证、处理反馈并提交、推送、创建或复用 PR，输出可核验的执行结果。它可由用户独立使用，也可由 SteerLine 或其他调用方经同一 CLI 契约使用。

用户已经确定：

1. 独立仓库为 SteerRunner；以一个 EXE 提供常驻服务与 CLI 客户端。
2. SteerLine 使用 CLI 契约；CLI 是短生命周期的通信客户端，执行由常驻服务承担。
3. 默认 workflow、提示词及必要程序资源内嵌，不依赖 SteerLine 的安装文件或文档资源。
4. SteerLine 指定项目运行目录下的独立 Runner 状态目录，Runner 自己管理该目录的执行数据和日志。
5. 任务规划、DAG、领取、候选选择及最终 TaskDelivery 留在 SteerLine。

首版包含 Windows/amd64 的独立程序、常驻服务、CLI、持久化运行状态、默认开发流程、提示词覆盖、现有本地 Codex 和 Orca 本机/environment 路径的提取与接入。远端一期沿用 Orca environment；不因共用代码而声明任意机器、其他代理或所有恢复行为已可用。

首版不包含任意 workflow 图编辑器、通用任务 DAG、新的 SSH/远端 daemon、Windows SCM 安装、独立 UI、自动合并 PR、旧 SteerLine 数据迁移、双写或旧执行回退。第三方运行时和持久化流程引擎的选型不构成本次前置。

### 1.1 现有实现调查

调查读取的 SteerLine HEAD 为 `a81341a515251e23b4fa19d7a09096c11d0db8dc`。下列路径均相对于 SteerLine 仓库；本次仅阅读，不修改该仓库。调查版本不是未来代码抽取基线，实施开始时应重新固定两个仓库的真实代码与文档 commit。

| 来源 | 结论及拆分要求 |
| --- | --- |
| `internal/execution/command_boundary.go`、`service_construction.go`、`service_state.go` | 执行仍依赖 TaskWorkPort、TaskResultPort、图快照及混合配置/宿主；需由独立 Run 输入、Runner 存储与宿主替代 |
| `internal/execution/prompt.go`、`staged_flow.go`、`staged_result.go`、`model/staged_flow.go` | 当前 development/v3 拥有实现、报告、发布及可选发布补报；提取行为，去除任务命名与状态依赖 |
| `internal/execution/backend/lifecycle.go`、`adapters/codex`、`adapters/orcatransport` | 可复用操作、身份、回执和原生取证；结果未知不能以新请求盲重发 |
| `internal/execution/workspace`、`publication`、`native_delivery_command.go` | Git/PR 能力可提取；Task 关闭及后继释放保留在 SteerLine |
| `internal/storage/ports/task_result.go` | ResultCommit 目前混合 Task、Workflow、Execution、Delivery；跨仓后改为 Runner 事实消费与 SteerLine 自有交付事务 |
| `docs/architecture/orca-native-worker.md`、`docs/spec/product-simplification-04-conversation-recovery.spec.md`、`docs/spec/worker-service-recovery.spec.md` | 普通最终回复、原来源核验、仅恢复原会话、停服后的人工继续门槛及后端能力差异需要保留；历史验收不能直接记作 Runner 验收 |

另一工作区的 `docs/spec/worker-three-layer.spec.md` 是衔接参考：Workflow → ReliableExecution → ExecutionAdapter，actor/存储为共同宿主。读取时 SHA256 为 `4bd58fa48ef2dba919fed095a5195b98e814e2a4a1c59997e778b34a121dac6e`，属于在途材料，不作为已合入的代码或固定规格。实施 Plan 必须核对最终产物，避免并行建立重复的旧/新运行时。

## 2. 产品边界与状态所有权

| 事实或能力 | 所有者 |
| --- | --- |
| 产品目标、Spec/Plan 编制、任务 DAG、前置资格、任务领取 | SteerLine 或其他调用方 |
| Task / Candidate 与 Run 的对应关系、候选选择 | SteerLine；Runner 的 correlation 仅为不透明关联值 |
| Run、冻结输入、workflow 步骤、交互问题、结果及 PR 绑定 | SteerRunner |
| Attempt、原生会话绑定、操作意图/回执、恢复与停止资格 | SteerRunner 可靠执行层，adapter 提供原生事实 |
| 具体原生会话历史与平台材料 | 相应后端；Runner 保存必要身份、可核验 final 和缺口 |
| PR 的实际存在、仓库/base/head、开放/关闭/合并事实 | Git / PR 平台；Runner 可查询、核验并返回事实 |
| 唯一 TaskDelivery、Task closed、后继释放 | SteerLine 自有事务 |

Runner 不读取任务图来决定准入，不领取或关闭 Task。Task ID、候选 ID、调用方 ID 可以作为 correlation 返回，不能据其值推导分支、资格或步骤。任意用户提交同一种 RunRequest 即可执行，不要求采用 SteerLine 的编号、Spec/Plan 文件命名或独立 acceptance 文档惯例。

每个 Run 只有一个 Runner 状态权威。Runner 的并发限制属于资源调度，不成为任务依赖系统。远端原生运行和连接客户端不另建可写 workflow 权威。具体存储可后续调整；CLI 和 SteerLine 不直接读写它。

## 3. 一个 EXE、服务与 CLI

`steerrunner.exe serve` 承载长期运行服务；其他业务命令是短进程客户端，经本机 IPC 向服务发请求，获得结果后退出。两种进程来自同一个 EXE，使用同一种执行机制。

| 角色 | 职责 |
| --- | --- |
| CLI | 解析参数/JSON，确定状态目录，发现服务、连接、发送请求、输出响应；不驱动业务步骤或直接写运行数据库 |
| 服务 | 唯一运行状态写者、命令接受、资源调度、后台执行、观察、对账、结果提交与恢复 |
| SteerLine 客户端 | 调用 CLI，解析 JSON/退出码，保存任务与 Run 关联，展示结果并应用自有任务规则 |

Windows 首版 IPC 使用命名管道。端点绑定规范化 `state-dir`、随机服务实例身份和当前用户；管道 ACL 限制为当前用户。端点发现、连接握手与目录写锁必须一致，不能仅凭 PID 或陈旧发现文件认定原服务仍有效。CLI 不能将请求改投到另一个目录或服务实例。

同一规范化目录最多一个服务写者。并发启动对账同一实例，不生成两个宿主；别名/大小写路径指向同一实际目录时也不能绕过锁。启动失败或实例失效返回明确错误，未证明旧写者结束不得接管。每个状态目录的服务独立管理其 Run。

调用方可在 resident 业务/查询命令上传 `--expected-instance-id <id>`：该预期实例在发现端及服务受理端核验，不匹配返回 `stale_instance` 且零业务派发；绑定的 submit 不隐式启动服务。`capabilities.instance_binding.expected_instance_id: true` 公布支持。version、serve、service start/status/verify-stopped 不接收此参数；service stop 仍由原输入正文 expected_instance_id 约束。

`submit` 和显式 `service start` 可在服务不存在时启动同目录的后台服务；后台启动不弹可见窗口。服务必须脱离 CLI/调用方的生命周期和会随其退出而结束的 Job 所有权；不能仅创建一个继承父 Job 的子进程就声称常驻。无法独立启动时在接受 Run 前明确失败，允许用户从独立入口启动同一个 serve 宿主，不增加另一种执行机制。`inspect`、`events`、`last-message`、`reply`、`resume`、`stop` 不隐式启动服务，服务不可用时返回 `service_unavailable`。`version` 不需要服务；`service status` 仅核验发现/进程/握手事实，不成为数据库读取后门。

Windows 实现先以原生 breakaway 挂起创建并检查 Job。创建被禁止时拒绝；创建成功但仍有关联时，可使用系统自带 Windows PowerShell/CIM 创建挂起进程，核验用户 SID、完整性级别、授权组/权限与 EXE，不能提升权限。系统能力缺失或核验失败明确拒绝。启动意图先保存；原进程创建结果无法核清时，状态/错误为 `startup_unverified`（exit 4），保留原意图，不盲目再次启动。手动 `serve` 的生命周期由独立宿主负责，`detached` 如实报告。

CLI 或 SteerLine 退出、IPC 断连不会自动停止已接受的 Run。首次输入明确接受后，正常服务实例可继续既定 workflow；客户端进程是否存在不改变其授权。服务停机后的规则见第 8 节。

`service stop` 是服务生命周期命令，`stop` 是指定 Run 的执行命令。停服请求携带 status 返回的 expected_instance_id，核验后才能作用于该实例；新请求不得停止已经换代的服务。停服接受回执不能证明所有原生执行已经停止；必须保存各 Run 的已确认/未核实处理事实。本机 Codex 的独立 owner 保留原生连接和 Job/工具树，服务只脱离观察；显式 Run stop 才停止原树，不撤销宿主登录。Orca 保留原观察脱离规则。相关契约和当前资格见[个人执行器简化 Spec](personal-executor-simplification.spec.md)；[旧服务换代 Spec](service-generation-recovery.spec.md)保留历史说明。Windows 系统服务注册与开机自启动另行设计。

服务在所有 worker 退出并读取持久化停止事实后、释放写锁前发布最终结算。`service status` 仅在写锁空闲且无未核实启动时公开 `last_shutdown`，绑定 `instance_id/request_id/state_dir/executable_sha256`，分别返回 `resources_settled` 和诊断；不读取或推导私有执行库。只有全部 Run 已持久化 stopped 且关闭无错误才标为 true；未知、unsupported、观察脱离、缺失证据不等于已停止。新写者获得写锁后首先移除旧结算，防止启动失败或异常退出后继续使用上一代证明。无记录不伪造成功。

显式 `service verify-stopped` 为停机后的独立只读取证入口：取得既有 writer.lock、拒绝未核实启动后，只读当前格式库，按逐 Run 冻结身份和范围返回结论。缺目录、锁、库、不支持格式、身份/权限未知、资源仍活动、清单不完整或未知创建均不能成功；不初始化服务/observer，不创建资源、不保存 stopped、不改原命令或 shutdown 回执。调用方只消费公开 CLI，不读取私有库。

Orca 查询原 runtime/worktree 的新鲜完整终端清单，计数须一致、未截断，原 local host 未省略；原 handle/incarnation 已退出或确实缺席才成立。不能用调用端 PID 推断远端。原生远端仅报告 `orca_terminal`；本机受管 Codex 按上文 scope 另查 helper 与 app-server 原根身份，即使已保存 stopped 也不跳过；签名退出文件和 control 关闭检查点均不作门禁。正常 history/Git 查询仍要求原 reader 及来源证据。直接 Codex 的 managed owner/精确 root 和 Pi/DSH 原生 helper 沿各自原证明，不能混用新 scope。普通停服完成观察清理后可按同一读取判据记录资源结论，不自动关闭业务或续发输入。历史全树停止取证设计见[旧 Plan](orca-shutdown-verification.plan.md)，新范围及验证以[本次验收](../acceptance/orca-local-codex-stop-simplification.md)为准。

## 4. 状态目录、内嵌资源与环境依赖

所有有目录上下文的命令使用 `--state-dir <绝对路径>`。独立使用时省略该参数，当前默认目录为 `%LOCALAPPDATA%/SteerRunner-v8`；SteerLine 等集成方须显式选择并维护自己的状态目录。CLI 参数优先，不能依赖调用时 cwd 或 SteerLine 环境变量改变目标，也不静默迁移显式目录。

建议布局如下；数据库和私有文件结构不属于调用方读写契约：

```text
<state-dir>/
  runner.db
  service/                  # 目录锁、实例与发现材料
  logs/                     # 服务及诊断日志
  runs/<run-id>/
    inputs/                 # 接受时固定的文档/流程/配置
    logs/                   # 本次运行的可取得日志
    artifacts/              # 报告、检查证据及资源引用
```

日志目录位于 `state-dir`。它不是用户代码工作区，也不因代理在本地/远端或切换 cwd 而改变。业务记录/操作 journal 是权威，日志和导出文件是诊断或投影，不能靠读取文本日志推进步骤。

内嵌资源包含默认流程、默认提示词、CLI 协议与内置后端配置 schema，以及必要程序资源。将 EXE 放入干净目录可完成 `version`、服务启动和协议查询，不要求附带模板目录、Python/Node 运行时或 SteerLine 安装文件。用户的定制内容接受后保存到本次 Run，不要求原提交文件在恢复时仍存在。

独立不等于零环境依赖。用户代码仓库、文档材料、Git、所选代理/Orca、账号认证、远端连接与平台访问是输入或环境依赖。缺失时明确拒绝准入或记录环境阻断，不伪造执行成功。首版不捆绑代理或复制 SteerLine 安装目录的数据库、认证和配置。

已知凭据字段不得进入请求、输入快照、普通日志或机器响应；配置使用认证引用，由对应后端在所在主机解析。拒绝含密码/token等嵌入凭据的仓库 URL；SSH URL/SCP格式中的登录用户名不是秘密，合法身份仍可使用。可取得的原生诊断按已知敏感字段脱敏；不承诺取得全部工具过程或识别任意模型正文中的秘密。

## 5. CLI 协议

机器模式使用 `--json`。首版协议标识为 `steerrunner-cli/v1`，可用 `--protocol steerrunner-cli/v1` 明确指定；省略时选择当前首版默认协议。`version --json` 返回程序版本、支持的 CLI 协议和内嵌资源版本，不启动服务。

### 5.1 命令面

下列均为待实现契约，所有业务请求均通过 CLI → IPC → 服务：

| 命令 | 输入与行为 | 成功响应 |
| --- | --- | --- |
| `version --json` | 离线版本查询 | 程序/协议/资源版本 |
| `service start --state-dir ... --json` | 幂等启动或连接同目录的同一 EXE 发行包；不同 EXE SHA/版本/资源摘要拒绝 directory_conflict，保留原服务 | 实例身份、目录、就绪事实、`executable_sha256` |
| `service status --state-dir ... --json` | 不自动启动；核验服务是否可达 | 实例身份与服务状态；不可达如实表达 |
| `service verify-stopped --state-dir ... --json` | 显式取得既有写锁后只读当前执行证据，核验原资源消失 | 目录、逐 Run 身份/输入摘要及资源结论；不修改原回执 |
| `service stop --state-dir ... --input - --json` | 幂等接受有界停服请求；保存处理意图后再执行，最终状态由 status/重启对账核验 | command 回执与已知执行处理事实 |
| `capabilities --state-dir ... --json` | 在线查询内置流程、adapter/config schema、平台能力与限制 | 能力版本和适用性；声明不替代现场事实 |
| `submit --state-dir ... --input - --json` | 固定 RunRequest 并接受异步执行；可按第 3 节启动服务 | 原请求的 command_id、run_id、接受状态 |
| `inspect --state-dir ... --run-id ... --json` | 查询已提交的同一快照；也可用 --request-id 或 --command-id 定位原命令，三个定位参数必须恰选一个 | 原命令回执及关联 RunView；提交丢响应时不需要先知道 Run ID |
| `events --state-dir ... --run-id ... --after ... --limit ... --json` | 有限分页，省略 after 从起点读取；limit 默认 200，允许 1–500 | 事件页、下一游标、是否还有数据 |
| `last-message --state-dir ... --run-id ... --json` | 只读进程内最佳努力活动摘要，不启动原生历史读取或新输入 | run_id、attempt_id、message（无样本为 null）；message 包含 Run/Attempt/Session/Turn、kind、text、observed_at |
| `open-native-session --state-dir ... --run-id R --attempt-id A --session-id S --generation G --json` | 不隐式启动服务；在原 worker gate 内验证当前身份、原进程及只读 thread/workspace，再开独立 Codex CLI 窗口 | 同一 Run/Attempt/Session/Generation 及 opened；仅说明窗口已创建，响应未知不自动重放 |
| `reply --state-dir ... --run-id ... --input - --json` | 回复指定问题或提交业务反馈，保存后异步对原目标执行 | command 回执；保存、投递、回合开始分别查询 |
| `resume --state-dir ... --run-id ... --input - --json` | 对当前恢复状态给出新授权，仅在资格满足时继续原会话 | command 回执；实际结果另查 |
| `stop --state-dir ... --run-id ... --input - --json` | 先持久化撤销本 Run 新输入授权，再按真实能力停止原执行；区分接受、授权撤销、实际停止与未知 | command 回执；实际结果另查 |

`--input -` 从 stdin 读取一份 UTF-8 JSON，不从命令行字符串传完整文档或秘密。请求最大 8 MiB，超限/非法 UTF-8/重复键/尾随第二对象拒绝；核心字段采用精确名称并拒绝未知字段，adapter settings 根据其版本化 schema 验证。人工 CLI 可以把文件重定向到 stdin，文件只在提交当次作为输入。

stdout 恰好输出一个 UTF-8 JSON 对象及换行；所有诊断走 stderr。机器模式不出现交互提示、颜色、进度条或原生工具输出混入。`events` 首版使用有限查询页，不要求长期订阅进程。原生程序无法启动等发生于 EXE 执行之前的宿主错误，由调用方作为调用失败处理。

### 5.2 请求与响应信封

除按目录幂等的 `service start` 外，所有写命令的 stdin 使用同一信封；操作既有 Run 时 run-id 由参数指定，input 中的绑定必须与之相符。查询命令和 `service start` 无需 request_id，响应中的 request_id 为 null。

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "caller-request-001",
  "input": {}
}
```

```json
{
  "protocol": "steerrunner-cli/v1",
  "ok": true,
  "request_id": "caller-request-001",
  "data": {
    "command_id": "runner-command-001",
    "run_id": "runner-run-001",
    "receipt_status": "accepted"
  },
  "error": null
}
```

协议/参数中的版本必须一致。成功查询的 request_id 为 null；失败响应使用 `ok:false`、同一协议及原 request_id（尚未解析则 null），`error` 至少包含 code/message，data 保存可核实的关联值或为 null。任何失败都不能造出 accepted 回执。

| 退出码 | 含义 |
| --- | --- |
| 0 | 命令成功查询或已持久化接受；不等于 Run 成功或动作已完成 |
| 1 | 内部错误，无法形成正常命令结果 |
| 2 | 参数/请求/schema 非法，尚未接受 |
| 3 | 协议、格式、命令或必要能力不支持 |
| 4 | 服务/环境不可用，且能证明本次请求尚未接受 |
| 5 | request_id 内容冲突、目标不存在、状态版本过期、身份或目录冲突 |
| 6 | 无法核实请求是否接受或外部动作是否发生，必须按原请求对账 |

超时或断连后不能一律返回 4。无法证明未接受时返回 `request_outcome_unknown` 与退出码 6。进程被杀、无输出或损坏输出也由调用方视为未知，不据此生成新 ID。CLI 有有界通信期限，不能等整次模型执行才返回；超时取消本次等待，不撤销已接受的操作。

### 5.3 幂等、并发与事件

request_id 是调用方生成的非空不透明字符串，以规范化状态目录为作用域，写命令之间共同去重。同 ID/同命令/同目标/同提交正文返回原 command_id、run_id 和当前原回执；同 ID 携带不同内容返回 `request_conflict`。摘要基于规范化提交值，不能在重试时用新版默认配置重渲染后创建另一执行。

`reply`、`resume`、`stop` 的 input 携带 inspect 返回的 `expected_revision`；问题回答还携带 question_id。`resume` 仅表达继续当前已展示恢复状态的明确授权，不接受更换会话/后端参数；`service stop.input` 必含 expected_instance_id。旧问题、旧授权或版本冲突明确拒绝。同请求的幂等重试先返回原回执，不因 Run 已推进而重新判为新命令。

接受回执的 receipt_status 和动作实际处理结果分开。inspect 返回的命令至少含 request_id、command_id、command、run_id（无关联则 null）、receipt_status、action_status、error；action_status 为 pending、running、completed、failed 或 unknown。completed 表示该命令动作已结束，不代表整个 Run/任务完成。停止、答案投递、恢复的原生处理事实另附，不把 accepted 自动改成动作成功。

`inspect --request-id` / `--command-id` 返回原回执及关联 Run；查不到返回 `request_not_found` / `command_not_found`。服务恢复对账尚未完成时，不能据暂未查到断言旧外部动作不存在；对已提交请求的可核实“尚未接受”与恢复中的未知分别表达。没有 Run ID 的 submit 重试仍使用原 request_id 和原正文。

事件拥有稳定 event_id、run_id、持久游标、类型及必要关联。游标不透明，客户端不能用时间或日志行数替代；错 Run 或无效游标明确拒绝，不能悄悄从头读取。分页/断线重读允许重复交付已提交事件，调用方按 event_id 去重。无新事件返回空页和可继续查询的游标。旧 Attempt/步骤结果可以记为历史事实，但不能推进当前步骤或污染其他 Run。首版保留持久事件，不自动裁剪游标对应范围。

## 6. RunRequest 与输入冻结

`submit.input` 的公共结构：

| 字段 | 要求 |
| --- | --- |
| `title` | 非空任务标题，不参与资格推导 |
| `correlation` | 可选对象，值为不透明字符串，如 caller/task_id/candidate_id；无 SteerLine 固定字段要求 |
| `name_prefix` | 可选通用资源命名前缀，1–48 位小写 ASCII 字母/数字，以单个内部连字符分隔；拒绝显式空值/null。与 correlation 独立，Runner 不解析业务编号。缺省有效值为 sr；Orca worktree 使用 `<有效前缀>-<完整 Run ID>`，创建与回执核验共用冻结值，分支规则不变 |
| `repository` | 必含 url、完整 base_revision、remote、base_branch；可选 local_path 作为本机已有仓库入口 |
| `documents` | 必含 spec、plan 的仓库相对 path；可选 references 数组；接受时固定路径 |
| `workflow` | 必含 id/version；当前支持 development/2、development-audited/2，可选 prompt_overrides；精确支持矩阵见 capabilities |
| `execution` | 必含 adapter、target、settings；target.kind 为 local 或 orca_environment，后者必含 environment 标识；settings 按内嵌后端 schema 固定 |
| `publication` | 必含 provider、repository、base_branch、settings；首版复用 command/gitea provider 配置及实际身份检查 |

base_revision 为完整 40 或 64 位 Git commit ID，不能仅保存可移动分支。local_path 是入口而非结果工作区；Runner 验证其 remote 与固定 url/目标一致，再为本 Run 准备独立工作区。保留调用方仓库未提交文件，不能 reset 用户现场。publication.base_branch 必须与 repository.base_branch 一致，provider 仓库与 Git remote 身份必须匹配。

接受新提交前，Runner 在私有缓存中从固定 remote 显式刷新 base_branch，再验证原 base_revision 属于该目标分支；local_path 的本地分支和已有缓存引用不作为最新远端事实。刷新失败则在接受前报告环境不可用，不能使用旧引用放行；远端前进不改变冻结基线，远端回退排除该基线时仍拒绝。此规则不重新解析已接受 Run 的固定输入，验证见[分支刷新 Plan](repository-base-refresh.plan.md)。

开发流程的每份文档只需提供安全的仓库相对 `path`，例如 `{"path":"docs/spec/task.spec.md"}`。spec/plan 的 path 必须不同；references 可引用验收或公共约束，不要求固定命名或目录。拒绝绝对路径及越界路径，此为输入形状校验，不检查实际文件或链接目标。

Runner 在指定 `repository.base_revision` 上创建实施 worktree 后，即认为所引用文档存在。文档是否已经提交、推送及是否在该 commit 中可读由调用方负责；Runner 不读取文档、不检查存在性、摘要或独立文档 commit，不获取或传输文档，也不提供缺失时的复制、内嵌或其他补救机制。代理遇到缺失文档时按实际执行情况反馈。

服务在返回 accepted 前持久化规范化请求、文档路径、实际流程/模板/配置及版本摘要；文档正文不形成开发流程的独立文件快照。为兼容既有请求形状，仍接受合法的 `kind:inline/content`、`kind:git/commit` 字段，但仅作为原请求数据保留，不作为执行材料来源，也不解析或获取对应 commit。新调用方应只提交 path。`conversation`、`generate` 的 inline 用户材料及快照机制保持不变。

本地 Codex、Orca 本机和 environment 的执行提示词均只引用 worktree 内的仓库相对路径；实施反馈、审计和报告沿用这些路径。公开 materials 使用 `kind:worktree`，commit 为执行基线，execution_location 为仓库相对路径，不提供独立 digest/snapshot_path；这些字段不表示文档存在性已核验。恢复和新阶段派发不依赖文档快照文件；已冻结的旧阶段输入保持原样，不在恢复时重渲染。Orca 只对实际渲染的单份提示词应用 12000 UTF-16 单元限制，正文大小不再计入独立材料额度。Orca 仍要求已注册项目的 local_path，environment 按原适配器规则解析项目并核验固定 base。更换文档路径、workflow 或后端应提交新 Run。

RunRequest 示例（占位 commit 应替换为实际完整 commit；这是形状示例，adapter/provider settings 的完整 schema 由内嵌能力提供）：

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "demo-submit-001",
  "input": {
    "title": "Implement the fixed Spec and Plan",
    "correlation": {"caller": "steerline", "task_id": "task-A"},
    "repository": {
      "url": "https://github.com/example/project.git",
      "base_revision": "<full-git-commit>",
      "remote": "origin",
      "base_branch": "main"
    },
    "documents": {
      "spec": {"path": "docs/spec/task.spec.md"},
      "plan": {"path": "docs/spec/task.plan.md"},
      "references": []
    },
    "workflow": {"id": "development", "version": 2, "prompt_overrides": {}},
    "execution": {"adapter": "codex-app-server", "target": {"kind": "local"}, "settings": {}},
    "publication": {"provider": "gitea", "repository": "example/project", "base_branch": "main", "settings": {"base_url": "https://example.com"}}
  }
}
```

首版 adapter ID 为 `codex-app-server`、`orca-terminal`；Codex 目标只支持 local，Orca 支持 local/orca_environment。`capabilities` 必须返回 settings schema 标识、版本和 schema 正文；程序内嵌相应 schema/默认值，实施 Plan 补齐模型、启动/权限、环境、超时及认证引用的具体字段，不把 settings 当作任意透传脚本。缺少必填配置明确拒绝，未知键不静默忽略。

配置 schema v1 的公开字段如下。默认值在字段省略时适用；字段出现时不得为 null，executable/model/reasoning_effort/wait_timeout 不得为空字符串。参数均为 argv，不经 shell 拼接。

| 配置 | 字段及默认值 |
| --- | --- |
| Codex settings | `executable` 默认 `codex.exe`；可选 `model`，省略继承工具配置；`reasoning_effort` 默认 `medium`，枚举 minimal/low/medium/high/xhigh/max/ultra；`access_mode` 默认 `sandbox`，可显式选 `full_access`。两种模式均为 approval never，不接受额外权限覆盖参数。GitHub worker 使用执行主机 gh 单仓库代理；`github_auth` 含 null 均返回 invalid_input |
| Orca settings | `agent` 默认 `codex`，本期受管 agent 包括 `codex`/`pi`/`dsh`；`access_mode` 默认 `sandbox`，可显式选 `full_access`，冻结后按对应模式准备和核验，不作失败回退。未知 agent/模式组合以 capabilities 和实际准备资格为准 |
| Orca settings.transport | `executable` 默认 `orca.exe`；`args` 默认空数组；`wait_timeout` 默认 `30s`，Go duration，须大于 0 且不超过 1 分钟。args 不得覆盖 environment/pairing-code，路由来自冻结 target |
| Orca settings.agent_launch | 原生 `executable` 按 agent 默认 `codex`/`pi.cmd`/`dsh.cmd`；`args` 默认空数组；可选 `model`、`reasoning_effort`。可信执行主机 helper 使用 `runtime_executable`；`runtime_sha256` 为 64 位小写十六进制，schema 允许省略，但正式验收及本期可核验的远端闭环/停止配置须冻结其摘要。transport 与 agent_launch 不混用 |
| command provider settings | 必填 `base_url`、`query.executable`、非空 `query.args`；可选 `authentication_refs`（环境认证的引用名称数组，不是凭据值）。实际查询协议/身份沿用提取的 provider |
| gitea provider settings | 必填 `base_url`；可选 `authentication_refs`。不接收内嵌 token/password 字段 |

所有字段精确命名、拒绝未知键；已知凭据参数不得放在 args。默认 schema 被内嵌并可查询不等于后端已 ready；阶段版本必须在 capabilities 中明确 execution 状态，未接通执行时拒绝提交而不持久接受虚构 Run。

## 7. 默认流程、提示词及查询结果

### 7.1 Workflow

当前内置定义为 `development/2` 和 `development-audited/2`，默认前者；保存冻结定义及摘要，不提供外部 workflow 装载。具体外层/内部状态、提示词和核验规则分别在[development](../workflows/development.md)、[development-audited](../workflows/development-audited.md)维护，统一结果契约、来源和预算见[结果获取 Spec](result-acquisition.spec.md)。本文不复制业务状态机。

普通执行不统一要求 JSON；程序提取可信 final 末尾成功声明或完整机器结果，必要时依定义选取独立解析或一次原 agent 报告。development 不调用 parser；审计版仅代码审计结果允许 parser，实施与发布同普通开发。只做一次代码审计并直接修复 P0–P2、验证、报告数量及逐项内容，无 Spec 审计或自动返修循环。发布结果仍须实际 provider 及 PR 身份核验；审计版另绑定修复后提交，解析成功不代替业务完成。

`prompt_overrides` 只覆盖 capabilities 公布的非空提示正文，不能改变绑定的文档路径、来源绑定、结果契约、成功声明、权限、预算或 PR 归属。普通四键为 `implement.execute`、`implement.report`、`publish`、`publish.report`；审计版另有 `audit.code`、`audit.code.report`。未知键拒绝，`audit.spec` 不保留。执行/报告共用原会话；独立 parser 使用 Runner 自有配置及独立会话，未选择 parser 不做其原生环境探测。提示词的工具约束和实际原生权限保证分别维护，见结果获取 Spec §6。

### 7.2 RunView 与交互

inspect 至少返回 run_id、revision、correlation、固定输入/配置摘要、workflow/step、当前执行/Attempt 来源身份摘要、Run status、new_inputs_allowed、原生结局、结果可用性、command 回执、交互问题、恢复/停止资格及已知 PR/result。内部数据库行或不透明 adapter 材料不是公共 DTO。

| Run status | 含义 |
| --- | --- |
| queued | 已接受，等待本 Run 准备/资源调度 |
| running | 正常准备或某步骤执行中；细节在 phase/step 和原生事实中 |
| waiting_input | 有绑定的业务或原生问题，需要新回复 |
| blocked | 已知环境、权限、前置或业务问题；给出原因与可用动作 |
| reconciling | 原操作、活动或结果未知，正在只读核验；不是可以重发的许可 |
| pr_ready | 本次 PR 已核验就绪，达到执行交付里程碑；当前两份定义不提供反馈返工边 |
| stopping | 已接受停止意图，实际停止尚未全部确认 |
| stopped | 已核实本 Run 的活动执行停止；现场与历史保留 |
| failed | 本次执行明确失败，保留失败来源；是否可恢复由资格另行说明 |

原生结局、完整 final 是否可取得和业务结果分别记录，不用单个 success 混合表示。缺失/截断/冲突的 final 明确标 gap，不能取屏幕、最后文本或模型自述填补。

`reply.input.kind` 为 `answer` 或 `feedback`，均含 expected_revision 和 text。answer 必含当前 question_id，核验该问题的原 Run/Attempt/回合/call；保存答案不等于投递或新回合开始。feedback 仅在当前定义允许的新实施入口及既有控制资格下接受；当前两份定义的 pr_ready 明确拒绝。原生问答继续绑定原请求，技术 resume 不授权重做已完成业务，不暗中排队未来授权。

PR 就绪不会自动销毁会话/工作区。反馈在同一 Run、原会话、原分支和同一 PR 上进行；保留以前结果，创建新的输入/步骤记录。同一 Run 返回另一 PR URL 时拒绝替换绑定。原会话不能安全继续时保留现场，调用方可提交新 Run，不隐式改投后端。

## 8. 操作可靠性、恢复与停止

外部副作用前先提交固定操作意图和原目标；意图提交失败时零执行动作。外部调用不进入数据库事务。回执/结果、相应步骤转移与下一输入意图一次提交；持久化失败后只核验原操作并补提交，不重复副作用。

request_id、Run、Attempt、操作、会话/原生回合和原生资源身份分别绑定。连接丢失、超时或进程退出不能直接认定原操作失败或原轮次成功。对 unknown 原操作只读对账；不能换 ID 再发、重新建会话/工作区或自动发布另一个 PR。

兼容服务重启先恢复观察和原操作/结果对账，保留已持久化的流程授权、暂停、人工等待、业务阻断和停止状态。原执行可以在服务离线时继续并保存结果；新服务补收原事实后，只能接续原授权范围内尚未派发的输入/后继。已有副作用意图只对账，不重复发送或重建执行资源；旧 epoch 且无原执行范围绑定的答案/反馈仍需人工确认。

`resume` 是当前恢复状态下的新显式授权，不是盲重放。仅在原身份/操作状态核验到足以安全继续后执行；未知创建/输入沿原准备期限对账，没有共同期限时最多 30 秒；耗尽转 blocked/manual_intervention_required 并允许显式 resume。受理后最多 30 秒只读查询原操作，持久保存期限且重启/保存失败不续期；不 Execute、不创建新 Attempt/owner/终端或替代 session，不重发输入。真实原事实解决未知且原资源有效时才解除技术暂停；仍未知、资源不可达或冲突则本次 action_status=failed，保留待人工，可用新 request_id/当前 revision 再查。同 ID 回放原结果。原撤销、业务等待和阻断保持，stop/close 优先；资源丢失后由用户处理现场或显式创建新 Run。

连接重接与同构建重启只接续原身份、原授权和原流程，不因此增加一次业务输入；既有人工门槛保持。升级须先由原版本排空原生资源及全部未完成副作用/控制，pr_ready 或 service stop 均不等于排空；verify-takeover 已删除，不提供跨构建活动任务接管。此次 v7→v8 使用新目录；未来格式/执行契约兼容且已排空时可继续原目录并查询历史，不因 EXE 字节或路径变化强制换目录。其他新 Run 的合法 submit 不被一个旧目标的不可达阻断。

`stop.input` 含 expected_revision。接受 stop 的同一事务先保存 `new_inputs_allowed:false`，使未发送的准备、步骤、答案和恢复意图失去投递资格；随后才按后端真实能力停止原执行。已经发送的动作不能撤回，不宣称所有原生工具立即停止。迟到结果可核验和保存，但不得自动产生或发送下一输入。

后端原生停止 unsupported 时仍保留这次授权撤销，命令停止部分记录 failed 及明确能力缺口，Run 转 blocked；实际活动未知时命令停止部分为 unknown，Run 保持 reconciling；不能长期伪装为正常停止进度。请求已接受、新输入被禁止、原生停止已确认、本地观察停止分别记录。不支持原生停止的 Orca 目标不能返回 stopped；保留活动/未知事实。只操作本 Run 精确拥有的资源，不能根据模糊 PID、路径或终端名结束其他执行。

授权撤销跨服务重启保留，业务反馈/旧答案不能绕过；只有符合原身份/旧写者/未知动作门槛的新 resume 授权才能重新允许输入。非法或过期 stop 在接受前拒绝，同请求重试返回原撤销回执。调用方 Task 终态后不会发这种重新授权。

stopped/failed 的后续 resume 仅在上列资格满足且仍有原会话时可用；历史失败/停止记录不能覆盖成从未发生。首版不自动删除工作区、分支或 PR；原生清理失败另记，不撤销已核实 PR 结果。

## 9. PR 成果与 SteerLine 协作

开发工作流必须显式选择 `full_access`，采用执行主机已有的 Git/gh/SSH 登录；不提供默认开发 sandbox、单仓库认证代理或认证租期。原生权限参数及启动条件仍按具体后端核验，不以模型声明代替已有程序检查。内容流程的 read_only 和 parser 自身约束不随开发权限简化而取消。现行规则见[统一权限](development-full-access.spec.md)、[宿主认证](host-native-auth.spec.md)及本文 PC-C06；旧[sandbox 认证设计](sandbox-authentication.spec.md)只供历史追溯。

原生依赖的会话历史、缓存和临时目录不成为 Runner 恢复权威；Runner 自身状态、日志及冻结材料归 state-dir。未知发布继续核实原操作，不因权限放宽而重新推送或创建 PR。

Runner 按冻结 publication 配置和当前实际工作分支推送，创建或复用该分支 PR。模型返回 URL 后，provider 需核验平台实例、仓库、base/head 仓库及分支与 Run 的绑定一致，才能进入 pr_ready。URL、仓库、number、查询状态和可取得的 revision/时间及诊断作为 PRFact 返回。

PRFact 记录 Runner 在 Workflow 中确认的 PR 绑定和观察。普通及离线 inspect 读取持久快照。实时合入状态由调用方直接向 PR 平台查询，按原 Run 的仓库、分支和 PR 绑定复核。Runner 自主核验发布及适用的已审计 head；TaskDelivery 归业务调用方。

发布响应未知时查询原配置/分支是否已有 PR；无法核实就保持 reconciling。发布补报不能再发布。不能用报告 done、CLI 退出 0 或任务测试通过代替真实 PR 存在/归属核验。

SteerLine 接入流程：

1. 在自己的事务中固定任务资格、候选及待提交请求 ID/正文；经 CLI submit。同请求重试返回原 Run，Runner 不解释领取。
2. SteerLine 幂等保存候选与 Run 的关联；响应丢失时用原请求对账，不能新增候选规避未知。接受之后调用方失败不使 Runner 回滚已执行的副作用。
3. 经 inspect/events 获取已提交快照，按 Run/request/event 关联和游标去重。SteerLine 只保存自身需要的事实，不复制另一套可推进的执行状态机。
4. PR 实际合并后，SteerLine 依据固定 provider/PR 绑定核验最新平台事实，在自己的事务中写唯一 TaskDelivery、Task closed 和后继释放。可消费 Runner 查询能力，但必须核验事实来源、完整性和归属；通知不是交付证明。
5. Task 关闭/丢弃后的执行处理由 SteerLine 发显式 stop；Runner 不订阅任务数据库。调用方先持久化待 stop 意图、按原 ID 对账直到得到真实处理结果。其客户端停止新回复/恢复，界面如实显示仍在运行或未知的原执行。只有 Runner 已提交授权撤销回执后，才能证明它不再派发新输入；Task 终态与这份回执之间有通信窗口，调用方不可显示全系统已停。TaskDelivery 不与远端停止组成跨库事务，停止失败不撤销真实交付。SteerLine 对 closed Task 停止 PR 合并查询，待 stop 对账独立进行。

SteerLine 使用单独的 CLI 客户端模块；每次启动短进程，传参数与 stdin，解析一个响应并检查退出码，不持有原生代理或 IPC 私有材料。目录内虽有两份数据库，但分别为不同领域的权威，没有跨库事务、双写或默认读取 fallback。

## 10. 代码归属、版本与交付安排

SteerRunner 内部分工为公共 CLI/DTO、服务与存储、Workflow、ReliableExecution、ExecutionAdapter、共享 workspace/publication。Workflow 不解释技术投递/身份；可靠执行不解释 Spec/Plan、业务步骤或 TaskDelivery；adapter 不持有业务写权限。通过独立宿主组装原子提交和副作用执行。

两仓各自构建、测试、发布；SteerLine 固定一个明确 Runner 发布版本及 `steerrunner-cli/v1`。首版开发期拒绝不支持的存储/配置/协议格式，在任何业务动作前报错，不读取旧 SteerLine 数据库、不迁移或双写。升级/安装不自动接管正在运行的旧服务，二进制运行中不强行替换；完整版本管理另行设计。

代码抽取实施 Plan 必须记录固定来源 commit、迁出/保留/删除清单、三层重构实际交接、Runner 独立验证及 SteerLine 切换 PR 的最小测试。切换 PR 接通客户端并删除原执行实现，合入前固定已经可取得的 Runner 发布版本；临时开发复制不形成长期双维护路径。

首版 Windows/amd64 安装产物为一个 EXE；CLI 查询实际程序/协议/资源版本。其他 OS/架构未验证时不宣称支持。原生 adapter 和 provider 的 required/unsupported/unverified/not_applicable 声明及现场核验保持区分，不把未知、零值或提示词宣称当成 ready。

## 11. 验收与本次文档交付

自动验收 SR-A01–SR-A12 与人工项 SR-M01 统一见[专属验收](../acceptance/steerrunner.md)。只验证本次独立化及实际支持能力；受控/native/live 证据分开，旧 SteerLine 验收不搬成 Runner 通过结果。不默认跑 Full 或完整真实模型矩阵。

当前文档阶段交付 README、此 Spec、实施 Plan 和专属验收文档，并核对链接、示例 JSON 和覆盖关系；不复制源码、不运行服务或真实模型、不创建/合并 PR、不修改 SteerLine 或安装环境。运行验收入口尚未存在，Plan 标明待建设的真实场景/命令、建设步骤与执行时点；先落实能力，再执行并登记结果。

## 执行可见性

`workflow_changed` 事件在 Run 提交事务内比较前后值，仅 workflow stage/status/diagnostic 发生变化时追加。`transition.from/to` 各含 `workflow_stage`、`workflow_status`、可选 `diagnostic`；外层保留原 event_id、Run/Attempt、revision、created_at。初次提交 from 为空。不用当前 Run 重画缺少 transition 的历史。

`last-message` 返回的 `observed_at` 为原生样本被本机观察源收到的 UTC 时间，不是调用该 CLI 的时间；同一摘要重复读取时间不变。采样本地 Codex completed agentMessage、固定工具标签，以及远端原生 Orca Codex 的可归属公开 assistant 字段；Orca 后台查询样本使用本机收到时间，不冒充远端消息创建时间。不暴露 reasoning、工具参数/命令/输出或用户输入。Run/Attempt/Session.Generation/Session/Turn 验证与脱敏后，每 Run 只保存一条 1200 UTF-8 字节加省略号的摘要。乱序/过期样本丢弃，不推进 revision/events。服务重启缓存清空，仅由之后的正常合法观察产生新样本；空值与读取错误都不能用来判断失败、卡死或恢复资格。Orca 字段发布时机属于最佳努力，不保证中途或逐 token 消息，具体来源及空结束 prompt 约束见[专项 Spec](orca-last-message.spec.md)。

客户端按当前显示的执行按需查询；SteerLine 在页面可见且在线时每 3 秒读取，后台/切换/离开时停止。具体边界与最小验证见[增量 Plan](execution-observability.plan.md)和[验收](../acceptance/execution-observability.md)。


### Orca 原生缺失 outcome 的结果契约

按 [远端原生 Spec](orca-remote-native.spec.md) 的用户确认增量，native_outcome 现有字符串扩展 unreported（接口未提供）。仅原请求正常结束、完整来源成立且无 Failure 时可进入既有业务核验；它不等于原生 succeeded。显式 unknown/failed/cancelled 和未知输入继续阻断，历史 unknown 不重新解释、不自动推进。CLI v1 结构、工作流/审计/PR 与停止部署门不变。

单次执行的完成回执与业务成功判定分开。对于 `execute/1`，原回合已核实结束且最终回复来源完整时，缺失原生分类的 `unreported` 可以进入关闭和停止结算。Runner 保留该分类并在停止核实后公开 `closed`。调用方校验其正式结果并决定业务是否通过。详见[单次执行的完成回执](../workflows/README.md#单次执行的完成回执)。
