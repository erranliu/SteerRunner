# Plan：Orca 原生合同正式启用与旧消费者收口 B6

日期：2026-11-03。状态：待实施。对应 [Spec](orca-native-rollout.spec.md)。基线 `0f22212`，B5 已签收。

## P0 变更面

冻结本机/明确 environment、Codex/development/2/full_access/native、codec9、orca_terminal；保留显式 managed、audited、Pi/DSH/其他 workflow 与旧状态消费者。列出支持/未取得原生资格表，不把原直接 Codex 纳入本批。

## P1 公开合同和删除

1. resources.Capabilities 中增加 Orca 原生合同选择和保留例外 metadata；stop_evidence 补原生双端 scope，不改变组合 catalog/schema/admission。用已有 metadata 测试形态核对 native 与 managed 的描述、范围及实际模型/审计边界。
2. 删除 lifecycle.PreparationFacts 中已由 agentExecutionSupport 覆盖的不可达 Pi/DSH guard；实际入口 agentAdmission/Preparation/双端原能力及依赖守卫回归。登记仍有生产消费者的 helper/control/history/owner/usage，不扩删。
3. 更新 README、产品承诺和长期设计，写 docs/orca-execution.md 说明当前双端配置、scope、保留例外及原 EXE 排空/fresh 目录。历史 Orca 设计加当前合同指向，原历史验收不改写。

## P2 受控及升级

定向 resources/contracts/application/Orca 两包与依赖守卫；候选正式 Engine 双端停止9模式、FIFO未派发、blocked首次 stop、反馈 idle/active。准确执行 TestSRServiceOwnership 的 different_release、offline_verification、remote_offline_scope、native_candidate_offline_scope、queries_do_not_start。核对旧编码及摘要未变；只对公开输出和已证不可达删除取证。生产和 live-tag 构建通过。

## P3 最终真实资格

固定源码与构建身份后提交实现，启动独立只读代码审计；按原 B5 注册脚本创建四个 fresh SteerlineTest 场次。仍选择原本机 runtime 与 台式机64G；不重用旧会话/PR/状态目录，不重发未知动作，准备与清理逐原身份记录。优先活动停止，再双端真实开发/原会话反馈/PR（不合并），全部正常 Run stop→service stop→真实 offline。实现修复影响资格时重建重验对应最终四场。

## P4 收口与统一 PR

归档含失败/skip 的 UTF-8 原证据及逐文件哈希；公开副本只去除无关注册仓库信息并保留源 SHA 与语义核对。独立审计复核代码、例外、格式、四场证据与升级，不以候选旧构建替代。确认修复独立提交；ONR-01–05 全部满足后更新总路线和验收索引，统一 push 并提一个 SteerRunner PR，不合并、不部署。
