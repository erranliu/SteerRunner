# Plan：SteerRunner 首版独立化与 SteerLine 切换

接受前 Git 缓存目标分支刷新修复单独见[分支刷新 Plan](repository-base-refresh.plan.md)及其测试计划，不扩大本计划历史 live/恢复结果。

> 2026-10-15 独立三级测试更新：当前定义与实施入口以 [独立测试说明](../testing.md) 为准。
> L1=规则/组件；Controlled=L2/L3 前置；L2=本机 Codex + SteerlineTest 真实创建/合入 PR；
> L3=可持续扩展的真机真实环境测试集，包含远端 Orca。以下首版历史入口中的 L1/L2
> 曾是 live 场景编号，旧取消范围和 SteerLine 测试依赖不再作为本项目测试目标。

日期：2026-10-14。用户已授权实施本计划。[Spec](steerrunner.spec.md)为行为依据，自动/人工验收及实时结果统一登记在[专属验收](../acceptance/steerrunner.md)。P0–P7 是跨仓交付检查点，不是 roadmap Task 或新的执行聊天。原调查基线保留下表；实施来源与主干交接事实见 [source-map](../extraction/source-map.md)。

## 1. 调查版本、交接与前置

| 输入 | 本轮实际调查 | 使用约束 |
| --- | --- | --- |
| SteerRunner | 仓库尚无 commit，已有 README/Spec/验收工作树；本轮新增 Plan 并修订契约 | 实施前提交并固定 Spec/Plan/验收实际 commit/path；未提交材料以摘要登记，不冒充 Git 版本 |
| SteerLine 主工作区 | HEAD `a81341a515251e23b4fa19d7a09096c11d0db8dc`；尚无 `internal/execution/workflow` / `reliable`；存在他人 Web/配置/文档在途改动 | 本轮只读；未来切换基于已核实的新主干，不覆盖当前在途变更 |
| 三层重构工作区 | 读取 HEAD `44f54055abe54fa08944afb6dfb31393e39a4a56`；核心提取提交 `4a6034f0ff8d8d9fb2ce1a56ca9dd58af4228255`，随后有可靠性/夹具修复；验收与夹具仍有工作树修改 | 仅调查，不能据存在新包认定已合入或已完成验收；P2 以最终进入主干的三层产物为技术交接 |
| 现有测试 | SteerLine `tests/e2e/README.md`、`suites.json` 和 `cmd/test-feedback` 定向入口 | 下文列出的旧场景存在于调查版本；切换时更新 driver 后重验，不复制历史通过结论 |

已读取 SteerLine `docs/spec/SpecGuide.md` 的 Spec/Plan 分工与最小验证要求，以及当前 PR、原会话、停服恢复和 Orca 取证约束。普通开发不需要 roadmap 转正或修改 execution-map。

P0/P1 可在三层交接前开展文档、公共 schema、CLI/宿主及纯协议夹具建设。P2 搬迁执行规则前，核实三层实现和必要修复已进入选定主干 commit；没有完成该交接就不从旧 Service 另拆第二套规则。实际交接路径/commit/差异和继承场景写入 `docs/extraction/source-map.md`。

