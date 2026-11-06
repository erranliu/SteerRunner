# 单事件确认与旧环境分支收口：实施验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-02 已完成[实现再审计](event-ack-reaudit.md)，最新精确提交边界、消费者/共享夹具和格式复核见该报告；以下保留初轮实施时的验证结果与交付状态。

日期：2026-11-02。结论：按 [Spec](../spec/event-ack-cleanup.spec.md) / [Plan](../spec/event-ack-cleanup.plan.md) 设计审计后完成 A/B，EAC-00–06 专项验收通过。基线为含 #83 的 main `72b589baebb47c248fe0845352f400237cc0ed02` 加本批 dirty 差异，实际源码以[逐文件身份](../../../../acceptance/evidence/event-ack-cleanup-20261102/source-identity.json)为准。未提交、部署或发布，无新增完整 L1/L2/L3 或真实模型资格。

## 1. 改动与实际收益

| 增量 | 生产新增 / 删除 / 净减少 | 结果 |
| --- | --- | --- |
| A 单确认/通用字段 | 5 / 43 / 38 行 | 事件 Persisted/Processed 收敛为原 Persisted；通用事件13→5字段，删除未消费标签/候选/投递透传，保留真实事实和确认；trace 从事实推导同一分类 |
| B 旧环境/零消费者声明 | 0 / 67 / 67 行 | 删除从未赋值的 BindingSnapshot 环境/权限分支，以及 Observation/OutputBatch/EventBatch/LifecycleResolver/CleanupStepLimit |

生产合计净减 **105行**；测试新增197/删除12行，包含两项真实边界回归，不计生产收益。[差异明细](../../../../acceptance/evidence/event-ack-cleanup-20261102/production-diff.json)。每个 Orca 事件从两个通道/等待减为一个；维护和夹具协调面减少，没有同条件耗时测量，不承诺业务提速。

## 2. 确认边界和实际消费者审计

[设计审计](../../../../acceptance/evidence/event-ack-cleanup-20261102/design-audit.json)在生产差异为空时完成并保存原 Spec/Plan 摘要和编码基线。确认两个通道在 worker 只是 acceptEvent 返回后的同一个 error，owner 在同一 journal drain 点回复两次 nil，没有独立第二提交阶段。

实施仅删除第二回复/等待；原 Persisted 回复位置保持：

- 持久事实仍在原 acceptEvent/commitLocked 返回后回复；保存失败回复错误，内存和持久事实不提前发布。重复事实沿原去重规则处理，不要求新增事务。
- PublicMessage/忽略事件沿原处理语义回复；nil 不表示每次都有新 Run 事务，不将可选消息升级成业务保存门禁。
- 独立 Codex owner 仍在原生 Observe 已写入并 Sync journal 后的原 drain 点回复；不声称拥有 Runner DB 提交权。原 journal、checkpoint、拒绝/转发和事件选择正文保持。
- 原 Orca 错误回复仍停止本次观察、不确认消息游标，取消打断等待，Close 非阻塞、awaitCleanup 有界；不自动补 nil，不重发业务输入。

9处 Processed 组件/live/探针夹具只删除第二回复，原队列恢复、原发送次数、结果/来源和停止断言未改。short_input_trace_test 的 Kind 是实际诊断消费者，已保留原 JSON key 和 turn/execution 分类，以 Observation/Fact.Kind 推导；没有删除整份探针。底层 Codex ObservationEvent.Kind/Status 仍被异步问答测试读取，原类型、生产和测试正文未改。

B 的唯一生产 BindingSnapshot 构造是 ApplySessionReceipt，原先没有设置 Environment/PolicyResources。删除对应零入口分支后，真实 Target/Workspace/Process/Session/Implementation 身份验证、SameProcess、合并失败返回原值保持。没有删除真正的 Snapshot.PolicyResources、Receipt.OriginalHost、Host/Probe/Delivery 等持久同名字段，也未触及 prepare/qualify、usage 租期或双操作表示。[消费者与夹具复核](../../../../acceptance/evidence/event-ack-cleanup-20261102/consumer-audit.json)

## 3. 持久格式和保留实现

LifecycleEvent/BindingSnapshot 为进程内值；现行 owner RPC 使用 Receipt，原生 journal 保存原 Envelope，未保存被删的事件标签/通道或旧环境模型。state/v8、SQLite user_version=8、CLI/Workflow/owner/native 协议、全部持久字段/动作/decoder、Intents/Snapshot.Operations 和 HistoryBinding 保持。

