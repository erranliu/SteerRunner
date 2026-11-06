# Orca Codex / Pi 结构化会话：P0 核验

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

后续首 PR 范围已收敛（2026-11-03）：本机 Codex 新 execute/1 结构化输入/完整结果/停止请求，资源证据不足明确未核实；不修改 Orca，同时删除 DSH 执行和专用历史/用量支持。见更新的 [Spec](../spec/orca-structured-agent-session.spec.md)、[Plan](../spec/orca-structured-agent-session.plan.md)、[设计复审](orca-structured-agent-session-mvp-design-audit.md)。下文为原 P0 调查与当时门禁，不覆盖用户后来确认的首 PR 取舍，原 evidence 字节不改写。

日期：2026-11-03。结论：**P0 调查已执行，放行条件未通过；P1 生产 mutation 实现阻断，P1–P6 未开始。** 对应 [Spec](../spec/orca-structured-agent-session.spec.md) / [Plan](../spec/orca-structured-agent-session.plan.md)。本轮没有修改 Runner 运行代码、Orca 安装或用户配置，没有切换当前后端、删除 helper 或缩减既有 Workflow。

后续同日[前三项阻断再调查](orca-structured-agent-session-blocker-review.md)修订初轮归因：B01 的严格 fence 假设由 Runner 既有授权/派发机制设计解阻；B03 本机工作区可复用现有 Git/原 checkpoint；B02 原会话资源的独立正面结算事实与 B03 逐回合实际模型公开来源仍缺。终止优先复用原 session close 一次，不默认新增 acquisition CAS。用户随后明确**不能要求修改 Orca**，上游补充方案已从现行 Spec/Plan 撤销；不能满足的证明继续记为缺口，不依赖 Orca 新字段/版本。下文保留初轮现场与当时判断，当前边界以更新后的 Spec/Plan 为准。未新增模型/RPC 调用，远端 Pi 不安装。

本报告记录公开 Orca RPC 的有限探针，不是正式 Runner 验收。使用 [orca-cli 技能](C:/Users/lmas1/.agents/skills/orca-cli/SKILL.md) 的公开 CLI 管理隔离工作区；Agent 输入通过公开 `agentSession` RPC。只读安装包用于匹配合同，没有 renderer 调用、私有数据库/会话文件读取或直接 provider 连接。[原始证据目录](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103)及 [SHA-256 清单](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/manifest.json)保留失败和意外接受结果。

## 1. 固定现场与证据边界

Runner 源码基线 `457ffaea20b6227a0bdaec9e56f5b9e7f54da337`；工作树已有本专项 Spec/Plan 与导引修改。本轮未构建或运行结构化 Runner EXE，不授予任何新 Run 准入资格。

| 项目 | 本机 | 明确配对 environment：台式机64G |
| --- | --- | --- |
| Orca runtime | `acbe88cf-6a61-4eeb-8178-6d4886af40c8` | `21d98aa2-d284-41f3-a6cc-42fb22d728a2` |
| environment ID | 不适用 | `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8` |
| runtime 报告 app 版本 | 1.4.246 | 1.4.246 |
| shell 可解析 Codex | 0.166.0 | 0.162.1 |
| shell 可解析 Pi | 0.87.1 | 未安装，`PI_MISSING` |
| Agent RPC 通道 | 本机命名管道 | 原配对加密 WebSocket |
| createSupport：Codex | supported，seedOptions `permissionMode=full-access` | 同左 |
| createSupport：Pi | supported，无 seedOptions | **unsupported，reason=agent** |

依据：[本机状态](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-status.json)、[远端状态](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-status.json)、[本机版本](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-agent-versions.json)、[远端 shell 版本](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-version-shell-after.json)、[远端 Pi 准入](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-pi-createSupport.json)。registry 两端均列出 Pi，但这不证明可创建。shell 解析版本也不独立证明 Orca 实际 provider 子进程的二进制路径、版本或 SHA；该边界仍待核验。

本机 CLI SHA-256 为 `6de1a5e85651185428de1ae20bdc6e352ff2c8bae90051ac77c48e83cd130125`。两端 `app.asar` SHA-256 均为 `39915f5d1acd910657d7597004fa595704e9672b37a4a0cddc836a812dbda01a`；本机合同/指纹/host 模块另有逐文件摘要，见 [source-identity](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/source-identity.json)。远端运行 EXE 和 app.asar 摘要来自其原测试终端，[提取结果](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/derived-results.json)保留字段。未独立哈希远端 unpacked JS，不能仅凭 app.asar 相同声称两端展开模块逐字节一致。

## 2. 实际接口与映射

