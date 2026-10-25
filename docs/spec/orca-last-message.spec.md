# Spec：Orca 最后消息与原生结束归属

日期：2026-10-23。状态：实现、结构自审和本轮隔离验证完成；真实来源限制见验收记录，未部署生产。源码调查基线：`b2a05f0684fa670e7ce84e8e9e97dd1c1a3bcb86`。配套 [Plan](orca-last-message.plan.md)。

## 1. 问题、目标与范围

2026-10-23 对以下现场作过只读调查，未停止服务、修改数据库或向 agent 发送输入：

| 项目 | 调查事实 |
| --- | --- |
| EXE / state-dir | `C:\SteerlineInstall\bin\steerrunner.exe` / `C:\slr\beret3eg7x3tdkzggckgqkuhmy` |
| Run / Attempt | `f86e6875ffee33f9f259e8121742f409` / `f371d08cfbcfab52977baccb2fc3b3ae` |
| Runner | `running / implement.execute / observing`，原输入 accepted、started；`last-message.message=null`，无可信 final |
| 远端 | 原 runtime、terminal handle、incarnation、worktree、pane 对应；`worktree.ps` 主 agent 为 done，公开 `lastAssistantMessage` 有正文，`prompt` 为空 |
| 已保存证明 | `WorkingAt`、`RootWorkingAt`、provider、authority 已存在，未 settled；`NativeProof.Diagnostic` 为 `Native original completion/input provenance is unverified` |

这些事实来自本次聊天中的 CLI 查询与 SQLite 只读检查，不是已归档的原始验收证据；实施前在隔离环境重新取证。空 prompt 会命中当前拒绝分支，但不能据后来一次快照证明历史诊断只由空 prompt 引起，更不能据此清除历史冲突。

交付目标有两个，分别验收：

1. 原生远端 Orca Codex 执行中或结束后，已有公开 assistant 消息可经现有 `last-message` 展示；不依赖业务结算成功。
2. 原生结束不再因为**连续可信观察链中仅结束快照缺 prompt**而永久拒绝；断线后也可通过 §5.2 的独立查询与原 working 起点标记核验恢复。真正归属缺失或冲突继续保留未核实，且原因可见。

本期生产适用范围固定为 `orca-terminal / orca_environment / codex`，Windows/amd64。共享状态源的改造覆盖本机端口编译与行为回归；本机受管 Codex/Pi/DSH 和 codex-app-server 的已有结果、消息语义不扩展。远端 Pi/DSH gate 保持原样。

## 2. 与既有契约的关系

沿用[产品契约](steerrunner.spec.md)、[执行可见性首版](execution-observability.plan.md)、[远端原生执行](orca-remote-native.spec.md)、[换代恢复](service-generation-recovery.spec.md)和[结果获取](result-acquisition.spec.md)。本文为已实现增量；实际取得的资格和未验证的来源形状分别登记在验收记录。

实施后，本文仅替代原“Orca 无最后消息样本”的支持限制，并为原生结束的 prompt 归属增加 §5 的严格窄化规则。调查基线在 application 的可信 final 结算路径已有通用补采样；缺口是 Orca 观察摘要没有直接接入，以及未能结算时无法依赖该补采样，并非所有 Orca final 都绝不可能显示。原生结果、业务结果、PR 和资源停止仍是各自独立的事实。消息显示不证明完成，done 不证明业务成功，`unreported` 不转换成底层明确成功。

公开 CLI 继续使用 `steerrunner-cli/v1`，`LastMessageView` / `LastMessage` 的字段与错误约定不变。无新增命令、HTTP 服务、用户配置、能力开关或上层容错选项。服务不可用仍返回既有错误，无样本仍返回 `message:null`。

## 3. 职责与唯一数据流

