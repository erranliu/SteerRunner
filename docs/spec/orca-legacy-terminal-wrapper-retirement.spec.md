# Spec：Orca 旧终端 create / send / read Go 包装器退役

日期：2026-11-11。状态：2026-11-11 已实施，合并前定向验收通过。审计主线 `e6cac6e17bf1b8de198c115a06308361b42d2b3c`（#142 已合入），原设计 HEAD `389f11952ce2825576b064df7d8ea460dcace8b3` 文件树相同，实施分支从审计主线建立。实施步骤与选测见 [Plan](orca-legacy-terminal-wrapper-retirement.plan.md)，职责边界见[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[Orca 执行契约](../orca-execution.md)和[测试指南](../testing.md)。

## 1. 目标

当前新 Orca Run 通过结构化 Agent Session 派发和读取历史。旧终端 Go create/send/read 包装器只剩各自协议单测，仍让读者判断它们是否参与现行派发、恢复或历史证明。移除三个专属实现及其测试，可缩小协议包的可调用面。旧冻结记录继续严格解码，公共终端调用、历史停止证明和独立诊断沿原入口保留。

## 2. 删除清单与实际消费者

路径均相对于 `internal/execution/adapters/protocol/orca/`。

| 生产对象 | 实际消费者 | 批准变更 |
| --- | --- | --- |
| `resource_create.go` 中 `TerminalCreate`、`TerminalCreateArgs`、`TerminalCreation`、`DecodeTerminalCreation` | `TestOrcaResourceCreateCapability` 中的 terminal 参数和回执子例 | 仅删除这四项声明及其正文 |
| `terminal_send.go` 中 `TerminalSend`、`TerminalSendReceipt`、`TerminalPromptReceipt`、`Client.SendTerminal`、`Client.SendTerminalWithTimeout` 和专属编码/验证/解码/匹配方法 | `terminal_send_test.go` 的 `TestOrcaTerminalSendCapability` | 删除生产文件及专属测试文件 |
| `terminal_read.go` 中 `TerminalRead`、`TerminalReadPage`、`Client.ReadTerminal` 和专属页/身份方法 | `terminal_read_test.go` 的 `TestOrcaTerminalReadCapability` | 删除生产文件及专属测试文件 |

混合的 `resource_create_test.go` 保留 `TestOrcaResourceCreateCapability` 原名称和两个 worktree 循环。删除 `initial` 与两个显式 command 参数子例（含空字符串）、四个 `terminal-*` 解码子例，以及只服务参数子例的 `ptrString`。原三项 `agent-*` 参数子例和七项 `worktree-*` 解码子例保留原名称、输入、断言和顺序。专属 send/read 断言随被测包装器一起退役，避免用测试副本重新建立旧协议模型。

实施允许的源码与测试差异仅以上六个文件，四个文件删除，两个混合文件只删除指定块。复核范围含 build tag、内部调用、方法值、类型引用、测试、工具与脚本；发现额外消费者先修订设计。

## 3. 保留清单及合法入口

| 保留面 | 当前入口与行为 |
| --- | --- |
| `WorktreeCreate` / `WorktreeCreateArgs`、`WorktreeCreation` / `DecodeWorktreeCreation` | 当前 `orcatransport/workspace.go` 消费编译与解码，创建 owner 沿原计划、认领、回执和只读对账工作。保留 `AgentTerminalHandle`、`StartupTerminal` 原字段与解码，包括 worktree 原生回执中的 terminal 事实。 |
| `Terminal`、`TerminalIdentity`、`ShowTerminal` / `InspectTerminal`、`CloseTerminal`、`CompleteWorktreeInventory` 及相关错误 | 历史 reader 沿 `terminal list/show` 与原 identity/scope 证明停止。当前结构化停止仍有原 shell 清理，独立终端关闭走原核验路径。保留真实关闭与原清单验证、reader 在 I/O 前拒绝 mutation。 |
| 共享 `Client` / `Result` / `ParseEnvelope` / `DecodeCLIResult`、命令编译、CLI 串行与 checkpoint | 现行工作区、终端清单、准备查询、结构化 IPC 和诊断继续消费，保留调用预算、配对、未知和原派发顺序。 |
| 结构化创建、输入、`agentSession.history`、关闭及原工作区复用 | 保留全部当前合法 Workflow/Agent/目标、session 与原身份、停止/排空和换构建边界。旧 `OpenSessionReader`、共享状态查询/订阅也留在本批范围之外。 |
| 持久和公开契约 | state/v8、user_version=8、实际历史 profile/state/receipt/NativeProof 与 scope、Workflow/提示词、公开 CLI/API/schema/capabilities/resource_digest 和用户数据保持。 |

### Pi 与 stability 独立入口

- `scripts/probe-orca-pi.ps1` 的 `Invoke-ProbeOrca` 直接调用指定 Orca executable，参数由脚本组装，原终端 create/send/read 及相关身份、回执和清理保持。该入口是独立旧 Pi 诊断，当前 Pi 结构化执行沿原接口。
- `tests/native/stability_test.go`（`windows && live`）的 `stabilityRun.cli → diagnostic → livetest.DiagnosticRead` 走原外部 CLI 与共享 `ParseEnvelope`，辅助终端 create/read 参数及结果解释在诊断侧。保留 `QueryOriginalAgent` 与 `tests/helpers/stability` 的状态/订阅、terminal show 装配；两条 Go 入口都不引用待删符号。原 raw argv 和 probe/driver/helper 文件原字节保持。诊断重试只针对明确的公共只读操作及连接错误。辅助关闭单独经 `stabilityRun.closeAuxiliary → livetest.CloseAuxiliaryTerminal → orcatransport.CloseReadonlyTerminal` 核对原身份和完整清单，保留现有回执丢失后的处理。

## 4. 最小验收

| ID | 通过条件 |
| --- | --- |
| OTC-01 | 上述删除闭包和六文件差异吻合。全仓实际消费者核对无遗漏，工作区及共享协议声明原字节保持。 |
| OTC-02 | 删除前确认三个旧能力测试的实际执行；删除后专属 send/read 与混合 terminal 子例消失，原十个 worktree 子例同名、同断言且均实际通过。 |
| OTC-03 | 保留的工作区创建参数、回执绑定与 owner 原操作恢复通过，未知及错来源不重派。原 terminal 身份、清单、停止范围及 reader mutation 拒绝通过。 |
| OTC-04 | Pi 脚本入口静态核对和解析通过。stability helper 与带 `live` tag 的 native driver 编译通过，入口文件原字节相同。两项现有隔离测试验证诊断 mutation 不重放及辅助关闭按原身份和完整清单对账。 |
| OTC-05 | 正式 Windows/amd64 构建成功。记录执行集合、失败/skip、构建身份与代表持久编码、固定资源摘要一致性。类型集合只减少批准的七个声明，全部保留类型相同。 |

精确集合、通过判据和证据归档见 Plan。最小资格为合并前定向组件验证、Pi 静态验证、两项诊断编译与正式构建。真实 Pi、stability、Orca/模型/PR、完整 L1–L4 或 Controlled 需各自原资格入口。本轮已获审计修订与实施授权。

2026-11-11，交付前刷新 main 仍为实施基线。前后定向测试、Pi 静态解析、stability helper/native driver 编译及正式构建均通过，见[独立验收](../acceptance/orca-legacy-terminal-wrapper-retirement.md)。
