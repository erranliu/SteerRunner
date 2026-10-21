# Spec：通用 Orca Agent 接入

日期：2026-10-18。状态：审计修补及必要验收完成。Codex、Pi sandbox/full_access、DSH sandbox，以及同服务 Pi/DSH 混合场景已在同一冻结构建批次真实通过；完整交付、真实投递丢响应、恢复、活动写者停止及构建绑定见[验收记录](../acceptance/orca-generic-agent.md)。采用现有 terminal/TUI 与 Runner 配套正式扩展，未修改 Orca。

本 Spec 扩展 [SteerRunner 产品契约](steerrunner.spec.md)：继续使用 `orca-terminal`，由 Orca 管理工作区和终端，由 Runner 对接 agent 的原生事实。目标是**已适配的 agent 只改配置，新 agent 补少量适配**，不承诺所有 Orca 可启动的 TUI 自动具备可靠执行能力。

## 1. 范围与硬约束

1. 执行仍为 Runner → 现有 Orca terminal → agent TUI。验证 Codex、Pi（`pi`）、DeepSeek Harness（`dsh`）的 `development/1`、业务 needs_input/reply、同会话反馈、发布和独立 PR 核验。
2. 不修改、打补丁、重编译或要求升级 Orca；不新增 Orca 命令、字段、能力声明，不伪装 agent 为 Codex，也不修改其全局 hooks/配置。
3. 不启用 Orca structured agent session，不直连 agent SDK/RPC，不以 ACP、headless 或普通一次性 CLI 替代本期 TUI。允许通过 agent **已有的进程内正式扩展 API**观察/拦截原 TUI 的输入和事件；扩展不另建会话或执行引擎。
4. 扩展源由 Runner 维护，按本 Run/受管实例显式装载，不修改 agent 核心或用户全局 profile。不能装载、不能隔离或缺少必要事件时，该对象保持 unsupported/blocked。
5. 复用现有 adapter、operation、lifecycle、observer、stop gate、恢复和持久化；不建设插件平台、能力注册中心、事件总线、常驻旁路服务、第二套状态机或通用进程管理框架。
6. 原操作先落盘、单写者、未知只对账、服务重启零自动输入和独立 PR 核验保持。`codex-app-server`、审计 workflow、独立 `environment-check` 不扩展；旧 Codex terminal 的权限、输入、证据语义不升级。

首期正向范围为 Windows/amd64 执行主机、target=local。保留原 `orca_environment` 和已验 Codex 远端行为；Pi/DSH 的远端扩展装载、回执读取和停止没有真实证据前不声明支持。

## 2. 调查事实与候选入口

调查基线为 Runner `0e34595f2e0c799f9920d1cd348344f51f06b105`、本机 Orca `1.4.224`；已检查本机 Pi `0.87.1`、DSH CLI `0.1.5-rc.2` 的包内文档/声明，并对照官方文档。以下是入口调查，不是真实任务通过记录；实施须固定实际版本，不把在线主线的字段反推到已安装版本。

| 对象 | 已有入口及限制 | 本方案用途 |
| --- | --- | --- |
| Orca terminal | Pi/DSH 可取得输入接受回执；input_accepted 不等于 turn_started，provider/observation 可能为 unsupported。开始证明名单不覆盖两者 | 保留原 request/terminal/incarnation 及投递对账；不足的开始事实由 agent 扩展提供，不改名单 |
| Orca 状态投影 | working/blocked/waiting/done；Pi/DSH 缺同等 mainAgent 完成事实；lastAssistantMessage 最多 8000 UTF-16，无完整标记 | 状态仅作提示，不作为新增 agent 的开始、完成或完整正文证明 |
| Pi 正式扩展 | 输入、before_agent_start、message_end、agent_before_settle、agent_settled 及会话上下文 | 关联原输入/会话，结合最终消息、结束原因和 settled；agent_end 后仍可能重试、压缩或续跑，不能单独结算 |
| DSH 正式插件 | agent 的 scoped 生命周期/inbox、session/event、持久记录及 flush 等入口 | 核验原 root agent、输入归属、真实 turn/end 原因、完整 assistant 记录和最终静止状态 |
| DSH 现有桥接 hook | SessionStart 可为边界 done；Stop 在 turn-stopping 阶段，后续 hook 还可能要求继续；子事件身份有歧义 | 不把桥接投影当作最终完成；本机 TUI/profile 及插件装载仍待核实 |

