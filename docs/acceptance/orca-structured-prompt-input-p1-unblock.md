# P1 远端会话停止、v1 封存及排空复验

2026-11-04。候选基于 `8d8e8647`，源码摘要 `6132d57859e947ac5386beba6fae0a04049b3350957f4b635811ad7a8c5f3d64`。双端全部业务运行使用同一 EXE，SHA256 `4101234165411523daf15bf5bf01118d33eb76a31a5e516f43eda23c139d2f38`。仅用于升级冷查询的第二 EXE 来自同一源码，版本标识不同，SHA256 `e03463838d0a5a6743cd84f562ea01d85702ae28ed0d42147241915486b1b83a`。SteerLine 固定 `a589f618`，未修改 SteerLine 或 Orca。原始证据保存在调用 worktree 的 `.evidence/prompt-input-p1-unblock/`。

## Runner 实现与受控验证

新受理的远端 Codex execute/1 冻结为 Orca 会话 profile v2，复用 #97 的原会话关闭回执、同事务停止状态与原作用域离线核验。结果文件使用原停止后 v1 封存。此前的远端结构化 profile v1 继续采用 unknown 与原快照，保留已取得结果。使用候选对旧远端 CLI 状态冷查，两条旧 Run 仍为 unknown，资源证明仍为 false。会话范围继续不声明 `WritersAbsent` 或主机级清理。

application、Orca adapter、Orca protocol、resources 和 contracts 定向回归通过。受控验证覆盖原关闭确认与旧远端不追认、同一回执不重放、无有效回执不确认、#98 outcome 及 v1 封存与历史快照边界。未取得完整 L1/L3 资格。

## 同一候选的双端 Runner 专项

本机与配对端各执行一场多行正常输入和一场命中活动回合的显式停止。四场均得到 `stop_status=stopped` 和 `steerrunner-result-file/v1`，正常场取 9 字节业务结果，活动停止场保留原 4 字节 seed。每端同构建重启核对原 session、停止状态、文件版本及 SHA256。停服后 writer 不在运行，两端各自原目录的两条 Run 均以 `orca_agent_session` 通过 `service verify-stopped`。临时 Go overlay 仅增加只读测试入口，实际调用未改动的 SteerLine `VerifyUpgradeDrained`，用两个不同 SHA 的 Runner 对两个实际原目录分别冷查通过。

## 正式 SteerLine 检测与升级

| 场景 | 结果 |
| --- | --- |
| 本机启动与正式检测 | 只读 canary 证实 Windows breakaway 后的外层 Job 没有 kill-on-close。通过该启动上下文使用正式 `steerline start` 和 HTTP `environment_start`。首场受理后，验收客户端因远端 POST 超时提前停服，原本机 Run `35eec1ffabf5afbcf4fe4af3afb3df3c` 尚未结算。原目录同构建恢复后核实取消并停止，保留失败封存事实。新本机场 `89d7c68d5d91438093f7d953dfe9ccc2` 的原脚本报告 completed/ready，已封存原 v1 结果，应用状态 ready，原 Run stopped。 |
| 远端正式检测 | 正式 `environment_start` 请求 `1b4d254336c04d959637614f8132ca60` 的客户端响应等待超时。原目录恢复后只查询原命令，持久结果仍为 outcome_unknown，未查到原活动，未补发。远端正式检测未通过。 |
| 远端脚本独立对照 | 同一候选在单独 Runner CLI Run `70eef2d31e1886229b5b2227246fcae6` 调用相同原检测脚本。原会话只有一个 user 项，公开原工具项证实 Windows PowerShell 以 UnauthorizedAccess 拒绝载入，退出 1。候选仍取得原会话 stopped、v1 封存和正向离线资源证明，封存字节等于初始报告，ready 为 false。该结果不作为正式 SteerLine 检测成功。 |
| 正式原目录升级预检 | 首次提前停服后，SteerLine 正确拒绝未排空目录。原 Run 恢复并结算后，`verify-runner-upgrade` 通过。本机新场 ready 后再次正式停服，同一原目录 `C:/slr/5btxl5yowklmheilyd7lbzg3tq`、原 EXE 和不同 SHA 冷读取器的正式升级预检再次通过。该目录没有已绑定的远端正式检测 Run，不能把此项算作远端正式环境升级。 |

**P1 仍不发布。** Runner 的远端原会话范围停止、v1 封存与独立原目录排空已有正向证据。本机正式检测及其原目录升级预检通过。远端正式命令仍待核实，独立脚本对照也未取得有效业务报告。没有降低 ready 或升级门禁，也没有为脚本注入执行策略绕过。P2 继续等待 P1 完成正式验收。
