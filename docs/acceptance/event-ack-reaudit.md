# 单事件确认与旧环境分支收口：实现再审计

日期：2026-11-02。结论：未发现新增确认缺陷，无需追加生产或测试代码修复。对象为 main `72b589baebb47c248fe0845352f400237cc0ed02` 加 [Spec](../spec/event-ack-cleanup.spec.md) / [Plan](../spec/event-ack-cleanup.plan.md) 的 A/B 实现；680项源码/构建输入、三份产物和[初轮证据](evidence/event-ack-cleanup-20261102/README.md)摘要全部匹配。最新 main 仍为原基线。本次按用户追加授权提交并创建 PR，不合并或部署。

## 1. 单确认是否仍在原提交边界

本次对 worker、owner 和整个 Orca observe 文件做基线精确变换比较，未仅依赖初轮 allowlist：

- `application.worker.run` 全文除原 Processed 回复块外一致。Persisted 仍在 `acceptEvent` 返回之后，传递同一个 error；没有提前回复或将错误改为 nil。
- `commitLocked` 原正文保持：`store.Commit` 成功后才发布 `e.runs`；保存失败原样返回。`acceptEvent` 的身份/冲突核验、去重及提交路径未改，故持久事实仍在原事务/发布边界之后确认。
- 重复、忽略事实和可选 `acceptMessage` 沿原返回语义，不为成功确认强制新事务；optional message 的 nil 不被重新解释为新增 Run 提交。
- `ServeExecutionOwner` 全文除原 Processed 回复块外一致，仍在原 drain 点回复 Persisted=nil。新原生通知先写/Sync journal；恢复 replay 从原 journal 读取。该路径从未取得 Runner DB 提交权，本次亦未赋予。
- Orca observe 全文仅删除通用标签、Processed 分配和第二 select；原 Persisted 创建/等待、错误、取消及之后的消息游标确认/观察循环/清理保持。未确认不推进，错误不被补成功，Close/awaitCleanup 和只读观察责任保持。

[精确边界核对](evidence/event-ack-reaudit-20261102/boundary-audit.json)记录全部比对。未发现独立第二处理阶段被删除，也未将一次确认扩展为主机资源停止、业务成功或所有事件都有新持久事务的证明。

## 2. 消费者、共享夹具与持久格式

重新检查当前模块的生产、组件、正式 live/探针、scripts/client 及历史独立模块边界。9份 Processed 夹具与基线仅去掉第二回复，原发送次数、队列恢复、来源/结果/停止断言均保留。短输入 trace 真实消费者按原条件保持分类：无 Observation 且 Fact 为 Started/Completed 时为 turn，其他为 execution；JSON key 与其他字段不变。底层 Codex ObservationEvent 的 Kind/Status 及异步问答消费者未改，未把同名字段当同一个消费链删除。

B 的生产构造/调用仍仅为 `ApplySessionReceipt → BindingSnapshot → MergeReceipt`，原先没有 Environment/PolicyResources 赋值。实际 Target/Workspace/Process/Session/Implementation 绑定及 SameProcess 保留；删除未使用批次/解析器声明没有移除真实 codex.Observation、ObservationStatus、PRClues 或通用事件事实。真实 Snapshot.PolicyResources、Receipt.OriginalHost/DeliveryID、Host/Probe 等持久同名字段保持。

8个生产变更文件重跑声明比较：177个保留声明完全一致（52个类型）、9个授权声明调整、7个零入口声明删除、0个新生产声明。授权调整另经上述精确变换、逐项 diff 和行为验证审计，不以 allowlist 单独证明安全。持久保存根仍为 Run/相关 snapshot/request/receipt/operation/profile 等；owner RPC 保存 Receipt，journal 保存原 Envelope，不保存被删除的瞬时字段。

重新生成固定编码与实施前基线 **145116字节完全一致**，SHA256 `47d983e8958718ee57ba1a13f00e3504c51ff6268e964062f2ad6ef35c471367`。[身份/编码复核](evidence/event-ack-reaudit-20261102/identity.json)、[声明比较](evidence/event-ack-reaudit-20261102/declarations.json)。结合持久根、全部实际差异及未变的存储实现，确认 state/v8、user_version=8、CLI/Workflow/owner/native 协议、公开 capabilities/资源摘要、字段/动作解码和双操作表示保持，非仅依据一个样例相等。

## 3. 保留承诺和本次验证

PC-R01/R02 的原意图、事务和未知只读对账，PC-R03 的事件/结果身份，PC-R04/R05 的同构建原 owner/session 恢复及有界资格，PC-R06 的持久撤销、精确停止及 Job/reader 清理均沿原机制。此次复核未发现新增 Execute、资源/session 重建、停止后新业务输入或“无证据报成功”路径。原生权限/read_only、消息采样和 PC-R08 可选观测容错保持。

| 本次运行 | 结果 |
| --- | --- |
| application 提交边界、原操作/人工恢复/观察、保存失败、迟到创建/结果、LocalCodex停止、服务代次/Workflow恢复 | exit 0，31顶层/25子用例通过，0 skip；包含新单确认的成功、提交失败、重复、可选消息四子用例 |
| Orca single ack、原队列取消恢复 | exit 0，2顶层/3子用例通过，0 skip；未确认不推进、一次 nil 即继续、原错误退出、Close/awaitCleanup有界；原 sends/creates 次数断言保持 |
| 静态/编码/身份 | 上述精确边界、9份夹具、关键未变消费者、177个声明及重新生成编码通过；680项输入、3个二进制、初轮证据摘要匹配 |

本次共33个顶层通过，不与初轮重复累加。[精确命令](evidence/event-ack-reaudit-20261102/commands.json)及[测试名称/计数](evidence/event-ack-reaudit-20261102/test-summary.json)单列。生产/测试源码未再改，继续适用初轮446个组件、46个 application、正式 native/integration live 编译和同一最终 Runner 的3个公开场景；本次未重复运行或扩充这些资格。

最终 Runner SHA 仍为 `8e5ab60d20fbf952742e878a4a3013d1a896595c57890796700f0ccee73e2ce3`，source_digest `5b08c0ed3d8e3ca39e408501706e7b2496bc71b329bd5cec1017594314fde3a3`；本次只补文档/审计证据，无需重建。生产净减仍为105行。初轮新 observer 夹具失败、8 skip/3无测试包和历史记录保持原判，无新增真实模型/helper、全后端或完整 L1/L2/L3 资格。按本次授权提交实现、Spec/Plan 和证据并提 PR，不自动合并、部署或发布。
