# Plan：执行与可靠性边界收敛

日期：2026-10-20。状态：五项代码收敛及本轮验收已完成。行为与验收依据 [Spec](execution-boundary-refactor.spec.md)。调查基线：`bf3ec581cb0b23a7358d95e5efa8ff5f607c2f55`。文档轮仅新增文档与 README 索引；2026-10-20 用户追加实施授权。实际命令、结果与限制见[验收记录](../acceptance/execution-boundary-refactor.md)。

## 1. 实施顺序与共同约束

顺序：P0 基线 → 第一批 P1（R2 锁）→ 第二批 P2a（parser）→ P2b（续执行）→ 第三批 P3a（模型边界）→ P3b（接管解码）→ P4 收口。每个小阶段独立审阅并有出口证据，前一阶段失败先修复再继续。阶段不要求新建包/聊天/PR。

保留 application 唯一持久协调者、原 gate/CAS、Store.Commit、Record 和现 state/v4。共用可靠性规则优先普通函数、窄值和已有端口，不创建全流程协调框架。整个 Record 不交给 reliable；业务完成、解析 fallback、PR 和模型角色不搬进可靠层；具体 codec/原生知识不留在宿主规则里。

实施沿上述基线与文档差异开始；保持用户在途文档，不操作用户安装环境、非测试 state-dir、真实业务任务或发布/合并。

## 2. 分批步骤与出口

### P0 文档与实施基线

文档阶段：完成 Spec/Plan、README 互链，核查引用、现测试入口、EBR-I/EBR-A 与阶段映射；差异仅 Markdown。出口为 EBR-A00，不运行功能/原生验收来替代文档检查。

后续实施开始时：

1. 登记实际 commit/dirty 清单；对照 Spec 的基线核实五个耦合点和最新支持范围。画出已有外发→提交、锁顺序、恢复调用者及 parser 普通/owner 两条路径清单。
2. 运行 §4 定向规则基线和拟复用受控场景，保存实际通过/失败/skip/场景数。失败先区分既有缺陷与重构回归，不修改断言来消除证据。
3. 从已有隔离受控流程留存原构建和样本：含未知原操作、待结算 parser/stop、已存结果与唯一未派发后继、内容等待/关闭、R2 产物。记录原 ID/key/digest/源/期限/授权/命令/事件；按需复用样本，不构建全状态笛卡尔积。
4. 样本切换只发生于测试专属目录，确认旧 writer 退出后取得排他锁；需保留活动 owner/parser 的场景使用既有换代 harness，先保留再精确 stop，不能先杀树再声称原执行保留。

出口：原行为、外发/提交边界、样本和原生风险清单可核查；本轮文档状态不冒充这些实施基线已完成。

### 第一批 P1：R2 外部采集释放全局锁

主要位置：[application/artifact.go](../../internal/application/artifact.go)、必要的 workspace 查询接线及 artifact/integration 测试。

1. 拆清只读投影、锁外采集、锁内复核/提交。list/read 只需已提交不可变数据；Git/目录/文件 I/O 不在 e.mu 下执行。
2. collect 保持 request_id 去重与所属 worker.gate 的原获取顺序，锁内取稳定快照后释放 e.mu；读取与前后来源校验完成再加 e.mu，复核 revision/绑定/限额/服务状态并原子提交。保持同 Run collect 先于后到 stop 的原排序；不增加 Allowed 门槛或改变 stopped Run 的证据能力。
3. discover/workspace-git 不获取新执行授权、不唤醒 worker；锁外观察返回前复核身份，维持双样本及原失败形状。观察可与 stop 并发，不能因此把只读请求升级成恢复动作。
4. 用测试侧可阻塞 I/O/命令端口建立屏障：A 阻塞采集期间，B 的 stop 已提交且 B 的原事实可提交；另测同 Run 排序、CAS/身份冲突和提交失败后的原 request_id 重放。无生产 fault 开关，无 sleep 代替屏障。

