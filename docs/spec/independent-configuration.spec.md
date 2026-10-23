# Spec：独立调用配置契约与内部测试配置

日期：2026-10-19。状态：已实施；本机基线与已配置 L3 通过，完整 L3 缺配置而不完整。运行证据见[登记](../acceptance/independent-configuration.md)及 [Plan](independent-configuration.plan.md)。

## 1. 目标、范围与当前基线

SteerRunner 是可靠的开发执行器：接收确定的材料、执行参数和 workflow，固定输入，在指定环境执行，处理反馈、恢复和停止，保存可核验结果；当前主要交付结果为 PR。独立用户、SteerLine 和本仓测试框架均为调用方，共用公开执行契约，各自维护配置和选择策略。

本规格固定以下边界：Runner 声明支持能力并验证、冻结和执行请求；外部系统管理实际部署、执行方案和默认选择；测试框架维护本仓独立测试配置。本地原生 Codex 为独立项目交付与基础真机测试的必需配置，不成为所有外部请求的统一架构依赖。

调查基线为本工作树提交 `59c7597ac8056fc5c29ad622034e10e74e4395de`。当前已有 `capabilities.configuration_schemas.run_configuration`、`contracts.ValidateRunConfiguration`、`ValidateRun`、`ValidateEnvironmentCheck`、接受时冻结、CLI 幂等与实例绑定。本仓测试不依赖 SteerLine checkout；`live-config/v1|v2` 的扁平字段由脚本装配为生产请求，场景需求、默认值及环境前置部分散布于脚本。公开 CLI 尚无本文的离线能力查询、纯校验及测试可用性入口；完整 Run 输入 schema 也不能由现有通用信封 schema 替代。

相关依据：[产品与 CLI](steerrunner.spec.md)、[配置发布现状](../acceptance/run-configuration-publication.md)、[workflow 选择](../archive/2026-10/spec/workflow-selection-audit.spec.md)、[环境检查](environment-check.spec.md)、[测试分层](../testing.md)、[测试清单](../test-catalog.md)。本文是职责、配置消费及可用性规则的增量，不改变原执行状态机、权限、停止、PR 核验和测试独有断言。

## 2. 职责与非目标

| 事项 | Runner 产品 | SteerLine／独立调用方 | Runner 测试框架 |
| --- | --- | --- | --- |
| 支持能力、组合限制、参数及错误语义 | 唯一权威 | 消费选定版本的声明 | 消费同版声明并验证一致性 |
| 机器、environment、工具安装、账号与模型部署 | 按请求核验，不维护部署清单 | 配置并维护；独立使用时由用户负责 | 维护测试所用部署配置，不修改用户安装来补齐条件 |
| 命名方案、项目绑定、默认执行选择 | 不维护方案库，不替调用方选择 | 维护并展开为完整请求 | 维护测试配置及唯一 case 需求引用 |
| 请求、request_id、重试正文 | 按原请求去重、拒绝冲突 | 提交前保存确定正文与 ID | manifest 保存本次正文与 ID |
| 有效输入、资源及结果事实 | 接受后冻结；原操作对账 | 查询并保存必要关联 | 通过公开路径断言并保存证据 |
| 任务规划、DAG、候选、TaskDelivery | 不拥有 | SteerLine 等上层负责 | 用例定义预期，不写产品任务状态 |
| 测试需求、可用性及批次结果 | 不读取测试配置 | 不承担本仓测试调度 | 唯一权威；Runner 仓库维护 |

Runner 的 state-dir、日志等宿主设置仍由 Runner 定义和消费；本期不扩展宿主配置系统，不增加 profile CRUD、环境注册表、动态模型目录、凭据库、配置热更新、插件市场或自动安装／修复依赖。CLI 不读取 SteerLine 数据库或配置。测试 profile 名称不进入生产路由，也不成为执行权威。

## 3. 统一配置契约

### 3.1 共用单元与校验层次

