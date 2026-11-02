# 旧历史采集支线与 Orca Backend 包装清理：验收

2026-11-02 已完成[实现再审计](history-adapter-reaudit.md)，最新全部声明、消费者及共享测试夹具核对见该报告；以下保留初轮实施时的结果与交付状态。

日期：2026-11-02。结论：按 [Spec](../spec/history-adapter-cleanup.spec.md) / [Plan](../spec/history-adapter-cleanup.plan.md) 设计审计后实施，并修正两处遗漏的共享消费者，HAC-00–05 专项验收通过。基线为已含 #82 的 main `e2bc0a578047fa38293edbe411b15a386ddd20a4` 加本批 dirty 差异；实际源码由[逐文件身份](evidence/history-adapter-cleanup-20261102/source-identity.json)绑定。尚未提交、部署或发布，不继承完整 L1/L2/L3 资格。

## 1. 范围与收益

| 增量 | 生产新增 / 删除 / 净减少 | 实际变化 |
| --- | --- | --- |
| A 旧 Codex 历史采集 | 11 / 886 / 875 行 | 删除无生产装配的发现/分页/游标、旧事件/能力模型及专用映射；保留原 MessagePhase 和 64 KiB 读取常量迁址，保留实际 history binding 和原恢复 |
| B 旧 Orca Backend | 0 / 111 / 111 行 | 删除未构造的包装、三个无消费者接口，以及 A/B 闭包之外无使用的私有能力报告/状态 |

合计生产净减 **986 行**；测试净减 341 行单列，不算生产收益。[分项明细](evidence/history-adapter-cleanup-20261102/production-diff.json)。只主张维护面和重复模型减少，没有同条件执行/测试耗时比较。

未实施 BindingSnapshot 环境分支收口、持久字段/动作退役、双操作表示合并或 usage 租期简化。codex.Backend、真实 Lifecycle/权限/read_only、Orca workspace/native observation、Job/reader、独立 owner/checkpoint 和实际停止均保持。

## 2. 设计与实现审计

生产差异为空时写 Spec/Plan、完成设计审计并捕获编码基线。构造器、接口、候选/helper 的当前模块引用和实际 application/host/CLI/storage/正式 tests/scripts 入口重新核对；历史 evidence 独立模块保留原字节。[初始设计记录](evidence/history-adapter-cleanup-20261102/design-audit.json)保留当时结论及原文档摘要，不改写成已经发现后续遗漏。

确认旧 NewConversationAdapter/DiscoverHistory/ReadHistory 只有专属测试，Orca Backend 没有构造者。旧采集事件/游标没有现行存储根；现行 PublicMessage → acceptEvent/acceptMessage/LastMessage 和内容 Workflow 属于真实独立链路。最终退役符号在现行源码引用为零，没有搬到新包充当替代框架。

保留的实际消费者包括：

- lifecycle.readRecoveryThread/writeHistoryBinding 的原 project/execution/attempt/workflow/session、工作区、home、命令/参数绑定和原 history 文件读写。
- OpenHistory 的 initialize 身份/home 核实、原 thread/read、Close/WritersAbsent 和 Job 实际结算；没有因旧 probe/getter 删除而改握手。
- lifecycle.readRecoveryOutcome → InspectHistoryTurn 的完整原生 final 判据，及同一包内原签名/checkpoint/服务脱离机制。
- 原操作受理、可能外发后只读对账、持久撤销、精确停止、迟到创建/结果、FrozenBudget 和普通重启授权。

设计审计漏掉两处共享消费者，均在编译阶段暴露并先收窄设计后修复：

1. InspectHistoryTurn 使用 evidence 中的 MessagePhase 常量。第一轮 wire/application/组件构建失败；原类型和 commentary/final_answer/unknown 三常量迁入 message_phase.go，恢复实际 import，核验函数不改。
2. recovery_history_test 调用旧适配测试文件中的 historyFixtureBinding。组件第二轮编译失败；原 helper 迁到 history_binding_fixture_test.go，恢复断言全部保留。夹具中的原 mock 命令是冻结比较值，测试没有进程能力且要求原绑定校验先于外发；没有重新接入旧 mock 协议。[夹具比较](evidence/history-adapter-cleanup-20261102/fixture-audit.json)

