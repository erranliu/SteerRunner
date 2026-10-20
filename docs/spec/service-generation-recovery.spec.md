# Spec：服务重启、更新与重新部署后的原执行接管

日期：2026-10-19，2026-10-20 补充创建恢复职责。状态：Runner 核心、parser 保留、离线审计和 SteerLine 接线已有定向证据；Orca 资源创建由 Runner 独立执行器保留原回执，只使用现有公开接口。当前源码的验证范围见[追补验收](../acceptance/pr-test-gaps-20261020.md)，历史证据和跨仓限制见[验收记录](../acceptance/service-generation-recovery.md)。[实施 Plan](service-generation-recovery.plan.md)。

## 1. 目标与适用范围

在相关契约兼容时，Runner 服务正常退出、崩溃、换构建或重新部署后，新服务接管原 state-dir 中的 Run、执行和结果。服务换代本身不撤销已持久化的流程授权，不要求原生执行先退出，不重做实施、审计、测试、提交或发布。已有暂停、等待人工、业务阻断、停止及里程碑保持原语义。

“保留任务”分别指：SteerLine 保留 Task/DAG/Candidate/TaskDelivery；Runner 保留原 Run 及其执行聚合。接管不创建替代 Task、Candidate、Run、会话或业务 Attempt。可自动补读原事实，并在原授权范围内从唯一未完成边界继续。自动接续不是业务失败自动重试，也不是自动回答问题。

适用同一执行主机、相同规范化 state-dir 和可访问的原资源。重新部署包括 SteerLine/Web/Runner 程序更新，允许服务 EXE 路径、SHA、产品版本和资源总摘要变化。操作系统重启、机器迁移、原生 owner/Orca runtime 自身退出、磁盘丢失不在“原进程必须存活”的保证内；这些情形仍须保留事实并沿既有原会话恢复资格处理，不能因服务恢复而重跑任务。移动数据库而不保留原路径和外部资源不算兼容接管。

本设计不增加 workflow 引擎、任务调度器、恢复服务或第二套状态机；不恢复已退役的远端 helper，不拓展远端权限/停止证据范围，不修改现有 workflow/2 的业务边或 pr_ready 终点。

## 2. 实施前调查基线与缺口

Runner 调查版本：`fa5e0a82da88f77eba4cb539d17bf0a4a18dce29`，调查开始工作树干净。适用祖先 `C:/Users/lmas1/.codex/AGENTS.md` 为空，仓内未发现 AGENTS.md，沿用 `docs/spec/<topic>.spec.md/.plan.md` 惯例。只读核对的 SteerLine checkout：`C:/Project/SteerLine`，HEAD `9554eb61eefa417f5cd900d335650dacff3d3311`；未运行其测试或改动其文件。

共同生命周期入口为 [Worker 流程异常处理与重试](C:/Project/SteerLine/docs/product-design/worker-lifecycle.md)，本次所读文件 SHA256 `0fc0caf2ff0ccd5abb60b79126e7d538cf3d0aef45bde0f12a0685f823fe1b52`。W01–W18/E01–E07/V01–V08 为语义引用，独立 Runner 不在运行时读取该文件。其中历史 workflow/1、旧报告重试描述以 Runner 当前 [结果获取 Spec](result-acquisition.spec.md)与 [workflow 定义](../workflows/README.md)为准。

