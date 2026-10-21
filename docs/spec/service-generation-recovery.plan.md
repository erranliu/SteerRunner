# Plan：服务换代后的原执行接管

日期：2026-10-19，状态于 2026-10-21 核对：核心接管、parser 保留、离线审计和调用方接线已实施并有定向证据；Orca 有限创建执行器已实现并受控验证，只调用现有公开接口。对应 [Spec](service-generation-recovery.spec.md)。当前构建、追补及未验范围见[近期 PR 补测](../acceptance/pr-test-gaps-20261020.md)与[创建恢复复验](../acceptance/pr38-review-20261020.md)；历史阶段、失败及跨仓限制保留在[专属验收记录](../acceptance/service-generation-recovery.md)，不因整理文档改记通过或部署。

2026-10-21 材料边界补充：下文实施时的 Material.Content/digest、原文件快照核验只在实际使用该类冻结材料的契约中适用。当前开发材料是固定基线 worktree 内的仓库相对路径，恢复和新阶段不再检查独立文档快照；conversation/generate 的 inline 材料与 parser 已冻结输入仍沿原来源规则。

## 1. 实施顺序与责任

沿现 application actor / Run Record / Store 单写边界实施。Runner 承担 W02–W16/W18 执行验证，SteerLine 只补 W01/W12 调用方授权、W17 交付及部署消费者验证，不在 SteerLine 重建执行专项矩阵。

### P0 固定能力与兼容基线

1. 固定 Spec §2 的 Runner/SteerLine 来源，复用现能力目录、record/envelope 版本及 decoder，列出精确支持范围。区分包 SHA、原 owner SHA 和契约版本；同 workflow version 不允许修改语义后强行放行。不新增 per-Run 兼容 manifest、恢复语义版本或兼容策略注册器。
2. 通过隔离无业务资源探针确认本机 owner 由现 Windows 独立启动机制创建后，服务正常退出与强杀均不杀 owner/Job/原生连接；确认当前 Codex 请求跨观察断连仍能完成且结果可读。所有权移交前不删除旧 Job 保护，不以杀树后同 thread 重建算通过。
3. Orca 创建恢复由 Runner 独立创建执行器实现，仅调用现有 worktree create / terminal create；不能依赖 Orca 源码修改。调用前固定原 operation/配置/路由/命令/期限，独占登记后只执行一次；执行器持有原 CLI，在服务退出后继续接收和可靠保存原回执。新服务只读原日志并核实 runtime/worktree/terminal/incarnation，不按名称认领。覆盖响应未到服务、回执未提交 DB；执行器也丢失或没有原资源身份时保持 unknown。
4. 核对原生结果持久性与审计 checkpoint 的离线窗口。缺原生来源/必要 checkpoint 的情况保持 unverified，不引入重跑源步骤或补报修代码来通过验收。
5. 完成旧本机活动树的首次升级可行性判定。原服务没有协作移交能力时，部署须在停服前拒绝破坏性切换并保留原执行；这只是存量保护，不能登记成无中断升级成功。P0 不以所有存量自然完成为后续新契约验收前提；明确新保证生效起点及无法追溯恢复的旧现场。
6. 固定 `service start` 可选 `pending_stops` 的最小 schema 和启动时序：复用原 stop 信封、commands 幂等、revision 校验及启动意图保存；在 worker/ready 前受理撤销，失败返回逐项事实而不派发业务。不增加全局持久 hold/release 接口，不等待 stop 的物理退出或未知操作结算才允许无关 Run 接续。

交付物：精确版本矩阵、Runner 独立创建执行器及原生能力/失败事实与小型探针证据。未取得某机制的本机存活或真实后端证明时，对应资格不宣布完成，不先移除旧保护。

### P1 契约与持久化

涉及 `contracts/resources`、`host/cli`、`storage/runstate` 和现 operation 入口，由 application 唯一提交。

