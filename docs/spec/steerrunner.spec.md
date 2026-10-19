# Spec：SteerRunner 首版产品与 CLI 契约

日期：2026-10-14。用户已授权按[实施 Plan](steerrunner.plan.md)实施。所属交付：SteerRunner 首版独立化及 SteerLine 接入。本 Spec 描述首版目标，阶段能力、实际验证及未完成项统一见[专属验收](../acceptance/steerrunner.md)；不把契约中的命令视为当前全部可用。

历史用户范围（2026-10-15）：本次交付只验收 Windows/amd64 本机 Codex。Orca 本机与远端验收均已取消，适配器和契约保留为未验能力，不声明真机支持；原失败证据不改写为通过。

扩展契约：[内置 Workflow 选择与审计模型 Spec](workflow-selection-audit.spec.md) / [Plan 与验证记录](workflow-selection-audit.plan.md)。多内置流程发现/选择、调用方指定审计模型和原链路审计修复的实际交付范围以扩展文档为准；暂不提供外部 workflow 装载。本文保留首版契约和历史范围。

## 当前产品边界（2026-10-19）

SteerRunner 面向个人，在本人管理和信任的多台执行主机上运行冻结的 Spec/Plan。调用方指定本机或已配对 Orca environment；SteerLine 负责任务规划、DAG、目标选择和最终交付组织。Runner 负责独立 Run、环境检查、执行事实、反馈、显式恢复、停止和可核验 PR。

每台执行机由用户安装工具、登录 gh 并准备固定仓库/base；Runner 不复制调用方凭据，也不自动安装、修复登录、提升 OS 权限或迁移正在执行的 Run。当前平台和 agent 支持仍以 capabilities 与对应验收矩阵为准，个人多机定位不代表所有 OS/agent 已通过验证。

GitHub 认证统一为执行主机已有 gh 登录的单仓库 broker。各 Run/Check 独立 capability、期限、操作边界、socket 与清理证明；停止一个 Run 不退出宿主登录、不撤销其他 Run。保留多 Run、双模型审计、sandbox/full_access 显式选择和未知结果零重发。

本阶段不提供 GitHub App 签发/私钥管理、团队/多租户账号系统、集中密钥托管或自动跨机故障迁移。移除 `execution.settings.github_auth`，所有形态（含 null）在 Run/Check 接受前返回 invalid_input；内部 profile/helper 配置同样严格拒绝，不能静默切换身份。旧 App 执行必须用原版本完成停止和认证收尾后升级；历史请求/报告保留，不转换为新身份或本构建资格。

## 1. 目标、依据与首版范围

SteerRunner 接收固定 Spec、Plan、参考材料及执行配置，在指定本地或远端环境按 workflow 实现、验证、处理反馈并提交、推送、创建或复用 PR，输出可核验的执行结果。它可由用户独立使用，也可由 SteerLine 或其他调用方经同一 CLI 契约使用。

用户已经确定：

1. 独立仓库为 SteerRunner；以一个 EXE 提供常驻服务与 CLI 客户端。
2. SteerLine 使用 CLI 契约；CLI 是短生命周期的通信客户端，执行由常驻服务承担。
3. 默认 workflow、提示词及必要程序资源内嵌，不依赖 SteerLine 的安装文件或文档资源。
4. SteerLine 指定项目运行目录下的独立 Runner 状态目录，Runner 自己管理该目录的执行数据和日志。
5. 任务规划、DAG、领取、候选选择及最终 TaskDelivery 留在 SteerLine。

首版包含 Windows/amd64 的独立程序、常驻服务、CLI、持久化运行状态、默认开发流程、提示词覆盖、现有本地 Codex 和 Orca 本机/environment 路径的提取与接入。远端一期沿用 Orca environment；不因共用代码而声明任意机器、其他代理或所有恢复行为已可用。

首版不包含任意 workflow 图编辑器、通用任务 DAG、新的 SSH/远端 daemon、Windows SCM 安装、独立 UI、自动合并 PR、旧 SteerLine 数据迁移、双写或旧执行回退。第三方运行时和持久化流程引擎的选型不构成本次前置。

### 1.1 现有实现调查

调查读取的 SteerLine HEAD 为 `a81341a515251e23b4fa19d7a09096c11d0db8dc`。下列路径均相对于 SteerLine 仓库；本次仅阅读，不修改该仓库。调查版本不是未来代码抽取基线，实施开始时应重新固定两个仓库的真实代码与文档 commit。

| 来源 | 结论及拆分要求 |
| --- | --- |
| `internal/execution/command_boundary.go`、`service_construction.go`、`service_state.go` | 执行仍依赖 TaskWorkPort、TaskResultPort、图快照及混合配置/宿主；需由独立 Run 输入、Runner 存储与宿主替代 |
| `internal/execution/prompt.go`、`staged_flow.go`、`staged_result.go`、`model/staged_flow.go` | 当前 development/v3 拥有实现、报告、发布及可选发布补报；提取行为，去除任务命名与状态依赖 |
| `internal/execution/backend/lifecycle.go`、`adapters/codex`、`adapters/orcatransport` | 可复用操作、身份、回执和原生取证；结果未知不能以新请求盲重发 |
| `internal/execution/workspace`、`publication`、`native_delivery_command.go` | Git/PR 能力可提取；Task 关闭及后继释放保留在 SteerLine |
| `internal/storage/ports/task_result.go` | ResultCommit 目前混合 Task、Workflow、Execution、Delivery；跨仓后改为 Runner 事实消费与 SteerLine 自有交付事务 |
| `docs/architecture/orca-native-worker.md`、`docs/spec/product-simplification-04-conversation-recovery.spec.md`、`docs/spec/worker-service-recovery.spec.md` | 普通最终回复、原来源核验、仅恢复原会话、停服后的人工继续门槛及后端能力差异需要保留；历史验收不能直接记作 Runner 验收 |

