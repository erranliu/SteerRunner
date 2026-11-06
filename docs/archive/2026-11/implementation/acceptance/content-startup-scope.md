# 核验已有 Git worktree 的首轮恢复

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

Runner 恢复原有 Git worktree 且尚未派发业务输入时，继续核对固定 HEAD 和工作区洁净状态。`generate` 和 `conversation` 使用只读内容目录，首轮沿原目录身份和权限流程启动。先前这两个 workflow 也会进入 Git 恢复核验，因目录绑定缺少 Git 分支和基线而报 `incomplete workspace intent`。

## 合并前验证

- `TestContentFirstInputUsesReadOnlyWorkspace` 从 worker 首轮入口验证 `generate` 和 `conversation` 各派发一次原生首轮输入。
- `TestRecoveredWorkspaceChecksOriginalGitBeforeFirstInput` 用重启前落库的 Git worktree 验证三种拒绝派发的情况。等树提交只改变 HEAD，已跟踪文件改动和未跟踪文件分别验证固定 HEAD 下的工作区洁净检查。
- `TestFirstInputReusesOnlyFreshBoundGitPaths` 复验新建 worktree 的原有分支。

上述定向回归通过。通过 `scripts/test-foundation.ps1` 运行公开接口的 `TestSRContentGenerate` 和 `TestSRContentConversation`，两项均通过，覆盖正常文本、JSON 生成与会话多轮续接。未运行完整级别测试。
