# Orca 工作区创建事实组装收敛 Spec

以 `main=a78d87da05120b7e6b99981b396d6cecd4d60d92` 为基线。`prepareWorkspace` 的 terminal 收尾与 `finishStructuredWorkspace` 重复组装 `WorkspaceInspection` 和 `ExecutionTarget`。本 PR 仅将这段组装提取为 `orcatransport` 包内的一个私有纯函数，两个调用点在各自原位置使用。两端已共用生命周期，本次直接收益是减少两种收尾路径的重复，不新增传输抽象。

函数接收原项目 ID、原 `b.ExecutionTarget`、已核验的 `native.Worktree`、创建请求的 base 和检查时间，按值返回 `b.WorkspaceInspection`、`b.ExecutionTarget`。可命名为 `workspaceCreationFacts`，放在现有 `workspace.go`。不接收 Lifecycle、context、完整请求或快照，不查询、不校验、不读时钟、不修改入参、不返回错误。原调用点继续以 `time.Now().UTC()` 取得时间后传入，无需时钟接口或新类型。

| 返回事实 | 原样组装规则 |
| --- | --- |
| Workspace | `InspectionScope="revision"`；Repository 来自原项目 ID；Worktree、Head 来自已核验的原 worktree；Branch 仅按原 `strings.TrimPrefix(branch, "refs/heads/")` 去掉一次前缀；Base 来自原创建请求 `BaseCommit`；CheckedAt 原样使用传入时间。 |
| Workspace 其余字段 | Tree 为空、Qualified 和 Dirty 为 false，沿原零值，不补 Git 检查或推断洁净、审计及发布资格。创建时 revision 事实不能替代原 HEAD/tree 检查点。 |
| Target | 从原 Target 值复制，只覆盖 WorktreeID、Branch、BaseRevision，分别使用原 worktree ID 和上述 Workspace 的 Branch、Base。ID、ConnectionID、ExternalRuntime、ExecutionHost 等其它字段保持。 |

调用方仍分别持有以下责任，不移入函数。

- `prepareWorkspace`：原计划、请求归属、仓库 selector/runtime、固定名称/base/HEAD、恢复时 worktree 及原终端身份核验；初始化与运行中终端的区别；原 ProcessIdentity、SessionID/generation、SessionAvailable/WorkerLive、checkpoint 编码、错误与回执判定。
- `finishStructuredWorkspace`：原 worktree 的 runtime/instance/path/base/branch 校验、完整终端清单、原辅助 shell 归属及排序、agentSession 身份；原 ProcessIdentity、State、agentEnvelope 和 confirmed 判定。
- 两端首次创建、原 owner/回执恢复、旧字段兼容、未知结果不补建、派发授权与原超时继续沿现有实现。只读准备可继续原首次派发的条件保持，不因缺回执新增创建或发现。停止、文件封存、原目录排空、HEAD/tree、审计和发布门禁均留在各自原调用方。

生产修改限该私有函数及两处字段组装，不调整两处 Process/State/Implementation 的差异或调用顺序，不改 checkpoint、公开 JSON、错误分类、持久格式及工作流。不修改 Orca、非 Orca Codex，不新增 builder、结果类型、通用 receipt 框架或校验策略。保留 [原创建与恢复合同](https://github.com/erranliu/SteerRunner/blob/a78d87da05120b7e6b99981b396d6cecd4d60d92/docs/spec/orca-workspace-creation-receipt.spec.md) 及 #113/#117/#118 各通道合同。

等价验收对照完整返回值和调用方回执，不只比较新函数与自身生成的预期。使用固定时间和独立预期值核对 Target 非覆盖字段、Workspace 零值、原 base/HEAD 来源、一次前缀处理与入参不变；复用已有本机/配对远端首次创建、原 owner 输出恢复、旧 checkpoint、错身份和未知结果零新增创建/发现的受控矩阵。真实验收复用现有双端 `execute/1` 创建流程及其停止、result-file/v1、原目录排空要求，不新增旁路；资格按 [测试指南](../testing.md) 单列，不追认历史证据。
