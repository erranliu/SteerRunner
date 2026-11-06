# Spec：旧历史采集支线与 Orca Backend 包装清理

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：已实施并完成专项最小验收，见[验收](../acceptance/history-adapter-cleanup.md)。基线：main `e2bc0a578047fa38293edbe411b15a386ddd20a4`，已包含 #82；实施前工作树与该版本一致。依据[长期产品承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)，配套 [Plan](history-adapter-cleanup.plan.md)。

## 1. 目标和实际收益

只清理没有生产调用的旧 Codex 历史分页采集支线（A）和旧 Orca Backend 包装/接口（B）。不降低 PC-C03/C07、PC-R01–R06/R08，不改变当前 conversation Workflow、消息查询、结果获取、原操作恢复和停止。第三项 BindingSnapshot 旧环境分支及持久字段/动作退役不在本批。

A 的 conversation_adapter.go、conversation/source.go、evidence/conversation.go 主体共 713 行，另有分页、事件映射和能力汇报专用外围；B 的 Orca 包装为 62 行，另有三个旧接口及能力类型。最终分项统计生产新增/删除/净变化，测试删行不计收益。收益为维护面和重复模型减少，没有可比测量，不承诺运行或测试提速。

## 2. 实际消费者和删除闭包

| 增量 | 当前引用/入口 | 删除范围 |
| --- | --- | --- |
| A 历史采集适配 | NewConversationAdapter、DiscoverHistory、ReadHistory 仅专属测试调用；application/host/CLI/storage、正式 tests 和 scripts 无装配入口 | 删除 codex/conversation_adapter.go、execution/conversation/source.go；evidence/conversation.go 只退役旧事件模型，原 MessagePhase 类型和三常量迁入 message_phase.go 保留，不删除其他实际业务类型 |
| A 专用外围 | ReadMetadata、ReadPage、HistoryPage/MaxHistoryCursorBytes、MapHistoryTurn、conversationItemKey、history capabilities/position、HistoryBinding.Event 只供上述闭包；CheckHistory、cloneStringMap、NativeHome 无调用，UserAgent 仅旧适配器调用 | 删上述声明；只退役 HistoryBinding.Event 方法，保留 HistoryBinding 本身及其有效读写；不删除整个 history.go/history_binding.go |
| B 旧 Backend | orcatransport.Backend 无构造和调用；ExecutionProbe/ExecutionBackend/ExecutionStopper 仅声明/组合，没有现行使用 | 删除 orcatransport/backend.go 和 backend/backend.go 的这三个接口；不是删除实际 codex.Backend 或 LifecyclePort |
| B 旧能力类型 | backend/capability.go 的报告/状态在现行模块只供旧 Orca 包装和 A 的旧 ConversationCapabilityReport | A 完成后删除该文件；公开 resources.Capabilities 属另一套机制，保持原值 |

conversation_adapter_test.go 和 final_history_test.go 的测试函数只测试退役适配/MapHistoryTurn，随闭包删除。startHistoryFixtureProcess、historyFixtureTurn 等 helper 无外部测试消费者；但 historyFixtureBinding 被保留的 recovery_history_test.go 使用，原夹具迁到 history_binding_fixture_test.go，不能删其有效测试。当前模块与 docs/acceptance/evidence 的历史独立 Go 模块分别审计，历史字节不改、不接回编译。

设计审计必须重新全仓检查所有 helper，不能只搜构造器。任何真实入口或共享消费者反例都先收窄删除，不能用兼容包装或新的分层替代死代码。

## 3. 必须保留的执行和恢复链

- lifecycle.readRecoveryThread 使用 ReadHistoryBinding 校验原 project/execution/attempt/workflow/session、工作区、home、命令和参数，之后查询原 thread。保留 HistoryBinding 全部字段、Validate/Identity、WriteHistoryBinding/ReadHistoryBinding、文件名/路径及编码。
- history_binding.go 的 readConversationSourceFile 真实消费原适配文件内的 maxConversationCursorBytes。把 **64 KiB 原常量**移到实际读取文件，值和读取/校验逻辑不变；不能随旧游标一起删除或放宽读取。
- 保留 OpenHistory/HistoryClient 的启动、initialize 身份/home 校验、call、Close、WritersAbsent 及真实 Job 结算。只删除不用的 getter，不顺手改握手/metadata 字段或资源生命周期。
- 保留 HistoryThread/HistoryTurn、ReadThread、verifiedHistoryThread、historyThreadIdle、HistoryTurnFacts/InspectHistoryTurn；InspectHistoryTurn 仍被 lifecycle.readRecoveryOutcome 调用，不能因旧适配器也使用它而误删。
- InspectHistoryTurn 直接读取的 MessagePhase 类型及 commentary/final_answer/unknown 三个原常量保留原声明，迁到 evidence/message_phase.go；不改实际核验为其他判据。
- 保留 canonicalPath、historyEnvironment、digestValue、ReaderCommand、EnvironmentHome 等真实共享 helper；codex/conversation.go 中的实际共享 JSON helper 不在本批。
- 保留 application 的 LifecycleEvent → acceptEvent/acceptMessage、PublicMessage/LastMessage/Observe、原生 final 和内容 Workflow；保留独立 owner/checkpoint 签名、原 session、未知零重发、保存失败零外发、停止撤销、迟到创建/结果、权限/read_only、Orca workspace/native observation 和 usage 租期。

## 4. 持久格式与兼容边界