以下标为“实测”的方法均经公开 runtime 调用；“源码”只说明该安装合同存在，不声称实际问答、故障恢复或退出资格通过。

| 能力 | 已核验字段/结果 | 覆盖与限制 |
| --- | --- | --- |
| `agents` / `createSupport` / `modelCatalog` | worktree 用 `id:<完整 ID>`，agent 独立选择；初始 savedOnly catalog 的 origin 为 unknown | 双端实测；不使用 registry 代替 host 准入 |
| `create` | envelope；worktree、agent、options；可带 firstMessage `{clientMessageId,body,selection?}` | 三个可用组合均空创建、原子首消息实测；`expectedRuntimeFence=null` |
| `send` | envelope；body `{kind:"message",role:"user",blocks:[{type:"text",text}]}`；selection `{model,effort}` | 三组合实测原文、显式已观察模型/effort；无 queue/steer/bootstrap |
| `history` | sessionId、direction tail/before/after、cursor `{epoch,sequence}`、limit≤200；结果在 `result.page` | 有界读取实测；未做完整丢页/过期/截断故障矩阵 |
| `item` | 按原 session/item 读取详情的合同存在 | 本轮未独立实测 item 详情；后续不能以 history 代其全覆盖 |
| `subscribe` | sessionId、可选 cursor | 本机命名管道明确 `method_not_supported`，要求 streaming transport；远端 WS 实测 snapshot。未验证本机 WS 认证/订阅路径 |
| `currentRequests` | sessionId、version UUID、可选 cursor；marker、json chunk、nextCursor、restart | 三组合实测，任意初始版本可获得当前 marker；未生成真实原生问题 |
| `options` / `handoffStatus` | 选项/报告来源；owner native、phase idle 等 | 三组合实测；不是实际模型或进程退出的充分证明 |
| `respondToQuestion` | envelope、itemId、expectedRevision；optionId 与 answers 二选一；answers 含 questionId、optionIds、可选 other | 安装 schema 核验；未实测原问题→回答→继续，不可计 Workflow 问答通过 |
| `cancel` | envelope + 原 public turnId | 三组合仅在 startedAt、working、running 同时成立后实测 |
| `close` | **仅 sessionId**，不接受 envelope | 双端实测；详见停止阻断 |
| `release` | sessionId、holderId | 源码：view holder release，不是精确资源终止；本轮不调用 |

RPC 外层 `ok=true` 与领域层 `result.ok=true` 必须分别验证。例如错误 create 指纹的 RPC 成功返回内层 refusal，不能当创建成功；见 [本机错误指纹](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-negative-bad-create-fingerprint.json)。pending submission 表示登记/接受，不等于 provider Started。

最小公共观察候选为**有界 unary history/item/currentRequests**，远端 streaming 可作加速/提示；先验证 item 和缺口处理。不为统一订阅外观增加本机配对流程、第二状态机或 Orca 补丁，也不把现有 StateSubscription 当成可用 agentSession stream。

### 2.1 身份和指纹

sessionId 合同为 `[A-Za-z0-9_-]{8,128}`；本轮使用 `oas_p0_` 加去横线 UUID。原生 operation ID 是 `<13位毫秒时间>-<32位小写hex>`；不能直接复用任意外部 request_id。各意图文件保存实际 operation/message/session 关联。

operation fingerprint 为 SHA-256(UTF-8(canonical JSON `{method,sessionId,fields}`))，递归对象 key 按 JS 字符串次序排序，undefined 属性删除，数组保序，primitive 沿 JSON.stringify。create 的 fields 包含实际提供的 worktree、agent、resumeFrom、tabId、firstMessage、optionsMode、options。send 的 operation 摘要覆盖 body 及提供的 selection/options；**submission 中同名 payloadFingerprint 则是 body-only send 摘要**，不能混用。独立 Node crypto 算法逐 mutation 与安装 helper 比较，未出现不匹配；[固定向量](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/contract-vectors.json)同时覆盖 Unicode、CRLF、空白及 JSON 特殊字符。后续 Go 编码须匹配 JSON.stringify，不能直接采用默认 HTML 转义或不同 U+2028/U+2029 转义结果。

所查合同 wire 限额为 JSON.stringify(blocks) UTF-8 的 256 KiB、最多 64 blocks，未做真实超限模型派发。Runner 预算保持原值，不随此限额放大。

## 3. 创建策略与正文

