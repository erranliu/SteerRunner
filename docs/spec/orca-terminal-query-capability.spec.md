# Spec：Orca 原终端查询能力 B1

日期：2026-11-02。状态：待实施。依据[长期能力层设计 B1](orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-terminal-query-capability.plan.md)。调查基线：`fe49bbb`；B0 的后续审计修复独立提交并保留。

## 1. 目标

把现有原终端 RPC 查询整理成使用 Orca 自身身份值的能力，供业务终端和本机控制终端共同消费。删除控制流每页读取前后的专属 CLI show 调用，保持两次身份围栏以及原完成、readiness/history、停止合同。

本批不将 Lifecycle 包装成公共能力，不抽取 B2 的命令体系，不重写 StateFeed、B3 原输入证明、B4 创建或停止。`protocol/orca` 其余模块暂有 backend 依赖，不以本批单个文件解耦宣称整个包已经独立。

## 2. 能力及消费边界

新增 Orca 自有终端身份值，仅包含 RuntimeID、Handle、IncarnationID。QueryBoundTerminal 及其底层查询接收该值并返回原生 Terminal；不接收 backend.ProcessIdentity、ExecutionTarget、Snapshot 或 Run。缺身份在连接前拒绝；返回的 runtime/handle/incarnation 必须对应请求。

身份不一致由 Orca 自有错误表达。执行适配器在消费边界映射到既有 BackendIdentityMismatch，保留可用的 ErrStateIdentityChanged 原因；请求/runtime envelope 冲突、取消、连接、协议错误仍保持区别，不能降成可自动换资源的普通断线。

共享执行侧窄读取函数将 ProcessIdentity 映射成 Orca 身份：真实 NewLifecycle 构造走 RPC；显式 state-factory 测试构造沿用隔离 CLI fixture。该选择只由构造器决定，生产 RPC 失败不回退 CLI。原输入查询内部只转换身份，保留全部前后围栏和 thin/corroborated 判据；旧停止 CLI 操作继续使用原合同，不扩大改动。

| 层 | 保留职责 |
| --- | --- |
| Orca 原终端查询能力 | 原目标连接、身份/请求核对、40 秒既有上限、短期限优先、取消和连接回收 |
| 业务 original | Snapshot 校验、原 worktree/path/branch/agent、初始化例外、connected/exited 等原有规则 |
| 控制消费方 | 原 control readiness/绑定、每页读取前后两次查询、原 cursor 与完整性、控制回执用途 |
| 旧 CLI 及原输入消费方 | 接入同一纯身份核对或作值映射，保留既有错误分类与对账/停止边界 |

接口和事实层实现不导入 backend。兼容执行类型的值转换只在原消费位置，不能增加长期双版本 QueryBoundTerminal、遗留 RPC 复制实现或生产 fallback。

## 3. 必须保持

1. 本机仍为 IPC、远端仍为明确 environment 的加密连接；查询不读取或修改隐式目标，不将 Caller local 与 executionHostId 混同。
2. 同一原终端的两次控制身份查询不减少、不移到分页循环外；错误后不读取/认领原回执。减少的是每页两次 CLI 子进程调用，terminal read 仍经现有 CLI。
3. 参数和身份错误、半包/EOF、拒绝、超时、取消均有原错误；对已知身份冲突的取消不能抹除原反证。
4. profile/state/creation plan、配置摘要、target hash、回执和公开 schema 不变。无新增命令、状态、重试、锁或恢复机制。
5. 原生输入/完成、managed history/审计、Pi/DSH、stop/reconcile/离线 verify-stopped 的适用规则保持。不能用终端查询可用替代业务或停止证明。

## 4. 验收

| ID | 完成条件 | 直接证据 |
| --- | --- | --- |
| OTQ-01 | 自有身份值；查询能力测试无需 backend 值、Run 或 Snapshot；迁入文件无反向依赖 | 编译/导入检查，缺身份零连接，IPC/WS 原生夹具 |
| OTQ-02 | 业务 original 和控制读取实际共用入口，生产不再在控制分页前后调用 CLI show | 生产构造器接线、RPC 调用计数、CLI show=0 |
| OTQ-03 | 每页前后 identity fencing 保持；中途替换和缺失失败，不采纳回执 | 分页受控夹具与前后请求/响应屏障 |
| OTQ-04 | CLI 队列被占用时终端 RPC 可返回，且取消/期限及连接回收保持 | 现有 queue/cancellation 测试、控制读取新增接线验证 |
| OTQ-05 | 原业务、原输入、control/history 和旧停止规则无回归，编码不变 | 受影响组件测试、生产/格式 diff 检查 |
| OTQ-06 | 双端真实无模型查询使用本批入口并核对原身份，资源精确收尾 | 本机/远端新建专属 shell 及查询记录；不启动模型 |

真实探针只需查询自有 shell，原生状态查询可用不授予 B5 开发资格；不为本批复制 B0 模型或完整稳定性矩阵。确认缺陷先修，无法确认目标/资源收尾时不能通过。
