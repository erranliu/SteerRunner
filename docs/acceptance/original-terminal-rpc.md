# 原终端只读核验 RPC 增量

2026-10-26。初次实施基线为 `5df57b69e98382abcd63676b04a73079fd939fa6`；本轮只替换 Orca `Lifecycle.original()` 的只读终端元数据传输。提交前审计已同步 main `84c942cd100840ad89b1c730a2d2cceeac671d36`；最新审计结果见末节。

生产构造器通过现有 IPC / 配对加密 WebSocket 调用 `terminal.show`，每次查询独立开关连接。CLI 与 RPC 共用 runtime/handle/incarnation 校验；工作区、分支、worktree、agent、connected/exited 及初始化/受管 TUI 的既有例外仍在原调用处执行。观察前后两次核验均保留。RPC 包络核对 request/runtime，只有通过后才填写 Terminal.RuntimeID。生命周期读取沿用 40 秒上限，覆盖连接、握手和响应；更短的调用方期限与取消优先。

已有 native-input 查询共用底层只读函数，保留自身校验及调用方预算。显式 state-factory 构造器继续用于隔离协议夹具；生产读取失败没有 CLI 回退。未改动创建/发送/关闭命令、CLI 互斥、业务锁、存储格式、轮询周期、Usage 或执行调度。

## 回归范围

- `TestOriginalTerminalRPCIdentity`：正常、orphaned、refs 分支前缀、初始化空分支/agent、Pi/DSH 受管标签例外，以及 runtime/request/handle/incarnation/worktree/workspace/branch/agent 不一致、断连/退出、空/损坏/拒绝响应。生产构造器，真实隔离 named pipe，禁止回退 CLI。
- `TestOriginalTerminalRPCBypassesCLIQueue`：持有同一私有 cache key 的真实 Windows CLI 互斥时，两个查询均完成，CLI 调用为零。
- `TestOriginalTerminalRPCCancellation`、`TestQueryBoundTerminalHandshakeCancellation`：调用前取消、握手/读取期间取消及更短期限，检查 context 原因和有界结束。
- `TestOriginalTerminalRPCChecksBothSidesOfObservation`：第二次核验发现终端替换时，不发布消息或完成结果，不重发输入。
- `TestQueryBoundTerminalMissingIdentityDoesNotConnect`：缺身份时零连接；`TestOrcaNativeStateStream/terminal-query*`：实际加密 WebSocket 握手、读取、错误 runtime/request 与超时。
- 既有 native-input、停止/恢复、同服务多 Run、原创建回执与 CLI 排队回归。

受控创建夹具原先只在 CLI 返回终端事实，RPC 对未知方法返回空对象。第一轮受控测试被原身份校验拒绝，未放宽产品检查；补齐夹具，使 RPC 从与 CLI 相同的 `worktree-created.json` 读取原终端并保留原响应故障门。首轮日志保留在 `.evidence/terminal-rpc/controlled`，修正后使用新目录重跑。

## 修改前后并发对比

将同一比较夹具加入未修改生产代码的基线，随后在修改后的代码上运行相同命令，每次三轮：

```powershell
go test ./internal/execution/adapters/orcatransport -run '^TestOriginalTerminalConcurrency$' -count=3 -v -timeout 2m
```

使用生产构造器和同一终端快照，1/5/10/20 个并发调用者各执行三轮、每轮两次原终端核验。CLI 通过真实 FIFO / Windows 命名互斥，进程输出由隔离 host 代替；RPC 使用实际 named pipe、认证握手和独立短连接。两端均为每次终端读取注入 20ms 服务时间，RPC 握手另计真实开销。每档 P95 列取三次运行各自 P95 的中位数，吞吐列取三次运行吞吐的均值。

| 并发调用者 | 修改前两次核验 P95 | 修改后两次核验 P95 | 修改前读取/秒 | 修改后读取/秒 |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 42.65ms | 55.85ms | 47.44 | 42.34 |
| 5 | 207.83ms | 73.66ms | 48.31 | 182.30 |
| 10 | 414.28ms | 94.76ms | 48.40 | 317.18 |
| 20 | 827.52ms | 106.37ms | 48.33 | 493.27 |

20 调用者档三次共 360 次核验，CLI 调用由 360 降为 0，改为 360 次 RPC。该档两次核验 P95 下降约 87%，吞吐约为原来的 10.2 倍；单调用档短连接握手有额外开销。原始记录：[修改前](evidence/original-terminal-rpc-20261026/baseline-repeat.log)、[修改后](evidence/original-terminal-rpc-20261026/after-repeat.log)、[汇总](evidence/original-terminal-rpc-20261026/comparison.json)。

此结果只证明受控原终端读取摆脱 CLI 全局串行，不是 Agent、完整 Run 或真实跨机器网络的容量压测，不能据此宣称支持 20/50 个真实任务。没有启动真实模型或部署到生产服务。

## 初次实施验证（审计前）

定向协议/适配回归、两包及集成测试包 `go vet`、`git diff --check` 均通过。

正式 `scripts/test.ps1 -Level L1` 的框架守卫、29 项终端扩展、live driver 编译及零匹配守卫通过；application 包累计运行达到入口的 12 分钟期限，原批次报告保持 failed，626 个顶层测试已通过，16 个既有允许跳过项单列。没有单测断言失败。根据该批次的完成记录与 application 测试完整清单，精确补跑余下 54 个顶层测试，54/54 通过（423.718 秒）；application 共 128 项全部得到通过结果，两批合计 680 个不同顶层单测通过。未修改仓库测试预算配置。[原始 L1 报告](evidence/original-terminal-rpc-20261026/l1-summary.json)、[分批覆盖与原日志摘要](evidence/original-terminal-rpc-20261026/unit-coverage-summary.json)。完整原日志保存在 `.evidence/terminal-rpc/l1/l1.jsonl` 和 `.evidence/terminal-rpc/application-remainder.jsonl`。