1. `Execution` 与 `RunConfiguration = {execution, workflow}` 保持现有公开形状。内部配置的 execution 直接符合公开 Execution；需要 workflow 的 Run 再校验完整 RunConfiguration，不另建 `codex_executable` 等生产字段的平行版本。
2. `repository`、`documents`、`publication`、环境检查 input 及 CLI 信封复用各自公开契约。`run_configuration` 校验通过不代表完整 Run／Publication 校验通过。
3. 对外发布完整 Run input schema，并复用既有环境检查 schema、adapter/provider schema。`cli.schema.json` 继续表示信封；文档必须区分信封、配置片段和完整 input。
4. JSON 类型、必填、枚举、未知字段、显式 null 等形状规则在 schema 与生产 gate 间保持一致。仓库与发布目标一致性等不能由标准 JSON Schema 表达的跨字段语义，由同一生产纯校验负责，公开说明限制；schema 单独通过不承诺准入通过。
5. 测试配置项的 execution 始终校验；workflow 如提供，连同 execution 校验 RunConfiguration，不接受“未消费所以可填非法值”。Run case 缺 workflow 不可用；environment-check 不要求 workflow，只投影 execution 及检查输入，不发送虚构 workflow。配置项携带的合法 workflow 也不能证明检查组合受支持。SteerLine 可独立做前端校验，Runner 的最终 gate 始终有效；测试不能放宽生产规则或借测试字段绕过它。

同一字段值由生产 schema／gate 解释；测试独有约束只决定用例是否满足验证目的。例如“两模型不同”、基线的绝对 native 路径、特定模式属于测试约束，不反向改变外部请求契约。新增组合描述采用有限、类型明确的声明，复用现有判定，不建设通用规则 DSL／配置解释器。

纯配置校验不读材料文件、查可执行程序、解析凭据、fetch、启动服务／Agent 或创建 PR。材料取得、原生身份、版本、权限及认证由原执行准备门槛验证。受理前明确拒绝与受理后 blocked／failed／unknown 保持既有区分，不能保证所有现场错误都在受理前发现。

本增量以原生产纯 gate 的接受／拒绝及原默认语义为兼容基线。发现既有 schema 与 gate 的差异时，先登记对照输入，再让 schema／说明匹配已保留的判定；不能用“统一契约”为由将新 schema 校验前置到旧写路径，静默收紧旧请求。标准 schema 无法表达的语义继续明示并由 gate 判断。任何确需改变旧判定的修复应另行明确范围与兼容策略，不夹带在本配置增量中。

### 3.2 默认值、解析与冻结

Runner 仅应用公开契约中固定的默认值。当前普通本机路径的 executable 缺省 `codex.exe`、effort 缺省 `medium`、access_mode 缺省 `sandbox` 等应逐项登记并由生产路径测试；底层辅助函数的默认值不能另成为公开规则。模型未提供时若允许采用原生默认，必须明示其语义和可复现性限制；空串、缺省和 null 不得被测试脚本自行混同。审计必需模型规则沿现有契约，测试可另要求两模型不同以证明切换。

调用方在提交前展开自己的默认方案及覆盖，并保存确定正文。Runner 不注入另一个用户 profile，不因 cwd、测试文件或 SteerLine 环境变量改变选择。禁止因不可用而静默更换后端、agent、模型、target 或权限模式。

接受时保存有效执行参数、材料、workflow 和资源摘要；凭据只保存非秘密引用，在实际执行主机解析。模型等后端实际值在原生协议可核验时记录，不能把未指定模型伪写为已经固定的模型。配置文件变更只影响未来请求；原 request_id 重试使用原正文，反馈／恢复使用原冻结值，原 unknown 只对账。调用方配置摘要用于追溯，不替代 Runner 的 input_digest、资源身份或版本绑定。

## 4. 能力声明与直接使用入口

接口分为能力描述、纯配置校验、执行／控制和实际环境检查四类。前两类不进入执行受理，后两类沿既有持久化与资源核验路径；不要求先运行 validate 或持有历史检查报告才能 submit。

### 4.1 能力目录

能力按 `adapter + target.kind + agent + workflow + operation/access_mode` 的支持组合表达，不能把维度任意做笛卡尔积。复用既有 catalog 与生产支持 gate，发布 schema 引用、必要条件、原生版本限制及已实现操作；实施时补齐当前可表达的组合，不能把任意 Orca agent 字符串被接受解释为可靠链路已实现。

目录只声明所选 Runner 构建的实现能力，不扫描机器来添加／删除支持项。某机器缺登录、程序或连接是现场不就绪；缺相应原生操作实现才是不支持。普通执行支持不自动推导 environment-check、审计或停止能力支持。历史测试通过单独绑定构建和配置，不成为现场就绪证明。

在 capabilities.data 中增量增加 `capability_catalog`，其 `version:1` 表示目录结构版本，`combinations` 是有限的类型化条目。每条给出 adapter、target_kind、agent、workflow（不适用时为 null）、access_mode、支持的 operations、公开 schema 引用及必要条件。operations 使用已公开命令标识，如 `submit`、`reply`、`resume`、`stop`、`environment-check start`；条件限制须落实到具体组合，不把某模式的支持泛化到其他模式。必要条件只表达实现约束、必需字段及依赖版本，不列用户机器、账号、模型资产或动态就绪值，也不承载可执行表达式。

