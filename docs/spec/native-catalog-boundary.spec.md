# 原生目录查询与调用方边界 Spec

R1–R3 已实施，审计日期为 2026-11-05，源码基线为 `e7876c0d`。实施顺序见 [Plan](native-catalog-boundary.plan.md)，实际验证范围及结果见[专属验收](../acceptance/native-catalog-boundary.md)。配套调用方文档位于 SteerLine 的 `SteerlineFlow/spec/runner-boundary-consolidation.spec.md` 和同名 Plan，核对基线为 `57e3c52e`。

## 1. 审计结论与职责

SteerLine Web 当前自行实现 Codex model/list、本机 Orca IPC、远端 Orca 配对及目录 RPC。Runner 已有独立的 [Orca 能力层](orca-capability-layer.spec.md)，连接与认证可以复用。将原生目录查询归 Runner，能让原生协议维护集中在同一项目，并供独立 CLI 消费。

| Owner | 本次职责 |
| --- | --- |
| Runner | 原生目标解析、连接认证、协议协商、目录读取、原生来源及完整性、查询资源关闭 |
| SteerLine | 产品支持组合、用户选项与配置 CRUD、项目环境资格、查询并发和 UI 展示 |
| Runner Workflow | 内置阶段、策略引用路由、执行、会话及资源结算 |
| SteerLine Task | 材料预检、候选及原请求关联、DAG、消费 PR 证据并写入唯一 TaskDelivery |

目录数据独立于执行支持矩阵。目标返回的 Agent 可被列出，即使当前 Runner 未实现该 Agent 的执行链路。例如原生目录包含 DSH 时可以展示发现事实，执行资格继续按当前公开组合判断。目录查询成功只证明本次读取，模型可调用性、账号额度和项目资格仍由各自现行入口确认。

本期交付两类原生目录读取和既有策略目录的消费约定。SteerLine 的内容请求对账重构单独实施，Runner 的 Run、停止和持久化契约继续使用现行接口。

## 2. NC-01 · 无服务的短命 CLI

新增 `steerrunner native-catalog --input - --json`，支持既有 `--protocol steerrunner-cli/v1`。它在解析参数后、任何 state-dir 规范化和服务发现之前执行。未提供 state-dir 时也不能创建默认目录、锁、DB、日志或命令回执。显式 state-dir、实例/完整绑定、Run/request 选择器、offline 及写信封参数均拒绝。stdin 是单个查询对象，request_id 为 null。

`capabilities --offline --json` 发布 `native_catalog`，包含 contract=`steerrunner-native-catalog/v1`、command=`native-catalog`、query_only=true、requires_service=false、queries、request_schema、response_schema 和 limits。queries 是已实现 `{kind,adapter,target_kind}` 的有限数组，environments 条目仅含 kind；模型目录不按当前执行支持的 Agent 枚举过滤。limits 为 `{input_bytes:65536,output_bytes:1048576,timeout_ms:30000,cleanup_timeout_ms:5000}`。schema 自包含，在线 capabilities 的静态声明同源。各阶段仅发布已实现组合。

请求对象最多 64 KiB，要求 UTF-8、单对象、无重复键/尾随 JSON，未知字段拒绝。字段约定如下。

| 字段 | 约束 |
| --- | --- |
| `kind` | `environments`、`agents` 或 `models` |
| `adapter` | agents/models 必填，`orca-terminal` 或 `codex-app-server` |
| `target` | agents/models 必填，沿公开 ExecutionTarget 的 local/orca_environment 及 environment 语义 |
| `agent` | models 必填，沿原生标识格式校验；agents 不接受该字段 |
| `executable` | 仅本机 codex-app-server 的 models 可提供，表示调用宿主已选定的原生入口，缺省使用现行平台 Codex 入口 |

