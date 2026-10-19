# Orca 停服核验

2026-10-19，普通开发，归属 [实施 Plan](../spec/orca-shutdown-verification.plan.md)。全部现场独立，不读取或复制安装配置、数据库或认证。

| 项 | 验证 | 结果 |
| --- | --- | --- |
| OSV-A01 | 原 runtime 完整 inventory、身份与否定边界 | 通过 |
| OSV-A02 | 原 reader/journal/helper exit 和停服事实 | 通过；含关闭记录仍缺 incarnation 拒绝，generic 缺 exit 拒绝 |
| OSV-A03 | 既有 managed/锁/只读/最终结算定向回归 | 通过 |
| OSV-A04 | 独立 debug 包真实 Orca + Runner EXE 无模型停止重验 | 最终包本机重验 11.95s、远端 14.19s 通过，资源清理通过；首次本机时序失败保留如下 |
| OSV-A05 | 调用方诊断及部署失败门槛 | 通过 |

不以单元夹具通过冒充原生或用户安装现场通过；后者保持未操作。

## 命令和构建

- `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout 120s`：相关两包全部通过（4.745s / 19.459s），原生 opt-in 未设置时不运行。
- 最后补严 reader incarnation 后：`go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport ./internal/application ./internal/host ./internal/storage ./internal/cli -run 'TestTerminalStopped|TestTerminalOffline|TestManagedRemoteReceipt|TestSRShutdown|TestSROffline|TestSRStopProgress|TestSRCLIContract' -count=1 -timeout 120s`：六包匹配测试通过。
- `go test ./tests/integration -run '^TestSRServiceOwnership/offline_verification$' -count=1 -timeout 150s`：通过，30.603s；真实 EXE 的 managed 原 owner/root 消失、写锁、零新输入和 DB 不变保持。
- SteerLine：`go test ./internal/app ./internal/runnerclient ./cmd/steerline -run 'TestDeployment|TestServiceSettlement|TestStoppedVerification' -count=1 -timeout 120s`：app / runnerclient 通过（28.616s / 3.306s），cmd 仅编译无匹配；`scripts/test-deployment-stop.ps1` 通过。
- PowerShell `scripts/test.ps1` 语法解析及两仓 `git diff --check` 通过。未执行全仓、完整模型/PR 流程或安装部署。

首包 `C:/SteerlineDebug/dev-20261019-092629-abd001f1` 原生本机通过（11.84s）；第二包 `C:/SteerlineDebug/dev-20261019-093128-abd001f1-dirty` 本机/台式机64G 分别通过（11.60s / 13.85s），原资源已清理。随后自查补严了缺 reader incarnation 的拒绝规则，不复用旧包作为最终构建证明。

最终包由 SteerLine `scripts/new-debug-release.ps1` 新建：`C:/SteerlineDebug/dev-20261019-093426-abd001f1-dirty`。Runner 来源 `59c7597ac8056fc5c29ad622034e10e74e4395de-dirty`，固定 EXE SHA256 `185f8367a1becc4aa8cdd1cfbaf82c170d4b9cb3d5c46ecf0026dea2449eed2e`。各 probe 独立 state-dir、Run/profile/worktree/PTY，固定本机与 environment 路由，未切换同一实例执行方案。

本机固定本包注册 clone，base `4ee6e6e8d5c56b1275aef6659aa087dc0f3d2684`；远端固定独立 SteerlineTest worktree，base `cebdd5ab552150c49743e40593c836689ecf789b`，没有修改原项目工作区。最终通过证据在本包 `evidence/orca-stop-local-recheck/`、`evidence/orca-stop-remote/`：`active-rejected.json` / `stopped-verified.json` 保存公开 CLI 结论，`orca-*.json` 保存创建/查询/精确关闭/清理事实，`result.json` 保存原 runtime/worktree、零模型输入及清理结论。Run 为绑定真实 PTY 的当前格式契约夹具，不冒充完整业务任务/真实 helper 执行通过。

本机首次最终包探针（`evidence/orca-stop-local/`）失败：Orca 已返回 `ptyKilled=true`，但随后清单仍短暂显示原 terminal 为 running，Runner 正确保留未核实。仅修正探针驱动，在原关闭回执之后最多等待 10 秒原 runtime 的完整清单确认原身份已停，不重发 close/输入；生产校验和最终 EXE 未改变。新独立 probe `orca-stop-local-recheck` 随后通过，保留首次失败文件；两次的自有资源均已清理。实际部署若恰在此窗口取证仍可失败，应稍后按原身份重新读取，不能以接受回执代替证明。
