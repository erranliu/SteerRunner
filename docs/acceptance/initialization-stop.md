# Orca 原生初始化期间停止验收

归属：[实施 Plan](../spec/initialization-stop.plan.md)。2026-10-19，普通开发。只使用新建独立调试包和隔离夹具，不操作安装环境，不修写历史 Run。

| ID | 自动验收内容 | 状态 | 证据 |
| --- | --- | --- | --- |
| IS-A01 | 原生初始化/换代停止、原身份与 scope 反例、local 回归 | passed | [组件 126 项](evidence/initialization-stop-native/components-serial.jsonl)；初始化空 branch 正例与已开始/错身份反例 |
| IS-A02 | before-readiness：原生已启动，caller 未确认时显式停止并离线重验 | passed | [原生 case](evidence/initialization-stop-native/before-readiness/scenario.json)、[离线结算](evidence/initialization-stop-native/before-readiness/offline-settlement.json) |
| IS-A03 | restart-before-readiness：初始化中服务换代，零自动输入/重建 | passed | [原生 case](evidence/initialization-stop-native/restart-before-readiness/scenario.json)、[换代原身份](evidence/initialization-stop-native/restart-before-readiness/initialization-restart.json) |
| IS-A04 | 注册/审计、并发缓存、停止/离线 scope/恢复前置 | passed | [framework 17 项](evidence/initialization-stop-native/framework/result.json)、[Controlled 3 项](evidence/initialization-stop-native/controlled/result.json)、[两个独立审计通过](evidence/initialization-stop-native/batch-summary.json) |

无人工验收项。当前记录不表示完整 L3 或安装现场已通过。

## 原生主干 rebase（当前）

Runner 最新主干 `0b55928` 已合入 #31（`3a053ea`），远端不再使用 helper 或固定控制终端。rebase 处理 10 处文本冲突后，还需移除旧方案的语义冲突：当前生产差异仅为 native stop 的初始化空分支元数据处理；两个 L3 case 改为隐藏 caller 原生 agentIdentity 并保留真实响应。local helper 代码保持主干原样，旧 helper/Job/认证/签名的通过记录不继承为 native 资格。

当前 native scope 只证明原 Orca 终端结束，不证明脱离终端的后台进程或执行主机登录撤销。旧 394/29/16 与两个 helper L3 的结果不作为新组合通过证明。

2026-10-19 15:48–15:52（Asia/Shanghai），经 `scripts/new-debug-release.ps1` 创建的 `C:/SteerlineDebug/dev-20261019-154407-be8f3b54` 完成原生增量验证。Runner 构建 clean 源为 `60c805861c0ae26626716b21d71b5f68aa9d4eef`，EXE SHA256 `777b69f0db1887edf079138e8d1c561fd5876eca74db5ee528e3e1d5a9d3858f`；测试 clean 源为 `735d8a293808f9ab2dc62e54ae579e4251007868`，两者 Go 源完全相同，后者只修复 framework PowerShell 文件的 UTF-8 BOM。SteerLine 包来源 `be8f3b54`，已 rebase 到 `4c5409f3`。配置冻结 workflow/2、profile v4 原生执行，无远端 helper 配置、上传或控制终端。

最小验证通过：三个相关组件 126 项、framework 17 项（含冷缓存并发）、live driver 编译、Controlled 3 项 `TestSRServiceOwnership` / `TestSRNativeUnknownAndStop` / `TestSRRestartRequiresResume`。Controlled 与两个 L3 使用同一 Runner EXE。[增量脚本](evidence/initialization-stop-native/run-native-initialization-l3.ps1)核对本轮前置、源文件/构建回执/配置/registry 摘要后，经现有协调器、隔离租约、worker 与独立核验器以 MaxParallel=2 执行；未新增通用跳过开关。

