# Spec：执行与可靠性边界收敛

日期：2026-10-20。状态：已实施并完成本轮验收。对应 [Plan](execution-boundary-refactor.plan.md)。调查基线：`bf3ec581cb0b23a7358d95e5efa8ff5f607c2f55`，包含内容流程、R2 证据和服务换代恢复的合入。

## 1. 目标与契约依据

保持 Workflow → ReliableExecution → ExecutionAdapter 的职责方向，收拢新增能力带来的重复派发、恢复与结算规则，并消除证据读侧对执行控制的全局锁阻塞。完成标准是规则有明确所有者、实际调用者复用同一规则、原行为可验证；不以新增包数、接口数或文件行数验收。

本轮是内部重构，不重新定义执行与恢复行为。依据为当前[服务换代恢复](service-generation-recovery.spec.md)、[结果获取](result-acquisition.spec.md)、[内容流程](content-workflows.spec.md)、[R2 证据](run-evidence-r2.spec.md)及[既有可靠层模块化](../archive/2026-10/spec/reliable-modularization.spec.md)。其中旧模块化文档的 state/v2 和“换代统一禁止新输入、要求 resume”是历史范围；本轮沿当前 state/v5 和服务换代契约，不恢复旧策略。现有能力缺口和未验资格沿[服务换代验收](../acceptance/service-generation-recovery.md)保留，不因本轮结构整理改记通过。

文档轮仅编写 Spec/Plan；2026-10-20 用户追加“实施吧”，已授权本轮代码、隔离构建与必要验证。部署、安装环境及发布/合并不在范围。实际实施和证据见[验收记录](../acceptance/execution-boundary-refactor.md)。

## 2. 已确认的问题与范围

| 编号 | 基线位置与事实 | 本轮目标 |
| --- | --- | --- |
| EBR-F01 | [application/artifact.go](../../internal/application/artifact.go) 的 HandleEvidence 在持有全局 e.mu 时做 Git、目录校验和文件读取；collect 还持有所属 worker.gate | 锁外采集，锁内复核/提交；无关 Run 不等待这段外部 I/O |
| EBR-F02 | [application/parser.go](../../internal/application/parser.go) 自行推进 pending/in_flight、容量、执行、对账、取消和回执，直接调用 port.Execute 或 parser.Start；普通操作走 operation.Invoke | 共用可靠性规则和宿主派发边界，保留 parser 专属资源与异步回执机制 |
| EBR-F03 | [control.go](../../internal/application/control.go)、[feedback.go](../../internal/application/feedback.go)、[content.go](../../internal/application/content.go) 重复编排原 session 核验及必要的 Attempt 恢复 | 统一资格规则与核验/恢复协调，业务输入选择仍由原业务入口负责 |
| EBR-F04 | [codex/remote_models.go](../../internal/execution/adapters/codex/remote_models.go) 按 audit/implement/publish 等 Workflow 步骤选择模型，remote_agent 使用 workflow.ModelSettings | 业务侧生成冻结模型绑定，adapter 只验证绑定并转换原生协议 |
| EBR-F05 | [application/takeover.go](../../internal/application/takeover.go) 直接理解多种私有 codec、owner 能力和 parser package；现[依赖守卫](../../internal/execution/workflow/boundary_test.go)未禁止 adapter 导入 workflow | 私有解码交给实现，宿主汇总接管资格；守卫覆盖实际禁止的反向依赖 |

这些事实说明结构耦合与维护风险，不等于已证明重复执行事故。F01 是明确的跨 Run 阻塞路径，F02–F05 是规则与实现知识的收敛对象。

包含必要的 application/reliable/backend/adapter/组装接线和定向测试调整。保持现单进程协调、独立原生 owner、Run Record、SQLite 和 Store.Commit。排除新 workflow/调度器/恢复服务、第二账本、通用 owner 操作队列、统一 View/Change 框架、操作投影重建、artifact 存储搬迁、状态格式升级、公开接口增量、认证/权限扩展、SteerLine 修改、部署及历史验收重写。

## 3. 职责与状态归属

