# 验收：权限资源与原完成证明的私有知识收口

日期：2026-11-02。依据 [Spec](../spec/native-proof-boundaries.spec.md) / [Plan](../spec/native-proof-boundaries.plan.md)，分支 `codex/native-proof-boundaries`，基线 `95e53cb191611837c59411a046da0c510c152d29`，与 main `761f916` 的原树相同。本报告记录初轮实施及专项最小验证，当时未提交/推送、提 PR、合并或部署。完整应用组未通过，不授予完整 L1/L2/L3 或真实模型资格。后续新增授权及最新验证见[实现再审计](native-proof-reaudit.md)，原始证据不改写。

## 1. 实际变化与消费者

A：新增 Codex.ValidatePolicyResourceForProfile，先执行原权限包严格解码，再解码原 profile，最后复用 contract/mode 关系核对匹配。application.verifyPolicyResources 的已有包分支只调用此校验，删除第二次 sandbox_mode 扫描和业务文件的 profile mode 解释。nil 包资格、工作区/common-dir 路径、30 秒版本探测、原构造和 worker 提交保持。

B：Codex.ReadCompletionCheckpoint 返回原合同 required、原 WorkspaceInspection/诊断和错误。engine.go 静态组装处只有纯转接；workspace_source.go 不再导入 Codex，不识别 owner 标志。适用时原文件缺失/损坏/错来源仍返回 nil 等待；不适用才沿原缓存/ownedInputs/unknown/inspectSource。仓库/分支/base、实施来源和审计祖先关系继续由 application 核对。

没有引入 resolver/注册器、可选端口缺失放行、通用证明模型或新持久状态。ExecutionOwner、ReadExecutionCheckpoint、原签名 writer 及实际停止/owner 消费者保持；独立 owner 的无 launcher 路径保持。Git 路径的运行时临时目录/身份用途、Orca local 工作区 owner、远端不使用调用方 Git 的边界保持。

生产仅改 5 文件、两个原协调函数，新增三个窄函数；净增 15 行，A 净减 2/B 净增 17。收益是删除一套跨层 argv 解释、把原完成证明适用性与读取归回 adapter；不宣称大幅删代码、耗时提速或自动减少所有回归。

## 2. 设计先行与源码/格式保护

生产/测试差异为空时完成引用/合法入口审计、Spec/Plan 和编码基线捕获；审计时版本见 [audited-spec](evidence/native-proof-boundaries-20261102/audited-spec.md)、[audited-plan](evidence/native-proof-boundaries-20261102/audited-plan.md)、[design-audit](evidence/native-proof-boundaries-20261102/design-audit.json)。

- 786 项原跟踪源/配置中 781 项原字节保持；仅白名单 5 个生产文件修改。350 个既有测试文件全部保持，含 auditEngine、原 cache/commit 重试、ownerTestRequest、MockAppServer/StagedBusiness/history 与停止夹具。
- 对全部改变的生产文件逐声明比较：41 个保留声明（9 个类型）相同；只允许 verifyPolicyResources/originalCheckpoint 修改，以及三个指定函数新增；零原声明删除。原 reader/writer/decoder/构造器正文保持，其余执行/恢复/停止/提交文件保持原字节。
- state/v8、user_version=8、CLI v1、owner/profile/policy/checkpoint 格式、字段/标签及动作没有修改。原操作与冻结输入不重写，Intents/Snapshot.Operations 保持。代表 Run/Request/Receipt/旧字段/能力及持久根捕获合计 145263 字节前后一致，SHA256 `a5ec2f5e94487c7b4cea1711a50b23a423bc41aaf0c2d5444e0b9ec5284c7c16`。

证据：[源码核对](evidence/native-proof-boundaries-20261102/source-audit.json)、[原源码 SHA](evidence/native-proof-boundaries-20261102/source-before.json)、[声明比较](evidence/native-proof-boundaries-20261102/declarations.json)、[编码比较](evidence/native-proof-boundaries-20261102/wire-comparison.json)。代表捕获不代替所有编码证明；其余依靠持久声明/原编解码/写入源码未变及现有存储回归。

