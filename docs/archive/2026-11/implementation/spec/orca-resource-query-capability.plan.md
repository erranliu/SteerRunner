# Plan：Orca 仓库与工作区查询 B4a

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：已完成，受控验证及实施后独立审计通过。依据 [Spec](orca-resource-query-capability.spec.md) 和[长期设计](../../../../spec/orca-capability-layer.spec.md)。B3 审计确认问题先独立修复，不混入本批。

## P0 现行范围

固定 workspace.go 的 workspaceRepository/prepareWorkspace reconcile、repository_query.go 的 queryRepositoryInventory，以及原 Worktree/creation checkpoint 字段。白名单只新增无执行依赖的资源 query 能力及这三处窄接线，不改 queryPreparation 算法、创建/选择/基线策略或持久类型。

## P1 能力与接线

1. 在协议包资源查询文件定义 Repository/RepositoryInventory，提供纯 RepositoryListArgs 和 DecodeRepositoryInventory。只解码原 repos(ID/path/kind)/truncated 范围并保留本地解析完整性；其他原生字段仍在 Result.Payload，不引入新类型前置。
2. Worktree 类型移到同一资源文件而不改声明/字段编码；提供 Client.ShowWorktree 的单次原 MetaCall、明确非空 selector 与统一解码。返回原 Result 和 worktree/解析事实，消费方继续比对全部原身份。
3. queryRepositoryInventory 继续调用原 queryPreparation，但参数来自能力；workspaceRepository 消费能力 inventory，原错误/截断/runtime/目录匹配/无匹配/歧义规则保持。
4. reconcile 接入 ShowWorktree，保留原查询错误分支、created.Worktree 比较及所有后续原终端判据。删除两处对应专属解码和参数，不保留并行 fallback。

## P2 验证

新增 TestOrcaResourceQueries：无执行快照，验证原 repo argv、ID/path/kind/截断/缺失/坏字段范围，以及 worktree show 精确 selector、全部字段和 runtime、调用错误/不确定结果、缺 selector 零调用；不使用新构造器生成旧参数 oracle。

复跑原 TestWorkspaceRepositoryFollowsProject、TestRemoteRepositoryDiscoveryRejectsBeforeCreation、TestRemoteRepositoryCreationRetainsReceiptOnMismatch、TestRepositoryQuery*、TestPreparation*、TestCreationOwner* 和 TestWorkspaceCreation*。保留原恢复零再发现/零重建、队列/预算/撤销、精确 selector/base/runtime 与回执计数。需要时使用正常权限原进程/锁夹具，不操作真实 Orca。

go test -json 记录真实 roots/子节点/fail/skip；新功能先跑再必要回归，workflow 依赖守卫、git diff --check、旧参数/结构删除及原持久声明 diff 必须通过。

## P3 收口

按 ORQ-01–04 写 docs/acceptance/orca-resource-query-capability.md，列实际两个查询、消费者、删除清单和未迁范围；同步路线/索引。提交实现后独立只读审计一次，确认修复独立 commit 并补受影响验证，不提阶段 PR。全部闭环才标完成。

实施记录：已接入两个真实查询消费者，原策略/预算/持久格式保持；验证与删除清单见[验收](../acceptance/orca-resource-query-capability.md)。
