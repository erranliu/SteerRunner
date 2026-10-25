# Plan：Orca 最后消息与原生结束归属

日期：2026-10-23。状态：P0–P5 已实施、自审并完成本轮隔离验证及收尾；现场未触发项明确登记，未部署生产。依据 [Spec](orca-last-message.spec.md)，源码基线 `b2a05f0684fa670e7ce84e8e9e97dd1c1a3bcb86`。

按用户授权实施 Spec/Plan，测试仅使用独立 fixture、测试仓库与原生终端；不部署生产、不重启生产服务、不恢复现场原 Run。生产现场只读调查不是测试现场。

## 2026-10-25 缺失 working 通知增量（验证中）

Task18 第二轮已发布 PR419，但 Runner 缺少发布输入归属；独立只读补查能核实唯一原输入与同一 done。受控复现确认缺失流式 working 后，原补查门槛不可达。只扩展 Spec 新定义的薄 done 路径：有冻结 baseline、accepted/started 时允许补查；只有现有不可反序列化、精确进程绑定的独立 query 证明可以在没有 working 起点时核实该结束，普通 history/metadata 不获得资格。不新增模块、持久字段、codec 或调用方逻辑。

扩展原 `TestThinOriginalQueryCompletesThroughNativeLifecycle` 为已观察开始、丢失开始、丢失开始后观察器重建，各含唯一原输入成功与重复输入拒绝。保留原未绑定历史、跨进程/会话、结果变更、分页和 outcome 反例。定向运行 Orca 两 owner 包后按 `docs/testing.md` 执行 L1/L2。真实连续两轮须在新固定构建和新独立现场重做；失败轮不复写、不通过重发原输入修复。实时结果记于[执行可见性验收](../acceptance/execution-observability.md)，原 Task18 Unity/业务未验状态不变。下文历史的 working 起点限制仅在本增量的独立薄 done 证明成立时由上述规则取代。

## 1. 已定决策与完成边界

1. 使用现有 StateFeed、后台查询、observer 队列与 `PublicMessage`，没有第二个采样器或消息持久库。
2. 样本收到时间在协议源固定；initial / notification / query 不混淆。`updatedAt` 不等于消息时间，正文不变不等于新消息。
3. `orcatransport` 私有瞬时上下文只记录基线、去重与连接覆盖；`NativeProof` 仍是唯一持久执行证明。公共 backend receipt 不增加消息字段。
4. 空 prompt 可由当前 feed 的连续可信链确认；2026-10-23 断线恢复补正另允许独立查询按 Spec §5.2 核验 `turnStartedAt == RootWorkingAt`，恢复原持久 working 起点归属。无原可信 start 或无匹配保留标记的 query-only、断连/重启仍不可核实；不伪造 turn ID，不重发输入。
5. 旧冲突记录不重新解释；新出现的缺失证据不写成永久冲突。源归属与缺失/冲突分类在 adapter 内集中实现，不再添加英文诊断字符串分支。
6. 可见诊断复用现有生命周期事件与 Run phase，和纯消息缓存分开计数；不改变成功、恢复、授权、PR 或 stop 规则。

全部 OLM 验收完成后才可宣布目标范围交付。受控测试通过而远端现场未执行时，只能声明实现与受控范围通过；上游字段不足则明确记录未通过项，不能将其替换成宽松成功条件。

## 2. 改动归属与删除清单

