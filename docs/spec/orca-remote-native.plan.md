# Plan：远端仅通过 Orca 执行

日期：2026-10-19。状态：P0 通过，P1–P4 已实施；L1 通过，Controlled/P5 验收进行中，最终结果以专属验收为准。依据 [Spec](orca-remote-native.spec.md)，调查基线 `a3dc15dc2f90787ea6e38e78d4dbc8a275470646`。结果登记在[专属验收](../acceptance/orca-remote-native.md)。

## 1. 最小改动范围

| 位置 | 必要变化 |
| --- | --- |
| contracts/run、内嵌 settings schema/capabilities、application/freeze、agentlaunch/config | 远端拒绝 helper 字段、沿用 envelope 写 profile v4、直接启动；local 保持 v3 |
| orcatransport 的 profile/lifecycle/agent/authentication/native_state/stop/verify_stopped | 复用原生命令、准备/完成/停止；隔离旧资源 |
| protocol/orca 的 client/stopped/state_feed/state_subscription | 优先原样复用；仅修正原身份/回执/清单验证实际缺口，不改 Orca 协议 |
| contracts/contracts、application/control/verify_stopped | 仅逐 Run 重验输出只读 scope；停止事实沿既有 receipt，RunView 使用已有 diagnostic |
| application/environment_check、远端 Check session 入口、内嵌 Check capabilities | 新远端完整检查 unsupported；旧记录可查，不重启 actor；local 保持 |
| 现有 livetest/native/integration、fixture/config/catalog/scripts | 去掉远端 helper 上传/审计前置，调整证据断言，复用 driver |
| 文档及 SteerLine 消费者 | 新契约、配置、scope、部署与 Check 支持声明同步 |

保留确有 local 调用的 agent EXE/broker/terminal tools 代码，不批量回退 PR #18/#27；不能因此保留远端 helper 接线。workflow/reliable、backend、RunView/runstate 结构、公共协议大版本与数据库表不改。实施前固定实际源码/依赖基线，不以调查 HEAD 冒充未来二进制来源。

### 必须删除或收窄的接线

| 当前入口 | 实施要求 |
| --- | --- |
| freeze/agentlaunch 的 runtime 参数与摘要编译 | 删除远端传递与编译；共享 local 调用保留，不增加 v4 helper 分支 |
| agent/authentication 的 helper readiness，remote_receipt 的 reader/inspection journal | 从远端启动/输入/完成/恢复链删除；确由 Orca local Codex 使用的实现限定 local，v3 旧远端不得调入 |
| stop/verify_stopped 的远端签名、停止文件与 reader 结算 | 从远端链删除；新路径只用 Orca 原清单，旧远端只返回原持久结论或未核实，不加载旧 helper 运行栈 |
| application/environment_check 到 RemoteEnvironmentSession 的远端 actor | 新旧远端均阻止执行；local Check 实际使用的 session 代码保留 |
| remote-runtime-upload、fixture/ready/audit/stability 的远端 helper 准备与查询 | 删除远端上传/探测/历史审计依赖及失效配置/测试；无 local 使用的工具/分支删除 |

交付记录给出实际删除清单、剩余共享函数的 local 调用者和远端调用回归；只写“native 分支未调用”不满足删除要求。禁止增加 legacy 远端 adapter、兼容服务、通用模式/证据注册器或新 helper 包装层。

## 2. P0：先核验真实 Orca 能力，再删除生产接线

首轮 [P0 实测](../acceptance/orca-remote-native-p0-20261019.md)证明核心链路并发现查询漏项；[本轮补验](../acceptance/orca-remote-native-p0-query-fix-20261019.md)已修复并取得完整远端快照。参数按用户指示仅承诺按配置传递，不新增实际生效核验；P0 按此范围通过。必须先确认没有 helper 的实际通路足以支撑本 Spec 承诺的功能；源码存在函数、历史 helper L3 或当前只读连接成功都不能代替此门槛。

