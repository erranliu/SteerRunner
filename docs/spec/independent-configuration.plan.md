# Plan：独立调用配置契约与内部测试配置

2026-10-31 当前开发权限修订：开发 sandbox 已退役，开发必须显式 full_access；read_only 与原认证链路保留。开发权限、迁移、旧 Run 与重复测试矩阵以 [统一权限 Spec](development-full-access.spec.md) / [Plan](development-full-access.plan.md) 为准。下文历史构建与验收事实不改写。

日期：2026-10-19。状态：已获授权实施；进度／证据见 §5 及[验收登记](../acceptance/independent-configuration.md)。[Spec](independent-configuration.spec.md)。以下分步内容保留原实施判据，实际结论以本次运行证据为准。

## 1. 实施边界与前置

调查基线：`59c7597ac8056fc5c29ad622034e10e74e4395de`。后续获实施授权时重新固定源码提交、dirty 状态、Runner／helper EXE SHA、资源摘要及依赖版本，不能继承当前历史验收作为新构建结果。

只安排 Runner 仓库：公开能力／纯配置契约、独立 CLI 消费、测试配置与 case 需求、可用性和报告。SteerLine 自行实现方案管理、默认选择、项目绑定和请求持久化；本仓交付其可消费的固定版本契约，不修改 SteerLine 数据或实现。

本期不建产品 profile 管理、环境资产清单、配置热更新、自动部署、凭据库或新执行状态机，不修改存储格式、不迁移旧 Run，不扩大权限／agent／原生版本支持范围。

## 2. 当前代码与拟改动位置

| 位置 | 当前事实 | 拟改动 |
| --- | --- | --- |
| `internal/contracts/run.go` | 已有生产纯 gate | 作为唯一语义入口，补共用结构化诊断／可靠映射；保留旧 gate 语义，避免 CLI／测试各写一套判定 |
| `internal/resources/configuration.go`、`assets/settings.schema.json` | 已发布 RunConfiguration；组合规则部分生成 | 补完整 Run input schema、有限类型化支持组合／默认／限制及一致性反例；保留现有字段与 schema 键 |
| `internal/resources/resources.go`、`internal/cli/cli.go`、`internal/host/server.go` | capabilities 当前经服务，validate 尚无 | 增加 Spec §4 的离线能力与两类原始对象校验，响应沿 v1；由服务附带真实来源身份，不以 status 拼接，不改变旧写请求读取 |
| `internal/application/freeze.go`、`runstate` | 已有接受冻结及原输入恢复 | 原则上复用；只在发现公开默认与实现不一致时定向修复，不因测试管理增加持久状态 |
| `internal/testkit/livetest/cases.json`、`cases.go` | 唯一 case 注册及现有 manifest 校验 | 扩充配置项引用／必要角色和约束；不建设通用规则引擎，不变更原场景目标 |
| `scripts/live-catalog.ps1`、`tests/catalog.json` | 校验注册一致性 | Go／PowerShell 直接消费唯一需求；catalog 不增加需求镜像，现有重复元数据继续核对 |
| `scripts/test.ps1`、`new-test-fixture.ps1`、`run-test-case.ps1` | 读取扁平 live-config 并选择性装配 | 读取新测试配置，纯解析、可用性、冻结后派发；生成最终请求仍走生产 gate |
| `scripts/stability-preflight.ps1`、`test-scheduler.ps1` | 前置检查／有界调度／排除与收尾 | 对接需求及结果；保留原锁、并行、原资源对账 |
| `tests/framework/`、`tests/integration/`、`tests/native/` | 已有框架、Controlled、真机链路 | 按 IC 项补最小反例及改造配置读取，不用 mock 宣称真实环境通过 |
| 示例与测试文档 | live-config/v1／v2 示例和旧入口 | 拟新增 `tests/test-config.example.json`、测试外层 schema，更新测试说明／清单；旧格式明确拒绝，不自动转换 |

测试外层 schema 和解析器属于 testkit／测试脚本，不放到生产配置加载路径。外层结构、单项公开字段、case 约束分层校验，单项无效不把全文件标为损坏。运行字段调用现有生产 schema／gate；严格解析优先复用 Go contracts 及已有测试 helper，不新建常驻服务或每层一套校验程序，也不能凭 ConvertFrom-Json 宽松结果推进。

## 3. 分步实施

### P0 固定契约与场景映射

