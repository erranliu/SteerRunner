# Spec M5：本机 Orca Codex audited 一次切换

2026-11-04。以已合入的 P2 为基础，按已确认的新结果、停止与文件合同切换本机 audited。[验收计划](orca-structured-audited.plan.md) · [helper 退役范围](orca-helper-retirement.spec.md)

## 范围与切换

一次切换全部新受理的本机 `orca-terminal` + Codex `development-audited/2`，包括配置 `result_file`、`progress_file` 或启用 usage 的组合。公开提交、回复、停止和查询入口保持。配置与切换资格在创建原会话前校验，无双轨、无 helper 回退。盘点原 helper 必填项、可执行文件、参数、摘要及实际用途。旧 helper 绑定按切换说明移除，仍有业务用途的配置须先找到等价公开映射或完成显式配置整改。未知或无法等价的配置拒绝受理，属于切换前阻断，不能用来把全量切换变成默认配置子集。

切换前冻结旧服务受理，使用旧构建按旧合同结清原状态目录全部待排空 Run、控制授权和资源，停掉原 writer，保存原目录离线证明。发现 unknown、可恢复授权、身份不符或未结清控制就停止升级。排空通过后保留旧目录及证据，新构建在独立目录受理新 Run。旧 Run 不迁移、不接管、不重释，候选无需读取旧目录。旧目录的排空资格与新 Run 的双 EXE 排空资格分别验收。

仅变更本机 Orca audited 的三项合同。开发仍要求显式 `full_access`。工具、项目策略、反馈和问答、既有结果获取边、progress、逐 command usage、工作区及发布门禁继续按原要求。直接 Codex 和独立 parser 保持原路径。普通 Orca Codex、Pi 和 helper 的切换另见退役范围。Orca 和 SteerLine 本身保持原样。

## 三项新合同

1. **原结构化回合结果。** 原成功完成的会话/回合须绑定冻结的 command、用户项及输入摘要。通过完整公开 history/item 确认同一 epoch、turn、item/revision/sequence 和完整正文，选出恰好一条满足当前阶段既有结果格式和输出识别规则的 assistant 消息，再执行原业务核验。其它回合的消息排除。若原回合分页、移除、截断或归属缺口妨碍证明唯一性，直接阻断。多条合格或来源冲突也阻断。零合格时，只有原阶段允许获取结果且存在唯一可归属的完整原材料，才可进入原有有界 parse/report 获取边，否则按原规则等待或阻断。候选筛选仅做确定性本地判定，不能试跑 parser 或发补报来挑候选。放弃证明原生 `final_answer` 和非异步交付，公开结果来源须标明结构化消息。既有单次 parser/报告规则及权限保留，迁移不新增报告轮、不重审、不重做修复。问答继续使用原请求归属。
2. **会话范围停止。** 复用 P2 的 `orca_agent_session` 证明和持久原 stop 意图。冻结原 host/runtime/worktree/session、attempt 和 generation。只允许向原会话发一次 close，原回执确认或创建确定未派发才可结算。确认时同事务撤销输入及恢复授权，拒绝新 resume 和重复 stop，同请求仍幂等。缺回执或未决创建、控制维持未核实，零补发并阻断排空。会话范围不再提供原受管终端、helper 或 App Server root 退出的独立证明。Runner 服务 writer 仍须退出，SteerLine 冷查仍须通过原门禁。
3. **业务结束快照。** 含文件的新 Run 冻结现有 `steerrunner-result-file-snapshot/v1` 合同。首个持久业务结束点开启唯一有界采集窗口。正常路径在现有发布核验通过并提交 `pr_ready` 时触发，其他业务终态仅在确定关闭原执行且再无业务推进授权时触发。显式 stop 按原 stop 意图首次持久化结果时触发，包括结果为 unknown 的情况。快照取得与停止证明独立，unknown 仍阻断排空。中间回合、等反馈、可恢复的 blocked、仅停服务及查询均不触发。沿用原文件初始化、固定路径和大小、稳定性检查、一次持久结果或失败、重启不刷新。公开结果区分快照与停止后封存，快照仅代表读到的字节，不保证此后无人写入。发布和其它消费者的可信最终文件门禁保持。

## 审计与发布硬门禁

- **实际模型和切回。** 原 Workflow 冻结执行与审计角色及 model/effort。本机 Orca 可显式使用同一模型的不同 effort，同模型同 effort 拒绝受理。直接 Codex 的双模型准入保持。首消息和续轮使用 P2 selection。实施、审计修复、发布及任何实际触发的原有报告回合逐一核验该原 turn 的实际 model/effort，发布须证明已切回执行角色。请求参数、options、自述和 usage 汇总只作对照。经已授权 Orca 公开受限通道读取与原 provider session/turn/CWD 绑定的 `turn_context.model` 和 `effort`。用途独立于 usage.enabled 和 collector permit，只持久化必要身份、字段、摘要及撤权证据。读后确认撤销原临时登记，异常和与 collector 并发时也隔离。缺失、错回合、冲突、选择不符或撤权未核实不能签收本回合。[首轮烟测](../acceptance/orca-structured-audited-smoke.md)
- **原完成 HEAD/tree。** 绑定原 host、worktree instance、原会话及完成 turn。实施完成保存干净的 pre_head/tree。审计修复结束保存包含 pre_head 的干净 post_head/tree。正数修复须有真实 tree 变化。原 inspection gate 复用 `LifecycleAuditWorkspacePort`、固定 Git reader 和 `InspectAudit`，核对 repo URL/common-dir/path、branch/base、祖先和检查期间一致性。reader 已结算后将检查点与原结果按现有事务绑定。缺原完成检查点时不能在恢复后从当前 HEAD、reflog 或模型声明补建。禁止 caller Git fallback。
- **发布。** 原审计修复结果、P0–P2 规则、工作区干净及 HEAD=post_head、授权、远端 PR repo/base/head 与并发复核保持。只有原检查点和原来源同时合格才可进入发布，PR head 必须等于核验过的 post_head。

## 验收

| ID | 必须取得的证据 |
| --- | --- |
| AUD-01 | 一次切换的配置盘点和旧构建原目录排空通过。新 Run 无 helper Agent 路由，非目标接线按影响回归。旧 Run 无迁移或双读。 |
| AUD-02 | 同一原会话逐 turn 证明执行、审计、切回执行的实际 model/effort。错 turn、错选择、缺字段及撤权失败拒绝推进。 |
| AUD-03 | 实际实施及审计修复产生不同 HEAD/tree，原检查点、reader 结算和真实 PR head 可核对。错 host、路径替换、HEAD 竞争、缺原检查点和恢复补造均拒绝。 |
| AUD-04 | 原回合唯一性、原有结果获取边、反馈/问答、progress/usage、两种文件触发、活动停止与缺回执拒绝排空均验证。未知操作零重放。 |
| AUD-05 | 同一固定候选取得本机正式发布、活动停止、快照、离线停止证明及原目录双 EXE 冷查资格。候选的失败场保留独立原目录，不混入正向排空。 |

正式验证和未验证项逐项记录在 [退役验收](../acceptance/orca-helper-retirement.md)。
