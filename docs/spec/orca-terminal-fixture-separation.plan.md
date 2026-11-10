# Plan：Orca 旧终端测试与当前执行器分离

日期：2026-11-10。状态：独立设计审计确认的一项 P2 已修正并复核关闭，按用户后续授权实施完成，合并前最小验收通过，见[独立记录](../acceptance/orca-terminal-fixture-separation.md)。依据 [Spec](orca-terminal-fixture-separation.spec.md)，调查/实施主线 `2f6e623efdc414b9d7a601c0c91c6bcf186c60b6`；开始实施时工作副本 HEAD `c69493a1` 文件树与主线相同。原设计/审计阶段仅文档，后续用户已明确授权修正并实施；验收冻结时尚未授权提交/PR/合并/部署；用户随后另行授权提交并创建 PR，未授权合并或部署。

## P0：基线、入口及原断言捕获

- [x] 实施开始刷新主线，记录目标/HEAD/源码与工作树差异；有实际上游变化先核对。按 AGENTS.md 处理真实内容冲突，不因 Rebase 提示重写已验证历史。
- [x] 全仓复核 Lifecycle.original/boundTerminal、capabilityFixtureLifecycle、originalQueryFixture/terminalQueryHost 的调用和函数值引用；对照三处消费者与 Spec 保留表。确认 SessionID 跨包调用、inspectionGate 当前审计调用、creation/终端/reader 实际入口；不顺带做其它候选。
- [x] 保存三处原测试的名字、全部子例/断言、隔离 pipe 服务/取消清理、CLI 两端 Golden 与哈希/owner 读取，以及原 production factory/历史 scope/close 正反例。特别登记 TestOriginalTerminalConcurrency 原先测了旧 wrapper，后续改为直接能力，不能将新结果写作原路径的性能比较。
- [x] 捕获保留生产声明、历史/结构化 profile/state/stop/RuntimeFence 代表编码、静态 capabilities/resource_digest 和共享测试/driver/helpers/JS/脚本摘要；原序列化或读取错误不忽略，不以迁移后值回填原证据。只复用或编写一次性证据工具，不新增生产审计框架。
- [x] 在 Windows/amd64 先运行原四项定向，保存原输出/退出/skip，记录基线失败而不改记通过：

  ```powershell
  go test ./internal/execution/adapters/orcatransport -run '^(TestOriginalTerminalConcurrency|TestOrcaCLICommandEncodingAndConsumers|TestCodexStoppedRequiresNativeExitProofBeyondPTY|TestRetiredExecutionFactoryKeepsFrozenProof)$' -count=1 -timeout=5m -json
  ```

## P1：迁移三处消费者，先保留生产旧方法

