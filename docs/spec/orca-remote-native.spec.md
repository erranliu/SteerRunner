# Spec：远端仅通过 Orca 执行

> 2026-10-31 适用范围更新：认证与存储版本以 [host-native-auth Spec](host-native-auth.spec.md) / [Plan](host-native-auth.plan.md) 为准。新建开发执行使用宿主 Git/gh/SSH 登录；broker、CA、socket、认证租期/撤销证明已移除。仅支持全新 state/v6，不读取或迁移旧 Run；旧版执行由原版本结算。本文的 read_only、原生远端、身份绑定和未知操作对账条款继续适用，旧认证与旧格式兼容条款仅供历史溯源。

2026-10-31 当前开发权限修订：开发 sandbox 已退役，开发必须显式 full_access；read_only 与原认证链路保留。开发权限、迁移、旧 Run 与重复测试矩阵以 [统一权限 Spec](development-full-access.spec.md) / [Plan](development-full-access.plan.md) 为准。下文历史构建与验收事实不改写。

日期：2026-10-19。状态：P0 已按收敛契约通过，原生查询漏项已修复；远端 helper 删除已实施；已实施 unreported/显式 unknown 最小契约，并通过最新基线的完整闭环与活动停止验收（workflow v2）。调查基线：`a3dc15dc2f90787ea6e38e78d4dbc8a275470646`。配套 [Plan](orca-remote-native.plan.md)，结果登记在[专属验收](../acceptance/orca-remote-native.md)。实现与实际验证范围以专属验收记录为准。

拟议的[统一用量与独立费用模块](run-usage-and-cost.spec.md)只经已有 Orca 文件接口取得原始字节，在本机做计量解析；不新增远端 helper、reader 终端、原生结果服务或完成判定来源。该增量目前仅为设计，不改变本文的执行、停止范围及远端 Pi/DSH 不扩展边界。

## 1. 目标与适用范围

用户明确的边界：远端只通过 Orca 链路，不部署、启动或运行 SteerRunner helper。workflow、可靠执行和状态权威留在本机；远端只使用已有 Orca、Codex、Git、gh 及用户已配置的登录。

