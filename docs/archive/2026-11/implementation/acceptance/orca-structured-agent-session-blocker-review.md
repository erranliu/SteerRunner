# Orca 结构化会话：前三项阻断再调查与最小解阻方案

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

**后续约束及范围修订（2026-11-03）：用户明确不修改 Orca，并确认首 PR 仅迁本机 Codex 新 execute/1，接受资源停止证据不足明确未核实；DSH 执行及专用历史/用量支持本批一并删除。** 本文调查事实保留，但“上游补充事实”不再是授权路径，完整停止证明/audited 全量资格不再阻断该窄范围。当前合同以 [Spec](../spec/orca-structured-agent-session.spec.md) / [Plan](../spec/orca-structured-agent-session.plan.md)及[本轮设计复审](orca-structured-agent-session-mvp-design-audit.md)为准；未实现代码。其它组合/原证据不追认通过，远端 Pi 不安装。

日期：2026-11-03。范围：延续 [P0 核验](orca-structured-agent-session.md)，只读调查安装合同及 Runner 生产调用者，不实现运行代码、不增加模型/RPC 调用、不安装远端 Pi。原始 P0 失败证据保持原字节；本报告细化初轮归因，不追认 P0 通过。**B01 可在 Runner 设计内解阻；B02 的公开原会话资源停止证据、B03 的逐回合实际模型来源仍阻断最终迁移。** 复用方案尚待实施验证，P1 未开始。

调查基线仍为 `457ffaea20b6227a0bdaec9e56f5b9e7f54da337`。本机安装 host chunk 的 SHA 与 P0 相同；[Orca 源码摘录及摘要](../../../../acceptance/evidence/orca-structured-agent-session-blockers-20261103/orca-source-findings.json)、[Runner 消费者摘要](../../../../acceptance/evidence/orca-structured-agent-session-blockers-20261103/runner-source-map.json)、[证据清单](../../../../acceptance/evidence/orca-structured-agent-session-blockers-20261103/manifest.json)固定本次依据。对远端的判断沿原公开回执，不声称本次核验了远端 unpacked 模块或新行为。

## 1. 分类结论

| 初轮阻断 | 实际发现 | 归属与最小处理 |
| --- | --- | --- |
| B01：旧 fence send 被接受、close 后可再运行 | 当前 admission **有意忽略** expectedRuntimeFence；conversationWrite 不要求现有 live child，新授权输入可触发 lazy acquisition | 先纠正 Runner 的 fence/CAS 假设。复用原 Run 授权、worker gate、一次派发 checkpoint、未知零重放及独占 session。无需让 Orca 永久封死会话，也无需为每个 send 新造 CAS 协议 |
| B02：精确停止及只读查证 | Orca 内部已有 provider 退出探测、lease deathEvidence、queue stop marks；cancel 也会尝试结束 provider。现公开布尔值/turn/history 不充分暴露原会话管理资源的退出与结算 | **主要是公开取证缺口，不是“Orca 没有停止实现”**。原 session 的 close 已有定向能力；复用 stop intent/只读 reconciliation，优先只补可查询的资源停止事实，不建通用 receipt 平台 |
| B03：audited 工作区来源 | 旧 helper 复用的本来就是固定 Git InspectAudit；WritersAbsent 是临时 Git reader 的结算 | 本机可由原 Orca adapter 在已核验执行主机/工作区上复用同一 Git 算法、managed reader 和原完成 checkpoint；不要求 Orca 提供新 Git 审计服务 |
| B03：audited 实际模型/effort | selection、submission.options 是请求/偏好；current options 可变；本轮 Codex turn contextUsage.model=unknown。安装适配器有 ran-options 的当前会话更新，未找到稳定的逐回合公开实际选项证明 | **仍有 Orca normalized 事实暴露缺口**。复用现有 Workflow 模型角色及校验，只在原 turn/history 补来源明确的实际模型/effort；不能改读私有 transcript 或用 requested 充当实际 |
| B04：远端 Pi | 原目标 createSupport=false、Pi 缺失 | 保持环境阻断；本轮零安装、零创建，不混为结构缺陷 |

