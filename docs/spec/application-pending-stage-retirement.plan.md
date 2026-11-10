# Plan：应用层旧阶段重建规则退役

日期：2026-11-10。状态：独立设计审计完成，按用户后续授权实施及合并前最小验收已完成，见[独立记录](../acceptance/application-pending-stage-retirement.md)。依据 [Spec](application-pending-stage-retirement.spec.md)，调查主线 `dce4dff8e5be8529a2a1a1d6a300f8cfd01087c1`；工作副本 HEAD `25f71fd6` 文件树相同。原设计轮仅写文档；用户后续已授权独立审计并实施，实施验收阶段未授权提交、PR、合并或部署；用户随后另行授权提交并创建 PR，未授权合并或部署。实际结果见独立验收，原失败不改记为通过。

## P0：实施前基线与原断言捕获

- [x] 获得后续实施授权后刷新 main，记录 HEAD、目标提交与工作树差异；有上游变化先核对实际内容。沿 AGENTS.md 处理真实冲突，不为 Rebase 提示改写已验证历史。
- [x] 全仓复查 preservePendingStage 的直接/函数值引用及各 build tag、工具/脚本；核对仍仅三处测试消费者。复查 New/Store.Runs、worker.advance/resumeRun、resumeWorkflow、applyWorkflowTransition、prepareRun、merge/ApplySessionReceipt 的真实接线，发现新调用根先修订 Spec。
- [x] 保存 feedback.go 与三个原测试文件的原字节及 SHA，逐条登记原断言。特别标明旧专属“清 CommandID→NextStage→重排”预期退役，原来源/策略/预算/已消费零重发判据迁移保留；不以“全部原断言不变”掩盖实际预期替换。
- [x] 捕获保留生产声明和固定 Record/Workflow/StageInput/TurnInput/Budget/Operation/Receipt 代表编码：原待发报告、未渲染后继、已有冻结 NextStage、已消费/未知/stop 状态均有固定身份样本；检查 marshal/decode 错误，不忽略错误制造空字节相同。另保存 capabilities/resource_digest，以及其余生产、共享测试/driver/helper/JS/脚本摘要。只用一次性证据工具，不新增产品审计框架。
- [x] 先运行原三项基线并保存 JSON、stderr、退出码、全部子例和失败/skip；八项预算组合必须实际匹配，原失败不得改记通过：

  ```powershell
  go test ./internal/application -run '^(TestSRResumeWorkflowRetainsOriginalReportSource|TestFrozenBudgetLegalWorkflowEntries|TestRARecoveryConsumedInput)$' -count=1 -timeout=10m -json
  ```

## P1：先迁移三处测试，保留生产旧函数作对照

1. feedback_test.go：由现有 auditEngine/finishStage 及真实 queueInput 产出完整合法冻结报告；在尚未有报告外发意图时保存。使用真实 Engine.New 重载同一 Store，核对原数据/授权/游标不改写、零外发。不得启动后台 workers 或调用 Shutdown 引入测试之外的业务停止；按现有测试方式取消新 Engine 上下文并关闭自有资源。
2. 经现行提交路径设置显式恢复控制，在 worker gate 下调用 resumeRun，使用计数隔离端口核验原资源。测试同入口正例及原资源不可用/输入资格不足拒绝；验证业务输入和来源/策略/预算/模型/版本不变、无新阶段/NextStage/执行/创建，以及真实存储的命令与授权结果。区分控制事务允许的 revision/时间变化与不可变业务事实；不得绕过恢复资格、伪造返回值或新增生产 seam。
3. frozen_budget_test.go：原八项 Workflow/adapter 组合及前半段 queueInput/预算/SQLite 断言保持；六项非 content 尾部保留原 PendingTurn，移除旧函数/NextStage 重排前置，测试清空副本 PromptBodies 后 resumeWorkflow 不改原 CommandID/AttemptID/InputVersion、正文/预算来源、策略/模型及 StageInputs。该纯转换测试不冒充真实重启/端口接线。原草稿首次冻结、副本精确修复和各拒绝矩阵均保留。
4. result_acquisition_test.go：TestRARecoveryConsumedInput 从现有接受/开始回执转换形成已消费原输入，使用原已结算 turn 事实验证 blocked 分支；经 resumeRun 与 Store 回读证明无再排队/业务发送、无操作身份重写。保留活动 consumed 原 final 只读测试及原 unknown/stop 回归，不以一个空 NextStage 断言代替转换证据。
5. 原三个测试名字优先保留；需要补充装配/计数时限于批准文件或明确的新窄测试，不复制旧重建算法，不改共享 fixture 默认行为。新名字必须同步到 Spec/命令与实际集合，零匹配不能判通过。先在生产旧函数仍存在时重跑 P0 同一三项，保存迁移基线；失败先核对前置与合法来源，不放宽生产规则。

