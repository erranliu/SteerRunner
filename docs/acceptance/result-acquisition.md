# Workflow 能力解耦与结构化结果获取验收

> 2026-10-19 固定控制终端最终验收：同源完整 L1 **366 Go/29 Node**、完整 Controlled **16/16**、正常重启/执行期强杀恢复/活动执行撤销 **3/3** 通过。三份原生记录均核实首条业务输入前的控制身份、原回执/来源、认证撤销及实际关闭，最终服务均 not_running/resources_settled=true。初始化创建身份未知继续阻塞；未新增状态权威，SteerLine/Orca 未修改。内部 Orca codec 5→6、旧版拒绝，无兼容迁移。四项管理员变体未执行，不宣称完整 L3；失败迭代及同一冻结 EXE 的 Controlled 25 分钟总预算重跑见[本增量证据](evidence/fixed-control-20261019/README.md)。

> 推送后的集成障碍：新主线 `a3dc15d` 的原生退出/只读查询进程结算证明与本分支冲突，尚未变基或验证组合后的代码。**当前不适合直接合入，PR 保持 Draft**；本轮通过仅绑定212ebee。具体冲突及必须保留的停止证明见[集成状态](evidence/fixed-control-20261019/checks/integration-status.json)。

> 2026-10-19 此前准备修复批次（固定控制终端实施前）：移除无关的匿名 GitHub 配额门槛，补充绑定原 owner 的工具初始化失败事实。同源完整 L1 360 Go/29 Node、Controlled 16、三终端 L3 及两项 L2 全部通过，原 Run/服务已清理。见[修复与原始证据](evidence/native-preparation-20261019/README.md)。当时准备阻塞已解除、强杀创建身份缺口仍缺原生保证，源 PR 保持 Draft，不宣称全量 L3 已通过。

> 此前 rebase 与影响范围复验：已接入主线 `59c7597`，完整 L1 358 Go / 28 Node、Controlled 16、两项 L2 通过；当时最终三项 Orca L3 在原生准备阶段失败，强杀恢复缺口未闭环。**该批次暂不建议合入，PR 保持 Draft**。见[该轮质量与证据](evidence/rebase-audit-20261018/README.md)。历史批次不替代最终构建验收。

> PR 前接入最新主线后的质量审查、修正及验证范围见[质量记录](evidence/pr-quality-audit-20261018/README.md)。原生历史结果不自动继承为本次构建通过。
> 2026-10-18 后续完整范围复验：L2 2/2 通过；L3 4 项通过、1 失败、7 项环境阻塞，未全通过。见[独立记录](evidence/full-l2-l3-20261018/README.md)。该记录维护后续修正及未通过项，不替换下述历史验收。


> 2026-10-18：用户确认最小独立 parser 方案，三方式与原聚合/资源接线已实现，必要验证已通过。L1 247、完整 Controlled 13、最后修正后的定向进程恢复/后备 2 及真实内部 3 cases 的批次关系见证据；原失败未改写。[本轮证据](evidence/result-acquisition-20261018/README.md)。

[Spec](../spec/result-acquisition.spec.md) · [Plan](../spec/result-acquisition.plan.md) · [原调查](evidence/result-acquisition-design-20261016.md)。无 roadmap 调度、无部署、无 SteerLine 修改。2026-10-17 强隔离阻断的历史结论保留，不倒改成当时已支持。

## 自动项登记

固定控制终端新增自动项：

| ID | 验收边界 | 最终覆盖与证据 | 状态 |
| --- | --- | --- | --- |
| RA-A08 | 控制身份在首条业务输入前随原准备操作提交；执行期仅恢复原终端/原请求，零重发业务/零新建辅助终端；原生 final、撤销与关闭分别核验；初始化未知零输入并阻塞；无第二权威 | Plan §5；最终源码212ebee完整 L1/Controlled、三项真实 native cases、逐 ledger/服务状态复核；[证据](evidence/fixed-control-20261019/README.md) | 通过（确认边界内，四项管理员变体未执行） |

下表保留初次最小 parser 实施批次的验收结论；后续 rebase、完整范围及最终构建的复验以顶部独立记录为准，不代表当前已满足合入门槛。