| 所有者 | 负责 | 边界 |
| --- | --- | --- |
| Workflow 与 application 业务衔接 | 固定材料/提示词、步骤/模型角色、来源适用性、报告/解析/审计/发布路由、内容完成与 history | 不自行决定未知副作用可重发，不实现原生资源恢复策略 |
| reliable / reliable.operation | 原操作身份与不可变输入、首次执行/原操作对账资格、状态/回执更新、未知/冲突、原 session 续执行与恢复资格、通用预算约束 | 只接收窄值/事实及所需端口；无 application/runstate/storage/Workflow/具体 adapter 依赖，无 DB、提交回调或整个 Record 修改权 |
| application 宿主协调 | 命令受理、快照、worker/gate/锁、容量协调、调用端口、资格复核、业务与可靠性组合、原子提交及公开投影 | 是可靠性策略的调用者；不复制规则，不解码私有 envelope；允许同包普通函数，不要求独立新包 |
| backend | 共享纯值和窄端口，包括实际需要的模型值、实现兼容事实和 parser 资源交互 | 无业务路由、存储权或平台 I/O；不变成大公共契约容器 |
| adapter / parser 资源实现 | 原生调用、私有 codec、精确资源身份、session/turn/final/退出证据、只读查询和具体取消 | 不决定下一业务步骤、自动续权、重试未知输入；错误不推断成功 |
| execution owner | 持有原连接、Job/工具树、认证及既有 parser 句柄；保存原生 journal/checkpoint，精确停止 | 不持有 DB/Workflow 游标/Allowed 副本；保留现具体动作集，不增加持久队列或另一恢复策略 |
| host / 组装入口 | 单 writer 与实例围栏、版本文件/owner 生命周期、选择具体实现及接线 | 可以知道具体实现；不成为业务步骤或恢复判定的另一所有者 |
| runstate / storage | 唯一聚合、原编码与事务、命令幂等、事件和不可变证据 | 不在加载时补造身份、授权、操作或后继；不新增派生状态权威 |

Allowed/ResumeRequired/PendingStop/PendingResume、Intents、Snapshot.Operations、Workflow/StageResults/NextStage、来源、期限及命令状态仍使用现字段与原事务。规则搬迁不要求拆分 Record，也不借机使两种操作表示结构全等。worker 的 observer/handle/parserTask 等仍是临时资源，不能替代持久事实。

## 4. 必须保持的行为

| 编号 | 不变项 |
| --- | --- |
| EBR-I01 | 同 state-dir 唯一 writer，request_id 幂等、实例围栏、revision CAS；gate 与撤销保持排序。Run/命令/事件按原边界原子提交，提交成功后才发布状态、确认事实。 |
| EBR-I02 | 外发前固定原 ID/key/payload/digest/绑定并持久化。普通 in_flight/accepted/unknown 不得重发；即使意图提交后实际调用前崩溃，也只对账。parser 原有 pending 是专属尚未派发状态，沿原规则才可转 in_flight；不推广为其他动作的新派发状态。 |
| EBR-I03 | 兼容服务换代保留持久授权、原 owner/session/活动 turn 和未完成控制命令。已授权且可证明未派发的唯一后继可自动接续；原暂停、人工等待、业务阻断、stop、关闭和里程碑保持。换代不自动 restoreOriginalAttempt、不批量 resume。 |
| EBR-I04 | 原资源存活时可接回观察，不要求树先退出。仅原资源确实丢失且旧 writer/工具树缺席、原 session 可用且 idle、未知操作已处理等既有资格满足时，才允许原 thread 的人工恢复；不创建替代 session。 |
| EBR-I05 | stop/close 撤销提交后零新业务输入/答案/后继。已越过发送界限的动作如实对账；迟到事实可提交，不能恢复授权。逻辑撤销、断开观察、原生停止与 writer/工具树退出分别表达，unsupported/unknown 不冒充 stopped。 |
| EBR-I06 | 结果核对原 Run/Attempt/配置/resource generation/session/command/turn/source/input version。原 final/checkpoint/PR 只补收、去重和冲突处理；缺失保持未核实，不采当前 HEAD 替代原完成 checkpoint，不重跑源步骤补证。 |
| EBR-I07 | 冻结材料、模型/effort、沙箱/写根/认证、输入/资源摘要、获取次数、绝对期限和 parser 原 owner 不变。重启/搬迁不刷新预算或权限；报告/解析 fallback 及资源结算门槛不变。 |
| EBR-I08 | conversation 的显式 message/interrupt/close 与 generate 的一次执行/完整 final/schema 校验保持；generate 无补报、独立 parser、发布。interrupt 不成为整个 Run stop，未知输入不重放。 |
| EBR-I09 | CLI/错误/退出码、schema/capabilities、workflow/state/operation/codec/owner 协议和既有持久 JSON 编码保持。新构建 SHA 可变；原冻结数据不重算为新默认。Task/DAG/TaskDelivery 归属不变。 |

