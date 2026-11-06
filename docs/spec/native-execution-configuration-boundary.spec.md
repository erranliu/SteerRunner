# 原生执行配置边界收口 Spec（Runner）

日期：2026-11-05，状态更新：2026-11-06。状态：实现、完整 L1/Controlled 及本轮同版原生分项已验收，调用方远端正向资格已补验通过；范围见专属验收，进入已授权的提交/推送/PR交付，尚未发布。基线 `a66038c89208392c47531e640c84e97728c62424`，独立 worktree 分支 `codex/native-execution-config-docs`。对应 [Plan](native-execution-configuration-boundary.plan.md)和[验收记录](../acceptance/native-execution-configuration-boundary.md)。不合并或部署，不切换安装现场。

配套调用方为 [SteerLine](https://github.com/erranliu/SteerLine) 的 `SteerlineFlow/spec/native-execution-configuration-boundary.spec.md`、同名 Plan 与专属验收；调查基线 `9b2e5d886b8f56eaf07bc697ca78c578c91eec00`。新增公共协议以本 Spec 与同版 schema 为 Runner 权威，调用方业务规则以其 Spec 为准；共享正文已独立审核签收，两仓实现和文档尚未提交。

## 1. 目标、调查与范围

SteerLine 调查基线在环境检测 useNativeOrcaExecutor、GenerationExecution 和 Web 编辑中解释原生字段；环境模型通过裁剪 model/effort 计算资格上下文。Runner 已有 [公开配置及能力目录](independent-configuration.spec.md)、[纯校验](../cli-guide.md)和[原生目录入口](native-catalog-boundary.spec.md)，本轮以有限纯配置构造/投影接口替代上述转换。

新增一个无服务短命 CLI，提供 describe/derive/compose，按两个阶段交付。复用当前生产配置语义，保持原 submit、输入、Workflow、权限、结果、停止和恢复 gate。它不是 profile 管理器、原生探测器、配置服务或通用转换 DSL。

本机核对 `internal/contracts/run.go`、`internal/resources/configuration.go`、`internal/cli/cli.go`、`internal/nativecatalog`、配置/元数据测试及既有验收。当前固定基线已拒绝 Orca 退役显式 executable/非空 args/runtime/helper；合法 native 源的 execute 派生通常原样保持。不得为迁移调用方旧规则而恢复这些字段，或建立没有当前消费者的 helper 检查/程序授权摘要协议。

## 2. 两仓职责及不变量

| Owner | 职责 |
| --- | --- |
| Runner | 原生 settings 结构、固定默认、合法组合、有限用途派生、通用配置身份、配置构造和原生目录对配置程序的解释 |
| SteerLine | profile/默认选择/产品支持范围、配置 CRUD 与条件保存、明确权限和 workflow 选择、可信部署来源、项目资格/key/basis、原请求与业务事务 |
| Runner 执行 owner | 原资源/Run/会话/Workflow、原生准备/输入/结果/停止；新配置接口不接管或重建它们 |
| 原生外部系统 | 已有登录、机器、Agent/模型资产与其当前可用性；纯配置结果不证明这些事实 |

不维护 SteerLine profile/项目/数据库，不解析其 execution.json，不返回项目 ready。不提供任意程序字段给浏览器：部署方仍可通过原公开完整 execution 供给程序，但逻辑编辑只从可信 base/default 构造。SteerLine 的 `profile_edits`、保存摘要、审计模型草稿建议和 SQLite 资格事务是调用方协议，不进入 Runner 生产类型。

capability_catalog、schema、validate、native-catalog 各自仍是原静态支持、输入校验及动态发现入口；不从维度列表推未声明组合。发现可用不意味着可执行。Runner EXE 构建 pin 和可信部署资产自身完整性继续核验；新增接口不维护安装器或程序注册库。

## 3. Runner 新增的公开配置接口（提案）

### 3.1 入口、能力与无副作用要求

新增固定短命命令 `steerrunner execution-config --input - --json`，沿 `steerrunner-cli/v1` Response。stdin 为单个原始 JSON 对象，不是写信封；request_id=null。仅允许 --input、--json、--protocol，拒绝 state-dir、实例/完整绑定、Run/request 选择器及 --offline。在服务发现和 state-dir 规范化之前处理。

离线/在线 capabilities 同源发布 `execution_configuration`，包含 contract=`steerrunner-execution-configuration/v1`、command、operations、request_schema、response_schema、limits。operations 首版为 describe、derive、compose；schema 自包含。derive 的源/目标组合与可编辑逻辑字段从同一生产配置描述生成，支持条目引用既有 capability_catalog 的组合，不维护第二套手工笛卡尔积。

输入与完整输出分别最多 1 MiB，严格 UTF-8、唯一键、单对象、深度最多 64、未知字段与显式非法 null 拒绝；总调用预算 30 秒。操作只读输入与本构建内置资源，允许核验自身 EXE 身份；不读 profile/登录/原生配置，不检查输入中的程序文件，不连接原生端，不启动服务/Agent，不创建目录、Run、回执或 PR。调用方拥有该短命 CLI 的取消与回收。

成功 data 固定携带 contract、operation、input_digest、source；source 为实际 version、executable_sha256、resource_digest。input_digest 为完整输入对象按两仓一致的 CanonicalJSON 规则取 UTF-8 字节 SHA256。对应操作结果见下表，响应与退出码必须一致。

| operation | 请求的业务字段 | 结果与语义 |
| --- | --- | --- |
| describe | execution；workflow 为仅含 id/version 的公开选择器 | 返回 view、identity。只投影该 execution，不构造可提交的替代配置。 |
| derive | execution；workflow 为目标 id/version；access_mode 为调用方明确要求值 | 返回派生后的 execution 和其 view，以及原 source_identity。仅允许已公开的用途转换；不改原输入对象。 |
| compose | 可选 base_execution；完整目标 workflow；selection；unset | 返回完整 execution、view、identity。构造后校验完整 RunConfiguration，不修改输入 workflow。 |

view 只包含公开逻辑值 adapter、target、agent，以及原配置显式存在时的 model、reasoning_effort、access_mode；隐式 Agent 按公开确定语义投影。模型/effort 未固定时省略，不把静态或原生默认填成显式选择，不声称已核实实际执行模型。配置中的命令、参数、helper 路径及凭据引用不放入浏览器的逻辑编辑 DTO。

execution_configuration.reasoning_efforts 发布按 Agent 的有限逻辑 effort 选择，与生产原生配置 gate 复用同一取值定义；调用方不得把 Codex 取值应用于其它 Agent。

selection 是有限结构：adapter、target、agent、model、reasoning_effort、access_mode；前三项确定执行组合，后几项缺省时按 base 的可继承值及公开固定默认解释。unset 仅允许 model/reasoning_effort，不能与同名显式设置并用；null/空串不充当删除操作。程序路径、args、runtime、token、任意 JSON pointer/patch 均无输入字段。

权限是上述缺省规则的例外：compose 的 access_mode 必须显式提供并满足目标 workflow；derive 同样必须显式要求目标权限。不能从源方案、workflow 名称或旧缺省推导一次新的权限授权。describe 只报告已有配置，不补写权限。

compose 有 base 时仅继承匹配 adapter、完整 target（含 environment）、agent 的合法程序设置；跨组合必须由可信调用方选另一匹配 base，或明确从该组合公开默认构造。不能通过换 target.environment、Agent 或 workflow 偷渡不适用程序字段。参数缺项/不适用时返回错误，不能把远端换本机或选择其它模型。model/effort 等可继承性由 Runner 解释，所有未公开字段拒绝。

程序授权通过可信 saved base/default、重新 compose 和完整规范字节比较完成，不再增加 program_binding_digest、integrity_checks 或原生程序授权票据。Runner 调查基线的 contracts/run.go 与公开 configuration schema 已拒绝 Orca 的退役显式 launch/helper 字段，不为它们建设新的部署检查协议。Runner EXE pin 及可信部署资产自己的文件完整性核验继续归原 owner，不能与退役 helper 协议混为一体。

非法 JSON/参数/配置为 exit 2；未实现操作、组合或用途转换为 unsupported_capability/exit 3；输入读取及全部处理共享 30 秒期限，超时为 execution_configuration_timeout/exit 4；内部故障 exit 1。失败沿 v1 error、data=null，不返回部分可用 execution。CLI 无响应/超时由调用方记 unavailable，不改走旧字段转换。错误仅含安全字段路径/代码和固定说明，不回显整个 settings、命令、参数或凭据。

### 3.2 derive 的首版范围

| 消费方与目标 | Runner 规则 | SteerLine 继续负责 |
| --- | --- | --- |
| 环境检测 → execute/1 | 同一 adapter/target/Agent/模型偏好；当前合法 native Orca 源通常原样保留 execution，核对目标 workflow/权限适用性。退役显式 launch/helper/非空业务 args 源直接拒绝，不通过删除字段清洗。权限保持原 full_access 且须与显式请求一致。 | 项目脚本/材料、固定 commit、检测期限、执行正文、result-file、结果验收及资格 |
| 测试安排、Roadmap → generate/1 | 首版仅已声明的本机 Codex 源组合；复制合法 executable/模型/effort 偏好，按显式要求生成 read_only。开发 audit、publication、认证、任意开发 settings 不进入结果。 | 原生成目的、schema、材料/原文、内容验收、home 额度与记录 |

derive 校验源 execution 的当前公开结构、适用生产配置 gate 和已声明转换语义，不把旧构建退役字段洗成有效新配置。不能只用较宽的 ValidateExecution 绕过当前配置 gate 对退役原生字段的拒绝；从同一生产规则抽取所需纯判定。目标仅 id/version，不能因为 execute/1 正文尚未提供而错误要求 prompt_overrides；完整 Workflow、Run 及材料在调用方组装完成和 Runner submit 时分别按原 gate 再验。接口不得生成、删改或返回提示词正文。

describe 同样按 workflow 选择器核对 execution 适用性，不因缺少尚未组装的正文/审计参数而运行完整 Run gate；compose 接受完整 workflow，必须执行完整 RunConfiguration 校验。SteerLine 对派生目标的资格校验消费返回的 execution，不能先用目标 workflow 去拒绝尚未转换的源开发 execution。

### 3.3 通用身份投影及资格不变量

identity/source_identity 包含 contract=`steerrunner-execution-identity/v1`、source_execution_digest 和 context。digest 绑定原 execution 的规范 JSON；context 使用 ExecutionInput 形状承载 adapter、target 和不透明 settings，纯粹用于配置身份比较，不能用于构造 Run。

首版 context 排除模型/effort 偏好，保留 Agent、目标、权限及程序绑定等现有环境依据。对当前两仓共同接受的基线输入，除去除 model/effort 后无其它字段的空 agent_launch 偏好容器统一省略外，其规范字节与旧 Context 一致。此处只规范身份投影，不改原 execution；有 base 的 compose 保留隐式 Agent 的存在性与原 launch 容器，不能借模型编辑增删非偏好字段。旧活动不迁移，新格式使用新 home；不得借迁移把原本不同的执行环境合并。模型/effort 更换不使资格失效；目标、Agent、权限、程序绑定变化须改变身份或明确拒绝。缺省、显式空值及原生默认的等价关系按固定契约处理并做对照，不临时猜测。

identity 不是运行环境指纹或就绪证据：不包含机器在线状态、实际账号、安装探测、项目名或材料。环境 key 仍由 SteerLine 以冻结 context 的规范字节取 SHA256；项目材料 basis、current revision、ready 和失效继续由原项目规则决定。Runner resource_digest 仅固定投影算法来源，不直接拼入环境 key。

## 4. 原生目录配置上下文（R2）

目录查询继续必须沿用原 profile 的程序上下文。R2 为既有 native-catalog agents/models 请求新增可选 execution 字段；Web 从匹配 saved profile 的服务器配置供给，Runner 内部解析原生字段。agents 仅核对 adapter/target 与 execution 一致，继续拒绝请求中的 agent 字段，不按 execution 的所选 Agent 过滤原生 Agent 目录；models 另核对 agent 与 execution 的公开 view 一致。有 execution 时拒绝另传 executable。无 profile 的纯目标/Agent 目录继续使用原选择器，不借此回退一个失败的 profile 查询。扩展由固定 EXE 的公开 request_schema 声明并验证，缺字段能力只禁用该 profile 查询，不恢复 Web 的 settings 解析。Codex models 在 Runner 内取该 execution 对应的 executable，沿原 metadata 方式 initialize/model/list，不执行业务 args、helper 或 thread/turn。浏览器不能提供 execution/程序路径；Web 也不把原生程序路径加入逻辑 view。保持原 native-catalog 的限额、来源/目标核对、无服务与清理要求，不新增目录投影 CLI。

该扩展由 `internal/contracts/native_catalog.go`、对应 resources/schema、nativecatalog 实现及现有元数据 owner 共同交付；不改 Orca 私有协议、目录新鲜度、认证或 native-catalog 原生资源清理范围。只有 models 可带 agent；agents 列出完整原生目录，不能按配置中 Agent 预过滤。environments 仍只接受原 kind。

## 5. 阶段、交接与完成要求

| 阶段 | Runner 交付 | SteerLine 消费与边界 |
| --- | --- | --- |
| R1 → S1 | describe 的 view/identity、derive、同源严格 schema/能力/配置 gate | 环境检测、测试生成、Roadmap及自动材料预检；冻结最小源绑定context，业务纯模型/事务只比较冻结值 |
| R2 → S2 | compose、有限逻辑编辑描述、native-catalog可信execution入参 | Web逻辑草稿及无状态profile_edits保存重建；保留原程序上下文，删除旧helper gate/原生映射/样例试探 |
| 最终同版联验 | 已声明组合的公开CLI、组件/受控和实际原生资格 | 相同EXE的产品入口、配置保存、实际检测/生成与原Run收尾分别登记 |

各阶段能力只声明已实现 operations。调用方缺能力阻断对应新操作，不能回退手工原生转换；已有冻结请求观察/停止不重新配置。旧安装仍由原构建结算，本轮不授权安装清理或迁移。

Runner公共协议与来源/权限/字段规则在两仓签收后才实施。提示词正文和固定材料不属于接口输入输出；如实现影响正文/接线/资产/对应测试，提供实际正文差异、UTF-8字节数、业务理由、验证及独立审核，更新golden不代替批准。Task交付、PR合入事务、原生workflow和[Orca停止合同](../orca-execution.md)不改变。

Runner验收项 RCFG-A01–A08 及实际状态见[专属记录](../acceptance/native-execution-configuration-boundary.md)，与SteerLine的NEC-A01–A10分别签收。接口与固定构建已存在；未通过的原生或完整级别项不得推定完成。
