# Orca 工作区创建回执收敛 Plan

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

按 Spec 实施。实施结果、主线夹具差异及合入前复验见验收记录。 [Spec](orca-workspace-creation-receipt.spec.md) · [验收记录](../acceptance/orca-workspace-creation-receipt.md)

## P0 核对基线

1. 刷新 `main`，在独立分支核对 `workspace.go`、`creation_owner.go` 和已有创建夹具。逐一记录首次 CLI 结果、原 owner 输出、外层 operation 加内层 checkpoint 的来源与现有接受范围。特别记录外层 runtime 与 payload 扩展的优先级、旧本机可选字段、远端仓库 ID，以及失败时保留原字节或重新编码 checkpoint 的既有分支。
2. 确认 `TestSRResourceCreationRecovery` 选中的用例在主线仍能到达要测的创建和恢复断言。测试若被已有终端夹具或环境阻断，记录基线结果，并用现有结构化夹具补足等价故障断言后才判定通过。确认双端 Orca、隔离项目、固定构建入口和 SteerLine 生产排空入口可用。

## P1 实施及受控回归

1. 仅替换两个 CLI 结果解码与绑定调用点。首次使用现有发现结果，恢复使用 `originalCreationPlan` 中的 selector/runtime。保留已保存回执的读取分支及原 owner 调用、查询、授权和公共后置校验。用 diff 确认没有新增持久字段或改变派发分支。
2. 在 Windows 运行 `go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1`。复用 `TestWorkspaceCreationCodecPreservesCheckpoint`、`TestWorkspaceCreationReceiptRecovery`、`TestWorkspaceCreationRecoveryRejectsChangedIdentity`、`TestRemoteRepositoryCreationRetainsReceiptOnMismatch` 和 `TestCreationOwnerUnknownAndForeignEvidenceNeverRedispatch`。补齐现有夹具缺少的双端 CLI 绑定等价断言及未被新代码生成的旧回执样本。核对原字节或摘要、旧字段保留、错配拒绝和恢复零新增写入。
3. 用 `go test ./tests/integration -run '^TestSRResourceCreationRecovery$/^(graceful|receipt-before-commit|wire-loss|owner-crash)$' -count=1` 覆盖原目录重启与创建边界，按 [测试指南](../../../../testing.md) 记录为定向受控。成功恢复须绑定原 worktree 并证明同一创建仅派发一次。回执丢失或原身份不符时记录原未知结果，并证明恢复阶段零创建、零重新发现。复用现有授权撤销和准备恢复负例。出现主线也有的夹具失败时，执行 P0 的对照并记录替代断言及通过证据。

## P2 同一候选双端真机验收

1. 从同一干净源码使用 `scripts/build.ps1` 构建固定写入 EXE，另用不同 `-Version` 标识构建冷读 EXE，记录源码摘要及两份 EXE SHA256。使用同一写入 EXE 在本机和真实配对远端串行各提交一次默认 Codex `execute/1`，使用独立 state-dir 和隔离仓库。复用 [#109 双端验收](https://github.com/erranliu/SteerRunner/blob/9c36f366814da4bbb2b24e68818c01e2b3ce3529/docs/acceptance/orca-agent-session-rpc.md) 的 Runner CLI 操作方式。
2. 记录原 Run/input、创建 operation 和原 owner 计划、派发及结果回执。核对每场一次原 `worktree create`、本机 `path:` 或远端 `id:`、响应 runtime、worktree ID 和 instance、固定 base 和结构化会话绑定。核对可信结果、原会话停止范围 `orca_agent_session`、撤销恢复授权和停止后 `result-file/v1` 封存。真实环境出现未知创建时只核对原操作，缺证据即该场未通过。
3. 用原写入 EXE 正常停服并确认 `not_running`。对两个实际原目录分别使用两份 EXE 调用 SteerLine 的生产 `VerifyUpgradeDrained`。保存原 Run/input 集合、原停止证明及无未决控制的结果，核对冷读前后数据库与停机证明摘要。该步骤只作原目录排空验证。
4. 将构建、用例、受控故障结果和两份原目录证据记入独立 `docs/acceptance/` 文件。证据不足如实记录为阻断，完整发布资格按既有门禁单独记录。
