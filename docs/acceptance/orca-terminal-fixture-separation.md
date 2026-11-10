# Orca 旧终端测试与当前执行器分离验收

2026-11-10，合并前定向验收。调查/实施主线 `2f6e623efdc414b9d7a601c0c91c6bcf186c60b6`；实施开始刷新后，工作副本 HEAD `c69493a1e65d9792326c448c898cec95c5ee9d42` 与该主线文件树相同。依据 [Spec](../spec/orca-terminal-fixture-separation.spec.md) / [Plan](../spec/orca-terminal-fixture-separation.plan.md)，按用户“独立子代理审计、修正确认问题，然后实施”授权完成。本记录冻结时尚未提交、推送、创建新 PR、合并或部署；用户随后授权提交并创建 PR。

## 独立审计与确认修订

独立只读子代理确认一项 P2：原设计只有 ShutdownLifecycle 的旧快照拒绝，没有同入口的成功控制；现有 close 正例实际覆盖 StopLifecycle/VerifyStopped/Reconcile，不能排除 ShutdownLifecycle 对任何快照均报错。

已同步 Spec §4.3/OTF-04 与 Plan P1：同一真实构造端口先取得原测试 agent 的 acknowledged-close，经 ApplySessionReceipt 合入合法当前快照，ShutdownLifecycle 必须成功；随后对原旧快照必须因精确绑定错误拒绝，不能以缺回执错误替代。原一次 close 单列计数，两次 Shutdown 不再触发 close/history/CLI，快照与 setup 不变。子代理只读复核确认 P2 关闭。

另澄清真实构造只约束本批新迁移端口，不扩展重构既有手工恢复测试对象。实施后的独立窄复核确认上述正负配对已落地，未发现生产越界、新旁路构造或原有效断言丢失；该复核未改文件、未运行测试，不新增运行资格。

## 精确删除与保留

生产只修改 orcatransport/lifecycle.go：删除无生产调用根的 Lifecycle.original、Lifecycle.boundTerminal，以及两方法独占的 errors/fmt/strings imports 和注释，净删 **42 行**。测试侧删除 capabilityFixtureLifecycle、originalQueryFixture、terminalQueryHost；没有新增同义工厂或兼容执行器。

NewLifecycle/原解码与 clone/错误顺序、Lifecycle.frozenRecord/validate、原 scope/close 判据、probeProcess/profileVersion/nativeAccess、inspectionGate、SessionID、历史 reader 和其格式、终端/creation/files/usage 能力均保持。其余生产文件原字节未改；preservePendingStage、无引用类型、JS/Node 和诊断工具未进入本批。

收益是当前执行器少两项由旧测试维持的方法，三处测试各自消费真实职责。测试增加身份反例和停止成功控制，不宣称全仓净删、任务提速或 EXE 变小。

## 三处消费者迁移及基线

| 消费者 | 最终入口和保留覆盖 |
| --- | --- |
| TestOriginalTerminalConcurrency | 直接 QueryBoundTerminal + 原隔离 terminalQueryIPC，使用 Terminal/TerminalIdentity，不构造 Lifecycle 或旧 snapshot/state。保留并发 1/5/10/20、三轮×两读、20ms 服务延迟、15s 期限和样本完整性；每组实际 RPC 回执为 6/30/60/120，新增精确计数断言 |
| TestOriginalTerminalQueryIdentity（新增） | 同一隔离 pipe 的合法样本先成功，再分别返回错 runtime envelope、handle、incarnation；均按 ErrStateIdentityChanged 拒绝，每例只读一个 terminal.show 回执。协议生产路径未改，没有 CLI fallback 或复制 workspace/agent 策略 |
| TestOrcaCLICommandEncodingAndConsumers | fixtureSetup/Profile → 完整合法 AgentSessionProfile → 真实 NewLifecycle，再注入原隔离 client/output seam。local version=1，paired close version=2/规范固定 PairingHostID；隔离坏 pairing store 确认不读用户环境。保留 CheckpointDispatch、cliLaunchCommand、两端原 Golden/哈希、恰两次实际调用、owner plan 和 literal Command 写回重读 |
| TestCodexStoppedRequiresNativeExitProofBeyondPTY | 原三个历史场景仍走 frozen reader，保留 PTY 不足/缺证明/活根和原快照不变；当前端口经现有真实构造 fixture，新增同端口 Shutdown 已结算当前快照成功、旧快照按 Orca terminal execution binding changed 拒绝。原 close 恰一次，两次核验零额外 I/O |

