# Spec：零消费者生产入口与孤立夹具清尾

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：设计审计后已实施，OEC-00–04完成，见[专项验收](../acceptance/orphan-entry-cleanup.md)。基线：main `111bcd35bdfd7969d4c4575b5fe62204b41486fb`，已含 #84。依据[长期产品契约](../../../../spec/steerrunner.spec.md#长期产品承诺)，配套 [Plan](orphan-entry-cleanup.plan.md)。

## 1. 目标与收益

只实施本次审计建议 A、B：删除当前构建中没有实际消费者或合法执行入口的生产函数，以及不再由任何测试/helper进入的旧终端夹具。现行执行、恢复、原操作对账、持久提交和停止规则保持。收益为消除误导入口及维护面；生产和测试支撑分别统计，不把夹具删行计作生产收益，不宣称任务耗时或吞吐改善。

## 2. A：生产删除与保留范围

| 文件 | 可删除闭包 | 必须保留 |
| --- | --- | --- |
| backend/record_validation.go | ValidateExecutionOperation、仅由它调用的 StableOperationID、ValidateExecutionRef，以及独占 commitHash 和导入 | StableExecutionID 及真实问答、命令、请求身份消费者；LifecycleRequest.Validate、Receipt.ValidateFor 和当前 decoder 不变 |
| workflow/execution_facts.go | FactAdmission、FactStatus，整文件退役 | application.acceptEvent 的真实受理、去重、冲突和提交；实际 Workflow 转换不变 |
| adapters/codex/conversation.go、intent.go | rawObject、stringField、firstNonEmpty、仅由后者调用的 nonEmpty；PendingIdentity，两个整文件退役 | 当前 conversation Workflow、HistoryBinding、OpenHistory/ReadThread、InspectHistoryTurn 和共享 historyFixtureBinding |
| adapters/codex/lifecycle.go | cloneTurn | 当前 LifecycleSnapshot/Input/TurnInput 的复制、执行/恢复/观察/停止正文 |
| adapters/orcatransport/profile.go、workspace.go | NormalizeProfile、InspectWorkspace | DecodeProfile、Profile.Target/编码、原 workspace 创建/核验、InspectLifecycleAuditWorkspace |
| adapters/protocol/orca/client.go | FindRepo、其独占 ErrRepoUnavailable；ValidateWorkspaceTerminal 及独占导入 | 真实 resolveRepository 路由、InspectBoundTerminal/validateBoundTerminal、CLI envelope/超时和 close 事实核验 |
| application/engine.go、workspace/progress.go | runLogPath；VerifyProgressDirectories 包装 | 实际日志、目录身份核验 verifyFileDirectories 和 progress/result 初始化、读写 |

17个函数为上表封闭清单。禁止通过接入旧校验器、增加兼容包装或改写真实规则来实现清理。独占关系必须先全仓查实，再删除；若发现真实消费者，先收窄本表，不删除消费者或有效断言来制造无引用。独占导入只能在删除后确无使用时清除。

## 3. B：夹具边界

删除 internal/testkit/protocol/terminal.go 的 TerminalFixtureState、FakeTerminal 及同文件辅助闭包（292行），以及 native_lock_windows.go / native_lock_other.go 中无任何调用的 tryNativeOrcaStateLock 两个平台定义（共41行）。平台后缀造成同名定义不代表互为调用；删除前同时检查 Windows 与其他平台源码。

必须保留同包 appserver.go、staged.go、staged_test.go。MockAppServerStdio/Main 由 Codex mock_helper_cd67_test 使用；MockAppServerConnection/AppServerState 由 reattach_receipt_cd67_test 使用；StagedBusiness 由 tests/helpers/native/main.go 和 staged_test 调用。保留 tests/helpers/native 的正式 --orca-create-fixture 分支及 orca_creation.go、全部正式 native/integration 探针、历史证据和有效断言，不删除整个 testkit/protocol 包。

审计范围包含 internal/cmd/tests/scripts/client、所有 Go 构建标签、脚本和动态生成 helper 文本；不能仅以默认 go test 可编译作为零消费者证明。

## 4. 持久格式与可靠性不变

本批没有字段、类型形状、枚举值或动作的退役。state/v8、SQLite user_version=8、CLI v1、Workflow/profile/owner/native wire、Run/Record/Snapshot/Input/Request/Receipt/Operation、ImplementationDataEnvelope、HistoryBinding、Host/Probe/Delivery 持久字段和严格 decoder 全部保持。Intents 与 Snapshot.Operations 及各自更新语义保持。

