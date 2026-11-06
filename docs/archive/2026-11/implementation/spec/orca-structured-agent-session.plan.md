# Plan：本机 Orca Codex execute/1 结构化执行与 DSH 退役

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-03。状态：范围与停止限制已获用户确认，设计已收敛；运行代码已实现，正式本机场景与回归记录见[实施验收](../acceptance/orca-structured-agent-session-mvp.md)，未部署。绑定 [Spec](orca-structured-agent-session.spec.md)，基线 `457ffaea20b6227a0bdaec9e56f5b9e7f54da337`。设计复审见[审计记录](../acceptance/orca-structured-agent-session-mvp-design-audit.md)。

## 1. PR 目标、顺序与停止条件

只交付两项：本机 Orca Codex 新 execute/1 Run 的原子结构化输入→原完整结果→同会话停止请求；DSH 执行和专用历史/用量支持删除。Orca 本身不改，Pi/远端/其它 Workflow 和非 Orca Codex 保持；不做全量迁移的 P1–P6、不建临时 helper 桥。

用户已接受：缺充分停止证据时 stop_status=unknown、resources_settled=false 并明确未核实；结果事实单独可读，离线同样不报通过。此限制不是本 PR 阻断，不再请求用户重复确认；完整资源退出/audited 取证仍是后续边界，不提前实现其替代机制。

按 S1 最小协议 → S2 单组合冻结/执行/结果 → S3 停止/恢复及 DSH 删除 → S4 定向与真实验收顺序实施。阶段可受控验证；正式生产工厂/能力声明只在此 PR 具备输入、结果、停止请求和未核实展示后一次接通，无临时启用开关。未知副作用零重发仍是硬门禁。

## 2. S1：只补本机原生会话所需能力

在 `internal/execution/adapters/protocol/orca` 复用本机连接和 runtime/request envelope 核验，增加 create（带 firstMessage）、history、close；只有 history 缩短确需取完整项时增加 item。复用实际 I/O 替换点，单连接一个 reader；只读分页/字节数/期限/取消有界，不增加远端结构化实现、stream subscription、WS 配对或通用 RPC 透传入口。

按 P0 的实际 wire 类型、ID 规则、canonical JSON/UTF-8/SHA-256 实现 fingerprint；operation 摘要与 body-only submission 摘要分别处理。使用现有固定向量覆盖 CRLF、中文/非 BMP、JSON 特殊字符和 key 顺序；双层 ok/refusal、pending 与 provider Started 不能混用。引用 [P0 证据](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/contract-vectors.json)，不依赖安装私有模块运行生产代码。

CheckpointDispatch 放真实 socket write 前；排队没有副作用，撤销/持久写失败零调用。每个原动作只允许一次写入，连接重建不重放；不把忽略 expectedRuntimeFence 的返回包装为严格 CAS。

S1 夹具验证正确字段、指纹、原 runtime、双层拒绝、半包/超限/分页缺口和 write 前后取消，至少一例无 Run/Snapshot 的普通能力调用。不为本 PR 之外的方法先建测试矩阵。

## 3. S2：冻结、工厂及最小纵向执行

| 消费者 | 最小修改 |
| --- | --- |
| `contracts/run.go`、resources schema/catalog | 新受理 agent 仅 Codex/Pi；限定本机 Codex execute/1 原生配置及旧 helper 字段拒绝，其它 Codex/Pi 规则保持 |
| `application/freeze.go` | 在旧 helper metadata/启动命令编译前识别唯一目标组合；冻结新协议/profile、原输入/模型请求、target/runtime；不增加公共路由字段 |
| 既有 Engine/adapter 工厂 | 按原冻结 protocol 装配 Orca 新生命周期；旧记录按旧协议，不能按当前 Workflow 重新选链 |
| `orcatransport` | 新协议的 create/history/final/close 和只读恢复，复用 worktree/file/operation，不调用旧 launchAgent/terminal observer/model bridge |
| backend/runstate/严格 codec 消费者 | 新 profile/state/资源识别、clone/预算/持久化/inspect/stop 的实际必要接线；不伪造 terminal/PID，不重构所有 backend |

profile/state key、v1 及内部协议沿 Spec §2。不得仅冻结新 key 而遗漏准入、工厂或离线读取。opaque payload 只存必要身份/游标/派发事实；正文和完整结果留在既有原输入/operation/ContentResult/消息存储，不重复塞入 64 KiB opaque envelope 或另建 blob 库。既有 opaque state/v8 可用则不升级全局版本、不移动用户目录；旧活动任务由原版本排空，不设计活动任务迁移。

继续用原执行角色的 model/effort，显式值映射 firstMessage.selection，权限用 Orca 现有 full_access 选项；先核验原子显式 selection 和执行权限接线，不能以之前 send selection 探针代替。无法兑现的显式 executable/args/helper 配置在本组合派发前拒绝，不影响未迁移组合。不读取/重写 Orca 用户配置。