| 位置 | 必要改动 | 必须避免或删除 |
| --- | --- | --- |
| `internal/execution/adapters/protocol/orca/state_feed.go`、`state_query_feed.go` 及直接调用方/夹具 | StateFeed 使用瞬时 StateSample，固定收到时间及来源；保持有界缓冲和身份检查 | 不改 RPC JSON；不在 Drain 时重打时间；删除被替代的裸样本端口，不能并存两个 feed API |
| `orcatransport/native_state.go`，必要的同包小文件 | 集中基础归属判定、缺失/冲突分类；原 proof 消费判定；唯一瞬时上下文处理消息基线与连续性 | 删除 signal/end 中重复 prompt 判定；不新增另一套 NativeProof、持久消息水位或恢复证明 |
| `orcatransport/lifecycle.go` | 维护每 command 唯一上下文；observeOnce 返回私有观察结果；前后原资源核验；首读候选交给原 observer | 替换内部无实际用途的 observer 返回位；不往 LifecycleReceipt/Operation/runstate 加展示字段，不更改公开 LifecyclePort 签名 |
| `orcatransport/observer.go` | 独立消息投递；原事实确认保留；诊断内容变化/解除事件；取消与 close 释放上下文 | 替换一次性 waiting 布尔量及固定 reply-missing 去重 ID；不增加投递后台协程或无限队列 |
| `internal/application/observation.go`、`worker.go` 的原 merge | 复用主线 ObservationStatus 及原 Session/Turn/event 原子事务；以既有 observation_unverified phase 控制诊断归属和清除；merge 不覆盖未解除问题 | 删除 InputWait/Diagnostic 并行消费方案；不识别 Orca 字符串/私有 state、不新增缓存、不把摘要当作完成事件 |
| `internal/application/last_message.go`、`results.go` | 原缓存与可信 final 补采样原则上复用；必要改动仅限发现的共同契约缺陷 | 不复制脱敏/UTF-8 截断函数、不在 last-message Handle 里做 I/O、不做历史回填 |
| 现有测试、文档、catalog | 在原测试族扩展正交断言；新增文档入口及真实资格登记 | 不新增同义 native harness、生产模拟开关或第二份 cases 来源 |

PR 整合以主线 PR #49 的 state v7 与唯一 v6 解码兼容边界为基线，本专项不新增持久字段或版本；复用其 Conflict/Waiting 分类及诊断事务。profile、公共 CLI/schema/request digest 不变。空 prompt 须有连续观察及同一保留回合标记，或由独立查询交叉核验原持久 working 起点标记；不能仅凭 WorkingAt 放宽重连/恢复资格。

## 3. 实施顺序

### P0：冻结证据和端口清单

1. 保存源码 SHA、测试 EXE SHA、资源摘要、fixture 配置、Orca runtime/版本及原始样本的脱敏副本到独立 evidence 目录；不复制生产数据库作为可执行 fixture。
2. 列出 StateFeed、queryNative、observeOnce 的所有调用方、夹具及本机实现；确认 StateSample 对原本机查询型 feed 的影响。列出消息、事实、诊断各自的事件确认路径。
3. 在 Controlled 夹具重现：原输入已开始、远端有正文但缓存 null；结束 prompt 空导致 provenance 拒绝；已保存泛化冲突无法推断历史具体原因。冻结旧 matching-prompt、缺 start、换 provider 等反例。
4. 保存同一远端连续 working→done 通知与独立查询的差异，核实 Spec §5.2 所需字段及缺口报告可用。还须核验该版本上游订阅的输入边界覆盖语义，不能从不断线、单调 revision 或本机队列无溢出推导上游不合并边界；未取得语义保证就按快照源拒绝空 prompt 正向资格。实际来源取证按验收记录登记，不把文档调查或受控数据当成真机字段形状。

退出条件：实际支持范围、端口改动清单和正反例明确；不可用字段按 Spec 保留未核实，不能靠猜测填值。

### P1：收敛原生观察与归属判定