另一工作区的 `docs/spec/worker-three-layer.spec.md` 是衔接参考：Workflow → ReliableExecution → ExecutionAdapter，actor/存储为共同宿主。读取时 SHA256 为 `4bd58fa48ef2dba919fed095a5195b98e814e2a4a1c59997e778b34a121dac6e`，属于在途材料，不作为已合入的代码或固定规格。实施 Plan 必须核对最终产物，避免并行建立重复的旧/新运行时。

## 2. 产品边界与状态所有权

| 事实或能力 | 所有者 |
| --- | --- |
| 产品目标、Spec/Plan 编制、任务 DAG、前置资格、任务领取 | SteerLine 或其他调用方 |
| Task / Candidate 与 Run 的对应关系、候选选择 | SteerLine；Runner 的 correlation 仅为不透明关联值 |
| Run、冻结输入、workflow 步骤、交互问题、结果及 PR 绑定 | SteerRunner |
| Attempt、原生会话绑定、操作意图/回执、恢复与停止资格 | SteerRunner 可靠执行层，adapter 提供原生事实 |
| 具体原生会话历史与平台材料 | 相应后端；Runner 保存必要身份、可核验 final 和缺口 |
| PR 的实际存在、仓库/base/head、开放/关闭/合并事实 | Git / PR 平台；Runner 可查询、核验并返回事实 |
| 唯一 TaskDelivery、Task closed、后继释放 | SteerLine 自有事务 |

Runner 不读取任务图来决定准入，不领取或关闭 Task。Task ID、候选 ID、调用方 ID 可以作为 correlation 返回，不能据其值推导分支、资格或步骤。任意用户提交同一种 RunRequest 即可执行，不要求采用 SteerLine 的编号、Spec/Plan 文件命名或独立 acceptance 文档惯例。

每个 Run 只有一个 Runner 状态权威。Runner 的并发限制属于资源调度，不成为任务依赖系统。远端原生运行和连接客户端不另建可写 workflow 权威。具体存储可后续调整；CLI 和 SteerLine 不直接读写它。

## 3. 一个 EXE、服务与 CLI

`steerrunner.exe serve` 承载长期运行服务；其他业务命令是短进程客户端，经本机 IPC 向服务发请求，获得结果后退出。两种进程来自同一个 EXE，使用同一种执行机制。

| 角色 | 职责 |
| --- | --- |
| CLI | 解析参数/JSON，确定状态目录，发现服务、连接、发送请求、输出响应；不驱动业务步骤或直接写运行数据库 |
| 服务 | 唯一运行状态写者、命令接受、资源调度、后台执行、观察、对账、结果提交与恢复 |
| SteerLine 客户端 | 调用 CLI，解析 JSON/退出码，保存任务与 Run 关联，展示结果并应用自有任务规则 |

Windows 首版 IPC 使用命名管道。端点绑定规范化 `state-dir`、随机服务实例身份和当前用户；管道 ACL 限制为当前用户。端点发现、连接握手与目录写锁必须一致，不能仅凭 PID 或陈旧发现文件认定原服务仍有效。CLI 不能将请求改投到另一个目录或服务实例。

同一规范化目录最多一个服务写者。并发启动对账同一实例，不生成两个宿主；别名/大小写路径指向同一实际目录时也不能绕过锁。启动失败或实例失效返回明确错误，未证明旧写者结束不得接管。每个状态目录的服务独立管理其 Run。

调用方可在 resident 业务/查询命令上传 `--expected-instance-id <id>`：该预期实例在发现端及服务受理端核验，不匹配返回 `stale_instance` 且零业务派发；绑定的 submit/environment-check start 不隐式启动服务。`capabilities.instance_binding.expected_instance_id: true` 公布支持。version、serve、service start/status/verify-stopped 不接收此参数；service stop 仍由原输入正文 expected_instance_id 约束。

`submit` 和显式 `service start` 可在服务不存在时启动同目录的后台服务；后台启动不弹可见窗口。服务必须脱离 CLI/调用方的生命周期和会随其退出而结束的 Job 所有权；不能仅创建一个继承父 Job 的子进程就声称常驻。无法独立启动时在接受 Run 前明确失败，允许用户从独立入口启动同一个 serve 宿主，不增加另一种执行机制。`inspect`、`events`、`last-message`、`reply`、`resume`、`stop` 不隐式启动服务，服务不可用时返回 `service_unavailable`。`version` 不需要服务；`service status` 仅核验发现/进程/握手事实，不成为数据库读取后门。

Windows 实现先以原生 breakaway 挂起创建并检查 Job。创建被禁止时拒绝；创建成功但仍有关联时，可使用系统自带 Windows PowerShell/CIM 创建挂起进程，核验用户 SID、完整性级别、授权组/权限与 EXE，不能提升权限。系统能力缺失或核验失败明确拒绝。启动意图先保存；原进程创建结果无法核清时，状态/错误为 `startup_unverified`（exit 4），保留原意图，不盲目再次启动。手动 `serve` 的生命周期由独立宿主负责，`detached` 如实报告。

CLI 或 SteerLine 退出、IPC 断连不会自动停止已接受的 Run。首次输入明确接受后，正常服务实例可继续既定 workflow；客户端进程是否存在不改变其授权。服务停机后的规则见第 8 节。

`service stop` 是服务生命周期命令，`stop` 是指定 Run 的执行命令。停服请求携带 status 返回的 expected_instance_id，核验后才能作用于该实例；新请求不得停止已经换代的服务。停服接受回执不能证明所有原生执行已经停止；必须保存各 Run 的已确认/未核实处理事实。本地 owned Codex 有界处理自己的进程树，Orca 可以只结束本地观察。Windows 系统服务注册与开机自启动另行设计。

