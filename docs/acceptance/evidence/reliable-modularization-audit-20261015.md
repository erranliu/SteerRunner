# 中间层模块化实现审计与修复

日期：2026-10-15。依据 [Spec](../../spec/reliable-modularization.spec.md) 和 [Plan](../../spec/reliable-modularization.plan.md)。用户要求检查遗漏、可靠性回归和过度设计，并直接修复。审计覆盖基线 `af545563c58171c584bd35d098166cf39012e699` 到当前工作树的全部重构变更；此前收尾证据保留为历史记录。

状态：两处既有缺陷已复现并修复；37 个顶层单元测试、adapter 定向测试、vet，以及最终同一构建的 8 个受控顶层场景和 4 个兼容子场景全部通过，无失败或跳过。此记录描述审计验证时的源码和构建；随后用户授权提交 PR，交付提交和分支状态以 Git/PR 记录为准。

## 确认的问题及修复

| 问题 | 影响与证据 | 修复 |
| --- | --- | --- |
| P1：stop 意图和 stop 命令关联分两次提交，第二次错误被忽略 | `TestSRStopBindingCommitFailureDoesNotDispatch` 仅在 stop_intent_bound 事件插入时令真实事务失败。修复前仍发生一次停止调用，并留下一个缺失命令关联的意图；下一次处理可能再造意图。该问题存在于源码基线，本次提取保留了它。 | application.saveIntent 在同一事务内保存意图及命令关联；提交失败不外发且不留下半份意图。保留 Snapshot 原独立幂等键。移除 control 中第二次绑定提交。 |
| P2：部分事件在新来源事实落盘前确认成功 | `TestSRObservationUnchangedProjectionStillCommitsIdentity` 的 completed 和 question 两个子项均在修复前失败：前者无 implementation 时提前返回，后者因公开问题内容相同提前返回，新的 SeenNative 记录未提交。后续冲突重复可能漏检。该问题同样来自基线。 | 有新来源 ID 或实现事实时先提交再成功确认；相同问题保留原路由，已持久化来源的重复仍幂等，冲突仍拒绝。 |
| RM-A05 覆盖不足 | 原受控 stop 场景以等待和调用计数证明停止后没有新输入，没有明确挡住派发中的调用。 | `TestSRStopSerializesWithNativeDispatch` 用 channel 屏障挂住真实宿主派发路径，核对 gate、撤销提交和后续零新输入；没有生产故障开关。 |
| RM-A08 覆盖不足 | 原三个 v2 样本只验证加载、写回和重启，没有让旧版本留下的操作在新版本实际结算。 | 兼容测试补 stop_pending：旧 EXE 经公开命令执行 stop，测试侧 SQLite trigger 使其结算失败；证明旧服务退出后移除 trigger，新 EXE 按原操作核验写者退出并结算，随后再次重启。 |

行为差异明确限定为：operation_committed 和 stop_intent_bound 两条既有事件保持顺序，但因原子提交共享 revision；此前被跳过的新来源事实会产生 revision 及原有类型的事件。没有新增事件类型、SQL/JSON 字段、状态加载规则或历史数据修复。

## Spec/Plan 核对与过度设计检查

| 验收 | 核对结论 |
| --- | --- |
| RM-A01：模块、依赖、状态归属 | operation 独立，依赖 application/runstate → operation → reliable → backend，无循环和宿主/存储/业务反向依赖。Record/Store 仍由 application 统一写入；没有重复操作实现。依赖守卫通过。 |
| RM-A02：持久意图和未知对账 | 原意图提交失败零派发、回执提交失败仅沿原 ID/输入/摘要/绑定对账、accepted/resource_pending 不重建继续通过；补齐 stop 绑定失败路径。 |
| RM-A03/04：会话和观察 | 会话合并失败无部分修改、accepted 与 started 区分、原 command/turn 转换及事件失败可重读通过；补齐不改变公开投影的事件分支。原生解码和 owned 句柄选择未修改。 |
| RM-A05/06：授权、恢复与准备 | gate 仍在宿主，撤销后无新输入；换代需 resume、原会话恢复、准备期限/迟到 ready 不恢复授权、权限与固定材料规则通过。 |
| RM-A07/08：业务和兼容 | 使用同一最终 EXE 的受控组合及四类旧 v2 样本验证；实际结果见下节。没有把“成功加载”替代“旧操作结算”。 |
| RM-A09：本机原生风险 | 未触发。修复只涉及宿主持久事务及事件确认；adapter、原生协议/参数/权限、StopLifecycle/ShutdownLifecycle 的选择和服务退出次序未变。受控验证覆盖此次故障边界，没有运行真实模型或创建真实 PR。 |

没有发现需要新增职责包、接口协议、状态容器或通用调度/停止策略的理由。session 的一个纯转换由回执和事件共用；observation/feedback/policy 的文件整理有具体重复或耦合依据。继续保留现有五项逻辑职责和一个独立 operation 包；没有为了审计扩建生产抽象。

## 验证与来源