1. 在已有 StateFeed 完成 StateSample 改造，通知回调、initial、后台查询分别固定时间/种类；同步改完本机实现、queryNative 接缝和全部夹具。
2. 引入同包私有的归属结果和瞬时观察上下文，不扩展全局接口。原状态只在已有观察循环修改，跨 goroutine 状态由现有 feed 缓冲交接。
3. 将身份、输入边界、prompt 判定集中为单一规则入口。开始必须严格匹配；结束按 Spec 区分匹配、空缺和冲突，signal/query 不另写近似规则。
4. 连续性须由当前 feed 实际消费的匹配 working 起点建立，连接报错、缓冲溢出、重连、身份/输入边界立即撤销。前后核验失败时先丢弃该批消息，再沿原执行错误路径返回。
5. 不清除已保存的泛化冲突；新“缺证据”仅通过瞬时结果返回，保留同来源后续读取。匹配 prompt 的现有查询完成路径不增加连续性要求。
6. 断线恢复补正：只在 `NativeProof.query` 使用严格的原 working 起点标记核验，不把 initial 或重连通知升级为连续证据。协议层先清空未核验标记，仅在独立主记录与 session 元数据一致时保留；无 marker、不同记录和冲突 marker 都不借用。回归包括断线、重建 adapter、延迟正文、完成事件及诊断解除，始终核验业务发送次数为 1。

退出条件：OLM-06/07 的 Controlled 正反例通过；持久 JSON 与版本无新增字段；没有将仅有会话级快照包装为本轮连续通知。

### P2：接通最后消息

1. 在发送前的原 baseline 读取处建立消息基线；working 起点重新锚定上轮残留正文。恢复时以首份当前快照建立基线，不发布 initial 中无法归属的正文。
2. 同一批先应用全部归属/冲突事实，最多保留一条有效公开消息候选；来源水位和正文摘要去重。query 返回后先处理积压 notification，拒绝旧查询覆盖新消息。
3. observeOnce 改为同包私有观察结果，包含 LifecycleReceipt 和候选；候选在核验全部成功后才可以发布。同步首读移交给随后创建的原 observer，首读已可信 final 则复用原 finish 补采样。
4. 纯 Reconcile 调用不提交消息去重水位，不能先“消费消息”又没有投递渠道；后续正式 observer 可以继续处理。没有正式 observer 且无可信 final 时允许无样本，不创建旁路。
5. 发出独立 PublicMessage 事件，填入原 SessionContext 的 Attempt/Generation/Session/Turn。消息事件与原 started、Implementation、completed 事件分别投递和确认。
6. 沿原 acceptMessage 去敏限长；测试原可信 final 与流式消息先后到达不会刷新同一摘要时间，也不阻塞结算。

退出条件：OLM-01 至 OLM-05、OLM-09 的 Controlled 断言通过；不依赖 final 补采样证明运行中显示，不以截断摘要证明 final。

### P3：接通诊断可见性

1. 复用主线 ObservationStatus：非空 diagnostic 是原观察问题，空 diagnostic 是解除。PublicMessage 单独投递，人类问答分支不变。
2. observer 在诊断变化、重附着或 envelope 变化时发送独立更新，使用新事件 ID，诊断与 Implementation 同事务提交；删除重复的证据状态投递路径。
3. application 沿原 Session/Turn/event 校验、SeenNative 和事务处理；同事件重放不增加 revision，冲突重复拒绝、失败不吞事件。覆盖 A→解除→A、A→B→解除；旧 observer 关闭后不能再消费其排队更新。
4. 只有主线 `observation_unverified` 允许由解除事件清空；更高优先级的 blocked/reconciling/waiting_input/stop/未知操作不被覆盖。在唯一 `worker.go:merge` 入口保留未解除 phase，删除 applyObservation 中重复保存/恢复逻辑。
5. 更高优先级阶段解除后，通过原正式观察首读或下个原 envelope 水位更新重发当前值；相同诊断的新变化使用新 ID，不被历史键吞掉，不增加高频轮询。

退出条件：OLM-08 的反复进入、解除、优先级和重启用例通过；控制资格不受观察 phase 影响，真实诊断变化可见，消息读取保持纯读。

### P4：受控、现场与兼容验证

