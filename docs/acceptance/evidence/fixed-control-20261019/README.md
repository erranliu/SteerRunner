# 固定控制终端验收（2026-10-19）

执行期强杀恢复、原生完整回执读取与认证撤销已在固定控制终端方案下验证。初始化创建身份未知仍阻塞，不重试创建、不按标题或空 inventory 认领。未修改 SteerLine 或 Orca，未部署或合并源 PR。本记录只覆盖本增量，不宣称完整 L3 或旧批次全部通过。

## 最终实现与状态边界

每次 Codex/Orca 执行在 agent 启动和任何业务输入前，经原资源准备操作创建并提交一个控制终端。它仅支持 `original-turn`、`authentication-stop`；执行期查询/停止不再创建临时 inspector/revoker。控制身份绑定原 runtime/worktree/handle/incarnation/helper/config/generation；主执行 final 及原生/Git/provider 判定保持原来源。

原 Record/Intents/Commit/CAS 是唯一权威。原适配器 implementation 保存控制身份；caller 辅助检查点只记录同一通道的请求、回执和关闭事实，不创建资源、不授权业务输入、不决定 Workflow 后继。原终端缺失或换代仍阻塞。内部 `orca.terminal-state` codec 5→6，旧版明确拒绝；profile3、CLI v1、Run state v3、Workflow2 和调用配置不变，不增加兼容迁移或 SteerLine 配置义务。

Run stop 取消原观察器，撤销请求之后拒绝晚到查询。撤销对账有独立 45 秒清理预算；业务写者仍沿有界停止路径关闭。若撤销证据未核实，保留同一控制终端，原业务停止不冒充完整结算；后续只补读原请求及关闭原资源。撤销、broker/socket 清理、业务写者停止和控制关闭分别核验。服务 continue-execution 停机只释放观察，不停掉原执行或控制终端。

## 源码、环境与执行命令

最终生产与测试源码为 clean `212ebee3abeae798c05fe53ede19f8b2a5e0f0f3`。本记录之后只提交文档/证据；各 native manifest/build 保留自身构建来源。受控二进制和 native helper 都来自该提交，见 [受控身份](prerequisites/controlled/source-identity.json)、[原构建](prerequisites/controlled/steerrunner.exe.build.json)、[helper](checks/helper-build-r5.json)、[文件摘要](source-files.json)。helper SHA 为 `afb0822526f4632200bfeab2862c80c40ae2c4f434093c51eec231fcfcdf7235`，原配对主机的不可变路径与配置见[冻结配置](checks/live-config-r5.json)。

Windows/amd64、Go1.26.4、PowerShell7.6.5、Node24.13.0、Codex0.159.2、Orca1.4.223；环境初始采样单独标记其原源码/时间，见[环境](checks/environment.json)。实际最终源码/资源身份以各 manifest 和结果为准。仅用已有原生认证，不复制凭据、不新增登录。

最终[环境复核](checks/environment-final.json)另存测试后时间点和 SteerLine 只读状态；495 份生产/测试源文件与冻结源码的摘要一致。归档原文件逐字节、JSON、常见凭据模式、文档链接与差异检查见[检查结果](doc-check.json)及[检查脚本](checks/validate-archive.py)。

```powershell
go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/testkit/livetest -count=1 -timeout=5m -v
./scripts/test.ps1 -Level L3 -Scenario @('orca-remote.service-restart','orca-remote.stop-revocation') -SkipAdminRequired -Config .evidence/fixed-control-20261019/live-config-r5.json -OutDir .evidence/fixed-control-20261019/native-r5 -MaxParallel 2
# 上一入口的完整 L1 已通过；Controlled 的 15 分钟总预算耗尽，尚未启动任何 native case。
# 用同一已冻结 EXE 重跑完整 Controlled16，仅总预算改为 25 分钟；单场景/产品超时不变。
./.evidence/fixed-control-20261019/run-controlled-25.ps1
# 校验同源 L1 与完整 Controlled16 后，以原 scheduler/driver 启动新现场三项 native cases。
./.evidence/fixed-control-20261019/run-final-native.ps1
python .evidence/fixed-control-20261019/verify-control.py native-final
```

