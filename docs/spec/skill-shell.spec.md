# Spec：独立使用的双 Skill 薄壳

日期：2026-10-23。状态：已实施，已完成本次受控验证及本机 Codex / 一个 Orca environment 的真实闭环；具体资格与未验边界见[验收记录](../acceptance/skill-shell.md)。源码调查基线：`1cfe2f9a065830ab62d540d3de4c0011ed603b56`。实施见 [Plan](skill-shell.plan.md)。

## 1. 目标与范围

交付两个 skill、一个共享 PowerShell helper、Runner EXE 及独立 Viewer EXE，让用户无需 SteerLine、Go 开发环境或手写 RunRequest，即可配置执行链路，以仓库中的 Spec + Plan 发起开发执行，随后查询、回答、恢复和停止原 Run，并取得经 Runner 核验的 PR。Viewer 通过只读 JSON 文件持续显示最小 Run 列表，双方唯一共享的接口是版本化文件格式。

配置 skill 默认直接调用，不要求参数、路径或额外自然语言说明。调用后自动发现可用候选链路、保存可确定的配置并报告缺口；只有无法推断且影响实际选择的事项才询问用户。发现不到 Orca 时，主动说明安装 Orca 才能使用当前 Runner 的远程执行链路；安装完成后还需运行 Orca、配置远端 environment、执行主机工具/登录及项目。

首版宿主为 Windows/amd64，主交付基线为本机 Codex + GitHub。发现和选用已有 Orca 本机/远端链路，但支持范围始终消费当前 EXE/服务发布的能力，不能将发现结果视为全组合真机验收。原生远端仅使用既有 Orca 能力，不部署 Runner helper。

本期不新增 Runner 业务 CLI、CLI/IPC 协议、数据库格式、workflow、服务或任务调度器；Runner 的唯一新增产品能力是第 7 节的非权威展示快照导出。不接入任务领取、DAG、自动合并或 TaskDelivery；不增加自动安装工具、配对机器、注册项目或模型枚举系统。本文的 helper 是调用方本机脚本，与 Orca local 的可信执行 helper 不同。

## 2. 职责边界

| 所有者 | 负责 | 边界 |
| --- | --- | --- |
| `steerrunner-configure` | 无参数发现入口、解释候选和缺口、必要选择、保存配置 | 不实施任务，不启动 agent，不把纯校验当作环境通过 |
| `steerrunner-run` | 从用户意图取得 Spec/Plan 和运行目标，提交、展示进度/问题/PR，承接查询、回答、恢复、停止及打开 Viewer | 不代替 Runner 推进阶段、判断恢复资格或重发未知副作用，不增加第三个查看 skill |
| 共享 helper | 工具发现、配置读写、请求装配/保存、固定 CLI 调用、有限查询、调用记录查找、按需打开 Viewer | 无业务状态机、后台轮询服务、通用插件/规则解释器，不读取 Runner 私有库，不持续采集或写展示快照 |
| 调用方配置 | 保存用户默认选择和项目绑定 | 不存凭据、Run 状态、动态就绪承诺，不成为 Runner 配置 |
| Runner | 最终校验、输入冻结、唯一 Run 状态、执行/恢复/停止、结果和 PR 核验；导出自身 Run 展示快照 | 展示导出是非权威投影，失败不改变原执行契约 |
| 独立 Viewer EXE | 读取快照目录，显示 Run、Workflow、Status、PR 和数据新鲜度 | 不引用 Runner 项目代码/内部包，不读取数据库，不调用 CLI/IPC，不控制执行；独立构建/发布 |

两个 skill 保持正常自动发现。configure 的描述只匹配 SteerRunner 链路配置；run 覆盖执行及原执行的后续管理，不因讨论 Spec/Plan 或普通编码请求自动发起执行。用户已有授权仍按原范围使用，skill 本身不扩大权限。

## 3. 无参数配置与自动发现

### 3.1 发现步骤与证据

1. 读取安装定位信息及已有调用方配置。Runner 选择顺序为已配置绝对路径、同发行包 EXE、PATH；未配置时若前一候选不可用可继续列出后续候选，已保存的显式路径失效则报告，不静默换构建。以 `version --json` 和 `capabilities --offline --json` 确认程序、协议和支持组合；没有 Runner 时仍报告其它发现与安装缺口，不能声称配置完成。
2. 发现 Git、原生 Codex、gh 和 Orca。只在 PATH、发行包及明确的现有配置/安装位置查找，不递归扫描全盘。分别记录入口、原生可执行文件、版本和查询错误；不能把 `codex.ps1`/shim 当作已满足 Runner 原生 EXE 要求。版本适用性按当前能力声明判断，不在 skill 中永久写死当前版本。

   发现侧解释当前能力声明中的固定版本或最低版本要求，不维护另一份版本支持表。最低版本按数值组件比较，同一基线的预发布版低于稳定版，较新版本的预发布版可满足最低版本；未知/坏版本保留未核实。这里只筛候选，协议和权限资格仍由 Runner 实际核验。