| 原生 case | 独立夹具 / Run | 结果 |
| --- | --- | --- |
| before-readiness | `ab7810b9fc43` / `8ad7520b6c192a9db3d2771c8ad9d7eb` | passed；真实原 Codex 已启动而 caller 未确认，显式停止及公开离线终端 scope 通过 |
| restart-before-readiness | `3ee15abdd90f` / `45bbbd02b43e3a00c2e21901afd76d71` | passed；服务换代保留原身份、零自动输入/替换，显式停止及同样离线核验通过 |

两者执行时间重叠，均核对一次精确直接原生启动发送、一个原工作区、零业务输入、零额外终端/文件部署。`agent-identity-withheld.json` 保留原始及仅隐藏 agentIdentity 的 caller 响应，并关联完整原始调用审计；初始化快照无 helper/控制状态。换代原资源快照保持，输入授权关闭。各 cleanup 均为 native_stopped/service_stopped/facilities_restored=true、driver_native_cleanup=false；离线 resources_settled=true，stop_evidence_scope=orca_terminal。

[本轮证据摘要](evidence/initialization-stop-native/summary.json)、[逐文件摘要](evidence/initialization-stop-native/hashes.json)及原始报告已归档。首轮并行组件测试曾因共享 CLI 排队耗尽 3 秒预算失败，未改 Go 源的串行重跑通过；framework 首次因编辑时误解码 BOM 在执行 case 前失败，恢复主干编码后 17 项通过。失败记录保留在[前置记录](evidence/initialization-stop-native/preflight-failures.json)，未计通过。另一个 `dev-20261019-154251-be8f3b54` 包未启动执行。

当前注册为 L3 21 场景/29 case，保留主干已退役的两个远端 Check case 的移除。本轮只验证所选两个 case 与最小前置，不表示完整 L1/Controlled/L3/L4 通过；未操作安装/保留运行环境，未部署、未改写历史 Run。归档中的 helper_sha256 是本机测试故障转发器身份，不是远端 steerrunner-agent。

## 历史 helper 方案最终验收（已退役）

2026-10-19，代码提交 `cf7b47a5eca1968de2e13997c3752f76a6caa0e6`，已合入主干 `95ae8a72792d2b11cc59b938257165b472b5b4de`，源码工作区在构建/执行时为 clean。五处冲突均已解决：保留主干固定控制终端、认证参数及观察关闭行为；初始化无已提交 readiness/业务 turn 时，仅按原 helper 签名的 Job/认证清理与原固定控制 PTY 的关闭结算。已就绪路径保留 authentication-stop 和关闭 checkpoint，未恢复旧 reader journal。证据 DTO 解除对适配器的依赖，消除新主干测试导入循环。

独立环境由 `scripts/new-debug-release.ps1` 生成：`C:/SteerlineDebug/dev-20261019-130825-dc5dc4a5-dirty`。该名称的 dirty 指 SteerLine 文档工作区，Runner 的构建与测试源为上述 clean 提交。Runner EXE SHA256 为 `18ebe2345c177f9a8b621694ec2e70ed1faa2864879b8c1e88a543a071935219`，远端 helper SHA256 为 `52a2261351290f49366a331119d2f72afa36b60d7ef7aff12063206053ae573a`，路径为 `C:/SteerlineDebug/initialization-stop-main-20261019-1308/steerrunner-agent.exe`；冻结配置使用 workflow/2。

公开 L3 入口的新构建前置全部通过：Go L1 394 项、Node 扩展 29 项、framework、live 驱动编译及零匹配保护、Controlled 16/16。Controlled EXE 与所选 Runner 摘要完全一致。首次两个 case 准备因漏复制 `steerrunner.exe.build.json` 回执失败，均未创建 manifest/Run；原报告 `evidence/l3-initialization-stop/summary.json` 保持 failed，局部夹具 `5e7aa5d4a5fa` / `d3c7a2e86e2c` 保留。不能把该批次计为 L3 通过。