environments 请求仅含 kind，只枚举本调用进程固定 Orca 用户目录中的登记项。local 拒绝非空 environment，显式空字符串沿公开 ExecutionTarget 语义接受；orca_environment 要求明确 selector。本机 Codex 仅支持 models、local、agent=codex。Codex 单一 Agent 的产品选项可由调用方按静态构建能力投影，无需启动原生查询。请求中没有文件目录、pipe、endpoint、token、任意 RPC 方法、工作区、任务及凭据正文的入口。

响应沿现有 v1 Response。成功 data 包含 contract、原 kind/adapter/target/agent（请求适用时）、state、complete、observed_at、context_unverified，以及 source。source 包含 version、executable_sha256、resource_digest，可证明时增加 native_version、runtime_id、protocol。登记枚举使用 environments，Agent 查询使用 agents，模型查询使用 models。每种数组仅在对应 kind 出现，完整空目录可以成功返回空数组。时间使用 UTC RFC3339。

环境项为 `{id,name}`。Agent 项为 `{id,directory}`，directory 仅含允许公开的目录来源。模型项为 `{id,label,is_default,efforts}`，efforts 是 `{value,label}` 数组，原生提供时增加 provider、default_effort。根级 origin、native_fetched_at、listing_in_progress 作为独立来源事实保留，未知时间省略，不能用查询接收时间覆盖原生缓存时间。模型/effort ID 不重复且非空。上下文未与具体执行会话/账号核实时，context_unverified=true。

state 为 ready 或 loading。ready 要求 complete=true 且原生完整性已核对。原生明确仍在生成目录时可返回 loading/complete=false 与当前允许显示的部分目录，消费方不能将其保存为完整目录。格式错误、身份冲突、分页重复和截断均为失败，不转换为空目录。

非法输入沿 invalid_arguments/exit 2，未实现组合沿 unsupported_capability/exit 3。原生不可达、超时及清理未核实分别使用 native_catalog_unavailable、native_catalog_timeout、native_catalog_cleanup_unverified，exit 4，data=null。内部故障沿 internal_error/exit 1。诊断通过允许字段构造，不回显完整原生错误、stderr、endpoint、token 或用户配置。

## 3. NC-02 · Orca 查询复用原能力层

本机与远端共用 [protocol/orca](../../internal/execution/adapters/protocol/orca/state_config.go) 的目标解析、配对连接、分帧、取消与关闭。新增有限的登记枚举、Agent/模型目录读取能力，自有类型只表示 Orca 事实。目录读取不导入 application、runstate、storage 或 Workflow，不调用执行 Lifecycle、创建 owner、重试队列或 Run 准入。

本机使用固定 Orca user-data 内的 runtime 元数据与本机认证，远端使用显式登记项及其既有配对信息。仅 Runner 内部读取凭据，响应和日志只保留公开字段。本机路径不加载远端配对，不继承 ORCA_ENVIRONMENT、ORCA_PAIRING_CODE、ORCA_REMOTE_PAIRING 改投目标。远端连接身份与 Orca 返回的执行主机身份分别核对。

按目标支持的协议调用 status.get、agentSession.agents 和 agentSession.modelCatalog。models 先确认 Agent 注册，保留原生协商、来源及 listingInProgress 语义。native capability 不足时明确返回 unsupported，禁止从 RPC 改投 CLI、读取 Agent auth/models 文件或调用 provider API 补结果。环境登记枚举可在 Orca 应用离线时返回已登记项，不能将枚举解释为连接成功。

整个查询预算 30 秒，所有分页、协商及支持的 listing 等待共享预算，原生协议响应累计与最终公开 JSON 响应各自上限 1 MiB。超限或预算耗尽时保留失败语义。关闭额外上限五秒，只结算本次拥有的连接和 reader，不关闭 Orca 共享进程或业务会话。静态支持声明与现行实际平台范围一致。

## 4. NC-03 · Codex 模型目录