“可复用”是方案归因，不代表当前旧 adapter 已支持新协议。旧 helper 的身份载体会消失，不能伪造其 readiness/PID/签名，或把公共查询结果硬塞进旧证明格式。

## 2. B01：授权围栏由谁负责

### 2.1 Orca 的真实合同

安装 `shared/agent-session/lifecycle/agent-session-mutation-envelope.js` 的 `admitAgentSessionMutation` 注释明确写明：忽略 expectedRuntimeFence，因为写操作按目标身份或 last-writer-wins 处理，以允许 unseen owner restart；旧 host 才检查该字段。顺序为 fingerprint→ledger→lease；conversationWrite 可在此直接 admit。host send 计划也明确设置 conversationWrite=true。原 P0 两端旧 fence 接受结果与此一致，不再保留“可能是意外遗漏”的推断。

Orca **内部 acquisition** 仍有 expectedFence/CAS、owner process、死亡探测、provider handle chain 的单 writer 控制；这与外部 mutation envelope 的 expectedRuntimeFence 是不同合同。Runner 不应再复制这套 provider lease 管理，也不应把 provider handle 延迟出现/lease 自然变化当作新 Run 或进程身份。

### 2.2 已有 Runner 机制足以负责的部分

- [engine.go](../../../../../internal/application/engine.go)：接受 stop 时持久设置 Allowed=false、ResumeRequired=true、PendingStop；同一 worker gate 与控制命令串行。外部 expected_revision/request_id 仍由既有受理机制处理。
- [worker.go](../../../../../internal/application/worker.go)：saveIntent 校验原 revision/Attempt/binding 后提交原意图，invoke 区分首次执行与原操作 reconciliation。mutation 不在查询释放 gate 的路径中运行。
- [effect.go](../../../../../internal/execution/reliable/operation/effect.go)：持久 accepted/unknown/confirmed/rejected 状态不能通过 ExecuteNewIntent 变回新派发；原 request 身份不重建。
- [dispatch.go](../../../../../internal/execution/adapters/protocol/orca/dispatch.go)：WithDispatchCheckpoint/CheckpointDispatch 已提供一次性实际 I/O 边界；P1 把同一门禁接到 socket write 前，不另造授权账本。当前 CLI 门禁通过不等于 socket 已接通。
- [preparation_query.go](../../../../../internal/application/preparation_query.go)：只读等待可释放 gate，返回后必须重取 gate 并核对撤销/revision；查询成功不授予 mutation 权限。

最小方案是每个 Attempt 预生成并冻结一个专用 sessionId，禁止复用到另一个 Run；运行状态/原操作绑定继续沿现有持久容器。停止后 Runner 不发新的 send/answer/feedback/ensure/reopen，也不恢复 native 队列；原未知操作不重放。将 native fence 记录为 Orca lease 观察值，原请求值不可改写，回复中的 lease 变化只按原 session/原消息/原 turn 因果映射认领。

这能解决**Runner 自己停止后再次派发或重放输入**，并不提供“Orca UI/其它客户端永远不能再打开这个聊天”的承诺。当前产品也没有宿主凭据撤销或全客户端永久封禁承诺，无需因此新增 Orca tombstone、session token 服务、Run 级远端锁或通用 mutation CAS。并发外部更改导致原归属/消息完整性不可核验时，沿已有 conflict/unknown 门禁阻断认领。

已可能写出的原未知输入不因 stop 而被宣称从未发生；跨连接迟到、尚在 Orca 启动/派发队列的输入必须纳入 B02 结算查询。原输入无法定案时保持有界对账/人工出口，不能靠“本次没再 send”判资源已结算。客户端查后写也不能冒充服务端原子资源围栏。

**归类：B01 设计层可解阻，待 P1/P3/P4 实施和受控计数验证；不再单独要求 Orca 为所有输入补 strict-fence 拒绝。** 原两次负例仍是“该旧假设不成立”的失败证据。

## 3. B02：停止实现已有，公开证据仍不足

### 3.1 应纠正的初轮表述