复用 `TestOrcaWorkspaceStartupLive`、`TestOrcaNativeStateLiveLifecycle` 及现有 Orca 协议调用，必要时只调整探针/fixture 去掉 helper 准备依赖；不新增后端、模式开关、长期 harness 服务或生产绕过。使用新的独立测试工作区/终端，直接启动原生 Codex；不调用当前普通 submit 中尚未删除的 helper 门，不把绕过该门的测试冒充完整 Run 通过。

| 项 | 最小实际验证 | 必须记录的事实 |
| --- | --- | --- |
| P0-01 启动/身份 | 原 environment 创建固定 base 工作区，直接启动 Codex；核对冻结 model/effort/access_mode 正确编译、传递且不会被额外参数覆盖 | Orca/Codex 版本、原 runtime/worktree/terminal/incarnation/pane/provider；请求配置与实际启动命令；不把请求参数标成已核验生效，不新增回读要求 |
| P0-02 原回合/反馈 | 原终端两个短回合，包含同会话反馈；复用 query-only 探针模拟没采到 working | 原接受/开始回执、发送前基线、新主代理结束、完整 final 与摘要；旧 done/短回合不会串轮 |
| P0-03 实际认证/发布 | 原代理用实际工具做最小文件变更、commit/push，在本次测试 base/head 间 create/query/close 一个临时 PR | 无本机凭据注入，Git/gh 实际操作和平台独立核验；宿主 gh auth status、READY 或模型自述不足；不合入 main/业务分支 |
| P0-04 重连/停止 | 只关闭再打开本机原观察连接，查询原资源且不重发；在活动短任务中经 Orca 精确 close 并核验完整清单 | 原身份不变、零重发、活动到原终端退出/缺席及完整 host scope；只证明 Orca 资源结束，不证明全 OS 树/登录撤销 |
| P0-05 能力盘点/零 helper | 对照实际支持和本次仍承诺的 workflow、发布、反馈、恢复、停止与检查范围逐项归类；审查探针全部调用 | 保留/明确收缩/新缺口清单；零 helper 上传、版本探测、reader、actor、broker 或替代程序；不借核验扩展未支持的远端多模型/其他代理 |

P0 不重跑稳定性矩阵：一套原生探针资源、一个临时 PR，结束后精确清理测试引用/PR/终端；发生未知只对账原对象，不重建补测凑成功。不修改共享 Orca、登录、安装目录或故障 Run。先核验连接/原文，失败便停止后续写探针，保存失败与清理事实。模型、测试仓库写操作及临时资源在实际执行 P0 时按明确执行任务的授权操作，本轮查询补验只调用真实原生只读接口，零远端资源和业务输入；只实施该查询缺陷修复，不提前删除 helper 接线。

已有 `TestOrcaStoppedNativeProbe` 的 helper 正向退出结果不能作为 P0-04 的无 helper 停止证明。P0 直接读取真实 Orca close/完整清单，最终 Runner 的离线入口仍由 P3/P5 验证。原生层无法观测的认证预检/全树/撤销保证须明确列为本方案已收缩，不可按“全功能等价”计通过。

退出条件：P0-01–05 的本次原始证据、能力差异和自有资源清理齐全，足以支撑 Spec 仍承诺的最小远端功能，才能进入 P1。新缺口先报告和修订范围，不临时加 helper、降低 final/身份标准或静默删功能。缺环境、未执行、skip 或结果未知均不能放行删除。现有已明确的边界不重复变成确认门槛；P5 仍验证最终生产实现，不用探针替代。

## 3. P1：配置、冻结与直接启动

依赖 P0 通过；该可行性门槛现已满足。当前按用户授权实施 P1–P4；删除及最终验证事实登记专属验收。