workflow 为已公开的 `{id,version}` 或 null；null 仅表示该条目服务于无 workflow 的操作，不是任意 workflow 通配。adapter 已决定的隐式 agent（本机 Codex）及契约允许缺省的 agent／access_mode，用生产既有默认规则建立只读匹配视图；不修改原输入、不查现场，也不把非法显式值当成缺省。operations 表示该链路的命令处理已有实现，不表示当前状态允许调用或必然完成；例如 stop 的授权撤销、原生停止与写者退出证据仍分别按既有契约判断，已知的原生限制须随该条目说明。

目录从现有 catalog／生产支持判定的共同描述生成；必要的专用语义仍调用原 gate 并做一致性测试，不再手工维护一张第二支持矩阵。列为支持只表示有实现，某次恢复／停止是否可核实继续取决于原资源事实。调用方不能由各维度列表自行推导未声明的组合；schema 接受通用 agent 标识也不构成可靠链路支持声明。

目录不是额外的生产准入白名单。`validate.valid=true` 只表示相应生产纯 gate 接受该值；目录未列出的扩展标识可能仍满足输入契约，但不能宣称链路已受支持或现场就绪。反之，目录列为支持的组合仍须满足其参数／schema／语义约束。validate 不因目录缺项额外拒绝原 gate 接受的输入；原 gate 明确返回 unsupported_capability／unsupported_schema 时继续拒绝。测试只选择已声明的链路；目录缺项记 `undeclared_capability`，与 gate 已明确判定不支持分别表达。

同一 capability_catalog 内增加固定的 `validation_kinds` 映射以发现本期三个校验入口；键为 kind，值为相对 capabilities.data 的 schema JSON Pointer：`run-configuration` → `/configuration_schemas/run_configuration`，`run` → `/configuration_schemas/run_input`，`environment-check` → `/environment_check_schema/$defs/input`。既有 environment_check_schema 是含 $defs 的整份文档，不能把文档根误当成 input schema；使用子 schema 时保留原文档的本地 $ref 解析上下文。对应 schema 均须随同一响应可取得；其中 run_input 为本期新增完整 Run input schema。顶层 commands 保留旧列表并增量列出 validate；不能仅凭该命令名推断所有 kind。该映射不增加运行时注册／插件／任意 schema 执行机制。

增量增加 `capability_source`，以 `kind:executable|service` 区分来源，并包含实际程序版本、CLI 协议、EXE SHA 和 resource_digest。离线值来自当前被调用的 EXE；在线值必须由实际响应的服务附带其 instance_id 与固定构建身份，不能由 CLI 先查 status 再拼接而产生实例竞态。同版两种查询的静态目录／schema 一致，来源对象不同；离线结果不能证明当前服务就绪或与该 EXE 相同。

### 4.2 增量接口

| 入口 | 输入与输出 | 副作用及错误 |
| --- | --- | --- |
| `capabilities --offline --json` | 当前 EXE 的公开目录、schema、版本及 capability_source；静态部分与同版在线声明同源 | 不启动／连接服务，不读测试或外部配置，不探测工具；`--offline` 与 state-dir、实例绑定等在线选项组合拒绝 |
| `validate --kind run-configuration --input - --json` | stdin 直接为 `{execution, workflow}`，调用配置片段纯校验 | 不要求协议／request_id／input 写信封，不加载命名 profile |
| `validate --kind run --input - --json` | stdin 直接为完整 Run input，调用完整 Run 纯校验 | 不受理 Run、不取得材料、不查询程序路径、不创建目录／回执 |
| `validate --kind environment-check --input - --json` | stdin 直接为完整 Check input，调用检查 input 纯校验 | 不要求 workflow，不执行实际环境检查，不创建 Check |

三个 validate kind 只接受对应的原始对象，不自动解包 `{protocol, request_id, input}`；误传写信封按未知／缺失字段拒绝，不启用两种输入格式猜测。沿既有 UTF-8、8 MiB、唯一字段、单 JSON 对象及严格字段规则读取；不填入材料内容、解析凭据或把原生默认伪写成确定结果。CLI 的协议选择仍沿全局协议参数，stdin 不重复包裹协议。