3. 有项目上下文时，使用 Git 公开只读命令发现仓库根、remote URL、目标分支线索；gh 只查询对应 host 的登录/仓库身份，不取得 token。origin 优先仅用于新项目的候选发现；多发布目标、无明确默认分支或身份不一致时保留待补字段，禁止猜测发布目的地。
4. Orca 使用选定且固定的 CLI，通过公开只读接口发现。当前已核对的命令为 `environment list --json`、`status --json`、`repo list --json` 和 `repo show --repo <selector> --json`；对已保存 environment 的后三项显式加 `--environment <selector>`。repo list 的 id/path 只用于定位，仓库 URL 等身份需从公开详情核对；该版本缺所需详情时保留待确认项，不按同名路径自动匹配。按所用版本 help/机器接口核对能力，不依赖另装 orca-cli skill，不读 `orca-environments.json`、令牌或内部数据库。
5. Orca CLI 的显式选择、开发/安装版本区分沿其公开入口约定；同一次发现固定入口，不在失败后切换另一个 Orca。查询子进程屏蔽 `ORCA_ENVIRONMENT`、`ORCA_PAIRING_CODE`、`ORCA_REMOTE_PAIRING` 等隐式路由，保持本机与明确 environment 分离。枚举 environment 时不传目标选择器。
6. 将观测到的链路与 Runner 的组合声明、`validate --kind run-configuration` 结果相交，展示候选和缺失项。Orca 的 SSH host 不自动转成 Runner 的 `orca_environment`；当前请求没有对应 host 选择器的链路只能提示不能直接使用。

自动发现只执行有界只读探测，不调用模型、创建终端/worktree、fetch/push、创建 PR、注册项目、配对、安装/升级程序或启动 Runner 服务。Orca 未运行时报告并提示启动；不因通用 Orca 操作指南的自动 open 建议而在配置发现中启动应用。现存服务可用时可以只读查询其身份/能力，结果与离线构建身份分别标注。

每个探测有期限，默认 5 秒、显式远端查询 10 秒；整次发现默认 60 秒，超时保留已取得结果和未核实项。Orca 查询串行执行以避免其共享配置文件竞争。不得无限重试或用失败的远端查询阻断本机发现。结果仅保留允许公开的字段及无秘密诊断，不落盘完整外部响应、环境变量、账号配置或原始 stderr。

### 3.2 “发现可用”的含义

发现报告逐项给出链路、查询时间、来源、静态支持、连接/项目/登录的已知事实和缺口。展示“可选候选 / 缺配置或依赖 / 未核实”的原因即可，不建设另一套环境状态机或就绪缓存。

命令存在、连接成功、仓库已注册、配置合法、真实执行通过分别陈述。远端工具/登录无法只读证明时标为未核实，不能为补齐发现而创建远端探针终端。历史通过不等于当前可用；配置再次调用即重新发现，执行时仍由 Runner 核验真实环境。

完整 `environment-check start` 是用户另行要求的检查，可由 configure skill 调用既有命令，但须先说明其专属引用、push 和测试 PR 操作及范围。无参数发现不隐含授权它；远端完整检查 unsupported 不阻断普通远端 Run，也不显示为检查通过。

### 3.3 默认选择与重复调用

- 已有默认方案、项目选择和显式模型/权限保持；重新发现更新报告并补齐缺省信息，不能覆盖显式字段。已有选择不可用时展示缺口，不自动换目标、模型、权限、EXE 或 state-dir。
- 首次配置优先将静态受支持且已找到原生入口的本机 Codex 设为默认候选，采用 `development/2` 和 `sandbox`。它仍可能有登录等现场缺口，报告必须列出，不能承诺提交成功。
- 没有本机候选而只有一个静态受支持、连接已核实的 Orca 方案时，可自动保存为默认；多个候选保留列表并只询问一次默认选择，不要求用户先编写说明。项目缺绑定不妨碍保存机器级方案，但妨碍该项目正式提交。
- 模型未知时保留公开允许的原生默认并解释未固定模型；不得杜撰型号或自动调用模型试探。审计模式仅在用户选择且所需显式模型及受管条件齐备时启用，不自动用双模型审计替换普通开发。
- 非 Git 目录也能完成机器级发现，不创建项目绑定、不要求用户先给项目参数。没有任何候选时仍完成诊断，清楚报告尚不能执行。