## P2：删除批准闭包及静态对照

- [x] 三处迁移通过后，删除 feedback.go 的 preservePendingStage 及独占注释；本基线其它 imports 仍有用途，无预计可删 import。除批准删除外生产正文保持逐字一致。
- [x] 全仓核对旧符号无调用/函数值入口，无同义 helper、PendingTurn 复制到 NextStage 并清 ID 的替代规则或新的重绑执行器；历史证据文本不作运行引用，也不改写。
- [x] 核对 resumeWorkflow/queueInput、真实 NextStage 与已冻结 NextStage 原处理、模型/提示词/预算、原输入消费/未知、授权/stop/事务/错误顺序、parser 例外全部保持。controlCheckpoint/initialTerminalCheckpoint/publication.Error、Orca CLI/nativeAccess 等其它候选不进入 diff。
- [x] 前后独立捕获原保留声明和代表编码/静态摘要，固定身份逐字节比较。真实测试新增的随机 ID、时间、临时目录和产物 SHA 分别记录，不要求新旧测试现场整体 Record 字节相同，不用新值回填旧证据。

## P3：最小运行验证

使用仓库根目录 PowerShell 7、Windows/amd64。以下实施清单已按用户后续授权执行，实际集合、原失败和最终结果见独立验收。所有 Go 测试 count=1；保存实际顶层与子例集合、退出、失败及 skip，普通宿主复验不抹去沙箱/环境失败。

### P3.1 应用精确集合

```powershell
go test ./internal/application -run '^(TestSRResumeWorkflowRetainsOriginalReportSource|TestFrozenBudgetLegalWorkflowEntries|TestRARecoveryConsumedInput|TestFrozenBudgetSuccessorsAndDraftResume|TestFrozenBudgetCopyRequiresOriginalUnsentBinding|TestFrozenBudgetUnknownReconcileAndStopDoNotRepair|TestFrozenBudgetMissingDoesNotGateOriginalResults|TestFrozenBudgetFirstRenderCommitFailureDoesNotPublish|TestServiceGenerationPreservesDurableAuthorizationAndBusinessState|TestRAResumeConsumedActiveInputWaitsForOriginalEvidence|TestRAFeedbackSupersedesUnsent|TestSRManualReconciliationExplicitRetryReplayAndLostResource|TestSRManualReconciliationStopWinsDuringRead|TestSRManualReconciliationCommitFailureAndRestart|TestSRManualLostResourceDuringContinuation|TestSROperationIntentCommitFailureDoesNotDispatch|TestSROperationReceiptCommitFailureReconcilesOriginal|TestStopContractKeepsWorkflowAndSavedScopeRules)$' -count=1 -timeout=10m -json
```

基线上述 18 个顶层名字均存在，要求逐项实际执行，保留原全部子例；三处迁移不新增顶层名字时仍为 18 项，不预报执行节点数。报告正/拒绝、重载、控制命令提交和计数必须出现在迁移后日志与断言中。原 copy/拒绝矩阵、草稿、活动 consumed、未知意图、保存失败/重启及 stop 竞态分别核对，不能只满足命令退出 0。这不是完整 application 或完整恢复矩阵。

### P3.2 完整共享语义组件

```powershell
go test ./internal/execution/workflow ./internal/execution/reliable ./internal/runstate -count=1 -timeout=5m -json
```

