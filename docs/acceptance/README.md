# 验收记录索引

核对日期：2026-10-22。每份报告只证明其固定源码、EXE、配置和场景；阅读报告开头的范围修订及最终记录。归档不改变原通过、失败、取消或未验结论。

| 范围 | 记录与适用边界 |
| --- | --- |
| 双 skill 薄壳与独立 Viewer | [专项验收](skill-shell.md)：原 ID 调用、只读 JSON、刷新/过期、独立安装、本机 / 一个 Orca environment 的真实 PR 与停止范围 |
| 当前公开读取与受理边界 | [完整绑定及原子 observe](reliability-simplification.md) |
| Run token 与独立费用 | [用量、归属去重及授权撤销](run-usage-and-cost.md)；本机三组合、远端 Codex 的主会话现场单列，远端 Pi/DSH 生产绑定仍不支持 |
| 默认审计提示与交付简化 | [默认提示及跨构建冻结](reliable-delivery-simplification.md) |
| 内部执行职责整理 | [执行边界收敛](execution-boundary-refactor.md) |
| 服务换代与原执行接管 | [恢复专项](service-generation-recovery.md)、[近期 PR 补测](pr-test-gaps-20261020.md)、[创建执行器修复复验](pr38-review-20261020.md)；后两项补充原历史矩阵，真实后端及调用方资格仍独立判断 |
| 内容工作流与接管 | [生成/对话](content-workflows.md)、[内容接管](content-takeover.md) |
| 固定 commit 的只读代码调查与多文档 | [GEN-REPO 验证](generate-repository.md)：受控停止/未知/重启与真实本机 Codex 调查分别记录 |
| 结果提取与 parser | [结果获取](result-acquisition.md) |
| R2 证据与 Git 查询 | [R2 专项](run-evidence-r2.md) |
| 运行时单文件汇报 | [PR #55 审计及当前构建验收](runtime-progress-pr55-review.md) / [主线整合验收](runtime-progress-integration.md) / [原始专项与失败历史](runtime-progress.md)；本地、Orca 本地/远端、恢复、多 Run 与实际并行分别登记 |
| 独立配置及测试体系 | [配置](independent-configuration.md)、[测试分层](test-levels.md)、[首批稳定性](l3-execution-stability.md)；旧整轮结果不代替当前全集资格 |
| 本机认证与环境检查 | [沙箱认证](sandbox-authentication.md)、[环境检查](environment-check.md)、[个人多机与 gh-only](personal-runner-gh-only-20261019.md) |
| Orca CLI 等待/读取重试 | [专项实施与独立验收](orca-cli-wait-retry.md)；local/remote 分目标、保留失败与清理证据 |
| 实施结束后的后续原生输入 | [证据、最小探针及正式 EXE 范围](native-followup-input-20261028.md)；旧发布原因与新发现分列，不计入 SteerLine 两轮结果 |
| Orca 当前路径 | [连接统一](orca-local-remote.md)、[远端原生](orca-remote-native.md)、[通用 Agent](orca-generic-agent.md)；旧 helper 结果不证明新远端能力 |
| 停止及资源范围 | [初始化停止](initialization-stop.md)、[Orca 停止取证](orca-shutdown-verification.md)、[停止验证](shutdown-verification.md) |
| 原会话与进度查看 | [原会话查看](native-session-viewer.md)、[执行可见性](execution-observability.md) |
| Git、命名与发布 | [基线刷新](repository-base-refresh.md)、[工作区前缀](worktree-name-prefix.md)、[PR 元数据](pr-merge-metadata.md)、[L2 发布及 Git 锁](l2-production-publication.md) |
| 构建、启动及首次接入 | [运行配置发布](run-configuration-publication.md)、[Windows 启动](windows-broker-install.md)、[首版独立化](steerrunner.md)；首版记录保留当时范围 |
| 历史设计与旧构建复测 | [归档目录与替代入口](../archive/README.md) |

原始证据继续位于 [evidence/](evidence/)。新结果追加到对应专项，登记固定构建和实际范围；不得把旧失败改记为通过，或把受控结果记作真机资格。
