# 本轮独立只读复核

final_review，2026-10-31。审计者没有编辑生产/测试文件、运行正式测试或操作Orca。

确认两项P2：普通本机无helper的queryInput构造接线遗漏；旧报告把本机实际model/effort证明泛化到六场。第一项只取消既有远端if，第二项收紧文档，不新增查询API、模型框架或旁路。

实施前审查Spec §4.2 / Plan增量及实际构造器红回归，最小方案合理。实施后生产仅改变既有闭包的目标限定，managed observeRecorded、extension与Codex补证条件保持，未发现新增P0–P2。新测试不注入Started/queryInput/queryNative，本机publicCandidate=nil原契约保持。

独立重计旧生产overlay四例全fail/query0；原查询3根15叶各30次=450/0/0；正式25roots239叶各30次=7170/0/0，当前源摘要C21与冻结值一致。450回归与正式矩阵有四叶重叠，不合称不重复覆盖。

模型范围来源：remote_models.go的RemoteTurnModel读取原turn_context；native_input_local_probe_test.go核对实际model/effort。远端native_input_live_probe_test.go仅要求启动Model非空，后续原输入/done/final/stop不含实际模型查询；contracts/run.go明确远端audited不支持。因此本机模型凭据与远端启动偏好必须分开，远端actual未独立核验。旧复核记录中泛称六场模型的概括也以本条纠正，旧封存字节不改写。

最终补充复核通过，无未修P0–P2，审计门禁可放行。独立核对41文件/2,046,209字节/512原记录（本条最终补记前），原件摘要一致；L1原事件722顶层/2735含子例通过、0失败，17明确测试跳过与1无测试包、34包通过，24框架/29扩展/driver编译/零匹配门禁通过。695源与index对应，仅2处既有CRLF→LF。六场agent实际SHA E6F…BE9、driver308…391F，与源身份一致；119处可读base64展开未发现秘密候选。

六场逐一核验原调用和正式数据：本机三场原READY与CLI stream解码逐字一致，原Ed25519退出/认证绑定对应；远端三场均只有一次原handle且ptyKilled=true的close。原文/final/stop不是只信摘要。runtime文字已澄清为核对当前身份，允许同一运行中的runtime承载新独立会话，无需为验证重启。普通无helper本机只有生产构造器+隔离IPC证明，不能扩写为managed真实场景；远端停止仅orca_terminal，物理路径未独立验证；未证明全文原子消费、任意人工写者排他或完整复现历史故障。
