# Runner #172 L1 / Controlled 失败闭环

2026-11-20，按用户要求继续修复 [PR #172](https://github.com/erranliu/SteerRunner/pull/172) 的剩余验证失败，通过完整验证后再取消 draft。起始提交 `9b9a1f18fb0332ee1c84863c93c31cd59c4ddaba`；原失败及原始证据保留在[启动索引专项](codex-startup-index-reuse.md)，不覆盖旧报告。未部署、未合并，不操作原任务。

## 定位与修复

- `TestSRStandalonePackage` 的旧断言要求提交准备失败后 inspect 返回 request_not_found。原 caller-commands.jsonl 实际显示命令已被正确保存为 receipt_status=rejected、action_status=rejected、run_id=null，并保留 environment_unavailable 错误；符合现行[提交持久化合同](../cli-guide.md)和[原验收](durable-submit-admission.md)。更新受控集成断言，核对原请求/命令、拒绝状态及原错误、无 Run，并再次用同一请求重放后按原 command_id 核对拒绝身份和无 Run；不修改生产受理逻辑。
- `TestSREndpointPublicationWithDiscoveryReader/persistent-reader` 原实现最多 100 次、每次睡 5ms，但 Windows 定时粒度与负载可能使一次短睡眠显著超时，实际 2.43 秒才失败。生产替换现在按单调时钟约束原预期 500ms 重试窗口，睡眠后若预算已尽立即返回原共享冲突；仍只重试 Windows sharing/access 错误、保持原子替换。既有真实文件句柄测试的两秒上限及原/新文件内容断言保持。
- `TestCLITimeoutStartsAfterAdmission` 的一秒 attempt 包括版本查询和 OS 子进程启动，整批负载下未到 started-2 屏障就已合法超时；这不表示排队期间启动了期限。测试改用生产最小合法值 10 秒，保留排队时未读 deadline、准入后已读 deadline、真实 CLI 到达屏障、DeadlineExceeded、零重试等断言；另外取消失败路径的排队调用，避免测试退出后留下等待进程。生产配置、attempt 期限实现及并发准入不变。

本轮不修改提示词、模型策略、注册测试范围、正式资格判定或生产超时配置。原 L1 的输入身份变化是上轮测试过程中修改了参与指纹的源码；本轮先固定代码，再串行执行完整 L1/Controlled，执行中不改输入。

## 定向验证

证据根 `C:/SteerlineDebug/wt-9ed5-runner172-qualification/`，受测版本为上述提交加本节生产/回归差异。

- `go test ./internal/host ./internal/textanalysis -run '^(TestSREndpointPublicationWithDiscoveryReader|TestCLITimeoutStartsAfterAdmission)$' -count=3 -json`：两项原失败各三次通过，端点两种实际 reader 子例均通过；`diagnostics/timing-01/tests.jsonl`、`stderr.txt`、`result.json`。
- `./scripts/test-foundation.ps1 -Pattern '^TestSRStandalonePackage$' -OutDir C:/SteerlineDebug/wt-9ed5-runner172-qualification/diagnostics/standalone-01 -TimeoutMinutes 5`：真实 EXE/IPC/SQLite/Git 独立包回归通过，1 项、零 skip、exit_code=0；`integration.log`、`result.json`、`source-identity.json` 与 caller journal 保留。其后只加强解码前清零以防缺字段复用前值，由下述完整批次覆盖。

## 完整验证与交付

在源码固定后使用正式入口 `./scripts/test.ps1 -Level L1,Controlled`，实际命令在最终结果中登记。必须完整 L1、原生 driver/零匹配守卫、完整 Controlled 的直接证据通过、输入指纹不变及资源结算后，才取消 draft；定向通过不替代完整资格。

结果待本轮完整批次结束后填写。本页的原定向结果不会被改写为完整通过。