1. 在既有 `version/capabilities` 中公开支持的版本集合，增加 `steerrunner-takeover/v1` 只读报告及 `service verify-takeover --candidate`；区分预检 compatible 与实际接管，不保存投影结论为第二状态。实现 P0 固定的启动 `pending_stops` 输入及其原请求幂等/拒绝诊断。
2. 实施 state/v4 / user_version=4 语义门，阻止 v3 服务重新写入并执行旧停服策略；保持现表、授权字段、Operation.Version=1 和操作状态。既有输入/后继与完整 Intents 共同判断是否曾建立副作用意图；已有 in_flight 只对账，不增加 not_dispatched/dispatching 状态、撤销代次、消费标记或导入游标。必需新增字段须用具体不可表达的崩溃反例证明。
3. 实施指定基线 v3→v4 的独占锁、整目录资格检查、一致性备份、原子升级和失败回滚。SQLite 在线备份或等价一致性快照必须含 WAL，不能直接拷贝活动 DB 文件。旧 SHA/原状态/命令幂等记录/事件序号/冻结文件保持，升级不生成业务输入、修改外部资源或重绑会话。升级前后逐条比对关键不可变字段，不要求合法新增字段或结算状态结构全等。
4. 对旧 Allowed=false/ResumeRequired、旧 epoch 答案和未知操作保留门槛；不推断新授权。未知必需格式拒写；可读退役记录隔离，不把旧远端 helper 搬回运行路径。
5. 调整 host selectedService：按 CLI 契约选活动服务，仍验证原实例/token/目录，构建 SHA 为身份和来源。具体 workflow 校验不再对整个 Profile 做 DeepEqual：沿现两个定义核对冻结执行语义及原 adapter 支持，Description/新增其它 adapter/新默认提示词变化不使旧 Run 失效；实际转移/contract 等同号变更仍拒绝。不引入泛型语义差异引擎。

### P2 原执行所有权与可找回的创建

涉及本机 `adapters/codex`、`execution/process`、现本机宿主入口；Orca 只扩展必要原生协议接线，不给远端重新引入 Runner helper。

1. 用原产品本机执行宿主模式承载每执行 owner，保留 Job/树管理、认证 broker 和必要原生连接；Runner 服务只持可释放的控制/观察连接。owner 版本文件使用不可变路径且不随部署删除。服务 Shutdown/observer close/preparation release 全部审查，禁止通过 defer Close、context cancel、管道关闭或环境清理间接结束原生业务树。
2. owner 不读写 runner.db、不运行 workflow、不复制授权状态，仅承载原 Codex 调用、身份/回执读取和精确 stop；现 application gate/revision/授权规则继续决定业务派发。用现 writer.lock、原实例/进程身份和唯一临时控制连接隔离旧写者，不建持久派发租约或通用 Operation 执行器。已经在途的操作只对账；认证策略/期限不因换代改变。
3. 将普通停服脱离与显式 Run stop 分为不同 port 语义。更新 `ShutdownLifecycle` 的调用方和本机 `stopRun`，显式停止不再借停服方法或旧内存 handle 执行；可通过原 owner 精确停止原 Job 并核验树/认证。保留 Orca 原停止证据 scope、未知结果及 reader 收尾规则。
4. 本机复用 process.json/thread.json/turn-start/原生 journal 与 implementation receipt，owner 持有必要原生连接，在 Runner 离线时继续接收并可靠保存原回执/结果。进程 root 绑定复用 `StartManagedBound`，服务重接按原 intent/source 去重导入。不得增加通用 receipt spool、消费确认协议或导入游标；owner 同时丢失回执的情况仍保留未知，不把多组件故障透明恢复作为本轮前置。
5. Orca 适配器按原 Runner 创建日志导入原 worktree/terminal/control 身份；保留部分成功 checkpoint。工作区、初始终端、native-control 共用有限独立创建执行器；未知/异源/损坏日志拒绝认领，恢复从不再派发 create。PR/引用沿原 provider 查询；所有未知输入/发布恒零重发。

### P3 接管与自动接续

涉及 `application/{engine,control,worker,observation,results,parser,workflow}.go`、可靠性/结果规则与既有 Store Commit。

