# 执行可见性增量 Plan

用户授权的普通开发，不进入 roadmap。共同流程规则参见 SteerLine 的 `docs/product-design/worker-lifecycle.md`；本次只增加 W11–W14/V04/V06 观察和查询的可见性，不改变输入授权、重试、结算或恢复规则。验收见[专属记录](../acceptance/execution-observability.md)。

## 实施

1. 在 aggregate/event 的同一事务比较真实 workflow stage、status、diagnostic，仅变化时追加 `workflow_changed`，携带 `transition.from/to`。不从操作名推断状态，不为旧事件补历史。
2. 复用本地 Codex 的 app-server 观察流，允许 completed agentMessage 和固定工具活动标签。排除 reasoning、用户输入、命令、工具参数和输出。时间使用通知到达的 UTC `observed_at`。
3. 按 Run/Attempt/Generation/Session/Turn 核验后保存每 Run 一条进程内摘要。脱敏后上限 1200 UTF-8 字节及省略号；过期、乱序或不支持的消息直接丢弃，不中断执行、不写业务 revision/event。
4. 新增只读 `last-message --run-id` CLI。读取缓存，不启动服务、不读私有会话文件、不启动原生查询进程、不发送输入。服务重启缓存为空；无样本返回 `message:null`，正常执行观察后可产生新样本。Orca 当前不采集该摘要。

## 测试计划

| 自动项 | 级别 / 场景 / 环境 | 命令 | 时点与通过条件 | 证据 |
| --- | --- | --- | --- | --- |
| ROV-A01 | 定向 Go，SQLite 隔离夹具 `TestWorkflowTransitionsAreAtomicHistoricalFacts`、既有 `TestSRRunAtomicCommit` | `go test ./internal/storage -run 'TestWorkflowTransitions|TestSRRunAtomicCommit' -count=1 -v` | 提交前；回滚零部分写入、真实前后值不可被后续状态改写、无变化不重复 | acceptance ROV-A01；原日志 `.evidence/observability` |
| ROV-A02 | 定向 Go，本地 mock 原生协议及 application 内存夹具；PublicMessage、LastMessage、AsyncQuestionObserve | `go test ./internal/application ./internal/execution/adapters/codex ./internal/cli -run 'TestLastMessage|TestPublicMessage|TestAsyncQuestionObserveSteerAndReconcile|TestSRCLIContract' -count=1 -v` | 提交前；白名单、身份隔离、乱序丢弃、脱敏有界、查询零状态推进、重启无伪样本，既有问答与终态照常 | acceptance ROV-A02；同上 |
| ROV-A03 | 受控真实 EXE/常驻服务，隔离临时仓库、原生 helper、loopback provider；`TestSRRunWorkflow` | `go test ./tests/integration -run '^TestSRRunWorkflow$' -count=1 -v -args -evidence-dir=<独立证据目录>` | 两侧接入后；真实 CLI 返回 transition 与原时间摘要；重复读取 revision/原生输入计数不变；反馈仍在原 session/PR 完成 | acceptance ROV-A03；集成产物目录 |

不跑完整模型/真实 GitHub PR、Orca 或全量回归：本次不改执行推进规则，以上已覆盖公开查询、实际 EXE/服务与观察流；不把这些结果声称为原生真机认证、恢复或交付通过。安装目录和任务 105 不作为测试现场。