Pi 扩展及 DSH 插件是正式扩展点，不等于 Runner 接管 agent SDK。DSH 的一次性 headless 不满足本期同会话反馈，不能用于替代 TUI 验收。

参考：[Pi 扩展](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md)、[DSH Agent](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/core/agent/README.md)、[DSH Session](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/core/session/README.md)。具体事件和持久性须以选定版本的声明及测试确认。

## 3. 责任边界与最小改动

| 编号 | 所属 | 责任 |
| --- | --- | --- |
| R01 | Runner 配置/application | agent 校验、默认 Codex、冻结、查询投影及受管扩展装载意图；不新增公共 adapter 或第二份模型配置 |
| R02 | Runner 原 lifecycle/native proof | 结合 Orca 投递事实与扩展原生事实，核验原请求、会话、turn、结果；application 仍为业务事实唯一持久写者 |
| R03 | Runner 现有终端客户端/本机读取 | 原请求对账，直接读取本 Run 的有界原生事实，再存入原状态载体；不预设宿主服务或另开诊断终端 |
| A01 | Runner 配套 Pi 扩展 | 通过 Pi 正式 API 取得原 TUI 输入、会话、最终消息、原因及 settled，转为 Runner 可核验的事实 |
| A02 | Runner 配套 DSH 插件 | 在原 TUI/profile 中按 root agent/session 收集接纳、回合、最终正文、原因及持久检查点，排除子 agent 与桥接误判 |
| H01 | Runner + 执行主机 | 按 agent 核验启动参数、权限、受限认证、owned 写者和原资源恢复；先复用既有 helper/进程事实，缺能力就阻断 |
| 固定依赖 | Orca | 仅使用现有资源、terminal send/read/show/close 和查询；不承担新增统一 agent 执行协议 |

扩展负责原生事实适配，不决定 workflow、PR 归属、授权恢复或后继输入。Orca 投递回执和扩展事实各自保留来源，不能把 Runner 派生记录说成 Orca 的 turn_started。Codex 沿用现有可信来源，不为统一外观强制迁移到新扩展。

本机 Orca 的 agent 标签是显示启发式；实测 Node bundle 启动的 Pi 标签可为空，DSH 可被误判为 Gemini。托管 Pi/DSH 不能借该标签确认或否定 agent 身份，仍必须绑定原 runtime/worktree/PTY/incarnation，并核验冻结 helper、原生进程创建身份、所持 Windows Job 和正式扩展的原 session/装载身份。标签不能替代上述门槛。

Pi 必须同时支持默认 `sandbox` 和显式 `full_access`。Windows 首期通过原 TUI 的正式工具扩展，将实际工具交给 Runner 的短期工具进程；`sandbox` 使用已验证版本的 `codex sandbox` OS 工具运行器，`full_access` 直接执行同一固定 PowerShell。该用途不启动 Codex 模型、app-server、SDK 会话或任何替代 agent。两个模式都在实际工具入口核验 OS token、允许目录读写、外部写边界、模型凭据环境隔离与仓库受限认证，不能互借通过结论。

扩展回执仍只有一个 Run 私有有界文件通道；readiness 中的最新 root 输入 ID 是原生边界指针，具体状态/结算沿该输入的原回执读取。它用于排除旧空闲状态和其它输入，不增加工作队列、业务状态机或重试授权。初始化和每个后继输入继续受原 lifecycle 门槛控制。helper 自身接口仅供 Runner 冻结版本/内容和运行工具，Orca 不补接口。

