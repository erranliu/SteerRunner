# Plan：Orca CLI 等待、读取重试与公开状态

日期：2026-10-25。状态：P0–P2 已实施；L1/定向受控通过，P3 完整验收被 Windows Defender 拦截。依据 [Spec](orca-cli-wait-retry.spec.md)，结果归[验收记录](../acceptance/orca-cli-wait-retry.md)。源码调查基线 `af5e244ddae05d4c19069852d56c5a74a68ab1ce`。

## 1. 独立交付与结构约束

Runner 自身完成竞争下自动准备恢复、有界读取及可解释查询；不需要 SteerLine 源码、EXE、DB 或浏览器。交接公开 schema/样例供任意调用方消费，UI 接线不是本仓完成条件。

仅扩展 SerializeCLI、两项准备读取、原 Run/operation 和公开投影。删除被替代的 repository_query 重试循环。不新增 dispatcher 服务、加权队列、后台合并器、采样退避、wait 表/revision/恢复入口；不改变写操作期限或 unknown 规则。

## 2. 实施步骤

### P0：固定边界与反例

- 固定实际 source/EXE/resource；盘点 SerializeCLI 与 owner 子进程，确认无父持锁、子等锁。
- 在独立 fixture 重现“未调用就于旧 15 秒阈值阻断”；关联当前 operation，不生成查询 ID。
- 固定公共 schema、queue/calling/backoff/exhausted/null 正反例；核对 PreparationDeadline、Engine.invoke 的 90 秒 context、preparationQuery gate/revision 和聚合 commit/clone 行为。

### P1：原入口公平等待与单层读取策略

- SerializeCLI 内实现取消安全的同进程 FIFO 门，保留命名互斥；后来调用不得插队，退避不占门。无优先级或跨进程队列。
- 保留队列未调用的类型化事实，无输出时不解析 JSON。普通命令现有期限不变，仅两项准备查询分开排队窗口与执行期限。
- workspaceRepository 的 remote repo list / local status 使用一层策略：复用原父调用的 90 秒窗口上限、单次最多 15 秒、最多 5 次、1/2/4/8 秒退避；与原准备/caller context 期限取最早者并公开实际 deadline，不延长父操作或写操作预算。
- 严格分类暂态和确定错误，删除 cache-conflict 私有循环。结果仍须原 runtime/仓库完整性、唯一性和授权核验。

退出：A01/A02/A03 组件反例通过；原 observer/采样频率、owner 路径与写操作策略不扩展。

### P2：持久事实、公开投影与恢复

- 在原 Run/operation 仅增加缺失等待事实，复用 ID、deadline、事务和事件。取得通道后、调用前持久提交保守计数；不建表或核验子操作。
- 用类型化进度接缝交给 application 提交，protocol/orcatransport 不导入 application/runstate/storage；优先最小扩展现有 ReadOnlyPreparation，不并存第二准备 API。
- 自有等待更新引起的 revision 不得被 preparationQuery 误判 stale；保留真实 stop/授权、目标/输入、资源身份及并发检查。最终提交合并当前聚合，防止旧 clone 覆盖等待/控制。
- 恢复原 deadline/次数/next time，过期即耗尽；成功清除公开显示后，父操作未结算前仍保留原预算，已有精确创建计划不重新选仓库。只按原授权继续，取消查询后在 gate 内重新核验，旧答案/resume、未知副作用零重发。
- capabilities/schema、inspect/observe/history 对象/null 和原事件同事务更新；GET 不发 Orca 命令。实现耗尽、清除和 stop/其它 blocker 优先级。

退出：A04/A05/A06 通过；没有自有进度误阻断、旧 clone 回写或重启预算清零。

### P3：本仓验证与文档收口

- 按[测试指南](../testing.md)扩展原组件测试；新增受控 `TestSROrcaCLIWaitRetry`（计划名，实施后注册 catalog/清单），以真实服务/SQLite/公开 CLI 验证。
- 复用原 native driver/fixture 新增 local / remote 等待 case，计划 ID 为 orca-local.cli-wait-retry、orca-remote.cli-wait-retry；同步唯一 case 定义、catalog 与清单，不复制 harness。
- 同用户 CLI cache 是共享资源；与其它同 cache 测试串行或证明独立 OS 用户/cache。独立 state-dir 不足以证明隔离，不向生产 Run/锁注入故障。
- 同步 CLI/恢复/测试文档，记录构建、结果、原命令和清理。交接公开 schema/样例即可，不加入 SteerLine 联验步骤。

