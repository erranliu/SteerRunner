# 个人多机执行器与 gh 单仓库认证

日期：2026-10-19。用户要求按个人多机执行器边界移除 GitHub App 支持，保留 gh 单仓库 broker 和 Run 隔离，并更新产品定位、契约、文档和测试。

## 交付范围

- 在本人管理和信任的预配置主机上执行冻结 Spec/Plan；调用方/SteerLine 选择本机或 Orca environment，组织任务与 DAG。
- 工具安装、仓库准备、gh 登录由执行主机用户负责。Runner 管理环境检查、Run、反馈、显式恢复、发布对账、停止和清理。
- 移除 App 引用、私钥/JWT/installation token 签发与撤销、App 特有就绪/停止证明和测试；不提供团队、多租户、集中密钥托管或自动跨机迁移。
- 公共 Run/Check、冻结 Codex profile、远端 helper 配置严格拒绝旧 github_auth（包括 null）；测试 fixture 同样拒绝 github_auth/remote_github_auth，不能静默改用个人身份。
- 升级前用原版本停止并核实已有 App Run/Check 的认证/进程清理；新版本不迁移旧身份，不接管旧 App 凭据。历史记录和 JSON 保留原结论。

保留每 Run/Check 独立、有限期的 gh 单仓库 capability、固定操作白名单、原身份校验、未知投递零重发及 broker/socket 清理。宿主凭据不进入 worker；停止不退出宿主登录，也不影响其他 Run。远端双模型审计、同服务多 Run、Pi/DSH 及显式 sandbox/full_access 保持各自契约。

## 验证要求

组件覆盖旧字段拒绝（对象/空对象/null）、内部 profile/helper 配置拒绝、缺登录零 native 输入、授权到期、跨仓库/跨 Run capability 拒绝、停止幂等和另一 Run 继续、原远端停止 owner 及秘密不进入 argv/持久化事实。正式 L1 包含 PowerShell fixture 旧配置零副作用保护、native driver 编译和零匹配保护；Controlled 保留审计和多 Run。

远端正式复测使用新独立现场和冻结的新 helper，覆盖可运行的非管理员范围：sandbox/full_access 闭环、原会话反馈/PR、双模型审计、同服务多 Run、活动停止、两种权限环境检查及调用方/服务换代。八项管理员故障沿历史缺口登记；不重复发起此前取消的 UAC，不把授权排除或组件通过记为真机通过。

## 首轮与补充修复

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