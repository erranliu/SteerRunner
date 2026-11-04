# 核验已有 Git worktree 的首轮恢复

Runner 恢复原有 Git worktree 且尚未派发业务输入时，继续核对固定 HEAD 和工作区洁净状态。`generate` 和 `conversation` 使用只读内容目录，首轮沿原目录身份和权限流程启动。先前这两个 workflow 也会进入 Git 恢复核验，因目录绑定缺少 Git 分支和基线而报 `incomplete workspace intent`。

## 合并前验证

- `TestContentFirstInputUsesReadOnlyWorkspace` 从 worker 首轮入口验证 `generate` 和 `conversation` 各派发一次原生首轮输入。
- `TestRecoveredWorkspaceChecksOriginalGitBeforeFirstInput` 用重启前落库的 Git worktree 验证三种拒绝派发的情况。等树提交只改变 HEAD，已跟踪文件改动和未跟踪文件分别验证固定 HEAD 下的工作区洁净检查。
- `TestFirstInputReusesOnlyFreshBoundGitPaths` 复验新建 worktree 的原有分支。

上述定向回归通过。通过 `scripts/test-foundation.ps1` 运行公开接口的 `TestSRContentGenerate` 和 `TestSRContentConversation`，两项均通过，覆盖正常文本、JSON 生成与会话多轮续接。未运行完整级别测试。
