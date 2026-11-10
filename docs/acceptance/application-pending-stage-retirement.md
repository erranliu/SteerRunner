# 应用层旧阶段重建规则退役验收

2026-11-10，合并前最小范围验收。依据 [Spec](../spec/application-pending-stage-retirement.spec.md) / [Plan](../spec/application-pending-stage-retirement.plan.md)，按用户“独立子代理审计、修正确认问题，然后实施”授权完成。调查/实施主线 `dce4dff8e5be8529a2a1a1d6a300f8cfd01087c1`；实施开始的 HEAD `25f71fd62f53f072ef0f30d7f06dcc60d1af837e` 与该主线文件树相同。验收冻结时尚未提交、推送、创建 PR、合并或部署；用户随后另行授权提交并创建 PR。

## 独立审计

独立只读子代理先核对实际消费者、合法入口、删除/保留、三处迁移及最小验收，未确认需修订的设计问题。实施后同范围窄复核未发现越界删除、测试有效覆盖丢失或新阶段重建算法；确认真实 New 重载、Handle resume 受理、resumeRun 成功/lost/busy 对照、预算八组合及原回执消费后的零重发已落地。子代理未改文件、未运行测试，其结论不新增运行资格。

## 删除、保留及消费者迁移

生产仅改 feedback.go：删除 preservePendingStage 及独占注释/空行，净删 **12 行**。未改任何其它生产正文；没有新增函数、接口、回调、配置或兼容恢复器。

保留 PendingTurn/NextStage、queueInput/resumeWorkflow、New/Store.Runs、resumeRun/资格与人工对账、prepareRun/预算副本恢复、原接受/开始回执、意图与未知只读对账、授权/stop/事务/CAS、业务等待/里程碑和 parser 例外。原持久类型、字段/tag/codec、state/v8/user_version=8、能力和资源摘要保持。其它无引用声明、Orca CLI/nativeAccess 和共享测试装配未纳入本批。

| 消费者 | 原专属预期与最终覆盖 |
| --- | --- |
| TestSRResumeWorkflowRetainsOriginalReportSource | 退役“原冻结报告清 CommandID、清 PendingTurn、转 NextStage 再取得新命令”。现由 auditEngine/finishStage 和真实 queueInput 生成合法报告，保存后经真实 New 重载，再通过 Handle resume 受理和 resumeRun 核验。available 成功，lost/busy 分别按原人工对账/普通拒绝处理；每例恰一次原快照恢复查询，Execute/Reconcile/Stop 调用均为零。完整 Snapshot/Workflow/Intents/Result 保持、NextStage 不产生，控制命令/授权与 SQLite 回读一致 |
| TestFrozenBudgetLegalWorkflowEntries | 保留原八项 Workflow/adapter、queueInput/预算/SQLite 断言；六项非 content 尾部保留原 PendingTurn，清空测试副本 PromptBodies 后直接测资格之后的 resumeWorkflow 转换。独立深拷贝核对原身份、InputVersion、正文/预算/来源、模型/策略与整个 Workflow 不变，无新增 StageInputs/NextStage；不冒称八条完整恢复或真实后端执行 |
| TestRARecoveryConsumedInput | 移除旧函数前置，通过真实 merge/ApplySessionReceipt 的原接受/开始回执消费 PendingTurn，再使用原已结算 turn 事实，经 Handle/resumeRun 验证原业务保持 blocked、恢复命令 completed、原 Snapshot/Workflow/Intents 不变且零 Execute/Reconcile/Stop；结果真实持久提交 |

原 TestRAResumeConsumedActiveInputWaitsForOriginalEvidence 未改，继续验证 reconciling/original_result_pending、一次原 final 读取和零业务发送；与上述已结算 blocked 及 unknown 对账分别举证。原后继草稿首次冻结、反馈覆盖未发后继、预算副本精确修复及缺源/错绑定/已有意图/保存失败拒绝、原结果与 stop 不被缺预算阻断、停止抢占恢复等既有测试与断言保持。

## 基线、原失败与修正

未改代码时，原三项 **3 个顶层、11 个执行节点**通过，无失败/skip，见[原基线](evidence/application-pending-stage-retirement-20261110/original-baseline.jsonl.gz)。生产旧函数仍保留时完成三处迁移，最终同三项 **3 个顶层、14 个执行节点**通过（新增报告 available/lost/busy 三子例），无失败/skip，见[最终迁移基线](evidence/application-pending-stage-retirement-20261110/migrated-baseline-final.jsonl.gz)。随后才删除生产旧函数。

迁移过程保留三份失败记录，不改记为通过：

- [迁移首轮](evidence/application-pending-stage-retirement-20261110/migrated-baseline.jsonl.gz)：新测试把 Snapshot.Clone 后的空 Operations 切片与原 nil 表示直接 DeepEqual，导致报告和六项非 content 比较失败。改为两侧使用同一 Clone 语义，并为 Workflow 比较取得独立深拷贝；未修改 Clone 或生产恢复。
- [首轮修正后](evidence/application-pending-stage-retirement-20261110/migrated-baseline-fixed.jsonl.gz)及[lost 定向诊断](evidence/application-pending-stage-retirement-20261110/lost-diagnostic.jsonl.gz)：测试错误要求所有拒绝均为 control_rejected；实际 lost 由 requireManual 及 commitLocked 保持 manual_intervention_required。按原规则分别断言 lost 的 ManualPause/人工对账与 busy 的普通控制拒绝，保留原来源、预算及零外发检查；未放宽生产资格。

