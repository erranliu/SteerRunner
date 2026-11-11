# Plan：orcaCLIOutput 恒真策略参数退役

日期：2026-11-10。状态：2026-11-11 已实施，合并前定向验收通过。依据 [Spec](orca-cli-output-policy-retirement.spec.md)，调查主线 `87b95cef4fc1624f602f6d311fe121d9352f2740`；本地 HEAD `11667669` 文件树相同。2026-11-11 获实施授权，基线为 `c404d6e5c50cc32650f9412f9b8b3449c5789e15`。

## P0：基线与实际调用根

- [x] 后续获得实施授权后刷新 main，记录 HEAD/目标/工作树实际差异，上游变化先核对；按 AGENTS.md 处理真实冲突，不为 Rebase 提示改写已验证历史。
- [x] 全仓核对 orcaCLIOutput 的调用/函数值、参数传入和各 build tag/工具，仍仅 NewLifecycle/newFrozenReader 两个恒真根才继续。分别登记 #141 新增的普通读取消费者、只读冻结身份入口及同名 Lifecycle.nativeAccess 字段的真实消费者，不按名字批量删除。
- [x] 保存 frozen_record.go/lifecycle.go、既有测试与共享 fixture/driver/helper/JS/脚本的原字节和摘要。对照原 WithTimeout/SerializeCLI、PreparationOwnsCLI、UsageHostID、命令编译/校验、reader 白名单和全局 dispatch 实际入口，记录所有要保留的正文。
- [x] 捕获保留类型/固定代表 Profile/State/Record/envelope 编码及 capabilities/schema/resource_digest；实际字段/tag/codec不变，原编码/读取错误不忽略。测试动态路径、时间/EXE SHA 不要求跨现场整体相同；只用一次性证据工具，不新增产品审计框架。
- [x] 先运行原 adapter 十一项（尚无三个新名）和 protocol 七项，分别保存原 JSON/stderr/退出/子例与失败/skip。如实记录原失败。

原 adapter 基线（#141 复用项可单独复验）：

```powershell
go test ./internal/execution/adapters/orcatransport -run '^(TestOrcaCLICommandEncodingAndConsumers|TestStructuredConstructorFacts|TestStructuredConstructorFaultPriority|TestCurrentFixtureUsesIsolatedHostWithInvalidPairingStore|TestFrozenReaderHasNoExecutionAndRejectsMutation|TestFrozenReaderKeepsConstructorFaultPriority|TestRetiredExecutionFactoryKeepsFrozenProof|TestRepositoryQueryRetryBudget|TestPreparationReadRecoveryRetainsBudget|TestPreparationRevokedBeforeOwnerClaimSettlesWithoutDispatch|TestOrcaWorktreeReuseRebindsOnlyWorkspaceAndRequiresStop)$' -count=1 -timeout=5m -json
```

## P1：先补真实接线测试，保留旧生产参数

1. 新增一个 internal/execution/adapters/orcatransport/cli_output_policy_test.go，不修改既有测试或共享 fixture。计划名 TestOrcaCLIOutputConstructorsKeepTransport、TestOrcaCLIOutputOrdinaryCallWaitsForQueue、TestOrcaCLIOutputPreparationOwnsCLI 当前均不存在；新增后必须实际匹配，若改名同步 Spec/Plan 与执行集合。
2. 用中性 fixtureSetup/fixtureProfile 在构造前冻结完整 setup/profile/target/版本；ProcessHost 使用现有接口的计数隔离 host。两条路径必须调用真实 NewLifecycle 或 newFrozenReader，随后使用原 client，不注入/替换 output，不手工拼 Lifecycle/frozenRecord，也不使用会改 client/nativeAccess 的 structuredFixtureOn。
3. 构造测试：两个入口各 local/paired 成功，分别用 status 与 terminal list/show；原构造零外发、读操作恰一次、Command/返回事实/上下文上限及 setup/profile/snapshot 不变。每例隔离 ORCA_USER_DATA_PATH/LOCALAPPDATA，禁止 t.Parallel 共享环境；host 仅返回本测试原格式回执，不启动真实 Orca CLI、owner 或网络连接。
4. paired current 用合法测试 store 与原 UsageHostID 冻结 host；同一合法构造后改公钥、移除/破坏 store，分别确认原配对错误且外发数不增。local 和历史 reader 不新增配对检查。reader 先合法只读成功，再 mutation 在外发前拒绝，不放宽白名单；原 CLI Golden 测试保持并注明其 client seam 的验证范围。
5. 普通取锁测试：原 SerializeCLI 持有同一隔离锁键，以 ready/release 屏障控制；原 client 的新只读请求在持锁期间取消/到期并保持零外发，返回原 queue/context 分类，不误记 unknown/JSON 错误。释放并等待持锁者退出后新只读调用成功一次。失败路径同样 cancel/release/drain；不用 sleep 冒充已入队事实，不新增锁窥探或生产 hook。
6. 准备测试只用当前端口原 client 的 PreparationCall(ctx, true, checkpoint, "status", "--json")；在 checkpoint 内对同一隔离锁键执行有界 SerializeCLI 竞争，要求 CLIQueueError 包装 DeadlineExceeded 且竞争回调零次。原调用继续成功，checkpoint 和 host 各一次；host 分别核对真实 marker、Command.Timeout=15s、实际生效截止等于更短的父 context 截止。checkpoint 失败子例要求 errors.Is 原错误且为原 CLIQueueError、零 host；不对 reader 调 status、不用 serialized=false、不伪造 marker 或改私有 key。若真发生重复取锁，必须由测试有界失败，不能挂到全局 go test 超时。
7. 在旧回调/false 分支尚保留时运行下列新增三项，证明测试前置能沿现行真实路径通过；遇失败先核对 profile/配对/锁与计数，不放宽生产规则。保留原十一项和新增三项的身份，不把新增验证写成原基线已有覆盖。