| ID | 验收边界 | 覆盖与证据 | 初次实施批次状态 |
| --- | --- | --- | --- |
| RA-A01 | 三方式、阶段成功声明、完整机器结果零额外模型；development 无 parser，每源单次补报；审计按需一次 parse/一次 report | T1/T4/T6/T7；原 final 和真实输入计数 | 通过（本期边界） |
| RA-A02 | 原生/业务/获取分离、缺失/歧义/冲突保留；模型不覆盖原失败/未知/外部事实；补报不补做业务 | T1/T4/T6/T7；获取来源、原文、工具拒收/停止 | 通过（本期边界） |
| RA-A03 | 同轮代码审计修复、数量/内容/遗留一致；实施 pre_head、审计 post_head、PR.HeadRevision、当前源/CAS；无自动返修 | T1/T2/T5/T6/T8 与 T7 发布闭环 | 通过（本期边界） |
| RA-A04 | Runner 本地配置/默认关闭/冻结/惰性资格；原生认证引用与独立模型；CLI v1、state/v3、workflow/2，不增加 SteerLine 义务 | T3/T4/T6/T7；配置/身份/版本证据 | 通过（本期边界） |
| RA-A05 | 原 Intents/Commit/CAS；未知零重发、重启零输入、显式恢复只获取、单槽/截止时间/停止/晚到结果 | T4/T5/T6/T7；真实受控重启及 owned Job 停止 | 通过（本期边界） |
| RA-A06 | 按 Spec §6 验证真实认证/结构/独立模型与会话、只读权限反例和资源停止；工具事后拒收不冒充前置隔离 | P0 原生接口/源码、T7 三内部 cases；不要求空工具表或零隐式上下文 | 通过（Spec §6 最小边界） |
| RA-A07 | 两内置定义共用机制；StepID 唯一业务游标，投影/意图/来源同次提交；parser attempt 引用原操作，无第二队列/状态服务 | T5/T6/T8；state-authority 与 definition-traces | 通过（本期边界） |

T1–T8 的环境、命令、执行时点、通过条件及证据位置在 Plan §3 维护。受控测试不替代原生认证/权限/模型验证，历史通过数不计为当前通过数，无人工后置项替代自动验收。

## 原生支持与限制

仅 Windows / Codex 0.159.2，默认关闭，真正选择 parser 才发现和资格核验。默认认证环境与显式已有 home 的非秘密引用交给 Codex 原生处理；不复制凭据、不新增登录、不提取 token。显式模型不回退；原生默认实际模型名未报告时记不可得，真实成功仅证明本次账号可用，不推定其他模型有权。

原生 read-only/mxc/never 限制命令动作，关闭 shell、外部连接、apps/plugins/hooks 等；公共 config/read 仅读取连接名/来源，不保存配置值。受模型要求保留 Code Mode 运行时。提示词只整理材料属于行为约定；exec JSONL 不是完整原生调用审计，检测与拒收只针对其实际暴露的 item，不承诺零工具调用、零隐式上下文或工具执行前拦截。原源/Git/provider 规则、未知副作用与停止核验继续优先。

parser 原意图、资源 generation、deadline、已绑定进程和回执在原 Record/Intents；schema/final/JSONL/journal 是原操作证据，不能独立授权推进。服务重启零自动模型输入；显式恢复先恢复原空闲执行资源，原补报权限从已冻结解析包读取，避免恢复时误用缺失的运行时对象。

## 实施记录

| 批次 | 范围/结果 | 证据 |
| --- | --- | --- |
| 2026-10-17 | 基线 f211e316、dirty；dev.1。定向 32、L1 237、Controlled 13、真实 smoke 通过；强隔离 P0 阻断，parser 当时未实现 | [历史记录](evidence/result-acquisition-20261017/README.md) |
| 2026-10-18 | dev.2；最小 parser 和生命周期实现；RA-A01–A07 在本期边界内通过，迭代失败、修正和代码差异逐项登记 | [本轮记录](evidence/result-acquisition-20261018/README.md) |

源码/构建身份、实际场景和调用计数、原生 EXE/SHA、认证路径引用、PR/停止与验收摘要归入本轮证据。state/v3、workflow/2 的旧目录/固定旧发行包限制仍按 Spec §8.1，不在本轮部署或修改 SteerLine。

## 本轮文档检查

2026-10-16：完成源码、共同生命周期和官方 Codex 文档调查，新增本任务文档及 README 导航。仅运行 Codex version/help/features/schema 只读探针；没有模型调用、凭据读取、服务启动、测试 PR 或实现修改。