按 §4 顺序运行适当测试。所有 EXE 现场使用同一最终构建，记录 SHA/资源摘要；若代码随后改变，重跑受影响门槛。不把生产 Run 的旧冻结启动配置或旧服务实例当作新实现资格。

远端测试复用原 CLI、StateFeed 与 live fixture，不部署 helper/读取脚本。使用独立 state-dir、仓库、原生终端和可清理资源；网络故障只影响该 fixture，不中断生产或其它并行测试。

### P5：文档收尾与结构审计

更新产品 Spec 的消息来源、观察时间、空结束 prompt 规则和诊断 phase；在可见性 Plan、远端原生 Spec、测试指南/清单与唯一 catalog 中同步已实际交付范围。实际结果追加到现有 `execution-observability.md`、`orca-remote-native.md`；同一份原始证据只保存一次，两份报告相互引用，不新建同义专项账本。

检查并删除已被替代的局部返回位、重复验证、固定诊断 ID、一次性 waiting 状态及失效注释。登记真实未通过边界，不以新增 TODO、双轨兼容分支或旧路径回退代替完成。

## 4. 验证安排

以下为验收入口；实际执行结果、构建 SHA 与限制见现有验收记录。用例必须可发现并实际运行，不能用零匹配退出码代替通过。

| 门槛 | 入口 / 范围 | 必须核验 |
| --- | --- | --- |
| T1：协议和 adapter Controlled | 下方 T1 命令；扩展原 NativeState/CompletionQuery/ObserverQueue 测试族，增加公开消息与诊断用例 | OLM-02/03/05/06/07/09；样本时间、连续性撤销、空 prompt 窄化、查询/流交错、首读与 stop |
| T2：application 与 CLI | 下方 T2 命令；扩展 LastMessage 和既有 CLI/消息测试族 | OLM-03/04/08/09；消息纯读、诊断事务及优先级、Generation 区分、同文 final 补读、本机 Codex 回归 |
| T3：真实 EXE/常驻服务 Controlled | 下方 T3 命令；扩展既有 `TestSRRunWorkflow`、`TestSRMultipleRunsIsolation`、`TestSRNativeUnknownAndStop` | 共享 application/CLI 契约回归：真实 last-message JSON、两 Run 隔离、读前后 revision/输入数不变、取消/停服无泄漏；这些现有本地 Codex fixture 不证明 Orca 生产接线或空 prompt 完成，adapter 特有断言由 T1、远端资格由 T4/T5 负责 |
| T4：远端原生定向 L3 | 扩展既有 `TestOrcaNativeStateLiveLifecycle`，按现有 `STEERRUNNER_ORCA_STATE_PROBE_*` 固定 manifest/runtime/environment/CLI；普通观察和 QUERY_ONLY 分开运行 | 连续可信链空结束 prompt 正向；query-only 空 prompt 未核实反向；严格匹配 prompt 原路径；本项不代替公开服务资格 |
| T5：远端公开链路 L3 | 在既有 `orca-remote.smoke/default`、`orca-remote.multirun/default`、`orca-remote.connection-reconnect/active-disconnect` driver 扩展断言，沿 `scripts/test.ps1` / catalog 既有生成执行方式 | OLM-01/02/03/05/08/09；运行中消息、可信结束、重连不伪连续、零重发、PR/资源范围仍按原场景核验 |
| T6：共享端口与完整受控回归 | 原 L1/Controlled 入口，确认所有新增非 opt-in Test 进入原包或 scenarios；本机 Orca 三 agent 的原夹具与 codex-app-server 回归 | StateFeed 变更无漏接；原 local Codex/Pi/DSH 结果和退出约定无变化；无新增远端执行支持 |

T1–T3 命令中的正则竖线表示或，不能改成匹配字面竖线的转义形式。目录占位符实施时替换成独立绝对路径。

