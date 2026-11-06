# 实施纯文本结果验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-05，基线为 GitHub `main` 的 `ae97f5a4`。本次变更用于新 Run，旧实施定义按现有冻结定义校验拒绝接续。

- `implement.execute` 和 `implement.report` 均逐字匹配本次可信最终回复的最后一个非空行 `实施及必要验证已完成`。旧标点、正文片段、围栏和旧 JSON 均不能完成实施。原生轮次、原实施来源及工作区核验仍是推进条件。
- 执行末行 `未完成` 直接等待输入。结论不明时在原会话补报一次。可信补报未命中成功标记时，保留说明并等待输入。

## 合并前定向验证

- `go test ./internal/execution/workflow ./internal/execution/resultacquisition ./internal/resources -count=1 -timeout=5m` 通过。
- 应用定向用例 `TestRAImplementationReportCompletesOriginal`、`TestRABudgetReportOnce`、`TestRAStateAuthorityIncompleteSource`、`TestRAUnreportedOutcomeBoundary` 通过。覆盖补报绑定原实施、补报次数、明确未完成直接等待和原生结果资格。
- 应用扩展定向回归通过，覆盖结果获取、输入预算、冻结预算、项目策略、原生证明和补报权限。
- 受控集成 `TestSRResultAcquisition/plain-marker` 和 `TestSRRunWorkflow` 通过，分别覆盖直接推进到 PR 就绪和一次补报后推进到 PR 就绪。
- 协议夹具包通过。扩大到 `internal/testkit/livetest` 全包时，`TestSRNativePreparationReferencesAreStaticConstraints/absolute-uninstalled` 失败，未修改的 `main` 基线复现同一失败。

本次验证使用本地协议夹具和隔离 Git 仓库。完整 L1、完整 Controlled 和真实模型测试未运行。