## 3. 验证结果与失败边界

| 验证 | 实际结果 |
| --- | --- |
| 新增定向 | 5 顶层通过，0 skip；含两模式/交叉模式、错身份/非法版本/参数/字段、错误优先级和输入字节，真实签名文件缺失/晚到/错 turn/篡改、SQLite 重载、缓存不得替代原证明、错仓库/非祖先和实际 advance 权限拒绝 |
| 组件 | execution/...、runstate/contracts/resources/storage/testkit/protocol：448 顶层通过；8 个既有 skip、3 个无测试包单列，20 个有测试包通过 |
| application 定向 | 57 顶层通过，0 fail/skip，覆盖新边界、RA、原操作/手工对账、冻结预算、准备、停止/离线核验；232.329 秒 |
| native / integration | 两个包 `-tags live -c` 编译通过，仅编译不计真机通过 |
| 最终 EXE 公开受控 | ServiceOwnership 55.15 秒、NativeUnknownAndStop 25.62 秒、RestartPreservesAuthorization 139.36 秒；3/3，0 skip，exit 0 |
| Controlled 计时导入 | 原三场景证据导入通过；qualification_eligible=false、complete_controlled_passed=false，无新增执行或完整资格 |

新增测试第一次 2 过/3 失败：新夹具缺 project/generation 等冻结身份；第二次 4 过/1 失败：新权限派发夹具未声明已准备 Workspace，先被原准备门禁拒绝。只补齐新增测试的真实前置并使用原签名 JSON 标签，最终 5/5；生产校验、原断言及共享夹具未改。

额外尝试完整 application：97 个顶层已通过，TestSRResumeWorkflowRetainsOriginalReportSource 在 feedback_test.go:30 因旧冻结输入没有 Budget 失败；随后整包 6 分钟总时限在 TestProjectPoliciesFreezeFixedCommitAndRecovery 耗尽，其余未完成，不能算全组通过。将 5 个改动生产文件以 Git HEAD 原源码的 Go overlay 覆盖、排除两个新测试文件，原基线单独运行该缺预算测试，得到相同失败。预算重建/旧夹具不属于本批，未放宽规则或修改它；另行完成上述受影响定向回归，不抹去完整组失败/超时。

测试名称/skip/失败和原始日志 SHA 见 [test-summary](evidence/native-proof-boundaries-20261102/test-summary.json)，精确命令见 [commands](evidence/native-proof-boundaries-20261102/commands.json)。目录内保留各轮终态和失败输出、公开完整日志；未收录的大型原始组件日志保留于本工作树 `.evidence/native-proof-boundaries/`，摘要不把未完成项当作通过。

## 4. 最终产物与可靠性审计

Runner SHA256：`7c2831a8cf7f3c94dc4d1c2729a8dddee36e0c20f7d1e1f6a0e0e58a7beabfa3`；生产输入 digest：`cc40463a6d7f3053a99db31e5a76b4f6a0e602ca5b1a6a44cb771421058e7389`，Go 1.26.4 Windows/amd64。生产/测试冻结后编译、构建和公开场景采用同一 EXE；见 [构建回执](evidence/native-proof-boundaries-20261102/runner-build.json)、[公开身份](evidence/native-proof-boundaries-20261102/public-source-identity.json)和[公开结果](evidence/native-proof-boundaries-20261102/public-result.json)。

PC-R01–R06/R08/R09 的保存后派发、未知仅原对账、原来源及资源、有界恢复、预算不刷新、停止撤销/原范围与单确认均保持。B 的 required/error 是原判断的只读返回值，不是新授权或持久状态；原包/签名不足仍不推进。没有旧记录迁移、当前 HEAD 补造完成点、资源重建或停止协议变化。NPB-00–05 按上述专项范围完成；未宣称完整应用组或完整平台资格。