出口：EBR-A01；原 R2 正反例通过。只把外部 I/O 移出全局锁，不移动 artifact 字节存储、Record.Clone 或另建采集服务。

### 第二批 P2a：parser 共用操作规则

主要位置：[application/parser.go](../../internal/application/parser.go)、[worker.go](../../internal/application/worker.go)、[reliable/operation](../../internal/execution/reliable/operation/record.go)、backend、codexexec 和 owner 的必要接线。

1. 将 parser 代码按“业务选择/来源→可靠操作→资源调用/取证”列出迁移清单。先用当前测试固定 pending→in_flight 的一次派发点、容量和绝对期限、挂起根身份提交、started/final 和资源退出条件。
2. 在 operation 集中本轮实际共用的资格、固定请求、状态/回执合并与冲突规则；为 parser pending 保留显式专属处理。宿主在原 gate/CAS 下检查容量并提交派发事实，提交成功后才经共同协调入口调用具体资源端口。
3. parser 启动/原 started/final 读取/取消归专属实现。异步调用允许返回原任务句柄或待结算回执；不把 operation.Invoke 改为持有 gate 等待整个模型轮次，也不要求所有资源实现都实现宽 LifecyclePort。
4. 保留原 owner 绑定与局部启动路径、绝对 Deadline、原 journal 路径、放行前 root 提交。服务换代不取消可保留 parser；Run stop 精确取消并等待结算，迟到 receipt 只记事实。owner 不新增通用执行器/操作队列。
5. 业务侧保留解析来源检查、Acquired/attempts 投影、RouteParsed、fallback 和审计 checkpoint 条件；同一事务仍组合可靠事实与适用的业务变化。删除迁移完成的重复规则及直接旁路，保留有明确理由的专属资源差异。

出口：EBR-A02 及受影响的 EBR-A04。证明意图/根身份提交失败零放行，已接受或未知不重发，重复/冲突/错源回执、停止、跨重启及结算后 fallback 行为保持。不能只把直接 Execute 包进一个新函数就算完成。

### 第二批 P2b：原 session 续执行资格与协调

主要位置：[reliable/recovery_policy.go](../../internal/execution/reliable/recovery_policy.go)、application/control、feedback、content 及必要 session 接线。

1. 对照 resume、业务 feedback/answer、conversation message 的现资格，列出共同项与专属前置：原 session、idle、resource/process/writers、unknown、控制范围、内容目录、问题/业务 source。
2. reliable 以窄事实集中共同判断；application 用共同函数协调 VerifyManualRecovery、必要的 BindRecovery 和提交前复核。保持原调用/释放/提交顺序，不能让抽函数改变恢复时机。
3. 各业务入口提供所需输入与来源，沿原 queueInput、命令/history 和问题结算。保留 NextStage/PendingTurn 关联、原 Attempt 历史及绑定重置规则；统一不是统一所有错误或状态。
4. 明确两条路径：兼容换代接回活动原执行；原资源丢失后的人工同 thread 恢复。对原授权、暂停、等待/blocked、已关闭、pending resume/stop 和旧 epoch/范围答案分别验证，禁止恢复 helper 自行续权或选下一业务步骤。

出口：EBR-A03/04。三入口确实复用判断与协调，专属限制不丢失；已消费输入只等原事实，重复重启/晚到结果不增加模型调用或后继。

### 第三批 P3a：模型解释离开 adapter

主要位置：codex/remote_models、remote_agent、相关测试，以及实际使用的 workflow/application/host/cmd 组装入口；Orca local 调用链一并核对。