### 3.4 缺少 Orca 的提示

| 观测 | 提示与处理 |
| --- | --- |
| 未找到 Orca CLI | “已发现的本机链路仍可使用。要使用 SteerRunner 远程执行，请先安装 Orca，再配置远端 environment、执行主机工具/登录及项目，然后重新调用配置 skill。”没有本机候选时相应说明当前也缺本机链路 |
| 已安装但未运行/不可连接 | 提示启动或检查当前 Orca，不误报需要重新安装 |
| environment 列表为空 | 提示配置已有远端 Orca environment；只有 Orca local 不算远程已配置 |
| environment 存在但离线/认证失败 | 保留 environment 与具体缺口，不回退本机，不把错误解释为未安装 |
| 远端可连接但未匹配项目 | 提示在该端注册对应项目并核对真实路径；不自动 repo add 或复用调用方 Windows 路径 |

安装建议使用发布时核对的 Orca 官方入口；不能提供猜测的下载链接。提示不是自动下载安装、登录或配对授权。

## 4. 配置与安装

### 4.1 最小配置

调用方文件位于 `%LOCALAPPDATA%/SteerRunnerClient/`，与 Runner state-dir 分开。第一版只维护以下文件，不引入多层继承、热更新或配置服务：

| 文件 | 内容 |
| --- | --- |
| `client.json` | `version:1`、可空的 `runner_executable`、`state_dir`、可空的 `default_profile`、`profiles`；未找到 Runner 时只保存可确定内容，不能提交 |
| `projects/<project-key>.json` | `version:1`、调用方规范化 Git common-dir/仓库身份、`profile`、公开 repository 模板和 publication；未确定字段省略并报告缺口 |
| `discovery.json` | 最近一次有界发现的允许公开字段、时间和缺口；纯诊断，不作为执行准入或默认选择权威 |

`profiles.<id>` 直接使用现有 `{execution, workflow}` 形状。每个项目首版只绑定一个方案；同仓库多 worktree 共用项目身份，分支/材料由每次提交指定。项目有显式 profile 时优先采用，未选定才使用全局 default_profile；显式选择缺失/无效不能退到全局。Orca 方案的项目 repository.local_path 为目标端已注册项目路径；调用方项目定位信息单独保存，不混用两端路径。不同目标路径变化时重新核对绑定；本次明确覆盖也必须形成一致的完整请求，不能沿用另一执行目标的项目路径。

Runner 默认 state-dir 沿公开约定，首次解析为绝对路径保存；之后始终显式传入，不随 cwd 或链路切换改变。已有服务构建/身份冲突明确报告，不能新建空目录绕过，也不自动停服、更新或接管。

机器方案先由 `validate --kind run-configuration` 校验；项目完成后由装配出的完整请求调用 `validate --kind run`。客户端只校验自己的外层字段、路径和绑定，不复制 Runner 的 adapter/provider/schema 规则。未完成的项目配置允许保存，但不能提交。配置不保存 token、配对码、完整 endpoint 认证数据或执行主机凭据。

配置严格读取、原子替换、保留显式字段；损坏或未知版本报告，不覆盖成空配置。写入以短期文件锁串行，发现期间不持锁，写前重新核对读取版本，冲突时重读而非覆盖其它调用结果。项目 key 只用于定位，读取仍核对原仓库身份；仓库搬迁或 remote 改变不能误用旧发布目标。

### 4.2 独立交付与定位

2026-10-26 交付校验增量：构建回执使用 `steerrunner-build/v1`，绑定组件、平台、commit、dirty、实际源码内容摘要及 EXE SHA256。构建前后核对源码摘要，变化则拒绝发布本次输出。包记录 Runner、Viewer 各自回执及客户端/skill/安装脚本来源，允许组件来自不同提交；不据来源相同推导兼容。打包要求新格式回执，旧 EXE 或缺失/过期回执需重新构建；原 CLI、数据库和 Viewer 快照格式不变。

