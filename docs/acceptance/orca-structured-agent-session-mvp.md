# 本机 Orca Codex execute/1 最小迁移实施与验收

日期：2026-11-03（Asia/Shanghai）。绑定 [Spec](../spec/orca-structured-agent-session.spec.md) / [Plan](../spec/orca-structured-agent-session.plan.md)。本机新 execute/1 已实现结构化首消息、原回合全文取回和最多一次原会话 close；DSH 执行及专用历史/用量支持已移除。**完整资源退出仍未核实**，不授予该资格；未部署或替换用户已有服务。

## 构建与范围

源码基线 `457ffaea20b6227a0bdaec9e56f5b9e7f54da337`，在本工作副本实现。以下三场真机验收使用审计前的同一构建：Go 1.26.4 / windows/amd64、Runner 0.2.0-dev.4，EXE SHA-256：

`e497382b170ea722d9983f2dc502c4aa047d303ec382b6c73ad074193b6bd6a3`

[构建和生产源码摘要](evidence/orca-structured-agent-session-mvp-20261103/build-source.json)记录具体文件；[证据清单](evidence/orca-structured-agent-session-mvp-20261103/manifest.json)记录原始结果和测试产物。运行端 Orca 为已有 1.4.246，runtime `acbe88cf-6a61-4eeb-8178-6d4886af40c8`、PID 27084；开始和结束公开 status 一致。没有修改 Orca 安装、源码或配置文件，没有 provider 直连、私有 Orca DB/transcript 读取或远端 Pi 安装。

唯一切换组合是新受理的 `orca-terminal + local/空 environment + codex/省略默认 + execute/1`，冻结为 `orca_agent_session_v1`。其它 Orca Codex Workflow、Pi、远端及非 Orca Codex 仍按原冻结协议/实现执行。旧 helper 仍服务其原消费者；本次不宣称全部 Orca 迁移完成。

## 实现与复用边界

| 位置 | 实现 |
| --- | --- |
| `contracts/run.go` / `application/freeze.go` | 精确组合选择；新组合允许多行 execute 正文，拒绝显式旧 executor/helper 字段，拒绝发生在 helper 查找/metadata/启动之前 |
| `protocol/orca/agent_session.go` / `state_local.go` | 复用本机 unary 连接、runtime/request 核验；只提供 createSupport/create/history/item/close；普通调用只需 native 参数，不依赖 Run/Snapshot |
| `orcatransport/agent_session.go` | 既有 Lifecycle 的新协议分支；原子 create.firstMessage、唯一消息/原 turn 关联、分页全文、只读恢复、原 session close |
| workspace / runtime files / reliable | 复用原一次性 worktree 创建 owner、固定 base、默认辅助 shell 登记、受限文件能力和持久意图；不另建调度器、状态库、消息总线或停止账本 |
| worker / control / offline reader | socket write 前复核原持久授权；丢响应只读核实；新 native 资源不落入 terminal/OS 缺席成功分支；自动收尾与显式 stop 共用原停止意图 |

首消息是最终冻结正文的一个 text block。没有 TUI 启动、业务 terminal send、换行替换、补 Enter、第二次 send 或错误 fallback。native fence 不当 incarnation/CAS；新资源使用独立 kind，不伪造 PID/terminal handle。profile/state 为独立 v1 opaque envelope，实际状态约 1.9 KiB；没有复制输入或结果到该状态。

model/effort 经 `firstMessage.selection`，full_access 经 `options.permissionMode=full-access`。准备和创建均检查实际 RPC status 的必要能力，随后检查原 worktree 的 createSupport。该检查消费**已有公开能力**，不会要求升级/修改 Orca。普通外部调用可复用 typed client；业务授权、原操作和结果事务仍由 Runner 管理。

公开 message 没有 commentary/final phase 字段。本版本返回原 turn 的全部完整 assistant 文本，按 journal 顺序以空行分隔，来源标记 `orca.agent-session.turn-result.v1`；包含工具前说明，不冒称 provider final-only。缺页、缩短后无法补全、归属不明、额外用户输入或未知 outcome 均不认领完整结果。仅按公开 `success/failure/cancellation` 映射 outcome，不从 stopped/working=false 推导业务成功。