三个可用组合均比较了 `create.firstMessage` 和“空 create→send”，六个短回合都得到唯一 user journal 行、同文公开 body、绑定原输入的正常结束及预期 assistant 标记；见 [派生核对](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/derived-results.json)。合成正文包括 CRLF、空行、中文、emoji、引号、反斜线、代码块和尾随空白。这里只证明 Orca 公开 journal 原文往返与接受 turn 关联，没有独立读取 provider 私有输入来证明其每个字节。

**选定候选：原子 firstMessage。** 它用一次创建 mutation 绑定首消息，减少空会话中间持久状态；不维护分步 fallback。外发前冻结全部正文、模型/effort、创建及消息 ID/摘要；原子首消息后禁止再 send 相同输入。显式 selection 已在分步 send 验证，但原子 firstMessage.selection 的显式模型及审计角色切换尚未实测，仍是候选必补项；远端 Pi 没有创建资格，不能把三组合证据平移过去。

空创建返回还未出现 provider handle 的情况和原子首消息最初 pending submission 均被保留。history 的 `latestTurn.turn.state` 是嵌套字段；正文 item、turn.userItemId、submission.providerItemId 等关联核对后才认领完成。

归属核对不能要求三个 item ID 相等：本轮 normalized user item 为 `orca:<clientMessageId>`，turn.userItemId 指向它；Codex submission.providerItemId 是另一 provider item ID，Pi 此次恰好与 normalized ID 相同。应分别保存这些公开身份和原 clientMessage 关联，不从 provider ID 字符串反解析一套事件。离线核验脚本首轮错误地要求 providerItemId 等于 user item，因此失败；改按上述实际公开关联复核，并保留该检查失败说明。

## 4. Fence 生命周期与意外接受

原子首消息观察到自然 fence 1→2，submission 随当前 lease 更新；split 本机曾维持 1，远端曾 1→2。**native fence 不是不可变进程 incarnation**，不能将任何变化一律判替换，也不能自动追随当前数字。原请求 expectedRuntimeFence 仍须保持原记录；新 lease 的认领需要可核验的因果/资源关联，不能改写原请求。

关键负例：在本轮自有 Codex 会话 close 后，history fence 已为 3，使用**新负例 operation**、合法指纹、旧 `expectedRuntimeFence=1` 的 send，在本机和远端均被接受。返回 pending submission fence=3，随后该会话重新取得 provider lease 至 fence=4，新增 turn 实际运行并完成。证据：[本机返回](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-negative-stale-send-fence.json)、[远端返回](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-negative-stale-send-fence.json)、[本机新 turn](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-negative-cleanup-before.json)、[远端新 turn](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-negative-cleanup-before.json)。这两次是失败的“应拒绝”假设，不能计为围栏通过，也不是未知原操作重试。

该行为可能属于 Orca 会话可重新获取的设计，本报告不认定为 Orca 缺陷。但它与 Runner 拟依赖的“旧资源 fence 严格拒绝、close 后不能重新输入”假设不等价，形成 **P0-B01**。不能用客户端先查后写声称原子围栏成立，不能增加自动 rebase/retry 来规避。发现后未再扩大此负例；精确关闭两次新 acquisition 后 fence=5、working=false，保留清理证据。

## 5. 停止语义

三组合在原 turn 活动时 cancel 得 `cancelled=true`；同一 turn 随后 `state=interrupted,outcome=cancellation`。这证明回合取消，不证明会话资源全部结算。[Codex 本机](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-codex-active-after-cancel.json)、[Pi 本机](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-pi-active-after-cancel.json)、[Codex 远端](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-codex-active-after-cancel.json)各自保留。

close 返回 ok、history fence 增加、working=false、chat tab 消失。安装路由会隐藏 presentation 并调用 host.close，包含 adapter 释放/停止尝试，不能将其简单描述为“只有 UI 关闭”。但它**没有 expected fence、operation ID/摘要和原运行实例原子核验**；加 envelope 在双端均返回 invalid_argument（Unrecognized key）。返回也没有独立可查询的原 provider/root 退出证据或完整资源结算 receipt。handoff idle、tab 缺席、subscription 中无 session、worktree 已移除均不补足该证明。

**P0-B02：尚无可签收的精确终止策略及未知结果只读对账。** 没有在公开方法目录找到可用的通用 operation receipt 只读接口；history/submissions 有消息归属事实，不等价所有 create/cancel/close 的独立回执查询。该“未找到”不是要求新建通用 receipt 系统：同日再调查明确只需原资源结算的充分查询，Runner 可复用已有 stop reconciliation。cancel 实测证据只证明回合中断，其内部另有停止 provider 的尝试。release 不是终止替代。`orca_agent_session` scope 保留为目标，当前未获资格；不强制 cancel→release/close 两阶段，不新增账本、provider 旁路或 OS kill 来伪造等价。

