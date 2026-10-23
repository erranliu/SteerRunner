# Plan：Workflow 能力解耦与结构化结果获取

2026-10-21 状态核对：三种获取方式与两种开发定义已实施，实际构建资格及历史失败见[专属验收](../acceptance/result-acquisition.md)。下文保留实施顺序与阶段记录；当前 state/v4、兼容换代及开发 worktree 文档路径规则按[换代 Spec](service-generation-recovery.spec.md)和[产品契约](steerrunner.spec.md)，不恢复旧文档快照门槛。

> 2026-10-19 主线接入：rebase 到 `a3dc15d` 后保留原生签名退出和固定控制边界，按 §6 审计并重验。旧批次源码/失败仍保留；最终记录见[本轮验收](../acceptance/evidence/rebase-control-20261019/README.md)。

> 2026-10-19 增量已完成：按 Spec §7.1 实施固定控制终端，执行期不再创建辅助查询/撤销终端；初始化创建身份未知继续阻塞。同源完整 L1 366 Go/29 Node、Controlled 16、三项原生用例通过；四项管理员变体未执行。下述 §5 及[本增量验收](../acceptance/evidence/fixed-control-20261019/README.md)维护依赖、实际命令与证据，历史失败不改写。SteerLine、Orca 不改，不部署。

> 2026-10-19 此前准备修复批次：修复原生准备中的匿名配额依赖和有来源的工具初始化失败诊断，并修正原生负例对已关闭 client 的后续调用。按依赖执行实际 sandbox 正反例 → 完整 L1 360 Go/29 Node → Controlled 16 → 三终端 L3 → 两项 L2（复用同源前置、真机并行上限 2），均通过。源码身份、环境、命令、时点、通过条件、停止及证据索引见[该轮记录](../acceptance/evidence/native-preparation-20261019/README.md)。当时原 terminal.create 强杀窗口的原生恢复保证仍缺；本轮执行期方案及初始化阻塞边界按 §5，不扩大为新的状态/恢复平台，不宣称完整 L3 通过。

> 实施状态（2026-10-18）：已按确认的最小边界实现并完成必要验证。原生权限、行为约定和事后检测见总 Spec §6；当前支持 Windows / Codex >= 0.159.2，默认关闭独立解析。实际支持范围、失败修正及最终证据见[验收登记](../acceptance/result-acquisition.md)。历史“仅设计”说明指此前设计轮次。
日期：2026-10-18（首次调查 2026-10-16）。按用户授权实施；本轮不部署、不创建 roadmap 调度。[Spec](result-acquisition.spec.md) · [验收登记](../acceptance/result-acquisition.md)。

首期按用户要求收敛为两个内置 Workflow、三种获取方式、一个结果模型及必要来源/恢复接线；不实现定制脚本端口/宿主、DSL、插件、多层引擎或通用快照/字段编排平台。具体定义先以[普通开发](../workflows/development.md)、[带审计开发](../workflows/development-audited.md)和[统一能力](../workflows/README.md)交付审阅；实施遵循已确认定义；独立 parser 的支持声明仍受 P0 门槛限制。

## 当前实施进度

| 步骤 | 2026-10-18 状态（证据见验收登记） |
| --- | --- |
| P0 | 2026-10-17 强隔离未通过的历史保留；本轮最小 profile 通过真实解析、原生权限反例、认证引用/显式模型和停止验证 |
| P0-D / P1 / P2 / P3 | 两份内置定义、契约、程序提取、一次原 agent 报告、配置/版本和提交级来源已接线，程序/报告分支及审计 parser 接线均已验证，原生能力证据另见 P0/P4/P6 |
| P4 | 独立 exec owner、配置资格读取、冻结输入/Schema、Job/有界日志/恢复回执已建设；无宽权限替代 |
| P5 | 原 Record/Intents/CAS、单槽、来源核验、重启/显式恢复/停止与晚到结果已接线；真实受控重启已发现并修复原执行资源恢复和补报权限冻结问题 |
| P6 | T1–T8 测试与专属真实 driver 已建设；L1 247、完整 Controlled 13、最终源码定向恢复/工具后备 2 及真实内部 3 cases 通过；完整入口 native 迭代失败保留，环境修正后用新现场复验 |

本表记录执行状态，不删减以下完整目标和自动验收项。最终通过项必须同时有非零测试匹配数及对应场景证据；受控夹具不代替 T7 真实原生权限/认证/模型验证。

## 1. 前置确认（P0）

2026-10-18 用户确认按 Spec §6.1 替代原 P0 强隔离门槛。本轮先实现一个受限 Codex exec owner：空工作目录、冻结 stdin/Schema、原生 read-only/never、关闭不需要的外部能力、保留默认原生认证；不以空 ToolPolicy 或零隐式上下文作为支持条件。全部工具不可见/全部上下文排除不再是本期验收条件。原生权限、行为约定、事后检测须分别取证；观察到工具事件不证明提前拦截。无已知业务副作用且确认原树停止后才可后备。

本轮依赖顺序：P0 只读固定接口/最小 profile → P1 冻结 parser 请求及原意图数据 → P4 owned Job/有界 JSONL/结果 journal → P3 审计 parse 状态接线 → P5 单槽/恢复/停止/CAS → P6 定向、Controlled、真实最小三 cases及证据。正常 program/agent 路径不探测 parser；不增加 SteerLine 配置或调用。

当前定义只解析原审计 final，不自主读取 artifact；首期缺少必要材料则走一次原 agent report，不建设检索平台。仍保留原始来源、pre_head/post_head 与发布核验。parser 操作直接保存在原 Intents；attempt 只引用 operation_id，启动/完成/恢复事实不另存可调度账本。

T4/T5 增加受控 native 子进程：正确/坏结构/超限/无 final/非零退出/未知或实际工具 item、timeout/停止后后备；原 executor 保持空闲；意图提交失败零启动、结果提交失败只读补提交、服务重启零 stdin、旧 owner/进程未核实停止占槽、双 Run 无 parser 不受影响、stop/晚到完成不推进。沿既有各组命令，在真实模型前执行，证据归本轮 E/t4.log、t5.log、process-facts.json、fault-windows.json。

T7 改为三个最小真实 cases：①默认认证/原生默认模型的一次正确审计文本整理；②现账号有权显式模型、污染材料/工具诱导及只读写入反例、短预算取消和 owned 树停止；③既有 development 真实业务 smoke（仍零 parser）。使用 Runner 实际 owner，不以裸 CLI/helper 代替；真实 parser 可通过当前 owner 的专属测试驱动，不另建外部状态服务。入口必须先 L1/完整 Controlled，命令保持 local-codex.result-acquisition；每 case 独立身份/来源/调用数及停止证据，①②不创建 PR，③仅一次测试 PR。验证本节收敛后的承诺，不继续要求空工具注册表或全宿主读隔离矩阵。业务审计/解析后备组合由 T2/T4/T5/T6 受控覆盖。

固定原生 EXE/version/SHA、原生 profile 和共同生命周期引用。新 P0 验收如下：

