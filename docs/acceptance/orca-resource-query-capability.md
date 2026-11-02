# Orca 仓库与工作区查询 B4a 验收

日期：2026-11-02。状态：实现、受控回归及实施后独立审计通过。依据 [Spec](../spec/orca-resource-query-capability.spec.md) / [Plan](../spec/orca-resource-query-capability.plan.md)。

## 职责和接线

protocol/orca/resource_query.go 提供 Repository/RepositoryInventory、纯 repo-list 参数和解码，以及单次 ShowWorktree。Worktree 声明从 client.go 原样移动，字段及 JSON 编码不变；能力不选择 Git 仓库、不解释固定基线、不处理创建恢复。

实际 queryRepositoryInventory 组合能力参数与原 queryPreparation；workspaceRepository 消费统一 inventory，保留原本机 status/path、远端明确 inventory/id、目录 basename/大小写、Git 筛选及缺 ID/无匹配/歧义/截断拒绝。prepareWorkspace 的 reconcile 消费 ShowWorktree，保留全部原 worktree 字段和 runtime 比较。对应参数和专属解析结构已删除。

原准备查询算法、白名单、90 秒/5 次预算、CLI 队列及派发前回调、撤销和原 creation owner 不变。Worktree/创建 checkpoint/Profile/State/Target 持久声明没有修改。没有新增 repo show/worktree list 或备用查询，也不重新发现/创建未知资源。

## 直接验证

源码基线 `1352885` 加本批差异；source_digest `d2ed831170cb1fcfa27fccc3fbba062f09d64791045ebbea8e354b942cce47da`。实际命令、源码身份及原始日志见[证据](evidence/orca-resource-query-capability-20261102/README.md)。

| 验收 | 结果 |
| --- | --- |
| ORQ-01 | 新能力 1 根/17 子节点、15 叶例通过；原 argv、全部原资源字段、截断、缺失/坏字段、忽略未消费扩展、原错误/响应丢失和非空 selector 零调用验证通过 |
| ORQ-02–03 | 原消费者/恢复/预算/撤销/owner 24 根/77 子节点、86 叶例通过，0 fail/skip；包括原仓库歧义/缺 ID/截断拒绝、恢复精确身份和零再发现/重建 |
| ORQ-04 | workflow 依赖守卫 1 根通过；原持久声明及依赖 diff、git diff --check 通过；独立审计通过 |

子节点含分组节点，叶例另计。没有新真实 Orca 资源或模型场次；这是纯查询结构等价验证，不计 B5 真实 Runner 资格。原停止 scope、输入和证据合同不变。

## 独立审计

固定提交 `d949e1c` 相对 `1352885` 的独立只读审计通过，未发现确认的 P0/P1/P2；原 Worktree 声明、参数、预算、策略及未知恢复保持。审计者复核 5 文件/归档摘要和固定源码摘要、全部统计，并独立复跑新能力、24 根原回归和依赖守卫及 diff 检查。没有真实 Orca/模型操作，无代码修复需求。