```powershell
go test ./internal/execution/adapters/orcatransport -run '^(TestOrcaCLIOutputConstructorsKeepTransport|TestOrcaCLIOutputOrdinaryCallWaitsForQueue|TestOrcaCLIOutputPreparationOwnsCLI)$' -count=1 -timeout=5m -json
```

## P2：删除恒真策略并核对批准变换

- [x] frozen_record.go：移除 orcaCLIOutput 的 nativeAccess func() bool 参数和 newFrozenReader 常真实参；去掉 if nativeAccess() 包装，保留原 true 分支 WithTimeout/cancel/SerializeCLI，仅作必要缩进；删除原 false 分支的 CheckpointDispatch/最后 invoke。不新增可达 checkpoint，不改变 marker 分支或 invoke。
- [x] lifecycle.go：只删 NewLifecycle 的常真实参；nativeAccess bool 字段、初始值和所有其它正文保持。两个文件 imports 仍有消费者，不顺带整理。
- [x] 对照原函数与调用根：普通超时/串行、准备已持锁直调、配对核验、命令编译/校验、结果与错误、reader 白名单逐字保持；proto PreparationCall/marker/锁、业务派发 gate 和全局 checkpoint 正文无改动。无新策略壳或重命名回调。
- [x] 除批准两文件和新增一个接线测试外，全部源/原测试/工具原字节不变；保留声明/代表编码及固定公开资源摘要相同，源码/EXE 身份变化单列。超出范围先修订设计，不能用精简名义扩大运行变化。

## P3：最小实施验收

仓库根目录 PowerShell 7、Windows/amd64。Go count=1，分别保存基线/新增前置/最终输出与实际测试名/子例/退出/失败/skip，不覆盖旧证据。普通宿主复验不抹去沙箱失败。

### P3.1 最终 adapter 十四项

```powershell
go test ./internal/execution/adapters/orcatransport -run '^(TestOrcaCLIOutputConstructorsKeepTransport|TestOrcaCLIOutputOrdinaryCallWaitsForQueue|TestOrcaCLIOutputPreparationOwnsCLI|TestOrcaCLICommandEncodingAndConsumers|TestStructuredConstructorFacts|TestStructuredConstructorFaultPriority|TestCurrentFixtureUsesIsolatedHostWithInvalidPairingStore|TestFrozenReaderHasNoExecutionAndRejectsMutation|TestFrozenReaderKeepsConstructorFaultPriority|TestRetiredExecutionFactoryKeepsFrozenProof|TestRepositoryQueryRetryBudget|TestPreparationReadRecoveryRetainsBudget|TestPreparationRevokedBeforeOwnerClaimSettlesWithoutDispatch|TestOrcaWorktreeReuseRebindsOnlyWorkspaceAndRequiresStop)$' -count=1 -timeout=5m -json
```

要求三个新名和十一个原名均实际执行；三项新测试的构造正反例、普通锁/取消与 marker 正/拒绝分支必须出现在实际子例/计数中。原 Golden、构造首错、坏 pairing store 隔离、历史证明与准备预算/撤销及 #141 worktree reuse 全部保留，不预报子例数量，不把少匹配/平台 skip 当作通过。此集合不是完整 adapter/application 或真实本机/paired 业务资格。

