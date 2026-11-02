# Spec：单事件确认与旧环境分支收口

日期：2026-11-02。状态：A/B已实施并完成专项最小验收，见[验收](../acceptance/event-ack-cleanup.md)。基线：main `72b589baebb47c248fe0845352f400237cc0ed02`，含 #83。依据[长期产品契约](steerrunner.spec.md#长期产品承诺)，配套 [Plan](event-ack-cleanup.plan.md)。

## 1. 目标与范围

A 将通用事件的双确认收敛为原 Persisted 单确认，删除无业务消费者的通用事件字段和标签透传。B 删除无合法生产赋值的 BindingSnapshot 环境分支，以及零消费者旧声明。不降低 PC-R01–R06/R08、PC-C03/C06/C07，不改业务执行、原生事件选择、恢复、停止或可选消息的责任边界。

收益为减少一个事件等待阶段、跨 worker/owner/适配器/夹具的重复协调和旧模型维护面。生产增删分 A/B 统计，测试不计生产收益；不宣称任务提速或吞吐提高。

## 2. A：单确认的实际提交边界

保留字段名 Persisted 和缓冲为 1 的 chan error，不增加确认封装、公共开关或新的持久 ack 账本。Orca 每事件只等待这一个结果；删除 Processed 的分配/等待，以及 worker、owner 和所有实际夹具中的第二次回复。

| 事件消费路径 | 唯一回复边界和含义 |
| --- | --- |
| application.run → acceptEvent 的持久事实 | acceptEvent 的身份/冲突核验和实际 commitLocked 返回后，将同一个 error 回复 Persisted；保存失败必须回复错误，内存/持久事实不得提前发布。未回复不得继续下一次观察、确认消息游标或将该事件视作已处理 |
| 已提交重复事实、按原规则忽略的事实 | 沿 acceptEvent 原去重/忽略语义返回 nil，不强制新事务，不把重复事实重新推进业务 |
| PublicMessage 等可选内存提示 | 沿 acceptMessage 原校验/采样语义返回；nil 仅表示本次处理结束，不承诺新增 Run 事务，不把消息故障提升为业务门禁 |
| 独立 Codex owner 的 drain | 原 Observe 已写入并 fsync 原生 journal 后才发布事件；owner 在原 drain 点回复 Persisted=nil。它不持有 Runner DB，不能把此回复解释成中心事务已提交 |

错误回复使原 Orca send 返回失败，保留原错误和清理行为；不确认消息消费游标、不重发业务输入。取消/Close 仍可打断等待，Close 非阻塞，原 awaitCleanup 有界等待实际 goroutine/reader 退出；不得以自动 nil 回复掩盖取消、失败或未处理事件。保留原 Codex 原生 journal 的写入、Sync、forward/reject、terminal checkpoint、启动/完成选择与所有回执来源判据。

## 3. 删除和保留清单

| 增量 | 可删除/调整 | 明确保留及实际消费者 |
| --- | --- | --- |
| A 确认链 | LifecycleEvent.Processed；Orca 第二等待；application.worker/独立 owner 第二回复；9处 Orca 正式/组件/探针夹具对应回复 | Persisted 原提交边界、Fact/Implementation、PublicMessage/Observation、取消/Done/finished/awaitCleanup、原生 journal 与未知零重发 |
| A 通用事件字段 | Candidate、DeliveryID、DeliveryFinal、Summary、Kind、Status、Title 及 Clone 的 Candidate 分支；Orca 通用标签生成、Codex 向 LifecycleEvent 的四字段透传 | 不删 LifecycleCandidate、持久 Receipt/输入/Snapshot 的同名字段。底层 codex.ObservationEvent 的 Kind/Status 等仍供原生问答测试使用，全部保留 |
| A 共享诊断 | short_input_trace_test.go 真实读取 LifecycleEvent.Kind，必须调整，不能整份删除 | 测试 trace 在原调用处依据 Observation 与 Fact.Kind 推导相同 turn/execution 分类，保留原 JSON key/value 含义、其余字段和 probe 断言；不为 trace 增加生产兼容字段 |
| B 旧环境分支 | BindingSnapshot.Environment/PolicyResources、EnvironmentBinding/PolicyBinding 及专用复制/回执合并分支 | 唯一生产构造 ApplySessionReceipt 未设置这两字段；保留 MergeReceipt 的实际 Target/Workspace/Process/Session/Implementation 身份验证、错误不泄露部分修改和 SameProcess |
| B 零消费者声明 | backend.Observation、OutputBatch、EventBatch、LifecycleResolver、reliable.CleanupStepLimit | 保留 codex.Observation、ObservationStatus、原生 StateObservation、PRClues/ToolReplyFact/TurnEvidence/EventFact；ExecutionRecordVersion 实际用于校验，ReconciliationStepLimit 有正式探针消费者，不删除 |

生产无 Processed 独立阶段：worker 紧接一次 acceptEvent 返回向两通道回复同一 error；owner 同一 drain 点回复两次 nil。所有 Processed 测试消费者仍需运行原断言，仅移除第二回复。全仓包含正式 live 测试、scripts/client 和历史独立模块；历史 evidence 原字节不改。若发现更多真实消费者，先修订本清单，不以删除有效断言或恢复无用兼容字段使编译通过。

## 4. 持久格式不变

