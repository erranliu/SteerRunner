# Orca 停服核验

2026-10-19，普通开发，归属 [实施 Plan](../spec/orca-shutdown-verification.plan.md)。全部现场独立，不读取或复制安装配置、数据库或认证。

**当前结论以文末“固定控制与正向退出组合复验”及其链接证据为准。** 首版仅用 PTY 清单放行 Codex 的结论已被审计否定；下面旧构建的探针通过只证明终端通信与只读行为，不证明 Codex native writer-tree/authentication 已停止。旧证据保留，不作为当前合入资格。

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


## Codex 正向退出证明

此前只关闭 PTY 清单误放行；本轮新增并实际验证能够通过的原生证明。原 helper 的私钥只驻留内存，原 readiness 固定公钥和 helper/native 身份。app-server/tools 与 TUI 都在原 Job 内，实际查询完整树为空、关闭认证后才签发退出凭据；工作区的公共文件只有验签后才有证明力。离线入口仅走既有 Orca 文件读取和完整清单，不新建 verifier 终端、服务或模型，不写 Run/失败回执。

原 history reader 在原 Job 清空后返回结算事实，既有 inspection journal 在关闭 PTY 前保存。业务查询失败或 reader 创建失败也可返回已核实的结算；它不构成业务完成。快照与 journal 的全部原 reader 均须满足 native settlement 与原 PTY 停止。未知创建、缺原证据或不完整主机清单仍拒绝。

| 项 | 验证 | 本轮结果 |
| --- | --- | --- |
| OSV-A08 | `TestRemoteExit*`、`TestRemoteReader*`：签名篡改/重放/认证未关闭拒绝，真实 Win32 Job 结算；不可加载 EXE 的 reader 启动失败有正向 no-reader 证明 | 通过；最终新增错误分支定向 3.545s |
| OSV-A08 | `TestCodexStopped*`、`TestManagedRemoteReceipt*`、`TestStopArtifact*`、`TestOrcaLocalIPCReadsOnlyBoundStopArtifact` | 通过；含完整正向、活 PTY/reader 拒绝、journal-only reader 纳入、查询失败保留 native settlement、helper 返回原 shell 后仍可正确 stop、只读与原主机绑定 |
| OSV-A01/A02/A06/A07/A08 | `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout 120s` | 最终生产改动后通过：4.801s / 20.557s；原 P1 与通用 helper 回归保持 |
| OSV-A08 | Codex `TestRemote|TestHistory|TestConversationHistory|TestRecoveryHistory|TestProbePreservesHistory` 定向 | 通过，6.749s；没有运行全仓 |
| OSV-A03 | application/host/storage/cli 的 `TestSROffline|TestSRShutdown|TestSRStopProgress|TestSRCLIContract` 与 `TestSRServiceOwnership/offline_verification` | 通过：4.723s / 5.476s / 1.480s / 4.381s；真实 EXE integration 71.603s |
| OSV-A09 | 下列本机、远端新调试包真实 Codex/Orca stop + 发布 EXE offline verify | 通过：50.10s / 46.50s；两边 `resources_settled=true`，原 DB/失败回执不变、不启动服务，自己的资源清理回执成功 |
| OSV-A05 | 调用方部署门槛与诊断 | SteerLine app/runnerclient 定向与 `scripts/test-deployment-stop.ps1` 通过；详见其同名 acceptance |

最终验证的 Runner EXE SHA256：`b5ba7890fca94b158e18d4a50d8aad0b60cc12673fde5ad88475a75221b15150`；helper SHA256：`d01d2206014bda80d171f2165926851590c6546e763e99bc6e0b4af89add613d`。构建来自本原 worktree 的 `34e4ee7-dirty`；两个最终包使用同一 Runner/helper 字节，后续只补文档，不改变生产实现。

