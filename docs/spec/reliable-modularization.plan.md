# Plan：SteerRunner 可靠执行中间层模块化重构

日期：2026-10-15。状态：本次范围 RM-P0–RM-P3 及后续审计修复均已完成并复验，含 RM-P2 所选整理。行为依据 [Spec](reliable-modularization.spec.md)，实际结果见 §6、[收尾证据](../acceptance/evidence/reliable-modularization-20261015.md)及[审计记录](../acceptance/evidence/reliable-modularization-audit-20261015.md)。不要求五项职责全部成为独立包，但要逐项核对收尾。

## 1. 基线与范围

源码调查基线为 `af545563c58171c584bd35d098166cf39012e699`。实施前核实实际源码/文档版本、工作树差异及与基线的变化；不修改他人在途代码或用户状态目录。

保持现有 Engine、Run Record、Store.Commit、锁/gate、CLI 和 v2 SQL/JSON 读写行为。首版历史验收见[原验收文档](../acceptance/steerrunner.md)，不计入本次实现通过证据。

不建设统一 View/Change/Decision、资格注入框架、操作投影重建/加载修复、通用停止/清理策略或旧二进制读回测试平台。复用现有类型和测试，只增加实际边界需要的函数、类型与反例。

## 2. 迁移清单

| 来源 | 第一轮处理 | 后续处理条件 |
| --- | --- | --- |
| reliable 的 operation/effect/uncertainty、intent 中固定输入规则 | 提取到 `reliable/operation`，复用 LifecycleRequest/Receipt、EffectResult 等值 | 迁移实际调用者和相关测试，不留重复实现 |
| application.saveIntent/invoke、stop 中已有操作更新 | 统一操作构造/状态/回执更新函数，宿主保留锁和提交 | 不把整个 worker/control 都搬入 operation |
| preparation 与 operation 紧密相关的小函数 | 直接复用或随必要依赖小范围移动，保持资格检查顺序 | 以无环且调用简单为准，不为拆包增设注入层 |
| binding/turn、事件/结果核验、授权/恢复 | 第一轮保持当前位置和现有调用，明确逻辑职责 | 有具体重复规则或交叉修改时再选最小一处提取 |
| Workflow 路由/反馈、材料/提示词、PR 核验 | 保持 application 业务衔接 | 仅整理第一轮实际触及处，不扩大为业务层重写 |
| Snapshot.Operations、Record、存储加载/编码 | 保持现有用途与表示；集中更新不改变读侧规则 | 历史投影治理如有需要单独计划 |
| StopLifecycle/ShutdownLifecycle、观察/进程句柄清理、权限组装 | 保持现有生命周期/adapter 路径 | 不改造成通用 operation 派发策略 |

迁移某个函数时同步调整它的消费者和测试；原入口可直接移除，不保留长期新旧双路径。可靠层根包可以保留未迁出的规则，无需清空为纯契约包。最终实际包依赖必须无环。

## 3. 阶段与出口

RM-P2 是依据实际痛点选择的整理步骤；本次已选并执行，实际顺序为 RM-P0 → RM-P1 → RM-P2 → RM-P3。未选职责也须记录保留原位的依据。阶段是审阅/验证检查点，不新建执行聊天或测试调度系统。

### RM-P0 文档与行为基线

文档阶段完成 Spec/Plan 收敛，检查链接、已有测试名、阶段/验收对应关系和当时的仅文档差异，对应 RM-A00。

实施基线步骤：

1. 登记实际版本，运行 §5 的现有单元及受控基线；已有失败保留记录，区分后续回归。
2. 列出本轮操作构造/更新/派发调用点，包括已有 Run stop 操作。核实准备资格、gate 和原提交位置，不改变服务退出顺序。
3. 从现有隔离受控场景选取少量 v2 样本，至少覆盖原未知操作、已停止 Run、正常完成 Run及其授权/来源；优先复用测试输出，不预建所有状态组合。保存来源版本并精确停止样本所属服务。

出口：文档检查与实施基线分别登记；本轮文档完成不表示实施基线已经运行。

### RM-P1 提取 operation

1. 移动固定输入、操作/回执校验、状态归约、execute/reconcile 分流到 operation。优先复用现有类型和普通函数，不统一改造其他职责的接口。
2. 将 saveIntent/invoke 和 stop 中已有操作的共用更新收拢；宿主仍负责快照、gate、锁、原子提交及命令状态组合。Intents 与 Snapshot.Operations 保持原语义，不在加载时重建/归一化。
3. 保留准备/资格检查和外部调用顺序。若现有准备与操作函数互相调用，按真实依赖调整少量函数位置，或在原 gate 内保持直接调用；不为隔离包而引入一组资格端口。
4. Run stop 复用必要操作更新函数即可，保留具体后端停止选择；服务退出、观察/owned-process 清理和权限组装不迁入新派发策略。
5. 迁移相关消费者/测试并删除本轮旧操作实现；更新现有依赖守卫覆盖新包。检查没有额外变更动作、新幂等键或重复操作入口。