```text
Orca 既有状态订阅 / 既有有界原始查询
  → protocol/orca.StateFeed（带本机收到时间的瞬时样本）
  → orcatransport（原资源/轮次归属、连续性、公开消息选择）
      ├─ PublicMessage 单独事件 → application.acceptMessage → 原内存缓存 → last-message
      └─ NativeProof / LifecycleReceipt / 原生命周期事件 → 既有结算与诊断投影
```

| 所有者 | 唯一职责与限制 |
| --- | --- |
| `protocol/orca` | 路由、完整列表、原 worktree/pane/主 agent 选择与元数据交叉核验；记录样本收到时间及来源种类；不判断业务成功或构造公开摘要 |
| `orcatransport` | 复用同一观察轮次，将样本交给原归属规则和消息选择器；唯一持久执行证明仍为 `NativeProof`；消息选择器不签发结束证明 |
| `application` | 原五维消息身份校验、脱敏、UTF-8 限长、缓存和公开读取；通用观察可用性事件的诊断投影，不解码 Orca 私有 envelope |
| SteerLine | 消费现有查询与 diagnostic，不推断轮次、不读远端文件、不实施补发或另行采样 |

不增加消息表、第二个 Run 状态机、旁路轮询器、跨包回调总线、全局事件订阅、远端 helper/reader、日志正文解析器或 TUI 抓屏回退。现有用量日志读取授权与消息展示互不依赖。

### 3.1 瞬时样本与生命周期

在现有 StateFeed 上使用内部 `StateSample`：原 `AgentState`、本机 UTC `ReceivedAt`、来源种类（initial / notification / query）。只改变该内部端口及其已有实现、夹具，不改变 Orca RPC JSON、公开协议或持久 envelope。

- notification 在连接回调收到并解码时计时，进入有界缓冲前固定时间；initial 不冒充新通知。
- query 在既有后台原始查询返回并完成身份核验时计时。`last-message` 查询本身不读取状态源、不更新时间。
- 原缓冲上限、缺口报告、关闭和取消约定保留。查询与通知在同一 adapter 观察循环消费，不并发修改 `NativeProof` 或选择器。
- adapter 每个原 command 仅持有一份瞬时观察上下文：原身份引用、开始时的消息基线摘要、最近已选择的消息摘要/来源水位、当前 feed 的连续性标记。正文候选最多一条，受现有 Orca 正文上限约束；不复制完整历史或 `Turn` 执行状态。
- 上下文在原 command 更换、Attempt/Generation/原资源身份改变、observer 关闭时释放。重连清除连续性资格；保留同一存活上下文的去重水位，不能因换 feed 把 initial 当新消息。服务重启无上下文，重新建立基线。

## 4. 最后消息契约

### 4.1 可采样来源与归属

只采样原主 agent 的非空 `lastAssistantMessage`，kind 固定为 assistant；不读取 prompt、toolInput、toolName、工具输出、用户输入或 reasoning 作为展示正文。本期不由 Orca 工具字段生成 tool 标签。

采样时机受原生公开字段的发布时机约束，不承诺 token/delta 流、每条 commentary 或工具执行期间必有新正文。运行中有可归属样本才显示，没产生新正文不能生成假进展；Controlled 可注入原生字段更新验证接线，现场须分别记录真实字段是否产生及 Runner 是否正确消费。

选择消息前，原终端身份须按现有路径前后核验，绑定 runtime/environment、worktree、terminal/incarnation、pane、agent 和 provider；仅查到相同路径或相同标题不够。身份冲突、恢复未确认、session boundary、子 agent、ToolOutput 标记和已知新输入冲突均拒绝采样。

公开 TurnID 来自已经关联到当前 accepted/started command 的 `SessionContext`，不能从“当前界面刚好在显示此 Run”推导。归属规则如下：