包内 `manifest.json` 使用 version=2，增加必需的 `provenance` 及组件 build 记录；Viewer 快照 schema 仍为 1。清单必须覆盖除自身外的全部文件，拒绝遗漏、多余文件、重复/越界路径、链接、摘要或组件来源失配。安装在写入前调用同一校验实现，保留已有安装/同名 skill 的拒绝语义。新格式包携带校验脚本与构建回执，可安装后再次核对。新校验器拒绝旧 version=1 包；旧完整包仍使用随包旧安装器，已有用户安装、调用方配置与服务不迁移。

受控交付资格保存于包外的 `steerrunner-delivery-qualification/v1` 报告，scope=`controlled-client-delivery`；四项必需结果为 installation、client、viewer、progress_transport，绑定最终包清单/内容摘要和各结果文件摘要，进度项另绑定原始快照、日志与进程/被测 EXE 身份记录。缺项、失败、包或证据变化不通过。原生 Viewer 只核验其既有 Run 快照，progress_transport 只验证原始字节通道；不宣称调用方测试进度页面或任何真实后端组合通过。该机制检测意外混装/变化，不是包签名或第三方来源认证。

发行包包含 Runner EXE、独立 `steerrunner-viewer.exe`、`client/steerrunner-client.ps1`、两个 skill 文件夹和最小安装脚本。安装脚本只负责本机放置/定位与 skill 发现，不是配置或执行引擎。两个 skill 均定位同一份共享 helper，不各自复制实现，不引用开发 checkout、内部 testkit 或另一个必须额外安装的 skill。helper 从发行包根目录定位 Viewer；不再增加用户必须配置的 Viewer 路径或新的方案字段。

安装时为各 skill 生成 `assets/client-location.json`，记录绝对 helper 路径；从 helper 所在 client 目录定位发行包根目录 EXE，作为首次发现候选，用户无需手填路径。默认安装到当前用户的持久目录，skill 安装到当前宿主支持的技能目录；明确指定目录时尊重指定值。安装与重装不得静默覆盖其它同名 skill、用户配置、运行记录或正在使用的 EXE。安装包记录版本及内容摘要，移动/删除后定位失败明确诊断。

最终用户只需受支持的 Windows、系统 PowerShell 和所选执行工具；Go/Node/Python、源码目录不作为运行依赖。Orca local 如需可信执行 helper，发现现有配置/发行物并核验其条件；缺失时列缺口，不从本调用方脚本伪造该执行 helper。

Viewer 为可单独复制运行的 Windows EXE，不需要 Runner、PowerShell、Orca、开发运行时或另装 UI 运行时。默认读取约定快照目录，可用单一可选 `--snapshot-dir <绝对路径>` 读取其它目录/测试夹具；该路径不反向配置 Runner。Viewer 缺失或启动失败只影响查看入口，不影响两个 skill 的执行管理，也不触发下载或安装。

## 5. 执行 Skill 与公开 CLI

### 5.1 新执行

1. 用户给出 Spec/Plan 或从明确上下文取得唯一文件对；路径有歧义才询问。读取项目绑定与所选方案，补齐标题、基线及本次明确覆盖，显示仓库、目标、权限、workflow、材料和发布分支摘要。明确请求以当前开发 workflow 执行包含提交和发布 PR；不额外要求重复确认。
2. 采用用户明确的完整 commit；未指定时，解析并固定所选 repository.base_branch 的远端当前 tip，而非调用方特性分支 HEAD。必要 fetch 只更新调用方仓库的 remote-tracking refs，不 checkout/reset 或修改工作区；失败不以旧引用顶替。Runner 自己的刷新与祖先关系校验仍是最终 gate。
3. Spec/Plan 转为仓库相对路径，用 Git 对象读取确认在固定 commit 内可读；按 Git 对该路径的正常 clean/换行规则比较工作副本的规范 blob 身份，避免把 Windows 正常 CRLF checkout 误报为材料修改。规范内容不同或尚未跟踪时，明确指出执行的是哪份材料并要求先使预期材料进入选定基线；检查不改写文件。helper 不自动 commit/push、拷贝材料或提交 inline 开发文档。远端沿公开 repo list/show 核对项目身份，其 commit 可达性由 Runner/执行端核验，调用方检查不冒充远端证明。
4. 组装公开完整 input，调用 `validate --kind run --input - --json`；校验通过仅表示形状/纯 gate 通过。确定 input 后不随配置变化重新装配，尚不发送 submit。
5. 执行时显式 start/connect 同 state-dir 的服务，取得实际在线能力和公开完整 binding。确认能力与已选请求适用后，先保存完整 v1 信封、request_id 和关联信息，再按现有 `--expected-binding` 通过 `submit` 发起。离线能力不冒充在线事实；身份改变先重新查询，不能静默改投。兼容性由现有服务/接管契约决定，不由壳根据 SHA 自行放行。
6. 保存原回执和关联 run_id；以 `observe` 为常规同快照观察，必要时 `inspect` 查询原命令，`last-message` 仅展示活动。事件游标只沿已消费页前进，不按时间或消息文本推导业务结果。