1. 移除 New/closeWorker 对所有 Run 统一撤销授权和重写状态的策略；保留旧格式原门槛及真实业务阻断。授权复用 Allowed/ResumeRequired/PendingStop、原问题/来源及 gate/revision；service epoch 只管理临时连接和旧观察隔离，PendingResume 保留原命令状态。
2. 启动调度先处理 stop/撤销、未知操作与原生观察/补收，再决定原授权内尚未执行的工作。活动原树直接接回观察，不要求 ProcessAbsent/WritersAbsent；这两个门槛继续只用于原资源确实丢失后的既有人工恢复路径。
3. 依据现 StageInputs/NextStage/PendingTurn/控制命令与 Intents 的完整关联处理队列、答案、反馈、获取调用及后继；没有对应意图才可首次建立，已有意图只 reconcile。业务 blocked/waiting_input/stopped/pr_ready 不被自动 resume；过期问题等待人工新答案。
4. 补齐既有结果/获取/后继的启动唤醒和只读重放入口，保持 finish 原子提交结果/转移，直接使用 StageResults、获取结果/source 引用和已保存后继。不得新建恢复 route/消费标记或从非法缺字段重造后继。Git/provider I/O 沿现 gate 外只读、返回后重新核验及 CAS；stop 后迟到成功只记事实。
5. 接管报告从原 Record 派生，轮询无新事实不刷 revision。重复重启、结果再次导入或 observer 重放保持幂等；原命令回执继续按 request_id 查询，不让服务换代制造“必须重新提交”。Check 保留原报告/期限/清理义务，不重跑探针。

### P4 SteerLine 与部署消费者

这是必要跨仓后续工作，本轮不修改 SteerLine，也不宣称 Runner 单仓完成即可打通 UpdateRunner。

1. 在 `runnerclient/runnercontract` 增加恢复描述/接管报告的严格读取，保留安装文件 pin 校验；用兼容接管结果重建当前服务实例绑定，原候选 Run/input/resource digest/PR 不变。保留原命令信封与事件 after 游标，断线后重读投影和原 request_id，禁止重新 submit、重发 reply 或批量 resume。
2. 调整 `scripts/deployment-runner.ps1` 的同 SHA/新环境门槛与 `deployment-stop.ps1` 的证明选择：在旧服务停止前用新包预检，分别记录 service 已退出、兼容可接管、resources_settled。仍有原生执行时允许合格兼容更新，不能把 restartable/takeover 改称资源已停止。
3. 共享应用宿主的项目 actor/store/锁/checkpoint 仍按原顺序关闭；正常部署先向原服务提交取消/交付 stop，离线重开经启动 `pending_stops` 在 Engine.Start 前受理原撤销。验证受理回执丢失仍按原 ID 找回、revision 冲突明确拒绝、调用方重开重建启动输入；无需二次 release。未知资源结算只阻断所属 Run，不使已成功受理后的无关 Run 等待。
4. 暂存构建→兼容预检→旧 writer 退出和锁核实→无损升级（仅首次）→新服务接管→重建投影。失败保留原 DB/冻结资源/版本文件；旧 v3 EXE 不得启动 v4，v4 只回退到支持相同契约的构建。若新构建已发出任何新动作，不能把旧数据库备份直接覆盖当前数据库；先对账原事实，禁止复制备份制造双写或丢失新操作。
5. 同步共同生命周期入口的 W18/E04–E07/V06–V08，及本仓 README/总 Spec、Orca/结果获取/测试指南中当前“统一显式 resume”表述；只写本增量差异，不复制 W/E 状态机或改历史验收。首次部署预检失败、支持缺口、实际接管失败分别展示。

### P5 最小验证与资格收口

先完成下表 L1 和真实 EXE Controlled，再对每种拥有不同生命周期/结果来源的机制做必要原生验证。不同构建的结果不能拼成一个“全部通过”；记录双方 commit/EXE SHA/资源摘要/契约、工具版本、冻结配置、实际命令、故障屏障和原资源身份。测试成功清理必须使用显式 Run stop 并核实原范围退出，最后再停服务。

## 2. 最小必要测试计划

以下是验收设计，实际已验/未验以专属验收记录为准；新增测试名在实施时确定，旧用例名称仅用于定位复用。每行一个 owner、一组必要正反例，避免全节点×全故障×全 adapter 笛卡尔积。