| 情形 | 消息处理 |
| --- | --- |
| 新输入前 / 新轮 working 起点仍保留上轮正文 | 仅作为基线，不发布 |
| 已核验本轮 working 起点之后，连续可信通知中正文发生变化，主 agent 仍属于同一轮 | 发布新正文；不要求 final 完整或 outcome 成功 |
| 已核验的本轮结束来源有正文 | 可发布；正文达到截断边界也可作为有界展示，不能因此成为完整 final |
| 正文没变，但本轮已有独立可信 final | 沿既有 final 补采样，可与上轮同文；不能因正文摘要相同否认本轮 final |
| initial、重连首快照、查询快照仅有会话级最后正文，无本轮消息/结束关联 | 不发布，不能补填当前 TurnID；重新建立基线，等待后续可归属样本或可信 final |
| 中途文字来源不可确认 | 保持原样本或 null，不用 `updatedAt` 代替证据 |

不把 `StateObservation.Revision`、`AgentState.updatedAt` 当作消息 ID 或消息创建时间；工具状态变化也可能改变它们。来源水位用于拒绝旧观察，正文摘要用于抑制原字段残留，两者不能互相替代。没有公开消息 ID 时，同轮连续同文消息允许合并为一个最佳努力样本，不能承诺逐消息完整覆盖。

### 4.2 时间、去重与缓存

`observed_at` 是可采样原生样本被本机收到的时间；后台查询所得样本明确属于本机观察时间，不冒充远端消息创建时间。沿用现有字段，不增加第二套时间。

重复通知、同文状态刷新、重复查询、重连 initial、可信 final 的重复补读均不能刷新已有同一摘要的 observed_at。乱序与旧来源样本被丢弃；一次 drain 中先按来源顺序验证全部边界，再最多选择最新一条仍有效候选，后到冲突使尚未发布的候选作废。查询期间先处理积压通知及身份变化，再决定查询候选是否仍适用，不让较旧查询覆盖较新通知。

复用 `acceptMessage`：Run/Attempt/Session/Generation/Turn 核验；已知凭据与模式脱敏；修复非法 UTF-8；脱敏后正文上限 1200 UTF-8 字节加省略号。generation 使用 `SessionContext.Generation`，不能误用 `ResourceGeneration` 或 RunView 的其它 generation。

每 Run 仍只存一条进程内公开摘要。重启缓存为空；本期不恢复历史缓存，也不从持久 `Turn.Reply` 扫描回填。旧轮样本可作为同 Attempt 的最后已观察消息保留，返回其真实 TurnID，不能重新标为当前轮；新 Attempt 不返回旧样本。服务在最终结果后重启且无新观察时仍允许 null。

消息失败、不可归属或被丢弃不改变输入授权、执行状态、revision、events、恢复资格、业务结果与停止范围。

### 4.3 事件投递与首轮读取

PublicMessage 使用独立 `LifecycleEvent`，不混入 Fact、Implementation、Observation 或业务 Candidate；`acceptEvent` 的消息分支会提前返回，禁止依赖混合事件完成持久事实提交。

内部 `observeOnce` 返回一个 adapter 私有的瞬时观察结果，包含原 LifecycleReceipt 及至多一条消息候选；替换当前内部永远为空的 observer 返回位。不得为了传递此候选向公共 `LifecycleReceipt`、Operation 回执或持久 runstate 增加消息字段。

`ObserveLifecycle` 初次读取后把候选作为原 observer 的首条消息移交，不能在建立 observer 时丢掉。若同步读取已得到可信 final，继续使用 application 现有 `finishWithGate` 补采样，不创建仅用于消息的 observer。纯 Reconcile 读取不消费/推进消息去重水位，不投递未受生命周期管理的回调；后续正式观察仍可处理原候选，已可信 final 走既有路径。

原 started、Implementation、completed 和结束证据按原确认协议完整投递。消息仅走现有 observer 队列，至多等待其现有取消感知的确认；不新增确认层、后台投递协程或无限排队。停止时取消等待并释放观察资源，不使消息阻塞 close/stop。

## 5. 原生结束归属的窄化修复

