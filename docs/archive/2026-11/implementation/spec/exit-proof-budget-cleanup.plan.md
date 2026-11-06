# Plan：退出证明生产端与旧预算重建清尾

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-01。状态：按后续实施授权完成 E/B 及最小验收，见[报告](../acceptance/exit-proof-budget-cleanup.md)。依据 [Spec](exit-proof-budget-cleanup.spec.md)及[长期承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)。实施前 main `a06c280` 与工作树 `fca735a` 文件树一致；实施使用隔离夹具和一个通过的真实 helper READY 闭环（首轮失败另存），不操作用户已有 Run。

## 1. 分增量交付

先 E（退出证明生产端），后独立判断 B（旧预算反推）。E 的删除依据已由生产消费者盘点确认；B 的静态不变量已查明，但实施前还须覆盖全部合法入口。任一增量遇到反例只停该增量，不把两项捆成新的总门禁。

复用当前 helper、Lifecycle/operation、预算纯函数和现有测试夹具；不引入资源管理层、通用文件系统 mock 平台、预算修复器、迁移器或新的输入调度器。只转移同样逻辑不算完成。

## 2. P0：本轮审计与基线

- [x] 盘点退出文件生成、专属 verifier/读取包装、请求定位、公钥比较、共享 signedRemoteExit、Job/reader 消费者。
- [x] 单列历史 export-exit/verify-proof 归档源码，不将其当当前产品或正式 catalog 的支持义务。
- [x] 追踪 queueInput/prepareRun 两处直接消费者，以及五种 Workflow、report/后继、feedback/message、NextStage 和服务恢复的 Prompt/Budget 写入路径。
- [x] 区分未渲染草稿、已冻结带 Budget、缺失副本和已冻结缺 Budget；发现当前模型 allowlist 仍须读取已保存 legacy 来源标签。
- [x] 明确原审计时预算文档仍允许可靠补算，随后按实施验收显式修订；不借 state/v8 拒绝旧格式就跳过当前条款的修订。
- [x] 实施开始时重新核对 HEAD/dirty、实际调用集合及用户改动，记录 E/B 各自生产增删基线。源码变化后不能直接沿用本轮静态结论。

P0 原文档阶段出口为 A00；后续实施与运行资格按 P1–P6 和报告分别登记。

## 3. P1：E——拆开绑定生成与退出证明

1. 复核当前生产路径仍不调用 VerifyRemoteExit/ReadStopArtifact；区分原生源、测试与历史归档。若发现新生产消费者，先界定其合同，不直接删符号。
2. 在 helper 原准备位置生成同形状的 Ed25519 公钥绑定，生成后立即清除不再使用的私钥。生成函数不接工作区、不建目录/文件、不持有退出 writer；保留原 ExitPublicKey、ready 编码、原值比较和停止路径。
3. 删除 RemoteExit、专用 writer/verifier 及 RunRemoteAgent 的 open/write/sync/close 和专属证明错误分支。维持 app-server/TUI Job 实际清理的调用次序、等待、Close 和失败返回，不因不产出文件就忽略未结算 Job。
4. 保留 RemoteExitPath 的纯定位与 RemoteStopPath/awaitRemoteStop；不改变 request.json 路径、完整原 readiness 匹配、上传目标和现有有界停止预算。保留目录 Git exclude，因为原请求仍写在这个范围内。
5. 删除 ReadStopArtifact/decodeStopArtifact 与专属 IPC 退出文件测试；同文件的 UploadWorkspaceArtifact、连接/路由检查和通用 files 通道保持。
6. 不删除 shared signedRemoteExit、直接 Codex owner-stop/checkpoint 私钥及验签、settleRemoteTree、history/turn/workspace reader 的 WritersAbsent 语义。搬动共享类型如非必要则不搬。
7. 调整当前 native/stop 的 exit.json 故障注入与归档：仍可定位旧路径证明新版本不消费，但真实准备/收尾不能再期待生成或导出签名文件。历史证据及固定旧工具不改写；不通过留一个恒真 verifier 维持编译。

出口：生产专用 writer/verifier/读取调用为 0，退出文件不是启动条件；实际停止与其他来源消费者保留。源码删除未满足这些条件时不进入“已完成”状态。

## 4. P2：E 的最小验证与独立结项

按 Spec A01–A03/A08/A09/A10 验证，优先扩展原夹具：

