# Spec：权限资源与原完成证明的私有知识收口

日期：2026-11-02。状态：设计审计通过，待实施。基线 `95e53cb191611837c59411a046da0c510c152d29`（与已合入 main `761f916` 的树相同）。依据[长期产品契约](steerrunner.spec.md#长期产品承诺)及[个人执行器职责](personal-executor-simplification.spec.md#5-实现职责收口pes-03)，配套 [Plan](native-proof-boundaries.plan.md)。

## 1. 目标与范围

A 删除 application 对 Codex 权限 argv 的第二套解释；B 将原完成 checkpoint 的能力识别及读取留在 adapter，application 只消费原生事实、核对业务来源。目标是减少后续修改需要理解的私有概念和跨层修改，不以净删行数、序列化体积或运行提速验收。

不建设通用 resolver、注册器、新状态机或持久证明模型。不改离线 inspect、停止策略、快照历史、执行 owner 角色、原生协议或支持组合。

## 2. 合法入口、消费者和夹具

| 项目 | 实际入口及消费者 | 本批处理 |
| --- | --- | --- |
| A 权限资源 | worker.advance 在普通新派发前调用 verifyPolicyResources；已有包先解码/匹配，缺包仅在原资格允许时构造并先提交；unknown/停止不经过此新输入门禁 | 保留宿主顺序和提交；私有包/profile/access mode 匹配归 Codex |
| A 编码 | BuildPolicyResourceEnvelope、DecodePolicyResourceEnvelope、NativePermissions、validateNativeLaunchArgs；full_access 与 read_only 均有消费者 | 构造、严格解码、最低版本及原参数验证保持；去掉 application 的 sandbox_mode 扫描 |
| A Git 路径 | attachWriteRoots → LinkedGitPaths/PendingTurn → prepareWorkerRuntime 的临时目录及 worktree/common-dir 身份 | 全部保留，不按旧写权限数据删除 |
| B 原完成点 | finishWithGate 在需要 Workspace 的可信原 final 上调用 originalCheckpoint；锁外采集后原 revision/授权/来源复核、原事务接收 | 只替换原生识别/读取调用，不移动业务检查或提交 |
| B Codex 原证明 | ExecutionOwner(snapshot).WorkspaceCheckpoints → ReadExecutionCheckpoint；签名、原 owner/command/session/turn/workspace/绑定严格验证 | 保留原分类及读取正文；新增窄查询，明确合同是否适用及错误 |
| B 其他路径 | 无此原合同则使用原 w.checkpoints；仅本服务 ownedInputs 且无 unknown 才首次 inspectSource；Orca local 使用原工作区 owner，remote 不以调用方 Git 代替 | 保留所有原后备、缓存及诊断；不扩大可恢复范围 |
| 共享夹具 | auditEngine/auditPort、reportPolicyPort、原 checkpoint/cache/原子提交重试、MockAppServer/StagedBusiness/historyFixtureBinding、ownerTestRequest、原停止端口夹具 | 不删除、不改写原断言；新回归以独立文件补充并复用真实现有入口 |

生产中的无 LaunchExecutionOwner 分支仍由 ServeExecutionOwner 使用，不是死路径。ExecutionOwner、ReadExecutionCheckpoint 还分别被停止证明/原生测试消费，不因业务文件解除导入而删除。

## 3. 职责与实现边界

### A：冻结权限包与 profile 匹配

Codex 新增一个纯校验函数：接收原权限包/资源身份和原 profile 数据/身份，依原次序先严格解码权限包，再解码 profile，最后校验匹配。复用 decoder 已核实的 contract 与 access mode 对应关系；不得在 application 或新 helper 再复制 argv 扫描，不改变原错误优先级或 mismatch 诊断。

application 保留 nil/已有包分支、已有资源不可补造、attachWriteRoots、30 秒版本探测、原 envelope 构造和赋值、原 worker 提交。仍允许具体构造函数作为现有静态接线；本批不以零 Codex 导入为目标。decoder 不放宽，包/profile 身份、版本、摘要、未知字段和模式矛盾仍拒绝，full_access 不得覆盖 read_only。

### B：原完成 checkpoint

Codex 新增窄只读函数，返回 WorkspaceInspection、原诊断、原合同是否要求 checkpoint、读取错误。适用判定精确保留原 ExecutionOwner 成功解码且 WorkspaceCheckpoints=true 的条件；不新增历史绑定分类或修复原损坏记录。适用时缺失/无效/签名或身份冲突必须返回“要求原证明”及错误，不能解释为不适用。读取不创建资源、不查询当前 Git、不补写 checkpoint。

application 通过现有 engine.go 静态组装区的一处纯转接消费该函数，不把端口可选缺失当作后备授权，不新增 mandatory LifecyclePort 方法或修改全部共享夹具。转接只传 Snapshot 和原 command/session/turn，不传 Record/Engine/Store，也不持有业务条件。

originalCheckpoint 不再导入 Codex 或识别 owner 标志：任何读取错误仍返回 nil 等待；合同适用且读取成功时，沿原代码核对 Repository/Worktree/Branch/Base、审计实施源及 merge-base 祖先关系。合同不适用时才走原缓存/ownedInputs/unknown/inspectSource。不得用缓存或当前 HEAD 替代暂不可读的合同证明。

## 4. 删除和保留清单

删除：application/policy.go 的私有 profile 解码和 sandbox_mode 参数解释；workspace_source.go 的 Codex owner 能力判断、直接 ReadExecutionCheckpoint 调用及 Codex 导入。

保留：已有持久类型/字段/codec、原 decoder 与构造器、签名 reader/writer、Job/reader、Execute/Reconcile/观察/问答、输入预算、双操作表示、停止范围/预算/恢复及共享测试夹具。A/B 只增加窄纯函数与静态转接，不改变准备顺序或 actor 状态组合。

## 5. 持久格式和长期承诺

state/v8、SQLite user_version=8、CLI v1、workflow、owner/profile/policy/checkpoint 协议、字段、JSON 标签/字节和摘要全部不变，不清空历史字段、不重写旧记录。本批与“仅新写入内容变化”不同：相同原输入/事实的编码结果也须保持。无迁移或跨构建接管资格增量。

PC-R01 原事务提交/确认，PC-R02 外发前固定身份和输入、未知仅原操作对账，PC-R03 原结果/完成来源，PC-R04–05 原资源恢复及有界预算，PC-R06 停止撤销/原证据，PC-R08 可选观测不阻断，PC-R09 精确资源归属保持。合同不足仍等待/拒绝，不增加自动补证明、重建或重发。

## 6. 最小验收及停止条件

| ID | 最小结果 |
| --- | --- |
| NPB-00 | 生产/测试编辑前复核实际消费者与夹具，审计 Spec/Plan，捕获源码、既有测试和代表编码基线 |
| NPB-01 | A 的合法 full_access/read_only 通过；合法但交叉模式、错资源/profile 身份、低版本/非法 argv/未知字段拒绝；原错误优先级和输入字节保持；应用原资格错误仍阻止新派发 |
| NPB-02 | B 适用/不适用分类与旧判断一致；真实签名文件可读，缺失/晚到/篡改/错身份时不落入缓存或 Git 后备；审计非后代及错仓库仍不合格；无原合同的缓存与恢复缺口保持 |
| NPB-03 | 原测试及共享夹具字节保持；持久类型、编解码/写入函数、原签名读取/写入、操作/恢复/停止/提交正文保持；代表编码/能力摘要前后相同；业务文件不再识别私有 owner 标志或 sandbox 参数 |
| NPB-04 | 受影响组件/application/storage 回归；live 编译及正式 Runner 构建；同最终 EXE 的 NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership 三项公开受控场景通过，失败/skip 如实登记 |
| NPB-05 | 验收、长期文档及索引记录实际收益/限制、命令/数量/源及产物身份；不授予完整 L1/L2/L3 或真实模型资格 |

若为保持旧行为需新增持久字段、弱化校验、修改停止规则或删除共享夹具，应暂停该项并修订设计。满足上述范围即结束，不继续扩展原生策略平台。本次授权实施和验证，不包含提交、推送、PR、合并或部署。

## 7. 实施前设计审计

2026-11-02，生产/测试差异为空时通过 NPB-00，已保存跟踪源/测试 SHA、实际引用、代表持久编码及设计文档摘要。A 的 strict decoder 已证明 contract 与 argv mode 对应，故 profile 匹配可复用其结论，不再扫描参数；仍按原次序报错。B 选用静态查询而非可选端口，避免缺方法时改变原 snapshot 的合同分类；保留原 ExecutionOwner 对不可解码绑定的既有处理，不夹带修复/新门禁。Git/common-dir、原完整签名 reader/writer、缓存/ownedInputs、原子提交重试及共享测试均仍有真实消费者。设计无未决阻塞，之后才编辑生产/测试。