1. 记录当前允许步骤、写者/审计模型选择和恢复到写者的行为；保存配置与 prompt envelope 的代表字节/摘要、非法步骤/错模型等拒绝样本。
2. 将共享模型值放到实际需要的小型纯值位置，保持 JSON tags/顺序/空值行为。由业务/组装侧从原冻结配置产生模型绑定；必要接线在 cmd/host 显式完成，不在 adapter 增加新的业务 switch。
3. adapter 按原 command/digest/session 和冻结模型绑定核验输入，再转换原生 model/effort/collaboration 设置。未知 step 仍拒绝；不信任 prompt 自述、不扩大允许步骤、不改持久 profile/bridge wire、不要求替换在用 owner/helper。
4. 在迁移完后扩充依赖守卫，禁止 adapters→workflow，保持 reliable→backend 的方向并核查 backend 无业务反向依赖。给守卫补实际负例；同时人工查找迁移后的步骤常量/字符串路由，避免只消除 import。

出口：EBR-A05。模型选择、审计后恢复、原连接和原生来源证明保持；旧配置/prompt 字节与摘要不变；缺模型/错绑定仍拒绝。若不能保持编码或需要原生协议变化，先修订设计，不直接升版绕过兼容约束。

### 第三批 P3b：实现私有兼容解码归位

主要位置：application/takeover、parser package 校验、具体 adapter decoder、host/takeover 的必要接线。

1. 原样列出当前判定条件及先后优先级，区分 workflow 语义、材料完整性、版本声明、具体 codec/权限/owner 与 parser package；保留退役/旧记录拒绝或只读边界。
2. 具体实现暴露只读校验与窄事实，封装私有解码；application 继续汇总材料、workflow 和版本适用性，沿 reliable 规则判断恢复门槛。复用具体 decoder 和显式选择，不引入动态注册器或整份 Record 端口。
3. 将 TakeoverFacts 中为校验而构造 lifecycle 的路径换为明确的只读校验接线；真实需要文件/进程探测的条件仍在对应实现边界执行，不省略探测或假称纯函数。
4. 对同一原记录比对新旧兼容结论与诊断；检查零连接/创建/文件写入/业务调用、原聚合字节无变更。未知 codec/owner 协议、缺能力/checkpoint、退役认证、parser schema 等反例逐项保留。

出口：EBR-A06；host 的离线/在线接管只读围栏保持；新 helper 无新增授权权威，不因解码归位扩大支持声明。

### P4：共同回归与收口

1. 在同一最终源码运行受影响 owner 包规则、依赖守卫、vet 和 §4 的真实 EXE 受控选择。分阶段结果可以帮助定位，最终通过不能拼接不同生产构建。
2. 用 P0 保留的原构建样本进行加载→原操作结算/写回→再次重启；比较固定输入/操作/来源/期限/授权/PR/命令和事件语义，允许原本合法的新事实与 revision，不要求动态记录字节全等。原 owner/codec 仍按原协议验证，不覆盖在用版本文件。
3. 按真实风险补最小原生验证：P1 仅锁范围调整可采用规则/受控验证；改到 parser/owner 放行、连接/退出或模型桥时，验证对应本机 Codex / Orca local 原机制。仅语法搬迁且调用/参数/寿命未变者说明未触发依据，不追加无关远端/权限矩阵。
4. 原生验证沿测试专属配置和目录，显式 stop 本轮拥有的资源并核实既有 scope；服务退出不替代业务资源结算。不能用受控通过冒充真实原生资格，也不补齐本轮未改的 Orca 创建接口缺口。
5. 核对无旧重复规则、无旁路、无反向/循环依赖；记录 EBR-A01–A07 的结果、来源、缺口和回退限制。

出口：EBR-A07 与所有受影响验收通过。本轮必需验收尚未通过时，不能将实现状态标为完成；既有且本轮未改变的外部接口/原生资格缺口单独保留，不扩为本轮新增实现目标，也不改记通过。

## 3. 验证映射

下表列出可复用入口，不表示覆盖已充分或本轮已运行。新增屏障/反例在实现时命名并登记，只补实际缺口。

