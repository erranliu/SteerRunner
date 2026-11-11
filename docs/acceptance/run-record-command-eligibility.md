# Run 记录命令资格与公开投影解耦验收

2026-11-11，基于 `a0d25c2cf21145e573a0f6d7fd426dbc38954443` 的合并前验证。[写入权契约](../workflows/conversation.md#写入权交接)

`workspace acquire` 与 `message` 从 `runstate.Record` 读取资格。公开 `RunView` 投影复用同一消息判定，受管会话的 `workspace_write.can_acquire` 沿用原消息资格。原普通会话、受管会话和开发完成后的续聊条件保持，待对账仍按原 `Intent.NeedsReconciliation` 判定。

`Engine.Handle` 仅替换两处资格读取。工作区与原生空闲核验、revision 重查、命令事务、持久租约、worker 派发门和 `interrupt` 的原分支保持。公开字段与持久格式保持。

## 实际验收

Windows/amd64，PowerShell 7。以下用例均无失败或跳过。

| 验证 | 基线 | 实施后 |
| --- | --- | --- |
| 普通会话及开发续聊定向应用测试 | 4 项、22 节点通过 | 4 项、22 节点通过 |
| 原记录资格与投影一致性 | | 1 项、11 节点通过 |
| 普通会话、受管写租约受控场景 | 2 项通过 | 2 项通过 |

记录级测试覆盖普通与受管会话的差异、开发续聊、活动轮、待停止、租约持有、缺原 worktree 绑定，以及已受理操作是否仍有未决资源。受管场景覆盖活动轮拒绝、并发获取仅一项成功、持有期间零新输入、服务重启保持原租约、归还后复用原会话与 worktree 完成下一轮。两个受控场景都通过真实 Runner EXE、CLI、IPC、SQLite 与隔离原生夹具执行。

```powershell
go test ./internal/application -run '^(TestContentConcurrentMessages|TestContentConversationControls|TestDevelopmentRunMessagesRetainOriginalWorkflow|TestDevelopmentRunMessageStopCancelsOnlyUnsentInput)$' -count=1 -timeout=5m -json
go test ./internal/runstate -run '^TestRecordMessageAndWorkspaceAcquireEligibility$' -count=1 -timeout=5m -json
./scripts/test-foundation.ps1 -Pattern '^(TestSRManagedConversationWorkspaceWrite|TestSRContentConversation)$' -OutDir .evidence/run-record-command-eligibility/after-controlled
```

受控入口内的正式 Windows 构建通过。候选 `source_dirty=true`，`source_digest=a93fb3f809b67fae6aeeceec63b9b6204388b547716d6336c767bf93746f3728`，Runner EXE SHA256 为 `cf67e7f84ebfb6bc361d6e0dcdb3d759133bcc8efa4f9e461cd9c17082027ff0`，与实际通过的 EXE 和构建输入一致。基线和最终原始输出分别保存在本工作副本 `.evidence/run-record-command-eligibility/`。

验收限于定向组件和两个定向受控场景。完整 L1–L4、完整 Controlled 和真实模型场景未运行。