| 控制 | 最小证据与处理 |
| --- | --- |
| 原生接口 | 0.159.2 的 exec stdin/schema/JSONL/final、公共 initialize/config/read 无 thread/turn；只保存连接名与来源判定，不保存完整配置 |
| 外部能力 | 逐项禁用有效 MCP 名称、关闭 apps/plugins/hooks/shell/search/子 agent 等；非用户层动作 hooks 或无法核实/禁用的连接拒用；不要求空工具注册表 |
| 原生权限 | Windows read-only/mxc/never；测试专用一次 shell 开启反例验证 canary 写入被原生拒绝，生产 shell 仍关闭；命令沙箱不冒充 MCP 全局边界 |
| 认证和模型 | 默认已有原生 home，以及同一已有 home 的非秘密环境名引用；原生默认及一个实测可用显式模型。无复制凭据/新账号；显式模型不回退。exec 不给实际模型名则记不可得 |
| 资源 | 全部资格子进程及解析主进程沿现 managed Job；主进程身份在恢复首条指令前提交；deadline/cancel/退出后核实树停止。未核实则保留 unknown/占槽，不后备 |

只读接口存在不等于这些条件已通过；真实结果逐项写入本轮 E。原强隔离调查见 2026-10-17 历史记录，不再作为本轮硬门槛。

## 2. 按依赖实施

### P0-D：固定统一动作与两份定义（先于 P1/P3）

先按 Spec §2.1 的责任表固定窄接口，再从 `route.go`、`profile.go`、`prompt.go`、`input.go`、`feedback.go` 迁出两个具体定义的步骤/提示/结果契约/后继。定义使用普通 Go 值、内嵌资源和只读事实的纯条件/核验函数；只有统一 actor 通过原生命周期派发并 Commit。机制只区分能力种类，不按 workflow ID、状态名或成功文案选择业务。新增只复用现有能力的定义不应修改原生可靠性/持久化/停止路径；验证用改名测试定义，不建设第三个产品 Workflow。

固定 Spec §2.3 的单游标映射：Workflow.StepID 保存完整业务内部状态，外层图从定义映射；Record.Status/Phase 仍是 actor 的兼容投影，NextStage 只承载原会话的真实提示输入，parse/程序核验/等待不创建 NextStage；获取尝试引用同一 Record 的实际 parser 操作，StageInputs/Results 是历史。逐个列出当前按 StepID/Status/Phase/NextStage 决策的路径，将业务选择收回定义、可靠性判断留在 ReliableExecution；删去多份当前步骤值或从“最后 Turn”推断当前阶段的分支。游标、源引用、已接受结果、待发送意图和投影同次提交；配置/提示冻结沿原 Run，不引入子 Run、并行区域、任意嵌套栈或新的状态服务。

区分原生问答与已结束 Turn 的业务 needs_input：前者业务游标留 execute 并投原问题，后者在审计 input 等待、不能回复已结束请求，也不自动重发审计。只有明确的新实施授权才走人工反馈、建新源/pre_head；不预建审计续轮/部分统计合并。T2/T6/T8 验证两类问题关联、仅补材料/resume 的业务输入计数为 0，以及明确新实施不继承旧获取资格。

development 本轮先落实整体 implement → publish → pr_ready，内部 implement 为 execute/report/input，不注册独立解析状态/提示词。execute 命中成功声明且核验通过直接完成；无法判断直接 report，原结果已明确未成功或报告未成功统一 input；每源 report 最多一次，格式无效或不足且操作已结算也 input，无格式修正或报告重试。外层/内部进度、声明及结果在原 Run 原子提交，进入提示词只发一次；input 保留原事实，只有明确输入/原会话资格满足才回 execute，技术 resume 不默认实施。development-audited 的实施/发布采用同样的成功声明及单次 report，不选 parser；仅 audit.code 的结果获取保留独立 parse。真实审计仍由原 agent 完成，程序完整结构可直接核验，其他材料经 parse 或一次审计补报取得候选；移除 Spec 审计状态、提示和发布前置，audit.code 改为同一次正式执行内审计并修复 P0–P2、完成必要验证并报告数量/内容；删除自动返回 implement、A-REPAIR、RepairCount 和两次上限。未完成保留真实等待/阻断，获取不能补做业务。

只接当前定义所需内置核验函数，删除定制脚本端口/配置/schema/宿主/权限探针/支持矩阵的实施与验收任务；不存在“脚本不支持”的新运行状态。既有 provider/Git 命令仍属原 owner，不能变成任意脚本接口。

来源前置按 Spec §3.1 固定提交级方案：复用 WorkspaceInspection 与固定 Git owner 的 HEAD/Dirty/身份事实，补稳定读取前后 HEAD、pre_head→post_head 的祖先核验（可相等），正数修复须 Git tree 有差异；识别首期不覆盖的 submodule/gitlink、稀疏/skip-worktree/assume-unchanged 情形并保留未核实，不自动转换工作区或清索引，不建设脏工作区快照/额外版本库。补齐审计版实施及审计正式输入的 commit 要求，Dirty/未提交不能被 report 修复。这些检查仅限制完成采纳/后继，不能阻止失败原文、业务未完成/未提交和部分统计保存；原生失败沿原恢复门槛，明确业务未完成沿定义等待/阻断，不生成 commit 输入。核实现 Codex 冻结权限能执行这些已有 Git 工作，不把改提示当成权限授予；能力缺失如实阻断。当前 provider 已返回 HeadRevision，但通用 ValidateWorkerPRIdentity 不固定 SHA：新增的是审计版定义中的 PR.HeadRevision=post_head 条件，不扩大查询 API。

### P1：契约、配置和版本

涉及 `internal/execution/workflow/{types,result,profile,input}.go`、`contracts/{run,view}.go`、`resources/`、`runstate/record.go`、`storage/`：实现 Spec §2–§3 的结果/尝试/来源、固定完成谓词和字段校验；在 Run 冻结本地配置、内置策略、各阶段成功声明/契约及注入后的模型提示/schema 摘要。只有真实执行/审计/报告输入产生相应业务 StageInput/StageResult，识别成功声明不伪造新报告轮。Result/Attempt/SourceRef 直接嵌入原 Record；实现完整候选和已选后备，不增加通用字段路由/合并/投票器、运行定义加载器或额外配置。

新增 Runner 本地配置加载器及内嵌 schema，入口为 `service/result-acquisition.json`；未配置/关闭时无需 Codex parser 环境。接受顺序保持原 request_id 去重先于新的配置读取/冻结；配置错误只拒绝新的 submit，不能阻止已有 Run inspect/stop 和服务收尾。恢复不从当前配置重新填默认值。

统一发布 workflow/2、资源版本/摘要、state/v3/user_version=3；保持 CLI v1、既有 schema 外层 1、公开真实 final 和控制字段。旧 workflow/1、state/v2 明确拒绝，不建立迁移或原路由兼容。新 inspect 字段为获取投影，不将 parser 文字塞进 final。修改 source_report_command_id 等真实含义已变化的持久字段名称及测试样本。workflow/2 的审计项和提示覆盖 catalog 仅保留 audit.code；移除 audit.spec 资源/覆盖键和旧双审计 gate，不新增兼容路径或 SteerLine 配置义务。审计公开/持久结果使用 fixed_count/fixed_issues/remaining_issues 及修复前后来源，不再发布旧 verdict/findings 返修契约。T3/T8 验证新 catalog/契约、旧覆盖键明确拒绝及新默认提示，无需运行 Spec 审计；不保留旧修复计数控制。