### 5.1 统一判定，缺失与冲突分开

在 `orcatransport` 内集中原身份、输入边界及 prompt 判定，让 signal、query 和消息选择消费同一组基础归属结论；保留 final 更严格的完整性与 outcome 门槛。判定结果在内部分为可核验、暂缺证据、明确冲突，不以英文诊断字符串比较驱动新分支，也不扩展成通用诊断框架。

沿主线 PR #49 的 state v7 分类复用 `NativeProof.Conflict/Waiting`；v6 旧 Diagnostic 只在原解码边界转换，模糊历史冲突继续保留。查询暂不可用、结束快照缺 prompt 且证据不足属于可恢复观察诊断，通过原 receipt/event 返回，不写成不可重试的冲突，不能因此停止后续同源有界查询。

### 5.2 允许空结束 prompt 的充分条件

仅在以下条件**全部**满足时，缺失结束 prompt 可以沿用本轮已确认的输入关联：

1. 原 command 已 accepted、started；当前 feed 亲自观察过晚于 baseline 的主 agent working 起点，起点 prompt 与原输入预览匹配，provider 完整。
2. 从该 working 起点到结束通知，feed 未重连、未报错或缺口，authority/incarnation 不变，来源 revision 单调向前；原主 agent 起点引用保持一致，未出现另一原生输入边界、恢复未确认、session boundary、身份冲突或中断。Orca 会合并 tab 通知，不能据 WebSocket 不断线或 revision 递增推导边界完整。必须从该 working 起点取得正值 `turnStartedAt`，直到结束保持相同；经实现核验，该原生标记只在同轮保留，新的主输入起点会重置。缺失、变化或未核实该语义的来源均不取得空 prompt 资格；不要求全局 revision 数字逐一相邻。
3. 结束来自该连续链的可信 hook transition，主 agent 为 done、结束时间晚于本轮起点。标题 done、initial、普通查询快照不能替代该结束通知。已证明的相同结束记录若正文暂缺，原有有界查询可在同一连续上下文及相同正值标记下补正文；查询核对的独立主记录与 session 元数据必须为同一 state/stateStartedAt，不能据补正文建立新的空 prompt 结束。
4. 原 terminal/pane/provider 前后核验通过。正文完整性、首份 final 不可变、原 outcome 规则及后续业务核验仍全部通过。

连续性只是当前连接的观察覆盖事实，保存在瞬时上下文，不写成可跨重启复用的布尔证明。不能只凭已经持久的 `WorkingAt>0` 声称连续性，也不能把未报告缺口的任意快照推导成连续链。

2026-10-23 断线恢复补正：以上连续通知路径之外，远端 Codex 的**独立原主 agent 查询**可以恢复空 prompt 结束归属，须同时满足：

- 原 command 已 accepted、started，已持久保存 prompt 匹配的可信 working 起点；`Authority` 非空，`WorkingAt>0`，`RootWorkingAt>0` 且晚于 baseline。
- 查询保留的正值 `turnStartedAt` **等于原 `RootWorkingAt`**，而非任意曾经出现的时间戳。原生输入 hook 在起点设置该标记，工具/结束沿用，新主输入重置；不从结束时间、标题或正文反推标记。
- `worktree.ps` 的独立主记录与 `session.tabs.list` 元数据为同一 state/stateStartedAt，且 pane/provider 一致；只有交叉核验后的标记可以使用。元数据缺标记、记录不一致或两来源非零标记冲突时不保留任何标记。
- 原 runtime、terminal/incarnation、worktree、pane、agent、provider 前后核验通过；查询若带 observation，须匹配原 authority/incarnation，revision 不早于已保存水位且晚于 baseline。
- done 时间晚于原 working 起点与已保存观察起点，`updatedAt` 不早于 done；原冲突、中断、恢复未确认、session boundary、工具输出、outcome 和完整正文门槛继续适用。

