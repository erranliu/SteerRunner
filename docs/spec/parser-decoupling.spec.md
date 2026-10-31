# Spec：Parser 独立文本分析与执行链路解耦

日期：2026-10-31。状态：CLI 内存任务已实施，并完成实现审计的四项修复及本轮定向复验；此前正式 L1、定向受控证据保留，不作为修复后全量重跑的声明。设计保持内存文本任务。对应 [Plan](parser-decoupling.plan.md)；实施与验证见 [验收记录](../acceptance/parser-decoupling.md)，先前 API 调查见 [P1 历史](../acceptance/parser-decoupling-p1.md)。调查基线：`e78c0db6f4e9be6242c6928cfeae596c8358ca5f`。

用户范围：保留特定输出文本的分析能力，移除 parser 与正常执行链路的权限、工具、恢复、停止取证等耦合。此前文档轮仅编写/修订 Spec/Plan；用户后续“实施吧”已授权实现和验证，不包括安装部署或操作真实业务资源。

## 1. 目标与范围

Parser 是一次有界的文本分析：接收调用方已经取得的文本、整理要求及输出 schema，返回结构化候选结果或失败。它不执行开发工作，不查阅工作区，不决定 Workflow 后继。

目标是普通开发、原生执行、owner、恢复和停止均无需理解 parser 的进程、权限或回执。需要解析的 Workflow 只依赖文本分析接口及少量结果采纳规则。实现采用进程内独立组件和一个推理后端，不新增守护服务、RPC、插件框架、DSL、通用任务账本或调度平台。

保留当前业务范围：普通 `development/2` 不调用独立 parser；`development-audited/2` 在审计原文无法直接提取、parser 已启用时调用。完整结构已有时跳过；无效或不足时沿现有一次审计 report / 等待规则处理。不扩大到实施、发布、conversation、generate 或任意仓库调查。保留审计字段、业务完成条件、原审计来源与 PR 核验。

不在本任务内重构整个 Run Record、观察数据存储、正常执行的未知副作用规则、主执行恢复、工作区权限、宿主登录或全量测试框架。

## 2. 基线事实与删除目标

