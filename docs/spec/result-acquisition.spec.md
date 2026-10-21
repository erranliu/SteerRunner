# Spec：Workflow 能力解耦与结构化结果获取

> 2026-10-19 主线接入已完成：rebase 到 `2aa17cc`，干净源码 `bb84ea8` 完整 L1 385 Go/29 Node、Controlled16、原生 workflow3 通过；测试修正 `51792ee` 后，复用相同生产源码/冻结 EXE 的停止探针2通过；签名原生退出、reader 结算与固定控制边界已组合核验。初始化创建身份未知仍阻塞，四项管理员变体未执行；不继承历史构建资格。见[最终证据](../acceptance/evidence/rebase-control-20261019/README.md)。SteerLine/Orca 不改。

> 2026-10-19 固定控制终端已实施：同源完整 L1 366 Go/29 Node、Controlled 16，以及正常重启、执行期强杀恢复、活动执行撤销三项原生用例通过；原控制身份、完整 final、撤销和关闭分别核验。初始化创建身份未知仍阻塞，无第二状态权威，SteerLine/Orca 不改。内部 Orca state codec 升至 6；四项管理员变体未执行。见[本增量验收](../acceptance/evidence/fixed-control-20261019/README.md)及 §7.1。

> 上述旧批次通过绑定212ebee；当时推送后发现新主线的冲突，未验证组合源码。原冲突事实保留为[历史记录](../acceptance/evidence/fixed-control-20261019/checks/integration-status.json)，不代表本轮最终状态。

> 实施状态（2026-10-18）：已按确认的最小边界实现并完成必要验证。原生权限、行为约定和事后检测见总 Spec §6；当前支持 Windows / Codex 0.159.2，默认关闭独立解析。实际支持范围、失败修正及最终证据见[验收登记](../acceptance/result-acquisition.md)。历史“仅设计”说明指此前设计轮次。
日期：2026-10-18（首次调查 2026-10-16）。状态：三方式及本期两定义已实现，必要验收通过。普通需求，不创建 Task/roadmap 调度项。[Plan](result-acquisition.plan.md) · [专属验收](../acceptance/result-acquisition.md) · [调查与原生能力记录](../acceptance/evidence/result-acquisition-design-20261016.md)。

本轮收敛：只实现两个内置 Workflow、三种结果获取方式、同一结果模型及必要的来源/恢复接线。统一机制与具体定义分开，agent 阶段动作统一发送定义中的提示词。首期只用内置核验函数，不建设定制脚本端口或执行宿主。可读定义先交付确认稿：[统一能力](../workflows/README.md) · [development](../workflows/development.md) · [development-audited](../workflows/development-audited.md)。下文原生解析设计继续适用；具体定义文档已按本轮选择更新，本期三方式与两定义已实现；支持范围以本页实施状态和 §6 为准。

## 1. 范围与依据

将通用状态执行能力与具体 Workflow 的状态、边、提示词及核验规则分开，并将“需要什么业务结果”与“从哪里、怎样取得结果”分开。提供程序提取、独立 LLM 解析、执行 agent 报告三种可组合方式；按状态选择方式、字段、顺序、完成条件、后备条件及预算。现有内置流程改由同一状态执行机制运行，不建设 DSL、外部策略插件或独立流程调度平台。普通执行轮不统一追加 JSON 要求；明确选择 agent 报告的状态可以要求结构化输出。

首期不做外部流程文件装载、DSL、插件/回调注册、动态状态图、任意层级子状态机、通用字段编排/合并平台、多解析后端或额外调度器。只提供现有 Git/provider 事实、成功行识别、完整结构解码、审计独立解析及一次原 agent 补报所需接线。未来新流程若只复用这些能力，应只增加内置定义/提示词/契约和测试；若确需新能力，届时单独设计，不为假想流程预留可执行扩展口。

development-audited 按本轮要求只保留代码审计，移除 Spec 审计；实施 → 审计并修复 → 发布；代码审计在同一次正式执行内直接修复 P0–P2、完成必要验证并报告修复数量/内容，不再自动返回 implement 或使用两次修复上限。保留真实工作、PR 核验及 TaskDelivery 的职责边界。固定报告调用改成获取状态及条件报告状态，不再要求每个状态对应一个 agent Turn。本次范围扩展包括现有推进代码的通用机制/具体定义解耦，替代最初“不重构流程推进”的范围限制；不重构原生恢复、任务调度、环境检查或沙箱认证，不新增一套状态权威。不修改 SteerLine，不增加其调用步骤或解析配置义务。不修改原执行 final，不为程序提取或独立解析伪造 agent 报告轮。

调查基线：本工作树 HEAD `f211e316b89e3b86527358f1bbd48eac9f58513a`，开始时工作树干净。工作树、主 checkout `C:/Project/SteerRunner` 均没有 `AGENTS.md`，Git 跟踪及工作树扫描也未找到子目录 AGENTS；适用祖先 `C:/Users/lmas1/.codex/AGENTS.md` 为空。因此采用已有 `docs/spec/<topic>.spec.md`、`.plan.md`、`docs/acceptance/<topic>.md` 及 README 导航惯例，不臆造 AGENTS 规则。

已读取共同生命周期入口 [Worker 流程异常处理与重试](C:/Project/SteerLine/docs/product-design/worker-lifecycle.md)，其只作设计引用；独立 Runner 运行不读取或依赖此文件。固定读取版本、摘要和关键条款见调查记录。本 Spec 仅维护 §7 的增量差异，不复制 W01–W18/E01–E07 状态机。

| 源码位置 | 当前事实与设计影响 |
| --- | --- |
| `execution/workflow/route.go` | `implement.execute` 成功固定追加 `implement.report`；报告格式错误一次修正；publish 解码失败进入 `publish.report`。必须把原轮终态接收与结果获取完成分成可独立提交的事件 |
| `workflow/result.go`、`prompt.go`、`resources/assets/prompts.json` | `Report` 同时承载 outcome/summary/outputs/question/blocker；publish 与 audit 直接追加 JSON 指令。移除普通发布的统一格式要求；严格字段校验继续用于统一业务结果及显式报告 |
| `workflow/{types,input,profile}.go` | `StageResult` 绑定真实 Command/Attempt/Turn；`AuditReportCommandID`、`AuditSource`、`AuditsPassed` 强依赖成功的 StepReport 和其原实施。改为核验当前统一结果及其真实实施源 |
| `application/{results,freeze,feedback,control,worker}.go` | `finish` 同时保存 final、路由、报告来源、审计及下一输入；queue/resume 默认可回实施。获取未完成时必须保留原步骤/获取意图，不能由通用 resume 分支意外重做实施 |
| `reliable/result.go`、`application/observation.go`、Codex/Orca evidence | 原生完成、Succeeded、NativeOutcome、完整 final 来源已分开；成功缺可信 final 只补读原来源。本期沿用，不让解析器填补原生来源缺口 |
| `application/results.go`、`execution/publication/` | PR clue、独立绑定、`verifyPublication`、当前 audits gate 已存在；程序提取复用 provider 身份核验，不能见 URL 就 pr_ready |
| `runstate/record.go`、`storage/runs.go` | Record 为唯一持久聚合，Commit 原子保存 Run/命令/事件及 revision。新增结果和解析资源仍在此边界，不另建结果数据库 |
| `adapters/codex/{native_params,policy,launch,observe,history}.go`、`execution/process/` | 现执行配置固定 workspace-write/MXC/never，报告只读是提示词策略；已有 managed process/Job、原意图和日志可复用。解析权限与配置不能直接照搬执行 profile |

表中源码路径相对 `internal/`。两种现有 Profile 为 `development/1`、`development-audited/1`；当前审计版依次 Spec 审计、代码审计，P0–P2 阻断、P3 建议，最多两次自动修复；这是源码基线，本稿 workflow/2 按新要求移除 Spec 审计，代码审计直接修复问题并报告，不采用旧自动返修循环。

## 2. 职责与最小模型

Workflow 定义内置的 `ResultContract` 与 `AcquisitionPolicy`：契约规定字段、类型、业务一致性及必需证据；策略规定哪些方式负责哪些字段、何时后备和何时结束。它们是普通 Go 类型/函数和内嵌资源，不接收任意流程图或用户策略表达式。

### 2.1 状态机定义与统一能力

两个定义以普通 Go 值、内嵌提示资源和纯条件/核验函数表达当前所需状态与边；不把文档图加载为可执行配置。首期状态执行只需“原生执行 → 取得候选结果 → 核验 → 提交已授权后继”，文档的外层/内部图是分组视图，不要求通用分层引擎。