文档静态检查通过：复用现有 Markdown 链接检查逻辑并在内存中将扫描根设为本仓（未修改 SteerLine 文件），43 份 Markdown、219 个本地导航链接、0 错误；本轮 5 份文件的 JSON 样例解析、RA-A01–A06 登记/测试映射、尾空白/冲突标记检查及 `git diff --check` 通过。调查记录中的原生 EXE/协议 schema 摘要已与实际文件核对。产品自动项均保持未实施、待验。

后续同日设计补充：用户确认状态机通用能力/具体定义分离，并将定制代码/脚本限定为结果核验。新增[统一能力](../workflows/README.md)、[development 定义](../workflows/development.md)、[development-audited 定义](../workflows/development-audited.md)确认稿，更新 Spec/Plan 范围和 RA-A07/T8；未修改实现或执行产品测试。新增文档静态检查另行登记，不把前一轮检查数字当作本轮结果。

本次补充静态检查通过：46 份 Markdown / 244 个本地导航链接 / 0 错误；7 份本轮新增/更新文档的尾空白、冲突标记与 JSON 样例检查通过，RA-A01–A07 在 Spec/验收均有登记。普通开发 11 个状态、6 个提示词，审计开发 16 个状态、9 个提示词，状态图/转移表与提示词引用逐项匹配；`git diff --check` 通过。该结果仅证明文档内部静态一致，不证明流程实现或原生能力通过。

随后逐步核对 development：用户确认整体图首阶段为 implement，execute/parse/report/input 属内部；执行后先分析，解析失败补报；有效解析/补报确认未成功统一 input，不要求明确问题或失败原因，不自动回 execute。development 已改为整体图 3 个节点、implement 内部 4 个状态，6 个提示词引用齐全，明确等待说明可来自 workflow_policy，原 outcome/获取缺口不改写，原未知操作门槛不放宽。Spec/Plan 同步该选择；publish 细节与审计稿仍待后续核对，上一段平铺状态检查仅保留为历史。更新后静态复验：46 份 Markdown / 244 个本地导航链接 / 0 错误，development 层级/提示词引用、相关文档尾空白/冲突标记/JSON 样例与 `git diff --check` 通过。未实施、未运行产品测试。


2026-10-16 状态图补充：按用户确认删除 development 的 pr_ready → implement 回边，并同步删除表格/恢复说明中的同义转移。整体图为 implement → publish → pr_ready；implement、publish、pr_ready 分别增加内部状态图，pr_ready 仅表示稳定里程碑及原 PR 事实刷新。implement 内 input → execute 仍保留；publish 规则与审计稿仍待后续核对，未更改六段提示词正文。同步统一能力/Spec/Plan，仅文档调整。静态检查通过：4 个 Mermaid 源图的预期节点/边及注释/围栏平衡、4 份关联文档的 20 个本地链接、尾空白/冲突标记和 git diff --check；未运行产品测试，未宣称图形渲染或功能实现已验收。

2026-10-16 提示词逐项核对：简化 development 的 D-IMPLEMENT，只保留实施/必要验证、材料、本次用户补充三行，删除不 push/不创建 PR、已有 PR 和输出格式说明。已有 PR 注入源于当前源码的 pr_ready 后反馈续做（workflow/feedback.go、input.go），不带入本 Workflow 已确认的单向阶段定义；发布相关提示词仍留待后续核对。本轮只更新文档，检查仅覆盖提示词/本地链接/围栏与尾空白，不执行产品测试。

2026-10-16 PR 范围清理：按用户要求，两份流程的实施/修复/发布提示词全部移除先前 PR 参数和复用分支，审计稿同步删除 pr_ready 后返回实施的边，Plan 真机 smoke 不再要求发布后反馈更新先前 PR。本次发布产生的 PR 的原操作对账、身份核验和事实刷新仍保留。用户提出的“状态成功输出可跳过解析/报告”机制正在核对方案，尚未修改状态图、提示词或实现。

2026-10-16 成功输出方案已获用户确认：将定义、注入及末行确定性识别归入统一能力，文案由 Workflow 阶段声明。先同步 development：implement 为“实施及必要验证已完成。”，publish 为“本次发布已完成。”；两段执行提示词及内部图增加成功路径，命中且必要结果/原执行/核验满足时跳过 parse/report，无关 parser 不探测，未命中保留原后备。PR URL/身份事实仍程序提取及 provider 核验，声明不覆盖失败/未知/冲突。Spec、Plan 与 RA-A01 的 T1/T3/T5/T6/T7/T8 覆盖同步，审计文案暂不配置；未修改实现、未运行产品测试。文档静态复验结果另行登记。

