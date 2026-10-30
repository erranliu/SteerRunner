# Orca 多行输入屏幕草稿依赖修复 Plan

日期：2026-10-30。配套 [Spec](orca-native-submit.spec.md)、[调查记录](../acceptance/orca-native-submit-reassessment.md)。
状态：按用户最新范围修订，**尚未实施**。只替换屏幕草稿多行提交及清理专用依赖，保留模型、原输入/完成和停止流程。上一版M1迁移与request-show恢复接线退出本次范围，不代表其能力已闭合；不新增Orca能力或Runner重试预算。

## P0 固定范围与基线

复核lifecycle.go的受影响条件：远端、普通Codex、无extension/wire、多行→submitNativePaste。其余路径已有通用text+enter。本次仅删除这条手动分支、屏幕附属依赖并增加必要回执校验；原wire与逻辑输入摘要、模型设置、本机history和远端NativeProof、prepare/auth/stop/codec均保持。M1不再是实施前置，也不保留其迁移任务供实现阶段展开。

本轮只改Spec/Plan及调查记录状态，不改生产、测试注册或安装环境。13项安装代码隔离核查、两个目标absent只读查询、旧快照/隔离重复及过去L1不是本方案实现验收；历史根因未确认，原字节和失败批次保留。

## P1 实施前独立审计

审计本版范围：是否真正只替换多行提交、是否保留wire/模型/history/完成与停止；是否有新私有/实验API、request-show恢复、codec/状态机或等待策略混入。核查原身份/intent、必要provider/observation校验、accepted/started/completed区分、未知零重投以及草稿专用删除清单。

保留宿主内部raw/held-Enter及任意人工输入排他的证据边界。旧版本审计结论不直接覆盖本版；本版审计需单列继承现有远端20秒观察后的共享锁风险，不把准备idle当空草稿或原子排他证明。

2026-10-30本版复审：design_audit与final_review独立只读核对Spec/Plan及相关代码，均无P0–P2文档发现。只认可本版窄范围设计与证据边界，生产实施、故障重复和两端真实验收仍未执行。

## P2 替换多行提交

1. 在lifecycle.go移除submitNativePaste条件及其提前Accepted返回，让该输入落入已有terminal send代码；继续使用原wirePrompt/nativePromptText结果、--enter、原--retry-request ID及原目标连接。不另建本机/远端发送器，不包装ESC，不保留屏幕/old-host回退。
2. 保持提交前原准备/idle/身份检查、wire冻结与预算、operation intent/Request/Attempted持久化顺序。原草稿路径的NUL/ESC检查移到受影响输入的副作用前，保持其它agent与本机wire输入范围；检查失败不能先发正文。
3. 沿用既有CLI等待参数与调用上限：NativeRemoteProfileVersion的--wait-submit 20、50秒调用上限及更短调用方期限不改，其它路径原参数不改。多行继承既有通用观察会改变锁占用，可能耗尽其它调用原期限；记录真实调用/排队和取消结果，排队与执行共用原deadline，不保证stop抢占。不增加计时器、统一成新等待策略或声称与旧40秒草稿路径耗时相同。若发现死锁、取消后派发、期限刷新、错误成功或违反既有停止约束，先修订设计，不临时添加锁或兼容分流。
4. 在现有send.prompt解析中增加普通原生Codex所必需的provider/observation和阶段一致性校验，保留原runtime/handle/request/incarnation/CallUncertain检查。可验证accepted只记受理，turn_started依原规则；不支持、缺字段、替换或未知不算可靠成功，保留原回执及原因，可能有副作用时不报未发送。其它agent契约保持。
5. 不改observeOnce分流、NewLifecycle.queryInput目标接线、native_start/native_state、observeRecorded或reader。未知沿原错误/Reconcile/observer，不新接request-show、不回放send；缺accepted不能绕过原完成前提。保留本机模型/长final能力和原stop证明，不新建桥接、结果来源或持久化字段。