### P2：程序提取及字段核验

新增窄 `internal/execution/resultacquisition/` 包放来源包、固定契约解码/核验和程序提取；Workflow 只声明契约/策略，publication 仍为 PR 核验 owner。禁止该包依赖 application/store/Task 或直接修改中心状态。

实现现有完整机器报告直接解码、声明的 artifact 快照读取/摘要/新鲜度、PR clues 及现 provider 查询；查询无唯一候选/出错/未知副作用继续原目标对账。统一结果保留字段定位、缺失、歧义、冲突；不自动扫全仓产物，不凭 native success 合成实施 done，不用 URL 存在合成 PR 发布成功。

同包实现通用 RecognizeSuccessOutput：当前可信 final 的末尾独立正文行与冻结声明逐字匹配，按统一规则处理行尾、围栏/引用/缩进代码，不读日志/旧轮、不做模糊匹配。返回命中与原来源定位，采纳仍受当前阶段契约/核验限制；命中且有效时统一写入程序来源结果并提交后继，原 final 不变。能力只依赖声明值，不出现 development 专用文案分支；覆盖提示时也统一注入声明要求并冻结最终正文，不额外发模型轮。

### P3：三种方式接线与当前 Workflow

在 `workflow/{route,prompt,input,profile,feedback}.go`、资源 prompts/development 和 `application/{results,freeze,worker,feedback,control}.go` 落实 Spec §4 逐步策略及已确认的两份状态定义。分开“接收原生终态”“接受统一结果”和“核验后提交转移”，具体后继/提示/模型/预算交由定义；共享状态执行机制复用原生可靠性与提交边界，不新增调度器或业务状态权威。

当前 finish 持有 actor 锁；新增 Git/provider 检查必须按冻结请求在锁外收集事实，使用原 worker context/managed 查询/期限，返回后再核源/授权/stop 并 CAS，不能直接把阻塞 I/O 加进已持锁的 finish。T2/T5/T6 用受控查询挂起信号验证 stop/新源能先提交、旧回执不能推进，沿原 query owner 收尾，不建设异步核验平台。

实施结束先识别成功输出，不足时最多一次实施报告；发布去掉 JSON 强制，程序取得候选并核验，不足时最多一次发布报告，两类均无 parse。真实审计输入直接审计、修复 P0–P2 并验证，报告实际修复数量和逐项内容；2026-10-21 新默认提示要求单一审计 JSON，旧冻结正文和显式覆盖保持，该文本要求不增加原生 schema 强约束。完整可信机器结构直接核验，否则材料适用且 parser 开启时最多一次独立解析，未选/不适用或已结算的解析失败最多一次原审计补报。全部无格式修正或报告重试，冲突/unknown 先对账，获取不重做源业务。source 绑定真正原实施/发布/审计，不是失败的上次报告。

上一段描述可选获取能力；development 以 P0-D 和更新定义为准：只选可信 final/程序提取和原 agent report。execute 无法判断直接报告，report 的契约解码/核验由程序完成，不属于独立 LLM 解析；所有 development 分支均不得启动/探测 parser。每个源实施、每次原发布分别最多一次 report，不追加格式修正或报告重试，恢复/补读不重置预算。成功声明缺发布 URL 等信息时程序提取及报告补足，不能补造，provider 核验保留；已结算但未成功/报告仍不足进入 input。审计版仅在代码审计获取阶段接线 parser，其后端、配置、恢复/停止和测试能力沿 P4–P6 实施；解析结果引用真实审计及所审实施源，不补造修复或完成结论，severity 不降级；fixed_count/列表/遗留问题必须有原依据且一致。

将审计绑定从 AuditReportCommandID 替换为当前实施统一 result ID 和原实施源，更新 AuditSource/AuditsPassed、审计输入正文和公开 audit view。真实审计修复 Command/Turn 保留；绑定原实施 result/Command/输入版本、实际审计 Command/Turn、pre_head 和 post_head/干净工作区检查；接受结果、派发发布前及 provider 回执时按 Spec §3.1 核验。PR HeadRevision 缺失或不是 post_head 不完成审计版发布；普通 development 的现有 PR 身份规则保持。本次已授权审计写入不创建新 implement Turn 或使自身失效；后续独立改动/新实施则失效旧审计。发布按 Spec §3.1 固定“本地核验→原 PR 查询→本地与当前源/授权/stop 再核验→CAS”的顺序；失败后只重核原目标，不重发 push/PR，PR 事实可独立保留。删除旧返修计数/调度及 verdict 分支，结果/来源/发布意图复用原子提交。移除所有“必须先存在 StepReport 的 Turn”隐含门槛。真实 StageInput/意图冻结本次所需结果 ID 和原业务源版本，发布不能挑最近成功审计，报告的新版本不能替换原源或重置预算；进入 parse 清掉已消费 NextStage，解析引用原 Intents，资源结算后后备才创建报告输入，同 Run 至多一个可派发业务/parser 意图。

审计版接收实施终态时记录绑定该实施 result 的干净 pre_head，实际派发审计前必须重读匹配，不能以入口的新 HEAD 重新绑定旧结果。接收审计终态先将原 final 及已取得的 Git 检查（含真实 Dirty 状态，合格时才产生 post_head）存入同一 Record，再按原策略解析/补报；非 done 问题/部分统计不以干净 checkpoint 为前置，done 采纳/发布仍须完整检查；Git 检查关联本次业务 Command/Turn/工作区，不新建检查服务、签名凭据或 Git journal。若缺原 Record 中的事实或既有完整操作证据（仅 final journal 不够），恢复保留 unverified，自报 SHA/reflog/当前 HEAD 不得补成旧检查；已有 journal 也不拥有游标/调度权。已有合格 checkpoint 时，审计版实施 report 前后核对原 pre_head、审计 parse/report 前后核对原 post_head；没有时仅整理非 done 信息，候选 done 也不得补成合格 checkpoint，新 HEAD 即使干净也不重绑。原审计 Turn 结束即可获取，不要求原 app-server 退出，保持原会话空闲供单次补报；有已知未决写操作仍先对账。在恢复分支先处理未决获取，再考虑原有人工实施反馈；`resume` 获取不生成 implement.execute。等待预算、次数及 deadline 在接受意图时冻结，不因等待限额/重启重置。P3 的独立 parser owner 可先用受控协议夹具，真实后端依赖 P0、P4。

### P4：独立 Codex parser owner

按新的 P0 边界，在 `execution/resultacquisition/codexexec/` 下新增 Codex exec owner，复用 `adapters/codex` 的 native EXE 解析、可信原生来源读取及 `execution/process` 的 managed Job、原子文件/有界日志接口，不能直接套业务 LifecycleSnapshot 的当前 session/turn。

启动前提交原操作意图、获取/配置/包/schema 摘要、绝对 deadline 和资源 generation；用独立 native EXE/会话/cwd、stdin 输入及固定参数。读取 JSONL 原生完成与指定输出，关联同一进程/原生会话/本次包；临时文件先完整写入并校验摘要，再在唯一 Record 中引用。文件存在或被进程替换不代表结果被接受。非 JSON、重复对象、无 final、超大输出、工具事件、非零退出均按获取语义处理。

