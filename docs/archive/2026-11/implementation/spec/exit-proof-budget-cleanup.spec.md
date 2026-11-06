# Spec：退出证明生产端与旧预算重建清尾

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-01。状态：按后续实施授权完成 E/B 源码变更与最小验收，见[实施报告](../acceptance/exit-proof-budget-cleanup.md)。不改存储/协议格式，不安装或处理用户已有 Run；历史失败单列。

审计基线：main `a06c2806c17056d76cb092b6a1045a0473765fb8`，已含 #80；本工作树 HEAD `fca735a` 与该 main 文件树相同。依据[长期产品承诺](../../../../spec/steerrunner.spec.md#长期产品承诺) PC-R01–R06/R08/R10、[已实施停止范围](orca-local-codex-stop-simplification.spec.md)及[输入预算契约](../../../../spec/workflow-input-budget.spec.md)。配套 [Plan](exit-proof-budget-cleanup.plan.md)。

## 1. 审计结论与范围

两项可分开实施、验收，互不作为前置：

- **EPBC-E：退出证明生产端清尾。** 本机 Orca 受管 Codex 已按登记资源结算。专用退出文件的生成/签名/读取验证不再有当前生产结算消费者，但仍参与 helper 启动与退出。可删除这些专用机制，保留实际终止、停止请求绑定和仍被业务/直接 Codex 消费的共享机制。
- **EPBC-B：删除缺预算时的文本反推。** 审计未发现正常 v8 生产入口生成“已冻结非空 Prompt、StageInput.Budget 缺失”的记录；正常新输入都会一起生成文字和 Budget。可以将这种缺失改为禁止新派发的异常，删除从提示词/材料/反馈标签重建来源的分支。尚未渲染的草稿、复用已保存 Budget、读取及对账历史事实仍合法。

实施前结论来自静态调用和赋值路径；实施时已先完成 §8 的受控生产入口矩阵，再删除反推。后续若扩展入口：若找到当前合法生产入口仍生成缺 Budget 的冻结输入，先修正该入口并补验，或暂停 B；不能仅因名称含 legacy 就删除。

E 只作用于 `orca-terminal`、本机 Codex、Environment 为空、Terminal 配置为空、RuntimeExecutable 非空的受管 helper。B 作用于 application 冻结的 Workflow 阶段输入，包括 development/2、development-audited/2、execute/1、conversation/1、generate/1，不扩大到其他输入机制的预算重构。

不重开已完成的 P1–P4 或 #80，不改直接 Codex/Pi/DSH 停止范围、原生远端、模型桥、独立 parser、审计业务规则、权限准备、构建指纹体系、恢复框架、PR 发布或外部 SteerLine。

## 2. 实施前退出证明的实际消费者

