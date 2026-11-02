# Orca 原生合同正式启用 B6 验收进展

日期：2026-11-03。状态：实施及受控验证已完成；主线集成、最终同构建四场真实资格和独立审计待闭环，不签收。

依 [Spec](../spec/orca-native-rollout.spec.md) / [Plan](../spec/orca-native-rollout.plan.md)，公开 capabilities 同步双端原生 Codex/development/2/full_access 选择、orca_terminal scope、模型/审计/流边界和保留例外；schema/catalog/admission 不扩组合。删除 PreparationFacts 已被紧邻 agentExecutionSupport 完全覆盖的4行重复分支，原 agent 拒绝与零副作用保持。

显式 Codex helper/audited、Pi/DSH managed、其他 workflow、旧 private profile/state 的实际消费者保留；当前合同 docs/orca-execution.md 分表说明，不能继承原生资格。本批不假称已删除有消费者的 helper/control/history，也不改原直接 Codex 实现。

受控五包、候选正式Engine停止/反馈、正式CLI ownership/offline 全通过；精确数量由 .evidence/orca-b6/test-statistics.json 与原始 jsonl 记录。旧格式/路由摘要和 receipt/stop progress 未改。different_release 验证兼容客户端选择原实例、没有替换 writer；不能误记为不同客户端 EXE 一律拒绝。

最终构建前需集成 origin/main 唯一新增 7f6c538（原 Codex 证明解释收敛），保留其既有改变并复验共享 application 边界；不另扩本任务的直接 Codex 范围。B5 四场及 PR #754/#755 仅前置资格，B6 最终源码/构建重新执行双端开发/反馈/未合并PR与独立活动停止、Run stop/service stop/真实 offline。完成后归档全部失败/skip及原身份、实施后独立审计，全部批次一起提一个 SteerRunner PR。
