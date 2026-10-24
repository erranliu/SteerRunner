# Spec：Orca CLI 等待、读取重试与公开状态

日期：2026-10-25。状态：已完成设计审计，功能未实施。配套 [Plan](orca-cli-wait-retry.plan.md) 与[验收规划](../acceptance/orca-cli-wait-retry.md)。本次只提交文档，不部署、不恢复生产 Run。

## 1. 目标与独立收益

源码调查基线 `af5e244ddae05d4c19069852d56c5a74a68ab1ce`。现有 repository inventory 把排队时间算入 15 秒预算，命令尚未调用便可能阻断准备；MetaCallWithTimeout 还会解析空输出，附加无关 JSON 错误。CLI 共用命名互斥，密集观察调用可以挤占准备读取。

Runner 单独通过 submit / inspect / observe 使用时，也应在短暂竞争消退后自动继续准备、有限重试暂态读取，并解释为什么等待。本仓实现与验收不依赖 SteerLine checkout、Task、数据库、Web 或调度器；任意 CLI 调用方均可受益。调用方 UI 联验不作为 Runner 自身完成前置。

首版只扩展两个明确只读的准备查询：远端 repo list（repository_inventory）与本机 status（runtime_status）。复用 workspaceRepository、ReadOnlyPreparation 和当前操作意图，不新建查询操作 ID。共用 CLI 入口增加同进程 FIFO，使后台后到调用不能越过已排队的准备请求。

创建、启动、业务输入、停止及未知结果核验继续沿原节点和[服务换代契约](service-generation-recovery.spec.md)。本项不改变其期限、次数或授权，不增加副作用重试；不改其它观察/进度/消息采样频率。不新增通用调度框架、优先级系统、第二恢复状态机或调用方专用分支。

## 2. 分类与预算

| 事实 | Runner 行为 |
| --- | --- |
| 准备查询尚未调用，等待进程内门或命名互斥 | 保持 queue，在原窗口内等待；不消耗查询次数，不转 blocked |
| 上述只读查询已调用，暂时传输/超时或精确识别的 Orca 缓存替换冲突 | 在同一读取循环内退避，重新查询原目标 |
| 登录/权限/信任、非法配置、缺能力、仓库缺失/歧义、身份冲突或确定拒绝 | 沿原 blocker/blocked 处理，不循环尝试；不能把所有 exit code、解析或未知错误视为暂态 |
| 创建、启动、发送或停止可能已调用但回执不明 | 沿原意图核验原结果；查无、空输出和超时都不授权重发 |

CLIQueueError 必须保留为类型化的“外部回调未执行”事实。此错误且无输出时不解析 JSON；真实调用后畸形响应保留原错误与副作用边界。适配层按实际命令/错误报告暂态资格，application 沿原准备授权消费；不扫描诊断正文决定是否发出输入。

| 项目 | 首版固定策略 |
| --- | --- |
| 一个准备查询的总窗口 | 上限 90 秒，沿用现有父调用上限；从首次进入查询起包含排队、调用和退避，与原 PreparationDeadline、caller context deadline 取最早者 |
| 单次只读执行 | 最多 15 秒，从取得 CLI 通道、进入外部调用边界起算，且受总窗口剩余时间限制 |
| 调用次数 | 最多 5 次，包含首次；未取得通道、取消、内部唤醒不计调用 |
| 失败后退避 | 1、2、4、8 秒；不增加 jitter 或多层退避 |
| 其它命令 | 保持原期限、计数与副作用规则；不套本项 120 秒窗口或 5 次重试 |

取得通道后再次核验取消、原操作/目标及当前授权。没有剩余时间不创建 CLI 子进程。超时的只读子进程先由原 owner 核实退出/回收，再开始下一次读取。退避不持 CLI 锁。

当前 Engine.invoke 已有 90 秒调用期限，本查询复用这一上限并公开实际更早的 deadline，不增加另一套更长预算，也不延长父操作或写操作期限。需要拆开的是查询内部把等锁也算作 15 秒执行时间的包装，取消和更早的父期限仍有效。