缺配置时 run skill 可调用同一 helper 的无参数发现并提示必要补齐，不要求用户先记住配置参数；没有可确定的项目/发布绑定时停止于提交前，不自动安装依赖或改变账户。

### 5.2 后续管理与结果

| 用户动作 | 复用 CLI / 规则 |
| --- | --- |
| 进度、跨聊天找回 | 调用记录定位同 state-dir/run_id，再用 `observe` / `inspect`；多条候选时展示供选择，不能仅凭“最近一次”操作其它项目 |
| 打开 Run 列表窗口 | helper 打开独立 Viewer；也可直接双击 Viewer，不启动 Runner、不自动提交任务 |
| 回答问题或业务续执行 | 先取当前 Run.controls、revision/question_id，使用 `reply`；只传用户当前明确内容 |
| 技术恢复 | `controls.can_resume` 与当前恢复原因允许且用户明确要求后，使用 `resume`；不自动回答旧问题或重做实施 |
| 停止 | `stop` 仅作用于明确原 Run；展示受理、撤销授权、原资源停止/未知的区别，不用 service stop 代替 |
| 结果与证据 | 使用 Run/PR 事实，按需 `inspect --refresh-pr`、`artifacts discover/collect/list/read` 和 `workspace-git`；不读私有库，不用日志推断完成 |

`pr_ready` 展示 PR、base/head、核验信息和已知资源事实，不自动合并或停止仍需保留的资源。当前开发流程在 pr_ready 后拒绝业务反馈，skill 不绕过 controls；后续改动可按用户意图新建执行，但不能把未知旧 Run 重新 submit 当作恢复。

helper 每次提供有限观察，单次等待不超过 30 秒；活动聊天可继续按合理间隔查询。聊天结束不自动 stop，也不安装后台监控/automation；返回足够的定位信息供稍后查询。Viewer 打开时独立读文件；持续导出属于既有 Runner 服务的生命周期，不由聊天或 helper 的轮询维持。

## 6. 请求保存与未知结果

`submissions/<submission-id>/` 位于客户端目录，保存原 EXE 路径、规范化 state-dir、项目身份、固定信封及摘要、最初 binding、回执/run_id 和消费游标。每个后续写命令同样先保存独立 request_id、完整正文及原目标；不只保存 submit。观察结果是可更新的投影，不成为业务权威。

保存成功后才可能外发；每条命令用短期排他锁防止两次客户端同时派发，不能以锁是否仍存在判定 Runner 是否受理。崩溃后在“已保存但可能已发送”的窗口，一律先查原 request_id。同 ID 重试使用原信封，不读取当前配置重新装配。正文丢失/损坏或摘要不符时只查询和报告，不能重建后重发。

退出码 6、进程被杀、无输出、损坏 JSON、超时等按未知处理，使用原 request_id 查询。`request_not_found` 在服务恢复未完成时不能证明未受理；即使可重试，也只允许同 ID/同正文通过现有幂等机制找回原结果，禁止生成新 ID 自动重提。已存在的动作未知继续由 Runner 对账。

重启后重新取得同 state-dir 当前服务的公开 binding；原提交 binding 保留，后续查询/命令绑定当前实例，不拿旧 binding 反复失败或绕过实例围栏。配置修改不改原正文、原目标或原 Run；切换 EXE/恢复服务遵循既有生命周期操作，不在壳里自动部署。

该目录是小型调用记录和索引，不保存另一套 workflow、重试预算、停止资格或恢复调度。缺记录时允许用户提供原 state-dir/run_id 只读找回；不扫描 Runner 数据库补建历史。

## 7. 只读快照与独立 Viewer

### 7.1 数据来源与单写者

Runner 服务导出该 state-dir 中当前可读取的全部持久化 Run，包括其它 skill、SteerLine 或直接 CLI 提交的 Run；不以客户端 submissions 记录作为列表来源。首版仅列 Run，不混入独立 environment-check。读取自己的既有状态投影即可，不新增公开 Run 列表 CLI，不让外部 helper 扫描私有库。