| 已核对位置 | 当前实现事实 | 设计影响 |
| --- | --- | --- |
| [host/release.go](../../internal/host/release.go) `selectedService` | CLI 选择活动服务同时比较 EXE SHA、产品版本、protocol、resource_digest | 限制的是 CLI→活动服务选择，不能误称整个 DB 都按 EXE SHA 拒读；应分开构建身份与恢复兼容性 |
| [storage/store.go](../../internal/storage/store.go) | 只接受 state/v3 和 user_version=3；单库、WAL/FULL、单 writer | 保留唯一持久权威；未知格式不得直接加载写入 |
| [application/workflow.go](../../internal/application/workflow.go) | 持久 Workflow.Config 必须与当前内置同 ID/version Profile 整体 DeepEqual，包含 Description/SupportedAdapters 等描述或准入字段 | 比必要边界更严格；不能仅保留这个整体比较并移除 SHA 比较，仍须区分原执行语义与新请求能力目录变化 |
| [application/engine.go](../../internal/application/engine.go) `New` | 全部 Run 被置 Allowed=false、ResumeRequired=true；未结束项变 reconciling/service_restarted，PendingResume 失败 | 活动、等待和业务阻断被重启统一改写，原授权与旧 epoch 耦合 |
| [application/control.go](../../internal/application/control.go) | `closeWorker` 撤销授权；旧 epoch 的保存答案拒绝发送；显式 stop 与关服务都可调用 ShutdownLifecycle | 应分离生命周期动作和输入是否已发送，不能只改一个布尔位 |
| [codex/lifecycle.go](../../internal/execution/adapters/codex/lifecycle.go)、[service_lifecycle.go](../../internal/execution/adapters/codex/service_lifecycle.go) | 本机声明 stop_execution；停服 owned.Kill/Close，等原树消失再关闭认证 | 正常停服结束执行；服务持有 Job 的路径也不能承诺崩溃后保留，仅改 ContinueExecutionOnServiceStop 不够 |
| [reliable/operation](../../internal/execution/reliable/operation/record.go)、[application/worker.go](../../internal/application/worker.go) | 先保存 in_flight 意图后调用；已有意图走 Reconcile；提交回执后才有原资源绑定 | 已有正确基础，但保存意图不等于已取得外部创建结果；不能从“库无回执”推断未调用 |
| [orcatransport/workspace.go](../../internal/execution/adapters/orcatransport/workspace.go)、[control_terminal.go](../../internal/execution/adapters/orcatransport/control_terminal.go) | 工作区恢复需要原 exact creation receipt；控制终端创建身份丢失时明确阻断，不重复创建 | 外部成功到回执提交之间仍有无法自动恢复的窗口；按标题、同名或空清单认领不可靠 |
| [codex/recovery.go](../../internal/execution/adapters/codex/recovery.go)、[application/results.go](../../internal/application/results.go)、[runstate/record.go](../../internal/runstate/record.go) | 已有原 session/turn 只读补收、完整 final、结果/NextStage/PR 事实、结果幂等与 CAS | 应补齐重启调度入口与持久化缺口，复用既有结果规则；不能为了接续重建源动作 |
| [SteerLine runnerclient/binding.go](C:/Project/SteerLine/internal/runnerclient/binding.go)、[deployment-runner.ps1](C:/Project/SteerLine/scripts/deployment-runner.ps1)、[deployment-stop.ps1](C:/Project/SteerLine/scripts/deployment-stop.ps1) | 实例绑定按 SHA/resource；同构建 restartable 可保留 Orca，换 Runner SHA 仍要求新环境或资源结算 | Runner 单仓修复不等于产品可更新；消费者需接受经验证的兼容换代，并保留原绑定 |

历史已有证据只证明当时行为：[固定控制执行期恢复](../acceptance/evidence/fixed-control-20261019/README.md)含显式 resume 和旧远端 helper；[远端原生验收](../acceptance/orca-remote-native.md)明确换代仍需显式恢复。它们均不证明本设计的跨构建接管、本机进程保留、初始化丢回执找回或自动接续。本轮没有运行功能测试。

## 3. 兼容边界与冻结事实（SGR-C01）

### 3.1 构建身份和契约身份分别判断

SHA/资源摘要继续用于校验选中的二进制、冻结资源和原运行事实，不能成为新服务接管的整体相等条件。不得删去原进程路径、PID/creation time、endpoint/generation、Orca runtime/worktree/terminal/incarnation/provider 的精确身份核验。

每个构建在既有 `version/capabilities` 输出中公开支持的 state 格式、workflow ID/version、operation/result 契约及 adapter key/codec；本机独立执行宿主还需声明自己的控制协议。沿用已有能力目录和具体 decoder，不新增兼容策略注册器。Run/Check 的实际要求从原 Workflow.Config、Operation.Version 和 implementation envelope 等既有字段读取；不再保存一份会与它们漂移的 per-Run 兼容 manifest 或独立恢复语义版本。

| 边界 | 接管规则 |
| --- | --- |
| CLI `steerrunner-cli/v1` | 保留写信封、request_id 幂等、expected_revision、expected_instance_id；新增兼容查询和有待处理 stop 时的启动输入，见 §9。协议不兼容即拒绝该调用 |
| state 格式及字段语义 | 同受支持格式可接管；缺版本、未知必需字段/枚举或不支持格式不能推断为当前。首期格式决策见 §3.2 |
| Workflow | 保留 `development/2`、`development-audited/2`、`conversation/1`、`generate/1` 的原 Config、游标、源与已冻结提示词。新构建须支持原初始状态、状态 kind/contract、转移和模型要求等执行语义；同号改变这些语义判冲突。Description、目录新增其它 adapter 或新请求默认提示词变化不应单独拒绝原 Run；仍须支持该 Run 原 adapter。沿具体 workflow Profile 的校验函数核对，不造通用“语义兼容引擎”，不以新定义重路由 |
| 输入、结果获取与 operation | 保留原 schema/语义版本、原操作 ID/key/digest/状态、来源和预算。解码/完成含义变化须显式升对应契约，不因总资源摘要相同便假定兼容 |
| Adapter profile/implementation | 按实现 key、purpose、codec 和冻结身份声明精确支持，不接受“所有版本≤当前”。当前 local Codex profile3、Orca local profile3/state6、远端 native profile4/state6 分开判断；权限资源等 envelope 也逐项校验 |
| 本机 execution owner | 新服务协议兼容才可绑定原 owner；原 owner、worker/tool/helper 的冻结 SHA 继续按原值验证。新 Runner 的 SHA 不要求等于旧 owner SHA，不替换正在运行的 owner。新所有权语义使用新的 local resource codec，不能将旧服务独占 Job 的记录解释为独立 owner |
| resource_digest / Version / EXE SHA | 作为来源和安装 pin 保留。仅新默认提示词、其他 workflow/schema、文档或打包变动导致总摘要变化时，不能拒绝兼容 Run。已有 Run 使用原内容；新增 Run 才用新默认 |

