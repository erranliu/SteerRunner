# Plan：独立使用的双 Skill 薄壳

日期：2026-10-23。状态：P1–P6 已实施，本次验证与实际资格见[验收记录](../acceptance/skill-shell.md)；下文保留实施分解和验收目标，未执行的完整矩阵不视为通过。依据：[Spec](skill-shell.spec.md)，源码调查基线 `1cfe2f9a065830ab62d540d3de4c0011ed603b56`。

## 1. 实施原则与产物

先完成调用方脚本与公开 CLI 的确定性接线，再加入 Runner 的最小展示导出、独立 Viewer、简短 skill 和打包。Runner 仅增加 Spec §7 的非权威 JSON 投影，不改变执行内核、业务状态机或 CLI。外壳与 Viewer 不直接 import internal 包或读取 Runner/Orca 私有存储；Runner 自己从既有 Run 投影取数。测试可以使用本仓受控设施，发行物不能依赖它们。

已实现以下产物：

| 位置 | 作用 |
| --- | --- |
| `client/steerrunner-client.ps1` | 两个 skill 共用的唯一 helper；有界发现、配置、请求保存、CLI 调用、结果传递及打开 Viewer，无持续采集 |
| `skills/steerrunner-configure/SKILL.md` | 无参数发现配置入口及缺口解释 |
| `skills/steerrunner-run/SKILL.md` | Spec+Plan 执行、原 Run 后续管理及打开 Viewer 的入口，不另加查看 skill |
| Runner 内部最小快照导出组件 | 服务内单写者生成 Spec §7 的完整 JSON，生命周期附属于原服务 |
| `viewer/` 独立构建目录与 `steerrunner-viewer.exe` | 仅依赖文件契约的 Windows 查看器，有独立构建入口，不引用 Runner 模块/内部包或要求其源码参与编译 |
| 各 skill 的 `agents/openai.yaml` | 简明展示元数据，保持普通自动发现 |
| `scripts/install-client.ps1` | 放置两个 skill，生成 assets/client-location.json；保留已有用户文件 |
| `scripts/package-client.ps1` | 将 Runner EXE、独立 Viewer EXE、helper、两个 skill、安装脚本及各自版本摘要组成发行包 |
| `tests/client/`、客户端专用测试入口 | 受控 CLI/工具夹具、隔离 Git 与真实公开 CLI 联验，不加入用户运行依赖 |

helper 只提供内部固定动作：discover/configure、submit、observe/inspect、reply/resume/stop、按需证据读取及打开 Viewer；这些不是新的 Runner 命令。默认 configure 不带其它参数，路径和宿主上下文由定位信息及当前工作目录取得。内部动作输入输出用有限的结构对象，保留原 CLI 错误/退出码，不建设新公共 RPC 协议或脚本插件框架。持续展示由服务导出与 Viewer 文件读取组成，不通过 helper 轮询维持。

## 2. P1：配置、定位与确定性调用

1. 定义 Spec §4 的客户端文件外层结构，profiles 原样保存公开 `{execution,workflow}`；项目记录区分调用方 Git common-dir 与执行端 repository.local_path。实现严格读取、规范化身份、缺字段诊断、原子写入和短期写锁/冲突处理。
2. 实现统一进程调用：参数数组启动，stdin 明确 UTF-8，解析 stdout 单 JSON 与独立 stderr，保存实际 exit code。支持系统 Windows PowerShell，不依赖 PS7、Node/Python 或复杂 shell 拼接；路径带空格/非 ASCII、JSON 引号和大输入不能改变 argv。
3. 实现安装定位资产读取、EXE 选择、version/离线能力获取和生产 validate；缓存仅限本次调用。不要复制 schema 条件或维护支持矩阵。配置损坏时停止写入，显式路径失败时不静默替换。
4. 固定并显式传入 state-dir；在线能力/服务绑定从公开响应读取。区分离线 EXE 和在线服务身份；服务冲突不自动停服、切新目录或升级。

完成条件：SS-04、SS-10 的配置/参数/隔离反例通过；单 helper 可由发行目录独立调用，尚不发送任何业务请求。