| 对象 | 当前实际调用/消费 | 处理 |
| --- | --- | --- |
| [remote_agent.go](../../../../../internal/execution/adapters/codex/remote_agent.go) 中 newRemoteExitWriter、exitWriter.write/close、RemoteExit | helper 启动前开专属 exit.json；准备完成后退出时生成签名、Write/Sync，失败并入 helper 返回错误 | 删除这组生产调用、文件句柄、专用证明对象及错误分支 |
| [remote_exit.go](../../../../../internal/execution/adapters/codex/remote_exit.go) 的 VerifyRemoteExit | 当前代码仅 remote_exit_test.go 调用；无生产调用 | 删除专用 verifier 和仅证明该已退役规则的测试 |
| RemoteExitPath | RemoteStopPath 使用其目录定位 request.json；三个当前 native/stop 故障注入测试使用 exit.json 定位 | 保留纯定位用途；不以此函数存在为由继续创建/读取退出文件，不强制为改名另起协议 |
| RemoteReadiness.ExitPublicKey | 停止请求地址；local readiness、history/reader 及 receipt_audit 的完整绑定比较；真实 probe 校验长度 | 保留字段、编码、32 字节公钥形状、原值比较及路径计算。绑定生成与退出 writer 解开 |
| RemoteStopPath / awaitRemoteStop / UploadWorkspaceArtifact | 当前 scoped stop 上传完整原 readiness，helper 比较原请求后结束已有执行 | 保留原路径、完整正文匹配、原 runtime/worktree 路由及有界请求；不新增 RPC、凭据或消息 |
| excludeRemoteExit | 对 native-stop 目录增加 Git exclude；目录中仍有 request.json | 保留请求文件的排除和既有错误处理，不能把它全部当作退出证明开销删除 |
| signedRemoteExit | [execution_owner.go](../../../../../internal/execution/adapters/codex/execution_owner.go)、[owner_checkpoint.go](../../../../../internal/execution/adapters/codex/owner_checkpoint.go) 写/验 owner-stop 和工作区 checkpoint | 保留共享签名封装及这些真实签名/验签、各自密钥；不全局删 Ed25519 |
| settleRemoteTree | helper 的 app-server/TUI 实际清理；history.go、remote_turn.go、remote_workspace.go 的 reader/工作区来源证明 | 保留 Kill、Job accounting、根身份核实、Close、期限和错误传播；不能替换为只关句柄或只看 PID |
| [protocol/orca/artifact.go](../../../../../internal/execution/adapters/protocol/orca/artifact.go) 的 ReadStopArtifact / decodeStopArtifact | ReadStopArtifact 只有 TestOrcaLocalIPCReadsOnlyBoundStopArtifact，decodeStopArtifact 另有 TestStopArtifactRequiresCompleteBoundedBytes；均无生产调用（解码专属测试在实施编译时补查发现并删除） | 删除专用退出读取/解码包装及其专属测试。保留同文件 UploadWorkspaceArtifact，以及共享 IPC/files 通道 |

另有[2026-10-19 历史取证源码](../../../../acceptance/evidence/rebase-control-20261019/README.md)中的 export-exit/verify-proof 调用 VerifyRemoteExit、ReadStopArtifact。它们属于固定旧提交的归档工具，未被当前 scripts/tests catalog 作为现行验证入口引用；[evidence/go.mod](../../../../acceptance/evidence/go.mod) 也已将历史源码隔离于当前产品模块，审计用 go list 确认其不属于主模块，不计当前生产消费者。保留历史文件及字节，需要重跑时用其原版本，不为它们保留当前兼容运行栈。不能宣称这些旧源码还能直接针对新主线编译。

## 3. 实施前预算重建的实际消费者与合法入口

### 3.1 两个直接消费者

[ensureStageBudget](../../../../../internal/application/input_budget.go) 同时承担两件事：Budget 为空时反推归属，以及 Budget 已存在时通过 InputParts/原生归一化/FreezeInput 校验。只有前一件是删除目标。

| 入口 | 当前作用 | 删除后 |
| --- | --- | --- |
| queueInput 的 Prompt 非空分支，[freeze.go](../../../../../internal/application/freeze.go) | 原冻结输入复用/技术恢复；可以触发旧反推，也校验已有 Budget | 原文字和已有 Budget 一起校验；缺 Budget 不重新渲染或推导，不生成新输入 |
| prepareRun 的 PendingTurn.Budget 为空分支，[preparation.go](../../../../../internal/application/preparation.go) | 查找原 StageInput，调用 ensureStageBudget，再复制 Budget 到 PendingTurn 并保存 | 可保留“从精确绑定 StageInput 复制已有 Budget”的窄路径；StageInput 自身缺失/无预算或绑定冲突则拒绝新派发，不推导、不改原意图 |

TestBudgetCallerOwnershipAndFeedback 中手工将已生成 Budget 清为 nil，验证旧补算成功；这是反推分支的显式测试消费者，不是正常生产写入者。须把相应旧断言换成“缺失拒绝且零派发”，保留同一测试的真实来源/反馈/Unicode/调用方正文不限量断言。

### 3.2 合法来源矩阵

