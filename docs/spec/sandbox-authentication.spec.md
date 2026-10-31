# 沙箱认证准备与发布验收

2026-10-31 当前开发权限修订：开发 sandbox 已退役，开发必须显式 full_access；read_only 与原认证链路保留。开发权限、迁移、旧 Run 与重复测试矩阵以 [统一权限 Spec](development-full-access.spec.md) / [Plan](development-full-access.plan.md) 为准。下文历史构建与验收事实不改写。

2026-10-19 范围修订（已实施，验收见专项记录）：新远端 `orca_environment` Run 按[仅 Orca Spec](orca-remote-native.spec.md)使用执行主机已有登录及代理权限，不要求 Runner broker、helper readiness 或同 app-server 确定性预检；不得将本节旧保证记作新路径通过。旧冻结 Run 与本地受管路径保留原规则，历史验收不重写。

本次独立验收见 [acceptance](../acceptance/sandbox-authentication.md)，实施与测试映射见
[Plan](sandbox-authentication.plan.md)。沿用 SteerLine
`docs/product-design/worker-lifecycle.md` 的 W05–W07、W16、W18 和 E/V 规则；不新增状态机。

## 权限模式作用域

本专项 SA02/SA04/SA08 的 workspace-write/MXC/零旁路结论仅约束 `sandbox` 分支。
显式 `execution.settings.access_mode=full_access` 沿普通 Run 权限契约；不作为沙箱失败回退，不计为 SA04/SA08 通过。两模式共用 SA01/SA03 的认证 owner，
各自首个输入前必须完成与实际模式一致的无模型准备；`full_access` 核实非 AppContainer 执行事实，
不伪造 MXC 证据。历史 SA 验收记录保持原结论。

## 契约

2026-10-23 版本范围修订：本机及 Orca local 受管路径的原生 Codex 最低版本为 `0.159.2`，按语义版本比较，无上限；高于基线的预发布版本可进入资格核验。既有协议、权限、认证与进程检查继续执行。契约标识中的 `0.159.2` 保留为最低版本基线，不代表实际运行版本；实际版本按原生探针记录。历史真机验收仍只证明各记录中的具体版本。

2026-10-19 产品边界：本机与 Orca local 受管执行使用宿主已有 gh 登录的单仓库 broker；新远端只使用已有 Orca 原生认证，不创建 Runner capability、不管理远端登录期限或撤销。全路径移除 GitHub App 签发、私钥引用和 installation token。旧 `execution.settings.github_auth`（包括 null）在 Run 受理前拒绝；旧 profile/helper 配置严格拒绝，不静默改用宿主身份。升级前须用原版本收尾已有 App Run，历史证据不改写。

Orca local 的受管 helper 在受保护目录接受绑定原 session/configuration 的停止请求，关闭该 Run 的 broker 并写出有限证明；调用端核验原固定控制终端及原业务 PTY。证明失败仍须终止原 worker，并保留认证收尾未验证；原 worker 停止不能替代 broker/socket 清理证明。未知控制创建不补建，丢失读取/关闭只对账原控制身份。宿主 gh 登录及其他 Run 的授权不受影响。

- SA01：可信执行主机准备认证。使用该主机已有 gh 登录，宿主凭据仅留在可信
  runtime 内存；worker 获得有限期、单仓库、限定操作的代理 capability。Git push 和
  gh PR 请求由沙箱内原工具发起，代理只验证范围并认证转发。拒绝跨仓库、管理接口、
  未知操作和重定向，不由宿主执行发布命令。不复制宿主 gh 配置/钥匙串，也不将宿主凭据暴露给 worker。
- SA02：首个业务输入前，使用实际 app-server 的 `command/exec`、相同 MXC、
  workspace-write、网络、工作区和 Git roots 验证工具环境、Git 管理目录读写、
  Git remote 和 GitHub 身份/目标仓库访问。检查不创建模型回合。
  准备失败具体阻断、零业务输入；准备结果绑定原进程/工作区/权限/输入与期限。
- SA03：代理 capability只注入子进程环境；模型生成代码能够读取该环境，这是明确的信任边界。
  不通过 argv/config set、文件、数据库、manifest 或输出传递秘密。限定继承，
  不继承宿主凭据；到期阻断新输入，退出关闭该 Run 的代理。
  Runner 异常退出时代理随进程消失，capability 立即不可用；不退出宿主 gh 登录，不影响其他 Run。
- SA04：固定权限为 workspace-write/MXC/network enabled/approval never，禁止提权、
  auto_review 和全权限旁路。失败不放宽权限，不换工具版本、模型或后端。
- SA05：W16 发布仍由原 worker 完成。只读预检不证明 push/PR create 权限；只有
  本次真实提交、推送、创建/复用 PR、反馈及收尾才证明完整发布。
  原分支/PR 查询、结果未知零重发和恢复授权保持。
