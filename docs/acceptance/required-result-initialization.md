# result_file 必需初始化验收

受测 Runner 生产提交为 **ed2ac946f8806f6ae3edfb13da8b78b6ec47d072**，构建版本为 **0.2.0-dev.4-required-result-init**，EXE SHA-256 为 **cb88b7a3982f1e9ab681e3f7955299ed5db33d03a54c4dfb3c5b240828e512e3**。以下为合并前验证。原始证据位于本机独立目录 C:\SteerlineDebug\wt-1851-required-result-init\。

| 验证 | 结果 |
| --- | --- |
| TestRequiredResultInitializationGatesFirstNativeInput | 正确种子恰好派发一次。初始化超时、缺失、摘要不符均无原生输入调用、无持久化输入意图，保留具体诊断。 |
| 直接受影响 Go 测试 | runtime files、result file、snapshot、workspace reuse、preparation deadline、公开 schema 通过。见 diagnostics/runner-gate/go-test.log 和 diagnostics/runner-file-owners/go-test.log。 |
| Runner Controlled 定向选择 execute-stop,runtime-files | 通过。覆盖普通调用四组文件故障、四组最终封存、progress、execute 冷停止、原生未知结果及重启，见 diagnostics/runner-controlled-8ceb5222/summary.json。 |
| 原两轮初始化故障复验 | 同一 worktree 第二轮 info/exclude 探测延迟 5.2 秒，原生进程读取了各 Run 的独立种子，两次封存均为版本 1，服务已结算。见 diagnostics/replay-delay-3a6c4b68/。 |
| SteerLine 消费者 | 仅维护可写请求启用。正常观察与宿主离线恢复均保留失败事实，原 Run 停止、封存后行动可收尾。公开契约夹具验证四次可写 Run 使用同一 worktree 与各自的种子。 |

Runner 的完整 application 包曾触及 Go 默认 10 分钟总超时。上表的定向回归和 Controlled 选择独立通过，覆盖范围以表中项目为准。

固定 Runner 在 SteerLine 的本机 Codex + GitHub 专项完成两轮结果封存，测试 PR #811 的初次 head/base 与报告正文逐字核验通过。该次整链在随后提交的 SteerLine 冲突决策处阻断，测试资源已清理。冲突接线修复及后续复验由 SteerLine 的专属验收记录维护，见本机 test-results/L4/live-20261112T102440Z-653300cd/。
