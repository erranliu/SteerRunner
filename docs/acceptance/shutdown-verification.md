# 停服后显式资源核验

2026-10-16，配套 SteerLine 部署更新修复。既有停机接受回执与 last_shutdown 保持历史事实；新增公开 service verify-stopped 只负责当前格式下的停机后取证，具体规则见 [Spec](../spec/steerrunner.spec.md)。

| 验证项 | 自动入口 | 结果 |
| --- | --- | --- |
| 原 managed Job 唯一 owner 与精确 root 必须同时消失；非 managed、Orca、缺身份或仍活动不成功 | TestSROfflineStoppedRequiresManagedOwnerAndRootAbsence | 通过 |
| 缺目录不创建；只读当前库，写操作被 SQLite 拒绝且文件字节保持 | TestSROfflineReaderDoesNotCreateOrWrite | 通过 |
| 活动写者拒绝；有本地 worker 的服务异常退出、无最终回执时，按原资源身份核验；不重启服务、不产生原生输入、不改变数据库或补造回执 | TestSRServiceOwnership/offline_verification | 通过 |
| 既有 stop、停服最终证据与 CLI 规则保持 | TestSRStopProgress、TestSRShutdown、TestSRCLIContract | 通过 |

命令与证据：

- go test ./internal/application ./internal/storage ./internal/host ./internal/cli ./internal/resources -run 'TestSROffline|TestSRCLIContract|TestSRStopProgress|TestSRShutdown' -count=1 -timeout 90s：通过；resources 无匹配测试，仅编译。
- go test ./tests/integration -run '^TestSRServiceOwnership/offline_verification$' -count=1 -timeout 120s：通过，30.853s。该反例归入既有服务归属场景，Controlled catalog 不新增场景。
- go test ./internal/application ./internal/host ./internal/storage ./internal/cli ./internal/resources -count=1 -timeout 120s：五个相关包全部通过，application 66.540s；覆盖共用 managed 恢复证明抽取后的既有规则，没有执行全仓或 live。

均使用本仓隔离夹具，没有使用安装环境、真实模型、Orca 或真实 PR。当前用户安装现场的实际资源状态仍需部署时核验；缺证据不能将本次隔离通过当作现场已停止。
