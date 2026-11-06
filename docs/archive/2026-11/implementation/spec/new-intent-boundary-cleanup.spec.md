# Spec：新意图写入边界与展示残留清尾

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：设计审计后已实施，NIB-00–05完成，见[验收](../acceptance/new-intent-boundary-cleanup.md)。基线main `98309f835012c5ea43cad2245a11341a0d9fc040`，包含#85及Controlled选择整理。依据[长期产品契约](../../../../spec/steerrunner.spec.md#长期产品承诺)，配套[Plan](new-intent-boundary-cleanup.plan.md)。

## 1. 目标和边界

A只收窄新操作意图的构造入口，删除无赋值的瞬时准备分支。B删除无消费者展示方法和原生观察事件只写标签。历史解码、已保存原请求、owner校验、原操作对账和停止均保留，state/v8及公开能力不变。收益为更明确的新写入范围和少量维护面减少，不宣称任务提速或显著缩短验证链。

## 2. A：仅收窄新写入侧

删除 reliable/operation.InputValue 对 report_completion、acknowledge_delivery、cleanup_resources、host_action 的四个构造分支。InputValue只被operation.New调用，生产由application.saveIntent进入；这四类没有当前生产候选。合法输入也必须在构造新意图时返回unsupported，不能生成操作、写入Intents/Snapshot.Operations或派发。仍使用原错误路径，不增加新公共开关、账本或新旧路由层。

删除backend.PreparationFacts.Cleanup瞬时字段和reliable.Next对应的清理候选分支：所有实际PreparationFacts适配器/夹具均未赋值。不得删除LifecycleInput.Cleanup、CleanupInput或对应持久枚举/解码；真正资源清理和停止不依赖这个虚设步骤。

必须保留：

- LifecycleInput.Validate/Clone、LifecycleCandidate.Validate、LifecycleRequest.Validate、Receipt.ValidateFor、CandidateForOperation全文及全部旧动作/字段。旧形状可解码、复制和校验，不等于当前适配器承诺执行这些动作。
- operation.New除InputValue拒绝新动作外的固定输入/摘要/身份；保存、Invoke、Reconcile、Settle/SettleStop、双操作表示及owner.validateOwnerRequest全文。
- 已提交旧操作直接按原Request进入对账，不重新通过operation.New/InputValue构造；不能在Invoke/恢复/停止入口增加本次新写入限制。
- submit_input全部现有新构造、派发前校验、Orca执行/对账分支；prepare_workspace/create_resource/prepare_session/start_turn/deliver_feedback/stop_observation/stop_execution现有路径。

新测试应使用合法旧输入和真实saveIntent/SQLite证明拒绝发生在持久写入前；另以手工构造的原保存字节（不调用已收窄New）核验四类CandidateForOperation、请求/回执校验及Invoke只走原Reconcile。该测试只证明原路径没有被新写入门禁阻断，不授予适配器未支持旧动作的新执行能力。

## 3. B：瞬时展示残留

删除agentlaunch.Config.Display（无调用者），保留Config字段、加载/校验、LaunchCommand/RemoteConfig及实际启动权限/模型配置。

删除codex.ObservationEvent.Summary/Title及Observe中对应赋值；保留PublicMessage/Persisted/Envelope/Forward/Kind/Status。Kind/Status由async_transport_test真实读取，不得删除或改写断言。Observe的replay、事件选择、校验/拒绝、journal.Write/Sync、terminal checkpoint、deliver调用次数/顺序和取消/完成判据必须逐字等价；不能顺带调整单确认或新增持久消息。

ObservationEvent是进程内值，原生journal保存codexapp.Envelope，owner RPC保存Receipt；不保存被删标签。共享MockAppServer、StagedBusiness、historyFixtureBinding、正式native helper/创建探针和原测试全部保留。若发现新真实消费者，先收窄清单，不删除消费者制造无引用。

## 4. 持久格式与长期承诺

全部产品持久类型、字段、动作字符串、严格decoder、profile/ImplementationDataEnvelope/HistoryBinding、Run/Request/Receipt/Operation、Intents/Snapshot.Operations、SQLite user_version=8、CLI v1及owner/native协议不变。PreparationFacts和ObservationEvent是瞬时值，必须从持久根捕获中明确排除其授权形状变化，不能因瞬时声明改变而误报持久变化，也不能仅凭零值编码相同判定格式不变。

PC-R01–R06/R08/R09沿原提交、原资源/结果归属、未知零重发、有界恢复、原预算和stop撤销/范围保持；可选展示缺失不成为业务门禁。没有持久旧格式退役、恢复框架或能力收缩。

