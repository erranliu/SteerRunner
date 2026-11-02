# Spec：Orca 状态与会话读取 B3

日期：2026-11-02。状态：待实施。依据[长期路线 B3 及职责迁移表](orca-capability-layer.spec.md#43-最小职责迁移表)，对应 [Plan](orca-state-session-capability.plan.md)。调查基线 `d30cd2b`；消费已完成的 B0 能力结论及 B1/B2 公共能力。

## 1. 目标

让 Orca 能力层只提供原生状态、会话消息和来源/完整性事实，把唯一原输入、冻结基线、thin/corroborated 完成资格、前后围栏组合和瞬态认领证明移回原 Orca 执行适配器。两端继续消费一份执行策略，不各复制算法，也不把 Lifecycle 包装成能力接口。

本批只做职责迁移和对应消费者收敛，不改变模型/工作区审计、原输入/Started/完成证明强度、支持组合、停止 scope 或 profile/state/target/operation 编码。B0 的未验边界继续成立，不启用正式本机无 helper 业务。

## 2. Orca 原生读取

### 2.1 状态和资源

状态 API/StateFeedFactory 使用 Orca 自有 StateResource，包含 runtime、worktree、terminal handle、incarnation；连接 environment 仍明确传入。DecodeOriginalState、OpenStateFeed、QueryOriginalAgent/Sample 及本机 query feed 不再接收 backend.ProcessIdentity。执行适配器在边界转换，原输入证明中对完整执行 ProcessIdentity 的比较留在执行侧，不降格为只比较四个字段。

AgentState、StateSample 继续表达原生原 pane/provider/main 状态、authority/revision、元数据有无、完整性/断线和 initial/query/notification/接收时间。原查询中的同 pane/provider/主状态元数据交叉核对属于原生资源事实，保持；thin 元数据只表示原投影缺失事实，不在协议层提供“可以认领输入”的方法。

AgentState 移除仅用于执行证明的 NativeInput 字段及 CanQueryCompletionInput 方法。不能改为 any/interface、把基线/Turn 写成字符串或给原类型换名后继续承载策略。原 NativeInput 字段不参与 JSON，删除它不改变既有持久/原生快照编码。

### 2.2 会话消息分页

新增有界原生会话 reader，接收明确 runtime/environment、agent、session ID、Orca transcript locator、page limit 和 page budget。它管理一次原连接、取消/关闭及向后分页，只返回原消息/blocks、role/source/timestamp/shortened、hasMore/beforeOffset、原始页和协议完整性。

reader 不接收 baseline、Prompt、command、Turn 或 ProcessIdentity，不选择用户行、不去重、不判断正文唯一性或业务完成。多条用户消息、相同正文的两条消息和原生不完整消息均作为事实返回；不能先过滤或拒绝它们再声称原生能力独立。页缺失、原生错误、无效/不推进 offset、错 envelope 身份等协议问题仍拒绝。

原读取约束保持：执行侧显式传入当前每页 100、最多 16 页，共用原 40 秒上限与短父期限；达到原基线后可停止读取并关闭，不能为了抽象强制读完整会话而扩大工作量或导致长会话新失败。每次读取仍只消费一个原连接，不从 Runner 文件系统打开远端 transcript、不自动换连接或目标重试。

## 3. 执行侧原输入策略

| 当前职责 | 迁移后所有者 | 必须保持 |
| --- | --- | --- |
| QueryNativeInput 基线后唯一用户算法 | orcatransport 的私有原输入策略，消费原生 reader | 完整页先校验、反向遍历、基线内外规则、相同正文重复也拒绝、全文/来源/时间/blocks/shortened 约束 |
| QueryOriginalInput、queryFencedCompletionInput | 同一执行策略 | 原 terminal/worktree/agent 与 pane/provider/main 前后围栏、thin/corroborated 资格及瞬态证明 |
| originalInputQuery、NativeInput 的匹配 | 执行侧私有类型 | 绑定完整原 ProcessIdentity、精确状态和原用户行；复制/篡改/换资源无效，wire 不能注入 |
| NativeProof/观察覆盖/输入缓存 | 原执行实现 | 原通知先后、已知反证优先、断线重置、旧 revision 不冒领、原正文/基线/回合约束及持久证明 |

查询返回一个执行侧结果，包含 raw AgentState 和可选私有 NativeInput；该结果不回写协议 AgentState。证明按显式参数传给需要它的原策略函数，原 nativeObservation 缓存只保留原生命周期；不把瞬态输入塞入持久 NativeProof、全局表、泛型扩展字段或新的账本。

执行侧可保留一个以 backend.ProcessIdentity 为入参的 state-factory 窄边界，以兼容现有隔离构造；生产 factory 显式映射到原生 StateResource，不发生生产失败回退。迁出的认领策略只保留一份，协议包旧 QueryNativeInput/QueryOriginalInput 及证明实现同批删除。

## 4. 验收

| ID | 完成条件 | 直接证据 |
| --- | --- | --- |
| OSS-01 | 原生状态/消息读取无执行身份和认领语义 | 迁入文件无 backend/执行包依赖；用自有资源值测试 IPC/WS/feed，消息能力可返回多个用户/不同来源及原始 blocks |
| OSS-02 | 基线/唯一原输入与完成资格已迁出，原策略仅一份，所有两端消费者接入 | 协议旧符号/字段删除、执行构造/原 observer/探针接线及策略测试 |
| OSS-03 | 证据不增权，瞬态证明无 wire/持久注入 | 原 thin/corroborated/部分元数据、重复/篡改、复制证明、换 process/pane/provider/authority/final/time、状态围栏与旧/新通知交错 |
| OSS-04 | 原分页、期限及资源生命周期不退化 | offset 反例、基线停止、16 页预算、取消/短期限、reader 单连接关闭/不重连，初始/查询不冒充通知 |
| OSS-05 | 持久格式及业务/停止回归闭环 | 原 codec/target/正文/Stopped diff、受影响两包及 workflow 依赖守卫、实施后独立审计；真实资格单列 |

B0 已证明的双端底层能力用于约束设计，不直接签收新构建。本批先以原协议字节及真实构造器受控测试证明等价；若改动触及原 wire 输入/生命周期或出现受控层不能排除的差异，补相应真实现场，不用文档推定通过。B5 仍负责完整真实 Runner 业务资格。