工作区准备与 Agent terminal 准备在新协议分开。Orca 默认辅助 shell 如存在，登记为本 Run 资源、通过已有公共 terminal 能力正常清理；不再启动旧 Agent，也不借其关闭推导 native 会话退出。

首次业务动作只执行一次 create.firstMessage；首消息身份和正文随原意图先持久。复用现有 lifecycle action 的原子副作用边界，不额外建立 operation 类别/调度器；原准备动作和后续 StartTurn 不得再 create/send。回执必须通过已有 ValidateFor/ResourcePending 等实际字段约束；不要用“准备已完成”隐含一个尚未保存的模型输入。

结果观察通过原 session/clientMessage/userItem/turn 关联、明确结束原因与完整原助手结果进入既有 TurnEvidence/ContentResult。验证带工具/多段输出的 final 选择与完整性；历史缺页、shortened、错误 scope/turn、多个不明 user item 均不认领成功，不通过模型自述/最后一段预览/旧 terminal history 补足。

execute 不新增 feedback、answer、resume 或发布能力；原生问题/审批仅记录受阻诊断、可停止，不代答。保留原完整结果失败/取消语义，optional usage/progress 未报告不阻断业务结果。

## 4. S3：停止、恢复与公开投影

先保存业务授权撤销和原 stop 意图；同 session close 一次，正常结果后的自动收尾和显式 stop 共享原动作，不重复关闭。不增加 cancel、两阶段停止、全局 receipt ledger、OS 扫描/kill 或 provider 旁路。

必须核对现有 `application/control.go`、`content.go`、`verify_stopped.go` 的实际消费者：

- 原 native session 已存在或 create 已可能外发时，空 `Ref.External.Kind`/terminal 不能走“无资源即 stopped”；有预生成 sessionId 也不能反向证明资源已创建/已退出。
- close ok 只认其实际请求回执，State.Stopped/WorkerAbsent/WritersAbsent 不得因此设 true。新 scope 的在线/离线结算缺证据均为 false/unknown；capabilities/CLI 文档明确此范围限制。
- 先持久完整 ContentResult，再沿原自动收尾；清理未知不能抹去结果、改为业务失败或伪造整体 completed。可沿现有 Run reconciling 与 stop_status=unknown 表达，不新增全局完成状态机。
- 原 close/create 丢响应、提交失败、服务重启、重复 stop 只读核实原身份；不 ensure/reopen/retryUnknown/replay。对查询已有窗口做有限恢复，到期沿现有人工出口，不因每次重启刷新预算。
- 正常 service stop 只关闭本服务观察/连接；不顺带 close Agent。离线核验不构造 Engine、不派发模型、不补写 stopped。stop revoke 对迟到事件仍有效。

冻结 `orca_agent_session` scope 并接入其必要 reader/stop 路由，不能依靠 `p.(*orca.Lifecycle)` 或 OS/terminal 旧成功分支遗漏新协议。保留残余工作区/线索，未核实资源不自动删除或判升级已排空；不扩建人工强制通过机制。

## 5. S3：删除 DSH 当前执行与专用历史支持

先按生产消费者拆删除边界，不以搜索结果整包删除。实际发现的最小清单：

| 类别 | 修改/删除点 | 保留边界 |
| --- | --- | --- |
| 新任务/资源配置 | `contracts/run.go`、`resources/catalog.go`、resources schema/assets、`application/freeze.go` | agent 默认 Codex，Pi 原校验保持；DSH/其它 agent 明确 unsupported、零 Agent 副作用 |
| 专用执行资产 | `orcatransport/dsh_profile.go`、`extensions/dsh-managed.mjs` 及专用测试/打包注册 | 不卸载主机软件，不修改 Orca |
| 共享执行文件 | `terminal_runtime.go`、`terminal_receipt.go`、input_contract、lifecycle、usage_binding 的 DSH 分支 | Pi/公共 terminal 仍用的函数、字段和夹具保留 |
| 专用历史/用量 | `usage/parsers` DSH 分支及 testdata/manifest、`usage/api/policy.go`、`usage/collect`、resources usage 中实际 DSH 消费者 | 非 DSH 用量/历史不变；通用容器可给旧 DSH 记录级 unsupported，但不保留 DSH 私有解码框架 |
| 测试/文档支持声明 | tests/catalog、livetest/cases/config/manifest、DSH live/受控注册、当前 README/操作说明 | 原 acceptance/evidence 留存，不能从历史已通过事实中删除 DSH 字样 |

旧 DSH 活动 Run 不被新版本恢复/停止；原构建先排空。移除专用历史支持不物理删用户数据，不把旧 DSH 当作默认 Codex，不使一个旧 DSH 记录阻断同目录 Codex/Pi 读取或恢复。新旧准入与通用记录容器分开，避免为了删除 agent 去破坏全局 JSON 解码。

## 6. S4：验证和收口

以下是本 PR 验收要求；实际执行和资格边界见实施验收，不将设计条目当作通过证据。

### 6.1 组件及受控入口