| 验证面 | 既有入口与调整 |
| --- | --- |
| 专属生成/文件门槛 | 将 TestRemoteExitRequiresOriginalSignedCompleteSettlement 的退役规则替换为纯绑定生成及无专属证明 I/O 断言；只删除无消费者部分。证明故障与请求目录故障分别记录，不能混称 |
| 实际清理与 reader | 保留 TestRemoteExitAccountsForOriginalOwnedTree、TestRemoteReaderFailedStartStillProvesNoNativeReader，以及 history/remote_turn/remote_workspace 对应正反例；不能整份 remote_exit_test.go 删除 |
| 共享签名 | 保留 TestExecutionOwnerExitRequiresSignatureAndCompleteOriginalBinding、TestExecutionOwnerCheckpointPinsOriginalTerminalAndSignature，覆盖篡改/异源拒绝 |
| scoped stop | 复用 TestLocalCodexStopCooperationAndLostClose、TestLocalCodexStopReconcileNeverClosesLiveResources、TestLocalCodexStoppedRootsAndInventory；统计上传、close、create/send，未知后零重复动作 |
| 测试消费者 | native/integration 各以 `go test -c -tags live -o <本次拥有的产物> <package>` 编译；不运行无 case 的 TestMain 冒充通过 |
| 真实接线 | 一个新隔离 TestOrcaStoppedNativeProbe，当前 Runner/helper 及已安装 Orca/Codex，真实 READY bootstrap、公开 stop、普通停服、无 exit.json 的离线核验、DB/回执字节不变和自有资源清理 |

精确登记实际选择器、顶层测试及子用例、退出码和 skip。helper/live 的默认 skip 不算真机通过；根身份缺席不推广为全部后代退出。真实工具后代由原 Job 夹具证明，不要求模型随机调用工具。沿用正式测试入口的适用预算，不把整包累计耗时塞进过短定向期限，也不延长生产停止期限。

E 可在 B 尚未开始或遇到反例时独立验收；不要求真实 PR/合并、远端模型或全 Agent 矩阵。

## 5. P3：B 的入口不变量复核

先形成下表的受控覆盖，再删反推分支；不从单一 happy path 推断全部来源：

| 情况 | 通过实际生产构造点证明 |
| --- | --- |
| development/2、development-audited/2、execute/1 首次输入 | queueInput 新渲染得到 StageInput.Budget 和 PendingTurn.Budget，正文/来源/模型不被改写；初始冻结与保存失败零派发 |
| 审计/发布/report 后继 | route 的 Prompt/Budget 为空是草稿；results 保存 NextStage 后，首次渲染仍产生合法预算；原 report 次数与来源保持 |
| 业务反馈与 conversation message | feedbackValue/contentMessage 到 queueInput，Budget 直接来自明确片段，而非 History/文本标签反推 |
| generate/conversation 首次输入 | contentParts 的材料转义、指令/schema/固定仓库说明计量正确，read_only 及材料身份保留 |
| 两种 NextStage 恢复 | 未渲染草稿首次生成；已渲染副本携带原 Budget，经历 JSON/SQLite 往返仍在，只校验原文字、不追加当前默认值 |
| 已接受/未知原输入 | reliable.ApplySessionReceipt/重启不丢失已保存 StageInput.Budget，不依赖补算取得新的输入权限；原意图正文/摘要和查询路径不改写 |

若出现正常 v8 writer 可持久化冻结文字却没有 Budget，记录完整调用点和反例：能在原装配/同事务保存处修复的，先修该写入点并补最小验证；需迁移或新增恢复权威的，则暂停 B，保留旧分支及未完成状态。不能把缺字段自动认作旧格式，也不能靠修改所有测试夹具掩盖合法入口。

出口：A04 的合法入口覆盖和可核对的写入不变量。静态审计已找到当前构造点，不代表上述运行矩阵已执行。

## 6. P4：B——删除反推，保留原预算验证

1. 在原 ensureStageBudget 中去除 Budget==nil 的正文/材料/反馈逆向解析，只保留明确的缺失错误及已有 Budget 的 InputParts、原生归一化和 FreezeInput 检查。不重渲染、不猜来源、不补另一个 fallback。
2. queueInput 继续区别 Prompt 为空的合法草稿与非空的冻结文字；前者按既有装配规则产生 Budget，后者缺 Budget 时不创建新的业务输入 CommandID/阶段/副作用意图。公开控制命令仍沿既有受理/失败回执契约处理，不因候选拒绝抹去原命令。
3. prepareRun 如保留预算副本修复，只允许从绑定一致的原 StageInput 复制已有合法值，并沿原事务保存；StageInput 缺预算或正文/命令/步骤/版本/Attempt 冲突时拒绝新候选。不从 native 意图反填 StageInput，不修改可能已外发操作的 payload/digest。
4. 不把 Budget 非空做成加载整库/所有读取的强门禁；原查询、观察、accepted/unknown 对账、stop/verify-stopped 均不以此次反推退出为障碍。原 unknown 不重新 Execute，原失败/结果/PR 不能因预算异常被追改。
5. 保留模型输入来源 allowlist 的已有 legacy 标签读取，保留有合法预算的旧正文恢复、原生 AnswerInput、parser、bootstrap/developerInstructions、Orca framing 和 1000B/调用方不限量/传输保护。
6. 将现行预算 Spec §4 的可靠补算能力明确修订为“已有预算校验，缺失拒绝新派发”，与长期文档的现行状态同步。没有运行验收前不将草案写成现行保证。

