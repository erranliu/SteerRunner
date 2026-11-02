# Spec：Orca 仓库与工作区查询 B4a

日期：2026-11-02。状态：已完成，受控验证及实施后独立审计通过。依据[长期路线](orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-resource-query-capability.plan.md)。调查基线 `8d77b9b`。

## 1. 目标与范围

抽取现有 repo list 与 worktree show 的原生命令/解码，返回 Orca 自有仓库、worktree、runtime 和清单截断事实。生产目前只有一个仓库清单解码和一个 worktree show 消费位置；不为批次制造不存在的重复查询或扩展未被消费的 repo show/worktree list SDK。

repo list 仍由原准备查询执行：沿用 queryPreparation 的持久预算、原白名单、CLI 队列、取得队列后的回调及取消。能力提供纯 repo-list 参数和基础 inventory 解码，消费方组合原执行机制；不为了提供一个完整方法而绕过现有准备预算或增加新的通用调用接口。普通 worktree show 可直接由 Client 提供原单次查询及同一原生解码。

Orca 能力只消费明确 selector/目标和原 Result，不接收项目路径、Run、固定 base、WorkspaceIntent 或 creation owner。仓库目录 basename 的跨平台/大小写匹配、Git 类型筛选、无/多匹配拒绝、原仓库/runtime/base/head/branch/instance 身份判定全部留给原执行消费者。

## 2. 保持与删除

- workspaceRepository 使用原准备查询和能力解码，删除专属仓库清单结构；queryRepositoryInventory 使用能力参数，删除对应 repo list 字符串拼接。原本机 status→path 与远端明确 inventory→id 的差别保留，不添加隐式发现或目标 fallback。
- prepareWorkspace 的 reconcile 使用能力 worktree show，删除专属参数和 shown 结构；原 created.Worktree 逐字段比较、runtime 及固定基线/名称判据不变。
- 完整性字段沿当前范围：repo list 保留 repos 与 truncated；不因新增未消费字段类型而改变旧解码。缺失字段不被能力升级成可信完整 inventory，原消费方仍按原规则判定；不新增原来没有的 totalCount 必需条件。
- 原 Worktree 类型及 JSON 字段不变，仍用于 creation checkpoint；不顺带改工作区创建/回执、恢复、停止、profile/state/target 或 selector 的持久编码。

能力文件无执行包依赖；原 Result 及调用阶段/不确定性保持，不在能力内新增重试。创建未知只对账原计划，不因统一查询重新发现仓库、换 selector 或重建资源。

## 3. 验收

| ID | 完成条件 | 证据 |
| --- | --- | --- |
| ORQ-01 | 自有资源/命令/解码，无执行语义；原 bytes/args/错误保持 | 原生值与真实 Client Output 夹具、明确 selector、坏包/截断/错误/不确定性 |
| ORQ-02 | 两个真实消费者接入，旧结构/命令删除，选择与基线策略保持 | 原 local/remote 仓库匹配/歧义/缺 ID/截断，恢复时精确原 worktree/runtime 与零再发现 |
| ORQ-03 | 原准备预算、回调及撤销/未知创建合同保持 | RepositoryQuery、Preparation、CreationOwner 与 Workspace 原定向回归及计数 |
| ORQ-04 | 持久/依赖及审计闭环 | 原 Worktree/创建/profile/state/target 编码 diff、依赖守卫、直接证据及独立审计 |

纯查询迁移采用受控验证，不新增真实资源或模型场次，不改变 B5 资格。

实施记录：已接入两个真实查询消费者，原策略/预算/持久格式保持；验证与删除清单见[验收](../acceptance/orca-resource-query-capability.md)。
