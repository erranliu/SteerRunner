# 当前契约与未结设计

核对日期：2026-11-06。本目录只保留长期维护的领域契约和仍有明确未结事项的设计。已实施任务的 Spec/Plan 进入[实施历史](../archive/2026-11/implementation/README.md)；旧路径可通过[集中映射](../archive/path-map.json)检索。归档不表示机制退役，原实施与验证结论仍按原构建阅读。

| 职责 | 当前文档 |
| --- | --- |
| 长期产品承诺、CLI 与可靠性边界 | [SteerRunner](steerrunner.spec.md) |
| Orca 能力层长期职责 | [能力层设计](orca-capability-layer.spec.md)；运行与停止使用[现行 Orca 指南](../orca-execution.md) |
| 配置发现、冻结与校验 | [独立配置](independent-configuration.spec.md) |
| 开发结果获取与来源核验 | [结果获取](result-acquisition.spec.md) |
| 独立文本 parser | [解析边界](parser-decoupling.spec.md) |
| 内容流程 | [生成/对话](content-workflows.spec.md) |
| skill、helper 与 Viewer | [薄壳交付](skill-shell.spec.md) |
| 持续进度与结束封存 | [Progress](runtime-progress.spec.md) · [Result-file](runtime-result-file.spec.md) |
| 输入预算及后续完成确认未结事项 | [Spec](workflow-input-budget.spec.md) · [Plan](workflow-input-budget.plan.md) · [未结记录](../acceptance/workflow-input-budget.md) |
| Orca CLI 等待与完整资格未结事项 | [Spec](orca-cli-wait-retry.spec.md) · [Plan](orca-cli-wait-retry.plan.md) · [原阻断](../acceptance/orca-cli-wait-retry.md) |
| Codex 旧配置规则退役（2026-11-10 已实施，定向验收） | [Spec](codex-configuration-rule-retirement.spec.md) · [Plan](codex-configuration-rule-retirement.plan.md) · [验收](../acceptance/codex-configuration-rule-retirement.md) |
| Orca 旧终端测试与当前执行器分离（2026-11-10 已实施，定向验收） | [Spec](orca-terminal-fixture-separation.spec.md) · [Plan](orca-terminal-fixture-separation.plan.md) · [验收](../acceptance/orca-terminal-fixture-separation.md) |
| 应用层旧阶段重建规则退役（2026-11-10 已实施，定向验收） | [Spec](application-pending-stage-retirement.spec.md) · [Plan](application-pending-stage-retirement.plan.md) · [验收](../acceptance/application-pending-stage-retirement.md) |
| 零消费者 checkpoint／错误声明清尾（2026-11-10 设计，待实施） | [Spec](unused-checkpoint-error-cleanup.spec.md) · [Plan](unused-checkpoint-error-cleanup.plan.md) |

长期契约及未结设计之外，目录中的归档导引只为原始证据固定引用保留，不作为当前契约。新任务材料仍按工作流写入 `docs/spec/`，完成实施后将一次性 Spec/Plan 归档，长期规则只在相应领域契约中维护。上述 Codex 退役、Orca 测试分离、应用层旧阶段重建规则退役及零消费者声明清尾专项于 2026-11-10 新增，其余索引核对日期保持如上。
