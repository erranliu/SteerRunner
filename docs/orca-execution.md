# Orca 执行链路设计

2026-11-06 实现收口：旧 `NativeProof` 仅保留原格式、历史严格解码及真实只读消费者，旧 hook/输入归属/完成补证计算器已移除；当前执行仍只走结构化协议。真机 driver 的当前接线及未取得资格的范围见[独立验收](acceptance/orca-structured-qualification-proof-cleanup.md)，不改变本页的执行、恢复和停止承诺。

本文维护 Orca 链路的长期设计、责任边界与已接受限制。实现范围以当前构建 capabilities 为准；具体变更由 Spec/Plan 描述，真实结果保存在独立验收记录，历史证据不随新设计改写。2026-11-03 用户确认的停止边界及对应修复见[本次验收](acceptance/orca-session-result-settlement.md)。

## 目标与职责

Orca 提供已有的会话、终端、工作区及执行主机能力。Runner 绑定原请求、原工作区、原会话和原回合，管理授权、Workflow、结果与持久恢复。SteerLine 只消费 Runner 公开的业务结果及带 scope 的停止事实，不再推断底层是否完成主机级清场。

本机和远端只改变明确的连接目标与仓库解析；不在失败后静默换主机、后端、session 或输入 ID。独立 worktree 提供任务文件隔离，不提供操作系统隔离。既有宿主登录仍由用户管理。

## 执行组合

| 组合 | 输入与结果 | 停止范围 |
| --- | --- | --- |
| 本机 Codex execute/1 | 原子 create.firstMessage，一条完整 user 消息；从公开 history/item 取得原 turn 的完整 assistant 文本 | `orca_agent_session`：原 session 的关闭成功回执，或原创建尚未派发 |
| 新配对远端 Codex execute/1 | 原子首消息及原 turn 完整 assistant 文本，按原会话/回合绑定公开 usage | `orca_agent_session`：按原会话关闭成功回执或原创建未派发结算 |
| 新本机/配对远端 Codex development/2 | 原子首消息及同一会话的授权续轮；原 turn 最后一条完整 assistant 消息继续交给现有 Workflow | `orca_agent_session`：原 session 关闭确认后，结束该 Run 的恢复授权 |
| 新本机 Codex development-audited/2 | 结构化原会话多轮；保留逐轮实际模型、工作区/审计 HEAD 和发布来源核验 | `orca_agent_session`：原 session 关闭确认；结果按原有快照合同封存 |
| 新本机 Pi execute/1、development/2 | 结构化原会话、现有宿主工具及原回合结果；Pi usage 未核实仍报告不可用 | `orca_agent_session`：原 session 关闭确认；保留 Pi 原有单次文件快照窗口 |

上表只列当前新 Run 的合法组合。旧 `agent_launch` executable、非空 args、helper 路径/摘要在受理前拒绝；传输端 Orca executable/args 与模型设置仍按现行配置契约处理，不把传输参数误作旧 Agent 启动入口。远端 Pi、远端 audited 和 DSH 不因此取得资格。依据[已实施 helper 退役](spec/orca-helper-retirement.spec.md)及[验收](acceptance/orca-helper-retirement.md)。

旧冻结终端配置不再是当前构建的执行选项：服务启动及执行 port 均拒绝旧协议。旧 Run 必须由原构建完成/停止并排空，新构建不接管或重发原输入；保留当前格式旧记录的离线 inspect、既有只读 scope/停止证明和严格解码，不能删除证据或按新 scope 重释。历史 `orca_terminal`、`orca_local_codex_resources` 的含义仍分别是原终端、登记终端及 helper/app-server 根范围；保留这些解码/证明不意味着恢复旧执行能力。公共终端、直接 Codex 和 parser 沿各自原入口，不能随 Orca Agent 写链一起删除。

远端 Runner 执行、SteerLine 环境检测和发布排空分别验收。实际模型偏好不是独立模型证明；普通 development 不继承 audited 资格，workspace audit 可为 unreported。usage 未提供时如实保持 unreported，不读取 Orca 私有 transcript。

## 输入身份与恢复