服务在所有 worker 退出并读取持久化停止事实后、释放写锁前发布最终结算。`service status` 仅在写锁空闲且无未核实启动时公开 `last_shutdown`，绑定 `instance_id/request_id/state_dir/executable_sha256`，分别返回 `resources_settled` 和诊断；不读取或推导私有执行库。只有全部 Run 已持久化 stopped 且关闭无错误才标为 true；未知、unsupported、观察脱离、缺失证据不等于已停止。新写者获得写锁后首先移除旧结算，防止启动失败或异常退出后继续使用上一代证明。无记录不伪造成功。

显式 `service verify-stopped` 为停机后的独立只读取证入口，不能隐式进入启动或恢复：取得既有 writer.lock、拒绝未核实启动后，只读当前格式数据库，逐 Run 返回原 ID、input_digest 和资源结论。已持久化 stopped 可作为原证据；未保存 stopped 的本地 managed Codex 仅在记录完整、原 Job 唯一 owner 和精确 root 都已退出时核实资源消失，复用现有 managed 恢复证明。Orca 另按冻结 profile/resource 查询原 runtime 的完整 worktree 终端清单：要求计数一致、未截断、该 runtime 自身的 local host 纳入且未省略；同 handle 必须保留原 incarnation 并已退出，或原终端在完整范围中缺席。无关配对 host 的省略不能冒充原 host 取证。PTY 清单不是 native writer-tree 证明；通用原生 helper 还须既有原 owner/exit/authentication 结算，且不能混入 Codex/readers 证据。Codex inline-v3 helper 在 readiness 中固定原 helper 身份和仅存内存的退出签名公钥；app-server/tools 与 TUI 分别由原非 breakaway Job 管理。显式 stop 只经既有原 runtime 文件通道提交与该 readiness 绑定的公开请求，先实际查询两棵 Job 至空并关闭认证，再由原 helper 签发完整 readiness 绑定的退出凭据，最后关闭原 PTY。helper 已返回原 shell 时也按原 PTY/worktree 和原生凭据核对，不依赖 agent 展示标签。公共凭据可放在工作区，但只有原 helper 持有私钥；文件改写、跨运行重放或缺签名不能放行。离线重验通过既有 files.readChunk 有界读取原 runtime/worktree 的固定相对路径并验签，不新建验证终端或执行。全部原 readonly reader 还须原 Job 清空事实以及原终端停止，快照和既有调用端 journal 均纳入；查询失败可单独返回已结算的 reader 事实，不能晋升业务完成。缺 readiness/key/认证/树证明、未知创建或旧 helper 无证据仍未核实，不能用调用方本机 PID 代替原执行主机事实，也不迁移或补写历史证据。普通停服完成既有观察清理后可以使用同一读取规则提交最终 stopped，但不能因此自动关闭业务终端或输入。缺记录、身份/权限未知、仍活动、清单不完整或仅观察脱离均不能成功。缺目录、锁、库或不支持格式直接失败，不创建/迁移现场，不保存 stopped、不重写历史回执。调用方只消费该公开 CLI，不读取 Runner 私有库。实施和验证范围见 [Orca 停服核验 Plan](orca-shutdown-verification.plan.md)。

## 4. 状态目录、内嵌资源与环境依赖

所有有目录上下文的命令使用 `--state-dir <绝对路径>`。独立使用时省略该参数，默认目录为当前用户 `%LOCALAPPDATA%/SteerRunner`；SteerLine 必须显式传入项目运行目录下的 `runner` 子目录。CLI 参数优先，不能依赖调用时 cwd 或 SteerLine 环境变量改变目标。

建议布局如下；数据库和私有文件结构不属于调用方读写契约：

```text
<state-dir>/
  runner.db
  service/                  # 目录锁、实例与发现材料
  logs/                     # 服务及诊断日志
  runs/<run-id>/
    inputs/                 # 接受时固定的文档/流程/配置
    logs/                   # 本次运行的可取得日志
    artifacts/              # 报告、检查证据及资源引用
```

日志目录位于 `state-dir`。它不是用户代码工作区，也不因代理在本地/远端或切换 cwd 而改变。业务记录/操作 journal 是权威，日志和导出文件是诊断或投影，不能靠读取文本日志推进步骤。

内嵌资源包含默认流程、默认提示词、CLI 协议与内置后端配置 schema，以及必要程序资源。将 EXE 放入干净目录可完成 `version`、服务启动和协议查询，不要求附带模板目录、Python/Node 运行时或 SteerLine 安装文件。用户的定制内容接受后保存到本次 Run，不要求原提交文件在恢复时仍存在。

独立不等于零环境依赖。用户代码仓库、文档材料、Git、所选代理/Orca、账号认证、远端连接与平台访问是输入或环境依赖。缺失时明确拒绝准入或记录环境阻断，不伪造执行成功。首版不捆绑代理或复制 SteerLine 安装目录的数据库、认证和配置。

已知凭据字段不得进入请求、输入快照、普通日志或机器响应；配置使用认证引用，由对应后端在所在主机解析。拒绝含密码/token等嵌入凭据的仓库 URL；SSH URL/SCP格式中的登录用户名不是秘密，合法身份仍可使用。可取得的原生诊断按已知敏感字段脱敏；不承诺取得全部工具过程或识别任意模型正文中的秘密。

## 5. CLI 协议

机器模式使用 `--json`。首版协议标识为 `steerrunner-cli/v1`，可用 `--protocol steerrunner-cli/v1` 明确指定；省略时选择当前首版默认协议。`version --json` 返回程序版本、支持的 CLI 协议和内嵌资源版本，不启动服务。

### 5.1 命令面

下列均为待实现契约，所有业务请求均通过 CLI → IPC → 服务：

