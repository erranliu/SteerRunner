# Codex 初始化期间停止与资源结算

普通开发，2026-10-19。依据[停服契约](steerrunner.spec.md)及[统一 Worker 生命周期](https://github.com/erranliu/SteerLine/blob/main/docs/product-design/worker-lifecycle.md)的 W03/W05–W07/W18、E04–E05、V01/V06–V07；本次差异仅为初始化资源归属与停止取证。原生已启动而 readiness 尚未保存时，停止不能依赖业务就绪。沿用 Orca 原 runtime/worktree/terminal/incarnation、现有 Job、原 helper 签名退出文件和停止请求，不重建会话、不重发输入、不追补旧运行身份。服务退出仍只撤销输入并结束观察。

1. 原 helper 在准备原生执行前发布独立资源归属回执，绑定 helper 身份、构建、配置、工作区和退出公钥；Runner 在初始化轮询时保存该回执。
2. 停止请求从资源归属即可发出，覆盖准备、bootstrap、TUI 等阶段。原 helper 查询原 Job 清空、关闭认证后签名；未就绪退出也保存该证明。停止先于归属 checkpoint 时，先把原回执固定到既有 stop progress，再发停止请求；原生停止已确认后，暂时查询失败不得丢弃 WorkerAbsent。未就绪且无业务 turn/reader 派发时不要求尚不存在的 readiness journal，已就绪的原生 session/readiness 和全部 reader 继续严格校验。
3. 新增 L3 `orca-remote.initialization-stop`，两个 case 为 `before-readiness` 和 `restart-before-readiness`。真实公开 submit 创建 Run，测试侧转发器隐藏 readiness，保留原资源归属及实际 Orca 响应；不修改生产配置增加测试开关。
4. 同步机器注册、可读清单、结果校验及统一生命周期差异。共享 Orca CLI 缓存只经唯一临时文件校验并原子发布，两个并行 worker 不读取半写文件；新增并发冷缓存回归。现场只用新建调试包及其独立夹具。
5. helper 清理同时记录进程是否实际创建与句柄是否可用。带类型的 nil 句柄归一为空；已创建但句柄未知不得签发资源消失证明，停止和取消路径均执行同一清理。

## 测试计划

专属验收：[初始化停止](../acceptance/initialization-stop.md)。所有结果和失败保留在该文档，Plan 不维护实时结果。

| 验收项 | 级别 / 场景与命令 | 环境 / 时点 | 通过条件 / 证据 |
| --- | --- | --- | --- |
| IS-A01 | L1 codex `TestRemoteOwnership*`，orcatransport `TestInitialization*`、既有 Remote/Stopped 定向组 | 隔离签名/协议/Job 夹具；实现后 | 早期归属可停止；错身份/签名/认证未关闭仍拒绝，零业务输入；独立日志及本验收 |
| IS-A02 | L3 `orca-remote.initialization-stop/before-readiness`，`scripts/test.ps1 -Level L3 -Scenario orca-remote.initialization-stop -Config <独立配置> -FixtureParent <新包>/cases -OutDir <新包>/evidence/l3` | 先运行 SteerLine scripts/new-debug-release.ps1 新建 C:/SteerlineDebug 包，使用固定新 Runner/helper、真实远端 SteerlineTest；最终实现后 | 原 helper/native 已启动，Run 无 readiness/业务输入，显式停止后原 Job/认证/PTy 结算，service verify-stopped 通过；case evidence 与本验收 |
| IS-A03 | 同入口 `restart-before-readiness` | 独立 Run/state-dir/worktree；最终实现后 | 初始化中停服换代，零自动输入/替换会话，显式停止仍能结算；case evidence 与本验收 |
| IS-A04 | L1 livetest registry/evidence，PowerShell framework（含 `tests/framework/runtime-cache-test.ps1`）；既有停服/公开 CLI 定向 Controlled | 各自隔离夹具；注册与最终构建后 | 两 case 可选、证据缺失拒绝，原停止/恢复门槛不回退；新包日志与本验收 |

不运行完整 L2/全 L3/L4：本次零业务输入的初始化资源流程没有发布/交付行为，精确故障反例归 L1/Controlled。人工项无；安装环境和历史 Run 的清退不在本次范围。

仅 helper 的启动失败保护增量采用以下定向验证安排：运行 `go test ./internal/execution/adapters/codex -run '^TestRemoteOwnership' -count=1 -timeout 60s -json`，覆盖未创建、带类型 nil 句柄与未知创建；固定此前正式批次已验证的同一 Runner EXE 摘要，仅替换 helper。再次通过 `scripts/new-debug-release.ps1` 生成新包，使用既有 `Get-SRLiveCases` / `Invoke-SRCaseBatch` 协调器并行执行两个注册 case，保留配置、租约、来源身份和证据校验。要求全部断言及资源收尾通过；脚本、逐 case 原始证据和汇总登记到本验收。此安排不提供跳过前置的通用生产开关，不计为重跑完整 L1/Controlled 或完整 L3。