```powershell
# T1
go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -run 'TestOrcaNativeState|TestOrcaCompletionQuery|TestObserverQueue|TestOrcaPublicMessage|TestOrcaObservationDiagnostic|TestOriginalAgentQuery|TestOrcaLocalQueryFeed' -count=1 -timeout=4m -v
# T2
go test ./internal/application ./internal/cli ./internal/execution/adapters/codex -run 'TestLastMessage|TestOrcaObservationProjection|TestPublicMessage|TestAsyncQuestionObserveSteerAndReconcile|TestSRCLIContract' -count=1 -timeout=4m -v
# T3
go test ./tests/integration -run '^(TestSRRunWorkflow|TestSRMultipleRunsIsolation|TestSRNativeUnknownAndStop)$' -count=1 -timeout=20m -v -args -evidence-dir=<独立绝对目录>
```

T4 的字段形状须真实采集，不在生产参数中增加“清空 prompt”或“丢通知”开关。Controlled 负责确定性反例；若真实 Orca 本次不产生空 prompt，则空 prompt 现场正向记未验证，不能以匹配 prompt 的成功替代。真实模型测试仅在独立测试 fixture 中执行。

T5 在 PowerShell 中沿现有入口选择：`& ./scripts/test.ps1 -Level L3 -Scenario @('orca-remote.smoke','orca-remote.multirun','orca-remote.connection-reconnect') -Config <独立测试配置绝对路径> -RunnerExe <本轮构建绝对路径> -OutDir <全新证据目录绝对路径>`。该入口按唯一 case registry 展开所选 scenario 的全部 cases，`connection-reconnect` 的 pre-send case 也随场景执行并如实登记，不虚构不存在的单 case 参数；目标新增断言放在对应原 driver/case 内。T4 使用原 opt-in 变量单独运行，不与 T6 的 L1 环境混用。

逐项映射：OLM-01 → T1/T5；OLM-02 → T1/T5；OLM-03 → T1/T2/T5；OLM-04 → T1/T2；OLM-05 → T1/T5；OLM-06 → T1/T4；OLM-07 → T1/T6；OLM-08 → T1/T2/T5；OLM-09 → T1/T2/T5。T3/T6 另负责共享契约及既有路径回归，不冒充远端来源资格。现场未产生运行中公开字段时，OLM-01 对应部分记未验证，不追加模型提示要求“报进度”来制造字段发布能力。

### 4.1 必须显式覆盖的反例

| 组 | 断言 |
| --- | --- |
| 时间与去重 | 通知入队后延迟消费时间不变；只变工具状态或 updatedAt 不刷新；同轮同文合并；不同轮同文可信 final 可显示；脱敏后同文遵守原缓存去重 |
| 残留与恢复 | 发送前旧消息、working 起点残留、重连 initial、服务重启已 done 均不自动成为本轮样本；旧轮样本保留真实 TurnID；新 Attempt 不泄漏 |
| 身份 | 分别替换 runtime、worktree、terminal incarnation、pane、provider、agent、Session.Generation；RunView 其它 generation 恰好相同也不能放行 |
| 来源竞争 | drain 内先消息后输入冲突使候选作废；查询途中有新输入/更晚通知不能覆盖；报缺口后只有独立查询核验原 working 起点标记才能恢复空 prompt；来源 revision 倒退丢弃 |
| final | 无 accepted/start、缺 baseline、空/截断正文、显式 unknown/failure/cancel、首份正文变更均拒绝正常完成；展示可存在且业务仍未核实；source 无输入边界覆盖保证时，即使连接不断且 revision 递增也不接受空 prompt 完成 |
| 永久与暂缺 | 旧泛化 provenance 冲突不清除；新 query 暂不可用/缺 prompt 不成为永久冲突；匹配 prompt 补证仍沿原资格恢复 |
| 诊断 | A→解除→A、A→B→解除使用不同事件 ID；同事件重放与事务失败可重试，冲突重复拒绝；解除不清掉 stop/问答/未知操作诊断；merge 重附着不重置未解除 phase |
| 生命周期 | 初读候选不丢、Reconcile 不吞候选、可信 final 无多余 observer；消息等待确认时 stop/关闭可结束；源和上下文只释放一次 |

