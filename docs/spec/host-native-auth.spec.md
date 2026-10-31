# Spec：开发执行使用主机已有认证

> 2026-10-31 parser 增补：独立解析已改为复用 Codex 登录/CLI 的内存文本任务；旧 parser operation、owner 接管、工具权限及停止取证条款由 [Parser 解耦 Spec](parser-decoupling.spec.md) 取代。工具禁用和进程清理只在 parser 内；其余主执行契约与历史证据结论保持。

日期：2026-10-31。状态：代码已实施，回归与原生资格按[验收记录](../acceptance/host-native-auth.md)单列。审计及用户允许丢弃旧记录后的收缩见 [Plan §8](host-native-auth.plan.md#8-文档审计记录2026-10-31)。

调查基线：`cf8df21374555bde90baf081c21f4d9581580f42`（已包含开发 sandbox 退役及后续原生提交修复）。实施见 [Plan](host-native-auth.plan.md)。

## 1. 目标和范围

SteerRunner 是个人可信主机上的多机执行器。开发 worker 使用实际执行账号已有的 Git、gh 及操作系统凭据设施，Runner 不再主动提取、代理、签发或撤销开发仓库凭据。删除单仓库 broker、临时 CA/API socket、capability/期限、认证控制和认证取证链路，减少启动、续执行、停止和测试之间的耦合。

适用于 `development/2`、`development-audited/2` 的本机 codex-app-server、Orca local 受管 Codex/Pi/DSH。Orca environment 原生路径已经使用执行主机登录，保持现有行为和支持组合。本任务不扩大 Agent、模型、provider、仓库 URL 或远端协议的支持范围。

保留显式 `full_access`、`approval_policy=never`、独立工作区、固定基线、输入去重、未知副作用零重发、原执行身份、业务状态机和 PR 核验。内容 workflow、仓库只读调查、独立 parser 的 read_only 与环境/工具限制保持；用量日志的 collection permit、selector 和租约不属于开发认证，不删除。

本 Spec 实施后取代 [full_access Spec](development-full-access.spec.md) DFA-03 及 DFA-04/05 中开发 broker/认证清理的适用条款，并取代[沙箱认证](sandbox-authentication.spec.md)、[通用 Agent](orca-generic-agent.spec.md)、[服务换代恢复](service-generation-recovery.spec.md)中与本次边界冲突的开发认证要求。用户已明确旧记录可以丢弃：本版本使用新的状态目录和 state/v6，不提供旧记录迁移、读取或执行兼容。历史仓库证据保持原结论。本轮不实施通用恢复简化、结果获取改造、测试框架重构、新凭据管理器或新的认证模式选择器。

## 2. 当前问题与删除结果

| 基线调用点 | 当前负担 | 完成后的行为 |
| --- | --- | --- |
| Codex `authentication*.go`、`worker_runtime.go` | 宿主 gh token 转换为有限 capability，经代理/CA/socket 传递，注入 Git header，维护期限 | Git/gh 自行使用执行账号已有登录；Runner 不执行 `gh auth token` 或读取凭据文件 |
| `backend.go`、`sandbox_readiness.go`、`policy_resources.go` | 业务输入前绑定认证目录并完成实际工具内的认证、Git transport 和权限探测 | 保留协议、工作区与工具可用性检查；不以认证网络探测或权限 canary 作为业务启动门禁 |
| `remote_agent.go`、Orca `local_authentication.go` / `control_authentication.go` | 本机 Orca helper 受理 readiness 后仍核对租期，停止还需认证控制及关闭证明 | 原 helper 仍持有 native session/Job；发送与停止只依赖原执行资格 |
| `terminal_runtime.go`、`terminal-tools.mjs` | Pi/DSH 工具准备依赖 auth_probe、期限及认证关闭字段 | 仍用现有工具宿主/Job，工具环境使用宿主已有认证；不准备 broker |

验收必须看到上述生产调用和旧测试义务实际消失，不能只设置 `Authentication=nil`，或保留一套永远不启用的 broker 实现。

## 3. 行为契约

### HNA-01：认证属于执行账号

1. 认证在实际执行主机解析。本机使用启动服务/owner 的账号；Orca local helper 使用 Orca 执行账号；Orca environment 使用远端已有账号。不同进程的环境或登录不一致时直接报告实际失败，不回退调用方主机。
2. Runner 不主动登录、登出、刷新 token、运行 `gh auth setup-git`，不修改用户全局/原仓库 credential helper、账户配置、证书或代理设置；不从调用方复制 token、凭据目录或钥匙串。原生 Git/gh/helper 自身按已有账号配置正常读取或维护登录缓存仍可发生，本任务不接管其认证机制。原有私有 clone/worktree 的 remote/base 初始化保持，不误解为禁止 Runner 设置新工作区的既定 remote。配置发现和离线 validate 不访问认证网络。
3. Git 使用该账号已配置的 credential helper/系统凭据；gh 使用已有配置目录或工具本身支持的宿主环境变量。Git 与 gh 必须分别可用，gh 已登录不代表 Git push 已配置完成。独立 PR 查询仍由现有 publication provider 在 Runner 服务主机执行：Gitea 查询读取其 `authentication_refs`/TokenEnv 并设置请求认证头，command provider 沿原命令及环境查询；这些不是待删除的 worker broker，不改成一律使用 gh，也不从远端 worker 取得凭据。本 Spec 的“不主动提取凭据”仅约束新增的开发 worker 认证实现，不禁止保留的 provider 查询和既有脱敏。
4. 不新增 `execution.settings.authentication` 或 broker/native 双模式开关；旧退役 `github_auth` 等配置继续拒绝。原 RunConfiguration 的 `access_mode=full_access` 及其余非认证字段不变。
5. full_access worker 可以使用执行账号已有的权限。Runner 不承诺单仓库凭据隔离、每 Run 撤权或抵御恶意 worker；停止不退出账号，也不影响其他 Run 的登录。

### HNA-02：环境与工具配置

认证通过子进程环境和账号配置自然继承，不序列化到请求、DB、profile、config.json、argv、报告或日志。Runner 不读取 token 后转换为 `http.*.extraheader`，不生成假的 GH_CONFIG_DIR、代理证书或临时认证 HOME。

新开发路径采用以下固定规则，不引入可配置环境策略引擎：

| 环境组 | 行为 |
| --- | --- |
| 原运行所需 PATH、系统目录、USERPROFILE/HOME、APPDATA/LOCALAPPDATA、CODEX_HOME、已绑定 TEMP/TMP | 保留既有运行时职责；使用实际执行账号路径，不复制配置内容 |
| 已有 gh 认证选择 | 允许继承 `GH_HOST`、`GH_CONFIG_DIR`、`GH_TOKEN`、`GITHUB_TOKEN`、`GH_ENTERPRISE_TOKEN`、`GITHUB_ENTERPRISE_TOKEN`；支持范围仍按原 provider/URL 限制 |
| 已有 Git 凭据/传输配置 | 保留 `GIT_CONFIG_GLOBAL`、`GIT_CONFIG_SYSTEM`、`GIT_CONFIG_NOSYSTEM`、完整且合法的 `GIT_CONFIG_COUNT/KEY_*/VALUE_*`，以及 `GIT_SSH`、`GIT_SSH_COMMAND`、`SSH_AUTH_SOCK`、`SSH_AGENT_PID`；不扩大 SSH 支持声明 |
| 主机网络配置 | 保留原有 HTTP_PROXY/HTTPS_PROXY/ALL_PROXY/NO_PROXY、SSL_CERT_FILE/SSL_CERT_DIR、GIT_SSL_CAINFO/GIT_SSL_CAPATH（按 Windows 环境键大小写不敏感）；不写成 Runner broker 地址 |
| 无交互执行 | 固定 `GIT_TERMINAL_PROMPT=0`、`GH_PROMPT_DISABLED=1`、`GCM_INTERACTIVE=Never`；保留命令期限，不能停在登录/确认对话框。Git helper 的非交互配置通过进程级设置，不改宿主文件 |
| 工作区与控制标识 | 保留原 Run/session/hooks/progress/result 绑定；不继承会重定向仓库的 `GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE/GIT_COMMON_DIR` 等现有禁止项，也不放开 ORCA 配对/远端控制凭据 |
| 模型 provider 凭据 | Agent 自身沿原提供方配置；Pi/DSH 通用文件/PowerShell 工具仍过滤 OPENAI/ANTHROPIC/OPENROUTER/DEEPSEEK 等原 provider 密钥，不能为仓库认证改成继承全部环境 |

Codex app-server 及其原生 shell 工具必须都能看到上述适用环境，不能仅让 server 继承、shell policy 再剥离。开发 shell policy 的允许变量名、ignore_default_excludes 与环境透传必须在删除 Auth 分支后独立生效；只读/parser 不能套用这份开发 allowlist。允许 shell policy 固定变量名及不含秘密的运行参数，禁止将 token、header、代理密码或任意继承的 GIT_CONFIG_VALUE 写入 `--config ...set...`。保留运行期已知 token 脱敏及有界诊断，不为了脱敏枚举凭据存储。

Orca local Codex 继续仅在 `OrcaHooks` 生效时传递原有限 hook/launch 身份，不放开 ORCA_*。继承已有 NO_PROXY，并仅在需要本机 hook/bridge 时合并原本必需的 loopback 例外，不覆盖主机其他代理例外。审计工作流的 RemoteModelBridge、桥接 token、同 thread 的模型/effort 切换和恢复实施模型保持；桥接 token 只进入原 TUI 接口，不能混入 Git/gh 凭据或通用工具环境。

Pi/DSH 沿现有 `SR_TERMINAL_TOOL_ENV` 内存传递通道构造工具环境；不将该值落盘或打印。DSH 的 Agent HOME/profile 隔离可保留，但工具使用准备时保存的真实执行账号 HOME/USERPROFILE/GH_CONFIG_DIR 等，不能在 DSH 隔离 HOME 上重新解析主机登录。环境去重、大小写和 GIT_CONFIG_COUNT 合并由固定组装函数处理；继承计数含空值、多个配置项时仍保持原序，不把原有配置整体覆写为 Runner 的三个 Git 设置。

### HNA-03：准备和失败

保留本机/原生协议握手、可执行文件、精确工作区/分支/base/remote 识别、必要 Git 可用性与现有工具宿主自检。删除开发业务输入前的 gh 身份/API/ls-remote 认证探测、token 到期检查、AppContainer/full_access 取证及工作区外写入 canary。不新增模型回合、远端 helper 或探测队列。

仓库材料已经可用时，没有可用发布登录仍可进行本地实施；仅凭未登录不能在首个业务输入前拒绝 Run。取得私有仓库基线所需的真实 fetch 仍可能先失败，不绕过材料可用性要求。实际 fetch/push/PR 请求失败时沿现有失败/等待入口保存原因，由用户处理执行主机配置。已知失败与结果未知分别处理：已知认证失败不能自动登录或重复发布；超时/回执丢失仍只查询原操作，不因认证调整重发。真实外部操作未成功不能登记 pr_ready。

执行路由保持：Orca local 的受管 Codex helper 路径仍是原 app-server→tool-free READY bootstrap→原 TUI resume；原非 GitHub 的无 helper 直接 TUI 路径不强加 helper。Orca environment 仍直接原生启动。删除认证预检不能删除 readiness 的 session/provider/运行构建身份、原 `runner_input`/`runner_bootstrap`、READY 的完整完成以及原输入预算/回执。所有后继仍发送给同一原执行。

认证期限不再是新输入资格的一部分。模型登录、原执行是否 idle/可恢复、业务授权、stop、来源与未知操作资格仍由原机制决定。本任务不额外增加新的全局状态机或错误协议。

### HNA-04：停止与重启

新 Run 停止仍撤销后继输入、停止原 owner/Agent/工具树并核验原资源范围。删除 broker/socket/CA/capability 关闭、认证过期和 `authentication-stop` 子动作；没有认证资源时不生成 `authentication_closed=true`、虚构到期时间或空的清理证明。

服务被动退出保留原 owner 和工作流授权的现有语义；同一新契约下重启可接回原执行。主机登录/环境不是 Runner 冻结凭据：已有进程继续使用创建时环境，新进程使用其启动主机的环境；Runner 不为读取更新凭据主动替换活动执行。不因停止一个 Run 或服务停止而更改宿主登录、代理或其他 Run 的配置。

原生会话、原 final、审计 checkpoint、输入回执、签名退出、Job 和控制通道的执行身份规则保留。owner RPC token、Orca hook token、日志读取 permit 不是 GitHub capability；名称相似不能成为删除依据。

Orca local 的控制终端收尾必须解除对 `ReadAuthenticationStopEvidence` 的依赖，不能只删调用或令它返回 nil。继续使用既有 control checkpoint，升级为 version 3：绑定冻结 control setup、原控制进程及 Run/Attempt/config 身份，不能以可空的 Control.Ready 或后来才加入的业务 readiness 作为路径的必要部分；Request/Response 可为空，以覆盖尚无查询和初始化失败的关闭。Control.Ready 尚不可读时沿现有精确身份关闭规则处理，已有 Ready 必须与冻结 setup 一致。关闭前如有原请求，须按已有原 ID 查询/缓存规则取得结算：original-turn 的 ReaderWritersAbsent、workspace-audit 的 WritersAbsent 均成立才可关控制终端；错误响应不能抹去 reader 未结算，无法确认时保留 unknown。无原请求可以直接关闭精确原控制终端，不为凑证明新发模型或查询动作。真实 close 受理及原终端缺席确认后保存 Closed；离线 verify-stopped 按该关闭事实、原终端和适用的业务 helper 签名 writer-tree 退出核验，未启动业务 helper 时沿原初始化停止资格，不伪造退出。控制关闭回执/保存结果未知不得重复 close、补建控制终端或伪造 Closed。

### HNA-05：新版本和状态目录

用户允许丢弃旧记录。本次统一以 state/v6 划断旧数据库，不实现旧 Run 的投影、被动解码、精确停止适配或新旧混合目录。新版本只接收自己创建的新格式记录；新格式内仍保留正常重启、原操作对账和资源停止能力。新建只读内容及原生远端执行行为保持，不承诺从旧 state/v5 接回它们。

| 记录/契约 | 本次固定边界 |
| --- | --- |
| 本机 Codex 新开发 | policy `codex-policy-v12:0.159.2:full-access:host-native-auth:never`；development worker-profile codec 4、policy-resource codec 5；新资源不含 authentication_transport |
| Codex read_only | 在新 state/v6 中保持 v10 read_only contract、profile codec 3、policy-resource codec 4、parser 独立契约；不得用新的全局开发常量校验只读数据 |
| Orca local 新 Codex/Pi/DSH | 使用 host-native local profile codec 6，区分原 profile 3/5。Codex helper 使用 `codex-remote-inline-v4` 和 v12 policy，readiness/exit 不含认证控制、到期或关闭字段 |
| Pi/DSH 工具及回执 | tools `windows-terminal-tools-v3:full-access:host-native-auth:never`，metadata 5、receipt 4、扩展 config.json 版本 2；新的 preparation 不含 auth_probe/expiry/权限 canary，仍记录必要工具与 provider 环境自检；startup-exit/exit 新版本 2，删除 AuthenticationClosed |
| Orca terminal-state | 新 local profile 6 写 state codec 8；原生 environment profile 4 仍写 state 7。编码由已验证 profile 选择，不能只把全局 StateVersion 改为 8。所有 state 读取、usage 绑定/摘要、checkpoint、RecoverySupport 统一处理对应组合，不用宽松 JSON 解码吞掉不匹配字段 |
| helper 元数据/控制 checkpoint | 沿现有 `--terminal-metadata` 发布 metadata 5，并加入 host-native 认证契约、Codex helper transport v4/policy v12 信息；本机 helper 启动前校验选定 EXE 的新元数据和 SHA，拒绝旧/混搭 helper，零 Agent/broker 启动。控制 checkpoint version 3，保留原查询身份/响应绑定和流读取，不再要求最后一条请求为 authentication-stop |
| 本机 execution owner | RPC、请求和签名退出字段形状保持，owner v1 与 binding codec 2 不因认证删除单独升级；用冻结开发 profile/policy 断代，不靠 owner v1 推断认证兼容 |
| CLI / workflow / DB | 保持 CLI v1、development/audited v2；DB 为 `steerrunner-state/v6`、PRAGMA user_version=6，表结构无额外重构；RecoverySupport 的可读/可写格式只发布 v6。资源摘要反映格式/支持变化 |

驻留服务、离线查询、takeover、verify-stopped 等入口在通过原有参数/实例准入并实际打开旧 v5 或更早数据库时返回现有 unsupported_schema，保留既有错误优先级；拒绝发生在 Engine/collector/原生执行 I/O 之前。不自动迁移、改 metadata/user_version、建空库覆盖或返回“无 Run/资源已结算”。使用一个全新目录；新目录空闲不证明旧目录对应的进程已停止。只为支持旧数据的 decoder、stop-only/退役投影及测试可以在零新消费者后删除，无需新增旧数据识别层。

升级前由原版本完成或停止旧活动执行并结算原资源；随后换新组件、新 state-dir、新 request_id。不要求保留旧历史查询或安排记录迁移，也不自动删除旧数据库、工作区、运行时镜像或宿主登录。client/skill 保存的 state-dir/instance 绑定须显式更新，错误旧目录给出“改用全新 state-dir”的具体诊断；不静默换目录或误绑定现有服务。本轮仅修改文档，不执行磁盘数据删除。

### HNA-06：保留边界与完成标准

| 验收项 | 必须证明 |
| --- | --- |
| 认证自然继承 | 本机 Codex 与 Orca local helper 的实际 Git/gh 工具能使用主机配置；DSH 隔离 HOME 不使工具丢登录 |
| 删除闭合 | 新开发零 broker/CA/socket/认证租期、零 auth_probe/authentication-stop/认证关闭证明；删除无消费者实现及旧测试义务 |
| 不暗改宿主 | 除保留的 provider 查询/脱敏外 Runner 不主动提取/输出 token，不更改用户登录、credential helper、证书、代理或原仓库 remote；私有 clone/worktree 初始化保持，诊断脱敏，非交互失败有期限 |
| 失败诚实 | 材料已可用时无发布登录可实施；真实认证失败明确等待/失败，无虚假 PR；未知发布只查原操作、零重发 |
| 格式断代 | 旧目录只报不支持，零执行/迁移/伪清理；client 指向全新目录；新 v6 的开发、只读、parser、原生远端和重启可用 |
| 核心执行保持 | 新 Run 重启观察、停止零后继、双 Run 停止互不影响、独立 PR 查询认证及 repo/base/head 核对、最新原生 paste+Enter、bootstrap 与审计模型桥保持 |
| 支持声明 | 代码、受控验证与真机组合分别记录；无环境的项记未验/incomplete，不引用旧 broker 构建通过充当新资格 |

完成时更新 README、CLI、skill、配置/测试指南及被取代 Spec 的当前适用范围，新增验收记录。历史 evidence/archive 不改写。下一轮阶段结果获取和独立 parser 简化不并入本任务。
