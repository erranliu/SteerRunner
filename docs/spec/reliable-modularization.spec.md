# Spec：SteerRunner 可靠执行中间层模块化重构

日期：2026-10-15。状态：本次范围 RM-P0–RM-P3 及后续审计修复均已完成并复验，含 operation 提取及其余职责的 RM-P2 最小整理。范围和结果见 [Plan](reliable-modularization.plan.md)、[收尾证据](../acceptance/evidence/reliable-modularization-20261015.md)及[后续审计修复](../acceptance/evidence/reliable-modularization-audit-20261015.md)。

本 Spec 细化 [SteerRunner 产品契约](steerrunner.spec.md)的内部职责边界。公开行为仍以产品契约为准；[首版验收](../acceptance/steerrunner.md)的历史结果不计为本次重构通过证据。

## 1. 问题、基线与交付目标

调查及实施基线为 `af545563c58171c584bd35d098166cf39012e699`，调查开始时工作树干净、HEAD detached。实施使用 `codex/reliable-operation` 分支及未提交变更；实际构建和代码摘要见实施证据，不把基线 commit 冒充最终实现 commit。

三层仍为 Workflow → ReliableExecution → ExecutionAdapter，application/runstate/storage 为共同宿主。调查时，worker.go 混合操作落盘/执行/对账、准备、身份合并和观察，control.go、results.go 还夹有业务衔接；本次按下列职责逐项整理这些耦合。

目标是减少重复规则和交叉修改，让调用者容易找到操作、身份、观察、授权和准备的负责位置。先收拢已有操作的更新入口；不以包数、类名或代码行数验收。

**先交付 operation 提取，再对其余四项逐项核对。** 有具体重复规则或明显耦合则继续最小整理；也可以保留在现有包内，用文件和普通函数组织。交付不以五个独立包为条件，但须列清每项职责的实际位置、处理依据和验证结果。本次已选择的 RM-P2 整理见 Plan §3。

## 2. 范围与不变项

保持一个 EXE、同一进程/数据库、同 state-dir 单写者、现有 worker/gate/锁顺序和 Store.Commit 原子提交。保持 CLI v1、公开 DTO/错误/退出码/事件语义、workflow/提示词/配置/资源摘要规则，以及 `steerrunner-state/v2` 的 SQL/JSON 编码和读写行为。

本次不包含：新消息总线/流程引擎/调度模型、模块独立存储、Record 重建、操作投影重建、加载时新增完整性阻断规则、旧格式迁移、服务退出协议改造、原生能力或权限变化、跨仓 SteerLine 修改、合并和发布。workspace/publication/process/原生协议包沿用现有职责。

发现既有可靠性缺陷时，记录触发条件和影响，必要修复单独说明并配定向反例；不能为了完成模块化，静默扩大行为或兼容性改造。也不能为保持重构范围而删去既有安全检查。

本次后续审计确认并修复两个既有缺陷：stop 意图与命令关联改为一次原子提交，避免关联提交失败后仍外发；对没有公开投影变化但带新来源 ID/实现事实的已接收事件，先提交再确认。前者保留原事件类型/顺序但两条事件共享同一 revision，后者会为此前被跳过的新事实产生 revision/同类事件；不新增事件类型或存储字段。原生调用、权限与停止事实判定不变。反例与完整复验范围见审计记录。

## 3. 五项逻辑职责与宿主边界

仅 operation 确定提取到 `internal/execution/reliable/operation`；其他名称表示职责，不指定必建目录或包。

| 职责 | 负责内容 | 边界与第一轮安排 |
| --- | --- | --- |
| operation | 已有操作的固定身份/输入、状态和回执更新、execute/reconcile 分流、未知结果处理 | 第一轮提取；宿主调用规则并提交，不持有 DB/Store，不决定业务步骤 |
| preparation | 准备顺序、能力门槛、发送前资格、准备期限与失效 | 通用规则保留 reliable 的 plan/intent/environment；具体权限助手集中到 application/policy.go，调用顺序不变 |
| session | Run/Attempt/代次/配置/目标/session/command/turn 绑定，回执合并与 Turn 转换 | reliable/session.go 复用 BindingSnapshot，集中快照合并和回执/事件共用的 Turn 开始转换；不解码后端私有材料 |
| observation | 观察读侧、事件去重/冲突、原问题路由、final 来源和结果归一化 | application/observation.go 承担宿主接收/提交，可靠层保留结果核验；与业务报告路由分开 |
| control | 输入授权、stop/resume、服务换代、旧答案失效、旧写者/原会话恢复门槛 | application/control.go 保留资格/授权/生命周期，业务反馈与恢复后的步骤选择集中到 feedback.go；停止事实语义不变 |
| application 宿主 | 命令接受、worker/锁/gate、快照、跨职责组合、原子提交及公共状态投影 | 唯一持久写入位置；避免重复实现已提取规则，不要求所有内部函数调用都经宿主转发 |
| application 业务衔接 | 固定材料/提示词、StageInput/TurnInput 关联、Workflow 路由、业务反馈、PR/provider 核验 | 保持与可靠性规则的区别；仅整理本轮实际触及的代码 |
| adapter 与现有组装代码 | 原生调用/取证、私有 envelope、具体权限与平台接线 | 不增加新的写库能力；无需为凑结构另建组装层或文件级框架 |

