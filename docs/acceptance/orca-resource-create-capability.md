# Orca 资源创建 B4b 验收

日期：2026-11-02。状态：实现及受控验证通过，待提交后独立审计。依据 [Spec](../spec/orca-resource-create-capability.spec.md) / [Plan](../spec/orca-resource-create-capability.plan.md)。

## 职责、消费者和删除

protocol/orca/resource_create.go 提供 WorktreeCreate/TerminalCreate 纯参数和 WorktreeCreation/TerminalCreation 原生解码。可选 agent/command 用指针区分缺省与显式空串，不重写原参数。参数、标题、base、setup 和 agent 选择均由执行消费者提供；没有自动创建、重试或恢复方法。

workspaceCreationArgs、initialWorkspaceTerminalRecovery、prepareControl 已接入公共构造，三个旧 argv 删除；初始/控制回执共用原生解码。workspace 创建/原计划恢复共用 decodeWorkspaceCreation，原生字段交给能力，runtime_id/resolved 身份/selected_terminal 这些旧 checkpoint 扩展继续由执行侧解码，保留原字段类型拒绝。原 checkpoint 类型声明/字段顺序没有修改。

creationCall/ServeCreationOwner 未修改；B2a 编译/解码、原创建目录/marker、命令/镜像摘要、deadline、首次派发/对账算法、调用次数和未知零重发保持。所有 runtime/worktree/incarnation、固定命名/base、初始空清单和控制绑定仍由原消费者核验。Profile/State/Target 和持久版本均不变。

## 直接验证

源码基线 `99ed34f` 加本批差异；source_digest `e1fca147ce141a3cb09e83e3f22b9ba4d3eb041b584e6d963965fccd54418a14`。实际命令和原始记录见[证据](evidence/orca-resource-create-capability-20261102/README.md)。

| 验收 | 结果 |
| --- | --- |
| ORC-01–02 | 新能力/旧 checkpoint 对照 2 根/25 子节点、25 叶例通过；完整旧 argv、bare/agent/显式空值、空格/引号/中文、坏原生/业务字段、旧 JSON/摘要保持 |
| ORC-02–03 | 原消费者、命令/owner/未知恢复和控制定向 34 根/141 子节点、147 叶例通过，0 fail/skip；原 caller-loss、延迟/缺失/外来回执、deadline 及零重建断言保留 |
| ORC-04 | 两包编译、依赖守卫 1 根及 git diff --check 通过；原持久声明及旧构造删除 diff 已核对；待独立审计 |

子节点含分组节点，叶例另计，重叠集合不相加。原 Plan 的 TestNativeControl* 当前无该命名根例，真实控制消费者由 TestFixedControl* 覆盖；不把空模式计为通过。没有真实 Orca/模型或新 B5 资格，现行停止 scope 和支持合同保持。