在 NC-02 之后独立交付本机 Codex models。复用可脱离执行链的现有进程/协议代码，按元数据需要补充小型读取函数。当前 codexapp.Client 含执行模型依赖，实施时先核对依赖，避免直接借完整执行 adapter 初始化查询。

只运行固定原生入口的短命 app-server，完成 initialize/initialized 后分页 model/list，默认不含 hidden。不创建 thread/turn，不提供业务输入。核对重复游标、重复/空 ID、努力档位、JSON、累积限额与响应身份。原生默认模型和 effort 按原字段返回，不能补造默认值。

查询总期限 30 秒，输出累计 1 MiB，关闭额外上限 5 秒。正常结束、取消、父进程断开、超时及 Windows 强制结束 Runner CLI 均须清理本次拥有的 app-server 进程树、管道与 reader。复用既有进程 owner，Windows 使用所属 Job 或等价的精确所有权约束。清理未核实返回独立失败。其它 Codex 或 Orca 进程不属于该次查询资源。

## 5. NC-04 · 缓存与调用上下文

首版 CLI 每次执行一次有界查询，不建立持久目录缓存。请求宿主负责总并发上限及相同目标的在途请求合并。返回事实带 observed_at 和原生来源，调用方刷新只保证重新请求原生查询入口。

SteerLine 迁移后的对应目录采用每次新查询，不复用原先依赖私有配对信息计算的五分钟新鲜缓存。页面可以保留上次结果并标明过期/刷新失败，同一请求的在途结果可以共享。将来确需跨请求新鲜缓存时，须另有能核实账号/配对/runtime 换代的公开上下文证明。

固定 EXE、CODEX_HOME、ORCA_USER_DATA_PATH 及选定程序属于调用宿主上下文，首版由可信宿主传入子进程环境。浏览器只提交已有 profile/目标/Agent 选择，服务端从固定配置解析入口和环境。该上下文与常驻执行服务分别陈述。

## 6. NC-05 · 策略槽位消费既有公开目录

审计发现已有 `capabilities.data.model_input_contract.workflow.project_policies.slots`，内容直接来自 [PolicySlots](../../internal/execution/workflow/policies.go)。因此调用方直接消费现有目录，Runner 仅补足使用说明与契约回归。本期不再增加第二份槽位列表或新 schema 命名空间。

现有 slot/path 顺序和默认路径作为当前行为基线。SteerLine 在固定候选 commit 上逐项预检，按目录顺序显式提交 project_policy_slots，确认无适用项时提交空数组。Runner 按原 Workflow 状态的 policy_slots 路由引用。文件缺失、不可读的调用方区分继续按新建 worktree 启动边界处理。独立调用方省略 project_policy_slots 的现行行为沿原入口执行。

目录缺失或非法时，调用方仅阻断需要策略预检的新开发请求，已有请求观察、原信封对账和停止仍有各自通道。新建 worktree 成功之后无需借此新增材料/HEAD/洁净度门禁。

## 7. 实施顺序与通过条件

| 阶段 | Runner 交付 | 调用方依赖与退出条件 |
| --- | --- | --- |
| R1 | NC-01/02/04 的 Orca 三类查询及公开目录 | 固定真实 EXE 受控无服务验证通过后，SteerLine S1 接入并删除原 Orca 连接 |
| R2 | NC-03 的 Codex models | 固定真实 EXE 取消/关闭取证后，SteerLine S2 接入并删除原 model/list |
| R3 | NC-05 的现有目录使用说明及回归 | SteerLine S3 可直接基于当前已公开字段实施，与 R1/R2 独立 |

R1/R2 的公开无服务调用、双端身份和完整性、取消清理与零业务副作用均须自动验证。真实本机 Orca、远端 Orca 和 Codex 各做一次无模型目录联验，按目标分别记录资格。普通执行、workflow 阶段、发布及停止语义沿受影响现有回归核对。具体矩阵见[专属验收](../acceptance/native-catalog-boundary.md)。