不纳入建议 C 的 PreparationFacts.Cleanup/planner 分支，不删持久旧动作，不删 ObservationEvent 标签，不更改单确认、owner journal、真实原生事件选择或任何共享夹具。PC-R01–R06/R08/R09 的保存后派发、未知零重发、原结果绑定、同构建恢复、有界核实、持久 stop 撤销与精确停止范围保持；保留当前权限、签名、预算和用量资源清理。

实施前保存符号引用/合法入口、Spec/Plan 摘要、持久根及代表编码/公开能力基线；实施后比较全部变更 Go 文件的 AST 声明：只允许上表删除，无新增或修改的保留声明。未变源码/共享夹具逐字比较，加上真实回归共同支持格式和行为结论；单个零值编码相同不能替代根审计。

## 5. 最小验收

| ID | 必要结果 | 最小验证 |
| --- | --- | --- |
| OEC-00 | 设计先于实现，删除闭包无外部消费者；真实入口和共享夹具明确 | 保存引用/文件清单、设计审计及文档摘要；设计不通过不编辑生产 |
| OEC-01 | 持久格式和保留声明未变 | 前后持久类型/代表 Run、Snapshot、Request、Receipt、Operation、HistoryBinding/公开能力编码逐字一致；变更文件AST只删除授权声明，原测试文件与共享夹具正文相同 |
| OEC-02 | 实际组件与相邻可靠性回归通过 | execution/...、runstate/contracts/resources/storage/testkit/protocol；application 定向覆盖原操作/原资源、保存失败、单确认、预算、停止竞争、人工恢复和重启授权；明确顶层数量与skip，不新增照抄删除实现的测试 |
| OEC-03 | 正式消费者仍能构建和执行 | native/integration 的 live 编译，构建正式 native helper 与最终 Runner；同最终 EXE 的 NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership 三个公开受控场景3/3无skip，保留资源清理 |
| OEC-04 | 收尾可复核 | A/B分别记账；精确命令/退出码/失败/skip、源码与产物身份、Spec/Plan/长期实现说明、验收与索引齐备 |

源码冻结后才构建/运行，禁止边编译边编辑生产或测试。失败保留原记录，先修复确认问题再定向复验。上述资格不扩展为真实模型或完整 L1/L2/L3，不为纯删行重跑所有真机组合。

## 6. 停止条件与交付

OEC-00–04全部满足即完成。发现真实消费者、格式变化或保留行为不等价时，先修订设计并收窄删除，不能添加新恢复框架绕过。初轮授权到实施验证；后续用户已明确授权审计后提PR，按新增授权提交/推送和创建PR，不自动合并或部署。

## 7. 实施前设计审计

2026-11-02，在生产/测试差异为空时完成 OEC-00：检查774个源码/配置/脚本文件的33个候选符号，保存790项源文件身份及前编码。A的调用只在授权删除闭包内部；B无文件外入口。初轮原始名称扫描识别到 publication 包的同名 commitHash，按 Go 包作用域复核为不同变量，其 provider.go 消费者和整个 publication 实现保持；没有放宽真实消费者保护。MockAppServer、StagedBusiness、historyFixtureBinding 和正式 --orca-create-fixture 的消费者均保留。设计通过不代表运行验收已完成。

## 8. 实施与验收结果

按授权范围纯删除：A生产净减201行，B测试支撑净减333行，无新增或改写保留声明，实际测试及共享夹具未修改。13个变更Go文件中100个保留声明（含12个类型）相同，删除33个不同包符号；两个平台锁定义计为一个包符号。其余777项已跟踪源/配置字节不变，持久编码/类型/能力145116字节完全相同。C和持久旧动作等排除项未动。

最终447个组件及49个application顶层通过，8个组件skip和3个无测试包单列；live编译、正式helper/Runner构建和同一最终EXE的三个公开受控场景通过。首轮沙箱权限/回环连接/原生操作限制及超时保留原记录，同源码在正常测试权限下复验通过；审计工具的名称碰撞和符号计数修正另记，不改生产或断言。未提交、推送、提PR、部署或发布，无新增真实模型或完整L1/L2/L3资格。

同日按后续授权完成[实现再审计](../acceptance/orphan-entry-reaudit.md)：未发现产品缺陷，无生产/测试源码修复；修复Git证据换行保护遗漏，保证交付后SHA可验证。33个删除符号的入口、18个关键消费者、全部原测试/共享夹具及持久格式复核通过，本轮25个定向顶层通过。上段保留初轮交付状态，后续按授权提交并提PR。