### P3.2 protocol 七项原边界

```powershell
go test ./internal/execution/adapters/protocol/orca -run '^(TestOrcaCLISerializesProcessesWithoutRetryOrDispatchAfterCancellation|TestOrcaCLIPreparationSharesMutexAndRecoversKilledOwner|TestOrcaCLIFIFOAndCanceledTicket|TestOrcaPreparationQueueBeyondOldExecutionLimit|TestOrcaQueueErrorDoesNotParseEmptyOutput|TestDispatchCheckpointRequiresSuccessfulSingleDurableWrite|TestAgentIPCDispatchCheckpointPrecedesEveryByte)$' -count=1 -timeout=5m -json
```

七个原名基线存在，实际必须全匹配。保留原跨语言/跨进程互斥、被杀 owner 释放、FIFO/取消、排队超过旧期限但执行预算从取得锁后开始、队列错误不解析空 JSON、checkpoint 单次与失败/撤销零 IPC 字节。Windows 场景不能因平台跳过计作通过。原 16 秒队列等待是该测试独有覆盖，不为加速随意缩短；它不代表本项改变或保证任务吞吐。

### P3.3 正式构建与身份

```powershell
./scripts/build.ps1 -OutFile .evidence/orca-cli-output-policy-retirement/final/steerrunner.exe
```

冻结最终生产/新增测试点后正式构建，保存退出、构建记录、EXE SHA、source_commit/source_dirty/source_digest 及最终输入清单，核对与通过点一致；固定资源摘要与源码/EXE正常变化分别表达。不要用旧 EXE 或构建成功冒充公开运行。

最小集合直接验证可达输出装配及原锁/预算/dispatch 边界；本项不默认安排公开受控、完整 adapter/protocol/application、L1/Controlled、L2–L4、live driver/helper 独立编译、Node 或真实 Orca/模型/PR。若生产/共享测试实际影响扩大，按 docs/testing.md 先修订范围再补选；通过项仅因新改动、失败或未解问题重跑。

## P4：独立验收与结束

- [x] 在 docs/acceptance/orca-cli-output-policy-retirement.md 独立登记 OCO-01–06：恒真根与批准删除、两入口原 client、隔离配对/命令/期限、普通取锁/取消与准备 marker 的实际计数/回执/清理、原/新集合及失败/skip、编码/摘要与产物身份、未验边界。
- [x] 复核 nativeAccess 字段及原 decoder/配对/reader/准备/锁/全局 dispatch 保留；所有既有测试和共享工具不改，新增测试没有替换 client/output 或复制队列/marker实现。区分假 host/本地配对样本与真实远端能力，不回填原基线或旧构建资格。
- [x] 实际验收完成后才更新 Spec/Plan、索引及必要长期实施说明；按 .gitattributes 保存原证据字节，不给中央 README 验收表插行，不做目录/历史归档大整理。
- [x] OCO-01–06 完成即结束，不自动推进其它候选，不提交/推送/PR/合并/部署。若后续另获 PR 授权，创建和最终交付前刷新目标并核对 GitHub 实际文件合并状态，优先 Squash and merge，不为 Rebase 提示改写已验证历史。

## P5：交付时主线复核

2026-11-10，文档完成前再次刷新 main 至 `bda3bc8ca54a01975420ef71acb8a59748edd904`（#140）。相对调查基线，新增改动仅为 L3 显式复用完整前置的脚本/框架测试、测试指南和独立验收；本批两个构造入口、输出闭包、协议队列/marker/dispatch 及所有计划测试声明未变。已按最新测试指南复核本批独立 Go 定向和正式构建入口，最小范围与未取得完整资格的边界保持，不复用或继承上游测试结果。本地 HEAD `11667669` 与原调查基线文件树相同，不表示与新增 #140 的整棵文件树相同；实施前按 P0 重新核对，结果见下一段。

2026-11-11，实施前刷新 main 至 `c404d6e5c50cc32650f9412f9b8b3449c5789e15`（#141）。独立子代理确认删除和保留清单成立，核对了两个构造入口的合法 profile 与配对装配。审计后登记 #141 下游消费者及其定向测试，并补入 checkpoint 同锁竞争与命令预算、父截止的独立断言。编码和资源基线取自该提交。

2026-11-11 交付前再次刷新 main，仍为上述 #141 提交。P0–P4 已完成，执行集合、初始夹具失败和复跑、构建输入与证据见[验收](../acceptance/orca-cli-output-policy-retirement.md)。
