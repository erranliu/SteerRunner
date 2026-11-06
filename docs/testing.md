# SteerRunner L1–L4 测试指南

2026-11-06 当前结构化真机接线：已注册的 Orca feedback、smoke/multirun 与 native-development/native-active-stop 使用原结构化 session、完整历史和 `orca_agent_session` close 证明，不再要求旧 helper/readiness/Job。本机测试侧 IPC 转发记录实际 create/send/close 并注入原回执丢失；远端保留原配对 host key，以完整历史和原操作核对业务输入，不冒充重复 wire 次数证明。close 后不强求宿主继续提供历史；以已取得历史、原输入集、撤销和原 close 回执结算。本机 Orca audited 产品组合尚无注册真机场景，execution-stability 的旧终端故障夹具也未随本批迁完，不能继承当前结构化资格。实现及实际验证范围见[独立验收](acceptance/orca-structured-qualification-proof-cleanup.md)。

2026-11-05：旧 Orca TUI/helper 输入专项及执行探针已退役，原 `test-native-input.ps1` 明确拒绝，不再作为当前资格入口；下文旧构建记录不变。当前结构化执行、共享创建与历史只读证明的保留范围及实际验证见[验收](acceptance/orca-reader-execution-boundary.md)。

2026-11-05 配置边界增量新增 Controlled `TestSRExecutionConfigurationPublicCLI`，完整集合为29个顶层测试；三操作、严格输入/来源、无服务副作用和原生目录可信execution上下文见[专项](acceptance/native-execution-configuration-boundary.md)。后文旧日期数量只代表其原版本。

2026-11-05 原生目录增量加入 Controlled 的 `TestSRNativeCatalogPublicCLI`，当前共 28 个顶层测试。该项使用固定公开 EXE 核对无服务入口、严格参数、原生元数据读取及父进程退出后精确 Job 清理。组件层覆盖 Codex 分页和 Orca 双端目录矩阵，旧日期的数量按其原版本理解。

2026-11-02 核对：现行数量与独立原生专项统一见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)。本次仅校正文档/专项描述，并将已有 `TestSRSingleExecutionColdStop` 纳入默认 Controlled 自动回归；其余测试内容、分级、调度与资格规则保持。