| 计划 / Spec 验收 | 层级与最小场景 | 通过条件与证据 |
| --- | --- | --- |
| T01 / A01 | L1 + Controlled：两个实际 EXE A/B，改变 SHA/路径/总资源摘要而相关契约相同；Profile Description/能力目录扩展/默认提示词变化正例，workflow 同号转移变更与未知 codec/state/owner 协议反例 | 正例原目录接管，冻结参数/原结果/PR/输入摘要不变；反例明确隔离/拒写、零业务输入。具体语义反例放 L1，真实双构建只做一个正例；不能仅复制同一 EXE 冒充不同构建 |
| T02 / A01,A03 | L1 + SQLite：已知 v3 聚合→v4；升级中断、WAL、一致性备份、旧 EXE 拒读；含原 v1 operation/旧答案/旧 resume_required/退役远端记录 | 成功保留原授权/ID/key/digest/事实/事件序号及 Operation.Version=1；失败无半升级；不猜授权；兼容项不被退役项阻塞。旧本机活动树预检拒绝破坏性切换，单独登记限制 |
| T03 / A02 | Controlled 真实 OS 树 + 本机 Codex 原生：正常停服和精确强杀服务各一次；原业务在离线期保持活动/完成 | 原 owner/root/session/turn/generation 不变，工具树不被服务退出强制终止，允许原工具自然完成；服务 PID 换代，零 interrupt/kill/重复 prompt；接管成功且能显式 stop 原树/认证。覆盖 observer Close/context/IPC 断开和在用版本文件保留 |
| T04 / A03 | L1 表驱动 + Controlled 一个混合目录：authorized queued/active、waiting_input/原生问题、业务 blocked、resume_required、stop pending/unknown/stopped、pr_ready；含未发送答案与 pending resume | 可继续项自动推进，无人工 resume；等待/阻断/撤销保持原语义；已接受控制回执不作废。未知只读、过期问题不投递；重启两次后计数仍一致 |
| T05 / A04 | L1 协议夹具 + Controlled 创建屏障：原输入已存无副作用意图、in_flight 已提交但未调用、外部已创建响应未到、响应到达 DB 未提交、部分 checkpoint | 无意图者合法首次执行；已存意图者只对账，按原键找回同一资源。完整调用审计证明创建只一次；无登记/多目标/错 runtime/incarnation/同键异正文拒绝。不能用正确阻断替代丢回执正例通过 |
| T06 / A05 | L1 + Controlled 结果屏障：离线完成、原 final 已保存、分阶段获取未结算、合法后继已存但未建立意图/已有 in_flight、PR 已创建未存；另造非法缺失后继反例 | 原结果只结算一次、已存后继首次派发一次；已有意图仅对账，非法中间态不重建。重复导入/重启无新增实施/审计/报告/发布，预算不重置；必要 checkpoint 缺失保持 unverified |
| T07 / A06 | 复用现操作/结果/stop 竞争测试，补 stop 与原结果导入、旧观察迟到、CAS 失败、unknown input/reply/publish、同 turn 冲突 final | stop 已接受后零新派发；在途事实保留，错误来源不推进；原 PR/head/merged 不丢失、不替换，零重复 push/create/模型输入 |
| T08 / A02,A04,A05 | Orca 原生最小探针：Runner 原创建执行器跨服务保留及原回执读取；本机保留管理路径与远端 native 各一个服务换代样本，复用 T06 屏障 | 原 runtime/worktree/terminal/incarnation/provider 及原输入回合不变；远端零 helper。管理路径保留原认证/reader/退出证明，远端只承诺原 scope；无原身份保持 unknown，真实未跑项原样登记未验 |
| T09 / A07 | SteerLine 定向单测 + 脚本受控测试：同构建重启、兼容 UpdateRunner、Web/项目重建；错目录/实例/契约、本地锁未关闭；启动 pending_stops 接受/回执未知/revision 拒绝 | Task/Candidate/Run/PR/投影游标/既有唯一 Delivery 不变；启动前撤销生效，接受后不等物理收尾便允许无关 Run 接续；零重复 submit/reply/resume，项目隔离。兼容更新允许 resources_settled=false，破坏性清理仍拒绝 |
| T10 / A01–A03,A05,A07 | 复用 T03 正常换代的同一真实本机样本，由 SteerLine 发起 A→不同构建 B 更新并恢复原投影；不额外创建一轮开发任务 | 原 Task/Candidate/Run/session/turn/结果与后继对应，零人工 resume/重复提交；真实 PR 事实（若该样本产生）与独立 provider 查询一致。Delivery/merge 规则未变，沿 T09 受控验证已存事实，不额外要求创建/合并 PR 才算服务接管成功 |