PR [#13](https://github.com/erranliu/SteerRunner/pull/13) 增加认证准备要求；PR [#18](https://github.com/erranliu/SteerRunner/pull/18) 的 `32e4b36` 首次加入 `cmd/steerrunner-agent`，以执行主机 helper 补齐认证、预检与历史取证；PR [#27](https://github.com/erranliu/SteerRunner/pull/27) 增加 inline-v3 退出证明。这些是后续实现选择，不是 Orca 通信的必要组件。

本次只改变 `adapter=orca-terminal`、`target.kind=orca_environment` 的新 Run，正向对象为 Windows/amd64 原生 Codex。保持一个 adapter，不增加公共 helper/native 模式选择或新的执行服务。

对该范围，本文取代[认证 Spec](sandbox-authentication.spec.md)的远端 broker/helper/同 app-server 预检要求、[连接 Spec](orca-local-remote.spec.md)的远端 helper 准备要求及此前专项改动白名单、[停止 Plan](orca-shutdown-verification.plan.md)的 helper 签名/Job/认证关闭要求。原生连接、路由、身份和未知结果对账规则保留。

本机 `codex-app-server`、Orca local 的 Codex/Pi/DSH 受管执行及其认证/树证明保持原语义。远端 Pi/DSH 本次不扩展。旧 Run 和历史验收不迁移、不改写、不借新规则降级放行。

## 2. 执行链、配置与冻结身份

```text
本机 steerrunner.exe
  → 本机 Orca CLI / 既有 Orca 原生协议
  → 指定 environment 的 Orca runtime
  → 原工作区、原终端中的 codex.exe
```

新远端 Run 不上传、安装、探测或调用 `steerrunner-agent.exe`；不启动 `--original-turn`、`--audit`、`--environment-session`、工具代理、history reader 或退出 verifier。不得以常驻脚本、临时 EXE、内嵌 Python/Node/PowerShell 或代理扩展替代相同远端 Runner 功能。普通 Orca shell、代理及正常业务工具不属于新增 Runner helper。

保留 transport、明确 environment、agent executable/args/model/reasoning_effort、access_mode。model/effort/access_mode 只承诺按冻结配置校验、编译并传递启动参数，不新增实际生效回读、核验门或相关字段；原生观察恰有 model 时仅作为附加事实，不升级为参数生效保证。直接使用既有 agentlaunch 编译器生成 Codex 启动参数，通过原 Orca 终端执行；原有无工具 READY 初始化可保留，只用于建立原生会话，不证明认证/沙箱/业务成功。

新远端请求显式包含 `agent_launch.runtime_executable` 或 `runtime_sha256` 的键（包括空字符串）时，在创建 Run/远端资源前以退出码 2、`invalid_input` 拒绝，说明应删除远端 helper 配置。schema、纯校验、capabilities 与调用方一致，不静默忽略或尝试执行。local 所需同名字段继续合法。

使用既有 profile envelope 的 `codec_version` 区分改变后的语义：新远端 profile 为 v4，仅含 transport/路由配置；local 继续 v3。新远端唯一执行路径为 Orca 原生路径，不新增 `execution_mode` 或第二个用户/私有模式字段。保留现有 payload 摘要校验、target ID、公共 input digest 算法和数据库表。2026-10-23 观察恢复修复将 state 写入版本升为 v7，并在 adapter 解码边界精确支持 v6；旧程序拒绝未知 v7，不承诺向前兼容。

v4 只适用于远端，禁止 helper 配置及 readiness/owner/inspection/recorded receipt 混存；未知版本或目标冲突拒绝。远端 v3 仅按 §6 作为旧记录处理，不根据 runtime/readiness 缺失推断 v4。版本判断只放在配置冻结和 adapter 入口，不把版本分支散布到 workflow/reliable。

**去 helper 必须删除远端接线，而非增加分支绕过。** 删除远端 helper 的启动编译、认证/readiness、历史查询终端、环境 actor、停止文件/签名等待、上传部署和专属测试准备的调用路径；不为旧远端 Run 保留可恢复的 helper 引擎。共享代码只在确有 local 调用时保留并限定 local；命名含 remote 的文件不代表都可删除，也不能成为保留远端执行入口的理由。无剩余调用的远端专属代码/配置/脚本应一并删除。实施交付需给出删除与剩余 local 调用清单。

## 3. 认证、权限与准入

| 对象 | 所有者与处理 | 可以声明的事实 |
| --- | --- | --- |
| Orca 连接 | 既有 environment 配对、认证、加密 | 原 runtime 连接及身份已核验；失败不回退本机 |
| 模型登录 | 远端预先配置的 Codex 原生登录 | 原生会话实际就绪；READY 不证明 Git/PR 权限 |
| Git/gh 发布登录 | 远端已有凭据、credential helper、gh 登录和代理实际权限 | 本次真实操作及独立 Git/provider 查询支持的结果 |

新路径不创建 Runner broker/capability，不转发本机 token、OAuth、私钥、gh 配置或认证文件，不管理远端登录期限和撤销。远端原凭据可能具备较宽权限；不得宣称 Runner 提供单仓库、限时、退出失效隔离。

取消“GitHub URL 必须取得 helper readiness”的门槛，不把缺 helper 变成另一种永久阻断。业务前仍核验 Orca 能力/身份、注册仓库、固定 base、唯一工作区/终端和原生会话就绪。已有准备操作可发现读取权限问题，push/PR 写权限只能由真实操作验证。历史完整 Check 不是普通 Run 准入条件。

登录失效、凭据在实际工具环境不可见、网络/权限拒绝，沿实际操作错误保留阶段，停止后继或进入既有 blocked/reconciling；不保证所有认证问题能在首个业务输入前发现。不把宿主检查或模型自述升级为同工具确定性预检，不自动修复登录、换认证、换会话或放宽权限。

原生 Codex 的初始化 READY 指令只对初始化回合有效，回复后即失效；后续业务回合及上下文压缩后继续遵循最新任务。该提示不授予业务、工具或文件修改权限，也不把 READY 当作业务完成。新 Run 使用收窄提示，已冻结启动命令保持原文；用量归属分别识别历史和新初始化输入的真实摘要，不将两者记成同一输入。

显式 stop 在远端关闭前核对并取消原观察器，避免其继续争用本机 CLI 串行队列。若 CLI 队列明确证明关闭命令尚未调用，保留首次关闭授权，后续仍按原操作核对原资源；已经调用但回执未知的关闭只对账清单，不重发。关闭失败与后续清单仍活跃同时发生时保留原关闭错误，不能仅以“仍活跃”掩盖调用失败原因。

沿用 sandbox 默认、显式 full_access 和 never 审批参数。启动参数表达请求模式，不冒充实际 OS token/同沙箱 `command/exec` 审计通过。sandbox 凭据不可见可以如实失败；不自动切 full_access、不改变冻结 Run。两个模式真实通过范围分别登记；本次取消 helper 专属保证，不伪造其通过。

2026-10-20 Git 发布修复：新建 sandbox 开发任务的既有原生启动命令先在执行主机读取 Git 身份，校验仓库 URL、当前工作区、`.git`/`commondir`/`gitdir` 对应关系及路径无重定向，再传递 shared objects/refs/logs 和当前 worktree admin 四个精确写根。共享 `.git` 根及其它 worktree admin 不因该准备而获准写入。已有冻结启动命令不重新编译。

对 HTTPS GitHub 仓库，有限启动命令在执行主机解析其已有活动 gh 登录，将 `GH_TOKEN` 及 URL 精确匹配的 Git HTTP header 放入该原生进程环境，固定 `GH_HOST=github.com`；使用固定工具环境白名单，保留原 pane 的 Orca hook 字段，拒绝继承认证跟踪及冲突 Git 进程配置，退出时恢复原环境。原生进程独立启动，不复用其它环境的 daemon。凭据不进入本机请求、启动参数或配置文件，不新增 helper、broker、登录操作或机器级权限改动。工具代码可以读取自己的环境，gh 仍具有原远端账号权限；URL header 不是对该账号的限仓库安全边界，不提供限时或退出撤销保证。原登录不可用、实际认证失败或发布未知仍阻断，不能重复提交证明成功。

## 4. 原输入、完成与成果

2026-10-23 已接入[最后消息与原生结束归属增量](orca-last-message.spec.md) / [Plan](orca-last-message.plan.md)：复用原观察流提供有界公开摘要，并仅对已观察可信起点、同一连续 feed 及相同正值原生 turnStartedAt 的可信结束通知允许缺 prompt。查询、重连 initial、重启或变更标记不能建立空 prompt 完成；旧冲突保持。实现与实际真机资格分别登记在原验收记录，未部署生产。

复用 `NativeProof`、`observeNative`、`StateFeed`、订阅加独立查询，不增加原生历史解析器或结果服务。`worktree.ps` 无分页游标；独立查询从 limit=100 起按返回 totalCount 扩展，在同一原 runtime 连接及 40 秒预算内最多查询三次，仅接受计数一致且未截断的单份快照，不拼接多次列表。缺完整性元数据、持续截断或持续增长时保留未核实，不能据当前页缺席判断原资源消失。

1. 发送前保存原 runtime/worktree/terminal/incarnation、pane/provider、主代理时间/版本基线、输入摘要与原 operation 请求。
2. `terminal send` accepted 只证明接受；首次业务提交用 Orca 原生 `--wait-submit 20` 有界等待同一请求的开始回执，不重发输入。超时缺开始仍为未核实，原请求开始回执及原生事实按既有规则证明开始。未知输入只对账，不自动重发或换终端。
3. 完成须为原输入后的原主代理结束、同一 pane/provider、匹配输入预览及完整原生 final；缺结束预览时，仅允许上述专项 Spec 的连续可信链窄化规则。保留既有多重绑定；200 UTF-16 预览不等于完整输入摘要，也不声称逐字历史已核验。
4. 排除旧 done、工具/子代理输出、恢复/会话边界、外部新输入、身份变化和中断。首份已核实 final/摘要不可被后续显示覆盖。
5. Orca 当前正文字段上限为 8000 UTF-16 单元；保留达到 7999 时的保守截断拒绝，包括 surrogate 边界。空正文、截断、冲突保持未核实；不读屏、不通过补报提示替代原文、不创建 helper reader。

2026-10-23 观察证据分类修订：`NativeProof` 分别保存可恢复的 `waiting` 与持久的 `conflict`，运行时不按诊断文本判断是否继续查询。prompt/agent/provider/主代理状态暂缺、工具正文不是输入冲突；保持原有 30 秒补查节奏，证据补齐后继续核验原回合。已用匹配 prompt 核实主代理开始时，后续 prompt 显示字段缺失是可恢复等待；空预览结束须满足专项 Spec 的同一连续观察链与保留回合标记，重连、查询和恢复不能仅凭持久开始证明空预览结束归属。其它必需身份字段暂缺仍等待，不用历史值补造当前身份。缺失字段也不能撤销已保存的完整原 final。非空输入不匹配、身份/权威变化、明确新输入/会话边界、中断及显式非成功 outcome 仍持久阻断；字段同时暂缺不能掩盖这些反证。

v6 的混合 `diagnostic` 仅在解码边界转换：明确的旧 waiting 文案转换为 `waiting`，其余非空诊断保留为 `conflict`，包括历史上可能由空 prompt 误触发的模糊 provenance 诊断。旧记录没有保存触发帧，不能自动清除、猜测来源或重发输入。v7 不接受旧字段，执行和用量读取共用版本边界；不增加工作流分支或兼容执行引擎。

观察可用性变化沿同一个生命周期事件流提交，绑定原 session/turn/event；首次缺证、原因变化及恢复都更新公开 `Run.phase/diagnostic` 和历史事件。缺证时 `running / observation_unverified`，恢复后回到原开始阶段；不创建人工问题、不改变输入授权、不覆盖已有停止或其它阻断。诊断与实现证据同事务落库，失败可重放，冲突重复不得覆盖。

原生结束结果区分 `succeeded | failed | cancelled | unknown | unreported`。只有新核实的 Codex 普通回合在原生接口未提供 outcome 时记为 `unreported`；它不是底层明确成功。既有原请求开始、主代理结束、身份/输入绑定、完整原文和无中断条件全部保留。显式 unknown、失败、取消以及原操作/输入结果未知继续阻断。

用户已明确授权最小调整 reliable.NormalizeResult：只有 `Succeeded` 的正常采集事实成立、最终原文来源完整且 Failure 为空的 unreported，才可交给既有 workflow 业务核验。workflow 的业务边、审计、成果和 PR 核验规则保持；持久结果回放、parser、报告来源和发布前置条件也必须保留原 native outcome，不能用历史 `Succeeded=true` 或新的成功报告覆盖显式 unknown、失败、取消或未知值。原生状态对外仍是 unreported，不能改写为 succeeded。历史已持久化 unknown 或空 outcome 的旧结果不重新归类、不迁移、不自动放行；空值保留既有解释，不补记 unreported，服务重启/显式恢复沿现有授权规则。

CLI v1 的 execution.native_outcome 与 final.native_outcome 扩展同一字符串值 unreported，结构不变。SteerLine 原样保存/传递，仍根据 Run.status、controls、PR 事实和 resources_settled 推进；不新增上层容错开关、成功推断或部署例外。

workflow 继续核验报告 schema、原结果来源和审计门槛；Git/provider 独立核验 PR repository/base/head/SHA/状态。模型自述、idle、发送接受或只读查询均不单独证明业务/发布完成。未知发布沿原分支/PR 对账，不重复创建。

## 5. 停止、停服与离线重验

### 5.1 停止范围

新模式只证明 **Orca 管理的本 Run 原终端执行资源已结束**，不证明所有 OS 写者、脱离终端的后台进程或远端登录已撤销。Runner 不再拥有远端 Job/broker，不能输出对应证明。

显式 stop 先持久化撤销新输入，再仅经 Orca 处理原 runtime/worktree/terminal/incarnation。关闭前仍核验原工作区/身份；不模糊找资源、不关闭项目/共享 Orca、不按调用方本机 PID 杀进程、不创建清理终端。

成功须原绑定完整、原 runtime 完整清单确认原终端 exited 或缺席。清单须原 worktree、计数一致、未截断、handle 唯一，且原执行主机 local 纳入范围。同 handle 换 incarnation、缺 host、断线或查询失败均保持 unknown。有 close 回执须验证 handle/runtime/ptyKilled；仅 accepted 或仅 ptyKilled 不能省略清单确认。

close 已尝试但响应丢失时，只读原清单可确认上述资源结束，不补造 close 回执、不重发 close。原资源已被外部关闭也可按完整清单结算此范围。清单短暂滞后只在既有有界预算内重读，超时保留未知；不将缺席升级为全进程树证明。

### 5.2 可见事实与持久化

仅在现有 `service verify-stopped` 的逐 Run 结果 `StoppedRun` 增加可选只读字段 `stop_evidence_scope`，v4 远端固定为 `orca_terminal`，从已验证的 adapter/target/profile 版本推导。它是输出说明，不是新授权或持久化事实；不向 backend、RunView、runstate、stop receipt 增加同名字段。RunView 沿已有 `diagnostic` 说明“已停止 Orca 原终端；未证明脱离终端的后台进程退出”。

- v4 `stop_status=stopped` 和 `resources_settled=true` 仍基于原停止回执/完整清单；不得仅凭状态字符串或 scope 放行。离线成功、活动及未核实逐 Run 结果均带适用 scope；scope 不等于成功。
- `WorkerAbsent` 只表示原 Orca 资源结束；不设置完整树 `WritersAbsent=true`，不声明认证撤销。只读 scope 不进入恢复或自动重发资格判断。
- 其他模式保留原证明要求。缺 scope 的旧记录不推断 v4；聚合须逐个 Run 按原范围结算，不排除旧未知项。

服务停止继续 `ContinueExecutionOnServiceStop`：结束本机观察，不自动停止远端业务；CLI/调用方退出亦不隐式 stop。离线 `service verify-stopped` 保留 writer lease/实例围栏，仅按同一原清单重验新模式并返回 scope；不启动 Engine/服务/模型/helper、不建终端、不发输入、不写原数据库或失败回执。

若未来需要完整 OS 写者树证明，应作为 Orca 原生能力缺口单独解决，不能重新加入远端 Runner helper/替代脚本或猜测证明。

## 6. 旧记录与恢复

旧 profile、resource、input digest、attempt 和证据不迁移、不补写；本次故障 Run 不会因新代码变成可续跑。

旧远端 v3 仅支持既有持久快照查询；服务加载时即隔离其自动观察/调度，自动恢复推进、resume、后继输入执行明确 unsupported，不启动 helper/reader/actor。旧记录仍可查询，不能因其不受支持而使整个状态目录服务启动失败；未结算资源仍阻断部署。禁止为兼容旧记录保留完整 v3 远端生命周期、历史查询和签名验证运行栈。

显式 stop 可从既有记录提取并核验原 runtime/worktree/terminal/incarnation，经 Orca 精确关闭原终端；该操作不把旧 Run 转成 v4。只存在 PTY 关闭事实仍未核实，不补记原 helper 树/认证已结算。仅复用原先已持久化且身份完整的停止结论；其余缺口交由原构建或人工处理，不在新构建里补建兼容引擎。旧文件与进程不自动卸载、不杀共享进程，历史保留。

新请求需删除 helper 字段，并遵守原资源未知、仓库冲突与并发围栏，不自动克隆重派旧任务。

v4 Run 服务换代后仅对账/观察仍存在的原资源，新输入仍需显式 resume；原身份冲突、未知输入、外部输入边界和 final 缺口均不授权重发。原终端已消失不以 PTY 范围证明满足全树恢复资格，不自动新建终端/代理。

## 7. 消费者边界

旧 Check 业务已整体退役；无专用命令、查询或资源接管兼容。SteerLine 的绑定、占用检查、HTTP/UI 和清理调用必须另行同步移除，不能直接连接旧客户端。当前状态只接受 v5；普通远端 Run 的路由、身份及停止 scope 保持。

## 8. 验收与非目标

**删除前必须先完成 Plan P0 的真实 Orca 能力核验。** 当前源码中存在原生启动/观察分支，仅说明有复用候选，不证明当前环境、代理版本和认证能够完成现有工作流。P0 通过前不启动 P1 的生产删除；允许调整现有探针及其测试夹具，以及先修复实测确认的原生查询漏项，不以临时放开生产检查作为成功证据。

P0 按“保留的能力、已明确收缩的保证、新发现缺口”逐项登记：至少覆盖原生启动和冻结参数传递、原请求开始/普通及短回合完整 final、同会话反馈、原代理实际 Git/gh 认证与最小发布、原连接重开后的只读对账、活动终端停止及完整清单、零 helper 调用。实际发布验证限独立测试仓库/分支和临时 PR，不合入业务分支。探针只证明底层可行性，不代替 P5 最终 Runner 验收。

完整环境检查、单仓库 broker/凭据撤销、全 OS 写者树证明已经属于本方案的能力收缩，不能在 P0 记成“现有功能等价”。新发现缺口若影响本方案仍承诺的功能，先记录证据并修订范围，不能一边删除一边增加 helper 回退、把功能静默改 unsupported 或放松完成判据。P0 未通过/未执行时，整体保持待核验。首轮 [P0 实测报告](../archive/2026-10/acceptance/orca-remote-native-p0-20261019.md)保留初始缺口；[补验报告](../archive/2026-10/acceptance/orca-remote-native-p0-query-fix-20261019.md)记录查询修复与完整远端快照。用户已明确参数只承诺按配置传递，不新增实际生效核验；该项不再是门槛。P0 已满足收敛后的可行性范围，不能替代 P1–P5 实现验收。

| ID | 完成条件 | 最小覆盖 |
| --- | --- | --- |
| ORN-A01 | v4 配置/直接启动；远端 helper 接线删除、全流程零 helper/reader/替代脚本 | L1 调用记录与删除清单；L3 |
| ORN-A02 | 远端已有认证、无凭据转发/新 broker/权限回退；错误如实阻断 | L1/Controlled；L3 |
| ORN-A03 | 原请求/回合/完整 final 驱动 workflow；截断/冲突/错身份拒绝 | L1；Controlled CLI；L3 原会话实施/发布及里程碑拒绝 |
| ORN-A04 | scoped stop/只读重验，未知零重发，不伪造树/认证证明 | L1；Controlled EXE；L3 活动停止 |
| ORN-A05 | 退出继续运行、服务换代显式恢复与原请求对账 | Controlled |
| ORN-A06 | 旧远端 v3 只读/精确处理资源、拒绝恢复，不保留 helper 运行栈；local v3 回归 | L1/Controlled 混合记录/local 回归 |
| ORN-A08 | SteerLine 配置/scope/旧 Check 消费者删除同步，旧未知项仍阻断 | 消费者定向及跨仓验证 |
| ORN-A09 | full_access 开发闭环与活动停止，复用现有 case；最终构建/零 helper/清理证据 | Plan 两 case 一轮 |

不整体回退 PR #18/#27，不修改 Orca、不增加 SSH/daemon、远端 shell 网关、检查服务、凭据 provider 或第二套可靠执行状态机。不为 sandbox 另跑完整 PR 闭环：保留参数及失败语义，以 Controlled 验证，未真机通过的范围如实登记；不能自动转 full_access。完整 L3/L4 资格不在本次范围。文档可审阅不等于功能完成；当前实现与验收状态见专属记录，不以文档状态代替现场结果。

## 最新主线整合

用户追加要求 rebase 后再完成。当前实施基线为 `95ae8a72792d2b11cc59b938257165b472b5b4de`（保留 workflow v2、result acquisition 和独立配置契约），原调查/P0 基线不改写。主线新增的固定 helper control 只在 local 受管路径使用；新远端 v4 拒绝 control 及其请求/回执资源混存。旧远端停止仅提取并核对原 Process，不加载历史 turn/reader/control 运行栈。不存在新的模式开关、迁移或远端 helper 回退。


### 最新主线的反馈边界

保留 workflow v2：pr_ready 为正常路径终点，其后 feedback 返回 invalid_state，原 revision/PR/head/turn 不变且零后继输入。P0/旧基线正向同会话反馈仅证明历史底层传输，不作为当前 workflow v2 的产品承诺；本次不恢复已由主线移除的反馈路由。最终 full-access/default 在合并前验证该拒绝边界，再精确合并测试 base。