preparation 可以直接调用所需的 operation 判断；observation 可以直接复用 session 身份校验。若提成独立包，保持实际依赖无环即可，不要求各职责彼此隔离。新 Attempt 不等于新原生 session，可信原生结果也不等于业务成功或 PR 已核验。

## 4. 状态归属

持久状态仍全部属于现有 Run Record，application 统一提交。下表指定规则负责位置，不要求为每行创建 View/Change、reducer 或新的持久对象。

| 现有字段/事实组 | 规则负责位置 | 处理约束 |
| --- | --- | --- |
| ID/revision/时间、请求幂等、CommandStates、Status/Phase/Diagnostic/事件 | application | 组合各规则结果，成功提交后再发布内存状态和公共回执 |
| Request/Materials/摘要/PromptBodies/Profile/仓库输入/WorkspacePlan | admission 与业务衔接，adapter 校验私有配置 | 接受时固定，恢复不读取移动源替换原输入 |
| Workflow/NextStage/Summary、业务问题、PR/PRClues/PendingPRURL | Workflow/application publication 衔接 | 业务步骤和平台核验不进入 operation |
| Intents 中原请求、操作状态与回执；Snapshot.Operations 中对应表示 | operation 的统一更新函数，application 提交 | 保留现有两种表示及各自语义，新增/结算操作时在现有事务内一致更新；不改读侧权威选择 |
| Snapshot.Ref/Session/代次/配置/Implementation/Workspace、Attempts/WorkspaceFact/NativeStarted/PendingTurn | session 规则及当前宿主应用位置 | 校验原绑定；业务输入由业务衔接提供，原生私有数据由 adapter 解释 |
| PreparationDeadline/LinkedWriteArgs/PolicyResources/PreparationProbe | preparation 与现有权限接线 | 保持期限/失效/权限语义和所属身份 |
| Allowed/ResumeRequired/PendingResume/PendingStop/StopStatus、NativeOwner/NativeManaged | control 与现有资源所有权代码 | 撤销授权和真实原生停止不是同一事实 |
| SeenNative/Result、原生问题集合/路由/游标/确认关联 | observation 与当前宿主提交位置 | 事实提交后才成功确认；保留来源和缺口 |
| PendingAnswer/PendingFeedback/AnswerHistory | control 管资格，业务衔接解释业务反馈，已有操作入口管投递 | 保留代次与原问题/原操作关联，不为同一交互创建多份可写状态 |
| Question/LastFinal/Controls | application 展示投影 | 保留来源、脱敏和最后结果展示语义，不作为反向补造执行事实的依据 |
| worker.port/observer/handle/wake/gate、service epoch/identity | application 临时运行资源 | 不持久化句柄；重启重新核验精确资源所有权 |

### 4.1 本轮不重建操作投影

第一轮只统一已有操作的构造、状态及回执更新，保留 Intents 与 Snapshot.Operations 的现有用途、历史和编码。不从其中一份批量重建另一份，不引入加载修复、差异白名单或新的 Run 阻断条件。

基线 EXE 生成的已停止样本确认，两种表示的 stop 幂等键确有差异。新构建加载、写回并重启两次后，样本的 journal 和 Snapshot 均与原数据相同；正常完成、未知结果样本也通过相同检查。此证据仅覆盖本次受控样本，不能据此宣称所有历史形态均已验证。实现不以结构全等代替原语义；若需要治理历史矛盾，仍须单独立项。

