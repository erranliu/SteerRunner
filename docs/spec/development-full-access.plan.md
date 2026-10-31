# Plan：开发执行统一 full_access

日期：2026-10-31。状态：P0–P4 已实施；P5 最终 L1/Controlled 与原生无模型专项通过，真实 L2/L3、read_only/parser 真机资格因缺配置未完整执行。依据 [Spec](development-full-access.spec.md)，调查基线 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`；实施阶段结果见 [实施与验证](../acceptance/development-full-access.md)，当前审计修复后的验证见[审计记录](../acceptance/development-full-access-audit.md)。

## 1. 实施原则与顺序

按 P0→P1→P2→P3→P4→P5 顺序实施。权限、认证、恢复和测试消费者要在同一可交付变更中闭合，不能发布只切默认值但保留 sandbox 正向配置/运行分支的半成品。本 Plan 不授权执行部署、修改真实用户配置或发布业务 PR。

| 阶段 | 产物 | 对应 Spec |
| --- | --- | --- |
| P0 | 消费者、删除/保留清单、旧记录样本、版本兼容表 | DFA-02～05 |
| P1 | 显式权限准入、schema/catalog、客户端候选与迁移诊断 | DFA-01、05 |
| P2 | 开发启动/工具权限收敛，保留认证与只读路径 | DFA-02～04 |
| P3 | 旧记录分类、恢复与幂等边界 | DFA-04、05 |
| P4 | 测试矩阵、脚本/fixture/注册及文档同步 | DFA-06 |
| P5 | 最终构建回归、真机专项与验收登记 | DFA-07 |

## 2. P0：固定删除与保留清单

以生产调用点为依据记录每项消费者，搜索限源码、当前配置/文档；历史 evidence 单独保留。实施前固定代表性样本：本机与 Orca local/remote 的 sandbox、缺省、显式 full_access 冻结 Run，内容 read_only、parser、活动/停止/结果未知和无原 worker 的记录。样本不含真实凭据。

| 文件/模块 | 删除或修改 | 必须保留 |
| --- | --- | --- |
| `internal/contracts/run.go`；`internal/resources/{configuration,catalog}.go`；`assets/settings.schema.json` | sandbox/空模式准入、默认值、开发能力组合 | 原形状严格性、组合 gate、内容 read_only、能力来源与摘要 |
| `internal/application/{freeze,policy}.go`；Codex `profile_config.go` | 默认 sandbox、开发沙箱权限资源编译；Pi/DSH sandbox EXE 发现 | 冻结身份、工作区检查、Git/认证资源的既有绑定 |
| Codex `policy.go`、`native_params.go`、`params.go`、`launch.go`、`policy_resources.go` | workspace-write 分支、开发 MXC/写根授权和无效兼容默认 | full_access 固定参数；read-only 编译；参数防覆盖和有用途的资源解码 |
| Codex `permissions.go`、`worker_runtime.go`、`sandbox_readiness.go`、`terminal_authentication.go` | 仅沙箱写授权和 MXC 正例；沙箱专用临时/提示补丁在确认无其他用途后删除 | Git admin/temp/socket 路径校验、broker/environment、full_access 无模型准备及有限 stage |
| `internal/execution/agentlaunch/{config,linked_git,github_auth}.go` | 原生 sandbox 的 linked/auth 启动包装及调用；无剩余消费者才删实现文件 | 当前 full_access 原生登录、仓库/工作区身份的非权限校验、模型/参数冻结 |
| `internal/execution/terminaltools/tool.go`；Orca `terminal_runtime.go`、`terminal_receipt.go`、extensions | Codex sandbox CLI 包装、EXE 字段/探针、sandbox 分支与拒写证明 | 工具宿主/Job/原生 TUI、脚本路径和 provider 环境过滤、认证/停止证明 |
| `internal/execution/nativeaudit`；testkit `sandbox_audit.go`/`receipt_audit.go` | 作为当前开发资格的 sandbox 审计入口和模式默认 | full_access/never 的已有原生事实审计、越权请求/参数覆盖反例；历史数据本身 |
| `internal/application/{engine,worker,takeover,run_observation,progress,result_file,native_session,verify_stopped}.go` 及各 adapter recovery/codec 的实际入口 | 历史 sandbox 可继续执行、后台采集/PR refresh/原生会话 open 的资格 | 持久事实只读投影、身份完整时精确 stop、兼容 full_access/read_only 接管 |

不能整块删除 `permissions.go`、认证模块、process/owner、workspace 或 resultacquisition。对 `LinkedWorktreeWriteArgs`、`WritableRoots` 等共享表示，先列调用者：保留认证/资源身份所需检查，停止将它们编译为开发沙箱授权；不为单纯改名迁移持久数据。

P0 核对并落实 Spec DFA-05 的固定版本表：新开发 policy v11、只读沿 v10；Codex profile 3 / policy resources 4 形状不变；Pi/DSH 新 tools v2、receipt 3、metadata 4、local profile 5；原生远端 profile 4 不变。用真实基线结构构造有限旧 decoder 样本，不能把兼容规则全部推迟为“实现时决定”。本机/Orca readiness、owner 客户端和 terminal receipt 校验须按原模式/契约分流，不能继续拿单一当前 `PolicyContract` 或 `terminaltools.Contract` 比较所有旧回执。未变的外层 state、workflow 和 CLI 协议保持。

完成条件：每个删除项都有消费者去向；每个保留项说明非沙箱用途；旧记录如何进入查询/停止路径有落点。此阶段不开展认证重构或引入新宿主隔离。

## 3. P1：公开准入、配置源与迁移

1. adapter schema 要求显式 `access_mode`。本机原生枚举为 full_access/read_only，Orca 为 full_access；workflow 规则进一步限制开发/full_access、内容/read_only。同步静态 settings 与动态 run_configuration/run_input schema，移除错误 default 声明。
2. `ValidateExecution`、RunConfiguration、完整 Run input、normalize/freeze 和 capability matching 同步拒绝 sandbox、缺省、空值/null/错误类型。生产 gate 保留既有错误优先级，模式错误提供无秘密迁移诊断。catalog 仅枚举原支持链路上的 full_access 开发组合。
3. `client/steerrunner-client.ps1` 发现本机和 Orca 时输出显式 full_access；读取已保存默认/项目配置，旧模式报错并引导修改配置源。更新 `skills/steerrunner-configure`、`skills/steerrunner-run` 的实际文件和帮助示例，不自动覆写真实安装目录或配置。
4. 更新 `tests/test-config.example.json` 及测试配置 schema/基线诊断。可编辑配置迁移仅替换 mode 和退役角色引用，不改变仓库、模型、目标环境、发布账号或 transport。两份旧配置的值不同需明确选择，不添加自动合并器。
5. 为幂等重试固定反例：已受理旧请求仍返回原关联，改正文沿原规则冲突；不经过“新请求校验”把原请求重新受理成 full_access。修改配置提交新任务使用新 request_id。

最小验证：schema/gate/catalog 表驱动交叉矩阵；客户端发现与旧配置加载的受控用例；被拒新 submit 的 Run 创建、工作区/执行文件准备、进程启动、网络/认证调用计数为零。该断言不禁止既有服务日志和请求去重读取，不把正常诊断写入当作权限副作用。离线 validate 的零外部副作用要求保持；资源目录变化更新 digest。通过后再进入真实启动改动。

## 4. P2：权限实现与认证边界

1. Codex 开发 `NativePermissions`、argv、ThreadStartParams、TurnStartParams 和 policy resource 新构建只产 full_access，保留 never/reviewer、模型及预算校验；低层接口也拒绝 workspace-write，避免绕过公开 gate。删除开发 `sandbox_workspace_write.*`、MXC 和 `--add-dir` 写授权。
2. `LaunchServer`、managed command、remote agent 不再为了开发授权扩展 writable roots。认证 transport/temp 路径仍从已校验资源得到，保留路径归属与替换反例。`worker_runtime.configArgs` 中的 socket lease 验证保留，其后用于开发授权的 `--add-dir` 删除；Git 环境、代理信任、期限和关闭流程不更换。更新 `WorkerInstructionFormat`：去掉 AppContainer 临时目录措辞及 MXC/AUTO_MERGE 拒写例外，保留精确暂存/检查/提交、退出码处理、认证失败停止及未知发布不重做；临时目录仍使用原 Run 已绑定路径。
3. 保留 `sandbox_readiness.go` 在原 GitHub broker 适用路径的 full_access 执行：非 AppContainer、真实工具环境、Git roots、remote、身份、仓库与 transport；认证准备不是已删除的能力。仅移除 workspace-write 选择和 MXC 正例；诊断 stage 兼容优先于纯命名整理。保留原 `Authentication == nil` 及各后端仓库支持边界，用非 GitHub 本机路径反例证明没有新增 gh/broker 前置；不改现有认证选择规则。
4. Orca 原生启动只走现有 full_access 分支；清理 `linkedGitLaunch`/`gitHubAuthenticationLaunch` 的 sandbox 专用包装，确认其他工作区/目标验证仍在原层完成。保留执行主机原生登录，禁止新增远端 helper/broker 或有效权限回读门槛。
5. Pi/DSH 删除 `SandboxExecutable` 新配置字段和 `codex.cmd`/native sandbox 版本探测；`terminaltools.Command` 固定直接启动现有 Windows PowerShell。保留 Config 的工作区/状态/脚本路径约束及工具进程树管理；非隔离条件下这些校验不宣称阻止 Agent 访问宿主。
6. Go/JS、metadata/冻结 gate、profile 编码、receipt 生成/校验、RecoverySupport 的 codec 声明和扩展 bundle 摘要同步采用 DFA-05 新版本。receipt 3 删除 `outside_write_denied`，要求 `outside_write_verified=true`；已有 full_access 的有限 outside canary 正向检查继续在独立测试/临时路径执行并收尾，不新增任意宿主写入探测。旧 profile 3 / receipt 1/2 的 full_access 走有限旧结构读取并验证原 tools v1 contract、false 拒写字段及冻结摘要；新 Pi/DSH local profile 5 / metadata 4 / receipt 3 不接受旧字段。受管 Codex 的 local profile 3 形状未变，继续使用，按 readiness 中冻结的 v10/v11 policy contract 验证。原 remote profile 4 判断与旧远端 helper 退役判断也须同步，不能因增加 local profile 5 错把它路由为 remote 或旧 helper。
7. 只读路径逐项核对：`content_permissions.go`、`content_readiness.go`、`connection.go` 的创建/恢复限制，以及 `resultacquisition/codexexec/owner.go` 的 read-only 和禁工具参数。保留其原生 sandbox/MXC；开发 helper 的 Codex 依赖解除不包括用户启用的独立 Codex parser。

最小验证：原生参数与工具命令快照、参数覆盖拒绝；Pi/DSH 无 sandbox EXE 时可准备/执行工具；broker、socket、worker_runtime 和 terminal auth 原正反例；read_only 创建/恢复和 parser 参数隔离。缺登录/期限/环境/绑定时仍在业务输入前阻断。

## 5. P3：恢复、退役与升级边界

1. 在创建/恢复 actor、生命周期或补派意图前，按原冻结 workflow+mode 分类退役开发 Run。Request、profile/resource 中的模式若矛盾或缺失，不推断 full_access；损坏身份沿原不支持规则阻断，不能补默认。复用单一无副作用分类函数，供 takeover、驻留服务和只读投影调用，不新增持久恢复状态表。
2. 分类至少覆盖 `Engine.New/Start/launch`、`worker.advance` 最前方、直接控制、PR refresh、Observe/inspect、progress/result 采集、native-session open 和 verify-stopped。当前 `advance` 在取得正常派发 gate 之前就可能核验 PendingPRURL，`launch` 会先启动进度采集；退役检查必须早于这些动作，不能只加在 `TakeoverFacts` 或生命周期构造处。退役项拒绝 reply/resume/推进/新 parser/发布，仅查持久事实和按原停止协议执行精确 stop；无法证明清理时保留原 blocker。
3. 合成基线 v10 full_access、旧 local profile 3 / receipt 1/2 和 read_only 的兼容样本，验证按原契约连接原 owner/terminal、原摘要不变且未知操作不重发。新增错模式、错/缺 contract、错 bundle、旧 outside 字段为 true、新旧字段混用、缺失 owner、认证过期等反例。新版本 decoder 不回落到新 argv/current bundle，也不能重新登录或启动旧 helper 修复兼容。
4. 混合状态目录包括旧 sandbox、兼容 full_access/read_only、不支持但身份可识别的记录，验证服务启动、所有读取投影与兼容 Run 可用；退役项零自动 PR/provider 查询、文件采集、lifecycle/parser/资源创建。以计数器验证，并比较原持久授权/Status/InputDigest 未变。旧回合可能仍在原 owner 执行，退役投影不能伪造 stopped、认证关闭或删除 pending stop；已有停止只对账原停止身份。
5. 写入操作说明：原版本停收新任务→完成/精确停止受影响 Run→核实资源结算→更新可编辑配置和同版组件→新构建 validate/接管清单→新请求。保留原版本和证据，不删除在用 helper/bundle，不在线改写数据库或 manifest。

最小验证：takeover/recovery/幂等/停止 Controlled；逐项比较旧 Request/InputDigest/profile/Attempt 与原证据未变。兼容性的声明限实际验证的契约，不承诺全部旧构建。

## 6. P4：测试与当前文档调整

### 6.1 删除、替换、保留

| 测试/入口 | 调整 |
| --- | --- |
| 权限及配置 L1 | 正向开发夹具统一显式 full_access；新增 sandbox/省略/空/null/非字符串/覆盖 args 拒绝；read_only 正反例保留 |
| `permissions_test.go`、`policy*`、native params/launch/profile tests | 删除 workspace-write/MXC 开发正例；保留 Git 资源身份检查并改为无沙箱授权断言 |
| `authentication*_test.go`、socket/broker、remote auth、terminal auth | 保留凭据/代理/期限/绑定/失败零输入及清理；真实工具准备改为 full_access；删除开发沙箱外拒写预期 |
| `authentication_native_test.go`、`TestLinkedGitNativeCommit`、`scripts/run-test-case.ps1` | 原生诊断从 sandbox 切到 full_access；L2 前置仍验证 private linked repo、packed refs/锁、commit 及归属/收尾；删除 MXC/AUTO_MERGE 拒写资格，保留普通 Git 失败处理 |
| `terminaltools/tool_windows_test.go`、extensions `.test.mjs` | 移除 sandbox command/拒写正例；验证直接 PowerShell、无 Codex sandbox 依赖、provider 环境过滤及取消/后代退出 |
| nativeaudit、`tests/native/sandbox_audit_test.go`、receipt audit | 当前资格只审 full_access 原事实；保留 never/禁止提权请求及固定参数；原 remote 有效参数仍只承诺传递 |
| Controlled `TestSRPermissionQualification`、`TestSRInputFreeze`、`TestSRNativeUnknownAndStop` 等 | 修正正向模式；保留状态机/停止/未知零重发与零副作用准入断言 |
| 内容、generate repository、parser 相关 L1/Controlled/专项 | 保留只读写入拒绝、禁工具/MCP 隔离、恢复与取消；开发 full_access 不改变其边界 |
| `internal/testkit/livetest/{config,manifest,cases,receipt_audit}.go` | 基线 full_access；去掉模式默认和按 scenario 改写配置；mixed peer、完整请求及能力匹配同步 |
| `cases.json`、`tests/catalog.json`、fixture/runner/framework scripts | 按 Spec DFA-06 合并两组场景和两项 active restart 重复 case；所有剩余 primary/peer role 引用 canonical full_access 配置 |

明确退役映射：`orca-local.pi-full-access/default`→`orca-local.pi/default`；`orca-remote.full-access/default`→`orca-remote.smoke/default`；active restart 的 `*-full-access` 两 case→对应无后缀 case。迁移表仅供人工与覆盖审查使用，运行时不做别名重定向。合并前逐条比对原断言，新增覆盖不得因旧场景删除而丢失。正常/强杀及 active/boundary 四种窗口仍保留。

集中核对 `scripts/new-test-fixture.ps1`、`live-catalog.ps1`、`test-configuration.ps1`、`run-test-case.ps1`、native-result 与 framework 消费者；固定构建/配置/registry 摘要及选定请求集。缺配置仍 unavailable/incomplete，退役 ID 明确未知，不能用缩小集合使旧“完整通过”继续成立。L1/Controlled 不要求真机登录。

### 6.2 文档与静态残留

同步 README、CLI 指南、测试指南/目录、workflow 开发说明、skill 帮助及对应 Spec/Plan 的“当前行为”条款；认证专项注明开发 sandbox 已退役，full_access broker 与准备仍生效；内容文档仍只读。新增专项验收记录并加入验收索引。

不重写 `docs/archive/`、`docs/acceptance/evidence/` 和历史验收事实。最终源码搜索中的 sandbox/workspace-write/MXC 每个剩余命中应归入：read_only/解析器、协议字段名、旧记录被动识别、拒绝反例、历史证据或仍保留的诊断名称。不能留开发 sandbox 正向执行分支；不要求机械地零字符串命中。

## 7. P5：验证与交付

先跑与改动对应的包和 framework 测试，修正后运行一次完整基础回归：

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
```