1. schema 按 adapter/target 禁止新远端 runtime 路径/SHA 键（含空值）；纯配置校验、submit、capabilities 一致，`invalid_input` 必须先于 Run/Git fetch/Orca 创建副作用。
2. 新远端 freeze 使用原 envelope 的 profile codec v4，payload 只有 transport/路由；local 仍 v3，state v6（最新主线既有版本）、target ID 和 input digest 定义不变。不加 execution_mode。旧程序拒绝 v4 是预期。
3. adapter 入口一次区分 v4 远端、v3 local 和只读旧 v3 远端；前两类复用正常生命周期。服务加载旧远端 Run/Check 时先阻止自动观察/调度，不因此使服务整体加载失败；其查询及未结算资源事实保留。拒绝未知版本、目标冲突、helper 配置/资源混存；不把版本判断扩散到 workflow/reliable。
4. 复用 agentlaunch 编译直接 Codex/model/effort/access-mode 命令；沿原启动 intent/接受回执，未知启动只对账，不重复发送。
5. 不探测/安装/上传远端 runtime，不新增诊断终端或替代脚本；无工具 READY 只绑定会话。

退出条件：版本/冻结/混存反例、直接启动和 local 受管配置 L1 通过，远端 helper 编译接线已删除且无调用。对应 ORN-A01/A06。

## 4. P2：认证与完成

依赖 P1。

1. 从新远端执行链删除 GitHub/helper readiness、认证租期和同 app-server 预检门及其接线。保留 workspace/base、原资源/会话就绪和发送前基线；local 受管规则保持，旧远端不再进入执行链。
2. model/effort/access_mode 只验证冻结值与编译命令一致、禁止配置覆盖，不新增实际生效查询、字段或准入条件。使用远端已有模型/Git/gh 登录与请求权限；既有 PolicyCapabilities/ReportToolPolicy 和诊断只声明实际启动参数及原生来源，不沿用 host-managed/broker/限仓库/撤销保证。不新增能力类型/字段。不传本机秘密，不用宿主检查或模型自述冒充同工具预检。
3. 新远端唯一完成路径为普通 wire prompt、NativeProof/observeNative；删除远端到 remoteWireCommand/observeRecorded/recordedTurn/inspector/journal 的接线，相关共享实现仅在有 local 调用时保留。不为兼容旧远端数据保留可执行路径。
4. 首次原生远端业务提交加已有 `--wait-submit 20`，仅有界等待同一请求，不建立重发/重试调度；超时缺开始继续未核实。保留订阅/独立查询、原请求开始、主代理结束、完整 final、7999 UTF-16/surrogate 截断拒绝、首份原文不变和会话/外部输入边界。P0 发现的 `worktree.ps limit=100` 漏项已先修复并补验：按 totalCount 扩展 limit，同一连接/40 秒预算内最多三次，只采用计数一致且未截断的单份快照；持续截断/增长/缺元数据保持未核实，不把当前页缺席当资源消失，不新增查询服务。
5. 原生 outcome 与正文分开：新核实的缺 outcome 记 unreported；显式 unknown、失败/取消和未知输入继续阻断。仅对有正常采集、完整 final 来源且无 Failure 的 unreported 调整 NormalizeResult 准入；保留 workflow 业务路由及审计/PR 门，历史 unknown 不重新解释、不迁移。同步上层 CLI v1 字符串契约。
6. 认证/权限拒绝保留原阶段 blocked/failed，未知操作 reconciling；不放宽 full_access、不重发、不修复登录。保留 PR 独立核验及原分支/PR 未知发布对账。

退出条件：原生事实和认证反例通过；Controlled 真实 CLI/SQLite/Orca 协议夹具完成 workflow，并拒绝旧 done、截断、缺开始、错会话/原文冲突和失败结果晋升。对应 ORN-A02/A03/A05。

## 5. P3：停止、离线重验与旧记录

依赖 P1。