| 命令 | 输入与行为 | 成功响应 |
| --- | --- | --- |
| `version --json` | 离线版本查询 | 程序/协议/资源版本 |
| `service start --state-dir ... --json` | 幂等启动或连接同目录的同一 EXE 发行包；不同 EXE SHA/版本/资源摘要拒绝 directory_conflict，保留原服务 | 实例身份、目录、就绪事实、`executable_sha256` |
| `service status --state-dir ... --json` | 不自动启动；核验服务是否可达 | 实例身份与服务状态；不可达如实表达 |
| `service verify-stopped --state-dir ... --json` | 显式取得既有写锁后只读当前执行证据，核验原资源消失 | 目录、逐 Run 身份/输入摘要及资源结论；不修改原回执 |
| `service stop --state-dir ... --input - --json` | 幂等接受有界停服请求；保存处理意图后再执行，最终状态由 status/重启对账核验 | command 回执与已知执行处理事实 |
| `capabilities --state-dir ... --json` | 在线查询内置流程、adapter/config schema、平台能力与限制 | 能力版本和适用性；声明不替代现场事实 |
| `submit --state-dir ... --input - --json` | 固定 RunRequest 并接受异步执行；可按第 3 节启动服务 | 原请求的 command_id、run_id、接受状态 |
| `inspect --state-dir ... --run-id ... --json` | 查询已提交的同一快照；也可用 --request-id 或 --command-id 定位原命令，三个定位参数必须恰选一个 | 原命令回执及关联 RunView；提交丢响应时不需要先知道 Run ID |
| `events --state-dir ... --run-id ... --after ... --limit ... --json` | 有限分页，省略 after 从起点读取；limit 默认 200，允许 1–500 | 事件页、下一游标、是否还有数据 |
| `last-message --state-dir ... --run-id ... --json` | 只读进程内最佳努力活动摘要，不启动原生历史读取或新输入 | run_id、attempt_id、message（无样本为 null）；message 包含 Run/Attempt/Session/Turn、kind、text、observed_at |
| `open-native-session --state-dir ... --run-id R --attempt-id A --session-id S --generation G --json` | 不隐式启动服务；在原 worker gate 内验证当前身份、原进程及只读 thread/workspace，再开独立 Codex CLI 窗口 | 同一 Run/Attempt/Session/Generation 及 opened；仅说明窗口已创建，响应未知不自动重放 |
| `reply --state-dir ... --run-id ... --input - --json` | 回复指定问题或提交业务反馈，保存后异步对原目标执行 | command 回执；保存、投递、回合开始分别查询 |
| `resume --state-dir ... --run-id ... --input - --json` | 对当前恢复状态给出新授权，仅在资格满足时继续原会话 | command 回执；实际结果另查 |
| `stop --state-dir ... --run-id ... --input - --json` | 先持久化撤销本 Run 新输入授权，再按真实能力停止原执行；区分接受、授权撤销、实际停止与未知 | command 回执；实际结果另查 |

`--input -` 从 stdin 读取一份 UTF-8 JSON，不从命令行字符串传完整文档或秘密。请求最大 8 MiB，超限/非法 UTF-8/重复键/尾随第二对象拒绝；核心字段采用精确名称并拒绝未知字段，adapter settings 根据其版本化 schema 验证。人工 CLI 可以把文件重定向到 stdin，文件只在提交当次作为输入。

stdout 恰好输出一个 UTF-8 JSON 对象及换行；所有诊断走 stderr。机器模式不出现交互提示、颜色、进度条或原生工具输出混入。`events` 首版使用有限查询页，不要求长期订阅进程。原生程序无法启动等发生于 EXE 执行之前的宿主错误，由调用方作为调用失败处理。

### 5.2 请求与响应信封

