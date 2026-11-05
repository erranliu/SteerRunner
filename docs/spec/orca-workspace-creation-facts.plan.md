# Orca 工作区创建事实组装收敛 Plan

已按以下步骤实施，完成等价验收、同候选双端联验、原目录排空及修改审计。[Spec](orca-workspace-creation-facts.spec.md) · [验收记录](../acceptance/orca-workspace-creation-facts.md)

1. 实施前刷新主线，在隔离分支逐项对照 `prepareWorkspace` 与 `finishStructuredWorkspace` 的字段来源、零值、取时位置及组装前后检查。确认首次创建和恢复能走到的原分支，保存受影响包基线。保留两处不同的 Process、State、Implementation 与 disposition 责任，不扩大为回执收尾统一。
2. 在现有 `workspace.go` 增加一个私有纯函数，入参仅项目 ID、原 Target、已核验 worktree、原请求 base 和时间，返回原有两个值类型。替换两处重复组装；Workspace/Target 指针赋给 receipt 的动作仍在各自调用方。检查实际 diff，确认校验、查询、取时、Process/State、编码和错误/confirmed 顺序未改变，未修改其它创建与恢复入口。
3. 用少量独立预期值验证字段等价。原 Target 填入全部非覆盖字段，worktree 与请求分别提供可区分的 HEAD/base；核对 Qualified/Dirty/Tree 零值、分支有/无前缀与固定时间原样返回、原输入值不变。纯函数测试不授予输入合法性，实际入口仍须通过原核验。复用现有调用方夹具核对 receipt 的 Workspace/Target 及两条路径各自的状态/回执，不另建通用测试框架。
4. Windows 执行 `go test ./internal/execution/adapters/orcatransport -count=1`。重点复用 `TestWorkspaceCreationReceiptRecovery`、`TestWorkspaceCreationRecoveryRejectsChangedIdentity`、`TestStructuredWorkspaceRecoversOriginalOwnerOutput`、`TestCreationOwnerUnknownAndForeignEvidenceNeverRedispatch`、`TestWorkspaceCreationCodecPreservesCheckpoint` 和 `TestWorkspaceCreationBindsCLIResult`，核对本机及配对远端、首次/恢复、原字段兼容及零重建。`TestStructuredAtomicCreateAndReadOnlyRecovery` 只补充会话回归，不能替代 prepare_workspace 覆盖。仅对未覆盖的实际分支补断言。
5. 从同一候选构建固定 EXE，沿原 `scripts/test-foundation.ps1 -Pattern '^TestSRResourceCreationRecovery$' -RunnerExe <固定EXE> -OutDir <新目录>` 执行公开受控恢复验收。复用原七个恢复/未知窗口，保存原准备操作、唯一工作区、原回执及零重发/重建证据，不把成功组装解释为可以重新认领创建。
6. 同一候选按原创建回执验收入口，在本机与已有配对远端各执行一次默认 Orca Codex `execute/1`，核对原 selector/runtime/worktree/instance、Workspace/Target、结构化会话及结果。沿原会话关闭、停止后 result-file/v1 封存与 SteerLine 双 EXE 原目录排空门禁结清；取时不要求两次 Run 相等。创建故障由受控矩阵覆盖，不在真实配对上制造替换故障；不安装远端 Pi，不修改 Orca。若环境阻断，记录缺口而不放宽资格或借旧结果代替。
7. 在独立 `docs/acceptance/orca-workspace-creation-facts.md` 记录基线、实际生产 diff、命令、返回字段对照、测试计数、双端同候选及原目录排空证据。审计确认只有一个纯函数和两处替换，发现问题按影响修复复验。区分受影响包、单个 Controlled、真实双端与完整 L1/Controlled、L2/L3 资格；提 PR 前刷新主线并核对实际内容合并状态，保持各发布门禁。

2026-11-05：基线仍为 `a78d87da`。生产只修改 `workspace.go`、`agent_session.go`，提取一个纯函数并替换两处组装（+10/-10）；无身份、状态、回执判定或门禁变更。受影响包 212 个顶层测试、1230 项含子测试通过，原 3 项 opt-in 跳过；公开恢复七个窗口通过。相同源码的固定写入/冷读 EXE 完成本机及配对远端 execute/1，原字段、可信结果、停止后 result-file/v1 与 SteerLine 原目录双 EXE 排空均通过。审计未发现运行逻辑缺陷，未修改 Orca 或 SteerLine。仅取得已记录的专项资格，不声明完整 L1/Controlled 或 L2/L3。