受控批次使用 Runner SHA256 `9aa799fa060a38a68f1a46de0997075df68f320c04131300b1714f2d06f6ddda`：CLI 排队六变体、同服务多 Run 隔离、未知输入后的停止均通过。创建恢复七窗口中五项通过，`owner-crash`、`missing-initial-plan` 两项失败。两项都在终端事实尚未取得时停在 queued/stop_requested，而测试要求 reconciling；没有进入此次替换的原终端读取。[逐项原日志](evidence/original-terminal-rpc-20261026/controlled-expanded.log)、[构建身份](evidence/original-terminal-rpc-20261026/controlled-source.json)。

使用 Go overlay 恢复基线的三份已有生产文件，并把本轮新增查询文件替换为空 package，从基线生产实现重新构建；对照使用修改夹具前的原集成测试 EXE，独立运行同一两个窗口，均复现同样的失败状态。保留原断言与未知创建事实，不在本轮修复其它状态机行为。[基线失败日志](evidence/original-terminal-rpc-20261026/baseline-recovery.log)、[结果](evidence/original-terminal-rpc-20261026/baseline-recovery-result.json)、[构建/夹具身份](evidence/original-terminal-rpc-20261026/baseline-recovery-source.json)。

五项受控聚合运行设置 15 分钟期限，进程在期限后已结束，未生成最终 result.json；最后的重启专项没有完整结论，不能计作整批通过。已按原证据目录及各 fixture state-dir 内的进程实际路径/创建时间核实并停止本批遗留测试进程，未改写失败数据库。沿用完全相同的 Runner 与集成测试 EXE，在新目录独立补跑重启专项，graceful、kill、creation_receipt、pending_stop 四项全部通过（251.80 秒），退出码 0。[重启专项日志](evidence/original-terminal-rpc-20261026/restart-focused.log)、[结果](evidence/original-terminal-rpc-20261026/restart-focused-result.json)、[构建身份](evidence/original-terminal-rpc-20261026/restart-focused-source.json)。

初次实施收尾复核四份生产源码的 [SHA256](evidence/original-terminal-rpc-20261026/production-source.json) 均未变化，本批已知测试 EXE 的匹配进程数为 0，当时尚未提交或部署。正式 L1/受控聚合失败记录均保留，分批补跑与基线复现分别报告。

## 提交前实现审计

审计范围为 RPC 包络与终端字段身份核验、40 秒/父上下文预算、取消和连接释放、观察前后核验、停止前原资源检查、手动恢复、未知发送/停止的原操作对账，以及新增抽象边界。

发现并修复一项取消竞态：共享 `queryTerminal` 原先在连接或响应已经返回 `ErrStateIdentityChanged` 后，以 `ctx.Err()` 覆盖它。该函数也供原输入补证使用，取消不能隐藏已观察到的身份冲突。只调整两个错误分支的优先级，未新增生产抽象、重试或回退。确定性回归 `TestTerminalQueryCancellationCannotHideKnownIdentityConflict` 在修复前返回 context canceled 而失败，修复后保留身份冲突并通过。[修复前](evidence/original-terminal-rpc-20261026/audit-identity-cancellation-before.log)、[修复后](evidence/original-terminal-rpc-20261026/audit-identity-cancellation-after.log)。

补充 `TestOriginalTerminalRPCStopAndRecoveryKeepOriginalIdentity`：生产构造器通过实际 IPC 检查停止前的原终端和手动恢复前后身份；原身份一致才通过，停止前或恢复末次核验发现 incarnation 被替换时拒绝，零额外 CLI 操作。[停止/恢复定向记录](evidence/original-terminal-rpc-20261026/audit-stop-recovery-rpc.log)。

其余审计未发现未处理的回归：runtime/request 包络核验先于 RuntimeID 发布，终端/worktree/路径/分支/agent/存活检查及已有初始化例外保留；读取沿用独立短连接、40 秒上限和更短父期限，取消会结束握手/读取；停止中的独立清理预算、原终端 close 和资源退出证明不变；恢复只读取原事实，未知动作不重放。`queryTerminal` 和 `validateBoundTerminal` 各自服务两个已有调用点，没有连接池、缓存、调度接口、新配置或第二套状态权威。

审计同步 main `84c942c` 后，原先两个创建恢复测试的粗状态断言修正已由上游 `e28040e` 提供；本增量未修改该断言或状态机。最新基线两包整包回归 172 个顶层测试通过、5 个既有 live 条件项跳过，7 项应用层停止授权/预算/提交失败回归通过，两包及集成测试包 `go vet`、`git diff --check` 通过。[包级摘要](evidence/original-terminal-rpc-20261026/audit-package-summary.json)、[最终源码 SHA256、应用测试清单与完整日志 SHA256](evidence/original-terminal-rpc-20261026/audit-summary.json)。本次没有重新执行全量 L1；上节初次实施的超时记录及分批覆盖单独保留。

最新基线独立构建的创建恢复集成验证七窗口全部通过（183.95 秒，退出码 0），包括 graceful、kill、receipt-before-commit、wire-loss、owner-crash、planned-initial-terminal、missing-initial-plan；各窗口始终检查原 runtime/terminal/incarnation、一次创建、零额外终端创建及零业务发送。[完整记录](evidence/original-terminal-rpc-20261026/audit-creation-recovery.log)、[最终结果](evidence/original-terminal-rpc-20261026/audit-creation-recovery-result.json)、[构建身份](evidence/original-terminal-rpc-20261026/audit-creation-recovery-source.json)。未部署生产或增加其它优化。