1. terminal_query_windows_test.go：用原 native.Terminal/TerminalIdentity 和隔离 terminalQueryIPC 直接调用 QueryBoundTerminal；保留固定并发/轮次/次数/延迟/期限、同步及全部样本/回执校验和诊断输出。删除本测试对 Lifecycle、旧 snapshot/state 的装配依赖，再删无消费者的 originalQueryFixture/terminalQueryHost。IPC 服务、随机 pipe、隔离环境与 Close/worker 清理原样保留；不引入假的零 CLI counter。
2. 在同一真实 pipe 上补 TestOriginalTerminalQueryIdentity（已新增）：合法样本正例，再测错 runtime envelope、handle、incarnation；要求明确身份拒绝，无 CLI fallback。不复制旧 workspace/agent/初始化规则，不改协议生产实现；保留协议包原缺身份/取消/冲突优先级测试。
3. cli_command_test.go：用 fixtureSetup/fixtureProfile 在 NewLifecycle 前装配完整当前结构化 profile/target。Transport 中提前放入原测试 EXE/前缀 Args；local version=1，paired 当前 close version=2/固定规范 PairingHostID。经真实 NewLifecycle 后再注入现有隔离 client/output，调用原 cliLaunchCommand/CheckpointDispatch/creationTestHost；普通单元测试不调用原远端 pairing 查询，不启动真实 Orca CLI。owner launch callback 继续原有限 serveCreationOwner 测试路径。禁止把 mutation 交给只读 reader 或修改生产白名单。
4. CLI 两端保留原完整 LaunchCommand Golden/哈希、两个实际调用、原 runtime/receipt、owner plan 读取及 literal Command 写回重读；输入/冻结配置不变、构造零外部调用。不改 Golden 迁就路由/字段遗漏，不比较动态 Deadline/EXE SHA 的整 plan 跨现场字节。
5. verify_stopped_test.go：三个原历史场景仍用 frozen reader 验 PTY 不足/缺证明/活根及旧 snapshot 不变。当前端口经真实 NewLifecycle 构造后，先沿 StopLifecycle 获取隔离 agent 的 acknowledged-close 原回执并以 ApplySessionReceipt 合入当前 snapshot；同一端口的 ShutdownLifecycle 对该已结算 snapshot 必须成功且 Stopped=true，再对原旧 snapshot 返回绑定/协议错误 Orca terminal execution binding changed、Stopped=false。原一次测试 close 单独计数；两个 ShutdownLifecycle 均不得增加 close/history/CLI 调用，两个 snapshot 和当前装配不变，不能以缺回执错误代替绑定拒绝。旧协议工厂 unsupported 和只读原 scope 仍由 TestRetiredExecutionFactoryKeepsFrozenProof 保留；原结构化 close 正反例保持且不充当上述成功路径的替代。仅本批迁移端口要求真实构造，不扩大修改其它既有恢复测试。
6. 先在生产仍含两项旧方法的基线上运行迁移后测试，确认以下五个顶层都实际执行：上面原四项，加 TestOriginalTerminalQueryIdentity。现有普通 structured fixture/constructor 错误顺序、隔离坏 pairing store 回归也要通过。保存结果；若需改名同步清单，零匹配不能判通过。发现基线问题先核对装配，不放宽产品校验。

   ```powershell
   go test ./internal/execution/adapters/orcatransport -run '^(TestOriginalTerminalConcurrency|TestOriginalTerminalQueryIdentity|TestOrcaCLICommandEncodingAndConsumers|TestCodexStoppedRequiresNativeExitProofBeyondPTY|TestRetiredExecutionFactoryKeepsFrozenProof|TestStructuredConstructorFacts|TestStructuredConstructorFaultPriority|TestCurrentFixtureUsesIsolatedHostWithInvalidPairingStore)$' -count=1 -timeout=5m -json
   ```

   这是迁移后的八项基线集合；新增身份测试与原七项均已实际匹配。首次因测试变量同名编译失败、零测试执行；修正后在生产两项旧方法仍保留时，八项顶层、54 个执行节点全部通过。原输出与失败在独立验收中分别保留。

## P2：删除批准闭包与静态复核

- [x] 三处调用迁移后删除 capabilityFixtureLifecycle，以及 frozen_fixture_test.go 中由它独占的 native/time imports；保留该文件的 envelope/frozenWorkspaceFixture 等真实历史夹具，保留 fixtureSetup/fixtureProfile/structuredFixtureOn 和共享 fakeHost/creationTestHost。
- [x] 删除 Lifecycle.original、Lifecycle.boundTerminal 及其独占 errors/fmt/strings imports/注释；除这两个方法和独占内容外，生产文件正文不变。不得删除 SessionID、Lifecycle.validate/frozenRecord、probeProcess/profileVersion/nativeAccess、inspectionGate 或任一 reader/stop/creation/协议能力。
- [x] 对照全仓引用和声明差异：生产无新入口或兼容壳，测试无旧方法调用和旧旁路工厂；不把同一手工 Lifecycle 装配改名藏到别的 helper。本批三处迁移中新建的当前端口必须经 NewLifecycle，直接原终端查询不再构造执行器；其它既有结构化恢复测试对象保持。
- [x] 对照 P0 原编码/摘要与旧 CLI Golden。保留生产 types/tag/codec/实际读写、原固定 scope/close 判据及构造首错原字节；测试 profile 从 legacy 迁为 structured 明确登记，不强求两种 profile 相同，不允许顺改生产 decoder/白名单。

## P3：最小实施验证

以下命令已按用户后续实施授权，在仓库根目录 PowerShell 7、Windows/amd64 完成；输出写入本批新目录。真实命令、退出码、测试节点及原失败/skip 见独立验收与原证据，不将原现场 opt-in 跳过记成通过。

1. 完整受影响组件：

   ```powershell
   go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1 -timeout=5m -json
   ```

   核对三处迁移测试、新直接身份测试、原工厂/历史证明/close、构造 facts/首错、坏 pairing store 隔离、CLI/creation、files/usage 均在实际集合；完整协议包保留缺身份零连接、握手取消/期限和取消不掩盖冲突。Windows 并发/pipe 测试不能因平台未匹配而静默遗漏，不重写旧性能报告。

