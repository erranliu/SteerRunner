# L3 真机验收结果（2026-10-16）

同一构建完整三轮：6 个场景、12 个 case，每轮 12/12，合计 **36/36 通过**，首次通过率 100%，管理员 case 无排除。运行时间为北京时间 2026-10-16 05:45:36–08:33:35。此前诊断失败不计入本批，也未覆盖其原始结果；过程与失败历史见 [专属验收记录](../../l3-execution-stability.md)。

此目录为原始结果文件的内容副本，Git 可规范化行尾。运行时的 `source_commit` 为当时基线，`source_dirty=true` 如实记录尚未提交的实施；真正受测来源由 288 个文件的 SHA256 和每现场的构建 SHA256 确定。提交前已重新逐项匹配全部冻结来源，受测实施已归档为提交 `9425d2a433d533ae6f3ceee24ec687e60bac6d83`。原始路径、时间、PID、状态与摘要均保留；路径字段用于本机追溯，不要求远程审阅者能访问测试主机。

| 文件 | 内容 |
| --- | --- |
| [summary.json](summary.json) | 完整 36 次执行、开始/结束时间、每 case 的轮次及状态；含 L1 163 项、Controlled 12 场景前置 |
| [final-audit.json](final-audit.json) | 独立核对 36 个 Run、27 个真实合并 PR 及 merge SHA、每现场原生结果和辅助终端关闭数、36 个服务退出、零本批规则/进程残留、构建一致性 |
| [cases/](cases/) | 36 个 fixture 独立目录，各含 result.json、scenario.json（断言及业务结果）、source-identity.json；33 个稳定性现场另有 cleanup.json，原 smoke 的清理记录位于 scenario.json |
| [stage-timings.json](stage-timings.json) | 逐 case 总时长、Go harness 时长与原始 driver 时间点；时间间隔不推断为 CPU 或业务阶段耗时 |
| [source-snapshot.json](source-snapshot.json) | 288 个冻结来源文件的 SHA256 |
| [config-identity.json](config-identity.json) | 运行配置的 SHA256，配置本身保留在测试主机 |
| [source-verification-r1.json](source-verification-r1.json)、[source-verification-r2.json](source-verification-r2.json) | 轮次之间来源及配置未变化的核对；最终轮由 final-audit.json 核对 |
| [l2-summary.json](l2-summary.json) | L2 真实交付/反馈/同一 PR 精确 head 合入回归；Runner 构建与最终 L3 一致性由 final-audit.json 核对 |
| [watchdog-cleanup.json](watchdog-cleanup.json) | 21 个最后注入记录中的原 watchdog 进程已退出 |
| [post-run-shells.json](post-run-shells.json) | 批次期间创建的 PowerShell 进程无残留 |
| [admin-host-cleanup.json](admin-host-cleanup.json) | 受限管理员宿主已退出，待处理请求为零 |
| [main-integration.md](main-integration.md) | 归档受测提交后整合最新 main 的变更说明和单独回归；不混入完整 L3 三轮结果 |

业务结果分别为 27 个 merged、6 个 stopped、3 个 revoked-unverified。最后一种表示不可达故障窗口中按契约保留未知，恢复后原资源仍已停止。199 个稳定性控制辅助终端已按原身份配对关闭，所有现场均由 Runner 确认原生停止，driver 未代替完成停止。

原始 journal、CLI 回执、网络物理证明、SQLite、二进制和所有失败现场继续保存在测试主机；本目录保存完整结果和审计材料，不包含二进制、认证数据或全部原始日志。每个 scenario.json 的证据字段保留其原始 journal 引用。
