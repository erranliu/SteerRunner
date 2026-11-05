# Plan M5：本机 Orca Codex audited 一次切换

2026-11-04。三项合同已获确认。首轮执行与受限撤权已有烟测证据，正式资格按独立验收记录。 [Spec](orca-structured-audited.spec.md) · [首轮烟测](../acceptance/orca-structured-audited-smoke.md)

## P0 核验与切换前置

1. 从最新主线核对 `development-audited/2` 的本机 Orca 准入、配置和实际消费者。列明 helper 专用绑定的配置转换及 custom executable/args 等实际业务语义的处理。确认 progress、usage、问答、工具、权限、现有 parse/report 及发布门禁的复用点。若有一类在用配置无法等价或现有消费者要求停止后文件，记录阻断并先解决配置或消费者前置。禁止子集上线或静默回退。
2. 用现有隔离入口先核验一次真实 pre_head/tree → post_head/tree 变化。原完成 turn、host/worktree instance 和固定 reader 必须同源，读取结束后确认 reader 已结算。确认原 `InspectAudit` 和发布可用该证据，以及缺历史检查点时恢复不能从当前 HEAD 补建。已授权的 model/effort 来源只沿原用途复用。
3. 在验收现场用旧构建创建并排空旧 Run，演练正式切换步骤。发布切换时对实际旧目录重复旧构建流程，停受理、结清控制和恢复权、停 writer、在原目录用旧构建 `service verify-stopped` 并核查冷读 Run/输入身份、停止、授权和未决控制。存在未核实资源则暂停。留存原目录，证明通过后新建独立状态目录。实际旧目录的发布门禁不能用演练代替。

## 最小实现顺序

1. **接线。** 仅改本机 Orca audited 的配置校验及冻结，复用 P2 的 session 创建、续轮、围栏、原输入归属和唯一 stop 意图。保留既有 Workflow 和 CLI。按真实消费者调整已存在的配置、合同描述及文件提示。原普通 managed、直接 Codex、Pi、parser 和其它已冻结路由按原规则。
2. **结果及检查点。** 用公开原回合完整读取和确定性阶段规则选唯一消息，复用原结果提交、parser/report 与发布门禁。每个业务 turn 验实际 model/effort 和撤权，原 inspection gate 绑定真实 worktree owner 并保存原检查点。未知输入和控制保持零重放。直接复用已有 lease、reader、事务和工作区 port，无需通用证据平台或新消息协议。
3. **停止及文件。** 将 P2 确认停止后撤销恢复权的接线限定到冻结该合同的新 audited。复用 snapshot/v1 的初始化、持久触发与唯一有界采集。分别接入原 `pr_ready` 发布事务和显式 stop 事务，沿现有 worker 在 `pr_ready` 早退前采集，保证首个触发的原截止时间不会被第二触发或重启刷新。公开能力及回执按已冻结的合同显示。复用原 offline verifier 及 SteerLine 消费函数。

## 验收与发布顺序

| 步骤 | 固定入口及验收 |
| --- | --- |
| 定向受控 | 按 [测试指南](../testing.md) 跑受影响包和现有受控用例。验证准入、权限和工具不变，完整历史/错 turn/重复候选拒绝，既有获取边在唯一来源上仍可触发，回合实际模型和撤权负例，错工作区与恢复补造拒绝，旧构建排空负例，快照首个触发、无刷新及缺回执阻断。缺 close 回执以同候选的受控故障覆盖，记录原 stop 只派发一次、`resources_settled=false`、无正向排空。 |
| 同候选正式发布 | 固定源码、Runner EXE 和 Orca 版本。通过正式 CLI 在隔离登记仓库提交带 `result_file` 的本机 audited，以同一会话真实执行、审计并提交一次会改变 tree 的 P0–P2 修复、切回执行模型发布。核对逐 turn 实际 model/effort 和撤权、已结算 reader、干净 pre/post HEAD/tree、真实 PR 的 repo/base/head 与原发布判定。核对 `pr_ready` 一次采集且后续 stop/重启不刷新，记录逐 command usage。使用正常业务入口，直接 Codex 的 audited 场景和旧探针仅作对照。 |
| 同候选活动停止 | 独立原目录另建带文件的本机 audited，在原活动续轮显式停止。核对原 session/turn、唯一 close、确认后无新输入/恢复授权、快照合同与字节、重启保持。至少一场真机覆盖 progress 和授权 usage。按原正式入口验证原有反馈/问答和报告后备，另行记账受控与真机覆盖。 |
| 新 Run 原目录双 EXE | 发布场和活动停止场各在本来写入的目录完成显式停止、正常停服和 writer 退出。调用未改动的 SteerLine `VerifyUpgradeDrained`，原写入候选与另一可读**新合同**且不同摘要的冷读 EXE 核对同一原目录、同一 Run/输入集合、`orca_agent_session` 正向证明、业务停止、无恢复授权及未决控制。复用 P2 的第二构建做法，第二 EXE 仅冷查，不承担真机业务写入。缺回执场应拒绝排空，独立留证。 |
| 发布切换 | AUD-01–05 均按证据签收，实际旧状态目录按 P0 由旧构建排空后，才允许新构建一次受理全量本机 audited。分别记录旧排空和新合同的专项证据。旧部署保留原构建用于原目录排空。 |

验收记录置于 `docs/acceptance/` 的独立文件，保留构建/工具摘要、目标/Run/回合、原目录、旧和新排空回执、真实 PR、快照及失败范围。定向或真机专项资格据实登记，不承接其它构建或完整等级的资格。未决门禁不签收。
