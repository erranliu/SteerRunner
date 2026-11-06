# 固定 Git 审计读取解耦 Spec

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

以 #110 合入后的 `main`（`5275d880`）为基线。本机结构化 Orca audited 通过 Codex 适配器的 `InspectOwnedWorkspaceAudit` 运行固定 Git 读取。将这段共用读取及其 HEAD/tree 检查下沉至现有 `internal/execution/workspace`。本次切断的是结构化 Orca audited 对 Codex Git 读取实现的调用。

## 共用范围

入口接收调用方已绑定的原路径、仓库 URL、remote 名称、`WorkspaceIntent`、可选 `preHead` 和既有受管进程宿主。复用 `workspace.Git`、`ResolveRepository` 和 `InspectAudit`，返回既有 `WorkspaceInspection`、本次 Git 读取器是否结清及错误。现有远程查询的 JSON 类型留在 Codex 适配器。若抽取查询参数校验，两个入口继续执行原检查，Codex 入口的校验时机和错误保持。

- 保持 Git 参数、命令顺序、路径、超时、输出上限和错误分类。保留 `--no-optional-locks`、禁用 fsmonitor 和 untracked cache 的设置，以及现有非交互执行。调用方提供的路径只用于原工作区检查，入口不接受任意 argv。
- 保留现有路径等价、branch、base、可选 `preHead` 和提交格式规则。共用入口沿现有规则校验查询中的路径与调用方传入的原路径一致。这项参数校验不构成工作区归属证明。继续校验仓库 URL、Git common-dir、干净状态、sparse checkout、隐藏索引标志及 gitlink，读取原 HEAD/tree，检查祖先关系和读取期间 HEAD 一致性。沿用既有检查顺序与失败时的事实字段。
- 读取器结清仍是 Git 检查合格条件。只结清本次创建的 Git 进程树，保留原 handle 内的进程树核对、原 PID 身份缺席、期限、轮询与关闭顺序。无法证明时返回原读取器错误并清除 `Qualified`。这项结清证明只覆盖 Git 读取器，会话、Codex 工作进程、Orca shell 及升级排空仍使用原门禁。

## 调用方边界

| 调用方 | 留在原位置的责任 |
| --- | --- |
| 本机结构化 Orca audited | 原 runtime、worktree ID、instance、路径和原会话状态的核验。继续在原 inspection gate 下，读取前后用 `originalAgentWorkspace` 核对原工作区。关闭已开始、工作区换代或读取器未结清时仍拒绝。 |
| Codex 远程控制审计入口 | 保留 `RemoteWorkspaceAuditQuery`、readiness、session、宿主进程、运行时摘要和配置身份校验，保留 `ValidateRemoteWorkspaceAudit` 及 `WritersAbsent` 的现行公开字段和含义。既有 Orca 历史控制兼容分支仍按原路由和 wire 处理。 |
| 非 Orca Codex 执行与 Workflow | 当前直接调用 `workspace.InspectAudit` 的路径继续使用原冻结仓库、原进程宿主及检查点事务。保留实际 model/effort、角色切回、原完成检查点的来源与恢复规则，以及 HEAD=post_head、真实 PR repo/base/head、授权、停止和排空判断。 |

生产改动仅限共用读取和两个调用点。读取器的清理与失败传播随固定读取处理，现有 Codex 会话、history、owner 的停止调用留在原处。无新接口层、运行时注册表或通用进程结算框架。不修改 Orca，不新增远端 Orca audited 准入，不修改 wire、持久格式、Workflow 或非 Orca Codex 行为。

## 验收

固定读取的成功事实、失败类别和进程退出证明与基线等价。用受控测试验证错仓库和工作区身份、无效分支或提交、脏树、HEAD 竞争、读取器未结清及旧控制回执。核对读取前后 HEAD、index 与原 owner 状态，确认 Git 读取不改变工作区。

同一候选验证本机 Orca audited 与非 Orca Codex audited。Orca 场须取得原实施和审计修复的 HEAD/tree、切回证据及真实 PR head 核对，显式停止后保留原会话证明和原目录排空。非 Orca Codex 沿已有 audited 验收确认原检查点、发布和停止。真机结果与组件、受控和完整发布资格分别记录。
