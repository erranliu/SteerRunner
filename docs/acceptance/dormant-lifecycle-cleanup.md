# 旧规则与 Lifecycle 接口清理：实施验收

2026-11-02 已完成[实现再审计](dormant-lifecycle-reaudit.md)，最新消费者/全部声明核对及整组复验见该报告；以下保留初轮实施时的结果与交付状态。

日期：2026-11-01。结论：设计审计后实施 A/B，DLC-A00–A06 专项验收通过，尚未提交、部署或发布。依据 [Spec](../spec/dormant-lifecycle-cleanup.spec.md)、[Plan](../spec/dormant-lifecycle-cleanup.plan.md)及[长期承诺](../spec/steerrunner.spec.md#长期产品承诺)。源码为 main `51e897fb694ef8afe61a380da83fe0de895ec4d1` 加本次 dirty 差异，含已合入 #81；以[逐文件与产物身份](evidence/dormant-lifecycle-20261101/source-identity.json)绑定实际源，不把 HEAD 当本轮实现。

## 1. 删除范围及实际收益

| 增量 | 生产新增 / 删除 / 净减少 | 结果 |
| --- | --- | --- |
| A 旧规则 | 0 / 407 / 407 行 | 删除四份旧环境/回合/恢复文件及无消费者工作区规则，清除 recovery_policy/result 的孤立规则；保留三个实际继续/绑定符号、结果归一化及真实回执/事件路径 |
| B 接口收口 | 5 / 286 / 281 行 | 删除 Probe/Cleanup/Workspace 三个未调用端口和包装、Codex preflight 链、六个无读取能力字段及专属枚举/赋值、KnownAction 重复名单 |

生产合计净减 **688 行**；LifecyclePort 组合从 11 个端口减到 8 个，LifecyclePolicyCapabilities 从 8 个字段减到 Preparation/ReportToolPolicy 两个。明细见[生产差异](evidence/dormant-lifecycle-20261101/production-diff.json)。测试/文档和纯移动不计生产收益，没有可比耗时提速结论。

没有删除动作枚举/解码、PreparationFacts.Cleanup、Snapshot/Request/Receipt/Operation/Run 的持久字段或双操作表示。state/v8、CLI、Workflow、资源摘要和公开 capabilities 保持。保留的工作区、权限/read_only、原 owner/Job/reader、实际停止和结果/预算机制继续按原入口执行。

## 2. 设计审计与实现再核对

设计审计在生产编辑前完成，[记录](evidence/dormant-lifecycle-20261101/design-audit.json)明确无生产消费者和混合测试边界，先捕获旧编码。静态盘点和最终编译分别检查生产、专属测试、混合测试及固定历史工具。

- verifyLocalObservation 仍由 Orca workspace 准备调用，保留；agentExecutionSupport、PreparationFacts/Execute 的不支持组合拒绝及零副作用检查仍在。
- Codex 无 launcher 分支仍供独立 execution owner 使用，保留；service_lifecycle_test 仅去掉字段声明断言，实际 Job 终止和独立 owner 服务脱离后存活的断言仍通过。
- policy_test 保留 CanContinueOriginal 正反例、准备 gate、NormalizeResult、MergeReceipt 的错来源拒绝及原 report 策略检查，只删除旧模型断言。真实人工对账、观察、迟到 final/创建和停止竞争由 application 定向及公开 CLI 验证。
- 退役符号在当前 internal/cmd/tests Go 源中引用为 0；历史 evidence 模块保留原路径及字节，不生成兼容包装。没有把旧规则搬到新包。

对涉及的六个生产文件进行 Go AST 函数比较：保留的 **86 个函数**与基线一致；仅允许 Codex 构造器去掉死 probe 回调、两个 PolicyCapabilities 返回更小私有值。动作/解码、实际准备/执行/恢复/停止方法未改正文。[比较记录](evidence/dormant-lifecycle-20261101/retained-functions.json)

## 3. 编码、公开行为及最小验收

实施前后用同一个固定工具捕获：持久类型定义、含 Host/Probe/Delivery/Operations 等字段的代表性 Run/snapshot/request/receipt JSON、resources.Capabilities 和资源摘要。结果 **139060 字节逐字相同**，SHA256 `941cfb9f94d2175617975200e426d56e15a99a4a45b41b7b2fd90ec07a3d414c`。[比较记录](evidence/dormant-lifecycle-20261101/wire-comparison.json)

这同时约束“无人调用的方法”与“必须保留的序列化形状”。代表性值只用于编码对比，不将其当已受理业务或新的合法 HostAction；真实受理和结果入口由下面的运行验证覆盖。

| ID / 验证 | 实际结果 |
| --- | --- |
| A00 设计先行 | Spec/Plan 及设计审计完成后才删除生产代码；共享方法、持久字段、混合测试均明确保留 |
| A01 引用与 A04 原生消费者 | internal/execution/...、runstate/contracts/resources/storage 整组执行；除 reliable 初轮测试编译失败外，442 个顶层、1331 子用例通过；修正后 reliable 单包 10 顶层、20 子用例通过。最终相关组件 452 顶层通过；8 个 opt-in/helper skip 和 4 个无测试包单列 |
| A02 编码与公开能力 | 上述 JSON、持久类型、资源摘要及 capabilities 字节对比通过，原模型/操作编码和 clone 测试仍在 |
| A03 application | 34 顶层、69 子用例通过、0 skip，19.926 秒；原操作、人工恢复、观察、准备查询与预算、停止竞争/迟到创建、FrozenBudget 和原结果路径 |
| A04 正式 live 测试 | native/integration 各以 `go test -c -tags live` 编译通过，不运行 TestMain 零场景 |
| A05 最终公开 CLI | TestSRNativeUnknownAndStop、TestSRRestartPreservesAuthorization、TestSRServiceOwnership，3/3 场景、22 子用例通过、0 skip；包含 graceful/kill、原创建回执和 pending_stop 恢复 |
| A06 结构及取证 | A/B 分项增删、端口/字段缩减、86 个保留函数与编码比较、固定源码/构建、精确命令/数量/skip/失败归档 |

组件加 application 共 **486 个顶层通过**，重复执行不累加；[统计及每包结果](evidence/dormant-lifecycle-20261101/test-summary.json)、[精确命令](evidence/dormant-lifecycle-20261101/commands.json)。8 个默认 skip 不算运行资格，4 个无测试包不计通过测试数。原 broad 命令整体退出 1 的事实保留；可靠性包单独复验后满足组件出口，没有把失败日志改成整体通过。

最终 Runner 由 scripts/build.ps1 构建，SHA256 `8d56777596af0b8bca36fec8e7228ca79e7a4a94e02194ce9135db5dfbcff978`，[构建记录](evidence/dormant-lifecycle-20261101/runner-build.json)。公开三场景使用同一产物及独立正式 foundation 执行器；成功退出包含各自受理 Run 的停止、服务和隔离夹具清理。没有操作用户已有现场或 Orca worktree。

## 4. 首轮失败与限制

1. wire-after 首次构建因删除包装后遗留 workspace/os-exec 两个 import 而失败，移除后复验通过；[失败摘记](evidence/dormant-lifecycle-20261101/build-first-failure.txt)记录工具返回，不冒充完整原始 stdout。
2. 执行组件首轮发现 staged_recovery_test 仍引用旧 TurnState/BeginTurn。该文件仅测试退役模型，不是生产入口；补查、删除专属测试并更新 Spec/Plan 后 reliable 单包通过。首轮完整 JSON 构建诊断和整组其他结果保存在 components-final.jsonl.gz。

本批删除的是无调用规则和接口声明；实际运行链路未改，不要求新的真实模型/helper/PR 探针。没有取得完整 L1/L2/L3、全后端或新组合支持资格，也未缩减实际停止、权限或恢复承诺。后续动作/持久字段删除、双操作表示收敛、远端日志租期均未实施，不因本次清理自动获得删除授权或证明。
