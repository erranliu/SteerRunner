# Spec：内容结果材料校验与提交锁边界（A）

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：设计审计后已实施、A 定向验收完成，见[记录](../acceptance/content-inspection-boundaries.md)。基线 main `f6a389cfd4ae7827d7c722555a85e94f1e1ba708`。依据[长期产品承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)，配套 [Plan](content-result-lock-boundary.plan.md)。与[离线 inspect](offline-inspection-boundary.spec.md)独立实施及验收。

## 1. 消费者、问题与收益

`worker.invoke` 的原 receipt、`observation` 原结果及 `resumeRun` 晚到结果都经 `finishWithGate`，持有原 worker.gate；`finish` 的 nil gate 包装仅有测试消费者。结果归一化、原输入/Attempt、重复/冲突、未知操作及完成来源检查均有实际用途。

目前 final 分支持有 Engine.mu 调用 finishContent，再调用 verifyFrozenMaterials。校验包含固定仓库导出目录的 WalkDir/文件读取/摘要、材料路径和字节核验；不是 Git 查询。全局锁因此同时承担磁盘证据采样和所有 Run 的提交/内存查询串行化。A 消除这个跨 Run 耦合，让后续结果状态修改无需推理文件系统等待与无关 Run 控制的组合；不以减少校验或未经测量的耗时作为收益。

verifyFrozenMaterials/verifyContentInputs/verifyFrozenDocuments 的输入生成、派发和恢复消费者保留。contentEngine、repositoryFixture、auditEngine、原 runtimeFileChannel 和 R2 阻塞 I/O 夹具仍被多处使用，不删除或改写共享夹具。

## 2. 新边界

1. 原结果归一化、重复/冲突、原输入/Attempt/状态、未知操作保留以及 final 原生结果/来源判定顺序保持。单执行 unreported、结构化审计取证与开发完成 checkpoint 的既有分支保持。
2. final 分支先在 Engine.mu 下通过原 revision 和 final 资格检查。仅当原成功、已验证内容、succeeded outcome、无 Failure、内容流程且未超限/非法 UTF-8 时，需要原材料校验；错误优先级仍为原来源/结果门、大小/编码、材料、输出 schema。
3. 释放 Engine.mu，在原不可变 Record 克隆上调用原材料校验器；**不释放同 Run 的 worker.gate**。这不改变同 Run stop/close、派发和结果的原先后关系，不宣称同 Run 停止在材料 I/O 中可抢占。
4. 重获 Engine.mu 后，必须再次比较当前 revision 与原快照。不同则返回 false，不写 Result/ContentResult/History/StageResults，不 signal，不确认消费；原结果可经既有观察/原操作对账重读。revision 相同意味着原持久输入、来源、授权、控制和游标未被提交改变，不另发明缓存有效期或授权票据。原 volatile activity 摘要不作为提交证明。
5. finishContent 只消费本次校验错误并修改原克隆；保留纯本地 schema 校验、完整 final/history、生成关闭与对话等待语义。仍由原 commitLocked 的 `result_committed` 原子提交成功后才 signal/返回 true；保存失败不发布内存状态、不消耗原结果。
6. stop 已提交时，符合原来源的晚到 final 仍可收录；不得为收录新增 Allowed 要求，也不得恢复 Allowed、替换原 PendingStop 或产生新派发。外部文件可在采样后被宿主修改，原全局锁也不能锁住外部文件；本批不承诺跨文件系统/数据库原子性。

## 3. 删除、保留及格式

删除 finishContent 内的材料 I/O 依赖，迁至 finish 的明确采样区；复用原校验器。允许私有函数参数作为确定性阻塞测试接缝，生产固定使用原校验器；不新增 Engine 配置、可选端口、后台任务、持久字段或跨回合校验缓存。

保留全部材料/仓库/路径核验、输入/恢复门、原预算、未知零重发、执行与原生 owner、恢复、停止状态机/范围以及已有测试字节。state/v8、Record/Request/Receipt/Workflow、事件与 command 编码、公开响应和能力不变。PC-R01–R06/R08/R09 保持。

联合公开验收暴露主线 `StagedBusiness` 不识别新的 plan-driven 实施提示词。最小渲染复现后，仅追加当前提示词前缀识别及独立回归；全部旧识别、真实 Git 动作和既有测试/共享夹具消费者保留。该测试支撑适配单列，不算 A 的生产精简或变更产品提示词。

## 4. 最小验收

| 编号 | 验收 |
| --- | --- |
| A01 | 阻塞实际 finish 边界的材料读取时，无关 Run 的 resident inspect 与 stop 事务可完成；同 Run gate 仍持有，结果未提交/未确认 |
| A02 | 读取中原 revision/输入身份改变，旧采样不得提交；原结果可重新收录且只有一份历史 |
| A03 | 材料漂移仍失败；超限/非法 UTF-8、失败/取消/未验证/错来源与 schema 的优先级保持 |
| A04 | result_committed 事务故障不发布或确认；撤除故障后原结果重读提交一次；stop 先到的晚到 final 不恢复授权或替换 stop |
| A05 | 既有内容/仓库/文件材料、恢复/停止和 R2 并发回归通过；持久声明/编解码不变，代表编码前后相同，既有测试及共享夹具未改 |

验收限定受影响组件及公开受控场景，不自动获得完整 L1/L2/L3 或真模型资格。
