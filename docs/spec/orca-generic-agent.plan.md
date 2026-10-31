# Plan：通用 Orca Agent 接入

> 2026-10-31 适用范围更新：认证与存储版本以 [host-native-auth Spec](host-native-auth.spec.md) / [Plan](host-native-auth.plan.md) 为准。新建开发执行使用宿主 Git/gh/SSH 登录；broker、CA、socket、认证租期/撤销证明已移除。仅支持全新 state/v6，不读取或迁移旧 Run；旧版执行由原版本结算。本文的 read_only、原生远端、身份绑定和未知操作对账条款继续适用，旧认证与旧格式兼容条款仅供历史溯源。

2026-10-31 当前开发权限修订：开发 sandbox 已退役，开发必须显式 full_access；read_only 与原认证链路保留。开发权限、迁移、旧 Run 与重复测试矩阵以 [统一权限 Spec](development-full-access.spec.md) / [Plan](development-full-access.plan.md) 为准。下文历史构建与验收事实不改写。

日期：2026-10-18。绑定 [Spec](orca-generic-agent.spec.md)。审计修补及必要验收完成；最新构建绑定以第 9 节和[验收记录](../acceptance/orca-generic-agent.md)为准。实施前的重复审计及失败描述已收敛到验收记录，不再把历史 blocked 状态作为当前状态。

## 1. 基线与边界