| 责任 | 统一机制 | 两个具体定义 |
| --- | --- | --- |
| 原生资格、意图与派发 | 复用 ReliableExecution/actor，先持久意图再执行，核实来源/停止/授权 | 声明本次动作和所需已核实事实，不能绕过原生门槛 |
| agent 输入 | 同一提示发送入口、变量冻结、真实 Command/Turn 和模型绑定 | 实施/发布/审计修复/补报正文、业务材料、所选冻结模型 |
| 结果获取 | 三种有界能力、原文/来源/尝试记录，成功行确定性识别 | 哪些字段、何时选程序/独立解析/原 agent、后备及预算；不让所有阶段执行三遍 |
| 核验 | 调用已有 Git/provider owner 读事实，承载完整结构与来源校验；具体业务谓词只读这些事实 | 成功文案、修复数量一致性/遗留规则、PR 与修复后提交一致等业务完成条件 |
| 后继与持久化 | 统一核实源/当前游标、CAS、原子接受结果和下一意图，拒绝迟到/重复/越权 | 正常后继、等待/阻断去向；纯条件不做 I/O、不派发、不写 Record |

运行能力按动作类型派发；不能在统一运行路径按 workflow ID、audit.code、publish 或成功文案选择业务行为。入口按 ID 查找已编译内置定义和公布 catalog 是允许的；禁止的是查找后的共享机制继续按业务名称选择顺序、规则或后备。业务核验函数属于对应定义，可复用已有普通函数，但无外部函数注册、插件 ABI 或脚本执行入口。只有 actor 写 Record；定义、adapter、parser、核验函数都不拥有自己的业务游标。