P0 实测只取得 `cancelled=true` 与原 turn interrupted/cancellation，所以“证据只证明回合中断”成立；**“cancel 的实现只会中断回合”不成立**。本次读到 host cancel→conversation Stop 的路径，会写 stop marks、撤回待发输入、捕获原 child，并在适用路径调用 stopAgent/adapter disposal；异步尾部还会尝试关闭 provider。晚到的 targeted turn stop 会先做 current/late 判定，避免作为对新 turn 的取消授权。

host close→lifetime.close→stopAgent→adapter disposal 也不是 UI-only。Codex/Pi adapter 分别检查 root/tree 的退出结果，并可能报告 unproven；内部 lease adjudication 保留 ownerProcess、deathEvidence.ownerFence/observedAt。内部有这些判断，不等于公共 `cancelled` 或 `close.ok` 已返回其全部证明。

本轮没有重新操作会话或用 OS 进程表补证。原 P0 的 after-cancel fence 未变、turn 结束和 tabs 变化仍不足以判 root/workload 完整退出；也不能根据本地源码自动推导远端全部分支通过。

### 3.2 哪些要求可以删掉

**不必要求 close 也具有全套 mutation envelope 或全局 operation receipt 查询接口，才能谈停止。** Runner 已有持久 stop 意图、实际一次派发和未知只读恢复。原 mutation 回执丢失时，只要能只读核实原停止目标已达成，就可以确认资源目标；无需补造 cancel 成功回执或再发 close。缺证时保留 unknown/人工出口，符合现有合同。

也不需要强制 cancel→close 两阶段。优先让一条现有终止路径完成原 session/acquisition 的停止；cancel 是否已经足够、idle/startup 是否需另一现有终止动作，由公开合同和证据决定。不能为了名称统一在两者外再造 stop daemon、root owner、第二 ledger 或私有 process killer。

### 3.3 最小候选：原 session close 一次，再只读核验

Runner 的资源单元是原 Attempt 创建、不可跨 Run 复用的**逻辑 session**；Orca 管理它内部的 acquisition。最小候选采用已有 `close(sessionId)` 一次，沿原 stop 意图/实际派发门禁执行；不要求另开一个带全套 envelope 的终止 RPC。原活动 turn 的 cancel 仅在确有必要时采用，不能预置为强制第一阶段。合法 lease 更新不把会话变成另一个 Run；无法解释的 runtime/worktree/provider 身份变化仍冲突，不自动跟随。

**真正剩下的最小缺口是有界、独立的只读结算事实**：从现有 history/status 或同一原 stop 的持久行，区分原 session 所管理资源已退出/从未启动/不可核验，提供对应 owner/acquisition 的因果身份及观察时刻；结合已公开的 submissions/queue/turn/后台工作，证明本 scope 内无未结算启动、迟到输入或待派发资源。原 close 丢响应、runtime 重启及 lease 更新后必须仍能核验或明确 unknown。当前已查公开合同不充分提供这个正面退出结论。

以上是所需**事实语义**，不是已存在的字段名/API，不在本轮凭空实现新 endpoint。优先暴露 Orca 已计算的 lease deathEvidence/owner verdict 和原 child disposal 结论；完整性未知必须显式表达。无需让客户端重做 process probe。仅当后续确有证据要求“只能终止某一次 acquisition，连原逻辑 session 后续合法 acquisition 都不能触及”时，才评审窄的 acquisition guard；不把这个更强 CAS 作为当前默认前置，也不将客户端查询包装为该 CAS。只需覆盖登记的原会话/Run 辅助资源，不要求宿主所有进程/所有后代退出或凭据撤销。

已有 `agentSession.sleepWorkspace` 也被检查：内部会校验关闭前后的 fence/admittedSendCount，并判断 pending attach/delivery/child work，返回 stoppedSessionIds/failures。但它针对整个 workspace、不是单会话只读查证，缺少可复用的公开原 acquisition 停止查询；本轮未调用。**不将其硬接为 Run stop**，以免影响同工作区其它会话，也不以它的内部更强判断冒充 close 的回执。