受理时冻结执行配置、目标、仓库基线、原输入及停止范围。accepted 不等于 Started；Started 和完整结果必须属于原请求、原 session、原 turn。原子首消息包括用户正文与 Runner 固定指令，模型/effort 经 firstMessage.selection 传入。普通开发续轮在原会话已核实空闲后使用结构化 send，由持久原操作派生消息身份。反馈、报告和发布仍使用原 Workflow。调用者继续使用 Runner 的 submit、reply、stop 和查询接口。

输入可能已发出而结果未知时，只读对账原操作，不补发 create/send/Enter，不重建替代会话。额外用户输入、错误归属、缺页或缩短正文不能被当作完整原结果。持久化授权撤销后不再派发业务输入；迟到结果不能恢复授权。

## 结果文件与脚本环境

Runner 在原工作区准备独立、受限的 progress/result 文件，保存关联身份、初始内容及路径。Orca 原生会话不能直接接收 Runner 的进程环境注入，因此最终冻结提示词提供明确的工作区相对文件绑定。

Agent 在执行读取这些变量的脚本前，将绑定解析为现有文件的绝对路径，在该次命令进程设置 `STEERLINE_RESULT_FILE` / `STEERLINE_PROGRESS_FILE`，让子脚本继承；保持调用者原 executable/参数。Agent 不猜路径、不补造初始报告。固定行为指令来自 workflow 包资产，动态部分仅携带本次文件路径，并计入现有 Runner 输入预算。

脚本负责结果内容；Runner 校验归属并保存原始字节。本机及配对远端新结构化 Run 的原会话关闭确认后，结果通道沿 v1 封存。普通开发的单轮结束、等待反馈和 pr_ready 不触发封存。早期远端 execute 与预集成 development profile 保留原停止未知和既有有界快照窗口。脚本退出、模型成功、报告 completed/ready、Workflow 成功分别表达：关闭成功不会把退出码 1、缺报告或无效报告改成通过。结果文件写者须在命令返回前完成；不承诺管理脱离会话的任意后台写者。

## 冻结记录与执行构造

当前 `Lifecycle` 只装配结构化 Agent Session 执行。scope、离线停止核验、usage 和工作区文件绑定读取复用包内冻结记录校验，不通过执行工厂获得这些读取能力；只读证明不派发 create/send/close，也不创建 observer、state feed 或 helper owner。

旧 terminal/helper 的派发、TUI 输入/观察、控制 actor 和主动停止分支已退役。原配置/状态/receipt 格式及严格历史解码保留，公共终端读取和当前结构化工作区创建/原回执对账仍有实际消费者。保留旧 scope 是为了解释原证明，不能据此恢复旧执行或扩大停止范围。实施范围与证据见 [A/B 验收](acceptance/orca-reader-execution-boundary.md)。

2026-11-06 职责进一步收口：application 的开发关闭合同标记、后端冻结 scope 要求及成功 scope 诊断由 adapter 统一解释，业务侧保留 Workflow、授权、相等比较及原事务边界。标记分类不增加严格解码门槛，也不代替停止证明；见 [A 验收](acceptance/orca-stop-contract-boundary.md)。

历史读取测试直接消费 frozenRecord，当前结构化夹具经合法 NewLifecycle 构造后注入隔离传输；生产 Lifecycle 删除 17 个由旧测试维持的方法，旧 State encoder 仅留测试侧。原终端 original/boundTerminal、并发及共享 CLI/creation、旧快照关闭拒绝等真实消费者保留。格式及执行、恢复、停止承诺不变；[B 验收](acceptance/orca-reader-fixture-boundary.md)单列首轮公开 fixture 读取失败和同 EXE 四项复验，不授予完整级别或真机资格。

同日[实现复审](acceptance/orca-stop-reader-reaudit.md)未确认生产缺陷；补强原输入不变和在线/离线 scope 边界测试，原合法入口、错误优先级及格式保持。最新产物与实际定向资格以复审记录为准。

2026-11-06 原始状态与历史资产继续分离：无产品入口的 StateFeed 双实现/采样队列已退役；SubscribeState/Next/Close、QueryState/QueryWorktreeAgents/QueryOriginalAgent 和原 AgentState/Observation 编解码保持。原始订阅仍有 stability probe/network 预检消费者，其通知、独立初始化 context、查询并存和取消/断开由真实协议测试保留；不恢复 Feed 兼容壳，见 [A 验收](acceptance/orca-state-feed-retirement.md)。

