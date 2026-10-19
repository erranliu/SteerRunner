# 个人多机执行器与 gh 单仓库认证

日期：2026-10-19。用户要求按个人多机执行器边界移除 GitHub App 支持，保留 gh 单仓库 broker 和 Run 隔离，并更新产品定位、契约、文档和测试。

## 当前基线与主分支冲突

本文的 helper 链路验证基于 main `95ae8a7`，生产 helper 来源为 `7c6ea40`；不代表后续架构的资格。测试期间 main 合入 `0b55928` / [PR #31](https://github.com/erranliu/SteerRunner/pull/31)，明确改为无远端 helper 的 Orca 原生执行，并移除远端 Runner broker、同工具完整环境检查及相关原生历史/退出证明保证。该增量与本分支的远端安全/审计链存在实质契约冲突。

用户已确认跟随最新 main：保留本机/Orca local 的 gh broker，远端使用 Orca 原生认证，不再保证远端单仓库 capability、helper 审计或完整环境检查。PR #28 继续按该边界整合并重新验证；不恢复被移除的 helper，也不把旧证明套用于新原生路径。下文分别记录当前原生范围与历史 helper 范围，各项资格绑定实际源码和批次。

## 交付范围

- 在本人管理和信任的预配置主机上执行冻结 Spec/Plan；调用方/SteerLine 选择本机或 Orca environment，组织任务与 DAG。
- 工具安装、仓库准备、gh 登录由执行主机用户负责。Runner 管理 Run、反馈、显式恢复、发布对账、停止和清理；完整环境检查限本机受管路径。
- 移除 App 引用、私钥/JWT/installation token 签发与撤销、App 特有就绪/停止证明和测试；不提供团队、多租户、集中密钥托管或自动跨机迁移。
- 公共 Run/Check、冻结 Codex profile、远端 helper 配置严格拒绝旧 github_auth（包括 null）；测试 fixture 同样拒绝 github_auth/remote_github_auth，不能静默改用个人身份。
- 升级前用原版本停止并核实已有 App Run/Check 的认证/进程清理；新版本不迁移旧身份，不接管旧 App 凭据。历史记录和 JSON 保留原结论。

本机 codex-app-server 与 Orca local 受管路径保留每 Run/Check 独立、有限期的 gh 单仓库 capability、固定操作白名单、原身份校验及 broker/socket 清理。宿主凭据不进入这些 worker；停止不退出宿主登录。新远端使用已有 Orca 原生认证，不提供 Runner capability、实际参数回读、双模型来源审计或完整环境检查；停止 scope 为原 orca_terminal。全路径保留 Run 隔离及未知投递零重发。

## 验证要求

组件覆盖旧字段拒绝（对象/空对象/null）、内部 profile/helper 配置拒绝、缺登录零 native 输入、授权到期、跨仓库/跨 Run capability 拒绝、停止幂等和另一 Run 继续、原远端停止 owner 及秘密不进入 argv/持久化事实。正式 L1 包含 PowerShell fixture 旧配置零副作用保护、native driver 编译和零匹配保护；Controlled 保留审计和多 Run。

旧 helper 正式复测曾覆盖 sandbox/full_access 闭环、原会话反馈/PR、双模型审计、同服务多 Run、活动停止、两种权限环境检查及调用方/服务换代。该范围属于下文历史，不代表新原生远端资格。当前选择本机两条 broker 路径、原生远端 full_access、多 Run、活动停止和初始化停止。八项管理员故障沿历史缺口登记；不重复发起此前取消的 UAC，不把授权排除或组件通过记为真机通过。

## 历史 helper 批次与补充修复

首轮源码 `424826d13d5af12fbcd4a0ee8f1a64ad5e3acb8f`、工作树干净。L1 318 项、终端扩展 28 项、Controlled 16 项通过；native 编译/零匹配与 PowerShell fixture 18 项守卫通过。七项缺原生现场的组件跳过不算真机通过。

远端批次 2026-10-19 02:05–03:00 UTC，`MaxParallel=2`：计划 19、管理员排除 8、执行 11、通过 8、失败 3。原始失败保留：sandbox 环境检查未发送任何分项，启动结果未核实；full_access 环境检查 11/12 分项通过但未完成 pr_close；同服务双 Run 中一个发布只看到 push 和建 PR 提示，未读取原异步命令完成结果，按 unknown_external 失败。原记录缺少足够诊断，不能据此断言前两项具体传输根因。

sandbox 检查原 native PID/创建时间和工作目录已独立核实不存在，报告仍保留原操作未知的 residual；full_access 检查原 PR #569 已关闭、专属引用和目录已清理、原写者已停止。同服务两 Run/服务已停止；只读查询后来确认原发布 PR #571，另行关闭该确切测试 PR，不补记原测试成功。

补充修复为原 actor 回执/CLI 读取增加有界只读重查，保持请求发送一次与原身份复核；区分明确未调用 turn/start 的拒绝和未知，持久化有限启动/工具回复/分派诊断。发布提示要求异步 shell 返回 session_id 后读取同一 session 的退出状态和 PR URL。组件新增丢回执、有/无原缓存、暂时/持续读取失败、身份变化、旧 helper 和矛盾 turn 标记、已确认操作后回复失败的守卫。

正式复测构建、命令和有限结果完成后追加；首轮失败及历史验收均不继承为新构建通过。

复测准备提交 `4a0aa59` 的 L1 323 项、扩展 28 项和 Controlled 16 项均通过。其前置期间主分支合入 `a3dc15d`（原生停止证明）；在任何真机 case 创建前中止外层调度，原 Controlled 自行通过并退出。该批次没有 L3 结果，不计为失败重试或通过。

合并同时保留原生退出签名/树为空证明、gh broker 撤销、原 reader 结算、双模型字段与核验。修复合并交互：helper 正常退出返回 shell 后，关闭前继续核对原 runtime/terminal/incarnation/worktree/path/branch，原生退出另验；不依赖已退出 agent 的显示标签。现有正向停止测试复现并验证修复，另加 shell 分支变化拒绝，认证 actor 的失回执测试使用实际 `local` host scope。

合并提交 `f2d6ee3` 的正式 L1 保留一次失败：辅助清理正向夹具将空列表序列化为 null，新完整清单校验正确拒绝。修正为显式空数组，补 null、外来主机及 local 被省略的拒绝子项；本批未启动 Controlled/L3，不能计入真机通过。

`641d460` 的 332 项 Go 与 28 项扩展全部通过，正式入口因合并遗漏主分支既有 `TestOrcaStoppedNativeProbe` 显式 opt-in 跳过名单而失败，未启动 Controlled/L3。恢复主分支名单；八项缺原生/独立现场参数的 L1 跳过只登记，不算真机通过。该修正不改变 helper 二进制。

`e70471a` 正式前置 L1 332、扩展 28、Controlled 16 通过。其 L3 前两项 sandbox/full_access 完成实现、原 PR 反馈及专属基线合入，但停止时完整清单读取耗尽预算；暂停调度后，让已启动的 caller-exit 和 graceful-active 自行收尾，两者也完成原业务但停止未确认。共启动 4 项、通过 0；其余尚未调度，原报告和中断记录保留。

复核确认同工作区原 reader 在 journal 和快照中被逐个重复读取，9 个已关闭 reader 可触发 15 次清单查询，加业务 PTY 共 16 次。修复为每次核验读取一份新鲜、完整、同 runtime/worktree 的清单，按全部原 terminal/incarnation 核验，并去除同一 reader 的重复查询；继续先验签名退出和各 reader 的原生结算，缺创建身份、活动/替换 reader、空身份、外来范围或省略 local 均拒绝。组件新增单清单多身份用例，并要求 journal/快照重复 reader 仅查询一次。

临时编译覆盖的定向测试通过，未修改当时仍在运行的旧源码。该只读实现对前两项原失败服务执行 `service verify-stopped`，约 5 秒及 8 秒返回 resources_settled=true，核验既有原生签名、原 writer 与 reader 清单；没有重启服务或回写旧 Run。物理资源结算不补记原 L3 成功，后续以新提交的新现场复测为准。

`916b206` 的干净源码正式复测：L1 Go 333 项、扩展 28 项、Controlled 16 项通过；L3 执行 11 项，通过 8、失败 3，另有 8 项管理员故障授权排除。full_access 完整闭环、两种服务重启、caller-exit、活动停止、本机反馈、full_access 环境检查（12/12）、同服务双 Run 均通过。停止批量清单修复在该构建中得到真实闭环验证，未借用旧构建结果。

三项原失败保持失败：sandbox 闭环的 Run 与服务均已停止，但独立测试清理清单遇到瞬时 remote_runtime_unavailable；sandbox 环境检查停于 git_admin 响应观察；双模型测试在实现阶段的内容校验失败后修正产物，却因测试提示要求只运行一次而没有重跑校验，未进入审计。环境检查现场已只读捕获原 actor 对同一 git_admin 请求的完整成功响应，ID 与原请求散列一致且可解码，不能据此断言原操作未执行或唯一传输根因。原写者和目录已清理，逻辑未知结论仍保留。

随后修复可独立复现的观察缺口：匹配原响应后遇到瞬时身份读取失败，保留该响应，只重查原 actor 身份；完整响应帧允许前置 PTY 输入回显片段；execute/verify 含排队与观察共用 40 秒上限，响应缺失、响应读取失败和身份读取失败保留有限诊断，未知操作仍不重发。旧源码覆盖新测试能复现响应丢弃和继承十分钟期限，修复后定向测试通过；这些证据不用于追认上述现场的唯一根因。

独立清理测试只对明确的 runtime_unavailable 只读重查，保持原截止时间，并严格拒绝 null 清单、外来范围和替换 incarnation。测试提示允许已确认完成的普通内容校验失败修正后重跑，异步或未知命令仍只能读取原 session。四个相关组件包完整回归与新增有限诊断/零重发用例通过；新的源码提交和不可变 helper 将重新执行全批前置与 11 项现场，不继承上一构建的 8 项通过。

`5b32b1a` 的干净源码前置全部通过：Go L1 335、扩展 28、Controlled 16。该批已启动的 6 项现场全部通过（sandbox/full_access 完整反馈闭环、caller-exit、服务 graceful/kill 恢复、活动停止）。期间主分支合入 `95ae8a7` 的 workflow/2、独立测试配置和固定控制通道，引发冲突。只停止未来调度，两个已启动 worker 均自然通过并退出；其余 5 项未启动。原 summary 保留，派生 drained-summary 明确为 interrupted；这些结果只归属 5b32b1a，不继承为整合构建通过。

整合采用主分支的固定 control actor、原生签名退出字节及 workflow/2；不恢复已经移除的按回合 reader/独立 revoker 路径。gh-only 原始配置拒绝、每 Run broker、原响应只读观察与有限诊断保留；重新清除上游带入的 App 字段、installation broker 和正向用例。远端代码审计修复及必要补报在同一原生会话使用 audit 模型，发布恢复执行模型；配置 schema、能力目录与测试登记同时覆盖远端受管 Codex，保留 Pi/DSH 的非审计边界。最终以整合提交和独立 helper 的新批次为准。


整合核实 workflow/2 的工作树来源原实现仅读取调用方本机 Git，不能支持远端审计资格。补齐原 adapter 的可选审计 owner 接口，以及固定控制通道内只读 workspace-audit；实际使用原执行主机 Git 完整检查，绑定原 session/helper/configuration/workspace intent 与 pre-head，核验各查询 Job 退出。未知 reader 不覆盖、缺 owner 不回退调用方 Git、成功查询不跨阶段复用。定向测试含真实临时 Git clean/dirty/隐藏索引/稀疏/外来仓库与分支/祖先、索引及 HEAD 不变、原进程不受影响，以及原请求失回执对账、模型不符与源证明拒绝。旧按回合 reader 与 revoker 测试由固定通道原归属/关闭测试替代，不恢复 App 正向资格。

`7c6ea40` 的正式入口在 framework guard 阶段失败，Go L1/Controlled 和真机均未启动（0 执行）：上游独立配置守卫仍写死 29 个 L3 case，加入本 PR 两项登记后为 31。修正完整缺配置/前置失败范围断言为 31，缺配置数 26→28，并显式断言远端审计/多 Run 不得继承本机配置。独立框架守卫完成，verified.json 保留 incomplete、31 请求、零执行及 complete_level_passed=false。此变更仅测试与登记文档，生产源码不变，远端继续使用明确归属 7c6ea40 的已冻结 helper；新批次记录新的测试提交。
冻结 helper 来源为 7c6ea40，SHA256 658e4b899c2947f60e529fa297fd165f210cc0221777131986c5d094b4f03bbe；部署于独立目录 gh-only-7c6ea40。后续框架断言/登记变更与该构建的 internal/cmd 生产源码完全相同，分开记录测试提交与 helper 来源，不覆盖旧 helper。

`1700a95` 的整合批次 L1 Go 408、扩展 29、Controlled 16 及框架/编译/零匹配守卫通过。19 项选择中授权排除 8、实际执行 11、通过 9、失败 2；sandbox 完整闭环、两种权限环境检查（各 12/12）、同服务双 Run、调用方退出、服务 graceful/kill 恢复、活动停止和本机反馈边界均通过。原批次保持 failed，不写成一次全绿。

远端审计原生 Go 用例通过，原 Run/服务已停止，远端 Git 来源实际 qualified=true；代码审计使用 gpt-6.1-sol/high，发布恢复 gpt-6-luna/high，均为同一原生会话。外层 PowerShell 仍要求 workflow/1 的三个原生上下文、两项审计、verdict 和 inspector_closed，故原批次失败。`917194a` 改为 workflow/2 的单项代码审计与发布，核验 source/result/pre/post-head、原 Command/Turn、不同 native turn 与 control request、reader_writers_absent 和模型/effort；新增缺失、重复、外来及旧 Spec 审计反例。只读复核旧事实通过新断言不补记旧测试成功，资格仍须新现场。

另一失败为 full_access 启动时 Orca 报告执行主机未响应，原启动请求 `48b5037b-6bec-48b5-6540-dcac4f6e5644` 未核实，Runner 零重发、零业务输入。12 分钟后用例超时；因缺原 native readiness/退出绑定，停止继续保留 unknown，服务已停止。独立只读完整业务流仅含初始 PowerShell 提示符；按原 runtime/worktree/两组 terminal incarnation 关闭该现场遗留终端，新鲜完整清单为空。原控制 helper PID 23972/创建时间 2026-10-19T07:01:42.916671Z 及其子进程已核实不存在。物理清理不补造签名退出，也不把原 Run 的 stop_status=unknown 或原失败改成通过，另存有限物理结算记录。

`917194a` 的审计补跑在 L1 停止，真机执行 0：407 项 Go 通过，模拟 CLI 的 IPC 夹具因与正在运行的真实 Orca 共享队列而耗尽 3 秒期限；扩展 29 和框架/编译守卫通过。`98e4e76` 保留真实 named-pipe 核验，但把夹具 CLI 固定到 fake host，不进入生产 CLI 队列；定向回归通过。生产代码未改动，helper 仍归属 7c6ea40；新正式批次以独立工作区的 98e4e76 为测试来源，不在原批次运行期间修改源码。

`98e4e76` 的旧 helper 定向批次完成：Go L1 408、扩展 29、Controlled 16、两项现场均通过（远端 full_access 与实际双模型审计）。结合同生产代码 `1700a95` 的其余九项，旧 helper 范围的十一项各有明确通过记录；原批次九通过二失败仍保持 failed。额外 `917194a` full_access 的正式前置及一项现场也通过。有限 JSON 与 SHA 清单在 evidence/personal-runner-gh-only-20261019，legacy-helper-qualification.json 明记 latest_main_qualified=false。

整合 #31 后的新验收范围：保留本机/Orca local 的 broker 与 Run 隔离，原生远端无 helper/capability。退役远端审计和完整 Check 正向登记，增加纯校验及 capability 的明确拒绝用例；同服务原生多 Run 固定 full_access、核实独立请求/命令/会话/事件/PR，服务停止后离线核验两个原终端 scope。新场景注册为 21 场景/28 case；Sandbox 远端参数与失败语义沿受控验证，不沿用旧 helper 的真机 sandbox 通过。当前原生验收等待新整合提交的正式前置及独立现场，不继承上面的旧资格。

随后 main 合入 #30 的原生初始化停止补丁（c65b193）；在独立工作区整合，当前 aa02b65 批次继续使用冻结源码。新停止分支仅允许尚未确认启动、零 turn 的原 terminal 缺分支元数据，仍核对确切 worktree/path/runtime/incarnation，不授权任何业务输入。新增 before-readiness 与 restart-before-readiness 两项登记，总计 22 场景/30 case；按新提交补跑完整前置与两项实际初始化停止，不把正常完成后的停止证据当作初始化验证。

## 当前原生批次（资格与历史分开）

`aa02b65`（live-13）正式前置：L1 Go 420、终端扩展 29、Controlled 16 和框架/编译/零匹配守卫全部通过。五项实际执行四通过、一失败，另排除两项管理员故障。原生远端 full_access、多 Run、活动停止和本机 codex-app-server 闭环均通过；多 Run 的两个原 PR、停止互不影响及服务释放写者锁后的离线终端 scope 均核验。

Orca local 的 sandbox 准备检查通过，后续 READY 回合原生完成记录为 server_overloaded（Selected model is at capacity），未发送业务输入。原测试保持失败，Run 停止因缺正式 readiness/退出绑定而未核实；broker/socket 已撤销。独立核对原 runtime/worktree 和两项 terminal incarnation 后关闭，完整清单为空，原 native PID 不存在；物理清理不提升逻辑停止结论。保留原模型和权限的新 Run 复测另记。

`e045d3c`（live-14）合入 main #30 的初始化停止：L1 Go 421、扩展 29、Controlled 16 与框架守卫通过；restart-before-readiness 真实 case 通过，before-readiness 在 fixture 运行时缓存准备阶段遭遇 Windows 文件共享冲突，业务执行为零，原批次保持失败。缓存已原子发布仍不足以避免并行哈希/硬链接/复制之间的共享冲突；补同缓存目录互斥、期限及 abandoned owner 处理，守卫核验持锁期间禁止发布、释放后两个冷缓存 worker 字节完整一致。补跑以新干净测试提交记录，不继承原失败为成功。
