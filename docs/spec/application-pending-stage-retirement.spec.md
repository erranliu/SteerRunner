# Spec：应用层旧阶段重建规则退役

日期：2026-11-10。状态：独立设计审计完成并已实施，合并前最小验收完成，见[独立记录](../acceptance/application-pending-stage-retirement.md)。原设计轮仅文档；实施验收阶段未包含提交/推送/创建 PR/合并/部署，随后用户另行授权提交并创建 PR。调查主线 `dce4dff8e5be8529a2a1a1d6a300f8cfd01087c1`（#135 已合入）；工作副本 HEAD `25f71fd6` 与该主线文件树相同。配套 [Plan](application-pending-stage-retirement.plan.md)，依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[输入冻结与预算](workflow-input-budget.spec.md)、[结果获取](result-acquisition.spec.md)及[测试指南](../testing.md)的现行条款，历史设计不覆盖当前合同。

## 1. 目标与边界

`internal/application/feedback.go` 的 preservePendingStage 没有生产调用，仅三处测试调用。它将尚未成为 CurrentCommand 的 PendingTurn 对应 StageInput 浅复制到 NextStage，清空 CommandID；后续 resumeWorkflow 经 queueInput 又生成命令身份。该机制不是当前服务重启或显式 resume 的接线，却使开发者需要判断另一套“丢弃原待发输入、重新排队”的恢复规则。

本项迁移三处测试后删除该函数及独占注释，减少恢复语义和搜索范围。现行恢复、派发、停止和持久格式保持；不将旧算法改名、搬入测试 helper 或加入真实生产入口，不承诺任务提速、EXE 缩小或全仓净删。

本项退役的是无人接线的重建算法及其专属测试预期，不是尚未发送输入的恢复能力。已冻结 PendingTurn、真实后继 NextStage、已消费输入必须分别举证，不得互相转换来制造测试通过。

## 2. 实际消费者及合法入口

| 对象 | 基线事实 | 处理 |
| --- | --- | --- |
| preservePendingStage | 仅 feedback_test.go 的 TestSRResumeWorkflowRetainsOriginalReportSource、frozen_budget_test.go 的 TestFrozenBudgetLegalWorkflowEntries、result_acquisition_test.go 的 TestRARecoveryConsumedInput 各一处调用 | 三处迁移后删除函数和紧邻两行独占注释；复核函数值、脚本/工具和其它 build tag 消费者，不只查调用括号 |
| Engine.New | Store.Runs 严格读取原 Record，保存原授权、待处理控制、业务等待及游标；不调用旧函数 | 保留；用真实存储往返和至少一处真实 New 重载验证未发送冻结输入，不把 Record.Clone 当作服务重启 |
| worker.advance → resumeRun → qualifyContinuationContext → resumeWorkflow → commitLocked | 显式恢复先处理未知操作和原资源资格，重锁复核 revision/控制，再提交授权及命令结果 | 保留全部顺序、错误与预算；迁移后至少一处正例真正走 resumeRun，不能仅以直接调用 resumeWorkflow 宣称恢复接线已覆盖 |
| applyWorkflowTransition → NextStage / queueInput | 原结果转换可以保存尚未授权派发的后继草稿；原 queueInput 负责首次冻结或校验已有冻结内容 | 保留；与已排队 PendingTurn 分开测试 |
| prepareRun / ensureStageBudget | 校验原待发输入身份、正文与原 Budget；仅精确绑定且没有原输入意图的缺预算副本允许复制并先提交 | 保留原正反例，不补算原 Budget，不借本项修改准备/派发算法 |
| merge / reliable.ApplySessionReceipt / 原操作对账 | 由原接受/开始回执消费 PendingTurn；未知沿原操作核实，已接受原结果继续观察 | 保留；消费负例优先使用这些现行转换，不用旧重建函数作测试前置 |

删除前若出现新增真实消费者，先修订范围；不能删除调用者、改 build tag 或降低资格条件来制造零引用。生产允许差异仅为 feedback.go 的旧函数及其独占注释删除，无新生产函数、字段、接口、回调或配置。

## 3. 删除与保留清单

| 删除/替换 | 明确保留 |
| --- | --- |
| preservePendingStage 的函数体、独占注释，及迁移后确认独占的 imports（本基线无预计独占 import） | feedback.go 的其它正文及 imports 实际用途，包括 revokeDevelopmentInputs、feedbackValue、resumeWorkflow 和 workflow/runstate 依赖 |
| 三处对旧函数的调用及“冻结待发输入必须变成无 CommandID 的 NextStage、重新获得 CommandID”专属断言 | 三个原测试的有效合同：报告原来源/策略、冻结正文/预算、原会话/Attempt、已消费输入不重发；旧专属预期为何替换必须逐项登记 |
| 为调用旧函数而手工清空 PendingTurn、模拟资源重绑的测试装配 | PendingTurn、NextStage、StageInputs/Results、InputVersion、ReportToolPolicy、SourceCommandID/TurnID/Version/ResultID 的真实字段与校验；不做字段清尾 |
| 不新增同义阶段重建 helper 或通用恢复框架 | queueInput 对已冻结/未渲染输入的原规则、后继草稿和既有已冻结 NextStage 的原处理、反馈覆盖未发后继、业务等待/里程碑及 parser 例外 |

