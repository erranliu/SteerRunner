# Spec A：退役旧 StateFeed 聚合层

日期：2026-11-06。状态：已实施并完成合并前定向验收。调查基线为 main `dda9a82727b6e7ac88e687a250750e8bbae236dc`（已包含 #127）；实施起点的工作副本 HEAD `da788478` 与该主线文件相同。本项不重复 #121/#124/#127 的执行退役、证明计算器删除或 Lifecycle 转发清理。依据[长期产品契约](steerrunner.spec.md#长期产品承诺)、[Orca 长期设计](../orca-execution.md)，配套 [Plan](orca-state-feed-retirement.plan.md)。A 通过并固定独立源码/证据后，再进入 [B](orca-helper-producer-retirement.spec.md)。

## 目标与职责边界

当前 Orca Run 由 observeStructured → readStructuredActivity → agentHistory 观察原结构化会话。旧 StateFeed 另行提供 Initial/Drain/Query/Close、接收时间戳、通知缓冲及本机/远端双实现，但已没有产品或注册 driver 的调用根。本项删除这一聚合模型，使维护原查询/订阅时不再同时维护一套无执行消费者的观察抽象。

原始查询和订阅仍是独立能力，不能随聚合层删除。它们的原目标绑定、错误、取消、连接与认证语义保持；本项不将通知改成当前 Run 的完成证明，也不把当前 history 读取改成旧 Feed。

## 实际消费者与合法入口

| 对象 | 已核实调用根 | 本项处理 |
| --- | --- | --- |
| OpenStateFeed、nativeStateFeed、localQueryFeed | state_stream_test.go 的 sample-feed 场景；state_query_feed_windows_test.go 的 TestOrcaLocalQueryFeed | 退役聚合层及独占采样语义；sample-feed 的更新传递覆盖改接 raw subscription，不整场删除，不保留生产 Feed 副本 |
| QueryOriginalSample、StateSample/SampleKind/StateFeed | 仅上述聚合闭包和 Feed 测试 | 可删除；它们不是持久执行证明 |
| SubscribeState、StateSubscription.Next/Close | tests/helpers/stability/main.go 的 probe-subscribe；tests/native/stability_test.go 的 networkPreflight；协议测试 | 保留生产入口、原实现和共享测试；网络断开预检仍消费原订阅 |
| QueryState、QueryWorktreeAgents、QueryOriginalAgent、DecodeOriginalState/DecodeOriginalAgent | stability driver/helper 的原 pane/provider 读取及协议本机/远端查询测试 | 保留原查询及身份/完整性校验，不改成 Feed 或当前 session 的替代解释 |
| StateResource、AgentState、MainAgentState、StateObservation 及其编解码 | 原查询结果、稳定性完成核验、协议身份/Observation/现存原生证据读取 | 保留声明、JSON tags、缺失/显式零值区别及原错误 |
| 当前 Agent Session、目录、文件、usage、终端查询等 RPC | 复用 state connection/socket/transport、配对/本机 IPC 和协议测试夹具 | 不在删除闭包；原字节、握手、能力和边界检查保持 |

“没有执行入口”只适用于 OpenStateFeed 聚合闭包，不适用于整个 state_* 文件族。probe-subscribe 在初始化成功后取消初始化 context，再以独立 context 继续 Next；原始订阅的这段生命周期必须保持。不能把 Feed 的 watch/cancel 删除扩大为缩短原订阅或单次 RPC 的生命周期。

## 删除与保留清单

候选生产删除闭包为：

- state_feed.go 的 OpenStateFeed、nativeStateFeed 及其 Initial/Drain/Query/Close、QueryOriginalSample；聚合层自己的 goroutine、64 项 updates 缓冲、互斥锁、接收时间戳和错误收集。
- state_query_feed.go 的 localQueryFeed、openLocalQueryFeed 及 Initial/Drain/Query/Close。
- 五个非持久聚合类型：SampleKind、StateSample、StateFeed、nativeStateFeed、localQueryFeed，以及 SampleInitial/SampleNotification/SampleQuery 常量。按基线为 11 个函数/方法，最终数量必须由实际调用审计确认，不能为凑数删除消费者。

