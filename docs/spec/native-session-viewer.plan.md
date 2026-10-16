# 公开原会话查看入口

关联[专属验收](../acceptance/native-session-viewer.md)。调用方只通过 `steerrunner-cli/v1` 的公开身份打开原会话；不暴露 adapter binding、endpoint 或私有路径。

`open-native-session --run-id R --attempt-id A --session-id S --generation G --state-dir <absolute> --json` 不接受业务输入，也不自动启动服务。原服务在 worker gate 内校验当前 Run/Attempt/ResourceGeneration/Session，适配器只读核对原进程和 thread/workspace，然后打开无 prompt 的 Codex CLI。响应含同一组身份和 `opened:true`，表示 Windows 已创建窗口，不保证 CLI 后续连接。丢失响应保持未知，调用方不自动重放；调用方 HTTP 持久化原开窗请求。capability 的 `native_session_viewer.adapters` 只声明当前 Windows Codex 实现；Orca 不宣称支持。执行状态、节点、异常、重试和恢复授权不变。

## 测试计划

NSV-R01/R02/R03 均由 `go test ./internal/application ./internal/cli ./internal/execution/adapters/codex ./internal/execution/process -run 'Test(NativeViewer\|NativeConsole\|SRCLIContract)' -count=1 -timeout=2m` 覆盖：实现后在隔离临时目录、loopback WebSocket、Windows 独立 helper 执行。分别要求过期身份/unsupported/停服零开窗且聚合不变、只有 handshake/thread-read 且进程/会话/工作区不符拒绝、窗口标准句柄与含特殊字符参数正确，以及无服务时零自动启动。证据 `.evidence/native-session-viewer/runner-tests.log`，结果仅写专属 acceptance。

调用方实际集成在 SteerLine 的独立 debug 包验证，见其 `docs/plans/native-session-viewer.plan.md` 的 NSV-A04；不直接使用安装环境，不跑全部 L1/L2/L3 或模型交付。人工真实 CLI 查看体验后置登记，不以 helper 代替。
