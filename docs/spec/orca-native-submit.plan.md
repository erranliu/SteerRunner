# Orca 原生输入通用提交 Plan

日期：2026-10-30。配套 [Spec](orca-native-submit.spec.md)、[重新核查](../acceptance/orca-native-submit-reassessment.md)。
状态：按后续深审修订，**尚未实施**。复用既有通用提交和缺失回执补查，不新增重试预算或修改Orca。上一版“无P0–P2”结论已撤回；本版明确M1本机模型/长final等价设计尚未闭合，M1通过前不启动生产切换。文档复审通过不等于M1能力已验证。strict接口、committed状态和三次重试仍不在范围内。

## P0 固定调查与审计边界

以当前checkout复核五个指定文件、local_codex_history.go、state codec、正式目录和输入脚本。已核实CLI仅text+enter进入agentPrompt，当前多行raw split绕开它；request-show已在本机/远端实测可用。13项安装代码隔离核验及只读查询不是历史原生复现，历史根因仍未确认。

本轮只修订Spec/Plan及调查记录的后续状态标记，并检查一致性、链接和删除清单，不改生产代码、配置、测试注册、安装环境或历史证据。完成独立文档审计后更新原单独PR；后续实现仍按下述顺序进行。

## P1 实施前独立方案审计

审计：同一次text+enter、无Runner框架/补Enter、原请求身份、accepted/started/completed区分、原请求只读查询、本机/远端共用、旧路径完整删除及测试证据层次。确认没有残留新Orca API、强提交proof、committed阶段、三次重试预算或新状态机作为实施前置。

审计也须检查保证边界：保留现有准备/权限/操作串行化及身份冲突阻断，但不宣称Orca已提供原子全文消费或所有人工写者排他事务。历史症状是否消失交真实验证，不能由正确接口或隔离通过直接推定。

上一轮文档审计通过结论已被深审发现的问题取代。本轮须重新审查四项修订及M1阻断的准确性，不把明确写出的未闭合能力当成已经验证。

2026-10-30复审结果：design_audit与final_review分别核对当前Spec/Plan及相关源码，均无新增P0–P2文档发现；均确认M1仍未通过，不能开始生产切换。此结论只认可文档准确约束了后续工作，不是能力等价或修复验收结论。

### M1 本机能力等价设计（未通过，生产实施前必须闭合）

提交入口统一、删除屏幕门禁与本机模型/结果能力迁移是不同工作，不能因文件同属adapter就视为小接线。先按Spec §5交付以下具体材料，再决定能否在既有能力内实施完整共同流程：

1. 按remote_models.go:69/145–174列出model wire、冻结step/command、turn/start模型/effort及clientUserMessageId的实际传递/核验；给出不依赖新增桥接、私有Orca API或提示词建议的等价接线，覆盖执行→审计→执行及恢复。
2. 对照local_codex_history.go:105/121、reply.go的32KiB上限与native_state.go:141–143的≥7999 UTF-16拒收，给出完整原final的现有权威来源和身份/唯一性/分页围栏；证明长结果、边界与超限语义均不退化，不能放松hook截断检查。
3. 给出准备、认证、原OS/终端所有权、停止与清理证明的保留清单和最小测试，不顺手改造凭据/进程控制。单列模型切换及长final回归，不能只用短nonce任务代替这些能力证据。

当前未证明以上材料已具备；M1不能仅靠此文档或stub通过。未闭合前不进入P2生产修改、不删本机wire/reader、不把保留旧本机完成分支作为最终同源交付；也不为消除阻断临时造执行框架、控制面或改codec。若确实需要独立能力迁移，应先提交明确的范围/接线设计再审计，而不是隐含塞入输入提交修复。最终本机/远端共同流程目标不变。

## P2 共同提交与对账接线（M1通过后）