保留 state_feed.go 中的原 AgentState/身份数据、stateIdentityError、DecodeOriginalState、DecodeOriginalAgent、ErrOriginalMainAgentUnavailable、QueryOriginalAgent；保留 state_observation.go 的原 wire 编解码和校验。保留 state_subscription.go 的所有原始查询/订阅；保留 state socket/connection/transport/local IPC、配对解密、request/runtime 核验、消息和调用边界。不得把公共连接缓冲或错误处理当作 Feed 的 updates 队列一起删除。

可以将保留的原声明移到职责更清楚的文件，但只改文件名不算收益；正文、签名和 wire 不改。不得新增 Feed 兼容壳、替代接口、缓存、自动查询、后台接收器或生产测试 hook。发现新实际消费者时先修订删除清单，不删除其调用或放宽其断言。

## 测试迁移与共享夹具

state_stream_test.go 的 TestOrcaNativeStateStream 同时覆盖 subscription-query、sample-feed、wrong-runtime、wrong-request、cancelled，以及四个 terminal-query 场景。只允许退役 sample-feed 对接收时间戳/Drain/缓冲的专属装配与断言；其余八个场景、原始订阅的初始快照、独立查询、取消/身份错误和共用加密 WebSocket 装配保持。不整删该文件，也不删除共享 server/socket、IPC 或认证夹具。

sample-feed 的 server snapshot/updated 发送和第二条通知的实际传递，不是聚合层独占语义。原其它八场景没有直接消费 StateSubscription.Next，不能以“八场景保留”替代更新传递覆盖。将此场景改接 SubscribeState/Next（可改名），复用真实加密连接发送 snapshot/updated；取消成功初始化的 context 后，用独立 context 接收同一订阅的原更新。期间保留独立 QueryState/QueryWorktreeAgents 不消费或中断原订阅的检查，并验证阻塞 Next 的调用 context 取消或对端断开返回原错误。使用明确屏障/原帧身份，不伪造 Next 结果，不恢复 Feed watch、采样时钟或 Drain 队列。原订阅 Close 的实现和清理语义保持。

TestOrcaLocalQueryFeed 的 initial/Drain、采样时钟、Feed.Close 及 Feed 生命周期属于退役对象，可以移除。它同时验证的原 pane/provider 查询、完整正文、provider 替换拒绝、零后台订阅及阻塞查询取消仍有意义，应改接 QueryState/DecodeOriginalState/QueryOriginalAgent 或已有原查询覆盖。取消测试使用原调用 context 和明确屏障，不能重建测试 Feed、加固定 sleep 或只删除原反例。允许将 Feed 专属测试退役/改名，但必须保存“删除语义→退役理由／保留语义→实际新入口”的逐项表，不机械要求旧测试数量不变。

保留 state_query_coverage_windows_test.go、state_observation_test.go、原生证据读取、当前 Agent Session/终端/目录/文件/usage 测试及其实际共享夹具。稳定性六组十四场景、probe-state/probe-subscribe、pre-send/network 工具和 evidence/manifest/cases 注册均保持；本项不修复其旧终端协议接线，也不删除场景换取通过。

## 持久格式与长期承诺

允许删除的五个类型是进程内聚合表示；StateSample 的 ReceivedAt/Kind 为 json:"-"，且全仓无 Run/Operation/Receipt 持久消费者。实施前仍须用真实引用和类型声明确认这一分类，不能仅凭字段名推断。保留 StateObservation 的缺失 incarnation 与显式零值等原 codec 语义。

