# Spec：旧规则与 Lifecycle 接口清理

日期：2026-11-01。状态：设计审计后已实施并完成专项最小验收，见[验收](../acceptance/dormant-lifecycle-cleanup.md)。基线：main `51e897fb694ef8afe61a380da83fe0de895ec4d1`，含已合入的 #81，与实施前工作树文件树一致。依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)，配套 [Plan](dormant-lifecycle-cleanup.plan.md)。

## 1. 目标及范围

清除没有当前生产入口的旧准备/回合/恢复规则，并收窄无消费者的 Lifecycle 方法和能力声明。分 A/B 两个可独立验收的增量；不通过重写实际运行链路取得删行。本批不收缩任何产品能力或可靠性承诺。

PC-R01–R06 的实际受理、固定意图、原操作对账、原结果身份、普通重启、撤销和停止保持；PC-C06 的 full_access/read_only、PC-R08 的观测容错保持。收益为规则和维护面减少，不宣称执行或测试耗时提速。

## 2. 实际消费者与合法入口审计

| 候选 | 当前生产消费者/入口 | 删除边界 |
| --- | --- | --- |
| reliable/environment.go：PreparationState、EnvironmentDeadline、AcceptHostFact、BlockerChange、InvalidatesPreparation、ProbeSettled；backend/environment.go：HostEnvironmentPort 及其专用值类型 | 候选间内部引用或专属单测，没有当前 Run 实现/调用；历史 Check 验收的共享保留描述不作为当前消费者 | 删除两文件；backend/host.go 的持久 ProbeIdentity/HostActionInput 不在本批 |
| reliable/turn.go：TurnState/Authority、Begin/Current/StopTurn | 专属 turn_test/staged_recovery_test（后者在实施编译时补查发现）；实际开始确认由 application.acceptEvent → RecordTurnStart / ApplySessionReceipt 处理 | 删除旧规则及专属测试，保留真实回执/事件机制 |
| reliable/recovery.go 及 recovery_policy.go 中旧 RecoveryRevision/ObservationIdentity、分类/超时/受理辅助规则 | 仅局部规则互相调用和 policy/recovery 测试；实际恢复在 application.manual/continuation/observation/control | 删除孤立规则；保留 CanContinueOriginal、ErrContinuationUnavailable、ValidateResultBinding 原正文 |
| reliable/result.go 的 AcceptedResult、ContinuationConstraints；reliable/workspace.go 的 OriginalWorkspace；plan.KnownAction | 无生产调用；旧约束/部分测试引用 | 删除无消费者规则；保留 NormalizeResult/VerifiedContent/ErrStale、实际工作区及原意图核验 |
| LifecycleProbePort.PreflightLifecycle、codex.ProbeLifecycle/probeLifecycleCapabilities、codex/capability.go 的专属 probe 工具 | 构造器装配和专属测试；没有 application 调用；独立 ProbeManagedCLI 亦无生产调用 | 删除这条独立 preflight 链及专属测试，保留 NativeExecutable/VerifyPolicyVersion、实际权限和准备检查；不扩大到其他 Backend.Probe/conversation capability |
| LifecycleWorkspacePort.InspectLifecycleWorkspace 及两适配器包装、workspace/lifecycle.go | 只有声明、包装及其内部 helper，没有当前业务调用 | 删除包装；实际 inspectSource/InspectAudit、原 checkpoint、Orca InspectWorkspace 等生产检查保留 |
| LifecycleCleanupPort.CleanupLifecycle | 无生产调用；Codex 返回 unsupported，Orca 包装 StopLifecycle | 仅删端口及适配方法；PreparationFacts.Cleanup、planner 分支、动作枚举/输入/解码及 StopLifecycle 保留 |
| LifecyclePolicyCapabilities 的 ServiceStopBehavior、StructuredFinalEvidence、ActiveTurnFeedback、LinkedWorkspaceWriteRoots、SeparateTurnStart、ResumeActiveObservation | 只有声明、适配器赋值、两处声明测试，没有生产读取；非持久值 | 删除字段、仅服务此字段的枚举/常量及赋值；保留 Preparation 和 ReportToolPolicy，真实 ShutdownLifecycle/owner/权限和结果核验不变 |

已核对 application 的直接候选构造及 reliable.Next；当前合法入口仍为工作区/资源/会话准备、业务输入、原问题回答、interrupt、stop 及原操作只读恢复。保留的实际组件不由上述旧模型驱动。编码字段的存在、被 Clone 或单测构造不等于合法业务入口。

历史 acceptance/evidence 的 Go 快照属于其固定版本且受独立 go.mod 隔离；保留原字节，不以兼容它们为由恢复已退役运行栈。实施前须重新全仓搜索，包括 cmd、正式测试及工具；发现新实际消费者时暂停对应删除并收窄范围。

## 3. 明确保留项