补入原始同构建回执后，执行 `evidence/run-registered-l3-retry.ps1`：核对本次已通过的前置、clean 源提交、Runner/Controlled EXE、原构建回执、配置源与冻结字节、helper 和注册清单摘要，确认两次失败均为创建 Run 前的同一回执缺失，再经既有注册协调器/隔离租约/worker 和独立证据校验器运行两个新夹具。源码、EXE、配置、helper 和执行方案均未变化，不重复已通过的相同构建前置；该批次为 `evidence/l3-initialization-stop-retry-02/summary.json`，2/2 passed，实际执行时间重叠。未新增通用跳过前置开关。

| 最终 case | 新独立夹具 | 结果 |
| --- | --- | --- |
| before-readiness | `cases/dddbaf198def` | passed；真实 native readiness 存在，Runner 只有原 ownership 与固定控制绑定；显式停止、签名清理、控制 PTY 关闭及公开离线结算通过 |
| restart-before-readiness | `cases/42799287026e` | passed；服务换代保留原资源与控制身份，零自动输入/替换会话，显式停止与同样结算通过 |

两者均只有一次与原 launch_digest 一致的 helper 启动发送，零业务输入、零控制请求；完整原终端清单与独立审计确认一个原控制终端已关闭，签名证明 writers_absent/authentication_closed=true，公开离线 resources_settled=true。各 cleanup 的 native_stopped/service_stopped/facilities_restored=true，driver_native_cleanup=false。helper 暂存工作区与两个 PTY 已关闭移除并保留原始回执。

可携带的[本轮证据](evidence/initialization-stop-main/summary.json)包含原正式批次失败/前置报告、重试报告/脚本、29 个最终源码文件摘要、逐 case 原始控制审计、签名退出、离线证明及换代快照；[文件摘要](evidence/initialization-stop-main/hashes.json)固定归档字节，签名文件不重新格式化。完整原现场仍在上述包。L3 清单仍为 23 场景/31 case，本轮仅验证所选 2 case，不表示完整 L3。未操作安装/保留运行环境，未部署，未修写历史 Run。

## 原主干验收结果（集成前）

2026-10-19，Runner 分支 `codex/initialization-stop`，测试基于 `2aa17cce7da6fd75e59db76e18f2521d9e9777db` 的工作区实现（现实现提交 `a98adb5`）。可携带的[证据摘要](evidence/initialization-stop/summary.json)固定原始报告摘要、Run、构建摘要和收尾结果；完整现场保留在以下独立调试包。

正式批次为 `C:/SteerlineDebug/dev-20261019-123304-d1277973-dirty/evidence/l3-initialization-stop/summary.json`，通过公开 `scripts/test.ps1 -Level L3 -Scenario orca-remote.initialization-stop -MaxParallel 2` 入口执行：L1 Go 324 项、Node 扩展 28 项、framework（含双 worker 冷缓存）及 Controlled 16/16 通过；所选两个 L3 case 均通过。Runner EXE SHA256 为 `bcf9cc4cd53303302804a08739e43ff224027e92bc464049272f9b4eeb062545`，该批次 helper SHA256 为 `fb087fe8611b5e1c970eafe25b1a39b0fcd80f8e70047cb9945f8d80965523b1`。

最终自查增加了仅 helper 的启动失败保护：启动返回带类型的 nil handle 不得触发清理 panic；已发生未知子进程创建又没有句柄时不得签发“写者已消失”。`C:/SteerlineDebug/dev-20261019-125200-d1277973-dirty/evidence/helper-guard-unit.jsonl` 中 `TestRemoteOwnership*` 四项通过。随后在该新包使用同一已合格 Runner EXE、最新 helper SHA256 `5a195821932a14784b4d0fb2c30fcc07ca5d25d773702bef3afb90996b45b3c4`，经既有注册协调器、隔离租约及证据校验器并行重跑两个真实 case；执行脚本保留为 `evidence/run-helper-guard-recheck.ps1`，结果为 `evidence/helper-guard-recheck/summary.json`，状态 passed。这是 helper 增量定向复验，不新增一次完整 L1/Controlled 通过声明。