实施前后固定捕获选定持久类型（包含 EventFact/TurnEvidence 等）、代表性 Run/snapshot/request/receipt/operation/HistoryBinding、公开 capabilities/资源摘要，**145116字节完全相同**，SHA256 `47d983e8958718ee57ba1a13f00e3504c51ff6268e964062f2ad6ef35c471367`。[比较](../../../../acceptance/evidence/event-ack-cleanup-20261102/wire-comparison.json)

8个生产变更文件中177个保留声明完全相同（含52个类型），仅9个授权声明调整、7个零入口声明删除，没有新生产声明。授权调整为单确认/通用字段、旧环境分支；逐项 diff 和行为回归核对，不以 allowlist 本身当语义证明。[声明比较](../../../../acceptance/evidence/event-ack-cleanup-20261102/declarations.json)。六个关键消费者文件（含 acceptEvent、acceptMessage、原生 Observe、实际 Orca Lifecycle、停止和 ApplySessionReceipt）原正文未变；9份原夹具的预期变更也与基线逐字核对。

这些证据共同支持持久格式和保留承诺结论，不将单个编码样例相同代替持久根审计。没有增加恢复框架、兼容字段、能力开关或新派发路径。

## 4. 最小验收

| 项目 | 实际结果 |
| --- | --- |
| EAC-01 worker/SQLite 新回归 | 1顶层/4子用例通过：处理未完成无 ack；提交成功时事实已落盘；提交失败回复错误且无部分发布；重复/可选消息在事务故障下仍按原语义处理 |
| EAC-02 实际 Orca observer 新回归 | 1顶层/3子用例通过：未确认不推进，一次 nil 即继续，错误确认原错误退出且无后续事件，等待期间 Close/awaitCleanup 可结束；既有 queue cancellation/发送次数回归通过 |
| EAC-03 组件 | `execution/...` + runstate/contracts/resources/storage，exit 0；446顶层/1343子用例通过，8个 opt-in/helper skip、3个无测试包单列 |
| EAC-03 application | exit 0；46顶层/81子用例通过，0 skip；包含新确认边界及原操作/人工恢复/观察、保存失败、预算、迟到创建/结果、本机停止和普通重启授权 |
| EAC-04 live 编译 | native/integration 各 `go test -c -tags live` 通过，包括 trace 与正式 Processed 消费者；不运行 TestMain 零匹配 |
| EAC-05 最终公开链路 | NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership 三场景通过，3/3，0 skip；成功包含原隔离夹具和资源清理 |
| EAC-00/06 | 设计先行、分项净删、编码/消费者/声明、稳定源码与最终产物、精确命令/失败归档及长期文档/索引完成 |

组件加 application 共 **492个顶层通过**；两项 focused 新回归包含其中，重复执行不累加。[精确统计/每包结果](../../../../acceptance/evidence/event-ack-cleanup-20261102/test-summary.json)、[命令及退出码](../../../../acceptance/evidence/event-ack-cleanup-20261102/commands.json)。默认 skip 不计通过资格。

最终 scripts/build.ps1 构建 Runner SHA256 `8e5ab60d20fbf952742e878a4a3013d1a896595c57890796700f0ccee73e2ce3`，source_digest `5b08c0ed3d8e3ca39e408501706e7b2496bc71b329bd5cec1017594314fde3a3`；[构建](../../../../acceptance/evidence/event-ack-cleanup-20261102/runner-build.json)绑定680项源码/构建输入和三份产物。三个公开场景使用同一最终 EXE，源码冻结后未再编辑生产或测试。

## 5. 首轮失败和限制

新 observer 回归首轮 single_success 使用零值 Lifecycle/Snapshot，原合法入口校验使其在首个确认后结束，未得到预期后续事件；该新测试因此失败。改用现有 fixture(t) 的合法原准备绑定后通过，没有放宽生产校验；原 JSONL 和[说明](../../../../acceptance/evidence/event-ack-cleanup-20261102/first-failure-note.txt)保留。application 新回归、编码、整组、构建和公开场景未发生失败。

本批保留原执行/恢复/停止责任和可选观测语义，只取得变更机制的专项资格；未跑真实模型/helper 或新后端组合，不继承完整 L1/L2/L3。未操作用户已有 Run/Orca worktree，未提交/推送、创建 PR、部署或发布。
