# 本地 Codex app-server 会话分页验收

2026-11-12，基于 `97b78b39` 和本 worktree 改动。原 `native-conversation-history` CLI、服务绑定和响应契约同时支持 Orca 与本地 Codex app-server。Codex 使用冻结的执行配置、Codex home 和公开线程元数据校验，再通过 `thread/items/list` 向前分页。只投影助手文字与可核实的用户文字。长正文重新校验原条目和正文摘要后按 UTF-8 字节分段。[公开 App Server 接口](https://learn.chatgpt.com/docs/app-server#list-thread-items)

## 保留的环境检测会话

- 原生 Codex 0.166.0。新 Runner 读取处理器用每页 1 个原生条目走完 9 页，第 1 页的 184 字节助手文字与已核实最终回复一致，第 7 页另有一条 138 字节的助手过程消息。错误 Attempt 被拒绝。
- 使用新 Runner CLI、实际 Runner 服务分派器和 SteerLine 公共活动读取路由联测同一记录。隔离命名管道只处理读取，原 Run 行用于验证绑定，原 Codex 会话通过公开接口读取。活动路由按时间顺序返回两条助手消息，先是过程消息，再是已核实的最终回复。日志位于本 worktree 的 `.evidence/native-conversation-original-run/original-run-activity.log`。Runner CLI 构建 SHA-256 为 `72735bb2bbc8ad0669d71c491bca85ca2713bb518af9008c03b220c6759f6305`。
- 现有安装保持原版本。联测未启动业务 worker、恢复线程或发送输入。

## 自动验证

- `go test ./internal/execution/adapters/codex ./internal/host ./internal/cli -count=1` 通过。
- `go test ./internal/execution/adapters/codex ./internal/application ./internal/host ./internal/cli -run 'NativeConversation|NativeViewerDoesNotStartService|SRFullBinding' -count=1` 通过，覆盖 Codex 分页、角色过滤、正文分段、异源游标拒绝，以及原 Orca 绑定和命令校验。
