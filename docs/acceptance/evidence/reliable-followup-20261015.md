# 中间层后续收敛证据

日期：2026-10-15。依据 [Spec §9](../../spec/reliable-modularization.spec.md#9-后续收敛清理旧规则并简化-advance) 和 [Plan §7](../../spec/reliable-modularization.plan.md#7-用户授权的后续收敛)，沿用 [PR #5](https://github.com/erranliu/SteerRunner/pull/5)。状态：本轮收敛及随后主线整合均完成；最终 47 个顶层单元测试、adapter 定向测试、vet、选定的 8 个受控顶层场景及 4 个兼容子场景全部通过，无失败或跳过。

## 范围与实际修改

- 基线为 `6f1db96ab0537743db4010256b92396d0557ddc8`。全仓 Go 引用核对后删除六个没有生产调用的函数：OperationKey、ValidateSavedIntent、HasUnknownOperation、ResourceOutcome、ReconciledResult、ReconciliationRequest；后两个只有测试消费者。相应删除 SavedIntent、OperationFact 和仅供旧辅助函数使用的 Reconciled 标记。
- 不保留只供测试调用的回执回退路径。操作单测直接调用 Invoke，并补真实 application 对账测试，覆盖原生返回部分会话事实后，后续对账缺少事实时仍保留原会话、原操作 ID/键/输入/摘要/绑定以及持久快照。
- 观察挂接移到 observation.go 的 attachObservation，复用已有 LifecycleObservePort；物化输入和准备推进移到同包 preparation.go 的 prepareRun。gate、授权/权限条件、stop/对账/resume/反馈的优先级继续由 advance 保持。
- 原代码块与新函数体逐段比较相同。advance 仅替换这两段，并移除已被前置条件覆盖的重复答案判断，从 144 行收敛到 71 行；行数不是验收目标。比较结果为本地 extraction-comparison.json。

没有新增生产包、接口、状态对象或存储字段。Record/Intent、stop、saveIntent/invoke、原生参数/权限和服务退出路径未修改，不触发本机真实模型验证。

## 首次收敛验证

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage ./internal/host -count=1 -timeout=3m -v
go vet ./internal/execution/reliable/... ./internal/application ./internal/runstate ./internal/host
```

42 个顶层单元测试及 vet 通过，无 FAIL/SKIP。新增 TestSROperationReconciliationKeepsPartialBinding，以及 TestSRObservationAttachmentCommitsBeforePublishingObserver 的 native_error/commit_failure/success 三个子项。后者验证禁止新输入时仍可只读观察、提交成功后才挂接、失败关闭临时 observer、不重复挂接且 owned 句柄不会降级。原 gate/提交失败/accepted pending/准备期限反例继续通过。

最终受控组合复用 Plan §5 的 8 个顶层场景和 4 个兼容子场景，使用基线 EXE `.evidence/foundation-20261015-085321/steerrunner.exe`；基线 SHA256 为 `9A215F39112E8BFC5807ED2C923DF7481DAAA2A339E1FA6F6EF67ECB651A815E`。

本轮构建来自干净提交 `839cd12466e8fe5c0695b94f9e72e2c543b32ebc`，本地证据目录 `.evidence/foundation-20261015-101842`。EXE SHA256 为 `DA3ED54E460EFFAA38D4EE8E8793FA06F59A8C2AC25E49DD19B217C94E754F21`，测试 EXE SHA256 为 `8BF0CEAFE64601348489FE66BB6B43F3608D2EA88B488609DA168B17C9041F74`。source-identity.json 记录构建来源和参数，reliable-followup-unit.log 与 extraction-comparison.json 保留本轮单元和代码搬迁比较结果；原阶段结果没有替代本轮复验。

result.json 记录 exit_code=0、scenario_count=8，完成时间 `2026-10-15T10:23:49.9807943Z`。integration.log 中 8 个顶层场景及 4 个兼容子场景全部通过，无 FAIL/SKIP。四对数据库的只读复核通过：三类原样本的整个 journal/Snapshot 保持，stop_pending 沿原请求/身份/摘要/绑定完成结算、核验写者退出，且未增加操作；结果为同目录 v2-compatibility-summary.json。

首次收敛的两项修改及其验证均完成；随后主线整合的修改和验证单独登记如下，不沿用旧构建结果。

## 随后的主线整合

推送后发现 main 新合入独立测试分层和执行可观测性，PR 出现 results.go 冲突。已通过 merge 保留发布过的分支历史，合入 main `0a34148c7505bf90b2027b3d2224baf19735ad63`。冲突处理将主线新增的 PublicMessage → acceptMessage 分流保留在迁移后的 observation.go；来源核验、脱敏、只读消息缓存、阶段准备预算重置及其他主线功能均沿用主线实现，没有引入新调度模型。

合入后实际执行：

```powershell
go test ./internal/application ./internal/storage ./internal/execution/reliable/... ./internal/execution/workflow ./internal/contracts ./internal/host ./internal/cli -count=1 -timeout=3m -v
go vet ./internal/application ./internal/execution/reliable/... ./internal/runstate ./internal/storage ./internal/host
go test ./internal/execution/adapters/codex -run '(PublicMessage|Lifecycle|Recovery|Preparation|Policy|Outcome|Receipt|Event)' -count=1 -timeout=3m
```

47 个顶层单元测试、adapter 定向测试和 vet 通过，含公开消息的只读脱敏测试与长回合后准备下一阶段测试；单元组合无 FAIL/SKIP。日志为本地 `.evidence/reliable-followup-merge-unit.log` 和 `.evidence/reliable-followup-merge-adapters.log`。

最终受控组合仍选择 Plan §5 的 8 项及 4 个兼容子项，来自干净合并提交 `f176d7ed3121d158147d9a7ebdf2ba1d6e1d8a48`，证据目录 `.evidence/foundation-20261015-102921`。result.json 为 exit_code=0、scenario_count=8，8 个顶层及 4 个兼容子项全部通过，无 FAIL/SKIP，完成时间 `2026-10-15T10:33:56.9822588Z`。EXE SHA256 为 `0AC2A4AB7616A4D7BE0430E722BE213396869729E72984AE9DCB4AAD68848743`，测试 EXE SHA256 为 `4493542368D80F458FDC71390BEB1FAB77BBA073EF815A6666CC817104A2149A`。四对 v2 数据库也通过同样的只读复核，见同目录 v2-compatibility-summary.json。

合并后的复验没有借用首次收敛 EXE 的通过结论。最终构建后的提交仅补文档；本轮收敛及主线冲突处理无待验项。这里的受控选择不冒充整个新测试分层或真机模型验收；未合并 PR 或发布。
