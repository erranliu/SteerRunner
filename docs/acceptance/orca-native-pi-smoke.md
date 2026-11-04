# 本机 Orca Pi 结构化烟测与 helper 退役范围

2026-11-04，本机 Orca 1.4.246，已安装 Pi 0.87.1。两场使用同一候选，源码摘要 `abe5038c9398bde4b7c0972c5e7d4f20f018b0322a15e957a2a53147b3cef728`，EXE SHA256 `6f2412ec0ec134a3d774c6f172789f37a3282985f4f328484e3994eb8904a5e9`。真实 Runner CLI 和隔离测试仓库，原始材料位于 `.evidence/orca-native-tail/`。

| 场景 | 实际证据 |
| --- | --- |
| `execute/1` | Run `f8570bbe7cfc0a7ab7bb7d7a36a8d8f4`。一个原用户文本块，摘要与冻结多行正文相同。公开历史可见 `bash` tool-call，最后返回原工具验证标记及多行结果。业务结束采集一次 4 字节快照。 |
| `development/2` | Run `3c87031e86d1827a34cabaa9b29e8b22`。同一原会话收到两条完整用户文本块，每条摘要均与对应冻结正文相同。第一轮等待业务反馈，包含 CRLF、空行与 Unicode 的一次回复进入不同的第二回合。显式停止后采集一次 4 字节快照。 |
| 停止与文件 | 两条 Run 均取得 `orca_agent_session` 的原会话关闭确认和 `steerrunner-result-file-snapshot/v1` 版本 1。停服后以原目录运行 `service verify-stopped`，两场均确认会话范围已结算。 |

新 Pi 使用本机 Orca 的工具及环境，停止证明限原会话，文件为一次采集。公开 Pi 消息身份无法证明原 usage 响应归属，当前不产生新 Pi 计量记录。未验远端 Pi、原生问答、发布或活动回合停止。两场使用 Orca 现有默认 Pi 选择。

烟测构建之后移除了 `steerrunner-agent` 入口，并在新 Runner 拒绝恢复旧终端执行。新 Codex 和 Pi 的显式启动覆盖在准入前拒绝。旧终端 Run 须由旧构建排空。最终组合需用最后候选按影响复验，烟测构建的资格不转给最终构建。