T03 本机原生共两个 service 退出方式，T10 是其中正常换代同一现场的消费者断言，不再安排第三个真实业务 Run。T08 的 Orca 创建前置只做无业务小探针；随后本机管理/远端 native 各一个执行样本即可，不扩张为全网络权限矩阵。受控层覆盖其它故障排列、PR/Delivery 保存和拒绝反例。缺某机制的完整原生样本时，保持该项未验，不以泛化循环测试掩盖。

定向入口在实现后从仓库根运行（Windows/amd64 / PowerShell，具体新增 `-run` 过滤随最终测试名登记，需证明确有命中）：

```powershell
go test ./internal/host ./internal/storage ./internal/application ./internal/contracts ./internal/resources ./internal/cli -count=1
go test ./internal/execution/reliable/... ./internal/execution/workflow/... -count=1
go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca ./internal/execution/process -count=1
```

Controlled/原生沿 [测试指南](../testing.md)的独立现场入口、实际 EXE/CLI/IPC/SQLite 路径与超时/收尾规则，不新增生产 fault flag。先复用/修订 `TestSRRestartRequiresResume` 为有授权自动接续与原已暂停保留门槛两组；复用 `TestSRNativeUnknownAndStop`、`TestSRStabilityRevocationAcrossRestart`、`TestSRExpectedInstanceFencesReplacement`、`TestWorkspaceCreationReceiptRecovery`、`TestFixedControlInitializationUnknownBlocksWithoutReplacement`、`TestRAAtomicCompletion`、`TestRARecoveryMissingCheckpoint`、`TestRAConcurrencyPublication` 的相关边界。新 Controlled 场景须登记到现测试入口并检查零匹配；不得依赖旧显式 resume driver 的通过作为新正例。

因改动跨 host/storage/操作/授权/所有适配器，实施收口需要一次同构建完整 L1/Controlled 作为共享回归；以上定向组通过后不重复无关真机矩阵。SteerLine 仅跑受影响 consumer/部署/交付定向回归和 T10。不要求全量 L3 Repeat 3、管理员断网或其它未改权限变体。第一次失败保留，修复后只重跑受影响项及必要共同前置；最终资格明确每项来源，不拼历史结果。

## 3. 已有证据与待验项

| 证据类别 | 当前可采用的内容 | 不得外推的结论 |
| --- | --- | --- |
| 本轮源码调查 | Spec §2 的文件/符号、v3 格式检查、统一 resume、stop_execution、创建缺回执阻断、结果/NextStage/CAS 现入口；SteerLine 同 SHA 部署门槛 | 仅静态调查，不证明新 owner、契约预检、迁移、自动接续和 UpdateRunner 已运行 |
| [可靠模块化既有记录](../archive/2026-10/spec/reliable-modularization.spec.md) | 原操作身份、两种操作表示的边界、受控结算/重启样本 | 不证明任意历史格式兼容；不能作为跨构建原生树保留证据 |
| [固定控制证据](../acceptance/evidence/fixed-control-20261019/README.md) | 对应旧构建/旧远端机制的执行期原身份、原 final/停止取证、显式恢复样本 | 不证明初始化创建未知可找回；不用于当前远端 native 或自动接续资格 |
| [远端原生验收](../acceptance/orca-remote-native.md) | 原生身份、完整 final/unknown 边界、停止 scope 与消费者受控记录 | ORN-A05 仍是显式恢复，不证明本次五个窗口/兼容换构建 |
| 原文档轮所读的旧测试定义 | 显式 resume、操作未知零重发、结果来源/预算/CAS、原精确 stop 等可复用反例 | 文档轮未执行；后续实施运行结果见专属验收，源码存在不算通过 |
| T01–T10 / SGR-A01–A07 | Runner 部分已实施并进行 L1/Controlled；Orca 原创建查询、原生样本和 SteerLine 消费者单列未完成项 | 任一必需正例未取得不能登记“全部目标完成”，失败/未验不被历史通过抵消 |