出口：RM-A01/02 通过，RM-A03–A06 的受影响行为无回归；意图提交失败零派发、外部成功但提交失败仅对账、accepted/resource_pending 不重建、撤销后零新输入有测试支撑。缺覆盖才补最小反例，不预先要求每个场景新建 Test。

### RM-P2 按实际痛点继续整理（条件步骤）

仅在 RM-P1 后发现具体问题时开展，例如：同一身份判断在多处重复、原生来源判断与 Workflow 解析仍混杂、同一授权条件需要多处同步修改。

选择一处最小整理，说明“当前重复/耦合→移动的函数/调用者→减少的修改点”。session/observation/control/preparation 可以继续留在同包不同文件，也可按收益拆包；允许 observation→session 等无环依赖，不要求协作全部经过 Engine。

出口：所选职责边界更清楚，无重复实现，相关现有/新增测试通过。未整理的职责不是第一轮缺项。若没有明确收益，在 §6 记录不执行原因，直接进入 RM-P3。

本次继续实施已选择以下最小整理，不新建四个职责包：

| 职责 | 具体依据 | 处理 |
| --- | --- | --- |
| session | 回执和 started 事件重复更新四个 command/turn 字段，worker 另承载会话快照字段合并 | 在现有 reliable 包集中快照合并及 Turn 开始转换，宿主保留事务和公开状态投影 |
| observation | acceptEvent 与 Workflow 结果路由混在 results.go，原生答案校验又混在 control.go | 分到 application/observation.go，沿用原事务/确认路径 |
| control | resumeRun 中夹有业务步骤选择和待发业务输入保留 | 将业务反馈/恢复后的步骤选择集中到 application/feedback.go；控制文件保留授权、资格和生命周期 |
| preparation | linked worktree 权限准备助手放在 worker 内，与已有 policy.go 分散 | 助手归入现有 policy.go；通用 Next/QualifyAction 等已独立，继续保留可靠层原位置 |

### RM-P3 第一轮回归与交付

1. 对最终同一源码运行 §5 的单元组合与受控选择；仅因新修改/失败补跑受影响项。记录真实命令和结果，不借用其他 EXE 的通过结论。
2. 使用 RM-P0 的隔离样本，验证新构建加载、操作按原规则结算/写回、再次重启及 CLI 公开事实。保留原输入/身份/授权/PR/游标语义，不要求旧二进制读回或构建投影差异白名单。
3. 核对旧操作实现已删除、无反向/循环依赖，RM-P2 已选整理完成且未改变准备/观察/控制/业务语义；各职责保留原位部分均有说明。
4. 原生协议/参数/权限/生命周期若完全未变，以受控回归为默认验收；若实际变更触及这些范围，单独说明必要性和风险，补最小本机 Codex 验证（RM-A09）。不新增 Orca 真机验收。
5. 在 §6 登记完成与待验项、实际依赖和残留问题。合并/发布不属于本次计划的执行动作。

出口：Spec §8 的交付条件满足；operation 与本次所选整理均完成，五项职责有逐项处理/保留依据和验证，不以包数代替完成判断。

## 4. 验证映射

“已有入口”仅表示基线存在，不代表本轮已经运行或完整覆盖。优先复用下表，再对本轮改变的边界补缺口；新增用例名称在实际建设时确定。