该路径只由既有有界独立查询调用，不使 initial/重连首快照或通知获得连续性资格，不补造 start、不重发输入，也不增加持久字段或 codec 版本。观察器或服务重建后仍可使用原 proof；已有结束但正文暂缺时可按同一标记补收。缺少原可信 working 起点的 query-only 来源继续未核实。

| 结束来源 | prompt | 行为 |
| --- | --- | --- |
| 上述连续可信链 | 为空 | 可使用该链已有输入关联，再检查原 final 门槛 |
| 独立查询与原持久 working 起点标记核验一致 | 为空 | 可恢复原结束归属，再检查原 final 门槛；不恢复通知连续性 |
| 既有有效结束来源 | 非空且匹配 | 保持原完成资格；不要求新增连续性前置 |
| 任意来源 | 非空且不匹配 | 明确冲突，不被下一份空/匹配 prompt 清除 |
| 查询、重连、重启或漏 start | 为空且既无连续链也无原起点标记核验 | 暂缺归属证据；继续有界原来源读取，保持未核实 |
| 任意来源 | session/输入/身份边界冲突 | 保留冲突、拒绝完成及错误归属的消息 |

2026-10-23 SessionStart 补正：Orca 的 Codex `SessionStart`（包括压缩后的会话事件）也会清空预览、重置 `turnStartedAt`，但主 agent working 起点可以保持不变。因此该标记变化本身不能证明新用户输入。以上原路径保持；另允许 Runner 在既有 30 秒独立查询中，使用现有 `nativeChat.readSession` 核验原输入，条件如下：

- 只用于远端 Codex，已有 accepted、started 和持久的 prompt 匹配可信 working 起点；历史冲突仍不清除。
- 通过原 providerSession 的 ID 和其公开 transcriptPath 请求 Orca 的只读接口；Runner 不打开执行主机的私有文件，不部署 reader/helper，不读屏。
- 从当前尾部向前分页到固定 baseline，必须恰好只有一条 baseline 之后的真实用户输入，完整文本与原输入逐字匹配。再次发送同文也算第二条输入，不能只找最近的匹配文本。缺时间、截断、非 transcript 来源、不完整分页、重复游标或预算耗尽均未核实。单次最多 16 页，每页 100 条，读取与后置核验共享 40 秒期限。
- 读前和读后独立主记录的 pane/provider/path、main state/stateStartedAt/outcome、交叉核验的 turnStartedAt 必须一致；每次独立记录还须与该次 session 元数据的 agent/完整 main 记录及非冲突标记一致，不能把冲突标记清零后冒充一致。原 terminal/incarnation 前后核验继续执行。两次 session 元数据的 hook authority/incarnation 保持一致，revision 不回退；当前值还须与原持久 proof 相符，观察时间不得早于主记录起点。
- 该证据只补输入归属。working 必须仍为原 working 起点；final 仍要求独立主记录 done、原结束时间、完整且不可变正文及原 outcome 门槛。会话历史中的 assistant、READY、工具正文或标题均不作为 final。
- 运行中可在瞬时观察器内保留该输入与当前显示标记的绑定；只用于同标记、同原 working 根的可信后续通知。标记变化、显式边界、身份变化、缺口或更换 feed/重附着立即失效，必须重新独立读取；旧于持久/查询水位的通知不能撤销有效绑定。完成通知不继承该缓存的完成资格。补证前后已取得的明确身份、输入或原生 outcome 冲突按原规则保留，不能被补查失败隐藏。

不新增持久字段、codec 或生产开关，不修改 Orca。原可信 working 起点缺失、原输入无法完整核验、API 不可用的来源仍保持未核实，沿既有预算补查；不以宽松接受空 prompt 掩盖缺证。