2. application 精确三项现有合同回归：

   ```powershell
   go test ./internal/application -run '^(TestStopContractOnlineAndOfflineKeepScopeAndDecoderOrder|TestStopContractKeepsWorkflowAndSavedScopeRules|TestStructuredDevelopmentStopBeforeResourceKeepsAdmissionOrder)$' -count=1 -timeout=10m -json
   ```

   逐项确认三项顶层实际执行、非零匹配、原子例/错误顺序/输入不变断言保持。这不是完整 application 或完整恢复资格；其它包未改且无新影响时不自动扩大成 host/storage/contracts 全组。

3. 编译共享实际消费者，确保测试迁移没有连带移除其依赖：

   ```powershell
   New-Item -ItemType Directory -Force .evidence/orca-terminal-fixture-separation/compile | Out-Null
   go test -tags=live -c -o .evidence/orca-terminal-fixture-separation/compile/native.test.exe ./tests/native
   go build -o .evidence/orca-terminal-fixture-separation/compile/native-fixture.exe ./tests/helpers/native
   go build -o .evidence/orca-terminal-fixture-separation/compile/stability-helper.exe ./tests/helpers/stability
   ```

   每条独立检查退出码，不让后续命令成功掩盖前项失败；只编译，不执行真实模型、稳定性场景或登录/配对工具，不把编译记为现场资格。

4. 正式构建与一项通用公开受控：

   ```powershell
   ./scripts/build.ps1 -OutFile .evidence/orca-terminal-fixture-separation/final/steerrunner.exe
   ./scripts/test-foundation.ps1 -RunnerExe .evidence/orca-terminal-fixture-separation/final/steerrunner.exe -Pattern '^TestSRNativeUnknownAndStop$' -OutDir .evidence/orca-terminal-fixture-separation/controlled-new
   ```

   冻结生产/测试点再构建，RunnerExe 固定为同一最终文件，保留脚本原有单元前置、EXE/受控 peer、超时、唯一匹配和清理。等待原宿主的最终 result.json/integration.log，核对 exit_code=0、scenario_count=1、原 dispatch 一次、unknown 零重放和 stop 后零新输入；脚本启动返回不代表通过。

   明确这是原通用本机受控链路，不能称为真实 Orca/paired/结构化会话端到端通过。Orca 的当前/历史合同在本批实际由组件、隔离 named pipe、假 CLI/owner 和应用定向分别举证。记录 EXE、driver/helper 与 source/test 身份，区分源码/EXE 正常变化与固定资源契约不变。

上述最小范围不授予完整 L1/Controlled、L2–L4、完整恢复、Node 或真实模型/Orca 资格。若生产 diff 或实际依赖变化触及共享 creation/连接/派发/停止/恢复，先修订 Spec/Plan 并按影响补选，不使用最小集合掩盖新增影响。通过项仅在有新改动、失败或未解决问题时重跑。

## P4：独立验收与结束

- [x] OTF-01–06 在 docs/acceptance/orca-terminal-fixture-separation.md 单独登记：两个方法及旧工厂删除、所有保留项、三处新入口、原断言/Golden/编码对照、直接能力测量范围、实际测试集合/命令/产物/SHA、原失败/skip 和未验项。证据按仓库既有 .gitattributes 规则保留原字节，不回填原构建/历史报告。
- [x] 核对生产 diff 没有静默删除历史格式/真正终端能力，测试迁移没有减少原有效覆盖，也没有再造绕过 NewLifecycle 的执行对象。真正错误和装配/环境失败分别记录；普通宿主重跑不抹去沙箱失败。
- [x] 实际验收完成后才更新 Spec/Plan 状态及索引，必要的长期实施说明明确本项收口旧例外而非原合同失效；不向中央 README 验收表头插行，不主动开展其它精简候选或全目录整理。
- [x] 原设计轮只交付文档；用户随后明确授权独立审计、修正确认问题并实施。本轮 OTF-01–06 已完成，原失败/skip 和最小资格分别登记，不自动提交/推送/PR/合并/部署。若另获 PR 授权，按 AGENTS.md 在创建和最终交付前刷新目标、核对 GitHub 实际内容合并状态，优先 Squash and merge，不为 Rebase 提示改写已验证历史。