原 TestRetiredExecutionFactoryKeepsFrozenProof、本机/paired 原 scope 矩阵、原 close 正反例及构造 facts/首错等保持。既有结构化恢复测试对象、共享 fakeHost/creationTestHost、reader encoder 与跨包 SessionID 消费者没有迁移或删除。

原四项测试在未修改测试/生产时先运行：4 个顶层、23 个执行节点通过，无失败/skip，见[原基线](evidence/orca-terminal-fixture-separation-20261110/original-baseline.jsonl.gz)。迁移首轮因新快照变量与原进程身份变量同名而编译失败，未执行测试，见[原编译失败](evidence/orca-terminal-fixture-separation-20261110/migrated-baseline.jsonl.gz)。修正测试变量名后，在生产两方法仍保留时重跑同一八项：8 个顶层、54 个执行节点全部通过，无失败/skip，见[迁移后基线](evidence/orca-terminal-fixture-separation-20261110/migrated-baseline-fixed.jsonl.gz)。随后才删除生产旧方法和无消费者的测试工厂。

编辑阶段另有一次 import 位置错误由 gofmt 发现并立即修正；取证脚本首次默认 GBK 读取含中文源码失败，改为显式 UTF-8 后核对完成。以上均为本批测试编辑/取证错误，没有修改生产校验、删除有效断言或改记失败为通过。

并发诊断现在带 scope=direct-terminal-capability；原基线通过旧 wrapper，新版直接读协议能力，测量范围不同。保留原日志和新版各项耗时，不比较为同路径加速或任务吞吐提升。旧假 CLI counter 已删，没有用未接线的计数器证明零 CLI；未改的直接 RPC 路径和 pipe 方法白名单分别保留。

## 原字节、声明和来源核对

实施前/后独立执行[一次性捕获程序](evidence/orca-terminal-fixture-separation-20261110/capture.go)，两次输出均为 **301,874 字节**，SHA256：

`4f01d1ac561e2be5d1756ea99a59b2b01b949303fce1a586d4e12f29299b1444`

捕获包含保留类型声明、导出历史 Profile/State/Turn/NativeProof 及 envelope/Record 的代表编码、结构化 profile、私有结构化 state 的原始 payload envelope 规范化、RuntimeFence 0/7 mutation envelope、静态 capabilities/schema/resource_digest。私有 state 的类型声明单独捕获；原始 payload 捕获不冒充新 decoder 或合法业务/停止证明，实际绑定与停止由组件测试验证。

[删除前](evidence/orca-terminal-fixture-separation-20261110/before-capture.json.gz)与[删除后](evidence/orca-terminal-fixture-separation-20261110/after-capture.json.gz)字节完全相同，resource_digest=`910c7fa06b933be3632f862062e966200fd581b34873a395268826d44c1bbbe7`。source/EXE 身份变化单列，不用新值回填原证据。

[范围核对](evidence/orca-terminal-fixture-separation-20261110/scope-audit.json)覆盖 808 个源码/测试/JS/脚本文件：只有批准的 lifecycle.go 与四个测试文件变化，其余 **803 个文件原字节相同**。lifecycle.go 扣除批准两方法和独占 imports 后的保留正文逐字一致；原 pipe 服务/清理 helper、CLI Golden 和 arguments 文字逐字相同。最终源码/测试/工具清单 SHA256=`b490694e1660dc5cea9fc7eb4cdf9ee8688b80bb41e0badcece23592748e35a0`。

