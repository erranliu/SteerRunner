# Runner 提交回执持久化验收

2026-11-12，基线 `37de4694` 已包含 Runner #117。SteerLine 配套基线已包含 #456。原 308 环境检测记录 `4ee2990b037c4f409ba1fa68725e54b8` 的 Runner 提交 `77b23682805a4d13f0aadb60d4a7b5d5` 已到达服务，回执发送失败，后续按原 ID 查询无记录。

Runner 在受理前持久化原请求与摘要。原命令在准备中可查，准备失败后保留拒绝原因。受理成功时，同一事务写入 Run、接受回执和事件。独占写入服务重启后结算未创建 Run 的准备中命令。同 ID 同正文回放，改正文冲突。回执发送失败日志记录处理耗时与返回错误码。此次沿用现有存储格式。

- `go test ./internal/storage ./internal/host -count=1 -json` 本机运行通过。用例覆盖断连后查询原拒绝、同 ID 冲突、重启结算、事务提交与重复 Run 防护。证据为 `C:/SteerlineDebug/wt-2db3-submit-receipt/diagnostics/runner-receipt/host-storage.jsonl`。沙箱内早期宿主执行因 Windows 访问受限失败，本机复跑通过。
- `go test ./internal/application -run 'Test.*(WorktreeReuse|BaseRefresh|PinnedBase|SingleExecution|ContentRepository)' -count=1 -json` 通过，证据为同目录 `application.jsonl`。
- 308 `AIRefactor` 固定提交 `d2eee8a6487ea076d8bba65fa52589a683611143` 走本机 Codex `project-default` 与正式环境检测入口。记录 `df29a8b7450744a8bd1000376aa5ea33`，唯一 Run `e884df58e817ed6ab4806c78e7dcf230`。十项通过，Run 已停止并封存结果，独立服务正常停止。Runner EXE SHA256 为 `b94dc53c2ec64f980bedbfd41c371d291d372c1548cb1758be064ca8d638d1b8`，真实证据在 `C:/SteerlineDebug/wt-2db3-submit-receipt/live-308/attempt-1/evidence`。断连路径由回归用例覆盖。
