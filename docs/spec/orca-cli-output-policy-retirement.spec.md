# Spec：orcaCLIOutput 恒真策略参数退役

日期：2026-11-10。状态：2026-11-11 已实施，合并前定向验收通过。2026-11-11 获实施授权。调查主线 `87b95cef4fc1624f602f6d311fe121d9352f2740`（#139 已合入）；本地 HEAD `11667669` 与该主线文件树相同。配套 [Plan](orca-cli-output-policy-retirement.plan.md)，依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[Orca 能力边界](orca-capability-layer.spec.md)、[CLI 等待与恢复](orca-cli-wait-retry.spec.md)、[原执行/停止合同](../orca-execution.md)及[测试指南](../testing.md)的现行条款。

## 1. 目标与范围

orcaCLIOutput 的 nativeAccess 回调参数只有两个调用者，NewLifecycle 与 newFrozenReader 均传入恒真闭包，没有 false 或其它函数值入口。该参数仍表达一项无人选择的串行/非串行策略，并保留不可达的 CheckpointDispatch→invoke 分支，增加后续修改输出路径时需要判断的状态。

本项删除恒真参数及不可达分支，让现有可达行为直接表达：普通调用沿原超时及 SerializeCLI；原准备查询已持锁时，凭 PreparationOwnsCLI context 直接 invoke。收益限于减少一个无实际选择者的策略参数和分支，不重做队列/传输/恢复，不承诺任务提速、EXE 缩小或全仓净删。

本项同时补足两个真实构造入口的隔离接线验证。旧 CLI 编码测试在构造后替换 client，能证明编译/creation Golden，不能单独证明原 orcaCLIOutput 闭包及其取锁行为。不得用新增测试副本、替换 client/output 或模拟 marker 回避这一缺口。

## 2. 实际消费者与精确删除清单

| 对象 | 基线消费者/合法入口 | 本项处理 |
| --- | --- | --- |
| orcaCLIOutput 的 nativeAccess func() bool 参数 | frozen_record.go 的 newFrozenReader、lifecycle.go 的 NewLifecycle 各一处，均为 func() bool { return true } | 删除参数和两处实参，复核函数值/测试/工具/build tag，无新增消费者才执行 |
| if nativeAccess() 包装 | 普通上下文永远进入 true 分支；已持锁上下文先经 PreparationOwnsCLI 返回 | 移除条件包装，原 WithTimeout、cancel、SerializeCLI 按原顺序直接保留 |
| false 分支的 CheckpointDispatch 与最后 return invoke | 无合法入口；不是当前结构化 IPC 派发检查的接线 | 删除这段不可达闭包，不把它补接到可达分支；全局 checkpoint 能力及其它调用保持 |
| Lifecycle.nativeAccess bool 字段 | 构造为 true；repository_query.go、workspace.go 和隔离测试装配仍消费 | 不删除、不改名、不改变默认值或测试赋值，不与同名回调参数混淆 |

生产允许差异仅两个文件：frozen_record.go 的上述签名、实参和分支收缩，lifecycle.go 的一处实参删除；允许必要 gofmt。其余生产正文、imports、字段、声明、错误、读写和调用顺序保持。不新增生产函数/回调/配置/测试 hook，不移动函数或重建等价策略接口。发现实际调用根变化先修订设计，不删调用者制造恒真结论。

实施基线 #141 新增 `ReuseManagedWorkspace → NewLifecycle → originalAgentWorkspace → client.ShowWorktree`，仍是普通调用的下游消费者。`ManagedWorkspaceIdentity → newFrozenReader → agentState` 只读冻结状态。现有 worktree reuse 测试经共享 fixture 验证复用边界，两个原构造接线由 §4 的隔离测试独立验证。

## 3. 明确保留清单

| 保留对象 | 必须保持的行为 |
| --- | --- |
| cliLaunchCommand、process.CheckCommandLine、invoke 与 ProcessHost.OutputCommand | 原 prefix/args、environment 路由、工作目录、环境屏蔽、超时和命令行限制；一次实际调用及原结果/错误，不增加重试、fallback 或新 checkpoint |
| PreparationOwnsCLI 分支、PreparationCall 及 context marker | 原外层取锁、calling checkpoint、15 秒执行预算和 marker 生成顺序；在该上下文输出直接 invoke，不能重复入同一 FIFO/OS 锁 |
| WithTimeout/SerializeCLI、FIFO、OS mutex、CLIQueueError | 普通调用原队列/调用期限、取消、同锁互斥及错误分类；不把等待取消记作已派发或未知后重发，不修改锁实现 |
| UsageHostID 与冻结 agentHostID | 结构化 paired 调用在实际 invoke 时核对原配对公钥指纹，缺失/改变继续拒绝；本机不读配对信息，不给历史终端 reader 新增结构化 pairing 门槛 |
| NewLifecycle/decodeFrozenRecord/profile decoders | 原旧协议拒绝、合法 profile/target/版本/host、clone 和首错顺序；构造零业务外发，不增加生产装配入口 |
| newFrozenReader 的 terminal list/show 白名单 | 保留只读性质和 mutation I/O 前拒绝，不扩大白名单以便运行准备查询；不变为 LifecyclePort |
| 全局 WithDispatchCheckpoint/CheckpointDispatch 及结构化 IPC 调用 | 原授权/落盘/检查及单次语义保持；仅删除此函数不可达分支中的那一次调用，不能按符号名称批量删除 |

