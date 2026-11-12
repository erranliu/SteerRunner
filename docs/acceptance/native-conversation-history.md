# 原会话分页读取验收

2026-11-12，基于 `37de4694` 的工作树改动。公开 `native-conversation-history` 命令按原 Run、Attempt、Session、generation 和 Orca epoch 读取一页原生消息，选中长消息后按游标读取该消息正文。读取复用公开 `agentSession.history` 和 `agentSession.item`，不会执行任务控制。

`go test ./internal/execution/adapters/orcatransport -run '^TestNativeConversation' -count=1` 通过。覆盖每次只读取一页、正向顺序、工具与组装指令过滤、拒绝其他 Attempt 和替换后的 epoch、独立读取一条长正文、UTF-8 分段和拒绝变更后的原消息。首次沙箱内包回归中的已有 Git 和跨进程用例遇到访问权限错误。该测试结果不代表生产 Runner 或历史会话已实机复验。

补验的 CLI、契约、服务、资源及 Orca 包回归通过。application 包的较广回归在已有用例 `TestRABudgetReportOnce` 的 Git 子进程中触发 10 分钟总超时，定向 application 查询用例通过。