出口：三类反推规则和补算保存路径均无新生产调用；不是把函数换名或搬到 adapter。

## 7. P5：B 的最小验证与收益

复用 TestBudgetCallerOwnershipAndFeedback、TestBudgetFileGuidesIsolation、TestBudgetParserOverflowRetainsReportAndLongFinal、TestBudgetAuditedFilesRemainUsable、TestBudgetLegacyDefinitionKeepsTextButNotRoutingChanges、AnswerInput/Content/SingleExecution、人工恢复及 operation 提交失败的相关夹具，按实际改动精确选择。旧测试里手工清 Budget 后“补算成功”的片段应改为对应拒绝/零副作用断言，其余来源和长度断言保留。

新增最小反例：

- StageInput 无 Budget、仅 PendingTurn 副本无 Budget、两者无 Budget；合法原预算可复制，缺失/冲突不反推。
- 正文 SHA/来源 SHA/片段/C-R-T/UTF-8、CommandID/Step/InputVersion/Attempt、模型偏好冲突；保存失败后无新 input/create/Execute。
- 未渲染 NextStage 与已渲染 NextStage 的暂停/重启/恢复；原业务等待与 stop 撤销仍阻止后继。
- accepted/unknown 缺预算的可查询/观察/只读对账/stop，原 intent 和结果不被补写；同 request_id 回放及停止竞争保持。
- 已保存 legacy 标签预算和旧默认正文仍能通过原验证；未知标签、篡改、Runner 超限继续拒绝；原生回答及 parser 的独立预算路径不变。

至少一个公开受控 CLI 场景贯通 inspect/原命令/恢复或拒绝/stop，复用现有 foundation 原执行器和计数；可以在同一隔离现场验证相关边界，不新增真实 PR、另一个 driver 或五种模型矩阵。先建立实际有匹配的选择器，再登记通过；无新测试时不能把计划中的名称当结果。

B 的完成要求 A04–A08/A10。当前 state/v8 历史与序列化形状不迁移，CLI 退出码/错误仍沿现有输入失败路径，不为缺预算新增一套公开运行状态。

## 8. P6：文档、实际收益和完成边界

- 分别记录 E/B 的生产新增/删除/净变化、真正删除的规则、剩余消费者及必要构建/测试成本，不能把测试删行或纯移动当生产收益。
- E 记录专属 exit 文件预开、签名、写和 Sync 的调用归零；保留的 request/config/readiness 文件及实际 Job 等待单列。B 记录反推调用/补算保存归零，正常 Budget 校验/复制保留。
- 建立新的专项验收报告，绑定源码、Runner/helper SHA、测试选择/计数、首轮失败与资源清理；无可比计时不报提速，不继承 #80 的真实 probe 为新 helper 资格。
- 验收后才修改长期文档中的保留实现描述和预算补算现行条款，标记各增量完成，更新索引。历史验收/取证工具保持原路径和字节；不回写为新版本支持。
- 不自动修改权限准备、构建指纹或扩大停止 scope，不重新打开 P1–P4。部署、用户任务排空、旧目录处理、外仓联验和发布另行安排。

当前结果：P0–P6 已完成。E 删除专属生产/消费和两份专属测试；保留真实 Job/reader 与直接签名。B 删除三类反推，窄副本复制检查绑定及未外发条件；额外补齐“原 StageInput 缺预算、PendingTurn 有预算”的准备入口拒绝，避免绕过该边界。正常入口先验基线通过，再实施删除。

实测选择器、构建/SHA、组件 432 项顶层通过（7 项默认 opt-in/helper skip 单列）、真实探针及公开 CLI 结果、首轮夹具失败和 PID 复用拒绝见报告。历史失败不改判；没有部署、发布或完整 L2/L3 资格。

实施后再审计：见[报告](../acceptance/exit-proof-budget-reaudit.md)。生产逻辑未再变动；修正 accepted 资源受理测试夹具，补上九个缺预算原结果/停止回归，最终构建与 PR 验证以该报告为准。