creation owner/plan/started/result、原未知只读核实、历史格式/原停止 scope、当前结构化 close、inspectionGate/SessionID、files/usage 和用户数据保持。既有全部测试、fixtureSetup/fixtureProfile、structuredFixtureOn/frozenFixtureOn、fakeHost/creationTestHost、共享 driver/helper、JS/脚本原字节保持；只允许新增一个 cli_output_policy_test.go 放本批接线测试及最小装配/计数 helper。不得修改 shared fixture 默认值；structuredFixtureOn 会改 nativeAccess 并替换 client，不用于本批原闭包正例。

## 4. 两个真实入口的隔离验证

### 4.1 构造入口与传输事实

计划新增 TestOrcaCLIOutputConstructorsKeepTransport。在构造前由 fixtureSetup/fixtureProfile 装配完整冻结 setup，将 ProcessHost 换成现有接口下的计数隔离 host；经真实 NewLifecycle（当前结构化 profile）或 newFrozenReader（合法历史 profile）构造，随后直接调用它们原 client。禁止构造后替换 client/output、手工 Lifecycle/frozenRecord 或直接调用被测闭包替代构造接线。

两个入口各覆盖 local/paired 的成功调用。当前端口可用原 status --json；reader 只能用原 terminal list/show。采用原 local/paired 版本和完整 target/host，CLI prefix/executable 等在冻结前设置。核对构造零 OutputCommand，成功请求恰一次 OutputCommand，原 Command 编译字段、传入 timeout/有界 context 与 runtime/result/错误解码不变，原 setup/profile/snapshot 不改写。采样 context deadline 时比较原父/调用上限，不将墙钟微小误差当作固定性能指标。

ORCA_USER_DATA_PATH 与 LOCALAPPDATA 分别设为本测试专属临时目录，不使用开发者实际配对、bootstrap、全局锁目录或真实 orca.exe。paired 正例写原 store 结构、合法 32 字节测试公钥/endpoint/token，并用原 UsageHostID 得到冻结 host；这只读本地测试文件，不启动远端连接。结构化 paired 正例后分别改变公钥和使 store 不可用，要求原 pairing 错误、OutputCommand 零增加；不能修改 resolver 或注入假的 UsageHostID。local 及历史 reader 不新增该门槛，配对文件异常不能错误阻断原本不使用它的路径。不得将模拟配对与假命令回执登记为真实远端资格。

reader 的 mutation 调用仍在任何外发前拒绝，计数不增加；同一 reader 的合法只读成功作对照，不能仅靠全部请求都拒绝通过。原 TestFrozenReaderHasNoExecutionAndRejectsMutation 等继续保留，不放宽白名单。

### 4.2 普通调用必须仍等待原队列

计划新增 TestOrcaCLIOutputOrdinaryCallWaitsForQueue。对两个原构造入口的普通上下文，先用 native.SerializeCLI 在同一隔离锁键持锁，以明确 ready/release 屏障再发起有界原 client 调用。持锁期间取消/期限结束，必须沿原 CLIQueueError/context 分类返回、OutputCommand 为零，不出现伪 JSON 错误或 effect-unknown；释放并确认原持锁 goroutine 退出后，用新只读调用成功且恰一次外发作为正对照。

这验证普通调用没有误用“已持锁”捷径；不以固定 sleep 声称 ticket 已入队，不额外承诺队列公平性。计数、错误和完成以同步/锁保护；所有 release/cancel/等待退出在失败路径也执行，父测试有界，不泄漏锁或 goroutine。原 FIFO/跨进程互斥/等待取消测试继续承担其已有独有断言。

### 4.3 PreparationOwnsCLI 必须避免第二次取锁

计划新增 TestOrcaCLIOutputPreparationOwnsCLI，只对 NewLifecycle 原 client 调用真实 PreparationCall(ctx, true, checkpoint, "status", "--json")。这是当前准备查询入口，不能把 reader 的 terminal 白名单扩成 status/repo；也不能通过 PreparationCall(..., false, ...) 验证“避免重复取锁”。

原外层 SerializeCLI 确实先取得同一锁，调用原 checkpoint 后，由 PreparationCall 自身设置 marker 和执行期限，再进入原输出闭包。checkpoint 内对同一隔离锁键发起有界 SerializeCLI 竞争，要求返回 CLIQueueError 且 errors.Is(DeadlineExceeded)，竞争回调零次，以确认原外层已持锁。随后原请求必须成功，checkpoint 和 OutputCommand 各一次。host 内分别断言 PreparationOwnsCLI(ctx)=true、Command.Timeout 为 15 秒、生效截止时间等于更短的父 context 截止。若输出闭包第二次进入同一 FIFO，该正例应失败而非等待全局测试超时；必须保留能确认最终完成的回执，不以“goroutine 已启动”签收。