2026-10-24 旧版远端薄投影补正：部分 Orca 版本在原生结束后，`session.tabs.list` 同时丢失完整 mainAgent、observation 和 turnStartedAt，而独立 `worktree.ps` 仍保留原主 agent 的 done 与正文。Runner 可在上述唯一原输入核验中使用另一个瞬时证明路径：

- 仅适用于已保留可信原 working 起点的 Codex、空预览、独立主记录 done。session 元数据必须是三项字段全部缺失的薄投影，agent/state 与独立记录一致。部分元数据、冲突字段、显式边界、恢复未确认、取消、工具正文仍拒绝。
- 在读取唯一原用户输入前后，通过现有公开 `terminal.show` 核对原 runtime、terminal、incarnation、worktree、agent、连接状态和不变工作区/分支；Lifecycle 还在接受补证后再次核验原绑定工作区。RPC 回执 runtime 必须匹配原身份。
- 读取后的独立主记录须与读前相同：pane、provider/path、main state/起点/outcome、updatedAt 和完整 assistant 正文均不变。用户输入必须满足既有有界分页、唯一条目、原全文及时间范围规则。assistant 历史条目不提供结束或正文。
- 使用不可从 JSON 构造的瞬时 query 证明，绑定精确原进程、主记录及用户条目；不补造 hook authority/revision/turnStartedAt，不授权 working 或后续通知，不缓存给另一个结束记录。已有持久冲突、原生结果截断和 outcome 门槛继续生效。

