# Orca 原生输入通用提交 Plan

日期：2026-10-30。配套 [Spec](orca-native-submit.spec.md)、[重新核查](../acceptance/orca-native-submit-reassessment.md)。
状态：按用户确认方案修订，**尚未实施**。复用既有通用提交和原请求查询，不新增Runner重试次数/预算，不以修改Orca为前置。此前新增strict接口、committed状态和三次重试方案均退出当前实施范围，旧调查记录仅供溯源。

## P0 固定调查与审计边界

以当前checkout复核五个指定文件、local_codex_history.go、state codec、正式目录和输入脚本。已核实CLI仅text+enter进入agentPrompt，当前多行raw split绕开它；request-show已在本机/远端实测可用。13项安装代码隔离核验及只读查询不是历史原生复现，历史根因仍未确认。

本轮只修订Spec/Plan并检查一致性、链接和删除清单，不改生产代码、配置、测试注册、安装环境或历史证据。完成独立文档审计后更新原单独PR；后续实现仍按下述顺序进行。

## P1 实施前独立方案审计

审计：同一次text+enter、无Runner框架/补Enter、原请求身份、accepted/started/completed区分、原请求只读查询、本机/远端共用、旧路径完整删除及测试证据层次。确认没有残留新Orca API、强提交proof、committed阶段、三次重试预算或新状态机作为实施前置。

审计也须检查保证边界：保留现有准备/权限/操作串行化及身份冲突阻断，但不宣称Orca已提供原子全文消费或所有人工写者排他事务。历史症状是否消失交真实验证，不能由正确接口或隔离通过直接推定。

本轮对本版两份文档的独立只读审计已通过（reassessment_review，无P0–P2）；P2起的生产实施和后续验收尚未执行。

## P2 共同提交与对账接线

1. 复用protocol/orca.Client和目标连接。普通原生Codex单行/多行、本机/远端调用一个提交函数，一次传入规范化正文、--enter和原--retry-request ID；统一使用既有--wait-submit 20/CLI50秒上限，原调用方更短期限优先。不加循环、额外计时器或预算配置。
2. 先持久化原operation intent和冻结输入，核验原runtime/worktree/terminal/incarnation、pane/provider、主回合基线、权限及准备资格。保留CRLF→LF/去尾换行、输入预算及NUL/ESC检查。正文不带Runner bracketed框架，不按屏幕投影决定提交。
3. 复用原send.prompt回执解析，校验runtime/handle/requestId/incarnation等真实返回身份。accepted仅记受理，turn_started沿既有开始规则，模型completed仍由原输入/final证明。不能要求或假造现有回执没有的消费摘要字段。
4. 在既有Reconcile/observer中复用orchestration request-show只读查询。保持同caller/runtime/request，核验method及保留send回执；completed仅为mutation回执保存，pending/absent/unknown不授权重投。沿既有查询节奏、期限和阻断/错误返回，不增加三次重试或新恢复执行器。
5. 统一NewLifecycle的本机/远端原输入查询接线，以及本机Codex prepareInput/observeOnce的wire、observeRecorded/helper选择，迁入共同原输入/final证明。保留执行准备、认证、模型参数和停止保证；先核实耦合再改，不能只统一函数名而保留不同正文/完成算法。

默认复用Turn、NativeProof、operation receipt和现有状态codec。若实现发现必要证据无法保存，先给最小字段/codec变更及保守读取测试，不建立新提交状态机。能力/身份不满足时在副作用前拒绝或保留未知，无旧host/raw send回退。旧未知记录不重派。

## P3 彻底删除旧路径

按Spec §6清单删除native_paste.go、fitComposer初始化及viewport类型/实现、独立paste/Enter、显示换行/标签/双稳定门禁及其专用期限；控制字符检查移至共同前置。删除本机Codex专用wire/完成选择中被共同实现取代的接线，不误删其它agent extension和共享准备/停止能力。

同步删除或迁移native_paste*、native_reflow、viewport测试及fixture字段；live probe取消草稿/resize门禁和segmented-shape专用模式，保留长输入、正式原文、唯一性、身份、final和清理断言。清理相关当前文档、脚本/目录声明及只为旧路径存在的配置/状态；不改历史evidence原字节和失败结论。

全仓搜索submitNativePaste/nativePasteDraftMatches/nativeDraftNeedsReflow/fitComposer/FitBoundComposer/ComposerViewport/ViewportRPC/ViewportFit/ViewportRestore及旧模式，结合引用判断删除完整性；共享WaitTimeout不能误删。最终仅一条原生Codex通用提交路径，不保留兼容回退、实验开关或第二套状态机。

## P4 确定性回归与正式入口映射

扩展现有orcatransport、protocol/orca夹具及tests/integration的CLI/SQLite/服务恢复入口。夹具用channel/受控回执顺序注入延迟、分段、丢失和身份变化，记录请求ID、调用次数、实际传给通用入口的字节/摘要及原结果。不要把模拟provider行为说成真实Orca保证。