## 5. 最小验收

| ID | 必要结果 | 最小方式 |
| --- | --- | --- |
| NIB-00 | 设计先于生产编辑 | 重新核对唯一InputValue调用及四类候选、PreparationFacts全部生产者、Display和标签实际消费者、历史/owner/共享夹具入口，保存文档摘要与源码/编码基线 |
| NIB-01 | 只拒绝新写入 | 合法四类输入进入真实saveIntent均拒绝；Run/SQLite原字节、Revision、Intents/Operations及events数量不变，零派发；保留动作仍能构造原输入，含submit_input和stop |
| NIB-02 | 原保存路径保持 | 四类原字节可严格解码并通过原请求/回执校验，unknown原Request只调用Reconcile；不经New重建；原操作/停止/晚到结果/缺预算及普通重启授权回归通过 |
| NIB-03 | 共享夹具与格式 | 原测试/共享helper字节保持；持久根类型、代表Run/请求/回执/旧动作字节、公开能力/资源摘要相同；AST仅允许指定5个保留声明修改及Display删除，Observe正文作精确标签移除比较 |
| NIB-04 | 最终最小执行资格 | execution/...与runstate/contracts/resources/storage/testkit/protocol组件、application定向；native/integration live编译、正式helper/Runner构建；同最终EXE三个公开受控NativeUnknownAndStop/RestartPreservesAuthorization/ServiceOwnership场景3/3、0skip |
| NIB-05 | 可复核收尾 | A/B独立净删、精确命令/退出码/名称/数量/skip/失败、源/产物SHA、Git原字节证据保护、验收/长期说明/索引完整 |

新增测试针对新写入与原保存路径的不同边界，不以删除旧动作夹具或有效断言取得通过。按最新主线的Controlled选择/计时约定登记这三个场景，不冒充完整Controlled或L1/L2/L3资格。源码冻结后才编译/执行，不边编译边编辑；发生失败先确认原因并定向修复，原记录保留。

## 6. 停止条件和交付

NIB-00–05满足即完成；若发现格式变化、历史消费者被新写入限制阻断或保留行为不等价，先修订/收窄设计再继续，禁止扩大恢复框架。初轮授权为设计审计后实施验证；后续用户已明确授权再审计后提PR，按该授权提交、推送及创建PR，不自动合并或部署。

## 7. 实施前设计审计

2026-11-02，在生产/测试差异为空时核对787项已跟踪源/配置，保存Spec/Plan摘要、引用与编码基线后通过NIB-00。四类动作仅保留在持久形状、Validate/Clone/decoder及新构造switch；移除空Cleanup候选后没有当前生产者。原Invoke和owner核验不调用InputValue，未知恢复直接读取原Intent。Display无调用，Summary/Title无读取，异步问答仍消费Kind/Status；共享夹具必须保持。

初轮名称检查还匹配到PowerShell的InputValue参数和livetest.Cleanup夹具值，均与本次Go函数/PreparationFacts字段不同，按作用域排除并保留，不删共享测试。新主线选集没有恰好三场景的组合，本次沿test-foundation精确选择这三个场景，再用test-controlled的FromEvidence导入计时，不扩大组选集或资格。此处仅为设计通过，运行结果待实施验收。

## 8. 实施结果

A/B生产分别净减11/15行，共26行。四类合法旧输入不能再新建/落盘，手工保存原请求经真实SQLite重载和正常advance只对账；8类保留动作（含submit_input/stop）原构造保持。90个保留声明及782项其余已有源/配置相同，原测试和共享夹具未改；持久根/旧输入/能力编码145263字节相同。历史解码、原请求校验、owner、Invoke、预算与停止正文保持。

最终498个组件/application顶层及三个公开受控场景通过；8个既有skip和3个无测试包单列。新夹具两轮失败为内部调用门锁约定和缺原冻结时间，修正新增测试后通过，生产不放宽。Controlled计时导入通过但qualification_eligible=false；无新增真实模型或完整L1/L2/L3资格。尚未提交/推送、提PR、合并或部署。

同日后续[实现再审计](../acceptance/new-intent-boundary-reaudit.md)未发现产品缺陷，原生产/新增回归无需修改。采用最新main66cc0ac的上游测试入口，重新确认历史解码/原对账/共享夹具/持久与停止边界，27个定向及最新EXE三个公开场景通过；既有文件夹具初始化失败和复验单列。上段保留初轮资格及当时交付状态，后续按新增授权提交提PR。