准备迁移后的独立测试配置，再构建一次最终 Runner/helper。以下命令在仓库根目录运行；`.evidence/test-config-full-access.json` 是实施时准备的测试配置，不指向用户安装配置。测试产物目录须全新，复测保留原目录并更换目录名：

```powershell
$taskTestConfig = (Resolve-Path -LiteralPath '.evidence/test-config-full-access.json').Path
$taskArtifacts = Join-Path (Get-Location) '.evidence/full-access-final'
if (Test-Path -LiteralPath $taskArtifacts) { throw 'Final artifact directory must be new' }
$null = New-Item -ItemType Directory -Path $taskArtifacts
$taskRunner = Join-Path $taskArtifacts 'steerrunner.exe'
$taskHelper = Join-Path $taskArtifacts 'test-config.exe'
.\scripts\build.ps1 -OutFile $taskRunner
go build -trimpath -o $taskHelper ./tests/helpers/native
if ($LASTEXITCODE -ne 0) { throw 'Test configuration helper build failed' }
$taskLiveArgs = @{ Config=$taskTestConfig; RunnerExe=$taskRunner; ConfigHelper=$taskHelper }
.\scripts\test.ps1 -Level L3 @taskLiveArgs -ListAvailability
if ($LASTEXITCODE -ne 0) { throw 'Availability query failed' }
.\scripts\test.ps1 -Level L2 @taskLiveArgs
if ($LASTEXITCODE -ne 0) { throw 'Local L2 qualification failed or incomplete' }
$taskScenarios = @(
    'local-codex.audited',
    'orca-local.codex', 'orca-local.pi', 'orca-local.dsh', 'orca-local.mixed',
    'orca-remote.smoke', 'orca-remote.service-restart',
    'orca-remote.stop-revocation', 'orca-remote.publication-unknown'
)
.\scripts\test.ps1 -Level L3 -Scenario $taskScenarios @taskLiveArgs
if ($LASTEXITCODE -ne 0) { throw 'Selected L3 qualification failed or incomplete; preserve per-case results' }
```

