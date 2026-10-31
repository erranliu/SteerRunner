# Plan：验证链路解耦与前置结果复用

日期：2026-10-31。状态：P0–P5 完成，完整前置及正式复用 L2 通过，见[验收记录](../acceptance/validation-reuse.md)；§9 保留设计审计历史。以 [Spec](validation-reuse.spec.md) 的 VLR-01–06 和 A1–A8 为验收边界。调查基线 `5c5c1653961ad08a836e82fc84f0898667864a5c`。

## 1. 最小实施顺序

先从现有入口抽出阶段函数并保持默认行为，再生成阶段证明，最后开放显式复用。不先加入 skip 开关，也不先以手工 job 作为正式重跑接口。

| 步骤 | 主要落点 | 交付与退出条件 |
| --- | --- | --- |
| P0 固定现有义务 | `test.ps1`、`test-framework.ps1`、`test-foundation.ps1`、`test-native.ps1`；`tests/framework` | 盘点执行顺序、判定、产物和报告消费者，形成改造前后对应表；明确当前 25 个 Controlled/内部 case 不变 |
| P1 提取阶段 | 新 `scripts/test-prerequisites.ps1`、新共享 `scripts/test-prerequisite-contract.ps1`；原 `test.ps1` | 两个入口共用 L1-core/Controlled/preflight 实现；保留默认构建与调用方式，记录 Controlled 的实际 Runner |
| P2 生成与核验阶段证明 | 共享 contract、`tests/framework/prerequisites-test.ps1` | 新 manifest、输入/证据/产物摘要、精确覆盖核验；完成反例，默认路线能生成直接阶段证明 |
| P3 接入正式复用 | `test.ps1`、可选 scheduler job 字段、`run-test-case.ps1`、`test-native.ps1` | 仅显式 L2 单级别复用；新 preflight/新 fixture；实际运行同一 guarded driver；L3/L4 和旧 job 的原路径保持 |
| P4 回归与真实资格 | 原测试入口、新 framework、专用 L2 配置 | 默认完整前置 + 最终代码下的正式复用 L2；native-only 变更另用隔离合同验证；保留失败历史与清理记录 |
| P5 文档收口 | `docs/testing.md`、`docs/test-catalog.md`、验收记录、本文状态 | 记录可执行命令、来源和耗时；不改场景清单；未验目标和历史报告不升级 |

这些是同一轮实现的顺序，不要求拆成多个 PR。P1/P2 未通过默认链路检查前，不开放 P3。

## 2. P0：先锁定不得退化的行为

实现前记录下列映射，放入实施验收记录，供最终 diff 对照：

- L1：拒绝遗留 live opt-in；全部 framework guards；catalog 的 Go 包、非零测试、现有 skip allowlist；两个正式 Node 扩展零失败零 skip；live 编译、零匹配 guard。
- Controlled：当前 catalog 的精确顶层名单，原 foundation 单元检查、45 分钟聚合预算和原等待余量；各测试内部断言、循环与期限不变，尤其 ResultAcquisition 的 11 个内部 case。
- live：配置/能力/requirements、无可运行目标提前退出、冻结输入和固定产物；L2 的无模型 Git 边界检查、真实 worker 发布和命令/权限审计；原 scheduler/lease、结果核验及清理。
- 输入：确认 L1 的 `TestContentRepositoryPublishedSchema` 读取 `docs/workflows/generate-repository.example.json`，将它和控制文件枚举的 `.gitignore`/`.gitattributes` 纳入固定范围；不将整个 docs 当作无关输入。核对执行工具，尤其 Controlled 实际使用的 Git 与两种 PowerShell。
- 报告：查找所有读取 L1/Controlled 条目、source identity、summary、coverage、baseline.l2_verified、first_pass_rate 的消费者，标注必须保留的字段。现有 `test.ps1` 的 finally 只按 case coverage 设置 l2_verified；新增的最终复核失败路径必须先定闸门再设置资格布尔值，不能被 finally 覆盖。区分 delivery qualification，禁止将本轮 manifest 当交付包资格。

不以耗时长为理由删除测试，不以历史某次测试数量替代实际覆盖集合。当前脚本本就存在的窄范围 PreparedArtifactsDirectory 能力继续保留，但不能成为新完整前置的可信入口。

## 3. P1：只抽编排，不动测试内容

共享文件先提供三个具体阶段函数和共同读取/写入工具，不构造插件系统、通用任务 DAG 或独立 cache service。