calls_started 是持久提交并进入调用边界的保守次数；崩溃可能发生在提交后、CLI 创建前，非零不能证明外部动作已执行。它只计此查询，不是业务 Attempt、模型启动或当前准备操作的副作用调用次数。

移除 repository_query 原单独缓存冲突重试循环，收敛为一层读取策略；transport/application 不另套重试。结果仍须通过原 runtime/仓库完整性、唯一性和授权核验，读取成功不自动授权后续写操作。

## 3. 通道、持久化与取消

### 3.1 最小公平入口

在现有 SerializeCLI 内增加取消安全的进程内 FIFO 门，不新增 resident dispatcher。后来的同进程调用不能越过已等待的调用，取消项及时出队；全部原调用方使用同一入口，不为准备另开快速通道。

保留原 cache key 对应的跨进程命名互斥与 OS 线程所有权。进程内门覆盖取得互斥和一次 CLI 调用；不得持锁等待模型或重复轮询。只读准备沿已有 ReadOnlyPreparation 释放 worker gate，不能阻塞服务查询、停止受理或其它 Run 事务；允许原 Run 的可取消查询 goroutine 等待，不为消除等待 goroutine 重构 worker。

创建 owner 子进程保留原互斥路径，父进程不得持锁等待也要取锁的子进程。跨服务/owner 进程不承诺全局 FIFO，不绕锁、删锁或杀无关进程。不增加加权优先级、后台合并器或统一采样退避。

### 3.2 原记录与恢复

在本查询所属原 Run/operation 聚合保留起点、deadline、次数、next_attempt_at、阶段与原因；最多一个当前准备查询。复用原操作 ID、PreparationDeadline、事务和历史事件。只增加确实缺失的字段，不建 wait 表、重试账本、子操作、第二 revision 或窗口历史库。

重启恢复原查询/目标、deadline、次数和下次时间，不重给 90 秒；停机时间计入绝对期限。运行中用单调时间，恢复按原 UTC deadline 限制；检测到明显时间倒退时沿现有诊断保守停止自动派发，不新增时间恢复协议。正常服务关闭仅取消本次查询，不等于用户撤销原 Run 授权；重启后仍按持久授权和既有换代资格决定是否继续。

等待提交也会推进 Run revision。preparationQuery 不得把本操作进度更新误判为外部竞争；也不得因此跳过 stop、目标、输入/授权、资源身份和真正并发变化的核验。原提交路径读取并合并当前聚合，避免旧 clone 覆盖等待/控制事实。

stop 在排队/退避期间撤销派发资格并及时取消，取消后零新查询和零后续创建/输入。已有资源按原 stop 契约结算，取消等待不证明资源已停。旧 epoch 答案/resume、未知创建/发送及原绑定不放宽。

成功清除公开等待并自动继续原准备，不生成人工恢复问题；原查询预算事实保留在所属操作直到该操作结算，不能在清除显示后、父操作提交前崩溃时重新给同一查询窗口。已有原创建计划记录了精确仓库绑定的，继续复用原计划，不重新选仓库。预算/次数耗尽沿原 blocked/recovery 处理并保留原因；适用原 resume 时沿其幂等/授权规则继续，重复请求不生成多个窗口。确定错误沿原 blocker，不再建立平行 wait 状态。

## 4. 公开最小契约

唯一契约名为 `steerrunner-execution-wait/v1`。capabilities.execution_wait 发布 contract、same_snapshot=true、run_property=execution_wait 和自包含 schema，无远程引用；schema/实际输出同源验证。实现不等待任何调用方改造。

inspect.RunView、observe.run 和 history 的完整 RunView 返回 execution_wait 对象或显式 null。它是原聚合的准备查询投影，不是第二状态机。status/phase/controls、operation 状态、unknown 与资源事实保持权威，不再增加 disposition/effect/dispatch_state 等重复字段。