**归类：Runner 停止授权/一次派发/未知恢复与 Orca 原 session close 均可复用；原会话管理资源的独立正面结算事实仍是 Orca 公开合同缺口。** 在找到充分的现有公开查询或上游补齐之前，`orca_agent_session` scope 不获资格。持久 stop=true 标签、tab 缺席、OS 扫描和修改安装包都不是最小解阻方案。

## 4. B03：工作区检查与实际模型分开

### 4.1 本机 workspace audit 可复用

[remote_workspace.go](../../../../../internal/execution/adapters/codex/remote_workspace.go) 的 InspectRemoteWorkspaceAudit 先用旧 helper readiness/PID/配置证明查询位于原工作区，再运行固定 Git；WritersAbsent 的更新来自 **auditGitHandle.Close 对本次 Git reader 的 settleRemoteTree**，不是“全部 Agent 写者已退出”。这是初轮容易过度扩大的一点。

[workspace.InspectAudit](../../../../../internal/execution/workspace/audit.go) 已完整实现仓库/common-dir/真实路径、branch/base、clean、head/tree、稀疏/隐藏 index/gitlink 拒绝、实现提交祖先关系及检查期间 HEAD 一致；[Git.Output](../../../../../internal/execution/workspace/git.go) 与 managed process 端口已有超时、输出界限和读取进程管理。无需将 Git 审计上移为 Orca 服务，也无需保留一个“不管理 Agent、却专门证明旧 helper 身份”的新 helper。

最小候选接线保持 [LifecycleAuditWorkspacePort 消费边界](../../../../../internal/application/workspace_source.go)：

1. 仅本机 Orca Codex audited 走此候选。原 adapter 通过已冻结的本机 IPC runtime、执行 host、原 session/turn/worktree ID/instance/path 及原输入关联确认检查目标；要求确认就是 Runner 可读的同一主机文件系统，不因字符串 `executionHostId=local` 就跨主机运行 Git。
2. 在原完成事实已归属、下一业务 mutation 尚未获派发的原观察/inspection gate 内，复用同一 InspectAudit 和固定 Git 参数/managed reader 结算；保留原 URL、common-dir、symlink、clean、head/tree、祖先关系校验。新协议用自己的真实 Orca 绑定，不伪造旧 RemoteReadiness。
3. 沿现有 originalCheckpoint / Workflow.Result.Workspace 及结果事务保存原完成检查。恢复只读原已保存的检查；缺少原完成观察或持久检查时保持 unqualified，不用恢复后的当前 HEAD 重建“当时”证据。

[application/workspace_source.go](../../../../../internal/application/workspace_source.go) 当前有意拒绝 native Orca 的 caller Git fallback。这条原规则继续有效；新 adapter 须显式实现并核验上述原 owner 端口，**不是删除拒绝分支后对任何路径直接调用 gitHost**。现有 direct Codex 的 signed owner checkpoint/reader 不迁移、不改写；只复用无 provider 所有权的 Git 算法及既有业务 checkpoint。

不新增远端 audited：当前远端无该支持项，不能为了四组合 development 目标搭建远端 Git 执行服务。新路径的主机错配/路径替换、临时 reader 未结算、错误 turn/缺 checkpoint、restart 后当前 HEAD 替代等仍须受控拒绝。

**归类：B03 的本机工作区部分可在 Runner 内设计解阻；新协议原 owner 绑定与事务接线仍待实现、测试，不是现有短探针已证明等价。**

### 4.2 模型角色复用，实际模型证明仍缺

[RemoteModelBindings](../../../../../internal/execution/workflow/remote_models.go) 已定义 implement/report/publish 使用执行模型、audit.code/audit.report 使用审计模型；[reliable.SameInput](../../../../../internal/execution/reliable/intent.go) 校验原 TurnInput.Model/ReasoningEffort 等冻结字段。继续将这些值映射到 firstMessage.selection/send.selection，不新增角色配置、模型路由器或模型命令头。