实际实施与验证登记到[专属 acceptance](../acceptance/service-generation-recovery.md)，保留首次失败与修复后重验，不修改历史验收结果。

## 4. 文档质量审计与设计收敛（2026-10-19）

主会话依据现有源码复核，未派生独立审计 agent。本节记录原稿的问题及本次文档修订，不代表任何实现通过。

| ID / 优先级 | 原稿问题与结构负担 | 源码依据 / 本次处置 |
| --- | --- | --- |
| Q01 / P1 | 单独再加 not_dispatched/dispatching、Operation v2、消费标记和导入游标，把现流程拆成更多提交点与可矛盾状态 | `operation.New` 已先存 in_flight，StageInputs/NextStage 与完整 Intents 可区分尚未建立副作用意图；finish 已原子提交结果/转移。删除平行状态/标记，保留已有意图只对账的保守窗口 |
| Q02 / P1 | 全局持久 hold + release-dispatch 形成另一个授权生命周期；调用方退出会遗留服务级闭锁，并可能等待无关未知资源 | 现 Run 已有 Allowed/PendingStop，Engine.Start 是明确调度边界。改成仅原 pending stop 信封的启动排序输入，worker 启动前保存撤销；无需全局持久 hold、release 或等待物理结算 |
| Q03 / P1 | owner 既核授权又建通用操作 spool/消费确认，逐步成为第二执行权威 | 现 Run Record 是唯一聚合，Codex 已有 process/thread/turn-start/journal。owner 仅负责原生存活/连接/回执/停止，授权仍归 application；复用原文件和操作/结果身份去重，无通用队列、确认游标或授权副本 |
| Q04 / P1 | 要求各类 native create 在 owner/原生端同时崩溃后也统一精确 lookup，把 Runner 恢复扩成多系统任意故障恰好一次，P0 无法收口 | 故障主体是 Runner 服务，独立宿主/Orca 同时消失另有边界。服务断开时由 Runner 独立创建执行器继续接收/保存原响应；多组件丢失证据保持 unknown，不依赖新增 Orca 查询接口 |
| Q05 / P1 | 沿用 Workflow.Config 全结构相等仍会被 Description/能力目录变化阻断，“删 SHA 门槛”后目标仍不成立 | `workflow.Profile` 含 Description/SupportedAdapters 等非原执行语义字段。改为两个具体 workflow 的冻结执行语义校验，保留真实同号语义变更拒绝；不另建通用兼容引擎 |
| Q06 / P2 | 每 Run 保存兼容 manifest，再叠 recovery 语义版和 Operation v2，形成重复版本权威 | 用已有 record/envelope 版本及能力目录；删除额外 manifest/语义版/Operation 升版。仅保留 state/v4 防旧写者、owner 协议 v1 和确实增加 owner 绑定的 resource codec2，各自有明确边界 |
| Q07 / P2 | 为接管再跑一轮跨仓开发、真实合并/Delivery，重复既有测试且把未改交付语义变成本轮门槛 | T10 改为 T03 正常换代同一现场的消费者断言；已存 PR/merged/Delivery 的保存用 T07/T09 受控验证。创建缺回执、正常/强杀本机存活等关键正例仍保留，不能仅以反例正确阻断替代 |

保留的必要结构：独立本机 owner 解决服务持有 kill-on-close Job/认证/连接的生命期耦合；state/v4 阻止旧服务重入执行旧停服语义；有限创建执行器补齐 Orca 已成功但服务未保存回执的窗口。旧本机活动树无法事后无损移交、原执行器也缺证据时不可找回两项边界仍在。

上述为首次文档审计时的结论：当时 P0 原生能力验证和启动输入 schema 尚待固定。后续实现及接口固定见 §6，实际资格看专属验收；不把这轮文档审计计为 SGR-A01–A07 通过。

## 5. 首次文档轮检查（历史）

以下为实施授权前的文档轮记录，不代表当前实现的工作树范围或功能资格；实际执行见专属验收记录。

交付检查范围：Spec/Plan 相互链接及本地引用、SGR-C01–C07 与 SGR-A01–A07/T01–T10 映射、表格/围栏完整性、`git diff --check`、最终 diff 只含文档。本轮不运行功能测试；检查结果在交付时说明。