## 3. P2：无参数发现与配置体验

1. 发现已配置/发行包/PATH 的 Runner、Git、原生 Codex、gh 和 Orca。复用 Runner 声明核对静态适用性，区分 shim 与可用原生 EXE；模型未知沿公开缺省，不调用模型探测。
2. 用当前 Orca 版本的公开 help/机器接口固定发现命令及允许输出字段。本次已核对 `environment list --json`、`status --json`、`repo list --json`、`repo show --repo <selector> --json` 和明确 `--environment` 路由；列表定位后用详情核对仓库身份，缺详情保留待确认项。`host list` 中 SSH 目标不映射为 environment。不读取私有 pairing store，不增加底层 native RPC 客户端。
3. 探测子进程屏蔽隐式 Orca 路由变量，固定 CLI、串行查询、有界期限。environment 枚举不要求本机 runtime 已运行；分别记录本机 runtime 与各远端连接，避免本机 status 失败就跳过全部远端。只允许公开字段投影，失败诊断不原样转储秘密。
4. 发现当前项目身份、remote/base 分支线索及对应发布 host 的 gh 认证事实。无法推断的信息保留缺口；非 Git cwd 只保存机器配置。Orca 项目通过对应端 repo list/show 匹配身份，名称/URL 相同但路径有多个候选时让用户选择，不能拿调用方路径猜远端路径。
5. 按 Spec §3.3 保存首次默认、保留既有选择；候选全部不可用仍完成诊断。按 §3.4 分别给出缺 Orca 的安装提示、启动提示、environment 配置及连接/项目修复提示。安装 URL 在实现发布时核对官方来源，不在 helper 中执行下载。
6. 给 GitHub command provider 提供符合现有 PRInfo 的固定 gh 查询模板，并沿既有 Gitea provider 形状处理用户已有配置。优先复用 gh 的结构化输出/转换能力，不生成依赖 testkit 的查询脚本、不新增 provider。模板必须验证实际字段、head SHA、merged/closed 状态和 host/仓库一致性，不能把原始 gh JSON 直接当 Runner PRInfo。
7. 显式完整环境检查复用现有 environment-check start/inspect/stop，先展示副作用范围；普通无参数配置无该调用。远端检查 unsupported 单独解释。

完成条件：SS-02、SS-03、SS-04 的发现与选择用例通过；一次无参数调用可给出可解释结果，无 agent、worktree、终端、PR 或自动安装副作用。

## 4. P3：提交记录、执行及后续管理

1. 从项目方案和用户材料装配公开 RunRequest，固定明确 commit 或目标远端分支 tip。必要 fetch 仅限已选 remote/分支引用；保留原工作区和分支，失败不退旧引用。用真实 Git 对象及正常 clean/换行规则检查 Spec/Plan 的规范 blob 身份，接受普通 CRLF checkout，拒绝实际未提交的内容变更；不自动 commit/push 或变更原材料协议。
2. 提交前保存原 v1 信封、摘要、request_id、EXE/state-dir/项目身份。所有后续写操作复用同一保存机制；按命令排他派发，崩溃窗口先查原 ID。调用记录只保存关联与公开回执，不复制 workflow。
3. 正式执行时 start/connect 原目录服务，取得在线能力和完整 binding，使用 expected-binding 调用 submit 和后续 resident 操作。服务换代后只通过公开身份重新绑定当前实例，保留原提交记录，原请求正文不重渲染。
4. 优先接线 observe 同快照读取；事件游标按实际消费保存，last-message 仅作展示。request-id 查询用于找回原受理结果；退出码 6、无/坏输出、进程中止或超时均不能自动换 ID 新提。
5. 接线 reply/resume/stop：现读 controls、revision 和 question_id，保持用户明确意图与原目标。旧 revision/问题拒绝时展示新事实，不自动重新回答、改内容或恢复。停止与停服分开。
6. 跨聊天以项目身份及调用记录查找原运行；显式 run_id 优先，多个候选只读列出供选择。没有记录可按用户给定 state-dir/run_id 查询，不扫描数据库、不模糊重建请求。
7. 展示 accepted/业务阶段/问题/PR/资源停止各自事实；pr_ready 后按当前 controls 处理请求。按需走公开 artifacts/workspace-git 取证；有限等待后返回定位，不安装后台轮询。

