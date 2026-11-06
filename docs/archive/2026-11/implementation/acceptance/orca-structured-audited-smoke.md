# 本机 Orca Codex audited 首轮烟测

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-04。固定本机 Orca 1.4.246 与隔离 Runner 候选。候选源码摘要 `1f08fded3553855d39b5b82fd3f13fe55a3d89f1dd6d27c64027541d958a7402`，EXE SHA256 `06e556aaad68693164171e24926e3a5ce8c7d433a6f577042231209296012b4c`。原始材料留在 `.evidence/m5-structured-candidate/`。候选之后补了一处失去 Run 授权时禁止发起模型取证的围栏，真机资格按原候选记账。

- Run `3890f0bdc87457e0af5594725b8ab147` 由正式 CLI 受理。原回合 `01a1a649-d63e-7500-90f6-2b7fda379002` 在 `orca_agent_session_v1` 完成，唯一业务结果入账并进入 `waiting_input`。使用专用测试仓库和本机 Orca 原工作区。
- 持久结果记录实际执行模型 `gpt-6.1-sol / medium`，含原 provider session、turn 和来源摘要。临时文件登记及其撤销记录均为 `revoked`。原工作区 Git 检查点为干净的 HEAD `9d81d9c2652177faf6354d4e9f9f3aec79535d11` 与 tree `919c889dab5c805857e263dd212fae354406ef94`，此场未修改工作区内容。
- 显式停止按 `orca_agent_session` 确认。`steerrunner-result-file-snapshot/v1` 采集一次 4 字节 seed，结果版本为 1。正常停服后，原目录 `service verify-stopped` 回报该 Run `resources_settled=true`。

第一场暴露了确认输入后 `PendingTurn` 被消费，后续只读观察无法选择原阶段结果。改为恢复原已确认操作中的冻结输入。第二场暴露了本机文件登记的撤销使用了全部主机完整性判断。本机精确目录清单已经完整，但 Orca 同时报告配对主机被省略，导致原撤销被拒绝。随后确认该场遗留的唯一登记已按原 group、folder 和本机清单撤销。修正后的本机分支与远端用量清单判定分开，第三场通过。

双角色及切回、真实修复 HEAD/tree、正式发布、活动续轮停止和双 EXE 排空仍待同一最终候选核验。本记录不授予 M5 发布资格。[M5 Plan](../spec/orca-structured-audited.plan.md)