2026-10-19 质量审计修订后，两份文档的本地引用、表格列数、围栏和行尾空白检查无问题，`git diff --check` 通过；工作树改动仍限 README 和这两份文档。未执行功能测试或原生探针。

## 6. 实施阶段接口固定

`service start --input -` 接受裸 `{ "pending_stops": [{ "run_id": "...", "request": { "protocol": "steerrunner-cli/v1", "request_id": "原 stop ID", "input": { "expected_revision": 1 } } }] }`，最多 500 项、8 MiB。省略 input 保留已有未决启动撤销；不能用空数组删除未决原请求。worker/ready 前顺序受理，原 commands 幂等先于 revision 校验。已受理请求留在原 Run/commands；任一确定拒绝即返回原故障且不开 worker，启动文件可移除，调用方查询各原 request_id 并修订明确被拒绝的项。提交结果未知保留原文件，禁止换 ID 猜重做。服务已 ready 时拒绝非空启动撤销，调用方应对当前实例提交原 stop。此接口只表达原撤销义务，不是持久全局调度闸门。

P4 已在 `C:/Project/SteerLine` 的基线工作树实施公开 CLI 消费者、application/project 当前 pin、启动撤销收集及兼容部署路径。规则与最小验证见该仓 `docs/spec/service-generation-recovery.spec.md`、`docs/plans/service-generation-recovery.plan.md` 和共同 worker-lifecycle。候选/Check 的原 pin、原信封和 PR 不改，只有当前服务 checkpoint/home pin 经预检更新。无安装环境操作。2026-10-20 创建恢复限定 Runner 实现；有限执行器不伪造上游能力，真实未跑资格仍按实际证据登记。

已补入 parser 的具体 owner 动作/原 started、final receipt 对账，以及原生完成事件发布前的签名 workspace checkpoint；复用原 Deadline、源码检查和结果路由，无新执行状态机。接管预检同时校验 result-acquisition/v1 配置与冻结 parser profile/schema，并只读核对原 Material.Content/digest、快照原字节、普通文件与非重定向路径；新 native 输入派发前沿既有 source 检查再次核验，不重写缺失/改动文件。T01 补 `TestRecoveryFrozenMaterialsRejectReplacementWithoutRepairingOriginal` 的替换/缺失和零修复反例，受控 A/B 和审计/parser 正例验证正常文件接续。真实 Codex/认证/工具树和 Orca 创建查询仍需按原要求收口，不以受控协议通过替代。

2026-10-20 主干合并补充：conversation/1、generate/1 复用同一 Run/owner/控制范围，不另建恢复机制。T04 补已接受未冻结 message 跨代仅冻结一次、已冻结后二次重启零重建及旧 epoch/错会话拒绝；T01 补两种内容 workflow 的只读兼容核验。T03 复用内容会话受控重启，核对原进程回执/存活、原 thread/轮次数和无新增 resume 门槛。合并后的新证据与历史原生资格分开登记，保留主干 gh-only 认证退役边界和新增 run-evidence 查询。

补充最小测试计划（T03/T06、A02/A05）：沿 `TestSRResultAcquisition` 的 `audit-offline|parser-restart|parser-kill|parser-stop|parser-unsupported` 子场景，Windows 隔离 Git/真实 Runner EXE/受控 native。parser 以文件屏障停在原进程；正常/强杀 Runner 后核对原 root/owner 存活，离线放行同一 parser，再启动 Runner，原启动计数 1、无额外报告/业务预算；显式 stop 必须结算原树且零后继。审计在第二 turn 完成前停服务，离线取得原签名 checkpoint，发布前 native turn 数保持 2，重开到原 PR ready。签名/错身份拒绝用 `TestExecutionOwnerCheckpointPinsOriginalTerminalAndSignature`；请求投影用 `TestTakeoverRequestFactsUseOriginalAcceptedCommands`。每轮固定 EXE/hash、命令、caller/native calls 和结果到专属验收证据目录；失败和修复重验分别登记。T09/T10 的消费者最小命令由 SteerLine Plan 承接，真实现场未执行则保留待验。
