# Orca 原生输入通用提交 Plan

2026-10-31。配套 [Spec](orca-native-submit.spec.md)，PR #73。当前再审计基线 597d669；新冻结源 c21d915a6d5ce1d03e7b437b94048614781052312279076ac2cb678f2c3ad894。

状态：最小查询接线修正、旧生产 overlay 红对照和正式7170次矩阵已完成；原查询回归450/0/0。完整L1与新源managed本机3/3、远端3/3通过，最终独立补充复核通过，无未修P0–P2；按Spec及验收报告边界放行。旧批次资格仅由对应验收报告证明，不平移为当前通过。

## P0 固定当前范围与审计发现

以当前checkout和PR基线核对全部生产diff。保留一条通用text+enter提交、原编译/冻结、模型、准备、认证、原输入/完成与stop证明；删除草稿/显示换行/双稳定/视口专用路径。不引入实验API、重试、状态机、通用框架或兼容链。历史阶段结论放入原验收报告，活动Spec/Plan不再叠加互相冲突的旧实施指令。

本次P2：所有Codex send stage都已降为提示，但普通本机无helper仍缺queryInput构造接线。直接NewLifecycle、未预置Started/queryInput的回归确认无法开始/完成；增加RPC计数后四例均因query=0击中旧实现。仓库契约仍允许非GitHub、development/2普通本机配置，不以宣称该配置不支持规避回归。

## P1 实施前与实现复核

先审Spec §4.2、旧生产红对照及其真实构造器接线，再仅删除NewLifecycle.queryInput的远端if，保留同一QueryOriginalInput和Environment路由。managed observeRecorded、extension分流、Codex正式补证条件、认证与身份校验不变，不恢复stage当Started。

复用thin原输入IPC夹具，terminal.show返回其实际worktree绑定路径；返回原输入查询计数，不新造夹具。测试不注入Started或queryInput，不改变本机publicCandidate=nil契约。独立只读方案和实际diff复核已无新增P0–P2；最终检查和新现场仍须分别完成。

## P2 静态清理与兼容边界

逐引用确认submitNativePaste/nativePasteDraftMatches/nativeDraftNeedsReflow/fitComposer/FitBoundComposer/ComposerViewport/ViewportRPC/ViewportRecorder/ViewportFit/ViewportRestore/segmented-shape在生产和当前入口不再存在；历史证据仅保留原事实。保留共享WaitTimeout、通用read、wire/reader、其它agent/extension所需功能。

仅保留既有状态和codec，不迁移旧未知为accepted，不修改旧Run绑定。核对RPC错误无正文/秘密、无诊断探针；新旧提交路径不能并存。提交等待和共享CLI锁仍用原deadline，取消后不得派发、刷新期限或追加输入。

## P3 测试ID与原正式入口

| ID | 原入口/测试 | 条件与证据 |
| --- | --- | --- |
| NS-T01 | TestNativePromptSubmission / WireFormats | 本机/远端多行、中文、空行、CRLF、长文；冻结逻辑正文、wire/预算/摘要及实际CR参数，唯一通用submit |
| NS-T02 | 同表及TestOrcaNativeStateLiveLifecycle | 折行/宽度/折叠标签/投影缺LF/不可读不授权提交；草稿read/fit/restore、text-only与单独Enter为0 |
| NS-T03 | 原input/history/native final反例 | 缺失/截断/改字节/混入/重复输入、工具/旧结果不成功归属，正式完整原文与唯一性保持 |
| NS-T04 | TestNativePromptProcessingOrder / ReceiptFaults | 延迟/分段/乱序、unsupported/缺字段/permission/替换；channel/屏障及真实调用数，不用固定sleep恢复 |
| NS-T05 | 原native start / history / TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput | accepted及turn_started提示不能提前业务started/completed；原唯一输入与新done或history才证明 |
| NS-T06 | TestSRStabilityLostSubmitResponse及native回执反例 | 各阶段回执丢失/超时保留intent/未知原因；submit≤1、0补Enter、0request-show新恢复 |
| NS-T07 | TestOrcaNativeStateBinding / RecoveryRejectsProviderReplacementInsideOriginalTerminal | runtime/terminal/incarnation/pane/provider/session及已知外来输入冲突；原原因不丢、不成功归属 |
| NS-T08 | TestSRRestartPreservesAuthorization及原reconnect/unknown | journal/Request/Attempted恢复后只对账；重复请求、断线、重启不追加业务send |
| NS-T09 | TestNativePromptSharedQueueWithStopAndCancellation / TestOrcaCLISerializesProcessesWithoutRetryOrDispatchAfterCancellation / TestSROrcaCLIWaitRetry | 持锁send、peer stop、第三Run取消，用屏障验证0取消后派发/原deadline/锁释放；three-runs-stop-queued验证服务三Run准备队列，不能冒称三个活动原生Run全链路 |
| NS-T10 | 原model/history/original-input/final/stop/reconcile | 本机writer/audit/writer、实际模型/effort、32KiB结果、hook截断拒收、正式原文及资源/认证/签名证明不退化；Pi/DSH不变 |
| NS-T02/07/10 | TestLocalPreparationOriginalStream / TestLocalNativeProbeTargetAndBusinessScope / CensusKeepsOriginalHostScope | READY原始流完整/空流/缺字段/身份/绑定反例、目标配置及原host清单；不把准备成功计业务完成 |
| NS-T02/07/10 | TestFixedControlRetainedStream / StreamFaults / StreamBoundaries | >1000行/分页/前缀淘汰、丢页/倒退/无进展/身份/取消/快照/半行；原请求与正式history/workspace/auth-stop证明 |
| NS-T11 | TestRemoteModelBridgeUsesOriginalNativeConnection / RejectsInvalidOriginalRequest / RejectsUnboundInvalidRequest / PreservesLargeNativeReplies | >16MiB原回复完整字节/摘要与同连接原提交；错误保留RPC id、零转发、查询仍通，认证/原线程/模型冻结不变 |
| NS-T12 | TestNativePromptWireFormats / ReceiptFaults / TestNativeCRLFPastePreservesOriginalInputAndCompletion / TestTerminalReplyRequiresOriginalStartedAndEnded | 实际CR参数与逻辑LF分开核验，正式原输入不可放宽；started-hint/accepted不能提前开始，busy与unstarted前提分别保持 |
| NS-T13 | TestOrcaNativeStateLiveLifecycle / test-native-input.ps1 long-input | 新冻结源、隔离agent/driver；双端各3新session，唯一send/原请求/正式原文/模型/final/stop及空census |
| NS-T14 | TestOriginalTerminalRPC* / TestOriginalTerminalConcurrency | 既有单测夹具stop/wake停Accept再Close，连接/worker在失败分支仍清理，错误明确；仅测试修正，不升级依赖或预算 |
| NS-T15 | TestLocalNativeProbeOriginalQueryConstructor；原controlled必需root | 生产构造器、不预置Started/queryInput；thin/corroborated/duplicate/altered，中文/空行/CRLF长原文；实际查询1次、业务submit1次，0伪造streamed working/helper final；每叶30次 |