## 5. 文档结构审计记录

本节保留设计自审记录，未使用独立审计 agent；实现审计及实际门槛结果另记在现有验收记录。审计关注数据权威、接口所有权、持久化、恢复和测试可执行性。

| 发现的结构风险 | 文档中的收敛决策 | 核对位置 |
| --- | --- | --- |
| 再建轮询器/消息数据库会形成第二条采样与恢复链 | 只扩展 StateFeed，原 application 缓存唯一 | Spec §3/4.2；P1/P2 |
| 用更新时间或“当前 turn”包装会话级正文，会串旧轮 | 基线、来源水位与正文摘要分工；无法归属就不发布 | Spec §4.1；T1/T5 |
| 在 Drain/界面查询时打时间，会把陈旧消息伪装成进展 | 源回调固定 ReceivedAt，消息读取零 I/O | Spec §3.1/4.2；T1/T2 |
| 在业务 receipt 增加消息字段，污染持久回执 | 仅使用 adapter 私有 observeOnce 返回结果及现有事件 | Spec §4.3；P2 |
| 消息与事实混发被 acceptEvent 提前返回吞掉 | 强制独立事件，保留全部事实确认 | Spec §4.3；T1/T3 |
| 首次同步读取/Reconcile 消费去重但没投递 | 首读移交原 observer；纯 Reconcile 不提交消息水位 | Spec §4.3；P2/反例 |
| 简单放行空 prompt、只看持久 WorkingAt 或以不断线推导边界完整会伪造恢复证明 | 连续可信链或独立查询严格核验原 working 起点标记；缺 start/标记及未知源仍无资格 | Spec §5.2；P0/T1/T4；断线回归 `TestOrcaCompletionQueryRecoversEmptyPromptAfterDisconnect` |
| 清除旧泛化冲突会重解释历史输入 | 历史冲突不清理、不迁移；本期不承诺原 Run 自动恢复 | Spec §1/5.3；T1 |
| 永久错误字符串阻止继续补证 | 缺失/冲突分类集中，暂缺用瞬时诊断返回 | Spec §5.1；P1 |
| 一次性 waiting 和固定事件键丢失解除/再次出现 | 复用主线诊断原子事件，不同变化各有 ID，同一事件幂等 | Spec §6；P3/T2 |
| 清诊断会覆盖人类问答/stop/未知操作 | phase 明确所有权及优先级，解除只清自身问题 | Spec §6；T2/T3 |
| Shared StateFeed 改造暗中改变本机支持范围 | 全调用方迁移与本机夹具回归，生产扩展仅远端 Codex | Spec §1；P0/T6 |
| merge 重附着覆盖观察 phase，使诊断反复消失或抖动 | 投影与 merge 一起收敛，只有明确解除可清自身问题 | Spec §6；P3/T2 |
| 把本地 Codex EXE 集成误算成 Orca 生产验收 | T3 只证明共享契约；远端接线与字段资格分别由 T4/T5 核验 | §4 T3/T4/T5 |
| 将字段存在误当作中途消息发布保证，或忽略既有 final 补采样 | 保留最佳努力采样时机，区分直接观察与结算补采样，不承诺 token/commentary 覆盖 | Spec §2/4.1；T1/T5 |
| 文档直接把设计写成已发布行为 | 设计阶段明确未实施；实施后同步产品与验收的实际范围，不冒充已发布 | 本文状态；Spec §2/7 |

复审结论：本版设计没有要求新增业务状态权威、消息持久库、旁路采样服务或远端 helper；不可证的 snapshot/历史记录保留明确边界。实际落地仍须按 P5 删除清单及 OLM 验收检查，不能把文档自审当作实现无结构债的证明。