1. 仅在 contracts.StoppedRun 与离线输出增加只读 `stop_evidence_scope=orca_terminal`，从校验后的 v4 profile/目标推导。backend、RunView、runstate、stop receipt 不增字段；普通停止说明沿用 diagnostic，不新增持久化 scope 或缺 scope 授权规则。
2. 新 native stop 先撤销输入，经原 close 加完整原终端清单确认；复用 VerifyTerminalStopped/originalTerminalInventory 的 runtime/worktree/incarnation/host/计数/截断校验。仅 ptyKilled 不足。
3. close 丢回执只读原清单，资源结束可结算范围内事实，不伪造回执、不重复 close；清单滞后仅有界重读，活动/断线/身份冲突保持 unknown。
4. Stopped/WorkerAbsent 不设全树 WritersAbsent；删除远端签名退出/停止文件/认证撤销依赖。stopped 快捷分支仍须原身份及既有已提交停止回执，缺证据只读重验；scope 不参与授权/恢复，也不进入持久化比较。
5. 离线复用相同清单规则并返回 scope，保留 writer lease/实例围栏和只读库；不启动 Engine/观察/helper/终端、不写原 DB/历史回执。聚合不排除旧未知 Run/Check。
6. 服务退出只停本地观察；调用方退出不关业务；换代后新输入仍需显式 resume。
7. 旧远端 v3 只查持久快照、复用原已提交结论；显式 stop 仅从记录提取核验原终端身份，经 Orca 精确关闭，不将 PTY 事实补记为原 helper 已结算。拒绝自动观察推进/resume/后继/Check actor 对账；缺证据保留未核实，由原构建或人工处理。不为旧运行保留签名/reader 验证执行栈，不迁移、不降级、不重派。

退出条件：清单/停止/丢回执/只读 scope/混合旧记录/EXE 离线反例通过，无 scope 跨层持久化；local 证明不回退、旧远端不误结算且无 helper 兼容引擎。对应 ORN-A04/A05/A06。

## 6. P4：环境检查、调用方与测试接线

依赖 P1–P3。

1. 新远端完整 Check 以可查询拒绝返回 unsupported，零工作区/actor/模型/PR；capabilities 只收缩该目标的完整 Check，不阻断普通 Run。旧 Check inspect/已有资源结算不重启 actor。
2. 修改现有远端 fixture、config example/白名单、manifest/ready/audit/driver：不要求 helper 路径/SHA、不上传/探测、不以 broker/Job/签名/reader 当通过门槛。保留 runtime/base/final/Git/PR/清理证据；local fixture 不变。
3. 已有普通开发 driver 改为 Orca 原生取证，保留 smoke/full-access 场景供各自后续使用，本轮只选 full-access。远端完整 Check 的 helper 正向 live 场景从支持/case 清单移除，拒绝行为放入 Controlled；不新增 unsupported live driver/outcome/manifest。两份 catalog、case registry 和现有计数断言同步，不改写历史结果。
4. 活动停止复用 `orca-remote.stop-revocation/active`，由显式配置冻结 full_access 并在 manifest 核对；不新增 native-full-access case、场景或调度入口。必要新增的 Controlled 断言放入现有测试并登记到统一前置，避免另建同义测试族。
5. README、主 Spec、认证/连接/停止/环境检查和 testing 文档同步，保留历史证据及 local 要求。
6. SteerLine preset/schema/请求去 helper 字段；Run 展示已有 diagnostic，逐 Run 重验展示只读 scope；部署沿现有 resources_settled，不建立 scope 策略。旧未知仍阻断，Check unsupported 不伪装通过、不阻断普通 Run。仅消费 CLI，不读私有库。

退出条件：公共配置/capabilities/拒绝/普通 submit、注册展开与 driver 编译通过，消费者定向与跨仓契约已登记；否则 ORN-A08 未完成。

## 7. 测试计划

扩展既有配置、native_state/input_reconnect/verify_stopped、service/environment-check integration 夹具；只为缺口增加断言/子例，不预建三套 Native 测试族或模拟业务后端。新增子例同步进入既有 Controlled 注册。零匹配/skip 不能记通过；登记实际非零测试/断言、退出码、源码与构建身份。