环境只保留 Codex 原生认证引用和必需宿主环境，不传 GitHub worker broker、provider 密钥或执行工具目录。Codex 负责认证和续期；Runner 没有 token 读取/复制模块。显式模型不回退；主机限额固定为一个活动 parser；等待复用原 Run 未发送意图/deadline，不另建持久队列、资源池或公平调度器。解析原生日志不写普通公开事件，不导出秘密。

### P5：恢复、停止和原子边界

涉及 `application/{worker,control,engine,verify_stopped,results}.go`（Shutdown 在 engine.go）、`runstate/`、`storage/`、`host/{shutdown,verify_stopped}.go`、`execution/reliable/operation/` 和 process owner。

扩展原意图与 Record 的资源角色，使现有单写原子提交、CAS 和恢复规则能区分 executor/parser；不是为 parser 新建 Run/中心库。分别保存未发送、可能已发送、完整 journal 待提交、已接受结果等事实，不能用启动失败擦掉部分进程身份。

parser 的单活动限额从原 Run/Intents 的启动/停止事实恢复；部分启动、可能已发送或未核实退出的旧资源仍占槽，不能因新服务内存为空就再启动。复用既有原资源对账/停止，不新建全局资源账本或调度队列，也不阻断无 parser 的 Run；当前未启动等待项的 deadline/后备按其自己的原事实判断。

修正 stop 的“无执行 External 即 stopped”捷径、worker 退出及服务最终结算枚举；包含 parser 及晚到资源、资源句柄、原 deadline 和停止未核实事实。verify-stopped 只读核验原进程/Job 绑定，不自动重开解析。重启对账只读、零自动输入；显式 resume 后只执行未发送意图/允许后备。旧/重复/冲突结果不能推进新源、已停止 Run 或新服务实例。增加游标/意图/公开投影一致性校验：矛盾记录停在原对账门槛，不选一个字段继续。验证 CAS 前源已换、原生/获取 journal 已完整但 Commit 失败只补提交、状态投影刷新不再入状态动作。pre_head/post_head 不因恢复重冻；HEAD/Dirty/分支变化或未决写操作不重发审计/修复/commit。

### P6：文档、定向测试与验收登记

先完成下节新测试/driver 建设，再执行命令。同步 README、docs/testing.md、两份 Workflow 文档/能力入口、公开 schema/示例、能力支持与限制、Spec 中“建议”到实际实现结论，以及本任务 acceptance；不改写首版/WA/SA/L3 历史通过记录。核对状态、边、核验和提示词与同版本内嵌定义一致，文档不被另行加载执行。共同生命周期链接只在本仓说明 W14/W15/W16/W18 差异，SteerLine 不改。

登记最终源 commit/dirty、构建 EXE SHA/version/resource digest、native EXE/version/SHA、策略/契约/配置摘要、原 Run/请求/结果/操作/资源身份、场景数/调用次数和证据位置。产品项失败/unsupported/未验不能记为通过。没有部署步骤或跨仓测试调度。

## 3. 最小测试计划

环境固定 Windows/amd64、PowerShell 7、Go（按 go.mod）、本仓源码；L1/Controlled 使用临时目录、隔离 Git、可控 native/parser/provider 夹具，零真实模型/外部 PR。全部命令从仓库根运行。下列入口现已按依赖建设；本轮新增 parser 断言须用本轮代码重新执行，不沿用历史通过数。已有通用入口 `scripts/test.ps1` 保持其前置纪律。

证据前缀 `docs/acceptance/evidence/result-acquisition-<日期>/`（以下简称 E）；测试原始现场在 `.evidence/`，归档脱敏后在 acceptance 引用 E 下的 summary/日志。每组需记录命令、退出码、非零匹配数/子场景数、失败与 skip；0 测试或意外 skip 一律失败。