## P3 清理屏幕专用依赖

按Spec §5删除native_paste.go、Lifecycle.fitComposer及初始化、composerViewport、terminal_viewport.go专用实现/测试及fit/restore回执；删除native_paste*、native_reflow的投影/稳定断言和fixture字段，将有价值故障迁入通用提交行为测试。short_input_trace等诊断删除专用fit包装，不能误删独立证据。

现有native_input_live_probe改为调用实际生产提交链路，取消手工paste/Enter、草稿read/resize门禁与segmented-shape专用模式；更新scripts/test-native-input.ps1、focused catalog及当前testing/test-catalog说明，计数改为Runner通用提交和正式历史原输入，保留原文/final/身份/清理断言。本机新增目标适配只用于同一正式测试入口，按现有准备与history流程验证，不改生产完成接线。

全仓搜索submitNativePaste/nativePasteDraftMatches/nativeDraftNeedsReflow/fitComposer/FitBoundComposer/ComposerViewport/ViewportRPC/ViewportRecorder/ViewportFit/ViewportRestore/segmented-shape，逐引用确认专用项已删、共享项仍有正当使用。保留wire/reader、共享WaitTimeout及通用read能力。历史acceptance/evidence不删除或改判通过，必要时只追加后续状态说明。

## P4 确定性回归与正式入口映射

扩展现有orcatransport夹具、protocol/orca协议测试和tests/integration CLI/SQLite/服务恢复入口。用channel/显式屏障控制处理延迟、分段、回执顺序与故障位置；记录原请求、通用submit次数、实际CLI正文/UTF-8或摘要、模型wire与原结果。模拟provider处理只能证明Runner调用行为，不冒充真实Orca完整消费证明。

NS-T09扩展TestOrcaCLISerializesProcessesWithoutRetryOrDispatchAfterCancellation及既有observer_queue/stop入口：同服务一个多行send持锁、第二Run的stop排队、第三Run调用排队后取消，用屏障释放持锁调用。核对取消回调0次、锁最终释放、未知不伪装已停止/完成及原提交不重复；不按外部OS锁的全局FIFO断言成功顺序，也不把已有独立锁测试充当新增接线验证。

| 需求ID | 入口与拟映射 | 条件与独有断言 | 证据 |
| --- | --- | --- | --- |
| NS-T01 | lifecycle既有提交表；focused native-input | 多行、中文、内部空行、CRLF、长输入；本机现有wire、普通远端规范化均保持 | 冻结prompt/digest、编译wire/参数逐字节、一次通用submit；0手动ESC框架 |
| NS-T02 | 同表及TestOrcaNativeStateLiveLifecycle | 折行、宽度改变、折叠标签、投影缺LF及屏幕不可读不影响Runner提交/归属 | 草稿正文read/fit/restore=0，text-only/bareEnter=0；正式原输入/final断言保留 |
| NS-T03 | 原输入/history/native final既有反例 | 原输入缺失、截断、改字节、混入/重复输入；工具/旧结果 | 不能归属原输入成功；用正式原文/唯一性证明，不声称执行前原子拒绝 |
| NS-T04 | 通用提交受控夹具 | 正文处理延迟/分段、回执乱序；unsupported/old-host/缺provider或observation/permission/进程替换 | submit≤1、无分开Enter/固定sleep恢复；异常不算可靠受理/开始、不谎报无副作用 |
| NS-T05 | 原native start及本机history tests | 首次仅accepted、观察期限内未开始；随后新done/原输入补证或本机history完成 | 不提前started/completed；保留working开始条件和目标原有完成来源，无新增查询门禁 |
| NS-T06 | 扩展TestSRStabilityLostSubmitResponse及native tests | 写入/提交、受理、开始、完成窗口回执丢失/超时；重复观察仍未知 | 原回执/原因/intent保留；submit≤1、request-show=0、0追加输入；缺accepted不伪造完成 |
| NS-T07 | 身份/准备/并发反例 | runtime/terminal/incarnation/pane/provider/session变化、已知外来输入、同Runner并发 | 已知前置冲突零发送，后见冲突不成功归属、原因保留；idle不证明空草稿或原子排他 |
| NS-T08 | 扩展TestSRRestartPreservesAuthorization及原reconnect/unknown入口 | 重复请求、断线、服务重启、原回执丢失/已清理 | 原journal/Request/Attempted恢复后只走现有对账；0追加send，无预算可重置或同ID安全重放假设 |
| NS-T09 | 参数化local IPC/remote WS；既有CLI锁/多Run入口 | 同组输入/故障经通用提交；同服务≥3个Run、原生观察/stop排队、调用和排队期间取消 | 两目标无草稿路径、原wire/完成接线保留；CLI参数/锁序/实际调用计数；无死锁、取消后0派发/追加输入，遵守原调用期限，锁风险如实登记 |
| NS-T10 | 既有model/history/original input/final及stop/reconcile tests | 执行→审计→执行模型/effort、本机32KiB结果边界、远端hook截断拒收、正式原文与资源清理 | 跑原有能力回归，不迁移模型/结果或放宽阈值；原证明链与完整stop census不退化 |

