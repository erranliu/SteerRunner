# Codex 初始化期间停止验收

归属：[实施 Plan](../spec/initialization-stop.plan.md)。2026-10-19，普通开发。只使用新建独立调试包和隔离夹具，不操作安装环境，不修写历史 Run。

| ID | 自动验收内容 | 状态 | 证据 |
| --- | --- | --- | --- |
| IS-A01 | 早期资源归属、准备取消、原签名/Job/认证清理及身份反例 | 通过 | 正式批次 L1；最终 helper 增量 4 项定向测试，见下文 |
| IS-A02 | before-readiness：真实原生已启动、readiness 未保存时显式停止并离线重验 | 通过 | 最终 Run `56b560dfd8dc639894f717b0b074f47b`，签名退出及公开离线结算通过 |
| IS-A03 | restart-before-readiness：初始化中服务换代，零自动输入/重建，显式停止并重验 | 通过 | 最终 Run `f360fc632ba10737d8c49c44750804f2`，原绑定及换代证据通过 |
| IS-A04 | 注册/证据校验、并发不可变缓存、既有停服与公开 CLI 回归 | 通过 | 正式批次 L1/Node/framework、Controlled 16/16；两个注册 L3 case 并行通过 |

无人工验收项。当前记录不表示完整 L3 或安装现场已通过。

## 最终验收结果

2026-10-19，Runner 分支 `codex/initialization-stop`，基于 `2aa17cce7da6fd75e59db76e18f2521d9e9777db` 的未提交实现。可携带的[证据摘要](evidence/initialization-stop/summary.json)固定原始报告摘要、Run、构建摘要和收尾结果；完整现场保留在以下独立调试包。

正式批次为 `C:/SteerlineDebug/dev-20261019-123304-d1277973-dirty/evidence/l3-initialization-stop/summary.json`，通过公开 `scripts/test.ps1 -Level L3 -Scenario orca-remote.initialization-stop -MaxParallel 2` 入口执行：L1 Go 324 项、Node 扩展 28 项、framework（含双 worker 冷缓存）及 Controlled 16/16 通过；所选两个 L3 case 均通过。Runner EXE SHA256 为 `bcf9cc4cd53303302804a08739e43ff224027e92bc464049272f9b4eeb062545`，该批次 helper SHA256 为 `fb087fe8611b5e1c970eafe25b1a39b0fcd80f8e70047cb9945f8d80965523b1`。

最终自查增加了仅 helper 的启动失败保护：启动返回带类型的 nil handle 不得触发清理 panic；已发生未知子进程创建又没有句柄时不得签发“写者已消失”。`C:/SteerlineDebug/dev-20261019-125200-d1277973-dirty/evidence/helper-guard-unit.jsonl` 中 `TestRemoteOwnership*` 四项通过。随后在该新包使用同一已合格 Runner EXE、最新 helper SHA256 `5a195821932a14784b4d0fb2c30fcc07ca5d25d773702bef3afb90996b45b3c4`，经既有注册协调器、隔离租约及证据校验器并行重跑两个真实 case；执行脚本保留为 `evidence/run-helper-guard-recheck.ps1`，结果为 `evidence/helper-guard-recheck/summary.json`，状态 passed。这是 helper 增量定向复验，不新增一次完整 L1/Controlled 通过声明。

| 最终 case | 独立夹具（相对最终调试包） | 结果与证据 |
| --- | --- | --- |
| before-readiness | `cases/dcfa1e1bebab/evidence/` | passed；`initialization-window.json`、`readiness-withheld.json` 证明真实原 native 已就绪、持久化尚无 readiness；`initialization-exit.json` 与 `offline-settlement.json` 证明原资源结算 |
| restart-before-readiness | `cases/91b3bccfc13b/evidence/` | passed；上述相同证明及 `initialization-restart.json` 证明服务换代保留原身份、零自动输入与替换会话 |

两个 case 只有一次可核对正文摘要的 helper 启动发送，业务发送为零；签名退出均验证 writers_absent/authentication_closed，公开离线重验 resources_settled=true。各 `cleanup.json` 均记录 native_stopped/service_stopped/facilities_restored=true；helper 暂存工作区及 PTY 已关闭移除，收尾回执保存在最终包 `evidence/guard-stage-*.json`。`evidence/source-snapshot.json` 固定最终 24 个代码/脚本/注册文件摘要。

测试清单现为 L3 23 个场景、31 个 case；本次选择其中 2 个，完整 L3 未执行。所有真实执行均限于新调试包及 SteerlineTest 专属夹具。未操作安装/保留运行环境，未修写历史 Run，未部署；原安装失败 Run 仍需保留其证据缺失状态。

## 已保留的首轮记录

- `C:/SteerlineDebug/dev-20261019-120959-d1277973/evidence/l3-initialization-stop/summary.json`：缺少显式注册项目路径，两个 case unavailable、零执行；补齐配置后使用新批次，不计通过。
- 同包 `evidence/l3-initialization-stop-02/`：L1 324 项、Node 扩展 28 项、Controlled 16/16 通过；新 L3 首轮失败，不能据此前置通过声称 L3 已通过。
- `restart-before-readiness` 在创建 Run 前因共享冷缓存的并发复制/读取文件占用失败，现场 `cases/373ebd16d384` 保留。改为校验临时副本后原子发布，新增双 worker 冷缓存回归已通过。
- `before-readiness` 现场 `cases/c6c4efc3d746`，Run `1f8c3ee1872ea1ca468db82ce27c170e`：已实际取得原 native readiness，Runner 持久化只有早期 ownership；驱动误把一次 helper 启动 send 计为业务输入而失败。已改为严格核对唯一 helper 启动正文摘要、原配置/工作区/公钥和零业务 send，服务换代期间保持该非零输入基线。
- 收尾进一步发现：初始化没有 readiness 时，离线重验错误地要求 reader journal；停止后查询失败又会丢掉既有 WorkerAbsent。两处均已修复并补完整 verifyStopped / 重复 reconcile 回归。原失败报告保持；最终核验器只读重验本次失败夹具，`cases/c6c4efc3d746/evidence/final-verifier-resource-recheck.json` 的 resources_settled=true，原测试仍记失败。
- 首轮暂存工作区/两个 PTY 已关闭移除。后续构建用另一个新调试包；不复用失败 Run，不修改其数据库/失败回执。