原 Request/Materials/PromptBodies/Profile/Workflow.Config、模型/effort、base/分支/仓库、沙箱和写根、认证策略/期限、AcquisitionConfig、input_digest/resource_digest 均冻结。接管不能重读移动材料或当前配置重新冻结，不能刷新预算、默认值或权限，也不能把认证过期当作服务换代后自动续权的理由。

主干 gh-only 认证边界同样适用：含已退役 `github_auth` 字段的旧冻结 profile 不能按相同 codec 号冒充兼容，预检须拒绝，不删除原字段或改用当前 gh 身份继续。这类旧认证路径不属于本次“相关契约兼容”的接管资格；保留原构建和事实处理，不恢复已移除的 GitHub App 能力。

材料资格沿现 Material.Content/digest 与 SnapshotPath 直接只读核对：原摘要、文件原字节、普通文件及非重定向绝对路径必须一致。新 native 输入派发前沿既有 source 校验再次核对，文件缺失/替换保持阻断，不从当前 checkout 或新默认重建；既有结果和未知输入仍可只读对账。此检查不增加每 Run 兼容 manifest。

### 3.2 首次接入与格式升级

首期使用 `steerrunner-state/v4` / SQLite user_version=4 作为唯一全库语义边界。理由是 v3 服务会在启动/退出时撤销 Run 授权并可能结束本机执行，不能让它重新写入采用新停服语义的目录。升版用于让旧 EXE 现有格式检查明确拒写，不代表需要增加新表或重建状态。Allowed/ResumeRequired、PendingStop/Resume、现有问题/来源和 command/operation 身份继续承担授权与派发事实；不预增撤销代次、派发租约、消费标记或导入游标。

Operation.Version 保持 1，复用既有 in_flight/accepted/confirmed/rejected/outcome_unknown；是否已建立可能调用的意图按 §7 判断，不增加平行派发状态。独立本机 owner 协议为 `steerrunner-execution-owner/v1`，`codex.app-server-binding` codec 从现有 1 增至 2，仅新增原 owner 身份/协议绑定；旧 codec1 不改写成已移交。未改变语义的 profile/权限资源/result/workflow 版本不随服务构建递增。若实施发现现有记录确有不可表达的事实，须先指出具体反例再增最小字段，不能以“未来恢复可能需要”为由建账本。

提供本调查基线 state/v3→v4 的一次受控无损升级：旧 writer 已退出且新服务独占 writer.lock；先只读检查记录及必需 codec；一致性备份 DB（含 WAL 内容），冻结文件原位保留，不覆盖或重新冻结；同一 SQLite 事务更新格式。文件来源/摘要的完整资格和不支持项隔离须按验收记录逐项证明，不能由格式升级推断。失败整笔回滚，原文件/外部资源不改。不能用“新空目录/重新 submit”作为升级方案；不支持任意历史 v1/v2 或早已退役的远端 helper 格式迁移。

现 v3 已丢失的授权不能补造：Allowed=true 且无 stop/等待/业务阻断等冲突时，可保留已保存授权；旧停服/启动已写入 Allowed=false/ResumeRequired=true 的记录保留原门槛，不能猜回“原本应该运行”。旧 epoch 的未投递答案依旧保持待人工确认；已尝试发送者仅对账。旧格式的 in_flight 操作一律按可能已调用处理，不能补成尚未发送。

旧本机 Codex 的 kill-on-close/停服 Kill 所有权不能由新 EXE 事后消除。首次从未支持本能力的旧服务切换时，预检必须在停服前识别这一限制；不得为了升级杀原执行、伪造接管成功或要求重跑。其活动树只有原服务提供经验证的协作所有权移交才可无中断接管；本轮调查未发现该接口。该存量限制是待解决/不可追溯的边界，不计入已实现保证。已完成的原结果、PR 和可接管的 Orca 记录仍保留；首次升级不能被宣传为所有历史活动执行均可无损接管。