- Runner 已 rebase 到 `7c48969`，保留上游本机 runtime/repository 冻结及无 pair IPC 查询，工作分支 `codex/orca-terminal-agents`；验收时的工作树构建按 SHA 绑定。
- Orca 1.4.224 不修改、不升级、不补接口；app.asar SHA256 `882fde748964d91737792c1e186673d7cbe5f7112aebd605f56b5228aa0f501d`。公开 Orca 源码只读参考，不是待开发前置。
- Pi 0.87.1；DSH 0.2.1-rc.2 + 社区 dsh-tui 0.16.1（[作者启动说明](https://github.com/ccch1mneyyy/dsh-TUI/blob/main/docs/getting-started.en.md)）；Node 24.13.0。DSH 使用独立安装及 Run 私有 profile，不改原有全局 web profile。
- Windows 权限工具复用 native Codex 0.159.2 的 sandbox 子命令和固定 PowerShell 5.1；它不运行 Codex 模型/会话。Pi、DSH 原 TUI 均保留，禁用 structured、SDK 会话驱动、RPC、ACP、headless 替代。
- 只承诺已适配版本的本机 development/1 可由配置选择；未知 agent、能力不足、来源不匹配仍拒绝/未核实，不扩成所有 Orca agent 自动通用。

## 2. 责任与改动位置

| 编号/位置 | 已实施职责 | 不扩大 |
| --- | --- | --- |
| R01 contracts/resources/application | 可选 settings.agent，省略保持 Codex；冻结 Ref/Session/inspect、模型/effort、权限和 helper/source SHA | 不回写旧记录、无 DB 迁移；profile 3/state 5 保持 |
| R02/R03 orcatransport | 现有 Orca 接受回执 + 原生事实进入原生命周期、operation、observer、结果和恢复；单一有界只读通道 | 不造 Orca turn_started、Codex pane/时钟/RemoteTurnReceipt，不加服务或私有 transcript parser |
| A01 Pi 正式扩展 | 原 input 关联、原 user entry 才开始、agent_settled 后原最终消息/原因 | 不用 agent_end 提前完成，不另建 agent/session |
| A02 DSH 正式 Cordis 插件 | 原 root user/message、assistant、turn/end 原因、whenIdle、真实 flush 和精确持久事件核验 | 排除子事件、会话替换和桥接提前 Stop；不修改 agent 核心 |
| H01 Runner/执行主机 | 原 TUI 装载、实际工具权限与短期单仓库认证、原 Job 重关联/停止、原 helper 根进程退出 | 无新常驻服务、进程注册平台或 broker 重构 |
| Orca | 原工作区、terminal、投递去重和本机原生查询 | 无待补接口、名单/状态字段、安装包补丁 |

Orca 的显示 agent 标签仅作提示：Pi 可为空，DSH 可被误标 Gemini。受管路径仍严格核验原 runtime/workspace/terminal/incarnation、原 owner/worker PID 与创建身份、冻结 helper/扩展来源及原生 session；Codex 原身份规则保留。

## 3. 实施阶段

### P0：原 TUI 最小验证（已完成，历史探针）

正式扩展禁用工具，只投递固定短答，验证唯一 command ID/摘要、同 prompt 不同请求、原生开始与 settled、原子持久重读、重复/人工输入拒绝、原实例退出。只使用独立临时无 remote 仓库，不注入仓库凭据。P0 不代替权限或 L3，摘要绑定原构建，不把后来源码视为原探针已验版本。入口与历史证据见第 7 节。

### P1：Pi 生产接线（已实施）

1. 在既有 backend/profile/state 复用配置和生命周期；受管 helper metadata 在接受时冻结实际 EXE、扩展集合及事实来源 SHA，重启不拿新源码核验旧实例。
2. 唯一 command ID 和逻辑输入摘要进入原 TUI；接受、开始、完成分别有证据。正文 32 KiB、单回执 64 KiB，完整性不足/原因未知/非文本最终内容/写入失败均不成功，不截断。
3. 原回执保持原子写入；现有业务 state 保存已提交结果。readiness 只增加最新输入 ID，供业务 reply/反馈与显式 resume 核验原结算边界，不引入第二套调度、消费确认或全历史复制。
4. 默认 sandbox 使用实际 AppContainer；显式 full_access 独立实测。原生 read/write/edit/powershell 均走同一冻结工具边界，无审批升级。provider 环境不进工具，仓库认证/有效期与 Git 权限用实际工具验证。
5. 绑定原 Job 的 owner/worker PID、创建身份及原 handle；服务停机只释放观察引用。Run stop 撤销原 Job、核验所有写者及原 helper 根退出，不能只凭 PTY close。

### P2：DSH 接线（已实施）

1. 使用原 DSH TUI、固定私有 profile/preset 与正式插件，只允许受管 powershell；原全局 ask-user 等工具在正式 agent scope 内限制，业务交互归 Runner needs_input/reply。
2. 原 root session 的提交、原生 turn、最终消息及持久检查点共同结算。空/失败 flush、错误 turn、子事件、原生失败均有确定性反例。
3. 原生 launchpad=false 直接进入 composer。Windows 原始发送使用 bracketed paste/CR；提交后的 LF 文本仍须逐字摘要一致，未使用单行折叠、模型标记或编码旁路代替。
4. 与 Pi 共用实际工具/认证/owned Job 生命周期，只保留原生事件映射差异。原生 model 与冻结请求核对，过程中切换模型或工具集合则失去授权。

### P3：真实 L3 与登记（以第 8 节结果为准）

1. `orca-local.codex/pi/pi-full-access/dsh` 复用 development-feedback：真实业务 needs_input/reply、原会话反馈、独立 PR/head/内容核验和实际合入，按 agent 审计权限及原生来源。
2. 核心现场先用独立短 Run 验证 Pi/DSH 启动失败正向退出及收尾，再真实扣留一次已返回的 Orca business-send 接受响应，核验原 operation 保持 unknown、无重发和原写者停止。正常 Run 的 request_id 查询/幂等重放另行命名，不计为故障注入。正常服务换代后显式 resume、真实父子写者活动 stop 与延迟写入消失继续验证；所有 agent 的零输入断言来自 test-only 原 Orca CLI 转发日志，必须先有非零原输入。
3. `orca-local.mixed` 复用同服务 multirun，固定 Pi/DSH 一个组合：各自输入/事件/原生回执/session/PR 隔离，停止 Pi 后 DSH 能继续原会话反馈；两组 Run 与服务均须停止。
4. L1 包含 Go 及两套扩展契约测试，Controlled 覆盖共享故障；不把原远端 12 case 乘以三个 agent。当前本机 Codex 作为旧路径真机回归，原 app-server/远端保留必要快速/受控检查，不从本机结果推导 Pi/DSH 远端支持。
5. 记录实际版本、构建/配置、通过/失败、清理残留及 Orca 未修改事实。零匹配、注册/编译、探针、前置阻断均不能冒充 L3 通过。

退出条件：Spec OG01–OG12 的适用项、四条核心 case 和一个混合 case 的真实证据齐全；未通过对象不宣称完成首批交付。

## 4. 测试映射

| 项目 | 对应要求 | 实现与验收 |
| --- | --- | --- |
| T01 配置/兼容 | OG01/02/10 | contracts/application/资源 schema、旧字节/摘要及无资格零输入；默认 Codex、不新增 schema 迁移 |
| T02 原生事实 | OG03/04/05 | Pi/共享工具宿主 20、DSH 8 项 Node 契约反例，加 Go 来源/状态/完整性校验；原生 L3 核验实际 entry/turn/session |
| T03 开发/交互 | OG06/08/09/11/12 | 四条 development-feedback L3，含业务 answer、原会话反馈、独立 head/新时间戳、真实 merge/内容及清理 |
| T04 可靠性 | OG02/05/07 | shared controlled + 各对象真实 Orca business-send 响应丢失/换代显式恢复/活动 stop；实际启动并核验写者，不以安静 PTY 通过 |
| T05 多 Run/回归 | OG10/12 | 同服务 Pi/DSH 一个混合 case，停止一方后另一方继续；Codex 原路径真机与现有快速/受控回归 |
| T06 权限/装载 | OG08/11/12 | 实际工具认证、AppContainer/full_access、provider 环境过滤、工作区外写入差异、工具/模型/扩展变化拒绝与 owned 收尾 |

同一服务多个 agent 只在混合 case 中；其它 case 各自拥有 fixture/state/service/Run/workspace/base/head/PR。测试结果是验收证据，不是生产必须加载的资格数据库。

## 5. 结构债约束

保留一个 orca-terminal adapter、一套业务生命周期和事务。无新常驻接收器、跨进程总线、第二结果服务、插件注册平台、模型目录、资格表、消费确认协议、全 transcript 复制或自动重发。配置 args 不套用 Codex，Pi/DSH 非空 args 拒绝。正式扩展不足时保留不支持，不补 Orca。

工具宿主只执行固定权限内的一次原生命令，拥有有界输入/输出/寿命及整个 Job 收尾；不管理模型轮次。Windows PowerShell 原生 stderr 是输出数据，不能因默认 Stop 提前中断 Git；受信任文件/权限准备脚本仍严格失败，原生 stderr、UTF-8 和失败退出码有实际回归。

## 6. 失败与收尾

各轮失败、修复及独立清理详见[验收记录](../acceptance/orca-generic-agent.md#失败与修复记录)。旧“需 Orca 补接口”、统一非 Codex blocked、只完成 P0 等叙述只属于实施前历史，不沿用为当前方案。失败现场保留，不通过重发、替代资源或删除现场来变成通过。测试分支/工作区用于取证，原写者和服务的停止分别记录。

## 7. 约束收敛后的实施：Pi P0

历史入口：`scripts/probe-orca-pi.ps1`、`extensions/pi-evidence.test.mjs`；`.evidence/orca-pi-p0-10/summary.json` 为该时点无工具原 TUI 的真实结果（Pi 0.87.1 / Orca 1.4.224）。原子回执、相同 Orca UUID 对账、原生开始/最终、原实例重关联及关闭退出当时通过；不代表生产工具或服务换代已验。

P0 固定短答用正式 hook 限制 256 token；生产 Pi 为 8192 token，DSH 使用原生 provider 输出配置，正文/回执限额另行生效。历史 L1 279 + Controlled 16、Node 16 的记录不代替当前构建验收。当前正式扩展及共享工具宿主的 28 项契约测试均已接入统一 L1，无第三方测试依赖。

## 8. 本机执行链路实施与真实验证

本节为审计前历史记录，最新统一验收见第 9 节。Pi sandbox、Pi full_access、DSH sandbox 及混合 Pi/DSH 已在真实本机路径通过，现场/构建与各断言见[验收记录](../acceptance/orca-generic-agent.md)。Codex 回归、294 项 Go + 22 项扩展 L1、16 场景 Controlled 已通过；原生退出码兼容修补另经双权限实际工具回归和 Pi/DSH 全链路补跑。通过、失败、构建和各现场收尾均分别登记。

## 9. 审计修补复验（已完成）

- 启动失败通过有界 `startup-exit` 正向事实收尾；缺 owner 本身不放行。正常停止等待原 helper 写出认证关闭事实并退出后再关闭 PTY，旧冻结 helper 沿旧来源读取。
- Pi 工具入口实时核验模型、thinking、工具、cwd、session；正式选择事件立即撤权。Pi/DSH 新回执记录实际 effort，显式配置被原生调整时阻断。
- PowerShell 最终成功不再继承历史 LASTEXITCODE；PS5 native stderr 的兼容例外只接受最终命令的原脚本、行列和实际退出码，失败 cmdlet 不借旧 native 状态变成功。工具超时/取消保留未知结果，扩展撤销后续工具授权。
- 增加接收端缺失/超限/来源、会话、命令、摘要、模型、effort 变更、晚到事实及冻结冲突反例；实际丢 Orca 响应保持 unknown。
- 将共享原子写入器移出 Pi 模块，删除假想通用 mainAgent 路径的测试声称；没有新增 adapter、平台或业务状态机。
- DSH 使用正式 `llm.resolveCallConfig` 和原 root `agent/request` 核验实际 effort，持久 request header 再核对；不从不存在的 TUI options 字段推断实际设置，不另建模型会话。
- 最新统一 L1 通过 300 项 Go + 28 项 Node；Controlled 16 场景通过。五条本机 L3 在相同 Runner/helper/driver 构建下全部通过，零排除；启动失败、真实丢响应及晚到完成不晋升、恢复零输入、原会话反馈、真实合入、活动写者停止及混合隔离证据齐全。Windows 原生反例单列平台测试文件，Linux 交叉编译通过。
- 实际双权限工具探针通过；最终 helper 的真实 60 秒工具超时验证未知结果撤权、父子写者退出和延迟写入消失。现场、SHA 及历史失败保留见验收记录。
