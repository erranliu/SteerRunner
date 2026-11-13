# 远端 execute/1 本机基线核验

2026-11-13，基线为 SteerRunner `1ace5c63` 加本次变更。配对 Orca 的 `execute/1`（包括 SteerLine 环境检测）在受理时只读检查原本机 checkout、remote URL、基线分支格式和固定 commit 对象，不克隆或 fetch。配对 Orca 仍按固定 commit 创建远端 worktree。

合并前定向验证通过：

- `go test ./internal/application -run '^(TestSRFreezeRemoteStructuredExecutePinsHostAndSealContract|TestRemoteExecuteLocalBaseRejectsInvalidInput|TestSRFreezeBaseBranchRefresh|TestSRCallerPreparedBaseSurvivesTargetRewrite)$' -count=1 -timeout=4m`。覆盖远端 Git 不可达、本机分支落后、缓存目录未创建，以及 remote、分支、checkout 和对象错误；本机缓存的刷新及固定提交行为沿用原回归。
- `go test ./internal/execution/adapters/orcatransport -run '^(TestWorkspaceRepositoryFollowsProject|TestRemoteRepositoryDiscoveryRejectsBeforeCreation|TestRemoteRepositoryCreationRetainsReceiptOnMismatch)$' -count=1 -timeout=3m`。覆盖远端仓库选择及创建回执绑定。
- `git diff --check`。

本次未部署，未重跑真实 308 环境检测。