> 2026-11-01 当前使用：v8、升级先排空及人工对账规则见[CLI 指南](cli-guide.md#人工对账与资源丢失)。未知操作查询预算耗尽后转待人工；显式 resume 最多 30 秒只读核实原事实，原输入零重发、零资源重建，stop/close 优先。下方按日期保存的旧数量与旧验收说明只描述当时构建。

## 快速定向验证（P1，2026-11-01）

开发反馈可直接运行受影响包的 `go test -run <明确模式>` 或 `scripts/test-foundation.ps1 -Pattern <明确模式>`。真实 case 使用正式定向入口，例如在仓库根目录执行：

```powershell
.\scripts\build.ps1 -OutFile .\.evidence\p1-build\steerrunner.exe
.\scripts\test.ps1 -Focused -Level L2 -Scenario local-codex.smoke `
  -Config C:\srt-config\local.json -RunnerExe .\.evidence\p1-build\steerrunner.exe `
  -OutDir .\.evidence\focused-smoke
```

`-Focused` 只接受单级 L2/L3、明确的 Scenario ID、现有 Config、匹配当前生产输入的固定 RunnerExe 和全新显式 OutDir。可选 ConfigHelper 沿原入口处理。不能与 Catalog、ListAvailability 或 PrerequisitesFrom 合用；不接受通配、重复、未注册或级别不匹配的场景。实际 case、配置评估、fixture、调度 lease、原生断言和停止清理保持。

该入口不启动完整 L1/Controlled，仍新编译 native driver、执行零匹配保护，并把同一 driver 传给 L2/L3 worker。报告新增 `validation_scope=focused`、前置实际启动次数和阶段耗时；case 成功可以是 `status=passed`，但 `complete_level_passed`、`baseline.*_verified` 和完整交付资格始终为 false，即使选中了该级全部 case。`framework/native-zero-match` 目录只是原生零匹配保护，不代表执行完整 framework/L1。

不带 Focused 的默认链路及已有 L2 前置复用规则保持；本增量不开放 L3 的前置证明复用。开发定向通过不替代发布所需的完整验证。P1 实测、历史失败和收益范围见[独立验收](acceptance/personal-executor-p1.md)。P1/P2 的 v6 报告只证明其原构建；P1–P3 及 P4 问答输入项在其原 state/v7 构建独立验收；当前停止条件收敛使用 state/v8、user_version=8，默认 `%LOCALAPPDATA%/SteerRunner-v8`，其受影响验证见[专项验收](acceptance/orca-local-codex-stop-simplification.md)。P3 完整 L1/Controlled 与本机、Orca local/environment 最小真实接线通过；P4 仅取得受影响组件与公开受控资格。当前状态与交付边界见[Spec+Plan 总结](spec/personal-executor-simplification.plan.md#71-本轮总状态与交付边界)，不继承成全 agent、完整 L2/L3 或产品发布资格。

> 2026-10-31 parser 增补：独立解析已改为复用 Codex 登录/CLI 的内存文本任务；旧 parser operation、owner 接管、工具权限及停止取证条款由 [Parser 解耦 Spec](spec/parser-decoupling.spec.md) 取代。工具禁用和进程清理只在 parser 内；其余主执行契约与历史证据结论保持。

2026-10-29 退役旧 Check：当时 Controlled 为 25 个顶层测试，L2 为 1 场景/1 case，L3 为 25 场景/35 case，L4 为 4 场景/4 case；完整通用目录为 30 场景/40 case。旧构建的日期记录不代表当前目录。删除范围与验证见[退役记录](acceptance/check-retirement.md)。

2026-10-27 历史核对：当时通用目录为 L2 1 场景/1 case、L3 29 场景/39 case、L4 4 场景/4 case，
Controlled 前置 26 个顶层测试，另有 5 个 focused native L3 专项。
该轮 L2/parser 拆分、feedback 吸收、内容生成及 L4 同批构建复用的历史依据与建议见[测试清单整理结论](test-catalog.md#测试整理结论-2026-10-26)；现行数量和专项入口见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)。

## 固定交付包验证（2026-10-26）

Orca → 原生 Codex 输入专项复用 `TestOrcaNativeStateLiveLifecycle`，登记为 focused native
`orca-remote.native-input`。真实入口为 `./scripts/test-native-input.ps1 -Target <固定目标JSON>
-OutDir <全新目录> -Mode long-input`，调用生产通用 text+enter+原请求 ID；不读取草稿或调整视口。
原 `segmented-shape` 手动两次粘贴试验已退出当前入口，旧证据保留。
保真修正另由原受控入口覆盖模型桥实际 WebSocket、超16MiB回复后同连接原提交、CR参数与逻辑LF原文、仅stage不提前Started。最新239叶各30次（含生产构造器接线回归）与双端各3场证据见[专项验收](acceptance/orca-native-submit-reaudit.md)。
确定性故障入口为 `./scripts/test-native-input.ps1 -Mode controlled -Repeat 30 -OutDir <全新目录>`，
无需真实目标，输出每个叶例及总执行/失败/跳过计数；它不是实际 Orca L3 资格。
目标 JSON 含 `Worktree`（完整 `id:repo::path`）、`Runtime`、`Environment`、`CLI` 和
`Launch`（现有 `agentlaunch.Config` 的 executable/args/model/reasoning_effort/access_mode）。
旧终端链路的本机探针要求 `Environment` 为空，并在 `Launch.runtime_executable` 指向原 Runner 构建提供的隔离 helper。该专项只记录旧链路资格。新本机结构化链路以正式 CLI 真机案例验收。
本机模型回合还须提供现有 `Launch.audit_model`（model/reasoning_effort），其模型名与 writer
不同且 effort 合法，与正式 workflow 冻结契约一致；只配置该能力，不额外投递 audit 输入。
本机适配依次调用原 control、launch、ready、模型 wire/history 和正式 stop 流程；
不预置 ready、不借远端完成来源。设置阶段与业务阶段的调用分别记录，业务阶段按原 terminal
及 request 计数，任意无 ID 正文、补 Enter 或错误 request 也不能漏计。
准备回执通过公开 `--cursor 0` 读取原始流，要求保留起点及来源/完整性元数据齐全；
持续控制查询允许旧前缀正常淘汰，按 `nextCursor` 分页至首次 `latestCursor` 快照。
`TestLocalPreparationOriginalStream` 核验准备完整/空流和拒收反例；`TestFixedControlRetainedStream`、
`TestFixedControlStreamFaults`、`TestFixedControlStreamBoundaries` 随同一 controlled 矩阵核验累计
输出后的 history/workspace/认证停止、逐页身份/缺页/取消，以及快照/空流/半行边界。
只用新的隔离工作区，先关闭已核实属于本次创建的空闲默认 shell；不能填历史业务终端。
测试拒绝非空/不完整清单、重复目录、跳过、零匹配、全文/唯一性/进程归属不符与未确认清理。
每次真实调用独立启动并关闭会话及所属业务/control 终端，保留 target、源码/EXE/CLI 摘要、请求/回执、
一次通用提交/零分开输入计数、正式唯一原输入、原结果与前后完整清单。
当前最小自然长输入包含中文 BMP、CRLF 和空行；非 BMP 与代码块的受控覆盖不算本批真实资格。
本机仍须具备原准备/模型/history条件，
未具备时单列未验证，不能借远端profile或补查询伪装本机完成资格。
默认 L1 不启动它；不将 opt-in skip 当真实通过。具体命令与范围见
[输入验收](acceptance/native-start-confirmation-20261027.md#仓库内可重复输入验收2026-10-27)。

`scripts/test-delivery.ps1 -PackageDir <已打包目录> -OutDir <新的包外证据目录>` 以包内 Runner、客户端 helper 和 Viewer 为被测对象，不重新构建替代 Runner。复用隔离安装/篡改检查、客户端受控矩阵、Viewer 原生窗口检查和 `TestSRProgressFile`；后者验证原始字节、原子替换、多 Run 隔离、读取失败保留、服务重启零重发和终态快照不刷新。

`qualification.json` 的 scope 固定为 `controlled-client-delivery`，记录包清单/完整内容摘要、测试 harness 的提交及源码摘要、四项结果文件摘要；进度项另绑定原始快照、日志与进程/被测 EXE 身份记录。失败保留独立报告，未执行或缺报告不能通过。报告必须位于包外；验证结束复核包与 harness 未变化。`scripts/verify-client-package.ps1 -PackageDir <目录> -Qualification <报告>` 复核同一包、完整检查和结果文件，之后改变包或绑定证据会拒绝。

该资格不替代 L1/完整 Controlled、L2/L3 真机资格，也不验证 SteerLine 的业务进度字段或页面。Task18 的 0/6 仍按[原负例](acceptance/task18-runner-retest-20261026.md#测试进度显示的独立负例)保留；Runner 不解释测试安排、不补写业务进度。本入口只建立可重复的 Runner 侧字节通道基线。

普通 L1 的 framework 增加 `tests/framework/delivery-test.ps1`，使用合成包核验清单遗漏/多余/重复/越界、构建回执失配和资格报告失败/不完整/跨包/证据变化；不启动模型、服务或 Viewer。`test-foundation.ps1 -RunnerExe <固定 EXE>` 和 `test-client.ps1 -ClientHelper <固定脚本>` 为显式选择被测产物的可选参数，省略时保持原入口行为。

`test-foundation.ps1` 的 `-RunnerExe` 与 `-PreparedArtifactsDirectory` 是两种互斥的产物来源：前者保持指定 Runner 不重建，后者复用已通过前置的 Runner/driver/fixture 并校验摘要；同时指定会在执行前拒绝，避免替换固定交付包中的被测 Runner。

文件清单及源码摘要使用固定 ordinal 排序，避免 .NET/PowerShell 的区域排序差异。framework 和交付入口均另用 Windows PowerShell 5.1 核对同一包摘要；最终资格只有在宿主与 Windows PowerShell 身份一致时通过。

2026-10-25 Orca CLI 等待增量：Controlled 新增 `TestSROrcaCLIWaitRetry`（当时 24 个顶层）；L3 新增 `orca-local.cli-wait-retry/default`、`orca-remote.cli-wait-retry/default`，当时 L3 为 29 场景/39 case，全目录 35 场景/45 case。两项原生 case 复用原 fixture 和转发 journal，以独占测试 lease 与同 cache 的其它 case 串行；独立 state-dir 不是 CLI cache 隔离证明。持锁故障只作用于 fixture 私有互斥 key，真实 CLI 转发器仍取得原用户共享 cache 的互斥；不会绕过生产入口。

最小受控入口为 `./scripts/test-foundation.ps1 -Pattern '^TestSROrcaCLIWaitRetry$' -OutDir <新绝对目录>`，覆盖真实跨进程 >15 秒排队、原服务正常/强杀重启、stop、换代前撤销、三 Run 相互隔离与暂态退避（七个变体）；组件/公开 schema/持久事务反例纳入原 L1。正式原生入口为 `./scripts/test.ps1 -Level L3 -Scenario orca-local.cli-wait-retry,orca-remote.cli-wait-retry -Config <独立配置> -MaxParallel 1 -OutDir <新目录>`；该入口仍运行完整 L1/Controlled 前置。案例实现不代表目标通过，实际构建、原生资格及清理见[专项验收](acceptance/orca-cli-wait-retry.md)。


审计默认提示简化的定向入口与结果见[本轮验收](acceptance/reliable-delivery-simplification.md)。
原生能力对比使用显式选择的[隔离试验](../tests/experiments/README.md)。历史跨构建冻结验证
`TestSRAuditPromptDefaultUpgrade` 及 foundation 的 `-BaselineExe` 参数已退役，不作为现行入口；原结果仅描述上述验收的原构建。

Run token/独立费用增量入口：`go test ./internal/usage/... ./internal/cost/... ./internal/storage ./internal/host ./internal/cli ./internal/contracts ./internal/resources -count=1`。真实 EXE 的用量查询使用隔离合成投影；三 agent 格式夹具均脱敏。原生产日志只读复算为显式 opt-in `TestUsageExistingNativeEvidence`，默认跳过，不启动业务 worker。实际范围、六组合缺口及首次整仓失败记录见[实施验收](acceptance/run-usage-and-cost.md)；这些测试随现有 L1 `./internal/...` 入口运行，不增加模型或 PR case。

授权回收规则的定向回归还需包含 `./internal/execution/adapters/protocol/orca`：验证原有 repo/group/folder 清单的完整性与引用、读取 selector 撤销证明。远端公开验收在正常停服后同时检查持久 collection permit 已 inactive，不能只以 token 已取得或服务退出代替撤权证据。

2026-10-22 提交前复审新增最终回复 command 归属、冲突双方证据同事务/幂等、新 epoch 空资源意图、分页清单完整性及费用排除项反例；三个用量/费用查询沿最新 main 的完整实例绑定和停止准入围栏。实际 EXE 正例带完整绑定；六组合当前边界与重跑结果见同一验收记录。

本机 Codex 只读现场入口为 `TestRunUsageLocalCodexPublicLive`，显式设置 `SR_USAGE_LOCAL_LIVE=1`、`SR_USAGE_LOCAL_STATE`、`SR_USAGE_LOCAL_RUN`、全新绝对 `SR_USAGE_LOCAL_EVIDENCE`；只接受当前可解码且已停止/完成的原 Run，并拒绝仍有业务授权的来源。测试不会创建模型输入，只复制已冻结、已撤权记录到隔离服务；原日志保持只读。普通 L1 入口拒绝上述遗留 opt-in；独立 seed 清理入口的 `SR_USAGE_LOCAL_REPO_CLEANUP`/`SR_USAGE_LOCAL_CLEANUP_ROOT` 同样被拒绝。

本机 Pi 入口为 `TestRunUsageLocalPiPublicLive`（DSH 专用入口已移除），复用同组 `SR_USAGE_LOCAL_*` 变量，以精确 `-run` 选择与原 Run agent 相符的测试。仅接受现存 pr_ready 主会话，以只读 `UsageRuns` 投影历史状态；隔离副本无业务授权/待执行意图。验证原生日志逐响应 command 归属、公开 EXE 自动统计、独立估价、采集服务停启/强制重读去重；不创建模型输入或目录登记，不停止原生产服务/终端。实际 token、合成价表及清理证据见同一验收记录。

远端已完成 Codex Run 的显式现场入口为 `TestUsageRemoteCodexLive` / `TestRunUsageRemotePublicLive`，所需变量及撤回/清理证明见同一验收记录，默认 skip，标准 L1 拒绝远端 opt-in 遗留。手工整仓 Go 入口应使用 `go test ./... -count=1 -timeout 45m -json`：2026-10-21 实测 application 全包 625.267 秒、`TestSRResultAcquisition` 串行整组 738.65 秒，均可能超过 Go 默认 package 10 分钟上限。该聚合预算不延长产品调用或场景内期限；完整受控入口本已采用 45 分钟预算。

固定 L1 的 Go 包级聚合预算为 30 分钟。2026-11-02 曾从 12 分钟增至 20 分钟，记录见[文件材料验收](acceptance/generate-files.md)；2026-11-05 application 全包一次 1159.076 秒通过，另一次在最后共享引擎停止场景仍连续推进时触及 1200.59 秒聚合上限，故经独立审核增加整包余量。调整不延长产品操作或用例内期限，不改包清单、skip 白名单、断言、覆盖率、框架和原生驱动守卫。独立的用例内失败仍保留且须复验，不由 watchdog 调整豁免，证据见[目录边界验收](acceptance/native-catalog-boundary.md)。

裸 `go test ./...` 还会包含 L4 的 `TestSREdgeResourcePressure`（20 轮完整提交/发布/停止及进程退出/句柄检查），并非 L1 入口。常规分层运行使用下文脚本和 catalog；全仓审计保留此测试，不为缩短耗时削减轮数。

内容流程的最小定向入口为 `./scripts/test-content-workflows.ps1`；附加
`-NativeExecutable <已有原生 Codex 0.159.2 EXE> -Model <可用模型>` 后，在三个内容受控场景
通过后运行真实生成/多轮/中断/重启与结算。原生测试创建新的 `C:/srt/cw-*`，不使用
SteerLine 安装环境、Git 或 PR。此专属 L3 入口独立登记在 `tests/catalog.json.focused_native_scenarios`，
不改变现有开发 manifest driver；命令、构建摘要与实际结果见[专属验收](acceptance/content-workflows.md)。

测试源码、夹具、现场生成、执行和结果判定全部归属本仓；不要求 SteerLine
checkout、EXE、数据库、配置或测试调度器。运行目标为 Windows/amd64。

本文件说明分层定义、设计与维护规则、执行入口和结果判定。
逐项测试内容、预期结果、并行条件及当前缺口见独立的[测试清单](test-catalog.md)。

独立调用配置契约、本仓内部测试配置、本地 Codex 交付基线及按 case 判定 L3 可用性的增量
见 [Spec](spec/independent-configuration.spec.md) / [Plan](spec/independent-configuration.plan.md)。
当前使用 `steerrunner-test-config/v1`；本次构建的实际范围和缺配置项见[验收](acceptance/independent-configuration.md)。

Orca 观察恢复定向回归：`go test ./internal/execution/adapters/orcatransport ./internal/application -run 'TestOrcaNativeIncompleteMetadataRecovers|TestOrcaNativeMissingPromptCannotHideConflict|TestOrcaNativeFinalRetainsVerifiedInputBinding|TestOrcaObserverReportsChangingGapAndRecovery|TestOrcaCompletionQuery|TestOrcaStateCodec|TestTerminalNativeFinalRecoveryNeverReplaysInput|TestSRObservation' -count=1 -timeout 5m`。覆盖工具执行后 prompt 持续缺失仍保留已核实输入绑定、其它最终状态字段补齐、观察器重建、短回合无 working、冲突不可抹除、旧 codec 保守转换、诊断事务/幂等与恢复。使用受控原生响应及真实 SQLite 事务，不创建模型输入或真实 PR；通过不等于该现场历史锁死记录已经修复或新版已部署。

## 分层定义

SessionStart 空预览补证定向入口：`go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -run 'TestOrcaNativeInput' -count=1 -timeout=3m`。默认只使用隔离原生响应和 IPC 夹具，核验原输入全文/唯一性、旧 proof、分页、读取竞争及零重发；无需生产状态或远端模型。只读现场探针独立记录于执行可见性验收，不作为默认测试入口。

断线后空预览定向回归：`go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -run 'TestOrcaCompletionQuery|TestOrcaObservationDiagnostic|TestOriginalAgentQueryRetainedStamp' -count=1 -timeout=3m`。覆盖原 working 起点持久化后断线/重建的独立查询收尾、延迟正文、观察器完成事件及诊断解除；缺 start/marker、不同 marker、身份/历史冲突、截断正文继续拒收。协议反例防止不同主记录或冲突来源借用 marker。受控夹具固定一次原输入、零重发，不读取生产状态或启动模型；实际范围见[执行可见性验收](acceptance/execution-observability.md#断线后空输入预览恢复2026-10-23)。

按测试目的、流程和隔离条件分层，不以是否使用真机、模型或远端环境作为层级定义。

| 层级 | 定义 | 设计要求 |
| --- | --- | --- |
| L1 | 快速的单元测试或集成测试 | 快速反馈规则、组件、协议、存储和依赖边界的正确性；现有默认入口不启动真实模型或创建真实 PR |
| L2 | 代表最小核心流程的 Smoke test | 用最少的固定流程判断核心链路是否可用；扩展流程和边界变体放入 L3/L4 |
| L3 | 可并行的多种不同流程及核心可靠性测试 | 覆盖开发、业务问答、审计、远端执行，以及可预期故障下的可靠执行；各项独立、互不干扰，主要验证目标尽量正交 |
| L4 | 更复杂的边界情况测试，或确实无法并行的测试 | 覆盖复合故障、精确竞态、极端资源条件等；记录超出常规可靠性流程的条件，或必须串行的原因及互斥范围 |

具体归类先看产品承诺和独有断言，再看故障组合及隔离条件。不能仅按故障数量、次数或
注入点命名判断：暂时不可达期间停止再重启、发布完成证据缺失后查询失败，虽然包含多个
事件，仍在验证核心保证，属于 L3。常规 stop/reply/resume 竞争、有限次重连和正常资源
范围内的多 Run 隔离也应优先放在 L3；只有超出这些基线的组合或极端条件才规划为 L4。

**Agent 可靠运行是 SteerRunner 的核心能力。** 调用方退出、单次断连、服务重启、停止
授权撤销、结果未知后的对账均属于 L3。故障注入本身不是归入 L4 的理由，case 名称中的
`boundary` 指实施回合与下一阶段之间的切换点，也不等于 L4 的边界情况。这些可靠性
测试保留在 L3，包括暂时不可达时停止、跨重启撤销和发布查询失败等核心保证。

L3 的正交性按主要验证目标判断：新增 workflow、交互路径、后端契约或可靠性性质时，
说明它补足的独立覆盖。不同可靠性测试可以复用同一个简单开发任务，独有断言分别核验
请求去重、连接恢复、服务换代授权、停止撤销、发布去重等性质，不必刻意更换业务任务。
只换模型、参数、目标机器或重复相似步骤，不能直接算作新的流程覆盖；平台差异须有独有
兼容性断言。服务换代的正常/异常退出与活动回合/阶段切换是不同恢复窗口，保留四个
变体；只有证实独有断言重复后才合并。共同的准备、产物核验、发布和收尾可以复用。

各测试必须隔离现场、state-dir、Run、工作区、分支、PR 和故障设施，不依赖其它测试的
结果或执行顺序。声明可并行须有资源隔离说明和实际并行验证；当前调度器串行执行、
需要管理员故障设施或尚未验证并行，都不作为把核心可靠性测试移入 L4 的理由，应先完善
隔离。确实无法隔离的独占环境测试归 L4，并明确互斥范围。

区分测试之间的并行与产品内部的并发：普通 case 之间使用独立服务/state-dir；验证
同一服务内多个 Run 时，一个 case 内应共享该服务/state-dir，各 Run 的请求、工作区、
会话、分支和 PR 独立。该 case 仍须与其它 case 隔离。独立服务的测试并行通过，不能替代
同一服务的多 Run 隔离验证；产品允许的资源排队也不能误判为要求所有 Run 同时执行。

Windows 的 Orca CLI 会刷新同一用户的环境配置文件，多个进程同时调用会产生
`orca-environments.json` 的 `EPERM rename`。Runner、真机诊断和 fixture 准备共用
规范化的 `%LOCALAPPDATA%/SteerRunner/orca-cli.lock` 路径摘要构成的 Windows 命名互斥，只串行 CLI 进程，远端
Agent 和 state stream 仍并行。等待占用原调用期限；取消后不启动命令，不重试未知发送。
业务 send 立即捕获受理回执，实际开始/完成继续由原 native history/state 证明，
不在持锁的 CLI 内等待模型回合。原始失败批次保留，不能用重跑改写首轮结果。
命名互斥按拥有线程持有，Go 调用期间固定该 OS 线程；取消通过独立事件结束等待。
持锁进程被杀后按 abandoned-owner 语义取得互斥，但必须重新核验原资源身份。
只有明确未调用 CLI 的排队错误才可重新观察原对象；未核实的创建/发送不据此重试。
只读回执查询在取得 CLI 互斥后、调用前持久化创建意图；排队中被杀不会留下模糊创建。
撤销 journal 先保存原停止意图，取得互斥后再标记实际创建尝试；恢复可执行尚未调用的
原意图。实际尝试已落盘但缺返回身份时仍保持未知，不补建辅助终端。
撤销失败只导出有限错误类别并持久保留，不发布原始输出、路径或凭据。
真机控制请求只有收到明确 `stale_revision` 拒绝后，才刷新原 Run 并用新请求 ID 提交；
未知或丢失回执仍保留原身份，不能重复授权。

`Controlled` 是现有脚本的受控集成前置入口，不是 L1–L4 之外的第五级。它使用真实
EXE/CLI/IPC/SQLite、隔离 Git 和原生协议/provider 夹具，可在本地隔离 push，不创建真实
PR。现行顶层数量统一见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)，覆盖独立包、服务归属、材料冻结、权限资格、完整工作流、
存储/日志/事件、PR 身份、工作流完成前的发布事实、原生未知结果与停止、重启后显式恢复、
丢响应找回、停止授权跨重启保持、审计发现/修复/预算、同服务多 Run 隔离、资源创建回执恢复、人工对账与 execute/1 冷启动停止。
审计场景的子变体单独在日志中登记，批次 `scenario_count` 按顶层测试计数。
是否纳入快速 L1 应按耗时和依赖评估。

## Controlled 分组、固定选测与耗时（2026-11-02）

主归属和固定组合维护在 `tests/catalog.json.controlled_selection`；完整执行集合仍只由原 `controlled_scenarios` 决定。
六组必须无遗漏、无重复地覆盖全部注册测试；固定组合只能引用已注册测试，完整名称可用下列命令只读列出：

```powershell
./scripts/test-controlled.ps1 -List
```

| Group ID | 主要内容 | 顶层数 |
| --- | --- | --- |
| foundation | 独立包、服务归属、材料冻结、权限、存储/日志/事件 | 5 |
| workflow | 开发闭环、发布身份/提前发布、工作流选择、审计结果获取 | 5 |
| recovery | 未知结果、换代、丢响应、跨代撤销、多 Run、创建恢复、人工对账 | 7 |
| content | generate、固定仓库调查、conversation、问答/unknown、内容恢复 | 5 |
| files | R2、progress、result-file、ColdStop | 4 |
| orca-cli | Orca CLI 排队/取消/读取重试 | 1 |

| Combination ID | 固定测试集合 |
| --- | --- |
| execute-stop | SingleExecutionColdStop、ResultFile 的 final-seal、NativeUnknownAndStop、RestartPreservesAuthorization |
| runtime-files | ProgressFile、ResultFile |
| content-flow | content 组的五项，加 NativeUnknownAndStop、ManualReconciliation |
| acceptance-control | foundation 与 recovery 两组的十二项 |
| result-plain | ResultAcquisition/plain-marker |
| result-audit | ResultAcquisition/audit-zero、audit-positive、audit-offline、audit-report |
| result-parser | ResultAcquisition/parser-unsupported、parser-complete、parser-tool |
| restart-continuation | RestartPreservesAuthorization/graceful、kill |
| restart-create-receipt | RestartPreservesAuthorization/creation_receipt |
| restart-pending-stop | RestartPreservesAuthorization/pending_stop |

表中测试名省略 `TestSR` 前缀。组合是常见改动的固定起点；修改共享事务、授权或原生协议时仍需按实际影响补选，不将组合当作所有改动的充分验证。

```powershell
./scripts/test-controlled.ps1 -Group foundation,recovery -OutDir .evidence/control-basic-new
./scripts/test-controlled.ps1 -Combination execute-stop -OutDir .evidence/execute-stop-new
./scripts/test-controlled.ps1 -Combination runtime-files -RunnerExe <固定Runner绝对路径> -OutDir .evidence/files-new
# 只读统计已有批次；输出必须使用原证据目录之外的新目录。
./scripts/test-controlled.ps1 -FromEvidence <已有foundation证据目录> -OutDir .evidence/timing-import-new
```

入口复用 `test-foundation.ps1`，不运行完整 L1；其原有四项 foundation 单元前置、真实 EXE/受控 peer、独立进程、原超时与清理保持。Group 与 Combination 二选一，支持多个精确 ID、拒绝未知/重复/通配 ID；展开后的测试去重并按原完整目录顺序记录；实际执行顺序沿用原 Go 测试执行器，不按分组重新排序。每次使用新 OutDir，可用 `-TimeoutMinutes 1..60` 选择原 foundation 聚合预算，单项调用期限不变。

`summary.json` 固定记录 `validation_scope=controlled-selection`、`complete_controlled_passed=false`、`qualification_eligible=false`，保存组合/精确测试集合、catalog 摘要及入口总耗时。即使选择六组全部测试，成功也只表示这次选择通过；不生成 `prerequisites.json`，不能用于 `-PrerequisitesFrom`。原 `test.ps1 -Level Controlled`、独立完整前置及 L2/L3/L4 的完整 Controlled 门禁继续执行原完整集合，不读取选测资格。

`timings.json` 只累计 Go 顶层 `--- PASS/FAIL/SKIP: Test… (…s)`，不重复累计子测试；记录逐项状态、主分组、分组已观测数量/缺项、已观测耗时合计、最慢十项和原 log/result/source-identity 摘要。未完成项的耗时为 null，零执行、缺结果、重复记录、未知测试、失败/跳过或精确集合不匹配不能报告选测通过。顶层耗时合计不等于构建/准备/清理在内的入口墙钟耗时；日志精度为 Go 输出的百分之一秒，不用它推导细粒度性能收益。

ResultFile 的实际子场景明细另见下节。报告新增 `subtests`（逐条实际运行/结果，不加入顶层合计）与 `result_file_coverage`；要求子场景回执和日志相互吻合，不能用父测试通过或顶层计数代替。

完整 Controlled 阶段也附加 `controlled/timings.json`，并在阶段 record 中记录路径。它是诊断附件，不增加完整资格的证据清单或门禁；统计不可用时记录 `timing_error`，原完整测试及资格判定仍按原规则执行。旧日志可通过 FromEvidence 只读统计，但不重写旧结果、不补签旧资格、不合并不同构建的通过或成本。

历史成本样本：按本次分组只读统计 [P3 原完整日志](acceptance/evidence/personal-executor-p3-20261101/complete/controlled/integration.log)，当时 26 项共 1441.64 秒顶层耗时；workflow 541.29、recovery 326.17、files 279.39、foundation 112.44、content 94.71、orca-cli 87.64 秒。最慢项为 ResultAcquisition 475.80 秒和 ResultFile 155.87 秒。files 当时只有三项、缺 ColdStop；这些是原 v7 构建的成本样本，不是当前完整集合耗时，也不代表已取得加速收益。

### ResultFile 职责与定向粒度（2026-11-02）

`TestSRResultFile` 仍是完整 Controlled 的一个顶层测试，默认执行下列两类子场景；完整集合仍为 27 项。仅调整测试编排和取证，不修改产品实现、等待预算、四种启用组合或原业务/停止断言。

| 子场景 ID | 保留的独有覆盖 |
| --- | --- |
| optional-file-failure-business-loop | enabled-0..3 四种 progress/result 启用组合；普通文件阻断两个通道目录；每项独立现场到达原 pr_ready、原生输入恰为 3、授权不被文件故障阻断、正式停止；启用 result 时闭合为 version=0 且有诊断，不能伪造字节 |
| final-seal | enabled-0..3 四种健康组合；同服务四 Run 的路径/原生环境隔离、禁用通道不创建目录、种子字节、运行中 pending/不刷新、progress 更新、错误实例查询拒绝；原子替换后的任意字节/大小/SHA/版本/时间、停止一次封存、查询及服务重启不刷新、原输入零重发 |

原健康流程只增加 `final-seal` 名称和四个 enabled 子场景，原失败条件与断言调用保留。服务、仓库和 Run 仍按原流程隔离；本轮不引入跨 case 的可变夹具共享。

`execute-stop` 固定组合现在只选择 ResultFile 的 `final-seal`；其他顶层测试和其 graceful/kill/creation_receipt/pending_stop 等子场景照常执行。`runtime-files`、files 组和完整 Controlled 仍执行 ResultFile 两类流程。修改共享停止、通道初始化或文件故障机制时，应选择完整文件矩阵；组合并集按较宽覆盖合并，不能因同时选择 execute-stop 就删掉故障流程。

```powershell
# execute/1 停止及封存；ResultFile 只选择 final-seal。
./scripts/test-controlled.ps1 -Combination execute-stop -OutDir .evidence/execute-seal-new
# 共享停止/文件改动：并集保留两类 ResultFile 和 ProgressFile。
./scripts/test-controlled.ps1 -Combination execute-stop,runtime-files -OutDir .evidence/shared-stop-new
# 显式覆盖只接受这两个精确 ID；可按现有 PowerShell 数组方式同时指定两个。
./scripts/test-controlled.ps1 -Group files -ResultFileCase optional-file-failure-business-loop -OutDir .evidence/file-faults-new
# 单个父测试的底层定向入口；沿原 foundation 机制等待独立进程与 result.json 后再核验结果。
./scripts/test-foundation.ps1 -Pattern '^TestSRResultFile$' -ResultFileCase final-seal -OutDir .evidence/result-seal-new
```

`-ResultFileCase` 仅作用于 ResultFile，保留为兼容入口；新的选测入口统一解析为按父测试的 Case 计划，foundation 仍兼容旧 `-result-file-cases` 标志。不使用全局 Go 子路径过滤。未知/重复 ID、选择集合没有 ResultFile、与 List/FromEvidence 混用均拒绝。同一父测试不能同时用 Case 与 ResultFileCase。未收窄的 foundation/full 入口仍执行两类。

每次新执行在 `result.json.result_file_cases` 记录实际执行的 ID、成功布尔和包含清理的耗时；`source-identity.json` 保存显式选择，选测 summary 保存解析后的精确集合。Go 端拒绝选中的父场景或 enabled 变体未执行；公开选测和完整前置另外核验对应 RUN/PASS、四个 enabled 叶例、重复/跳过/失败/多余场景及实际回执，完整前置必须具备两类实际覆盖。定向执行全部父测试但缺其中一类仍不能取得完整资格。

历史日志缺新子场景回执时只能只读统计耗时，`result_file_coverage.status=unverified_import`，不补写实际覆盖或完整资格。子场景耗时用于说明成本分布，不与父测试时间重复累计。

本轮成本验证固定同一 Runner，最终默认全量和定向使用字节相同的 integration driver。默认 ResultFile 两类/八变体通过，顶层 205.11 秒（故障闭环 121.76、封存 83.35）；最终 execute-stop 的 ResultFile 只执行 final-seal/四变体，82.15 秒，该样本定向成本减少 59.95%。这是省去定向范围之外的四项故障业务闭环，不是让完整验证跳过它们。

execute-stop 原版基线墙钟 307.29 秒，最终样本 279.10 秒；中间候选样本分别为 238.40 和 363.18 秒，波动明显，不能认定有稳定的组合整体提速。原版 ResultFile 的旧样本为 150.33 秒，最终默认全量为上述 205.11 秒，完整验证没有取得提速证据。本轮只验证相关测试及实际子场景，不授予完整 L1/Controlled 27 项或真实模型资格。原日志、失败反例、产物摘要与全部耗时样本保留于本工作副本 `.evidence/resultfile-granularity-20261102/`，可用同一固定 Runner 和上述入口复验；不把不同样本相加为覆盖或收益。

### ResultAcquisition 断言对照（2026-11-02）

本轮先以固定 Runner 复测 `TestSRResultAcquisition` 的原八项，再调整定向编排。下表按现有测试源码登记，不要求真实模型制造审计发现，也不将 parser 的接线验证替换为纯文本单元测试。

| 职责 | 子场景 | 原生 turn/start 预算 | 原有独有断言及共同收尾 |
| --- | --- | --- | --- |
| 普通结果获取 | plain-marker | 2 | 普通开发到达 pr_ready；一个原 thread；保留完整工作流和原资源清理 |
| 审计结果 | audit-zero | 3 | 显式 executor/reviewer 模型准备；唯一审计结果，SourceResultID/CommandID/TurnID 与最终 PR head 来源 |
| 审计修复 | audit-positive | 3 | 上述审计来源；pre/post head 不同且 fixed_count=1 |
| 离线审计 | audit-offline | 3 | 审计活动时停服；等待原 owner 至少两个 checkpoint；离线时 turn/start 仍为 2；原服务换代后原流程继续 |
| 一次补报 | audit-report | 4 | 原审计以纯文本返回后补报；审计来源及单 thread/精确 turn 预算 |
| parser 后备 | parser-unsupported | 4 | 保留审计工作流；llm_parser 有 failed attempt，不能隐藏不可用资格；原流程后备完成 |
| parser 成功 | parser-complete | 3 | 保留审计工作流；实际结果 Method=llm_parser、Status=satisfied |
| 工具事件回退 | parser-tool | 4 | 原 PARSER_TOOL 夹具输入、审计来源、补报回退后的精确预算；不增设真实工具调用或模型要求 |

八项共同保留独立现场、公开服务/submit、最终 pr_ready、thread/start=1、原权限/模型夹具、停止及服务清理。各操作和整流程的既有期限不延长。已清理的 parser-restart/parser-kill/parser-stop 条件不在原八项常量列表内，且循环变量未被重写；只删除该不可达测试分支及其专属导入，不删除业务 owner 的恢复/停止义务。

本轮将 ResultFile 的回执/选择机制与 ResultAcquisition 共用，使用 `-Case <父测试/子场景>` 和 Go driver 的 `-controlled-cases`，支持的父测试受登记清单限制，包含 ResultFile、ResultAcquisition 和下节 Restart。每个父测试独立解析，未指定的父测试仍运行自己的完整集合；不会以全局 `-test.run` 子路径过滤其他测试。原四个组合保持，结果获取增量新增三项职责组合；加上下节 Restart 三项，当前共十个固定组合；workflow 组和完整 Controlled 的 ResultAcquisition 仍执行全部八项。

```powershell
./scripts/test-controlled.ps1 -Combination result-plain -OutDir .evidence/result-plain-new
./scripts/test-controlled.ps1 -Combination result-audit -OutDir .evidence/result-audit-new
./scripts/test-controlled.ps1 -Combination result-parser -OutDir .evidence/result-parser-new
# 合并三类，得到原八项；不能据此获取完整 Controlled 资格。
./scripts/test-controlled.ps1 -Combination result-plain,result-audit,result-parser -OutDir .evidence/result-all-new
# 每个父测试单独选择；Restart 等同批测试不受到子路径过滤。
./scripts/test-controlled.ps1 -Combination result-plain,execute-stop -OutDir .evidence/result-mixed-new
# 显式选择只接受已登记的精确 parent/case；父测试须在组合/组中。
./scripts/test-controlled.ps1 -Combination result-parser -Case TestSRResultAcquisition/parser-complete -OutDir .evidence/parser-one-new
```

不同组合对同一父测试取并集；任一所选组/组合要求全量时保留全量。显式 Case 仅覆盖其父测试的选择，未知父测试、未知/重复 case、通配符、额外路径层级、缺父测试和旧参数冲突均拒绝。`summary.selection.cases` 与 `source-identity.controlled_cases` 记录实际请求，`result.json.controlled_cases` 按父测试记录实际完成 ID、成功布尔和包含清理的耗时。ResultFile 的原 result_file_cases 回执保留，存在通用回执时两者必须一致。

`timings.case_coverage` 按父测试核实实际回执与 RUN/PASS 路径，ResultFile 还要求全部选定 enabled 叶例。完整资格须具备 ResultFile 两类、ResultAcquisition 八项及 Restart 四项，不能仅凭 27 个父测试通过；所有定向入口仍不生成前置证明、完整资格标志始终 false。历史 ResultAcquisition 日志没有新回执时只统计成本、标 unverified_import，不补签覆盖。子场景和父测试时间不重复累计。

当前原版基线实际八项通过，顶层 706.39 秒（plain 50.38；audit 83.38/79.97/85.93/117.32；parser 114.49/88.90/86.01）。本轮使用固定 Runner `a7117a33ccdb0e531bad46d677b60bb502cda60d68e2c6e4d7a6aefffa03db1f`，串行采样：

| 执行范围 | 实际 ResultAcquisition 子场景数 | 父测试耗时（秒） | 相对新版八项的选测节省 |
| --- | --- | --- | --- |
| 原版默认八项基线 | 8 | 706.39 | 原基线，不补签新回执 |
| 新版默认八项 | 8 | 592.19 | 回执与八项 RUN/PASS 均核验通过 |
| result-audit | 4 | 321.39 | 270.80 秒，45.73% |
| result-parser | 3 | 290.42 | 301.77 秒，50.96% |
| 混合回归中的 result-plain 范围 | 1 | 30.01 | 562.18 秒，94.93% |

上述均为单轮受控样本，使用本轮基线而非旧构建 475.80 秒；完整八项前后差异不能归因于本轮提速。定向收益来自少选无关职责，不减少已选场景的流程与断言。父测试耗时含场景清理，不含构建和入口准备，子场景耗时不重复累加。基线与新版八项的原始调用记录逐项核对 thread/start=1、turn/start=2/3/3/3/4/4/3/4 均不变；源码 AST 对照保留可达分支原 41 处 Fatal/Error 调用及直接失败条件，仅排除已证明不可达的旧分支。

混合实测仅传 `-Case TestSRResultAcquisition/plain-marker`，父测试正则同时包含 ResultAcquisition、ResultFile、RestartPreservesAuthorization：三项顶层全部通过，共 310.96 秒。ResultFile 未传选择，实际保留两类和八个 enabled 叶例，旧/新回执一致；Restart 的 graceful、kill、creation_receipt、pending_stop 四项全部执行。各次新 driver 摘要一致，原生预算与实际子路径另在 `comparison.json` 对账。实际驱动反例确认退役 ID、未知父测试、通配符、额外路径层级、重复项均在创建证据前拒绝；子路径压掉选中项、其他父测试通过但显式父测试未运行也均失败。

本轮完整脚本框架 27 组通过，包含选测/耗时 67 项、前置复用 75 项与 focused 契约 26 项检查；这些是框架守卫验证，不是 27 项 Controlled 全量执行。

本轮实际日志、原始回执、断言对照和反例保留在 `.evidence/result-acquisition-20261102/`。这轮执行范围为定向 Controlled，完整 Controlled 集合仍为 27 项，L3/L4 不变。

### Restart 职责与断言对照（2026-11-02）

`TestSRRestartPreservesAuthorization` 保持一个顶层注册、默认四项，先复测原完整集合，再按职责提供定向选择。原 `execute-stop`、recovery 组及完整 Controlled 保持四项；完整 Controlled 仍为 27 项，L3/L4 不调整。

| 职责 | 子场景 ID | 原有独有断言与原生预算 |
| --- | --- | --- |
| 原会话换代继续 | graceful、kill | 活动回合时分别优雅停服/强杀；原 owner/native 仍存活；离线原 final 完成且无后继输入；同构建换代沿原 Session 到达 pr_ready，thread/start=1、turn/start=3；显式停止后再次换代，授权关闭、ResumeRequired、revision 不变且零新输入 |
| 创建回执恢复 | creation_receipt | 原 thread/start 已发生但 SessionID 未提交时强杀；原创建完成后换代，不要求 resume、不换 session、不重建或重发；最终 pr_ready、thread/start=1、turn/start=3 |
| 冷启动停止优先级 | pending_stop | 活动原回合时离线，以原 run/revision/请求 ID 传 PendingStops；原回合完成也不得派发后继；重复携带已受理、revision 已旧的同一请求再换代，原请求仍可查且 turn/start=1 |

四项保留独立现场、公开 EXE/CLI、原进程身份核验与资源清理。graceful/kill 退出方式及创建/停止窗口各有独有断言，保留全部四项，不合并故障窗口，不修改产品等待期限或缩短必要观察。

```powershell
./scripts/test-controlled.ps1 -Combination restart-continuation -OutDir .evidence/restart-session-new
./scripts/test-controlled.ps1 -Combination restart-create-receipt -OutDir .evidence/restart-create-new
./scripts/test-controlled.ps1 -Combination restart-pending-stop -OutDir .evidence/restart-stop-new
# 三类并集恢复默认四项；现有 execute-stop 或 recovery 组也要求四项。
./scripts/test-controlled.ps1 -Combination restart-continuation,restart-create-receipt,restart-pending-stop -OutDir .evidence/restart-all-new
# 显式覆盖仅作用于对应父测试，ResultFile 仍按 execute-stop 运行 final-seal。
./scripts/test-controlled.ps1 -Combination execute-stop -Case TestSRRestartPreservesAuthorization/pending_stop -OutDir .evidence/startup-stop-new
```

沿用按父测试的 Case 计划、实际完成回执和包含清理的子场景耗时；每个选中 ID 都必须在回执和 RUN/PASS 路径中精确出现。三类组合取并集，原 execute-stop、acceptance-control、recovery 及完整前置默认不收窄；显式 Case 可覆盖对应父测试。完整门禁除顶层集合外还要求 Restart 四项，只有父测试通过、完整 27 项计数或部分回执不能代替；单独核验 Restart 时也执行四项门禁。定向结果的完整资格标志保持 false，不生成前置证明。旧无回执日志只统计成本并标 unverified_import，不补签覆盖。

本轮固定 Runner 为 `1e6ae83cb636f5d7043d3e4ede5b76dd92b3e4b2935ba8fbedbf60ac03d69045`，所有新版样本使用字节一致的 integration driver，串行实测如下：

| 执行范围 | 实际 Restart 子场景数 | Restart 父测试耗时（秒） | 相对新版四项的样本节省 |
| --- | --- | --- | --- |
| 原版默认四项基线 | 4 | 134.38 | 原基线，不补签新回执 |
| 新版默认四项 | 4 | 109.48 | 四项回执与实际路径核验通过 |
| restart-continuation | 2 | 65.71 | 43.77 秒，39.98% |
| restart-create-receipt | 1 | 25.05 | 84.43 秒，77.12% |
| restart-pending-stop | 1 | 24.82 | 84.66 秒，77.33% |

混合回归仅选择 Restart/pending_stop、ResultFile/final-seal，ContentGenerate 未传子场景选择；实际三项顶层全部通过，共 176.46 秒。ContentGenerate 原八项和 ResultFile 四个 enabled 叶例均完整执行，回执与日志一致。该批 Restart 单项为 55.04 秒，明显高于独立执行的 24.82 秒；全部样本保留，不以最佳单次结果推导稳定整体加速，也不将不同批次耗时相加作为一次完整覆盖。完整四项前后差异同样不能归因于本轮提速。

原 graceful/kill 完整业务体提取后除缩进外一致，generation 中原创建回执及 PendingStops helper 逐字节未变，其他 workflow 测试未修改。原版/新版默认四项、三类定向和混合批次的原始 native journal 逐项核对：thread/start 均为 1，前三项 turn/start=3，pending_stop turn/start=1。实际 driver 的五种非法选择在证据创建前拒绝；显式 case 被子路径过滤、其他父测试通过但选中父测试未执行也均失败。

完整脚本框架 27 组通过，包含选测/覆盖 86 项、前置复用 75 项和 focused 契约 26 项；覆盖缺项、错误父归属、跳过、重复、假回执及“完整 27 个父测试但 Restart 不全”等拒绝路径。框架组数不等于本轮执行了完整 27 项 Controlled。

完整本地样本、原始调用记录、断言/职责对照和反例位于 `.evidence/restart-selection-20261102/`，汇总为 `comparison.json`。这轮是定向 Controlled 验证，完整注册仍为 27 项，未据此授予完整 L1/Controlled 或真实模型资格。

## 测试清单维护规则

L2/L3/L4 的具体内容统一维护在[测试清单](test-catalog.md)。已实现项以
`场景 ID / Case ID` 标识；增加、删除、拆分或调整可执行测试时，必须在同一变更中更新
测试清单、可执行注册和相关断言。规划项使用
文档编号，明确未注册及实现缺口，落实时再绑定真实场景/case ID。清单至少维护：所属层级、
主要验证目标、执行入口、前置与资源隔离/互斥约束、固定预期结果、实现状态和验收证据。
未实现、前置阻断、受控通过与真机通过分别登记，不能把清单存在或 driver 已实现当作
当前构建通过。测试 outcome、业务 outcome、资源清理和并行资格分别报告。

执行目录为 [tests/catalog.json](../tests/catalog.json)，唯一 case 定义为
[cases.json](../internal/testkit/livetest/cases.json)；PowerShell 的
[目录读取](../scripts/live-catalog.ps1)将场景 ID 展开为注册元数据；旧格式自定义目录继续核对层级、driver、target、超时和完整 case 集，批次冻结展开后的完整目录。
[Go 注册测试](../internal/testkit/livetest/cases_test.go)与
[脚本检查](../tests/framework/stability-test.ps1)核对现行 L3 注册完整性；数量统一见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)。四个 L4 场景绑定精确的受控测试名，不以受控结果代替真机资格。
新增场景须注册需求并将 ID 加入执行目录，同步 manifest 校验和结果报告。workflow/2 在 `pr_ready` 后拒绝业务反馈；L2 不创建发布后反馈模型轮，也不读取反馈资格文件；smoke 在原 PR 上核验反馈拒绝后 revision、Turn 与 PR head 不变。
耗时和模型使用按实际测试目的安排：快速规则/路由反例放 L1，复杂确定性分支使用受控
集成，真机测试验证真实后端接线与核心行为，不强迫模型制造可重复的 finding。
文档规划不表示入口已经支持新 case、L4 或并行调度。

## 当前执行入口

初始化资源停止新增 L3 `orca-remote.initialization-stop/before-readiness` 与 `restart-before-readiness`。公开 submit 直接启动原生 Codex，测试转发器仅隐藏 caller 的原生 agentIdentity，保留原响应与调用 ID；第二项在该窗口停服换代，验证原终端绑定和零自动输入后显式停止。两者均要求完整原 Orca 清单、`orca_terminal` 离线 scope，零业务输入、额外终端或文件部署，不要求远端 helper/Job/签名/认证撤销。独立现场可并行，无管理员故障设施；新增结果与历史记录见[初始化停止验收](acceptance/initialization-stop.md)。

Orca 本机普通 Run 接入使用 `orca-local.smoke`，沿用现有 driver/schema。对应配置项的
`execution.settings.agent_launch.runtime_executable` 指向本机 helper，
`fixture.repository.local_path` 明确选择真实注册项目，不能任选同名项目。
本机 agent/model/effort 取该项 execution.settings 的公开字段，不需要远程
environment。执行主机 helper 安装在业务工作区外。本机 fixture 的 caller clone 与 Orca
已注册项目分别记录，原 runtime 从准备证据核验，不复用 `remote_runtime_id`。
本机通过原生 IPC 查询观察，远程保留订阅加补查；注册不代表已验通过，当前构建结果见[专属验收](acceptance/orca-local-remote.md)。
停服取证探针为 `go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout 280s -v`。
先用调用方 `scripts/new-debug-release.ps1` 生成独立 `C:/SteerlineDebug` 包；显式提供
`STEERRUNNER_ORCA_STOP_PROBE_ROOT`（新的包内 evidence 子目录）、`_CLI`、`_RUNNER`（该包固定 EXE）、
`_REPO`（Orca 注册的测试项目 ID）、`_BASE`（固定完整 commit）、`_REPOSITORY`（本包本机 clone）；
变量名均以 `STEERRUNNER_ORCA_STOP_PROBE` 开头，远端另用 `_ENVIRONMENT` 固定路由。
不设 `_HELPER` 时沿用真实 PTY+存活 root 的拒绝探针；设置 `_HELPER`、`_HELPER_SHA256`、
`_NATIVE`、`_MODEL` 时运行真实 Codex/Orca：原 helper 启动并完成一次无工具 READY bootstrap，
真实原生历史 reader 查询该 bootstrap 并清空原 Job，随后实际适配器显式停止原 Codex，关闭原 PTY。
发布 Runner EXE 的 `service verify-stopped` 必须成功，普通被动 shutdown 使用同一证明；
活动期拒绝，DB/原失败回执不变，不启动服务。只清理本次 owned worktree/PTY。
当前格式 Run 是绑定这些真实身份的隔离契约夹具，零业务输入/PR；不冒充完整 L2/L3 或用户安装现场。
普通 L1 拒绝该 native opt-in，默认 skip 不计真机通过。结果见[本次专属验收](acceptance/orca-shutdown-verification.md)。
无模型探针使用显式 `STEERRUNNER_ORCA_IPC_PROBE_ROOT/CLI/REPO/BASE` 环境变量，运行
`go test ./internal/execution/adapters/protocol/orca -run '^TestOrcaLocalIPCNativeProbe$' -count=1 -timeout=2m -v`。
ROOT 必须为新的绝对证据目录，CLI 是选定原生 Orca EXE，REPO 是真实已注册 repository ID，
BASE 是该项目已有完整 commit；探针只创建/清理自己的 worktree/terminal 和 nonce 文件。
L1 拒绝该 live opt-in，默认 skip 不计真机通过。

在仓库根目录使用 PowerShell 7：

宿主认证快速回归随 L1 运行，覆盖环境继承/过滤、Git 进程配置、缺登录有界失败、helper metadata 资格、无查询/未就绪停止、reader 结算与关闭意图。旧 broker/socket/CA/capability/租期测试已删除。独立真实无模型入口为 `go test -tags=live ./internal/execution/adapters/codex -run '^(TestLinkedGitNativeCommit|TestHostNativeReadOnlyCommand)$' -v -count=1 -args -host-native-executable=<Codex绝对路径>`；验证 loose/packed refs 提交与 read_only 拒写，不证明真实 push/PR。

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
# 将 example 复制到本地非秘密配置，填写原生可执行文件与已配对 environment。
New-Item -ItemType Directory -Force .evidence | Out-Null
Copy-Item tests/test-config.example.json .evidence/test-config.json
.\scripts\test.ps1 -Level L2 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.smoke -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Scenario local-codex.result-acquisition,local-codex.audited,local-codex.multirun -MaxParallel 2 -Config .evidence/test-config.json
# 全 L3 保留全部注册 case；缺配置的项目不派发，报告 incomplete，退出 1。
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L2,L3 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L4
```

L2/L3 自动先运行 L1 和完整受控前置。任何前置失败都不会启动模型。
L1 每个 package 上限为 12 分钟：独立仓库冻结回归含多次真实 Git clone/fetch，application 包还包含 owner/parser/checkpoint 受控回归。聚合预算不延长各单次调用期限，实际耗时以当前批次日志为准。
完整 Controlled 独立 Go 宿主聚合上限为 45 分钟，入口额外等待 30 秒以覆盖结果落盘；前置必须完整执行并通过。单场景/定向入口保持原预算，CLI、原生准备、故障和业务轮次期限不随聚合预算延长。
PowerShell 交互调用多个场景时可使用数组；外部命令行请传实际数组参数，不把逗号串作为
单个场景 ID。审计配置 `local-codex-audited` 的 settings.model 与 workflow.audit_model.model
须显式且不同，effort 位于各自的 reasoning_effort。独立两模型场景仍保留原会话，不要求模型产生 finding。

有可执行 case 的 L2/L3/L4 自动先运行 L1 和完整受控前置；全缺配置时只报告不可用，不启动这些前置。共享前置失败阻止所依赖的 case；L2 专属原生 Git 检查归其 worker 准备，失败不阻止独立 L3。L4 复用同批已通过 Controlled 的三份冻结二进制，逐项校验摘要、前置零退出与无 skip，仍独占执行；不复用其它构建或历史通过。L4 不需要真机
配置，执行真实 EXE/服务/Git/SQLite 和受控原生协议，不消耗模型 token 或创建真实 PR。
`-MaxParallel` 默认为 1，范围 1–8；L2/L3 可并行，L4 先排空队列后独占执行。调度器还
用当前用户宿主的八个 OS 文件锁槽协调不同批次的场景；L4 占全部槽，L2/L3 各占一槽。
该约束只协调使用本调度器的场景，不限制其它应用或手动诊断。
锁由协调器持有；协调器异常退出会释放锁，但后台 worker/原生服务可能仍存活。必须先按
原现场对账并停止这些资源，再启动需要独占的批次；锁释放不是原资源已清理的证据。
单一协调器写批次 summary，各 worker 写独立 case 记录；worker 丢失结果、构建摘要变化、
原生资源身份缺失/跨 case 碰撞均失败。启动 worker 不算执行通过，未收尾证据继续保留。
`-Scenario` 为精确场景 ID，未匹配、重复或跨级选择失败。每次生成新现场，
不复用上次 Run、请求授权或已经执行过的 manifest。脚本等待完成后返回退出码；
不能以后台 PID 或启动成功作为通过。

L2 原生资格要求 Codex >= 0.159.2，并保留生产协议与权限检查；`configurations.local-codex` 必须提供本机
codex-app-server、development/2、显式 full_access 和绝对 native EXE 路径；不接受脚本包装器或提示词覆盖。
未指定模型沿生产契约使用原生默认，可显式固定 model 和 reasoning_effort。
每个远端、Pi 或审计配置项均填写完整 execution，不继承本机配置。
原生远端不注册有来源证明的审计或完整环境检查正向用例；纯校验、capabilities 和零副作用拒绝由 L1/Controlled 覆盖，不计为真机成功。`orca-remote.multirun` 使用完整的 `configurations.orca-remote-codex-full-access`，同服务两个 Run 独立发布，核验 request/command、会话、工作区、分支、分页事件、last-message 和 PR 不串线；停止一项后另一项原生 owner 保持可用，最后逐一核实原终端停止。远端不校验或伪造 Runner broker/capability 关闭。

L3 远端 Orca 使用指定 CLI、已配对 environment 和远端已有的同名 SteerlineTest Git
项目。两端分别在实际执行主机准备认证，不从调用方复制认证，也不修改全局权限配置。

本机 Orca 的 `orca-local.codex`、`orca-local.pi`
使用 L3 development-feedback driver，覆盖业务 needs_input/reply、发布后反馈拒绝且原来源不变、独立 PR/head/合入，
每个核心现场还用独立短 Run 验证受管 agent 的启动失败收尾，以及真实扣留一次 Orca business-send 接受响应后的 unknown、零重发和原写者停止。正常 Run 另验 request_id 查询/幂等重放和兼容服务换代后的原授权保持、原会话继续；独立实现 Run 验证实际活动写者停止。
本机 fixture 使用 test-only CLI 转发器记录真实原终端 send；重启零输入指纹必须有非零基线，Codex 不再读取不存在的扩展目录。原 Orca CLI 和 TUI 仍是真实执行来源。
DSH 专用与 Pi/DSH 混合场景已退役；历史验收仍保留，不作为当前场景注册。
例如 `./scripts/test.ps1 -Level L3 -Scenario orca-local.pi -Config <配置路径>`；真实结果见[通用接入验收](acceptance/orca-generic-agent.md)。

Orca 配置使用 execution.settings.transport.executable、agent_launch.runtime_executable／runtime_sha256 和 fixture.repository.local_path；
运行期间不依赖 pairing/environment，manifest 冻结 `local_runtime_id` 与 `local_repository_path`。
Pi 读取配置的 agent_launch.executable／model／reasoning_effort，
不继承 Codex 参数。本机原 TUI 和 helper 均在所选执行主机运行。

受管版本为 Pi 0.87.1、Node 24；Windows 工具使用原 owned PowerShell host，开发仅支持 full_access，不依赖 Codex sandbox EXE。独立结果 parser 仍需要其自己的 Codex。
上述是旧 Pi helper 场景。新本机 Pi 由 Orca 原生结构化会话运行，旧冻结实例由原构建排空。新链路的工具、停止及文件边界见 [本机 Pi 真机烟测](acceptance/orca-native-pi-smoke.md)。
Pi 要求原生依赖已安装，Run 不联网安装、不修改全局 profile。DSH 执行和专用历史/用量已移除，不再提供安装/运行入口。

原 TUI 的正式扩展提供开始/完成事实，现有 Orca 只负责资源和投递；不修改 Orca。
保留的 Pi 扩展及共享工具宿主契约测试已纳入统一 L1，可单独运行：

```powershell
node --test internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs
```

`scripts/probe-orca-pi.ps1` 仅诊断 Pi 0.87.1 / Orca 1.4.224 的无工具原 TUI，先运行上述测试。
须提供已在 Orca 登记、位于用户临时目录直属 `steerrunner-orca-pi-p0-<32位UUID>` 的独立无 remote Git 仓库，
以及新的 `-EvidenceDirectory`；不能使用业务仓库或旧现场。脚本只请求固定短答，使用已有 Pi 模型访问，
限定探针输出 256 token，关闭工具和发现，只显式装载 Run 私有候选。它不安装依赖、修改全局 profile、
发布 PR 或放行生产。真实证据见[新实施记录](spec/orca-generic-agent.plan.md#7-约束收敛后的实施pi-p0)；
P0 探针不计为三个已注册 L3 case 或 Runner 真实重启/生产权限通过。

本机/Orca local 开发使用执行主机已有的 Git/gh/SSH 登录，原工具在 full_access 下执行，沿用原账号权限。
GitHub App 支持已移除：`execution.settings.github_auth`（含 null）严格拒绝；fixture 的旧
`github_auth`/`remote_github_auth` 同样在环境/仓库准备前拒绝。远端使用已有 Orca 原生认证。Runner 不再提供 broker/capability 或认证撤销证明。
宿主登录成功不能证明就绪；适用链路的实际认证准备失败仍零业务输入，不回退审批发布。
历史认证资格见[原验收](acceptance/sandbox-authentication.md)，当前 full_access 的实际范围见[实施与验证](acceptance/development-full-access.md)。

权限模式由配置项的 `execution.settings.access_mode` 冻结；开发仅接受显式 `full_access`，内容流程保留显式 `read_only`。旧 sandbox/空模式不得作为正向开发配置。模式迁移、重复 case 合并及旧 Run 边界见 [Spec](spec/development-full-access.spec.md)。
普通开发复用生产 full_access 权限编译；已退役的环境检查不恢复。`execution.settings.agent_launch.args` 只填写支持的启动参数（示例为
`["--no-daemon"]`），不以额外 `-c` 重复覆盖冻结权限。不能把不同模式/配置的运行
合并为同一通过结果。Orca local 的 `agent_launch.runtime_executable` 指向用户安装的受管 helper，核验摘要后沿同权限准备、原控制回执与停止证明；这些保证不适用于原生远端。

远端只使用既有 Orca/Codex/Git/gh 和用户登录，不上传、探测或调用 Runner helper，不转发本机 token、OAuth 或 gh 配置。远端 runtime_executable/runtime_sha256 键（包括空值）在接受前拒绝。model/effort/access_mode 仅核验冻结参数及传递命令；native-launch-contract 明记 effective_parameters_verified=false，native-call-audit 要求零 helper/query terminal，不冒充 OS token、权限审计或实际模型回读。完整远端 Check 和有来源证明的双模型审计明确不支持。

历史远端 GitHub sandbox 任务曾通过有限启动命令校验四个 Git 写根并注入宿主登录；该开发分支已退役，新任务沿现有 full_access 原生登录路径。Git 的 URL header 和 gh token 仅经进程环境传递；沿用原账号权限，未恢复历史 broker 或撤销保证。专项真实沙箱命令和六个管理员故障窗口的结果分别记录；只读身份查询或 push dry-run 不能代替真实 PR 交付。运行时模块为每个故障现场复制独立文件，避免跨现场硬链接与缓存校验争用。参见本轮 [Git 权限修复证据](acceptance/evidence/pr-test-gaps-20261020/git-permissions-fix/)。

原生停止按原 runtime/worktree/terminal/incarnation 的新鲜完整查询判断，缺少执行内结果时保留 unreported/unknown。offline verify-stopped 的 scope 只描述该原终端，不证明全部 OS 后代或远端登录撤销；历史签名/Job 证明不继承到新路径。登录或权限失败按实际操作结果保留，不自动换登录、后端或 full_access。

## 当前完整 smoke 契约

本节以 workflow/2 的 driver 为准，包含实现、发布核验、反馈拒绝、真实合并与停止。L2 仅保留 `local-codex.smoke`；parser 专项属于 L3，不再重复运行开发交付。反馈拒绝复用原 PR，不创建新实施轮或更新 PR。

固定任务改编自 SteerlineTest 的 native-101 Spec/Plan，源 commit 为
`cebdd5ab552150c49743e40593c836689ecf789b`。自有版本位于
`tests/fixtures/evidence-smoke/`，只更新 `evidence/native-101.md`，实际读取 UTC，
运行检查并提交。执行步骤：

L2 使用生产默认的实施/报告/发布提示词，不覆盖 `prompt_overrides`，也不把
`publish.ps1` 发布器放入 L2 夹具；Git 提交、push 和 gh PR 创建由原 worker 完成。
夹具显式保留 packed refs。L2 case 的准备另通过实际 native `command/exec` 验证 loose/packed
head 提交的 stderr 为空，且提交未改动独立 peer 仓库；不使用模型或远端写操作。
worker 命令日志审计会拒绝权限错误，即使命令退出码为 0、提交及 PR 已成功。
文件系统拒绝及 HTTP 401/403 均在审计范围内；宿主认证失败需保留有界诊断，不记录凭据。停止按各后端的现行证据 scope 核实原资源；L2 不启动独立 parser，不生成认证关闭证明。
L3 的固定发布器及故障门保持，用于原有反馈/可靠性场景。

1. 生成 `C:/srt/<随机ID>` 短路径现场（可用 `-FixtureParent` 指定父目录），构建 Runner，
   冻结 EXE/Spec/Plan 摘要、配置、来源与专属 `codex/srtest/<ID>/base`。
2. 克隆固定 SteerlineTest 源提交，提交本轮 checker/publisher 和材料作为测试 base，
   推送这个新 base。远端通过独立准备终端 fetch/cat-file，记录固定 runtime 并确认终端关闭。
3. 通过真实 Runner CLI 提交冻结 Spec/Plan，由 Runner 实施、检查、提交、按需报告、发布、
   核验 PR，到达 `pr_ready`。宿主不代替模型提交或发布业务改动。
4. Driver 从 GitHub fetch 实际 PR Git objects，校验原生 final、PR 身份、仅目标文件变更、
   精确格式和新鲜时间戳。远端同样核验实际 Git 内容和固定 runtime 的原会话。
5. workflow/2 的 pr_ready 没有返工边；保持首次已核验的 PR/head，检查冻结原 final 与程序结果来源；原材料不足时允许一次同源 report，parser 始终为 0。
6. Driver 查询 GitHub 核对原 PR 和精确 head，以 `--match-head-commit` 合入本轮专属测试 base。
   合入后直接查询 GitHub，将平台响应与原 Run 绑定分别保存为 `merge-platform.json` 和 `merge-bound-run.json`。
7. Fetch 合入后的 base，验证 merge commit、原 head 的祖先关系和文件内容，并在实际合入
   checkout 再运行 checker。
8. 显式 stop 原 Run，确认 `new_inputs_allowed=false` 和 `stop_status=stopped`；再停止服务并确认
   `not_running`。保留 Git 工作区、分支、PR 和原始现场供审计。

L2 和普通远端 smoke 均执行首次交付、合入和收尾；L2 在合入前额外验证发布后反馈拒绝，原独立 `local-codex.feedback` 已退役。audited/multirun 按各自清单验证真实 PR，保留未合入并停止原资源。
两模型入口额外生成 `audit-models.json`，从原 session/workspace 的原生 history 按审计
turn_id 和发布 turn_id 校验实际 model/effort，不能仅凭请求配置认定发生了模型切换。

Runner 生产契约的交付里程碑仍为 `pr_ready`。合并属于测试 driver，结束判定包含 Runner
观察 merged 后的显式停止；不引入 SteerLine TaskDelivery，也不新增生产自动合并行为。

## 证据与失败

### 独立前置与显式 L2 复用

入口和资格见 [Spec](spec/validation-reuse.spec.md)、[Plan](spec/validation-reuse.plan.md)及[验收记录](acceptance/validation-reuse.md)。默认 test.ps1 仍运行完整前置；显式复用首轮只支持单级别 L2 和默认 catalog。

```powershell
# 不需要真实模型；L1/Controlled 也可分别选择 Stage
.\scripts\test-prerequisites.ps1 -Stage All -OutDir C:\srt-evidence\prereq-a
.\scripts\test.ps1 -Level L2 -Config C:\srt-config\local.json `
  -PrerequisitesFrom C:\srt-evidence\prereq-a -OutDir C:\srt-evidence\l2-b
```

来源必须有新格式直接成功的 l1-core 和完整 Controlled，且同机同账号/工作副本、输入和实际证据/产物匹配。native-only 或无关说明文档可命中；共享源码、脚本、依赖、catalog/registry、参与 L1 的 docs JSON 改动失效。历史报告不补签，单阶段/focused Pattern 不能拼装完整前置；失配明确拒绝，不自动补跑。

来源后续 native 失败不改写已封存的前置，原失败批次仍 failed。本轮重新评估配置/可用性、新编译/guard driver、新 fixture，完整执行 Git 边界、真实发布/合并/审计及停止。L3/L4、混合级别、自定义 catalog 和 ListAvailability 不接受复用参数，继续原入口。

### 批次报告

批次 `.evidence/tests-*/summary.json` 包含来源、每级结果及现场路径。
报告保存原请求范围、每 case 的 availability／原因，以及每级 registered／requested／excluded／unavailable／executed／passed／missing
集合和 `complete_level_passed`；repeat 单独计数。缺配置的项为 unavailable，零派发，独立的
已配项仍执行；只要请求范围有不可用项，批次为 incomplete、退出 1，实际失败优先记 failed。
显式 Scenario 子集可通过，但不能代表全级通过。本机基线配置完整性与实际同构建 L2 通过分开报告。
前置失败仍保留全部请求项和 unavailable 原因；尚未派发的可用项为 queued，不计执行。
现场 manifest 记录冻结输入；`evidence/` 保存 `result.json`、`scenario.json`、CLI journal、
driver 的 Git/GitHub/checker 调用、实际 PR、合并与停止快照。Runner 自身业务数据和
原生协议日志在该现场 `s/` 中，模型 token 信息以原生记录可提供的真实数据为准。

smoke 通过要求：非零且精确的场景数、无意外 skip、真实产物正确、检查通过、PR 合入、merged
事实核验、原资源和服务均停止。L1 编译 live driver 并验证零场景失败门槛，同时运行入口
的零匹配/缺配置/重复场景反例、Windows
PowerShell checker 和发布夹具的受控反例，覆盖空 PR 列表、复用原 PR、冲突和查询失败
禁止重复创建。L1 只允许脚本逐名列出的 subprocess helper、Orca 取证入口、原生工具环境诊断及用量只读/清理现场入口默认 skip；相应 opt-in 遗留仍在运行前拒绝，任何其它 skip 均失败。这些 skip 不计真机通过。真实宿主认证与 PR 交付须使用新 state/v8 和固定 Runner/helper/extensions，按现有 L2/L3 case 单列验收；缺配置或环境记 incomplete。

未满足冻结 case 预期的失败、超时、unknown、unsupported、缺 final 均不通过；不可达停止
的 `revoked-unverified` 按其故障窗口契约独立判定，不冒充停止确认或业务交付。清理未确认
一律不通过。保留原现场，不自动
重发原生输入或替换会话。`scenario.json` 在外部动作前保存阶段和已知 Run/PR 身份；若
进程异常退出没有结果，批次失败，须用原 state-dir、request_id/Run ID 对账及停止。
修复后创建新现场；旧失败证据不改写。

这里的 unknown 指操作、结果来源或副作用无法对账。原生主代理已经核实当前回合 done、
原正文和来源完整时，可省略的 native outcome 仍按 unreported/unknown 原样保留，不补成
success；它与“原操作结果未知”分别判断，沿用既有 Orca 完成契约。

准备期限只覆盖当前阶段的准备。一个已完成的长业务回合切换报告或发布阶段时，
新输入获得自己的有限准备预算；未决原操作的对账仍沿用原期限，不因重试获得新预算。

单步诊断入口为 `new-test-fixture.ps1 -Scenario ... -Config ...` 和
`test-native.ps1 -Manifest <绝对路径>`。它们供已有前置通过证据的开发诊断使用；正式分级
验收使用 `test.ps1`，确保前置门槛和批次汇总完整。

审计 workflow 的两模型定向诊断复用同一个 native driver。配置中先显式指定执行 `model`，
并填写 local-codex-audited.workflow.audit_model；省略审计 effort 时继承执行 effort。此诊断核验代码审计修复、
执行模型回切及实际 PR 后停止原 Run/服务，保留测试 PR 未合并，不计作普通 L2 完整 smoke。
它不改变默认 L2/L3 场景选择或生产默认发布提示词；该新 Run 冻结测试用发布提示词覆盖。

```powershell
$auditManifest = ./scripts/new-test-fixture.ps1 -Scenario local-codex.audited -Config .evidence/test-config.json
./scripts/test-native.ps1 -Manifest $auditManifest
```

## 独立测试配置与迁移

测试文件使用 [example](../tests/test-config.example.json) 和[外层 schema](../tests/test-config.schema.json)。
`configurations` 中每项为 `{execution, workflow?}`，直接复用公开生产字段和纯 gate；提供
workflow 就必须合法，Run 需要 workflow。需求／角色／固定配置项
引用由唯一 [case 注册](../internal/testkit/livetest/cases.json) 维护，用户无需再填写 case 绑定表。
`fixture` 声明完整 repository／publication 测试源，继续只写专属 SteerlineTest；每 case
生成独立分支／材料／查询取证工具。当前 fixture 使用 GitHub command publication；其他
产品合法 provider 会明确标记不符合 case 要求，生成器不自动改选 provider。query 取证工具
由 case 生成，配置中的认证引用等其余设置保留。`harness` 只含 admin_host；
证据目录沿既有 `-OutDir` 管理。测试输入不进入生产 profile，也不影响外部 submit。

文件只从显式 Config 读取，入口相对路径按调用 cwd，harness 相对路径按原文件目录解析。
批次冻结规范化副本／摘要，worker 使用该副本及同一 Runner 构建；不重读原文件选择后端。
根结构、版本、未知外层字段、重复键错误退出 2；单项无效只使依赖它的 case 不可用。
旧 live-config/v1、v2 明确拒绝；按上述层次手动迁移，不自动继承扁平字段。旧冻结现场仍用原构建结算。

静态列表只判配置是否足以尝试，不查询登录、程序存在、模型或连接。它要求预先产出的待测
Runner 和匹配 helper，不触发构建、L1、服务、调度锁或现场：

```powershell
.\scripts\build.ps1
go build -trimpath -o bin/test-config.exe ./tests/helpers/native
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json -ListAvailability
```

可用 `-RunnerExe`／`-ConfigHelper` 指向已有构建；实际测试使用相同参数可冻结选定产物，
Runner 的 `.build.json` 须随包提供。列表成功退出 0 不代表已通过，缺产物／文件损坏退出 2。
本机 Codex 是独立项目交付基线；缺基线不阻止无依赖的 L1、Controlled 或其他已配置 L3。

## 扩展场景与 L3 稳定性入口

场景注册在 `tests/catalog.json`，新增内容先按本文 L1–L4 定义归类并更新
[测试清单](test-catalog.md)。
L3 扩展须说明独有流程/可靠性覆盖和并行隔离；同一流程的新真实目标须有独立的兼容性
断言。常规可靠性及阶段切换变体保留在 L3，更复杂边界或确实无法隔离的测试才归 L4。
当前全跑 `-Level L3` 包含远端 smoke 及全部稳定性 case，不包含清单中未注册的规划项；
也可用 `-Scenario` 选择子集。报告始终记录实际选择，不将子集通过表述为全级通过。

新的流程在 `tests/native/run_test.go` 的 `drivers` 注册表增加 driver，并在
`new-test-fixture.ps1` 增加对应现场构造。Driver 使用公开 Runner CLI，定义可独立核验的
产物和完整停止判据。增加错误 PR/错误 runtime/陈旧产物等受控反例后，再加入真机目录。
每个 case 都要有自己的现场、Run、base/head 和结果；交付 case 必须核验本轮真实 PR，
停止 case 保存专属分支的无 PR 证据。禁止用模拟目标替代真实环境。

L3 执行稳定性契约见 [Spec](spec/l3-execution-stability.spec.md)、
[Plan](spec/l3-execution-stability.plan.md) 和[实际验收登记](acceptance/l3-execution-stability.md)。
主线 rebase 后的一轮复测及未通过项另见[复测记录](archive/2026-10/acceptance/l3-rebase-20261016.md)。
连续版本冲突的修复与后续来源验证见[不可达撤销修复复测](archive/2026-10/acceptance/l3-revocation-fix-20261016.md)；历史远端认证阻断未计为通过。本轮完整范围复验、后续修正与未通过项见[2026-10-18 记录](acceptance/evidence/full-l2-l3-20261018/README.md)。
选择场景即执行其全部变体，每个 case/repeat 独立建现场。稳定性通过还要求故障命中、
原生调用审计、原身份和完整收尾；预期停止/撤销未知不要求 PR 合入，也不能记成业务交付。

完整稳定性套件需要管理员测试宿主、Node.js、已安装的 Windows Orca CLI 公开运行文件及
已配对的真实 environment。使用 `tests/test-config.example.json` 创建本地非秘密
v1 测试配置并补全 registry 引用的配置项；保持原生自身认证。完整资格入口：

主测试保持普通用户身份，管理员终端单独运行
`./scripts/stability-admin-host.ps1 -Directory C:/srt-admin`，并将配置中的
`harness.admin_host` 设置为该目录的绝对路径。该宿主只接受固定现场的网络 block/block-state/restore，
核验宿主 PID/创建时间与原 manifest；每次操作有独立回执和恢复 watchdog。
宿主最多运行十二小时，可在该目录写入 `stop` 文件结束；先确认本轮故障设施均已恢复。
重复验收请使用新的空目录，避免上次的 `stop` 文件使宿主立即结束。

`orca-remote.connection-reconnect` 同时运行 `active-disconnect` 和 `pre-send-disconnect`。
后者在发布输入提交后、首次状态建连前由测试 helper 设门，通过 `block-state` 只阻断
现场 Runner EXE，保留 CLI 通道；核验原发布 operation 仍处于 in-flight、授权未撤销，
至少一次建连失败后重新读取原终端，且阻断期无新 send。恢复后无需 resume，到达同一
原会话/Attempt 的 `pr_ready` 并只发布一次。结果校验独立读取原门、原生调用日志、
Runner-only 网络 scope 和阻断期快照，不接受仅凭 passed 标志的结论。
L1 另覆盖准备/发送前多次断连、独立查询失败、取消/期限耗尽、认证/身份变化不可重试、
恢复后原 provider 绑定及已尝试输入零重发。新 L3 case 仍需有效的对应配置项和管理员故障宿主
做真机验收；注册和受控测试通过不代表该 case 的真机或并行资格通过。

```powershell
.\scripts\test.ps1 -Level L3 -Repeat 3 -Config .evidence/test-config.json
```

全 L3 每轮范围统一见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)；原远端六场景十二 case 的历史三轮仍为三十六次，不能
与新增本机流程混用计数。L1/Controlled 每批完整执行一次。
`-Repeat` 默认 1，范围 1–10。`-SkipAdminRequired` 仅供用户明确选择排除管理员场景时使用，
报告保存所有排除项及 `passed_with_exclusions`，不能声明完整 L3 通过。当前最终验收要求全部执行。

## 结果获取重构的当前验证边界

当前 parser 的设计与验证见 [解耦 Spec](spec/parser-decoupling.spec.md)、[Plan](spec/parser-decoupling.plan.md) 和 [验收](acceptance/parser-decoupling.md)。Controlled `TestSRResultAcquisition` 保留八项内部 case：成功行、零/正数审计修复、离线审计、一次补报、parser 不可用后备、正常解析和工具事件拒收；不创建真实 PR。旧 parser 原 owner 换代/强杀离线完成及资源停止取证义务已退役；正常业务 owner 的离线审计、恢复与停止仍独立验证。

`local-codex.result-acquisition` 继续为 L3 parser 专项，预期 `parsed`。新 driver 只验证文本分析和取消两项内部 case，不创建 development Run、服务或 PR，不执行权限 canary、工具重启用探针或主执行停止取证。CLI 的 read-only、工具关闭、异常事件拒收和进程释放只由独立 parser 管理。全级 L3 仍按原脚本执行 L1/Controlled 前置。

开发时可定向执行 `go test ./internal/textanalysis ./internal/storage` 及 application 的 `TestTextAcquisition*`；真实 parser 独立入口为 `go test -tags=live ./internal/textanalysis -run '^TestCLIParserLive$' -count=1 -v -timeout=180s -args '-parser-cli=<现有原生Codex绝对路径>' '-parser-model=<已有账号可用模型>'`。PowerShell 中含点号/等号的整个参数须引用，避免模型名称被拆分。它复用现有登录，不引入 API 服务或密钥，不代表完整 L2/L3、PR 或多机资格。

### 历史 helper 验收说明（不适用于新远端）

下述 inline/独立查询终端描述保留给旧构建记录；当前 local 受管路径以固定 control actor 实现为准，原生远端不使用该历史查询链。

受控 Orca helper 的 inline-v2 输入包含唯一 command ID 和完整原输入摘要。普通开发回合通过
执行主机的只读 Codex `thread/read` 核对原工作区/会话、完整实际输入摘要、native turn/user item
及 `final_answer`，用于处理 Orca 漏报短回合启动事件。只读查询不 resume 或发送业务输入；
独立查询终端必须按 runtime/terminal/incarnation 确认关闭后才能采用回执。重复匹配、后继外来输入、
原身份变化或未知查询资源归属均保持未核实，不能用终端画面、静默或 accepted 回执推断完成。
可靠性日志继续要求唯一业务工作区/终端及零业务重发。新增查询终端只在冻结 helper 路径/摘要、
精确 `--original-turn` 命令、原 ready/session/workspace、实际完整输入摘要、terminal incarnation
和确认关闭全部吻合时单独计数；不能仅凭 title 前缀豁免。取消的只读查询不产生业务证据，
仍需确认其资源关闭；最终审计仅在原 case 预算内等待既有关闭，不发出任何补建/重发动作。
`native-resource-audit.json` 的计数还须由原始 native journal 独立复核，不能自报通过。
服务优雅关闭会取消原观察器，并在既有服务关闭期限内等待其查询清理 goroutine 结束；
不等待 actor 的事件确认，也不停止继续运行的远端业务会话。关闭期限届满仍报未核实。
调用端在创建查询终端前持久保存意图，取得原 runtime/terminal/incarnation 后立即保存身份，
关闭后保存回执；这些文件只位于调用端 Run 目录，不把远端 PID 或路径作为本机对象探测。
强杀重启后先清理记录中的原查询终端，完整原工作区清单可证明其已经不存在；仅原 incarnation
仍存活时可继续关闭。创建结果未知且没有资源身份时继续阻断，不能凭 title 或补建推断成功。
业务终端关闭已确认、查询清理未确认时分别保存进度，恢复只继续原查询资源清理。
已受理回合首次历史查询沿用 30 秒观察间隔；原会话的完成通知可提前触发查询，但通知本身
不作为完成证据。此间隔和后续查询均占用原 case 预算，不延长超时或改变故障命中条件。
工作区创建不切换 Orca 当前视图。新 worktree 已有终端时继续使用原终端；若连续完整清单
确认没有终端，在原准备预算内等待其异步出现后，仅允许一次首个终端创建。调用端先持久保存
创建意图、再保存返回的原 runtime/worktree/terminal/incarnation，重启只核对原身份；未知创建
不补建。返回后仍要求原工作区仅有该终端，才进入原 agent-start 和无模型权限准备步骤。
原生审计将这一可选首次创建单列为 initial_terminal_creations（0 或 1）；business_creations
仍要求只有一个业务工作区。首次终端必须有完整空清单、精确固定参数、原身份及后续 ready
证明，不能按 title 单独豁免；第二个业务终端创建、身份变化或不完整清单均失败。

### PR 平台证据与 Run 快照

普通 inspect 读取 Runner 持久快照。workflow/2 在 pr_ready 后拒绝业务反馈。需要当前平台状态时，验收直接读取原 PR，并与原 Run 中固定的 PR、仓库、base/head 分支和 head SHA 核对。合入验证同时检查平台时间、提交和 Git 对象。

2026-10-19：远端 helper 接线删除后的本次验收以 [原生方案](spec/orca-remote-native.plan.md)及[专属记录](acceptance/orca-remote-native.md)为准。该旧构建当时保留本机 Check；当前已全部退役，见本文件开头的版本边界。原生参数仅核对冻结值与传递命令，离线停止输出的 scope 只描述原 Orca 终端，不证明全 OS 树或撤销登录。


## 运行时单文件汇报增量

新增五项 progress-file driver L3，场景/隔离/准确命令见测试清单的同名专项。该增量登记时为 L3 25 场景/35 case，总 30 场景/40 case；Controlled 当时 25 顶层，原有项全部保留。正式脚本自动执行 L1、完整 Controlled；可用 -MaxParallel 6 同时调度一项 L2 与五项 progress L3，结束后运行 scripts/verify-progress-parallel.ps1，核对同一构建五项全通过、资源隔离及各自 120 秒实际原生观察窗口重叠。source、快照、结果、清理及失败历史见 [专项验收](acceptance/runtime-progress.md)。不要用默认跳过、零选择、Controlled 或旧构建结果代替本次 native/remote 通过。

PR #55 审计补充：`TestSRProgressFile` 在同一服务中停止一项后修改其源文件，再次换代服务；必须保持原终态快照逐字段不变，活动同伴继续更新且零额外原生输入。快速回归同时核对已收尾不重开、待收尾仍采样、停服终态 closed、活动/恢复 detached，以及未启用 Orca 通道的环境隔离。`tests/framework/progress-parallel-test.ps1` 纳入正式框架入口，验证有效窗口及四种不合格报告；合成窗口只证明核验器行为，不授予 L3 或实际并行资格。

2026-10-23：远端 Orca Codex 最后消息沿现有 smoke、multirun、connection-reconnect drivers 核验公开 CLI 身份、摘要边界及重复读取时间；不新建 catalog case。`TestOrcaNativeStateLiveLifecycle` 的原 opt-in 入口记录普通通知和 query-only 的实际样本、独立消息/完成事件及原资源关闭。缺 prompt 的查询场景若真实出现，应保持未核实且零重发；若来源保留 prompt，只登记匹配路径通过。Controlled 覆盖空 prompt 连续链、标记变化、断线/重启和诊断重附着。详见[Spec](spec/orca-last-message.spec.md)、[Plan](spec/orca-last-message.plan.md)及[原验收](acceptance/execution-observability.md)。


## result-file 本地与受控增量（2026-10-26）

该增量新增 Controlled `TestSRResultFile`，当时 Controlled 注册 25 项。本轮只选取 `TestSRProgressFile|TestSRResultFile`：保留原 progress 回归；result 验证四种启用组合、实际环境清理、运行中 pending、结束后任意字节/原子替换、停止后封存及服务重启后不刷新。result 仅结束时采集，版本只有 0/1，不包含持续观察、交错采样或观察重开测试。

定向单元覆盖封存失败/预算/原绑定、初始化未知只读对账、resume 不刷新、旧业务/progress/result 保存保护及持久化失败。完整 content/resource recovery 长流程不作为本功能默认回归；受控和真实远端资格分别登记，真实远端和部署留待后续。详见 [验收记录](acceptance/runtime-result-file.md)。

原生输入正式 focused 入口的远端准备与业务共享现有本机同等六分钟 fixture 总期限，不单设90秒准备上限。原 READY 只读查询、原冻结输入/intent、一次通用提交、原输入/完成及正式停止均保留；生产等待参数不变。双端各三个真实会话与失败边界见[最终远端验收](acceptance/orca-native-submit-remote-final.md)。

P2 观测解耦的定向组件入口：go test ./internal/application -run 'TestRuntimeFiles|TestProgress|TestResultFile|TestSRStopSerializes|TestSROperation|Test.*Preparation' -count=1 -timeout=4m。公开 EXE 回归使用 test-foundation.ps1 的 TestSRProgressFile、TestSRResultFile、TestSRRunWorkflow、TestSRNativeUnknownAndStop、TestSRRestartPreservesAuthorization；TestSRResultFile 内增加四启用组合的目录故障→原 PR 闭环→明确停止，不改变原结果资格回退。

最小真机探针复用 new-test-fixture.ps1 的 development-smoke driver：-ObservationFiles 启用两个健康通道，-ObservationFault 在新 fixture 的专用 base 提交两个同名普通文件，使通道目录无法创建。二者互斥且只接受 development-smoke；原 test-native.ps1 核验 PR、原生审计和停止，额外断言 progress、pr_ready 时 result 仍 pending、明确停止后 result 一次封存。原场景 ID/超时/业务断言保持；这些定向探针不授予完整 L2/L3 或发布资格。结果与命令见[独立验收](acceptance/personal-executor-p2.md)。

P2 Orca local 补验：完整 L1 使用 `./scripts/test-prerequisites.ps1 -Stage L1 -OutDir <新目录>`，沿用原 12 分钟每 Go 包预算，包含 framework、全部 internal 包、原生扩展和 driver 守卫。定向组件的短预算不应直接套到全包累计执行；包总超时须结合 JSON 中的包启动和当前子测试启动时间判断，不据栈所在函数推断其卡死。本次没有增加超时、排除慢包或删除断言。

本机受管 Codex 的 bootstrap READY 不等于 TUI composer 已可接收原输入；发送前沿原终端做既有 `tui-idle` 查询，只用于输入就绪，最终结果仍取原完整输入/原生回执。测试权限审计从 cursor=0 原始 stream 读取完整回执；画面折行文本不能用作审计输入。控制关闭证明的键使用原 actor 的稳定 workspace，允许只读核实当前 v6 的同一资源空 workspace 旧键，foreign canonical 拒绝仍保留。新旧现场、补验构建与结果见[验收报告](acceptance/personal-executor-p2.md)。
## 本机 Orca Codex 停止条件收敛（2026-11-01）

定向组件、公开受控和 `TestOrcaStoppedNativeProbe` 的显式隔离原生探针见[专项验收](acceptance/orca-local-codex-stop-simplification.md)。新 scope 覆盖登记终端/helper/根身份，不依赖签名退出文件或 control Closed；直接 Codex、Pi/DSH、原生远端保留各自范围。探针使用真实 readiness 和原资源，Run 为隔离注入夹具，业务输入为 0，无真实 PR，不计完整 L2/L3。未设置显式探针环境时的 skip 不计真机通过。

execute/1 复用正式 L1（含 SingleExecution 契约/原生 final/停止/拒绝额外输入测试）及 Controlled 的 `TestSRResultFile` 最终封存回归。2026-11-02 起，已有 `TestSRSingleExecutionColdStop` 也纳入默认 Controlled，覆盖公开 EXE 的原请求离线查询、PendingStops 冷启动停止、原输入一次与 result-file 封存；入口与范围见[当前清单与资格边界](test-catalog.md#当前清单与资格边界)。跨仓消费者的真实单次调用另由 SteerLine 的 testing.project-environment-local 验证，不能以受控 peer 替代；Runner 自身测试无需 SteerLine checkout 或新测试框架。