除按目录幂等的 `service start` 外，所有写命令的 stdin 使用同一信封；操作既有 Run 时 run-id 由参数指定，input 中的绑定必须与之相符。查询命令和 `service start` 无需 request_id，响应中的 request_id 为 null。

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "caller-request-001",
  "input": {}
}
```

```json
{
  "protocol": "steerrunner-cli/v1",
  "ok": true,
  "request_id": "caller-request-001",
  "data": {
    "command_id": "runner-command-001",
    "run_id": "runner-run-001",
    "receipt_status": "accepted"
  },
  "error": null
}
```

协议/参数中的版本必须一致。成功查询的 request_id 为 null；失败响应使用 `ok:false`、同一协议及原 request_id（尚未解析则 null），`error` 至少包含 code/message，data 保存可核实的关联值或为 null。任何失败都不能造出 accepted 回执。

| 退出码 | 含义 |
| --- | --- |
| 0 | 命令成功查询或已持久化接受；不等于 Run 成功或动作已完成 |
| 1 | 内部错误，无法形成正常命令结果 |
| 2 | 参数/请求/schema 非法，尚未接受 |
| 3 | 协议、格式、命令或必要能力不支持 |
| 4 | 服务/环境不可用，且能证明本次请求尚未接受 |
| 5 | request_id 内容冲突、目标不存在、状态版本过期、身份或目录冲突 |
| 6 | 无法核实请求是否接受或外部动作是否发生，必须按原请求对账 |

超时或断连后不能一律返回 4。无法证明未接受时返回 `request_outcome_unknown` 与退出码 6。进程被杀、无输出或损坏输出也由调用方视为未知，不据此生成新 ID。CLI 有有界通信期限，不能等整次模型执行才返回；超时取消本次等待，不撤销已接受的操作。

### 5.3 幂等、并发与事件

request_id 是调用方生成的非空不透明字符串，以规范化状态目录为作用域，写命令之间共同去重。同 ID/同命令/同目标/同提交正文返回原 command_id、run_id 和当前原回执；同 ID 携带不同内容返回 `request_conflict`。摘要基于规范化提交值，不能在重试时用新版默认配置重渲染后创建另一执行。

`reply`、`resume`、`stop` 的 input 携带 inspect 返回的 `expected_revision`；问题回答还携带 question_id。`resume` 仅表达继续当前已展示恢复状态的明确授权，不接受更换会话/后端参数；`service stop.input` 必含 expected_instance_id。旧问题、旧授权或版本冲突明确拒绝。同请求的幂等重试先返回原回执，不因 Run 已推进而重新判为新命令。

接受回执的 receipt_status 和动作实际处理结果分开。inspect 返回的命令至少含 request_id、command_id、command、run_id（无关联则 null）、receipt_status、action_status、error；action_status 为 pending、running、completed、failed 或 unknown。completed 表示该命令动作已结束，不代表整个 Run/任务完成。停止、答案投递、恢复的原生处理事实另附，不把 accepted 自动改成动作成功。

`inspect --request-id` / `--command-id` 返回原回执及关联 Run；查不到返回 `request_not_found` / `command_not_found`。服务恢复对账尚未完成时，不能据暂未查到断言旧外部动作不存在；对已提交请求的可核实“尚未接受”与恢复中的未知分别表达。没有 Run ID 的 submit 重试仍使用原 request_id 和原正文。

事件拥有稳定 event_id、run_id、持久游标、类型及必要关联。游标不透明，客户端不能用时间或日志行数替代；错 Run 或无效游标明确拒绝，不能悄悄从头读取。分页/断线重读允许重复交付已提交事件，调用方按 event_id 去重。无新事件返回空页和可继续查询的游标。旧 Attempt/步骤结果可以记为历史事实，但不能推进当前步骤或污染其他 Run。首版保留持久事件，不自动裁剪游标对应范围。

## 6. RunRequest 与输入冻结

`submit.input` 的公共结构：

| 字段 | 要求 |
| --- | --- |
| `title` | 非空任务标题，不参与资格推导 |
| `correlation` | 可选对象，值为不透明字符串，如 caller/task_id/candidate_id；无 SteerLine 固定字段要求 |
| `repository` | 必含 url、完整 base_revision、remote、base_branch；可选 local_path 作为本机已有仓库入口 |
| `documents` | 必含独立 spec、plan；可选 references 数组；所有材料接受时固定 |
| `workflow` | 必含 id/version；首版支持 development/1，可选 prompt_overrides |
| `execution` | 必含 adapter、target、settings；target.kind 为 local 或 orca_environment，后者必含 environment 标识；settings 按内嵌后端 schema 固定 |
| `publication` | 必含 provider、repository、base_branch、settings；首版复用 command/gitea provider 配置及实际身份检查 |

base_revision 为完整 40 或 64 位 Git commit ID，不能仅保存可移动分支。local_path 是入口而非结果工作区；Runner 验证其 remote 与固定 url/目标一致，再为本 Run 准备独立工作区。保留调用方仓库未提交文件，不能 reset 用户现场。publication.base_branch 必须与 repository.base_branch 一致，provider 仓库与 Git remote 身份必须匹配。

接受新提交前，Runner 在私有缓存中从固定 remote 显式刷新 base_branch，再验证原 base_revision 属于该目标分支；local_path 的本地分支和已有缓存引用不作为最新远端事实。刷新失败则在接受前报告环境不可用，不能使用旧引用放行；远端前进不改变冻结基线，远端回退排除该基线时仍拒绝。此规则不重新解析已接受 Run 的固定输入，验证见[分支刷新 Plan](repository-base-refresh.plan.md)。

每份文档为下列来源之一，并含用于展示与物化的安全相对 path：

- `kind:inline`：含 UTF-8 content，接受时保存正文与 SHA256。
- `kind:git`：含完整 commit；从上述仓库该 commit/path 读取并保存正文及 SHA256。

spec/plan 的 path 必须不同。references 可以包含验收或公共约束，但不要求固定命名、目录或单独验收文档。拒绝绝对/越界路径和会逃出材料目录的链接目标。文档 commit 可以不同于代码 base_revision，分别记录，不在恢复时重新读取移动分支或原提交文件。

服务必须在返回 accepted 前，持久化完整材料快照、规范化请求、实际流程/模板/配置内容及版本摘要；任何尚未完成的准备动作保存为原操作意图。输入无法完整取得则拒绝本次 submit，不启动业务代理。读取 Git 材料所需的缓存/fetch 可在接受前进行，但缓存副作用不代表 Run 已接受；并发同 ID 只能提交一个 Run。

执行时向代理提供本 Run 冻结文档的实际可读位置，并保持其来源与摘要；准备方式由 adapter 实现，不能悄悄退回调用方文件。本地 Codex 使用独立工作区 Git admin 下的固定文件；Orca 使用实施输入中附带的固定正文、来源 commit 与 SHA256，`prompt:<path>` 标识该输入内的文档，反馈实施再次附带同一正文。调用方仍可在 snapshot_path 找到本机持久快照，execution_location 表示代理实际读取位置。Orca 当前 CLI 以参数传输文本，首版固定材料的 JSON 表示与四份提示词正文合计最多 12000 UTF-16 单元，超过时在接受前明确拒绝。Orca 必须提供已注册项目的 local_path，environment 中按原适配器的同名项目规则解析并核验固定 base。运行过程中修改调用方文档、全局配置或模板不影响已接受 Run。反馈沿用固定材料；更换 Spec/Plan、workflow 或后端应提交新 Run。

RunRequest 示例（占位 commit 应替换为实际完整 commit；这是形状示例，adapter/provider settings 的完整 schema 由内嵌能力提供）：

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "demo-submit-001",
  "input": {
    "title": "Implement the fixed Spec and Plan",
    "correlation": {"caller": "steerline", "task_id": "task-A"},
    "repository": {
      "url": "https://github.com/example/project.git",
      "base_revision": "<full-git-commit>",
      "remote": "origin",
      "base_branch": "main"
    },
    "documents": {
      "spec": {"kind": "inline", "path": "spec.md", "content": "# Spec\nImplement the requested change.\n"},
      "plan": {"kind": "inline", "path": "plan.md", "content": "# Plan\nImplement and run the specified checks.\n"},
      "references": []
    },
    "workflow": {"id": "development", "version": 1, "prompt_overrides": {}},
    "execution": {"adapter": "codex-app-server", "target": {"kind": "local"}, "settings": {}},
    "publication": {"provider": "gitea", "repository": "example/project", "base_branch": "main", "settings": {"base_url": "https://example.com"}}
  }
}
```

