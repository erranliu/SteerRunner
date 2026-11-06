# 旧规则与 Lifecycle 接口清理：实现再审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。结论：未发现误删实际消费者、改变持久格式或破坏保留执行/恢复/停止承诺的确认问题，无需追加生产代码修复。审计对象为 main `51e897fb694ef8afe61a380da83fe0de895ec4d1` 加 [Spec](../spec/dormant-lifecycle-cleanup.spec.md) / [Plan](../spec/dormant-lifecycle-cleanup.plan.md) 的 A/B 实现；相对于[初轮验收](dormant-lifecycle-cleanup.md)，694 个源码/构建输入身份全部一致，初轮证据摘要亦全部通过复核。未部署或发布。

## 1. 实际消费者与合法入口

重新提取全部 14 个生产变更文件的删除声明，检索基线中当前模块的 Go 标识符，再按包、生产/测试、入口逐项复核；本次没有依赖上轮“引用归零”的结论。没有发现被删声明在退役闭包以外的实际消费者。唯一闭包外的同名标识符 `backend.PreparationProgress` 是准备查询回调，与删除的 `reliable.PreparationProgress` 不同；其 application 入口、持久准备预算与回调均保留。历史 evidence 的独立模块维持原字节，不作为现行产品入口。

另补查所有非历史文本入口：PowerShell `Read-ProbeReceipt` 读取 Pi 扩展原 JSON，`Wait-RecoveryResult` 等待正式测试的 result.json；与已删 Go ProbeReceipt/RecoveryResult 无关，保留脚本及实际文件消费者。

- `PreflightLifecycle`、`CleanupLifecycle`、`InspectLifecycleWorkspace` 没有 application 调用；Codex probe 构造只装配未调用的回调。能力声明的六个删除字段没有生产读取。删除测试仍仅对应退役模型或混合测试中的死声明断言。
- Orca `workspaceRepository → verifyLocalObservation` 的真实准备核实仍在；`agentExecutionSupport → PreparationFacts/Execute` 的拒绝与零副作用仍在。原生权限资源的 `NativeExecutable`、`NativePermissions`、`VerifyPolicyVersion` 及 read_only 路径没有修改。
- 直接 Codex `ServeExecutionOwner` 使用无 launcher 的实际分支；其执行、原签名/checkpoint、Job/reader 与服务脱离机制保持。没有因为删除服务停止的能力声明而改变实际所有权。
- `PreparationFacts.Cleanup`、planner cleanup 分支、动作枚举及解码均保留。现行适配器没有产生该准备事实，Execute 也未通过被删的独立包装调度它；删除包装不会使原已支持路径失去执行入口，保留编码也不新增支持承诺。

完整删除清单、基线文件级标识符引用及保留声明比较见[声明审计](../../../../acceptance/evidence/dormant-lifecycle-reaudit-20261102/declarations.json)。同名盘点包含定义和其他包的标识符，本报告的消费者结论还包含上述人工入口核对。

## 2. 持久格式确实未变的依据

本次把比较扩展到每个生产变更文件的全部函数、类型、变量和常量声明。**165 个保留声明完全一致，其中 42 个为类型**；无新声明。仅六个声明发生变化：Codex 构造器和两个 `PolicyCapabilities` 方法，以及运行时 `ExecutionLifecycle`、`LifecyclePolicyCapabilities`、`LifecyclePort` 三个类型。变化分别为去掉死回调、缩小私有返回值和接口。它们没有持久编码消费者。

其余生产文件在 Git 差异中完全未改。删除的旧类型虽有 JSON 标签，但没有持久根或现行编码消费者；没有删除 Run、snapshot/request/receipt/operation、Host/Probe/Delivery 等持久字段，未动存储 schema/迁移、动作解码、CLI/Workflow 版本或公开资源定义。state/v8 和 user_version=8 保持。