1. 逐项核对公开 gate、schema、freeze 的默认值、遗漏／空串／null、能力组合，形成对照用例。以原 gate 判定／主 fault／退出码及默认值为本期兼容基线；差异修正到 schema／说明，不能用新 schema 前置拦截旧写路径。模型默认保留现有语义，不用新增测试框架改变产品选择。
2. 从唯一 case 注册逐 case 列出固定配置项引用、必要角色、workflow／agent／模式、模型关系、夹具和管理员设施需求。Run 按实际 input 验证；混合场景列出全部角色。当前数量从 registry 计算，不复制可能过时的数字或增加用户绑定表。
3. 固定 Spec §5 的外层 schema、严格解析及字段诊断；保存基线和完整 L3 资格的独立结论。没有本机 Codex 时仍能做 L1／Controlled。

完成条件：IC-01／04／05 的输入矩阵、注册映射及预期结果可以确定性验证；不启动模型、修改部署或对外宣称支持。

### P1 公开契约与独立使用

1. 在已有发布中增加 configuration_schemas.run_input、capability_catalog/version/combinations/validation_kinds 及组合／默认描述；保留既有字段和 run_configuration。validation_kinds 的两个固定引用随响应可解，不做动态校验注册。由共同实现描述生成有限组合，缺省值只建立只读匹配视图；workflow null 不作通配。目录缺项与 gate 接受分别测试，不能把目录变为旧请求的新白名单。
2. 实现 capabilities --offline：静态目录与在线同版同源；capability_source 标明 executable 或 service、构建／资源身份，在线 instance_id 由实际响应服务提供，不能先查 status 再拼接。新字段增量发布，离线结果不冒充当前服务事实。
3. validate 支持 run-configuration、run 两个 kind，stdin 直接读取对应对象，不读取／自动解包写信封，不要求或回显 request_id。严格 JSON／UTF-8／大小限制沿既有规则；在 host／state-dir 处理前完成。既有写命令继续走原 ReadRequest 与幂等处理。
4. 共用校验输出 issues 的 JSON Pointer、稳定 code 和 message；根级定位作为明确兜底，不靠解析人类文案或重复规则。沿 v1 Response，request_id=null；原 gate 决定 valid、主 fault 及退出码，附加 schema issues 不改变错误优先级。已知 kind 输入格式错误为 invalid_input／exit 2、有结论的 gate 错误为原 exit 2/3，参数／协议错误 data=null；I/O／内部故障 internal_error、data=null、exit 1，不伪造 valid=false。按 Spec §4.3 表格验证，不改变旧命令错误结构。
5. 区分 CLI 协议、schema／目录结构版本、构建／资源／实例身份和调用方配置版本；更新 help／命令列表／裸对象与响应样例。新增静态生产目录／schema／默认规则纳入 resource_digest，排除来源身份、摘要本身及测试配置／case 注册／时间／辅助路径；用确定序列化避免自引用与现场漂移。断言公开目录变化会改变摘要，测试／实例变化不会。新 CLI 与旧服务仍按既有构建身份拒绝混用。
6. 用隔离目录及动作计数证明零副作用。保存旧合法／非法请求与旧能力消费样例作对照，验证接受／拒绝、主错误／退出码、默认值、原写信封／request_id、实例绑定及普通 submit 不读测试文件。已知消费者未通过不能宣称兼容，需适配时由调用方仓库明确完成；不增加自动双 payload／协议降级，不强制外部改用 validate。

完成条件：IC-01、IC-02、IC-03、IC-10 的相关 L1／Controlled 通过；不改生产写请求形状、存储格式或执行生命周期。

### P2 独立测试配置与本地基线