| 来源 | 生产路径 | Budget 的来源与应保留行为 |
| --- | --- | --- |
| 开发、带审计开发、execute/1 首次阶段 | prepareRun → queueInput，或其初始阶段选择 | 新 StageInput 的 Prompt 为空；在当前 owner 装配片段，FreezeInput/DescribeInput 同时生成 Prompt/Budget，之后再保存 StageInputs 和 PendingTurn 副本 |
| 实施后审计/发布、一次 report 等后继 | workflow/route.go 生成未渲染的 StageInput → results.go → queueInput | transition.Next 未填 Prompt/Budget 是合法草稿；首次渲染才冻结，不能在草稿阶段因 nil 拒绝 |
| 业务等待后的显式反馈/答案 | feedbackValue 创建新 StageInput，正文经 queueInput 装配 | 新正文与来源一起冻结，不能用文本标签反推替代已知的 caller.feedback 片段 |
| conversation 的 message | contentMessage → queueInput，FeedbackDecisionID 绑定原命令 | contentParts 以 caller.message 直接描述文字；这是当前生产构造，不是从 History 重建 Budget |
| generate/conversation 首次内容输入 | freezeContent 建立材料/目录；prepareRun → queueInput → contentParts | 指令、材料 JSON 转义、schema、固定仓库说明在原位置确定归属后冻结；read_only 与材料固定规则保留 |
| 暂停后尚未渲染的 NextStage | results.go 保存 transition.Next；resumeWorkflow → queueInput | Prompt 为空的草稿按原授权首次渲染，须复核当前继续资格；不是旧预算补算 |
| 已渲染但未消费输入的保留/恢复 | preservePendingStage 从原 StageInput 复制；resumeWorkflow → queueInput | 复制同时带上 Budget，校验原文字/来源，不追加当前默认提示、文件说明或反馈；不能删掉这个正常恢复分支 |
| 服务重启与 SQLite 加载 | storage/runs.go、Record.Clone、reliable.ApplySessionReceipt | 序列化保留已有 Budget；回执合并不重建 Prompt，已接受时清除 PendingTurn。审计未发现正常 writer 把冻结 StageInput.Budget 清为 nil |

原生问答的 AnswerInput 编译、bootstrap/developerInstructions、adapter 原生头/换行编译、模型桥、parser 自身文本预算不经过该旧 StageInput 反推分支，均保留其当前预算检查。`DescribeInput` 也被这些正常入口使用，不能因为其注释提及 legacy 就删除。

## 4. 明确删除与保留（EPBC-E01/B01）

### 4.1 退出证明生产端

必须删除：RemoteExit、VerifyRemoteExit、remoteExitWriter 及专属 new/write/close；RunRemoteAgent 对它们的启动/退出调用；退出证明专用 Write/Sync/长期文件句柄及其错误晋升；ReadStopArtifact/decodeStopArtifact 和失去消费者的专属测试。

绑定生成不再接受工作区或打开文件。本方案选择保持现有 Ed25519 公钥生成形式，保留公钥用于原 readiness/请求地址，生成后立即清除不再使用的私钥；不存储私钥、不签专属退出文件。这样不改变公钥语义、字段或请求路径，不借清尾设计新的随机 token 协议。

必须保留 §2 表列的停止路由、纯路径定位、Git exclude、实际 helper/Job 清理、reader 证明和直接 owner/checkpoint 签名。helper 自身资源未结算的错误仍须报告。新的“不产出 exit.json”不能让任何新旧读取路径要求它，也不能为了无文件场景补造假的退出证明。

### 4.2 旧预算反推

必须删除 ensureStageBudget 中 Budget==nil 时的三类推导：从 History 找 caller.message；拆旧内容标签/材料 JSON/schema；拆 PromptBodies/反馈标签及后缀。删除仅为此补算而服务的保存/修复分支，且不把同样推导移入 adapter、storage、CLI 或恢复逻辑。

必须保留：

- 新输入在现有装配位置生成 Budget；已有 Budget 的 Prompt SHA、provenance SHA、片段来源、UTF-8 与 C/R/T 一致性校验；Runner 1000B、调用方正文不限量及传输保护。
- Orca 的逻辑正文与原生实际文字分别计量、CRLF/尾换行规则、命令头和模型/effort 绑定；不通过修改正文绕过缺预算。
- 尚未渲染草稿、原已有 Budget 的恢复、原问题回答预算、parser 的独立例外。
- model/input.go 中现有 `legacy.wrapper`、`legacy.content.*`、`legacy.parser` 来源标识的读取支持。本版代码曾能补算并保存这些标签，删除生产者不等于可以让已保存 Budget 校验失败；本批不清理共享 allowlist。
- TestBudgetLegacyDefinitionKeepsTextButNotRoutingChanges 所保护的原冻结文本/路由边界；名字含 Legacy 不构成删除理由。

