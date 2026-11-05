# Orca agentSession 单次 RPC 验收

2026-11-05。基线 `f5c417e8`，候选源码 `1b6f231dc540f8d6e2e785ddec4be449d0242df0`。两个 Windows/amd64 EXE 均由 `scripts/build.ps1` 构建，源码摘要为 `d7dcf9994526a1bee0e128aae40d26771eff85ae4ed7da10ac6a2d9914b8a00a`。

| EXE | SHA256 |
| --- | --- |
| 原写入 | `0b0b9aeb5e964007aec95b142acf7fabcdb6256f352841a9486d2ecc7906fc85` |
| 第二冷读，`-Version 0.2.0-rpc-reader` | `933088cb89edd750a6cd7a6c4963edfd0b02de4a4c92386361ac0f60b23b80c3` |

Windows 定向运行 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1` 通过。补齐本机 Codex/Pi 探针能力集夹具后，协议包再次通过。现有 IPC 和加密 WebSocket 夹具验证身份或能力不符时业务零派发，丢回执时业务只写一次。生命周期夹具验证关闭未知后不补发，已持久关闭回执可按原规则恢复。

五场串行使用同一个写入 EXE。首消息和反馈包含 CRLF、空行和 Unicode。Runner 已确认原文本块的输入证据，冷读记录核对每个已接受回合的原消息 ID 及冻结正文 SHA256。两个 `execute/1` 都返回预期三行可信结果。三个 `development/2` 都在原会话取得两个不同的已验证回合，按现行 `未完成` 规则等待输入，第二轮后显式停止。五场停止后的恢复授权均已撤销。

| 场景 | 原 Run | 文件合同 |
| --- | --- | --- |
| 本机 Codex `execute/1` | `4943bfdda189a7af429725b7aef1194e` | `steerrunner-result-file/v1` |
| 配对远端 Codex `execute/1` | `ce3da15650000cdf6f8f75b689464f66` | `steerrunner-result-file/v1` |
| 本机 Codex `development/2` | `43fd15189371c30a2a81b5695df7aa5e` | `steerrunner-result-file/v1` |
| 配对远端 Codex `development/2` | `8449b13e0048d094b298a3e1c4a3fdbe` | `steerrunner-result-file/v1` |
| 本机 Pi `development/2` | `f34b35c4b207f184fe41cf3531ee12c1` | `steerrunner-result-file-snapshot/v1` |

五场用相同预置文件内容，均返回版本 1、采集状态 `closed` 和预期 SHA256。本项覆盖停止后封存及 Pi 单次快照的读取，模型回合只操作文本。原始 CLI 回执、输入绑定与冷读日志存于本机 `.evidence/orca-agent-session-rpc-live/`。

五个原目录均通过 SteerLine `e010373e` 的生产 `VerifyUpgradeDrained`。临时 Go overlay 只调用现有入口，以两份固定 EXE 读取同一原目录。已确认原服务为 `not_running`，原 Run/input 和 `orca_agent_session` 停止范围一致，无未决控制。前后对比原数据库和停机证明摘要一致。验收范围为受影响组件和专项真机。
