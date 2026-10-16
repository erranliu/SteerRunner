# 不可达撤销 revision 修复复测证据

结论与范围见 [验收记录](../../l3-revocation-fix-20261016.md)。停止修复在 `46f0946` 的不可达真机用例通过；rebase 至 `a778ba7` 后，实际重跑在主线新增的远端认证能力门槛失败，未进入稳定性窗口。各来源与批次单独保留，不能拼接为完整 L3 通过。

| 目录 / 文件 | 来源与含义 |
| --- | --- |
| [run-01/](run-01/) | `46f0946`，停止撤销全部三 case 首轮 **1/3，failed**；active/boundary 原生启动失败，不可达通过。含 L1 192 项、Controlled 12 场景、来源快照、独立审计及逐 case 材料 |
| [run-01/cases/f0b9c9cd4d80/post-fix-cas-diagnostic.json](run-01/cases/f0b9c9cd4d80/post-fix-cas-diagnostic.json) | 原五次上限内取得 resume_required、旧版本仍 stale_revision，249.655 秒实际断连，之后 82 次 revision 均为 30 |
| [recheck-01/](recheck-01/) | 同一 `46f0946` 构建，两个启动失败各用独立新现场补测，**1/2，failed**；boundary 通过，active 启动路由发现超时再次失败 |
| [rebased-prerequisites-01/](rebased-prerequisites-01/) | `a778ba7`，L1 206 项、脚本 12 项、Controlled 12 场景、driver 编译和零匹配 guard 通过；L3 尚未执行，不是 live 通过报告 |
| [rebased-01/](rebased-01/) | `a778ba7`，新现场不可达重跑 **0/1，failed**；诊断 remote_sandbox_authentication_unsupported，业务输入 0、稳定性故障命中 0。含来源快照、回归/vet 结果、严格阻断与清理审计 |
| [original-source-regression.log](original-source-regression.log) | 在原生产代码上执行新增 SQLite 回归，明确以 stale_revision 失败；原始红灯日志 |
| [task-cleanup.json](task-cleanup.json) | 六个现场、两个 Controlled 目录、程序及网络规则、四个最后记录 watchdog 和受限管理员宿主的最终清理核对 |

逐现场材料保留 native 结果、断言、来源/构建身份、关键 Run 快照、故障及清理诊断。首次 active 收尾的 close 回执丢失，其原 Runner unknown/失败保持原样；单独的 `post-failure-manual-cleanup.json` 记录按原 runtime、终端 incarnation 和 worktree 验证后的 operator 清理，不计为测试通过。便携副本中的本机路径用于溯源；完整 SQLite、journal、CLI 回执及二进制保留在测试主机。
