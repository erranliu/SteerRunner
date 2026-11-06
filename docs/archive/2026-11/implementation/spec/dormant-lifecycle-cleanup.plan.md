# Plan：旧规则与 Lifecycle 接口清理

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-01。状态：P0–P4 已完成，见[验收](../acceptance/dormant-lifecycle-cleanup.md)。依据 [Spec](dormant-lifecycle-cleanup.spec.md)和[长期承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)。

## P0：冻结范围和审计设计

- [x] 核对最新 main/工作树、实际调用、测试及历史模块，记录删除前清单和编码基线。
- [x] 审计 Spec：生产入口与测试入口分开；所有持久类型/动作及双表示保留，真实 workspace/权限/owner/停止不误删。
- [x] 逐个检查混合测试，列出只退役断言与仍应运行的真实规则；确认没有把 capability 字段当实际行为。
- [x] 确认无需新 state 版本/迁移/公开 schema，审计通过后再改生产源码；如有实际消费者或编码变化先收窄设计。

## P1：A——旧规则清理

1. 删除 reliable/environment.go、turn.go、recovery.go 和 backend/environment.go。
2. recovery_policy.go 仅保留三个实际符号及原正文；result.go 保留结果归一化/来源与 ErrStale，删除 AcceptedResult/ContinuationConstraints；删除无调用 OriginalWorkspace。
3. 删除专属 turn/staged_recovery/recovery 测试；policy_test 仅退役相关旧断言，保留真实 gate、继续资格、结果/来源和回执冲突检查。不要用新孤立规则补同名能力。
4. A 的生产增删独立记账；不得触及 HostActionInput 等持久字段。

## P2：B——接口和声明收口

1. 从 LifecyclePort 移除 Probe/Cleanup/Workspace 三端口及定义、两适配器对应方法、Codex preflight 构造与专属工具、workspace 旧包装；删除 KnownAction 的无调用重复枚举校验。
2. 删除六个无读取能力字段及其专属 ServiceStopBehavior 类型/常量，保留 Preparation 和 ReportToolPolicy；准备、native 受理、权限和 Shutdown 方法正文不改。
3. 调整混合 agent/service 测试，只去除声明断言；保留真实路径零副作用、Job 清理和独立 owner 脱离存活。
4. 保留有真实消费者的 verifyLocalObservation、动作/解码、PreparationFacts.Cleanup、Snapshot/Request/Receipt。B 单独记账；不顺手扩大到另一套 Backend/Conversation 能力接口。

## P3：验证及审计实现

- [x] 全仓引用与共享函数比较；基线/新编码及公开 capabilities 逐字比较；源码稳定后运行组件，避免边编译边删文件。
- [x] backend、reliable/...、codex、orcatransport、protocol/orca 相关整包；application 选择真实原操作/恢复/观察/停止/FrozenBudget 及相邻结果回归，登记实际名称、数量及 skip。
- [x] native/integration 正式 live 编译；不运行零场景 TestMain。
- [x] 正式脚本构建最终 Runner，执行 Spec 的三项公开受控场景，使用新隔离目录、固定最终 SHA，并确认夹具清理。
- [x] 再审计生产差异：没有持久格式变化、没有删真实准备/停止/reader、没有改变未知恢复及保存失败路径；必要问题修复后定向复验。

## P4：文档及完成边界

- [x] 新验收报告绑定源码、基线、A/B 生产增删、编码对比及精确运行结果；保留失败/skip，不继承旧资格。
- [x] 更新长期实现说明、Spec/Plan 状态和索引，保持旧验收及归档字节。
- [x] 最终 diff/链接/证据摘要检查。报告收益及未验范围；初轮不部署、不自动提交/推送或创建 PR。2026-11-02 用户追加授权再审计后提 PR，按此授权交付。

停止条件：A/B 各自的候选引用归零、共享合法入口不变、DLC-A00–A06 通过；无可比耗时不报提速。任一项出现合法消费者或编码反例则暂停该删除，修订范围后再继续，不用扩大恢复框架来掩盖反例。

实际结果：A/B 生产分别净减 407/281 行；编码和保留入口不变，相关 486 个顶层通过、公开场景 3/3。首轮 unused imports 与 staged_recovery 专属测试编译失败保留，后者补查后已复验通过。不以删除测试提高通过率，不继承完整 L1/L2/L3 资格。

2026-11-02 已完成[实现再审计](../acceptance/dormant-lifecycle-reaudit.md)：无确认代码缺陷，补齐完整组件组通过记录、全部声明和消费者审计，原证据保持；提交并创建 PR，不合并或部署。