默认导出到当前用户 `%LOCALAPPDATA%/SteerRunnerView/snapshots/<source_id>.json`。`source_id` 是服务现有 `host.Normalize` 结果的 UTF-8 字节的完整 SHA-256 小写十六进制标识，复用 writer lease 的路径规范化，不能由 helper/Viewer 另写一套路径或大小写规则；仅供定位/去重，不暴露真实目录，不作为执行身份或授权。每个 state-dir 对应一个文件；沿现有 writer lease 保证只有其当前服务写入，不另外维护全局聚合文件或注册中心。Viewer 枚举该目录的 JSON 文件，以 `(source_id, run_id)` 区分来源；首版仅支持本机普通目录，不实现跨机同步、网络共享采集或远端部署。Orca 远端 Run 仍由管理它的本机 Runner 导出。

导出从已成功持久化的 Run 投影取数，短时持原有读取保护，只复制下节允许的字段为独立 DTO，再在业务锁/事务外排序、序列化和写文件；不浅拷贝可并发变化的指针/集合，不 clone 或序列化整个 RunRecord/RunView。未提交或回滚的状态不能进入快照；启动尚未完成状态加载、读取失败或来源不完整时，不发布空列表或新鲜心跳覆盖原文件。空列表只有成功读取并确认该来源确实无 Run 时才能发布。数据是 Runner 当前已知事实，不证明原生后端此刻仍在运行；心跳只证明近期取得了该服务的状态投影。

### 7.2 最小文件契约

UTF-8 JSON，每个文件是一份完整快照；无增量事件流、数据库、索引文件或无限追加日志。首版形状如下（示例值只说明字段）：

```json
{
  "schema_version": 1,
  "source_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "instance_id": "service-instance-id",
  "sequence": 1,
  "generated_at": "2026-10-23T08:00:00Z",
  "runs": [
    {
      "run_id": "run-id",
      "title": "实现示例功能",
      "workflow": {"id": "development", "version": 2},
      "step": "implement",
      "status": "running",
      "pr": null
    }
  ]
}
```

`workflow` 仅含 RunView.Workflow 的 `id`、`version`，禁止整体复制该对象中的 prompt_overrides/audit_model 等其它字段；`step`、`status` 为对应原字段。`run_id`、`title` 及所有嵌套对象也按白名单逐项投影。有已知 PR 时 `pr` 仅为 `{"url":"https://github.com/example/project/pull/1","number":1}`，否则为 null。只取 Runner 原有 PR 事实，不触发 provider 刷新；显示 PR 链接不额外宣称当前 merged 状态。未知 workflow/step/status 按原字符串显示，不能把不认识的值当作成功或已结束。

`sequence` 为正整数，当前 `instance_id` 内在每次发布尝试前分配递增值，失败或替换结果未知也不复用；Viewer 允许跳号。计数仅存当前服务内存，换代可从 1 开始，不增加数据库字段/操作日志。新服务加载成功后以新 instance_id 覆盖同 source 文件，不重新生成一个来源，不修改原 Run 身份。`generated_at` 为成功取得该一致投影时的 UTC，而非写入完成时刻，不得把排队、重试或慢写前取得的旧内容重新标为当前时间。`source_id` 必须与文件名一致，`instance_id`、`run_id` 非空，`runs` 为数组，空列表写 `[]`。Viewer 不依赖 Runner 的 Go 类型、资源摘要或私有 schema，只实现该文件的小型 DTO；同一 schema_version 内忽略未知附加字段，未知版本/重复键/重复 Run 身份/必需字段缺失或类型错误拒绝本次文件。

不导出输入正文、Spec/Plan 内容、会话日志、问答、认证、工具参数或真实 state-dir。文件内容是展示数据，不能解释成命令；标题按普通文本显示，PR 只允许用户点击打开有效 HTTP(S) 链接。快照不成为 reply/resume/stop 的参数或权威依据，执行 skill 始终重新通过公开 CLI 查询当前事实。

### 7.3 发布、刷新与故障