阶段还可声明有语义的成功输出与其结果契约。定义、提示注入与 RecognizeSuccessOutput 属统一程序提取能力，具体文案由 Workflow 阶段冻结，运行层不内置业务关键词。在当前轮原生资格满足且可信 final 可读时，统一 CRLF/LF、忽略末尾空白行，对最后一行逐字匹配；不接受代码围栏/引用/缩进代码内文本、正文片段、旧轮、工具日志或 commentary，不做模糊匹配。未闭合围栏等情况不命中；原 final 不可核实则先原来源补读。命中且本阶段必要结果及核验通过直接完成阶段，跳过 parse/report，零额外模型调用和 parser 环境探测；未命中不等于失败，沿定义的原获取策略。成功声明不覆盖原失败、冲突、未知副作用或 provider 事实，也不补造缺失字段。详细规则由[统一能力 §4.1](../workflows/README.md#41-阶段成功输出的定义注入和识别)维护。

统一能力为共同准备、向原执行会话执行提示词、程序提取、隔离解析、结果核验及已有 owner 的原生/外部事实查询。实施/审计/修复/发布/原 agent 报告全部只通过同一执行提示词入口驱动原 agent 进入相应状态；提示正文与冻结模型绑定属于具体定义。原生传输、输入授权和来源核验仍由现可靠执行负责。程序完成可直接触发非模型状态转移，不能为与原 agent 阶段对齐而造假 Turn。

首期不提供定制核验脚本的配置、发现、schema、沙箱、宿主、命令端口或支持资格矩阵，也不新增一个“缺脚本环境”的运行分支。两个定义只调用所需内置核验。未来若单独设计脚本能力，其用途仍只能是结果核验，不能实施/采集/调用模型/发布/选择后继；这一原则不转化为本期建设项。现有 provider 查询及 Git owner 继续使用原端口，不把其外部命令接线扩展成任意脚本平台。

每个具体 Workflow 有独立的可读文档，维护外层业务状态图、每个外层状态的内部状态图、核验/异常/预算及提示词；整体图不展开内部流程。development 外层只设计 implement → publish → pr_ready，implement 内部为 execute/report/input，不选择独立解析，不设计 pr_ready 返回 implement 的边；内部 input → execute 的明确输入续执行仍保留。pr_ready 的内部图只表示稳定里程碑及 PR 事实刷新。外层/内部进度在原 Run 一起原子提交，不引入子 Run/第二套生命周期；进入 implement 的提示词只发一次。文档是内嵌定义的说明，与同版本定义/提示资源一起更新，不另行加载执行；development-audited 同样分层，实施/发布无独立解析，仅 audit.code 的审计修复结果获取保留 parse，无自动返修或重新审计边。

### 2.2 结果与持久化职责

application 负责冻结材料、构造来源包、持久意图、调用 owner，并将已核实原生事实、业务结果与获取状态合成一次原子提交。ReliableExecution 继续负责操作身份、投递、核验、授权和恢复；adapter 负责原生进程/协议/权限证据，不解释业务字段。独立 parser 是专用资源 owner，消费固定包、返回候选结构，无 Workflow 推进权、PR 写权限或中心数据库写权。

在现有 Run 聚合增加以下最小数据，允许内嵌到原 Record，不要求独立仓库/服务或每个概念一个包；三类事实的语义不得合并：

| 概念 | 最小内容 |
| --- | --- |
| ResultContract | 内置 contract ID/version；字段与校验；源执行资格；必需来源；完成谓词。不包含“必须某报告轮成功” |
| AcquiredResult | Run 内 result ID、contract、源业务 step/输入版本/Command/Attempt/Session/Turn、材料摘要/实际 Git 提交关联、business 候选值、各字段来源、缺失/歧义/冲突、获取状态、accepted revision |
| AcquisitionAttempt | 获取 ID、方式、所需字段、不可变材料包摘要、策略/配置摘要、开始/结束、状态/原因、已消耗次数与原 deadline；模型方式另有实际进程、会话/轮次、模型、原操作及日志引用 |
| SourceRef | 原生 final 的 source/item/Command/Attempt/Session/Turn；artifact 的固定路径/摘要/大小；机器报告格式/字段定位；provider 查询的目标/时间/摘要。模型产生的字段须指向输入来源定位及本次模型输出，不能只引用模型自己的断言 |

这些数据只支持当前两个 Workflow 所需契约的完整候选及选定后备：PR 字段由程序事实核验，审计语义由一次解析或补报提供。不建设任意字段的分工、并行编排、来源评分/投票或自动融合框架；已知事实与候选冲突时保留并阻断。

成功输出识别保存当前阶段/源输入、声明及契约摘要、原 final source/item/Turn、匹配行定位、命中/采纳及核验依据；复用上述来源与获取记录，不产生伪报告 Turn。声明与注入后的提示正文接受前冻结。成功声明提供的完成结论与原 final/程序事实一起形成统一结果；原文保留，summary 不冒充逐项测试记录；后继意图与结果原子提交，重复读取/重启补读不重复推进，停止后的命中不派发后继。

result ID 只是同 Run 的持久对象引用，不替换 Command、Attempt 或 Operation。程序提取没有 Turn；parser 的真实原生会话关联保存于 AcquisitionAttempt，不进入业务 StageInputs/StageResults，不占用执行 Snapshot 的当前 turn。真正的 agent 补报有自己的真实 StageInput/Turn，并指向原执行源和获取 ID。

程序提取/核验所用 V-NATIVE 指向原业务动作的已核实资格，不为这些状态要求一个新的 agent Turn；Git/provider 自身查询的进程/回执由原 owner 管理，不能混入执行会话历史来满足该条件。

三类事实独立：

1. 原生执行情况沿原 reliable.Result 保存，包含原终态、来源是否完整、失败与未决操作；Orca 已核实 done 但省略 outcome 的现有语义不变。
2. 业务结果保留 `done | needs_input | blocked`。材料不够时未取得业务结论，用缺失字段和获取状态表达；unknown 只是说明文字，不是第四个业务 outcome，也不写成 `outcome="unknown"` 或用 blocked/failure 填空。question/blocker 继续按既有规则校验；原生 NativeOutcome 的 unknown 不受此约束影响。
3. 获取状态为 `pending | running | satisfied | insufficient | conflict | failed | cancelled | unverified`。failed 是获取执行/格式失败；insufficient 是材料不足；conflict 是事实冲突；unverified 是原获取操作无法对账。成功 JSON 仅是候选值，不直接等于 satisfied。

非 satisfied 不推进正常业务后继，投影沿已有 blocked/reconciling/waiting_input 和 diagnostic 区分原因，不能新增一种“解析失败所以业务失败”的含义。业务 needs_input 仍必须有真实问题依据；但具体 Workflow 可以声明“未成功统一等待输入”。development 的 implement 已确认采用此规则，业务 blocked、未取得业务结论或获取不足的原事实不改写为 needs_input，Workflow 可生成来源为 workflow_policy 的通用等待说明，不冒充 agent 问题或编造失败原因。等待用户与原生对账/停止资格分开。

### 2.3 一个业务游标和一个提交权威

`runstate.Record` 是唯一持久聚合，现 actor + storage.Commit/revision CAS 是唯一写入/提交权威。复用 `Workflow.StepID` 为唯一当前业务游标，记录完整内部状态 ID，例如 implement.execute、implement.report、implement.input、audit.code.execute/parse/report、publish.verify、pr_ready；命名可统一，但不得再持久一份可独立推进的 outer_state/current_phase。定义中的静态映射从该游标投影外层阶段，本文图仅分组展示。

| 信息 | 权威/用途 | 不允许的推进方式 |
| --- | --- | --- |
| Workflow.StepID 及源输入/当前结果引用 | 当前业务位置和本次工作来源，在 Record 同一次提交 | 从最新 Turn、报告文本或 parser 完成自行猜下一状态 |
| Record.Status / Phase、inspect.step/外层阶段 | 既有运行/等待/对账/停止事实的公开投影；actor 同次更新，Phase 只作诊断 | 仅修改 waiting_input/running 或 Phase 就生成新业务动作 |
| StageInputs / StageResults | 真实原 agent 输入与结果历史，冻结 StepID 是当时动作关联 | 以“列表最后一项”代替当前游标；为程序/解析伪造报告 Turn |
| AcquisitionAttempt.status、parser 操作/资源 | 本次获取是否运行/结算/未知的事实，受原预算约束 | parser 自己保存后继或成为另一个 Run；satisfied 自动覆盖游标 |
| Snapshot / Intents | 原生可靠性、投递及资源事实，沿原生命周期权威；parser 尝试引用同一 Record 中对应的实际操作 | 用原生进程状态代替业务后继，或为获取另存一份操作/恢复账本 |
| NextStage | 仅原执行会话下一次真实提示输入的 StageInput 载荷，不承载 parse、程序核验或等待 | 将 NextStage 当通用动作队列、第二业务游标，或与当前动作种类/来源不符仍发出 |

每次发动作前核对当前游标、所引用源版本/Command、Run/服务/资源身份和未决操作；预算在接受意图时消耗。接收原生终态可先只提交原 final/来源；获取结束后，候选采纳、对应来源/核验、游标后继及后继确需外部动作时的至多一个待发送意图同次 Commit。已 journal 但未聚合提交只补提交，CAS 失败不派发；重复事件不能消费两次预算或创建两条下一输入。

同一 Run 至多一个可派发的业务输入或 parser 意图。进入 parse 时清除已消费的 NextStage，获取尝试只引用现 Intents 中的 parser 操作；进入 report 时原 parser 必须已结算，建立一个真实提示输入及其 NextStage。程序核验/input/pr_ready 没有 NextStage，也不为统一外形创建空操作。Attempt 的运行展示从实际操作/资源事实投影，契约是否满足由 actor 采纳结果，不保存两份可各自宣布完成的操作状态机。

本次所需前置结果 ID 和源业务输入版本在相应真实 StageInput/意图中冻结；报告的新 Command/Turn/输入版本不能替换原业务源，发布始终引用已选定的审计修复 result 及 post_head，不能从结果列表挑“最近一次成功”。新源或授权变化时，旧意图/结果仍保留历史，但不能靠相同 SHA 或状态文字恢复其后继资格。原 Allowed、PendingStop、恢复授权和 Operation 事实继续持久化，不从 StepID 或公开 status 推导/重授授权。

恢复读取原 Record、日志和 Git/provider 事实，不从当前提示词/配置或“最近 final”重建业务位置。游标与意图/来源不一致时保持 unverified/reconciling，补核验原记录，不能选择较新的字段继续；stop/原操作 unknown 优先。只做投影/补读不触发状态进入动作，不需要第二套状态协调器或对账数据库。

原生轮内问答可以使公开 status 等待输入，但业务游标仍为原 execute；只有原执行结束后，定义决定的业务等待才转 implement.input。Snapshot/Operation 对原生资源和投递事实保持原有权威，不等同另一份业务阶段。

审计原生问答同样留在 audit.code.execute，通过原问题/请求关联答复。审计 Turn 已结束后的业务 needs_input 才是 audit.code.input：不能向已结束的请求投递答案，也不把普通 reply/resume 自动当成新审计输入。本期没有该等待状态自动返回 execute/implement 的边；用户明确授权进一步实施时，沿定义的人工反馈入口建立新实施源、重新取得 pre_head 后再进入一次审计。仅补材料/恢复读取而未授权新业务工作时继续等待，不为此建设审计续轮链或统计合并器。

## 3. 结果契约、来源及判定

实施契约包含 outcome、summary，条件字段 question/blocker；实现/检查说明保留在 summary、原 final 及已有 artifact 引用中，不统一增加实施必须填写的检查清单或产物文件。机器报告或原 final 中已有的检查声明必须标注“执行方声明”，不冒充 Runner 重新执行过检查。artifact 不存在、不可访问、截断或过期均显式记缺失。

| 契约字段 | 必需性与完成条件 |
| --- | --- |
| 实施 outcome / summary | outcome 为既有三种业务值之一，summary 为非空字符串且有原材料依据；done 还须源实施满足现原生资格。原生 completed 本身不能代填业务 done |
| question / blocker | needs_input 必须 question 非空、无 blocker；blocked 必须 blocker.category/action/error，category 沿现 failure/unknown_external/permission/prerequisite，known/unknown 可选；done 不得夹带 question/blocker |
| 实施 outputs / artifact references | 仅消费已有字段和声明产物，不存在并非自动失败；若本步结论确实依赖某 artifact，该 artifact 成为该次获取包的必要来源，缺失不能由猜测补全 |
| 发布候选 / 身份事实 | 候选 URL 唯一、格式合法；真正完成须原发布资格、无未决副作用、现 provider 的精确 repository/base/head/PR 核验和现审计门槛。summary 可由已核实发布事实程序生成，不能据此生成实施/测试成功声明 |
| 审计修复结果 | done 时 fixed_count 为非负整数且等于 fixed_issues 的去重条目数；逐项 severity 为 P0/P1/P2、问题内容/修复内容及已有位置或变更依据完整；零修复须明确 0/空列表。remaining_issues 明确遗留 P0–P2，正常完成须无遗留且必要验证有原依据。非 done 可保留已知部分修复/缺口，不伪造完整统计 |

获取 satisfied 的共同条件是必需字段与必要来源齐全、字段类型/条件一致、无未解歧义/冲突，且候选未与原生或外部事实矛盾。satisfied 的 needs_input/blocked 按具体定义等待或阻断；只有 done 且源执行/原操作资格通过才有相应正常后继。development 的 implement 中，原结果已明确未成功或有效报告确认未成功，一律转内部 input；无法判断直接 report，不调用独立解析，不自动重做 execute，也不要求精确分类用户介入原因。缺失不能以省略必需字段改变契约。

发布契约需要唯一 PR 候选及当前固定 repository/base/head 身份、原发布执行资格与没有未决发布副作用。URL/number 只是线索；`pr_ready` 必须经过定义要求的审计 gate（审计版须当前源的审计修复完成、报告完整且修复后工作区核验有效）、provider 成功查询和 `ValidateWorkerPRIdentity`，PR 合并仍由既有 refresh 独立核验。仅有 PR 存在不证明所有业务检查成功，更不证明已合并或 TaskDelivery。

审计修复契约使用 outcome/summary、fixed_count、fixed_issues 和 remaining_issues，详细字段与来源见[审计版 §3.2](../workflows/development-audited.md#32-auditcode审计并修复与结果获取)。不再使用 verdict/changes_requested 决定返修；已修复 P0–P2 不再作为遗留阻断，P3 不计修复数量也不单独阻断。独立 parser 不审查、修代码或验证；不能补造修复记录、由沉默推断零修复/无遗留或降低 severity，报告字段有原依据才可接受。

统一规则：

- 仅完整、来源已核实且与当前输入绑定的机器报告可直接满足语义字段。严格拒绝重复键、相互矛盾字段、多个候选对象；不从任意文本中挑一段 JSON 当真。可以有显式格式的固定 artifact；不为猜测格式建立扫描平台。
- 程序负责已核实原生状态、artifact 元数据、PR/provider 事实；模型负责从材料中整理语义。模型不能覆盖原生失败/中断、未知外部副作用、已绑定 PR 或外部事实。业务 done 必须同时满足原执行资格及契约，不从“解析成功”或“没有发现问题”推出 done。
- 对每个字段记录来源与可用性：present/missing/ambiguous/conflicting；保留冲突双方。已核实事实优先于模型声明；两个同等可信事实冲突时阻断，不能最后写入获胜，也不能用模型投票消除。查询快照按时间/目标分别保留，不把后来真实状态变化误当重复回执冲突。
- 解析输入只包含已存在材料。完整 raw final、原协议日志与固定 artifact 保留在原位置；包内保留引用/摘要/范围/截断标记。传给模型的副本按既有脱敏规则处理，普通事件和公开 inspect 不泄漏原始秘密。脱敏导致语义缺口时记 insufficient。
- 对原执行缺可信 final，先沿原来源补读；已有机器报告可以保存候选信息，但不得绕过当前生命周期的成功缺 final 门槛。原执行失败可保存诊断及已核实 PR，不将其格式化为业务成功。补报不运行实现、测试、审查或发布。

### 3.1 审计修复的提交来源绑定（首期固定方案）

复用 `WorkspaceInspection` 的 repository/worktree/branch/base/HEAD/Dirty 及原 Git owner。现检查已核实工作区身份并执行 `status --porcelain=v1 -z --untracked-files=all`；它没有通用脏工作区内容快照。本期选择提交级绑定，不补建快照/文件指纹平台：审计版的实施应在原正式 execute 中提交实现，审计修复在本次 execute 中提交最终改动；Runner 的提取、核验、parser 和 report 都不能替它们 commit、清理、修改 ignore 规则或补做工作。

首期来源条件限定普通完整 Git 工作区：内置 Git 检查须识别 gitlink/submodule、稀疏/skip-worktree、assume-unchanged 等使 Dirty=false 不能代表所核验源码范围的情况；遇到这些情况保留 unverified/既有阻断，不自动展开子仓库、清索引标志或建设支持矩阵。该窄检查归原 Git owner，不能只信模型所说的“干净”。必要 ignored artifact 仍按已声明引用冻结，不把 Git clean 解释为全目录所有内容已入提交。

以下检查是接受完成结果和正常后继的条件，不是保存原 final、失败诊断或已知部分修复的前置。原生失败/未知沿原生命周期优先处理；原生已结束但业务明确未完成时，仍可保留未提交状态和部分报告并按定义等待/阻断，不为生成 pre_head/post_head 自动补 commit，也不将来源未核实改写成业务失败。原 final 来源完整且无未决操作时，现有审计获取策略可整理非 done 的问题/部分统计，所用 Git 检查可如实记录 Dirty=true，此时不产生“合格 post_head”或发布资格；不能把本节完成门槛误用为解析业务 needs_input 的前置。对 done 的采纳仍须全部检查，解析不能补造缺失的原提交事实。

| 绑定 | 保存和核验的最小事实 |
| --- | --- |
| 审计输入 | 当前实施 result ID、真实实施 Command/Turn/输入版本；实施原终态核验时记录干净提交为 pre_head 并绑定该 result。固定 repository common dir/worktree/branch/base，实际进入审计前重读 HEAD=已冻结 pre_head 且 Dirty=false，不能到入口才把任意当前 HEAD 绑定到旧实施结果 |
| 正式审计修复 | 唯一真实审计 Command/Attempt/Session/Turn、冻结输入及原 final 来源；在同一工作区允许本次正式执行修复并 commit |
| 审计输出 | 原审计 Turn 完整结束且无已知未结算/未知业务写操作，由 Git owner 读 `post_head`、Dirty=false 和同一身份/base；post_head 包含 pre_head（可相同），读取前后 HEAD 一致。结束的是本次写工作，不要求原 Codex/app-server 退出，原会话可保持空闲供一次补报。完成谓词和 fixed_* 字段指向此审计及其前后提交 |
| 获取尝试 | 解析/补报绑定上述原审计及 pre_head/post_head，材料包摘要固定；获取本身不更新 HEAD、不写新代码或把报告 Turn 当新审计源 |
| 发布资格 | 结果接受时、实际派发发布前、provider 核验时重读原工作区；当前 HEAD=post_head、Dirty=false、身份不变，无未知写操作。发布只能 push/create PR，不重写/补做实现 |
| PR 结果 | 沿现 repository/base/head/URL 核验，审计版再由定义要求已查询的 PR.HeadRevision=post_head；当前 ValidateWorkerPRIdentity 有意允许同分支 SHA 前进，故此提交比较是明确新增的审计版条件，不改变通用 provider 接口 |

`pre_head`/`post_head` 使用 Git 既有提交 ID，不新造全局内容版本、子 Run 或审计专属版本库。无修复可沿用同一提交，fixed_count=0 仍须明确完成审计及必要验证、无遗留；不能从 HEAD 未变推断没有问题。入口脏/完成后未提交、缺 Git 事实或关系不符时，不接受审计完成、不派发布；已知业务未完成按原等待/阻断，事实缺失/冲突/未知按原对账门槛。补报只报告缺口，不能补 commit。

接受 done 且 fixed_count>0 时，还须 pre_head/post_head 对应的 Git tree 确有差异；同一提交或仅新增空提交不能支持已完成的正数代码修复。此为必要一致性检查，不证明每项修复正确；内容和验证仍是有原依据的 agent 报告，不新增独立审查/重跑测试。tree 可从固定提交由原 Git owner 只读取得，不新增持久内容版本或通用 diff 分析器。未完成时的已知部分统计仍可保留，不用此条件擦除原报告。

原实施 result 始终指向 pre_head，不被 post_head 覆写；审计修复是在该输入上的已授权变更，故 post_head 前进不使自身失效，也不产生新的 implement Turn。新实施或后续独立工作使 HEAD/Dirty/身份变化，则旧审计材料可保留，但其后继资格失效；迟到解析、报告、重复回执只能归档原来源，不能把旧结果绑到新 HEAD。重启保留原 pre_head/post_head，只重读并比较，不能接受当前 HEAD 为新默认，也不自动重做修复/验证。

pre_head/post_head 事实分别随原实施/原审计终态记录到原 Record，再用于后继/获取。检查值复用 Git owner 的 WorkspaceInspection 及必要只读补充，关联本次 Run/业务 Command、Attempt/Turn、工作区身份、观测时点和原源引用；actor 在源/授权未变化时以 CAS 接受，不新增检查服务、签名凭据或独立 Git journal。只有既有操作证据已经完整记录这些关联事实时才可复用其 journal；仅有原生 final 的 journal 不算提交检查完整。单独的 agent 自报 SHA、当前分支/reflog、未固定摘要的现存材料文件都只能是线索，不能补成原检查事实。若崩溃时缺少上述持久事实，保留 unverified，即使当前 HEAD 干净也不能首次创建原 pre_head/post_head；不为缩小该窗口再建设一套日志/版本系统。已有完整证据仍由 actor 在原源/授权适用时只读重放补提交，不重做源工作。

审计版实施已固定 pre_head 时，实施补报前后必须比较该值；审计已固定 post_head 时，解析/补报前后必须比较该值，均核对相应工作区事实。没有合格 checkpoint 时只能整理非 done 的原问题/部分信息，不能借补报后的新 HEAD 也干净而建立或重绑原完成资格。后备前还须原获取操作已结算、原源和授权仍适用。对已经观测到的补报修代码/commit、重做检查等违反纯整理契约的行为，不能因为 HEAD 最终相同或结构有效就采纳；保留诊断/真实事实且不推进。原会话报告目前仍沿冻结执行权限，不能把提示词或几次 Git 检查宣称为完整只读隔离，也不在本期另建工具行为监控平台。普通 development 不因此新增提交/干净检查要求，其报告仍受 V-SOURCE/V-REPORT 及已知事实约束。

发布采用固定的只读核验顺序：冻结本次已选审计 result/post_head 及源引用；派发前核对授权/原操作和本地版本；原发布结束后先核对本地 HEAD/Dirty/身份，再查询原 PR 的身份及 HeadRevision，查询返回后再核对本地版本和 actor 中的源/授权/stop 状态，最后 CAS 提交结果与 pr_ready。查询期间新源、停止、HEAD 变化或 CAS 失败不能提交正常后继；已取得 PR 事实仍可按原迟到事实规则保存，重试仅核验原目标，不能重发 push/create PR。

外部 Git/provider I/O 使用已冻结源的只读请求，在 actor/存储提交临界区外执行，并沿既有 worker context、期限及 managed 查询收尾；actor 只接受返回事实、复核当前源和授权后 Commit。不得将新 Git 检查直接塞入 finish 已持有的锁中，使 stop/新源无法在查询期间被接受；也不为此新建核验调度服务或第二写者。

Git、provider 和 SQLite 不是一个事务：结论只证明已记录观测时点的提交一致性，不能保证外部 PR 此后永不前进。使用现 Run/工作区归属和单写边界，不增加分布式锁、轮询监控或快照服务；只读前后检查不能声称对任意外部写者实现文件系统隔离，已知并发/未知写操作仍先对账。已到 pr_ready 的里程碑保留当时已验 post_head，后续 refresh 仅记录真实 PR 变化，不能声称新 head 也已审计，不自动返工或提交 TaskDelivery。

## 4. 当前步骤的建议策略

以下是本期实际内置策略，不是所有组合的穷举。配置默认关闭独立 parser；开启后才使标注的外部解析分支可选。每次源实施/发布/审计版本分别计预算，明确新实施产生新源；正式审计内修复保留本次审计来源并关联修复后工作区；获取重试不产生新源或重置计数。

| Workflow / 步骤 | 选择、后备与完成条件 | 理由 / 预算 |
| --- | --- | --- |
| development 的外层 implement / 内部 execute | execute 结束识别可信 final 中的阶段成功声明；命中且原执行/必要结果核验通过直接完成。无法判断时直接 report，原结果已明确未成功或报告确认未成功则 input；报告有效且业务 done 才完成；报告仅一次，不追加格式修正或重试，格式无效/信息不足且操作已结算则 input。事实冲突/原操作未知先对账。本定义无独立解析状态/提示词 | 命中且核验通过为 0 次获取模型调用；后备每源最多一次原 agent 报告，无格式修正或报告重试，独立 parser 调用/探测恒为 0，不因未成功自动重跑 |
| development-audited 的实施结果获取 | 与 development 相同：可信 final 命中阶段成功输出且核验通过直接完成；无法判断直接 report，未成功或已结算的获取失败统一 input。完成后进入 audit.code；真正新实施使旧结果及代码审计原子失效 | 每源最多一次 report，无格式修正或报告重试，不调用/探测 parser；审计修复不单独回到实施或增加实施输入 |
| `development-audited` 的 `audit.code`（审计并修复） | 原 agent 在同一次正式执行内审计代码、直接修复 P0–P2 并验证，报告实际修复总数和逐项内容、遗留问题。2026-10-21 新 Run 默认提示优先要求同一审计 JSON 契约，旧冻结文本/显式覆盖保持；只是文本要求，不新增原生 schema。已有完整可信机器结构直接核验，否则 parser 开启且材料适用时独立 parse。未选/不适用或已结算的解析失败 → 原上下文一次 report；仍无效阻断 | 每当前源一次正式审计修复输入，获取最多一次 parse 和一次 report，无格式修正、自动返修或重审。修复报告完整、确已完成/无遗留 P0–P2 且修复后工作区绑定有效才发布；不以解析成功或修复数量大于零推断完成 |
| 两种流程 `publish` | 发布 agent 正常执行并输出自然语言；两份定义均注入阶段成功输出，命中后仍取 URL 并核验。程序从原输出/原生 PR clues 取得唯一候选，再用现有 provider Query(URL) 校验固定分支等身份，核验发布契约；完整不调用模型。现 provider 没有按 head 列举 PR 的公开端口，无 URL 时直接选原 agent 定位补报，不假定已有分支反查能力。查询失败/未决副作用只对账原发布；仅语义/PR 定位缺失且原发布已核实结束时，原 agent 补报一次 | PR 主要是可查询事实；这两份定义均不选独立解析，不探测 parser 环境。最多一次 `publish.report`，保持现有无二次发布、无补报再修正规则；本期不扩大 provider API |
| `implement.report` / `publish.report` 实际被选择时 | 显式结构要求，仅整理指定原源；程序校验并写统一结果，不把此 Turn 的 success 当原执行 success | report 是获取方式，不是新的实施/发布；审计补报同样只整理原审计，来源指向真正原输入而非上一次坏报告 |
| 发布前明确人工输入后的新实施 | 复用实施策略；新实施输入原子失效旧统一结果/审计资格；审计版之后重新进入一次 audit.code | 保留原工作区/会话；技术 resume 不默认新实施/审计，原审计内修复不另建实施 Turn；无发布后返工或复用先前 PR |
| PR refresh、`pr_ready`、last-message / 原生问答 | PR 沿现查询；last-message 保持真实原生消息；原生问题仍按原请求投递，无解析插入 | 这些不是新的语义报告步骤，不增加模型调用、不影响 TaskDelivery |

统一能力仍保留可信 final/程序提取、独立解析、原 agent 报告三种获取方式；development 只选第一、第三种，development-audited 仅审计结果获取选独立解析。两份定义的实施/发布命中阶段成功声明且必要结果与核验满足时无需补报，无法判断直接报告；每原源 report 最多一次，无格式修正。原生 completed、任意正文片段或部分字段不能代替声明及核验；程序查询默认每次最长 30 秒，模型不能覆盖程序已确认事实；所有重试沿原目标和预算，重启不重置。

审计独立解析后材料仍缺失，agent 补报只能利用已有审计上下文报告知道/不知道；需要真正检查时停止获取并留下缺口，由既有人工反馈/实施授权另行处理。不得把“请补证据”作为自动重做测试的暗门。

## 5. Runner 独立配置与认证

首期配置入口为当前 `--state-dir` 下可选的 `service/result-acquisition.json`，由 Runner 读取，按显式 schema 精确校验未知键/重复键/数值范围。不是 RunRequest、execution.settings 或 SteerLine 项目配置字段。文件缺失采用内置默认；配置改动仅影响后续新接受的 Run；接受前冻结有效值/文件摘要/策略版本/提示与 schema 到原 Run。已有 request_id 的重试先返回原回执，不按当前文件重冻。

Runner 独立配置如下；启用后仍按需核验原生环境：

```json
{
  "schema": "steerrunner-result-acquisition/v1",
  "codex_parser": {
    "enabled": false,
    "executable": "codex.exe",
    "model": "",
    "reasoning_effort": "",
    "codex_home_env": "",
    "timeout_seconds": 120,
    "max_input_bytes": 131072,
    "max_output_bytes": 32768
  }
}
```

上述是缺省值；允许 timeout 10–300 秒、input 16–256 KiB、output 4–64 KiB；首期每个 Runner 服务固定最多一个活动 parser，用进程内限额控制即可。等待只保留原 Run 中尚未发送的意图和原 deadline，不另建持久队列、公平调度或资源池，不新增并发配置面；重启不自动派发等待项。方式/字段/重试预算在内置 Workflow，配置只调整 parser 的能力与资源上限，不提供策略 DSL。max_input_bytes 约束 Runner 冻结材料包，不是 Codex 内置指令/工具描述或 API 总 token 的硬上限；实际 usage 仅按原生可观测值记录。包超限直接选原 agent，不隐蔽截断后宣称完整。单个 final 的外部解析选择阈值为 64 KiB，且所需 artifact 全部已纳入包，总包受 max_input_bytes 限制；阈值与策略版本冻结。首期仅 UTF-8 文本/机器结构，二进制或无法导出的必要 artifact 选择原 agent。

限额不能在重启后简单清零：从原 Record/Intents 的实际 parser 资源事实恢复占用，可能已启动但未核实退出的原解析仍占槽；原资源未结算前不启动第二个 parser。只是原事实的内存投影，不另存全局占用账本。不依赖解析的流程仍可沿既有资格执行；当前尚未启动解析的等待项超时可按其已冻结策略后备，不把其他 Run 的未知操作移作本 Run 的失败。

`enabled=false` 或本步骤策略未选择 parser 时，不发现、启动、检查其 EXE/登录/模型。已启用也在真正需要 parser 的边界才惰性资格核验；明确不可用进入可后备的获取失败，不让普通 Run 的 submit 因无关 parser 缺失被拒。配置文件本身语法无效属于 Runner 配置错误，须明确拒绝新 submit、保留已有 Run 控制/停止能力，不能静默采用不同配置。

显式 executable 优先，否则 Runner 用现有 NativeExecutable 解析规则发现 PATH 上的原生 EXE，拒绝不透明 wrapper/共享 daemon，保存绝对路径/版本/文件摘要。首个资格候选固定 Codex `0.159.2`，其 parser 权限 profile 与执行 profile 独立；版本相同不代表解析隔离已通过。

`codex_home_env` 是一个非秘密环境变量名，指向服务主机上已有 Codex home；非空但未定义/路径无效为明确环境错误，不回退其他登录。空值不设置新 CODEX_HOME，继承 Runner 服务启动环境，由 Codex 按其原生默认环境发现认证。使用环境名及解析后的非秘密路径作为引用；不保存 auth.json、OAuth/token、API key 或账户凭据摘要，不复制 home 来创建解析身份。

Codex 自己登录、读取原生 credential store 并续期，Runner 只管理资源；不新增登录账号、不调用 account/login、不提取 token 调私有接口。宿主已有 API-key 环境如被原生使用仍归 Codex 的原生路径，Runner 不读取值或转换成请求头。解析子进程不继承业务 GitHub auth broker/capability、provider token 或执行工具环境。保留 Windows 系统目录引用（含 ProgramData、ProgramFiles、SystemDrive）供原生 MXC 定位平台文件，不把环境删到沙箱本身无法启动。

模型/effort 可与执行及审计模型不同；空值采用受管解析调用下的 Codex 原生默认，不隐式继承执行模型。严格配置下用户文件的 model 不保证生效，须在原生探针核实并公开实际模型。指定模型不可用时明确失败并按策略后备，不偷偷换模型或账户。model catalog 只作候选发现，不证明当前登录有权限；以当前真实调用成功为证据。认证失效、额度、模型无权限分开诊断，不由 Runner 登录或无限重试解决。

## 6. Codex 接口选择与实际隔离边界

### 6.1 2026-10-18 确认的最小边界（替代下文旧 P0 强隔离门槛）

用户确认：取消“全部工具在模型侧不可见”和“所有隐式上下文完全消失”的准入要求。首期只整理已经完成的审计输出；不建设纯推理容器、Codex fork 或新沙箱。使用新的原生 `codex exec`，不 resume/fork 执行会话。独立 cwd 位于业务仓库之外；stdin 是绑定原审计及原实施的冻结 final/契约包，当前两定义无需自主 artifact 检索，材料不足或超限选择一次原 agent 补报。

| 边界类别 | 本期承诺 | 不承诺 |
| --- | --- | --- |
| 原生权限 | 显式 read-only、approval_policy=never；无业务写根/发布授权；关闭不需要的 shell、apps、MCP、搜索、子 agent、插件等能力。沿原生 Windows 沙箱执行，不能以宽权限重试 | 不把只读 cwd 当 ACL，不把命令沙箱当作所有 MCP/连接器的统一边界，不声称整个宿主只可读输入包 |
| 上下文控制 | ignore-user-config 保留原生 CODEX_HOME 认证；project_doc_max_bytes=0、关闭 memory/可关闭 hooks、独立无仓库 cwd；材料显式供给 | 不要求排除 Codex 内置指令或受信任宿主管理策略，不证明零隐式上下文。会执行额外业务动作的宿主管理 hooks/外部连接不得启用本 profile |
| 行为约定 | 提示词要求只整理输入、缺失如实表达、不调用工具、不审查/测试/修复/发布 | 提示词不是安全边界，模型可能违反约定；结构合法不代表业务成功或语义正确 |
| 事后检测 | 读取本次原生 JSONL；任何工具 item（含未知类型）、协议/输出不完整均拒收，终止 owned Job 并核实停止，保留原文与诊断 | JSONL 检测不保证在工具动作执行前拦截；不得用事后拒收替代原生权限和外部能力关闭 |
| 结果与资源 | 仅 Runner 沿原来源/Git/provider 规则采纳；每源一次解析、截止时间、并发 1、原 Record/Intents/Commit/CAS、owned Job 与既有停止核验 | 不建立第二状态权威；进程归属/停止未知不转补报，不重新调用模型来恢复未知原操作 |

明确工具违规但 owned 树已停止、原源/工作区仍适用且没有已知业务副作用时，记获取失败，可用预算内一次 agent report；发现业务写入、外部副作用未知或资源停止未核实时阻断并对账。仅在固定原生命令权限内的本地 command/todo 违规可按上述条件后备；MCP、文件修改或未知工具 item 记 side_effect_unverified，进程停止不解除该未知，原意图回执保留且阻断后继/技术 resume。本期不增加通用副作用消除或人工覆盖接口。此前“任何工具事件都永久隔离失败、禁止后备”的规则由本段替代。保留已知事实、冲突和原生失败优先级。

配置仍默认 enabled=false。显式 model 不静默回退；空值采用 ignore-user-config 下的原生默认，不等同于用户 config.toml 的 model。真实资格要求正常解析、违规调用/写入反例、原认证和资源停止证据，不要求空 ToolPolicy 证明。此前 2026-10-17 的 unsupported 记录保持历史原样，不倒改验收结果。

### 6.2 原生资格和支持范围

当前 profile 固定 Windows / Codex 0.159.2，原生版本与 EXE 摘要随实际获取记录；主机或显式模型不可用只使该次获取明确失败，不改变业务执行策略。先用已有公共 app-server 的 initialize + config/read（不创建 thread/turn）读取连接名称与配置来源，逐项追加 mcp_servers.<name>.enabled=false；空表会深度合并，不能用 mcp_servers={} 冒充清空。配置值不返回、不记录；有已声明的非用户层动作 hooks、不可安全禁用的连接名称、缺少配置来源或未核实配置读取进程树退出时不启动模型。原生插件/apps/hooks 等同时关闭。受信任主机配置须在本次资格核验与启动间保持稳定；不建设与恶意管理员竞态的防御平台。

模型进程采用新的 exec、冻结 stdin/schema、独立空临时目录（不得位于 Git 仓库内），原生 read-only/mxc/never，并显式选择同名只读 permission profile，避免受管主机默认选成可写；schema/final/JSONL/回执存原 state-dir 的 owned acquisition 目录，不占执行上下文。未指定 model 时采用跳过用户配置后的原生默认，exec JSONL 未报告实际模型名时记不可得，不从 catalog 或模型自述补造。显式模型调用成功才证明本次账号可用。

部分原生模型元数据强制 Code Mode；关闭 code_mode_host 会在 turn 开始前失败，因此保留此原生运行时，继续关闭其业务工具/外部连接。它不等于空工具注册表或禁止模型请求工具；JSONL 实际暴露的工具 item 仍事后拒收并终止 owned 树。0.159.2 的 exec JSONL 不输出所有原生 item（包括 Code Mode 外层计算），故零观测事件不证明零模型工具调用；不将它当完整调用审计或提前拦截。外部连接关闭与命令原生权限仍是主要动作边界。兼容性不通过扩大命令权限解决。原生启动错误即使在 turn.started 前出现，也记明确获取失败，不采纳为结果。

解析完整输出、原生 session/完成、final 文件与 JSONL 消息一致、零工具事件、退出和 owned Job 停止均合格才形成候选。缺失不能补零，原生失败不能由候选覆盖，语义是否真实仍按源/Git/provider 核验。提示词与事后事件检测不能证明提前拦截，原生认证/历史缓存写入仍由 Codex 管理。

2026-10-17 的强隔离调查及 unsupported 结论保留在[原实施记录](../acceptance/evidence/result-acquisition-20261017/README.md)，不倒改历史结果；本期改按 §6.1 的承诺验收。当前代码与测试执行状态另见[本轮记录](../acceptance/evidence/result-acquisition-20261018/README.md)。

## 7. 生命周期、恢复与原子性增量

共同 W/E/V 规则继续作为语义入口；下表为本功能的唯一差异维护，不修改 SteerLine 文档。

### 7.1 固定原生控制终端（2026-10-19 确认增量）

主线签名退出增量沿 [Orca 停止取证 Plan](orca-shutdown-verification.plan.md#正向退出证明补齐2026-10-19) 维护，不复制其状态机。固定控制查询须完整保留主线原 reader 的 `reader_writers_absent`，失败查询的资源结算与业务结果分离；在原 readiness/owner/公钥/输入及 reader 退出核实前，不允许新请求覆盖该回执。原请求不确定时仍只对账原请求。原生准备沿主线 readiness v3，内部 Orca state codec6/profile3，无旧格式兼容迁移。

停止认证对账后，必须先通过主线已有 artifact 通道取得并验签原 helper 的退出凭据，确认原 app-server/tools/TUI Job 与认证结算，才能关闭业务 PTY；缺该凭据时保留原 helper，不以 PTY 清单或认证报告替代。原控制认证/关闭仍独立核验；被动服务停止仅释放观察并读取已有证明，无法核实停止时不宣称 stopped。具体接线、测试与证据见 Plan §6。

Codex 经 Orca 运行时，原生回执查询和认证撤销不再逐次 `terminal create`。沿现有资源准备操作，在启动执行 agent、发送任何业务输入之前创建每次执行独立的控制终端；原 runtime、worktree、terminal handle/incarnation、helper SHA、配置摘要及资源 generation 随适配器 implementation 同次提交到原 Record，确认固定 helper 就绪后才允许 agent 启动。Pi/DSH 的现有本机回执路径不增加此资源。

控制 helper 只接受三类有限操作：查询原 session/输入的原生回执、核验原工作树审计来源、请求并读取原认证 owner 撤销证明；工作区绑定按 Windows 等价路径规范化；失败查询也沿原 30 秒间隔退避。请求关联 ID 和完整载荷绑定，禁止任意命令、模型输入和业务执行。原生完整 final、session/输入摘要、原进程/helper/config/workspace 及状态核验保持；尚未完成的控制请求与业务发送计数分别核验：前者不构成业务输入，也不能通过最终完整性/关闭验收。控制响应不是新的业务结果权威，也不替换原 final。查询可重复读取已有事实；认证撤销复用原 protected request/cleanup 的幂等语义。

Orca local 受管审计沿同一固定通道的 `workspace-audit`，只接受原 workspace intent 与可选完整 pre-head；没有任意命令或新路径参数。原 helper 核验 readiness、session、二进制摘要、配置/仓库和原 native 进程，在原执行主机复用 Git `InspectAudit`：完整 HEAD/tree、clean、分支、固定 base 与 pre-head 祖先、隐藏索引/gitlink/稀疏配置检查。禁用 Git 可选锁和 fsmonitor；每个固定 Git 子进程在原受管 Job 退出核实后才形成 `writers_absent`，失败/未结算不得伪装来源合格或覆盖唯一原回执。请求/响应沿同一 ID 和完整载荷绑定；每次新阶段重新读取，成功旧查询不缓存成当前状态。调用方只经原 adapter owner 取得原事实，不读取远端路径对应的本机文件，也不把 revision 元数据晋升为 clean/tree 证明。代码审计修复和其补报使用冻结 audit 模型，实施/发布恢复 execution 模型，实际原生 model/effort 回执独立核对。资源、权限、Run 授权与 Workflow 后继仍归既有 owner。新远端遵循原生 Orca 契约：不调用 helper/workspace-audit，不读取调用方同名 Git；来源保持 unreported，development-audited 在接受前返回 unsupported_capability，不以启动参数冒充实际双模型或工作树来源证明。

执行期、服务恢复与停止不得新建控制终端。原身份可核验时，只观察原控制终端，按同一请求 ID 对账丢失响应；必要的终端发送恢复使用已核实的原生请求去重能力。普通 helper 的原生回执只保证 input_accepted/原 PTY incarnation 与去重；provider/turn-start 可为 unsupported，实际操作完成必须由有限协议的关联应答核验，不伪造 agent 轮次开始证明。沿现有 caller-owned 辅助检查点保存传输中的请求/回执，绑定原 Record/资源；它只能恢复原派发和事实读取，不能决定 Workflow 后继、重新授权输入或调度第二套任务。原 Record/Intents/CAS 仍为唯一状态权威。未知或冲突回执不推进；原控制终端丢失/换代后阻塞，不启动替代者。

Run stop 先取消原观察器，控制通道在撤销请求之后拒绝新回执查询，防止晚到观察覆盖撤销证明；随后在独立的 45 秒有界清理上下文中对账原撤销请求并核验 broker/socket 清理（不因调用方预算用尽而提前销毁回执通道），仍须在有界预算内停止原业务写者，即使撤销暂时无法核实；撤销证明尚未核实时保留原已绑定控制终端，原业务写者停止后也只能继续原请求对账，不能关闭唯一通道或宣称结算；证明合格后再核验控制终端关闭。服务停机沿既有 continue-execution 规则仅释放观察，保留已绑定控制终端供原执行恢复。缺任一归属、撤销或关闭证明均不能声明资源结算。停止重入只读取原证明、关闭原资源，不创建临时撤销者。

原生 `terminal create` 缺少创建请求对账保证仍是初始化边界：身份未知时保留原未决资源操作，不按标题/空 inventory 认领、不重试创建，零业务输入且阻塞；本增量不承诺初始化任意强杀自动恢复，也不改写旧失败现场。内部 `orca.terminal-state` codec 从 5 升为 6，旧版 Orca implementation 明确拒绝，不建设兼容迁移。公开 CLI/调用配置及业务阶段顺序不变，SteerLine 和 Orca 均不修改。执行期强杀恢复、实际回执读取、认证撤销及初始化反例的验收安排见 Plan §5。

执行准备继续归原适配器管理。本机受管 GitHub 就绪探针由同一原生 sandbox 内实际使用的认证 `gh` 身份/仓库查询和 Git 远端访问核验连通性，不额外依赖匿名 `/meta` 的共享 IP 配额；权限、凭据摘要及 Git/remote 资格条件保持。

Pi/DSH 公共工具初始化失败可在原执行端证据目录写入内部 `preparation-failure.json`，仅保留有限错误码及 Run/会话/bundle/worker PID 来源。只有初始化尚无原 readiness/owner/launch/turns、且 readiness 文件缺失时，原适配器才在核对原 owner 后将其作为失败诊断；它不构成就绪、业务结果或停止结算证明，不覆盖活动会话缺失的原生证据，不新增 Workflow 状态、操作权威或重试路径。该准备诊断不另增公开接口或持久权威；本轮固定控制终端的内部 codec 升级按本节前述规则。准备修复源码及复验见[执行准备修复记录](../acceptance/evidence/native-preparation-20261019/README.md)。

| 位置 | 增量与保留规则 |
| --- | --- |
| W14 / V05,V08 | 先原子保存原执行终态、原 final 及待获取意图。可同次完成程序提取；需要异步解析时先提交意图再启动。获取结果 satisfied 与真实下一业务输入意图原子提交；提交失败/相同回执幂等，冲突不推进 |
| W15 / E06 | 报告成为策略选项；原生缺口仍先补读。获取意图与 agent 补报各有来源/次数；无 agent 调用的方式不伪造 StageInput。报告权限沿实际执行 adapter 冻结策略，如现状仍只是提示词只读则明确记录，不冒充强隔离。实际补报原生失败沿原会话核验/人工恢复门槛，不自动再发；恢复获取不能被旧默认分支解释成重做实施授权，源实施本身失败后的既有人工继续规则仍保留 |
| W16 / V05 | 程序取得 PR 线索及查询不重发发布；unknown_external、原生失败和外部事实优先。PR 可在流程未完成前独立绑定；不因解析成功直接到 pr_ready/merged |
| W08–W13 / E04,E05,E07 | parser 子操作同样先保存意图与资源 generation，再启动/投递；未知输入只对账原进程/原日志/原会话，零换 ID 重试。不能把 clientUserMessageId 或本地获取 ID 当服务器 exactly-once 保证 |
| W18 / V06,V07 | Run stop、service stop、shutdown 和 verify-stopped 必须包含解析进程/Job、晚到启动、日志句柄和工作目录；不能在现有“无执行 External 就已停”分支漏掉 parser。只结束本 Run owned 资源，不碰用户 Codex/其他 Run；parser 未核实停止则资源未结算 |

原执行来源不完整或存在未知副作用时不启动语义解析来解锁后继。parser 明确失败且原 owned 进程已证实停止后，若原授权有效可按预算转 agent；输入是否送达/进程归属未知则 acquisition unverified，先对账，不自动并发补报。输出已完整 journal 但聚合提交失败可只读核验补提交，不重复模型。回执读取保持 2 MiB 有界，覆盖已允许材料/输出的 JSON 与 Base64 封装开销，材料/输出本身上限不变；超限日志拒收，不重发原操作。因超时取消须核实原进程树退出；仅 cmd context cancelled 不证明停止。

服务重启：保留未决操作、截止时间和计数，只读对账/停止允许；不自动启动 parser、不重发其 stdin、不发 agent 补报或业务后继。显式 resume 后，原未知操作已解决且资源已结算才可执行尚未发送的获取意图/预算内后备。已经发送的 parser 不因纯函数语义而自动重跑。原日志不可恢复时记缺口；不能用新会话替换未知原解析。恢复获取的分支须先于现有默认“回到 implement”逻辑，且仅恢复获取，不改原业务步骤。parse 后可能选择原会话补报，故显式恢复须先绑定/恢复原执行资源但不投递业务输入；原 adapter 的补报权限政策在原解析意图内冻结，不能从尚未重建的临时 port 推断。

新实施/反馈导致源变化后旧获取结果和审计全部失效；迟到 parser/补报只可记到原获取记录，不得覆盖当前结果或重开已 stop/关闭的 Run。stop 撤销输入后，即使结果晚到 satisfied，也不能排入新补报、审计、发布；按既有 late evidence 规则保存事实。解析目录清理只在资源停止已核实及持久来源可读后进行，保留验收所需的原始证据。

## 8. 必须调整的接线及版本影响

必须调整 `Route` 的输入接口：不再把每个 StageResult 都假设为业务 JSON；分别接受原执行事实、已接受统一结果和核验结论，由当前冻结定义的边决定后继。现有 if/switch 中的业务条件迁入两个具体定义，统一运行层只提交和派发合法转移；后继选择不移入 parser、adapter 或核验 owner。`DecodeReport` 可复用业务字段校验，但不再等于“读取某真实报告 Turn”。

`AuditReportCommandID` 改为当前实施 result 引用；`AuditRecord` 和审计输入绑定 result ID + 原实施 Command/Turn + 源版本/工作区证据。`AuditSource` 不要求源 StepReport，须验证实施成功/完整原生来源、统一结果 done/satisfied、当前源及 artifact 关联；审计来源只按 §3.1 的 pre_head/post_head 协议保存/核验，实际需要调整 AuditSource/AuditsPassed 和 workspace/provider 接线；不另建快照模型，也不把已授权修复误认为新实施。真实审计 Command/Turn 仍保留。原 agent 报告作为来源之一保留，不能让无模型结果冒充报告命令。同步 `RenderInput`、queueInput、finish、feedback、resumeWorkflow、AuditsPassed、view 投影和相关测试。

保留公开 CLI `steerrunner-cli/v1` 信封、原请求幂等、错误大类、Run/控制/PR/真实 final 字段。inspect 增加可选 `result_acquisition`（当前 contract/状态、结果 ID、来源摘要、缺失/冲突和尝试列表），无秘密或凭据正文；完整材料仍在 state-dir。`last-message` / final 永远是实际执行/审计/补报 agent 的原生消息，parser 的文字仅在获取记录中展示。事件沿当前流增加获取事件，不新建事件游标或查询命令。

内置流程变更为 `development/2`、`development-audited/2`：版本表达报告由固定轮次变成获取策略，并将审计版缩减为仅代码审计。保留 `implement.report`、`publish.report` 提示覆盖名称，只有选中真实报告方式才使用；不因用户有提示覆盖就强迫调用报告，也不把补报覆盖发送给独立 parser。真正代码审计沿已有 `audit.code` 提示覆盖；workflow/2 不再提供 `audit.spec` 状态或覆盖键，提交该键须按未知覆盖键拒绝，不保留旧路径。新增审计整理提示为内嵌资源、无新增调用方必填配置或调用步骤；独立解析提示只进 parser 会话，审计补报只进原会话。`development.json`、catalog、prompt 资源、结果契约及版本同源冻结。configuration_schemas 的既有 adapter/provider 外层仍为 1，新增 Runner 本地配置 schema 单列，resource_version 增为 2、resource_digest 和产品构建版本更新。不继续提供 workflow/1 的执行分支。公开审计结果改为审计修复契约（含 fixed_count/fixed_issues/remaining_issues），移除旧 verdict/findings 返修语义及 RepairCount/audit_repair_limit 控制，来源字段改为 source_result_id、原实施关联及修复后工作区证据；外层 CLI v1 不代表 workflow/1 的审计字段语义继续适用。

当前持久 `steerrunner-state/v2` / SQLite user_version=2 升为 v3 / 3：原因是报告命令型审计关联和恢复判断被统一结果关联取代，不能用省略字段默认值让旧运行误推进。仍用原表和单写 Record，不新增独立状态权威；旧格式启动前明确 unsupported_schema，保留旧目录，不迁移、双写或建设兼容路由。升级/部署不属本轮。

### 8.1 “SteerLine 不改”的核查与具体限制

已只读检查 SteerLine `runnerclient/{catalog,views,selection}.go`、`app/runner_config.go`、`taskrunner/service.go`：响应以 json.Unmarshal 读新增字段，schema 外层要求 1；但 workflow ID/version 从已配置 profile 透传，并按 capabilities 校验，空值还会回填 development/1，不能误认为调用方会自动采用 workflow/2。保持 CLI v1、adapter/provider schema 外层和提示名称可以避免新增 parser 请求字段和调用步骤，但不消除既有版本选择约束。具体来源见调查记录。

具体障碍有三项：已有 v2 state-dir、已配置/冻结的 workflow/1、固定的旧发行包。新构建不能就地接管旧运行，旧 profile 也不能提交 workflow/2。最小后续接入方案是在既有发行/安装和 profile 选择机制中固定新 Runner 版本、对新运行选择 workflow/2 和新的空 Runner state-dir，旧运行由原包在原目录收尾并保留历史；不需要新增 parser 配置字段、UI 或调用步骤。鉴于本轮明确 SteerLine 不改、不增加其配置义务，本期以独立 Runner 的新目录/新请求为实现与验收边界，上述接入仅登记为以后单独确认的部署事项，不作为本任务要求 SteerLine 执行的步骤。不能声称原 SteerLine 现场无调整即可接入，也不能为此建设用户禁止的旧格式兼容、自动映射或迁移。

## 9. 最小验收

| ID | 自动通过条件 |
| --- | --- |
| RA-A01 | 契约独立于报告 Turn；三方式及选定后备可用；成功输出的定义/注入/识别为统一能力，文案归各阶段；有效命中或完整机器报告零获取模型调用，未命中沿原后备；普通执行/发布无统一 JSON 收尾要求 |
| RA-A02 | 原生/业务/获取三类语义、字段来源/缺失/歧义/冲突及原输出完整；解析 done 不覆盖原失败/未知/provider 事实，补报不产生实现/检查/发布输入 |
| RA-A03 | 两种流程逐步骤策略、真实审计修复来源/数量/逐项内容，无自动返修，反馈失效、PR 独立绑定/身份核验/里程碑保持；下游不依赖伪报告 Command/Turn |
| RA-A04 | Runner 本地配置/冻结/惰性资格/独立模型与原生认证引用；无 parser 的策略不被环境缺失阻断；不新增 SteerLine 请求义务，公开 v1 与版本拒绝可验证 |
| RA-A05 | 原意图/原子提交/CAS、未知零重发、重启零自动输入、恢复仅获取、预算不重置、迟到无推进；Run/服务停止及 verify-stopped 包含所有 parser owned 资源 |
| RA-A06 | 首个真实 Codex parser 在固定版本/profile 下完成结构输出、原生默认认证及指定已有 home 引用、独立模型/会话和真实停止；按 §6 分别验证原生命令只读权限、上下文收敛、行为约定及工具事后拒收，零凭据复制；不要求空工具表或零隐式上下文，不满足原生门槛时如实拒用 |
| RA-A08 | 固定控制终端身份在业务输入前随原资源准备操作提交；执行期强杀仅对账原终端/原请求与完整原生 final，零新建辅助终端/零重发业务；撤销与关闭证明分开，未核实不结算；初始化未知零业务输入且阻塞，无第二状态权威，SteerLine/Orca 不改 |
| RA-A07 | 两个内置定义共用运行机制，业务后继/提示/契约/模型/预算由定义冻结，无共享运行路径的 Workflow 特判；Workflow.StepID 为唯一业务游标，外层/status/phase 投影和待发送意图同次一致提交。首期无定制脚本端口/宿主/DSL/多层引擎；改名的测试定义复用同一能力，流程文档与定义一致 |

RA-A01 的零模型机器报告是能力与选择该策略的定义的验收；development 用阶段声明实现零模型获取。覆盖末行匹配/原来源反例、缺 PR/原失败及冻结/恢复/停止幂等。RA-A03/A07 对 development 补充：外层 implement 与内部 execute/report/input 分开、无 D-PARSE 或独立解析转移，所有路径 parser 调用/探测为 0；无法判断直接 report，每源报告最多一次，无格式修正或报告重试；已明确未成功或报告未成功/已结算的获取失败均 input，零自动实施输入；只有明确用户输入及原会话资格满足才继续。development-audited 的实施/发布同样无 parse、无格式修正；仅代码审计修复结果获取可 parse，不补造修复或完成结论。每原审计最多一次 parse 和一次 report，获取失败已结算才可后备；核验 fixed_count/列表/遗留情况及 §3.1 提交前后绑定、干净工作区和 PR.HeadRevision 一致性。无 Spec 审计、A-REPAIR、两次自动返修计数或再审轮；未完成不发布。

测试级别、命令、环境、时点、证据及各条覆盖见 Plan；独立 parser 不通过 RA-A06 不得宣称三方式均已支持。程序/agent 分支不依赖其未验环境。实际验收状态在专属登记维护，无人工验收后置项替代必需自动项。
