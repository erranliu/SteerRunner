# SteerRunner L1–L4 测试指南

测试源码、夹具、现场生成、执行和结果判定全部归属本仓；不要求 SteerLine
checkout、EXE、数据库、配置或测试调度器。运行目标为 Windows/amd64。

本文件说明分层定义、设计与维护规则、执行入口和结果判定。
逐项测试内容、预期结果、并行条件及当前缺口见独立的[测试清单](test-catalog.md)。

## 分层定义

按测试目的、流程和隔离条件分层，不以是否使用真机、模型或远端环境作为层级定义。

| 层级 | 定义 | 设计要求 |
| --- | --- | --- |
| L1 | 快速的单元测试或集成测试 | 快速反馈规则、组件、协议、存储和依赖边界的正确性；现有默认入口不启动真实模型或创建真实 PR |
| L2 | 代表最小核心流程的 Smoke test | 用最少的固定流程判断核心链路是否可用；扩展流程和边界变体放入 L3/L4 |
| L3 | 可并行的多种不同流程及核心可靠性测试 | 覆盖开发、反馈、审计、远端执行，以及可预期故障下的可靠执行；各项独立、互不干扰，主要验证目标尽量正交 |
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

`Controlled` 是现有脚本的受控集成前置入口，不是 L1–L4 之外的第五级。它使用真实
EXE/CLI/IPC/SQLite、隔离 Git 和原生协议/provider 夹具，可在本地隔离 push，不创建真实
PR。目前固定 16 个顶层场景，覆盖独立包、服务归属、材料冻结、权限资格、完整工作流、
存储/日志/事件、PR 身份、工作流完成前的发布事实、原生未知结果与停止、重启后显式恢复、
丢响应找回、停止授权跨重启保持、审计发现/修复/预算及同服务多 Run 隔离。
审计场景的子变体单独在日志中登记，批次 `scenario_count` 按顶层测试计数。
是否纳入快速 L1 应按耗时和依赖评估。

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
[目录校验](../scripts/live-catalog.ps1)核对两者的层级、driver、target、超时和完整 case 集，
[Go 注册测试](../internal/testkit/livetest/cases_test.go)与
[脚本检查](../tests/framework/stability-test.ps1)固定 L3 的十一场景/十七 case 数量；原 11 个
可靠性 case 保留。四个 L4 场景绑定精确的受控测试名，不以受控结果代替真机资格。
未来新增场景或收敛 L2 时须同步这些约束、manifest 校验和结果报告；L2 中的反馈验证
只有在独立 L3 反馈 case 注册、批次可选择且有通过证据后才能移除，不能先删后补。
当前 `-FeedbackQualification <case.json>` 可选择无反馈的 `development-core` driver：
它要求正式反馈 case 为 passed、协调器批次通过、真实新 head/合入/清理完整、Runner 与
native driver 摘要及执行配置匹配，资格文件摘要冻结进 manifest。也可在非秘密配置的
`feedback_qualification` 填写该 case 记录的绝对路径；未提供资格时保留原完整 smoke。
耗时和模型使用按实际测试目的安排：快速规则/路由反例放 L1，复杂确定性分支使用受控
集成，真机测试验证真实后端接线与核心行为，不强迫模型制造可重复的 finding。
文档规划不表示入口已经支持新 case、L4 或并行调度。

## 当前执行入口

在仓库根目录使用 PowerShell 7：

