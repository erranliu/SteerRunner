# Plan：开发执行使用主机已有认证

日期：2026-10-31。状态：代码已实施，验证范围见[验收记录](../acceptance/host-native-auth.md)。依据 [Spec](host-native-auth.spec.md)，调查基线 `cf8df21374555bde90baf081c21f4d9581580f42`。旧记录按用户明确要求不做兼容，见 §5；审计发现和修正见 §8。

## 1. 实施顺序

| 阶段 | 产物 | 对应契约 |
| --- | --- | --- |
| P0 | 调用清单、新版本表、新目录与组件匹配规则 | HNA-01/05 |
| P1 | 开发 host-native 环境及真实工具接线 | HNA-01/02 |
| P2 | 删除认证执行/准备/停止链路，写入新格式 | HNA-03/04/05 |
| P3 | state/v6 格式断代、客户端切换与新记录恢复 | HNA-04/05 |
| P4 | 测试义务替换、受控回归、当前文档 | HNA-06 |
| P5 | 固定最终构建的原生专项与资格记录 | HNA-06 |

后续实现只做本任务内的认证减法。不得为了保留旧活动任务自动接管而恢复旧 broker，也不趁机更改工作流、未知操作语义、原生输入/完成识别或用量读取授权。本轮不执行部署或修改真实安装/账户配置。真实 PR 交付须使用明确的专用测试配置；缺配置时单列未验。

## 2. P0：删除清单与格式

| 模块 | 动作 | 保留内容 |
| --- | --- | --- |
| Codex `authentication.go`、`authentication_broker.go`、`authentication_socket.go` | 删除开发 credential resolver（hostGitHubCredential 在 broker 文件内）、签发、代理、CA/API socket、租期、认证资源构建 | 仅共享且仍被只读/通用工具使用的纯函数移至实际使用处；不保留关闭的认证子系统 |
| `config.go`、`launch.go`、`backend.go`、`lifecycle.go`、`service_lifecycle.go`、`worker_runtime.go` | 删除 Authentication/AuthenticationTransport、prepare/expiry、Git header/helper 覆盖及 broker shell 参数 | app-server 启动、模型、shell、Git maintenance/longpaths、temp、hooks、Job 与输入预算 |
| `sandbox_readiness.go`、`terminal_authentication.go` | 删除开发认证/权限探针；拆出仍有消费者的错误包装或 Git/工作区检查 | read_only 的 `content_readiness.go`、协议可用性和精确目标检查 |
| `remote_agent.go`、`remote_exit.go`、`remote_authentication_control.go`、`remote_control.go` | helper 直接用主机环境；删除 watcher、authentication-stop 和认证证据 | 同一 app-server/thread、原 turn/history、workspace-audit、固定控制通道、签名退出 |
| Orca `local_authentication.go`、`control_authentication.go`、lifecycle/stop/verify_stopped | 删除发送/续执行租期检查和认证关闭步骤 | 原终端/session/Job 归属、停止/工具树/reader 结算 |
| `terminal_runtime.go`、`terminal_receipt.go`、extensions、`terminaltools` | 新格式去掉 AuthenticationState/auth_probe/expiry/closed；工具环境直接从执行账号构造 | Pi/DSH 原生 TUI、脚本路径、provider 过滤、临时文件清理、工具 Job 与 DSH profile |
| application `freeze/policy/takeover/engine/worker` 及 storage/读取侧 | 新契约写入、state/v6 格式拒绝；去掉权限资源中的认证 transport | 新记录的单一 Record/Commit、固定材料、业务授权/去重、恢复与停止 |
| `cmd/steerrunner-agent/main.go` | 删除新构建 `--authentication-stop` 命令和消费者，旧命令明确拒绝且无执行副作用 | owner/helper/tool/runtime metadata、原 turn 和 workspace 查询入口 |
| resources/contracts/testkit、usage codec 消费者 | 发布 state/v6、新 profile/state/metadata 及 stop 断言；校验新增 codec 的 usage 绑定/摘要 | collection permit/lease/selector、publication provider 的现有查询认证与脱敏 |

