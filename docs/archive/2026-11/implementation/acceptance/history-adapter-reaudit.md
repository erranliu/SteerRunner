# 旧历史采集与 Orca Backend 清理：实现再审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。结论：未发现新增确认缺陷，无需追加生产/测试代码修复。审计对象为 main `e2bc0a578047fa38293edbe411b15a386ddd20a4` 加 [Spec](../spec/history-adapter-cleanup.spec.md) / [Plan](../spec/history-adapter-cleanup.plan.md) 实现；685 项源码/构建输入、三份二进制及[初轮验收](history-adapter-cleanup.md)证据摘要全部匹配。本次用户已授权提交并创建 PR；不合并或部署。

## 1. 消费者与共享夹具

重新提取全部 12 个生产/测试变更文件（含两个新文件）的包内声明，按包及方法接收者比较；盘点 107 个删除声明在基线全仓当前模块的文本引用，逐项区分实际调用、定义、同名字段、其他包方法及注释。历史 evidence 独立模块保留原字节，不作为现行消费者。

- NewConversationAdapter/DiscoverHistory/ReadHistory 的构造与调用仍只在退役适配器/专属测试闭包；旧 Orca Backend 没有装配或构造者，旧三个接口没有实际消费者。未发现脚本、CLI、host、application、storage 或正式测试中被删的实际入口。
- 同名 Observe、Probe、InspectWorkspace、Backend、UserAgent、FinalAvailable 等仍有真实使用，分别属于 Engine/Store、进程事实、独立 workspace 函数、直接 Codex Backend、握手字段或公开视图字段；这些实现全部保留，不能按名称重复判定误删。[完整引用盘点](../../../../acceptance/evidence/history-adapter-reaudit-20261102/baseline-reference-files.json)
- 逐个复核被删测试文件的非测试 helper：historyFixtureBinding 已原样迁出，recovery_history_test.go 的调用和全部断言未改。historyFixtureStop/startHistoryFixtureProcess/historyFixtureTurn 只有退役测试消费。
- 迁出夹具中的 `-test.run=^TestConversationHistoryProtocolHelper$` 是原冻结 Command.Args 比较值。现行唯一调用者测试九类错绑定；readRecoveryThread 必须在调用任何原生进程前拒绝，且该测试没有 ProcessHost 能力。因此该字符串不是执行入口，不需恢复旧 mock 或重写原夹具来消除文本引用。
- MessagePhase 原类型/三常量继续供 InspectHistoryTurn 使用；64 KiB 常量继续供真实 history binding 读取使用。初轮两处遗漏的修复均确认有效，无新消费者丢失。

## 2. 持久格式与可靠性承诺

本次比较覆盖全部变更文件，而非仅初轮的三个共享文件：**46 个保留声明完全相同，含17个类型；6个声明只迁址，新增和改写声明均为零**。迁址为 phase 类型/三常量、64 KiB 常量和原共享测试夹具。其余生产差异均是已审计的删除，其他生产文件在 Git 中无差异。[声明比较](../../../../acceptance/evidence/history-adapter-reaudit-20261102/declarations.json)

重新运行固定编码工具，含 HistoryBinding 的 140590 字节仍与实施前基线完全相同，SHA256 `3e1bc96bf664599f27e75b8c3845da2ace8c1b36cf4aa55068f2dbd6efad3cd8`。[身份与编码复核](../../../../acceptance/evidence/history-adapter-reaudit-20261102/identity.json)。结合全部保留声明、未变的存储实现和持久消费者核对，确认 state/v8、user_version=8、Run/Snapshot/Input/Request/Receipt/Operation/profile、历史绑定文件/标签/读取限制、旧动作解码及 Intents/Snapshot.Operations 保持；公开 capabilities/资源摘要和 CLI/Workflow 版本未变。样例相同不单独充当全部格式证明。

| 承诺 | 实际入口复核 |
| --- | --- |
| PC-R01/R02 持久受理与未知零盲重发 | application.worker、operation.Invoke/Settle、原意图/回执保存及 Reconcile 调度未改；保存失败零外发、可能外发后只读恢复保持 |
| PC-R03 原结果归属 | readRecoveryOutcome → InspectHistoryTurn 保留原正文/phase 值；PublicMessage → acceptEvent/acceptMessage、完整 final、原 owner/checkpoint 身份和签名保持 |
| PC-R04/R05 普通重启与有界恢复 | readRecoveryThread 的原绑定校验、OpenHistory/ReadThread、VerifyManualRecovery、CanContinueOriginal/原授权和预算保持；未新增 session/资源重建 |
| PC-R06 撤销与停止 | application.control 仍调用实际 StopLifecycle/ShutdownLifecycle；原 Job/reader 结算、冻结资源范围、本机停止只读对账及迟到创建处理未改 |
| PC-C03/C06/C07、PC-R08 | 当前 conversation Workflow、权限/read_only、公开消息/核心 Observe 及可选观测容错均有实际实现并保留；删除的是未装配采集支线 |

未引入新的依赖、能力开关、迁移器或兼容包装；生产净减仍为986行，不增加提速或组合支持承诺。

## 3. 本次验证和交付

| 本次执行 | 结果 |
| --- | --- |
| Codex `TestRecovery.*`、`TestExecutionOwner.*`、实际服务停止/脱离及 PublicMessage | exit 0，14个顶层、20个子用例通过，0 skip；包括共享夹具的错绑定先拒绝、原停止回合 idle、签名/checkpoint 和独立 owner 存活 |
| application 原操作/人工恢复/观察、保存失败、迟到创建、LocalCodex 停止、服务代次/Workflow 恢复、原 late final | exit 0，30个顶层、21个子用例通过，0 skip |
| 静态和身份 | 全部保留声明与迁址、重新生成编码、685项输入和三份产物及初轮证据摘要通过；最新 main 仍为原基线 |

本次共44个顶层通过，不与初轮重复累加。[命令](../../../../acceptance/evidence/history-adapter-reaudit-20261102/commands.json)及[精确测试名称](../../../../acceptance/evidence/history-adapter-reaudit-20261102/test-summary.json)单列。生产/测试源码与初轮最终验收完全一致，因此继续适用初轮445个组件、45个 application、正式 native/integration live 编译，以及同一 Runner 的两个公开场景；本次没有重跑或重新命名这些完整资格。

最终 Runner SHA 仍为 `bc04dfd98bf3ecb4eb0b3573229b74d90765c3e03942a21c67d923bbb5efe31a`；本次只新增文档/审计证据，无需因其重建。初轮两次编译遗漏、8项 skip/3个无测试包及相同产物关联保持原判，不把专项验证写成完整 L1/L2/L3、全后端或新增真机资格。提交实现、Spec/Plan、验收及证据并创建 PR，不自动合并、部署或发布。