覆盖原定义/来源、输入/回执合入和 Record 投影/clone 等现行组件；某包没有测试或出现 skip 按实际登记。应用测试使用原 SQLite Store 完成真实往返和保存失败；不因文档列举了持久类型就自动扩大为完整 storage/host/全 adapter 测试。

### P3.3 同一最终 EXE 的公开受控

冻结生产及测试点后正式构建；使用本批全新目录，固定同一 RunnerExe，不复用上一批 EXE：

```powershell
./scripts/build.ps1 -OutFile .evidence/application-pending-stage-retirement/final/steerrunner.exe
./scripts/test-foundation.ps1 -RunnerExe .evidence/application-pending-stage-retirement/final/steerrunner.exe -Pattern '^(TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -Case 'TestSRRestartPreservesAuthorization/graceful','TestSRRestartPreservesAuthorization/kill' -OutDir .evidence/application-pending-stage-retirement/controlled-new
```

每条独立检查退出，不让后续成功掩盖构建失败。保留脚本原单元前置、integration driver/helper 编译、真实本机 EXE/CLI/IPC/SQLite/Git、宿主超时、精确选择及清理。等待最终 result.json/integration.log，要求 exit_code=0、scenario_count=2、两个父测试及 graceful/kill 两个实际 case 均通过且无 skip；核对 controlled_cases 选择与实际回执，不以脚本启动返回代替完成。

核对原 unknown 外发一次后零重放、stop 后零新输入；普通停止服务/强杀服务后的原 owner/原 session 与授权恢复、原输入不重发沿原断言执行。creation_receipt/pending_stop 未选，不登记通过；不修改受控夹具或断言来适配本批。报告原来源与预算由应用定向举证，公开场景只是原通用本机恢复/停止的补充，不代表真实 Orca/paired/模型、完整报告流程或全部恢复资格。

记录正式 EXE、受控 driver/helper SHA、source_commit/source_dirty/source_digest 和最终源码/测试清单；固定资源摘要不变与源码/产物身份正常变化分别表达。此最小范围不授予完整 L1/Controlled、L2–L4、真实模型/PR、Node 或部署资格。批准范围未扩大时不自动追加 live driver/helper/Node/全部 application；新改动、实际失败或未解问题再按影响补选，不重复已通过项求次数。

## P4：独立验收与结束

- [x] 在 docs/acceptance/application-pending-stage-retirement.md 独立记录 PSR-01–06；列明精确生产删除、三处测试旧/新断言映射、真实入口与 I/O 计数、编码/摘要、实际集合/命令/失败/skip/未验及最终产物身份。若需提交原证据，按仓库 .gitattributes 规则保护原字节，不回填历史构建。
- [x] 核对生产仅批准闭包，三处测试有效覆盖不丢失，新增正反例不因无效前置走错分支；原授权、原来源、控制事务允许变化与禁止改写分别列明。失败根因及后续通过分开登记，未跑不能写通过。
- [x] 实际验收完成后才更新 Spec/Plan 与索引状态，必要时给长期契约追加实施事实；原设计轮未预写实现完成，后续按实际结果更新；不改中央 README 验收表、不做归档/目录大整理。
- [x] PSR-01–06 完成即结束，不自动提交/推送/创建 PR/合并/部署。若后续另获 PR 授权，创建与最终交付前刷新目标并核对 GitHub 实际内容合并状态，按 AGENTS.md 优先 Squash and merge，不因 Rebase 提示改写已验证历史。

## P5：实际执行说明

独立设计审计与实施窄复核均未确认需修订问题。P0 原三项通过；迁移初轮的 Clone 表示差异、后续 lost 断言与原人工对账投影不符的失败分别保留，修正后 P1 三项通过，再删除生产旧函数。P3 应用精确十八项、完整 workflow/reliable、runstate 编译及同一正式 EXE 的两个公开父测试/重启两子例完成；runstate 无测试文件不计用例通过。最终日志、字节/摘要和资格边界见独立验收。验收冻结时尚未提交/推送/创建 PR/合并/部署；用户后续 PR 授权与交付见验收补记。