同入口补 checkpoint 返回原错误的拒绝子例：errors.Is 保留原 checkpoint 错误，仍为原 CLIQueueError 分类、OutputCommand 零次；不伪造 marker、不修改 protocol 私有 context key、不手工调用 invoke，也不在 callback 外调用另一条假路径来生成成功。普通上下文无 marker、原普通等待/取消有 §4.2 对照；原 protocol 的串行锁/PreparationCall/CLIQueueError/dispatch 测试继续保留。以上只证明真实组件接线，不是 application 授权或真实业务派发端到端资格。

## 5. 不变量与最小验收

PC-C01–C07/PC-R01–R10 保持，重点为原操作身份、未知零重发、停止撤销、原目标和有界查询。不改变准备查询的白名单/重试预算、队列与调用预算归属，不为旧 reader 增加执行权。state/v8/user_version=8、所有实际持久字段/tag/codec、Profile/State/Record/Operation/Receipt、RuntimeFence、CLI/schema/capabilities/resource_digest、Workflow/提示词/JS 不变。源码/EXE 身份正常变化分别记录。

| ID | 必须证明 |
| --- | --- |
| OCO-01 | 回调仅两个恒真调用根，参数/实参/不可达分支删除，无新策略壳；生产只改批准两文件 |
| OCO-02 | 两个真实构造入口原 client 经隔离 host 实际成功，原 Command/结果/期限及输入不变，paired 绑定拒绝和 reader mutation 拒绝有效 |
| OCO-03 | 两入口普通调用在原锁持有时不外发并可取消，释放后原只读路径实际成功，无误绕队列 |
| OCO-04 | 当前 client 经真实 PreparationCall(true) 的 checkpoint 内同锁竞争有界失败且回调零次，原请求设置 marker 后成功，checkpoint/外发各一次，checkpoint 失败零外发；不重复取锁，不伪造 marker |
| OCO-05 | Lifecycle.nativeAccess、准备查询/锁/配对/reader/dispatch 原规则及所有既有测试/工具保持；固定保留声明/代表编码/资源摘要相同，Plan 最小集合与正式构建完成 |
| OCO-06 | 实施后 docs/acceptance/orca-cli-output-policy-retirement.md 独立记录原/新增接线、计数/期限/锁屏障、实际集合/命令/失败/skip、来源/产物身份及未验范围，再更新状态 |

最小范围为原 adapter 十一项（含 #141 worktree reuse）和 protocol 七项基线；新增三个接线顶层先在旧参数仍存在时通过，再删除生产闭包；最终 adapter 十四项、protocol 七项和正式 EXE 构建。名字/命令见 Plan，不以零匹配、旧 Golden 或单独 protocol 队列测试替代新接线。实际变更超出批准两文件/新增测试，先修订范围并按影响补验。

实施验收见[验收](../acceptance/orca-cli-output-policy-retirement.md)。批准变更只收缩恒真策略，最小集合直接举证其可达普通/准备分支；不默认扩大为公开受控、完整 adapter/protocol/application、L1/Controlled、L2–L4、真实 Orca/模型/PR、Node 或全恢复。OCO-01–06 完成即结束，不自动开展其它精简、提交/推送/PR/合并/部署。

## 6. 交付时主线复核

2026-11-10，文档完成前再次刷新 main 至 `bda3bc8ca54a01975420ef71acb8a59748edd904`（#140）。相对调查基线，新增改动仅为 L3 显式复用完整前置的脚本/框架测试、测试指南和独立验收；本批两个构造入口、输出闭包、协议队列/marker/dispatch 及所有计划测试声明未变。已按最新测试指南复核本批独立 Go 定向和正式构建入口，最小范围与未取得完整资格的边界保持，不复用或继承上游测试结果。本地 HEAD `11667669` 与原调查基线文件树相同，不表示与新增 #140 的整棵文件树相同；实施前按 P0 重新核对，结果见下一段。

2026-11-11，实施前刷新 main 至 `c404d6e5c50cc32650f9412f9b8b3449c5789e15`（#141）。独立子代理确认删除和保留清单成立，核对了两个构造入口的合法 profile 与配对装配。审计后登记 #141 下游消费者及其定向测试，并补入 checkpoint 同锁竞争与命令预算、父截止的独立断言。编码和资源基线取自该提交。

交付前刷新 main 仍为上述 #141 提交。原 adapter 十一项和 protocol 七项基线、旧生产下三项新接线复跑、最终 adapter 十四项和 protocol 七项及正式构建通过，详见[验收](../acceptance/orca-cli-output-policy-retirement.md)。
