# Rebase L3 真机复测证据

详细结论、完整一轮 12 个 case、两项失败和独立补测见 [验收记录](../../l3-rebase-20261016.md)。最新受测提交 `95d24e5` 基于 `e04d07f`；完整轮 **10/12，failed**，补测 **1/2，failed**。补测不合并进完整轮首次通过率。

| 目录 / 文件 | 内容 |
| --- | --- |
| [run-01/](run-01/) | `946875b` 完整一轮，10/12；summary、final-audit、来源/配置摘要、阶段耗时及每现场结果 |
| [recheck-01/](recheck-01/) | 同一 `946875b` 构建的两个失败 case 独立补测，2/2；不改写 run-01 |
| [preflight-02/](preflight-02/) | `e04d07f` 的入口提前超时，L3 执行 0；独立 Controlled 12/12 的稍后结果保留 |
| [run-03/](run-03/) | `95d24e5` 完整一轮，10/12；L1 191 项、脚本检查 12 项、Controlled 12 场景前置通过 |
| [recheck-02/](recheck-02/) | 同一 `95d24e5` 构建独立补测：正常重启通过，不可达反馈版本冲突复现失败 |
| [task-cleanup.json](task-cleanup.json) | 全部 28 个现场及 Controlled 宿主的程序/规则核对，15 个最后注入记录中的 watchdog 和管理员宿主退出 |

每个 live case 在 `cases/<fixture>/` 下保存 result.json、scenario.json、source-identity.json；稳定性现场另有 cleanup.json。失败现场保存 failure.log、failure-run.json 和关键 post-failure 诊断。独立审计逐一核对原生结果、原构建、原生停止、服务退出和辅助终端配对；补测也核对 Runner/driver 与对应完整轮一致。

这些是原始报告的内容副本，Git 可以规范化行尾。原 source_commit、source_dirty、时间、PID、SHA256 和本机路径均保留；本机路径用于追溯测试主机原现场，远程审阅可直接读取本目录副本。run-01 的额外来源快照在运行过程中采集，逐现场另有从准备时就记录的干净提交和二进制摘要；run-03 的 321 文件快照在批次开始前采集。两个完整轮都在生成文档前核对来源未变化。

stage-timings.json 记录总耗时、Go harness 时长和原始 driver 时间点间隔，包含真实准备与核验开销，不推断为 CPU 或业务阶段耗时。原 smoke 没有故障 journal，不虚构细分阶段。