首版 adapter ID 为 `codex-app-server`、`orca-terminal`；Codex 目标只支持 local，Orca 支持 local/orca_environment。`capabilities` 必须返回 settings schema 标识、版本和 schema 正文；程序内嵌相应 schema/默认值，实施 Plan 补齐模型、启动/权限、环境、超时及认证引用的具体字段，不把 settings 当作任意透传脚本。缺少必填配置明确拒绝，未知键不静默忽略。

配置 schema v1 的公开字段如下。默认值在字段省略时适用；字段出现时不得为 null，executable/model/reasoning_effort/wait_timeout 不得为空字符串。参数均为 argv，不经 shell 拼接。

| 配置 | 字段及默认值 |
| --- | --- |
| Codex settings | `executable` 默认 `codex.exe`；可选 `model`，省略继承工具配置；`reasoning_effort` 默认 `medium`，枚举 minimal/low/medium/high/xhigh/max/ultra。GitHub worker 使用执行主机 gh 登录的单仓库代理；`github_auth` 已移除，含 null 也返回 invalid_input。无权限覆盖字段，固定 workspace-write/MXC/network enabled/never |
| Orca settings.transport | `executable` 默认 `orca.exe`；`args` 默认空数组；`wait_timeout` 默认 `30s`，Go duration，须大于 0 且不超过 1 分钟。args 不得覆盖 environment/pairing-code，路由来自冻结 target |
| Orca settings.agent_launch | `executable` 默认 `codex`；`args` 默认空数组；可选 `model`、`reasoning_effort`（同枚举，省略继承工具）。transport 与 agent_launch 不混用 |
| command provider settings | 必填 `base_url`、`query.executable`、非空 `query.args`；可选 `authentication_refs`（环境认证的引用名称数组，不是凭据值）。实际查询协议/身份沿用提取的 provider |
| gitea provider settings | 必填 `base_url`；可选 `authentication_refs`。不接收内嵌 token/password 字段 |

所有字段精确命名、拒绝未知键；已知凭据参数不得放在 args。默认 schema 被内嵌并可查询不等于后端已 ready；阶段版本必须在 capabilities 中明确 execution 状态，未接通执行时拒绝提交而不持久接受虚构 Run。

## 7. 默认流程、提示词及查询结果

### 7.1 Workflow

对外默认流程为 `development/1`，保存其实际定义摘要；它继承 SteerLine development/v3 的当前行为，不沿用 Task/DAG 语义：

```text
implement.execute → implement.report → publish → pr_ready
                                       ↓ 必要时
                                  publish.report
```

实施轮按固定 Spec/Plan 实现、测试及修复；检查和审查范围来自任务材料，不固定增加 verify/review 阶段。报告轮只格式化前一指定实施结果；发布轮推送并创建/复用 PR。发布正文无法解析时按现有规则补报，发布补报只格式化原发布结果，不再次发布。每个源结果最多一次报告格式修正；源失败或缺口不能由报告 done 覆盖。

首版 `prompt_overrides` 只覆盖上述四个步骤的非空指令正文，未知步骤拒绝。固定材料引用、报告输入来源、publication 目标和机器结果格式由 Runner 附加并保存，不允许模板删除这些契约。覆盖内容是 UTF-8 文字，不执行脚本或任意模板表达式。override 不改变权限、步骤门槛、重试次数或 PR 归属。

机器报告沿用 `outcome:done|needs_input|blocked`、summary、可选 question/blocker/outputs；发布 done 必含实际 pr_url。机器报告最大 128 KiB，超限明确记录格式/结果缺口。已知业务字段必须唯一且一致，接受一份可完整识别的 JSON 或完整 JSON 代码围栏，不从散文、工具输出或多个对象中猜选结果。额外解释字段不驱动业务；报告需绑定源输入、回合和来源，不能取另一次用户输入后的正文代替。

正常流程共用本 Run 的原会话。模板中“不得修改/检查”的要求不是原生强只读证明；原生权限与可用性另由冻结 Profile 和能力核验表达。

### 7.2 RunView 与交互

inspect 至少返回 run_id、revision、correlation、固定输入/配置摘要、workflow/step、当前执行/Attempt 来源身份摘要、Run status、new_inputs_allowed、原生结局、结果可用性、command 回执、交互问题、恢复/停止资格及已知 PR/result。内部数据库行或不透明 adapter 材料不是公共 DTO。

| Run status | 含义 |
| --- | --- |
| queued | 已接受，等待本 Run 准备/资源调度 |
| running | 正常准备或某步骤执行中；细节在 phase/step 和原生事实中 |
| waiting_input | 有绑定的业务或原生问题，需要新回复 |
| blocked | 已知环境、权限、前置或业务问题；给出原因与可用动作 |
| reconciling | 原操作、活动或结果未知，正在只读核验；不是可以重发的许可 |
| pr_ready | 本次 PR 已核验就绪，达到执行交付里程碑；仍可接受后续反馈 |
| stopping | 已接受停止意图，实际停止尚未全部确认 |
| stopped | 已核实本 Run 的活动执行停止；现场与历史保留 |
| failed | 本次执行明确失败，保留失败来源；是否可恢复由资格另行说明 |

