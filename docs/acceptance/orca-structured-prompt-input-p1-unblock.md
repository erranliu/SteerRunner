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

**首轮发布门禁。** 当时 Runner 的远端原会话范围停止、v1 封存与独立原目录排空已有正向证据，本机正式检测及其原目录升级预检通过。远端原正式命令待核实，独立脚本对照未取得有效业务报告。

## 远端正式检测前置只读复核

- 原 `environment_start` 请求 `1b4d254336c04d959637614f8132ca60` 的持久信封与保存的原命令一致。SteerLine 命令行记录仍为 `pending`，无结果字节，公开 `outcome_unknown` 为查询投影。原项目库无该请求的活动映射、无对应活动或远端环境当前记录。Runner 原目录共有两个本机 Run，无远端 Run，也无绑定该活动的 Run。证据将阻断定位到命令登记后、正式活动持久化之前，未确定导致超时的具体调用。保留原记录，不将其改判为成功或拒绝。
- 在同一配对 runtime 和独立脚本对照的原测试工作区，用一次性 Orca shell 运行 `powershell.exe -NoProfile -NonInteractive`，只查询账户、版本及有效策略。远端账户为 `Administrator`，Windows PowerShell `5.1.26100.9711`，有效策略为 `Restricted`。MachinePolicy、UserPolicy、Process、CurrentUser、LocalMachine 五项均为 `Undefined`。这与原工具退出码 1 和 `UnauthorizedAccess` 一致。未执行原脚本或改动策略，探针终端已关闭。
- 原候选 EXE 的 SHA256 匹配固定 pin。该次复查时脚本执行前置仍阻断，未提交新的正式检测或补发原命令。只读核验摘要为 `.evidence/prompt-input-p1-unblock/p1-remote-readonly-followup.json`。

## RemoteSigned 生效后的远端正式复验

2026-11-04。用户完成主机策略设置后，在同一配对 runtime 和原远端测试工作区复查。远端 `Administrator` 账户的 Windows PowerShell 5.1 新进程有效策略为 `RemoteSigned`，`CurrentUser=RemoteSigned`，其余四项为 `Undefined`。只读探针终端已关闭。

| 验收项 | 核验结果 |
| --- | --- |
| 同一候选与原目录 | 正式 SteerLine 启动仍绑定 `fd17f5b6` 的受测 EXE SHA256 `4101234165411523daf15bf5bf01118d33eb76a31a5e516f43eda23c139d2f38`，使用原应用及 Runner 目录。 |
| 单次正式检测 | 新活动 `e6b500aa6d2448c8bb27c79a0350638a`，原远端 Run `b4149a6591c24f204cfc52b4a7ebb0b3`，使用 `orca_agent_session_v1` 和冻结 profile v2。公开原历史有一条 user 消息，原检测脚本调用一次，退出码 0。PowerShell、Git、Python 三项通过，报告 completed/ready，SteerLine 状态 ready。 |
| 原会话结算与报告 | 原 session `oas_e51b674c3832ba2b19818cda05ca9ae5`、turn `01a1a57f-0ddc-7a83-bfa0-a99f35949839` 的完整 final 已核实且 native outcome 为 succeeded。唯一原停止意图保留成功 close 回执，`stop_status=stopped`、scope 为 `orca_agent_session`。原 `result-file/v1` 已关闭，版本 1，1,858 字节，SHA256 `1701323adea199d4171b483fde104f627bc4806269d78aa73384ff2e6e09e896`，与解码报告一致。 |
| 正式升级排空 | SteerLine 正式停服通过，原 Runner writer 已退出。原状态目录 `C:/slr/5btxl5yowklmheilyd7lbzg3tq` 的三条原 Run 均通过离线核验。正式 `verify-runner-upgrade` 使用原 EXE 和不同 SHA 冷读取器通过。两端冷查的远端 Run、输入摘要、停止状态相同，四项操作均 confirmed，输入授权关闭。 |
| 历史请求保留 | 原 `1b4d254336c04d959637614f8132ca60` 查询仍为 `outcome_unknown`，其持久行仍为 pending、无结果和活动映射。原目录仅有本次新增的一条远端 Run，未补发原请求。 |

本轮关闭 P1 的远端正式检测与原目录双 EXE 排空门禁。双端正式检测和各自的既有验收范围成立，未取得完整 L1/L3 或生产发布资格。原始回执、冷查和公开工具项见 `.evidence/prompt-input-p1-unblock/remote-signed-*`。P2 等待 P1 合入。

## P1 PR 审计与按影响复验

公开 `result_file_schema` 和 `model_input_contract` 原先仍把历史快照写成新远端 Run 的合同。两处发布说明已明确历史 profile 与新 Run 的边界。固定构建的 `capabilities --offline` 对比确认，schema 约束、结果文件能力和实际提示词均一致，仅两处说明文本改变。

审计版源码摘要 `6e4fdeb653a862a8a7ac061827fce24e41a8f92a7a738ec38d9b0b12fe07ee38`，EXE SHA256 `b7f1c5b9ee5bb89ab22b55f8a68630dcbad78aa86e0ad0ed940207897e4e496e`。使用它作为第二个 EXE，再次通过正式 SteerLine `verify-runner-upgrade`，原应用和 Runner 目录、原固定 EXE 均相同。另逐字段比对两条正式 ready Run 和两条历史远端 Run 的冷读结果。两条历史远端 Run 仍为 unknown，离线 `resources_settled=false`。SteerLine 的环境结果、ready 条件及升级排空门禁定向测试通过。双端正式业务检测的证据对应原固定 EXE。审计版只做能力对比、冷查询和升级预检，没有发起新检测。

Runner 定向复验覆盖 profile 冻结、原停止回执、禁止补发、usage 绑定、v1 封存与历史快照。完整 `go test ./...` 在本机的 application Git fixture 和 integration 结果获取用例达到默认 10 分钟包超时。已有本地文件故障用例的准备阶段曾间歇返回 `read_failed`，独立重跑已通过，主线同用例也通过。完整回归未签收。
