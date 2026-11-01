# Spec：本机 Orca Codex 停止条件收敛

日期：2026-11-01。状态：审计后已实施，专项最小验收完成；未部署或发布。原“先不改代码”的设计阶段已结束，本次实施依据用户后续“审计后实施”授权。实际构建、失败历史与验证边界见[验收](../acceptance/orca-local-codex-stop-simplification.md)。

依据[长期产品承诺](steerrunner.spec.md#长期产品承诺) PC-C06/C07、PC-R01–R10，尤其是 PC-R06 的范围停止、PC-R08 的观测容错和 PC-R10 的升级边界。配套 [Plan](orca-local-codex-stop-simplification.plan.md)。调查代码基线为 `8bed4c4`；长期契约为当前工作副本中的已修订内容，不冒充已提交的新构建。

## 1. 目标与取舍

将本机 Orca 受管 Codex 的停止与结算，从“证明原工具进程树完整退出并取得签名及关闭检查点”收缩为“终止原资源，并核实登记的终端和原进程身份已结束”。实际取消及 Job 清理继续执行，移除对强证明文件的强制等待，不将同一等待搬到离线核验或升级排空。

接受的代价：结算成功不再证明所有后代写进程已退出；未登记的派生进程或脱离管理的后台服务可能需要本人检查。停止中断后的部分清理允许转人工，不新增自动完成所有清理动作的恢复框架。结果、审计和 PR 核验保持原保证。

实际收益是删除两类停止专属证明规则及依赖。adapter 生产代码净减 10 行；计入格式、公开投影和客户端后生产代码净增 22 行。没有可比耗时或故障率实测，不承诺正常任务提速或固定加速比例。

## 2. 范围与基线

### 2.1 唯一受影响路径

同时满足：`execution.adapter=orca-terminal`、本机目标、Agent 为 Codex、冻结 profile 的 `Environment` 为空、`Terminal` 为空且 `RuntimeExecutable` 非空，并核实原 Orca runtime 是当前执行主机的本机 runtime。不能只按 Agent 名称或 Environment 空值套用新规则。

包括该路径的新 Run 的显式 stop、原 stop 只读对账、相关资源释放、普通服务关闭中的资源核验、公开投影及离线 `service verify-stopped`。未就绪、准备中停止和创建回执迟到也在范围内。

排除直接 `codex-app-server`、Orca environment 原生远端、Pi/DSH、内容流程、独立 parser、模型桥与审计业务语义、新传输/daemon、资源重建、自动清理 worktree/分支/PR、外部 SteerLine 修改及部署。全局格式切断只隔离旧数据，不改变这些后端的新执行语义。

### 2.2 审计基线调用与共享消费者（实施前）

| 位置 | 当前职责/依赖 | 本方案处理 |
| --- | --- | --- |
| [lifecycle.go](../../internal/execution/adapters/orcatransport/lifecycle.go) 的 StopLifecycle | 本机受管 Codex 先 stopCodexNative，再关业务 PTY、closeControl、verifyStopped | 替换本路径结算条件；保留真实停止、身份及持久授权边界 |
| [verify_stopped.go](../../internal/execution/adapters/orcatransport/verify_stopped.go) | verifyCodexExit 读取签名退出文件；verifyControlStopped 要求控制检查点 Closed | 本路径改为登记资源的有限查询；其他路径原条件保留 |
| [stop.go](../../internal/execution/adapters/orcatransport/stop.go) | stopProgress 分阶段；business-confirmed 后继续控制结算 | 本路径停止中断后只读核实；不新建分阶段恢复账本；Pi/DSH 不套用 |
| [control_terminal.go](../../internal/execution/adapters/orcatransport/control_terminal.go)、[control_checkpoint.go](../../internal/execution/adapters/orcatransport/control_checkpoint.go) | 控制查询与停止关闭共用 journal；关闭前结算 pending reader | 保留运行期查询及原来源核验；本路径 stop 不以 pending 查询响应或 Closed 文件为前置 |
| [local_codex_history.go](../../internal/execution/adapters/orcatransport/local_codex_history.go)、[workspace_audit.go](../../internal/execution/adapters/orcatransport/workspace_audit.go) | 控制通道查询原回合和审计工作区 | 保留，不能为简化停止整体删除 control/reader 机制 |
| [remote_exit.go](../../internal/execution/adapters/codex/remote_exit.go)、[remote_agent.go](../../internal/execution/adapters/codex/remote_agent.go) | 退出文件、公钥及停止请求路径；原 helper 持有 app-server/TUI Job | 保留真实取消及 Job；停止请求仍用原绑定路径；共享签名生成/读值不作为本批必删项 |
| [execution_owner.go](../../internal/execution/adapters/codex/execution_owner.go)、[owner_checkpoint.go](../../internal/execution/adapters/codex/owner_checkpoint.go) | 直接 Codex owner 退出及审计 checkpoint 签名 | 不改，不全局删除签名类型、密钥或验签函数 |
| [application 核验](../../internal/application/verify_stopped.go)、[host 核验](../../internal/host/verify_stopped.go) | 只读当前格式库、原 writer 锁、原身份、聚合资源结算 | 使用新范围，但仍只读、不启动服务、不补写 stopped |

## 3. 新停止契约（OLCS-01）

### 3.1 冻结范围和必需对象

新范围标识固定为 `orca_local_codex_resources`，不能复用仅表示原业务终端的 `orca_terminal`。它是该路径的停止合同，不是额外权限或调用方可选的宽松模式。

| 对象 | 原身份来源 | 新结算要求 |
| --- | --- | --- |
| 业务终端 | 原创建回执及 Snapshot.Ref.External | 原 runtime/host/worktree 完整清单中已退出或确实缺席；同 handle 换 incarnation 不算成功 |
| 控制终端 | 原 State.Control.Process 及创建回执 | 同上；只核实已绑定原终端，不要求控制 journal.Closed 或 pending 查询响应 |
| Codex helper 原进程 | 已接受的 RemoteReadiness.Owner | 用本机进程事实核实原进程缺席，不能用 PTY 消失替代 |
| app-server 原根进程 | 已接受的 RemoteReadiness.Process | 同上；不要求整个 Job 的可公开 writer-tree 证明 |

进程身份至少使用原 PID、创建时间、可执行文件及主机绑定。原进程仍活动不能成功；查询权限不足或身份冲突返回未核实。PID 复用不能导致终止新进程或认领新身份。TUI/工具子进程仍由原 helper 的既有 Job 清理；不从退出文件中补读 TUI 身份作为结算门禁，也不宣称以上根身份缺席足以证明所有后代退出。

准备期只有已证明尚未创建/启动的对象可以不适用。已尝试 helper 启动却缺 readiness、未知创建却缺终端回执，均不能按“没有记录”等同于“不存在”。沿原创建执行器补收唯一原回执；仍无法确定时保留 unknown，不 create、不扫描同名对象认领、不新建探针终端。control 已创建但未 READY 时仍须关闭并核实其原终端。

登记对象及其来源必须足以覆盖本次已尝试的资源创建。已经持久保存的原 readiness/身份与外部当前观察发生冲突，不能通过少列对象获得成功。

### 3.2 撤销、真实终止和结算

1. 保持现有 request_id/正文幂等、expected_revision、gate 和原子提交。stop 接受事务先撤销后续业务输入授权，之后才执行已持久化的原停止意图。事务失败零新停止调用。
2. 本路径首次 stop 在原操作下执行一次有界清理：向原 helper 提交现有 readiness 绑定的停止请求，保留 helper 内原 app-server/TUI Job 的取消、Kill/Close 与工具清理；随后关闭可核实归属的原业务及控制终端。停止请求的 key/路径值继续沿现有机制，不因取消退出验签而另造请求协议。
3. helper 的协作停止最多等待 2 秒，占用既有本机 stop 总预算；只查询已登记根进程，不等签名退出文件。协作失败、文件上传失败或控制查询尚未结束，不能作为提前跳过其他可定位原资源终止的理由。仍须在剩余期限内尝试既有身份约束下的终端关闭；不引入模糊 PID 强杀或额外清理服务。
4. 本机停止全部等待沿既有 15 秒上层预算，包括 CLI 排队、停止请求、协作等待、关闭和查询。不以内部独立 30 秒 context 延长此路径，不增加用户配置项。到期保留未知及具体缺口，不能因预算耗尽宣称停止。
5. 对 §3.1 全部适用对象核实结束后，才可保存 stop_status=stopped、结算原命令并报告该 scope。文件缺失、损坏、签名无效或 control 关闭检查点未保存，均不单独否定已核实的资源结束。可保留这些历史诊断，但不能让其成为后置门禁。
6. 尚在途的业务结果可按原来源规则补收，不产生新的实施、补报、审计或发布输入。可选 result/progress 收尾不能阻止停止，不能借停止成功修改 PR、审计或 Workflow 结论。

### 3.3 中断、响应丢失与普通重启

沿用原操作 Execute/Reconcile 边界：首次调用执行上述清理；一旦原 stop 可能已外发，重启或轮询只读查询原对象，不重发 helper 请求、不重复 close、不新建资源。已证明尚未进入调用边界的原意图仍可首次执行，不能用“缺回执”证明未调用。

停止调用中断且只清理了一部分对象，若剩余对象仍活动或不可核实，则保持 unknown 并给出人工处理信息。接受这项有限自动恢复的取舍，不为剩余步骤新增持久子任务、清理队列或多阶段自动重试策略。同 request_id 回放原命令；本路径已有可能外发且未解决的原 stop 时，不同 request_id 的再次 stop 返回现有 `operation_unverified` 拒绝，不创建第二个停止意图，不重放 helper 请求或 close。原操作仍可只读核实已由外部完成的清理。

现有 `CloseReadonlyTerminal` 内有重试循环，不能未经收窄直接复用于新路径：首次清理每个已登记终端最多一次 close，响应未知后只查询；不得因其被称为只读设施就重复关闭。保留原终端身份核验及底层 CloseTerminalChecked，其他路径的既有辅助关闭行为不在本批全局调整。

普通 service stop 仍只脱离观察，不默认停止业务。停止后零新输入、暂停/业务等待、技术 resume 的原限制及 parser 例外均保持；本方案不新增“停止后重新开会话继续”的能力。

## 4. 所有结算入口一致（OLCS-02）

- 新路径的显式 stop、CleanupLifecycle、ShutdownLifecycle 中的资源核验以及离线 verify-stopped 使用同一组 §3.1 范围判据；共享窄读取函数即可，不建立新资源管理平台。
- 离线核验仍要求原 writer 已退出、既有锁、无未核实启动、只读当前格式库。只查询已保存身份，不初始化 Engine/observer、不执行 helper 停止请求或关闭终端、不修改 DB、原命令、历史失败或 shutdown 回执。
- 核验业务/控制终端复用原完整清单校验。同一原 runtime/host/worktree 的对象在一次核验中共用采样，不按对象逐一重拉，不跨核验缓存；原 host 缺失、离线、清单不全/重复/截断、runtime/incarnation 冲突都不能成功。进程事实必须属于本机，不能用调用端 PID 推断远端。
- 新路径不能仅凭已持久化 stopped 跳过离线原资源核验，也不能据旧 shutdown 回执跳过当前查询。resources_settled 只聚合当前各 Run 声明范围的结果；任何未解决的资源创建或适用对象缺口均保持 false。
- 在线 RunView 新增可选 `stop_evidence_scope`，在 inspect、observe.run、history.run 中使用相同已提交投影，值为本方案固定标识；原 StoppedRun 同名字段返回相同值。CLI v1、stop_status、resources_settled、退出码及事件结构不另起协议，capabilities/公开 schema 与实际投影同步。其他后端不被赋予新 scope。新增字段不承诺兼容旧严格响应解码客户端，本仓消费者同步，外部调用方升级要求单列。
- scope 在首次受理时由固定后端合同确定并随 Run 冻结，读取不执行 I/O；即使准备失败或缺少身份也能说明该 Run 要求的范围，不能因证据缺失退回较窄范围。它不随新 EXE 或当前配置改变。现有 service status 的聚合结算不宣称更强的全树保证，说明混合后端时仍须查看逐 Run 范围。
- 统一诊断明确“已核实登记资源结束，未证明全部后代进程退出，未撤销宿主登录”；未核实的诊断列出原对象及有限错误类别，不包含凭据或猜测身份。

## 5. 删除与保留清单（OLCS-03）

| 类别 | 本批必须删除或解除 | 必须保留 |
| --- | --- | --- |
| Codex 退出证明 | 本路径 stop/verify/服务结算读取并验签退出文件的强制依赖及轮询；因此失去消费者的专属包装函数 | readiness、helper/根进程身份；现有停止请求路径与上传；共享退出写入/签名类型与直接 Codex 验签、审计 checkpoint |
| 控制关闭证明 | 本路径停止对 settleControlQuery、reader 退出响应、journal.Closed 的强制依赖；只服务这些停止条件且无其他消费者的分支 | control 创建/归属、实际关闭；运行期原回合/审计查询的 journal、响应及 reader 来源/结算规则，不能降低正常业务采纳标准 |
| 停止恢复 | 本路径依赖 business-confirmed 等证明阶段才能继续结算的分支；停止专属关闭字段如仍有共享消费者则保留但不读取为门禁 | 原停止意图、一次派发、原资源只读对账；其他后端实际使用的 stopProgress；未知不重发 |
| 资源清理 | 无业务价值且仅为新 scope 补足全树证明的动作/等待 | 原 helper Job 及工具树的实际取消、终止和释放；精确身份、创建回执；未就绪停止 |
| 宿主与业务 | 无新增删除 | 单 writer、原子提交、授权、来源、PR/provider、模型桥、审计、用量日志 permit/lease、parser 自身清理及独立例外 |

删除以实际消费者为准；不能以同名函数包含 Remote 就认定仅属远端，也不能因机制共享而把未受影响路径改成新范围。共享代码保留时记录消费者，不为追求净删行数复制一份实现。

## 6. 旧 Run 和格式边界（OLCS-04）

本设计选择一次显式切断：实施时启用 state/v8、SQLite user_version=8、默认 `%LOCALAPPDATA%/SteerRunner-v8`。在新格式接受阶段冻结 §4 scope；不为旧 v7 Run 回填或重新解释 scope，不增加 per-Run EXE SHA 权限或新旧停止引擎选择器。

选择原因是停止保证和停止回执语义不兼容，不能只在同一旧 profile 上按新 EXE 改判成功。新版本在 worker/owner/远端调用前拒绝 v7 及更旧目录，不迁移、清理、修改旧记录或代原版本收尾；旧任务和历史由原版本读取。升级顺序仍为原版本排空全部原生资源及未完成操作/控制，停止原 writer，再使用新版本/目录。pr_ready 和 service stop 均不足以证明排空。

该选择有实际成本：默认目录、storage/header、客户端发现、schema/capabilities、fixture 和操作说明已同步，旧历史不自动出现在新目录。此成本计入本批净改动与收益。v8 已在本次源码实施，不自动升级安装或搬迁用户目录。未来兼容且已排空的升级仍可复用目录，不因 EXE 字节变化再次切版本；不建设 v7/v8 兼容矩阵。

## 7. 收益与实施门槛（OLCS-05）

实施前记录当前 stop、verify-stopped、服务结算各自必需的外部文件/查询/检查点及分支，并记录基线计数。实施后至少证明：

1. 本路径三类结算入口对签名退出文件的强制读取/验签次数均为 0；对 control Closed 和 pending 查询响应的停止前置依赖均为 0。不能仅把它们移入服务退出、离线核验或升级检查。
2. 最少删除“签名退出文件决定停止成功”和“控制关闭/reader 响应决定停止成功”两类专属规则及对应失败分支；保留共享签名机制不计作删除。
3. 登记资源已结束但证明文件缺失/损坏的同一受控场景，旧版不能完成结算，新版在 scope 内完成；原对象仍活动时两者均不得误报。记录查询/写调用数与实际耗时，不用模型耗时推导改造收益。
4. 列出包含格式/投影/客户端在内的生产代码新增、删除、净变化及必改模块；仅移动代码不计收益。目标是停止专属生产代码净减少；若新增适配和版本成本抵消了全局净减少，必须如实列出，不能宣称全局净删或显著提速。若两类规则实际上仍为必需门禁，或需新清理账本才能成立，则缩小/重审方案，不能标记本目标完成。

## 8. 最小验收

以下为验收要求，实际测试名、构建、结果和局限逐项登记于[验收报告](../acceptance/orca-local-codex-stop-simplification.md)。新增断言复用现有夹具和原 case。

| ID | 必须证明 | 最小方式 |
| --- | --- | --- |
| OLCS-A00 | 本文/Plan/长期契约一致；新旧范围、v8 成本和实施状态明确 | 文档及本地链接检查 |
| OLCS-A01 | 新范围正向、证明文件缺失/损坏和 pending control 查询不再阻塞；helper/根进程仍活不能成功 | adapter 原终端/进程夹具，精确读写调用数；不修改运行期业务查询验签标准 |
| OLCS-A02 | 资源及清单反例、PID/terminal 复用、缺 readiness、初始化阶段、迟到创建/未知创建不误报；实际 Job 工具终止仍执行 | 原协议完整清单反例 + 本机 Job 子进程夹具；不靠固定 sleep 判定退出 |
| OLCS-A03 | 持久化失败零派发；stop 与下一输入竞争后零新业务输入；迟到结果不续权；其他 Run 不受影响 | application 定向屏障/事务断言，至少包含另一个 Run 的原资源保持活动 |
| OLCS-A04 | stop 受理/关闭响应丢失及服务重启后只读原对象；全部缺席可结算，部分仍活保持 unknown；同 ID 回放、新 ID 拒绝均不重复动作 | 原操作及公开 CLI 受控夹具，固定原身份并统计所有 stop-request/close/create/send；覆盖辅助关闭内部重试 |
| OLCS-A05 | 显式停止、服务关闭核验、离线 verify 使用同一 scope；离线零写入/零隐式启动，资源仍活/写锁占用均拒绝 | 一个公开受控闭环及 host 只读/锁回归；前后 DB、原命令和失败回执字节/摘要对照 |
| OLCS-A06 | v8 拒绝旧目录且零原生调用；新目录及 scope 投影一致；旧停止范围不被重释；相邻后端语义保持 | storage/CLI/客户端配置合同 + 直接 Codex、原生远端、Pi/DSH 受影响组件回归，不要求全 Agent 真机矩阵 |
| OLCS-A07 | 真实本机 Orca Codex 仍能实际停止；没有签名/Closed 证明时，公开离线核验可按新范围成功 | 一条隔离原生停止探针，复用既有 native stop probe；原业务/控制终端及已知 helper/根身份逐一核实，见 Plan |
| OLCS-A08 | 两类规则确实删除且未迁移；计数、净改动和局限可核对 | OLCS-05 收益清单，绑定实际源码/构建及上述结果 |

A01–A06 使用受控夹具验证可重复边界，A07 只验证真实后端接线和实际清理。无需真实 PR、合并、审计 finding、全仓 L1/Controlled 或全部 L2/L3 作为本批前置。若改动实际越出范围，补对应最小验证并登记，不能借“纯停止”免验真实机制变化。

## 9. 完成与发布边界

本次 A00–A08 按专项范围完成，长期契约和现行操作说明已更新。真实探针使用隔离注入的当前 Run 绑定真实 Orca/Codex readiness，经公开 stop 和两次离线核验；业务输入 0，不是完整任务/PR 交付验收。安装、用户现场排空、旧数据处理、外仓集成、合并和发布未执行；历史验收证据不改写。

## 10. 再审计补正

停止、scope 读取和关闭核验只消费严格解码且来源绑定的资源身份、启动事实及当前创建状态，不以历史业务回合/reader 证明或控制查询 READY 为成功门禁。业务回执异常继续由正常业务路径拒绝，不借资源停止晋升为业务完成。迟到创建对账只更新本次读取快照的已提交资源/操作事实，原 stop 意图、输入和目标不改写；上层及 adapter 共用 15 秒预算。再审计修复与最终验证见[报告](../acceptance/orca-local-codex-stop-reaudit.md)。