完成条件：SS-05、SS-06、SS-07 通过，且可用实际公开 CLI 在隔离服务上完成绑定/受理/查询接线；不得为通过测试修改 Runner 原业务/恢复判定。

## 5. P4：Runner 最小展示导出

1. 固定 Spec §7 的 UTF-8 文件契约、默认目录和复用 host.Normalize 的 source_id 计算；用文件格式样例验证字段，无新增 RunList CLI、IPC、配置服务或存储迁移。导出全部已持久化 Run，不依赖客户端 submissions，首版不混入 environment-check，不做跨机采集。
2. 在原服务加载就绪后建立单个串行导出者，每 1 秒采样既有投影；不挂业务提交回调/队列。短时持原有读取保护，只复制白名单字段为独立 DTO，释放锁后按 run_id 排序、比较与写入；workflow 只取 id/version，禁止序列化完整对象。变化则发布，无变化每 5 秒心跳重新取数。未提交/加载不完整/读取失败不能发布新鲜或伪空快照。
3. generated_at 固定为取到一致投影的 UTC，不在写入完成/重试时后移。每次发布尝试先分配当前实例的新 sequence，失败/替换未知不复用，允许跳号，仅用内存计数。最多一份在途副本，无积压/重叠；用非 JSON 后缀临时文件，关闭后原子替换。替换前失败保留原文件，替换未知不删目标/不假定未生效；收尾后重新取数发布新序列。连续失败时尝试间隔至少 5 秒且诊断限频。文件 I/O 不在业务事务/受理回执路径同步等待，不触发 provider/原生查询，不修改 Run revision/事件。
4. 单次文件发布 2 秒预算，超时取消并确认原操作收尾。停服先撤销新发布、取消并收尾在途写入，在既有停服期限内确认结束后才释放 writer lease；未能证明退出则报告既有停服未核实/失败，不能留下仍可迟写的任务再释放锁。服务换代覆盖同 source 文件，实例/序列变化不建立第二来源；失败/崩溃/停服保留最后文件自然过期，不补造 stopped。

完成条件：SS-11、SS-13、SS-14 的服务侧受控验证通过，包括非薄壳提交、大小写/目录别名同源、多个 state-dir、workflow 附提示覆盖/审计参数不泄漏、回滚、慢写的采样时间/取消/收尾、重启和旧写者退出。新增投影不阻断业务或正常启动；不重复原生执行验收来证明文件导出。

## 6. P5：独立 Viewer

1. 只实现第 7 节文件 DTO，不 import Runner 内部类型、不链接 Runner 业务库；独立构建/发布为可复制运行的 Windows EXE，避免要求最终用户另装 UI/开发运行时。构建输入只含 Viewer 自身源码与文件契约样例，不需要 Runner checkout/EXE。
2. 默认读取 `%LOCALAPPDATA%/SteerRunnerView/snapshots/`，另接收单一可选本机 `--snapshot-dir`。每 2 秒后台枚举 JSON，单文件读取 2 秒预算，取消慢读、不积压同来源任务/接受迟到覆盖；及时关闭文件，允许原子替换。UI 与其它来源/期限计时不等待单文件 I/O。按 source_id/run_id 整体替换；缺目录与目录枚举错误区分，不做业务控制、日志查看、终态分类或原生/PR 查询。
3. 仅在进程内保存最后有效视图和期限，沿 Spec §7.4 明确 age/剩余期限算法；读取完成才计算年龄，14 秒旧文件只剩 1 秒，同序列错误恢复不续期。受控时钟覆盖回拨/前跳/未来值/睡眠，序列比较只比较已识别字段、忽略 JSON 重排/未知附加字段。损坏/删除/无权限/未知版本保留旧内容并标错；一份有效新快照中消失的 Run 才移除，不清空其它来源。
4. 窗口关闭即取消并收尾读取，不启动 Runner/helper 或残留采集进程，不写缓存库/修复文件。用静态文件即可运行，验证四列、未知状态原样展示、PR HTTP(S) 链接白名单、普通文本标题及字段排除；文件内容不能触发程序执行。

