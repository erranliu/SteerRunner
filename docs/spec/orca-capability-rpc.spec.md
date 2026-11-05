# Orca 带能力声明的 RPC 包装复用 Spec

以 `main=a66038c89208392c47531e640c84e97728c62424` 为基线，包含 #113、#115、#117。仅收敛 structured usage 和目录查询的调用代码：复用现有 `structuredUsageState` 包装，消除远端目录查询手写的发送、错误返回、等待回执，以及 `QueryCatalog` 两端重复的调用闭包。

将该包装移至现有 `state_transport.go`，使用私有中性名称（如 `capabilityState`），沿用内嵌 `*stateSocket` 和调用方提供的能力列表。`request` 原样委托 `requestWithCapabilities(..., false)`，`next`、`close` 继续落在同一个 socket。structured usage 使用此包装；目录查询两端连接建立后，通过现有 `stateConnection`、一个共用调用闭包和 `callState` 完成请求。保留 `readCatalog` 的函数参数形式及现有 runtime 取值闭包，无需新接口或公共 API。

`mutation=false` 只表示不消费业务输入派发授权，不能解释为协议层只读权限隔离。usage 通道已有临时 group/folder 授权范围的创建、撤销和清理，继续由原 usage 机制负责；包装不新增方法白名单或改变这些操作。目录入口仍只调用原有 `status.get`、`agentSession.agents`、`agentSession.modelCatalog`，不开放任意 RPC。

| 入口或边界 | 必须保留 |
| --- | --- |
| Structured usage | 显式远端环境要求、原配对解析、公钥指纹和 runtime 检查顺序、原错误分类、原握手能力及每次请求的 `remoteAgentSessionCapabilities()`。普通 usage 的 nil 能力分支和本机路径不改，使用原 socket/能力行为。来源绑定、授权与撤销、结果和 usage 业务解释仍由原调用方负责。 |
| 本机目录 | 固定元数据读取及 named-pipe 验证、原认证和 runtime 检查；仍由原 catalog 模式发送 `catalogCapabilities`，不读取远端配对。 |
| 远端目录 | 用原解码所得配对值完成加密连接；保留目录专用的首次 runtime 发现、后续 request/runtime 绑定及已登记 pairedDevice 校验。握手和建立连接后的目录请求继续使用原 `catalogCapabilities`。 |
| 请求与资源 | 方法、参数、能力列表内容和顺序、字段的缺省/null 表达、请求次数和错误透传保持。包装仅在连接建立后用于调用，不改变引导 `status.get` 的能力字段或次数。每次请求只发送一次并等待原 ID，无重连、补发或额外协商。整个目录查询共用原连接，查询结束由原调用方关闭，不在每次 RPC 后关闭。取消、接收预算、reader 和错误清理沿原实现。 |

完整保留 [#113 文件通道合同](https://github.com/erranliu/SteerRunner/blob/a66038c89208392c47531e640c84e97728c62424/docs/spec/orca-workspace-file-connection.spec.md)：文件通道不纳入本次包装，每次打开配对只解析一次，使用同一份已核验的 endpoint、公钥、token 拨号，本机仍校验原 runtime；worktree/instance、进度、停止后封存和升级排空合同不变。

完整保留 [#117 目录合同](https://github.com/erranliu/SteerRunner/blob/a66038c89208392c47531e640c84e97728c62424/docs/spec/native-catalog-boundary.spec.md)：无服务、零业务副作用、协议及能力检查、来源与完整性、ready/loading、原 listing 等待、错误分类与脱敏，30 秒总预算、累计原生响应及公开输出各 1 MiB、额外 5 秒关闭预算均沿原路径。目录能力不足不转 CLI 或其它来源。Codex 目录及执行链路不改。

生产改动限包装移名复用、`openUsageFiles` 的包装引用和 `QueryCatalog` 的调用收敛，预期涉及 `state_transport.go`、`usage_files.go`、`catalog.go`。不调整解析器、握手、分帧、预算、能力清单、业务派发或生命周期。不修改 Orca，不新增连接工厂、策略对象、缓存、重试或持久状态。收益应体现为删除重复调用代码；若需要额外适配层才能完成，收缩实现。

等价验收以原入口的线级方法/参数、能力字段、调用次数、返回值、错误和关闭行为为准，复用本机 IPC、加密 WebSocket、usage 和 #113 配对替换夹具。补齐实际缺失的精确能力及授权范围操作覆盖，不只验证成功响应或能力数量。同一候选 EXE 在本机 Orca、已有配对远端分别执行公开 agents/models 目录查询，无模型调用；如环境阻断则如实记录，不用历史 #117 证据代替。完整资格仍按 [测试指南](../testing.md) 判定。