## 实际最小验收

所有 Go 测试使用 count=1。Windows/amd64，普通宿主上下文；只操作隔离 named pipe、假 CLI/agent/owner 和本地 Git 现场，不连接真实 Orca/模型。命令见[执行清单](evidence/orca-terminal-fixture-separation-20261110/commands.txt)，逐条结果见[完整汇总](evidence/orca-terminal-fixture-separation-20261110/summary.json)。

| 范围 | 实际结果 |
| --- | --- |
| 完整 orcatransport | 83 顶层、364 执行节点通过，零失败/skip |
| 完整 protocol/orca | 62 顶层、360 执行节点通过；原 4 项现场 opt-in skip，零失败 |
| application 精确三项 | 3 顶层、7 执行节点通过，零失败/skip；实际集合与 Plan 一致 |
| live native driver、native/stability helper | 三份分别编译通过；仅编译，不登记真实模型或稳定性运行资格 |
| 正式 EXE | scripts/build.ps1 构建成功，产物与公开受控 Runner SHA 一致 |
| test-foundation 原单元前置 | 四包、5 个顶层通过，零失败/skip；两个包各自的 TestSRCLIContract 分别计数 |
| 同一 EXE 公开受控 | TestSRNativeUnknownAndStop 通过，17.57 秒；最终 exit_code=0、scenario_count=1，无 skip |

原四项跳过分别是 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestOrcaTerminalQueryLive、TestUsageLocalAcceptanceRepoCleanupLive。它们未运行，不计通过；本批隔离并发/身份测试均实际运行，不能用 opt-in skip 代替。

构建 source_commit=`c69493a1e65d9792326c448c898cec95c5ee9d42`、source_dirty=true，source_digest=`c3c35ef0170667cd600be64eb3307fd93c3cd844beb7e978a7d9b239b2b5d82f`。正式 EXE SHA256：

`c26b421907d4148368d246c64fabf241693e732451ac66e185f1bd8672dc857d`

[构建记录](evidence/orca-terminal-fixture-separation-20261110/build.json)、[受控产物身份](evidence/orca-terminal-fixture-separation-20261110/controlled-source-identity.json)、[最终回执](evidence/orca-terminal-fixture-separation-20261110/controlled-result.json)和[公开日志](evidence/orca-terminal-fixture-separation-20261110/controlled-integration.log)均保留；driver/helper SHA 在 summary 中。完整 EXE/driver/现场保存在本工作副本 `.evidence/orca-terminal-fixture-separation/`，没有替换已安装服务。

公开用例沿原通用本机链路，验证原 dispatch 一次、unknown 零重放和停止后零新输入。已等待最终 result.json/integration.log，不以启动脚本返回代替完成；该用例不证明真实 Orca/paired/Agent Session 端到端资格。

## 结束与资格

OTF-01–06 完成。仅取得本次合并前的组件、应用定向、共享消费者编译和一个通用公开受控路径资格；未运行完整 L1/Controlled、L2–L4、完整恢复矩阵、真实 Orca/模型/PR、Node 或 probe，不扩大既有资格。

独立验收和原证据按 .gitattributes 保持原字节，见 [SHA 清单](evidence/orca-terminal-fixture-separation-20261110/SHA256SUMS.json)和[最终一致性核对](evidence/orca-terminal-fixture-separation-20261110/final-verification.json)；Spec/Plan 与长期实施状态已同步。本批不开展其它精简候选，不自动提交、推送、创建 PR、合并或部署。

## PR 交付补记

2026-11-10，用户在上述验收完成后授权创建 PR。交付分支从刷新后的 main 建立，只包含本批改动；源码/测试与已验证清单逐项核对相同，因此沿用原定向结果，不因分支或提交身份变化重跑。原构建的 source_commit、source_dirty 和最终一致性记录均保留其捕获时的事实，不改写为提交后构建或合并后资格。