本专项的最小真机资格包括：本机 L2 的 Git/PR/broker 收尾；audited 的执行/审计权限；Orca local Codex/Pi/DSH 原会话交付及活动停止、mixed；远端 smoke、正常/强杀×active/boundary 重启、stop 与未知发布；以及下述只读/parser 专项。上述所选项缺配置或未执行须报告专项未完整验证；其他未选 L3 case 仍不计通过，专项完成不等于完整 L3 通过。L1/Controlled 已通过且源码未变时，分级脚本自动重复的前置沿既有入口处理，不另行手工反复运行。

静态清单、L2/L3 均显式传入同一 `RunnerExe/ConfigHelper`，保存每次实际摘要及 registry/config 快照。列表脚本本身不构建默认 bin；不能用未构建的默认路径查询，也不能各阶段隐式重建后只记录一个“最终构建”。专项脚本若自行构建自己的原生测试 Runner，须核对相同最终源码/资源身份并单独记录实际构建摘要；不能把专项 EXE 和上述产品 EXE 混成同一个文件。

真实验收还包括原生工具无模型准备/认证清理：沿调整后的 `TestAuthenticationNativeToolBoundary` 验证 full_access 下的 broker/API socket、无效 capability 拒绝及退出清理；不再要求认证目录外拒写。read_only/parser 通过 `scripts/test-content-workflows.ps1 -NativeExecutable <绝对原生Codex路径> -Model <测试模型>` 和 `scripts/test-generation-recovery.ps1` 的同名参数验证，另保留固定仓库调查现有专项。此处尖括号只表示说明中的待填参数，实际命令须替换为已解析路径/模型变量。专项须记录受控/真机分项及实际执行数量，缺少 `NativeExecutable` 时不能以只跑 Controlled 证明只读真机资格；这些入口不伪装成通用 L3 registry case 计数。Pi/DSH 工具宿主专项用不含 Codex sandbox EXE 的隔离依赖配置证明解除依赖；不卸载机器上的 Codex。parser 启用时仍需其自己的 Codex。