| 字段 | 约束 |
| --- | --- |
| contract | steerrunner-execution-wait/v1 |
| operation_id | 当前既有准备操作 ID，不是新查询操作 |
| query | repository_inventory / runtime_status |
| stage | queue / calling / backoff / exhausted |
| reason_code | orca_cli_busy / orca_query_running / orca_query_transient / wait_budget_exhausted / query_attempts_exhausted |
| started_at / updated_at | UTC RFC3339，原窗口起点/最后持久更新；GET 不刷新 |
| deadline_at | 本窗口、原 PreparationDeadline 与 caller context deadline 中最早的 UTC 时间 |
| calls_started / max_calls | 本查询保守边界次数/上限 5，不是准备写操作次数 |
| next_attempt_at | backoff 的实际下次读取 UTC 时间，其余阶段 null |
| diagnostic | 原原因，去敏且最多 1024 UTF-8 字节；详细错误归 owner 日志 |

queue 对应 orca_cli_busy，calling 对应 orca_query_running，backoff 对应 orca_query_transient，exhausted 对应两种耗尽码之一。queue 也可能是重试前重新取得通道，不能推断 calls_started=0；没有调用时才为 0。backoff 至少调用过一次且尚未到上限；耗尽不自动安排读取。

```json
{
  "contract": "steerrunner-execution-wait/v1",
  "operation_id": "original-preparation-operation",
  "query": "repository_inventory",
  "stage": "queue",
  "reason_code": "orca_cli_busy",
  "started_at": "2026-10-25T00:00:00Z",
  "updated_at": "2026-10-25T00:00:00Z",
  "deadline_at": "2026-10-25T00:01:30Z",
  "calls_started": 0,
  "max_calls": 5,
  "next_attempt_at": null,
  "diagnostic": "Orca CLI channel is busy."
}
```

正常等待保持原 queued/running 等合法状态，预算内不设置 ResumeRequired。必要时 phase=execution_wait，仅由本查询拥有；成功/取消/确定错误清除当前对象，耗尽暂保留 exhausted。stop、人工问题、原 unknown 和终态优先，wait 解除不恢复旧状态或覆盖其它诊断。模型是否启动不能由此对象推断。

沿原流增加 execution_wait_changed，Event 顶层 execution_wait 为对象或清除时 null，其它事件不附带。无通用 payload、专用端点或第二游标。对象、事件、原 phase/controls 和 Run revision 同事务；重投幂等。进入/改变阶段、调用、安排退避、耗尽和清除才写事件，无每秒 tick。GET 只读原快照，不调用 Orca、不重置预算。

不提供队列排名、确定 ETA、锁持有者 PID、其它 Run 信息或新配置页。旧开发记录不迁移、双读/双写或按错误正文补字段；格式影响沿原兼容预检明确判断，不改写生产 Run。

## 5. 验收与独立完成定义

| ID | 通过条件 |
| --- | --- |
| OCW-A01 | 占锁超过旧 15 秒、有效窗口内释放，原准备自动继续一次；查询服务可用，无人工 resume/替代 Run/会话 |
| OCW-A02 | 两种准备查询按单层预算/次数/退避执行，永久错误立即返回；未调用不计数、不附带空 JSON 错误 |
| OCW-A03 | 同进程后台不会越过已排队前台，取消项出队；跨进程互斥、owner 无重入死锁；不改采样频率 |
| OCW-A04 | 重启保持原身份/预算；自有 revision 不误伤，真实 stop/授权变化仍阻断，旧 clone 不覆盖新事实 |
| OCW-A05 | 能力/schema/inspect/observe/history 一致，状态/事件/controls 同事务、清除/优先级/重投正确，GET 零副作用 |
| OCW-A06 | 原创建/发送/停止期限和 unknown 核验不变，stop 后零新增输入；资源义务不被等待解除 |
| OCW-A07 | 独立 Runner CLI/服务和 Orca local / remote 验证真实竞争、自动恢复、取消及清理，分别记录目标资格 |

全部证据由本仓生成、核验和登记。SteerLine 或其它 UI 接线另验，不阻碍满足上述条件的 Runner 独立交付。当前仅文档，不宣称功能通过。