原生结局、完整 final 是否可取得和业务结果分别记录，不用单个 success 混合表示。缺失/截断/冲突的 final 明确标 gap，不能取屏幕、最后文本或模型自述填补。

`reply.input.kind` 为 `answer` 或 `feedback`，均含 expected_revision 和 text。answer 必含当前 question_id，核验该问题的原 Run/Attempt/回合/call；保存答案不等于投递或新回合开始。feedback 适用于 pr_ready 或允许业务反馈的等待状态；活动回合仅在 adapter 声明并核实支持时发送，否则明确返回能力/状态冲突，不暗中排队成未来授权。

PR 就绪不会自动销毁会话/工作区。反馈在同一 Run、原会话、原分支和同一 PR 上进行；保留以前结果，创建新的输入/步骤记录。同一 Run 返回另一 PR URL 时拒绝替换绑定。原会话不能安全继续时保留现场，调用方可提交新 Run，不隐式改投后端。

## 8. 操作可靠性、恢复与停止

外部副作用前先提交固定操作意图和原目标；意图提交失败时零执行动作。外部调用不进入数据库事务。回执/结果、相应步骤转移与下一输入意图一次提交；持久化失败后只核验原操作并补提交，不重复副作用。

request_id、Run、Attempt、操作、会话/原生回合和原生资源身份分别绑定。连接丢失、超时或进程退出不能直接认定原操作失败或原轮次成功。对 unknown 原操作只读对账；不能换 ID 再发、重新建会话/工作区或自动发布另一个 PR。

服务重启先恢复观察和原操作/结果对账。受重启影响的既有 Run 建立持久的人工继续门槛：服务启动及轮询不自动发送 prompt、答案、下一步骤或重建执行资源，重启前未送达输入仅保留为事实。原执行可能在服务离线时继续运行，界面按实际状态显示；已完成结果可以只读接受与保存。

`resume` 是当前恢复状态下的新显式授权，不是盲重放。仅在原身份/操作状态核验到足以安全继续后执行；原会话可核实时允许新 Attempt/进程恢复同一原 thread。旧写者及其可能继续写入的活动必须先确认结束。原会话、准备中的原目标或未知副作用不可核实时阻断，不新增替代会话。再次停服后尚未消费的继续授权不能绕过下一次门槛。

连接重接与服务重启分别表达：服务实例仍有效时可以自动重接原观察并继续既定正常流程，不因此增加一次业务输入；服务换代执行上述人工门槛。其他新 Run 的合法 submit 不被一个旧目标的不可达阻断。

`stop.input` 含 expected_revision。接受 stop 的同一事务先保存 `new_inputs_allowed:false`，使未发送的准备、步骤、答案和恢复意图失去投递资格；随后才按后端真实能力停止原执行。已经发送的动作不能撤回，不宣称所有原生工具立即停止。迟到结果可核验和保存，但不得自动产生或发送下一输入。

后端原生停止 unsupported 时仍保留这次授权撤销，命令停止部分记录 failed 及明确能力缺口，Run 转 blocked；实际活动未知时命令停止部分为 unknown，Run 保持 reconciling；不能长期伪装为正常停止进度。请求已接受、新输入被禁止、原生停止已确认、本地观察停止分别记录。不支持原生停止的 Orca 目标不能返回 stopped；保留活动/未知事实。只操作本 Run 精确拥有的资源，不能根据模糊 PID、路径或终端名结束其他执行。

授权撤销跨服务重启保留，业务反馈/旧答案不能绕过；只有符合原身份/旧写者/未知动作门槛的新 resume 授权才能重新允许输入。非法或过期 stop 在接受前拒绝，同请求重试返回原撤销回执。调用方 Task 终态后不会发这种重新授权。

stopped/failed 的后续 resume 仅在上列资格满足且仍有原会话时可用；历史失败/停止记录不能覆盖成从未发生。首版不自动删除工作区、分支或 PR；原生清理失败另记，不撤销已核实 PR 结果。

## 9. PR 成果与 SteerLine 协作

本地 Windows 使用固定 MXC 权限资源契约。app-server 的宿主临时目录位于所属 Git 管理目录；实际工具的 TEMP/TMP 由原生 AppContainer 管理，Runner 不要求它们等于宿主路径，也不通过新写入根或手动环境覆盖绕过容器隔离。验收须核实真实工具 token、操作系统返回的容器目录内临时文件读写及未授权兄弟目录拒写。原生依赖自有认证、会话历史、缓存和内核临时目录不成为 Runner 恢复权威；Runner 自身状态、日志及冻结材料仍归 state-dir。

GitHub worker 的 W05–W07 新增可信执行主机认证准备与确定性原生沙箱检查，
固定权限改为 workspace-write/MXC/network enabled/approval never，关闭 shell snapshot。
不再申请自动审核发布。单仓库认证代理、进程环境、期限、失败零业务输入及远端能力
边界见[认证契约](sandbox-authentication.spec.md)；W16 原发布责任和未知结果对账保持。
本机可执行此准备；Orca terminal 缺同沙箱确定性能力时在业务前阻断，不接受模型检查声明。

Runner 按冻结 publication 配置和当前实际工作分支推送，创建或复用该分支 PR。模型返回 URL 后，provider 需核验平台实例、仓库、base/head 仓库及分支与 Run 的绑定一致，才能进入 pr_ready。URL、仓库、number、查询状态和可取得的 revision/时间及诊断作为 PRFact 返回。

