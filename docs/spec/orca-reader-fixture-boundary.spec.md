# Spec B：Orca 历史读取转发与共享夹具边界

日期：2026-11-06。状态：设计审计后已实施，B 独立最小验收完成，见[验收](../acceptance/orca-reader-fixture-boundary.md)。调查基线 main `6dd4360a42314e65b427627508ffd2ee3e4287da`，承接 [A Spec](orca-stop-contract-boundary.spec.md)/[Plan](orca-stop-contract-boundary.plan.md) 的独立通过点；配套 [B Plan](orca-reader-fixture-boundary.plan.md)。依据[长期产品契约](steerrunner.spec.md#长期产品承诺)和[Orca 长期设计](../orca-execution.md)。设计阶段仅交付文档；本轮按用户追加授权实施，B 前已整合 main `45818503`（#126），A 原通过点保留。

## 目标和审计结论

当前 NewLifecycle、服务启动及 application.port 只允许结构化 Agent Session 执行。#121 已把历史证明迁至 frozenRecord，#124 已删旧计算器，#123 已收口构造解码；本批不重复这些工作。缺口是历史测试仍用可执行对象形状的夹具，促使 Lifecycle 保留一批旧读取转发和旧状态编码方法，当前结构化测试又从该历史夹具起步。

目标是历史格式测试直接消费历史 reader，当前执行/创建测试使用明确的当前执行或共享能力装配；消除由测试维持的旧 Lifecycle 转发，不再要求开发当前结构化功能时理解旧 control/receipt API。只搬文件或改夹具名字不算完成。生产历史校验仍共用原 frozenRecord，不复制为一套测试专用校验器。

## 实际消费者、合法入口和例外

| 代码/夹具 | 已核实的消费者 | 边界 |
| --- | --- | --- |
| Lifecycle.resourceState/state/stateVersion/envelope | 旧 codec/control/receipt/stop 测试；其中 original 仍调用 state | 历史测试改接 frozenRecord；旧 envelope 编码辅助转为测试私有；original 的 state 调用可直接复用 frozenRecord，同样校验不变 |
| Lifecycle.usesControl/controlSetup/validateControl | 旧 control 和 shared native 读取测试；实际生产校验在 frozenRecord | 删除 Lifecycle 转发，保留 reader 实现和历史控制格式/断言 |
| Lifecycle.localCodexScope/localStopObjects/verifyLocalCodexStopped | 旧 scope/停止读取测试；独立 VerifyStopped 已直接消费 reader | 删除转发，保留原 scope、原资源探测和缺证明拒绝 |
| Lifecycle.validateTerminalReceipt/readTerminalOwner/terminalOwnerBinding/terminalExitVerified/validateTerminalTurn | 旧 receipt/access-mode 测试；原历史解码/停止仍使用 frozenRecord 的同名实现 | 删除转发，保留原 hash、owner/Job、退出文件及完整严格验证 |
| Lifecycle.verifyStopped/codexReadiness | 旧停止与 readiness 读取测试；导出 VerifyStopped 已使用 frozen reader | 删除转发，保留真实导出入口与 reader |
| fixture/fixtureOn/frozenFixtureLifecycle/frozenWorkspaceFixture | 被上述历史测试、structuredFixture、workspaceFixtureOn、CLI/creation 和原终端并发测试共享 | 必须按消费者逐个迁移，不得整段删除共享基础设施 |
| fakeHost/request/bind、creationTestHost/owner 和 IPC 服务 | 当前结构化、创建/原操作对账、原 CLI 编码和并发测试均有消费者 | 保留现有能力与有效断言，不把“历史文件名”当成无消费者 |
| #126 ProgressHost 及 profile_construction/progress_host 测试 | B 开始前主线新增的共享 fixture 消费者 | 整合 main `45818503`，保留其原路由/严格错误和数据不变断言；上游 ProgressHost 的单次解码不算本批变更 |

本批有意保留两个非纯转发例外：Lifecycle.original/boundTerminal 及 `TestOriginalTerminalConcurrency`。它们核对原终端读取/RPC 并发，不属于可以直接删除的薄转发。为保持这项测试可保留一个用途明确的测试侧原终端装配，不再作为通用历史 codec/receipt 或当前结构化夹具的基座。除 state 调用直接改接同一 frozenRecord 外，不修改其生产读取/身份/错误语义，不另造一套仅供测试使用的生产 reader API。消除这两项例外不是本批验收要求。

另有 `TestCodexStoppedRequiresNativeExitProofBeyondPTY` 的实际 ShutdownLifecycle 调用：原断言要求旧快照交给当前端口仍不能报告 stopped。历史读取部分改接 frozenRecord，当前端口的这条拒绝断言用最小 capability 装配保留；不能给 reader 补一个 ShutdownLifecycle 方法，不能删除断言或把它改成 reader 的自我检查。这不增加生产方法删除/保留例外。

`TestOrcaCLICommandEncodingAndConsumers` 不只是读取测试：它直接核对 CLI 命令和 creationCall 的原回执/Golden 字节，并使用原路由装配。不能把它强行接到禁止 mutation 的 frozen client，或改变协议/路由后改写预期。它应使用独立的共享 CLI/creation 测试装配，保留原事实和黄金字节；这不授权旧协议 Execute/Reconcile/Stop，也不能放开 NewLifecycle 拒绝规则。

## 实施后职责与删除范围

1. 历史读取夹具返回 frozenRecord 和原 host/snapshot。沿原严格解码与校验读取已保存事实；测试数据写入由测试私有 encoder 生成原 envelope。encoder 保持原版本、封套身份、JSON 字节及摘要，不进入生产代码，不成为旧 Run 新写入入口。
2. structuredFixture、workspaceFixtureOn 从共同的中性 setup/host 数据起步，明确装配合法结构化 profile；不能先装配旧 Lifecycle 再换协议/profile。保留版本代表值及现有测试端口注入；测试 seam 不进入产品。增加/复用真实 NewLifecycle 的接线检查，不能全用手工拼字段替代当前构造器。
3. CLI/creation 和原终端并发各有明确的测试装配入口，复用共享 host、命令组装和真实生产能力；不借 reader 白名单开放 mutation，不增加测试生产开关。
4. 按上表核实后删除 17 个候选 Lifecycle 方法：resourceState、state、stateVersion、envelope，三个 control 转发，三个 local Codex 转发，五个 receipt/owner 转发，以及 verifyStopped、codexReadiness。`state` 的 original 消费先改接同一 reader。实际删除数量需报告；发现其它真实生产消费者时先保留并修订清单，不能为了凑数量删除。

frozenFixtureLifecycle 不再作为通用读取/当前执行构造入口；只有明确例外仍需原终端或 CLI 能力装配时，保留或拆出最小测试帮助函数。不得要求“所有旧 fixture 字节完全不变”，因为本批确需迁移调用对象；应记录允许修改的夹具/调用点清单，保持测试事实、覆盖和有效断言，范围外测试及共享基础设施不随手改写。

## 测试装配的语义和隔离

历史 helper 的单元测试与生产冻结入口测试分别装配。access_mode_test、terminal_receipt_test、fixtureFixedControl 等会在构造后修改已解码 profile、setup.RepositoryURL、probeProcess 或 client，用来验证缺原摘要、权限/receipt/根进程等局部条件；这些修改未必回写 setup.ProfileData。迁移后的单元测试应继续把同一组已解码事实交给原 frozenRecord 方法，不能每次从旧 setup 重建 reader 而丢失修改，也不能新增完整 profile 校验阻断原负例。涉及生产解码/离线核验的测试仍通过 newFrozenReader、导出 StopEvidenceScope/VerifyStopped 等真实入口，独立验证严格校验与 mutation 白名单；手工装配不能替代这些入口覆盖。

当前远端 workspaceFixtureOn 的 profile 没有 PairingHostID，旧 client 回调又捕获旧协议 setup；直接改为 NewLifecycle 会先遇到配置错误，补齐 host 后仍可能在 orcaCLIOutput 中读取真实 UsageHostID。新夹具必须显式形成与 target、profile version、agentHostID/Development/Audited 一致的合法配置，并通过现有测试注入位置隔离 client/agent port；构造和测试过程不得查询真实配对目录、IPC、启动进程或访问真实宿主。需要模拟的 host 身份使用固定合成值，不从本机安装或原真实 session 取值。不为通过单元测试放宽生产 host 校验、添加生产测试开关或临时换回旧协议；真实配对/队列校验仍由其原专属回归承担。

test 传输装配应保留原调用记录、队列/超时/取消及 Golden 的被测部分。若某测试本身验证生产队列/host 绑定，则必须继续调用原生产路径与原 IPC/元数据夹具，不能改成绕过这些逻辑的统一 fake；普通 workspace/历史单元测试的隔离不授予该资格。#123 TestStructuredConstructorFacts 依赖 structuredFixture，故新夹具需在合法构造完成且外部 I/O 为零后再按原方式注入；不能产生递归构造或遗漏 agentAudited 等已验证派生字段。

隔离还包括原 `nativeAccess` 测试 seam：当前构造器默认值为 true，普通单元夹具必须在构造后恢复其原 false 设置，否则 workspaceRepository 会绕过 CLI seam 进入 LocalBootstrap。专门的构造/IPC/队列测试继续验证生产 true 路径；不改生产默认值或校验。`TestPreparationReadRecoveryRetainsBudget` 属于当前准备能力，迁至当前结构化夹具，不误归为历史 reader。

## 明确保留的能力与排除范围

保留 Lifecycle.validate（当前 workspace audit 真实消费）、structured/agentState/agentEnvelope/close 读取投影、Policy/Execute/Observe/Reconcile/Stop、当前创建/队列、inspectionGate 及原 inspectionOnce/inspectionSerial。control_checkpoint.go 同时包含当前审计串行门，不能整文件删除。profileVersion 仍决定原关闭合同；nativeAccess 仍参与现行 CLI/准备队列，不能按历史字段一起删除。

保留导出 StopEvidenceScope/VerifyStopped、ConfirmedTerminalStopReceipt、usage、progress/result/workspace 绑定和离线 inspect 的严格读取。公共 InspectBoundTerminal 仍由当前默认 shell 处理、只读关闭核验和 livetest 使用；协议层 QueryBoundTerminal/AgentState/StateFeed 的其它消费者均不属于本批删除闭包。旧 profile、State/Turn/Control/TerminalOwner/NativeProof、receipt、owner 与签名/Job/进程证明全部保留原格式和含义。

不顺带删除 terminalAssets/TerminalRuntimeMetadata/TerminalConfig.Command、Pi 扩展及独立诊断脚本、L1 Node 测试；这些属于另项审计。remaining execution-stability 的六组场景及共享故障工具不在本批迁移或删除，不追认结构化资格。直接 Codex、公共 terminal、独立 parser 均不变。

## 持久格式及可靠性

以 A 通过点为 B 的字节对照基线，不能把主线与旧工作副本混比。state/v8、user_version=8、CLI/能力/Workflow、Request/Record/Operation/Receipt、opaque Profile/State/Turn/NativeProof 及 #125 RuntimeFence 等既有字段/tag/顺序/codec 不变。旧 encoder 从生产方法转为测试帮助函数不改变历史解码或持久格式；代表旧/新 payload 与封套必须逐字节相同，不迁移、回填或重写记录。允许删除的 envelope 方法是生产编解码声明差异中的明确例外：报告其无生产写入消费者及等价测试 encoder，不要求已授权删除的函数正文仍留在生产；其余实际持久读写/解码函数、格式类型与标签保持。不能用测试 encoder 自己编码再自己解码代替独立 A 原字节对照。

执行、恢复、停止调用路径和原提交边界保持。未知 create/send/close 只对账原操作；缺失、坏证明仍等待/拒绝，不能让读取 fixture 默认成功；缺预算只阻新派发。#125 pr_ready 消息/问答、原 RuntimeFence、停止撤销未发输入、已提交未知输入和同构建重启保持。会话停止不扩大为主机清场，旧协议仍由原构建排空。PC-R01–R06/R08–R10 均保持。

## 最小验收及范围停止条件

| ID | 必须取得的证据 |
| --- | --- |
| ORF-B01 | 按函数与测试消费者固定删除清单；17 个候选薄方法不再由 Lifecycle 承担；历史读取、当前结构化、共享 CLI/creation、原终端例外的装配入口可区分，无新增纯转发平台 |
| ORF-B02 | 历史单元测试的 profile/probe/client 修改仍到达原 reader 方法，原负例不被新增构造校验截走；生产冻结入口另验错身份/坏 codec/重复/未知字段/尾随 JSON、Conflict/Waiting、scope/owner/receipt/退出文件；reader 无 LifecyclePort、mutation、feed/observer/Job 所有权 |
| ORF-B03 | 原测试清单与有效断言保留，夹具修改逐项有消费者依据；当前 structured/workspace 不再从旧 Lifecycle 起步，本机/远端夹具构造及单元调用外部 I/O 为零，派生字段完整；CLI Golden/creation 原回执和原终端并发/RPC 回归保持，当前 validate/inspectionGate 不误删 |
| ORF-B04 | A 基线到 B 格式类型/tag/codec、实际持久读写/解码和新旧代表编码相同；单列无生产消费者的旧 envelope 删除及等价测试 encoder；#123 单次解码与 clone/首错回归、#125 消息/答案与停止窗口保持 |
| ORF-B05 | 完整受影响 adapter 包、相关历史读取/usage/files/host/application 定向及最终 EXE 的公开受控创建恢复、原输入未知与停止通过；记录失败/skip，不替代完整 L1/Controlled 或真实模型资格 |

收益按旧执行对象方法与测试装配依赖的实际减少验收，报告仍保留的两个原终端方法及共享能力例外，不宣称完全消除历史知识或运行时提速。若需要放开旧执行、弱化测试/严格校验、修改格式或增建第二套生产 reader，应停止扩展并修订设计；否则完成上述闭包即结束 B。实际验收存于 docs/acceptance/ 独立文件；本批不授予完整级别、部署或发布资格。

## 设计审计修订

2026-11-06，按 main `6dd4360a` 核对候选方法、旧/新 fixture、CLI/creation、原终端并发与 #123 构造回归，确认三项设计缺口并修订：历史单元测试的构造后修改不能在迁移中丢失；当前远端夹具不能带入真实配对查询或错误派生字段；删除旧生产 envelope 与格式不变的验收须区分源码删除和编码等价。17 个候选删除项及两个原终端例外保持。

修订后的测试装配和格式对照已写入 ORF-B02–B04 与 Plan。此结论来自静态审计，不表示新夹具或行为已验证；实际源码修改、P0 基线捕获和所有 B 验收仍待后续实施。

## 实施状态

2026-11-06 用户追加授权先 A 后 B 分开实施。B 实际删除 17 个方法，保留原终端及共享能力例外；原 80 个 adapter 顶层测试全部保留，新增隔离测试，最终 81 项通过。15,528 字节原编码一致；相关组件/应用定向及同 EXE 四公开场景复验通过。首轮公开 fixture 读取失败根因未确立，原失败保留，不改记通过，详见验收。上节为设计时静态审计，实际运行及初轮装配修正以验收为准；未提交、提 PR 或部署。