| 验收项 | 层级/场景 | 环境、时点与命令 | 通过判据 |
| --- | --- | --- | --- |
| A01/A06 | L1 配置/冻结/启动 | P1 后；`go test ./internal/contracts ./internal/application ./internal/execution/agentlaunch ./internal/execution/adapters/orcatransport -count=1 -timeout 20m` | 旧键含空值零副作用拒绝；v4/摘要正确；v3 远端不入执行；local v3 合法；接线删除及零 helper |
| A02/A03/A05 | L1 认证/原生完成 | P2 后；`go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1 -timeout 120s` | 短回合、补查、截断/surrogate、错身份/外部输入/冲突/失败 outcome；无权限回退或未知重发；local 回归 |
| A04/A06 | L1 停止/离线/旧证据 | P3 后；`go test ./internal/application ./internal/host ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -run 'TestSROffline\|TestSRShutdown\|TestTerminalStopped\|TestCodexStopped' -count=1 -timeout 180s`，原测试中增加 v4 子例 | 清单完整性、丢回执只读对账、只读 scope 不等于全树；旧缺证明拒绝；离线零新执行/写入 |
| A01–A07 | Controlled EXE/SQLite/Orca 协议夹具 | P4 后；现有 `scripts/test.ps1 -Level Controlled -OutDir <新目录>` 统一构建 Runner/协议夹具并独立运行，不裸跑缺 EXE 输入的 integration | workflow/final、认证拒绝、未知输入、换代显式恢复、混合旧记录、只读 DB 摘要不变、scope 不落库、Check unsupported 零资源 |
| A06 | 既有本地锁/managed 回归 | P4 后；`scripts/test-foundation.ps1 -Pattern '^TestSRServiceOwnership$' -OutDir <另一新目录>` 及相关 local adapter L1 | 独占/锁、原 owner/tree、offline 证明和 local 读取不回退；未运行 live opt-in 不记通过 |
| A07 | 公共能力/注册/driver | P4 后；`go test ./internal/contracts ./internal/resources ./internal/cli ./internal/testkit/livetest -count=1 -timeout 120s` 及既有 test-framework 检查 | schema/capabilities/远端 Check 拒绝/普通 Run 一致；注册、driver 编译、零匹配保护通过 |
| A08 | SteerLine 定向及跨仓契约 | P4 后；在实际消费者仓库执行 `go test ./internal/app ./internal/runnerclient ./cmd/steerline -run 'TestDeployment\|TestServiceSettlement\|TestStoppedVerification\|TestOrcaNative' -count=1 -timeout 180s`，及该仓实际部署停止夹具 | 无 helper 配置；scope 展示；Check unsupported 不挡 Run；旧未知仍阻断；记录实际 commit/非零测试数 |
| A09 | L3 full_access 闭环 | P5；既有 orca-remote.full-access/default | 实施、发布、pr_ready 拒绝反馈且无后继输入、独立核验/测试 base 精确 head 合入及 Orca stop；零 helper |
| A04/A09 | L3 活动停止 | P5；既有 orca-remote.stop-revocation/active，配置 full_access | 活动期重验拒绝；显式 stop 后原清单/scope 成功；零后继输入；不伪造树/认证证明；清理精确 owned 资源 |

不为每个负例创建真机 PR，不重做完整 L3/L4、不增加网络故障平台或全模式矩阵。统一入口的 L1/Controlled 前置跑一次；已通过且无新变化的组不重复运行。所有结果登记专属 acceptance。

## 8. P5：两个现有 case 的一轮原生验收

依赖 P4。最终实现固定源码/Runner EXE/resource digest、Orca/native 版本及非秘密 mode/model/effort。使用新的本机调试目录、独立短 state-dir、测试仓库/base、原 environment 与精确原生资源，不操作安装环境或故障 Run。只在本机放 Runner EXE，不部署远端 Runner 文件、不更改共享 Orca/登录/代理安装。

审查 driver 全路径调用并保存实际 Orca 调用日志，确认无 helper/version probe、上传、reader/actor/broker/替代脚本。远端历史安装目录中恰有 helper 不影响判定；本次必须零访问/零调用。证据脱敏，不保存 token/完整环境。