Engine.New/Store.Runs/commitLocked、revision CAS、受理命令、resumeRun/manual/reconcile、worker gate、原 operation/receipt、授权及 stop 竞态均保留。原模型选择、文件通道指令、原冻结 PromptBodies、预算来源/计量、缺预算拒绝、历史来源读取保持，不从当前目录、默认提示词、模型或新会话重建原输入。

共享 auditEngine/finishStage、operationEngine、reportPolicyPort、原 store/SQLite 与受控夹具继续供原消费者使用。不全局重构测试装配；本批可在三个消费者文件内增加最小只读计数端口或装配 helper，但不得包含旧 PendingTurn→NextStage 复制/清 ID/替代会话算法。除批准三文件和必要新定向测试外，其它原测试不删不改；发现必须扩大时先说明原因并修订本稿。

controlCheckpoint、initialTerminalCheckpoint、publication.Error/providerError、Orca CLI 恒真回调、nativeAccess、提示词包装和 Record/operation 重构均不进入本批。历史 evidence、既有验收和长期产品条款不因本项退役而改写。

## 4. 三处测试消费者迁移

### 4.1 报告来源：原冻结待发输入沿真实恢复保留

TestSRResumeWorkflowRetainsOriginalReportSource 保留原名，改用现有 auditEngine/finishStage 的真实结果转换和 queueInput 形成合法报告输入，包含完整原来源、策略、正文、Budget、模型与版本。暂停点在报告已冻结排队且尚无其外发意图时；用 commitLocked 保存，真实 Engine.New 从同一 Store 重载，不启动后台派发。不通过删除 PendingTurn 或改为新 session 模拟恢复。

先核对重载前后整个冻结输入/原来源、原 session/Attempt、授权与待处理控制一致，且重载零执行/创建/发送；重载自身不能制造 revision 或新输入。随后沿现有提交路径保存显式 resume 控制，在 worker gate 约束下调用 resumeRun；用现有隔离恢复端口返回原会话可用/idle 正例，并记录核验次数。成功后 resume 命令 completed、授权按原规则恢复且结果持久，原报告 CommandID、AttemptID、InputVersion、Prompt/Budget、Source*、ReportToolPolicy、模型保持，StageInputs 数量不增加、NextStage 不产生，原 Result/操作不改写；允许原 resume 必需的 revision/时间/控制状态变化，不要求整个 Record 在显式恢复后字节不动。

同入口至少配一个原资源不可用或不满足输入资格的拒绝子例，使用同一原冻结报告前置。要求无新输入/替代资源、原冻结数据与意图不变，允许原 failControl/requireManual 的诊断及控制状态提交。不能以没有合法报告输入导致的前置错误替代恢复资格拒绝。原“生成新 CommandID”断言明确退役，不能为保留它改生产恢复。

### 4.2 冻结预算：保留原待发输入，而非转成 NextStage

TestFrozenBudgetLegalWorkflowEntries 保留 development、development-audited、execute 的 Codex/Orca 两分支及 conversation/generate 的 Codex 分支，共原八项组合。queueInput、预算有效性、SQLite 往返、StageInputs 不变的原断言全部保持；既有非 content 尾部改为保留从 Store 读回的 PendingTurn 和 StageInput，直接验证现行 resumeWorkflow 对原排队输入不重建、不再冻结、不增加 StageInputs、不产生 NextStage。

保留原六项非 content 的“清空测试副本 PromptBodies 后原正文/预算不被重渲染”扰动检查，并增加 CommandID/AttemptID/InputVersion、原 Budget 内容与来源及模型/策略一致断言；扰动只作用于测试副本，不改原数据库与冻结输入。该直接函数调用只证明资格核验之后的业务转换，不能冒称完整 resume 或两个后端真实执行通过；真实 New/resumeRun 接线由 §4.1 补足。

NextStage 的现行正路径另由原 TestFrozenBudgetSuccessorsAndDraftResume 保留：结果转换产出未渲染后继，首次排队才能产生自己的命令/预算。原草稿测试不能替代原待发输入的身份保持；原预算精确副本修复、缺源/错绑定/已有意图/保存失败拒绝、原结果与 stop 不被缺预算阻断仍由原测试逐项保留，不减少矩阵。

### 4.3 已消费输入：从原回执事实验证零重发