state/v8、SQLite user_version=8、Request/Record/Operation/Receipt、Profile/State/Turn/NativeProof、结构化状态/RuntimeFence、原始 RPC wire、CLI/Workflow/capabilities/resource_digest 和结果文件合同不变。不迁移、回填、改写历史或替换冻结 scope；源码/构建摘要随代码变化正常更新，不要求新旧 EXE 哈希相等。

PC-R01–R10 保持：原受理/提交、派发前固定原身份与输入、unknown 只读对账且不重放、同构建原资源恢复、有界等待、stop 撤销新输入和原 scope 证明、结果归属/封存、观测错误与精确资源边界均不变。缺元数据或原证明仍未知/等待，不能用空 Feed 或新查询结果伪造完成/停止。预算缺失仍只阻新派发，旧执行仍由原构建排空；不新增跨构建接管或真机资格。

聚合层的取消/关闭不是原资源停止证明。删除它不扩大 orca_agent_session 为派生进程/主机级清场，也不把 WritersAbsent 置真；原 close 失败/丢回执仍未知，原未创建事实与原成功回执保持各自结算条件。

## 实际收益、最小验收与结束范围

收益是删掉一个无产品入口的 Initial/Drain/Query/Close 模型、两个实现及采样缓冲规则。原始 RPC 和当前 observer 各自保持原职责；不宣称减少运行开销、改进实时性或扩大支持范围。

| ID | 必须取得的结果 |
| --- | --- |
| SFR-A01 | 全仓生产/测试/driver 调用清单证明删除闭包；生产不再有 OpenStateFeed、两套实现、五个聚合类型/采样常量，无新增兼容转发层 |
| SFR-A02 | 原始查询/订阅签名与正文保持；八个共享 stream 场景、sample-feed 改接 raw SubscribeState/Next 的更新/独立查询/取消覆盖，以及 local 原查询反例实际执行；原身份/完整性/缺失/生命周期语义不弱化 |
| SFR-A03 | 类型差异仅为批准的五个瞬态类型，保留 wire/持久声明与 codec 相同；原始状态、Observation、Profile/NativeProof/结构化状态及 Record/操作的独立代表编码相同 |
| SFR-A04 | 完整协议包、受影响 adapter 和应用执行/结果/恢复/停止定向通过；live driver/helpers 编译及稳定性消费者完整；无模型输入或测试侧生产 hook |
| SFR-A05 | 正式构建及同最终 EXE 的 SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 定向公开受控通过；独立记录源码、产物、失败/skip、未验范围 |

SFR-A01–SFR-A05 完成后结束，不继续退役 raw SubscribeState、AgentState/Observation、共享 socket 或迁移稳定性 driver。若需改 wire/持久格式、当前 observer、取消/身份/停止语义或新增生产替代模型，超出本项，应先修订设计。实际实施结果见[独立验收](../acceptance/orca-state-feed-retirement.md)；不扩大完整级别或真机资格。

## 设计审计修订

2026-11-06，按上述主线静态复核删除闭包、stability 原始订阅消费者、共享 stream/本机查询测试和持久类型。11 个候选函数/方法与五个瞬态类型没有新增实际调用根，原始查询/订阅和全部 wire/持久声明仍是保留项。

确认并修订一处覆盖缺口：sample-feed 同时承担真实 snapshot/updated 传递测试，原其它八场景只覆盖初始化和独立查询，没有直接消费 Next。不能连同聚合器删除这部分共享通知语义；已要求改接 raw subscription，并把初始化 context 取消后的独立 Next、查询并存及取消/断开纳入 SFR-A02 和 Plan。此为文档静态审计，代码、测试迁移和 A01–A05 仍未实施。

## 实施状态

2026-11-06，按修订后的设计先 A 后 B 分别实施和验证，原设计审计段落保留当时文档阶段事实。实际删除闭包、测试迁移/保留、原字节、构建和公开受控结果以[本项验收](../acceptance/orca-state-feed-retirement.md)为准；完整 L1/Controlled、L2/L3 和真实模型未运行，不以定向通过替代。