重新运行固定编码工具，139060 字节仍与实施前基线逐字相同，SHA256 `941cfb9f94d2175617975200e426d56e15a99a4a45b41b7b2fd90ec07a3d414c`。它覆盖代表性编码及选定持久类型、公开 capabilities 和资源摘要；不将样例相等单独当作所有编码的证明。全部声明差异、真实消费者闭包和未变的编码实现共同支持格式结论。[源码/历史证据复核](../../../../acceptance/evidence/dormant-lifecycle-reaudit-20261102/identity.json)

## 3. 保留的可靠性承诺

| 承诺 | 复核结果 |
| --- | --- |
| PC-R01/R02 持久受理、原意图和 unknown 零盲重发 | application 保存意图、回执及 Reconcile 调度未改；Intents/Snapshot.Operations 和原 payload/digest 未改；删除的旧规则没有参与实际受理 |
| PC-R03 受理/开始及原结果绑定 | acceptEvent/RecordTurnStart、ApplySessionReceipt、ValidateResultBinding、NormalizeResult 与真实结果消费均保留，未用旧 TurnState 或 AcceptedResult 维持该承诺 |
| PC-R04/R05 普通重启和有限人工恢复 | qualifyContinuation/CanContinueOriginal、VerifyManualRecovery、原 session/资源身份及授权保存未改；没有恢复重建或跨 build 接管的新分支 |
| PC-R06 撤销和精确停止 | application.control 仍调用 StopLifecycle/ShutdownLifecycle；本机原 stop 已可能外发后仍只读对账，原资源/迟到创建/Job 清理和停止后零输入保持；没有把 CleanupLifecycle 包装当实际清理责任 |
| PC-C06/R08 权限及非阻塞观察 | 原生权限准备、read_only、原事件/结果观察保持；死 preflight 删除未改真实准备检查或把可选观测变成新门禁 |

先前六文件比较的 86 个保留函数正文未变；本次所有保留声明比较进一步确认，没有新 helper、公共能力开关、状态迁移器或生产依赖。收益仍为生产净减 688 行和维护面收口，不宣称执行提速。

## 4. 验证与交付

| 本次运行 | 结果 |
| --- | --- |
| `go test ./internal/execution/... ./internal/runstate ./internal/contracts ./internal/resources ./internal/storage -count=1 -timeout 5m -json` | 单条完整命令 exit 0；452 顶层、1351 子用例通过；8 个 helper/opt-in skip，4 个无测试包，不计为通过用例。补齐初轮整组失败后仅 reliable 单包复验的证据 |
| application：`^(TestLocalCodex.*\|TestServiceGeneration.*\|TestWorkflowRecovery.*\|TestAnswerInputCapability.*\|TestContentRecovery.*)$` | exit 0；11 顶层、8 子用例通过，0 skip；原停止回执/请求只读恢复、隔离其他 Run、未知创建拒绝、重启授权、问题/来源范围和内容恢复 |
| 源码与编码 | 694 个源码/构建输入、原历史证据摘要全部匹配；新生成编码匹配原基线；最新 main 仍为同一基线，无需引入额外合并差异 |

命令、原 JSONL、精确名称及 skip 见[本次证据](../../../../acceptance/evidence/dormant-lifecycle-reaudit-20261102/README.md)。组件重复运行不与初轮累加。生产/测试源码未变，继续适用初轮已记录的 34 个 application 定向、正式 native/integration live 编译及同一最终 Runner 的 3 个公开 CLI 场景；本次不把它们写成重新运行。构建 SHA 仍为 `8d56777596af0b8bca36fec8e7228ca79e7a4a94e02194ce9135db5dfbcff978`，无需因文档和审计证据变化重建。

确认缺陷为零，未制造额外代码改动。按本次用户授权提交实现、文档和证据并创建 PR；不合并、不部署。没有新增真实模型/helper 或完整 L1/L2/L3 资格，初轮失败/skip 与历史结果保持原判。