`--state-dir`、实例绑定及 Run 选择器不适用于纯校验。响应继续使用现有 v1 Response，request_id 始终 null，不建立幂等记录且不能通过 inspect 查到校验。真正 submit／environment-check start 的写信封、request_id、去重和受理行为保持原样，并再次校验完整输入；纯 validate 成功不替代写信封合法性或现场核验。

不新增自动 profile 合并或模板执行接口。独立用户直接维护完整请求文件，通过既有 stdin 提交；SteerLine 和测试框架各自装配同形状请求。纯校验不保证随后现场仍可用。实际 Agent、push／PR 探针继续使用显式 environment-check，不进入能力查询或纯校验。

### 4.3 校验结果与字段诊断

校验结果 data 包含 `kind`、`valid`、`issues`、当前程序 `version` 和 `resource_digest`。只有本次校验有确定输入判定时才返回该对象；不能把工具本身未能完成校验记成 valid=false。所有 validate 响应的 request_id 均为 null，按下表处理：

| 情况 | 响应与退出 |
| --- | --- |
| 合法对象且对应生产纯 gate 通过 | ok=true、valid=true、issues=[]、error=null、exit 0 |
| 已知 kind 的输入超限／非法 UTF-8／JSON／重复键／非对象，或 gate 判定非法 | ok=false、valid=false、issues 非空；读取层格式错误用 invalid_input／exit 2，gate 错误沿原 code／退出码 2 或 3 |
| 未知／缺失 kind、非法 CLI 参数、误用在线参数 | ok=false、data=null、invalid_arguments、exit 2；全局协议不支持仍为 unsupported_protocol／exit 3 |
| stdin I/O 故障、无法取得内部契约或校验器内部故障 | ok=false、data=null、internal_error、exit 1；不形成“配置无效”结论 |

仅新增 validate 的输入错误响应允许 data 携带诊断，不改变既有命令错误时的数据约定。无法写出 stdout 时沿现有调用失败规则，不保证还能输出 JSON 错误。原纯 gate 的最终判定、fault code 和优先级决定 valid／exit；schema 及附加 issues 不能抢先把原来的 unsupported_schema（exit 3）改成通用 invalid_input（exit 2）。多条 issues 只用于定位，不由它们重新推导主 fault。

每个 issue 为 `{path, code, message}`：path 使用相对本次校验对象的 JSON Pointer，根级／无法精确定位时为 `""`；code 使用稳定分类，例如 required、invalid_type、invalid_value、unknown_field、invalid_json、unsupported_capability、unsupported_schema、inconsistent_fields；message 用于解释，不作为程序分支依据。可以返回多条已确定的独立问题，不承诺一次穷尽所有后续错误，不在说明中回显凭据或整份原输入。

issue 示例（不是完整响应；该路径相对 RunConfiguration，同一字段在完整 Run input 中路径相同）：

```json
{
  "path": "/execution/settings/model",
  "code": "required",
  "message": "所选 workflow 要求明确指定执行模型。"
}
```

结构化定位应由共用校验产生或由其可靠映射得到，不为 CLI／测试框架各写一套近似语义判定。暂时无法定位的跨字段错误使用根 path，保留真实 code，不猜测具体字段。旧合法请求的 gate 判定与默认值不得为了生成诊断而改变。

### 4.4 版本与调用方配置边界

| 标识 | 权威与语义 |
| --- | --- |
| `steerrunner-cli/v1` | Runner 的调用、信封和既有命令语义；本次采用增量扩展，不整体切换 v2 |
| schema 标识／版本、`capability_catalog.version` | Runner 定义的输入形状及能力目录结构；调用方须按声明解释，不从显示名称猜测支持 |
| 程序 version、EXE SHA、resource_digest、在线 instance_id | 实际实现／资源及服务实例身份；资源摘要变化不等于 CLI 协议不兼容，旧冻结 Run 的摘要也不被新值覆盖 |
| 调用方 profile 版本／配置摘要 | SteerLine 或独立调用方管理，提交前展开；Runner 不据此解析或选择 profile |
| `steerrunner-test-config/v1` | 本仓测试外层格式，仅由测试框架消费，不进入生产配置版本协商 |

resource_digest 覆盖静态生产资源、schema、默认规则与公开能力目录（含 validation_kinds）；生成时不包含 capability_source 的实例身份、EXE SHA、计算中的摘要本身等运行来源字段，避免自引用。测试配置、case 注册、测试辅助路径及检查时间不属于生产资源摘要。目录生成与摘要计算复用确定的静态内容及稳定序列化顺序，不能出现公开目录已变而摘要保持原值，也不能因实例／测试现场变化而改变摘要。