| 组 / 自动项 | 级别、场景与环境 | 命令（建设后） | 时点 / 通过条件 / 证据 |
| --- | --- | --- | --- |
| T1 / A01,A02 | L1：成功声明末行精确匹配、两阶段文案隔离、CRLF/末尾空白行；省略/措辞变化/正文/引用/代码/日志/旧轮/截断反例；命中但原失败/未知/缺必要结果；有效命中及完整机器报告零模型，未命中原后备；业务 outcome 仅三值、缺失不写 unknown；审计 fixed_count/去重列表/遗留语义，正数但 Git tree 未变不通过，缺失不能填零，已修复不当遗留；其余契约/来源/预算回归 | `go test ./internal/execution/workflow ./internal/execution/resultacquisition ./internal/execution/reliable ./internal/execution/workspace -run '^TestRA(Contract\|Selection\|Sources\|Semantics\|Budget\|SuccessOutput\|WorkspaceSource)' -count=1 -timeout=2m -v` | P1–P3 后；冻结声明精确识别，零 false success，原 final 保留；有效命中 parse/report/probe 计数为 0，原失败/未知/provider 事实不被覆盖；`E/t1.log`、`E/case-counts.json` |
| T2 / A03 | L1：两种流程完整顺序、实际 audit 输入、无报告 Turn 的实施源、当前审计修复完成/来源、fixed_count/逐项内容/遗留 P0–P2/P3，pre/post 提交/Dirty/PR.HeadRevision、查询期间源/HEAD/stop 变化、新实施/反馈失效；publish 程序成功/补报、错误 PR/base/head/多候选/query fail、先绑定 PR 后流程完成 | `go test ./internal/execution/workflow ./internal/application ./internal/execution/publication -run '^TestRA(Workflow\|Audit\|Feedback\|Publication)' -count=1 -timeout=3m -v` | P2–P3 后；只允许更新定义的后继，审计直接修复且无返修/再审输入，无 Spec 审计输入，独立绑定不提前 pr_ready，补报零发布；`E/t2.log` |
| T3 / A04 | L1：本地配置缺失/关闭/有效/非法、默认值、request_id 原回执先于配置变化、Run 冻结、EXE/认证/模型惰性；不同模型配置、无权限显式失败；CLI v1/新增审计修复统计及 pre_head/post_head 来源可读、旧返修控制已移除、state/workflow 旧版明确拒绝、capabilities/resource identity | `go test ./internal/contracts ./internal/resources ./internal/application ./internal/storage ./internal/cli -run '^TestRA(Config\|Freeze\|PublicContract\|Version)' -count=1 -timeout=3m -v` | P1/P3 后；关闭或未选 parser 时启动/探测计数均 0，配置错不妨碍停止；caller fixture 模拟已调查 v1 解码和 catalog 校验，无 SteerLine import；`E/t3.log`、`E/public-fixtures.json` |
| T4 / A02,A04,A05 | L1/组件：受控 parser JSONL 完整/坏结构/超限/缺 final/工具事件/非零退出；允许包内重转义大文本的原回执恢复和超 2 MiB 日志拒收；原审计单次解析及一次补报预算，无格式修正/再审，原 executor 空闲存续可补报；解析明确环境失败后补报；timeout 后确认停止才后备；unknown 禁后备；stderr 脱敏/认证引用 | `go test ./internal/execution/resultacquisition ./internal/execution/resultacquisition/codexexec ./internal/execution/adapters/codex ./internal/execution/process ./internal/application -run '^TestRA(Parser\|Fallback\|Process\|Redaction)' -count=1 -timeout=4m -v` | P3–P5 后；真实 helper 进程树归属/回收，保持 raw executor final 不变；不调用 token/凭据读取；`E/t4.log`、`E/process-facts.json` |
| T5 / A05 | L1/存储和宿主：意图提交失败零启动，外部成功/聚合提交失败只补提交、CAS/重复/冲突、启动部分回执、原未知零重发；restart/resume 只获取；stop 与 late start/result 并发、无 executor resource 但有 parser、service shutdown/verify-stopped、游标与意图一致/审计提交来源恢复、并发上限/deadline 不重置 | `go test ./internal/storage ./internal/application ./internal/host -run '^TestRA(Atomic\|Recovery\|Stop\|Shutdown\|Late\|Concurrency)' -count=1 -timeout=4m -v` | P5 后；检查原数据库记录和所有启动/发送/业务输入/停止计数，parser 残留则不结算；`E/t5.log`、`E/fault-windows.json` |
| T6 / A01–A05,A07 | Controlled：真实 EXE/CLI/IPC/SQLite；新增 `TestSRResultAcquisition`，下述固定 cases 与两份定义轨迹；继续执行既有完整受控前置 | `./scripts/test.ps1 -Level Controlled -OutDir .evidence/ra-controlled` | P0-D、P1–P6 后、真实模型前；当前 catalog 的全部场景（本轮 16 项，含新增结果获取）全部非零完成，公开状态/操作与调用审计一致；`.evidence/ra-controlled/summary.json` + 原现场，归档 `E/controlled/` |
| T7 / A06（原生条款关联 A01,A02,A04,A05） | L2：新增 `local-codex.result-acquisition` 专属 driver；固定本机 Codex，已有原生登录，真实模型/专属 SteerlineTest PR；下述三 cases | `./scripts/test.ps1 -Level L2 -Scenario local-codex.result-acquisition -Config .evidence/ra-live-config.json -OutDir .evidence/ra-live` | P0 所需探针及所有 L1/Controlled 通过后；入口自动先 L1 和完整 Controlled。固定预期 case 数=3、零意外 skip、实际来源/工具边界/业务 PR 及收尾齐全；归档 `E/native/` 和原现场 |
| T8 / A07 | L1：两个定义与改名测试定义共用动作机制，提示/模型/预算冻结；单游标映射、原生问答不移动业务状态、待发送意图/投影/来源同次一致；NextStage 仅用于提示，parser 不另存操作权威，前置结果 ID/原源版本固定，报告版本不重置预算；无脚本配置/执行入口，无业务名特判；文档与内嵌定义一致 | `go test ./internal/execution/workflow ./internal/execution/resultacquisition ./internal/application -run '^TestRA(Definition\|Transition\|Verification\|StateAuthority)' -count=1 -timeout=3m -v` | P0-D/P3 后、T6 前；原路径的观察/恢复/查询不重发状态动作，矛盾记录不推进；改名测试定义不修改核心 dispatch。`E/t8.log`、`E/definition-traces.json`、`E/state-authority.json` |

上述 Axx 均指 RA-Axx。TestRA*、Controlled 与新增真实 driver 已建设；命令是否通过以实际非零结果登记，不以存在入口代替执行证据。定向单测用于迭代；T7 按仓库原有入口规则运行 L1/Controlled 是必要前置，不意味着额外默认要求全 L3、三轮稳定性或多平台矩阵。若最终未改动的原生边界有可复用固定证据，引用实际相同版本/配置，不重复历史资格矩阵；新增 parser 边界必须实测。

### 3.1 Controlled 固定覆盖（T6）

在现 `tests/catalog.json` 的 controlled_scenarios 注册 `TestSRResultAcquisition`，扩展现宿主，不增加另一套测试调度器。最小 cases：

1. 零获取模型路径：两种 Workflow 实施成功行有效且核验通过，无 implement.report Turn 消费统一结果，普通/审计后继正确；真实审计已有完整可信机器结构时直接核验，parse/report 均为 0。另用能力夹具验证完整机器报告直接提取；均无 parser probe/start。
2. 审计修复 parser 短包：真实审计输入授权直接修复 P0–P2 并验证，parser/executor 进程/会话分离，原 final 不变。报告至少一项实际修复、数量/逐项内容/修复后绑定核验通过再发布；明确 0/空列表且完成无遗留亦可。数量不一致、重复问题、只报数量缺内容、未知被填零、未完成验证或仍遗留 P0–P2 均不发布；已修复项不继续阻断，P3 不计数且不单独阻断；明确未完成且未提交的部分统计仍保留，不强制先获得合格 checkpoint 才展示/等待。无 Spec 审计或自动 implement/修复/再审输入，解析不补造修复或降低 severity。
3. 审计 parser 结构不合法/材料缺失或环境明确不可用且已结算 → 原审计上下文一次 report；parser 关闭直接一次 report。报告仍无效/不足则阻断，无格式修正或再审；原审计真实 needs_input/blocked 保留。unknown、已知业务副作用或停止未核实不自动后备；本地 command/todo 违规在确认停止且原源仍适用后可单次补报；MCP/文件修改/未知工具事件保留副作用未核实，零补报且技术 resume 不得绕过。仅覆盖这些选定路径，不排列全部方式组合。
4. 审计大包/不可导出必要 artifact 直接一次审计 report；两份定义的实施/发布均不调用/探测 parser，即使 enabled 但环境不存在。发布程序查询成功零模型，定位不足一次 publish.report；所有补报的调用审计证明没有新增实现、测试、真正审查或发布输入。
5. unknown/冲突/原执行失败/成功缺原 final：零获取解锁、零发布重发；错误/多 PR 与已经 merged 但未到 pr_ready 的外部事实保持现规则。
6. 故障窗口：意图/回执/结果/下一输入提交边界、重启待发送/可能已发送/完整 journal 三状态、显式 resume、stop/服务停机/迟到解析资源、verify-stopped。每个故障使用新的 state-dir，核验原 ID 和计数。

上述测试断言公开 inspect/events/final/controls、原 Run 原请求、数据库 revision 和 native/provider 调用审计，不只检查策略函数返回值。

扩展同一 Controlled 宿主覆盖 RA-A07：两个真实定义记录统一能力/进入提示/核验/后继轨迹；检查从唯一 Workflow.StepID 投影外层、Status/Phase/NextStage 与原源一致，原生问答仍停原 execute。注入游标/意图错配、只更新投影、旧来源或 CAS 冲突，必须零新派发；完整 journal 在原源仍适用时只补提交，缺 checkpoint 但有模型 SHA/reflog 也不重建旧事实。parse/verify/input 不生成 NextStage 或假 Command，旧 native 输入和新 parser 意图不得同时可派发；即使 SHA 相同，新业务源也不能复用旧审计引用。不存在脚本动作/schema/宿主资格项，也不以测试夹具建立第三个可装载产品定义。沿 T6 原命令/环境/时点，证据 `E/controlled/` 和 `E/controlled/state-authority.json`。