复用的是现有 LifecycleReceipt/TurnEvidence、operation 和状态写入流程，不代表复用 Codex 的 RemoteReadiness/RemoteTurnReceipt 协议、native ID 规则或 Orca mainAgent 时钟。Pi/DSH 的来源与原始身份按实际事实映射，不伪造字段来适配 Codex DTO，也不为统一名称先重构整套 helper。

原 O01–O03 的“修改 Orca 补接口”工作取消；其行为要求分别落在 R02/R03、A01/A02、H01。若现有 Orca 加正式扩展仍不能证明某项事实，报告该组合不支持，不再以改 Orca 作为退出条件。

## 4. 配置与兼容

保持 `steerrunner-cli/v1`、`execution.adapter=orca-terminal` 和现有 target/transport/access_mode/agent_launch。`settings.agent` 是可选字符串，省略按 Codex；必须匹配 `^[A-Za-z][A-Za-z0-9_-]{0,63}$`，null、空白、控制字符等返回 invalid_input。

```json
{
  "workflow": {"id": "development", "version": 1},
  "execution": {
    "adapter": "orca-terminal",
    "target": {"kind": "local"},
    "settings": {
      "agent": "pi",
      "transport": {"executable": "orca.exe", "wait_timeout": "30s"},
      "access_mode": "sandbox",
      "agent_launch": {
        "executable": "C:/AgentTools/pi.cmd",
        "runtime_executable": "C:/SteerRunnerRuntime/steerrunner-agent.exe",
        "model": "deepseek/deepseek-flash",
        "reasoning_effort": "low"
      }
    }
  }
}
```

此例要求先安装受管 helper、Pi 0.87.1、Node 和 native Codex 0.159.2 权限工具。合法配置缺少正式扩展、准确参数映射、权限或读取/停止能力时，返回 unsupported_capability，或在异步准备中明确 blocked，零业务输入。

受管扩展的版本、内容摘要和实际装载/原 session 关联沿现有冻结材料及 implementation envelope 保存，不增加用户自报可信的 enabled/verified 开关、脚本模板或可任意指定的回执路径。默认启动可复用 Orca 定义，但发送业务前必须证明受管扩展已装载；若需原生启动参数，在 Runner 现有 launch 编译中注入并冻结。不修改全局配置，也不默认加载一个新会话冒充原资源。

agent_launch 仍是 executable/args/model/reasoning_effort 的唯一位置。映射由 Runner 对应适配负责；Pi/DSH 不套用 Codex 参数。显式设置无法准确兑现时拒绝；Pi 使用原生 provider/model 和 thinking 参数；DSH 固定 deepseek-official provider，省略时部署 profile 为 deepseek-flash/low，显式值进入原生配置并核对真实模型。 DSH TUI 的 effort 不在 Agent.options 中：正式插件先以原生 resolveCallConfig 严格校验所选能力，再在原 root 的 agent/request waterfall 落实冻结值，并核对持久 request/header；不读取私有 TUI 状态、不接受静默降级。非空 args 拒绝。不同 agent 不共用模型或参数默认值，不建设模型目录或发现服务。

旧记录不回写 agent、不重算原字节/摘要、不提高旧证据要求。当前 profile codec 为 3、state codec 为 5（见 [profile.go](../../internal/execution/adapters/orcatransport/profile.go)）；不降版，不把旧文档的 state v3 当作实现基线。仅在新增来源确有格式/语义冲突时在原包演进 codec，先写出冲突与读旧记录规则，不预建迁移框架。

## 5. 执行事实与可靠性

### 5.1 准备、身份与输入

绑定 Orca runtime/执行主机、worktree、terminal handle/incarnation/pane，及扩展提供的原 agent、native session、原请求/输入摘要与原生回合。Pi 的 session/file 关联由原实例正式扩展和宿主核验，DSH 必须区分 root/child；私有路径不进公开投影。