- 服务加载就绪后首次发布；之后由单个服务内串行导出器每 1 秒读取既有投影，不挂接业务提交回调，不新增通知队列。按 run_id 固定排序后比较展示白名单字段；变化则发布，无变化则每 5 秒重新取数发布心跳。该周期为正常调度目标，不承诺磁盘异常时的硬实时；中间状态可合并跳过，不记录展示历史。
- 先写同目录临时文件并关闭，再原子替换目标 JSON；Viewer 不应看到半份新文件。Windows 读句柄允许替换/删除共享，读取后及时关闭；临时文件使用非 JSON 后缀，不参与枚举。替换前失败保留原有效目标，不先删文件；替换结果未知时也不删除/回滚目标，不声称它必定还是旧文件。在原操作收尾后，后续定时轮次重新取数、分配新序列再发布，不在本轮循环重试；失败连续发生时最多每 5 秒尝试发布一次，诊断限频。更改重排/序列化格式不能被误当作业务字段变化。
- 导出器最多保留一份进行中的快照，不排队积压旧副本或启动重叠写入。采样/文件 I/O 失败不回传为业务命令失败，不改变 Run revision/事件、授权、恢复或停止结果，不让正常服务启动失败。除短期复制白名单字段外，不占用业务锁；文件 I/O 不在业务提交回执路径同步等待。持续失败会使展示自然过期，不能只更新时间戳给旧数据续期。
- 文件发布单次预算为 2 秒，超时取消并收尾原操作；不能用“调用方已超时”代替文件操作已结束。正常停服先撤销该导出者后续发布并取消在途操作，在既有停服期限内确认导出者退出后才释放 writer lease，防止旧实例迟写。若无法确认退出，沿既有停服未核实/失败路径报告，不能提前释放 lease 或宣称已停服；不新增脱离原服务的写进程。
- 不删除最后快照、不把所有 Run 改成 stopped；服务退出、崩溃、聊天或 Viewer 退出均不能据此推断原执行已停止。读盘/写盘变慢不会通过写入完成时间给旧投影续期。

Viewer 首版每 2 秒在后台枚举并读取快照，无文件监听、CLI 查询、独立采集器或系统服务。单文件读取预算为 2 秒，慢读取消后标该来源错误，不在 UI 线程等待，也不堆积重叠的该来源读取；晚到结果不能覆盖已经显示的新快照。其它来源与 UI 新鲜度计时继续进行，不能因为一份文件不可读而冻结整窗。

一次完整成功解析后按来源整体替换显示；一份有效新快照中不再出现的 Run 才从该来源列表移除，不叠加历次快照。目录确实不存在且从未读到数据时显示“暂无快照”；枚举权限/其它 I/O 错误显示“无法读取快照目录”，不能伪装成空目录。文件损坏、被删除、暂时不可读或 schema 不支持时，保留该来源最后一份有效内容并显示原因；首次读取即失败则显示来源错误，不能显示“没有运行”。某来源失败不清空其它来源。Viewer 只保留本次进程内的最后有效视图和计时信息，不写缓存库、源注册表或文件修复内容。

### 7.4 新鲜度与最小窗口

每个来源单独计算新鲜度，固定有效窗口 15 秒。接受一份新实例或更高序列的有效快照时，在读取完成处计算 `age = max(0, 当前UTC - generated_at)`，剩余有效期为 `max(0, 15秒 - age)`，并用本机经过时间计时；不能每次读到文件就重新获得 15 秒。首次读到 14 秒前的快照只剩 1 秒，旧文件超过 15 秒立即过期。生成时间超前本机 UTC 超过 5 秒标为时间异常/未核实，不替换最后有效视图或续期；较小时间偏差最多从读取完成起计 15 秒。

重复读取同一 `(instance_id, sequence)`、相同已识别字段时，只能清除已经恢复的临时读取错误，不重置生成时间、剩余有效期或过期标记。同一实例序列倒退，或序列不变而已识别字段改变，视为异常；比较忽略 JSON 键顺序和未知附加字段，与向前兼容规则一致。不合并实例的序列；新实例仍需自身时间有效，不能仅凭 instance_id 改变把旧文件变新。

经过时间需覆盖系统睡眠；时钟回拨不延长已接受快照的期限，向前调整可保守判为过期。恢复窗口/系统唤醒时先重新计算期限或读到新快照，再显示为当前；不能恢复一张缓存画面并重新授予 15 秒。异常/过期来源的行内容标为上次已知状态，计时独立于磁盘读取是否完成。

同一份已接受快照一旦被判为过期，后续时钟校正或同序列重读不能复活它；有效的新实例/更高序列按自身采样时间重新计算期限。

文件读取/格式异常立即显示来源错误；是否超过 15 秒单独判断。过期的 running 显示“上次状态：running”，不改写 Run.status，也不推断 failed/stopped/succeeded。新实例的有效新鲜快照恢复展示；旧实例不再拥有写权限，Viewer 不需要接管或恢复服务。