PRFact 是带来源的观察事实；Runner 的业务结果不是 TaskDelivery。inspect 默认返回持久快照，不保证读取时已经刷新平台状态。`inspect --run-id ... --refresh-pr` 可有界查询已绑定 PR 并持久化新的观察，不推送、发布、合并或继续代理；查询失败保留已核实的历史绑定并返回 query_error/观察时间，不将旧事实冒充最新。未绑定 PR 时明确无可刷新目标。PR head 可因反馈变化，不增加历史候选 SHA 锁；保留固定 workspace/base 和 PR 身份检查。当前 head 与历史不一致、缺可选 HeadRevision 不单独阻止合法反馈/发布。

发布响应未知时查询原配置/分支是否已有 PR；无法核实就保持 reconciling。发布补报不能再发布。不能用报告 done、CLI 退出 0 或任务测试通过代替真实 PR 存在/归属核验。

SteerLine 接入流程：

1. 在自己的事务中固定任务资格、候选及待提交请求 ID/正文；经 CLI submit。同请求重试返回原 Run，Runner 不解释领取。
2. SteerLine 幂等保存候选与 Run 的关联；响应丢失时用原请求对账，不能新增候选规避未知。接受之后调用方失败不使 Runner 回滚已执行的副作用。
3. 经 inspect/events 获取已提交快照，按 Run/request/event 关联和游标去重。SteerLine 只保存自身需要的事实，不复制另一套可推进的执行状态机。
4. PR 实际合并后，SteerLine 依据固定 provider/PR 绑定核验最新平台事实，在自己的事务中写唯一 TaskDelivery、Task closed 和后继释放。可消费 Runner 查询能力，但必须核验事实来源、完整性和归属；通知不是交付证明。
5. Task 关闭/丢弃后的执行处理由 SteerLine 发显式 stop；Runner 不订阅任务数据库。调用方先持久化待 stop 意图、按原 ID 对账直到得到真实处理结果。其客户端停止新回复/恢复，界面如实显示仍在运行或未知的原执行。只有 Runner 已提交授权撤销回执后，才能证明它不再派发新输入；Task 终态与这份回执之间有通信窗口，调用方不可显示全系统已停。TaskDelivery 不与远端停止组成跨库事务，停止失败不撤销真实交付。SteerLine 对 closed Task 停止 PR 合并查询，待 stop 对账独立进行。

SteerLine 使用单独的 CLI 客户端模块；每次启动短进程，传参数与 stdin，解析一个响应并检查退出码，不持有原生代理或 IPC 私有材料。目录内虽有两份数据库，但分别为不同领域的权威，没有跨库事务、双写或默认读取 fallback。

## 10. 代码归属、版本与交付安排

SteerRunner 内部分工为公共 CLI/DTO、服务与存储、Workflow、ReliableExecution、ExecutionAdapter、共享 workspace/publication。Workflow 不解释技术投递/身份；可靠执行不解释 Spec/Plan、业务步骤或 TaskDelivery；adapter 不持有业务写权限。通过独立宿主组装原子提交和副作用执行。

两仓各自构建、测试、发布；SteerLine 固定一个明确 Runner 发布版本及 `steerrunner-cli/v1`。首版开发期拒绝不支持的存储/配置/协议格式，在任何业务动作前报错，不读取旧 SteerLine 数据库、不迁移或双写。升级/安装不自动接管正在运行的旧服务，二进制运行中不强行替换；完整版本管理另行设计。

代码抽取实施 Plan 必须记录固定来源 commit、迁出/保留/删除清单、三层重构实际交接、Runner 独立验证及 SteerLine 切换 PR 的最小测试。切换 PR 接通客户端并删除原执行实现，合入前固定已经可取得的 Runner 发布版本；临时开发复制不形成长期双维护路径。

首版 Windows/amd64 安装产物为一个 EXE；CLI 查询实际程序/协议/资源版本。其他 OS/架构未验证时不宣称支持。原生 adapter 和 provider 的 required/unsupported/unverified/not_applicable 声明及现场核验保持区分，不把未知、零值或提示词宣称当成 ready。

## 11. 验收与本次文档交付

自动验收 SR-A01–SR-A12 与人工项 SR-M01 统一见[专属验收](../acceptance/steerrunner.md)。只验证本次独立化及实际支持能力；受控/native/live 证据分开，旧 SteerLine 验收不搬成 Runner 通过结果。不默认跑 Full 或完整真实模型矩阵。

当前文档阶段交付 README、此 Spec、实施 Plan 和专属验收文档，并核对链接、示例 JSON 和覆盖关系；不复制源码、不运行服务或真实模型、不创建/合并 PR、不修改 SteerLine 或安装环境。运行验收入口尚未存在，Plan 标明待建设的真实场景/命令、建设步骤与执行时点；先落实能力，再执行并登记结果。

## 执行可见性

`workflow_changed` 事件在 Run 提交事务内比较前后值，仅 workflow stage/status/diagnostic 发生变化时追加。`transition.from/to` 各含 `workflow_stage`、`workflow_status`、可选 `diagnostic`；外层保留原 event_id、Run/Attempt、revision、created_at。初次提交 from 为空。不用当前 Run 重画缺少 transition 的历史。

`last-message` 返回的 `observed_at` 为现有观察流收到原生通知时的 UTC 时间，不是查询时间；同一摘要重复读取时间不变。只采样本地 Codex completed agentMessage 或固定工具标签，不暴露 reasoning、工具参数/命令/输出或用户输入。Run/Attempt/Generation/Session/Turn 验证与脱敏后，每 Run 只保存一条 1200 UTF-8 字节加省略号的摘要。乱序/过期样本丢弃，不推进 revision/events。服务重启缓存清空，Orca 无样本；空值与读取错误都不能用来判断失败、卡死或恢复资格。

客户端按当前显示的执行按需查询；SteerLine 在页面可见且在线时每 3 秒读取，后台/切换/离开时停止。具体边界与最小验证见[增量 Plan](execution-observability.plan.md)和[验收](../acceptance/execution-observability.md)。
