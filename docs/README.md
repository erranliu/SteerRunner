# SteerRunner 文档索引

核对日期：2026-10-22；整理基线：`70c19a0`，用量专项增补基线 `4d11148`。构建、启动和 CLI 示例从[项目 README](../README.md)开始。

## 当前使用与开发

| 需要了解 | 入口 |
| --- | --- |
| 详细 CLI 用法、服务维护与状态目录 | [CLI 与运维指南](cli-guide.md) |
| 产品职责、请求与 CLI 生命周期 | [产品契约](spec/steerrunner.spec.md) |
| 请求、执行、回执、恢复与交付的全链路 | [可靠性交付链路](reliable-delivery.md) |
| 四种内置流程、阶段及结果获取 | [Workflow 总览](workflows/README.md)、[普通开发](workflows/development.md)、[带审计开发](workflows/development-audited.md)、[持续对话](workflows/conversation.md)、[一次生成](workflows/generate.md) |
| 配置发现、离线校验及内部测试配置 | [独立配置 Spec](spec/independent-configuration.spec.md) / [Plan](spec/independent-configuration.plan.md) |
| 独立使用的双 skill、共享 helper、只读快照及独立 Viewer | [薄壳 Spec](spec/skill-shell.spec.md) / [Plan](spec/skill-shell.plan.md) / [实现与验收](acceptance/skill-shell.md) |
| 开发结果提取、可选 parser、一次补报 | [结果获取 Spec](spec/result-acquisition.spec.md) / [Plan](spec/result-acquisition.plan.md) |
| 无仓库的对话与生成输入 | [内容流程 Spec](spec/content-workflows.spec.md) / [Plan](spec/content-workflows.plan.md) |
| 权限、单仓库认证与环境检查 | [认证 Spec](spec/sandbox-authentication.spec.md) / [Plan](spec/sandbox-authentication.plan.md)、[环境检查 Spec](spec/environment-check.spec.md) / [Plan](spec/environment-check.plan.md) |
| Orca 路由、原生查询与后端范围 | [本机/远程连接 Spec](spec/orca-local-remote.spec.md) / [Plan](spec/orca-local-remote.plan.md)、[远端原生 Spec](spec/orca-remote-native.spec.md) / [Plan](spec/orca-remote-native.plan.md)、[通用 Agent Spec](spec/orca-generic-agent.spec.md) / [Plan](spec/orca-generic-agent.plan.md) |
| Orca CLI 排队、读取重试与公开等待状态（2026-10-25） | [Spec](spec/orca-cli-wait-retry.spec.md) / [Plan](spec/orca-cli-wait-retry.plan.md) / [独立验收](acceptance/orca-cli-wait-retry.md)；Runner 只读准备与原操作恢复，调用方 UI 接线另验 |
| 服务换代、原执行接管与创建回执 | [换代恢复 Spec](spec/service-generation-recovery.spec.md) / [Plan](spec/service-generation-recovery.plan.md) |
| 停止、初始化与停止证据范围 | [初始化停止 Plan](spec/initialization-stop.plan.md)、[Orca 停止取证 Plan](spec/orca-shutdown-verification.plan.md) |
| 报告、日志及工作区 Git 查询 | [R2 操作指南](run-evidence-r2.md)、[Spec](spec/run-evidence-r2.spec.md) / [Plan](spec/run-evidence-r2.plan.md) |
| 运行时单文件汇报、原始字节快照 | [接入指南](runtime-progress.md)、[Spec](spec/runtime-progress.spec.md) / [Plan](spec/runtime-progress.plan.md)、[主线整合验收](acceptance/runtime-progress-integration.md) / [专项历史](acceptance/runtime-progress.md) |
| Run token、独立估价及日志授权 | [用量与费用 Spec](spec/run-usage-and-cost.spec.md) / [Plan](spec/run-usage-and-cost.plan.md)、[实现及现场验收](acceptance/run-usage-and-cost.md) |
| 实例绑定与同快照 observe | [公开读取与受理边界 Plan](spec/reliability-simplification.plan.md) |
| 执行可见性与原会话查看 | [可见性 Plan](spec/execution-observability.plan.md)、[原会话查看 Plan](spec/native-session-viewer.plan.md)；[Orca 最后消息 Spec](spec/orca-last-message.spec.md) / [Plan](spec/orca-last-message.plan.md)，实现结果归入原可见性及远端原生验收 |
| Git 基线、工作区名称及 PR 元数据 | [基线刷新 Plan](spec/repository-base-refresh.plan.md)、[名称前缀 Plan](spec/worktree-name-prefix.plan.md)、[PR 元数据 Plan](spec/pr-merge-metadata.plan.md) |
| 当前内部执行职责 | [执行边界 Spec](spec/execution-boundary-refactor.spec.md) / [Plan](spec/execution-boundary-refactor.plan.md) |
| 测试入口、场景、资格与缺口 | [测试指南](testing.md)、[测试清单](test-catalog.md)、[验收记录索引](acceptance/README.md) |

## 当前契约与历史范围

当前使用 `steerrunner-cli/v1`、state/v4、资源版本 2，内置流程为 development/2、development-audited/2、conversation/1、generate/1。开发材料是固定基线 worktree 内的仓库相对路径；内容流程仍冻结 inline 材料。兼容换代保留原授权，未知副作用只对账。当前开发流程到 `pr_ready` 后拒绝业务反馈；审计在一次正式执行中修复 P0–P2，无 Spec 审计或自动返修循环。

运行时支持组合由 `capabilities` 发布，字段以[内嵌 schema](../internal/resources/assets/cli.schema.json)和生产校验为准；工作流定义见[源码](../internal/execution/workflow)，默认提示见[内嵌资源](../internal/resources/assets/prompts.json)。支持声明还需要对应构建的验收证据。

已取代的设计、首版迁移计划和旧复测报告见[历史归档](archive/README.md)。保留原文中的日期、构建、失败和未验项，不用文档整理推导功能通过或正式发布。仍有适用契约或待验边界的专项文档继续保留在 `spec/`、`acceptance/`；其中调查基线与历史章节按其标注阅读。

## 维护规则

- 改变公开行为时，同时更新对应 Spec、Workflow/操作说明、测试清单和验收引用；实现状态与实际资格分别登记。
- 测试场景以[执行目录](../tests/catalog.json)及[唯一 case 定义](../internal/testkit/livetest/cases.json)核对；focused native 专项单列，不混入通用 L2–L4 计数。
- 仅当内容整体已被取代或只用于历史溯源时归档，登记原因和当前入口；原路径保留简短导引，保证已有证据引用可继续查阅。
- `acceptance/evidence/` 的原始日志、JSON、快照和说明保留原路径与字节；历史路径或构建值是取证事实，不批量改成当前值。
