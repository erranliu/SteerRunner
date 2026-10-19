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
