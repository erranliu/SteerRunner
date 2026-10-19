# Orca 停服核验

2026-10-19，普通开发，归属 [实施 Plan](../spec/orca-shutdown-verification.plan.md)。全部现场独立，不读取或复制安装配置、数据库或认证。

**当前结论以文末“P1 审计修复”段为准。** 首版仅用 PTY 清单放行 Codex 的结论已被审计否定；下面旧构建的探针通过只证明终端通信与只读行为，不证明 Codex native writer-tree/authentication 已停止。旧证据保留，不作为当前合入资格。

## 首版记录（历史）

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

## P1 审计修复

本轮关闭新核验路径的错误放行，不新增回执格式、远端进程查询、迁移或恢复能力。Codex 当前缺少原 owner/tree/authentication 绑定的离线退出证明，即使原 PTY 或记录 root 已消失也保持未核实；包括缺 readiness 和远端。历史已持久化 stopped 保持既有读取语义。通用 helper 仅复用既有原 owner/exit/authentication 证明，混入 Codex/readers 证据拒绝。

清单校验收拢为 `originalTerminalInventory`，offline VerifyTerminalStopped 与 CloseReadonlyTerminal 共享原 host/计数/截断/唯一身份规则，各自保留停止和关闭动作判断。删除了不再能用于 Codex 成功证明的离线 reader 分支；未新增缓存、调度器或通用框架。

| 项 | 当前通过条件 | 结果 |
| --- | --- | --- |
| OSV-A02 | 既有真实原生子进程/Job helper 夹具：原绑定退出证明通过，认证清理 false 拒绝；不冒充真实 provider 运行 | `TestManagedStopWaitsForOriginalExitAndAuthenticationCleanup` 通过 |
| OSV-A06 | 原 P1：真实存活 OS root + 完整空 PTY 清单必须拒绝；缺 root/缺 readiness/远端同样拒绝，快照不变 | `TestCodexStoppedRequiresNativeExitProofBeyondPTY` 四子例通过 |
| OSV-A07 | 两消费者共用原 host 完整清单；原 reader 缺席可以结算，但省略原 host/身份变化/清单不完整均拒绝，零重复输入 | 协议表格与原 `restart-absent-close` 回归通过 |
| OSV-A03 | 既有 managed/锁/只读/停止规则保持 | application/host/storage/cli 定向回归及真实 EXE offline_verification 通过（32.325s） |
| OSV-A04 | 新 debug 包本机/远端真实 PTY：关闭后记录的 live root 缺原生退出证明仍拒绝，DB/原失败回执不变、不启动服务、清理自己的资源 | 本机 12.10s、台式机64G 13.65s 通过；两个 result.json 的 passed/resources_cleaned 均 true |

本轮 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout 120s` 通过（4.368s / 18.690s）。共用校验最初揭示旧夹具 hostIds 使用占位值 execution-host；已改为真实路由契约的 local 后，既有 recovery reader 反例和全两包回归通过，未放宽生产 scope 校验。

新包由生成脚本创建：`C:/SteerlineDebug/dev-20261019-094824-2b16b5df`，Runner 来源 `367fc8519ddab001be0fee3974762dc88cc7d602-dirty`，固定 SHA256 `44087a088919f83dfec95c0849ae268f5b80608667440617a295b34a7e4e89d9`。本机 clone/base 为本包 `SteerlineTest` / `ac246e4c7cf04768c47adde7f1d26858ea496da0`，远端独立 SteerlineTest worktree/base 为 `cebdd5ab552150c49743e40593c836689ecf789b`。证据分别在 `evidence/orca-stop-p1-local/`、`evidence/orca-stop-p1-remote/`，期望 `active-rejected.json` 与 `closed-pty-native-unverified.json` 都为资源未结算；`result.json` 的 passed 表示成功验证拒绝边界，不能解读为 Codex 已结算。root 使用真实存活调用方 OS 进程作隔离记录夹具；远端探针不以它冒充执行主机原生 root 的观测。