可选 result-file 复用原文件通道；因为 Orca 管理执行环境，其固定相对路径在冻结正文中提供，不安装环境变量。新协议 usage 暂未报告，不增加私有日志读取。

## 审计前同一 EXE 的正式场景

使用 Orca 已登记的 SteerlineTest、固定 base `9d81d9c2652177faf6354d4e9f9f3aec79535d11`，由正式 Runner submit、工厂、工作区和 SQLite 持久化链路创建独立 Run。模型只打印工具标记/同步等待；不修改仓库文件、commit、push、创建 PR 或安装软件。

| 场景 | Run / 原 session | 结果 |
| --- | --- | --- |
| 多行 + 两次工具 + 多段输出 | `3d5f07fc32e344cc444bea61b0ff7f46` / `oas_b4f45044c616d148a8614534d9612e32` | 1 user、2 tool-call、3 assistant；300 行，完整聚合 7,021 字符，content_result=succeeded |
| 原活动回合 stop | `6b937ec8a51e471bfd8942c702b56288` / `oas_76cd08a53b1b15554fa49cc99386bb6d` | stop 前原 turn=running、working=true；之后公开记录 interrupted/cancellation；授权撤销、无第二条 user |
| 超过旧预览上限 | `8e21a8ab7ac9ef87f296a6f892610ba7` / `oas_5bd4b72845246ab2effad083ee89e505` | 1 user、2 tool-call、3 assistant；600 行、13,921 字符，超过旧 8,000 字符预览上限；正文逐字等于完整原 assistant 项聚合 |

三场都使用 `gpt-6.1-sol / medium` 请求。原子探针独立核验首消息 selection，活动回合的公开 options 还报告 `reportedPermissionMode=full-access`。这是接线及当前 options 的证据，不是 audited 逐回合实际模型证明；`acceptedInputSources=saved` 也不包装成该证明。

[持久化独立核对](evidence/orca-structured-agent-session-mvp-20261103/verified-persistence-checks.json)通过只读 **Runner** DB，对照公开 Orca history，证明 CRLF/LF、空行、中文/emoji、引号、路径、代码块和尾空白正文等于原 StageInput、SHA 一致、只有一条 normalized user；providerItemId 与 normalized user item 分别绑定。正常与长结果均逐字等于原回合全文。每场只有原 prepare_workspace / prepare_session / start_turn / stop_execution 意图各一次，Agent launch command 为空。该意图计数不是独立抓包计数；丢响应和重复调用的 wire 调用次数由下述夹具另验。

正常与活动两场在同一 EXE 服务停启后仍为原 session/turn、1 user、同一停止意图；没有 create/send/close 重放。长结果另验离线读取。辅助默认 shell 逐场真实登记，经过原 terminal 身份检查/关闭；不能据此推导 native Agent 后代全部退出。

## 停止事实与保留现场

三个最终 Run 都保存原 close 的 `ok` 回执，并且：

- Run：`stop_status=unknown`、`resources_settled=false`、`new_inputs_allowed=false`，诊断明确“资源结算未核实”。
- 正常/长结果：完整 content_result 继续可读，业务成功没有被 cleanup unknown 覆盖，也没有把聚合 Run 伪装为 completed/stopped。
- 活动 stop：原取消事实保留在公开只读证据；不利用迟到正文恢复业务授权。原 stop 未解决时重复 stop 被拒绝。
- service stop：服务已退出，last_shutdown.resources_settled=false；同目录 verify-stopped 的三个 Run 和 aggregate 都为 false。

见[最终服务退出](evidence/orca-structured-agent-session-mvp-20261103/verified-all-service-final-status.json)、[最终离线核验](evidence/orca-structured-agent-session-mvp-20261103/verified-all-offline-stopped.json)。没有以 worktree 删除、OS kill、另建目录或再次 close 追认退出。隔离服务已关闭，原 worktree/Run 线索仍保留，资源结算需后续独立处理。

## DSH 删除与保留

删除新任务支持声明/准入、冻结分支、DSH profile、dsh-managed 扩展及打包、专用正文描述/输入分支、用量 parser、专用夹具与 root policy、DSH 和 Pi/DSH mixed 测试注册/peer fixture。当前 Orca 新任务只接受 Codex/Pi，DSH/其它 agent 返回 unsupported；旧 DSH 工厂/用量绑定也不会按 Codex/Pi 解码。