1. 复用protocol/orca.Client和目标连接。普通原生Codex单行/多行、本机/远端调用一个提交函数，一次传入规范化正文、--enter和原--retry-request ID；两目标均不传--wait-submit，不在共享CLI锁内追加20秒开始观察。保留原50秒调用上限/更短调用方期限，承认宿主正文处理仍占锁。之后使用原native观察及现有独立原输入/新done开始补证；不得把working通知直接升级为Turn.Started，也不新建开始轮询器。
2. 先持久化原operation intent和冻结输入，核验原runtime/worktree/terminal/incarnation、pane/provider、主回合基线、权限及准备资格。保留CRLF→LF/去尾换行、输入预算及NUL/ESC检查。正文不带Runner bracketed框架，不按屏幕投影决定提交。
3. 共用一个send.prompt解析器处理直接返回与补回的回执；校验runtime/handle/requestId/incarnation及provider/observation。原生Codex仅provider=codex、observation=supported且绑定完整可作可靠受理/开始来源；unsupported、old-host、缺字段、incarnation_replaced和未知值保持未知，permission沿原阻断规则。它们可能已有副作用，不能报成未发送或改用raw补发。accepted仍不等于started，模型completed仍用原输入/final证明。
4. request-show仅恢复因传输丢失/不完整而缺少的send回执，前提是原输入已尝试且没有可验证accepted/rejected/started/final或持久冲突。使用现有Turn/operation/receipt推导，不按诊断字符串判断，不新增状态。已知accepted后只用原native观察，即使尚未started也不查询mutation；未知provider等明确异常回执也不触发补查循环。结果只单调合并，不能清除已有开始/final/冲突；query失败或弱回执不能阻断已有完整native结果。completed仅表示mutation回执保存，pending/absent/unknown不授权重投。
5. 严格按已通过M1的具体接线统一NewLifecycle的原输入查询，以及本机wire/observeRecorded替代；不得在此阶段重新发明模型/结果获取方案。将M1要求的逐阶段模型和长final、准备、认证、停止保证逐项落到既有测试，不仅合并函数名。

本次固定复用Turn、NativeProof、operation receipt及现有codec，不新增字段/版本或状态机；新持久化需求属于必须先重新设计的范围变化。调用前有明确无副作用拒绝证据时才记本次未发送，其余保持未知。Runner不主动选择旧host/raw回退，但不声称能阻止Orca内部在前台识别失败时转raw；必须保留unsupported等回执并零追加输入。只使用已发布CLI和既有原生查询RPC，不导入私有bundle实现，不加实验开关。旧未知记录不重派。

## P3 彻底删除旧路径

按Spec §6清单删除native_paste.go、fitComposer初始化及viewport类型/实现、独立paste/Enter、显示换行/标签/双稳定门禁及其专用期限；控制字符检查移至共同前置。本机wire/reader仅按M1已验证的等价设计清理，不把它们当成屏幕附属物直接删；不误删其它agent extension和共享准备/停止能力。

同步删除或迁移native_paste*、native_reflow、viewport测试及fixture字段；live probe取消草稿/resize门禁和segmented-shape专用模式，保留长输入、正式原文、唯一性、身份、final和清理断言。清理相关当前文档、脚本/目录声明及只为旧路径存在的配置/状态；不改历史evidence原字节和失败结论。

全仓搜索submitNativePaste/nativePasteDraftMatches/nativeDraftNeedsReflow/fitComposer/FitBoundComposer/ComposerViewport/ViewportRPC/ViewportFit/ViewportRestore及旧模式，结合引用判断删除完整性；共享WaitTimeout不能误删。最终仅一条原生Codex通用提交路径，不保留兼容回退、实验开关或第二套状态机。

## P4 确定性回归与正式入口映射

扩展现有orcatransport、protocol/orca夹具及tests/integration的CLI/SQLite/服务恢复入口。夹具用channel/受控回执顺序注入延迟、分段、丢失和身份变化，记录请求ID、调用次数、实际传给通用入口的字节/摘要及原结果。不要把模拟provider行为说成真实Orca保证。