本轮静态复验通过：46 份 Markdown / 246 个本地导航链接 / 0 错误；5 份更新文档的围栏、JSON 样例、尾空白/冲突标记检查通过。development 保留 4 个外层/内部 Mermaid 源图，确认 implement 成功直达出口及未命中 parse/report 后备、publish 成功仍经过 verify；两段阶段声明与注入后提示词逐字一致，结构化 report/parse 未追加成功行，RA-A01–A07 登记完整。`git diff --check` 通过；未宣称图形渲染、匹配器实现、原生能力或产品测试已通过。

2026-10-16 全部提示词精简：逐条整理 development 的 6 段及审计版的 9 段正文，每段保留当前任务、必要输入/输出和必要动作边界；去除重复 PR URL 要求、无需 JSON 的说明、流程次数/顺序及对 schema 字段的逐项复述。两段 development 成功声明原文保留；report/解析/修正不补做工作，审计不修改实现；审计结论的 pass/changes_requested 判断只适用于审计完成时。审计版仍不新增成功声明或改变待确认的流程策略。静态核对 15 个提示词及全部原输入占位、成功文案/位置要求、格式修正自包含性和发布 URL 单次要求均通过，每段 3–5 行；46 份 Markdown / 246 个本地导航链接 / 0 错误，围栏/尾空白与 `git diff --check` 通过。未修改实现代码、状态转移或核验规则，未运行产品测试。

2026-10-16 development 去掉独立解析：按用户确认，implement 内部仅 execute/report/input，删除 parse 状态及 D-PARSE 正文，现有 5 段提示词；可信 final 成功声明和核验通过直接完成，无法判断直接 report，已明确未成功直接 input，补报未成功或获取失败结算后也 input。补报最多一次纯格式修正，由程序解码/核验结构，不调用独立 LLM。所有 development 分支均不探测/启动 parser，不受其环境缺失影响。统一三种获取能力、Runner 独立解析配置及其他定义的解析选项保留；审计稿本轮不改。Spec/Plan 和 T2/T6/T8 同步状态、等待及零 parser 调用条件，T7 独立 parser cases 与 development smoke 分开。此前四状态/六提示词及解析后备的记录仅为历史；本轮仍只改文档，未实施、未运行产品测试。

本轮静态复验通过：46 份 Markdown / 246 个本地导航链接 / 0 错误；5 份更新文档的围栏、JSON 样例、尾空白和冲突标记检查通过。development 保留 4 个外层/内部 Mermaid 源图，implement 图无 parse，成功/补报/等待及明确输入续执行的边齐全；5 段提示词与两段成功声明一致，统一 ParseResult 保留，审计稿 9 段提示词未变。RA-A01–A07 登记齐全，`git diff --check` 通过；未运行图形渲染或产品测试。

2026-10-16 development 单次报告：按用户要求删除 implement 的 report 自环及 D-REPORT-CORRECT，现为 4 段提示词。每个源实施、每次原发布各最多一次 report，无格式修正或报告重试；无效/不足且相关操作已结算时 implement 进入 input、publish 保持阻断。重启、resume、补读不重置次数。同步统一能力说明、Spec、Plan 及 T2/T6/T8 覆盖，其他定义的可选格式修正能力未改；此前一次格式修正的记录仅为历史。本轮只更新文档，未实施、未运行产品测试。

本轮静态检查通过：46 份 Markdown / 246 个本地导航链接 / 0 错误；5 份文档的围栏、JSON 样例、尾空白/冲突标记及 `git diff --check` 通过。development 4 个 Mermaid 源图、4 段提示词，无 parse 或 report 重试边，两段成功声明保持一致，统一 ParseResult 保留。未运行图形渲染或产品测试。

2026-10-16 development-audited 同步精简：整体图只列 implement → audit.spec → audit.code → publish → pr_ready 及预算内返修边；各阶段另列内部图。实施/修复/发布采用成功输出和一次 report，不使用或探测独立 parser；去掉 implement.parse、报告格式修正及相应提示词。仅两项真实审计的结果获取保留 parse：完整可信机器结构直接核验，否则按 Runner 配置/材料条件最多解析一次，未选/不适用或已结算失败时最多一次原审计补报，仍无效阻断，无格式修正或重审。审计提示词不强制 JSON，不以完成声明判断 pass；原 verdict/findings/severity、双审计同源、P0–P2 阻断及两次修复保持。统一说明、Spec、Plan 和 T2/T4/T6/T8 同步；T7 解析使用审计材料夹具，不新增真审计模型矩阵。先前“审计稿待同步”及实施解析/格式修正记录只作历史。本轮只改文档，未实施、未运行产品测试。