交付登记须包含：

- 最终 commit、Runner/helper/bundle/resource digest、开发权限及 receipt 版本；迁移和退役项清单。
- L1/Controlled 结果；新 registry 的请求/可用/未配置/执行/通过集合，合并前后覆盖映射。
- 本机真实交付、受影响后端原会话/PR/停止及本地 broker/socket 收尾；远端只按原 `orca_terminal` scope 声明。
- read_only 创建/恢复/拒写和 parser 禁工具边界的实际验证结果。
- 缺环境、失败或未验证项；只跑子集时不能称完整 L3 通过，历史 full_access 成功也不替代最终构建。

完成条件：Spec DFA-01～07 均有实现/验证落点；开发 sandbox 执行分支和重复权限资格删除；认证与 read_only 回归通过；当前文档无相反承诺。代码完成、选定后端真机资格和完整测试资格分别登记，不用文档审阅结果代替运行证据。

## 8. 本轮文档审计（2026-10-31）

本轮为主会话对照源码的单轮审计及直接修正，不是独立多代理复审，也不计代码或真机验收。

| 优先级 | 确认的问题与影响 | 文档修正 |
| --- | --- | --- |
| P1 | 退役规则未列出早于正常派发 gate 的 PR 核验、启动即运行的进度采集与公开读操作；仅修改 takeover 会遗漏副作用，且“只读”可能被误作原 worker 已停 | DFA-05 / P3 固定统一分类的完整入口、原停止核验例外、无副作用计数和仍存活原执行的边界 |
| P1 | 共用 v10 policy 常量及当前 argv/terminal contract 全等比较，会使只读和旧 full_access 一起失效；新旧终端字段缺少明确版本区分 | DFA-05 固定 policy/codec/metadata/receipt 版本表；P0/P2/P3 列出原契约 decoder、冻结摘要和禁止缺字段回落的反例 |
| P1 | 认证表将 GitHub broker 前置泛化到全部本机任务，会给已支持的非 GitHub 路径新增 gh/GitHub 依赖 | DFA-03 / P2 明确原适用条件与无 broker 路径回归，不扩张或重选认证 |
| P2 | 验证样例用 PowerShell 不可解析的尖括号路径；静态清单缺预构建，分次默认构建也不能保证同一产物；缺少明确审计/停止/未知发布资格 | P5 改用可解析变量/参数表、一次构建并固定产物，补最小真机矩阵与专项实际摘要要求 |
| P2 | “submit 文件调用为零”范围过宽，会误禁正常诊断或去重读取，混淆纯校验和有状态提交 | P1 将零副作用限定为新执行资源/网络/认证创建，保留现有日志和幂等读取 |

修正后进行 Spec↔Plan、相对链接、Markdown 空白和 PowerShell 示例静态语法复核；未运行产品测试，未据此宣布实现或运行资格通过。
