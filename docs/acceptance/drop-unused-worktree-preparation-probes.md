# 本机 worktree 准备失败路径精简验收

2026-11-11，基于 `d455d30a43c7733b5485290c6343c483ab913ef3` 的合并前定向验收。

`workspace.Prepare` 的唯一生产调用方是 Codex 的 `prepareWorkspace`。它总是读取 `Status`，只在 `confirmed` 时传递 `Inspection`。普通 checkout 失败后的 `observePreparation` 只为无人采纳的返回值查询 worktree 清单、分支及可选检查结果。本次删除该探测及 `PathCreated`、`WorktreeRegistered`、`BranchPresent` 三个内部字段。

普通 Git 失败仍返回原错误和 `unknown`，保留原目录。创建前查重、成功后的 revision 回执、超时拒绝和剩余准备预算保持。丢回执后仍由 `InspectPrepared` 只读核对原 HEAD、正式索引、索引锁和干净工作树。首条输入前的 `VerifyInitialClean` 继续独立核验。[原检出预算与恢复验收](local-worktree-checkout-budget.md)

## 实际执行

Windows/amd64、PowerShell 7。原测试在删除前通过。强化后的既有失败子例在旧实现上按预期失败，记录到 `worktree list --porcelain -z` 和 `for-each-ref` 两次追加查询。删除后该子例通过，断言原 Git 退出错误及诊断、`unknown`、原目录存在、无成功回执和零追加 Git 调用。

最终 7 项顶层、12 个执行节点通过，无失败或跳过。执行节点含顶层与子例。定向覆盖成功 revision 回执、LFS 检出预算、部分检出拒绝、丢回执只读恢复和首条输入前核验。

```powershell
go test ./internal/execution/workspace -run '^(TestLocalWorkspacePrepareInspectResourcesAndCleanup|TestPrepareDoesNotRunCheckoutHookOrRecheckGit)$' -count=1 -timeout=5m -json
go test ./internal/execution/adapters/codex -run '^(TestWorkspacePreparationWaitsForLFSSmudgeWithinOriginalBudget|TestWorkspaceLifecyclePrecedesSessionAndRejectsRetargeting)$' -count=1 -timeout=5m -json
go test ./internal/application -run '^(TestLocalCheckoutUsesSavedPreparationDeadlineAndBlocksInputOnFailure|TestRecoveredWorkspaceChecksOriginalGitBeforeFirstInput|TestSROperationReceiptCommitFailureReconcilesOriginal)$' -count=1 -timeout=5m -json
./scripts/build.ps1 -OutFile .evidence/drop-unused-worktree-preparation-probes/final/steerrunner.exe
```

正式构建通过，构建记录与通过点输入及实际 EXE 一致。构建时 `source_dirty=true`，`source_digest=9de9292351c616d914e811ed46a7b9886eaa1978891732f8c56e49414745a1ff`，EXE SHA256 为 `e80340120ddcacddba0747a2d783834b31c7f5ebafb5f7039c6b04e9001a1fe9`。原始输出在本工作副本的 `.evidence/drop-unused-worktree-preparation-probes/`。

本次验收限于定向验证和正式构建。完整 L1–L4、Controlled 和真实模型场景未运行。