问题在**事实来源**。旧 [recorded turn 校验](../../../../../internal/execution/adapters/orcatransport/local_codex_history.go) 比较原 turn 实际 Model/ReasoningEffort；[InspectRemoteTurn](../../../../../internal/execution/adapters/codex/remote_turn.go) 对 audited 通过原 provider turn 的原生历史取得这些实际值。新链路不能继续读私有 provider transcript 或开另一个 App Server。P0 Codex 的公开 contextUsage model 为 unknown，submission.options 是输入选项；会话 options 的 saved/policy/confirmed、模型 catalog 或 usage 模型名均不能替代同一原 turn 的实际模型证明。

安装 host 的 ran-options 路径会更新**当前会话选项快照**，Codex normalized turn 构造包含 turnId/state/userItemId/时间/usage 等；所核对公开合同未给出稳定、逐回合的实际 model/effort 及报告来源。开放 schema 允许扩展也不证明 producer 已输出。仅切换两个模型并看到答案/当前 options 改变，仍不足以把“选择已接受”当作“原回合实际使用”。本轮不为此继续消耗模型调用。

最小 Orca 补充是：**在现有原 turn/history 事实里保留 provider 确认的实际 model/effort、报告来源，并绑定原 turn/user input**。请求值与报告值明确区分；未报告就 unknown，不能从 selection/default 兜底合成。Runner 仍用现有角色/原 turn 校验，字段足够则无须增加 provider 原始事件 decoder、第二查询服务或全量日志镜像。具体字段及 provider 能否报告需上游合同核验，不能预填通过。

另一个过度设计风险是误将“保持 report 工具策略”扩大为“新增原生禁工具隔离”。当前 Orca PolicyCapabilities.ReportToolPolicy 明确为 host_managed_permissions（原 native 分支为 configured_native_launch_parameters）加 **business_read_only_is_prompt_policy**。保持其既有提示策略和原权限事实，不新增 tools=[]、新 sandbox 或逐回合权限切换作为 audited 迁移前置；显式配置仍不能静默丢弃。Pi 同样不能借本批被强加 Codex 专用权限模式。

**归类：模型角色、输入冻结、阶段和原判定复用 Runner；逐回合实际模型/effort 的公开稳定来源仍是 Orca 合同缺口，保持 audited 门槛不变。**

## 5. 最小工作顺序与验收门槛

1. 本轮文档先纠正 B01 的 strict-fence 假设、B02 的 cancel 实现描述和 B03 的 reader/workspace 归因；保留原失败回执，最终迁移仍阻断，不实现代码。
2. 后续只针对两类原生事实补证：原 session 所管理资源的独立只读结算；原 turn 的 provider-reported 实际模型/effort。终止优先复用 close 一次。先确认是否有遗漏的既有公开查询；确实没有再提出窄的上游事实补充，不向运行中的 Orca 打补丁。
3. 获得充分原生合同后，P1–P4 复用现有 socket/IPC/远端连接、一次派发、stop intent/reconciliation、Workflow 模型角色、LifecycleAuditWorkspacePort、InspectAudit/managed reader 和 checkpoint。没有新运行期开关、第二后端、通用 receipt 中心或全局状态目录迁移。
4. 受控验证必须区分：Runner 停止后新 mutation=0、未知原动作重放=0、已可能写出原输入的结算未核实；自然 lease 变化合法认领与原资源身份冲突；终止前后换代/丢响应/重启/启动中/idle/队列；原 workspace 检查绑定与缺 checkpoint 拒绝；执行/审计两个不同模型的实际报告匹配及未报告拒绝。
5. 最后仍按原 P5 正式 Runner 取得四组合、原 audited/execute 和停止/离线资格；远端 Pi 先保留环境阻断，不安装、不撤下该目标，也不因此缩减既有 Workflow。非 Orca Codex/parser/owner/signed checkpoint 不变。

本轮未解决上游事实缺口，也未授予任何生产资格。[离线检查记录](../../../../acceptance/evidence/orca-structured-agent-session-blockers-20261103/validation-results.json)的 246 项来源/证据/文档核对通过，337 个本地链接检查通过；11 个 Orca 来源文件、17 处摘录、20 个 Runner 文件及原 P0 的 189 份证据逐一核对。新证据目录另有 SHA 清单；运行代码未修改，新增模型/RPC 调用为 0，未运行生产测试。上述检查不是运行资格。