| 需求ID | 入口与拟映射 | 条件与独有断言 | 证据 |
| --- | --- | --- | --- |
| NS-T01 | orcatransport共同提交表；既有focused native-input | 多行、中文、内部空行、CRLF、长输入；逻辑冻结不变，wire按既有规则规范化；无手动ESC框架 | 原prompt/digest、实际CLI参数/UTF-8、1次通用提交 |
| NS-T02 | 同表及TestOrcaNativeStateLiveLifecycle | 屏幕折行、宽度变化、折叠标签、投影缺LF、屏幕不可读不影响Runner提交/正式判定 | Runner正文read/resize=0、text-only/bareEnter=0；不把Orca内部行为计为Runner调用 |
| NS-T03 | 原输入query及final归属反例 | 实际输入缺失、截断、改字节、额外/重复输入不能算原输入成功 | 独立原文/唯一性核验失败、0成功归属；不伪称现有API能保证错误文本执行前必被原子拒绝 |
| NS-T04 | 共同提交/受控Orca入口 | 粘贴处理延迟、分段、回执乱序；Runner只调用通用text+enter，不另按屏幕/固定sleep/补Enter恢复 | 参数/步序、业务submit=1、额外输入=0；Orca真实粘贴处理由P5验证 |
| NS-T05 | 既有native start/completion tests | accepted尚未started；query的mutation completed只保存accepted；均不提前完成 | Turn事实/事件、原开始补证与完整final |
| NS-T06 | 扩展TestSRStabilityLostSubmitResponse及native tests | 写入/受理、开始、完成各窗口丢回执/超时，原请求只读查询；查不到、pending或查询失败不触发send | 原请求ID/目标一致，Runner业务submit≤1、text-only/bareEnter=0、结果未知保留 |
| NS-T07 | 协议身份/本机准备与并发反例 | runtime/terminal/incarnation/pane/provider/主回合变化、已知外来输入冲突、同Runner并发操作 | 副作用前可见冲突阻断；后见冲突不得成功归属；记录原因，不借idle/屏幕证明原子排他 |
| NS-T08 | 扩展TestSRRestartPreservesAuthorization、原unknown/reconnect入口 | 重复调用、断线/重启前后、回执丢失/被清理；既有intent只Reconcile，request-show absent不新发 | 原journal、只读query计数、重启/恢复后0追加send；无新预算可被重置 |
| NS-T09 | 同一表参数化local IPC/remote WS | 同组输入/故障共用提交、查询、开始及完成语义，不保留本机wire/helper完成旁路 | 两目标逐case核验、target之外无算法分流；准备/模型/认证/停止能力不退化 |
| NS-T10 | 既有original input/final、stop/reconcile tests | 原全文唯一性、完整final、工具输出/旧done/冲突拒绝，原资源停止清理 | 原证明链、完整stop census；未知不伪装为已停止 |

NS-T03–T08每个叶子故障、每个目标30次；报告实际执行、失败、跳过及用例清单，保留首轮失败和新批次。重复不替代覆盖，旧240次或新增13项安装函数核查不计作本矩阵。

实现时把需求ID绑定实际测试名/case ID：优先扩展tests/catalog.json.focused_native_scenarios及scripts/test-native-input.ps1、原TestOrcaNativeStateLiveLifecycle，统一local/remote目标格式，不再强制Environment非空；取消segmented-shape旧门禁。新增正式case同步cases.json/manifest/目录校验、docs/testing.md及test-catalog.md；规划阶段不预注册未实现case。

## P5 项目检查与真实验证

实施完成后按scripts/test.ps1 -Level L1及受影响Controlled/L3正式入口完成检查，运行框架目录一致性校验及P4重复矩阵。当前只有文档变更，不把过去基线通过当成未来实现验收。

真实验证沿扩展后的既有输入入口，本机/远端各3个独立原生会话、全新隔离现场/证据目录，使用无副作用最小任务（只返回nonce，不写项目、不跑业务/Unity、不建业务PR）。同组样例覆盖多行/中文/空行/CRLF/长输入，并使用同一受测构建与契约。

发送前保存源码/EXE/CLI摘要、原目标/身份/完整清单和native基线；原请求及冻结正文先持久化。记录一次Runner通用提交、原accepted/started回执与必要的只读request-show结果、正式唯一原输入/完整final，最后精确关闭本次资源并用完整清单核验清理。

分别报告Runner RPC、mutation记录及provider历史输入次数；Orca内部按键/传输次数若公开接口不可观察则明确记不可观察，不能拿一次RPC反推一次Enter。请求未知时只读对账，不向历史Run重发或补Enter。缺真实条件记未验证，不用夹具通过或另一目标结果代替。

历史症状是否消失由这些新现场证据判断，不能因选择了正确入口就宣布修复。原始更强的原子全文消费/任意人工输入排他事务不在本次新增保证内，也不要求修改Orca取得它们。真实测试暴露的冲突/未知必须如实失败或未验，不静默绕过。

## P6 独立复审与交付

将最终diff、测试实数、真实原输入/final/清理证据及全仓删除清单交独立只读审计者，修正确认问题。更新原单独PR，不合并、不部署；未完成实施/真实验收前保持draft并明确未验项。

最终报告说明：Runner哪些屏幕依赖已删；可靠处理沿用哪些Orca既有契约及其边界；原身份/执行证明保留位置；稳定测试计数、六个真实会话结果及未验项；旧代码/测试/配置/文档清理情况与PR链接。不新增重试预算，不把旧失败改成通过。
