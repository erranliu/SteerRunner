# 提交前主线整合回归

完整 L3 三轮验收对应归档提交 `9425d2a433d533ae6f3ceee24ec687e60bac6d83`。此后整合 main `f156263`（已合入可靠执行模块重构和执行可观测性），解决 `internal/application/control.go` 的停止结算冲突：保留主线的 `operation.Outcome` 与 `Intent.SettleStop` 调用，把原修复中经过原操作身份校验的进度持久化放进 `internal/execution/reliable/operation/record.go` 的 `SettleStop`。原操作 ID、授权撤销及未知结果不重放规则保持不变。

整合版本执行 `./scripts/test.ps1 -Level Controlled -OutDir .evidence/l3-pr-main-integration-01`，完整通过：

- L1 178 个顶层测试；含原停止进度经 SQLite 重载后继续同一操作、外来回执拒绝等回归。
- 脚本检查 12 项；live driver 编译及零匹配 guard。
- Controlled 12 场景；含调用方丢失响应、撤销跨重启、服务所有权和完整工作流。
- `go vet ./internal/... ./cmd/...` 退出码 0。

证据：[批次汇总](main-integration/summary.json)、[脚本检查](main-integration/framework-result.json)、[Controlled 结果](main-integration/controlled-result.json)、[Controlled 日志](main-integration/controlled.log)、[305 个受测来源文件 SHA256](main-integration/source-snapshot.json)。回归时处于尚未提交的 merge 工作树，因此报告仍如实保留当时 HEAD 和 dirty 标志；回归结束后逐项核对来源未变化再完成合并提交。

整合版本没有重跑完整 L3 真机三轮。先前 36/36 的原始结果、构建身份及 288 文件来源快照独立保留，属于归档的受测提交，不能冒充整合版本的新资格。