提交级审计来源由 T1/T2/T5/T6 定向覆盖：临时 Git 将原实施 result 绑定干净 pre_head，进入前再次匹配，真实执行夹具修改并 commit 得 post_head，解析/补报不写工作区；固定修复数量/原报告与 post_head，PR 身份正确但 HeadRevision 错误/缺失仍不完成。补充零修复 pre=post、入口/出口脏、HEAD 非后代、分支/工作区错配、读取中 HEAD 变化，以及原实施已绑后/实施补报期间/审计派发前、审计 report 期间、发布前发生独立提交。预期均无自动清理/commit/修复/发布重发；旧审计失效且真实 PR 事实仍保留。T5/T6 再覆盖 pre_head/post_head/结果/发布意图的提交失败与重启、只有原 final journal 或模型 SHA 而无原 Git 检查事实时不采纳当前 HEAD、完整 journal 只补提交、停止/迟到解析，日志归 `E/t1.log`、`E/t2.log`、`E/fault-windows.json` 和 `E/controlled/`，再覆盖正数修复但同一/空提交的 tree 未变、Dirty 被索引标志隐藏或 gitlink/稀疏情形未覆盖（拒绝而非自动修复），以及原 app-server 空闲存续时获取和补报仍可完成。provider 查询期间注入 HEAD/源/stop 变化，CAS 不得提交 pr_ready，已知 PR 事实可留存且 native 发布计数不增加；已观测补报写入即使最终 HEAD 相同也不采纳。T5/T6 增加双 Run 受控反例：重启时原 parser 可能存活但停止未核实，新 parser 启动计数为 0、无解析流程仍可执行，原槽只在原资源事实结算后释放；不增加真机资源矩阵。上述反例沿 T1/T2/T4/T5/T6/T8 原命令、环境、时点、证据位置，无需额外真机矩阵。

T2/T6/T8 同步 development 的外层/内部图及提示词：内部只含 execute/report/input，成功声明有效时直接完成，无法判断直接 report，已知未成功可直接 input，报告未成功或一次报告格式无效/不足且已结算也 input；验证 implement/publish 对各自原源 report 最多一次，格式错误不追加修正输入，重启/resume/补读不重置次数；不要求具体问题或精确原因。所有路径 parser 调用/探测计数为 0，分别在 parser 关闭、启用但环境不存在的夹具下核实，与选择独立解析的其他定义隔离。publish 即使命中也取 URL 并查询核验；外层不返工。T3/T5/T6 继续验证提示只发一次、声明冻结/原子提交、重复读取/重启补读/stop 后不派发、原 outcome/获取缺口及用户输入/unknown 门槛。测试命令/环境/时点/证据沿对应组。development-audited 复用上述实施/发布规则，独立解析与 parser → agent 后备只在真实代码审计结果获取中覆盖，不重新接回非审计阶段。T2/T6/T8 增验审计和执行模型回切、独立解析模型冻结；一次报告无效不再输入、重启/resume 不重置两类获取预算、审计合法写入的修复前后工作区关联、发布禁止沿用旧快照、后续独立改动/新实施使旧审计失效且迟到解析不能污染当前源，以及成功行不能代替修复数量/内容/完成依据。测试级别/环境/命令/时点/通过证据沿本节对应组，不新增真机矩阵。

### 3.2 真实本机最小三 cases（T7）

当前使用主线独立配置入口 `tests/test-config.example.json` 的配置/现场构造支持（原扁平 live-config 示例已删除，不恢复旧格式）；用户本地 `.evidence/ra-live-config.json` 使用现已有 Codex 登录和实际原生 EXE，以及既有 SteerlineTest 发布配置，不含凭据值。不增 SteerLine checkout/EXE/数据库依赖。

- `parser-default`：通过实际 owner 对短审计材料夹具整理契约，不指定认证 home 或模型。记录冻结包、原生 session/JSONL/final、退出/归属/回执；实际模型名未报告则记不可得，不由 catalog 或模型自述推定。原生默认认证只在此次调用成功后算通过。不创建 PR。
- `parser-boundaries`：非秘密环境名引用同一已有 home，显式选择候选模型，以真实成功证明可用性，不复制凭据或回退模型。污染材料诱导写 canary，核验仅接受正常整理或明确工具违规拒收、canary 不变。另一次测试专用 shell 开启反例，在同一 read-only/mxc/never 下要求实际尝试写入并遭拒；生产 shell 继续关闭，反例不作为获取成功。真实 turn.started 后取消另一解析，核实 owned 树停止。原生 Code Mode 运行时可按模型要求保留，实际工具事件仍事后拒收；不以自然未调用工具证明空工具面或零隐式上下文。
- `workflow-smoke`：复用 `development-smoke` 的真实实施、PR/合入/merged 核验与停止，不执行发布后反馈更新 PR。冻结成功声明、真实 final 和程序来源；原 final 已满足程序获取（完整机器结果、有效精确成功声明或足够的发布线索并经核验）时 report=0，parser 调用/资格探测始终为 0，发布真实查询。真实模型若遗漏或改写成功行，按原设计仅允许一次同源 report；不能把合法后备本身判为失败，也不能放宽成功行匹配、掩盖额外调用或覆盖原 final。测试须读取冻结原文/来源，区分这两条实际路径。一次报告无效/不足后的等待/阻断由 Controlled 确定覆盖，不强制真模型遗漏成功行。

一个注册 L2 case `default` 内执行三项内部 case；catalog planned/executed=1 与内部 case_count=3 分开登记。解析原生 exec 调用三次（默认、显式、取消），权限反例一次，不把 exec 次数当 HTTP 请求数。原生 usage 有报告才记录 token；不建立额外模型/主机矩阵。前两 case 不创建 PR，第三只一个测试 PR。测试专用 owner 驱动不伪造业务 Turn；原聚合/恢复/后备组合由 T2/T4/T5/T6 确定覆盖。

T7 首轮入口先 L1/完整 Controlled。仅修正 owner 原生环境/资格且不再改 actor/Workflow 时，先通过变更组件回归，再用 new-test-fixture.ps1 / test-native.ps1 的新现场重跑全部三内部 case；沿用已通过的前置及最后 actor 修正后的 L1/定向 Controlled，记录代码差异与构建身份，不重复无关全量。失败保留原现场；修复用新场景，不改预期或用 helper 替代真实权限/认证/模型证据。若固定原生版本、只读 profile、外部连接关闭、有效结构或资源停止不可核实，则拒用该次 parser，不扩大权限。取消空工具与零隐式上下文门槛不取消这些条件。远端执行认证缺口沿原 SA07，本功能不增加 Orca 真机或新增登录义务。

## 4. 文档交付与发布限制

此前设计轮只写 Spec、Plan、专属验收、调查记录、两份具体 Workflow 确认稿/共用能力入口和 README 导航，并做文档静态检查；其只读 CLI 探针不是产品验收。本轮用户已授权实施和必要验证，实际非 parser 构建、服务、Controlled 与真实模型/测试 PR 结果见实施记录；不安装或部署产品，不修改 SteerLine。

