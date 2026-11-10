# Spec：Orca 旧终端测试与当前执行器分离

日期：2026-11-10。状态：独立设计审计确认问题修正后已实施，合并前定向验收完成，见[独立记录](../acceptance/orca-terminal-fixture-separation.md)。原设计阶段仅文档的范围不改写。调查/实施主线 `2f6e623efdc414b9d7a601c0c91c6bcf186c60b6`（#134 已合入）；开始实施时工作副本 HEAD `c69493a1` 与该主线文件树相同。配套 [Plan](orca-terminal-fixture-separation.plan.md)。依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[Orca 能力层](orca-capability-layer.spec.md)、[现行执行与停止合同](../orca-execution.md)及[测试指南](../testing.md)。

## 1. 目标与范围

合法生产入口 NewLifecycle 已拒绝旧终端协议，只构造结构化 Agent Session 执行器。旧终端并发测试仍调用 Lifecycle.original/boundTerminal；另两个 CLI/creation、停止测试与它共用 capabilityFixtureLifecycle，绕过 NewLifecycle 手工装配可执行对象形状。它们不是当前生产执行入口，却维持了两项旧生产方法和一条额外测试构造路径。

本项把三处测试分别接回终端能力、经真实构造的当前执行器和历史 reader，删除测试维持的两个旧方法与旧测试工厂。收益是减少当前执行功能开发时需要调查的旧终端方法和旁路装配，不以搬文件/改名签收，不承诺任务提速、EXE 缩小或全仓净删。

本项只收口[历史读取/夹具专项](../archive/2026-11/implementation/spec/orca-reader-fixture-boundary.spec.md)当时有意保留的 original/boundTerminal 与专用测试装配例外。原构建的保留结论、通过/失败与证据不改写；历史读取、原冻结停止 scope 和当前结构化执行合同不退役。

## 2. 实际消费者与精确删除表

| 对象 | 基线实际消费者/合法入口 | 本项处理 |
| --- | --- | --- |
| Lifecycle.original | 仅 terminal_query_windows_test.go 的 TestOriginalTerminalConcurrency | 迁移该消费者后删除方法；不复制旧 workspace/agent/初始化策略到能力层或测试专用 executor |
| Lifecycle.boundTerminal | 仅上述 original | 随调用闭包删除；QueryBoundTerminal 和 InspectBoundTerminal 本身保留 |
| capabilityFixtureLifecycle | cli_command_test.go、terminal_query_windows_test.go、verify_stopped_test.go 各一处；直接构造 Lifecycle | 三处迁移后删除测试工厂及其独占 imports，不新增同义旁路工厂 |
| originalQueryFixture、terminalQueryHost | 仅旧并发测试的 Lifecycle/假 CLI 装配 | 改为原生 Terminal/TerminalIdentity 与隔离 RPC 装配后删除旧桥接；实际 IPC 服务/取消清理保留 |
| lifecycle.go 的 errors/fmt/strings imports 及两方法独占注释 | 仅服务上述旧方法 | 无其它新消费者时删除；其它生产正文保持 |

删除前复核全仓生产、测试、脚本、工具与函数值引用，不只查直接调用；历史 evidence 的符号文本不当作运行消费者，也不改写。发现新增真实生产调用根时先修订范围，不能删调用者制造零引用。允许的生产差异限于两个方法及独占 imports/注释删除，不新增生产函数、字段、接口、回调或配置入口。

## 3. 明确保留清单