受管扩展装载是已登记的准备操作，初始化不授权业务工作。工作区仍跳过仓库默认 setup；文档路径/实际 base 和 owned 清理保持。开发文档由代理直接按仓库相对路径读取，Runner 不准备或校验正文；12000 UTF-16 单元限制仅用于实际发送的单份提示词，遵循[总 Spec 的文档契约](steerrunner.spec.md)。

P0 可先在独立临时工作区做无工具、无发布且无需仓库认证的固定文本探针，不向探针注入仓库凭据；须通过所选 agent 的正式机制确认工具已禁用、实际启动模式明确，做不到就不发送探针。该探针只验证输入/事件/回执与停止通道，工具禁用不算 OS sandbox 或生产权限/认证通过。任何 development、Git/PR 等业务输入仍须先通过完整准备门槛。

输入意图先落盘，再经现有 Orca terminal 投递。Orca requestId/processIncarnation/接受回执与扩展取得的原输入、完整摘要、native session/turn 必须可关联。只靠相同 prompt、200 字符预览、时间接近或工作状态不够。UserPromptSubmit、pre-step 或 inbox claimed 等事件的确切提交语义由 P0 按版本验证，不能一概当作 started。

新增 Pi/DSH 路线优先沿用已有 wirePrompt 的命令关联样式：Runner 将唯一 command ID 与输入摘要随原 TUI 输入一同投递，扩展在原生输入接纳处记录它们；扩展不能凭观察事件自动获得 Orca requestId。业务输入与实际投递文本的编码/规范化在发送前分别固定并校验，不在接收后任意 trim 或只算预览摘要。该标记只携带关联信息，不是完成证明；具体原生提交边界和失配处理必须由 P0 实测，旧 Codex 输入不改写。

投递去重/回放仍归 Orca 已有请求机制和 Runner operation。扩展只报告、核验原生事实，不另建投递重试、durable inbox、业务队列或 exactly-once 协议。重复 command ID、人工插入、自动续跑或切会话造成歧义时，保持未核实并撤销后继自动推进，不靠扩展再次驱动 agent。

### 5.2 结束、完整正文与回执载体

开始、原主回合结束、成功原因、完整正文和业务成功分开。Pi 须结合最终消息/原因与 agent_settled，不能在 agent_end 或 agent_before_settle 就提前成功；DSH 须结合真实主会话的结束记录、原因、正文、持久检查点与原实例静止状态，不能用 Stop 或 SessionStart 代替完成。缺原因、缺完整性或无法关联原输入时保持未核实。

扩展事实必须来自正式原生事件/会话记录；模型自然语言、自写完成标记、工具输出、屏幕/OCR、Orca 8000 单元预览不作为原生完成证据。Runner 不扫描私有 transcript 猜格式。正式会话记录的读取由对应扩展在原实例内适配，按选定版本验证，不在 Runner 核心引入 per-agent 日志解析器。

本期 target=local，优先由 Runner 直接只读获取本 Run 的原生事实。P0 只选定一种有界、可原子落盘和重读的实际入口：正式持久记录若不能在无私有格式 parser 下直接复用，则由扩展导出最小 Run 私有回执；不并存文件、socket、HTTP、helper 多种回退。不增加常驻接收器、新诊断终端或通用 IPC/结果服务，不把 Codex 专属 helper 改造作为此前置。原 implementation envelope 是 Runner 的保存位置，本身不是扩展到 Runner 的跨进程通道。

新受管 helper metadata 为 v3，冻结 `receipt_version=2`；回执记录原生实际 reasoning effort，显式值必须一致，并在每次工具授权及输入/结算边界核验绑定。旧冻结 helper 保持其原 source SHA 和 v1 回执规则，不用新字段追认旧实例。

回执只包含核验所需的原生身份、输入关联、事实及限额内最终正文，不复制完整 transcript、工具/推理流或全部历史，不承接业务状态。扩展写原生事实，application 按既有事务写业务状态；不增加双写提交、消费确认/清理队列或 exactly-once 协议。未决回执和已冻结结果随原 Run 保留，不能因一次读取就删除。

