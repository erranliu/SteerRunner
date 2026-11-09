# Plan A：StateFeed 聚合层精确退役

日期：2026-11-06。状态：已实施并完成合并前定向验收。依据 [Spec A](orca-state-feed-retirement.spec.md)，调查基线 main `dda9a82727b6e7ac88e687a250750e8bbae236dc`。实施起点的工作副本与该主线文件一致；以下为已完成步骤，实际范围见[验收](../acceptance/orca-state-feed-retirement.md)，未提交、提 PR、合并或部署。

## P0：源树、消费者和保护基线

- [x] 刷新 main，实际实施源树包含上述调查基线或已审阅的新主线，保留已有工作；不为 Rebase 提示改写已验证历史。相关消费者变化时先更新 Spec/Plan。
- [x] 保存 OpenStateFeed/QueryOriginalSample/五个聚合类型的所有引用，并对照原始 SubscribeState/QueryState/QueryOriginalAgent、current observeStructured、probe-state/probe-subscribe 与 networkPreflight，证明原始订阅不是删除项。
- [x] 固定源码和测试清单/摘要，列出 state_stream_test 九场景的保留/改接表，明确 sample-feed 的真实 snapshot/updated 传递不随采样层退役；另列本机 QueryFeed 的采样/生命周期语义与原查询反例如何分开。核对共享加密 socket、IPC、原生证据、目录/文件/usage 夹具。
- [x] 捕获全生产声明和原始 wire/持久代表编码；显式把五个瞬态类型列为允许删除项，保留 AgentState/Observation/StateResource 等原声明。独立捕获原字节，序列化失败不能忽略，含函数 setup 用独立 Clone/结构比较。
- [x] 审计设计：无当前执行消费者；raw 订阅初始化 context 与 Next 生命周期不改变；没有把同文件的公共 transport 缓冲当成 Feed 队列；若发现实际调用，不删其入口或反例。

## P1：先保留有效测试，再删除聚合闭包

1. 将 TestOrcaLocalQueryFeed 的原 pane/provider、完整正文、替换拒绝、无后台订阅和查询取消反例改接真实保留 RPC；继续使用原 IPC fixture 和明确屏障。Feed.Initial/Drain/Close、采样 Kind/时钟及 Feed 自身生命周期断言单列为退役，不重造测试 Feed。
2. 将 TestOrcaNativeStateStream 的 sample-feed 改接 raw SubscribeState/Next，可改名；删除的只是 Feed/Drain/接收时间戳/缓冲断言，保留共享 snapshot/updated 真实发送。用明确屏障验证初始化 context 取消后仍可在独立 context 接收原更新、独立 QueryState/QueryWorktreeAgents 不干扰订阅，以及阻塞 Next 的取消/断开。保留其它八个场景和共享 crypto/server/socket；不以只执行初始化代替消费更新，不 fake 恒成功回执，不重造 Feed。
3. 删除 Spec 明列的 11 个候选函数/方法、五个瞬态类型及三个采样常量；state_query_feed.go 无保留声明后可整文件删除，state_feed.go 不可整文件删除。按实际引用核实最终闭包，不以数量代替证据。
4. 保留 QueryOriginalAgent/原 decoder 和 raw subscription 实现正文；若搬文件，以声明对照证明保留内容相同。清理失去用途的 import/comment，不增加新 facade、缓存、观察线程或 production hook。

## P2：原能力、格式和当前执行复验

- [x] 对照 P0 删除清单与测试语义表；旧样本层无生产残留，原查询、订阅、配对/IPC、取消、能力握手及身份/完整性负例保持。
- [x] 运行完整 `go test ./internal/execution/adapters/protocol/orca -count=1` 和 `go test ./internal/execution/adapters/orcatransport -count=1`，设置合适包时限并保存 JSONL；Windows 专项在合法平台执行。核对 stream 原八场景、改接 raw Next 的通知/独立生命周期场景及本机原查询反例都确实匹配执行；保留错误分类和原调用实现，不只核对测试总数。
- [x] 按实际 diff 运行 application 停止/scope/结构化/消息/恢复/预算/封存、相关 host/usage/文件读取回归。编译 live native driver 和现存 helpers，特别是 stability 的 probe-state/probe-subscribe；不把编译当 L3 通过。
- [x] 比较原始 RPC/持久声明及独立原字节。批准删除的五类瞬态声明单列，不能将所有带 JSON 标签的类型机械视为持久类型，也不能借此删 AgentState/Observation 的 wire/缺失值语义。

## P3：独立公开验收点

- [x] 冻结生产/测试源码后用 `scripts/build.ps1` 正式构建；同 EXE 经 `scripts/test-foundation.ps1` 明确 Pattern 运行 `TestSRSingleExecutionColdStop`、`TestSRNativeUnknownAndStop`、`TestSRRestartPreservesAuthorization`。
- [x] 正式脚本异步启动独立宿主，等待并核对原 result.json/integration.log 的最终 exit_code/场景/子例，不把启动命令返回或零匹配当通过。不为等待结果重复启动该轮测试。
- [x] 在 docs/acceptance/orca-state-feed-retirement.md 保存删除/保留声明、测试语义迁移、消费者、原字节、源码/EXE 身份、实际命令/数量及失败/skip。历史失败保留，不按新一次通过改写。
- [x] SFR-A01–A05 完成后固定 A 源码摘要与可恢复归档，再进入 B。A 的结果不由 B 或后续上游变化倒追确认。

## P4：结束范围与交付

- [x] 按 docs/testing.md 区分定向/完整、组件/公开受控/真机、合并前/合并后。没有实际执行就不登记完整 L1/Controlled、L2/L3、模型或部署资格。
- [x] 只在实施验收后更新长期文档的实际状态；中央 README 无导航需要则不改。本批到删除聚合层结束，原始订阅、查询、历史证明及稳定性迁移不展开。

## 完成记录

A 完整验收和源码归档后再实施 B；各项命令、实际跳过与首轮失败保留于[独立验收](../acceptance/orca-state-feed-retirement.md)。源码、产物及三项公开受控分别冻结，不回填资格。B 协议源码/测试与 A 相同，明确复用 A 完整协议结果；其余受影响组件、应用定向组与公开受控分别执行。