旧记录不构造逐版本样本和迁移分类。只保留一个 v5 SQLite 拒绝夹具，验证旧目录不会被新程序打开为可执行/空白目录；新建 v6 fixture 覆盖开发、只读、parser、原生远端、活动/初始化失败/未知输入等新记录。所有新测试使用全新目录，不能复制生产 v5 数据后改 metadata 伪造 v6。

按 Spec 版本表定义新编码和严格 decoder。新 local profile 6 必须按 Terminal 有无及 RuntimeExecutable 是否存在区分 Codex helper、直接 TUI、Pi/DSH，不得与 environment profile 4 混淆。`envelope/state/decodeState/UsageBinding/UsageInputDigest/RecoverySupport` 必须一致处理 local state 8 和 environment state 7；不要用一个新全局 StateVersion 改变远端写入，也不要让新全局 ProfileCodecVersion/PolicyContract 拒绝新建 read_only 的原格式。

为本机 Codex helper 复用现有 `--terminal-metadata` 无副作用入口：metadata 5 新增认证契约与 Codex transport/policy 信息；freeze 在创建 Agent/helper 资源前读取并绑定选定 EXE 的 SHA，Pi/DSH 使用同份 metadata 的 tools/bundle 字段。旧 metadata、空 SHA、配置与所选二进制不符均拒绝；不等到旧 helper 已创建 broker 后才从 readiness 判断不兼容。直接 TUI 和 environment 路径不增加该前置。

## 3. P1：host-native 环境

1. 在现有执行环境组装位置实现一个小型、无 I/O 的开发环境函数，供本机 Codex 和 Orca local 工具使用。输入仅为实际执行进程环境及已有 Run 运行参数；不接受 submit 传入秘密、回调凭据解析器或动态 allowlist。
2. 按 HNA-02 固定环境组处理。保留 Git 原配置 helper、用户配置路径、既有代理/CA，合并完整 GIT_CONFIG_COUNT 且不把继承值编译到 argv；新增非交互值和既有公开 Git 运行参数时正确保留索引。继续清除仓库重定向和跨 Run 标识。
3. Codex `worker_runtime.environment/configArgs` 移除 Auth 分支与头注入；开发 shell_environment_policy 的 inherit/include_only/ignore_default_excludes 和 OrcaHooks 接线独立保留，不可因原先位于 `if r.Auth != nil` 内而一起消失。允许 pin 公共 shell/PATH/temp 及固定非秘密设置，不能 pin 主机 token、带密码代理、继承 GIT_CONFIG_VALUE。app-server 模型环境与 shell 工具环境分别验证；不全局放宽 content/parser 或通用 process host。
4. Pi/DSH 工具环境先从原 os.Environ 构造，在 Agent 侧隔离 HOME 之前保存。沿现有环境通道传到工具宿主，去掉 `auth.Environment` 依赖。保留 provider key 过滤，兼容既有 GIT_CONFIG 及模型自身所需环境；DSH TUI 仍可用专有 HOME。
5. publication/gitea.go 读取服务主机 TokenEnv 生成查询请求认证头、command provider 的现有命令环境和 authentication_refs/脱敏保持。这是 PR 只读核验链路，与 worker 主机原生登录分别验证；不能把 provider 的 token_env/Authorization 一并删除或替换为 gh。非 GitHub 直接 TUI 不新增 helper 或 gh 依赖。Runner 不运行 credential helper 的安装/登录命令，不制造 GH_CONFIG_DIR；不调用生产凭据做单元验证。
6. 保留 RemoteModelBridge、桥接 token 仅传原 TUI、原 thread/turn 的模型冻结与审计后恢复实施模型。主机代理继承时合并有限 loopback NO_PROXY 例外并保留原条目；hooks 仍只传原有限变量，不传播配对凭据。新增含代理、Orca hook、不同实施/审计模型的受控反例。

