# Plan：Parser 独立文本分析与执行链路解耦

日期：2026-10-31。状态：CLI 内存任务及实现审计四项修复已完成，本轮定向业务/组件/相邻包、vet 和现有登录真实 CLI 复验通过；此前正式 L1、定向受控记录保留，不声明修复后全量重跑。依据 [Spec](parser-decoupling.spec.md)，调查基线 `e78c0db6f4e9be6242c6928cfeae596c8358ca5f`；当前命令、结果见 [验收记录](../acceptance/parser-decoupling.md)，先前调查见 [P1 历史](../acceptance/parser-decoupling-p1.md)。此前仅文档的范围已由后续“实施吧”扩展为代码和验证，不包括安装部署或真实业务资源操作。

## 1. 实施次序与范围

顺序：P0 基线 → P1 现有 CLI 封装 → P2 独立组件 → P3 内存任务接入与旧链删除 → P4 局部旧数据边界及当前文档 → P5 定向验证与收口。各阶段可分提交，但最终交付必须删除旧链；不得并存两种 parser 来回退兼容。

沿现有 CLI、state/v6、SQLite/Store.Commit、Workflow 和开发 owner 实施。保持默认目录及客户端绑定，不重写主执行恢复、停止、原生协议或整个测试框架；不创建持久解析任务、恢复预算、新服务、队列表或全局迁移。

## 2. P0：基线与变更清单

1. 实施时登记实际源码 commit/dirty，复核本设计与主干差异，保护已有用户改动。当前文档基线不代替未来实施基线。
2. 按调用者列出 parser 入口、配置读取、schema/capabilities、原文获取、业务路由、owner RPC、停止、接管、CLI helper 和测试清单；确认同时存在的普通本机与 retained-owner 路径全部覆盖。
3. 留存原正常路径的受控样本：普通开发零解析、审计结构完整零解析、审计原文可解析、原文不足走一次 report、审计源失效阻止发布。使用测试文本和临时目录，不读取生产状态/凭据。
4. 记录要保留的外部行为：原审计 final、修复统计契约、post_head/PR 业务校验、原生未知操作零重发；要删除的行为：parser 未知工具副作用阻断整个 Run、parser 资源结算阻塞停止/后备、原 owner 接管 parser。
5. 保存实际 v6 编码样本：无解析的开发/内容记录、实际旧 parser 操作及 package、只有旧能力标志/配置的记录。确定 PD-05 的最小加载拒绝条件，不扩展为旧 owner 查询、历史迁移或全版本兼容矩阵。

出口：PD-A00；后端已由用户明确选择当前 Codex 登录/CLI，工具禁用和进程管理封装在 parser 内。

## 3. P1：固定现有 Codex CLI 后端

用户已明确选择现有 Codex 登录和 CLI。本阶段不再寻找 API 服务或密钥；旧 API 传输原型退出实施范围并删除，历史调查保留在 P1 记录中。

1. 从现有实现保留 CLI 参数、登录环境及 JSONL 解析的必要部分，移入独立文本组件；去掉开发 adapter、backend、owner、operation 和 journal 类型。
2. 工具关闭、read-only/never、临时目录、版本检查和进程树释放均为组件私有实现。工具事件或清理异常只返回普通解析失败，不能传回写者缺席、unknown operation 或恢复要求。
3. 保留现有 executable/model/effort/codex_home_env 与限额配置，新增简单并发上限；配置只在需要解析时读取，不冻结进 Run。用 CLI 本次调用的临时 provider 参数保留现有原生认证和默认地址，关闭 HTTP/流重试、WebSocket 和无限连接重试，局部至多一次重试只归 Analyzer；不新增 endpoint/key 配置、服务或宿主配置改写，并在真实 CLI 上验证参数和已有登录兼容。
4. 受控测试验证真实子进程的 stdin/schema/输出、工具事件拒绝、超时/取消及临时资源释放。不得只改包装函数而保留旧原生账本。
5. 最小真实验证复用已有登录及本机 CLI，从非仓库临时目录整理审计文本，再验证取消或期限；不创建工作区、PR 或原执行会话。

出口：CLI 接口及私有能力边界明确，可继续内存任务接线；真实模型未验时单列缺口，不再因没有 API 凭据停止主链实施。

## 4. P2：独立文本分析组件