- 本机包：`C:/SteerlineDebug/dev-20261019-102911-35068b10`，证据 `evidence/codex-stop-final-local-recheck/`，base `da8da7beb462df7443cd6008e095875be27c07f7`，原 runtime `456f70a1-9c53-406c-a48a-72833d36abdb`。
- 远端包：`C:/SteerlineDebug/dev-20261019-103050-35068b10-dirty`，证据 `evidence/codex-stop-final-remote/`，base `cebdd5ab552150c49743e40593c836689ecf789b`，原 runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`，固定 environment `台式机64G`。public helper 通过原 Orca 通道暂存并核对 SHA，只放到远端新 `C:/SteerlineDebug/orca-native-stop-20261019-final/`；原 `C:/sr-runtime/2ba0/01/native/codex.exe` 只读使用（0.159.2）。暂存用自己的工作区和 4 个终端已关闭/移除，回执在 `dev-20261019-101836-35068b10/evidence/remote-stage-cleanup-*.json`。

每个最终证据目录含 `native-readiness.json`、`native-reader.json`、`native-exit.json`、`adapter-stop.json`、`active-rejected.json`、`stopped-verified.json`、原生创建/查询/关闭/移除回执和 `result.json`。两份签名正文均为 `writers_absent=true`、`authentication_closed=true`；reader 的 `reader_writers_absent=true`，最终 result 的 passed/real_codex/resources_cleaned 均 true。

Run 是隔离的当前格式契约夹具，但绑定实际执行主机的真实 helper/native/reader/PTY；只运行一次无工具 READY bootstrap，零业务输入/PR。公开重验前未把 Run 改为 stopped，所以成功确实消费本轮原生证明。普通被动 shutdown 随后使用同一证明通过。此探针不代表完整业务 L2/L3、SteerLine 全应用停机或用户原安装现场已解阻。

保留迭代失败：`dev-20261019-101143-35068b10/evidence/codex-stop-local-first/` 因探针未归一化原生路径而失败，`codex-stop-local-second/` 因夹具保存 refs/heads 前缀而失败；修正夹具后该包第三次真实 stop 通过（50.38s）。`dev-20261019-102153-35068b10/evidence/codex-stop-final-local/` 通过（69.51s），但随后补 reader 启动错误分支，故不是最终构建证明。最终本机包首个 `codex-stop-final-local/`（16.47s）与旧包同名工作区发生碰撞，生产正确拒绝旧 runtime；探针改为每次唯一 worktree 名，未删除或重置旧证据，最终新实例通过。所有失败仍保留，不计通过。

本轮复审范围：签名信任根/跨运行绑定、原 Job 统计和认证顺序、reader 的成功/失败/未知创建、显式 stop 与 reconcile、离线只读/原主机、ContinueExecutionOnServiceStop、消费方聚合围栏及设计规模。新增机制限定为原 helper 的签名公共退出文件与停止请求，复用既有 Job、文件通道和 journal；无新服务、通用调度/恢复状态机、迁移或双写。未发现本轮变更中的未解决 P1/P2，适合在已验范围合入；最终两 PR 仍保持 open，不合并/部署。

原安装环境未被读作测试夹具、修改或重置。旧 helper 从未产生或已经丢失的 owner/tree/authentication 事实不能由新版本追补；没有其原事实就不能把历史 Run 放行。本轮通过证明的是新能力在真实执行主机上成立，不能据此声称原报错的四条历史 Run 已经核实。

## 固定控制与正向退出组合复验

2026-10-19，主线 `2aa17cc` / 干净组合源码 `bb84ea8`。沿 OSV-A06/A08/A09 的原退出/只读边界，固定控制只负责同源查询及撤销；停止仍须原 helper 签名证明完整原生树/认证退出，之后再关闭业务 PTY，并核验原控制关闭及原主机完整清单。完整 L1/Controlled、三项 workflow 通过；测试修正 `51792ee` 后复用相同生产源码及原冻结 EXE，本机/远端无业务停止探针也通过；五份签名正文复验通过，公开离线核验不改 DB/原失败回执、不启动服务或业务。探针 helper 资格准备有真实 READY 模型调用，不能把“零业务输入”写成整场零模型。初始化创建未知仍阻塞。原始结果、范围与未执行的四项管理员变体见[组合验收](evidence/rebase-control-20261019/README.md)，不继承旧 PTY-only 的通过结论。