后续须先找到原合同已提供且足够的终止/查证路径，或明确所需 Orca 上游合同能力后再评审；不能通过降低 scope、撤下现有组合、重试未知 close 或修改 Orca 安装解阻。

## 6. Workflow 与配置等价性

| 必须保持项 | P0 发现 | 结论 |
| --- | --- | --- |
| Codex 本机 development/2、execute/1 | public 原文/短 turn/cancel；permissionMode full-access 有实际 options 报告 | 未执行工具、正式反馈/问答、完整开发/execute、PR、停止/离线；未取得等价资格 |
| Codex 本机 development-audited/2 | 现准入依赖 helper 控制/工作区审计来源；新 options 显示 gpt-6.1-sol/high、来源 saved/policy，turn contextUsage.model=unknown | **P0-B03**：实际逐回合执行/审计模型及原 workspace proof 的等价来源未建立，阻断 helper 退役；保留 Workflow |
| Codex environment development/2、execute/1 | 远端相同短探针成立 | 仍缺正式业务、工具/权限/反馈/问答与停止证据；不扩展远端 audited |
| Pi 本机 development/2、execute/1 | public 原文/短 turn/cancel；options 报告 openrouter/deepseek/deepseek-v4.1-flash、high | 未证明原 tool/full_access/thinking 策略、可执行参数及正式业务等价；不扩展 Pi audited |
| 新目标 Pi environment development/2 | createSupport=false，目标未安装 Pi | **P0-B04** 环境阻断，create 次数 0；不借 Codex/本机 Pi 证据签收 |
| 非 Orca Codex 五种原 Workflow 与 parser | 生产调用者读取核对，仍由 codex-app-server 分支承接 | 本轮无代码改动，无新增运行回归资格；保持原行为和支持表 |

原 workspace audit 消费链：`orcatransport/workspace_audit.go` 的 InspectLifecycleAuditWorkspace 调 helper workspace-audit，验证原 terminal/control/RemoteReadiness；`codex/remote_workspace.go` 验证 helper/code SHA/PID、原会话/配置、仓库/branch/base/head/tree/dirty/scope/checkedAt、writersAbsent 等。普通 history/worktree/head 查询不足以证明相同来源与 writersAbsent，不能用模型自报或通用 Git 查询替换。

显式 send selection 已按观察值核验。**两个不同模型的执行/审计切换、原子 selection、真实 question/revision 回答、Pi 工具限制、agent executable/args 精确兑现、provider 子进程身份、服务重启/丢响应/离线停止均未核验完成。** 显式字段不能静默忽略，旧 helper 设置的拒绝待 P2 实现；P0 不改准入，也不提前删去保留项。正式四组合八场及 audited/execute 案例仍须由 P5 正式 Runner 执行。

## 7. 复用、外部解耦与兼容性决定

| 边界 | P0 决定/消费核对 |
| --- | --- |
| 公开调用 | 仍用 `orca-terminal` adapter、原 agent/target/workflow/model 配置；不暴露 session/fence/RPC，不拆公开 Codex/Pi 后端 |
| 内部共享 | 协议 `orca_agent_session_v1`；同一连接能力、共享生命周期及 normalized Orca facts；不重做 Codex App Server/Pi RPC decoder |
| 持久化候选 | profile key `orca.agent-session-profile` v1；state key `orca.agent-session-state` v1。沿现有 versioned opaque ImplementationDataEnvelope、SessionContext.SessionID，原生 fence/message/provider 引用放新 payload；不借 terminal/incarnation 字段承载会话 |
| 格式门槛 | `implementation_data.go` 既有 opaque payload 64KiB；runstate/storage JSON 容器与 state/v8/user_version8 不需因新 key 自动升级。尚未发现必须改全局表的字段；新 key 的严格解码、admission/clone/停止消费者仍待实现验证，不能声称旧二进制兼容新记录 |
| 状态目录 | 当前不提出全局版本/目录变更。原构建先排空，后续只有实际不兼容才最小修订；本轮 fresh 测试工作区不是产品强制换状态目录 |
| 公共读取 | 沿既有 messages/inspect/progress/result/usage；`application/native_session.go` 当前 viewer 仅支持非 Orca Codex，本次不新增 Orca native viewer 承诺 |
| 删除边界 | Orca 专用 helper/模型桥/control/history、managed Pi/DSH 活动生产者为后续退役对象；共享 adapters/codex、parser、历史读取/用量及真实 terminal 消费者按实际调用保留 |