state/v8、SQLite user_version=8、CLI/Workflow/原生协议及公开 resources.Capabilities 和资源摘要不变。不改 Run、Snapshot、Input、Request、Receipt、Operation、ImplementationDataEnvelope、profile 或 HistoryBinding 的任何持久字段、类型标签和实际编码实现，不删 Host/Probe/Delivery、旧动作/decoder，不合并 Intents/Snapshot.Operations。

被删的 ConversationEvent/Write/Checkpoint/Capability 和 source-position 是未接线采集支线的类型，不是现行 Run/storage 的持久根；不能仅凭有 JSON 标签判定为现行格式，也不能仅凭没有当前生产者删除真正的持久字段。设计审计须确认不存在存储/工具消费者。实际生成且被恢复读取的 history binding 文件保留，旧历史证据原字节保留；不增加迁移器或要求排空用户 Run 来掩盖格式变更。

实施前保存代表性 Run/snapshot/request/receipt/operation、HistoryBinding 和选定持久类型定义、公开 capabilities/资源摘要的固定基线，实施后逐字比较。再以全部保留声明 AST 比较约束共享函数和类型；差异只允许原 64 KiB 常量和 MessagePhase 声明迁址及必要 import 调整，不以单个 JSON 样例相同代替全部持久形状审计。

## 5. 最小验收

| ID | 要求 | 最小方式 |
| --- | --- | --- |
| HAC-00 | 文档及设计审计先于生产删除 | 记录消费者闭包、64 KiB 和 InspectHistoryTurn 两处共享边界；先捕获编码基线 |
| HAC-01 | 无实际入口误删，无候选搬家 | 全仓引用/Go 编译、历史模块单列；两个专属测试文件外的有效断言保持 |
| HAC-02 | 持久格式/公开能力未变 | 固定编码及 HistoryBinding 前后字节一致；生产变更文件全部保留声明相同，常量只迁址；实际状态版本和编码消费者核对 |
| HAC-03 | 原恢复/结果/停止保持 | execution 相关组件及 contracts/resources/runstate/storage；application 定向覆盖原意图/回执失败、unknown、人工恢复、观察、FrozenBudget、本机停止只读对账和普通重启授权 |
| HAC-04 | 正式消费者和最终公开链路 | native/integration 各 `go test -c -tags live`；正式构建最终 Runner；foundation 的 TestSRNativeUnknownAndStop、TestSRRestartPreservesAuthorization 两场景，匹配数量必须为 2 且无 skip |
| HAC-05 | 交付可复核 | 源码/产物 SHA、命令/退出码/数量/skip/首轮失败、分项生产净删、验收报告和索引 |

不新增只照抄实现的测试；保留现有 recovery_history、owner_checkpoint、真实 final 和 PublicMessage 测试，不将死 MapHistoryTurn 测试当成全部 final 资格。无运行链路变更时不强跑真实模型/helper 或完整 L1/L2/L3；发现实际运行变更则先修订范围、补对应验证。

## 6. 完成边界

设计审计通过后实施 A、再实施 B，各自记账；源码稳定后构建和验证，禁止边编译边删文件。更新长期文档实现说明、Spec/Plan、验收和索引；历史结果不改写。初轮不自动提交/推送或创建 PR；用户后续已授权再审计、修复确认问题后提 PR，交付按该授权执行，不合并、部署或发布。验收不通过时如实保留失败和剩余工作。

## 7. 实施前设计审计

2026-11-02，在生产差异为空时复核全部候选及 helper，HAC-00 设计通过。旧采集的构造/发现/分页只在闭包及专属测试中使用；未发现当前 host/application/CLI/storage/正式测试/脚本入口。确认 HistoryBinding/InspectHistoryTurn 及 64 KiB 读取常量三项共享消费者，按第 3 节保留。已经保存固定编码、类型、公开能力及资源摘要基线。无扩大删除的授权或格式变更；实现和运行资格仍待后续验证。

首轮编译暴露上述设计审计漏查 InspectHistoryTurn → MessagePhase 常量的共享引用，wire/application/组件构建失败。已先修订本 Spec/Plan：保留原 phase 声明迁址，恢复 history.go 的实际 import，不能通过删改核验函数来使编译通过。原失败保留，源码冻结后重新执行验证。

组件第二轮编译另发现原测试文件中的 historyFixtureBinding 被真实恢复测试消费。设计审计没有穷尽测试 helper，已收窄删除，迁出原夹具并保留全部恢复断言。夹具原命令参数只用于冻结绑定比较，恢复测试没有原生进程能力且要求校验先于外发，不恢复已退役的 mock 运行链。此前已通过的 application 结果保留；组件修正后重新整组运行，最后重新绑定正式构建记录。

## 8. 实施结果

A/B 生产分别净减 875/111 行，共 986 行；持久格式不变。40 个保留声明、4 个迁址 phase 声明及原共享夹具一致，固定编码 140590 字节前后相同。最终组件 445 顶层和 application 45 顶层通过，公开两场景通过；8 skip/3 无测试包及两轮编译遗漏单列。HAC-00–05 完成，最终构建和产物关联见验收。未提交、部署或发布，无新增完整 L1/L2/L3 资格。

同日[实现再审计](../acceptance/history-adapter-reaudit.md)未发现新增确认缺陷；全部12个变更文件的46个保留声明（含17个类型）一致，6个声明仅迁址。原共享恢复夹具、持久格式及执行/恢复/停止承诺保持，本次44个顶层定向通过；按新增授权提交并创建 PR。