| 基线位置 | 当前耦合 | 目标 |
| --- | --- | --- |
| [application/parser.go](https://github.com/erranliu/SteerRunner/blob/e78c0db6f4e9be6242c6928cfeae596c8358ca5f/internal/application/parser.go)、[worker.go](../../internal/application/worker.go) | parser 进入普通 worker 前置推进，使用 gate、全 Run 扫描容量、原生 operation 和资源结算 | 业务获取处调用独立组件；普通执行推进无 parser 专属前置分支 |
| [backend/lifecycle.go](../../internal/execution/backend/lifecycle.go)、[reliable/operation/parser.go](https://github.com/erranliu/SteerRunner/blob/e78c0db6f4e9be6242c6928cfeae596c8358ca5f/internal/execution/reliable/operation/parser.go) | `LifecycleParseResult`、ParseInput/ParseEvidence、pending/in_flight、两份操作表示、原请求对账 | 文本请求与候选结果，不进入 `Intents`、`Snapshot.Operations` 或原生 unknown 判定 |
| [codexexec/resources.go](https://github.com/erranliu/SteerRunner/blob/e78c0db6f4e9be6242c6928cfeae596c8358ca5f/internal/execution/resultacquisition/codexexec/resources.go)、[owner.go](https://github.com/erranliu/SteerRunner/blob/e78c0db6f4e9be6242c6928cfeae596c8358ca5f/internal/execution/resultacquisition/codexexec/owner.go) | 原 owner 绑定、started/final journal、根身份提交、Job/写者缺席、恢复读取 | 一次推理调用；连接或短进程由独立后端负责取消和释放，不保留跨服务 owner |
| [codex/execution_owner.go](../../internal/execution/adapters/codex/execution_owner.go) | `RetainsParser`、OwnedParser、cancel_parser 和 parser 树结算 | 开发 owner 仅管理开发执行与其原有必要资源 |
| [application/control.go](../../internal/application/control.go)、[takeover.go](../../internal/application/takeover.go)、[backend/recovery.go](../../internal/execution/backend/recovery.go) | stop/shutdown/接管等待 parser 资源、解码冻结 package、要求 parser 恢复能力 | 普通停止及接管不依赖 parser；解析中断只作为可重试文本请求处理 |
| [application/parser.go](https://github.com/erranliu/SteerRunner/blob/e78c0db6f4e9be6242c6928cfeae596c8358ca5f/internal/application/parser.go) 的 checkParserSource | 文本分析前后检查工作区、执行资格与解析结果混合 | parser 仅处理已保存原文；原审计/工作区有效性由业务接受结果及派发后继的既有边界核验 |

用户追加：继续使用已有 Codex 登录与 CLI，不增加 API 服务或密钥。CLI 的工具禁用、输出事件检查和进程清理全部封装在独立 parser 内；CLI 本身的原生能力限制不升级为主执行权限、恢复或停止取证要求。

## 3. 职责与接口（PD-01）

接口概念为 `Analyze(ctx, {text, instruction, schema}) -> candidate_json / error`。使用普通 Go 值；parser 不解释 Run/Attempt/Session/Turn。任务与原业务源的关联由调用方在内存中保存，不要求模型返回关联身份。模型、effort、CLI 登录环境和并发上限由独立组件配置持有，不从开发 adapter/owner 取值。

| 所有者 | 负责 | 不接收或不承担 |
| --- | --- | --- |
| Workflow | 选择是否解析、分析要求、当前内置审计 schema、候选业务校验、补报/等待/后继 | 不直接调用模型，不管理连接或子进程 |
| application 获取协调 | 内存任务与原源关联、异步调用、取消、结果或失败的原子采纳 | 无持久解析任务、派发前计次或跨重启预算，不等待 parser 写者缺席 |
| 独立 parser 组件 | 简单并发限制、单次推理超时、至多一次局部重试、文本及结果 JSON/schema 校验 | 无 Workflow、application、runstate、storage、backend、reliable、开发 adapter 依赖 |
| Codex CLI 后端 | 复用已有登录，关闭工具/项目能力、检查输出并管理本次短进程 | 不接收业务工作区、Git/PR 查询、开发会话及 owner |

组件不接受整个 Record、LifecycleSnapshot、执行端口、Store、回调式业务提交权或文件路径作为分析材料。当前仅接现有内置审计 schema，不建设外部 schema 注册器。来源引用和业务结果保存在调用方；parser 的推理结果始终是候选，不能自称原生执行证据。

## 4. 文本与推理边界（PD-02）

1. 原文由调用方从已保存的审计结果取得；每个内存任务捕获本次原文、instruction、schema、模型选择，在该任务内不变化。所有分析材料均以文本传入，parser 不按文本中的路径、链接或指令读取外部内容。无需为重启重建同一推理请求而冻结额外配置。
2. 后端固定为本机 Codex CLI `exec`，复用已有 CODEX_HOME 登录；每次在独立临时目录运行，忽略项目/用户执行规则及用户功能配置，显式关闭 shell、MCP、插件、hooks、浏览和子 agent 等工具能力。出现工具或异常输出事件时终止该次解析并返回普通失败，不将候选交给 Workflow。
3. 不接收或编译开发 execution access_mode、写根、仓库凭据或权限 envelope；不做仓库权限 canary、来源调查或副作用事后取证。CLI 所需 read-only/never 参数、工具禁用、短进程树清理及必要配置检查均属 parser 私有实现，不能传回原生资源状态或阻塞主执行 stop/恢复。
4. 不读取或复制 Codex token，不登录/登出/修改宿主配置，不引入 API endpoint、API key 或新的模型服务。支持现有 executable、model/effort、codex_home_env 配置，沿用当前原生最低版本；工具禁用不是操作系统级无工具保证，能力说明如实标注。
5. 不复用原开发 thread、执行 owner 或 Orca 终端；不为每次解析创建持久 journal、根身份回执或恢复接口。真实 CLI 调用及取消在独立 parser 专项验证，不以“无原生工具 dispatcher”的强保证作为实施前置。
6. 默认保持原文最多 65,536 字节、调用方提供的完整分析材料最多 131,072 字节、输出最多 32,768 字节。完整材料按最终 stdin prompt（边界提示、instruction、格式要求、JSON 编码后的原文）加独立 schema 的 UTF-8 字节数计量，等于上限可用，超限在取得名额/启动 CLI 前失败，不静默截断；CLI 自有系统说明和传输封装不计入此材料限额。解析要求保留 unknown/partial，不从沉默推断零修复、审计通过或工作已完成。输出经现有审计契约的结构及语义检查，不相信模型自报成功。

本次后端选择已由用户确定；真实模型和版本资格仍按实际测试记录，不承诺操作系统隔离或防御恶意本机程序，也不为本模块新增安全子系统。

## 5. 内存文本任务与结果采纳（PD-03）

### 5.1 任务只驻留内存

继续使用原有 `audit.parse` 业务游标和获取结果记录。业务层提交“等待解析当前审计输出”后，在锁外启动一个内存任务；同一服务内、同一待获取源至多一个当前任务。内存句柄仅保存本次输入、原 Source/ImplementationResultID、取消函数和完成值。轮询或唤醒复用当前任务，不重复启动。

不增加持久解析槽、任务状态、parse_id/invocation_id、派发前调用计数、绝对期限或配置快照。已有游标用于识别业务仍在等待；已有获取结果用于保存被采纳的分析结果及失败后的业务去向，不承担在途任务恢复。获取历史可继续标记 `method=llm_parser` 和真实业务 Source，不制造原生 operation_id/command_id/turn。

进入等待状态的事务失败时不启动任务。业务转移成功后由获取协调发起调用；重启后仍在等待且符合调用条件时也可重新发起。无解析需求时不创建任务、不初始化后端、不检查 parser 配置或登录。无需新增后台协调服务或另一持久队列。

### 5.2 单次超时和局部重试

组件采用简单内存 semaphore，默认并发 2，可用一个正整数配置调整。等待名额响应调用方 context 取消；不创建持久排队记录。单次推理的默认超时为 120 秒，在取得名额、实际发起推理时开始，不把排队时长扣入该超时。

一个内存任务遇到暂时网络失败或响应丢失时至多局部重试一次，计数仅在内存中；每次推理使用自己的超时。明确取消、非法结构、内容不足、矛盾或后端不支持直接返回，由既有 report/等待规则处理，不追加格式修复轮。底层自动重试关闭或计入该局部上限，不由 application 逐次记账。

当前 CLI 实现由 Analyzer 单独负责这一次重试；本次调用临时选择 `steerrunner_text` provider，只设置现有原生 OpenAI 认证、默认地址解析、Responses 协议及 HTTP/流重试为 0，并关闭 WebSocket 和无限连接重试。不覆盖保留的内置 provider ID，不提供 base_url、env_key 或 token，不写宿主配置、不增加 API 服务或密钥。CLI 不接受这些设置时按普通解析失败处理，不退回默认的多层重试。

任务完成后，结果或失败留在当前内存句柄，直到业务采纳、后备转移成功或任务被取消；不能因提交失败、周期轮询或一次 revision 变化重新建立任务、反复调用模型。失败必须进入既有后备/等待，不在同一等待状态自动循环新建任务。

只有现有 `status=running`、parse 游标且仍获准推进的业务等待可以发起或采纳解析；`blocked`、`waiting_input`、`reconciling` 等状态即使保留 parse 游标，也不自动重算。冲突提交成功后消费句柄并失去运行资格；需要重新分析时由已有显式恢复重新获得资格，不新增持久“已消费”字段。

服务重启或用户显式恢复后允许基于当前业务源重新分析，使用当时的独立 parser 配置；不保证跨重启的总调用次数、总费用、相同模型或相同输出。接受未提交结果丢失及重复推理费用，不实现原操作对账、预算继承或请求重建。

### 5.3 原子采纳与必要检查

调用外部模型期间不持有 e.mu 或 worker.gate。完成通知交给既有业务获取协调，不由后端直接写状态。任务是否仍是当前句柄、输入来自哪个源，由调用方内存关联判断，不接受模型自行声明的身份。

采纳时重新读取当前记录，检查：原业务 Source/ImplementationResultID 仍匹配、业务仍等待该源的解析、当前任务未取消且未被替换、Run 仍允许推进。停止、关闭、暂停、服务关闭或源变化后的迟到结果丢弃；旧任务即使原源稍后再次被选中，也不能复活。

候选结果、获取结论、Workflow 转移及必要后继意图仍在原 Store.Commit 边界原子保存。重复通知不能重复转移；提交失败时保留内存候选或失败，重读并复核后重试提交，不重新调用模型。无关 revision 变化不要求重新分析。

业务层继续核实原审计执行、原来源、审计字段/统计及完成条件、必要 workspace checkpoint 和发布前 PR/head 关系。parser 不采集这些事实；JSON 合法不能跳过业务核验。现有纯 `RouteParsed` 等逻辑可按窄值保留，去掉 parser operation_id/权限策略依赖。已采纳的分析结果使用原有获取结果表示，不另设任务完成记录。

## 6. 必要取消与服务中断（PD-04）

- stop/close、暂停或源替换沿原业务事务更新已有状态，不增加 parser cancelled 字段或持久失效代号。提交成功后，在释放现有状态锁前使当前内存句柄失效，再在锁外通知组件取消；避免暂停后立即恢复时旧回调被误采纳。此处只作本地失效，不等待资源释放；业务事务失败时不能提前将合法任务永久作废。
- 任务一经取消，内存句柄和 context 保持失效。显式恢复后若业务仍需要分析，建立新的内存任务；即使源相同也不能接受旧句柄的结果。正常执行的 resume 只做原有资格判断，不恢复 parser 资源或继承其预算。
- `stop_status=stopped`、service stop、verify-stopped 和主执行可恢复性均不等待 parser 退出证明。组件负责自己的连接/短进程释放；清理异常可有界记录，不改变开发资源停止结论、不撤销整个 Run 授权、不阻塞其他 Run。
- 服务关闭取消内存任务，不将它转交常驻 owner。新服务只按已有业务游标和获取结果判断：仍在等待且获准推进时可以重新分析；已有结果及已提交后备/后继不重复执行；停止或暂停状态不因重启解除。没有 pending 槽、旧进程重连或解析专属接管流程。

取消不保证服务提供方已停止计费；只保证不再接受旧任务结果。模型响应丢失、未采纳前崩溃或配置变化后的重新分析均属于可接受的文本重算，不生成原生 unknown 状态。

## 7. 格式、配置与旧记录（PD-05）

保持现有 state/v6、默认状态目录、`steerrunner-cli/v1`、四种 workflow ID/version 和正常执行 operation 语义。本任务不引入全局格式断代、默认目录切换、安装迁移或无关的客户端/Viewer 配置改写；也不为 parser 增加数据库表或在途任务字段。资源元数据只更新实际删除的 parser 能力声明。

旧原生 parser 数据采用局部、非破坏的拒绝策略：在加载并启动执行前识别实际的旧 `parse_result` 操作或旧 parser package；含此类数据的目录明确报“旧 parser 执行记录不支持”，由原版本处理原资源并另选空的 v6 目录使用新实现。不保留旧 parser 执行/恢复/停止实现，不自动删改旧记录，不把旧 operation 当成新的内存任务重放。为避免引入历史 codec 解释器，首期对含实际旧 parser 操作的目录统一拒绝，不区分其是否完成。

没有实际旧 parser 数据的现有 v6 目录保持正常加载；旧配置的存在、曾声明 RetainsParser 的能力标志或 `method=llm_parser` 的纯业务结果本身，不作为整目录拒绝依据。识别只在存储加载边界做小型旧数据检查，不将旧 decoder、恢复判断或拒绝分支散布到正常 worker/stop/owner。旧 owner 身份中的 retains_parser 布尔值仅作为签名元数据原样透传，新 owner 不写入、不参与任何能力或恢复判断；保持原签名/checkpoint 的编码和比较。实施 P0 用真实编码样本固定识别条件；若发现不能以此边界安全删除旧链，应先记录具体格式反例修订设计，不自行恢复全局升版方案。

Parser 配置仍独立、默认关闭且按需验证；沿用 codex_parser 的 executable、model/effort、codex_home_env、超时/输入输出限额，新增可选 max_concurrent（省略或 0 使用默认 2）。可复用现有配置文件；确有不兼容字段时仅更新 parser 配置 schema 并明确诊断，不预设整个服务换代。继续使用既有 CLI 参数，不引入 API 配置；没有解析需求的路径不因缺失或损坏的 parser 配置而失败。

P1 确定单一后端实际字段，模型及配置只需在一次内存任务内稳定，不写入 Run 的冻结执行授权。能力声明区分接口已实现与指定后端已实测，不继续声明 parser owner/recovery 支持。升级前处理含旧 parser 数据的目录是运维提示，本轮不操作任何状态、进程或安装。

## 8. 验收条件（PD-06）

| 编号 | 必须证明的结论 |
| --- | --- |
| PD-A00 | Spec/Plan、基线和索引一致；本轮仅文档；后端选择、实现及原生资格均未冒充完成 |
| PD-A01 | parser/后端不依赖执行、权限、owner、reliable、Workflow、DB；原生 backend/owner/恢复/停止无 parser 执行/恢复/取证分支，旧签名元数据仅透传；普通开发及可直接提取路径零 parser 初始化/调用 |
| PD-A02 | 真实 CLI 只接指定文本/要求/schema，工具关闭及异常事件拒绝封装内部；复用现有登录，无需仓库、Orca 或新增凭据；成功、无效结构、缺失字段、unknown/partial 和超限均有明确结果 |
| PD-A03 | 只有内存任务和局部重试；取得名额后开始单次 120 秒超时；重复唤醒、提交失败及无关 revision 不重新调用；源/当前句柄/业务资格检查及原事务保证一次采纳，无持久派发计次或跨重启预算 |
| PD-A04 | 用显式屏障挂起解析，所属 Run stop 和其他 Run 的执行/停止仍完成；迟到和旧句柄结果不推进；服务关闭不等 parser 取证；重启可重算未取得结果的源，已采纳结果/后继不重做，暂停/停止不解除 |
| PD-A05 | 审计原文已有完整事实但格式不符时，解析挽救完整结构并正常进入既有业务校验；事实缺失/矛盾保持失败、一次 report/等待不被跳过；源或 checkpoint 失效阻止发布；原审计记录及 final 不被解析结果覆盖 |
| PD-A06 | 保持 v6 和默认目录；无实际旧 parser 数据的目录可用，含旧 parser 操作/package 的目录仅在加载边界非破坏拒绝；配置懒校验；真实后端、删除清单可核查，无旧 parser fallback 或全局迁移 |

只删除工具事件测试、放宽停止断言或包装旧 Execute 接口，均不构成完成。结构解耦须有真实消费者迁移和旧路径删除；文本分析能力须有真实后端最小验证。

## 9. 与既有文档的关系

本文件记录用户确认的解耦设计；实现与资格以验收记录为准。在本任务范围内取代 [结果获取 Spec](result-acquisition.spec.md) 中独立 parser 的原生 operation/权限/owner/资源结算条款、[执行边界 Spec](execution-boundary-refactor.spec.md) 中 EBR-F02/§5.2 的 parser 共用可靠操作要求，以及 [服务换代恢复 Spec](service-generation-recovery.spec.md)、[宿主认证 Spec](host-native-auth.spec.md) 中 parser 保留与恢复要求。正常执行的未知副作用零重发、原生身份、来源与 PR 校验仍有效。

对应旧验收属于历史构建证据，不删除、不改记为新实现通过。后续实施同步更新当前 Workflow 说明、测试清单、配置/schema/capabilities 和运维文档；不从设计阶段推导任何新支持声明。