共享 Pi TUI、工具、原生扩展、公共 terminal 和非 Orca parser 保留。只保留一个不消费/不生成的旧 `dsh_sha256` metadata 字段，以免严格解码误拒既有 **Pi** helper；这是共享 metadata 容器兼容，不是 DSH 运行或历史 decoder。旧 DSH 与 Codex 记录同目录的通用读取和逐记录拒绝有测试。未删除用户 DB/历史文件，未卸载 DSH；旧活动任务仍须原构建处理。

P0 的 189 个文件、阻断再调查的 3 个文件全部通过原 manifest 哈希复核，未改其字节或原资格。旧 source-map 描述调查时的代码，不能当作本次实现源码摘要。

## 回归、失败与修复

[测试摘要](evidence/orca-structured-agent-session-mvp-20261103/test-summary.json)及压缩原日志保留实际结果。

| 检查 | 实际结果 |
| --- | --- |
| 首轮 `go test ./internal/... ./tests/... -timeout 45m` 广泛回归 | 33 包通过、6 无测试包 skip、1 包失败；顶层 820 pass / 16 skip / 1 fail。tests/integration 的 34 个顶层测试全部通过，包含非 Orca内容流程、执行、恢复、文件和服务行为 |
| 上述唯一失败顶层 | 旧 TestSharedNativeEngineStopReconciliation 的 3 个子例在 200ms 内未到达 close；原夹具复用真实用户 CLI mutex。保留首次全量和未隔离复测失败日志 |
| 最小测试修复 | sharedNativeEngine 使用独立 LOCALAPPDATA/cache；保留原 200ms 期限和全部副作用断言，不改旧运行代码。完整 18 个停止子例、排队首派发组、新协议持久化/DSH 记录隔离复核通过 |
| 最终 RPC 修复后相关全包 | protocol/orca、orcatransport、contracts、resources 通过；另有 usage 全包、能力/组合/历史反例和新增路径定向检查通过 |
| Pi / 测试资产 | 保留的 Node 扩展 21 pass / 0 fail / 0 skip；livetest 全包通过；live tests 用 `go test -c -tags live` 编译通过，未计真机 Pi 或其它组合资格 |

没有把首次全量失败改写成通过，也没有声称最终重新取得一次官方 scripts L1/Controlled 前置资格。全量之外的最终局部修复分别复测；本机新结构化组合的真机资格来自上表同一最终 EXE 的三场正式入口。

实现过程中还发现并修复：execute/1 的原单行准入不允许多行；能力发现目录缺少带必填 execute 提示词的校验样本；RPC status.get 的 capabilities 位于 result 顶层，不能误用 CLI status 的 result.runtime 层。最后一项的中间候选在 prepare_session 明确 rejected，未派发业务正文；其两个失败 Run 和停止记录留在 `final-*-rejected*` 证据，不以换 ID 重试未知副作用。实际 RPC status 回执已纳入最终能力解析测试。最初默认 Go 10 分钟预算超时、sandbox 文件/本地通道拒绝、首次 live compile 命令触发 TestMain 缺 evidence-dir 的拒绝也保留，不计产品通过。

## 实现后结构审计

本会话自审，未冒称独立子代理审计。确认：一个既有 Lifecycle 加冻结协议分支；只有实际 native 方法、IDs/cursor 和原意图；无未来 Pi/远端实现、通用 registry、第二 DB/blob/journal 镜像或新停止框架。首消息只在 StartTurn 的一个原子副作用中提交，准备和恢复不补发；关闭与业务结果分离；状态没有用 lease fence/PID/terminal 假造资源退出。

审计收口修复了共享 metadata 对 Pi 的误伤风险、旧测试 mutex 污染，以及已退役 mixed 场景剩余 peer fixture。保留的旧 Codex/Pi 代码仍有真实消费者，既不是新链 fallback，也没有为本 PR 新增桥接。MVP-01–06 的实现/反例/正式入口证据如上；完整资源退出、provider final-only、audited 实际模型、Pi/远端结构化仍不声明已获得能力。

用户要求提 PR 前的再次质量审计、修复、更新 main 后的测试和最终构建边界，见[质量审计](orca-structured-agent-session-mvp-quality-audit.md)。本页原始真机证据及 manifest 保持不变；其 EXE 摘要不冒充后续审计构建。