现有 capabilities 字段、类型和旧 schema 键保留；新目录、来源和命令通过新增字段／条目发布，不能用新目录替换原 adapters／workflows／configuration_schemas。调用方采用新入口前做能力发现；只使用旧执行接口的调用方无需采用新内部测试格式或纯校验步骤。若已有消费者拒绝新增响应字段，应在交接中明确并做消费兼容验证，不能仅凭 CLI 仍为 v1 宣称所有客户端零改动。

新增目录、schema 及诊断内容采用固定字段，既有响应字段／schema 键不改名、不改类型。已知现有消费者的兼容样例必须通过 IC-10；若拒绝新增响应字段，需要在该调用方仓库完成明确适配并登记交接，不能把未通过的消费者宣布为兼容。此处不引入新旧双重默认 payload、自动协商降级或第二版执行协议。

## 5. 内部独立测试配置

### 5.1 文件归属与结构

使用独立版本 `steerrunner-test-config/v1`，由测试框架通过显式 `-Config <path>` 读取。仓库提交示例与测试外层 schema；真实机器文件保存在忽略目录（例如 `.evidence/`），不写入发布 EXE，不被生产服务自动读取，也不要求 SteerLine 提供。Config 相对路径按调用入口 cwd 转为绝对路径；文件内允许相对的测试辅助路径按原配置文件所在目录解析。本地 Codex 基线路径须显式绝对，其余执行字段沿公开契约及 case 明确约束。

| 外层字段 | 拟定形状与规则 |
| --- | --- |
| `schema` | 固定 `steerrunner-test-config/v1`；未知版本拒绝 |
| `configurations` | 测试配置项 ID 到 `{execution, workflow?}` 的映射。ID 由唯一 case 注册引用，例如 `local-codex`、`local-codex-audited`、`orca-local-pi`；execution 必需，workflow 对 Run 必需、检查不消费。不支持继承、表达式或自动 fallback |
| `fixture` | 一份 `{repository, publication?}`，字段使用完整公开对象，声明本批测试源。源 commit／分支只读；真正测试写入使用每 case 独立命名空间。当前专属 GitHub 现场要求 command publication，其他合法产品 provider 不自动替换。缺字段／不符 case 要求仅影响依赖它的 case |
| `harness` | 仅测试设置：admin_host 故障宿主；字段由严格测试 schema 限定，不能携带自由 shell 或覆写生产参数。证据目录沿既有 -OutDir 管理，不重复配置 |

缺失 configurations 视为空；fixture／harness 可缺省以支持诊断。根 JSON 畸形、重复键、未知外层字段／版本、外层容器类型错误是整文件错误；可解析容器内单项配置／夹具无效，标记依赖它的 case，不隐藏其他独立配置的诊断。解析按“外层严格结构 → 单项公开字段 → case 约束”分层，不能先用整文件 schema 把单项错误变成全局拒绝。显式空配置不等于未配置；错误配置不能回落到其他项。

基线固定读取 `configurations.local-codex`，不再另设选择指针。case 注册直接引用所需配置项及角色，用户只填写这些项的实际值，无需维护逐 case 绑定表。当前测试继续使用既有专属测试仓库及隔离规则；本期不扩展为多项目 fixture 管理器或开放任意业务仓库发布。需要多 Run／多个工作副本的 case 沿原生成器隔离创建。

测试源仓库／publication 的身份须匹配；要求发布的 case 缺 publication 时不可用，不能改为不检查 PR。Orca 注册项目入口来自 fixture.repository.local_path，须保留并核验；本机 caller clone 等工作副本路径由原生成器分配，不能混为注册项目或猜测远端环境。准备现场时允许框架按已声明算法分配本次专属 base/head、固定源 commit、工作副本 local_path 和测试材料；原配置、生成值及来源写入 manifest，并重新走完整生产校验。配置未授权的仓库、agent、模式不能被生成器补上。固定材料后失败的重试不得重新装配旧 request_id。

既有 case 的 provider/query 取证、故障注入 helper 如需现场路径，仍由该 case 的受控生成器构造并记录具体变更；不要求用户预填尚未生成的 helper 路径或伪造完整请求。最终 Publication 必须符合公开契约及原仓库／分支归属规则，不能把测试 helper 的成功冒充真实发布核验，L2 的真实 Git／PR 断言保持不变。

场景要求 full_access、审计、特定 agent 或混合配置时，须由注册需求引用符合要求的配置项；不能继续以场景名称暗改 access_mode、继承另一个目标的模型，或从公共 model 字段猜测远端选择。场景只生成其应负责的任务、引用与故障现场。

