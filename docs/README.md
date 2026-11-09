# SteerRunner 文档索引

核对日期：2026-11-06；归档核对基线：`dda9a827`。安装、构建和快速开始见[项目 README](../README.md)。

## 使用与运维

| 需要了解 | 入口 |
| --- | --- |
| CLI、配置、服务维护、升级前排空及人工对账 | [CLI 与运维指南](cli-guide.md) |
| 长期能力、可靠性承诺及责任边界 | [产品承诺](spec/steerrunner.spec.md#长期产品承诺)；待实施条款不代表当前行为已改变 |
| 请求、执行、回执、恢复及 PR 交付 | [可靠性交付链路](reliable-delivery.md) |
| Orca 当前执行、结构化会话、历史读取及停止限制 | [Orca 执行链路设计](orca-execution.md) |
| Workflow、阶段与结果获取 | [总览](workflows/README.md) · [普通开发](workflows/development.md) · [带审计开发](workflows/development-audited.md) · [持续对话](workflows/conversation.md) · [一次生成](workflows/generate.md) |
| 运行时汇报及结果文件 | [进度通道](runtime-progress.md) · [结果文件](runtime-result-file.md) |
| 报告、日志及工作区 Git 查询 | [Run 证据读取](run-evidence-r2.md) |

## 开发与验证

| 需要了解 | 入口 |
| --- | --- |
| 配置发现与离线校验 | [独立配置 Spec](spec/independent-configuration.spec.md) / [Plan](archive/2026-11/implementation/spec/independent-configuration.plan.md) |
| 执行配置与原生目录边界 | [执行配置 Spec](archive/2026-11/implementation/spec/native-execution-configuration-boundary.spec.md) · [原生目录 Spec](archive/2026-11/implementation/spec/native-catalog-boundary.spec.md) |
| 双 skill、共享 helper、只读快照和 Viewer | [薄壳 Spec](spec/skill-shell.spec.md) / [Plan](archive/2026-11/implementation/spec/skill-shell.plan.md) |
| 结果提取、可选 parser 及一次补报 | [结果获取 Spec](spec/result-acquisition.spec.md) · [Parser 解耦 Spec](spec/parser-decoupling.spec.md) |
| 内容输入与固定版本仓库调查 | [内容流程 Spec](spec/content-workflows.spec.md) · [仓库调查 Spec](archive/2026-11/implementation/spec/generate-repository.spec.md) |
| 开发权限与宿主已有登录 | [full_access Spec](archive/2026-11/implementation/spec/development-full-access.spec.md) · [宿主认证 Spec](archive/2026-11/implementation/spec/host-native-auth.spec.md) |
| 测试入口、场景、资格与缺口 | [测试指南](testing.md) · [测试清单](test-catalog.md) |
| Workflow 模型、分阶段用量及 SteerLine 参考估价 | [模型与用量契约](model-usage.md) |
| 专项设计、计划及验收 | [当前契约与未结设计](spec/README.md) · [当前验收与未结事项](acceptance/README.md) · [历史实施与验收](archive/2026-11/implementation/README.md)；各目录列表和搜索供完整检索 |

实现状态与实际资格分别以各专项和固定构建验收为准。运行时支持组合由 `capabilities` 发布，字段以[内嵌 schema](../internal/resources/assets/cli.schema.json)和生产校验为准；流程定义见[源码](../internal/execution/workflow)，默认提示见[内嵌资源](../internal/resources/assets/prompts.json)。

## 待验范围与历史材料

- [输入预算 Spec](spec/workflow-input-budget.spec.md) / [Plan](spec/workflow-input-budget.plan.md) / [验收](acceptance/workflow-input-budget.md)：保留真实后续输入与 Runner 完成确认的原资格边界，不因文档整理判定缺口已关闭。
- [历史归档](archive/README.md)：已实施任务、固定构建报告和被取代的方案；[首批清单](archive/2026-11/README.md)及[目录收敛清单](archive/2026-11/implementation/README.md)登记归档原因与当前入口。
- 测试指南只维护运行、选测和资格规则，测试清单只维护注册、覆盖和缺口；旧计数、耗时及交付试验见[精简前快照](archive/2026-11/maintenance/README.md)。
- 长期领域契约和明确未结事项保留在[当前 Spec](spec/README.md)；已实施专项及固定构建报告进入[实施历史](archive/2026-11/implementation/README.md)。归档按材料职责，不表示仍适用的机制失效。

## 维护规则

- 能力及可靠性承诺统一维护在[长期产品承诺](spec/steerrunner.spec.md#长期产品承诺)，专项 Spec/Plan 细化机制，验收记录固定构建事实；变更承诺时同步对应 PC-C/PC-R 项，区分现行与待实施。
- 改变公开行为时同步对应 Spec、Workflow/操作说明、测试清单和验收引用。普通修复在 `acceptance/` 保存独立记录，仅在有导航需求时修改中央索引。
- 测试场景核对[执行目录](../tests/catalog.json)和[唯一 case 定义](../internal/testkit/livetest/cases.json)；focused native 专项单列，不混入通用 L2–L4 计数。
- 一次性任务 Spec/Plan 和固定构建报告在批次结束后归档，长期规则维护在领域契约；旧路径集中登记于[映射](archive/path-map.json)。原始证据或运行诊断实际固定引用的入口保留短导引，其余旧入口移除；正文的日期、构建、失败和未验项保持。
- `acceptance/evidence/` 的原始日志、JSON、快照和说明保留原路径与字节；取证路径、源码摘要和构建值不批量改写。归档不授予新测试或发布资格。