以上 key/版本是文档决定，尚无运行类型/编码实现。精确 stop scope 与 native lease 映射被 B01/B02 阻断，不能把“已选名字”当成能力已实现；也不因此扩建通用 Runtime、全量 journal 镜像或格式迁移平台。

## 8. 阻断处置与计数

| ID | 阻断 | 下一步证据要求 |
| --- | --- | --- |
| P0-B01 | stale fence send 被接受且可重获 closed 会话 | 明确 native lease/rebinding 合同及原资源原子围栏路径；证明 Runner 所需身份边界可实现，禁止自动追随新 fence |
| P0-B02 | close 无原运行 fence/operation，独立退出与未知查证不足 | 原精确资源的终止、结算/清单及只读查证证据；本机/远端和 Agent 差异逐项覆盖 |
| P0-B03 | 原 audited 实际模型与 workspace audit 来源不等价 | 原逐回合模型/角色和精确工作区 provenance 的公开替代证据；保持原门槛和 Workflow |
| P0-B04 | 台式机64G 无可用 Pi | 由环境准备补齐 Pi/登录后重新查 createSupport，并另行验证；本轮不自动安装/登录或改配置 |

B01/B02/B03 是生产迁移阻断，B04 是第四目标现场阻断。其余未验项见 §6，不能因这四项列出而视作已通过。允许继续只读合同调查/文档补证；在解阻并复核前不开始相关 P1 生产 mutation、更不切换后端。无需为了获得“P0 通过”继续制造模型调用。

实际计数：[derived-results](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/derived-results.json)。有效创建 12（3 空、3 原子、3 分步、3 活动取消）；业务模型输入 11（6 短回合、3 活动回合、**2 意外接受负例**）。错误 create 指纹拒绝 2，close 加 envelope 拒绝 2；close 成功回执共 14（含负例重获后的两次清理）。远端 Pi 创建 0。模型调用没有 Git 修改/提交/PR，不代表正式 Workflow 成功。

保留的探针失败：最初三个 blank 准备调用将 UUID 字符串传给要求回调的 operation ID helper，在 intent 保存、连接和副作用之前异常；修正回调后才形成当前有效意图。普通沙箱本机 IPC 报 EPERM，使用已批准的本机 runtime 访问后成功，没有重启 Orca。首次本机 worktree Git 检查受沙箱拒绝；[原文件](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-worktree-git-after.json)标记 verified=false，[后续只读核验](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-worktree-git-verified.json)单列成功。这些前置失败不增加模型输入次数，也不覆盖两次围栏负例失败。

## 9. 隔离与清理

两测试工作区来自已注册 SteerlineTest，各用独立 `sr-oas-p0-20261103-local/remote` 分支，`--setup skip --no-parent`，HEAD 均为 `256b47afc14604f6f138c965156b3faf2ae634af`。创建结果记录完整 worktree ID 和 instance；所有 Agent 会话均以 `oas_p0_` 标识。本机 shell 无模型输入；远端 default shell 只用于版本/Git/包摘要，未向 Agent 注入 terminal 按键。

清理前核对原绝对路径、ID、instance、Git clean、各自创建的 session；负例新 acquisition 另行关闭。最终 tabs 只剩原 default shell，随后公开 CLI `worktree rm --delete-branch` 得两端 removed=true；再用原 ID show 均为 selector_not_found：[本机清理](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-worktree-cleanup.json)、[复查](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/local-worktree-after-cleanup.json)、[远端清理](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-worktree-cleanup.json)、[复查](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/remote-worktree-after-cleanup.json)。未执行 OS kill，没有独立核验 branch 删除或全部 provider 后代进程退出；不能将此次 fixture 清理写为新 stop scope 通过。

证据中的 `probe.cjs` 是本次逐步修正的诊断工具，非 Runner 接线，也非正式测试 driver。最终清单固定的是留存版本，不追认所有早先请求均由该最终文件哈希产生；每次保存的原意图/回执才是对应调用事实。证据只含合成模型正文和必要身份，运行认证材料只在内存使用并过滤保存。未来正式资格应另存最终构建证据，不覆盖本 P0 的失败结论。

回填复核：36 项离线证据/文档检查、312 个本地链接检查通过，结果见 [validation-results](../../../../acceptance/evidence/orca-structured-agent-session-p0-20261103/validation-results.json)；意外接受负例的检查只确认失败事实已被忠实记录，不计安全围栏通过。最终证据 189 文件（清单自身除外），逐文件 SHA-256 固定；未发现凭据值、常见 token 格式或邮箱。检查不授予生产资格。未运行 Go/正式 L1–L3 测试，因为本轮没有运行代码变更；正式候选测试仍待后续阶段。