- state/v8、user_version=8、CLI/Workflow/原生协议不变。不删除 LifecycleSnapshot、LifecycleInput、LifecycleRequest/Receipt、Operation 的任何持久字段、动作枚举或解码支持，包含 Host/Completion/Ack/Cleanup 等候选下一批内容。
- Record.Intents 与 Snapshot.Operations 的现有表示、写入、状态及原键差异保持；不合并账本、不重新计算原 payload/digest，不给未知操作新 Execute 权限。
- CanContinueOriginal、ValidateResultBinding、RecordTurnStart、ApplySessionReceipt、NormalizeResult、真实预算与来源检查保留。不能整包删除 reliable。
- Orca verifyLocalObservation 仍被 workspace 准备消费；agentExecutionSupport/PreparationFacts/Execute 的不支持组合拒绝、原登录/路由核实保留。
- 直接 Codex 无 launcher 分支仍由独立 execution owner 真实调用；owner 的签名、原 session、checkpoint、停止与服务脱离保留。Pi/DSH、原生远端、独立 parser 不重构。
- 保留真实 StopLifecycle/ShutdownLifecycle、Job/reader、原根/终端 scope、创建迟到回执及停止后零输入，不以删除 CleanupLifecycle 包装取消清理。
- 远端日志租期/撤销、progress/result 文件、read_only 和现有权限准备均有实际消费者，本批不改。

## 4. 测试删除与调整规则

专属测试随死规则删除；混合测试只能删除对应孤立断言。policy_test 的 CanContinueOriginal、NormalizeResult、MergeReceipt、ReportToolPolicy、真实准备 gate 测试保留；service_lifecycle_test 的真实 Job 终止/独立 owner 存活断言保留，只删未使用能力字段断言；agent_test 改从真实 PreparationFacts/Execute 验证不支持组合，保留零创建/零输入检查。

不保留恒真函数、无效字段或空端口来维持旧测试。也不通过新 mock 框架、迁移器或新的公共能力开关替代被删除内容。

## 5. 最小验收

| ID | 要求 | 方式 |
| --- | --- | --- |
| DLC-A00 | Spec/Plan 审计先于代码，候选闭包与保留项有依据 | 记录消费者、混合测试和序列化风险复核及结果；发现问题先改设计 |
| DLC-A01 | 无误删实际入口，候选未搬家 | 当前模块引用复查及 Go 编译；历史模块单列；真实共享方法仍可达 |
| DLC-A02 | 私有/公开编码和持久字段不变 | 基线与新版本对同一 snapshot/request/receipt/Run 固定值序列化逐字比较；公开 resources.Capabilities 逐字比较；原操作/预算模型测试保留 |
| DLC-A03 | 真实受理、未知零重发及停止可靠性保持 | reliable/backend/适配器组件，application 原操作、人工恢复、观察、停止竞争、迟到创建/结果及 FrozenBudget 等定向；保存失败零外发 |
| DLC-A04 | 共享准备和原生消费者仍有效 | 不支持 Agent 生产入口零副作用；实际 Orca workspace/IPC 与 Codex Job/owner 测试；native/integration `go test -c -tags live` 编译 |
| DLC-A05 | 最终 Runner 公开链路 | 原 foundation 的 NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership 三场景；记录精确匹配/数量、退出码和 skip，不拿零场景通过 |
| DLC-A06 | 结构收益及资格可核对 | A/B 分别统计生产增删/净变化、接口/字段数量，独立归档源码与产物 SHA、命令和首轮失败；不将测试删行当生产收益 |

本批无真实运行路径变更，不强跑模型、PR 或新的真实 helper 探针；实际越界时再补对应最小验证。不是完整 L1/L2/L3 资格，不部署、不排空用户 Run。

## 6. 交付

完成后更新长期文档的实现说明、Spec/Plan 状态与验收索引；历史验收不回写。仅在实际验收通过后标记完成。初轮不自动提交/推送或创建 PR；2026-11-02 用户追加授权实现再审计、修复确认问题后提 PR，交付沿该授权执行。合并或发布不属于本次范围。

## 7. 实施前设计审计记录

2026-11-01，生产代码未改动时完成复核：通过 DLC-A00，无扩大范围的阻断项。确认三处共享边界：verifyLocalObservation 仍由 workspace 准备调用；service_lifecycle_test 混合实际 Job/脱离行为与死字段断言；policy_test 混合有效的继续/结果/回执规则与旧模型。实施分别保留实际方法和混合测试有效部分。

审计另确认动作、Snapshot 的 Host/Probe/Delivery 等序列化字段均不删除，PreparationFacts.Cleanup 也保留；只删无人调用的方法，不把未接线分支改成新的调度器。已在原源码捕获持久类型、代表性 Run/snapshot/request/receipt、公开 capabilities 和资源摘要的固定基线，实施后逐字对比。当前文件树与已合入 #81 的 main 一致。剩余运行验证按 Plan 执行，不把本次静态通过当运行验收。

## 8. 实施结果

A 净减 407 行，B 新增 5/删除 286、净减 281 行，生产合计净减 688 行；端口 11→8、私有能力字段 8→2。DLC-A00–A06 已完成，486 个顶层组件/application 测试与 3 个公开 CLI 场景通过；8 项默认 skip、4 个无测试包及首轮失败单列。编码/公开能力逐字一致，86 个保留函数未变正文。最终构建与实际资格以验收报告为准，未部署/发布、无新增真机资格。

2026-11-02 [实现再审计](../acceptance/dormant-lifecycle-reaudit.md)未发现确认缺陷；补查全部保留声明与实际消费者，完整组件组 452 顶层和补充 application 11 顶层通过，持久格式保持。生产/测试源码与初轮验收完全一致。