审计另补 stop_pending 样本：旧 EXE 的停止副作用已发生但结算提交失败，新 EXE 核验旧写者退出后沿用原 stop 操作结算，再重启复核。该样本允许结算状态变化，仍比较原请求、ID、幂等键、摘要及完整绑定；不要求结算前后结构全等。

## 5. 依赖与接口的最小要求

- 可靠性规则不依赖 application/runstate/storage、Workflow/PR 业务或具体 adapter；不获得 DB、提交回调或可任意修改的整个 Record。
- application 可以调用各职责；职责内部允许明确的单向调用，独立包之间无环。沿用现有 Workflow→reliable 结果值关系，不新增“大公共契约包”。
- 优先复用 LifecycleRequest/Receipt/Snapshot、BindingSnapshot、EffectResult、Result 等已有类型和普通函数。只有实际边界需要时新增小类型，不规定统一 View/Change/Decision 协议。
- 优先复用现有窄接口；仅对本轮触及且确有收窄收益的函数调整参数。无需全量重写 LifecyclePort 消费者，接口注入只服务真实外部依赖或测试隔离。
- 读侧函数不经由 map/slice/指针别名修改宿主状态；使用现有 Clone 或小范围复制。revision、授权和身份复核集中在现有提交/外发边界，不给每个纯函数附带完整绑定令牌。

现有依赖守卫随实际新增包更新即可；不新增“各职责互不依赖”或特定组装文件的架构测试。根包可以保留尚未提取的准备/身份/恢复规则；实现不得造成循环依赖或保留两份操作策略。若准备与操作规则的现有双向调用阻碍拆包，允许把紧密相关的小函数一起迁移或调整直接调用位置，不为维持命名边界增加接口层。

## 6. 必须保持的可靠性约束

这些约束来自现有产品契约，用于行为回归，不要求各自对应一个新抽象。

| ID | 约束 |
| --- | --- |
| RM-I01 | 保留单写者、revision CAS、锁/gate 和原子提交；Run、命令和事件一起提交，结果与对应业务变化/下一输入保持同一事务。提交成功后才更新内存和确认事件。 |
| RM-I02 | 首次外发前持久化原操作身份、输入、摘要及关联；状态/回执可更新，固定输入和原绑定不能被当前快照覆盖。保留现有 stop 命令关联语义。 |
| RM-I03 | 已在途/未知操作以及 accepted/resource_pending 按原操作对账，不因超时、重启或模块搬迁换 ID 重发；缺回执不视为成功。 |
| RM-I04 | stop 撤销提交与新输入派发仍共享 gate；撤销回执后不再外发新的准备/业务输入/答案/反馈，已发动作可结算，迟到结果不自动触发后继。 |
| RM-I05 | 服务换代维持禁止新输入及显式 resume 门槛，旧答案/反馈不重发；恢复核验原会话、未知操作和旧写者，不以替代 thread 掩盖未知。 |
| RM-I06 | 回执/事件/问题/结果核对适用原身份和来源；重复幂等、冲突拒绝、陈旧事实不推进；成功确认晚于事实提交。 |
| RM-I07 | 逻辑撤销、本地断观察、原生停止、旧写者退出分别表达；unsupported/unknown 不虚报 stopped；owned 句柄不被较弱句柄覆盖。 |
| RM-I08 | Workflow 只消费可信执行结果；报告补正不重做发布，PR 仍经原 provider/仓库/base/head 核验，TaskDelivery 留在 SteerLine。 |
| RM-I09 | 准备门槛、期限/失效、原生权限、linked worktree 根、固定材料和 report policy 保持；迟到 ready 不恢复失效授权。 |
| RM-I10 | CLI、配置/资源/存储读写语义、凭据脱敏、日志归属及支持范围不变；不新增动作、迁移或加载阻断规则。 |

### 6.1 操作收拢的终点

operation 只覆盖**已有持久操作**的共用规则和更新入口。宿主保留现有准备/执行调用顺序、资格检查与提交位置；不能为了统一函数名新增外部调用或改变副作用次序。

Run stop 中已经持久化的操作可以复用操作更新函数，但 StopLifecycle/ShutdownLifecycle 的选择和后端差异仍留原生命周期路径。服务退出、观察关闭、owned-process teardown 和资源释放保持现有处理方式；本轮不将它们全部改造成 operation，也不引入通用停止/清理策略。相关授权撤销、所有权核验和停止事实记录继续保留。

## 7. 分阶段验收

