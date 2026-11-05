# 固定 Git 审计读取解耦 Plan

按 Spec 实施。本次只交付文档，以下为待执行步骤。 [Spec](workspace-fixed-git-audit.spec.md)

## P0 对照基线

1. 刷新主线，在独立分支盘点 `codex/remote_workspace.go`、`orcatransport/workspace_audit.go`、`workspace/audit.go` 和 `application/workspace_source.go`。记录 Git 读取命令及顺序、前后身份检查、可选输入、错误与返回事实。区分 Codex 早期拒绝的默认返回值与实际 Git 读取的结清结果。
2. 核对现有 `settleRemoteTree` 的读者、history 和运行 owner 调用。仅在 workspace 中承接固定 Git 读取需要的进程树证明，沿用 `ProcessTreeHandle`、`process.Probe` 等现有原语及基线顺序。原 Codex 停止入口和调用图保持。验证复用现有路径比较时的有效输入集合与基线一致。
3. 核实本机 Orca audited 原证据通道、执行和审计角色模型配置、隔离仓库与测试 PR 权限。核实非 Orca Codex 的 `local-codex.audited` 场景、两种不同的模型及固定 EXE 构建入口。无法取得正式证据时记录具体阻断。

## P1 小步提取与受控回归

1. 在 `execution/workspace` 的审计文件内提取固定读取，沿用现有结果类型。替换结构化 Orca 和 Codex 远程控制审计对旧读取函数的调用。保留 Codex wire 查询校验、原 owner/readiness 校验、远程证明验证，以及 Orca 读前读后检查。检查依赖方向，workspace 只使用已有底层组件。若需要改造运行 owner 停止或直接 Codex 完成检查点，收窄改动并重新核对边界。
2. 运行 `go test ./internal/execution/workspace ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -count=1`。复用 `TestRAWorkspaceSource`、`TestRemoteWorkspaceAuditUsesOriginalGitOwner`、`TestRemoteWorkspaceEvidenceCannotPromoteRevisionOnlyOrForeignFacts`、`TestStructuredWorkspaceAuditReadsOriginalChangedTree` 和 `TestFixedWorkspaceAuditUsesFreshOriginalControlAndReconcilesPendingRead`。应用层按影响复验原工作区 owner、无 caller Git fallback、原完成检查点和祖先检查。
3. 先核对既有负例，仅补未覆盖的差异。重点核验固定 Git 调用与基线等价、读取中 HEAD 变动拒绝、进程树证据缺失时取消资格、仅触及本次 Git 读取器，以及读后 Orca worktree 身份改变时拒绝。继续覆盖错误 URL/common-dir、路径、分支、base、祖先、dirty、sparse 和隐蔽索引状态。适配器的结果拒绝和结清范围不得变化。

## P2 固定候选验收

1. 用 `scripts/build.ps1` 从干净源码构建固定写入 EXE，记录提交、源码摘要和 EXE SHA256。用同一候选串行执行以下两场。测试入口和资格按现有要求。 [测试指南](../testing.md)

   | 场景 | 通过条件 |
   | --- | --- |
   | 本机 Orca Codex audited | 沿 #107 专项验收入口，在隔离仓库产生真实实施和审计修复，原 pre/post HEAD/tree 可核对且正数修复确有 tree 变化。核对原会话的执行、审计、切回实际 model/effort、原临时授权撤销、发布前原 HEAD/tree 及真实 PR repo/base/head。显式停止，按现有合同验证原会话和 `result-file` 快照。正常停服后用同源码的独立冷读 EXE，走 SteerLine 生产 `VerifyUpgradeDrained` 冷读原目录。 |
   | 非 Orca Codex audited | 复用现有 `local-codex.audited` 场景和受支持的双模型配置。核对原完成检查点、发布绑定及原进程停止证明。按 `docs/testing.md` 使用固定候选的 `-Focused -Level L3` 入口记录专项，完整发布资格继续按原门禁。 |

2. 用独立 `docs/acceptance/` 记录构建身份、检查点、真实 PR、原停止证明及阻断。历史证据仅作操作参考。现行门禁缺证据时该场未通过。 [#107 本机 audited 正式验收](https://github.com/erranliu/SteerRunner/blob/5275d8807f00e3b0dbb38ec9d016a2da615f1d9d/docs/acceptance/orca-helper-retirement.md)