内存事件、回调触发或空的 flush 成功都不证明持久化。必须核实选定 profile 的实际 persistence/backend、写入检查点和重启只读重读；尤其 DSH session 未挂载持久后端时不得以 flush 返回成功放行。

正文维持 32 KiB UTF-8、implementation envelope 维持 64 KiB；workflow 的 128 KiB 解码上限不是容量承诺。不增加任意大小结果文件、分块、大对象引用或额外提交/垃圾回收协议；超限明确阻断。结束先到正文后到时只读原回执，冻结结果后变更为冲突。载体随原 Run 的既有 owned 资源管理，未决证据不因读取或 stop 被删除；最终清理须遵守写者退出和原 Run 保留规则，不另设回执清理状态机。

### 5.3 对账、停止、恢复与交互

复用原准备期限、transport.wait_timeout、调用预算和重连退避，不新增四组 deadline。缺失/迟到/乱序/重复事实、扩展异常或宿主暂时不可达不能触发无限同步等待、业务重发或替代创建。

丢响应只对账原 Orca 请求与原扩展记录；记录不存在/尚未刷新/过期均不证明未执行。服务重启零自动输入，显式 resume 前核验原 terminal、agent session、扩展身份、原输入、未知操作和旧写者；不自动重建退出或未知的 TUI。

stop 先撤销后继授权；受管路径撤销原 Job、证明写者退出，并等待原 helper 完成认证关闭和退出记录后才关闭原 PTY，避免切断收尾。旧 Codex 仍沿原停止证明。PTY killed、agent idle/settled 或取消回执均不能单独证明所有 owned 写者退出。现有入口不足时，该组合保持停止未核实，不修改 Orca、不启动替代写者或提前删除回执/认证资源。

启动尚未产生 agent owner 时，不能以文件缺失证明退出。helper 在同步准备失败后可导出同一私有目录中的正向 startup-exit 事实：冻结构建/Run/配置、原 helper 进程身份、未启动 agent、认证关闭及退出时刻；只有该原进程确已退出，且原状态从未确认 readiness/输入，才允许沿原 close 收尾。准备仍在进行或事实缺失/冲突时保持未核实，不另建实例。

新 helper 的 startup-exit 同时保留有长度上限、已遮蔽认证环境秘密的具体启动错误。Runner 核验上述事实后立即拒绝原启动操作，将 Run 置为 blocked 并显示该原因，不继续等待 preparation deadline，也不重发启动或业务输入。旧记录缺少具体错误时显示原 helper 在 worker 启动前退出；记录已写出但原 helper 尚存活时仍等待原进程退出。

三对象必须通过业务 needs_input 后的 reply 和原会话反馈。原生工具问题是独立可选能力，支持时精确绑定 session/turn/question；不支持时明确阻断，不能猜 TUI 输入位置。旧/重复答案、stop 后输入不得发送。

## 6. 权限、认证与装载边界

维持 sandbox 默认、显式 full_access、始终 never。正式扩展、tool hook 拦截或 Orca 代答审批都不证明原生 OS 沙箱。每对象先验证一个明确权限/认证组合；只有 full_access 通过时只声明该范围，不能自动升级 sandbox 或借旧 Codex 报告通过。

Runner 冻结策略并管理已有单仓库、限时、可撤销认证资源；执行主机落实各 agent 实际工具绑定、写根、模式及 owned 子进程检查。Codex helper 保持专属，Pi/DSH 复用以真实绑定为依据；不强制重构整套 broker/helper，也不继承宿主广域登录验收。

新 Pi/DSH Run 的认证 transport 以服务 state-dir 为根，生成 `a/<身份摘要>/s`，避免在 socket 路径重复嵌入完整 Run ID；摘要仍绑定 Run、attempt、generation、配置和工作区。仅将该摘要目录授予原工具，不扩大写权限到 state-dir。已有冻结配置保留原路径。