| 最终 case | 独立夹具（相对最终调试包） | 结果与证据 |
| --- | --- | --- |
| before-readiness | `cases/dcfa1e1bebab/evidence/` | passed；`initialization-window.json`、`readiness-withheld.json` 证明真实原 native 已就绪、持久化尚无 readiness；`initialization-exit.json` 与 `offline-settlement.json` 证明原资源结算 |
| restart-before-readiness | `cases/91b3bccfc13b/evidence/` | passed；上述相同证明及 `initialization-restart.json` 证明服务换代保留原身份、零自动输入与替换会话 |

两个 case 只有一次可核对正文摘要的 helper 启动发送，业务发送为零；签名退出均验证 writers_absent/authentication_closed，公开离线重验 resources_settled=true。各 `cleanup.json` 均记录 native_stopped/service_stopped/facilities_restored=true；helper 暂存工作区及 PTY 已关闭移除，收尾回执保存在最终包 `evidence/guard-stage-*.json`。`evidence/source-snapshot.json` 固定最终 24 个代码/脚本/注册文件摘要。

测试清单现为 L3 23 个场景、31 个 case；本次选择其中 2 个，完整 L3 未执行。所有真实执行均限于新调试包及 SteerlineTest 专属夹具。未操作安装/保留运行环境，未修写历史 Run，未部署；原安装失败 Run 仍需保留其证据缺失状态。

## 原草稿 PR 集成边界（已解决）

最初提 PR 时最新主干为 `95ae8a7`，已合入另一批固定控制终端/认证撤销与 workflow 改动。当时本分支尚未集成该主干；只读合并预检显示 `docs/test-catalog.md`、Codex `remote_agent.go`、Orca `lifecycle.go` / `stop_test.go` / `verify_stopped.go` 五处冲突，因此提交草稿。该边界现已由 `cf7b47a` 及上文新构建验收解决；下文与“原主干验收结果”继续保留原事实，不将旧报告冒充新组合证明。

## 已保留的首轮记录

- `C:/SteerlineDebug/dev-20261019-120959-d1277973/evidence/l3-initialization-stop/summary.json`：缺少显式注册项目路径，两个 case unavailable、零执行；补齐配置后使用新批次，不计通过。
- 同包 `evidence/l3-initialization-stop-02/`：L1 324 项、Node 扩展 28 项、Controlled 16/16 通过；新 L3 首轮失败，不能据此前置通过声称 L3 已通过。
- `restart-before-readiness` 在创建 Run 前因共享冷缓存的并发复制/读取文件占用失败，现场 `cases/373ebd16d384` 保留。改为校验临时副本后原子发布，新增双 worker 冷缓存回归已通过。
- `before-readiness` 现场 `cases/c6c4efc3d746`，Run `1f8c3ee1872ea1ca468db82ce27c170e`：已实际取得原 native readiness，Runner 持久化只有早期 ownership；驱动误把一次 helper 启动 send 计为业务输入而失败。已改为严格核对唯一 helper 启动正文摘要、原配置/工作区/公钥和零业务 send，服务换代期间保持该非零输入基线。
- 收尾进一步发现：初始化没有 readiness 时，离线重验错误地要求 reader journal；停止后查询失败又会丢掉既有 WorkerAbsent。两处均已修复并补完整 verifyStopped / 重复 reconcile 回归。原失败报告保持；最终核验器只读重验本次失败夹具，`cases/c6c4efc3d746/evidence/final-verifier-resource-recheck.json` 的 resources_settled=true，原测试仍记失败。
- 首轮暂存工作区/两个 PTY 已关闭移除。后续构建用另一个新调试包；不复用失败 Run，不修改其数据库/失败回执。