如实施发现维持上述语义需要新增字段、版本、调用或改变错误/支持范围，先记录具体反例及最小设计修订，不以“重构”名义混入。既有行为缺陷另列修复与验证；F01 的等待消除是本轮明确目标。

## 5. 各项收敛要求

### 5.1 R2 锁与采集

list/read 读取已提交不可变记录；discover/workspace-git 基于锁内取得的原身份快照，在锁外做有限 I/O，返回前复核所属 Run/工作区身份。与 stop 竞争的只读观察仍可返回带 observed_at 的原采样；不新增“必须 Allowed”门槛，不唤醒 worker、不授予输入权。身份变化时不得把旧样本标记为当前已核实。保留前后身份校验、Git 双样本一致性和 verified=false 的原失败表达。

collect 保留原 request_id 锁和所属 worker.gate 的停止排序：不在本轮改变同 Run collect/stop 的先后关系。锁内取快照后释放全局 e.mu，完成受限路径读取与原身份校验，再取 e.mu 复核 revision/绑定/数量/总字节限制与服务可提交条件，沿 Store.Commit 一次写入 artifact/命令/事件。变化则按现失败/CAS 语义拒绝；未知提交只能查询/重放原请求，不能生成第二份 artifact。同 Run 后到的 stop 可等待当前 collect gate，无关 Run 不得等待其外部 I/O；不承诺 DB 原子提交完全无短暂锁等待。

不新增采集队列、后台补采或文件快照保证；保持原字节/摘要/限额、os.Root 路径约束、stop/工作区删除后已收集证据可读。

### 5.2 parser 与共用操作规则

先区分业务选择、可靠操作和具体资源三段。workflow/业务衔接决定是否解析、源、预算和解析后路由；reliable.operation 集中 pending→in_flight 的资格及状态/回执规则；application 负责容量检查、gate/CAS 和提交后派发；专属端口执行/读取/取消原 parser。使用现已有窄接口或确有必要的小端口，不强迫所有动作走完整 LifecyclePort。

parser 不再自行复制普通操作的固定输入、未知对账和冲突策略；普通操作不接入 parser 的容量、文件路径和进程细节。允许异步的 started/final 两次结算，统一入口不等于同步等待最终回执。保留全服务当前 parser 容量限制及绝对 Deadline，保留原 started.json/receipt.json 和挂起进程放行前身份落盘。

服务退出只脱离可保留的原 owner/parser；显式 Run stop 才按原请求取消，原树结算前禁止后继。原兼容旧路径也按原能力执行，不删除其必要收尾。不得将 owner 改造成通用操作队列，或新增 journal/消费游标绕过原回执。解析失败后的报告 fallback 仍由 workflow 且仅在原资格/次数及资源结算后选择。

### 5.3 续执行资格与宿主协调

reliable 的共同资格函数只消费原执行事实、未知操作/控制约束及是否需要输入等窄值；业务侧提供输入意图及来源适用性。application 的共同协调函数执行原 session 核验、必要 BindRecovery 与锁内复核；不得把 Store 或业务 route 塞进可靠层。

resume/feedback/message 共用该规则与协调，各自保留准入、原问题/目录/source 校验、命令结算、queueInput 和业务结果。恢复 Attempt 的原字段更新、绑定清除/保留、句柄释放顺序和原子提交范围逐项保持；已活动树的观察接回与资源丢失后的人工恢复继续是不同分支。

### 5.4 模型绑定与实现接管

步骤→模型角色的解释由 workflow/业务组装负责；adapter 可携带原 step 字符串作为绑定值，但不得按业务步骤选择模型。优先复用 TurnInput 的已冻结 model/effort。现 Orca local 模型桥需要白名单时，由组装侧根据原冻结配置生成只读绑定，adapter 校验精确匹配；不能仅信任输入中声称的模型，也不能扩成任意步骤白名单。