原失败 JSONL 和摘记归档；未通过删除真实恢复测试取得通过。

## 3. 持久格式与保留声明

state/v8、SQLite user_version=8、CLI/Workflow 和原生协议不变。Run/Snapshot/Input/Request/Receipt/Operation、opaque profile、Host/Probe/Delivery、旧动作解码、Intents/Snapshot.Operations 的持久形状和实际编码逻辑未改；HistoryBinding 的字段、Validate/Identity、Write/Read 及 64 KiB 文件读取界限保持。

固定捕获含代表性 Run/snapshot/request/receipt/operation、HistoryBinding 编码和选定持久类型定义、公开 capabilities/资源摘要，前后 **140590 字节完全相同**，SHA256 `3e1bc96bf664599f27e75b8c3845da2ace8c1b36cf4aa55068f2dbd6efad3cd8`。[编码比较](evidence/history-adapter-cleanup-20261102/wire-comparison.json)

三个共享生产文件的 **40 个保留声明完全一致（含 16 个类型）**，额外迁出的 **4 个 MessagePhase 声明**与原声明一致；原 64 KiB 常量只迁址。其余生产文件要么是已核实的整份死闭包，要么不在 Git 差异中；没有修改其他有效实现。[声明比较](evidence/history-adapter-cleanup-20261102/retained-audit.json)。固定样例不是完整格式证明，消费者/持久根和全部保留声明审计共同支持格式结论。

## 4. 最小验证与产物

| 验证 | 最终结果 |
| --- | --- |
| execution/... + runstate/contracts/resources/storage 整组 | 单条命令 exit 0，445 顶层、1340 子用例通过；8 个 opt-in/helper skip、3 个无测试包单列 |
| application 定向 | exit 0，45 顶层、77 子用例通过、0 skip；保存失败零外发、原操作/人工/观察、预算、迟到 final/创建、本机停止只读对账、重启授权/问题来源及内容恢复 |
| 正式 live 编译 | native/integration 各 `go test -c -tags live` 通过，不以 TestMain 零匹配作为验收 |
| 正式最终 Runner 的公开链路 | TestSRNativeUnknownAndStop、TestSRRestartPreservesAuthorization，2/2 场景、4 子用例通过，0 skip；包含 graceful/kill/creation_receipt/pending_stop，成功结果包含隔离夹具清理 |
| 源码/格式审计 | 上述保留声明、原夹具、原编码及公开资源比较通过；最终 685 项源码/构建输入与产物逐文件固定 |

组件加 application 共 **490 个顶层通过**，各次重试不重复累加。[精确数量/名称/skip](evidence/history-adapter-cleanup-20261102/test-summary.json)、[命令与退出码](evidence/history-adapter-cleanup-20261102/commands.json)。首次构建失败及组件第二次失败保持失败，未将其他包的通过改判成整组通过。

最终 Runner 由 scripts/build.ps1 构建，SHA256 `bc04dfd98bf3ecb4eb0b3573229b74d90765c3e03942a21c67d923bbb5efe31a`，source_digest `27dfbf4af085f233d1d01a4ac59aeb1bf3c6c722ea667191107d57901ad9f2eb`。[最终构建](evidence/history-adapter-cleanup-20261102/runner-build.json)

公开两场景通过后仅迁出共享测试夹具，生产源码未再变；正式重建的 Runner SHA 与已测产物相同，原场景适用于最终相同字节，不重复执行或累加。最终源码绑定和重建记录同时保留，[产物关联](evidence/history-adapter-cleanup-20261102/artifact-binding.json)明确前后 source_digest；正式 live 编译已在夹具迁址后再次完成。application 生产依赖也未变化，其已通过结果保留。

无新增真实模型/helper、全后端或完整 L1/L2/L3 资格。此次清理不调整长期产品承诺，不操作用户已有 Run 或 Orca worktree；提交/PR/部署仍不属于本轮自动动作。