建议新模块为 `internal/textanalysis`，含普通类型、输入/输出校验和单一后端实现；命名可按仓库现状调整，边界不得变宽。

1. 按 PD-01 定义窄请求和结果。schema 使用现有内置审计结构；只移动/复用所需纯契约，避免为了导入整个 resultacquisition 带回 Workflow、backend 或 native process 依赖。
2. 使用简单内存 semaphore，默认并发 2，支持一个正整数配置调整；等待名额可由 context 取消。取得名额后对每次实际推理设置 120 秒默认超时，排队时长不扣入该超时。不新增调度器、持久队列或预算记录；组件至多一次局部重试，取消/无效结果不重试。
3. 保留输入总长、原文和输出限额；总长按最终 stdin prompt 加 schema 的 UTF-8 字节数计量，包含 JSON 编码的扩张，取得名额/启动 CLI 前拒绝超限。拒绝无效 UTF-8、非法/额外 JSON、schema 不符及超限，不截断后猜测完成。语义不足留给调用方现有审计校验表达，不构造成功。
4. 后端网络或短进程资源归本组件持有，ctx 取消后关闭连接/终止本组件短进程并有界释放；不使用开发 ExecutionOwner、ProcessIdentity journal、RetainsParser、writer-absent 证明或旧资源目录。基础进程辅助函数如确有需要，应为无执行业务依赖的局部实现；不能借共享 backend 类型重新挂接原生命周期。
5. 添加依赖边界检查：textanalysis 不导入 application/runstate/storage/workflow/backend/reliable/开发 adapters；开发 adapter、owner、主执行 reliable 不导入 textanalysis 或 parser。组装入口与获取协调可依赖两侧。

出口：独立调用可完成有界文本分析；无需启动 Runner、数据库、仓库或开发 owner。先通过有限的契约和取消/排队测试，不建设通用任务管理系统。

## 5. P3：内存文本任务、接入与旧链删除

### 5.1 接入与原子性

1. 复用 `audit.parse` 游标与当前获取结果，不向 Workflow/Record 增加在途任务字段。业务事务提交等待状态后，获取协调在锁外启动内存任务并捕获原文、Source/ImplementationResultID；同一源只保留一个当前句柄。
2. 将 `selectParser` 的业务选择与原生派发拆开；普通 `worker.advance` 删除 stop/unknown/resume 之前的 parser 前置分支。获取协调只在当前业务需要分析时调独立组件；完成通知沿现有 worker 唤醒/获取入口处理，不新增执行循环或原生 LifecyclePort。
3. 轮询/重连唤醒复用现有内存句柄。组件内部管理单次超时和至多一次局部重试，不持久派发计数、任务代号、配置快照或跨重启期限。仅 `status=running` 且 parse 游标、获准推进时发起/采纳；失败返回既有 report/等待分支，冲突等 blocked/reconciling 状态不因仍有 parse 游标而重建任务，不增加消费标记。
4. 按当前内存句柄、原业务源、游标和已有推进资格采纳，复用 NormalizeParserReport 和业务路由的纯部分。移除 RouteParsed 的 operation_id/ReportToolPolicy 耦合；历史可保留 `method=llm_parser` 和原 Source，不新增持久 parse_id 或伪造原生身份。
5. 在原事务内提交候选、获取结论和业务转移；提交失败留住内存候选或失败，重读当前记录并复核关联后重试提交，不重新推理。不要求调用开始时的整条 revision 相等。成功提交后才消费句柄，重复通知不能重复转移/派发后继。
6. 原审计资格、workspace checkpoint、修复统计与发布前核验保留在业务已有边界。删去 parser 内的工作区查询；明确缺原事实仍不能发布，禁止让 parser 补做审计、Git 查询或编造证据。

### 5.2 取消和服务中断

1. stop/close、暂停及源失效只更新已有业务状态；提交成功后在同一状态锁临界区使当前内存句柄失效，再锁外通知组件取消。不写 parser cancelled 状态，不持久失效任务代号，不等待模型或资源释放；结果采纳仍复核最新业务资格。提交失败时保留合法任务。
2. 服务 shutdown 通过 context 取消内存任务，不再调用 stopParserForShutdown 等取证/结算流程，也不等解析 Done；组件自行释放资源。旧句柄被取消或替换后，即使源相同、Run 已显式恢复，也不能接受旧结果。
3. 新服务沿现有业务游标判断是否还需要解析：没有已采纳结果且仍在等待/获准推进，可用当前 parser 配置新建内存任务；已提交的结果、后备和后继不重复执行。停/暂停不自动解除，resume 不处理 parser 资源。接受跨重启重复推理和额外费用，不恢复次数、期限或模型快照。

