# 本机 Codex 多轮会话的 worktree 写入权验收

验收日期为 2026-11-11，基线为 `401bfe6f05327e07925237eafce5cadcb70ac6a6`。以下为合并前验证。公开接口和请求格式见[会话文档](../workflows/conversation.md#写入权交接)。

## 定向受控测试

Windows/amd64 上通过仓库的 `scripts/test-foundation.ps1` 运行
`TestSRManagedConversationWorkspaceWrite` 和 `TestSRContentConversation`，两项均通过，退出码为 0。
测试通过真实 Runner EXE、CLI、服务 IPC、SQLite 和临时 Git 仓库运行，原生 app-server 由协议夹具提供。

| 验收项 | 已核验结果 |
| --- | --- |
| 活动轮与写入互斥 | 首轮活动时获取写入权返回 `workspace_not_idle`。首轮结算后，两个并发获取请求只有一个成功。租约 ID 与成功回执的 command ID 一致。持有期间拒绝新消息，原生 `turn/start` 仍为一次。错误的租约 ID 无法归还写入权。 |
| 重启后核验租约 | 持有租约时停止 Runner 服务。离线 `inspect --command-id` 核对原租约和原 session，新服务实例的在线 `inspect` 返回同一租约，原生进程身份一致。重启后新消息仍被拒绝，原获取请求重放返回同一回执，归还请求可幂等重放。 |
| Rebase 冲突中的原会话续作 | 调用方在租约期间执行真实 Git Rebase 并使其停在冲突。归还后，用 `message` 在原 Run、Attempt、session 和同一 worktree 实例中完成下一轮，夹具从冲突文件读到冲突标记并在该 worktree 写入标记。全程 `thread/start` 一次、`turn/start` 两次，续作前后的未合并索引一致。最终确认原 Run 已停止。 |

证据位于 `.evidence/workspace-write-final/` 的 `result.json`、`integration.log`、
`caller-commands.jsonl`、`TestSRManagedConversationWorkspaceWrite-native-calls.jsonl`
和 `source-identity.json`。Runner EXE SHA-256 为
`b3cb28b55b9980498e905d05f4054532100982d8d12a9501b4120f7184ffa0b3`。
同一 EXE 的定向能力发现回归 `TestSRWorkflowDiscoveryAndSelection` 另行通过，证据位于
`.evidence/workspace-write-discovery/`。复核时该 EXE 的摘要和构建输入摘要均与当前工作树的对应值一致。

SteerLine 负责 Rebase、冲突处理和 Git 结果核验。此处验证 Runner 的交接、持久化与派发契约，
覆盖 Runner 服务重启。真实 Codex 模型、整机重启、完整 Controlled 及合并后资格未验证。

## 完整内部 Go 回归

在 Windows/amd64 宿主使用 Go 1.26.4 运行 `go test ./internal/... -count=1 -timeout=30m -json`。
退出码为 0。37 个包全部结算，33 个包通过，4 个包无测试。784 项顶层测试通过，
12 项 live/helper 测试按仓库默认规则跳过，失败为零。测试前后输入文件摘要一致。
日志、输入摘要和退出回执位于 `.evidence/workspace-write-internal/`。
该结果覆盖完整 `internal/...`，完整 L1 还包含框架、扩展和原生 driver 等独立步骤。

上轮手动运行的 8 分钟包级上限导致 `internal/application` 整包超时。栈快照位于
`TestProgressTerminalPendingFinalizationStillStarts` 的 SQLite 夹具提交。该用例单独复验通过，
在本轮完整回归中也通过。`internal/application` 全包实际耗时约 13 分钟。重跑使用
[仓库测试指南](../testing.md)规定的 30 分钟包级预算，生产代码无需因此调整。

交付前查询 GitHub `main`，远端提交与本工作树基线一致。本记录的测试均在合并前执行。
