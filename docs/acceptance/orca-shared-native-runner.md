# Orca 双端原生 Runner 候选 B5 验收进展

日期：2026-11-02。状态：进行中，未取得真实 Runner 资格，不开始 B6。

候选完整接线到公开 submit/freeze、profile/state codec 9、同一原生 Lifecycle、正式 Engine 停止与持久重入、在线/离线 orca_terminal scope。旧本机 managed 合同及其他组合仍保留。

受控六包回归：456 个根、1565 个子节点、1789 个叶例，2013 pass、7 skip、1 fail；失败为旧远端无冻结 scope 时工作区来源错误回退到 helper owner。已恢复旧远端目标判断，并补本机候选同类拒绝；定向复验 2 根、6 子节点、6 叶例全通过。原始失败保留，未改记为全套通过。

双端停止矩阵在真实构造器、CLI FIFO、正式冻结/Engine/存储路径覆盖未知关闭、活动资源、替换 incarnation、缺 host、错 runtime、截断、查询失败、回执提交失败及原请求重入。上述六包运行中矩阵全通过。排队取消零派发、后续首次关闭通过，额外加了测试异常退出释放屏障。

受控正式离线 CLI 回归：1 根、30 子节点、26 叶例全通过（旧本机/远端及新双端候选）；测试配置/Manifest 包 41 根、191 子节点、215 叶例全通过。live-tag 编译、场景目录脚本校验通过；编译和受控 fixture 不计真实资格。

新增注册 driver 只通过正式 CLI 提交/反馈/停止，不构造 Run/Snapshot，不代模型发布、不合并测试 PR；记录精确 CLI/转发器/Runner 身份，验证同原 terminal/provider 的 pre-pr_ready 反馈、测试 PR 仓库/base/head、原清单、Run stop/service stop/真实离线复核。四个真实场次尚未执行。OSR-03/04 真实部分及 OSR-05 审计待闭环。

当前原始记录在 .evidence/orca-b5；完成时按已有可复现 UTF-8 文件与 SHA256 格式归档，含初始失败。