| 对象 | 必须保留的原因与边界 |
| --- | --- |
| NewLifecycle、decodeFrozenRecord、decodeAgentSessionProfile、decodeLifecycleProfile | 当前构造与历史证明共用原解码；原协议拒绝、完整 profile/target/host/development/audited、clone、错误类型及首错顺序保持；本批不解耦整个 frozenRecord |
| Lifecycle.frozenRecord/validate/structured、closeSettlesAgentSession/verifyAgentSessionStopped、StopEvidenceScope | 当前审计/停止与原冻结合同有消费者；不能因这批删除 original 就连带删转发或复制停止判据 |
| Lifecycle.probeProcess/profileVersion/nativeAccess、setup/profile/client、inspectionGate | 仍属原共享读取/测试 seam 或当前执行装配；inspectionGate 被结构化工作区审计使用，不按旧文件名清理字段/锁 |
| frozenRecord 全部实际校验、State/Turn/NativeProof、Profile/TerminalConfig/owner/receipt/control 及 codecs | 原历史 decoder、usage 和原 scope 只读证明仍有消费者；原缺失/错身份/错摘要/缺退出证据继续拒绝，不引入历史执行器或补证逻辑 |
| QueryBoundTerminal、InspectBoundTerminal、CloseTerminalChecked、完整清单核验、CloseReadonlyTerminal | 原生终端能力、结构化停止的附加 shell 清理、历史停止、诊断和测试仍使用；不扩大成主机级清场 |
| SessionID | frozenWorkspaceFixture 与 application/shared_native_stop_test.go 的跨包历史身份装配仍使用；本批保留函数及原前缀/算法，不夹带迁移 |
| fixtureSetup/fixtureProfile、structuredFixtureOn、frozenFixtureOn、frozenWorkspaceFixture、测试侧 envelope、fakeHost/creationTestHost/原请求 helper | 当前/历史/creation 多个消费者共用；只移除批准工厂及并发旧桥接，不能整删共享夹具文件 |
| orcaCLIOutput、cliLaunchCommand、creationKey/creationCall、creation owner/plan/result/started | 当前工作区创建、冻结 CLI、首次派发与原回执对账仍使用；串行锁、dispatch checkpoint、原预算和零重放保持 |
| 原始查询/订阅、files/usage、JS/Node、诊断脚本、native/stability driver/helpers | 独立能力和工具仍有合法消费者；不因测试改接底层能力而退役或改装配 |

preservePendingStage、controlCheckpoint、initialTerminalCheckpoint、publication.Error/providerError 等其它候选不在本批；不做源码目录重组、通用 fixture 框架、错误体系统一或字段清尾。

## 4. 三处测试消费者迁移

### 4.1 终端并发：直接消费终端查询能力

TestOriginalTerminalConcurrency 保留在原 Windows 测试入口，可继续沿用原名；其职责改为明确的原终端 RPC 能力并发，不再装配 Lifecycle、旧 State 或 resource snapshot。使用原 native.Terminal、TerminalIdentity 和 terminalQueryIPC 服务，直接调用 native.QueryBoundTerminal。生产能力代码及 runtime/handle/incarnation 校验、父 context 的取消/期限和已观察身份冲突优先级不改。

保留并发 1/5/10/20、三轮、每轮两次读取、每次 20ms 服务延迟、15 秒调用上限、同步启动、全部调用/样本完成和错误检查；每组 terminal.show 回执数量应为 concurrency×3×2，不能仅等待 goroutine 结束。保留 p95/吞吐/总耗时为诊断，明确新旧测量路径不同，不据此声称端到端或同路径提速。

IPC 继续只允许原 status.get 握手和 terminal.show，核对原 auth token、runtime/terminal 及实际返回身份，不接用户 Orca。保留原随机 pipe、ORCA_USER_DATA_PATH/LOCALAPPDATA 隔离、listener/连接/worker 清理与取消响应；不能用全局 sleep 或永久后台进程绕过清理。删除不再接线的假 CLI counter，不用一个永远不被调用的 fakeHost 冒充零 CLI 证明；真实路径仍由未改的 QueryBoundTerminal→原连接/RPC 实现和服务方法白名单确认。

补一个最小直接能力身份测试（已新增 TestOriginalTerminalQueryIdentity）：合法样本先成功，再分别返回错 runtime envelope、handle、incarnation，明确拒绝且不退回 CLI；使用同一隔离 pipe。原协议包的缺身份零连接、握手取消/期限和取消不能掩盖已知身份冲突测试继续通过。workspace/branch/agent 的执行策略不由 TerminalIdentity 承担，不把已删除旧 wrapper 的策略复制成新的能力规则；当前 originalAgentWorkspace、结构化状态绑定和历史 reader 的原核验保持。

### 4.2 CLI 编码与 creation：先真实构造，再注入隔离传输

TestOrcaCLICommandEncodingAndConsumers 不再从 frozenFixtureOn 和 capabilityFixtureLifecycle 起步。由中性 fixtureSetup/fixtureProfile 装配完整 AgentSessionProfile，经真实 NewLifecycle 成功构造：Ref 使用 ProtocolOrcaAgentSessionV1、合法 full_access、Transport 与 target 一致；本机 execute 使用原 version=1，paired 使用当前 close profile version=2 并合成规范 PairingHostID，不从用户配对文件取值。原测试 executable/前缀 Args 在构造前写入 Transport，不能构造后只改 l.profile 破坏冻结一致性。

