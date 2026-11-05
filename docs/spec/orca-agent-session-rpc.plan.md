# Orca agentSession 单次 RPC 收敛 Plan

按 [Spec](orca-agent-session-rpc.spec.md) 实施。当前只交付文档，以下均待执行。

## P0 核对基线

1. 实现前刷新 `main`，从其建立独立候选分支。对照 `agent_session.go`、`state_local.go`、`state_stream.go` 和 `state_transport.go`，记录连接取得顺序、现有能力集、派发点、请求/回执匹配和错误传播。确认受控夹具能数到真正的业务写入，将握手、`status.get` 和只读对账与业务写入分开计数。
2. 只读确认两端正式 Codex 已有可用目标、授权、隔离仓库及配对，以及本机现有 Pi。确认有可调用的 SteerLine `VerifyUpgradeDrained`、固定的构建入口和独立证据目录。前置不足记录具体阻断，避免用假远端、历史通过或重复提交替代。

## P1 实施及受控回归

1. 保留三个客户端构造入口和两种连接验证。收敛已验证连接后的重复代码，保留原读写标记、检查顺序、超时边界和每次连接所有权。写入仍经本机/远端原发送实现；状态分类和对账仍由原适配器处理。
2. 在 Windows 运行 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1`。沿用 `TestLocalAgentSendWireAndDispatch`、`TestRemoteAgentSessionWireAndDispatch`、`TestRemoteAgentSendWireAndDispatch`、`TestRemoteAgentDispatchRechecksAfterWriterQueue`、`TestAgentIPCDispatchCheckpointPrecedesEveryByte`、`TestStructuredLostCloseRemainsUnknownWithoutReplay`、`TestStructuredDurableCloseReceiptReconcilesWithoutReplay` 和续轮测试。先检查已有断言，只补未覆盖的两端身份/能力零写入和派发后断连的一次写入证明。断连在夹具收到完整业务请求后注入，复用 IPC/WebSocket 夹具及现有生命周期对账用例。
3. 审查 diff。生产改动应集中在现有 Orca 协议包；若共用函数迫使变更 wire、扩大状态接口、修改生命周期或调整工作流，撤回该抽象并缩小接缝。核对 Pi 客户端仍使用原能力集。

## P2 同一候选真机复验

用 `scripts/build.ps1` 生成固定 Windows Runner EXE，记录提交、源码摘要及 EXE SHA256。两端和 Pi 全程使用同一写入 EXE，沿既有验收的真实 Runner CLI、隔离仓库、独立 state-dir 和现有请求方式。实施结论按 #108 的现行规则检查，原报告或提示词只作输入参考。共用 Orca 账户/CLI cache 的用例串行。 [双端与 helper 退役验收](https://github.com/erranliu/SteerRunner/blob/f5c417e84c42737b36a8d632bad02dee166187f5/docs/acceptance/orca-helper-retirement.md) · [#108 的结果规则](https://github.com/erranliu/SteerRunner/blob/f5c417e84c42737b36a8d632bad02dee166187f5/docs/acceptance/implementation-plain-text-result.md)

| 场景 | 操作与通过条件 |
| --- | --- |
| 本机及配对远端，各跑 `execute/1` | 提交含换行、空行和 Unicode 的正文。核对原会话首轮只有一个用户文本块，正文与冻结输入对应；可信最终结果完整，原会话关闭并取得 `result-file/v1`，核对摘要和版本。 |
| 本机及配对远端，各跑 `development/2` | 首轮进入可回复阶段，用一次 `reply` 提交多行反馈。核对原会话 ID 保持、两个原 turn ID 不同、对应用户文本各只有一个。取得第二轮可信结果后显式停止，核对原会话关闭、无可用恢复授权和原 `result-file/v1`。 |
| 本机 Pi `development/2` | 用现有 Pi 配置提交多行首消息，再回复一次多行反馈。核对两条原用户文本各成一个块且同属原会话，显式停止后取得原会话证明和一次 `result-file-snapshot/v1`。复用 [本机 Pi 烟测](../acceptance/orca-native-pi-smoke.md) 的续轮用例。 |
| 双端原目录 | 业务 Run 停止后用原写入 EXE 正常停服。保留原目录，确认服务 `not_running`。同一源码用 `scripts/build.ps1 -Version` 指定不同版本标识，构建第二个只读验证 EXE，记录两个 EXE 的不同摘要及同一源码摘要。通过 SteerLine 生产 `VerifyUpgradeDrained` 用两 EXE 冷读同一原目录，确认原 Run/input 集合、原停止范围及无未决控制。期间只调用读取入口。 |

每场保留 Run、脱敏后的目标与 runtime/worktree/session/turn 绑定、原输入或摘要、最终结果、文件版本/摘要、停机回执和排空结果。原会话关闭状态与停服后 `verify-stopped` 分开记录，在线 Run 状态标签不足以证明排空。真机回执缺失保留原请求并用现有查询核对，禁止补发，无法取证则该场未通过。确需修复时，从新固定候选重跑整张真机矩阵。

把实际命令、候选身份、通过范围及阻断写入独立的 `docs/acceptance/` 记录。已有验收只作操作参考。受控、专项真机和完整发布资格分别记录，保留既有发布门禁。