### 5.3 删除清单

| 位置 | 删除或收缩 | 必须保留 |
| --- | --- | --- |
| application/parser、worker、control、verify_stopped、takeover | parserTask 的原生资源句柄、advanceParser 原生前置、parserResourcesSettled、stopParserForShutdown、parser 恢复资格/解码 | 内存文本句柄、必要取消与结果采纳，正常执行自身的 gate/stop/接管 |
| backend/lifecycle、parser.go、recovery/capability 类型 | LifecycleParseResult、ParseInput/ParseEvidence、ParserTask/ParserOwnerPort、Parser recovery 标志与旧回执校验 | 主执行原生类型/动作、主执行及只读流程仍使用的权限类型 |
| reliable/operation/parser.go 及普通 operation 分支 | NewParser/InvokeParser/MarkParserDispatch/ParserReceipt、parser pending 特例 | 正常执行操作的固定输入、unknown 零重发、对账与提交规则 |
| resultacquisition/codexexec 及 adapter parser 配置代码 | 冻结原生包、Args/工具限制探针、事件工具检测、journal/owner/recovery、原 exec 后端，以及无消费者的 ParserConfiguration/hasHookHandler 与专属测试 | 有消费者的纯审计 schema/规范化函数，移至明确所属模块 |
| codex execution_owner/service、Orca local helper/control、cmd 组装 | RetainsParser 能力判断、OwnedParser/StartOwnedParser、cancel_parser/RPC 分支、parser 关闭/保留参数；旧 retains_parser 签名元数据只透传 | 原开发 owner、连接、Job/工具树、checkpoint、Orca 控制和原生退出证明 |
| resources/contracts/config/usage/测试 helper | 旧 parser profile/恢复声明、配置和专属字段消费者、虚构的解析原生 usage 绑定 | 开发及内容流程能力、现有用量权限与真实执行统计 |

逐项搜索所有消费者再删除；共享函数有实际其他用途则拆出纯部分，不删正常执行机制。业务 parser 不进入正常执行 token/原生会话对账；若后端返回用量，可作为分析元数据有界记录，本期不增加费用平台。

出口：PD-A01、PD-A03、PD-A04、PD-A05。不得通过仅关闭旧链、保留永不启用的兼容实现或放宽断言交付。

## 6. P4：局部旧数据边界、配置与文档

1. 保持 storage.Format/state/v6、metadata/user_version、默认目录和 CLI 协议。只在加载边界识别 P0 样本中的实际旧 parser 操作/package：命中时在启动 worker/owner 前非破坏拒绝该目录，提示原版本处理并另选空 v6 目录；不解释其恢复/停止状态，不静默清除或重放。
2. 证明没有实际旧 parser 数据的原 v6 目录能继续加载；旧 capability 标志/配置或纯业务分析结果不构成拒绝依据。含实际旧 parser 操作的目录首期统一拒绝，包括已完成操作，以避免增加历史解码器。只为原生 parser 删除保留这一存储边界检查，正常 worker/stop/owner 不新增旧执行分支。旧 retains_parser 布尔值仅透传以维持原签名/checkpoint 字节，禁止作为能力或恢复条件。若出现具体格式冲突，先修订设计，不自动引入全局断代。
3. 使用 P1 选定后端的最少配置，复用现有配置文件；只有 parser 配置实际不兼容才局部升 schema。默认关闭、按需验证。普通开发、内容流程和审计完整直提路径不被 parser 配置阻塞；继续使用现有 CLI 和登录，无 API 后端或密钥配置。
4. 删除 Run 冻结/接管中的 parser 执行配置依赖，新任务从独立组件取得本次配置，不要求跨重启相同。四种 Workflow ID/version、开发 profile、原生协议及现有结果存储保持；资源元数据更新实际能力删除，不改客户端绑定、Viewer 来源、安装目录或默认 state-dir。
5. 更新当前结果获取、Workflow、服务换代、执行边界、宿主认证和定向测试说明，标记被替代的 parser 条款。旧证据及历史结论保留；不新增全局迁移或安装验收矩阵。

