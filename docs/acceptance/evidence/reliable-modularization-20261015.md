# SteerRunner 可靠执行模块化收尾证据

日期：2026-10-15。依据 [Spec](../../spec/reliable-modularization.spec.md) 和 [Plan](../../spec/reliable-modularization.plan.md)。状态：本次 RM-P0–RM-P3 全部执行并验收，单元/vet/最终受控及兼容复验通过。首次 operation 阶段的独立证据见[阶段记录](reliable-operation-20261015.md)。

本文件保留 RM-P2 收尾时的构建证据。随后用户要求的实现审计发现并修复两个既有可靠性缺陷，同时补齐并发及操作结算兼容覆盖；最新源码和验证见[审计记录](reliable-modularization-audit-20261015.md)，不沿用本文件的构建 SHA 作为审计后结果。

## 1. 五项职责的处理结论

| 职责 | 本次处理 | 保留原位的依据 |
| --- | --- | --- |
| operation | 独立 operation 包集中原意图、固定输入、派发/对账、普通/stop 结算 | 宿主继续持有锁/gate、原子提交、资源所有权和命令回执 |
| session | reliable/session.go 集中快照回执合并；RecordTurnStart 共用于回执和 started 事件，移除重复字段转换 | 原恢复资格和 Attempt 切换仍由控制路径按原顺序编排，无重复实现，不额外拆包 |
| observation | application/observation.go 集中事件接收、去重/原问题/原答案、观察结果提交 | 宿主接收/公开投影与提交仍需同事务；来源归一化沿用 reliable/result.go，无须新连接框架 |
| control | control.go 保留授权、资格和生命周期；业务反馈/恢复步骤选择进入 feedback.go | 原生 stop/shutdown 的后端差异、旧写者核验及精确句柄所有权均保留，未引入通用停止策略 |
| preparation | linked worktree 权限助手归到现有 policy.go；worker 不再导入 concrete Codex/workspace 实现 | Next/QualifyAction/environment 规则原本已独立，继续复用；私有权限材料留原生接线，不增加新准备包 |

application 仍是唯一 Record/Store 写者。没有新 View/Change 协议、模块存储、schema 字段、加载阻断、历史投影重建或四个额外职责包。

## 2. 当前来源

- 基线：`af545563c58171c584bd35d098166cf39012e699` 加 `codex/reliable-operation` 工作树变更，尚未提交。
- 当前受控证据：仓库本地 `.evidence/foundation-20261015-091418`，未纳入源码；source-identity.json 记录真实版本/构建/测试 pattern。
- 当前 EXE SHA256：`99E3388C2AA73DC0D9391F73DAA1CC1916757FDB77A7D638A05120552F603B55`。
- 兼容基线 EXE：首次源码基线构建，SHA256 `9A215F39112E8BFC5807ED2C923DF7481DAAA2A339E1FA6F6EF67ECB651A815E`。
- Go `1.26.4 windows/amd64`，Windows `10.0.26200.0`。当前 32 个变更/新增/删除的 Go/PowerShell 路径记录在本次目录 source-changes.json，清单 SHA256 为 `a6683cf93814db2ed1311d9c7aa391a55073e36edec80cf4d0460ff5e0c4a769`；不以旧阶段清单代替。

## 3. 单元与行为复验

本次最终源码已经通过：

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage -count=1 -timeout=3m
go vet ./internal/execution/reliable/... ./internal/application ./internal/runstate
```

新增 TestSessionReceiptSeparatesAcceptedInputFromTurnStart、TestSessionReceiptRejectsForeignBindingWithoutPartialChanges、TestSRObservationCommitFailureKeepsEventReplayable、TestSRResumeWorkflowRetainsOriginalReportSource。分别验证接受输入不冒充开始、回执/事件共用原 command/turn 绑定、拒绝异会话时不泄漏部分字段、事件提交失败可重读且重复幂等/冲突拒绝、恢复报告保留原来源/权限/会话与历史输入。既有事务失败及 stop 表示保护继续通过。

本次真实 EXE 复验命令见 Plan §5，选择七个业务场景加兼容项共八项。result.json 为 exit_code=0、scenario_count=8，integration.log 中八项及兼容的三个子项全部通过，无失败/skip。34 个顶层单元测试通过，清单见 unit-test-list.log；vet 无错误。

兼容项使用真实旧 EXE 创建 pr_ready/stopped/unknown 三类 v2 状态，然后新 EXE 接续同一隔离目录并两次重启；不是直接伪造业务数据库。只读复核三对数据库的格式仍为 v2、整个 Intents/Snapshot 相同，结果见本次目录 v2-compatibility-summary.json。当前代码清单与实际测试时保持一致。

另逐函数核对 closeWorker、writersAbsent、feedbackValue、nativeAnswers、applyObservation、attachWriteRoots、worker.run 七个函数体与源码基线相同（仅位置可变），记录在 unchanged-lifecycle.json。会话合并/Turn 开始字段转换集中到可靠层，业务恢复选择通过两个普通函数搬迁；没有更换原生调用、退出/确认顺序或提交边界。

## 4. 完成边界

本次 RM-P0–RM-P3 均完成，RM-A01–A08 按本次范围通过；RM-P2 五职责逐项有处理/保留依据，RM-P3 采用当前 EXE 复验。operation 的旧阶段通过没有代替本次收尾。

原生协议/参数/权限/停止与服务退出次序均未改变，RM-A09 不触发；不运行真实模型或真实业务 PR，也不扩大 Orca 真机支持。本阶段验收时尚未提交、合并或发布；后续交付授权和提交以 Git/PR 记录为准。