| 需求ID | 入口与拟映射 | 条件与独有断言 | 证据 |
| --- | --- | --- | --- |
| NS-T01 | orcatransport共同提交表；既有focused native-input | 多行、中文、内部空行、CRLF、长输入；逻辑冻结不变，wire按既有规则规范化；无手动ESC框架 | 原prompt/digest、实际CLI参数/UTF-8、1次通用提交 |
| NS-T02 | 同表及TestOrcaNativeStateLiveLifecycle | 屏幕折行、宽度变化、折叠标签、投影缺LF、屏幕不可读不影响Runner提交/正式判定 | Runner正文read/resize=0、text-only/bareEnter=0；不把Orca内部行为计为Runner调用 |
| NS-T03 | 原输入query及final归属反例 | 实际输入缺失、截断、改字节、额外/重复输入不能算原输入成功 | 独立原文/唯一性核验失败、0成功归属；不伪称现有API能保证错误文本执行前必被原子拒绝 |
| NS-T04 | 共同提交/受控Orca入口 | 粘贴延迟/分段/乱序；宿主前台识别失败返回unsupported/old-host、缺provider/observation、permission及进程替换 | Runner通用submit≤1、text-only/bareEnter=0；不支持/异常回执不算可靠受理/开始，不谎报无副作用、不补发或新建私有路由 |
| NS-T05 | 既有native start/completion tests | 首次仅accepted、无wait-submit；随后独立原输入/新done补证完成；已知accepted但尚未started/缺final；mutation completed仅保存accepted | 不提前started/completed；已知受理后的request-show=0，不放松working开始规则；最终原input/final归属正确 |
| NS-T06 | 扩展TestSRStabilityLostSubmitResponse及native tests | 只有缺失send回执时才补查；retained回执/pending/absent/失败；查询前后已有native开始/final或持久冲突，返回弱/旧事实 | 查询条件与单调合并准确；正常完成不必查询mutation，弱/失败查询不清除或阻断既有完整证据；原ID/目标一致，submit≤1，0追加输入 |
| NS-T07 | 协议身份/本机准备与并发反例 | runtime/terminal/incarnation/pane/provider/主回合变化、已知外来输入冲突、同Runner并发操作 | 副作用前可见冲突阻断；后见冲突不得成功归属；记录原因，不借idle/屏幕证明原子排他 |
| NS-T08 | 扩展TestSRRestartPreservesAuthorization、原unknown/reconnect入口 | 重复调用、断线/重启前后、回执丢失/被清理；既有intent只Reconcile，request-show absent不新发 | 原journal、只读query计数、重启/恢复后0追加send；无新预算可被重置 |
| NS-T09 | 同一表参数化local IPC/remote WS；扩展既有CLI锁/多Run入口 | 相同语义；同服务至少3个Run共用CLI锁，受理后native观察及stop并行排队，控制队列顺序/取消；不同目标也共用调用方锁 | 无--wait-submit追加观察，记录实际锁序/调用而非猜墙钟耗时；无死锁、取消后0派发、0追加输入，观察/停止沿原期限处理；不得持锁等模型开始 |
| NS-T10 | 既有original input/final、model/stop/reconcile tests及M1证据 | 原输入唯一性、执行→审计→执行模型/effort、>7999 UTF-16且≤32KiB final、32KiB边界/超限，工具/旧结果拒绝及原资源清理 | M1逐项等价回归及原证明链；不以放松hook截断或丢模型设置求通过，未知不伪装已停止 |

NS-T03–T08及NS-T09共享锁故障子例，每个叶子故障、每个目标30次；报告实际执行、失败、跳过及用例清单，保留首轮失败和新批次。重复不替代覆盖，旧240次或新增13项安装函数核查不计作本矩阵。

实现时把需求ID绑定实际测试名/case ID：优先扩展tests/catalog.json.focused_native_scenarios及scripts/test-native-input.ps1、原TestOrcaNativeStateLiveLifecycle，统一local/remote目标格式，不再强制Environment非空；取消segmented-shape旧门禁。新增正式case同步cases.json/manifest/目录校验、docs/testing.md及test-catalog.md；规划阶段不预注册未实现case。

## P5 项目检查与真实验证

M1先行证据与实施完成后的回归分开登记。实施后按scripts/test.ps1 -Level L1及受影响Controlled/L3正式入口完成检查，运行框架目录一致性校验及P4重复矩阵；共享CLI锁的同服务多Run/观察/stop回归是放行条件。当前只有文档变更，不把过去基线通过当成未来实现验收。

真实验证沿扩展后的既有输入入口，本机/远端各3个独立原生会话、全新隔离现场/证据目录，使用无副作用最小任务（只返回nonce，不写项目、不跑业务/Unity、不建业务PR）。同组样例覆盖多行/中文/空行/CRLF/长输入，并使用同一受测构建与契约。短nonce仅验证通用输入接线；M1模型切换和长final须有各自实际能力证据，不能用这六次短任务代替。

发送前保存源码/EXE/CLI摘要、原目标/身份/完整清单和native基线；原请求及冻结正文先持久化。记录一次Runner通用提交、原accepted/started回执与必要的只读request-show结果、正式唯一原输入/完整final，最后精确关闭本次资源并用完整清单核验清理。

分别报告Runner RPC、mutation记录及provider历史输入次数；Orca内部按键/传输次数若公开接口不可观察则明确记不可观察，不能拿一次RPC反推一次Enter。请求未知时只读对账，不向历史Run重发或补Enter。缺真实条件记未验证，不用夹具通过或另一目标结果代替。

历史症状是否消失由这些新现场证据判断，不能因选择了正确入口就宣布修复。原始更强的原子全文消费/任意人工输入排他事务不在本次新增保证内，也不要求修改Orca取得它们。真实测试暴露的冲突/未知必须如实失败或未验，不静默绕过。

## P6 独立复审与交付

将最终diff、测试实数、真实原输入/final/清理证据及全仓删除清单交独立只读审计者，修正确认问题。更新原单独PR，不合并、不部署；未完成实施/真实验收前保持draft并明确未验项。

最终报告说明：Runner哪些屏幕依赖已删；可靠处理沿用哪些Orca既有契约及其边界；原身份/执行证明保留位置；稳定测试计数、六个真实会话结果及未验项；旧代码/测试/配置/文档清理情况与PR链接。不新增重试预算，不把旧失败改成通过。