1. 新配置只由显式 Config 指定；按入口 cwd 解析文件路径，读取一次并固定本批副本／摘要，文件内辅助路径按原文件目录处理。worker 与 fixture 生成器只消费快照，同时保存批次 catalog；固定 Runner／helper／需求身份，生成器在解析后再次对照批次原摘要，拒绝旧 helper 使用不同注册。execution 始终校验，workflow 若提供则校验完整 RunConfiguration；Run case 缺 workflow 不可用。共用一份 fixture 的完整 Repository／可选 Publication。
2. 严格区分全文件损坏与单项无效、未配置与显式空值。拒绝未知版本、字段、重复键及隐式继承；按字段返回无秘密诊断。需求引用由 registry 维护；用户不维护 case 绑定、baseline 指针或多项目 fixture 目录。
3. 基线固定 configurations.local-codex，为 development/sandbox；检查显式绝对 native 路径和 L2 的注册引用。路径／版本等现场核验复用原资格机制，不写进静态可用性状态；不把它作为全部外部请求／L3 的硬依赖。
4. 现场生成器只替换 Spec 允许的临时仓库、分支、固定 commit、路径和材料，记录原值／生成值；保留 Orca 注册项目入口，caller clone 不能替代它。既有 provider/query 取证与故障 helper 仍由 case 生成，记录具体路径／参数，不要求配置填伪造占位值。不通过场景名改变模式或模型，不扩展既有测试仓库范围。生成后校验完整请求及测试独有约束，再保存 request_id 和正文。
5. manifest 保存源配置与需求摘要、最终请求、EXE／helper／资源摘要和原生身份。重启／重复查询沿原材料，不重新读取当前 Config；不将认证秘密写入 manifest。
6. 同时替换示例和脚本消费；旧 live-config/v1|v2 给明确迁移说明，已有冻结现场用原构建／manifest 结算。外层格式升级不修改生产协议或数据库。

完成条件：IC-03／04／05／07 受控通过；旧 L1／Controlled 不因真机配置缺失而失效。

### P3 可用性、调度与结果

1. 扩展唯一 registry 的需求，计算 available／unavailable 及全部原因；这是配置可用性，不新增 configuration_status／environment_status 或就绪缓存。缺角色列原因，不猜测回落；目录未声明的所需链路记 undeclared_capability，不冒充 gate 拒绝或现场失败。
2. 只新增 ListAvailability 静态模式，限定 L2／L3，零 L1／Controlled、调度锁、现场、服务、工具／账号／网络探测与模型。查询前明确既有流程产出的待测构建；缺产物按查询失败，不能查旧 EXE 后跑新 EXE。实际 worker 使用同一产物并核对摘要。沿既有实际测试准备核验环境，不增 CheckEnvironment／AvailableOnly 两套模式。
3. 保留请求集合；unavailable 项零派发，其他独立 available 项仍能执行。通过现有 Scenario 显式选子集；所选缺项标记 incomplete／非零退出，不能自动缩小覆盖后宣称通过。整文件错误和公共前置失败才全局阻止派发。
4. 沿原 L1／Controlled、锁和隔离前置启动 worker，再做各 case 必要现场准备；L2 专属原生 Git 前置由其 worker 承担，失败不阻断独立 L3，不额外强制 L2。准备／执行失败保留原事实并收尾，不倒改成配置不可用或排除。
5. 复用 scope、原 case outcome／cleanup，增加配置原因和逐级 complete_level_passed，从注册／请求／排除／不可用／执行／通过集合计算覆盖，重复次数另计。前置失败／中途退出时补齐仍为 queued 的原请求项，queued 不计执行，保留 unavailable 原因。整文件／选择／查询错误 exit 2、实际失败或请求缺项／零真机执行 exit 1，显式子集沿原通过规则；失败优先且保留缺配置事实。

完成条件：IC-05／06／07 的框架测试通过；原并行／独占、丢响应、重启、停止和清理语义保持。

### P4 定向真机与交接

1. 新配置、本次独立现场，先通过本地 Codex 确定性前置，再执行原 L2。路径核验、真实请求、默认提示词、PR 事实和停止／清理均须留证；缺本地配置不能宣布交付合格。
2. 对已配置且纳入本次支持声明的可选链路，复跑其相关既有 L3，证明新绑定进入真实链路；审计／混合等配置转换发生变化的链路追加其相关既有 case，不重写业务测试来绕过失败。没有可选配置时记录不可用／未验，只能交付已证明的本机基线范围。
3. 全 L3 是单独资格：全量配置缺失或任何 case 未执行，保留 incomplete；子集通过不替代。版本原有的其他必验链路仍按原交付要求执行。
4. 交接公开 schema／支持组合、离线／在线来源、三种裸对象 validate 及字段诊断样例、固定构建摘要和服务切换要求。SteerLine 后续自行验证新响应字段消费、配置选择、请求持久化、实例绑定及任务关联；Runner 不代记调用方通过。

完成条件：IC-08、选定范围 IC-09 有真实证据；报告清楚划定本机基线、可选链路及完整 L3 各自结论。

## 4. 最小验证安排

以下为实施测试安排；实际结果见 §5 和验收登记。优先扩展既有测试入口，不为每个配置字段写镜像测试。