后续独立的 B 删除旧 helper 元数据/命令生成，四资产 embed/摘要仅供测试装配。TerminalConfig/owner/receipt 和原冻结证明读取不变，不从当前 bundle/EXE 推导原 hash。五个 JS、Node TAP 入口和旧 Pi 诊断 probe 保持原字节；该 probe 不成为当前结构化资格。持久格式及现行执行、恢复、停止不变，两批独立构建和实际资格见 [B 验收](acceptance/orca-helper-producer-retirement.md)，不追认完整级别或真机结果。

同日[实现复审](acceptance/orca-feed-helper-reaudit.md)未发现删除闭包越界或新的生产缺陷。原始查询/订阅、共享历史与当前执行夹具、Node/诊断及持久格式保持；新跑完整协议/adapter 与原 Node TAP 通过，其余原应用/公开受控资格在源和产物一致后明确复用，不虚记重新执行。

## 会话停止与已接受限制

**Orca 链路不保证全部派生进程或额外主机资源正确清理。这个限制不成为正常业务链路的额外门禁。** 对本机及配对远端已启用原会话结算的结构化会话，停止资格是精确绑定原 session 的 close 成功回执，不是主机级资源清场。`working=false`、回合结束、观察断线或界面消失均不能替代该回执。

| 原事实 | Runner 行为 |
| --- | --- |
| 原 close 明确成功 | 确认原 session scope 停止；允许结果收取和 Workflow 收尾 |
| 原创建尚未派发 | 明确记录未创建，不伪造 Orca close 回执 |
| close 明确失败、响应丢失或缺少原回执 | 原会话停止仍未知；保留原操作与人工对账出口，不重发 close |
| 原操作已保存成功回执 | 重启/离线核验复用绑定原 Run/attempt/session 的回执，不再次关闭 |
| 额外 shell 或派生资源清理未知 | 可尝试既有最佳努力清理；记录范围限制，不阻断已确认的会话停止 |

`stop_status=stopped`、`resources_settled` 和 `verify-stopped` 都必须结合 `stop_evidence_scope` 解释。`orca_agent_session` 的 settled 只声明原会话范围结算；`WritersAbsent` 不因此置真，不宣称所有后代退出、主机凭据撤销或额外资源清理成功。不得用伪造成功字段掩盖 close 自身未知。

这个原则也适用于其它 Orca 组合：满足各自明确停止对象的证明即可，不额外要求范围以外的全进程退出；现有证明方式不被静默替换。

## 对 SteerLine 各链路的影响

环境检测消费原最终报告和原会话停止事实；会话关闭确认后可正常完成，不因额外清理缺少保证继续显示检测中。SteerLine 继续要求 stopped、原封存合同、原生成功和匹配报告。新远端停止与 v1 封存的复验及远端环境检测的发布条件见[解阻验收](acceptance/orca-structured-prompt-input-p1-unblock.md)。普通执行、结果发布和取消同样遵守各自业务条件。默认普通开发的多轮停止和排空见 [P2 验收](acceptance/orca-structured-development-p2.md)。

服务停止仅代表 Runner writer 退出，不代表活动 session 已关闭。重启核对原身份及原操作，不重发输入。换构建部署仍要求原 writer 已退出、业务与未完成控制已经收尾、各 Run 满足其冻结停止 scope；原 session 已确认关闭时，额外资源清理未知不单独阻断部署。close 自身未知仍按原停止边界处理，不用换目录或删库绕过。

SteerLine 不复制 Orca 私有状态机、不直接读取其数据库，也不增加一套更强的清理要求。配置和安装维护只通过正式入口，历史失败及未知事实保留。

## 设计维护与验证

变更会话选择、输入、文件接线、结果、停止或恢复时，同一 PR 更新本设计、公开能力与受影响合同，记录实际固定提示词差异/字节数及验证。回归至少覆盖：关闭成功后结果封存；额外清理失败不阻断；丢响应不重放；错误身份/回执拒绝；离线及重启复用原回执；脚本读取真实环境变量并生成可核验报告。

本文件描述当前设计决策，不以历史验收代替本次资格。结构化迁移的历史范围与限制见[原实施记录](acceptance/orca-structured-agent-session-mvp.md)；终端双端资格见[B6 验收](acceptance/orca-native-rollout.md)；能力层分工见[能力层设计](spec/orca-capability-layer.spec.md)。