本轮静态复验通过：46 份 Markdown / 246 个本地导航链接 / 0 错误；6 份文档的围栏、JSON 样例、尾空白/冲突标记及 `git diff --check` 通过。审计版 6 个 Mermaid 源图、9 段提示词，parse 仅出现在两项审计内部，无报告自环或格式修正提示；实施/修复/发布三段成功输出一致，外层返修及同源门槛在文档/Plan 中保留。RA-A01–A07 登记齐全；未运行图形渲染或产品测试。

2026-10-16 审计阶段更正：按用户明确要求，development-audited/2 仅代码审计，整体为 implement → audit.code → publish → pr_ready。删除 Spec 审计状态、内部图、A-AUDIT-SPEC 提示及代码审计的 Spec-pass 输入/前置；发布仅核验当前实施源的代码审计 pass，修复后重新代码审计，原两次修复上限保留。只有代码审计结果获取保留 parse 和必要时一次 report；实施/修复/发布仍不使用 parser。统一说明、Spec/Plan、公开覆盖键及 T2/T3/T6/T8 验收同步，workflow/2 不再提供 audit.spec 覆盖键，无旧路径兼容。旧源码调查/已完成历史记录描述双审计不表示新设计要求；本轮只改设计文档，不修改实现或 SteerLine，不执行产品测试。

本轮静态复验通过：46 份 Markdown / 246 个本地导航链接 / 0 错误；5 份文档的围栏、JSON 样例、尾空白/冲突标记及 `git diff --check` 通过。审计版 5 个 Mermaid 源图、8 段提示词，无 Spec 审计状态/提示/输入占位；parse 仅在代码审计内部，外层实施 → 代码审计 → 发布、预算内修复及阶段成功行一致。未运行图形渲染或产品测试。

2026-10-16 审计直接修复：按用户要求，audit.code 在一次正式执行内审计、直接修复 P0–P2 并完成必要验证，报告实际修复总数和逐项问题/修复内容；删除返回 implement 的自动返修边和 A-REPAIR，取消旧 RepairCount/两次上限/audit_repair_limit 与 verdict/findings 返修判定。结果契约为 outcome/summary、fixed_count、fixed_issues、remaining_issues；数量须与去重条目一致，明确 0/空列表与未知区分，未完成/遗留 P0–P2 不发布。审计修复来源关联进入实施源、真实审计轮和修复后工作区/变更证据，正常授权审计写入不伪造实施轮，后续独立变化失效旧资格。只有审计获取保留独立 parse/一次 report，二者都不补做修复/检查。统一说明、Spec/Plan 及 T1/T2/T3/T6/T8 同步，原生解析能力门槛和最小真机矩阵不扩大。此前两次修复/只审计不改代码的记录为历史；本轮仅更新文档，未实施或执行产品测试。

本轮静态复验通过：46 份 Markdown / 247 个本地导航链接 / 0 错误；5 份文档的围栏、JSON 样例、尾空白/冲突标记及 `git diff --check` 通过。审计版 5 个 Mermaid 源图、7 段提示词，无审计返回实施或独立修复提示；数量/逐项内容/遗留契约与修复后来源在 Spec/Plan 一致，parse 仍只在审计结果获取中。未运行图形渲染或产品测试。

2026-10-17 首期边界收敛：只做两个内置 Workflow、三种获取能力及同一结果/恢复接线。移除定制核验脚本端口、配置/schema、沙箱/宿主及测试矩阵，不建设 DSL/插件/任意层级引擎/字段编排/通用快照。Spec §2.1 明确机制与定义责任，§2.3 固定 Workflow.StepID 为唯一业务游标、Record/actor/Commit 为唯一提交权威，外层和 Status/Phase 为一致投影，NextStage 是对应动作载荷。Spec §3.1 固定提交级审计来源：审计版正式实施先 commit，进入干净 pre_head，审计修复验证后在同次 execute 提交 post_head；完整报告后才发布，核验当前 HEAD/Dirty/身份及 PR.HeadRevision=post_head，无事后默认重绑。忽略目录材料另按原来源引用，不建设文件快照；Git 只读核验不声称外部写者隔离。两份 Workflow/共用说明、Plan/验收同步，只有正式输入增加必要 commit 要求，解析/补报不能补做。此前脚本建设/资格条目只留历史，不属于当前自动项；本轮仍未实施、未运行产品测试。

