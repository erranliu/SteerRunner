# Spec：开发执行统一 full_access

日期：2026-10-31。状态：代码已实施，最终 L1/Controlled 和原生无模型专项通过；真实 L2/L3、read_only/parser 真机资格未完整验证。实施阶段结果见 [实施与验证](../acceptance/development-full-access.md)，后续审计修复及当前交付验证见[审计记录](../acceptance/development-full-access-audit.md)。

调查基线：`a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。实施见 [Plan](development-full-access.plan.md)。

## 1. 目标与适用范围

开发工作流 `development/2`、`development-audited/2` 只支持显式 `execution.settings.access_mode=full_access`，移除开发 sandbox 的准入、默认值、权限编译、启动包装、运行检查和重复测试组合。适用于所有当前受支持的开发执行链路：本机 codex-app-server、Orca local 受管 Codex/Pi/DSH、Orca environment 原生 Agent。既有 Agent、target、审计模型及后端组合限制继续生效；统一权限不新增支持组合。

保留 `conversation/1`、`generate/1`（含固定仓库版本调查）的 `read_only`；保留结果获取中独立 Codex parser 的只读执行。保留现有认证链路和可靠执行语义。本次不引入认证简化、容器/虚拟机、权限插件、自动配置迁移器或第二套生命周期。

本规格实施后，在开发权限、默认值及其测试矩阵上取代下列文档的冲突条款：[产品契约](steerrunner.spec.md)、[独立配置](independent-configuration.spec.md)、[沙箱认证](sandbox-authentication.spec.md)、[Orca 本机/远端](orca-local-remote.spec.md)、[远端原生](orca-remote-native.spec.md)、[通用 Agent](orca-generic-agent.spec.md)。其中认证 owner、停止与结果对账规则仍适用。内容与 parser 的只读契约继续分别以 [内容流程](content-workflows.spec.md)、[仓库调查](generate-repository.spec.md)、[结果获取](result-acquisition.spec.md)为准。

## 2. 行为契约

### DFA-01：公开配置必须显式声明权限

| 上下文 | 接受 | 拒绝 |
| --- | --- | --- |
| 开发 RunConfiguration / Run input | `access_mode: "full_access"`，且符合原组合限制 | `sandbox`、`read_only`、省略、`""`、null、非字符串、未知值 |
| 内容 RunConfiguration / Run input | 本机 codex-app-server，显式 `read_only` | `full_access`、`sandbox`、省略、空值及原本不支持的组合 |
| 独立 execution 校验 | 本机 codex-app-server 显式 `full_access` 或 `read_only`；orca-terminal 显式 `full_access` | 无权限默认值；完整 workflow 另做组合校验 |
| 内部 parser | 原固定只读配置和工具限制 | 不继承主开发 worker 的 full_access |

`capabilities`、内嵌 settings schema、动态 configuration schema、纯配置 gate、完整 submit gate 与 testkit 使用同一规则。开发 capabilities 只发布 full_access；内容条目保留 read_only；去掉空模式默认和 sandbox 组合。现有 CLI 信封、workflow id/version、外层 state 格式不因本项自动升级；能力资源摘要必须反映契约变化。

模式拒绝沿用 `invalid_input` / exit 2，并在既有错误/issue 结构中定位 `execution.settings.access_mode`，说明开发模式须显式改为 full_access、旧 sandbox 已退役。`validate` 沿其既有结果信封呈现原 gate 结论，不另建错误协议。保留既有错误优先级，不覆盖更早的协议或认证退役字段错误。

无效请求在受理前拒绝：零 Run 创建、零 fetch/worktree/Agent/认证 broker 创建和零业务输入。内部 normalize、freeze、profile、catalog matching、manifest 不能再把省略或空值解释为 sandbox 或 full_access。启动参数覆盖仍按现有校验拒绝，不能用额外 args 重新启用 workspace-write 或切换审批模式。

### DFA-02：开发原生权限只有 full_access

Codex 开发启动、thread/start 和 turn/start 固定 `danger-full-access` / `dangerFullAccess`、`approval_policy=never`、原 reviewer。移除开发 workspace-write/MXC 的参数和策略分支，包括 `sandbox_workspace_write.*`、开发专用 `windows.sandbox=mxc`、为授权写入而产生的 `--add-dir` 及 Pi/DSH 的 Codex sandbox CLI 包装。不能改为 `--dangerously-bypass-approvals-and-sandbox` 或交互审批。

Orca remote 仍只承诺按配置传递启动参数，不新增实际权限回读、OS 隔离证明或执行主机认证 broker。通用 Agent 维持原原生权限管理范围，不由 full_access 字段推导新增的安全保证。

Git 对象、refs、logs、worktree admin/common、工作区和认证目录仍须按原用途识别、绑定与校验。删除的是这些路径的沙箱写授权；工作区归属、防路径替换、临时目录定位、认证 socket 绑定和 Git 可用性检查继续保留。现有 `LinkedWorktreeArgs` 等载体若被认证/工作区共同消费，可保留内部表示，不因名称含 Write/Sandbox 而整块删除。

Pi/DSH 开发启动及单次工具执行不再要求安装、发现或探测 Codex sandbox 可执行文件。它们继续通过现有 Runner 工具宿主、原生 TUI 扩展及 owned Job 执行，保留 provider 环境过滤、脚本/状态路径验证、取消、后代退出和收尾；本次不恢复 Agent 默认不受管工具，不改为 SDK 执行。

### DFA-03：认证链路保持

| 执行链路 | 保留的认证行为 |
| --- | --- |
| 本机 codex-app-server；Orca local 受管 Codex/Pi/DSH 中原本使用 GitHub broker 的任务 | 执行主机已有 gh 登录、单仓库 broker/capability、认证代理/CA/API socket、原环境过滤、期限、绑定和停止清理 |
| Orca environment 原生 Agent | 沿现有 full_access 路径使用执行主机已有原生登录；不新建 Runner broker/helper，不从调用方传 token |
| 内容 workflow / parser | 沿各自既有登录、环境及工具限制；不因开发 full_access 注入 GitHub broker 或扩大环境 |

保留原本启用 broker 的本地受管路径首个业务输入前的无模型准备：实际工具环境、Git 管理目录读写、remote、gh 身份、仓库访问及 Git transport；沿现有 full_access 分支验证非 AppContainer 执行事实。准备结果仍绑定原进程/工作区/操作/配置与期限；失败、认证到期均零后继业务输入，不自动换登录、后端或重试发布。非 GitHub 仓库沿各后端已有支持范围及认证方式执行，尤其不能给本机原生 Codex 已支持的非 GitHub 路径新增 gh 登录、GitHub broker 或 GitHub 身份预检前置，也不据此扩展 Pi/DSH 的仓库支持范围。

`sandbox_readiness.go` 同时承担 full_access 认证准备，不能整文件删除；其中仅 workspace-write/MXC 正向分支退役。允许整理名称，但保留仍被日志/消费者使用的有限 stage 名称；改名不得成为额外诊断协议迁移。

移除 Orca 原生 sandbox 独有的 Git/认证启动包装时，保留现有 full_access 原生认证路径；不得把本地 broker 改成直接继承宿主凭据，也不得将本地 broker 强加到远端。broker 限定代理请求的范围，不是 full_access 进程对宿主文件及凭据访问的隔离保证。

### DFA-04：read_only 与可靠性保持

内容流程保留原生 read-only/MXC、网络及工具限制、外部 MCP/apps/plugins/hooks 隔离、创建/恢复时权限验证、目录身份核验。parser 保留 read-only、禁工具/联网等原参数及 Deadline/source/receipt 绑定。本次不得全局删除字符串 `sandbox`、`windows.sandbox` 或所有权限编译器。

开发执行仍保留每 Run 的工作区与分支、冻结配置、原 request/command/session 身份、问答与授权、未知副作用零重发、服务换代对账、原 PR 核验、活动写者停止和认证清理。`pr_ready` 后拒绝业务反馈，停止未知保留 unknown/unreported。独立 worktree 不宣称为操作系统权限隔离。

### DFA-05：配置迁移与旧 Run

这是一次开发权限能力收缩及显式授权边界变化。新构建不将任何历史 sandbox/省略模式请求自动提升为 full_access。

1. 调用方修改其可编辑配置源，显式写入 `full_access`，用新构建离线 capabilities / validate 核对后再提交新请求。保留既有 full_access 配置；内容 read_only 配置不改。客户端发现仅生成 full_access 开发候选；加载旧默认/项目配置时给迁移诊断，不静默覆盖或自动另选候选。
2. 新建 Run 使用新 request_id。旧已受理请求按原幂等规则查询/对账；不能复用旧 ID 配新正文，也不能用相同正文重试创建一个更高权限 Run。旧非法且未受理请求修改后亦使用新 ID，避免不明确的调用方重试状态。
3. 冻结 Run 的 Request、InputDigest、profile/resource envelope、Attempt、原操作、认证身份与证据不改写。新构建遇到旧开发 sandbox 或省略/空模式冻结记录时，分类为退役只读，禁止新服务发起 actor 自动接管推进、reply/resume、新回合、parser、发布以及准备资源创建。即使旧记录尚未启动 worker，也不能补启动为 full_access。
4. 旧显式 full_access 记录仅在原冻结契约、owner/terminal、认证期限、workflow 和 operation 均被现行恢复校验支持时保持原接管语义；不能通过重编 argv、换 owner、重新登录或重建会话来制造兼容。形状不兼容的旧记录只读并报告不支持执行。只允许有限旧数据解码/参数验证，不保留 sandbox 的执行栈。
5. 旧 read_only 记录继续走原恢复规则。开发 policy/tools contract 变化不能全局使内容或 parser 失去兼容资格。实施时区分开发与只读契约的版本选择；需要变更 envelope/receipt 形状时登记具体新版本及受影响范围。
6. 旧记录可查询原历史、结果和停止事实。精确 stop 仅在已有身份/停止协议被支持时调用原 owner 或 Orca 原终端；不创建业务 lifecycle/helper 来兼容。证据缺失保留未结算，由原构建或人工处理。旧不支持项不拖垮同目录中兼容 Run 的服务启动与执行。

退役是新服务的执行准入分类，不是原生进程的暂停或停止证明。已在旧 owner/终端运行的回合及工具可能仍继续产生文件或外部副作用；新服务不自动终止它们，也不宣称它们已经停止/撤销认证。实际 worker、parser、broker/socket 结算仍须单独核实。展示的退役诊断/可用操作是只读投影，不把 `execution.settings.access_mode` 改成 read_only，不改写持久业务授权、Status 或原结果。

分类须在自动 PR 核验、进度/结果文件准备与采集、parser 恢复及正常 lifecycle 构造之前生效，并覆盖直接控制入口与所有公开读取投影。退役记录仅提供持久事实查询及上述精确 stop；不通过 PR refresh、原生会话 open、工具查询/辅助进程或后台采集恢复旧运行栈。精确 stop 所需的原身份/退出/认证关闭核验可沿原停止协议执行；已有 stop 请求可对账原停止结果，但不得派发旧业务意图。

本次固定的契约边界如下；版本用途不同，不能用新全局常量校验所有旧记录：

| 契约/记录 | 新写入与恢复规则 |
| --- | --- |
| Codex 开发权限 | 新开发采用 `codex-policy-v11:0.159.2:full-access:repository-auth-broker:never`；版本基线与认证保证不变。profile codec 3、policy resources codec 4 的字段形状未变，保持版本 |
| Codex read_only | 保持原 v10 policy contract、只读参数和 codec；content readiness 也引用只读契约，parser 独立契约不变 |
| 基线显式 full_access 的 v10 policy resource | 保留有限 decoder，严格验证原 envelope 身份、模式、原允许的 argv 和 owner 协议；旧 workspace-write 配置键及写根只作为冻结 argv 数据验证，不用于生成新启动命令。不能套用删减后的新 argv 集合重新认定旧凭据/权限 |
| Pi/DSH 新受管启动 | `windows-terminal-tools-v2:full-access:never`；terminal receipt 3；metadata 4；新 local terminal-profile codec 5。新配置去掉 sandbox EXE；准备回执以 `outside_write_verified=true` 记录原有 full_access 正向 canary，不再输出 `outside_write_denied` |
| 基线 Pi/DSH 的旧 full_access | local profile codec 3、metadata 3、receipt 1/2 只按原版本、冻结 helper/bundle/evidence 摘要和 tools v1 contract 读取；原 `outside_write_denied` 必须为 false。新服务仅连接/停止原 helper，不补启动旧 helper/工具或用新 bundle 替换原文件；原 helper 在原授权内运行工具仍使用其原冻结实现 |
| Orca 原生远端 | native remote profile codec 4 和既有停止 scope 不变；新请求 gate 拒绝 sandbox，旧模式依冻结请求分类，不新增 helper 字段 |

旧 v10 full_access 或旧终端回执只有在模式明确、数据一致且原恢复资格满足时才兼容；损坏/矛盾/未知契约明确 unsupported。被动解码不授予重新创建已丢失 owner/会话/认证资源的资格，旧格式不得被新写入路径复用。不同版本回执使用各自严格结构解码；不能通过忽略未知字段、补 false/true 或缺失摘要回落到当前 bundle 来伪造兼容。

推荐升级顺序：停止旧版本受理新任务；在原版本完成或显式停止受影响 Run，并核实 worker、parser、broker/socket 均已结算；保留原数据和仍在用的版本文件；更新客户端配置与产品/helper/扩展包；用新版本检查接管清单和配置后再提交新任务。`service stop` 本身不等于 Run 或认证资源已结算；未结算项仍按既有部署门槛阻断。新版本不承诺接管任意历史构建。

外部 SteerLine 等调用方在自己的配置源完成相同迁移，本仓提供变更说明与契约样例，不在本任务中修改外部仓库、部署环境、用户安装或真实默认配置。

可编辑配置的最小迁移示例（仅展示 settings 片段，其余字段保持）：

```json
{"access_mode":"sandbox","model":"configured-model"}
```

改为：

```json
{"access_mode":"full_access","model":"configured-model"}
```

原来省略或填写空模式的开发配置也必须显式补入 full_access；这一步代表配置所有者接受完整权限，不能由读取旧配置的程序自动完成。旧冻结请求不使用此替换；内容配置仍为 `{"access_mode":"read_only"}`。

### DFA-06：测试矩阵收敛

所有保留的开发正向配置显式 full_access，包括 `configurations.local-codex` 基线、audited、mixed、多 Run、结果获取、进度、等待重试、停止及恢复场景。不得在 fixture 根据场景名把用户配置从 sandbox 强改为 full_access。旧模式配置静态不可用/拒绝，不降低缺项和 incomplete 判定。

| 当前条目 | 实施后的处理 |
| --- | --- |
| `orca-local.pi` 与 `orca-local.pi-full-access` | 保留 `orca-local.pi`，使用 `orca-local-pi` full_access；合入后者独有断言，退役后者 |
| `orca-remote.smoke` 与 `orca-remote.full-access` | 保留 `orca-remote.smoke`，使用 `orca-remote-codex` full_access；合入后者的参数、PR 终点拒绝及收尾断言，退役后者 |
| `orca-remote.service-restart` 的 active 两套权限 case | 保留 `graceful-active`、`kill-active`，均 full_access；合入重复项独有断言后退役 `graceful-active-full-access`、`kill-active-full-access` |
| `graceful-boundary`、`kill-boundary` 及 stop/publication-unknown/initialization-stop 等 | 保留故障窗口和原断言，改用显式 full_access；不得因权限合并删掉窗口 |
| `orca-local.smoke` 与 `orca-local.codex` | 本次都保留；它们的路径差异另有契约，不以权限合并为由一起删除 |
| test config 的 `orca-local-pi-full-access`、`orca-remote-codex-full-access` 角色引用 | 全部改为对应 canonical 配置；用户若两份配置不同，迁移前自行选择，不自动覆盖合并 |

registry、`tests/catalog.json`、manifest、fixture、framework、脚本选择器和文档必须同步。退役场景/case 名按未知选择明确拒绝，不增加静默别名。当前 environment-check/旧 Check 已退役，不为本改动恢复它们。历史验收日志、JSON、原生 session 与旧数量保留原字节，不将旧 sandbox 验收改称 full_access 通过。

### DFA-07：验收标准

| 验收项 | 必要证据 |
| --- | --- |
| 准入与零默认 | schema/gate/catalog/CLI 一致；开发 sandbox/省略/空/null/错误类型均拒绝且无副作用；内容仍只读 |
| 删除闭合 | 开发新启动无 workspace-write/MXC/写授权包装；Pi/DSH 无 Codex sandbox 依赖；保留项有明确用途 |
| 认证不退化 | broker/socket/期限/环境/目标绑定/失败零输入/停止清理反例及原 full_access 准备正例 |
| 旧记录不升权 | sandbox 与缺模式退役、幂等不变、原数据不改写、兼容 full_access/read_only 接管、混合状态目录及精确停止 |
| 可靠性不退化 | L1/Controlled 通过；停止、未知结果、换代、问答、PR 终点及 parser 只读边界保留 |
| 真机资格 | 同一最终构建的本机 L2 和受影响的已支持后端专项；缺环境明确记未验/incomplete，不用受控或历史证据代替 |

full_access 的实际权限为执行账号已有的权限；文档与配置迁移说明必须明确其可写工作区外路径，不再承诺开发写隔离。真实性、历史证据和未验范围按 [测试指南](../testing.md)登记。完成实现与实际后端资格分别记录。