构造后仅通过现有测试 seam 替换 CLI client/output，沿未改的 cliLaunchCommand 与 creationTestHost 记录命令；保留 CheckpointDispatch。真实 orcaCLIOutput 在结构化远端调用时会核对 UsageHostID，因此普通编码测试必须使用隔离传输，不能错误触发真实配对查询，也不能为此修改生产 pairing 校验。creation owner 的测试 launch callback/有限执行继续沿原 host；不把 terminal create 送给只读 frozen reader，更不能放宽其 mutation 白名单。

本机与 paired 两分支均保留前缀参数、带空格路径、中文/引号正文、末尾 environment 路由、WorkingDirectory、EnvironmentRefs=null、BlockedEnvironment 顺序、40s Timeout 的原 LaunchCommand Golden JSON 与 digest；普通 CLI 与独立 creation owner 两个实际调用都必须等于 Golden，仍恰为两次记录。保留原 runtime 回执、owner plan 严格读取、原 literal Command 写回后可读的断言。构造本身零外部调用，输入及冻结配置不被就地改写；使用新的合法 profile/envelope，而不是忽略 codec/digest/target 的无效装配。

这是共享 CLI/creation helper 的组件验证，不表示当前新 Run 会使用旧 terminal Agent 执行链。原完整 creationPlan 含动态路径、Deadline 和 EXE SHA，不要求跨测试现场整文件字节相同；固定比较原 Command JSON/哈希及原回执约束，不能通过重写 Golden 掩盖变化。

### 4.3 停止拒绝：历史证明与当前端口各走合法入口

TestCodexStoppedRequiresNativeExitProofBeyondPTY 的 live-root/missing-root/no-readiness 三个历史场景仍使用 frozenFixture/frozenWorkspaceFixture 和原 frozenRecord.verifyStopped，保留完整原清单、PTY 缺席不足以证明 helper/root 退出的错误、活根仍匹配以及旧 snapshot 前后字节不变的断言。

其中当前端口拒绝旧快照的断言，改为先用真实 NewLifecycle 的合法结构化 fixture 构造当前端口，通过原 StopLifecycle 取得隔离 agent 的 acknowledged-close 回执并沿 ApplySessionReceipt 合入当前 snapshot，再对该合法已结算 snapshot 调用 ShutdownLifecycle，要求 err=nil、facts.Stopped=true。随后用同一个端口接收原旧协议 snapshot，要求绑定/协议拒绝（原校验的 Orca terminal execution binding changed）、facts.Stopped=false，而不是仅因当前会话未停止而报缺回执错误。两次 ShutdownLifecycle 都不增加 close/history/CLI 等调用，旧快照、合法已结算快照和当前装配均不变；原一次测试 close 的发生单独计数。不能通过手工 Lifecycle 字面量、旧 profile 偷换 protocol、给旧状态伪造结构化 close 回执或新增兼容构造器完成迁移。

原 TestRetiredExecutionFactoryKeepsFrozenProof 继续独立验证 NewLifecycle(legacy) 在 I/O 前明确 unsupported，同时 StopEvidenceScope/VerifyStopped 仍按原历史 scope 只读核实，保留本机/paired、absent/live/incomplete/foreign-runtime/missing-host 矩阵。原 TestStructuredCloseAcknowledgementSettlesOnlySessionScope 等结构化停止测试保持其 StopLifecycle/VerifyStopped/Reconcile 及缺/未知回执覆盖；它们不能代替上述同端口 ShutdownLifecycle 的正负配对。真实构造要求针对本批迁移端口，不扩展成改写所有既有结构化恢复测试对象。

## 5. 格式与产品不变量

state/v8、user_version=8、Record/Request/Operation/Receipt、Profile/State/Turn/NativeProof/结构化状态/RuntimeFence、所有实际字段/tag/codec/持久读写和历史严格解码保持。新 Run 仍拒绝旧终端执行，旧记录由原版本排空；当前只读证明不重释 scope，不生成旧状态、不用当前资产/进程替原身份补证。

PC-C01–C07、PC-R01–R10、原受理/提交、结果来源、授权撤销、unknown 原操作核实且零重放、预算、同构建恢复、可选观测和原 close 停止合同不变。生产 CLI/schema、静态 capabilities、resource_digest、Workflow/提示词/JS 资产不变；source_digest 与新 EXE 哈希随源码变化，来源身份分别记录，不要求含实际 EXE 身份的响应整体字节相同。