协调器一次读取并保存本批非秘密配置快照，按原文件基准处理辅助路径；worker／fixture 生成器均消费该快照，不重读可变源文件。每 case 的最终请求和生成值再分别冻结；批内编辑源文件只影响下一批。

批次同时固定所选 catalog、Runner／测试 helper 及需求摘要。helper 内嵌需求必须匹配当前唯一注册；worker 的文件预检不能替代生成器对实际解析所得配置／构建／需求身份的核对，发现漂移在现场准备前失败。真机生成器要求的本机 Codex、Orca transport 及本机 runtime helper 的显式绝对路径属于测试约束；静态阶段只检查引用形状，不访问工具或账号。

### 5.2 本地 Codex 基线

交付合格必须同时满足：

1. `configurations.local-codex` 存在，对应 `codex-app-server + local + development/2`，显式 `access_mode:sandbox`，`execution.settings.executable` 为实际本机原生 Codex 文件的绝对路径；交付基线不接受空值、PATH 命令、npm shim 或 shell 包装器。
2. 按生产原生资格规则核验格式、版本、实际权限、认证及所需发布依赖。最低版本为 Codex `0.159.2`，更新版本共用生产语义版本下限检查；历史调查证据仅证明其记录的具体版本。
3. `local-codex.smoke/default` 的注册需求固定引用该配置项；L1、完整 Controlled 前置和 L2 真实闭环在明确构建下通过，原写者停止及资源收尾满足既有要求。driver 的 merge 属于测试断言，不新增 Runner 自动合并职能。

本地路径是配置要求；路径存在／`--version` 成功只是前置，不等于闭环通过。模型允许沿公开缺省，但资格证据必须保留请求及可取得的实际模型，不能声称验证了任意模型。构建／配置变化按既有资格规则重新核验，历史通过不自动继承。

L1、Controlled 和无真机依赖的 L4 仍可在未配置本地 Codex 的开发环境运行。基线缺失／无效会令交付资格不成立，不全局阻断不依赖它的 L3；外部请求不读取本配置，也不要求提交这份内部文件。所选链路本身依赖本机 Codex 权限工具时仍按该链路核验。

基线“已配置”和“本次构建 L2 已通过”分别报告。只运行 L3 的批次不伪造 L2 已验；复用证据沿已有构建／配置资格规则，不新建资格数据库。完整 L3 测试结论与项目交付结论分别计算：即使 L3 全部通过，缺少合格本地 L2 证据仍不得声明交付基线成立。

## 6. L3 需求与可用性

### 6.1 需求权威与计算

case 的配置项引用、角色、组合、模式、模型约束、夹具及故障设施要求归 `internal/testkit/livetest/cases.json` 的测试需求扩展维护。需求限于少量类型明确的字段／已知 case 断言，不新增通用表达式求值器。`tests/catalog.json.scenarios` 仅列所选场景 ID，由共享读取函数从唯一注册展开层级、driver、target、agent、超时和完整 case 集，不手写元数据镜像。已有自定义 catalog 的完整对象格式继续接受并核对元数据；批次冻结展开后的完整对象，worker 不重新解释 ID 选择。完整层级仍拒绝缺少注册场景。配置文件只填写环境值，不能修改 case 预期、删掉注册 case 或声明自己已通过。

| 场景族 | 最小需求示意；具体以每个已注册 case 为准 |
| --- | --- |
| 本机反馈／恢复／停止 | 指定本地 Codex 配置及隔离夹具 |
| 两模型审计 | 本地审计 workflow，显式实施／审计模型且不同 |
| Orca 本机 Codex／Pi／DSH | 对应 local/agent 配置、helper、注册项目及匹配工具 |
| Orca 远端及可靠性 | orca_environment、明确远端配置；故障 case 额外要求其独有设施 |
| 两种权限环境检查 | 对应检查支持组合及显式匹配模式；普通 workflow 支持不能代替检查支持 |
| 混合多 Run | 两个不同角色配置、同服务隔离要求及相应夹具 |

本规格的 L3 可用性专指“配置是否足以选择并尝试此 case”。只加载所选 EXE 的静态能力声明、配置、schema 与 case 需求，不探测真实工具／账号／网络，不调用模型或创建现场。程序路径存在性、原生版本、登录、权限及远端连接仍由实际测试的既有准备阶段核验；不额外建立一轮全链路预检。