1. 从 `test.ps1` 原样抽出 framework/Go/Node 和原判定，作为 l1-core；将 native 编译与零匹配检查抽成 preflight。原 L1 调用两者，保留既有结果字段。
2. 抽出完整 Controlled 的选名、启动、等待和原判定。新独立入口只接受 L1/Controlled/All，完整名单来自默认 catalog，不加入 Pattern 子集选项。
3. 默认路线保留既有 Runner 构建顺序、显式 Runner 和 foundation 参数行为；纯前置路线沿原 build.ps1 生成 Runner。证明绑定 Controlled 实际使用的 Runner 及其回执，不拿同批 live 的文件替代，不为生成证明重建受测产物。
4. 不改 `test-foundation.ps1` 的独立进程启动、账号核验、超时及 PreparedArtifactsDirectory 互斥语义；stage wrapper 等待真正结束后才提交成功。
5. 新前置入口输出阶段报告和 manifest，不输出声称完整 L2/L3 的 coverage；省略复用参数时 test.ps1 仍走所有原阶段。

用现有 framework 和小型编排替身核对执行顺序/早停边界。替身仅用于测试编排，不能进入正式验收配置，也不能成为默认的阶段证明来源。

## 4. P2：保守身份与直接阶段证明

按 Spec VLR-03 实现固定输入清单；不要为了命中把 `internal` 内的测试文件、资源或 Node 扩展从摘要范围排除。覆盖已确认的 docs JSON 外部输入。核对所有框架/fixture 读取路径，确认 `tests/native/**` 在前置中只负责独立 preflight，没有被 L1-core/Controlled 动态消费；若发现反例，先修正范围定义，不能照排除规则强行复用。不增加自动依赖发现引擎。

每个阶段独立采集开始/完成源码身份，不能只存批次最后的身份。产物在构建后、测试开始前采集摘要，测试结束后复核。Controlled 同时核对顶层 PASS 名单和原 result.json，不改 Go TestMain 计数口径；Go 的包通配符展开成实际包集合，JSON 核对所有预期包完成，Node TAP 和 framework 继续沿原严格判定。确认独立 harness 已退出、证据完成后才封存成功证明，沿原期限等待。范围、环境、命令、回执或完成状态不能完整表达时，不产出可复用成功证明；默认执行仍按原入口规则可用，复用明确拒绝。

manifest 只记录直接执行阶段。按阶段原子提交成功记录，允许后续 native 失败后仍保留可核验的前置成功；不能对旧报告补签。只绑定封存的阶段证据，不绑定后续仍更新的 batch summary/case 记录或整个输出目录；用“后续 native 改写 summary 为 failed”反例确认前置证明仍有效。来源和引用报告均只读原始证据。记录失败原因时给出输入/产物/覆盖差异，不实现自动修复或兜底补跑。

在 `tests/framework/prerequisites-test.ps1` 增加必要合同测试，并注册进原 test-framework。证据和源文件变化反例使用隔离的小仓库/合成报告，不临时修改真实源码或借用生产现场。至少覆盖：

| 合同组 | 正向/反向检查 |
| --- | --- |
| 输入范围 | native-only、无关说明文档可复用；参与 L1 的 docs JSON、生产/测试/helper/framework/catalog/扩展/依赖修改及增删均失效；Git/构建工具身份变化失效；unsupported 输入拒绝；排序跨现有支持 PowerShell 一致 |
| 覆盖 | 完整集合通过；少一项、多一项、重复、同数量换名、包通配符字符串相同但实际包缺失、零测试、意外 skip、failed/running/incomplete 拒绝；L1 原允许 opt-in skip 不变 |
| 来源 | 新直接成功证明可读；总批次 native 失败但两个前置完整可读；失败前置、旧无证明报告、多来源/递归来源拒绝 |
| 文件身份 | 每种必要产物、build receipt、manifest、日志的缺失/变化均拒绝；阶段输入开始/结束不一致、两个阶段身份不同、result 已出现但 harness 未结束均不构成成功来源 |
| 兼容 | 原参数/目录、无 build.json 的显式 Runner、自定义 catalog 的默认执行、配置早停、PreparedArtifactsDirectory 的失败/skip/摘要/互斥反例、scheduler 和 delivery guards 保持；缺少复用资格不导致默认路线失败 |

不要求测试所有字段的机械排列组合。重点证明旧成功不能掩盖源码、执行集合或实际证据变化。

## 5. P3：正式入口消费，worker 只执行

