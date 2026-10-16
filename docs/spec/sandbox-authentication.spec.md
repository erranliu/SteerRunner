# 沙箱认证准备与发布验收

本次独立验收见 [acceptance](../acceptance/sandbox-authentication.md)，实施与测试映射见
[Plan](sandbox-authentication.plan.md)。沿用 SteerLine
`docs/product-design/worker-lifecycle.md` 的 W05–W07、W16、W18 和 E/V 规则；不新增状态机。

## 契约

- SA01：可信执行主机准备认证。默认使用该主机已有 gh 登录，宿主凭据仅留在可信
  runtime 内存；worker 获得有限期、单仓库、限定操作的代理 capability。Git push 和
  gh PR 请求由沙箱内原工具发起，代理只验证范围并认证转发。拒绝跨仓库、管理接口、
  未知操作和重定向，不由宿主执行发布命令。支持可选 GitHub App installation token，仅
  contents/pull_requests write、metadata read。私钥仅通过所在主机环境变量引用解析，
  不复制宿主 gh 配置/钥匙串，也不将宽权限 OAuth token 暴露给 worker。
- SA02：首个业务输入前，使用实际 app-server 的 `command/exec`、相同 MXC、
  workspace-write、网络、工作区和 Git roots 验证工具环境、Git 管理目录读写、
  Git remote 和 GitHub 身份/目标仓库访问（可选 App 校验 installation 范围）。检查不创建模型回合。
  准备失败具体阻断、零业务输入；准备结果绑定原进程/工作区/权限/输入与期限。
- SA03：代理 capability（可选 App 模式为单仓库 token）只注入子进程环境；模型生成代码能够读取该环境，这是明确的信任边界。
  不通过 argv/config set、文件、数据库、manifest 或输出传递秘密。限定继承，
  不继承宿主 OAuth 或 App 私钥；到期阻断新输入，退出关闭代理或撤销 App token。
  Runner 异常退出时代理随进程消失，capability 立即不可用；App token 最多剩余一小时。
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

旧 L2 `C:/srt/96158bdc5ce7` / PR #272 和 L3 `C:/srt/2cd651115cc9` 保留原结论，
其审批路径不作为本次 SA04/SA08 通过。仅在新独立现场验证；禁止触碰安装环境。

## 既有节点、异常与验证的本次差异

这里记录对共同生命周期的增量，不定义新的状态或恢复规则。

| 既有节点/分组 | 本次动作与正常出口 | 异常/恢复边界 | 最小验证 |
| --- | --- | --- | --- |
| W05/W06、V01/V02 | 执行主机解析 gh 登录、启动单仓库代理（或可选 App 签发）；原 endpoint/protocol 握手 | 缺登录、401、目标/权限/期限/传输能力不符具体阻断；不创建业务 thread/turn，不自动重试登录 | broker/issuer 受控反例、缺认证零 native start |
| W07、V02/V03 | 相同 app-server `command/exec`、固定 workspace/Git roots/MXC/network，验证 AppContainer、工具环境、Git 管理目录、GitHub 身份/仓库及 remote | 任一工具事实缺失/过滤/拒写/网络/身份失败阻断；不是模型业务回合，不消费或刷新恢复授权 | 确定性 RPC 受控用例、MXC 无效 canary、默认 broker 真实正例及 L2；可选 App issuer 受控 |
| W16、V05/V08 | 原 worker 在 never 策略下执行原发布器，复用原 PR；provider 独立只读核验 | 过期或权限拒绝不放宽策略；未知结果沿原分支/PR 对账，不能重复发布 | publisher 固定身份与 query-failure 夹具、原生权限审计、L2 闭环 |
| W18、V07 | 原 owned 树退出后关闭代理/所有连接并清空内存凭据，或撤销可选 App token；保存非秘密关闭/期限事实 | 清理未确认另记 blocker；保留工作区/历史/PR | broker 关闭、issuer 撤销、原进程停止及 native cleanup.json |
| 服务换代、V06 | 原身份/Attempt/期限/显式 resume 规则保持 | 不从准备探针恢复旧业务输入，不从历史 ready/旧进程继承秘密；没有实际新准备证据不发送 | 既有重启/未知投递/撤销受控前置 |
| 远端 W05–W07 | 必须由实际执行主机提供可信认证与同沙箱确定性能力 | 当前 terminal transport 缺能力：unsupported，零业务输入；只读对账/停止原资源仍可用 | remote capability 反例；本次 L3 未验 |

只读检查不承诺远端写权限、分支保护允许或 PR 创建权限。首次真实发布、反馈更新和完整
收尾必须单独取证。关闭 shell snapshots，且从命令行 pin 中排除 token 和 Git HTTP header；
capability（可选 App 为单仓库 token）与目标 URL 范围内的 header 仅在 worker 进程
及其工具子进程环境中存在；宿主 OAuth 始终留在可信认证代理内存中。