最终源码单元命令：

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage -count=1 -timeout=3m -v
go vet ./internal/execution/reliable/... ./internal/application ./internal/runstate
go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -run '(Lifecycle|Recovery|Preparation|Policy|Workspace|Binding|Outcome|Receipt|Event|Question)' -count=1 -timeout=3m
```

37 个顶层单元测试全部通过，adapter 定向测试及 vet 通过。日志为本地 `.evidence/reliable-audit-unit.log` 和 `.evidence/reliable-audit-adapters.log`。上述两处缺陷均先观察到反例失败，再修复并复验。

最终受控命令：

```powershell
.\scripts\test-foundation.ps1 -Pattern '^(TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRRestartRequiresResume|TestSRPermissionQualification|TestSRPublicationIdentity|TestSRStorageLogsEvents|TestSRPublicationBeforeWorkflowReady|TestSROperationV2Compatibility)$' -BaselineExe .\.evidence\foundation-20261015-085321\steerrunner.exe
```

最终组合证据目录为 `.evidence/foundation-20261015-094642`。EXE SHA256 为 `36D2ABBB69C577E254C8D92363E829242B954228E28DD4110351B05DBD6A66D4`，测试 EXE SHA256 为 `237FCA6438B27BCA5174F858053DA5C35B1E1AE447DC2BF0768D84DA50664A29`。旧 EXE SHA256 为 `9A215F39112E8BFC5807ED2C923DF7481DAAA2A339E1FA6F6EF67ECB651A815E`；source-identity.json 记录其来源和实际运行参数。stop_pending 还先在 `.evidence/foundation-20261015-093419` 独立通过，不能替代最终组合的结果。

第一次完整组合 `.evidence/foundation-20261015-093922` 在原 5 分钟时限下未取得完整 result.json，未计为通过；其遗留测试服务按精确实例 ID 请求停止并确认 not_running，见 incomplete-run-cleanup.json。加入第四类兼容样本后，脚本测试总时限调整为 8 分钟再跑相同组合；生产调用时限未改。

最终 result.json 记录 exit_code=0、scenario_count=8，完成时间为 `2026-10-15T09:51:27.4319128Z`。integration.log 核实 8 个顶层场景和 4 个兼容子场景通过，没有 FAIL/SKIP。source-changes.json 记录 32 个变更/新增/删除代码与脚本路径，清单 SHA256 为 `2604700f0d0376296d57ea9f73778157e5e053e7318ab3c480bf15b4dd9041b5`。测试注释中“先存意图再绑定”的表述也已注明为旧 v2 行为，相应保护测试再次通过。

兼容结论仅覆盖四类隔离受控样本。三个原样本继续要求 journal/Snapshot 保持；stop_pending 允许结算状态变化，但要求原请求/ID/幂等键/摘要/完整绑定不变、操作数量不增、写者退出已核验且没有新 thread/start 或 turn/start。

对本次最终输出的四对数据库另作只读复核，全部符合上述要求，见 v2-compatibility-summary.json。Spec/Plan 所选范围及审计确认的问题均已处理，没有遗留待验项；真实模型/真实 PR 和未覆盖的历史状态组合不在此次通过结论内。

## PR 交付前与 main 的整合复验

用户随后授权提交 PR。分支已无冲突地更新到 main `d2f0d094e857d6ff162b68ab1dc9f81d49261dc5`，包含远端已合入的启动权限与基线刷新修复；它们不计入本 PR 的变更。整合后的可靠层、Workflow、application、storage、host 共 40 个顶层单元测试及 vet 通过，日志为本地 `.evidence/reliable-pr-unit.log`；比原审计组合增加 main 的基线刷新测试及 host 测试。

第一次整合复验 `.evidence/foundation-20261015-095739` 中七个业务场景通过，但 unknown 兼容项失败：测试在旧服务退出前取公开视图，旧 worker 随后的合法对账更新了诊断，导致比较时点不一致。现改为严格比较旧服务确认退出后的最终持久基线，保留身份、输入和诊断检查；没有为通过测试而放宽字段比较。该次失败不计为兼容验收通过。

修正后的完整组合以干净源码提交 `69d9a47b2416a7534f74262033e36b6580d5c87c` 重新构建验证，证据为 `.evidence/foundation-20261015-100201`。result.json 为 exit_code=0、scenario_count=8，8 个顶层场景和 4 个兼容子场景全部通过，无 FAIL/SKIP，完成时间 `2026-10-15T10:05:32.8930709Z`。EXE SHA256 为 `B41FE23D30E682C84989F8DBA0EFC41DF67777AFC4BF8428C07C4B85C28BA15D`，测试 EXE SHA256 为 `3C25C2F76EBC2AEB499B0EB59290311ACAD0626BEE4616D407926F4BB12E4D50`；source-identity.json 记录来源。四对数据库的只读复核也通过，见同目录 v2-compatibility-summary.json。

单元测试验证的整合提交为 `e400258131291988ba4cbf602f121e907cb722c9`，到上述构建提交只修改兼容测试取样时点，生产源码与这些单元测试均未改变。此后的提交仅补交付文档；不以文档提交另冒充构建来源。未合并或发布。