完成 P4 fixture 修订后，先经统一入口跑一次 L1/Controlled，再使用现有 fixture 的 `-Case` 精确选择两项。当前 `-Catalog` 强制保留每场景全部注册 case，不裁剪、不新增 case 或 scheduler。配置须显式 access_mode=full_access，两个 manifest 都核对实际冻结模式；不能在同一配置下将 smoke 误标成 sandbox。下面示例使用本轮独立配置和最终构建；实际执行结果与增量补验见 acceptance：

```powershell
$ErrorActionPreference = 'Stop'
$taskConfig = (Resolve-Path '.evidence/orca-native-rebase/test-config.json').Path
$taskRunnerExe = (Resolve-Path '.evidence/orca-native-unreported-20261019/steerrunner.exe').Path
$taskConfigHelper = (Resolve-Path '.evidence/orca-native-rebase/test-config.exe').Path
if ((Get-Content -LiteralPath $taskConfig -Raw | ConvertFrom-Json).configurations.'orca-remote-codex-full-access'.execution.settings.access_mode -ne 'full_access') { throw 'This qualification requires explicit full_access' }
$taskPrerequisites = Join-Path (Get-Location) ('.evidence/orca-native-prerequisites-' + [Guid]::NewGuid().ToString('N'))
.\scripts\test.ps1 -Level Controlled -OutDir $taskPrerequisites
$taskCases = @(
    @{scenario='orca-remote.full-access'; case='default'},
    @{scenario='orca-remote.stop-revocation'; case='active'}
)
foreach ($taskCase in $taskCases) {
    $taskManifest = & .\scripts\new-test-fixture.ps1 -Scenario $taskCase.scenario -Case $taskCase.case -Config $taskConfig -RunnerExe $taskRunnerExe -ConfigHelper $taskConfigHelper | Select-Object -Last 1
    & .\scripts\test-native.ps1 -Manifest $taskManifest -TimeoutMinutes 30
}
```

直接 native 入口不补做 L1/Controlled，必须引用上述同源码/构建的完整前置记录。一个开发 case 使用独立测试分支/PR，只将精确 head 合入本轮测试 base，不操作产品 PR/业务 main；一个活动停止 case 不发布 PR。每项独立 Run/Attempt、工作区/终端、state-dir 和证据目录，共享只读工具/runtime。

sandbox 参数、认证不可见和零权限回退由 L1/Controlled 覆盖，不另设完整 sandbox PR 闭环为本次交付门槛；未真机验证的范围如实保留，不借 full_access 结果声明 sandbox 通过。失败补验用独立现场、不覆盖原结果，只补受影响 case。主机不可达/清理未知不宣称完成；分别登记业务/停止/清理，不用 Orca 范围 stopped 证明后台进程全灭或长期登录撤销。

## 9. 交付门槛

交付须完成 P0、查询修复及 P1–P4，并满足 A01–A09；实际结果只在专属 acceptance 登记。非零测试、一个 full_access 闭环和一个活动停止、消费者接通与历史隔离完成才能声明交付。只完成 Runner 端时如实保留调用方缺口；旧构建 helper 测试不能代替新证据。

实施完成后更新 acceptance 的精确来源/EXE/工具/协议身份、命令、case 数、首次失败/补验与 scope/收尾事实。文档检查不计功能通过。本轮用户已授权删除接线及验收；不修改安装目录，不卸载旧 helper，不处理原故障 Run。


### Rebase 后补验

用户要求先 rebase 到最新主线，当前基线 95ae8a7。保留 workflow v2/result acquisition 与 steerrunner-test-config/v1，不恢复已删除的 live-config/v2 示例或入口。沿新独立配置的公开验证/availability 选择现有 case，两个 case 均使用显式 full_access 的原生配置；active 的既有角色要求绑定同一 full-access 配置，无新增 case/模式。最新固定 helper control 与签名/认证收尾代码仅 local 可调用，原生远端的测试收尾使用原清单与公开 offline scope。