实施收尾：改动、审计修复及实际门槛统一见[执行可见性验收](../acceptance/execution-observability.md)，远端资格引用[原生验收](../acceptance/orca-remote-native.md)。正式 L1/Controlled 通过；T5 四 case 的最新资格通过，首次 smoke 停止未确认及串行重验均保留。P5 未新增旁路账本、持久字段、来源服务或生产开关；已确认的实现问题均修复并验证。T4 未触发的真实空 prompt 与原生 working 字段发布不记为通过，不列为隐藏实现 TODO。

2026-10-23 SessionStart 增量：仅修改 Runner，复用原 bounded query，调用已有 `nativeChat.readSession` 补证输入归属。实现位于 protocol/orca 的分页/原身份后置核验和 orcatransport 的统一归属判定；不新增最终结果来源或状态格式。测试覆盖压缩后 working/adapter 重建/最终收尾、运行中缓存的边界失效、再次发送同文、完整正文匹配、历史冲突、分页完整性/游标/期限及读取途中资源变化。生产 Run 只读快照在独立评估中验证，不替换 EXE、不重启服务、不提交原 Run 状态或新输入。新增实际结果单列在验收文档，不继承以前的完整 Controlled/远端发布资格。

提交前质量审计先新增失败用例，复现两路记录混用、分页校验短路、缓存的旧样本/换 feed 行为及确定冲突被补查失败吞掉；随后修复并复验。原生元数据必须属于同一完整 main 记录，游标在任何成功返回前检查；缓存水位和 feed 所有权独立处理；补证前后沿原新鲜度及冲突分类准入，不清除历史冲突。最终回归和精确源码摘要见同一验收新增审计小节。

## 2026-10-24 既有远端薄投影兼容

在 protocol/orca 为完全缺失 main/observation/turn 字段的旧版投影增加瞬时 query 输入证明；用公开 terminal.show 前后围栏和不变独立原生 done 记录代替缺失的 hook 元数据证明。NativeProof 仅在已有可信原 start、完整唯一原输入、相同结束记录时使用该证明；Lifecycle 再核验原进程及工作区。无需修改 Orca、codec 或持久字段。

回归包含原终端替换、runtime/provider/path/工作区/分支变化、相同文本重复输入、assistant 历史、部分冲突投影、子 agent、正文变化与 JSON 伪造；完整 Lifecycle 验证 final/last-message 来自独立主记录且业务发送仍为一次。通过后独立构建 Runner，从同一业务基线开新运行；真实两轮 PR 门槛继续记录未完成。

### 2026-10-24 真实第四轮后续发现

`c74f80d` 的新独立远端运行已经取得可核验原生 final，原薄投影仍可见，实施结果进入发布阶段，last-message 与独立原生正文逐字一致；该轮因目标仓库发布技能将两个 tea 应用路径当成单一路径而 blocked，未产生 PR，不计连续成功。

后续 Runner 启动适配仅在非 GitHub 仓库的原生 Codex 子进程中，消除同一 tea 二进制的重复 PATH 暴露：保留首个路径及 SHA256；只移除内容相同、且没有其他可执行工具的后续目录；不同版本、混合工具目录和解析变化均拒绝。退出或失败恢复原 PATH；不修改机器配置、仓库技能、原 Spec/Plan、原业务提示、模型或 Orca。PowerShell5.1/7 均须验证真实进程启动、失败恢复及拒绝反例。

第四轮原 stop 发生 CLI 队列/命令期限耗尽，15秒外层预算短于40秒单次 CLI，关闭结果未知后不得自动重发；该轮已通过精确 incarnation 核验人工清理，不计自动 stop 通过。Runner 对远端 Orca 整个 stop 操作使用90秒有界预算，包含清单、身份、关闭与核验；本机15秒预算保持。既有未知关闭只查原清单，不因预算变更重发。需要两轮全新真实交付及自动 stop 重新证明最终构建，当前0/2。