2026-10-15 实施补充：三层及其审计修复已经进入来源 `d20a72284705b4daac476d4fa5770151dca03e56`，P2 按该提交抽取。Worker 统一规则以该提交的 [worker-lifecycle.md](https://github.com/erranliu/SteerLine/blob/d20a72284705b4daac476d4fa5770151dca03e56/docs/product-design/worker-lifecycle.md) 为输入；保留 W02–W16/W18 的准备、原意图、身份、结果及恢复边界，W01/Task 与 W17/TaskDelivery 留在 SteerLine，Runner 的成功里程碑改为 pr_ready。本文不把来源中尚未实现的初始化重试、Orca 替代会话恢复声明为已支持。

材料实施补充（2026-10-21 修订）：开发流程只绑定 Spec、Plan、references 的仓库相对路径。Runner 在固定 base commit 创建 worktree 后直接引用这些路径，不读取/校验文档、获取独立材料 commit、保存正文快照或物化副本，不内嵌或传输正文。文档提交、推送及存在性归调用方。`TestDevelopmentPromptsReferenceWorktreeDocuments` 覆盖本地/Orca 初次与反馈提示词仅引用路径，`TestDevelopmentAdmissionDoesNotResolveDocumentFilesOrCommits` 覆盖文档/独立 commit 不存在时不增加受理门槛。Orca 长度限制仅检查实际提示词。内容流程保留原 inline 材料机制；已冻结的阶段输入不重写。

首版依赖现有 Git、原生 Codex/Orca、账号与目标主机准备，能力验证按实际现场进行。当前支持范围固定为 Windows/amd64；通用远端服务、第三方运行时和任意 workflow 图不串入本次交接。

## 2. 本轮契约审阅结论

| 问题 | 本轮修订及实施含义 |
| --- | --- |
| CLI 子进程下启动的服务可能随父 Job 一起退出 | Spec §3 要求服务独立生命周期；P1/P4 验证 CLI/SteerLine 退出后的存活及不允许 breakaway 时的明确失败，不用“后台启动”代替事实 |
| stop 在原生能力 unsupported 时全部拒绝，会留下自动后继输入 | Spec §8 改为先提交新输入授权撤销，再处理真实原生停止；unsupported/unknown 不撤销逻辑禁止，不能报告 stopped |
| Task 终态与 Runner 接受 stop 之间无法跨库原子化 | Spec §9 明确确认窗口、持久待 stop 和回执对账；P6 测试回执之后零新输入，窗口内/原生仍运行的事实如实表达 |
| “嵌入凭据”可能误拒绝合法 SSH 用户身份 | Spec §4 区分 SSH 登录用户名与密码/token；复用原 publication URL 校验，不把秘密写进诊断 |

原请求 ID 查询、服务实例绑定、PR 只读刷新、接受与完成分离、原会话恢复及人工继续门槛保留。具体 adapter/provider 配置 schema 在 P1 固定为内嵌资源；schema 的公开字段/默认值变更先修订 Spec，不由代码临时另立行为。

## 3. 目标代码和迁出/保留清单

下列为目标 Runner 代码位置。包数随实际职责可收敛，但依赖方向与所有权不可混合；已建/未建与实际验证见验收文档。

| SteerRunner 位置 | 来源与工作 |
| --- | --- |
| `cmd/steerrunner`、`internal/cli`、`internal/contracts` | 新建 EXE 入口、命令参数/信封/公共 DTO；CLI 只访问服务，无 SQL/步骤驱动 |
| `internal/host` | 从 SteerLine `app/host`、`execution/process` 复用纯进程/锁事实，新增命名管道、发现与脱离调用方生命周期的启动；不复制产品 facade |
| `internal/application`、`internal/storage` | 新建 Run actor、命令接受和 SQLite 声明提交；参考原 operation/commit/CAS，不搬 Task/Graph/Delivery 表或 checkpoint 权威 |
| `internal/resources` | 内嵌默认 workflow、四个提示词正文和协议/后端/provider schema；资源版本与内容摘要可查询 |
| `internal/execution/workflow`、`reliable`、`backend` | 从最终三层产物提取规则和窄契约；TaskFacts 改为固定文档材料，await_merge/delivered 改为 Runner 的 pr_ready 里程碑；可靠执行不解释 Task/PR |
| `internal/execution/adapters`、`workspace`、`publication`、`process` | 迁移已有原生协议/取证、Git/PR、进程资源实现；去掉 SteerLine 服务和混合模型依赖，保留原回执与能力差异 |
| `tests/integration`、`tests/helpers`、`tests/native` | 真实 EXE/CLI/IPC 测试、测试专用原生协议/provider helper、显式 live 入口；生产不导入测试包，也不增加模拟专用生产 adapter |

SteerLine 的保留/替换安排：

| 来源范围 | 切换后的所有者与删除条件 |
| --- | --- |
| `internal/task`、任务图与任务提交端口 | 留在 SteerLine；不向 Runner 传整个 TaskRecord/TaskContract |
| `execution/model` 中候选关系、TaskDelivery 与纯归属门槛 | 留在 SteerLine 的任务/候选 owner，建立最小 CandidateRunBinding；可迁至任务模型，不能因包名 execution 全部删除 |
| 原 WorkflowRun/Execution/Attempt/Operation/会话执行表与 WorkerCommit | Runner 版本成为独立权威；SteerLine 删除旧执行写路径与表，只保留自身关联、待发请求、消费游标及必要只读投影 |
| `native_delivery_command.go`、`storage/ports/task_result.go`、SQLite `task_result_commit.go` | 改为基于候选 Run 绑定及最新正确 PRFact 的任务事务，不接收可写 Runner Workflow/Execution 记录；唯一交付/closed/后继仍一次提交 |
| app 的 adapter directory、worker/服务接线与平台进程句柄 | 用新 `internal/runnerclient` CLI 客户端替换；接通后删除原执行实例构造和内部 adapter 依赖 |
| `product/api`、query、TUI/HTTP/Web 当前消费者 | 保留产品入口，投影 Runner 已提交事实；在途 Web 变更以实际新主干为准，不重建其 UI，也不绕过产品服务直接访问 Runner 数据库 |
| publication 的平台调用 | 放在 Runner；SteerLine 经 CLI refresh-pr取得原 provider 的新事实，保留自己的纯归属/完整性/唯一交付检查，避免两仓各维护平台查询实现 |
| execution 配置、debug-template、安装/构建/测试驱动 | SteerLine 保存程序路径、协议、state-dir及选中 RunRequest 配置，settings schema由Runner提供；生成包带固定Runner构建。旧 schema/模拟接线随切换删除 |

不创建两仓共同依赖的“大公共库”，不由 Runner 反向导入 SteerLine。迁移的测试夹具也各有 owner；只复制需要的纯协议/故障能力，不能把完整 SteerLine 产品/测试 runner 成为 Runner 的运行依赖。

## 4. 实施步骤

### P0 固定交付输入与基线

1. 固定两仓文档/代码 commit、Runner module 路径、来源清单及三层交接状态。module 依据新仓库实际远端核对，不根据文件夹名称猜远端 URL。记录在 source-map；不改他人分支或未提交文件。
2. 按第 3 节逐项列出迁出、保留、替代、删除和测试消费者，记录旧协议/类型的最终删除位置。新增配置/状态格式只接收新执行，不搬旧安装数据库。
3. 建立新仓库最小构建、资源内嵌和测试目录；首版发一个 Windows EXE，并维护依赖声明与发布说明。此步骤不安装或替换正在运行的程序。

### P1 落实公共契约与独立宿主

1. 实现 Spec §5 的命令、严格 UTF-8 JSON、退出码、request_id/command_id/run_id查询、公共 DTO、版本和能力输出。核心 JSON/schema用内嵌资源维护；不暴露内部模型。
2. 固定公共配置 schema：Codex 复用 executable/model/reasoning_effort及原固定权限；Orca把transport executable/args/wait_timeout、agent_launch及target.environment分开，路由只能来自冻结target；command/gitea provider复用base_url、query argv、认证引用等必要字段。settings非法或未知字段拒绝，模板覆盖不能扩大权限。把完整字段/默认值补入Spec并附可验证样例后再接业务执行。
3. 实现规范化state-dir、实例发现、目录锁、当前用户ACL命名管道、握手和有界通信。服务单写，CLI不打开DB；状态不兼容在原生准备前拒绝。
4. 实现同EXE的serve宿主和独立后台启动。检查Windows Job继承，不能仅用新窗口/进程组模拟脱离；父Job不允许安全独立启动时返回明确错误且零Run接受。SteerLine调用CLI的超时清理只结束本次CLI，不结束已独立运行的服务。
   原生 breakaway 被禁止时拒绝；创建成功却仍有关联时使用系统 Windows PowerShell/CIM 挂起创建，经用户/完整性/授权组/权限/EXE核验后恢复。先保存启动意图；原创建未知保留 startup_unverified，禁止盲重建。此路径属于 Windows 宿主实现，不增加后台执行引擎或外部脚本资源依赖。
5. 新建服务锁/别名/并发启动/陈旧实例/正常停止/父进程退出用例；无代理环境也可启动服务并查询schema。所有有副作用命令先持久化接受，再执行外部动作。

### P2 提取三层规则和原生 owner

1. 在技术交接满足后，从固定来源commit一次性提取workflow/reliable/backend及实际依赖，改module导入。删除Task/Service/RunView/SQL等泄漏；新Runner持久模型不进入可靠执行包。
2. 搬迁Codex/Orca协议codec、原请求回执和final取证，不改变原native调用顺序、权限或未知输入处理。source-map记录每个原生owner及测试的来源。
3. 提取共享Git/PR/process能力和app内必要注册/构造逻辑，由Runner组装。提供本机仓库入口/克隆缓存到独立workspace的准备，核对remote/base、工作区/分支身份，不重置用户现场。
4. 在新仓运行迁移后的纯规则和codec定向测试。收口默认development/1、四个模板覆盖、机器报告格式及一次补报，不另建通用workflow引擎。

### P3 接通 Run、持久化、操作与结果

1. 定义Runner自己的Run、步骤、Attempt、Command、Operation、问题、事件、PRFact记录和窄提交端口。SQLite仍单写；结果/后继意图与操作来源组合一次提交，日志不成为权威。
2. 开发流程固定文档路径；内容流程保留 inline 材料与摘要校验。接受提交前固定请求正文及实际资源版本；并发同ID只有一次接受；重复请求优先对原正文/回执对账，避免用新默认值重渲染。
3. 开发流程直接向代理提供固定的文档仓库相对路径；不物化材料、不读取文档或校验其存在性。派发和接管不依赖开发材料快照；内容流程仍核验其用户材料快照。
4. 接通准备→实现→报告→发布→pr_ready；PR URL须经原provider/branch归属核验。实现refresh-pr只读平台刷新和明确query_error，保持PR head可变化及同PR反馈规则。
5. 接通问题回答、业务反馈、事件分页和最终结果投影，区分保存/投递/开始/结束。只记录可核实final/必要来源及诊断，不重建完整工具过程采集系统。
6. 实现持久输入授权代次：stop事务撤销new_inputs_allowed，发送前再次核验；已发动作可只读结算但不产生后继。unsupported原生停止保留撤销和能力缺口；resume重新授权满足原身份/旧写者/unknown门槛。
7. 服务换代持久化人工继续门槛并核对旧意图；只重接观察/接受原结果，不发送旧答案、下一步骤或新会话。原始Operation和Attempt不可被迟到事实改投。

### P4 补齐最小受控驱动并跑独立闭环

1. 从现有controlled helper迁移必要的Codex公开app-server、Orca命令/主代理状态和PR查询响应。helper只用于测试，支持准确动作计数、丢回执、部分准备、错身份、缺final及可控制的结束点。
2. 真实构建EXE并从CLI进入服务；测试不能直接写业务SQL/私有state绕过准入。内部提交失败反例可在同owner的unit端口fixture中注入，进程级断线/kill由外部驱动制造，不加生产fault开关。
3. 落实下文U1/U2、C1–C6的真实Test入口。父Job关闭、CLI丢响应/被结束、服务重启用状态/回执屏障定位，不以固定sleep冒充操作已接受。
4. 断言未知输入/发布零重发、授权撤销后零新输入、停服不虚报原生结束、来源/游标及日志目录正确；原分支和PR在反馈后保持。证据记录真实调用次数与版本。

### P5 建立 SteerLine CLI 客户端与跨仓桥接

1. 在SteerLine计划新建 `internal/runnerclient`，通过参数向量和stdin调用固定EXE，解析单JSON/退出码；不导入Runner内部Go包或IPC协议，不打开runner.db。建立有界进程调用和版本/schema检查。
2. 候选启动事务固定领取/候选/待提交request正文与ID，后台经CLI提交，幂等保存Run关联。响应丢失先inspect原request；正文不变，未知零新ID。用户放弃/Task终态期间迟到接受的Run也保留关联并进入待stop，不遗失孤立执行。
3. 候选保存最小绑定（Run、state-dir、request摘要、冻结publication/workspace目标、消费游标），RunnerView是只读投影。问题/恢复资格来自最新Runner事实，版本冲突作为原操作明确拒绝处理。
4. 原native delivery用例改经CLI refresh-pr核验原provider新事实；任务事务只组合任务声明、候选绑定和PR事实，不再写Runner状态。保留正确已绑定候选首次合并即可唯一交付、其他候选未知不阻塞、closed后零PR查询；不新加Runner必须pr_ready才能接受真实合并的门槛。
5. 同Task终态事务保存每个关联Run的待stop意图；收到Runner授权撤销回执后才显示禁止新输入已确认，原生停止事实另列。版本明确拒绝时有界刷新当前Run再建立新stop请求；未知请求只用原ID对账。客户端绝不因native stop失败撤销真实TaskDelivery。
6. 迁移任务级丢弃/候选停止的产品命令到上述桥接，Runner自身仍不认识Task；原有API/TUI/Web消费SteerLine投影。P5完成后运行I1跨仓真实CLI+受控provider测试。

### P6 接通配置/调试/测试并删除原执行实现

1. 在同一个SteerLine切换PR完成接通与删码。移除原worker/actor/adapter/执行存储写路径、旧配置和专用后门，保留任务、候选关系、CLI客户模块及任务交付事务。搬迁TaskDelivery等任务模型后再删除旧包，不能整目录盲删。
2. 更新安装/构建与debug-template。新生成包包含来源明确的Runner EXE，模板只存输入配置，EXE在生成时加入。项目state-dir固定到该包项目运行目录的runner子目录，不共享安装状态。
3. 计划给SteerLine `scripts/new-debug-release.ps1`增加 `-SteerRunnerExe <绝对路径>`，核验版本/hash、记录双仓来源并复制本次构建；这是待实现参数，当前脚本并不支持。保留既有live/simulate方案冻结；模拟通过测试helper配置原生adapter，不增加Runner模拟业务链路。
4. 更新既有testkit、driver/native/evidence和被改动的查询/console测试；运行R1已登记的最小场景。定向编译所有受影响消费者，删除审计核对旧类型/SQL/schema/配置不再被使用。无旧数据迁移或fallback。
5. 二进制/客户端公开版本一致后，SteerLine固定Runner可取得版本。还在运行的服务不可被覆盖或静默接管；测试清理精确处理本次服务/原生资源及未知情况。

### P7 最小真实验证与双仓交付

1. 相关controlled通过后，按本次native manifest跑用户保留的本地Codex最小实现→PR；Orca 两目标已由用户取消验收。同一小任务材料可复用，各目标独立Run/工作区/base/head/PR，不扩大为真实故障矩阵。
2. 真机前核验工具/账号/目录信任/模型配置和主机版本；能力缺失保持未验并如实声明，不以受控environment替代真机。未通过承诺范围的验收不标首版交付完成。
3. 两仓分别提交可审阅 PR，SteerLine 固定远端可取得的 Runner commit、EXE 版本及 SHA，两仓相互引用实际 PR。按用户最新要求，本轮停在审计修复与 PR 交付；合并、预发布及最终版本 I1/R1/live 验收另按已满足的门槛推进，不在本轮自动合并或发布。
4. 同步README使用例、source-map和验收证据，明确未实现能力。源码复制仅限抽取过渡；切换后功能修复归Runner，SteerLine只维护客户侧任务集成。

## 5. 测试计划

全部自动项 [SR-A01–SR-A12](../acceptance/steerrunner.md#2-自动验收项)如下映射。Runner尚无分级runner，本次使用unit/controlled/live的定向Go入口；SteerLine沿用其定向集成和L3入口，不复制一套完整测试调度框架。

### 5.1 入口建设及执行约定

下表含已建基础入口和后续待建设入口；在指定P步骤落地后才能执行，当前可用入口及结果见验收文档。P4需验证每个选定Test非零匹配且无意外skip，P6对R1核验迁移后确实经Runner CLI，没有旧执行fallback。

Runner进程测试接受测试专用环境变量 `STEERRUNNER_TEST_EXE`（或测试参数 `-runner-exe`）和 `STEERRUNNER_TEST_EVIDENCE_DIR`（或 `-evidence-dir`），指向B1实际构建和本次空证据根；仅测试driver消费，生产仍用公开state-dir/请求。`scripts/test-foundation.ps1` 构建后以系统 CIM 启动隐藏独立测试宿主，避免工具自身的 Job 链影响允许/禁止反例。脚本返回仅代表已启动；须等待 result.json 的 exit_code=0、scenario_count=2 且无skip，查看 integration.log/caller-commands.jsonl。driver记录EXE/hash/来源、清理本次owned资源；native组额外使用 `STEERRUNNER_NATIVE_MANIFEST`，只存非秘密配置、双仓身份、目标主机、专属仓库/base/head及状态目录。

live测试使用 `//go:build live`，只有显式 `-tags=live` 和完整manifest才启动真实代理；缺必需前置失败并保持未验。真实调试现场统一由P6扩展后的debug生成器创建在C:/SteerlineDebug；测试现场构造工具不成为Runner发布包依赖。

| 入口 | 级别/场景与命令（cwd明确） | 环境、时点 | 自动项 | 通过条件与本文外证据 |
| --- | --- | --- | --- | --- |
| B1 | Runner构建：`./scripts/build.ps1`（底层 `go build -trimpath`） | Windows/amd64；P1后、代码变更后必要重建 | SR-A01及所有进程组前置 | 一个EXE，version/资源可查询；exe.build.json记录来源/hash，不覆盖运行服务 |
| U1 | Runner unit：`go test ./internal/contracts ./internal/cli -run '^TestSRCLIContract$' -count=1 -timeout=2m -v` | 无模型；P1、公共协议改变后 | SR-A02、SR-A04 | 全部公开命令/错误分类/UTF-8/schema/请求指纹反例有效；cli-contract.jsonl、unit-cli.log |
| U1-S | Runner存储：`go test ./internal/storage -run '^(TestSRShutdownReceipt\|TestSRUnsupportedStorage)$' -count=1 -timeout=2m -v` | 独立临时目录；P1、回执/格式变更后 | SR-A02、SR-A03、SR-A04（命令回执部分） | 停服回执持久、旧ID重试不作用新实例、不同正文冲突、外来格式拒绝且原文件不变；unit-foundation.log |
| U1-H | Runner宿主：`go test ./internal/host -run '^TestSRStartupIntent$' -count=1 -timeout=2m -v` | 同owner临时启动意图fixture；P1、启动逻辑变更后 | SR-A03 | 未核清启动可见、后续启动拒绝、原意图不改且零DB；unit-foundation.log |
| U2 | Runner unit：`go test ./internal/execution/workflow ./internal/execution/reliable -run '^(TestSRWorkflowDevelopment\|TestSRReliableRules)$' -count=1 -timeout=5m -v` | 纯原生事实fixture；P2/P3后 | SR-A01、SR-A05–SR-A07 | 三层依赖守卫、业务/技术规则及原会话/授权/来源反例有效；dependency-audit.md、unit-rules.log |
| U2-E | 来源组件定向：`go test ./internal/execution/... -count=1 -timeout=5m -json`；新增 `TestSRExecutionDependencyBoundary`、`TestSRWorkflowDevelopment` | 仅本次实际迁出的包；P2后，复用来源codec/规则/故障fixture；live opt-in 保持关闭 | SR-A01、SR-A05–SR-A07、SR-A09 | 禁止反向依赖、文档路径与机器格式不可由正文覆盖、原始native身份/未知零重发及来源规则通过；ported-tests.jsonl，单列真正skip，不计真机通过 |
| C-RUN | 真实EXE受控：`./scripts/test-foundation.ps1 -Pattern '^(TestSRRunWorkflow\|TestSRNativeUnknownAndStop\|TestSRRestartRequiresResume)$'` | 隔离Git/SQLite/当前用户服务，独立宿主；测试helper只经Codex公开RPC；P3/P4后 | SR-A01–SR-A09 中相关执行边界 | 实现→报告→真实本地Git push→受控PR核验；反馈更新原PR/分支/session；unknown零重发、stop撤销后零输入、重启零自动输入、当前resume沿原session。fixture原始调用计数、CLI/结果/版本均入本次证据根；不替代其他Run/Orca/跨仓项 |
| C1 | Runner controlled：`go test ./tests/integration -run '^TestSRStandalonePackage$' -count=1 -timeout=3m -v` | 只有EXE的干净目录，无SteerLine资源/代理；P1/P4后 | SR-A01 | version→服务→capabilities闭环；package.log、resource-manifest.json |
| C2 | Runner controlled：`go test ./tests/integration -run '^TestSRServiceOwnership$' -count=1 -timeout=5m -v` | 独立/别名目录、并发CLI、允许/禁止独立启动的父Job；P1/P4后 | SR-A03 | 唯一写者、CLI父进程退出服务仍活、非法接管拒绝；service-identity.json、write-counts.json |
| C3 | Runner controlled：`go test ./tests/integration -run '^TestSRInputFreeze$' -count=1 -timeout=5m -v` | 隔离Git与受控helper；P3/P4后 | SR-A04 | inline/git快照及完整接受、同ID查询/并发/冲突、移动源材料不变；input-manifest.json、request-receipts.jsonl |
| C4 | Runner controlled：`./scripts/test-foundation.ps1 -Pattern '^(TestSRNativeUnknownAndStop\|TestSRRestartRequiresResume\|TestSRPermissionQualification)$'` | 实际 CLI/本地受控 Codex RPC；Orca 两目标另复用其来源 adapter 定向测试并保留未覆盖缺口；P3/P4后 | SR-A06、SR-A07 | 未知零重发、同thread恢复、重启零自动输入、stop授权持久、权限资源不合格零原生会话；integration.log、原调用计数和原生绑定 |
| C5 | Runner controlled：`go test ./tests/integration -run '^TestSRStorageLogsEvents$' -count=1 -timeout=5m -v` | 两state-dir、分页/重复事件、已知秘密测试值与缺final；P3/P4后 | SR-A08 | 日志定位与缺口真实、事件可去重、日志不推进业务；directory-audit.json、events-pages.json、redaction-cases.json |
| C6 | Runner controlled：`go test ./tests/integration -run '^TestSRPublicationIdentity$' -count=1 -timeout=10m -v` | 实际隔离Git push配loopback provider，三目标helper及反馈；P3/P4后 | SR-A05、SR-A09 | 三轮/补报有界、原source、正确PR/错误归属/复用/查询未知与刷新；workflow-events.jsonl、pr-bindings.json、side-effects.json |
| I1 | SteerLine定向集成：`go run ./cmd/test-feedback -- ./tests/integration -run '^(TestSteerRunnerCLIIntegration\|TestSteerRunnerTaskDelivery\|TestSteerRunnerTerminalStop)$' -count=1 -timeout=10m -v` | 真实Runner EXE/CLI、独立双库和受控PR事实；P5/P6后、最终固定版本后 | SR-A10及SR-A06的跨仓边界 | 候选绑定、未知提交、唯一交付/后继、终态待stop重启/逻辑撤销/native缺口；caller-commands.jsonl、task-delivery.json、stop-receipts.jsonl |
| R1 | SteerLine L3 controlled：`go run ./cmd/test-feedback -level=L3 '-suite=delivery,recovery' -env=controlled '-scenario=delivery.basic,recovery.waiting-merge-restart,recovery.codex-resume,recovery.terminal-environment-unknown-send'` | 已有隔离夹具，经P6 driver改为真实Runner CLI；P6/固定版本后 | SR-A10、SR-A06、SR-A07 | 4个当前登记场景非零、实际新链路、闭环身份/恢复保持；summary.json及场景日志 |
| L1 | Runner live：`go test -tags=live ./tests/native -run '^TestSRNativeLocalCodex$' -count=1 -timeout=15m -v` | 新debug包及native manifest，本地真实Codex/SteerlineTest专属PR；P7、相关controlled通过后 | SR-A11 | 小任务实际实现/指定检查/完整final/真实PR归属，所有版本可追溯；native-facts.json、checks.log、pr-facts.json |
| L2 | Runner native + live：`go test -tags=live ./tests/native -run '^TestSRNativeOrcaTargets$/^environment$' -count=1 -timeout=30m -v` | 同上；用户已放弃本机 Orca，仅远端 environment；独立 Run/分支/PR；P7 | SR-A12 | 两端公开原生准备/完成取证及最小真实PR；按target登记native-facts.json、capabilities.json、checks.log、pr-facts.json |
| D1 | 文档/来源/删除审计与受影响闭包编译：两仓`git diff --check`，必要时`go build ./...`；定向`rg`审计实际迁出符号/导入/SQL | 文档完成后；P2/P6及最终 | SR-A01、SR-A10（辅助，不能代替行为验证） | 无漏链接、零旧写者/反向依赖/闲置schema，当前消费者可编译；deletion-audit.md、compile.log |

每组结果登记到验收文档相应SR-A项，并附本次证据根和实际Test/场景数量。源码/规则守卫放在U2的真实负例中；D1文本搜索只辅助。SR-A01–SR-A12均有行为入口，不能以构建或文档检查代替。

不默认跑Full、完整L2、全部候选/恢复/live矩阵或新远端硬件故障链。C4覆盖故障反例；L1/L2只证明首版声明的实际路径。新代码或失败触及其他范围时补相关最小场景，已经通过的组只在新改动或未解决风险影响它时重跑。

### 5.2 人工窗口

[SR-M01](../acceptance/steerrunner.md#3-人工验收)安排在独立CLI和SteerLine接入可用之后，参与者/时间/主机待定，可后置，不作为无关步骤前置。实际执行、归属或恢复失败仍阻塞对应自动项，不因人工窗口后置而豁免。

## 6. 交付与停止条件

Runner独立可用、全部承诺自动项通过、SteerLine固定版本并切换删码后，才标记这次跨仓交付完成。PR/发布按P7执行；基础层阶段不创建发布或修改产品业务状态。

各P步骤只有满足本步骤能力和测试门槛才进入依赖步骤；三层代码尚未交接不阻止P1纯宿主/协议建设，但不能启动P2另拆旧规则。运行验收环境缺失时明确保留未验，不能把文档计划、helper输出或历史结果写作通过。

Spec/Plan不维护实时运行结果。实施源版本、命令、结果和限制登记专属验收；source-map记录出处与实际删除，不恢复roadmap编号、外部Task准入或长期双维护路径。


2026-10-15 用户范围调整：本机 Orca 路径已放弃，不再执行或阻塞本次验收。原 L2 只选择 environment；R1 去掉本机 terminal-local-reconnect，保留本机 Codex 恢复和远端未知输入零重发。历史失败/已冻结运行结果仍保留，不能改写成通过。


最新用户范围：取消全部 Orca 路径验收，本次仅本机 Codex。L2/SR-A12 不再执行，历史结果保留；R1 仅 delivery.basic、recovery.waiting-merge-restart、recovery.codex-resume。补充 SR-A09/A10 的更深路径回归：`go test ./internal/application -run '^TestSRCompactCacheCanPublishLinkedWorktree$' -count=1 -timeout=2m`，以原 hex 布局超过 Windows Git 管理路径界限、当前私有紧凑布局低于界限的实际 linked-worktree push 核验，证据 `compact-cache-deep-publish.log`。本机 L1 使用修正后新 EXE/新 debug 包复验，证据按 SR-A11 登记，不替换冻结现场。


本机产品冷启动的原生版本资格核验曾在 10 秒预算耗尽；该步骤保持单次只读探测，预算与现有外部命令统一为 30 秒，不自动重试、不缓存跳过资格核验、不创建未合格会话。SR-A06/A11 复验：现有 PermissionQualification 定向/实际 EXE 场景和新包本机 live，证据 `native-policy-budget.log` 及对应新包。

审计补充（SR-A06，测试计划）：本机隔离 SQLite/受控原生端口，在本轮修复后、PR 推送前运行 `go test ./internal/application ./internal/execution/adapters/codex -run '^(TestSRNativeAnswerRejectsAmbiguityBeforeIntent|TestAsyncQuestionNativeIdentityOptionsReplayAndFinal|TestAsyncQuestionObserverUsesNewInputCommand|TestAsyncQuestionObserveSteerAndReconcile)$' -count=1 -timeout=2m -v`。原题集精确匹配，重复键/额外题目/固定选项额外答案/空回答/未提供选项须在保存原生意图前拒绝，合法选项 ID 映射为标签；现有原问题路由、原回合和恢复零重发场景不得退化。证据登记至专属验收 §9 与 `.evidence/audit-answer-native.log`。不因这次输入校验变更重跑已通过的完整 workflow/live 矩阵。

安装启动补充测试计划（[WB-A01–WB-A04](../acceptance/windows-broker-install.md)）：

| 级别/场景 | 环境与命令 | 时点、通过条件 | 证据 |
| --- | --- | --- | --- |
| Windows token 定向；WB-A01/A02 | 本机隔离 token/临时 state；`go test ./internal/host -run 'TestBrokerPreservesDisabledPrivilegesAndRejectsMissingRights\|TestSRStartupIntent' -count=1 -v` | 修复后、打包前；仅禁用多启用权限、缺少原权限或启用状态拒绝且不启用、原 token 不变、旧 startup 意图不重试 | `.evidence/token-diagnostic/host-tests.log` |
| 实际 CLI；WB-A03 | 本机构建 EXE；`service start` / `service status`，以固定 instance/request envelope 执行 `service stop`；独立 `.evidence/token-diagnostic/state` | 定向通过后；ready/detached，短命启动者退出后仍 ready，停止核验原实例不在运行 | `.evidence/token-diagnostic/`、SteerLine `.evidence/install-migration/` |
| 产品安装核验；WB-A04 | 用户授权的新安装目录；SteerLine `build-deploy.bat -NoPause`、`status --json` 及各项目 Runner `service status` | 打包后；三项目和宿主/Web ready，正常停启、配置/任务保持；仅安装运行操作，不派发任务 | SteerLine 安装迁移专属验收与安装目录 migration 记录 |

仅宿主 Windows token 恢复发生变化，不跑模型/PR、L2/L3/full；本次空闲安装核验不替代历史任务恢复和 live 验收。

## 执行可见性增量

用户授权的流程/状态展示及最后消息查询独立见 [Plan](execution-observability.plan.md) / [验收](../acceptance/execution-observability.md)。保持单写事务和原恢复规则，不作为原生真机验收通过证据。