另有一次编辑补丁因同文件重复操作被工具拒绝，未应用；证据命令提取脚本首次遗漏缩进代码围栏，修正后生成完整清单。两者不属于产品测试通过或产品故障，未改写原运行日志。

## 原字节和范围核对

[一次性捕获程序](evidence/application-pending-stage-retirement-20261110/capture.go)在实施前后各独立运行，保留生产类型声明、固定代表 Record/Workflow/StageInput/TurnInput/Budget/Operation/Receipt 编码及 JSON 往返。样本分别包含冻结待发报告、未渲染后继、已有冻结 NextStage、已消费、未知和 stop；这些是固定编码样本，不冒充完整合法业务执行，业务接线由上述应用测试举证。

[前](evidence/application-pending-stage-retirement-20261110/before-capture.json.gz)/[后](evidence/application-pending-stage-retirement-20261110/after-capture.json.gz)均为 **288,929 字节**，完全相同，SHA256：

`5436f793b82c5d7c335fe819bf673bcac2592c8fc81dc5c9a7e3ff95ab555e70`

resource_digest=`910c7fa06b933be3632f862062e966200fd581b34873a395268826d44c1bbbe7`，capabilities/schema 不变。对照 808 个源码/测试/JS/脚本文件，仅批准的 feedback.go 与三个测试文件变化，其余 **804 文件原字节相同**；feedback.go 扣除批准删除后原字节完全相同，见[范围核对](evidence/application-pending-stage-retirement-20261110/scope-audit.json)。最终源码/测试/工具清单 SHA256=`0c1813d764a740344bfba74f26e0938b217dfbe61e0cf6de36d078006092a419`。

## 实际最小验收

Windows/amd64、PowerShell 7，普通宿主隔离现场；所有 Go 测试 count=1。[命令](evidence/application-pending-stage-retirement-20261110/commands.txt)、[汇总](evidence/application-pending-stage-retirement-20261110/summary.json)及原 JSON/stderr/退出码分别保留。

| 范围 | 实际结果 |
| --- | --- |
| application 精确集合 | 18 顶层、63 执行节点通过，无失败/skip；实际名字与 Plan 完全一致 |
| 完整 workflow | 19 顶层、27 执行节点通过，无失败/skip |
| 完整 reliable | 10 顶层、30 执行节点通过，无失败/skip |
| runstate | 编译成功，无独立测试文件；Go package skip 单列，不计通过用例 |
| 正式 EXE | scripts/build.ps1 成功；受控 Runner SHA 与正式产物相同 |
| test-foundation 单元前置 | 四包、5 个顶层通过，无失败/skip，重复 TestSRCLIContract 按所在包分别计数 |
| 通用公开受控 | TestSRNativeUnknownAndStop 14.59 秒通过；TestSRRestartPreservesAuthorization 的 graceful 21.34 秒、kill 20.87 秒通过；最终 exit_code=0、scenario_count=2，无 skip |

公开受控已等待最终回执；它沿原通用本机链路验证 unknown 原 dispatch 一次且零重放、stop 后零新输入，以及普通/强杀服务后的原 owner/session/授权和原结果恢复。creation_receipt/pending_stop 未选择，不计通过；报告来源/预算的特定接线由应用测试证明，不以公开场景代替。

正式构建 source_commit=`25f71fd62f53f072ef0f30d7f06dcc60d1af837e`、source_dirty=true、source_digest=`c2dacc922c5a053e7c66c9c5949c436db7500ab5eedd1fa9a92d538be262e49d`，EXE SHA256：

`df1ffd5465b13b438b6505e060911c674a82ee0483b1cfc000d83fd79d35afe2`

[构建身份](evidence/application-pending-stage-retirement-20261110/build.json)、[受控身份](evidence/application-pending-stage-retirement-20261110/controlled-source-identity.json)、[最终回执](evidence/application-pending-stage-retirement-20261110/controlled-result.json)与[原日志](evidence/application-pending-stage-retirement-20261110/controlled-integration.log)保留；完整产物与现场仍在本副本 `.evidence/application-pending-stage-retirement/`，未替换安装文件。

## 结束与资格

PSR-01–06 完成。仅取得合并前应用定向、三个共享组件入口和上述两项通用公开受控父测试的最小资格；未运行完整 application、完整 L1/Controlled、L2–L4、全恢复、真实 Orca/paired/模型/PR 或 Node，不扩大既有资格。收益是删除一套无生产接线的输入重建规则，测试增加真实接线覆盖，不声称全仓净删或提速。

Spec/Plan、索引与长期实施状态同步；原证据按 .gitattributes 保持原字节，见 [SHA 清单](evidence/application-pending-stage-retirement-20261110/SHA256SUMS.json)。本批不自动开展其它精简，不提交/推送/PR/合并/部署。

## PR 交付补记

2026-11-10，用户在验收后授权创建 PR。交付分支从刷新后的 main 建立，808 个源码/测试/工具文件与最终验收清单逐项一致，44 份原证据哈希核对通过；本次提交包装沿用原最小范围结果，不因分支或提交身份变化重跑。原构建与最终一致性记录保留捕获时的 source_commit、source_dirty 和交付状态，不回填为提交后构建或合并后资格。本次授权不包括合并或部署。