三处测试接线与批准桥接删除可以变化；其它有效断言及共享消费者保持。构造与历史格式的代表编码在实施前后独立捕获，用固定身份核对原字节；测试从 legacy profile 改为合法 structured profile 是明确迁移，不要求二者配置字节相同，更不能据此修改生产 codec。旧错误/身份/取消和停止拒绝边界保持，不新增强制门槛。

## 6. 最小验收与结束条件

以下是实施验收要求；实际完成、原失败/skip 和未验范围见[独立验收](../acceptance/orca-terminal-fixture-separation.md)。入口及资格按 docs/testing.md，不把要求本身当作通过事实。

| ID | 必须证明的结果 |
| --- | --- |
| OTF-01 | 两项生产方法和旧测试工厂删除；三处消费者均有明确新入口；无新旁路 Lifecycle 工厂/兼容 executor；SessionID、审计锁及其它保留项成立 |
| OTF-02 | 并发固定工作量及样本/回执完整；真实 RPC 身份正反例/取消保持，未复制旧执行策略；测量范围变化如实表达 |
| OTF-03 | 本机/paired 的真实构造和隔离传输有效；原 CLI Golden/哈希、两次调用、owner 原回执和 literal Command 重读保持，不触及用户配对/真实命令 |
| OTF-04 | 历史 PTY 不足/活根/原 scope 负例、同一合法端口 ShutdownLifecycle 对已确认关闭的当前 snapshot 成功而对旧 snapshot 因绑定/协议拒绝、生产工厂拒绝旧协议及原 close 正反例通过；原输入不变，ShutdownLifecycle 无额外副作用或伪回执 |
| OTF-05 | 生产仅批准闭包删除，保留声明和代表编码、静态能力/资源摘要相同；完整 adapter/protocol、应用三项定向、driver/helpers 编译与正式 EXE 的一项公开受控通过 |
| OTF-06 | docs/acceptance/orca-terminal-fixture-separation.md 独立记录删除/保留、三处迁移、原/新测量边界、实际命令/集合/失败/skip、源码和产物身份及未验范围；实施后才更新状态 |

最小运行范围见 Plan：完整 orcatransport 与 protocol/orca；application 三项现有停止合同定向；编译 live native driver 与两个原 helper；正式构建后以同一 EXE 运行 TestSRNativeUnknownAndStop。该公开受控用例沿原通用本机链路，只补充原操作零重放/停止撤销检查，不代表真实 Orca 或 paired/Agent Session 端到端资格；Orca 组件、pipe 和假 CLI 的实际覆盖分别登记。

不默认运行完整 L1/Controlled、L2–L4、真实模型/PR、真实 Orca、Node/probe 或全恢复矩阵。实际 diff 若超出批准闭包或影响公共连接、creation、派发、停止或恢复，先修订范围并按影响补验，不能只靠最小集合继续签收。OTF-01–06 完成即结束，不进入阶段恢复规则/无引用清尾等后续批次，不自动提交、推送、PR、合并或部署。

## 7. 独立设计审计修订

2026-11-10，独立只读子代理确认一项 P2：原设计仅给 ShutdownLifecycle 旧快照拒绝配上其它入口的 close 正例，不能排除 ShutdownLifecycle 无条件报错。§4.3/OTF-04 和 Plan P1 已同步加入同一合法端口的已结算当前快照成功、旧快照因精确绑定错误拒绝，并验证零额外 I/O/输入不变；子代理复核确认关闭。

其余删除/保留、三处迁移和最小验收未确认需修订问题。另澄清真实构造要求限于本批新迁移端口，既有结构化恢复测试对象不纳入全局重构。设计审计阶段只读、未跑产品测试；随后实施按新增授权推进，实际结果另记。

## 8. 实施状态

2026-11-10，OTF-01–06 已完成。生产仅删除两项旧方法及独占 imports，共 42 行；三处测试消费者已按各自真实入口迁移，同入口 Shutdown 正负控制已落地。完整 adapter/protocol 共 145 个顶层通过、4 个原现场 opt-in skip，application 三项通过，共享 driver/helpers 分别编译通过；同一正式 EXE 的一项通用公开受控通过。原四项基线、迁移首轮编译失败和修正后八项基线分别保留。保留生产声明与代表编码捕获前后字节相同。

实施后独立窄复核未发现新问题；详见[验收与证据](../acceptance/orca-terminal-fixture-separation.md)。结果属于合并前最小范围，不授予完整测试或真实 Orca/paired/模型端到端资格；验收冻结时尚未提交、推送、创建新 PR、合并或部署；用户随后授权创建 PR，见验收交付补记。