LifecycleEvent/其确认通道和 BindingSnapshot 是进程内值，不是当前 Run、SQLite 或 owner RPC/journal 的保存根。owner RPC 使用 ownerResponse.Receipt，原生 journal 保存原 Envelope；先核对实际编码消费者，再删瞬时字段。B 的旧 Observation/Batch 亦无当前保存根，不因带 JSON 标签而恢复未使用栈。

state/v8、SQLite user_version=8、CLI/Workflow/owner/native 协议、公开 capabilities/资源摘要不变。不删改 Run/Snapshot/Input/Request/Receipt/Operation、ImplementationDataEnvelope、profile、HistoryBinding、Host/Probe/Delivery/OriginalHost/PolicyResources 持久字段、动作/decoder 或 Intents/Snapshot.Operations。尤其不能把 BindingSnapshot.PolicyResources 与实际 Snapshot.PolicyResources 混为一项删除。

实施前捕获选定持久类型及代表性 Run/snapshot/request/receipt/operation、HistoryBinding、公开能力和资源摘要；实施后逐字比较。另用声明/函数比较逐项约束所有未授权变化。实际事件源选择、身份绑定、保存、消息采样、结果/原生权限、恢复和停止正文应保持；只对本 Spec 的字段/分支/确认回复做必要调整。测试 trace 当前分类语义保持，历史诊断证据不回写。

## 5. 最小验收

| ID | 验收要求 | 最小方式 |
| --- | --- | --- |
| EAC-00 | 设计审计先于代码 | 确认三类 ack 含义、owner journal 边界、trace/原生问答/正式夹具真实消费者，保存设计与编码基线 |
| EAC-01 | 单确认保留真实提交边界 | 用真实 worker.run 和 SQLite：阻止 acceptEvent 完成时无成功回复；提交完成后 nil 且已落盘；注入提交失败返回错误且无部分发布；重复/可选消息不被迫新增事务 |
| EAC-02 | producer 等待、错误和取消正确 | 实际 Orca observer：未 ack 不推进、一次 nil 后可继续、错误 ack 原错误退出、不推进；Close 在等待时非阻塞且可有界 awaitCleanup。保留 queue cancellation、原发送次数和来源断言 |
| EAC-03 | 消费者/保留承诺 | execution/... + runstate/contracts/resources/storage 组件；application 原操作/人工恢复/观察/停止竞争/late result/FrozenBudget/LocalCodex/服务代次/Workflow 恢复定向；原生问答 Kind/Status 测试仍通过 |
| EAC-04 | 格式与正式消费者 | 前后编码/公开能力相同；持久根和声明比较；native/integration `go test -c -tags live`，包括所有 Processed/trace probe 调整，不以零匹配通过 |
| EAC-05 | 最终公开链路 | 正式脚本构建最终 Runner；foundation TestSRNativeUnknownAndStop、TestSRRestartPreservesAuthorization、TestSRServiceOwnership 三场景，数量3、无 skip，含原资源和夹具清理 |
| EAC-06 | 可复核交付 | 分项生产净删、源码/产物 SHA、精确命令/退出码/数量/skip/首轮失败、Spec/Plan/验收/长期实现说明及索引 |

新增测试只覆盖真实确认边界和取消/错误行为，不照抄被删字段或用恒真断言替代原测试。不强跑真实模型/helper 或全 L1/L2/L3；本次不改变协议或实际业务执行，对变更机制做上述最小验证。运行失败先修复确认问题再定向复验，不能靠删除测试取得通过。

## 6. 完成和停止条件

设计审计通过后按 A、B 分别实施并记账；源码冻结后才构建/运行，禁止边编译边编辑源码。EAC-00–06 满足后更新验收和长期文档；历史验收保持原判。初轮不自动提交/推送或创建 PR；后续用户已授权再审计、修复确认问题后提 PR，按该授权交付，不自动合并或部署/发布。任一格式/合法入口反例或确认边界不等价时先收窄设计，不能通过新增恢复框架绕过。

## 7. 实施前设计审计

2026-11-02，在生产差异为空时通过 EAC-00 设计复核并捕获编码基线。确认 Persisted/Processed 不是两个独立提交阶段；持久事实、可选/去重事实及 owner journal 三类边界按 §2 保持。补查发现短输入 trace 确有 Kind 消费，已列为需保持语义的真实诊断；底层 Codex Kind/Status 原测试和9处 Processed 实际夹具均保留有效断言。B 仅删无入口瞬时模型，不删持久同名字段。此处仅为设计审计结论，运行资格待实施后验证。

## 8. 实施结果

EAC-00–06完成。A/B生产分别净减38/67行，共105行；持久类型/编码、公开能力和资源摘要不变。446个组件与46个 application 顶层通过（新回归含在其中），三个最终公开场景通过。8项skip/3个无测试包及新 observer 夹具首轮失败单列，生产入口校验未放宽。单确认仍在§2原边界回复，177个保留声明相同、9个授权声明调整；最终构建及资格以验收为准。未提交或部署，无新增完整 L1/L2/L3 资格。

同日[实现再审计](../acceptance/event-ack-reaudit.md)未发现新增确认缺陷，补充 worker/owner/Orca observe 全文精确变换及9份夹具核对，持久格式和保留承诺保持，33个定向顶层通过；生产/测试源码与初轮最终验收一致，按后续授权提交并创建 PR。