重新运行统一 L1/Controlled、当前 EXE 的消费者契约及上述两个真实 case；rebase 前结果保留为历史，不作为新构建通过。源码、版本、配置与要求 digest 使用最终现场新值。公开版本仍为 steerrunner-cli/v1；主线上游资源/工作流版本按其 v2 契约保留，不在本任务降级。


## 用户确认的 unreported 最小修正

aaf5480 的显式 unknown 阻断保留。只有首次新核实的缺 outcome 原生结束记 unreported；既有 unknown 和历史结果不改。NormalizeResult 允许正常采集且完整来源、无 Failure 的 unreported 进入既有业务核验；不新建结果服务、执行模式、字段或状态机。

测试计划增量（ORN-A03/A05/A08/A09）：

| 层级 | 时点/命令 | 通过条件与证据 |
| --- | --- | --- |
| L1 | 实现后 go test ./internal/execution/adapters/orcatransport ./internal/execution/reliable ./internal/execution/workflow ./internal/application -count=1；应用仅选择 TestRAUnreportedOutcomeBoundary 等受影响项 | 新缺失/显式 unknown 区分；失败、取消、缺开始、缺来源、未知输入拒绝；历史 unknown 序列化恢复后不晋升；workflow 成功/业务阻塞各保留；新独立 .evidence/orca-native-unreported-* |
| 消费者契约 | SteerLine runnerclient / storage 定向测试及实际 EXE published contract | unreported 和 unknown 原样往返及存储不可篡改，业务状态与 native outcome 独立；无调度/部署修改 |
| Controlled | 复用既有 TestSRNativeUnknownAndStop 及已通过的统一 Controlled，按源码增量补验 | 未知输入无重发，原 stop 仍结算；不重复无关已通过组 |
| L3 | 最终 EXE 新 fixture，orca-remote.full-access/default 和 stop-revocation/active | 实施、原生结果采集、发布、pr_ready 拒绝反馈且无新输入/新 head、测试 base 合入与 stop；原调用日志零 helper；分别收尾，失败不覆盖 |

保持此前统一 L1/Controlled 的历史证据，末轮按变更补验；所有成功/首次失败、源码/EXE/driver 身份和实际数量登记 acceptance。

旧远端 state v5 仅显式 stop 提取 Process；兼容读取范围严格为 profile v3 remote + state v5/v6。其他旧版 state、v4 remote + state v5、local + state v5 均不因此开放。保持原 envelope 身份/摘要校验，未恢复 reader/control/认证运行栈。


当前主线 workflow v2 的完整正常路径到 pr_ready 后不接受 feedback；P0/旧基线的正向反馈只作历史传输能力证据。当前 A09 保留主线边界，复用 full-access/default 补充公开 reply 拒绝、revision/PR/head/turn 不变和实际 native send 数不变断言，不恢复旧反馈路由，不新增 case。

### 最终审计修正

历史 StageResult 可同时保存 `Succeeded=true` 和显式 unknown。新结果的 NormalizeResult 门不能覆盖恢复后的 parser、报告来源与发布前置条件；这些入口必须核对原 Outcome，不被新报告的成功值晋升。仅复用一个私有准入判定，并在 RouteParsed 保留原 Outcome；业务边、快照结构、旧空值语义和恢复授权不变，无迁移或新模式。

复用 workflow 的 `TestRAHistoricalOutcomeCannotBePromoted` 覆盖序列化恢复后 unknown/failed/cancelled/未知值与 succeeded/unreported 正例，检查直接路由、parser、报告与发布前置条件。审计后执行 workflow/reliable/adapter/protocol/contracts/resources/livetest 全包、消费者契约、Controlled 的 ResultAcquisition/NativeUnknownAndStop，以及最终 EXE 的既有 full-access/default。应用层选择 `TestRA|TestSRFreezeOrcaAgent|TestEnvironmentCheckLegacyRemote`，预算 20m；180s 的首次预算不足记录保留，不能计通过。新增日志、最终构建身份及实际结果登记专属验收；活动停止复用未改停止代码的上一轮真实证据，明确其构建差异。