| 验收 | 现入口 | 必要补充/核对 |
| --- | --- | --- |
| A01 | TestR2UnknownSourceAndReadOnlyQueries；TestSRRunEvidenceR2 | 两 Run 的慢采集/stop/事实提交屏障；同 Run collect/stop 排序；锁外期间 CAS/来源变化及 request_id 重放 |
| A02 | TestRAParserAcquisition、TestRALateParserReceiptAndDuplicate、TestRAAtomicParserRecovery、TestRAConcurrencyParserStopResources、TestRAStateAuthorityParserReferences | 迁移后执行/对账/取消计数；派发与根身份提交失败；pending 容量与过期；停止后原 receipt 和 fallback 围栏 |
| A03/04 | TestReliableRecoveryAndAuthorization；TestContentRecoveryKeepsContractsAndRejectsForeignMessageScope；TestServiceGenerationPreservesDurableAuthorizationAndBusinessState、TestServiceGenerationControlScopeRetainsOnlyOriginalQuestionAndSource | 三入口共同资格与专属来源反例；活动资源直接观察、丢失资源人工恢复；重复换代不续权/预算 |
| A02/04 | TestSROperationIntentCommitFailureDoesNotDispatch、TestSROperationReceiptCommitFailureReconcilesOriginal、TestSRStopSerializesWithNativeDispatch、TestSRStopBindingCommitFailureDoesNotDispatch；TestSRObservationCommitFailureKeepsEventReplayable | 新调用点仍受原 gate/CAS；stop/迟到事实竞争与提交失败不提前确认；精确动作计数 |
| A04/07 | TestRAAtomicCompletion、TestRARecoveryMissingCheckpoint、TestRAResumeConsumedActiveInputWaitsForOriginalEvidence；受控 TestSRRestartPreservesAuthorization | 原结果/获取/唯一后继的已发与未发窗口，暂停/等待/业务阻断/stop/pr_ready，重复重启与原 owner/parser 保留 |
| A05 | TestSRExecutionDependencyBoundary；TestRemoteAuditedModelRoutingAndRestoration、TestRemoteModelBridgeUsesOriginalNativeConnection、TestRemoteNativeModelProofRejectsMissingAndChangedContexts | adapters→workflow 守卫负例；迁移前后编码/摘要；未知步骤/模型拒绝和审计后写者模型恢复 |
| A06 | TestServiceGenerationTakeoverRejectsUnknownCodecAndKeepsOriginalFacts、TestWorkflowRecoveryChecksExecutionSemanticsInsteadOfCatalogMetadata；host/storage 的 takeover/recovery 测试 | 新旧判定差分、零写/零创建、精确私有绑定/owner/parser/退役配置反例 |
| A07 | 受控 TestSRRunWorkflow、TestSRNativeUnknownAndStop、TestSRContentGenerate、TestSRContentConversation、TestSRContentQuestionAndUnknown、TestSRRunEvidenceR2、TestSRRestartPreservesAuthorization | 同构建闭环、原构建样本写回与二次重启；不借旧格式专用测试证明当前 v4 语义 |

## 4. 后续实施命令与证据要求

从仓库根运行；实际过滤、时限及新增用例以实施时核实结果为准。以下为实施参考命令；实际运行与修订的超时、受控原构建参数及 parser 专项见验收记录。

```powershell
go test ./internal/execution/workflow ./internal/execution/reliable/... ./internal/execution/backend ./internal/execution/resultacquisition -count=1 -timeout=3m
go test ./internal/application -run 'TestR2|TestRA|TestContent|TestServiceGeneration|TestWorkflowRecovery|TestSROperation|TestSRStop|TestSRObservation' -count=1 -timeout=6m
go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/execution/resultacquisition/codexexec ./internal/host ./internal/storage -count=1 -timeout=6m
go vet ./internal/application ./internal/execution/... ./internal/host ./internal/storage
```

```powershell
.\scripts\test-foundation.ps1 -Pattern '^(TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRContentGenerate|TestSRContentConversation|TestSRContentQuestionAndUnknown|TestSRRunEvidenceR2|TestSRRestartPreservesAuthorization)$'
```

