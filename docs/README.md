# SteerRunner 文档索引

核对日期：2026-11-01；整理基线：`70c19a0`，用量专项增补基线 `4d11148`。构建、启动和 CLI 示例从[项目 README](../README.md)开始。

## 当前使用与开发

| 需要了解 | 入口 |
| --- | --- |
| 详细 CLI 用法、服务维护与状态目录 | [CLI 与运维指南](cli-guide.md) |
| 长期能力、可靠性承诺、责任边界及待实施收敛 | [产品承诺](spec/steerrunner.spec.md#长期产品承诺)；能力/可靠性使用稳定 PC-C/PC-R 编号，待实施条款不代表当前行为已改变 |
| 请求字段、服务与 CLI 生命周期 | [CLI 契约](spec/steerrunner.spec.md#3-一个-exe服务与-cli) |
| 请求、执行、回执、恢复与交付的全链路 | [可靠性交付链路](reliable-delivery.md) |
| 内置流程、阶段及结果获取 | [Workflow 总览](workflows/README.md)、[普通开发](workflows/development.md)、[带审计开发](workflows/development-audited.md)、[持续对话](workflows/conversation.md)、[一次生成](workflows/generate.md) |
| 配置发现、离线校验及内部测试配置 | [独立配置 Spec](spec/independent-configuration.spec.md) / [Plan](spec/independent-configuration.plan.md) |
| 独立使用的双 skill、共享 helper、只读快照及独立 Viewer | [薄壳 Spec](spec/skill-shell.spec.md) / [Plan](spec/skill-shell.plan.md) / [实现与验收](acceptance/skill-shell.md) |
| 开发结果提取、可选 parser、一次补报 | [结果获取 Spec](spec/result-acquisition.spec.md) / [Plan](spec/result-acquisition.plan.md) |
| Parser 独立文本分析与执行链路解耦 | [Spec](spec/parser-decoupling.spec.md) / [Plan](spec/parser-decoupling.plan.md) / [验收](acceptance/parser-decoupling.md)；复用 Codex 登录和 CLI，内存任务，工具禁用/进程管理仅在 parser 内；无持久解析槽、跨重启预算或全局断代 |
| 无仓库的对话与生成输入 | [内容流程 Spec](spec/content-workflows.spec.md) / [Plan](spec/content-workflows.plan.md) |
| 固定 commit 只读调查并生成多文档 | [GEN-REPO Spec](spec/generate-repository.spec.md) / [Plan](spec/generate-repository.plan.md) / [接口与示例](workflows/generate.md#固定仓库版本调查) / [验证](acceptance/generate-repository.md) |
| 历史 broker 设计（现行见宿主认证） | [认证 Spec](spec/sandbox-authentication.spec.md) / [Plan](spec/sandbox-authentication.plan.md) |
| 开发执行统一 full_access、配置迁移及旧 Run | [Spec](spec/development-full-access.spec.md) / [Plan](spec/development-full-access.plan.md) / [实施验证](acceptance/development-full-access.md) / [实施后审计](acceptance/development-full-access-audit.md) |
| 开发认证改用执行主机已有登录（已实施，真机资格单列） | [Spec](spec/host-native-auth.spec.md) / [Plan](spec/host-native-auth.plan.md) / [实施验收](acceptance/host-native-auth.md)；删除 broker/CA/socket/租期，该批 state/v6 的历史证据；现行 state/v8 见停止条件收敛，人工对账沿个人执行器简化 P3，保留新建只读与原生远端链路 |
| 旧 Check 退役与新版本边界 | [删除契约、消费者和验证](acceptance/check-retirement.md) |
| Orca 能力层长期设计与双端逐步统一（待实施） | [长期设计](spec/orca-capability-layer.spec.md)：前置无 helper 可行性核验，按能力/操作迁移；B5 实现候选生产接线并经真实 Runner、离线核验和活动停止验收，B6 最终启用及退役旧链；目标不代表当前支持已改变 |
| Orca 能力层首批 B0 无 helper 原生可行性（已完成） | [Spec](spec/orca-native-capability-baseline.spec.md) / [Plan](spec/orca-native-capability-baseline.plan.md) / [验收](acceptance/orca-native-capability-baseline.md)：双端两轮原输入及原终端清理通过，独立审计确认问题已修复；不改变正式停止合同，不代表真实开发资格 |
| Orca 能力层 B1 原终端查询（验证通过，待审计） | [Spec](spec/orca-terminal-query-capability.spec.md) / [Plan](spec/orca-terminal-query-capability.plan.md) / [验收](acceptance/orca-terminal-query-capability.md)：自有身份值、业务与控制终端共用查询，每页两次身份围栏保持，双端无模型查询通过 |
| Orca 路由、原生查询与后端范围 | [本机/远程连接 Spec](spec/orca-local-remote.spec.md) / [Plan](spec/orca-local-remote.plan.md)、[远端原生 Spec](spec/orca-remote-native.spec.md) / [Plan](spec/orca-remote-native.plan.md)、[通用 Agent Spec](spec/orca-generic-agent.spec.md) / [Plan](spec/orca-generic-agent.plan.md) |
| Orca CLI 排队、读取重试与公开等待状态（2026-10-25） | [Spec](spec/orca-cli-wait-retry.spec.md) / [Plan](spec/orca-cli-wait-retry.plan.md) / [独立验收](acceptance/orca-cli-wait-retry.md)；Runner 只读准备与原操作恢复，调用方 UI 接线另验 |
| Orca 原生多行提交修正（已实施） | [Spec](spec/orca-native-submit.spec.md) / [Plan](spec/orca-native-submit.plan.md) / [原双端验收](acceptance/orca-native-submit-remote-final.md) / [远端重跑](acceptance/orca-native-submit-remote-rerun.md) / [持续控制读取增量](acceptance/orca-native-submit-control-stream.md) / [原保真验收](acceptance/orca-native-submit-integrity.md) / [最新再审计与普通本机接线修正](acceptance/orca-native-submit-reaudit.md)；删除草稿/视口门禁，最新双端各3/3与7170次稳定回归；远端实际模型未独立核验；历史失败和各批资格分别登记 |
| 普通重启、升级排空、人工对账与原创建回执 | [当前 Spec](spec/personal-executor-simplification.spec.md#4-恢复与升级收缩pes-02) / [操作指南](cli-guide.md#人工对账与资源丢失)；[旧换代 Spec](spec/service-generation-recovery.spec.md) / [Plan](spec/service-generation-recovery.plan.md) 保留历史取代说明 |
| 停止、初始化与停止证据范围 | [初始化停止 Plan](spec/initialization-stop.plan.md)、[Orca 停止取证 Plan](spec/orca-shutdown-verification.plan.md) |
| 本机 Orca Codex 停止条件收敛（已实施，专项验收） | [Spec](spec/orca-local-codex-stop-simplification.spec.md) / [Plan](spec/orca-local-codex-stop-simplification.plan.md) / [初轮验收](acceptance/orca-local-codex-stop-simplification.md) / [再审计](acceptance/orca-local-codex-stop-reaudit.md)：登记资源 scope、两类证明门禁删除、实际终止保留；现行 v8，不重释 v7 旧 Run |
| 退出证明生产端与旧预算重建清尾 | [Spec](spec/exit-proof-budget-cleanup.spec.md) / [Plan](spec/exit-proof-budget-cleanup.plan.md) / [初轮验收](acceptance/exit-proof-budget-cleanup.md) / [再审计](acceptance/exit-proof-budget-reaudit.md)：已实施；保留请求绑定、Job/reader 与直接签名，缺 Budget 只拒绝新派发 |
| 报告、日志及工作区 Git 查询 | [R2 操作指南](run-evidence-r2.md)、[Spec](spec/run-evidence-r2.spec.md) / [Plan](spec/run-evidence-r2.plan.md) |
| 运行时单文件汇报、原始字节快照 | [接入指南](runtime-progress.md)、[Spec](spec/runtime-progress.spec.md) / [Plan](spec/runtime-progress.plan.md)、[主线整合验收](acceptance/runtime-progress-integration.md) / [专项历史](acceptance/runtime-progress.md) |
| Run token、独立估价及日志授权 | [用量与费用 Spec](spec/run-usage-and-cost.spec.md) / [Plan](spec/run-usage-and-cost.plan.md)、[实现及现场验收](acceptance/run-usage-and-cost.md) |
| 实例绑定与同快照 observe | [公开读取与受理边界 Plan](spec/reliability-simplification.plan.md) |
| 执行可见性与原会话查看 | [可见性 Plan](spec/execution-observability.plan.md)、[原会话查看 Plan](spec/native-session-viewer.plan.md)；[Orca 最后消息 Spec](spec/orca-last-message.spec.md) / [Plan](spec/orca-last-message.plan.md)，实现结果归入原可见性及远端原生验收 |
| Git 基线、工作区名称及 PR 元数据 | [基线刷新 Plan](spec/repository-base-refresh.plan.md)、[名称前缀 Plan](spec/worktree-name-prefix.plan.md)、[PR 元数据 Plan](spec/pr-merge-metadata.plan.md) |
| 权限资源与原完成证明职责收口 | [Spec](spec/native-proof-boundaries.spec.md) / [Plan](spec/native-proof-boundaries.plan.md) / [验收](acceptance/native-proof-boundaries.md) / [再审计](acceptance/native-proof-reaudit.md)：原生私有知识回归 adapter；初轮448组件/57定向/3公开场景，最终整合38定向及3公开场景通过；原预算夹具随上游修复通过，初轮失败/时限边界保留 |
| 新意图写入边界与展示残留清尾 | [Spec](spec/new-intent-boundary-cleanup.spec.md) / [Plan](spec/new-intent-boundary-cleanup.plan.md) / [验收](acceptance/new-intent-boundary-cleanup.md) / [再审计](acceptance/new-intent-boundary-reaudit.md)：仅收窄四类新构造，保留历史/对账/停止与共享夹具/格式，净减26行；初轮498个组件/application及3公开场景通过，再审计27定向和最新主线3公开场景通过 |
| 零消费者生产入口与孤立夹具清尾 | [Spec](spec/orphan-entry-cleanup.spec.md) / [Plan](spec/orphan-entry-cleanup.plan.md) / [验收](acceptance/orphan-entry-cleanup.md) / [再审计](acceptance/orphan-entry-reaudit.md)：生产净减201行、孤立测试支撑净减333行；共享夹具与持久格式不变，496个组件/application顶层及3个公开场景通过，再审计25个定向通过；首轮沙箱失败单列 |
| 单事件确认与旧环境分支收口 | [Spec](spec/event-ack-cleanup.spec.md) / [Plan](spec/event-ack-cleanup.plan.md) / [初轮验收](acceptance/event-ack-cleanup.md) / [实现再审计](acceptance/event-ack-reaudit.md)：保留原提交语义，双确认收敛及无消费者字段/旧环境模型退役，生产净减105行 |
| 旧历史采集支线与 Orca Backend 清理 | [Spec](spec/history-adapter-cleanup.spec.md) / [Plan](spec/history-adapter-cleanup.plan.md) / [初轮验收](acceptance/history-adapter-cleanup.md) / [实现再审计](acceptance/history-adapter-reaudit.md)：无装配历史采集和旧包装退役，真实 history binding/final/恢复保持，生产净减 986 行 |
| 旧准备/恢复规则与 Lifecycle 接口清理 | [Spec](spec/dormant-lifecycle-cleanup.spec.md) / [Plan](spec/dormant-lifecycle-cleanup.plan.md) / [初轮验收](acceptance/dormant-lifecycle-cleanup.md) / [实现再审计](acceptance/dormant-lifecycle-reaudit.md)：无入口规则及三端口/六字段删除，编码与真实可靠性机制保持，生产净减 688 行 |
| 当前内部执行职责 | [执行边界 Spec](spec/execution-boundary-refactor.spec.md) / [Plan](spec/execution-boundary-refactor.plan.md) |
| 个人多机执行器简化（选定范围及文档收口完成，汇总 PR 待评审） | [Spec](spec/personal-executor-simplification.spec.md) / [Plan](spec/personal-executor-simplification.plan.md) / [P1](acceptance/personal-executor-p1.md) / [P2](acceptance/personal-executor-p2.md) / [P3](acceptance/personal-executor-p3.md) / [P4](acceptance/personal-executor-p4.md)：该批 state/v7，净删除恢复/兼容，人工对账出口；P3 完整 L1/Controlled 与真实最小接线通过；P4 仅收口问答输入，其他候选不展开 |
| 测试入口、场景、资格与缺口 | [测试指南](testing.md)、[测试清单](test-catalog.md)、[验收记录索引](acceptance/README.md) |
| 验证链路解耦与前置结果复用（已实施，首轮限定 L2） | [Spec](spec/validation-reuse.spec.md) / [Plan](spec/validation-reuse.plan.md) / [原验收](acceptance/validation-reuse.md) / [实现审计](acceptance/validation-reuse-audit.md)；L2 新现场完整执行，默认及 L3/L4 链路保留，修复后资格单列 |

## 输入预算整改

- [Spec](spec/workflow-input-budget.spec.md) · [最小实施 Plan](spec/workflow-input-budget.plan.md) · [验收与真实短探针](acceptance/workflow-input-budget.md)：预算与提示词收敛已实现；真实后续输入已接受，Runner 完成确认仍有独立验收缺口。

## 当前契约与历史范围

当前使用 `steerrunner-cli/v1`、state/v8、资源版本 3，内置流程为 development/2、development-audited/2、conversation/1、generate/1、execute/1。开发材料是固定基线 worktree 内的仓库相对路径；内容流程仍冻结 inline 材料。同构建重启保留原授权；升级须由原版本排空，未知副作用有界查询后转人工，显式 resume 只读对账原操作。当前开发流程到 `pr_ready` 后拒绝业务反馈；审计在一次正式执行中修复 P0–P2，无 Spec 审计或自动返修循环。

运行时支持组合由 `capabilities` 发布，字段以[内嵌 schema](../internal/resources/assets/cli.schema.json)和生产校验为准；工作流定义见[源码](../internal/execution/workflow)，默认提示见[内嵌资源](../internal/resources/assets/prompts.json)。支持声明还需要对应构建的验收证据。

已取代的设计、首版迁移计划和旧复测报告见[历史归档](archive/README.md)。保留原文中的日期、构建、失败和未验项，不用文档整理推导功能通过或正式发布。仍有适用契约或待验边界的专项文档继续保留在 `spec/`、`acceptance/`；其中调查基线与历史章节按其标注阅读。

## 维护规则

- 能力及可靠性承诺统一维护在[长期产品承诺](spec/steerrunner.spec.md#长期产品承诺)，专项 Spec/Plan 细化机制，验收记录构建事实；变更承诺时同步对应 PC-C/PC-R 项，并区分现行与待实施，不另建并行的承诺来源。
- 改变公开行为时，同时更新对应 Spec、Workflow/操作说明、测试清单和验收引用；实现状态与实际资格分别登记。
- 测试场景以[执行目录](../tests/catalog.json)及[唯一 case 定义](../internal/testkit/livetest/cases.json)核对；focused native 专项单列，不混入通用 L2–L4 计数。
- 仅当内容整体已被取代或只用于历史溯源时归档，登记原因和当前入口；原路径保留简短导引，保证已有证据引用可继续查阅。
- `acceptance/evidence/` 的原始日志、JSON、快照和说明保留原路径与字节；历史路径或构建值是取证事实，不批量改成当前值。

- [个人执行器 P2 独立交付与验收](acceptance/personal-executor-p2.md)：观测不阻塞业务、首次共享预算、故障/停止/重启；Orca local 与完整 L1 补验已通过。

- [个人执行器 P3 独立交付与验收](acceptance/personal-executor-p3.md)：三类规则删除、生产净减少 558 行；v7、人工对账、完整 L1/Controlled 和三条真实最小链路通过；该报告保留 P3 交付时状态，后续 P4 见下项。

- [个人执行器 P4 独立交付与验收](acceptance/personal-executor-p4.md)：原生问答输入选择 2→1，application 去除私有解码；共享准备保持，采用组件和 3 项公开受控验证，不扩大为完整后端资格。