定向验证使用临时 HOME/GH_CONFIG_DIR、合成 token/代理 sentinel、测试专用 Git helper 和临时仓库，证明真实子进程继承与非交互失败。校验配置文件字节不变、sentinel 不出现在 argv/持久 JSON/日志，provider keys 不进入 Pi/DSH 工具。仅检查函数返回字符串不能证明原生 shell 继承。

## 4. P2：新执行链路与格式

1. 本机 Codex 不再在 lifecycle constructor、policy resource、PrepareSession/StartTurn 创建或检查认证对象。read_only 的 Config/资源生成独立保留；删除只供 broker 使用的 Config 字段、transport envelope 和 schema。
2. `remote_agent.go` 保留 app-server→原 tool-free READY bootstrap→同 thread TUI resume→原模型桥路径。只从 readiness 删除认证字段，保留 Input/Bootstrap、session/provider、构建/SHA、公钥、原生完成和输入预算。删除 prepareHostBroker、CA/socket、remote auth watcher、认证关闭及 ExpiresAt；按 v4 生成/核验。RemoteControl 保留原 turn/workspace 查询、流读取和请求摘要；删除认证方法时必须同时删除 dispatcher 的隐式 else→Revoke 分支，未知方法明确拒绝，不能落入 workspace 或 native stop。
3. Pi/DSH metadata 5/receipt 4/profile 6、工具 v3 和扩展 config.json v2 同时更新 Go/JS；Pi/DSH 的配置读取器及 receipt 生成一起升版。新配置不再含 auth_probe/authentication_expires_at/authentication_state。工具准备只做必要自检与 provider 过滤，去掉网络认证及 inside/outside 写权限 canary。startup/exit v2 没有 AuthenticationClosed；进程/工具树退出仍需真实证明。
4. 新停止只结算原执行、工具树、reader 等现有资源，不返回假的认证 revoked/closed。`verifyControlStopped` 当前最后调用 ReadAuthenticationStopEvidence；必须改为 Spec HNA-04 的控制关闭证据，不能保留此认证依赖或无条件略过控制结算。control checkpoint 升 version 3，用冻结 control setup、原进程及 Run/Attempt/config 生成稳定路径/绑定，不依赖可空的 Control.Ready 或业务 readiness；Request/Response 为空的初始化关闭也能存取，已有 Ready 仍核对。实现已有请求结算→原 control close→原终端缺席→保存 Closed；适用的业务 helper 签名/Job 退出独立核验，未启动时保留原初始化停止资格。Control.Ready/RemoteReadiness 未取得、无查询、最后是 original-turn/workspace-audit、查询失回执、reader 未退出、close/落盘结果未知及服务重启分别有反例。不得新增 close 查询、认证替身或补建控制 actor。
5. 删除开发派发、reply/resume、恢复、PR 结果对账中的凭据租期门槛；认证失败走现有 blocker/waiting 路由。禁止自动重试登录/发布、替换原 session 或清空未知 operation。
6. `policy.go/profile_config.go/policy_resources.go`、Orca profile/state、resources RecoverySupport 和 freeze 按 HNA-05 写入新版本。新生成能力/帮助明确 host-native，允许现有 capabilities 元数据说明，无需新增用户认证选项或 CLI 协议。原生远端 profile 4/state 7 和 read_only 不全局升版。
7. 删除生产认证实现与纯 broker 测试；逐项搜索到零新执行消费者后再删文件。共享执行权限编译、通用环境工具、owner RPC token、原生 hooks、usage lease 不删除。

## 5. P3：新状态目录与新记录恢复