本轮静态复验通过：46 份 Markdown / 252 个本地导航链接 / 0 错误；7 份文档的围栏、JSON 样例、尾空白/冲突标记及 `git diff --check` 通过。两份定义共 9 个 Mermaid 源图、11 段提示词，审计版正式实施/审计有提交要求，解析/报告保持纯整理；T1–T8、RA-A01–A07 与单游标/pre_head/post_head/PR SHA 覆盖一致。工作区改动仅 Markdown；未运行图形渲染、产品测试、原生模型、服务或部署。

2026-10-17 本轮主会话文档复审：直接修正以下问题，未启动子 agent 或产品执行。此前静态通过仅说明格式/引用，不代替这次语义复审。

| 问题 | 修正及覆盖 |
| --- | --- |
| NextStage 被泛化为所有状态的动作载荷，可能复制 parser 操作/恢复权威 | 仅承载真实原会话提示，程序/等待无空操作，parser 引用同一 Record/Intents；原结果 ID 和原业务源随输入冻结，T5/T6/T8 验证互斥派发/同 SHA 新源/报告版本不重置预算 |
| 业务 unknown 与既有三值 outcome 混淆 | 未取得结论用缺失及获取状态，不新增枚举；原生 unknown 保留，T1 覆盖 |
| 原 Git 检查缺失时“可核验定位”允许误信模型 SHA 或当前 reflog；“原写者结算”可能误要求先关原会话 | 检查事实须由 Git owner 产生并与原源存入 Record，只有既有完整证据可补提交，仅 final journal 不够；不新增 Git 日志系统。源 Turn 结束可保留 app-server 空闲，T4/T5/T6 覆盖 |
| Dirty=false 可掩盖首期未覆盖的索引/子仓库情形，正数修复也可能无实际 tree 变化 | 明确普通完整工作区边界，识别隐藏索引/gitlink/稀疏情况不自动修复；正数须 tree 不同但不声称独立审查，T1/T2/T6 覆盖 |
| PR 查询与本地/源变更之间的复核顺序不固定，新增检查还可能阻塞 finish 持锁区 | 固定本地检查→原 PR 查询→本地和源/授权/stop 再检查→CAS；I/O 在锁外，stop/新源能先提交，失败只核验原目标、PR 事实独立保存，T2/T5/T6 覆盖 |
| 补报前后的版本检查仅写 post_head，未明确实施补报的 pre_head | 审计版实施 report 固定原 pre_head、审计 parse/report 固定 post_head；已观测违规写入即使 HEAD 最终相同也不采纳，T1/T2/T4/T6 覆盖 |
| 单活动限额可能重启清零，等待队列/原生能力建设又隐含额外平台 | 从原资源事实恢复占槽，未核实退出不释放；等待仅原意图/deadline，无独立队列/资源池。原生隔离缺口如实 unsupported，不扩为 Codex fork/沙箱平台，T5/T6 覆盖 |
| 干净 checkpoint 被误作保存失败/部分结果的前置，已结束的业务 needs_input 又可能被当作原生轮内问答 | 完成/发布才要求合格来源；非 done 问题/部分统计照常保存整理，缺 checkpoint 不反向补造 done。原生问答留 execute，终态业务等待不投旧请求，也不自动重发审计；仅明确新实施授权创建新源，T1/T2/T6/T8 覆盖 |

Spec、Plan、两份 Workflow/统一说明及当前 RA 表已同步；新增反例沿原测试组/命令/环境/时点/证据，T1 补覆盖实际 Git owner 包，不新增真机矩阵。仍只修改设计文档，所有产品自动项保持未实施/待验。

另修正 Plan 命令表中正则的管道符 Markdown 转义，避免测试命令被渲染成额外表格列；显示命令语义不变。文档静态复验在本轮完成后登记。

本轮复验通过：46 份 Markdown / 252 个本地导航链接 / 0 错误；7 份文档 / 30 个表格列数一致，围栏、JSON 样例、命令正则及尾空白/冲突标记检查通过。两份定义共 9 个 Mermaid 源图、11 段提示词，代码审计输入/提问边与来源条件、单次补报、T1–T8 和 RA-A01–A07 映射一致；`git diff --check` 通过，全部工作区变更为 Markdown。未运行图形渲染、Go/Controlled/L2 产品测试、模型、服务或部署；原生解析隔离仍待 P0/T7，不由本轮文档复验替代。