出口：PD-A06。局部拒绝不能冒充原资源已停止；本轮不操作任何旧记录或资源。

## 7. P5：定向验证与完成条件

### 7.1 必需验证

| 范围 | 有意义的场景 | 验收 |
| --- | --- | --- |
| 文本组件/单后端 | 合法结构、缺失未知、冲突、畸形 JSON、prompt+schema 恰好限额/超一字节及编码扩张、输出超限、实际排队中取消、取得名额后才开始推理超时、默认及可调整并发 | PD-A02/03 |
| 内存任务与事务 | 等待状态提交失败零启动；重复唤醒不重复调用；CLI 底层重试关闭且局部 transient 最多两次调用、终态/取消零重试；候选/失败提交重试不重算；冲突 blocked 后多次唤醒不重建任务 | PD-A03 |
| 竞态与隔离 | 屏障挂起解析，stop 先提交后迟到；源替换、暂停/显式恢复后旧句柄结果丢弃；重复完成；无关 revision 变化；另一 Run 正常执行/停止 | PD-A03/04 |
| 服务重启 | 未取得结果且业务仍等待时可重新分析，允许使用当前配置；已采纳结果/后备/后继不重做；停止/暂停不自动解除；无解析资源接管或预算恢复 | PD-A03/04 |
| 获取业务 | 完整审计零解析、非结构原文解析成功、缺失/矛盾走一次 report/等待、源/checkpoint 失效不发布、原审计 final 未覆盖 | PD-A01/05 |
| 删除与旧数据 | 依赖守卫、旧字段/分支无生产消费者、普通 v6 可用、实际旧 parser 数据在加载边界非破坏拒绝、配置懒校验、无默认目录/客户端绑定改变 | PD-A01/06 |
| 真实最小接线 | 固定最终后端和构建，一次真实解析经过现有获取结果采纳；一次取消或超时；普通执行不创建 parser 资源 | PD-A02/04/05 |

竞态使用测试屏障和实际调用/提交计数，禁止固定 sleep 推断。测试验证外部调用次数、业务转移及停止是否被阻塞，不镜像私有函数实现。

### 7.2 运行范围

先运行受影响包的定向 `go test`、边界守卫及 vet，再运行最小受控获取闭环。预期涉及 textanalysis、application、workflow/resultacquisition、backend/reliable、storage 加载边界及 resources 的实际变更；实施时按真实包名列出命令，不把示意包名当成已运行。不为不存在的全局格式/客户端迁移追加测试。

本任务不改全局 `scripts/test.ps1` 的 L1/Controlled 前置规则。开发阶段可直接使用定向包测试和独立后端专项，报告标明定向；若使用正式 L2/L3 入口，遵循其实际前置，不能通过新增 skip 开关宣称全级通过。最终补受影响的现有快速回归，完整发布资格沿项目原门禁单列。

真实验证集中于一种 parser 后端和一次内存任务/结果采纳接线，不扩张为 Agent × 主机 × 工作流矩阵；使用主执行受控事实即可验证解析路由，但不能冒充真实 PR 已交付。若删除 owner parser 分支实际触及主执行启动/退出接线，补对应受影响的最小本机/Orca local 原生验证；未改远端协议无需重跑无关远端矩阵。没有真实后端验证不能将文本能力记为通过。

原测试中专门证明 parser 工具权限、未知副作用、旧 owner 保留、parser 树停止、started/final journal 和重启对账的义务随实现一起删除，以本表替换。通用 stop/recovery/原生输入/审计来源测试保留，不可把同文件所有测试整体删除。

## 8. 交付与状态记录

文档轮完成条件仅 PD-A00：新增 Spec、Plan，更新文档索引，核对链接、编号、边界与差异仅文档；不创建实现验收报告或声称新后端已验证。

后续代码完成条件为 PD-A01–A06 全部满足：登记唯一真实后端、最终源码/构建、准确命令、结果、删除清单和未验范围。普通执行无 parser 生命周期依赖、需要解析时仍能真实分析，两项缺一不可。不能只写出接口、关闭功能或移走文件就标记完成。