1. storage.Format、创建 metadata、PRAGMA user_version、validateFormat 和 resources.RecoveryCapabilities 统一为 v6。复用既有格式拒绝边界，Open/OpenReader/verify-takeover/verify-stopped/服务启动实际打开 v5 时返回 unsupported_schema，保留原参数/实例准入错误优先级；不先加载记录或创建 worker/collector，不自动迁移/清空/备份旧库。升级仅改格式标识及原 schema 常量，不做数据库架构重写。
2. client/skill 配置中的 state-dir、expected binding、服务选择和 Viewer 快照来源改为新实例；离线配置校验不自动换目录。旧路径诊断给出明确切换说明，不能让错误旧路径的命令操作另一个新服务。沿已有 instance/目录绑定验证，避免新增第二套服务选择机制。
3. 新 v6 的 submit 去重、同新契约重启、原 pending 输入/停止/问答/parser、未知外部操作、progress/result/usage 继续按现规则；format bump 不等于删除新 Run 的恢复能力。用全新 v6 fixture 做 submit→重启→observe/继续/stop，不能只证明空库可打开。
4. 不实现旧 read_only/开发/原生远端记录的查询、混合目录或精确停止适配。新记录中 read_only profile 3/v10 与新开发 profile 4/v12 按模式分别选择；Orca environment profile 4/state 7 与新 local profile 6/state 8 按 target 分别选择。实际读取点和支持声明必须一致，usage 不能把 state 8 误判为 FORMAT_UNSUPPORTED。
5. 只为旧 v5 记录存在的 retiredDevelopment/stop-only/兼容数据结构及其测试可在零新消费者后删除；不能删除同时承担新只读权限、资源身份、原操作对账或后台采集准入的逻辑。原生远端不因格式断代增加本地 helper/认证前置。
6. 操作说明：原版本完成/停止活动任务→换新组件及全新 state-dir→更新 client binding→新请求提交。丢弃历史不代表旧进程/broker 已停；新目录“无 Run”只描述新目录。无需迁移或保留历史查询，但不得自动删数据库/工作区/运行时或退出宿主登录。本轮不执行这些运维操作。

## 6. P4：测试义务和文档

| 测试组 | 调整 |
| --- | --- |
| authentication_broker/socket/test、native 认证探针 | 删除 scope/capability/CA/socket/租期/撤权的正向义务；以 host 环境、无认证启动和配置不变测试替代，不保留空壳正例 |
| `TestAuthenticationMissingBlocksBeforeNativeStartAndBusinessInput` | 改为使用已具备材料的仓库，无发布登录仍能准备并完成本地实施；真正发布操作报认证失败，无虚假 PR；私有基线 fetch 的真实失败保持 |
| `TestAuthenticationExpiredReconnectRejectsNewInputOnOriginalSession`、Orca local lease 测试 | 改为没有 Runner 认证期限仍可接回/输入；主机实际认证失败由原操作报告，不使用 fake future expiry |
| `TestSRPermissionQualification`、InputFreeze、工作区/原生权限测试 | 保留 full_access/never、工作区身份、输入冻结及原 session/READY readiness；仅移除认证预检/租期及权限 canary 断言，内容/parser 的只读反例保持 |
| terminal receipts/exit、`TestManagedStopWaitsForOriginalExitAndAuthenticationCleanup` | 新格式验证 writer/Job 与 control/checkpoint 结算且没有认证字段；旧退出兼容义务删除，初始化失败与查询未结算的停止反例保留 |
| storage/format、generation/takeover、progress/result/usage | v5 拒绝且文件不变；v6 的非空 Run 重启/未知操作零重发；新建 read_only/parser 和 environment 正常；local state 8 的 usage/collector 可用 |
| publication、双 Run、原生输入 | Gitea TokenEnv 和 command provider 查询认证、repo/base/head、已知失败/未知区别、停止独立性、原 paste+Enter/READY、审计模型桥与 hook/代理组合保持 |
| testkit/framework/scripts | 从成功/清理判定移除 broker/expiry/cleanup 字段；同场景继续验证执行和停止，不重复登记一套 host-auth 场景矩阵 |

