# PR 合并提交只读元数据修复

2026-10-17，用户直接授权排查 SteerLine Orca 101 阻塞并跨仓修复。验收归本仓[专属记录](../acceptance/pr-merge-metadata.md)，调用方接线另由 SteerLine 维护。

## 实施

实际 worker 的发布查询复用了 SteerLine 提供的 `gh pr view --json url,state,mergedAt,mergeCommit,headRefName,headRefOid,headRepository,headRepositoryOwner,baseRefName`。受限 broker 已允许创建 PR 和这些普通字段，却缺少 mergeCommit 对象，查询错误返回 403。

只增加目标仓库 PR 的 `mergeCommit { oid }`，不允许任意 commit、history、repository 扩展或 mutation；原 repository scope、变量/fragment/alias、参数与深度限制保持。补同查询正例及跨仓、history、repository、额外参数反例。生命周期/权限/重试规则不变。

后续真机还确认 gh 对同仓 head 使用合法 `owner:branch`，现有 broker 一概拒绝冒号。仅在 createPullRequest 的 repositoryId 已匹配目标仓库后，允许 qualifier 等于目标 owner；仍拒绝其他 owner、空分支及多重冒号。新增正反例，不允许跨仓创建。

## 测试计划

| 项 | 级别/场景 | 环境/命令/时点 | 通过条件与证据 |
| --- | --- | --- | --- |
| PMM-A01 | L1 broker 规则 | 修改后 `go test ./internal/execution/adapters/codex -run 'Test.*(AuthenticationBroker|GraphQL|Gh|GH)' -count=1 -timeout=2m` | 实际字段正例和已有/新增拒绝项通过；本次 .evidence/pr-merge-metadata/ |
| PMM-A02 | Orca 真实接线补验 | 执行主机独立目录从固定 d4969d0+本次两文件 diff 构建 helper；新 SteerLine debug 包固定 SHA；本包任务实现/报告/发布/PR 查询/停止 | 原查询通过；原会话三阶段、实际 Git/PR 事实正确；执行资源停止。证据登记本专属验收及 SteerLine 新包 evidence |

不重复完整 L2/L3：本次只修改 broker 一项只读选择，保留全部身份/权限边界；运行定向 L1 与用户要求的独立产品 live 调试。失败与补验独立登记，不更换已启动包的 helper。
