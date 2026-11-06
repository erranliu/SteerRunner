# 单次执行完成回执验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

远端 Orca 的单次执行现场已经核实原回合结束和最终回复，但原生结果分类未上报。旧 Runner 将通用 execute/1 留在 reconciling，调用方直到超时仍无法按正常完成路径处理结果。业务通过判定归调用方。长期边界见[工作流文档](../../../../workflows/README.md#单次执行的完成回执)和[总 Spec](../../../../spec/steerrunner.spec.md)。

## 合并前验证

基线为 main 的 `2ff992d0`。以下为定向验证。

- `go test ./internal/application -run '^(TestSingleExecution|TestContentGenerateResults|TestRAUnreportedOutcomeBoundary)' -count=1 -timeout=5m` 通过。新增应用回归验证原回合完成、`unreported` 原样回传、关闭与停止后的结果封存，以及显式 unknown、未核实或空回复、回合尚未结束的反例。
- `go test ./internal/execution/reliable ./internal/contracts ./internal/cli -count=1 -timeout=5m` 三包通过。
- `go test ./tests/integration -run '^TestSRSingleExecutionColdStop$' -count=1 -timeout=8m -v` 通过。现有跨进程场景核验原输入一次、离线停止接续和结果封存。

本轮使用隔离夹具，真实远端 Orca 修复后的回放和完整分级测试待另行验收。