能力查询须有明确的待测 Runner 产物，沿既有构建流程预先生成；独立列表缺少该产物时按查询失败处理，不悄悄构建或猜测支持范围。实际派发使用同一产物，逐 case 核对 EXE／资源摘要；禁止用旧构建目录计算可用性后实际执行另一构建。L1／Controlled 组件验证继续可直接调用同仓生产 gate，无需把全部单元测试改为 CLI 子进程。

### 6.2 状态

每个 case 返回 `scenario_id`、`case_id`、需求与配置引用，以及一个配置可用性字段：

| 字段 | 值与含义 |
| --- | --- |
| `availability` | `available`：所需配置片段合法、测试约束与静态支持组合匹配；`unavailable`：缺少或错误配置，或所选构建不支持所需组合。available 不表示环境已就绪或测试通过 |
| `reasons` | 不可用的全部已知原因，固定 code 如 `missing_configuration`、`invalid_configuration`、`unsupported_capability`、`undeclared_capability`、`missing_fixture`、`missing_test_facility`；含字段引用及无秘密说明；undeclared 专指所需链路未在目录声明，不冒充 gate 拒绝或现场探测结论 |

列表返回全部缺口，不用首个错误掩盖混合场景的其他需求。记录计算时间、所选 EXE SHA／resource_digest、配置与需求摘要；只使用同批快照。实际环境失败、模型不可用、401、连接未知等一旦在执行准备／测试中出现，沿现有测试失败及业务诊断记录，不能倒改配置可用性掩盖失败。此处不增加环境就绪缓存、资格数据库或后台监控。

### 6.3 测试入口与选择规则

沿现有 `scripts/test.ps1` 增加 `-ListAvailability` 静态模式，保留 `-Level`、`-Scenario`、`-Config`、`-OutDir` 和既有显式管理员排除选项。`-RunnerExe`／`-ConfigHelper` 仅用于选择既有待测产物，不增加运行模式：

- `-ListAvailability` 仅支持 L2／L3 级别组合，只生成清单，不跑 L1／Controlled、模型或任何现场准备；有明确待测构建而缺配置文件时列出 missing_configuration，合法查询完成 exit 0，即使存在不可用 case。畸形文件／未知版本等整文件错误、非法选择或查询本身失败 exit 2。输出可选写入显式 OutDir，但不创建测试现场、服务或调度锁。基线配置完整性另列，不能标记 L2 已通过。普通 L1／Controlled／受控 L4 不读取或强制要求本文件。
- 默认 `-Level L3` 的请求范围为全部注册 L3 case，先静态评估选中集合。不可用项明确列出且不派发，其他相互独立的 available 项仍可按原前置和隔离规则执行；requested 范围保持不变，批次不能自动缩成已运行集合。
- 显式 `-Scenario` 的请求范围为该场景全部注册 case，同样逐项处理。任一未显式排除的所选项 unavailable，无执行失败时最终 incomplete、有失败时保留 failed，均非零退出；某一场景全不可用时该场景零派发。无关场景和非依赖基线的 L3 不因此全局阻断。
- 用户可通过现有 Scenario 选择配置齐备的范围；不增加自动选择可用子集的模式。原 `-SkipAdminRequired` 保留，明确记录其排除项与范围，不能将缺普通配置伪装为管理员排除。
- 真实执行仍须通过原 L1／完整 Controlled 前置，再为该 case 做必要环境准备。前置失败保留失败并阻止所依赖执行；不新增必须先跑 L2 或持有历史环境检查报告的 L3 门槛。
- L2 的原生 Git 边界检查由 L2 case 准备承担；失败记该 case failed，保持证据，并继续调度独立 L3，不把本机基线现场故障升级为所有真机 case 的公共前置。

实际运行中的前置失败、超时、原生执行或断言失败，保留 failed／blocked／unknown 等实际结果；不能事后改成“未配置”或排除来挽救批次。中途失败仍按原资源身份停止和收尾。根文件损坏、公共 L1／Controlled 失败或无法确保隔离时可全局停止；单个配置不足本身不是批次事务失败。

### 6.4 结果与交付声明

沿既有批次报告增加每 case 的配置可用性、原因及按级别独立的 `complete_level_passed`，复用原测试 outcome、业务 outcome、cleanup 和 scope，不另建平行执行状态机。分别保存注册、请求、显式排除、不可用、实际执行和通过的 case ID 集合；计数从这些集合及已有执行记录派生，repeat／尝试次数另计。报告基线配置完整性与可核验的 L2 证据引用，不把列表计算变成资格授予。