| ID | 验收重点 | 最小验证方式 | 适用阶段 |
| --- | --- | --- | --- |
| RM-A00 | 文档明确职责、状态、依赖、第一轮边界和非目标 | 链接、现有入口及文档差异检查 | RM-P0 文档部分 |
| RM-A01 | operation 独立、无存储/业务反向依赖、无重复策略 | 扩展现有依赖守卫；检查实际调用/操作更新点 | RM-P1、P3 |
| RM-A02 | 原操作输入/身份及更新语义保持，失败/未知/accepted pending 不重发 | 复用操作单测及宿主用例；不足时补提交失败反例与精确调用计数 | RM-P1 |
| RM-A03 | 身份/Turn/句柄规则不回归 | 复用身份与恢复测试；仅为本轮实际触及的缺口补反例 | RM-P1、P3；若选择整理 session 则加 RM-P2 |
| RM-A04 | 去重、来源和提交确认顺序不回归 | 复用事件/结果用例；改到相关代码时补提交失败不确认等缺口 | 同上；若选择整理 observation 则加 RM-P2 |
| RM-A05 | stop/gate、重启/resume 与原会话约束保持 | 现有受控停止/重启；缺派发边界覆盖时补可控屏障，不扩建控制框架 | RM-P1、P3 |
| RM-A06 | 准备门槛/期限/权限保持 | 现有 preparation/权限/accepted pending 测试 | RM-P1、P3 |
| RM-A07 | 正常 Workflow 与 PR 边界保持 | 现有真实 EXE→受控原生协议/provider 闭环 | RM-P3 |
| RM-A08 | v2 与公共输出保持，未新增投影治理 | 隔离基线样本加载→本轮操作结算/写回→重启及公开事实核对 | RM-P3 |
| RM-A09 | 有具体原生风险时补最小本机验证 | 原生协议/参数/权限/生命周期变更才触发，单独记录原因与范围 | 条件项，不是纯模块提取的固定门槛 |

先复用既有可靠性测试，再补实际改变边界的缺口；不要求每项验收新建一组测试。Windows/amd64 本机 Codex 的现有支持声明保留；Orca 真机验收仍取消，不因本次重构新增真机矩阵。实际验收结果统一登记在 Plan §6 和实施证据。

## 8. 完成定义

本轮按第一轮范围实施 operation 及必要调用点/测试整理；不修改 schema/资源/原生协议/权限/生命周期行为，不启动真实代理或发布真实 PR。

第一轮完成条件为：operation 提取、相关旧入口移除、实际新增依赖无环、RM-A01–A08 针对本轮变更有足够验证。RM-A03/04 等允许复用现有测试，不能以未拆成包为由判失败。若触发 RM-A09，完成相应最小验证；未触发则记录原因即可。

其余职责允许保留原位，但须逐项核对并记录依据；operation 完成不替代这一核对。本次交付还须完成 Plan 中已选的 RM-P2 整理及同一最终构建的 RM-P3 复验。状态与证据登记在 Plan §6。

## 9. 后续收敛：清理旧规则并简化 advance

用户在 PR #5 打开期间授权继续建议中的两项整理，基线为 `6f1db96ab0537743db4010256b92396d0557ddc8`：

- 删除没有生产调用的 OperationKey、ValidateSavedIntent、HasUnknownOperation、ResourceOutcome、ReconciledResult、ReconciliationRequest 及只服务它们的 SavedIntent、OperationFact、Reconciled 标记。测试保留原意图对账断言，并通过真实 application 路径验证部分原生事实不会因后续对账缺失而丢失。
- 观察挂接进入 application/observation.go 的 attachObservation，使用已有 LifecycleObservePort；输入物化和准备推进进入同包 preparation.go 的 prepareRun。它们均为普通函数，不新增职责包或状态对象。

advance 保留 gate、控制优先级、授权/权限门槛及反馈与准备的先后顺序；移除的重复答案条件已由紧邻的前置判断覆盖。两个搬迁代码块的主体与本次基线逐段比较相同，不移动提交或外发边界。观察挂接须在事实提交成功后接收 observer，失败则关闭临时观察；保留更强的 owned 句柄。

持久 Intent/Record、SQL/JSON、事件、原生调用/参数/权限/生命周期均不变。验收复用既有依赖、准备、停止和原意图测试，补宿主部分事实保留及观察提交/句柄边界；对最终构建重跑现有 8 个受控场景和 4 个兼容子场景。此次没有进一步拆包的要求。