完成条件：SS-12、SS-13、SS-14 的 Viewer 侧验证通过；可把 EXE 和夹具复制到无 Runner 的隔离目录独立展示。首版不做文件监听、全局注册中心、额外后台服务或操作按钮。

## 7. P6：两个 Skill 与独立发行

1. 使用 skill-creator 约定写两个短 SKILL.md：purpose、何时调用 helper、意图映射及必须保留的业务区别。schema、重复调用和失败恢复机制只放 helper，不在 prompt 中重复实现。配置 skill 示例首先是直接 `$steerrunner-configure`，无额外说明。
2. 创建元数据并保持自动发现；配置描述不吸附普通开发请求，执行描述只在用户选择 SteerRunner 或明确继续其 Run 时发起动作。对两个 skill 运行 skill-creator 的 quick_validate；它只证明格式，行为仍按下节验证。
3. 复用 build.ps1 的 Runner 输出及独立 Viewer 构建输出，打包 EXE/helper/skills，生成各组件版本与 SHA 清单。Viewer 只需支持文件 schema_version，不要求与 Runner 相同构建号。安装脚本生成两份轻量定位资产指向同一 helper，不复制 helper，不覆盖外部同名 skill；重装保留配置、调用记录、快照与活动服务。
4. run skill 的查看意图通过 helper 打开发行包根目录 Viewer；用户也可直接双击 EXE，启动失败不影响原执行。不增加配置 skill 的必填参数或第三个 skill。
5. 在无开发 checkout 的临时用户目录按真实包布局安装，验证带空格/中文路径、搬迁后的明确错误、已有同名 skill/活动 EXE 冲突和卸载/重装不丢调用记录。单独复制 Viewer 验证无 Runner/额外运行时仍可读夹具。更新 README、docs 索引与验收入口，说明本机依赖、展示新鲜度和真实支持范围。

完成条件：SS-01、SS-10、SS-12 通过；产物可独立安装、两个 skill 不依赖源仓路径，Viewer 可单独交付，尚未取得真机证据的链路不标记已通过。

## 8. 验证矩阵与交付门槛

验证按以下层次推进。所有脚本写入隔离临时目录，受控假工具要记录调用动作，用计数证明没有业务副作用；不靠匹配提示词或文档标题代替行为验证。

| 范围 | 场景与断言 | 对应验收 |
| --- | --- | --- |
| 配置/调用单元 | 原子写冲突、坏配置/未知版本、两端路径、显式路径失效、生产 validate 拒绝、UTF-8/argv/退出码、凭据投影 | SS-02/04/10 |
| 发现受控矩阵 | 无工具、仅本机、多个候选、非 Git cwd、重复保留选择；Orca 缺失/停机/空列表/离线/认证失败/项目歧义/SSH；部分超时仍保留本机结果，零 agent/创建/安装 | SS-02/03 |
| 真实隔离 Git | remote tip 与本地 HEAD 不同、旧 tracking ref、fetch 失败、Spec/Plan 不在基线/工作副本改变；明确 SHA 不漂移，无工作区修改 | SS-05 |
| 调用故障与并发 | 保存前失败零发送；发送后断连、落盘回执前崩溃、两 helper 同命令、正文损坏、恢复中 not_found；submit/reply/resume/stop 同 ID/正文，不重复业务 | SS-06 |
| 公开 CLI 受控联验 | 实际 Runner EXE/服务、完整绑定、错实例零受理、observe 分页、旧 revision/question、服务换代及查回原请求；不读 Runner DB | SS-07/10 |
| 包与 skill 行为 | 干净安装两入口定位同 helper；无参数配置；明确 Spec/Plan 执行、进度、回答/恢复/停止、多项目同名材料、跨聊天定位；冲突保留用户文件 | SS-01/04/07/10 |
| Runner 导出受控联验 | 非薄壳提交、多 state-dir/别名同源、空列表、workflow 附正文不泄漏；回滚/加载失败无假新鲜快照，周期采样/心跳/失败退让、慢写不改采样时间、替换成功但回报错误仍不复用序列、Windows 读共享、取消与停服未核实不越过 lease；无业务回调/额外 revision/原生/provider 查询 | SS-11/13/14 |
| 独立 Viewer 文件夹具 | 无 Runner/运行时、四列/多来源、目录缺失与无权限、坏/删/未知版本保留旧内容、有效快照删行；14 秒旧文件剩 1 秒、同序列错误恢复不续期、重排/附加字段不误判、时间异常/睡眠/新实例；标题/PR 与零 CLI/数据库/缓存库访问 | SS-12/13/14 |
| 展示生命周期 | 关闭聊天/helper 后持续更新，关闭 Viewer 不影响执行；服务结束旧 running 标过期；慢读/枚举错误不冻结其它来源与过期计时，晚到结果不覆盖新值；慢写超时必须收尾，导出不可写不影响业务或正常启动 | SS-11/12/14 |
| 本机真实闭环 | 同版发行包、本机 Codex/Git/gh，在隔离授权仓库执行 Spec+Plan；核验 PR/provider/head，中途问答、关闭客户端后查询、停止及资源事实 | SS-08 |
| Orca 真实闭环 | 至少一个已有 environment，真实发现、目标端项目绑定、普通 development 执行、PR 核验及原终端停止范围；记录远端完整检查不支持 | SS-09 |