- SA06：任务样本只描述产物、范围、实现和最小验证。测试配置承担阶段、发布器、
  故障注入和收尾责任；删除认证修复/审批指令，不能移动到另一份任务文档。
- SA07：远端必须在执行主机解析认证并提供与实际工具一致的确定性沙箱检查。
  现有 Orca terminal 协议若不能提供此能力，明确 unsupported、零业务输入；
  不能用宿主检查或模型回报替代，也不能从调用方传私钥/token。
- SA08：每个 live worker 审计原生权限、工具与审批；零提权请求、零沙箱外审批、
  零 bypass。驱动准备 base、故障注入及合并与 worker 发布分别记证据。

Git 2.55 的已知兼容边界：成功 commit 后无条件删除不存在的 `AUTO_MERGE`，会在四个
Git 写根之外尝试创建共享 `.git/packed-refs.lock`；该清理写仍被拒绝，不扩大权限。
只有独立保存的 commit 自身退出码为 0、原分支 HEAD/父提交/任务改动/干净工作区均已核实，
且不存在 AUTO_MERGE 或未完成 merge/cherry-pick/revert，才将这一条精确诊断记录为非致命警告。
不得补执行或修复被拒清理；非零/未知退出码、其他拒写及任何未核实 Git 状态仍阻断。
运行时指引须明确要求顺序暂存精确任务路径、核对非空且仅含任务路径的 staged diff，
然后提交并立即保存 commit 自身退出码；退出码捕获模板不能省略暂存前置步骤。
真机工具边界测试保留该警告，并独立断言共享 config/packed lock 拒写和原 commit 成功；
不能把模型忽略警告或复合命令最后一项的零退出码当作提交证明。

旧 L2 `C:/srt/96158bdc5ce7` / PR #272 和 L3 `C:/srt/2cd651115cc9` 保留原结论，
其审批路径不作为本次 SA04/SA08 通过。仅在新独立现场验证；禁止触碰安装环境。

## 既有节点、异常与验证的本次差异

这里记录对共同生命周期的增量，不定义新的状态或恢复规则。

| 既有节点/分组 | 本次动作与正常出口 | 异常/恢复边界 | 最小验证 |
| --- | --- | --- | --- |
| W05/W06、V01/V02 | 执行主机解析 gh 登录、启动单仓库代理；原 endpoint/protocol 握手 | 缺登录、401、目标/权限/期限/传输能力不符具体阻断；不创建业务 thread/turn，不自动重试登录 | broker 受控反例、缺认证零 native start |
| W07、V02/V03 | 相同 app-server `command/exec`、固定 workspace/Git roots/MXC/network，验证 AppContainer、工具环境、Git 管理目录、GitHub 身份/仓库及 remote | 任一工具事实缺失/过滤/拒写/网络/身份失败阻断；不是模型业务回合，不消费或刷新恢复授权 | 确定性 RPC 受控用例、MXC 无效 canary、默认 broker 真实正例及 L2 |
| W16、V05/V08 | 原 worker 在 never 策略下执行原发布器，复用原 PR；provider 独立只读核验 | 过期或权限拒绝不放宽策略；未知结果沿原分支/PR 对账，不能重复发布 | publisher 固定身份与 query-failure 夹具、原生权限审计、L2 闭环 |
| W18、V07 | 原 owned 树退出后关闭代理/所有连接并清空内存凭据；保存非秘密关闭/期限事实 | 清理未确认另记 blocker；保留工作区/历史/PR | broker 关闭、独立 Run 授权撤销、原进程停止及 native cleanup.json |
| 服务换代、V06 | 原身份/Attempt/期限/显式 resume 规则保持 | 不从准备探针恢复旧业务输入，不从历史 ready/旧进程继承秘密；没有实际新准备证据不发送 | 既有重启/未知投递/撤销受控前置 |
| 远端 W05–W07 | 指定执行主机上的可信 `steerrunner-agent`，复用认证 owner，在实际 app-server 上完成同权限无模型预检；inline TUI 保留准备回执，Runner 持久化后直接发送首个业务输入，无额外控制提示 | 未配置 helper 仍 unsupported；构建/配置/原会话/transport/期限不匹配阻断，零业务输入；未知启动不重发；只读对账/停止原资源仍可用 | 缺能力、身份/摘要/期限/旧 transport 反例及启动零重发；真实闭环和权限审计按新构建单独登记 |

只读检查不承诺远端写权限、分支保护允许或 PR 创建权限。首次真实发布、反馈更新和完整
收尾必须单独取证。关闭 shell snapshots，且从命令行 pin 中排除 token 和 Git HTTP header；
capability与目标 URL 范围内的 header 仅在 worker 进程
及其工具子进程环境中存在；宿主 OAuth 始终留在可信认证代理内存中。