NS-T03–T08及NS-T09锁/取消故障：每个叶子故障、每目标30次。报告实际执行、失败、跳过及case清单；目标固有不支持能力单列，不伪造双端模型能力等价。重复不替代覆盖，不计旧快照或13项隔离核查为新矩阵。

实现时把NS-T01–T10绑定实际测试名/case ID；优先扩展tests/catalog.json.focused_native_scenarios、scripts/test-native-input.ps1及原TestOrcaNativeStateLiveLifecycle。统一测试目标格式，允许本机Environment为空，生产流程不因此改造。必要新增正式case同步cases.json/manifest/目录校验和当前文档，规划阶段不预注册未实现case。

## P5 项目检查与真实验证

实施后运行scripts/test.ps1 -Level L1、受影响Controlled/L3正式入口、目录一致性及P4重复矩阵。核对diff没有模型、history、NativeProof/原输入、prepare/auth/stop/codec的生产重构；回执必要校验和草稿清理需逐项解释。多Run/观察/stop锁与取消回归为放行条件。

使用受测构建，在全新隔离现场，本机/远端各至少3个独立原生会话。同组无副作用最小任务覆盖多行、中文、空行、CRLF、长输入，只返回nonce、不跑业务/Unity或创建业务PR。各目标按已有准备、认证和完成证明运行，不新增生产旁路。本机模型/长结果跑既有不退化回归，不要求先完成M1迁移，也不把六个短任务当作全部能力证明。

发送前保存源码/测试EXE/CLI摘要、目标runtime/terminal/incarnation/session、原请求/冻结正文及完整资源清单。记录一次Runner通用提交、实际accepted/started及原完成证据、正式唯一原输入/完整final，精确停止本次资源并用清单证明清理。宿主内部按键/传输不可观察则明确标记，不能以一次RPC推断一次Enter。

丢回执或未知不人工补Enter、不向历史Run重发；没有新增request-show自动恢复，缺证沿原错误处理，不能为了六次通过临时绕过。缺真实条件如实记未验证，不用受控通过或其它目标代替。历史症状是否消失只能由新现场证据判断。

## P6 独立复审与交付

提交最终diff、实际测试计数、六个真实会话证据、清理清单与未验项做独立复审，修正确认问题。更新原单独PR，实施和验收前保持draft，不合并或部署安装环境。

最终说明删除的屏幕依赖、沿用的通用提交契约及未获证边界、保留的模型/身份/原输入/完成/停止位置、稳定与真实测试实数、专用代码/配置/测试/文档清理和PR链接。不新增重试预算，不把历史失败改为通过。