此兼容修复只修改 Runner，不修改、构建或部署 Orca。2026-10-24 最终构建的两轮真实交付及限定验证范围见[原生验收](../acceptance/orca-remote-native.md#2026-10-24-n308-19-真实远端-l4-最终结果)；协议回归自身不计 PR 成功。

2026-10-25 缺失 working 通知补正：仅对上段定义的薄 Codex done 投影，允许没有持久 working 起点时执行同一只读原输入核验。必须已有原输入 accepted/started、正值冻结主记录 baseline 和原 provider/pane；done 必须晚于 baseline。补查仍须以原 terminal/incarnation/worktree 前后核验、唯一完整原用户条目、原全文及时间界限、前后不变的独立 done/正文构造不可反序列化且绑定精确进程的瞬时 query 证明，Lifecycle 再核验原资源。没有 working 起点只解除这一路证明对 working 的依赖，不能以普通历史条目、tab snapshot、通知或文本相似度接受结束。已有 working 起点仍须满足原结束晚于该起点；已有冲突、重复输入、错身份、截断/工具正文、失败 outcome 和不完整分页均拒绝。没有正值 baseline 的历史记录继续未核实。本增量不补造 working/authority/revision，不改持久状态、codec、观察节奏或 SteerLine。

### 5.3 历史记录、结果与恢复

整合已合入主线的 PR #49：复用其 `NativeProof.Conflict/Waiting`、state codec v7 及唯一 v6 解码兼容边界，本专项不另增持久字段或版本。profile v3/v4、state 数据库版本及请求摘要不变。历史泛化 provenance 诊断不能被证明仅由 prompt 缺失引起，必须保留；不清库、不清诊断、不重解释已保存 unknown/outcome，也不补造 start/end。

本次现场原 Run 因此**不承诺升级后自动结算或自动补出最后消息**。对其只能按原授权、原身份与原证明继续只读对账；证据不足由公开诊断明确说明，另行处理不能借本文实施隐式执行。

仅经原 TurnEvidence → reliable.NormalizeResult → workflow 处理结果。last-message 不回流成为 final，截断摘要不进入 parser，消息正文也不证明测试、PR 或成功；显式失败/取消/unknown 继续阻断原生正常结果接收。

## 6. 诊断可见性

复用主线现有 `LifecycleReceipt.Diagnostic`、`LifecycleEvent.Observation`、RunView.phase/diagnostic 和原事件事务，不新增诊断查询或第二份存储。Observation 的非空 Diagnostic 表示不可核实，空 Diagnostic 表示原观察问题已解除；它不表示向用户提问。删除原 InputWait/Diagnostic 并行消费方案，只保留这一条链路。

adapter 用当前诊断值替代只报一次的 waiting 布尔量，在内容变化、解除、正式观察重附着或原 envelope 观察水位变化时投递独立更新。事件绑定原 Session/Turn 和新的原事件 ID；不能继续使用固定 `<command>:reply-missing` 作为永远只消费一次的键。沿主线约定，诊断与对应 Implementation 同事务提交，完成 Fact、PublicMessage 分别投递；不新增持久诊断计数器。

application 沿原事件分支核验原 Session/Turn/event，并原子合并诊断和实现证据；同一事件重放由既有 `SeenNative` 去重，内容冲突拒绝，事务失败保持可重放。相同诊断在解除后再次出现使用新事件 ID，不被历史键吞掉。observer 的串行投递与旧 observer 关闭保证更新次序，不能接收已撤销观察者的延迟更新。

同一原 Session/Turn 且仍处于正常 observing 时，将观察问题投影为主线已有 `phase=observation_unverified` 与脱敏 diagnostic，status 仍为 running，controls/授权不变。解除仅在该 phase 下清除本模块拥有的 diagnostic 并恢复原开始阶段。blocked、reconciling、waiting_input、stop、未知操作及其它阶段的诊断优先，不覆盖或清除它们；原 `merge`/`applyObservation` 也不能在尚无解除更新时把观察专属 phase 重置成 observing。回到正常观察后，由首读当前值或下个原观察水位更新重新投影现存问题；不为等待重新显示而另建轮询器。

诊断变化是实际生命周期可见性变化，可提交既有业务 revision/event；最后消息变化和重复读取不提交。query 失败原因、空 prompt 证据不足、正文截断及明确冲突应可区分，但不暴露提示词、凭据或私有路径正文。

## 7. 验收条件

| ID | 必须证明的结果 |
| --- | --- |
| OLM-01 | 新远端原轮可归属公开消息经过真实 EXE/服务返回；运行中与结束后分别有样本，不只依赖 final 补采样 |
| OLM-02 | 去重、乱序、工具状态变化、查询/通知交错和重连 initial 均不制造新 observed_at；真实来源收到时间不被消费时间替代 |
| OLM-03 | 多 Run、Attempt、Session、Generation、Turn 及子 agent、旧 provider/pane/终端替换隔离，旧轮残留不冒充新轮 |
| OLM-04 | 脱敏、非法 UTF-8、多字节截断和内存上限满足原契约；无消息表、额外查询器、直接日志文件读取或模型输入；§5.2 只读 RPC 用户输入补证不成为展示/结果正文 |
| OLM-05 | 消息与 started/completed/Implementation/诊断分开发送，事实不丢失；首轮同步读、observer 取消、队列压力和 stop 不死锁 |
| OLM-06 | 连续可信链、独立原起点标记或 §5.2 唯一真实输入补证的空结束 prompt 可以完成；断连/重建及延迟正文零重发恢复；无原 start、标记缺失/变化且无独立输入补证、非空冲突、另一输入及身份冲突不错误完成；旧匹配 prompt 路径仍通过 |
| OLM-07 | 旧持久冲突与结果不被清除或重分类；已核实正文、原 outcome、业务获取、PR 和停止门槛保持 |
| OLM-08 | 原观察诊断出现、变化、解除可见，重复不刷事件；其它诊断和人类问答不被覆盖，消息读取不推进 revision |
| OLM-09 | 服务重启摘要为空，之后仅由正常合法观察产生新样本；无恢复缓存或伪连续链，恢复全过程零业务重发 |

实现、受控验证、远端现场资格分开登记。缺少连续 hook 字段的现场不能把 OLM-06 正向记为通过；只有 final 后消息不能把 OLM-01 执行中部分记为通过。测试入口、改动位置与结构审计见 Plan。