| 验收项 | 已有可复用入口/位置 | 实施时重点核对 |
| --- | --- | --- |
| RM-A01 依赖/唯一操作规则 | TestSRExecutionDependencyBoundary | 新 operation 包无宿主/存储/业务反向依赖，无重复实现；不检测各职责互不依赖 |
| RM-A02 固定意图/状态/对账 | TestReliableUnknownEffectAndGateCannotResubmit、TestReliableFreshActionKeepsOriginalIntent、TestReliableUnverifiedReceiptRequiresOriginalReconciliation、TestReliableReconciliationRejectsForeignIntent、TestReliableOutcomeCannotTreatMissingReceiptAsSuccess、TestSRAcceptedResourceUsesOriginalOperation、TestSRRunAtomicCommit | 原 ID/输入保持、accepted pending 不重建；缺宿主提交失败覆盖时补最小端口反例和调用计数 |
| RM-A03 身份/Turn/句柄 | TestReliableTurnIdentityAndNotifications、TestReliableLateFacts、TestReliableRecoveredNativeStartRequiresOriginalIntent，现有 adapter recovery/lifecycle 测试 | 只回归实际触及的绑定/句柄逻辑；必要时补错身份或 owned 句柄被覆盖反例 |
| RM-A04 观察/来源/确认 | TestReliableInputOutcomeBoundaries、TestSRNativeAnswerRejectsAmbiguityBeforeIntent，现有事件/结果处理测试 | 若改到接收/提交路径，核对重复/冲突及失败不确认；无需新建观察连接框架 |
| RM-A05 授权/停止/重启 | TestReliableRecoveryAndAuthorization、TestClassifyRecovery、TestSRNativeUnknownAndStop、TestSRRestartRequiresResume | 原会话、旧答案不重放、未知操作不重发；派发边界若缺覆盖，在测试内补可控屏障 |
| RM-A06 准备/权限 | TestReliablePreparationFacts、TestReliablePreparationApplicability、TestReliableExpiredIntentCannotExecute、TestSRReportFreezesOriginatingNativePolicy、TestSROrcaFrozenMaterials | 保留资格/期限/权限和 accepted pending 行为，不扩大原生矩阵 |
| RM-A07 Workflow/PR | TestSRRunWorkflow、TestSRPublicationIdentity、TestSRPublicationBeforeWorkflowReady、TestWorkflowSourceAndFormatRules | 正常闭环、可信来源和 PR 归属保持；触及反馈/报告时复用相应用例 |
| RM-A08 存储/公开事实 | TestSRRunAtomicCommit、TestSRStorageLogsEvents、TestSRRestartRequiresResume，RM-P0 隔离样本 | 基线数据可加载、操作结算/写回/重启事实不变；不新增完整性阻断或投影重建 |
| RM-A09 条件性本机验证 | scripts/test-native.ps1、tests/native/run_test.go | 只在具体原生风险触发后核实参数和范围，记录触发原因；未触发说明理由即可 |

故障注入限已有测试端口/协议 helper/驱动，不增加生产 fault 开关；进程级接受路径仍经真实 CLI/服务。对竞态使用明确屏障和调用计数，不以固定 sleep 推断撤销已经提交。现有测试内部使用的等待策略仅在影响本轮验证可信度时调整。

## 5. 后续实施命令