`test.ps1` 接受一个 PrerequisitesFrom，只开放恰为 `-Level L2` 且使用默认 catalog 的请求；L3/L4、混合级别、纯前置、ListAvailability、自定义 catalog 与该参数组合时明确拒绝。先核验来源和两个阶段，再选择来源 Controlled Runner、核对显式覆盖，然后构建或核实配置 helper，执行当前能力/配置评估和 case 展开。无可运行目标继续原 incomplete 路径；有可运行目标才冻结配置并执行当前环境检查与 preflight。不得在来源 Runner 选择前走默认重建分支，也不能在得到当前评估前判断可用性。两个阶段必须同时命中，不增加“尽量复用”的分支。

仅在复用 L2 路线上，在 fixture 副作用前重新编译 driver 并执行零匹配 guard，再向既有 case job 加入可选 driver 路径/摘要。`run-test-case.ps1` 与 `test-native.ps1` 在执行前核对并消费相同文件。两个新字段都缺省时，原默认/直接调用/旧 job 继续原自行编译路线；任一字段出现时必须成对齐全且匹配，不得降级、换文件或静默重新编译。L3 默认和 L4 的 `<本批输出>/controlled` 产物目录约定不动，不新增共享 Controlled 路径参数或复制适配层。

本轮不复用来源的 live 配置可用性、case 结果、fixture/job 或 PR；不重新计入来源失败轮次为本轮成功。本轮来源字段与原始结果分别记录，coverage/aggregate status 仍由 coordinator 汇总。最终先复核输入/产物与完整 `(scenario, case, repeat)`，再计算 aggregate status 和资格布尔值；不能在 catch/finally 中由 passed case 去重集合重新授予资格。增加“case 全通过但最后复核失败”的反例，要求 status=failed、complete_level_passed=false、l2_verified=false，实际 passed case/清理记录保留，不重发外部动作。

增加无模型的入口合同测试，验证不支持的级别/来源拒绝时 fixture helper/worker 从未启动、命中时 L1/Controlled 从未启动、实际 worker 接收的是本轮 preflight driver。覆盖可选字段缺省兼容及半套字段/摘要变化拒绝。测试用替身只在隔离测试进程注入，不提供正式参数绕过前置。

## 6. P4：有界验证与验收证据

验证按依赖推进；通过后不无故重复完整长测。

1. **静态与合同回归**：参数兼容、摘要/覆盖/来源/早停反例、新旧 framework guards。使用宿主及现有要求的 Windows PowerShell 5.1 检查新增共享摘要逻辑；不把运行时版本变化写成同一环境命中。
2. **默认路线完整前置**：用实施后默认 `test.ps1 -Level L1,Controlled -OutDir <新目录>`，实际执行完整 L1 与 Controlled，生成新格式直接证明。核对当前默认 25 个顶层名单、内部测试内容未改、零意外 skip、Node/Go/framework/zero-match。后续源范围若再变，此前置失效，必须重建。
3. **正式复用路线**：最终代码稳定后，经 `test.ps1 -Level L2 -PrerequisitesFrom ...` 使用来源 Controlled 的同一 Runner，在新 fixture 运行当前完整 L2；实际 driver 必须与本轮 guard 一致，不得手工复制 request job。native-only 变更后的命中与重新编译在隔离合同中验证；若实施本就需要 native driver 修改，可将其合并进这次真实验证。不能为了改变 SHA 强行添加无用诊断代码，不能将临时改动或已撤回 driver 的结果授予最终版本。
4. **实际结果核验**：按原 L2 要求核验提交、push、测试 PR 与专用 base 的真实合并、合并后内容、原生 history/turn/命令审计和 Run/服务停止。保留任何失败轮次；有 unknown 先按现有路径对账，不为完成验收自动重试发布。
5. **必要负例**：在隔离数据中改变来源文件、摘要/集合、Runner/driver、环境身份和重复次数；证明正式入口不准备真实 fixture，且不出现 passed/verified 残留。真实目标的不可用/排除/清理失败继续由原合同测试覆盖，不为反例制造新的远端副作用。

本轮新能力只开放 L2，必须取得完整真实 L2。L3/L4 用现有合同回归核对不带复用的路径，尤其缺省 job 字段与 L4 PreparedArtifactsDirectory；实际目标未跑的列未验，不开放其复用资格。若实现必须改变 L3/L4 专属行为，应先停下该范围扩展、调整设计；不能顺手改动后只以 L2 通过结案。

阶段记录至少包含前置执行时长、复用核验时长、preflight、fixture 准备、native 和收尾。成功标准是 driver-only 重跑无 L1/Controlled 进程、无手工证据组合，同时仍达原 L2 断言；不设模型响应必须低于几分钟的硬阈值。

