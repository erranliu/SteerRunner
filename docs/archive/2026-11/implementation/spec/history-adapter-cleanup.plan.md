# Plan：旧历史采集支线与 Orca Backend 包装清理

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：P0–P4 已完成，见[验收](../acceptance/history-adapter-cleanup.md)。依据 [Spec](history-adapter-cleanup.spec.md) 和[长期承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)。

## P0：设计和基线

- [x] 冻结最新 main、干净工作树及 A/B 文件/符号清单，区分历史独立模块、专属测试与实际入口。
- [x] 审计设计：确认旧采集无接线，HistoryBinding/InspectHistoryTurn 有真实消费者，64 KiB 原读取常量须迁址；发现反例先修订文档。
- [x] 在生产未改时捕获持久类型/代表性编码（含 HistoryBinding）、公开能力及资源摘要；保存消费者和设计审计记录。

## P1：A——旧 Codex 历史采集闭包

1. 删除无入口主体及 conversation_adapter_test/final_history_test 的专属测试；historyFixtureBinding 原夹具迁到独立测试文件，保留真实恢复断言；evidence 原 MessagePhase 类型/三常量迁到 message_phase.go 保留。
2. history.go 只删 Spec 指定专用分页/事件映射、无消费者 probe/getter/helper，保留实际 phase import；HistoryBinding 只删 Event，移入原 64 KiB 常量，去掉必要 import。
3. 保留真实 history binding 读写/编码及原恢复、final、Job/reader；逐个复核共享 helper，生产 A 独立记账。

## P2：B——旧 Orca Backend 包装与接口

1. 删除未构造的 orcatransport/backend.go，只删 backend/backend.go 的三个旧接口。
2. A 完成后删除已无消费者的 backend/capability.go；保留 StopReceipt、ProcessIdentity/host ports、实际 Lifecycle、公开资源能力和通用持久类型。
3. B 独立记账。不扩大到 BindingSnapshot 环境分支、持久动作或双操作表示。

## P3：验证和实现审计

- [x] 全仓引用复查、全部保留声明 AST 对比及前后编码逐字相等。
- [x] execution/...、runstate/contracts/resources/storage 组件及 application Spec 定向；登记精确名称、数量、skip 和失败，不拿零匹配通过。
- [x] native/integration 正式 live 编译；正式脚本构建最终 Runner，记录稳定源码和产物 SHA。
- [x] 同一最终 Runner 运行两个公开 foundation 场景，确认数量、退出码、无 skip 和夹具清理。
- [x] 对照长期承诺复核有效消费者、持久格式、原执行/恢复/停止；确认问题先修复，再补必要定向验证。

## P4：交付

- [x] 更新长期实现说明、文档/验收索引、Spec/Plan 状态和专项验收，保存最小可复核证据与摘要。
- [x] 核对 diff、链接和证据；报告实际收益与未验范围，初轮不自动提交/推送、创建 PR 或部署。后续用户已授权再审计后提 PR，按该授权交付。

停止条件：HAC-00–05 全部满足；候选没有实际消费者、共享消费者及持久编码不变，所有指定运行验证通过。任何编码或合法入口反例先收窄本批范围，不引入替代框架。

首轮失败修订：设计漏查了 InspectHistoryTurn 的 MessagePhase 常量消费者，已收窄 P1 为原声明迁址；保留首次构建诊断，修复后重跑整组和原编码，不改实际 final 核验正文。

组件第二轮发现 historyFixtureBinding 共享测试消费者，按原夹具迁址，恢复测试不删；组件重新整组验证。application 已通过且生产源码未再改，不重复累加；正式构建与最后的测试源码重新绑定。

实际结果：A/B 生产净减 875/111 行；共享 phase 和恢复测试夹具迁址保留，固定编码/持久形状不变。组件 445 顶层及 application 45 顶层通过，公开场景 2/2；8 skip/3 无测试包、两轮编译失败及相同产物关联均归档。未提交或部署。

后续[实现再审计](../acceptance/history-adapter-reaudit.md)已完成：无新增确认缺陷，补充全部保留声明/夹具/消费者及编码复核，44个定向顶层通过；原生产/测试源码和初轮证据不变。按新增授权提交并创建 PR，不合并或部署。