保留 RemoteAgentConfig、模型提示 envelope、JSON 字段/顺序相关编码、配置摘要和 bridge 原生权限语义；Go 共享值移动到小型纯值位置时，必须证明旧字节/摘要一致。不得把新模型映射写回旧 profile、要求替换在用 owner/helper 或因解耦开放新的步骤。cmd/host 的接线可以依赖两侧；不把 workflow switch 隐藏搬到另一个 adapter 文件。

各实现提供针对原 profile/implementation/权限/owner 的只读解码与兼容事实；parser 实现负责其冻结 package 解码。application 汇总原事实、workflow 执行语义、版本支持和材料完整性，reliable 判断恢复门槛。优先现具体 decoder 和显式接线，不增加动态注册器/兼容策略引擎。材料/文件/进程查询归有副作用边界的实现端，接口不得谎称纯函数。

接管检查不构造活动 worker、不启动连接/资源、不写文件、不发业务输入。仍严格区分 compatible/read_only/unsupported/conflict，保留原拒绝条件、诊断及退役认证/远端 helper 边界；只读取候选声明不能替代实际 decoder。原本未证明的 Orca 创建关联能力仍为缺口。

## 6. 验收条件

| 编号 | 必须证明的结论 | 阶段 |
| --- | --- | --- |
| EBR-A00 | Spec/Plan 对齐基线、现契约、五项范围、职责/不变项、实施顺序和证据状态；本轮差异仅文档 | P0 文档 |
| EBR-A01 | A 的 Git/文件采集被屏障阻塞时，B 的 stop 撤销和事实提交仍完成；同 Run collect/stop 保持原排序；采集期间 CAS/身份变化不误提交；R2 原字节/限额/幂等/查询零原生调用保持 | P1 |
| EBR-A02 | parser 与普通操作复用规则；意图/放行提交失败零新调用，外部接受但结算失败只对账；原 pending/in_flight、容量/Deadline、started/final、重复/冲突/错身份、stop 与迟到回执、结算后 fallback 保持 | P2a |
| EBR-A03 | resume/feedback/message 共用资格与协调；原 session idle/缺席/活动/身份变化/未知操作反例有效，目录/问题/source 规则不丢失；不误将换代转成人工恢复或恢复业务阻断 | P2b |
| EBR-A04 | 兼容换代保留原授权/活动执行；暂停/等待/stop/close/pr_ready 保持；两次重启、原控制/结果重放、提交失败和旧观察均不重复输入或后继、不续预算；原 owner/parser 退出策略不变 | P2、P4 收口 |
| EBR-A05 | adapters 无 workflow 导入及步骤路由，reliable 无业务/宿主/存储/具体实现反向依赖；守卫有负例，实际依赖无环；模型桥仍拒绝错 step/模型/摘要/session，审计后模型恢复与旧编码/摘要相同 | P3a |
| EBR-A06 | 私有解码由实现负责；同原记录的新旧检查结论/拒绝边界一致，检查零创建/写入；未知 codec/owner/package、退役认证、缺 checkpoint/能力等不被放宽；接管报告不授权 | P3b |
| EBR-A07 | 同一最终源码/构建完成受影响规则与受控闭环，存量隔离样本加载→原操作结算→写回→再重启保持身份/冻结值/授权/事实；差异有解释，旧重复入口移除 | P4 |

并发反例使用显式屏障、提交结果及精确调用计数，不以固定 sleep 推断。新增测试不镜像函数实现，而证明外部动作数、持久事实和控制效果。原生协议/进程寿命若完全不变可采用受控验证；触及模型桥、owner/parser 启停/连接/放行等真实机制时须补对应最小原生验证，不能以纯接口搬迁为由豁免实际风险。

## 7. 完成定义

文档交付为 EBR-A00；后续实现完成须五项收敛全部落地并满足 EBR-A01–A07，登记每项实际版本、命令、结果和限制。不能只修 R2 就将本轮标为完成，也不能用 import 守卫通过代替运行时可靠性验收。原生资格未验或外部接口缺口必须明确保留，不扩大本轮支持声明。