窗口只提供以下四列及一条来源/最近更新时间提示，首版显示全部 Run，不默默截断历史，不为判断“进行中”另建业务终态分类器：

| 列 | 内容 |
| --- | --- |
| Run | 标题、短 Run ID；多来源时在同格附来源短 ID，保留完整身份供区分 |
| Workflow | `id/version · step` |
| Status | Runner 原始状态；过期/错误时明确为上次已知状态 |
| PR | 可点击编号/URL，无则 `—` |

可做简单列排序，不加入日志详情、图表、回答/恢复/停止按钮、自动合并或后台通知。新鲜快照中的 pr_ready 仍只是业务里程碑，不宣称资源已停止。用户可双击单独运行 Viewer，关闭后不影响 Runner；退出 Viewer 不再继续文件轮询。

## 8. 验收条件

| 编号 | 通过条件 |
| --- | --- |
| SS-01 | 干净安装无 SteerLine/源码/Go/Node/Python，两个 skill 均自动定位同 helper 和 EXE，用户可直接无参数调用 configure |
| SS-02 | 无配置、本机候选、多候选、非 Git 目录、显式路径失效及重复调用均有确定结果；保留用户选择，发现无模型/服务/工作区/PR 等副作用 |
| SS-03 | Orca 未安装、未运行、无 environment、远端离线/认证失败、项目不匹配分别提示；缺 Orca 主动给出远程安装指引，Orca local/SSH host 不伪报可用的 Runner 远端链路 |
| SS-04 | 配置直接复用生产 run-configuration；项目/完整 input 校验沿公开 CLI。文件冲突/损坏、身份改变和两端路径混用不会误投 |
| SS-05 | Spec/Plan 按确定 commit 检查，未提交/内容不一致/错误基线明确阻止误执行；首次 submit 前固定并保存信封，目标/基线不随重试漂移 |
| SS-06 | submit 及后续写命令丢响应、崩溃、并发、损坏记录与恢复中查无请求时，不换 ID/改正文或重复创建 Run/副作用；跨聊天可找回原运行 |
| SS-07 | 公开 binding、observe 游标、controls/revision/question_id、reply/resume/stop 接线成立；旧实例/旧问题拒绝不触发自动重做或服务切换 |
| SS-08 | 本机默认路径真实完成 Spec+Plan → 经核验 PR；中途问答、跨客户端继续查询及停止/资源事实可核验。accepted、模型自述、pr_ready 和资源结算分别展示 |
| SS-09 | 至少一条现有 Orca environment 链路真实通过发现、项目绑定、普通开发与 PR 核验/停止范围展示；未验证的组合显式保留缺口，不用远端完整检查 unsupported 阻断普通 Run |
| SS-10 | 交付与两个 skill 不依赖私有库/testkit/开发绝对路径；凭据不进入配置/发现报告/调用记录；没有新增 Runner CLI 或业务状态机 |
| SS-11 | Runner 导出同 state-dir 全部 Run，含非薄壳提交来源；仅成功持久化状态进入白名单 DTO，workflow 不泄漏 prompt_overrides/audit_model；空列表/规范化来源/换代/单写者边界正确，无新增业务回调、agent/provider 查询 |
| SS-12 | Viewer 可在没有 Runner/源码/额外运行时的机器上只靠文件独立运行；四列字段正确、多来源不冲突、不依赖 Runner 包/CLI/数据库、不控制任务 |
| SS-13 | 1 秒采样/变化发布、5 秒心跳、2 秒读取周期和 15 秒期限符合第 7 节；14 秒旧快照只剩 1 秒，同序列恢复读取不续期；覆盖原子替换/损坏/删除/未知版本/序列/时间/睡眠/换代，过期不被假刷新 |
| SS-14 | 慢读不冻结窗口/其它来源/过期计时；导出失败不改变执行/受理/资源结算，慢写超时不等于收尾，旧导出者不越过 lease 释放迟写；关闭聊天/Viewer 不影响服务，服务结束不伪造 stopped，无新增采集进程或后台服务 |

实施状态与真实证据另登验收，不因本文建立而标为已实现。依据：[独立配置](independent-configuration.spec.md)、[产品与 CLI](steerrunner.spec.md)、[可靠性交付](../reliable-delivery.md)、[完整绑定与 observe](reliability-simplification.plan.md)、[Orca 本机/远程](orca-local-remote.spec.md)、[原生远端](orca-remote-native.spec.md)。