- 能力：wire/指纹及双层回执、实际写边界、有界 history/item、身份和消息关联。
- 生产接线：新组合及省略 agent 的选择；其它 Codex/Pi/直接 Codex 不变；旧 helper 字段/显式不支持设置零派发拒绝；旧 Run 按冻结协议恢复。
- 事务故障：派发前撤销/DB 失败零调用；create/close 生效丢响应、回执提交失败、重复 stop、重启只读补收；实际调用计数，不用 mock 未覆盖等同于零调用。
- 结果/停止：原消息和完整 final，未知 outcome/错来源拒绝，完整结果已存而结算未知仍可读；空 Ref 不漏 native session；service 脱离、在线及离线不假报 stopped。
- DSH：新受理全目标拒绝、执行/专用历史注册与资产删除；旧 DSH unsupported 与同目录 Codex/Pi 隔离；Pi 与公共 terminal/非 Orca 受影响回归。

复用现有 Engine/存储/工厂与既有受控注入点。新测试名称在职责落实后注册并核对 `go test -list`/叶例命中，不能用拟定名称的零匹配或 skip 签收。实现后按变更实际运行 protocol/orca、orcatransport、contracts/resources/application、usage 相关包及直接 Codex/parser 定向回归，沿现有 foundation/Controlled 入口完成必要集成；不另建手工 Snapshot driver 代替正式 Runner。

已有 `TestOrcaAgentSettings`、`TestSRFreezeOrcaAgent`、`TestSingleExecutionAdmissionWithoutPublication`、`TestUsageCapabilitiesBoundariesAndSeparation` 等按新范围补断言；受影响组件基础命令可沿 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport ./internal/contracts ./internal/resources ./internal/usage/... -count=1`。受控集成复用 `TestSRSingleExecutionColdStop` 的真实停止/结果封存路径，新协议另加期望为 unknown 的原身份断言，不能将旧 stopped 通过平移。application/直接 Codex/parser 与新测试的具体命令在实现确定后按实际注册列入报告；本轮没有运行这些命令。

### 6.2 两场本机正式资格

沿已有 live-test 基础注册/参数化本机 Codex execute/1，使用同一最终构建、隔离 Run/session/worktree 和正式 CLI/API/持久受理；不要求新增 L3 体系或每场 PR。

| 场景 | 验收 |
| --- | --- |
| 多行单消息及正常结果 | 含 CRLF/LF、空行、中文/emoji、路径、代码块、尾空白的原输入；一个归属正确 user item、原始字节/摘要一致；使用工具并产生多段输出以验证完整结果选择；结果持久可读，自动收尾最多一次 close，缺结算证明明确 unknown/false |
| 原活动回合停止 | 由原消息/turn 的 started/running 事实确认仍活动后 stop；持久撤销、原 session close 最多一次、零后继业务；保存可用结果/取消事实，正常 service stop 和同目录离线 verify-stopped 不冒报资源结算 |

停止未核实是允许且必须检查的边界，不能计成“完整退出通过”。测试 fixture 的人工/公开清理另记，不能借换 ID 重跑、worktree 删除或 OS kill 追认原停止成功。首消息/close 计数分开，未知原操作不重试。

### 6.3 证据与产品同步

记录源码/dirty 摘要、最终 EXE SHA、Orca/目标身份、冻结配置/协议、原 operation/session/message/turn、正文摘要、完整结果及资源核验诊断；失败、skip 和清理未核实保留。后续实现证据另建目录，不覆盖 P0 或本轮设计审计。

交付时同步 schema/catalog/capabilities/准入、执行指南、产品停止限制、测试目录和验收索引：只声明本机 execute/1 多行修复及 DSH 撤下，不称四组合完成或所有后代已退出。当前导引标明结构化组合、原回合全文聚合方式及停止未核实限制。

## 7. 本 PR 不做及后续债务边界

不做 development/反馈/PR、audited、Pi/远端 native、远端 Pi 安装、native viewer、通用消息总线、全量 journal 镜像、模型插件注册平台、全局 DB/状态目录迁移或完整停止证明服务。

保留原 Codex/Pi 实现是未迁移组合的实际消费者；新协议没有其 fallback。后续扩展沿同一 profile/state/能力/原 turn 管线，在有真实第二消费者时提炼必要共享代码；最后一个消费者退出时删除旧 helper/TUI/模型桥，不在此 PR 预建它们的替代控制面。DSH 不等待后续批次，本 PR 的新执行及专用历史均删除。

## 8. 完成检查

按 Spec MVP-01–06 逐项给实现位置、实际测试命中、两场正式证据、未核实停止诊断和 DSH 删除/保留清单。完整结果/可靠派发/DSH 回归失败仍阻断本 PR；未取得完整资源结算、未迁移组合的 P0 缺口不冒称通过，也不重新阻断已确认的窄范围。实现使用既有 Lifecycle 加新协议分支、本机 unary RPC、opaque 身份与原 operation；没有通用消息总线、第二状态库或未来 Pi/远端占位实现。实际审计、回归及两场正式运行见实施验收。