TestRARecoveryConsumedInput 保留原名和“已消费原实施不重新成为未发送输入、无后继时保持 blocked”的有效判据。移除旧函数调用；通过现有 merge/ApplySessionReceipt 的合法原输入接受/开始事实形成消费状态，再以原已结算 turn 的事实构造该 blocked 分支，核对 PendingTurn/NextStage 均为空、StageInputs/原来源和操作身份不变。用计数隔离端口沿 resumeRun 验证恢复不调用 Execute/创建/业务发送，结果经真实 Store 回读确认；不能只删旧调用再检查本来为空的 NextStage。

保留原 TestRAResumeConsumedActiveInputWaitsForOriginalEvidence：原活动 turn 恢复后为 reconciling/original_result_pending，advance 只读一次原 final，原输入发送计数为零。另由原 unknown/receipt 保存失败对账及 stop 测试核对“不确定不当作已消费成功、不清意图、不重发”。上述已结算、活动及未知三个状态不得混为一个 blocked 断言，不新增替代执行器或恢复算法。

## 5. 不变量与最小验收

PC-R01–R06/R08–R10 与 PC-C02/C03/C07 保持：原操作先提交、未知零重发、同构建重启保留原授权/输入、显式恢复有界核验、停止撤销优先。其它长期承诺不变；parser 的既有例外不变。state/v8、SQLite user_version=8、Record/NextStage/Workflow/StageInput/TurnInput/InputBudget/Operation/Request/Receipt 的全部字段、tag、codec 和实际持久读写不改；不迁移旧数据、不引入跨构建接管。

| ID | 必须证明 |
| --- | --- |
| PSR-01 | 生产仅删除旧函数及独占内容；所有真实入口/保留声明保持，三处调用消失，无改名重建或新旁路 |
| PSR-02 | 合法冻结报告经真实存储、New 重载和 resumeRun 正/拒绝对照；原输入、来源、策略和身份不变，无多发/新建，控制结果真实提交 |
| PSR-03 | 原八项预算组合保持，六项非 content 尾部改测原 PendingTurn 不重建；后继草稿、预算副本恢复/拒绝、结果与 stop 的原覆盖保持 |
| PSR-04 | 已结算 consumed 不重排、活动 consumed 只收原结果、unknown 只原操作对账；停止和恢复竞态无新输入 |
| PSR-05 | 保留声明与固定代表编码前后相同，固定 capabilities/resource_digest 不变；Plan 的应用精确集合、共享组件及同最终 EXE 的公开受控路径完成，原失败/skip 如实登记 |
| PSR-06 | 实施后在 docs/acceptance/application-pending-stage-retirement.md 独立记录原/新断言映射、入口/计数、命令/实际集合、源码/产物身份、失败/skip/未验范围，再更新设计状态 |

最小运行范围为三处原消费者基线、迁移后应用定向（含保存/重载/恢复资格/预算/未知/stop）、完整 workflow/reliable/runstate 组件，以及同一正式 EXE 的 TestSRNativeUnknownAndStop 和 TestSRRestartPreservesAuthorization（仅 graceful/kill）。具体命令见 Plan。公开受控使用原通用本机夹具，不证明报告特定来源或真实 Orca/模型端到端；报告与预算由应用测试举证，不以公开场景替代。

原设计轮只交付文档；独立审计未确认需修订问题，随后按新增授权实施。原失败、最终通过及实际资格见独立验收。最小范围不代表完整 application、L1/Controlled、L2–L4、全恢复或真实模型/PR/Orca 通过。若实际变更触及保留生产逻辑或共享夹具，先修订范围并按影响补验，不能靠最小清单掩盖新增影响。PSR-01–06 完成即结束，不自动开展其它精简或提交/推送/PR/合并/部署。

## 6. 独立设计审计

2026-11-10，独立只读子代理核对实际消费者、合法入口、三处测试迁移及最小验收，未确认可操作设计问题；三项基线和最终十八项名字、共享组件路径、受控父项与 graceful/kill 选择均静态核对。子代理未编辑文件、未运行测试，设计审计不新增运行资格。

## 7. 实施状态

2026-11-10，PSR-01–06 已完成。生产仅删除旧函数及独占内容 12 行，三处测试消费者均按本稿迁移；设计与实施独立复核均未确认新问题。原三项基线通过；迁移中的 Clone 比较和 lost 投影预期失败分别保留，修正后在生产旧函数仍存在时三项通过，再删除旧函数。最终应用 18 顶层及 workflow/reliable 共 29 顶层通过，runstate 无测试文件单列；正式同一 EXE 的 unknown/stop 和 graceful/kill 两个重启子例通过，保留声明与代表编码前后相同。详见[验收](../acceptance/application-pending-stage-retirement.md)。仅取得合并前最小范围资格；验收冻结时尚未提交/推送/创建新 PR/合并/部署，后续 PR 授权与交付见验收补记。