| 验证组 | 覆盖与关键断言 | 入口／时点 | 对应项 |
| --- | --- | --- | --- |
| 公共契约 | 内外同值同判；目录声明／gate 接受／现场就绪分层，缺省匹配／workflow null／操作限制；三个 kind/schema 可发现；裸对象、误传信封、重复／未知／null、定位及输入／I/O／内部错误、主 fault 优先级 | P1：`go test ./internal/contracts ./internal/resources ./internal/cli`，扩展现有发布测试 | IC-01、IC-02 |
| 无副作用及外部隔离 | 离线／validate 零 host／目录／Agent／Git／PR；request_id=null；在线来源由同一实际响应服务给出，换代不混拼；测试文件不影响外部请求 | P1/P2：扩展 `TestSRStandalonePackage`、`TestSRInputFreeze` 等 Controlled 用例 | IC-02、IC-03 |
| 外部兼容 | 旧请求判定／默认／主错误及退出码保持；不加目录白名单；旧字段／schema 键保留，已知消费样例通过；静态目录变摘要、测试／来源不变摘要；原实例绑定不绕过 | P1：旧请求与消费样例对照，定向 CLI／host Controlled；SteerLine 自有消费另验 | IC-10 |
| 配置与基线 | 固定本地配置缺失，绝对 native 与 shim／错版本；无 Codex 仍可跑 L1；外部不读取基线；L3 全通过仍不能伪造 L2 | P2：配置 testkit／framework 测试及定向原生前置 | IC-04、IC-05 |
| 可用性与选择 | 单项无效不掩盖其他项；审计／混合／模式／设施；available 只表示配置齐备；缺项零派发且独立项运行、请求范围不缩小、失败优先级／空集／排除／覆盖准确 | P3：`tests/framework/` 受控矩阵，动作计数及 summary 断言 | IC-05、IC-06 |
| 冻结与可靠性 | 原配置批内变化不改变 worker 输入／路径基准；旧 request_id／resume 不变；查询与实际执行构建同源；原停止与未知对账保持 | P2/P3：复用 Controlled；相关 L3 只验证变更涉及的链路 | IC-03、IC-07 |
| 本机基础闭环 | 新配置、本地原生 Codex、原 L2 全流程及收尾 | P4：实现后的 `scripts/test.ps1 -Level L2 -Config <新文件>` | IC-08 |
| 可选链路／完整范围 | 所声明的选定链路真实运行；缺配置报告不完整；完整 L3 不被子集替代 | P4：实现后的 `-Level L3 -Scenario <已配场景>`；全量单独运行 | IC-09 |

完整 L1／Controlled 沿 `scripts/test.ps1 -Level L1`、`-Level Controlled`，L2／L3 也自动包含上述前置。新增的配置／框架测试已纳入这些既有入口。

使用示例（静态清单要求先构建 Runner 与 test-config helper，见测试指南）：

```powershell
.\bin\steerrunner.exe capabilities --offline --json
Get-Content -Raw -Encoding utf8 .\run-configuration.json | .\bin\steerrunner.exe validate --kind run-configuration --input - --json
Get-Content -Raw -Encoding utf8 .\run-input.json | .\bin\steerrunner.exe validate --kind run --input - --json
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json -ListAvailability
.\scripts\test.ps1 -Level L3 -Scenario local-codex.feedback -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json
```

三个校验文件均直接保存对应配置／input 对象，不含 protocol、request_id、input 写信封。真正 submit 仍使用原信封；不要把 run-input.json 原样当成 submit 信封。Scenario 明确选择范围；完整 `-Level L3` 缺项非零退出，但独立已配置项仍可运行。Availability 列表正常返回只表示配置诊断完成。

## 5. 文档、验收与当前交付

实施时同步 README、docs/testing.md、docs/test-catalog.md、测试 schema／示例、help、注册需求及调用样例。产品／测试入口变更与规则文档在同一变更中完成；历史验收不改写为新规则通过。