认证 socket 的快速回归随 L1 运行（`TestAuthenticationSocket*`），覆盖原安装布局的 124 字节旧路径与 90 字节新路径、冻结资源身份、各 Run/attempt 的隔离、目录 junction/替换、socket 替换、错误 capability、部分准备失败及精确清理。定向原生入口 `TestAuthenticationNativeToolBoundary -sandbox-auth-native <codex.exe> -sandbox-auth-host-broker` 使用独立长路径 Git 夹具和只读 GitHub 查询，检查实际 MXC 工具连接、共享认证父目录写入拒绝及退出清理，不运行模型或发布 PR。

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
# 将 example 复制到本地非秘密配置，填写原生可执行文件与已配对 environment。
New-Item -ItemType Directory -Force .evidence | Out-Null
Copy-Item tests/live-config.example.json .evidence/live-config.json
.\scripts\test.ps1 -Level L2 -Config .evidence/live-config.json
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.smoke -Config .evidence/live-config.json
.\scripts\test.ps1 -Level L3 -Scenario local-codex.feedback,local-codex.audited,local-codex.multirun -MaxParallel 2 -Config .evidence/live-config.json
# 全 L3 含稳定性 case，必须另备 v2 配置及管理员故障宿主，并填写两种不同模型。
Copy-Item tests/live-stability-config.example.json .evidence/live-stability-config.json
.\scripts\test.ps1 -Level L3 -Config .evidence/live-stability-config.json
.\scripts\test.ps1 -Level L2,L3 -Config .evidence/live-stability-config.json
.\scripts\test.ps1 -Level L4
```

PowerShell 交互调用多个场景时可使用数组；外部命令行请传实际数组参数，不把逗号串作为
单个场景 ID。审计场景配置须提供显式 `model`、不同的 `audit_model`，可配置
`audit_reasoning_effort`。独立两模型场景仍保留原会话，不要求模型产生 finding。

L2/L3/L4 自动先运行 L1 和完整受控前置。任何前置失败都不会启动模型。L4 不需要真机
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

L2 当前原生资格固定 Codex 0.159.2；配置 `codex_executable` 为实际 native EXE。
本机 `model` 留空使用原生默认模型；可显式固定模型和 reasoning_effort。远端可单独设置
`remote_model`、`remote_reasoning_effort`、`remote_args`；`remote_model` 显式留空表示采用
远端 Codex 的原生默认模型和 effort，与手动运行 `codex` 一致。省略该字段才继承共用 model。
L3 远端 Orca 使用指定 CLI、已配对 environment 和远端已有的同名 SteerlineTest Git
项目。两端分别在实际执行主机准备认证，不从调用方复制认证，也不修改全局权限配置。
本机 GitHub worker 默认使用执行主机已有 gh 登录的受限认证代理。OAuth 只在可信
runtime 内存中，worker 只继承随机单仓库 capability，一小时到期、退出即失效。
Git smart HTTP 经进程级代理及公开 CA，gh 经独立公开配置的 AF_UNIX 接口；原工具
仍在 MXC 内执行，原 GitHub URL 保持。socket 位于已有 Git 写入根，证书不安装到系统。
代理仅允许目标仓库的 Git 上传/下载、身份/仓库只读查询和 PR 查询/创建；禁止其它
仓库、管理接口、未知 GraphQL 及重定向。可选 `execution.settings.github_auth` 使用
App 的非秘密 `app_id`、`installation_id`、`private_key_env`，不要求 App 才能测试。
宿主登录成功不能证明就绪；实际沙箱门槛失败仍零业务输入，不回退审批发布。
参见[本次认证验收](acceptance/sandbox-authentication.md)。

权限模式由非秘密配置 `access_mode` 冻结，默认 `sandbox`；`full_access` 为显式选择。
普通业务与环境检查复用生产权限编译。`remote_args` 只填写支持的启动参数（示例为
`["--no-daemon"]`），不以额外 `-c` 重复覆盖冻结权限。不能把不同模式/配置的运行
合并为同一通过结果。当前 Orca terminal 缺执行主机认证及同权限确定性预检能力，
两模式均保持 unsupported；配置 `full_access` 不补齐该缺口。
出现交互权限确认、原生拒绝或 shell 初始化失败时保留失败；手动诊断中的单次确认
必须另记介入证据，不能记作无人值守通过。

## 当前完整 smoke 契约

本节记录当前 driver 的必跑契约，仍含一次反馈和真实合并；[测试清单](test-catalog.md)
中的 L2 最小化为后续调整目标，当前执行步骤或结果判定尚未改变。

固定任务改编自 SteerlineTest 的 native-101 Spec/Plan，源 commit 为
`cebdd5ab552150c49743e40593c836689ecf789b`。自有版本位于
`tests/fixtures/evidence-smoke/`，只更新 `evidence/native-101.md`，实际读取 UTC，
运行检查并提交。执行步骤：

L2 使用生产默认的实施/报告/发布提示词，不覆盖 `prompt_overrides`，也不把
`publish.ps1` 发布器放入 L2 夹具；Git 提交、push 和 gh PR 创建由原 worker 完成。
夹具显式保留 packed refs。L2 前置另通过实际 native `command/exec` 验证 loose/packed
head 提交的 stderr 为空，且另一个仓库仍不可写；不使用模型或远端写操作。
worker 命令日志审计会拒绝权限错误，即使命令退出码为 0、提交及 PR 已成功。
文件系统拒绝及 HTTP 401/403 均在审计范围内；默认 host broker 的可信收尾事实还须
包含拒绝计数且为 0、closed 和 socket cleanup 已确认，不能用隐藏输出掩盖拒绝。
L3 的固定发布器及故障门保持，用于原有反馈/可靠性场景。

1. 生成 `C:/srt/<随机ID>` 短路径现场（可用 `-FixtureParent` 指定父目录），构建 Runner，
   冻结 EXE/Spec/Plan 摘要、配置、来源与专属 `codex/srtest/<ID>/base`。
2. 克隆固定 SteerlineTest 源提交，提交本轮 checker/publisher 和材料作为测试 base，
   推送这个新 base。远端通过独立准备终端 fetch/cat-file，记录固定 runtime 并确认终端关闭。
3. 通过真实 Runner CLI 提交冻结 Spec/Plan，由 Runner 实施、检查、提交、报告、发布、
   核验 PR，到达 `pr_ready`。宿主不代替模型提交或发布业务改动。
4. Driver 从 GitHub fetch 实际 PR Git objects，校验原生 final、PR 身份、仅目标文件变更、
   精确格式和新鲜时间戳。远端同样核验实际 Git 内容和固定 runtime 的原会话。
5. 发送一次真实反馈，生成更新的 UTC 记录，检查新 commit、原 session/attempt/branch/PR 复用。
6. 刷新原 PR，精确绑定 head，driver 以 `--match-head-commit` 合入本轮专属测试 base。
   Runner 通过 `inspect --refresh-pr` 保存真实 merged、merge commit 和时间。
7. Fetch 合入后的 base，验证 merge commit、原 head 的祖先关系和文件内容，并在实际合入
   checkout 再运行 checker。
8. 显式 stop 原 Run，确认 `new_inputs_allowed=false` 和 `stop_status=stopped`；再停止服务并确认
   `not_running`。保留 Git 工作区、分支、PR 和原始现场供审计。

资格通过的 L2 `development-core` 仅省略第 5 步，合入和收尾要求保持。独立 L3 feedback
保留完整步骤；audited/multirun 按各自清单验证真实 PR，保留未合入并停止原资源。
两模型入口额外生成 `audit-models.json`，从原 session/workspace 的原生 history 按审计
turn_id 和发布 turn_id 校验实际 model/effort，不能仅凭请求配置认定发生了模型切换。

Runner 生产契约的交付里程碑仍为 `pr_ready`。合并属于测试 driver，结束判定包含 Runner
观察 merged 后的显式停止；不引入 SteerLine TaskDelivery，也不新增生产自动合并行为。

## 证据与失败

批次 `.evidence/tests-*/summary.json` 包含来源、每级结果及现场路径。
现场 manifest 记录冻结输入；`evidence/` 保存 `result.json`、`scenario.json`、CLI journal、
driver 的 Git/GitHub/checker 调用、原始/反馈 PR、合并与停止快照。Runner 自身业务数据和
原生协议日志在该现场 `s/` 中，模型 token 信息以原生记录可提供的真实数据为准。

smoke 通过要求：非零且精确的场景数、无意外 skip、真实产物正确、检查通过、PR 合入、merged
事实核验、原资源和服务均停止。L1 编译 live driver 并验证零场景失败门槛，同时运行入口
的零匹配/缺配置/重复场景反例、Windows
PowerShell checker 和发布夹具的受控反例，覆盖空 PR 列表、复用原 PR、冲突和查询失败
禁止重复创建。L1 只允许明确列出的两个 subprocess helper、三个显式 Orca 取证入口及一个
显式 MXC 工具环境诊断 skip；这些 skip 不计真机通过。后者用
`go test ./internal/execution/adapters/codex -run '^TestAuthenticationNativeToolBoundary$' -v -count=1 -args -sandbox-auth-native=<Codex绝对路径>`
单独执行，使用无效受控 canary，不运行模型、不证明真实认证/发布。
同一命令增加 `-sandbox-auth-host-broker=true` 可在全新诊断现场使用宿主登录的受限代理，
验证真实 MXC 的 Git/gh 只读能力；仍零业务输入、不证明 push/PR 创建。此入口保留原进程
身份、准备事实和关闭计数，不导出宿主 token。

未满足冻结 case 预期的失败、超时、unknown、unsupported、缺 final 均不通过；不可达停止
的 `revoked-unverified` 按其故障窗口契约独立判定，不冒充停止确认或业务交付。清理未确认
一律不通过。保留原现场，不自动
重发原生输入或替换会话。`scenario.json` 在外部动作前保存阶段和已知 Run/PR 身份；若
进程异常退出没有结果，批次失败，须用原 state-dir、request_id/Run ID 对账及停止。
修复后创建新现场；旧失败证据不改写。

这里的 unknown 指操作、结果来源或副作用无法对账。原生主代理已经核实当前回合 done、
原正文和来源完整时，可省略的 native outcome 仍按 unreported/unknown 原样保留，不补成
success；它与“原操作结果未知”分别判断，沿用既有 Orca 完成契约。

准备期限只覆盖当前阶段的准备。一个已完成的长业务回合切换报告、发布或反馈阶段时，
新输入获得自己的有限准备预算；未决原操作的对账仍沿用原期限，不因重试获得新预算。

单步诊断入口为 `new-test-fixture.ps1 -Scenario ... -Config ...` 和
`test-native.ps1 -Manifest <绝对路径>`。它们供已有前置通过证据的开发诊断使用；正式分级
验收使用 `test.ps1`，确保前置门槛和批次汇总完整。

审计 workflow 的两模型定向诊断复用同一个 native driver。配置中先显式指定执行 `model`，
再用下列命令选择不同的审计模型；省略审计 effort 时继承执行 effort。此诊断核验两项审计、
执行模型回切及实际 PR 后停止原 Run/服务，保留测试 PR 未合并，不计作普通 L2 完整 smoke。
它不改变默认 L2/L3 场景选择或生产默认发布提示词；该新 Run 冻结测试用发布提示词覆盖。

```powershell
$auditManifest = ./scripts/new-test-fixture.ps1 -Scenario local-codex.smoke -Config .evidence/live-config.json -AuditModel '<审计模型>' -AuditReasoningEffort high
./scripts/test-native.ps1 -Manifest $auditManifest
```

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
主线 rebase 后的一轮复测及未通过项另见[复测记录](acceptance/l3-rebase-20261016.md)。
连续版本冲突的修复与后续来源验证见[不可达撤销修复复测](acceptance/l3-revocation-fix-20261016.md)；最新主线的远端认证能力阻断未计为 L3 通过。
选择场景即执行其全部变体，每个 case/repeat 独立建现场。稳定性通过还要求故障命中、
原生调用审计、原身份和完整收尾；预期停止/撤销未知不要求 PR 合入，也不能记成业务交付。

完整稳定性套件需要管理员测试宿主、Node.js、已安装的 Windows Orca CLI 公开运行文件及
已配对的真实 environment。使用 `tests/live-stability-config.example.json` 创建本地非秘密
v2 配置；保持原生自身认证。完整资格入口：

主测试保持普通用户身份，管理员终端单独运行
`./scripts/stability-admin-host.ps1 -Directory C:/srt-admin`，并将 v2 配置中的
`admin_host` 设置为该目录的绝对路径。该宿主只接受固定现场的网络 block/restore，
核验宿主 PID/创建时间与原 manifest；每次操作有独立回执和恢复 watchdog。
宿主最多运行十二小时，可在该目录写入 `stop` 文件结束；先确认本轮故障设施均已恢复。
重复验收请使用新的空目录，避免上次的 `stop` 文件使宿主立即结束。

```powershell
.\scripts\test.ps1 -Level L3 -Repeat 3 -Config .evidence/live-stability-config.json
```

当前全 L3 每轮十一场景十七 case；原远端六场景十二 case 的历史三轮仍为三十六次，不能
与新增本机流程混用计数。L1/Controlled 每批完整执行一次。
`-Repeat` 默认 1，范围 1–10。`-SkipAdminRequired` 仅供用户明确选择排除管理员场景时使用，
报告保存所有排除项及 `passed_with_exclusions`，不能声明完整 L3 通过。当前最终验收要求全部执行。

## 环境检查定向验收

规则：`go test ./internal/application ./internal/contracts ./internal/cli ./internal/storage ./internal/execution/... -run '^TestEnvironmentCheck' -count=1`。
`TestEnvironmentCheckPublicFixtures` 通过生产存储/查询路径校验内嵌公开样例；合约有意变化时可用
`STEERRUNNER_UPDATE_CHECK_FIXTURES=1` 生成后复验，样例文件变化会改变 resource_digest。
`TestEnvironmentCheckCLI` 已注册到 Controlled，验证真实 EXE/IPC/SQLite、受理后失败、两种查询、原 stop
回执、无 Check 拒绝、只读查询、重启与 verify-stopped。测试不会访问安装目录。

```powershell
./scripts/test.ps1 -Level Controlled
./scripts/test.ps1 -Level L3 -Scenario @('environment-check.local-sandbox','environment-check.local-full-access') -Config .evidence/live-config.json
```

两种原生场景沿正式分级入口，分别创建新 fixture 和 Check。driver 只准备
`codex/srtest/<fixture>/base`、独立源仓库及只读取证；Worker 通过已保存意图的固定原生工具创建
`steerline-check/<check_id>/base|head`、commit/push 和 PR，driver 不代做这些动作、不合并 PR。
每种模式必须独立核实权限、实际工具、Git/PR 及原对象收尾，不能用 L1/Controlled 或另一模式替代。
远端缺 SA07 的两个模式保持 unsupported，由确定性反例验证，不作为已通过的 live 场景注册。

定向 native canary 使用有效 gh broker、同一原生准备和 `command/exec`，无模型输入：
`go test -tags=live ./internal/execution/adapters/codex -run '^TestEnvironmentCheckNativeBoundary$' -sandbox-auth-native <原生Codex绝对路径> -count=1 -v`。
该测试为 sandbox 记录工作区外拒写、为 full_access 记录允许写；只操作 `C:/srt/ecb-*` 中本次
owned canary，证据目录保留。普通用户检查不重复执行沙箱安全 canary。

已完成 native driver 的 environment-check fixture 可用
`./scripts/test-native.ps1 -Manifest <原 manifest> -ValidateOnly` 只读复验完整结果。
该路径核对原 manifest/Runner/driver SHA、完整 Check、上下文/分项与资源结算，拒绝失败或未核实结果；
不重新启动 Agent、服务或 Git/PR 操作，也不重写原批次记录。正常首次执行仍自动调用相同结果校验逻辑。