PendingTurn 预算副本缺失、原 StageInput 有合法预算时，只能在 CommandID、StepID、InputVersion、Attempt 和 Prompt 等原绑定一致且尚无可能外发意图时复制原值，沿原事务保存；不扩大为通用记录修复器，不补写已持久操作的正文或预算。

## 5. 异常、历史及版本边界（EPBC-C01）

本方案保留 state/v8、user_version=8、CLI v1、Workflow 版本及原生 readiness/停止请求编码。公钥字段/形状、路径与完整请求匹配保持；缺 Budget 不生成新格式，也不自动迁移记录。helper 字节/SHA 改变后沿现有构建冻结及升级先排空规则，不携带活动 Run 换 helper，不回写原 RuntimeSHA256。

输入预算原 Spec §4 允许只凭原冻结数据可靠补算旧输入来源；本次实施已显式收窄该条及其验收表：v8 已冻结输入的新派发只接受已有合法 Budget，缺失则保留现场并拒绝候选；不再保证替缺字段输入恢复预算。其代价是这类受损或不完整现场可能需要本人处理，即使旧算法曾能从正文算回。

已有预算（包括 legacy 标签）继续读取和校验；已派发/accepted/unknown 的操作按原身份只读对账，原接受、结果、失败、停止及 PR 事实不改写。预算异常只阻止新的输入，不能升级为整个数据库 unsupported_schema、阻止 inspect/history/原命令读取、观察原结果、stop 或离线资源核验。

本次实施已生效上述新派发边界。后续若发现缺 Budget 是合法 v8 writer 的必然输出，或必须改变存储/原生合同才能成立，暂停对应增量并修订方案，不暗加 v9、迁移器或兼容矩阵；E 可独立完成。

## 6. 保留的可靠性约束

原 request_id/正文幂等、单 writer、意图/回执两侧提交、gate/CAS、可能外发后零重发、原资源与结果归属、停止后零新输入均保持。不能把旧文本补算改成重新调用当前渲染器、重新授权业务、重建 session 或重复输入。

E 不降低 Job 实际清理、正常 reader/工作区来源和直接 Codex 签名规则；B 不取消输入预算本身，也不改变审计结果、Workflow 完成或 PR 核验。协议和预算读取只接受现有明确字段，不新增通用策略/权限/文件系统框架。

## 7. 实际收益与计数口径

以下结构目标已实施并核对；E 生产新增 7/删除 110，净减 103 行；B 新增 25/删除 108，净减 83 行。实际构建、计数、首轮失败及真实 READY 验证见验收报告，没有可比耗时加速结论。

| 增量 | 可验证收益 | 保留成本 |
| --- | --- | --- |
| E | 每个成功准备并正常退出的 helper，专属退出文件预开句柄、退出签名、Write、Sync 各从现有一次路径降为 0；取消证明文件无法建立造成的启动拒绝；专属生产 verifier/读取包装归零 | 公钥绑定生成、request.json 上传/匹配、Git exclude、原 readiness/配置文件、Job 实际清理及其等待保留，不宣称整个启动/退出零文件或零证明 |
| B | 三类正文反推规则及补算保存路径归零；恢复不再解释旧提示词、材料编码或反馈标签来发明 Budget | 新输入装配、已有预算的校验/复制和旧来源标签读取保留，不宣称所有预算代码或恢复入口删除 |

实施分别统计 E/B 的生产新增、删除和净变化，含必要调用点/测试接线成本，纯移动不算收益。目标为各自明确规则减少且生产净减少；若必要适配抵消净删，按实际报账并重审范围。不能用测试删行冒充生产收益，不能把保留的 Job 等待算作已消除的延迟。不设固定提速比例，不为量化强跑全后端模型矩阵。

## 8. 最小验收

