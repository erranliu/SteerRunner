# 中间层后续收敛证据

日期：2026-10-15。依据 [Spec §9](../../spec/reliable-modularization.spec.md#9-后续收敛清理旧规则并简化-advance) 和 [Plan §7](../../spec/reliable-modularization.plan.md#7-用户授权的后续收敛)，沿用 [PR #5](https://github.com/erranliu/SteerRunner/pull/5)。状态：实现、42 个顶层单元测试、vet、8 个受控顶层场景及 4 个兼容子场景全部通过，无失败或跳过。

## 范围与实际修改

- 基线为 `6f1db96ab0537743db4010256b92396d0557ddc8`。全仓 Go 引用核对后删除六个没有生产调用的函数：OperationKey、ValidateSavedIntent、HasUnknownOperation、ResourceOutcome、ReconciledResult、ReconciliationRequest；后两个只有测试消费者。相应删除 SavedIntent、OperationFact 和仅供旧辅助函数使用的 Reconciled 标记。
- 不保留只供测试调用的回执回退路径。操作单测直接调用 Invoke，并补真实 application 对账测试，覆盖原生返回部分会话事实后，后续对账缺少事实时仍保留原会话、原操作 ID/键/输入/摘要/绑定以及持久快照。
- 观察挂接移到 observation.go 的 attachObservation，复用已有 LifecycleObservePort；物化输入和准备推进移到同包 preparation.go 的 prepareRun。gate、授权/权限条件、stop/对账/resume/反馈的优先级继续由 advance 保持。
- 原代码块与新函数体逐段比较相同。advance 仅替换这两段，并移除已被前置条件覆盖的重复答案判断，从 144 行收敛到 71 行；行数不是验收目标。比较结果为本地 extraction-comparison.json。

没有新增生产包、接口、状态对象或存储字段。Record/Intent、stop、saveIntent/invoke、原生参数/权限和服务退出路径未修改，不触发本机真实模型验证。

## 验证

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage ./internal/host -count=1 -timeout=3m -v
go vet ./internal/execution/reliable/... ./internal/application ./internal/runstate ./internal/host
```

42 个顶层单元测试及 vet 通过，无 FAIL/SKIP。新增 TestSROperationReconciliationKeepsPartialBinding，以及 TestSRObservationAttachmentCommitsBeforePublishingObserver 的 native_error/commit_failure/success 三个子项。后者验证禁止新输入时仍可只读观察、提交成功后才挂接、失败关闭临时 observer、不重复挂接且 owned 句柄不会降级。原 gate/提交失败/accepted pending/准备期限反例继续通过。

最终受控组合复用 Plan §5 的 8 个顶层场景和 4 个兼容子场景，使用基线 EXE `.evidence/foundation-20261015-085321/steerrunner.exe`；基线 SHA256 为 `9A215F39112E8BFC5807ED2C923DF7481DAAA2A339E1FA6F6EF67ECB651A815E`。

本轮构建来自干净提交 `839cd12466e8fe5c0695b94f9e72e2c543b32ebc`，本地证据目录 `.evidence/foundation-20261015-101842`。EXE SHA256 为 `DA3ED54E460EFFAA38D4EE8E8793FA06F59A8C2AC25E49DD19B217C94E754F21`，测试 EXE SHA256 为 `8BF0CEAFE64601348489FE66BB6B43F3608D2EA88B488609DA168B17C9041F74`。source-identity.json 记录构建来源和参数，reliable-followup-unit.log 与 extraction-comparison.json 保留本轮单元和代码搬迁比较结果；原阶段结果没有替代本轮复验。

result.json 记录 exit_code=0、scenario_count=8，完成时间 `2026-10-15T10:23:49.9807943Z`。integration.log 中 8 个顶层场景及 4 个兼容子场景全部通过，无 FAIL/SKIP。四对数据库的只读复核通过：三类原样本的整个 journal/Snapshot 保持，stop_pending 沿原请求/身份/摘要/绑定完成结算、核验写者退出，且未增加操作；结果为同目录 v2-compatibility-summary.json。

本轮两项收敛及其验证均完成，未留下待验项。验收后的提交仅补文档，不冒充新的构建来源；未合并或发布。