[受控重跑脚本](checks/run-controlled-25.ps1)和[原生续跑脚本](checks/run-final-native.ps1)保存实际参数、严格前置核验与排除记录。最后入口采用相同仓库 scheduler/driver/catalog，未替换产品 EXE 或场景断言；没有重复调用模型来填补失败证明。

## 自动验收结果

| 层级/边界 | 结果与证据 |
| --- | --- |
| 组件 | 有限协议、父绑定/异源/载荷冲突、丢响应同请求恢复、初始化未知/提交失败零业务输入、晚到查询拒收、停止取消预算、撤销未核实保留控制及原停止对账通过。[停止预算回归](checks/cleanup-budget-regression.log)；较早组件批次另存[原日志](checks/component-final.log)，不替代最终完整 L1 |
| 完整 L1 | 366 个 Go 顶层测试、29 个 Node 测试通过；7 个仓库允许的 helper/非本层项目跳过，逐项见 [summary](prerequisites/summary.json)。native driver 编译、零匹配守卫、[框架16检查](prerequisites/framework/result.json)通过；[Go JSONL](prerequisites/l1.jsonl)、[Node TAP](prerequisites/terminal-extensions.tap) |
| 完整 Controlled | 16/16 顶层通过，结果获取8子路径全部通过、无跳过；[结果](prerequisites/controlled/result.json)、[日志](prerequisites/controlled/integration.log)。源提交与两份 EXE SHA 和原15分钟批次一致，只有 harness 总预算变为25分钟 |
| 原生有限协议探针 | 同一 native request ID 重试被去重，helper 有可关联应答，验证用原终端已关闭；[接受](checks/probe-send.json)、[同ID重试](checks/probe-retry-exact.json)、[应答](checks/probe-after-retry.json)、[关闭](checks/probe-closed.json)。普通 helper 原生 provider/observation 为 unsupported 属已确认边界，不能冒充 agent turn-start；实际方法完成另验有限协议。此探针使用较早 prototype helper（[身份](checks/prototype-helper-upload.json)），只证明原生接口能力，不计为最终产品用例 |
| L3 子集 | 以下3/3通过，最大并行2；明确排除4项管理员网络/阶段变体，名称及原因见 [summary](prerequisites/summary.json)。不是完整 L3，不新增真机矩阵 |

| 原生场景 | 现场 | 结论 | 证据 |
| --- | --- | --- | --- |
| `orca-remote.service-restart/graceful-active` | `7c0acf7390e4` | 通过；原 Run 停止、service not_running、resources_settled=true | [场景](native/orca-remote.service-restart/graceful-active/7c0acf7390e4/scenario.json)、[控制终端原生证明](native/orca-remote.service-restart/graceful-active/7c0acf7390e4/fixed-control-native-proof.json)、[认证/关闭](native/orca-remote.service-restart/graceful-active/7c0acf7390e4/remote-authentication-cleanup.json)、[最终服务](native/orca-remote.service-restart/graceful-active/7c0acf7390e4/service-status-final.json) |
| `orca-remote.service-restart/kill-active` | `ffa62d9a9638` | 通过；原 Run 停止、service not_running、resources_settled=true | [场景](native/orca-remote.service-restart/kill-active/ffa62d9a9638/scenario.json)、[控制终端原生证明](native/orca-remote.service-restart/kill-active/ffa62d9a9638/fixed-control-native-proof.json)、[认证/关闭](native/orca-remote.service-restart/kill-active/ffa62d9a9638/remote-authentication-cleanup.json)、[最终服务](native/orca-remote.service-restart/kill-active/ffa62d9a9638/service-status-final.json) |
| `orca-remote.stop-revocation/active` | `38fc48687e2b` | 通过；原 Run 停止、service not_running、resources_settled=true | [场景](native/orca-remote.stop-revocation/active/38fc48687e2b/scenario.json)、[控制终端原生证明](native/orca-remote.stop-revocation/active/38fc48687e2b/fixed-control-native-proof.json)、[认证/关闭](native/orca-remote.stop-revocation/active/38fc48687e2b/remote-authentication-cleanup.json)、[最终服务](native/orca-remote.stop-revocation/active/38fc48687e2b/service-status-final.json) |