本轮单元基线和最终实现均使用以下组合：

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage -count=1 -timeout=3m -v
```

现有 Windows 受控选择：

```powershell
.\scripts\test-foundation.ps1 -Pattern '^(TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRRestartRequiresResume|TestSRPermissionQualification|TestSRPublicationIdentity|TestSRStorageLogsEvents|TestSRPublicationBeforeWorkflowReady)$'
```

当前选择 7 个顶层场景。脚本返回只代表独立宿主已启动，须检查本次新 evidence 目录的 result.json：exit_code 为 0、scenario_count 为 7、日志中选择项执行且无意外 skip。新增或迁移用例时核实实际匹配并更新清单，不能只看退出码。

本次增加一个可选的测试参数 BaselineExe，使用本轮保留的基线 EXE 验证 v2 状态：

```powershell
.\scripts\test-foundation.ps1 -Pattern '^TestSROperationV2Compatibility$' -BaselineExe .\.evidence\foundation-20261015-085321\steerrunner.exe
```

此组预期 1 个顶层场景、pr_ready/stopped/unknown/stop_pending 四个子场景全部通过。基线 EXE 仅为本次隔离证据；复验须使用来源明确的原版本构建。测试经公开命令创建/接续同一状态目录，证明旧服务退出后才切换二进制；stop_pending 仅通过测试侧 SQLite trigger 注入结算事务失败，不直接修改业务记录。未提供 BaselineExe 时该可选场景 skip，不能据此记兼容性通过。

RM-P2 后的最终复验将上述七项与兼容项合并为同一构建的八个顶层场景：

```powershell
.\scripts\test-foundation.ps1 -Pattern '^(TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRRestartRequiresResume|TestSRPermissionQualification|TestSRPublicationIdentity|TestSRStorageLogsEvents|TestSRPublicationBeforeWorkflowReady|TestSROperationV2Compatibility)$' -BaselineExe .\.evidence\foundation-20261015-085321\steerrunner.exe
```

本组预期 exit_code=0、scenario_count=8，兼容项四个子场景也须全部通过。

按实际改动补充 adapter 编译/定向测试；不默认运行全仓 Full 或真实模型。RM-A09 若触发，读取当时 native 脚本，固定 EXE、本机模式、工具版本、本次隔离 state-dir/材料/测试仓库和证据位置。精确处理本次拥有的服务/原生资源，保留原验收对用户安装现场的隔离要求。

## 6. 状态和证据

文档阶段 RM-A00 已通过。2026-10-15 用户授权实施后，沿用基线 `af545563c58171c584bd35d098166cf39012e699`，工作分支为 `codex/reliable-operation`；实施及审计验证时源码为未提交工作树，来源按基线加代码清单/构建 SHA 登记，见实施证据。后续用户授权提交 PR，交付提交和分支状态以 Git/PR 记录为准。

实际依赖为 application/runstate → operation → reliable → backend。operation 直接复用可靠层原有资格检查，未引入新资格端口或统一变更模型。原 Intent 的 Go 类型迁入 operation，字段和 JSON tags 不变；New/Settle/SettleStop 收拢构造与两种现有结算语义。核心继续负责锁/gate、事务、脱敏和业务衔接。

operation 阶段的历史构建及验收保留在[阶段证据](../acceptance/evidence/reliable-operation-20261015.md)。当前追加 RM-P2 的会话转换集中、观察/业务结果分离、恢复业务衔接分离和权限助手归位，并新增接受/开始区分、异会话无部分变更、失败事件可重读、恢复报告原来源保留四项边界测试。34 个顶层单元测试及 vet 通过；最终新构建的七个业务场景加兼容项共八项通过，兼容项的三个子场景也全部通过。没有沿用旧阶段 EXE 的通过结论。

上述 34 项/三类兼容样本为 RM-P2 当时的验证。用户随后要求按 Spec/Plan 审计并直接修复：已用失败反例确认 stop 分次提交和事件提前确认两个既有缺陷，分别修复；补 stop/派发可控屏障及旧 stop 操作结算兼容样本。此后最终源码的复验状态以[审计记录](../acceptance/evidence/reliable-modularization-audit-20261015.md)为准，不把历史构建当作审计后证据。

审计后复验已通过：37 个顶层单元测试、adapter 定向测试及 vet 通过；`.evidence/foundation-20261015-094642` 的最终同一构建完成 8 个受控顶层场景及 4 个兼容子场景，exit_code=0、scenario_count=8，无失败/skip。旧 stop 操作在新版本结算并再次重启的验证已经补齐。原组合的 5 分钟时限不足以容纳新增兼容样本，脚本总时限调整为 8 分钟，生产调用时限不变；未取得完整结果的首次运行不计通过。

PR 交付前分支又更新到最新 main `d2f0d094e857d6ff162b68ab1dc9f81d49261dc5`，整合版本的 40 个顶层单元测试（含 host）、vet、完整 8 个受控场景及 4 个兼容子场景通过。兼容测试补正为比较旧服务退出后的最终持久基线，避免旧 worker 合法对账造成取样竞态；字段比较未放宽。最终构建提交为 `69d9a47b2416a7534f74262033e36b6580d5c87c`，证据目录 `.evidence/foundation-20261015-100201`，详见审计记录最后一节。

| 检查点 | 状态 | 后续证据 |
| --- | --- | --- |
| RM-P0 文档 / RM-A00 | 已通过 | 文档检查完成；实施后的链接/状态再复核 |
| RM-P0 实施基线 | 已通过 | 原源码单元组合、真实 EXE 七场景通过；基线 EXE 生成三类 v2 样本 |
| RM-P1 operation | 已通过 | 新包/调用点/旧入口/依赖守卫已整理，两个宿主提交失败反例及 stop 表示保护通过 |
| RM-P2 其他职责 | 已通过 | 四项职责逐项处理；具体依据见 §3 的已选整理表；未创建四个新职责包 |
| RM-P3 本次收尾及审计复验 | 已通过 | 审计后 EXE 八个顶层受控场景 exit_code=0、scenario_count=8，无失败/skip；兼容项四个子场景通过，含原 journal/Snapshot 保持及旧 stop 原请求结算 |
| RM-A09 本机验证 | 未触发 | adapter/原生协议/参数/权限/服务退出路径未修改；本轮仅既有操作规则提取，采用受控验证 |

实施证据记录实际源码/文档版本、命令与结果；进程测试另记录 EXE SHA、场景数及必要的操作/会话关联与动作计数。按风险记录所需信息，不要求每阶段重复生成一套完整环境报告。失败修复后重跑受影响项，不能把“已编译/已启动”记作验收通过。

回退时停止本次服务、核实资源事实，在隔离副本上验证；禁止不同版本并写同目录。源码回退不撤销已接受的外部动作，未知仍按原操作对账，不删除状态或换 Run 规避。