| ID | 必须证明 | 最小验证 |
| --- | --- | --- |
| EPBC-A00 | 本文/Plan/长期及预算专项状态一致，消费者、合法草稿与历史归档分类明确 | 本轮静态调用/赋值盘点、文档链接和 diff 检查；仅文档资格 |
| EPBC-A01 | E 不创建/签名/写/Sync 专属退出文件；证明导出故障不再作为绑定生成/启动前置 | 纯绑定生成边界 + 专属证明 I/O 反例/调用盘点；保留其他真实配置/请求错误。不能只破坏同时承载 request.json 的整个目录来冒充仅退出证明反例 |
| EPBC-A02 | 原请求地址/公钥/完整正文匹配保持，协作请求与原终端终止仍发生；错 readiness/身份和未知响应不引发重发 | 原 stop/response-lost 夹具，精确计数；停止后零新业务输入，最终同 scope |
| EPBC-A03 | Job 与 reader 实际终止、直接 owner/checkpoint 签名的正反例保持 | 原真实 Job 子进程和 reader 失败创建用例；直接 owner/checkpoint 签名篡改/异源拒绝 |
| EPBC-A04 | §3.2 所有当前合法生产路径均产生/保留可校验 Budget，未渲染草稿不被误拒绝 | application 入口矩阵：五种 Workflow、report/后继、反馈/message、两种 NextStage、重启；含覆盖正文、Unicode/JSON 转义、文件说明、原生换行，不靠手造合法预算替代调用真实构造点 |
| EPBC-A05 | 已冻结且缺 Budget 的新候选拒绝，已有合法预算原文不变；窄副本复制不反推 | Stage/副本各自缺失、正文/来源/命令/版本冲突、保存失败，断言零新 Execute/create/send、未保存推导结果；合法原预算恢复仍可继续唯一未派发后继 |
| EPBC-A06 | 已 accepted/unknown/结束但缺预算的现场仍可查询、只读对账及停止，不重写原意图或结果 | 原操作/事务/停止及公开 CLI 受控；含重启、迟到 final、同 ID 回放、stop 竞争；不能只测“报错” |
| EPBC-A07 | 已保存 legacy 来源片段、旧默认正文和正常预算上限仍有效；原生回答/parser 预算不受影响 | model/input 的现有 SHA/来源/计量反例 + application Budget/answer/parser 定向；旧文本不被当前默认值覆盖 |
| EPBC-A08 | 没有专属旧证明/补算逻辑迁移或漏掉当前测试消费者；原生测试仍可编译 | 全仓引用分类复查；正式 native/integration 用 `go test -c -tags live` 编译，归档工具继续按固定旧源码，不拿 TestMain 零场景计通过 |
| EPBC-A09 | 修改后的真实 helper 能准备、停止并按原范围离线核验，始终不需要 exit.json | 复用 TestOrcaStoppedNativeProbe 的一条隔离本机 Orca/Codex READY 闭环；绑定 Runner/helper SHA，原终端/helper/root 结束、无证明文件核验成功、只读字节不变并清理自有资源；业务输入 0，不授予任务/PR 资格 |
| EPBC-A10 | 删除规则、实际生产增删、失败历史与局限可核对 | 按 §7 分增量登记真实源码/构建和调用数；无等条件计时不报加速 |

E 的独立完成要求 A01–A03/A08/A09/A10；B 的独立完成要求 A04–A08/A10。A00–A10 的实际选择器、通过/跳过和边界见实施报告；非完整 L1/L2/L3。只因 E 修改真实 helper 而要求 A09；纯预算分支删除不增加五种 Workflow 真模型或完整 L1/L2/L3 前置。实际越界时补对应最小验证，不删仍有消费者的断言以换通过。

## 9. 文档和交付边界

本次已分别更新状态、记录实施前消费者及删除结果；长期文档已去除本机专属签名写入的保留描述，预算专项旧补算条款已收窄为已有预算校验。保持当前已完成的停止范围、P1–P4、历史验收和归档工具结论，不改历史字节。

依据后续用户授权，本批审计后提交、推送并创建代码审查 PR；不部署、安装、排空或处理用户旧现场。真实探针只有 READY 与停止、业务输入 0，不创建真实模型业务任务或业务 PR。历史取证工具保持原字节，不取得新版本编译资格。

实施后再审计：见[报告](../acceptance/exit-proof-budget-reaudit.md)。生产逻辑未再变动；修正 accepted 资源受理测试夹具，补上九个缺预算原结果/停止回归，最终构建与 PR 验证以该报告为准。
