# Orca 原终端查询能力 B1 验收

日期：2026-11-02。状态：实现、受控回归及双端无模型现场通过，实施后独立审计待完成。依据 [Spec](../spec/orca-terminal-query-capability.spec.md) / [Plan](../spec/orca-terminal-query-capability.plan.md)。本批不改变生产支持组合、完成/停止合同，不授予 B5 真实 Runner 资格。

## 1. 能力和接线

`protocol/orca/terminal_query.go` 使用自有 TerminalIdentity 与 TerminalIdentityError，不再导入 backend。QueryBoundTerminal 及底层 queryTerminal 接收原 runtime/handle/incarnation，保持既有 RPC、40 秒上限、短父期限、取消及独立连接关闭。缺身份在连接前拒绝，身份错误保留 ErrStateIdentityChanged。

`Lifecycle.boundTerminal` 负责执行身份映射和既有 BackendIdentityMismatch 分类。真实 NewLifecycle 使用 RPC；显式 state-factory 构造继续隔离 fixture，生产失败不回退 CLI。original 保留原 Snapshot、worktree/path/branch/agent 和初始化判据；controlLines 每页前后各调用一次相同入口，删除两处专属 CLI InspectBoundTerminal，read/cursor/完整性及回执用途不变。

旧 CLI bound 核对复用同一纯身份校验；原输入查询仅进行三字段值映射，原证明组合不变。包内 client、native_input、native_input_query、readonly_close、state_feed、state_query_feed、stopped 仍有 backend 依赖，按 B2–B4 继续迁移；B1 不宣称整个包已解耦。

生产变更共 5 文件：terminal_query.go、client.go、native_input_query.go、lifecycle.go、control_terminal.go。未改 profile/state/creation plan、配置摘要、target hash、公开 schema 或停止策略。

## 2. 受控验证

原始记录见[证据归档](evidence/orca-terminal-query-capability-20261102/README.md)，所有 Go 检查使用实际 test2json 结果统计。

| 检查 | 结果与边界 |
| --- | --- |
| B1 定向 | 11 根、52 子例；业务/控制 RPC、缺身份、取消及原观察围栏通过；远端 WS 的 8 个子例首次受沙箱网络限制，正常权限下整根 9 子例复跑通过 |
| 受影响两包全集 | 213 根、918 子例；6 个既有 opt-in 根跳过；15 个根首次因沙箱子进程、互斥锁或 WS 访问失败，原 15 根/50 子例正常权限下复跑全部通过；其余结果保留 |
| 依赖守卫 | TestSRExecutionDependencyBoundary 1 根通过 |
| 源码检查 | git diff --check 通过；查询能力文件无执行包导入；配置/codec/创建 owner/公开资源 diff 为空 |

全集的 6 个 skip 为 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestUsageLocalAcceptanceRepoCleanupLive、TestOrcaNativeStateLiveLifecycle、TestShortInputProbeInspect、TestOrcaWorkspaceStartupLive。新增 TestOrcaTerminalQueryLive 在全集启动后加入并单独编译、实际运行，未计入上述全集根数。

TestControlTerminalRPCIdentity 的 6 个子例验证有效双页、错 handle/incarnation、缺终端和错 runtime/request；有效场为 4 次 RPC、2 次 read、CLI show=0。TestControlTerminalRPCFencesPages 在第 1、2、3、4 次查询分别换 incarnation，均拒绝返回任何控制行；CLI-queue 子例持有真实原 CLI mutex，读前身份查询仍完成，释放后两页正常结束。原取消、原输入、history、managed 停止、恢复及编码测试保留。

## 3. 双端真实 shell

现场源码基线 `f6b0da3` 加本批差异，source_digest 为 `a335289546493ac868fa6c37d87360c71d4098d711ff2738946e61a9f8943d36`；同一查询测试 EXE SHA256 为 `ada50cf11a5c24584beb3f8b28d15222af09dd79bcc7b0d830ff455099d9d5f0`。CLI SHA256 为 `e7a6fa40a59d237cdb7cf32444df6660da850d1b31e4162634287d91fdfc3cba`，同 B0 的已选 executable。

| 目标 | 原 runtime / 终端 / incarnation | 结果 |
| --- | --- | --- |
| 本机 | 8a0d209e-094d-4ec3-a923-5cf6c064df67 / term_2b5d2177-c73c-45bd-87ee-e423b7a82e19 / 5c1265e3-5c63-4824-9b5f-d69d013eba2a | 两次独立 RPC 连接核对原身份/worktree，均通过；一次精确 close，原主机完整空清单 |
| 台式机64G | 64bdec68-d3ed-4216-a894-0cc143ed4653 / term_1b220b32-edd7-4740-af1a-1f75d78d77ec / c03099a8-3d93-496b-8a59-f0e098cf9f2f | 同上，经明确 environment 的配对连接 |

复用本任务 B0 的两端专属空 worktree，各只创建一个默认 shell，未启动模型、helper、控制 actor 或业务输入。测试仅直接调用 QueryBoundTerminal，CLI show 为关闭前身份对照，不用它替代能力证明。两个 shell 均已收尾，fixture worktree 仍归外部 fixture 所有。

预检初次把其他主机 omittedHostIds 也要求为空，因此拒绝继续；未派发创建。修正为现行合同的原执行主机 local 可见且未遗漏，其他主机遗漏事实原样保留。第一次 EXE 调用因 PowerShell 拆分含点 flag，在参数解析阶段退出，无查询或资源重建；将 flag 作为完整字符串后，在同一原 shell 进行只读核验。两项失败记录保留，不计通过。

## 4. 验收闭环

OTQ-01 由自有类型/无 backend 的查询文件、零连接缺身份和双端原生夹具证明；OTQ-02–04 由生产构造器、控制双页/四围栏/queue 及原取消测试证明；OTQ-05 由两包回归、依赖守卫及白名单 diff 证明；OTQ-06 由双端同 EXE 的两次查询、原身份及完整空清单证明。

提交后独立只读审计一次；确认问题用独立 commit 修复并补受影响验证。该审计完成前长期路线保持“验证通过、待审计”。
