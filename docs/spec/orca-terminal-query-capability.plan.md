# Plan：Orca 原终端查询能力 B1

日期：2026-11-02。状态：P0–P4 完成，P5 验收已记录、独立审计待完成；见[验收](../acceptance/orca-terminal-query-capability.md)。依据 [Spec](orca-terminal-query-capability.spec.md) 和[长期路线](orca-capability-layer.spec.md#6-分批实施路线)。本批只实施 B1，B0 独立审计的确认问题已用独立 commit `1393d0e` 修复。

## P0 固定接线和变更白名单

- 核对 QueryBoundTerminal/queryTerminal/validateBoundTerminal 的全部生产和测试消费者；保留 current original、控制分页、原输入证据和旧停止调用清单。
- 允许 `protocol/orca/terminal_query.go`、必要的 client 身份核对/原输入映射，以及 `orcatransport/lifecycle.go`、control_terminal.go 的窄读取接线；测试和文档另列。
- 不改 public CLI/schema、profile/state、创建 owner、停止策略、模型、权限或原输入证明。保存现行错误映射和编码样本，不以实现便利扩展范围。

| 现有位置 | 迁移或删除 | 保留 |
| --- | --- | --- |
| protocol/orca/terminal_query.go | backend 参数与错误迁为自有身份/错误；保留单一 RPC 实现 | 原连接和 envelope 核对 |
| protocol/orca/client.go | validateBoundTerminal 复用纯身份核对并映射旧错误 | CLI inspect/旧停止接线 |
| protocol/orca/native_input_query.go | 三字段值映射到 queryTerminal | 原输入业务判据，B3 再迁 |
| orcatransport/lifecycle.go | original 中构造器路由提为窄读取函数 | 原 worktree/path/branch/agent 策略 |
| orcatransport/control_terminal.go | 删除循环中两处专属 CLI inspect，接入同一读取 | 每页前后两次围栏、read 及回执用途 |

## P1 自有身份值与读取能力

1. 在原终端查询实现定义 Orca 身份值与身份不符错误，使该能力文件无 backend import；QueryBoundTerminal、底层读取及纯核对只消费原生值。
2. 更新原输入内部查询和现有 CLI bound 检查的值映射；保持共享核对逻辑，不复制另一套 RPC 或保留旧参数的公共兼容函数。
3. 仍保留 40 秒原查询预算、短父期限优先、独立连接、取消与正向身份冲突。外层执行错误继续由消费方解释。

## P2 两个实际消费者接入

1. 把 original 已有的生产 RPC/隔离 fixture 选择收口成窄读取函数，接受原 ProcessIdentity，映射到 P1 的 Orca 身份。
2. original 先保留原 State/执行绑定校验，再调用共享读取，之后执行原 worktree/path/branch/agent/初始化规则。
3. controlLines 每页读前、读后各调用同一窄函数；删除两处 CLI InspectBoundTerminal 调用，不动 terminal read、cursor、boundary、trim 和控制响应解码。
4. 新身份错误映射到既有执行错误时保留原原因链；生产连接故障没有 CLI fallback。旧停止 InspectBoundTerminal 不改接线路径。

## P3 确定性验证

优先复用 terminalQueryIPC、原 state socket/IPC 和 fixed-control fixture。按实际构造接入，不能仅给新函数注入成功回执；保留旧 fixture 构造的隔离能力，不把它误计为生产 RPC。

| 验证 | 入口和必需断言 |
| --- | --- |
| 能力身份/取消 | 现有 QueryBoundTerminal tests 改用 Orca 值；缺 runtime/handle/incarnation 零连接；错误请求、错 runtime/handle/incarnation、拒绝、取消和已知冲突保持 |
| 双端协议 | state_stream_test 真实 WS 夹具与本机 IPC，实际调用 QueryBoundTerminal，不使用 backend.ProcessIdentity |
| 业务接线 | OriginalTerminalRPCIdentity/BypassesCLIQueue/Cancellation/ChecksBothSidesOfObservation 原断言保持 |
| 控制接线 | 新增生产 nativeAccess 的 controlLines IPC 夹具：至少两页、每页前后 RPC 计数、CLI show=0；改 incarnation 时不采纳回执；持有 CLI gate 时身份查询仍可执行 |
| 控制业务 | FixedControlRetainedStream/StreamFaults/StreamBoundaries/ReceiptRecovery、history 与工作区原查询保持 |
| 原输入/停止 | thin/corroborated 构造器和原输入围栏、原 stop/verify/reconcile 定向不退化 |

执行受影响两包和 workflow 依赖守卫，使用 go test -json，记录实际 roots、叶例、fail/skip，不将零匹配算通过。新测试名在实现前固定为 `TestControlTerminalRPCIdentity`、`TestControlTerminalRPCFencesPages`；既有测试的语义断言不为新调用方式删除。

```powershell
go test -json ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout=12m
go test ./internal/execution/workflow -run '^TestSRExecutionDependencyBoundary$' -count=1
git diff --check
```

整包含已有 opt-in skip，报告实际 skip 名称并与必需 roots 区分；真实资格不得由 skip 计入通过。新功能定向测试先跑；整包只在定向完成后运行一次，后续只补受影响失败或审计修复项。

## P4 双端无模型真实查询

复用本轮已明确配置的 Orca runtime、专属测试 worktree，分别创建一个新 shell（不启动模型）。保存确切 runtime/terminal/incarnation/worktree；通过本批 QueryBoundTerminal 读取并核对，关闭读取连接后再次查询原身份。测试方法可作为现有 native 测试中的 opt-in 子入口，不新增生产探针命令。

真实查询前后分别保存 CLI/协议对照；不把 CLI show 当本批 API 的证明。每个原 shell 最多一次精确 close，完整清单确认结束，未知只对账。CLI 队列并发与身份替换在受控层验证，不修改真实共享环境制造故障，也不关闭共享 Orca。

## P5 验收与提交

- 按 OTQ-01–06 写 `docs/acceptance/orca-terminal-query-capability.md`，列自有能力、消费者、删除调用、剩余未迁 backend 模块和实际测试/原始证据。
- 同步长期路线及 Spec/Plan 状态，检查 profile/state/target/公开 schema 无变化，明确 B1 只减少控制身份查询的 CLI 调用。
- commit 本批实现，不提阶段 PR；随后独立只读审计一次，确认修复用独立 commit 并补受影响验证。B0 已有资格不平移成 B1 通过。

完成条件为 OTQ-01–06 的直接证据齐备、原资源收尾及独立审计完成。只有类型重命名或新增接口、生产消费者未迁入时不得标完成。