前置失败和调度中途失败仍保留全部请求记录：未派发的可用项沿既有 queued 表示待执行，不计入 executed；原 unavailable／excluded 及原因保持不变。共享前置失败记为批次 failed，不能伪造尚未派发 case 的执行失败或将缺配置事实丢失。

完整 L3 通过要求同一选定构建及冻结配置下，全部注册 L3 case／所要求 repeat 实际通过且收尾满足原规则，没有 unavailable／excluded／未执行项；`complete_level_passed` 从实际注册覆盖计算，不仅看命令是否带 Scenario。明确选择的子集全部通过或仅有用户允许的管理员排除时，沿原规则可以 exit 0，但非完整覆盖时该字段必须为 false。

退出判定优先级：非法参数／整文件错误 exit 2；前置或实际测试失败 exit 1（沿现有 failed），即使同时有未配置项也保留两类事实；无执行失败但所选未排除项 unavailable／未执行时 incomplete、exit 1；请求真机级别却零 case 实际执行时 exit 1；其余沿原通过／显式排除规则。L1／Controlled 单独运行不受“零真机 case”规则影响。

基线合格但可选链路未配置，可只声明本机基线交付及已验证范围；本规格不取消任何既有版本明确承诺的必验链路。L3 测试结论不能代替 §5.2 的项目交付基线。

## 7. 兼容、证据与验收

本期保持生产 CLI v1 的既有请求形状、默认值、存储格式及恢复事实，新增静态入口／声明采用明确能力发现并更新资源摘要。新 CLI 不强行接管旧服务；沿既有构建身份检查切换服务，并刷新实例绑定及能力缓存，原 unknown／冻结 Run 不重渲染或自动重发。内部测试配置新版本与旧 live-config/v1|v2 不隐式混读；实施时一次性更新本仓示例、脚本、框架断言及文档。旧文件明确报迁移提示，不自动补字段／默认选择；旧现场按原 manifest／构建结算，不用新文件重建旧授权。

实施验收项如下；执行记录写入 [验收登记](../acceptance/independent-configuration.md)及 Plan，真实现场另保留原报告，本表只定义判据。

| ID | 必须证明的行为 | 级别 |
| --- | --- | --- |
| IC-01 | schema／目录与生产语义共用来源；目录声明、gate 接受与现场就绪不混淆；缺省匹配／workflow null／操作限制及内外同值同判明确，不增目录白名单收紧旧输入 | L1 |
| IC-02 | 离线能力与三个可发现 kind 的原始对象纯校验零服务／目录／外部动作；schema 引用可解，来源同版一致；无 request_id；输入／I/O／内部错误及主 fault 优先级正确 | L1 + Controlled |
| IC-03 | 独立 CLI、测试组装和外部完整请求共用契约；测试文件变化不影响普通 submit，原重试／恢复不读新配置 | Controlled |
| IC-04 | 固定 local-codex 基线缺失、相对／包装器／错版本路径不能合格；L1／Controlled 可独立运行；L3 通过不能伪造 L2 基线，不误加远端统一依赖 | L1 + 定向原生前置 |
| IC-05 | case 需求覆盖全部注册项；无用户逐 case 绑定副本；检查不强带 workflow；混合／审计／模式精确匹配，原因区分缺配置／错误／不支持，available 不代表环境就绪 | L1 + 框架受控测试 |
| IC-06 | 不可用 case 零派发且不阻断独立已配置项；请求范围不收缩、子集／显式排除／空集／失败优先级及完整覆盖判断正确 | 框架受控测试 |
| IC-07 | manifest 固定真实请求、配置／构建／需求摘要；原停止、幂等和未知结果对账断言保持 | Controlled + 复用 L3 |
| IC-08 | 使用新配置完成本地 Codex L2，生产提示词、真实发布与收尾证据满足原 smoke 要求 | L2 真机 |
| IC-09 | 已配置且纳入本次支持声明的可选链路，以新配置完成相关既有 L3；缺配置明确不可用／未验，完整 L3 仅按实际全量结果声明 | 按声明选定的 L3 真机 |
| IC-10 | 旧合法／非法请求及默认值保持原 gate 判定、主错误及退出码；schema 对齐不偷偷收紧旧输入；旧能力字段／键保留，新声明增量发布且已知消费样例通过；原幂等／实例绑定不变；生产摘要不含测试／动态来源 | L1 + Controlled；调用方消费另验 |

SteerLine 配置存储、UI、请求保存及 TaskDelivery 接入测试属于其仓库；本仓仅给出公共字段／版本、样例和消费要求，不在本 Plan 中修改或代记 SteerLine 验收。本轮实施和专属测试仓库验收已获用户授权。