下表保留首次实施的构建结果。实现审计已同步到主线 `a3dc15d` 并完成复测：完整 L1 318 个 Go／28 个 Node、Controlled 16/16、L2 1/1、已配置 L3 3/3；26 项 unavailable，完整 L3 仍不通过。最新身份、修复、补充反例及运行中父入口差异见[验收末节](../acceptance/independent-configuration.md#实现审计与主线复测)。

| 阶段 | 当前状态 | 证据／边界 |
| --- | --- | --- |
| Spec／Plan | 初稿与审计完成；本轮已授权实施 | 保留 §6 历史文档审计，不将其算作运行验收；本轮结果见验收登记 |
| P0–P1 契约与静态 CLI | 已实现；最终 L1 与 16 个 Controlled 场景通过 | 307 个 Go 顶层测试、28 个 Node 扩展测试及 framework；新增 gate 对照／静态接口／摘要与来源断言 |
| P2 测试配置与本机基线规则 | 已实现；静态与框架通过 | 新格式、单项诊断、原目录路径、基线／Run workflow、旧格式拒绝与冻结摘要 |
| P3 可用性与报告 | 已实现；框架通过 | 缺配置零派发、独立项执行、完整范围／覆盖判定；真实批次缺项事实继续登记 |
| P4 本机 L2 | 通过，baseline.l2_verified=true | 原生 Codex 0.159.2、生产默认提示词、真实 PR #564 更新／合入、原 Run／服务／认证收尾，IC-08 |
| P4 选定 L3／完整 L3 | 已配置本机 3/3 通过；26 项不可用，完整 L3 不通过 | 实际 4/4 case 通过，完整请求保持 30；批次 incomplete／exit 1，无实际失败或显式排除。见原报告及便携清单，IC-09 |
| SteerLine 消费 | 本仓不实施 | 固定 Runner 契约交接，调用方在其仓独立验收 |

用户已授权按本 Plan 实施、测试和验收。本轮仅修改 Runner 仓库；SteerLine 消费验收仍由其仓库负责。

## 6. 文档审计修正（2026-10-19）

本轮为当前会话的静态审计与直接修正，不宣称独立多代理复审，也不计运行验收。

| 问题 | 已落实的文档修正 |
| --- | --- |
| baseline 选择指针、逐 case 用户绑定及多 fixture 映射重复承担测试注册职责 | 固定 local-codex 基线、registry 直接引用配置项、一份共用测试源；不建多项目配置管理器 |
| 三组状态与独立环境探测形成第二套就绪判断，新增模式过多 | 仅配置 available／unavailable 及原因；只加列表入口，现场失败留在原准备／测试流程 |
| 全量缺一个配置就零派发全部用例，超出单项不可用边界 | 不可用 case 不启动，独立已配置项可运行；请求范围不缩小，缺项仍 incomplete／非零退出 |
| L3 全通过与本机交付基线、子集与全量、失败与未配置的优先级不够明确 | 基线仍要求同构建 L2 真实证据；逐级覆盖按实际集合计算；失败不被 incomplete／排除掩盖 |
| 批内重读配置、相对辅助路径、能力来源与执行构建、现场 helper 路径可能形成隐性漂移 | 固定一次快照及原目录基准；查询／执行同产物；case 生成 helper 后再按公共契约冻结完整请求 |

当时文档审计后的实施范围仍为 P0–P4，尚未实现。当前实施结果见 §5，不将历史修正文档记为功能通过。

本轮接口增量继续归 P1：原始对象 validate 取代尚未实现的信封校验设计，新增配置片段 kind、字段诊断、类型化支持组合与来源／版本说明；IC-10 单独约束旧外部调用兼容。保留原执行接口和测试职责边界，不将接口整理扩展为 profile 管理或整体 v2 迁移。

接口增量后的再次审计修正：

| 问题 | 本轮修正 |
| --- | --- |
| 有限能力目录可能被当成新白名单，使旧 gate 接受的扩展标识突然被拒绝 | 明确目录声明、纯 gate 判定和实际就绪三个边界；未声明与明确不支持分别诊断，旧输入不额外拦截 |
| kind 发现、workflow null／缺省匹配、operations 的承诺范围不够明确 | 增加三个固定 kind 到已有 schema 的可解引用；null 非通配，默认只用于只读匹配，操作存在不证明原生停止完成 |
| 校验对象错误与 stdin／内部故障混淆，收集 issues 可能改变旧 fault 优先级 | 明确响应／退出表；I/O／内部故障不形成 valid=false，主 fault 始终由原 gate 决定 |
| Plan 的摘要表述可能把测试目录／动态身份加入生产资源摘要 | 明确仅静态生产契约参与，测试配置／case／时间／路径／来源不参与，并安排双向摘要断言 |
| schema 一致性和增量字段兼容只写目标，缺少遇到差异时的处理边界 | 原 gate 语义作为基线，schema／说明对齐；已知消费者实际通过后才能声明兼容，不建双协议补丁层 |