## 3. 改动归属

| 位置 | 必要变更 |
| --- | --- |
| protocol/orca cli_lock/client 与调用边界 | 同进程 FIFO/取消、原互斥；队列错误不解析空 JSON |
| orcatransport repository_query/workspace/lifecycle | 两项准备读取的单层策略/进度；删除旧循环、保持 side-effect owner |
| application preparation_query/preparation、原 operation/runstate/storage | 原授权下持久等待与取消/恢复，自有 revision/当前聚合合并 |
| contracts/view、resources、原 host/CLI | 最小 schema/同快照字段与原事件；无第二服务 |
| 测试/catalog/文档 | 独立反例与 local/remote 资格；无 SteerLine 依赖 |

字段/接缝以 P0 真实缺口决定，不为其它命令预建通用框架。

## 4. 最小验证映射

| 验收 | 环境与入口 | 时点和通过证据 |
| --- | --- | --- |
| A01/A02/A03 | `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout 5m` | P1；隔离 FIFO/取消/互斥、预算分类与调用数；至少一次真实 >15 秒持锁正例，其余用受控时钟 |
| A04/A05/A06 | `./scripts/test-foundation.ps1 -Pattern '^TestSROrcaCLIWaitRetry$' -OutDir <新绝对目录>`（新增测试后） | P2；真实服务/DB/CLI，重启/stop、自有 revision/真实竞争、原 unknown/副作用、投影事件 |
| 固定回归 | `./scripts/test.ps1 -Level L1 -OutDir <新目录>`；`./scripts/test.ps1 -Level Controlled -OutDir <另一新目录>` | 实现提交前；按本仓入口完整通过，零匹配/skip/超时不算通过 |
| A07 | 注册后 `./scripts/test.ps1 -Level L3 -Scenario orca-local.cli-wait-retry,orca-remote.cli-wait-retry -Config <独立配置> -OutDir <新目录>` | P3；同 Run 原生竞争→恢复/取消、零重复输入与原资源结算，分目标记录 |
| 核心链路 | `./scripts/test.ps1 -Level L2 -Config <独立配置> -OutDir <新目录>` | 交付实现前；本构建登记核心集合及清理完整 |

独立输出 `C:/SteerlineDebug/runner-cli-wait-retry/<唯一执行ID>/`，native scratch 沿框架在 `C:/srt` 分配；失败与补测分开。不以受控或另一目标的通过替代缺失原生资格。

实现与功能测试仅在本仓独立现场运行，不部署、不修改 SteerLine、不恢复生产现场；调用方联验另行安排。原设计轮次的文档检查不作为本次功能资格。

## 5. 实施定位（2026-10-25）

- P0/P1：SerializeCLI 统一进程内 FIFO 和原跨进程命名互斥；PreparationCall 仅接受 repo list/status，在取得通道后提交计数并启动 15 秒执行期限。旧 repository_query 私有循环已被同一 90 秒/5 次/1、2、4、8 秒策略替换。
- P2：预算放在原 operation.Intent.PreparationQuery，ReadOnlyPreparation 的 context 进度接缝负责提交；未建立新操作、表、revision 或恢复服务。自有提交推进局部预期 revision，其它变化仍拒绝派发。invoke 在当前聚合结算，stop 受理与清除同事务；迟到的取消读取不能覆盖 stop 状态/诊断。
- 恢复只在原绑定/授权有效且创建目录尚未声明时继续原读取；已有目录只读原创建计划与回执，目录不完整仍保持未知。成功隐藏投影后原预算仍保留；停机取消读取不把 Run 授权变成人工恢复。
- P3：已注册 Controlled TestSROrcaCLIWaitRetry 与 L3 orca-local.cli-wait-retry / orca-remote.cli-wait-retry。沿原原生 fixture、转发 journal、停止和离线结算驱动；测试调度器为这两个共享 CLI cache 的 case 取得独占测试 lease，故障互斥使用新 fixture 的私有 cache key，不持生产 key。

执行目录、准确命令、实际结果和清理持续登记到[验收记录](../acceptance/orca-cli-wait-retry.md)。