实施后每批结果填入 acceptance，附实际执行命令、证据、原生缺口处理和支持范围。后续 state/v3 与 workflow/2 的旧目录/旧包障碍按 Spec §8.1 处理；本 Plan 不含 SteerLine 改动或安装更新授权。若产品隔离能力受阻，报告准确阻断，不将未完成的 RA-A06 标成已交付。

## 5. 固定控制终端增量实施与验收

本节覆盖 Spec §7.1 / RA-A08。按依赖实施：①固定两操作 helper 协议，验证既有原生 terminal send 的请求关联、丢响应重读/去重及 helper 标准输入能力；②在 `orcatransport` 的原 CreateResource 准备链中先创建、核验并提交控制终端，再启动 agent，更新内部 implementation codec，旧版明确拒绝；③将 `remote_receipt`、`authentication_stop` 改为同一已绑定控制终端，复用现有 caller-owned 辅助检查点恢复传输请求，删除逐次 inspector/revoker 创建及其重复资源状态；④接入原恢复、Run stop/service shutdown/verify-stopped，区分业务写者、认证撤销、控制资源关闭证明；⑤更新 native audit/现有 driver 的来源核验、文档及验收。不得为测试豁免任意终端或扩大 agent 计数规则。

控制身份以原 Record 的适配器 implementation 为准；检查点只保留同一资源下未决请求与回执，严格核对父绑定，不驱动工作流。初始化创建未知不重新创建、不按标题/空 inventory 结算；服务恢复不发送任何业务输入。辅助查询传输恢复不消耗 agent/report/parser 调用预算。关闭证明与撤销证明分开核验，旧失败不改写。

环境为 Windows/amd64、当前 Go/PowerShell、固定 Codex/Orca/helper SHA，现有配对执行主机及隔离 Run/worktree/PR；不改原生应用。证据目录 `docs/acceptance/evidence/fixed-control-20261019/`（F），原始现场在 `.evidence/fixed-control-20261019/`。不得收录凭据。所有命令、非零匹配数、构建/配置/资源身份、失败迭代、最终业务与资源结算分别登记。

| 级别/场景 | 命令与执行时点 | 通过条件与证据 |
| --- | --- | --- |
| L1/组件：有限方法、异源/坏载荷拒收、原回执不变、请求 ID 冲突、丢响应恢复、初始化未知/提交失败零业务输入、控制身份变化阻塞、重复撤销、停止/关闭失败及服务释放观察 | 实施后 `go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/testkit/livetest -count=1 -timeout=5m -v`；随后完整 `./scripts/test.ps1 -Level L1 -OutDir .evidence/fixed-control-20261019/l1`（也由下述 L3 入口先执行） | 新增控制协议/生命周期反例与既有受影响测试全部通过，无临时 inspector/revoker 创建；`F/checks/cleanup-budget-regression.log`、`F/prerequisites/summary.json`、`F/prerequisites/l1.jsonl` |
| Controlled：原 EXE/IPC/SQLite 的工作流与恢复接线 | L1 后 `./scripts/test.ps1 -Level Controlled -OutDir .evidence/fixed-control-20261019/controlled`（也由 L3 入口先执行）；最终同源冻结 EXE 的完整重跑命令见本节末 | 完整现有 16 场景通过，零真实模型；`F/prerequisites/controlled/` |
| L3：执行期强杀恢复、正常重启、停止及跨重启撤销；原生完整回执和控制资源归属 | 前置通过后用同源构建运行 `./scripts/test.ps1 -Level L3 -Scenario orca-remote.service-restart,orca-remote.stop-revocation -SkipAdminRequired -Config <冻结配置> -OutDir .evidence/fixed-control-20261019/native -MaxParallel 2`；实际运行 graceful-active、kill-active、active 三项，明确排除四项管理员网络/阶段边界用例，不另造“成功”预期 | 原控制终端在首条业务输入前已提交；kill-active 恢复仍为同一 handle/incarnation，原请求/final 可核验、零重发输入/零新建辅助终端；stop 证明原认证 broker/socket 清理、业务写者及控制终端关闭，无晚到写入，原 Run/service 结算完整；`F/native/`、`F/cleanup-summary.json` |

上述入口保留注册 case 集并明确记录管理员变体排除，不将子集标为完整场景通过；不扩大为全 L2/L3 或管理员网络矩阵。辅助资源创建前置、初始化未知与身份变化用确定性 L1 覆盖；真实强杀只验证用户确认的执行期窗口。测试发现问题直接修复并对新源码重跑相关前置/场景，未经验证的分支保持明确未验。

验收时点增量：固定控制终端在业务完成时仍应存活，关闭审计在 Run stop/cleanup 后执行，必须同时保存原认证撤销、完整 inventory 与控制关闭证明，不能把“仍归属本 Run”计成“已关闭”。原生批次若仅因旧 driver 的提前关闭断言失败，保留原失败；改正 driver 后用新现场重新运行，不将事后复核改写成原用例通过。仅测试代码/文档变化时，逐文件证明生产源码与已通过完整 L1/Controlled 的源码完全相同，记录 helper 原构建 SHA/源提交与新 driver 提交；可复用该同源生产前置，再执行审计包回归、完整 native driver 编译/零匹配保护/注册检查和受影响真实 cases。生产源码有变化则重新执行原前置。

停止补充验收：覆盖挂起只读请求、调用方 stop 上下文取消后原撤销的独立 45 秒清理预算；撤销未核实必须保留同一控制终端、原业务写者仍停止，后续 business-confirmed 对账只补读原请求并关闭原控制终端，不重关业务终端。测试计数允许已验证身份/有限载荷的控制请求暂未完成而不误算为业务输入，但最终资源审计必须拒绝任何未决控制回执。此项为生产改动，重新冻结 helper、完整 L1/Controlled 后再执行三项真实用例，不沿用此前生产前置通过结论。

最终执行记录：clean 源码 `212ebee3abeae798c05fe53ede19f8b2a5e0f0f3`、helper SHA `afb08225…df7235`。标准 L3 入口先完整 L1 366 Go/29 Node 通过，Controlled 的 15 分钟总预算耗尽，原批次失败且未启动 native cases；遗留服务仅经原实例公共 stop 清理并核实结算。随后 `run-controlled-25.ps1` 校验并复用同一组 EXE 的源提交/SHA，将 harness 总预算设为 25 分钟，单场景/产品预算及断言不变，16/16 无跳过通过（实际约 15 分 51 秒）。`run-final-native.ps1` 严格校验同源 L1/完整 Controlled 前置后，调用原 scheduler/driver 在新现场运行三项原生用例，3/3 通过，四项管理员变体明确排除。脚本、原超时/失败记录、实际配置/构建及每项原生证明见 `F/checks/`、`F/prerequisites/`、`F/historical/` 和 `F/native/`；汇总及精确命令以 `F/README.md` 为准。未复跑无关全量 L2 或管理员网络矩阵。

推送后主线已到 `a3dc15d`，原生退出/只读查询进程结算证明与本分支存在五处源码冲突；本批次未变基，PR 保持 Draft。后续集成必须保留原 Codex owner/退出公钥、查询进程 writers-absent、原控制身份及撤销/关闭证明，不能恢复临时 reader 创建或以 PTY/inventory 代替新原生退出证明。组合实现须重新冻结 helper/源码并验证受影响路径，本批次通过不自动继承。只读预检与差异见 `F/checks/integration-status.json`、`F/checks/main-merge-preview.txt`。