现受控选择预期 7 个顶层场景；先核对脚本/注册列表确实匹配。脚本独立启动后会返回，必须等待本次新 evidence 目录的 result.json，核查 exit_code=0、scenario_count=7、实际子场景命中且无意外 skip；不能把“进程已启动”记为通过。新增场景后同步修订匹配与预期数量。新增屏障主要在 owner 包测试完成；真实 EXE 场景继续验证 CLI/IPC/SQLite 边界。受控测试沿现 harness，不直接启动真实模型。

每阶段记录源码版本/dirty 清单、命令/过滤和结果；进程测试另记录 EXE SHA、实际场景、原操作/会话/owner 绑定和副作用计数。原生测试按[测试指南](../testing.md)记录工具版本、冻结配置、现场与收尾范围。已有服务换代验收的未完成项独立保留；本轮任何结论不外推为全原生或跨仓通过。

源码回退不撤销外部已接受动作。只在隔离现场、确认 writer 排他后切换兼容构建，保留当前数据库和原 owner 版本；不能用覆盖旧备份、删 Run、换 request_id 或重建 session 消除未知。

## 5. 状态登记

| 阶段 | 状态 | 证据/待办 |
| --- | --- | --- |
| P0 文档 | 已完成 EBR-A00 | 仅 Spec/Plan 和 README 索引；文档检查见下，无代码修改 |
| P0 实施基线 | 已登记 | 原构建已保留；初次规则超时与受控创建屏障失败保留，不冒充全通过 |
| 第一批 P1 | 已实施、定向通过 | A01；R2 并发/提交失败/身份围栏 |
| 第二批 P2a/P2b | 已实施、定向通过 | A02/A03/A04；parser 原生资源及取消前结算已验证 |
| 第三批 P3a/P3b | 已实施、定向通过 | A05/A06；真实模型桥三阶段及原编码/拒绝边界验证 |
| P4 收口 | 已完成 | A07；最终冻结源码全 internal 测试、vet、8 个受控顶层场景和 2 项原生机制验证通过，详见验收记录 |

2026-10-20 文档轮检查：两份文档的仓内相对链接全部可解析，引用的既有测试入口已在源码核对；受控过滤对应 7 个现有顶层场景。验收编号 A00–A07 与阶段映射已核对，Markdown 空白检查通过；工作树差异仅 README 索引和本 Spec/Plan。未运行功能测试、构建或原生执行。前一轮只读调查的测试通过不是本轮实施或最终构建验收证据。


## 6. 实施收口

2026-10-20 已完成 P1/P2a/P2b/P3a/P3b/P4。原构建与最终源码/EXE、实际调用与持久样本、每阶段结果及早期失败统一登记在[验收记录](../acceptance/execution-boundary-refactor.md)。不以历史构建拼接最终结论。

最终规则命令为 `go test ./internal/... -p 2 -count=1 -timeout=20m`，exit 0；application 全组 657.235s。全 internal/cmd 的 vet 通过。初次 6 分钟不足以容纳完整 application 组，后续调整的是测试进程总预算，生产准备/原生调用/获取/清理期限不变。

最终受控使用 §4 的 7 个业务顶层场景加 `TestSRResultAcquisition`，共 8 个顶层场景，`-BaselineExe` 指向实施前保留的原 EXE；exit_code=0、scenario_count=8，无失败或 skip。结果获取含 11 个子场景；换代含 graceful/kill/creation_receipt/pending_stop 四项。原 parser 的 restart/kill 样本用原构建创建、最终构建结算；R2 原产物经最终构建写回和两次重启读取。测试复用现 baseline-exe 参数，不把旧 v2 专用测试作为当前 v4 的通过证据。

实际触发的原生机制采用新 opt-in 测试 `TestBoundaryNativeModelBridge` 和 `TestBoundaryNativeParserResources`；必须显式设置原生 EXE 和全新证据目录，缺参数的 skip 不计资格。真实 Codex 0.159.2 两项通过，原生机制与作用范围见验收记录；既有 Orca 创建接口/跨仓资格缺口仍保留。