扩展按 Run/原 session 隔离，不能吞掉用户原有 hooks、修改全局 profile、默认联网安装依赖或影响其它 Run。首期只验一个明确的受管扩展集合及装载顺序，不承诺任意第三方扩展兼容；无法在该实例按正式配置隔离，或运行中出现未声明扩展/装载变化时，明确拒绝或保持未核实，不为此建设扩展冲突管理系统。所选集合内部合法的重试/压缩/续跑仍须等到真正 settled；用定向反例检查未知改写、重载和异常，不展开生态组合矩阵。不能仅凭扩展文件存在或配置 enabled 就发送业务。凭据、launch token、私有路径和非本 Run 原文不进入公开投影。

## 7. 验收条件

| ID | 必须证明的行为 | 最低验证 |
| --- | --- | --- |
| OG01 | agent 校验/冻结/默认 Codex，非法或缺扩展/映射/权限零业务输入，旧摘要兼容 | L1 + controlled |
| OG02 | 原资源、扩展及会话绑定；创建/装载丢响应和服务换代不替代创建 | controlled + 恢复 L3 |
| OG03 | 唯一 command ID/实际投递文本与原生接纳/开始/结束关联，快回合/同 prompt/手工输入/续跑/子事件不串轮次 | 三对象定向夹具 + controlled |
| OG04 | 真正 settled、原因、完整正文、晚正文/冲突、预览及 32/64 KiB 边界正确 | L1 + controlled + 原生接线 |
| OG05 | 原请求及原回执对账，断连、丢响应、刷新缺口/重复/乱序/过期不重发 | controlled + 代表性原生 L3 |
| OG06 | 业务 reply/反馈必过，原生问题按声明处理；旧答案/stop 后输入不发 | controlled + 业务交互 L3 |
| OG07 | 现有 close + 宿主证明 owned 写者退出；显式 resume 只核验原 TUI/回执 | controlled + 代表性原生 L3 |
| OG08 | 按 agent 核验权限、认证及参数映射，不支持组合拒绝、无秘密泄露 | controlled + 定向原生/L3 |
| OG09 | Codex/Pi/DSH 逐对象完成 TUI 开发、业务 reply、原会话反馈、真实 PR 核验与收尾 | 每对象一条 L3 核心 case |
| OG10 | 旧 Codex terminal/app-server/已验远端回归，同服务不同 agent 不串身份/结果 | 必要已有回归 + 一条混合 agent L3 |
| OG11 | 原工作区/base/材料、准备副作用、限额及 owned 收尾保持 | controlled，真实准备随 OG09 |
| OG12 | Orca 未改动，仅核验明确的受管扩展集合；按 Run/原实例隔离，缺失/变化/异常不旁路，原全局配置与其它 Run 不受影响 | 版本/配置差异记录 + controlled + L3 |

按 [测试指南](../testing.md)分层。先用 Pi 验证新边界可行，再做 DSH；Codex 保持原证据来源。共享故障语义在 controlled 覆盖；原生 L3 每对象须有原请求丢响应、活动停止、Runner 换代后显式恢复三项代表性证据和真实起止事实，不要求把既有远端 12 个可靠性 case 乘以三个 agent。优先复用已有断言/故障设施；既有 driver 的远端/Codex 前提不适用时，只补必要本机场景，不为复用而泛化整套调度。混合 agent 只保留一条。四条核心 L3（含 Pi full_access）及一条混合 L3 已接线并真机通过，结果按实际构建和现场登记。

通过、失败、前置阻断、可选能力不适用及清理残留分别登记。必需项不能以不适用结算；拒绝不算正向支持。全部本机适用必需项通过才宣布首批交付。测试证据用于验收，不作为生产资格数据库。旧方案的 279 项 L1/16 场景 Controlled 不证明本次扩展路线已通过。
