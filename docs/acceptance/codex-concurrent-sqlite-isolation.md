# Codex 并发启动与原会话历史索引隔离

2026-11-18，基线 `84ef7e2d29f522dec04426eddf5c2ea398d43db2`，验收时为本地未提交修复。交付前刷新 `origin/main`，仍为同一提交。按 [测试指南](../testing.md)记录本次定向验证，不声明完整 L1、Controlled 或发布资格。

## 问题与修复

SteerLine 真实双任务验收中，一台 Codex app-server 在首次 SQLite 初始化时退出，另一台成功。两台服务器共用一个仅复制登录的 CODEX_HOME；原日志为 `failed to initialize sqlite state runtime`。无登录、无模型探针在本机 Codex `0.166.0` 复现同一现象：共享 home 的两个进程仅一个就绪，为每台指定 sqlite_home 后两个均就绪。

执行服务器现将 SQLite 目录固定为原资源的 `ServerDir/sqlite`。认证和配置仍使用原 CODEX_HOME；权限、原端点、进程及 generation 不变。历史 stdio reader 同样显式使用原 ServerDir，不能另开空索引。本机恢复取原 Backend.Config.ServerDir，分页与启动共用 attempt 目录计算，远端审计及回合读取取原 RemoteRuntimeDirectory。未加入旧数据迁移或新会话回退。

无认证历史探针确认：两个默认 stdio reader 同样会发生初始化竞态。创建原 Session 和无认证回合、关闭原服务器后，使用原 sqlite 的公开 `thread/read` 能读到相同 Session/Turn；新建或默认 sqlite 虽能发现 Session，却遗漏原 Turn。这证明历史与恢复必须绑定原索引。该探针没有可用认证，也不证明模型输出成功。

## 验证与证据

证据根：`C:/SteerlineDebug/wt-99aa-maintenance-conversation/`。

| 范围 | 结果与位置 |
| --- | --- |
| 共享/隔离 WebSocket 启动探针 | 共享一台失败、隔离两台成功；diagnostics/native-sqlite-probe-01/result.json |
| stdio 并发与原索引恢复探针 | 复现共享初始化失败，原索引保留 Session/Turn；diagnostics/native-history-probe-02/result.json |
| 启动、历史配置及 attempt 隔离定向测试 | 通过；新增测试保留共享认证、原 SQLite 路径和 attempt 绑定 |
| 完整 Codex 适配包 | 通过，diagnostics/runner-codex-02/test.jsonl；子进程专用 TestServiceStopWorkerHelper 根在父进程按设计 skip，不据此声明零 skip 的完整分级资格 |
| SteerLine 正式 go.runner-result | 通过，test-results/L4/runner-result-03/summary.json；固定实际 EXE、原生协议夹具及公开结果/停止结算 |
| SteerLine 真实 local-codex-02 | 已观察两个独立原生会话重叠，旧 SQLite 启动故障消失；整个场景因材料定位/后续认证检查失败，未通过，正常自动清理 |
| SteerLine 真实 local-codex-04 | 最终整链通过，test-results/L4/local-codex-04/summary.json；包含双任务交付、实际内容冲突、原 PR 续作、第二版评审及完整清理 |

实际构建为 artifacts/steerrunner-sqlite-isolated-final.exe，版本 `0.2.0-dev.4`，source_dirty=true，source_digest=`411304498b6fd3d085eb1147578512bd6582022ec8ad3db18024b7683ae3e60d`，SHA256=`baf3d36aa2154182cb94a97aaf1b638c1ea02e96e65448f3162ea7152e7d8af7`。原失败与旧构建证据保留，不以新结果覆盖。

独立只读审核由当前聊天的 maintenance_review 完成，批准启动、历史、本机恢复、分页、远端接线与对应测试。真实 PR 整链补验另按 SteerLine 本次场景报告登记。

## 最终真实链

SteerLine 修正材料实际路径及实施/发布职责后，用上述同一固定 Runner 完成本机真实链，正文含清理 604 秒，summary/scenario 均 passed=true、cleanup_complete=true。主 PR #850、并发 PR #849 分属独立工作区和分支；PR 创建均晚于宿主 publish 操作。

原三次 inspect 采样处于 Session 已建立、TurnID 尚未回填的阶段，不能单凭该采样证明模型回合重叠。补取两个原始 Run 的完整公开 observe 页面，分别验证独立 Session/Turn、turn_started=true、成功来源及唯一 start_turn accepted 与 completed 事件。公开原生回合生命周期交叠约 21.328 秒，证据为 diagnostics/native-overlap-04/verified-overlap.json 及两份原页面；它证明两个首个只读模型调用的公开生命周期重叠，不宣称两个可写实施回合同步。

专属 base 在 alpha.cjs、beta.cjs 形成真实内容冲突后，原主 PR 更新到新 head/base，保留两文件简化、并发注释和原 PR 正文，两棵代码树检查通过。四次 execute 保持最新工作区持有者链并使用独立原生会话。两版独立评审成功产出，但因材料没有 CI 结果而给出 insufficient_evidence，未建议合入。

两份测试 PR 已关闭，唯一自有 base 和两个 head 已删除，所有 Run/服务结算、登记进程和临时认证副本已清理。正式证据位于 test-results/L4/local-codex-04，最终只读复查为 diagnostics/live-final-cleanup.json。独立审核已核对最终通过范围和补充并发证据，无需扩大测试。远端 Orca 实际运行、完整 Runner L1/Controlled 及部署不在本次真实验收范围。