## 6. 接入主线原生退出证明

1. rebase 到最新主线，逐处保留 `remote_agent` 的 owner/公钥/Job/签名退出、`remote_turn/history` 的 reader Job 清理、原主机 artifact/inventory 和只读 verify-stopped；冲突中的旧逐次 reader 资源接线改接固定控制，不保留失效字段或第二套恢复机制。
2. `remote_control` 在查询失败时保留资源回执；`control_checkpoint` 在替换查询回执前核验原 reader 已退出及完整父绑定。未知 reader 不得被后续查询/撤销覆盖；不新增协议方法或持久权威。
3. `StopLifecycle/reconcileStop/verifyStopped/ShutdownLifecycle` 组合认证撤销、原签名退出、业务 PTY 和控制关闭证明。签名缺失不得提前销毁原 helper，认证未核实仍沿既有边界停止原业务树、保留控制对账；被动服务继续执行规则不变。
4. 迁移主线单元/原生测试的资源夹具到真实固定控制准备，不构造旧 inspection 字段。测试同源 helper/Runner，真实原生 cleanup 归档并验签退出正文；零业务停止探针在 helper 前先准备控制，离线 probe 禁止服务启动、模型/业务输入和 DB/原失败回执修改。
5. 审计后冻结源码与 helper，执行下表；归档最终与失败结果，更新 Spec/验收及 PR，再给出是否适合合入的结论。SteerLine/Orca 不改，不部署/合并。

证据目录 `docs/acceptance/evidence/rebase-control-20261019/`（R），原现场 `.evidence/rebase-control-20261019/`。Windows/amd64、现有 Codex/Orca/认证、原有配对主机及隔离 SteerlineTest worktrees；无需新账号或管理员网络设施。审计补齐主线新增 native stop opt-in 的允许跳过登记；正式入口的 L1/Controlled 整套预算按实测设为12/25分钟，单场景/产品期限、断言和计数门槛不变。最终由新冻结源码的正式入口整体重跑，早期超时/夹具失败均保留，不用组合报告替代最终通过。

| 级别/场景 | 环境、命令与时点 | 通过条件/证据 |
| --- | --- | --- |
| 组件 | 改后适配器/协议/审计包定向回归，application/host 的 `TestSROffline|TestSRShutdown|TestSRStopProgress|TestSRCLIContract` 定向；整包由下述完整 L1 的12分钟预算覆盖 | 原签名改写/重放/缺认证/活树/原主机等负例、失败查询但 reader 已退出/未知 reader 禁止覆盖、认证/控制/只读及现工作流回归通过；`R/checks/component-*.log`、`R/checks/stop-consumer.log`。早期扩大至 application 全包的5分钟总超时保留，不算通过 |
| 完整 L1 | 冻结源码后 `./scripts/test.ps1 -Level L1 -OutDir .evidence/rebase-control-20261019/l1` | 全部本层测试通过，允许的 opt-in/helper 跳过逐项登记；native compile/零匹配守卫/框架通过；`R/prerequisites/l1.jsonl` / `summary.json` |
| 完整 Controlled16 | L1 后，以 `tests/catalog.json` 全部受控场景 pattern 经正式入口构建/运行 integration harness，总预算25分钟、保持原构建/独立进程启动/断言/单场景预算 | 16/16 顶层及8条获取子路径无跳过通过，EXE/SHA/源码一致；`R/prerequisites/controlled/` |
| 原生 workflow 3 cases | 前置通过后，原 `Invoke-SRCaseBatch` 入口执行 `orca-remote.service-restart` 的 graceful-active/kill-active、`orca-remote.stop-revocation` 的 active，冻结配置，最大并行2，明确排除4管理员变体 | 原控制在业务前就绪、同身份恢复、原 complete final/reader 退出、零业务重发；原签名退出/认证/控制关闭/完整清单/服务结算均通过；`R/native/` |
| 既有无业务停止探针 | 前置通过后本机与已配对远端各一次 `go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout=8m`；显式 `STEERRUNNER_ORCA_STOP_PROBE_*` 固定新隔离目录、同源 EXE/helper/native/model/repo/base/原 host；与其他真机合计并行不超2 | 活动拒绝、原 helper 真实签名退出后公开 EXE 离线 verify 成功，原 DB/失败回执不变、不启动服务、零业务输入、原控制及自有 worktree 清理；`R/probes/`。Reader 真实结算由前一行实际业务查询覆盖，不再创建旧一次性 reader 终端 |

RA-A08 与主线 OSV-A06/A08/A09 的本轮交叉覆盖沿上述有限场景维护。没有全量 L2/L3 或新增庞大矩阵；若生产改动发生在冻结后，重新冻结并重跑受影响前置，历史通过不改写为当前通过。

主线追加接入（2026-10-19，`2aa17cc`）：保留独立配置的公开 validate/capability、配置冻结与 case 可用性边界；测试基线与 manifest 更新为 workflow/2，结果获取 case 补齐同一注册需求，删除已废弃的反馈资格参数。配置/CLI/manifest 组件与框架全量检查须通过，再由最终冻结源码重新执行本节正式 L1/Controlled/native 入口。`5861ded` 批次完整 L1 已通过，Controlled 因主线推进主动取消，未启动 native；保留中断记录，不能算最终验收。

测试接线补充：最终产品批次 bb84ea8 已完整通过 L1/Controlled/三项 native workflow。随后补充停止探针在 helper 启动前因把内部 ID 当 Orca UUID 重试身份而失败，删除首次发送中无原 UUID 的 retry 标志，不重发未知请求，在新现场重跑两项探针。另发现测试报告 MarshalIndent 会改写嵌入 RawMessage 的签名正文；证据改为原字节 base64 保存，新增实际签名经过缩进归档仍可验签/改写必拒绝的回归。原报告保留，从原 runtime/worktree artifact 重读原始退出字节并另存、验签，不重签或从格式化 JSON 猜测原文。上述只改测试/证据；以 checks/code-equivalence.json 证明生产、集成及脚本源码与 bb84ea8 相同，复用其冻结 EXE/模型批次，重跑 host/livetest、native driver 编译/零匹配守卫及本机/远端停止探针，不重跑无变化的真实业务模型。失败原记录、实际产品/测试提交和原始字节来源分别归档。

最终组合执行：以 `bb84ea80cab2e37c2c7d184fcd82765dde2b1d32` 为冻结源码、主线 `2aa17cce7da6fd75e59db76e18f2521d9e9777db`，正式入口整体通过完整 L1 385 Go/29 Node、Controlled16、三项所选原生 workflow；其后本机/远端停止探针各一次通过，五份原签名退出逐份验签。原始失败/主动中断及配置冻结、原控制身份/reader 结算/清单/服务证据均归档到 R；以 [R/README](../acceptance/evidence/rebase-control-20261019/README.md) 的命令及范围为准。产品批次与随后测试取证修正的关系见 code-equivalence.json；生产源码未变，实际产品/测试提交分别登记。