新格式建立后，相同相关契约的 A→B→C 构建接管无需再次迁移或人工 resume。旧远端 helper Run 按 [现有退役规则](orca-remote-native.spec.md#6-旧记录与恢复)只读/精确停止；不恢复其运行栈，不因保留历史事实让兼容的新 Run 无法加载。完整性损坏导致无法安全识别记录时拒绝写入；可识别但不支持执行的单项保留只读诊断，其他兼容 Run 可继续。

## 4. 状态权威与生命周期规则复用（SGR-C02）

Run Record、Snapshot、Intents、Workflow 的既有归属不变，由 application actor 经 Store/CAS 一次提交；见 [模块化 Spec](reliable-modularization.spec.md)。不另增可写 recovery status/恢复数据库。新增事实放回原聚合，现有 journal 与 Snapshot.Operations 保持各自语义和原身份，不趁升级做结构全等归一。

| 共同规则 | 本增量差异及保持内容 |
| --- | --- |
| W02–W07 / E04–E05 / V01,V06 | 创建意图提供可跨服务找回的原资源身份；部分成功逐步落原 receipt。无身份不建替代资源；权限/ready/环境核验不省略 |
| W08–W10 / E04,E07 / V03,V08 | 区分未调用与可能调用，旧请求只对账；服务接管可派发原授权内已证明未发送的输入，不生成另一份业务命令 |
| W11–W14 / E01,E06,E07 / V04,V05,V08 | 离线完成补收原回合完整结果，结果与后继意图保持原子/幂等；缺原 final/审计 checkpoint 不重新跑源动作补证 |
| W12 / E03–E05 / V03,V06 | 人工等待保持；只有明确持久化并绑定原问题/原授权范围的未发送答复可续投，未知答复仅对账，问题变化不自动重答 |
| W16–W18 / E04–E05 / V06–V08 | 已有 PR/merged 事实保留；服务退出与资源停止/任务停止分别取证；部署不以资源退出为兼容接管的前提 |

共同入口中“换代一律零新输入、显式 resume”的现行规定，被本 Spec 限定替换为“原授权持久有效且派发可证明安全时自动接续”。未知输入零重发、业务失败待人工、stop 优先、身份冲突阻断继续适用。本仓当前行为说明随实施更新，历史验收不改写；共同入口及 SteerLine 消费者同步属于跨仓 P4，未完成前不得宣布 UpdateRunner 贯通。

## 5. 服务退出与显式停止任务（SGR-C03）

`service stop` 和服务崩溃只终止服务写者/连接，不提交 Run stop、不清空 Allowed、不制造 ResumeRequired、不清除 PendingResume/业务等待；服务启动不统一把 Run 改成 reconciling。服务的接管核验可用事件/diagnostic 表示，与持久业务状态分开，事实未变不反复递增 revision。

本机 Codex 需要独立于 Runner 服务存活的每执行原生 owner。由同一产品的本机执行宿主模式实现，使用受校验的独立进程创建机制；它持有 worker/tool Job、原 app-server、必要原生连接和认证 broker。服务不再独占 kill-on-close 句柄；断开服务 IPC、父进程退出、关闭 observer/查询进程均不杀业务树。owner 不持有 runner.db、不解释 Allowed/ResumeRequired 或再保存授权副本；业务资格和是否发下一轮由 application 唯一决定。owner 只承载现 Codex 适配器需要的原生调用、取证与精确停止，不建设通用 Operation 执行器/队列。不能只把 Job kill-on-close 关掉而失去显式 stop 和树退出证明。

owner 保存原 endpoint、generation、PID/creation、session/turn、操作绑定和认证原期限；新服务重新连接原 owner，不能重建 owner 冒充存活执行。不依赖旧服务 PID 存活作为资格，不用 `writersAbsent` 要求原活动树先结束。受控窗口还须证明原生连接丢失不会导致原 RPC/执行被取消；否则必须由 owner 持有该连接。owner/工具包放不可变版本路径，部署保留原文件至所有引用它的执行及认证资源结算；不覆盖或删除在用版本。

结果获取中独立启动的原生 parser 也须保留原执行及 Deadline/source/回执，不能用服务退出时取消 parser、随后另走报告回合来冒充原生执行保留。当前 `stopParserForShutdown` 仍会取消它，列为未实现项；受控 parser 重启测试只证明原输入零重发和原流程接续，不能据此验收本条。

显式 `stop <run>`、任务取消/交付所触发的停止仍先持久撤销授权，再通过原 owner/Orca 精确停止原资源，持久对账原 stop 意图；新服务不依赖旧服务独占内存 StopHandle 才能完成停止。服务离线期间调用方尚未送达的停止义务不等于 Runner 已接受 stop，SteerLine 重开时必须先提交/对账这些义务再允许该候选后继。stop 提交与派发使用既有 gate/CAS；已越过发送界限的在途动作如实保存，不能宣称跨进程原子取消。

保留 `resources_settled` 和 `service verify-stopped` 的原含义：仍有业务执行时不为 true，原未知/失败回执不改写。兼容重启/更新改用单独的接管资格，资源可继续存活。清除 state-dir、卸载执行宿主或破坏性迁移仍须按原范围逐项停止/结算。Orca remote 的 `orca_terminal` scope 不升级为全 OS 树/登录撤销证明。

parser 复用同一独立 owner：只允许 `parse_result` 这一具体动作，依赖从 host 注入既有 codexexec 启动器，避免包循环。冻结 ParseInput.Owner 指向原 owner 的精确进程；原 Payload、Generation、Deadline 和 Operation v1 不变。owner 内存只保留原任务句柄与请求 digest，根身份仍在挂起放行前写入原 started.json，最终树结算和结果写入原 receipt.json；不新增获取队列或回执账本。Runner 停服只断开调用，重启只读原文件；显式 Run stop 才按原请求取消 parser，原树结算前禁止后继。owner 的 `retains_parser` 和 `workspace_checkpoints` 是原绑定能力事实；旧 owner 缺所需能力时预检拒绝，不能替换它补能力。当前 audited workflow/2 只支持本机 codex-app-server，因此没有为 Orca 引入本机 parser helper。

## 6. 原授权自动接续（SGR-C04）

新 writer 的顺序为：取得排他写权并隔离旧写者→核对相关契约及冻结完整性→处理已持久 stop/撤销→对账原操作与补收原事实→按原业务状态和授权决定唯一后继。初始化无需人工点击 resume；显式 resume 继续用于原本已暂停/撤销、真实业务失败或必须重新授权的情形，不用它批量“恢复全部”。

授权复用 Run 的 Allowed/ResumeRequired/PendingStop、原 command/问题/来源绑定与既有 revision/gate，不能仅因 service epoch 不同失效。已接受的 resume 回执保留 pending/completed，接管继续原资格检查，不把它改成 failed 再要求另一 request_id。排他 writer.lock、实例身份和临时控制连接用于隔离旧写者，不再增加持久派发租约；原 command/operation/input 绑定不重编号。

| 停服前持久状态/事实 | 接管后的行为 |
| --- | --- |
| 已授权 queued/准备中，证明该动作尚未调用 | 保留原预算和输入，核验原环境后首次执行；已创建但未 ready 继续查原资源 |
| 原生轮次活动或等待原生问题 | 连接原执行并观察；不发送实施/发布/补报；问题保持原 question/Decision，等待人工 |
| 结果已保存、合法后继尚未派发 | 校验当前源/工作区/权限与原授权，派发已保存的唯一后继，不重跑完成步骤 |
| 原来 paused/resume_required、waiting_input/人工审批、业务 blocked、失败/中断 | 保留状态和原因；可收原迟到事实，不自动恢复业务或刷新授权。仅服务断连造成的读取未核实可通过原事实对账解除 |
| 原 stop pending/unknown/stopped、撤销或已结束里程碑 | 只继续原停止/对账或只读结果；迟到成功不重开授权，不触发后继。pr_ready 不恢复已移除的反馈边 |
| 原发送已尝试/accepted/in_flight/unknown | 对账同一 ID/payload 的原请求，绝不补发。已确认 started 等原回合；完整完成证据才能结算 |
| 原答案/反馈已保存且有“尚未调用”持久证明 | 仍须原问题/会话/来源、完整答复校验、原授权适用；首次投递同一命令。缺路由、已过期或语义不适用保持待人工，不自动改答案 |

执行身份仍存活却核验失败、权限/认证过期、外部人工新回合、会话/provider 替换等进入既有 E03/E05，不创建新会话。原进程确实丢失时才讨论既有人工原 thread 恢复；服务换代不主动进入 `restoreOriginalAttempt` 来更换 Attempt/资源 generation。

## 7. 外部创建与未知输入的崩溃窗口（SGR-C05）

### 7.1 两个系统之间的持久事实

沿用“提交 in_flight 意图→外部调用→提交回执”。既有 StageInputs/NextStage/PendingTurn/控制命令已保存，但在完整 Intents 中没有其对应的副作用意图，且状态/命令/来源一致，才可认定该动作尚未越过派发边界，继续首次建立意图。只要对应意图已持久，即使崩溃发生在实际调用前，也按可能已调用处理，恢复只走 Reconcile。不能为缩小这个保守窗口增加 not_dispatched/dispatching 两套状态和第二次提交；“不知道是否调用”不等于“证明未调用”。

创建资源仍使用原 operation ID/key、payload digest、目标和现有部分 receipt；确需原生创建关联键时，将其放入该创建操作的原请求。适配器须能按原请求读取已创建的精确对象；不要求每种资源实现统一 `lookup` 框架或统一未执行证明协议。业务 prompt/answer/publish 的 unknown 恒只对账，不能凭查询暂未找到结果重发。

本机 owner 复用并延续 process.json、thread.json、turn-start 和原生 journal 等现有回执文件：在服务断开后继续保存已取得的原身份/完整结果，新服务据原路径及绑定补读，再用现 Intents/StageResults 的身份去重提交。文件先可靠保存再应答，不能被业务 worker 改写；不再增加 receipt spool、消费确认、导入游标或第二个操作账本。进程创建优先复用现 `StartManagedBound` 的“挂起创建→固定精确 root 身份→放行”能力；thread 等原生响应由存活的 owner 接收保存。P0 必须验证服务退出不会取消这段读取。

此处保证的是 Runner 服务崩溃而独立执行宿主/Orca 仍存活的窗口；独立 owner 同时崩溃、原生服务丢失自己的创建回执，仍按身份/原生事实不足处理，不承诺任意多组件崩溃下的通用恰好一次。该范围与 §1 一致。不能把 Runner 所持连接随停服消失造成的丢回执也归入例外。

Orca 资源创建恢复由 SteerRunner 实现，不能依赖修改 Orca 代码或新增上游接口。Runner 在调用现有公开 `worktree create` / `terminal create` 前可靠保存原 operation、Attempt、generation、配置、路由、完整命令和原期限，并独占登记唯一创建执行器。执行器与服务进程分离，持有原 CLI 子进程和响应连接；服务退出只取消服务的等待，原执行器继续接收并可靠保存原始响应。新服务只读原执行器的结果，经绑定和内容摘要校验后按原 worktree ID、terminal handle/incarnation 核实资源，再回填原 intent。覆盖 worktree 的自动终端、初始终端和 native-control 创建，不派发替代执行器或重试 create。

该机制覆盖“Orca 已成功、原响应尚未到达 Runner 服务”和“原响应已保存、服务 DB 未提交”两种服务退出窗口。如果原执行器也崩溃、原 CLI 未返回资源身份、回执丢失/损坏/异源，或旧记录没有原创建日志，则保持 unknown。不得凭名称、标题、列表唯一性或空清单认领/重建资源；原期限不因换代重置。创建日志只保存有限原生动作的事实，不拥有业务授权、队列或第二份 Run 状态。真实后端资格按实际运行证据登记。

PR/引用沿冻结仓库、base/head、执行关联和既有 provider 查询找回原对象；同分支多个不确定对象、错误仓库/身份或无完整创建关联保持冲突。按名称找到一个 PR 不足以覆盖原 PR 绑定。成功后只补写事实，绝不重做 push/create/release。对 agent 内部任意未知外部副作用不承诺通用恰好一次；本契约保证 Runner 不重发该业务轮次。

### 7.2 必须覆盖的时点

| 崩溃时点 | 恢复动作与禁令 |
| --- | --- |
| 输入/后继已提交，对应副作用意图尚未提交 | 完整记录证明没有该动作的历史意图时，沿原授权首次建立意图并调用 |
| in_flight 意图已提交、调用未发生或结果未知 | 仅查询原操作；不能为了可用性当成未调用 |
| 外部资源已创建、回执未到达或尚未入 DB | 从执行端原请求登记读取精确身份，导入原 intent；缺登记保持未知，不第二次创建 |
| 部分创建回执已提交、ready/后续绑定未提交 | 从原 checkpoint 只读接续，不能从准备入口重建全部资源 |
| 输入接受但 started/最终结果尚未落库 | 查原命令/原回合，补收完整事实；不重复输入 |

## 8. 离线完成、已存结果与后继窗口（SGR-C06）

原生 owner/Orca 保存的原执行终态、完整 final、问题/工具请求、已取得的源 checkpoint，在服务离线时继续可读。取证绑定原 Run/Attempt/session/command/turn/input version/配置与资源 generation；不得用屏幕、最后一条文本或无来源 done 代替原结果。采用既有 `NormalizeResult`、来源核验、完整性、报告/解析次数和审计 pre_head/post_head 规则。

本机原完成 checkpoint 是 owner 在匹配 session/turn 的 `turn/completed` 写入原 journal **之前**取得的只读 Git 事实，沿用 InspectAudit 的工作区、干净提交、Tree/Head 与隐藏 index 检查。以原 owner 的退出公钥验签，绑定原执行/Attempt/配置与资源 generation/command/session/turn/工作区；文件为 owner/checkpoints/<原 command>.json，仅保存这次完成观察，不包含 workflow cursor 或新授权。Runner 仍负责审计相对原 implementation 的 ancestry 和来源适用性。原采集失败保存未核实事实；文件缺失、验签失败或错身份保持只读对账，可显示 completion_checkpoint_unavailable，不能采当前 HEAD 代替。其他观察连接可能先看到终态，结果结算必须等待原 checkpoint，不能先提交缺证 StageResult 抢过采集。

1. 离线完成但服务从未看见：新 writer 只读原生来源，补收原 final/终态；缺正文、来源或必要完成 checkpoint 保持 E06。不得重跑实施、审计或发布来补齐证明。
2. 回执/结果已取得但 DB 提交失败：导入原事实；按原 command/turn/source/input version 去重，同身份不同正文保留冲突。旧写者迟到消息只登记所属原操作，不能覆盖新授权或推进别的 Attempt。
3. 原 native result 保存但 acquisition 未结算：沿原获取意图重放程序提取/读取已保存 parser/report 结果；只在原预算、授权、源适用且明确未调用时启动定义允许的一次新获取调用。已尝试的报告/解析只对账，不重置次数。
4. 统一业务结果已提交但后继尚未派发：读取已有 StageResults、获取结果/source 引用、StageInputs、NextStage 和操作状态，不从旧步骤入口再走一轮。当前正常 finish 已把结果与转移一起提交，接管直接使用它；获取分阶段提交的合法中间态仅沿现有 route/获取结算入口完成下一次原子提交。缺字段不能证明未发送，非法中间态保持冲突，不增加恢复专用路由或新消费标记。后继可能已发送只对账。
5. PR 已创建、发布 final/核验尚未提交：查原 PR、原发布回合与固定 head；有合格 PR 事实可先保存但不伪造 pr_ready。已有 merged/PR 事实不得丢失或换绑；按原核验边界接续，只读失败可重试，发布不重做。

终态、获取结果和后继意图保持现有 actor 原子提交边界。事务外 Git/provider I/O 仍释放 actor/gate，返回后复核 source/revision/stop/权限再 CAS。重复启动两次、新旧观察并发、提交失败后再启动均只产生一个合法后继；stop 与补收结果竞争时，事实可保留，授权不能恢复。超时和获取预算按原绝对期限执行，服务离线不续期；长执行或人工等待不按普通 RPC 期限误判失败。

## 9. Runner、SteerLine 与必要接口差异（SGR-C07）

| 所有者 | 必须承担的职责 |
| --- | --- |
| Runner host/storage/application | writer/instance 围栏、版本资格、无损格式升级、保留授权和业务状态、原操作/结果导入、唯一后继派发及精确 stop |
| Runner reliable/workflow | 沿现 W/E 规则判断未知/来源/预算和合法后继；不读取 Task/DAG，不把业务规则交给 owner/adapter |
| Adapter / 本机 owner / Orca | 提供原资源身份、原创建查询、回合/完整结果、观察脱离、精确停止及原证据范围；不决定 Task 重新调度 |
| SteerLine | 保存 Task/DAG/Candidate→Run、原 CLI 信封与 request_id、投影游标及 PR/Delivery；换代重绑服务实例前核验接管证明，重连事件补投影，不重新 submit/批量 resume。先办理已存取消/交付停止义务；TaskDelivery 仍由本地事务决定 |
| 部署工具 | 新包暂存与契约预检、旧写者退出/锁释放、不可变 owner 文件保留、新服务启动、逐项接管报告和失败回退；不删除原 state-dir 或强迫业务树退出 |

最小公开增量：`version/capabilities/service status` 的版本化恢复描述；一个只读 `service verify-takeover --candidate <新EXE绝对路径> --json` 预检入口，在线通过当前服务、离线在既有 writer.lock 下只读检查。输出 `contract=steerrunner-takeover/v1`、目录和旧 writer 身份、候选 EXE SHA/契约、逐 Run/Check 的 revision/input_digest、compatible/read_only/unsupported/conflict 及原因；附原创建 command 的 request_id/request_digest，供丢失 submit 回执的调用方在 worker 启动前精确关联。request_digest 沿现 commands 定义：对保留 JSON number 的输入对象及 Protocol/Command/Target 字段做 canonical JSON SHA-256；不是另一份 Run manifest。它不停止、迁移、启动服务或授权输入；预检通过不代表接管已完成。

新服务实际取得 writer.lock 后重新核验候选和状态变化，在线提供绑定新 instance_id 的接管结果（原 Run ID、当前 revision、观察/继续/等待/仅对账/阻断原因）。报告为现有事实的只读投影，不保存第二份执行结论。预检失效或旧 writer 未退出不能抢锁；崩溃无干净 shutdown receipt 时，可凭精确旧 writer 退出与排他锁接管，不能伪造旧停服成功。新 CLI 可以按兼容 CLI 契约调用旧活动服务进行查询/停止，但必须保留 expected_instance_id、token、目录和原服务身份检查，不能无声切到另一实例。

旧服务尚无恢复描述/预检接口时，只能按本 Spec 固定的已知基线执行只读能力核查；不能臆造在线预检通过。若不能在停服前证明原活动执行可保留，更新提前失败并保留原服务；离线格式升级不补造这份存活证明。

SteerLine 的安装 pin 仍校验所选文件确是选中的构建；把“pin 必须等于历史构建”改为“经同目录逐项兼容接管后更新当前服务 pin”。候选原 input/resource digest 不改。部署同构建 restartable 与跨构建 takeover 分开记录，`resources_settled=false` 不阻止合格接管，也不变为 true。SteerLine 本地 actor/DB/锁/checkpoint 必须正确关闭；换代不能绕过项目隔离与共享宿主围栏。

已有取消/交付停止义务须在 Runner 自动派发前提交：正常部署先向原服务提交 stop；原服务已离线时，SteerLine 用 `service start` 的可选启动输入 `pending_stops` 携带每项原 run_id 与原 stop 信封（保留原 request_id、正文及 expected_revision）。Runner 在 `Engine.Start` 前沿既有受理规则先保存这些 stop 的撤销事实和原回执，再启动 worker；只需受理撤销，不等资源真正退出或未知发布结算。只有受影响 Run 的 Allowed/PendingStop 改变，其他 Run 正常接续。原请求重复先查 commands，revision 冲突等明确拒绝返回当前事实，由调用方沿原停止义务处理；不能静默修改原信封。受理未完成则此次启动不得派发业务，失败不写全局 hold。

这是一项有原 stop 事实依据的启动排序输入，不是通用控制批处理接口；不新增 `release-dispatch`、持久调度锁或第二份授权状态。输入须在发布 ready/启动 worker 前完成处理，未知受理通过原 request_id 找回。调用方崩溃后从自身持久停止义务重建启动输入，不依赖内存 release 步骤；独立 Runner 无此类义务时直接启动。启动输入的严格 schema 及现启动意图保存方式在 P0/P1 固定，不能启动后再“尽快 stop”弥补竞态。

环境检查沿独立 [Check 契约](environment-check.spec.md)（实际定义见 [总 Spec](steerrunner.spec.md)）处理：保留原 ID、报告、绝对期限、已建引用/PR、停止和清理义务；重启只对账原写操作并继续已授权且可证明未执行的清理，不重跑探针/创建 PR 证明 passed。远端完整 Check 当前 unsupported 仍不变。未接管 Check/未知创建须在报告中逐项列出，不得从聚合中省略。

## 10. 验收条件

| ID | 必须证明的结论 |
| --- | --- |
| SGR-A01 | A/B SHA、路径或总资源摘要不同，相关契约相同，原目录/Run/冻结材料/操作/PR 可接管；不支持契约明确拒绝/隔离，源事实不变 |
| SGR-A02 | 本机 Codex 在正常停服和服务强杀期间保留原 owner/原生进程树/session/活动 turn/权限；新服务接回；显式 stop 仍精确收尾且有证明 |
| SGR-A03 | 原活动授权自动接续；原暂停/等待/业务阻断/stop/pr_ready 保持；保存答案与 pending resume 按原状态处理，未知零重发 |
| SGR-A04 | 外部创建成功→回执落库前（含回执尚未到达）服务崩溃后，按 Runner 原创建日志找回；缺原资源证据保持 unknown，不猜选、不二次创建，不依赖 Orca 修改 |
| SGR-A05 | 离线完成补收原结果；已存结果、获取结果和后继未发/可能已发各窗口接续正确，重复重启不增加业务调用或消耗预算 |
| SGR-A06 | 未知 publish/input/reply 只读对账，原 PR/head/merged/结果保留；stop/旧观察/CAS 竞争不恢复授权或生成重复后继 |
| SGR-A07 | SteerLine 同构建重启、兼容 UpdateRunner 和项目/Web 重新部署均保留 Task/Candidate/Run/投影/Delivery；不要求资源退出、不自动重投或批量 resume |

具体最小测试与已有证据边界见 Plan。仅文档完整、源码可编译或历史显式 resume 场景通过，均不满足上述新验收。