NS-T01–T12故障仍按目标/叶例重复30次，重复不替代覆盖。夹具必须确定控制处理延迟、分段、回执顺序和故障点，以原请求、调用计数、字节/摘要判定，不以同一离线快照充数。

## P4 正式检查与当前实数

- 原入口 `scripts/test-native-input.ps1 -Mode controlled -Repeat 30 -OutDir <新目录>`：当前25必需roots、239叶、7170执行/7170通过/0失败/0跳过/缺失root 0；含NS-T15四叶各30次。
- 原thin/corroborated与新构造器回归：3 roots、15叶各30次，450/0/0。旧生产overlay最终四例全失败，原事件与退出码1保留。
- `scripts/test.ps1 -Level L1`：独立运行完整规定检查、框架/扩展/native编译和零匹配门禁，原默认并行与12分钟单包期限，不跳过失败、不改全局配置。完整L1已覆盖受影响整包，无需重复同一整包来凑次数。
- 目录/测试入口与源码冻结检查照常。不同源/EXE/批次分别记录，缺失或意外skip不算通过。

## P5 新源真实验证

在本次隔离目录构建现有cmd/steerrunner-agent，固定源/EXE/CLI摘要。启动前读取并核对当前runtime，不套用过期模板；运行中的同一runtime可用于新的独立会话。用原long-input正式入口，在全新空现场本机与192.168.1.151远端各3独立session，只返回nonce，不跑业务/Unity，不创建业务PR。

记录各场helper/WireFormat类别，不能把managed本机现场说成无helper分支真实通过。本次既有正式本机目标为GitHub/managed，未覆盖无helper的非GitHub真实目标，生产构造器+隔离IPC证明与真实证明分开；不改GitHub仓库URL、认证、业务材料来绕过该条件。

发送前保存Run/Attempt/runtime/terminal/incarnation/session/原request、冻结正文及完整资源清单；核验一次Runner业务通用submit、accepted/正式started/completed、唯一原文/final；本机managed另核验实际模型/effort，远端只冻结启动偏好，实际模型/effort未独立核验。0text-only、0补Enter、0业务屏幕读取；宿主内部不可观察次数不推断。

按既有正式stop核验原资源、认证/reader/writer/签名及原host完整空清单，精确移除本次工作区并保存异步rm和后续not-found。远端停止仅orca_terminal；未独立核验物理路径则保留未验。未知或失败不向旧Run重发或补Enter，改用全新现场也不能追认旧失败。

## P6 复核与PR

独立只读审实现、测试/源身份、原事件实数、六场原调用及正式证明、停止范围和离线归档。确认活动文档单一契约、旧代码/状态/配置/测试清理完整，未引入实验API、回退、第二状态机或诊断探针。修确认问题后按影响补验证，不重复无关检查。

新生产源完成新资格后更新原PR #73，不合并或部署；报告当前结果、已清理范围、权威来源、未验边界和PR链接。旧构建和历史失败保留在对应[实施](../acceptance/orca-native-submit-implementation.md)、[原双端](../acceptance/orca-native-submit-remote-final.md)、[持续读取](../acceptance/orca-native-submit-control-stream.md)、[保真](../acceptance/orca-native-submit-integrity.md)验收中，不改写或平移。