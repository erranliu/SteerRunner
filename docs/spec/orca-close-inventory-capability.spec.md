# Spec：Orca 关闭与完整清单 B4d

日期：2026-11-02。状态：实现及受控验证通过，待实施后独立审计。对应 [Plan](orca-close-inventory-capability.plan.md)，依据[长期设计](orca-capability-layer.spec.md)。调查基线 `5cd7c79`。

## 目标与边界

Orca 能力提供明确原身份的 CLI 终端核验、一次 close 的原 Result/handle/ptyKilled 事实、worktree terminal list 的参数/分层解码和完整原主机清单。移除最后三处 backend 依赖及未使用的 Config.Target；协议包最终不导入执行所有者。能力不接收 ProcessIdentity、ExecutionTarget、StopReceipt，不定义 stopped/resources_settled。

终端身份映射、BackendIdentityMismatch、ptyKilled 到 accepted/confirmed、哪些对象属于 stop scope、原终端退出/缺席如何满足合同，全部留在执行侧。原 VerifyTerminal(s)Stopped 和 CloseReadonlyTerminal 的执行策略迁至 orcatransport，小函数组合公共能力；现行双端停止、离线验证及 testkit/探针消费者接入这一份策略。辅助只读设施的现有有限重试策略不推广为业务停止重发规则。

完整清单保留明确 runtime/worktree、terminal 数量/截断/原 host 在场/omitted/重复身份核验，替换 incarnation 不证明原资源已停。列表 decoder 分别保留 sole-terminal 原消费字段范围与 initial/完整清单的全字段解析范围，不能因新增类型前置而改变旧消费者。startup 的空清单等待和选择仍在原执行层，不以完整停止清单算法替换它。

业务 close 仍经原 stop operation：排队未实际调用与可能派发但响应丢失分别表达。单次能力绝不重试，响应丢失只读原清单对账；不补造关闭回执、不建资源、不改变 request/operation 或降级 scope。B4d 只验证原合同，B5 再通过候选实际接线验收新增本机原生合同。

## 迁移与删除

client.go 移除执行 Config 字段/原执行签名和结算映射；readonly_close.go 删除协议内执行流程；stopped.go 改为原生 inventory 操作。准备清单消费者共用 list 参数/解码，删除各自专属原生结构，保留原字段范围/身份/空清单判据。原策略测试迁至执行侧，协议补独立能力测试。调用方不保留协议包兼容 wrapper，也不新增所有权账本。

所有持久状态、creation/stop progress 编码、Profile/Target 摘要、现行 scope、helper/control/根进程结算判据保持。最终依赖守卫覆盖原生能力包，不能只以无导入签收语义边界。

## 验收

| ID | 条件 | 直接证据 |
| --- | --- | --- |
| OCI-01 | 原生一次关闭/完整清单可独立使用 | 原 argv/错误/不确定性、ptyKilled 不等于业务成功、缺身份零调用、完整/缺 host/截断/重复/冲突 |
| OCI-02 | 所有停止/只读及准备消费者接入，原策略只有一份 | 原 exact identity/backend 错误、分层字段范围、startup 与 scope 保持、旧实现删除 |
| OCI-03 | 未知关闭只读恢复，未派发区别保持 | 原双端 stop/队列/回执丢失/替换 incarnation/完整清单负例及调用计数；无候选资格平移 |
| OCI-04 | 依赖/编码和审计闭环 | 原持久声明、能力依赖守卫、受影响编译/原回归/证据及实施后独立审计 |

实际迁移/删除、全部消费者、持久边界和验证见[验收](../acceptance/orca-close-inventory-capability.md)。