## 7. 回退与范围控制

默认完整执行路径始终可用。显式复用失败只能要求重新生成前置或由用户改用默认命令，不静默接受旧证明、不静默创建第二批外部执行。取消复用参数即可回到完整路线；实现不能依赖全局缓存状态。

若现有报告消费者不能接受新增可选字段，先补兼容读取和合同测试；禁止为了方便改掉旧 summary 字段含义。若旧证据不足，重新生成一轮前置，不引入历史迁移项目。若某个共享输入难以界定，保守扩大前置摘要或拒绝该输入复用，不扩大本轮成自动依赖追踪工程。

Controlled 业务分组、内部 case 并行、ResultAcquisition 拆分、生产执行解耦另开任务；P0–P5 中不得顺手实施。

首轮不统一默认批次的全部 Runner 构建、不统一所有级别的 driver 分发、不适配 L4 跨目录产物、不加入通用环境探测或 harness 监管服务。完成证明只补必要等待与复核；这些设计不应成为复用 L2 的前置项目。

## 8. 交付检查表

- Spec A1–A8 各有测试或证据引用；默认路线与复用路线均符合原资格要求。
- 测试目录、断言、循环、期限、skip allowlist、lease/调度/清理与生产代码未因提速而削弱。
- 新增脚本、共享阶段与报告合同没有第二套注册表或资格判定真源。
- 实施完成后更新新增命令与状态；历史失败、旧资格与未验目标保留。
- 独立验收记录列固定 Runner、来源 manifest、各阶段输入/产物 SHA、新 driver、真实 L2 结果与耗时，便于审查是否真的只省掉重复前置。

## 9. 文档审计与修正（2026-10-31）

以下记录设计阶段确认的问题与修正；实施与实际资格以文首状态和验收记录为准。

| 编号 | 确认问题与依据 | 修正 |
| --- | --- | --- |
| R1 / P1 复用误命中 | [L1 资源测试](../../internal/resources/content_repository_test.go)直接读取 docs JSON，原范围遗漏且“文档独改可复用”过宽；环境身份还遗漏 Controlled 使用的 Git | 增补 JSON 和枚举控制文件，区分参与测试的文档；固定记录 Git 和实际 shell/tool 身份，不做全环境扫描 |
| R2 / P1 范围及 L4 断链 | [case worker](../../scripts/run-test-case.ps1)硬编码本批 controlled 目录，原 Plan 开放 L4 复用却未传来源，且只安排真实 L2 | 首轮明确只开放 L2 单级别复用；L3/L4 和混合级别使用原完整路线，不新增 L4 目录适配 |
| R3 / P1 失败仍授予资格 | [test.ps1](../../scripts/test.ps1) finally 按 case coverage 赋 l2_verified；新最终身份复核失败时仍可能被置 true，complete_level_passed 也可能残留 true | 最后复核后统一结算资格，失败布尔值不可覆写；保留实际 case 集合，补 case 全通过但资格失败和缺 repeat 反例 |
| R4 / P2 默认兼容与额外改造 | 原设计要求默认统一 Runner、全部级别统一 driver，又未区分新回执要求与旧显式 Runner 支持，可能拒绝原来合法的默认测试 | 严格证明只控制复用；默认构建和直接调用保留；固定 Runner/driver 仅用于显式复用 L2，可选 job 字段向后兼容 |
| R5 / P2 阶段成功与覆盖身份 | 原文未钉死每阶段身份、通配符实际包集合，A5 的笼统 skip 拒绝也与 L1 原 allowlist 冲突；[foundation](../../scripts/test-foundation.ps1)独立启动、[TestMain](../../tests/integration/service_test.go)在退出前写 result | 每阶段分别记录身份，核对当前实际包和默认 Controlled 集合，保留 L1 既有允许 skip；确认 harness 退出且证据完成后封存，不能只等结果文件出现 |
| R6 / P2 执行顺序含糊 | 原 P3 在选择来源 Runner 和配置评估前就要求判定可用性，易先重建 Runner，或错误复用旧 available | 明确来源核验、选 Runner、当前评估、无目标早停、冻结、新 preflight、原调度顺序 |
| R7 / P2 验收引入无用改动 | 原 P4 强制 native driver SHA 改变并将等价诊断改动留在提交，可能为验收制造无用代码 | 隔离合同验证变更命中，最终代码执行真实复用 L2；必要 driver 修改可合并验证，不强制添加诊断 |