逐项原始 ledger 复核见[控制验证](checks/control-verification.json)与[校验脚本](checks/verify-control.py)：每次执行仅1个控制创建，就绪先于原主终端首条输入；恢复期间 runtime/handle/incarnation 不变；读取原 session/输入对应的完整 final；无逐次 inspector/revoker 创建；显式恢复授权前的围栏窗口及清理期间没有新增业务输入，授权后的后继沿用原会话/来源；原认证 revoked/broker/socket 与终端关闭均核实。[资源清理汇总](cleanup-summary.json)的3个最终 service 查询均 not_running/resources_settled=true。证明中的 receipt/authentication observations 是重复读取的观测数，不是额外执行或模型调用数。

归档保留控制通道调用及服务停止/恢复授权事件的原始子集、原执行审计结论及完整原 ledger/faults SHA；完整 ledger 留在各现场，不归档环境/凭据目录、数据库或二进制。本目录沿既有证据惯例禁用 Git 换行转换，保留原始字节。归档原文件路径/摘要见[来源索引](raw-origins.json)，全部文件摘要见[hashes](hashes.json)。测试 PR 仅在隔离 SteerlineTest 仓库产生，见[PR清单](created-test-prs.json)，不合并 SteerRunner 源 PR。

## 失败迭代保持原结论

- 首两次在 native/模型调用前主动停止前置构建，见 [r1](checks/interrupted-r1.json)、[r2](checks/interrupted-r2.json)，不计通过。随后修复 Windows 等价路径规范化与停止时晚到观察覆盖证明的问题。
- `native-r3`（源码11e5e22）：完整 L1/Controlled通过；两个恢复场景业务成功后，旧 driver 在清理前要求控制已关闭，导致两项原生失败；stop active通过。[原summary](historical/native-r3/summary.json)。清理后独立[复核](historical/post-cleanup-audit-recheck.jsonl)确认资源关闭，但没有改写失败。
- `native-r4`（源码382656c，生产与11e5e22逐文件一致）：改为清理后审计；graceful/stop通过，kill-active因待决控制查询被旧计数器误判而失败，清理又暴露真实产品缺口：调用方预算耗尽后销毁了尚无撤销证明的控制通道。[原summary](historical/native-r4/summary.json)及同目录原case/log保留。现场 service 已停止但 resources_settled=false，原 stop unknown 不清除；[最终服务](historical/r4-failed-final-service.json)、[原生最终inventory](historical/r4-failed-final-inventory.json)不冒充认证结算。随后增加独立45秒预算/保留原通道，并将待决有限控制请求与业务计数分开，最终完整审计仍拒绝任何未决控制证明。
- 最终源码212ebee的标准入口：L1通过，Controlled在15分钟总超时终止最后场景；没有 native case 启动。[原失败](historical/native-r5-timeout/summary.json)、[原日志](historical/native-r5-timeout/controlled/integration.log)。仅通过原实例公共 stop 清理遗留服务，[回执](historical/controlled-timeout-service-stop.json)、[最终状态](historical/controlled-timeout-service-status.json)为 not_running/resources_settled=true。随后同一冻结二进制完整16项在25分钟总预算下通过，再跑本记录三项新 native 现场。原失败不改写，测试/产品单场景预算未放宽。

RA-A08 在确认的执行期/初始化边界内通过。RA-A01–A07 的本次接线回归由完整 L1/Controlled覆盖；此前 parser/L2 原生验收仍保留自身源码，不声称本增量重新执行全量 L2。初始化身份未知、控制 actor 丢失/换代、缺原认证/关闭证明仍必须阻塞，未增加猜测恢复或第二状态权威。