实施后增加 `scripts/test-client.ps1` 作为受控客户端测试入口，支持显式隔离输出目录。公开 CLI 联验使用同一次 build 产物；客户端包使用相同 EXE/SHA。沿项目现有测试分层执行适用回归，不把两份 skill 的打包强加为所有 Runner 调用方的前置。

受控入口为 `scripts/test-client.ps1 -RunnerExe <本次构建的绝对EXE路径> -OutDir <隔离证据目录>`，覆盖配置、发现、真实隔离 Git、调用故障和公开 CLI 接线；可选 `-ViewerExe <绝对路径>` 再执行 `scripts/test-viewer.ps1` 的原生窗口文件夹具验证。包/skill 安装验证另由 `scripts/test-client-package.ps1 -PackageDir <包目录> -OutDir <隔离目录>` 执行；服务导出由 `tests/integration/display_test.go` 的两个专项用例验证。Viewer 独立模块内执行 `go test ./...`，不能从 Runner 根模块以 `go test ./viewer/...` 代替。各命令及真实模型现场见验收，不把受控结果或已有 Runner 真机通过记作新客户端资格。

本次已执行客户端/Viewer 专项、受影响生产包回归及本机/远端真实模型闭环。展示导出只沿现有读取保护和服务生命周期接线，不改业务提交通知；持久化回滚、服务换代和 lease 收尾有定向证据，未重复全部远端可靠性矩阵。时钟使用受控时间，慢读使用真实可取消 Windows 管道，慢写收尾使用受控 publisher；实际文件系统永久挂起等未注入情形单列。真实测试 PR 已关闭且未合并，原 Run 和服务资源结算通过，并保留 Orca 仅原终端的停止范围。

验收证据登记在 `docs/acceptance/skill-shell.md`，允许公开的原始回执与结果保存在该专项 evidence 目录，包括发行清单、摘要、请求关联、PR/head、停止范围、快照样例及失败/未验项。完整现场保存在 `.evidence/skill-shell/`，不打进发行包。未知和 unsupported 保留原值；本次一个 Orca environment 真实通过，不推导其它 agent/权限/操作系统组合均已验收。

完成后复核产物仅有两个 skill、一份共享调用实现、Runner 的最小文件导出、一个独立 Viewer EXE 和必要安装打包资源；Runner 公开 CLI、数据库格式、workflow 及业务权威保持。新增公开接口仅为 Spec §7 文件契约；无第三个 skill、后台采集进程、RPC/注册中心或通用 UI 平台。发现 CLI 本身不足时先记录具体缺口、评估已有公开路径，不顺带增加通用配置服务或新执行引擎。