新增定向测试名固定为 `TestHostNativeAuth...`（环境/真实子进程/无认证启动/secret 不落盘）、`TestHostNativeControlClosure...`（控制关闭）、`TestSRHostNativeAuth`（受控公开 CLI/失败/双 Run）。旧格式拒绝复用 `TestRetiredStateFormatsRefusedWithoutChangingOriginalResources` 并加入 v5；不新增旧记录兼容用例。集成新增项纳入 catalog/registry，明确它新增的覆盖；现有 case 能承载同断言则合入，保持单一注册。

开发反馈入口（新增测试落地后）：

```powershell
go test ./internal/application ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/execution/terminaltools ./internal/resources ./internal/host ./internal/storage -run 'TestHostNativeAuth|TestHostNativeControlClosure|TestRetiredStateFormats|TestFreshState|TestContentReadOnlyPermissions|TestTerminalTool' -count=1 -timeout=5m
```

对实际改动再跑 publication、usage、read_only/parser、native submission 的对应定向包；本命令不代表所有保留边界已覆盖。发布前使用现有正式入口各跑一次完整 L1/Controlled，不在本任务改造其前置制度：

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
```

更新 README、CLI、client/skills、full_access/认证/Orca/恢复等当前 Spec 的冲突适用条款、测试指南和验收索引。原业务配置保持显式 full_access，但 state-dir/instance binding 和受管 helper 必须换新；不保留旧活动执行接管说明。新增 `docs/acceptance/host-native-auth.md` 记录源码/构建、命令、结果和缺口；本轮文档不预先创建“通过”记录。archive/evidence 原字节保留。

## 7. P5：固定构建验证

准备全新隔离现场和固定 Runner/helper/extensions，沿原测试配置格式及场景入口运行。使用专门测试账号/仓库的既有登录；不能用生产账号配置的移除/登出模拟缺登录。

| 范围 | 最小真实证明 |
| --- | --- |
| 本机 Codex | `local-codex.smoke` 完成实现、Git push、PR、停止；使用实际 shell 工具登录，不是 harness 代发 PR |
| Orca local Codex | `orca-local.codex` 的 helper/app-server/READY/TUI、原 native turn 和发布闭环；受控覆盖审计模型桥，实际可用组合再验证切模型/恢复实施模型；已有代理/Orca hook 保持可用 |
| Orca local Pi、DSH | 各自原 case 验证工具的 Git/gh 登录、provider 环境过滤、Job 停止；DSH 特别核对工具的真实 HOME。缺一项只声明对应组合未验 |
| 缺登录/错误权限 | 隔离 HOME/config 或测试假 helper 下实际命令有界失败，实施可继续/发布不能成功，日志无 sentinel；不要求消耗模型来重复同反例 |
| 同服务双 Run / 重启 | 复用原受控 case 证明新契约停止 A 不影响 B、原会话接回和未知输入不重发；涉及真实 Job/helper 寿命的改动再补最小原生停止专项 |
| 原生远端与只读/parser | 新 v6 中执行 focused 原生远端单次提交/完成/停止，及原生 read_only 无模型 command/exec 准备/拒写检查；共享版本选择和环境路径实际改变，不能只用“协议未改”豁免。parser 参数/环境/工具禁用定向验证必跑；若触及实际 parser owner/launch 再补其原生执行，缺环境记未验 |

沿原 `test.ps1 -Level L2/L3 -Scenario ... -Config ... -OutDir <新目录>` 或已有 focused 入口运行，不 invent 未实现的 skip 前置/产物复用开关。缺登录、模型、网络或 Orca 环境明确登记未验/incomplete，真实已派发操作保留原身份并收尾，不为凑通过换配置重发。测试 PR、执行资源和目录各自记录清理结果。

最后核对版本表、删除清单、残留调用、文档链接和 diff。认证字符串残留应只属于保留的 publication provider/模型登录/只读概念、失败诊断/拒绝反例或历史材料；不能有业务调用旧 broker 的路径。交付同时报告删除的生产实现/测试义务、新覆盖与仍未验证组合，不以文件改名或编译通过代替链路简化。

## 8. 文档审计记录（2026-10-31）

审计依据为同一调查基线的实际调用，不代表已实施或已通过产品测试。本轮修正 7 项：5 个 P1、2 个 P2。

| 编号 | 等级 | 已确认问题/代码依据 | 文档修正及必须验证 |
| --- | --- | --- | --- |
| HNA-A01 | P1 | [verify_stopped.go](../../internal/execution/adapters/orcatransport/verify_stopped.go) 的 verifyControlStopped 强制读取认证停止证据；[control_terminal.go](../../internal/execution/adapters/orcatransport/control_terminal.go) 仅在 Request.ID 非空时写 Closed，[control_checkpoint.go](../../internal/execution/adapters/orcatransport/control_checkpoint.go) 又要求 session/readiness，直接删认证会使无查询/初始化失败停止无证明 | HNA-04、P2-4 明确无认证的 checkpoint v3、固定绑定、reader 结算、无请求/未就绪关闭和未知 close 不重发 |
| HNA-A02 | P1 | [state_codec.go](../../internal/execution/adapters/orcatransport/state_codec.go)、lifecycle.envelope、[usage_binding.go](../../internal/execution/adapters/orcatransport/usage_binding.go) 共用版本；[policy_resources.go](../../internal/execution/adapters/codex/policy_resources.go) 共用 profile/policy decoder，简单升全局常量会破坏 environment/read_only/usage | HNA-05、P0/P3 按新记录的模式和 target 分别选择 codec；原生远端写 7、本机写 8，新只读保留 3/v10；新 v6 非空记录和实际读取链路验证 |
| HNA-A03 | P1 | [worker_runtime.go](../../internal/execution/adapters/codex/worker_runtime.go) 把 shell allowlist、OrcaHooks 和 NO_PROXY 放在 Auth 分支；[remote_agent.go](../../internal/execution/adapters/codex/remote_agent.go) 同时承载 READY、模型桥和 TUI 的 token 过滤 | HNA-02/03、P1-3/6、P2-2 保留环境/hook 独立接线、READY/输入事实、模型桥/token 作用域及审计后恢复模型；增加代理/hook/双模型反例 |
| HNA-A04 | P1 | [publication/gitea.go](../../internal/execution/publication/gitea.go) 必须读取 TokenEnv 设置 Authorization；原文全局“Runner 不提取凭据”可能误删 PR 查询认证 | HNA-01-3 明确 worker 认证与服务主机 provider 查询的边界；P1-5/P4 保留 Gitea/command 查询及各自认证、失败脱敏测试 |
| HNA-A05 | P1 | [freeze.go](../../internal/application/freeze.go) 原来仅 Pi/DSH 查询 terminal-metadata，Codex helper 的旧配置形状不带认证模式；只等新 readiness 拒绝，会先启动旧 broker helper | HNA-05、P0 增加复用 metadata 5 的 Codex 无副作用前置，校验认证/协议及 SHA；旧/混搭 helper 零启动；无 helper 及 environment 不强加该入口 |
| HNA-A06 | P2 | 原 P5 把原生远端/read_only/parser 视为可只跑受控，但共享 codec/env/readiness 被本任务改动 | P5 明确新 v6 的原生远端单次执行与只读无模型验证必需；parser 保留专项且触及原生启动时补测，未验不记通过 |
| HNA-A07 | P2 | “不改仓库 remote/凭据文件”过宽：[freeze.go](../../internal/application/freeze.go) 在新私有 clone 设置既定 remote，原生 Git/gh helper 也可能正常维护自身登录缓存 | HNA-01/HNA-06 限制 Runner 主动修改用户原仓库/账户设置，保留私有工作区的既定初始化和工具自身正常认证；测试比较用户原配置而非禁止所有新目录写入 |

审计过程中用户补充“旧记录可以丢弃”。据此删除原旧 Run 退役投影、兼容样本、被动解码、原 owner 精确停止适配和混合目录要求，统一 state/v6 + 全新目录；这属于明确范围调整，不计为第 8 项缺陷。旧执行须由原版本结束/停止，本轮不执行物理删除。
