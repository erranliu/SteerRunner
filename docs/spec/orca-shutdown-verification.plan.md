# Orca 停止取证与部署重验

普通开发，2026-10-19。对应 [停服契约](steerrunner.spec.md)和调用方的 worker-lifecycle W18/E04–E05/V06–V07。实现 worktree：`codex/orca-stop-proof`；不修改安装环境、不兼容或迁移旧格式。

## 实施

1. 在原生协议层提供无副作用的原终端停止核验：仅查询原 runtime 的 terminal list，验证完整计数、未截断、执行主机 local 已纳入范围和原 worktree。原终端仍活动、同 handle 换 incarnation、原主机缺席、查询失败均拒绝；原终端已退出或完整范围内缺席才证明 PTY 已结束。其它配对主机的省略不替代或否定当前原 runtime 的 local 事实。
2. 适配器复用冻结 profile/resource 身份核验。PTY 停止只是必要条件；通用原生 helper 复用既有 owner/exit/authentication 证明，并拒绝混入 Codex/readers 证据。Codex 当前没有与原 owner/tree/authentication 绑定的离线退出证明，包括已记录 root 已消失、缺 root、未收到 readiness 或远端情形都保持未核实；不以 PTY 或调用方本机 PID 替代原执行主机证明。历史已持久化 stopped 仍沿既有读取边界。本次不新增远端协议、回执格式、迁移、恢复或清理动作。
3. 普通服务退出仍遵循 ContinueExecutionOnServiceStop；完成既有观察清理后，只在原资源可核实停止时保存 stopped。离线 service verify-stopped 在既有 writer lease 和只读库内使用相同原资源读取规则；成功证明另行返回，不补写数据库/历史失败回执。
4. 调用方保留逐 Run 失败诊断，方便部署定位；不放宽聚合资源门槛。
5. 停止重验与 readonly close 共享一个原 runtime 清单读取/校验函数，统一完整计数、host scope、唯一终端身份；不同动作保留各自的状态判断。工作区准入的部分清单读取语义不合并为新通用框架。

## 测试计划

专属验收：[Orca 停服核验](../acceptance/orca-shutdown-verification.md)。不跑全仓或开发/PR 全流程：本次仅改变停服取证，业务模型、发布和合并不在范围内。

| 验收项 | 级别 / 场景 / 命令 | 环境与时点 | 通过条件与证据 |
| --- | --- | --- | --- |
| OSV-A01 | L1 `TestTerminalStoppedRequiresCompleteOriginalHost` | 独立响应夹具，实现后 `go test ./internal/execution/adapters/protocol/orca -run TestTerminalStopped -count=1` | 缺身份/离线/错 runtime/incarnation/截断/缺 host/计数错误均失败，查询零写动作；结果登记专属验收 |
| OSV-A02 | L1 `TestManagedStopWaitsForOriginalExitAndAuthenticationCleanup` | 独立原生子进程/Job 夹具，改后定向 go test orcatransport | 原 helper 退出证明通过，认证关闭失败拒绝；复用既有夹具，不冒充真实 agent/provider 运行 |
| OSV-A03 | L1/Controlled `TestSROffline*`、`TestSRShutdown*`、`TestSRServiceOwnership/offline_verification` | 各仓隔离夹具，合入前定向 go test application/host/storage/cli 和 integration | 既有 managed 原身份、写锁、只读、停服与未知停止规则保持；结果登记专属验收 |
| OSV-A04 | 显式原生无模型 `TestOrcaStoppedNativeProbe` | 先由 SteerLine scripts/new-debug-release.ps1 创建 C:/SteerlineDebug 新包，注册该包 SteerlineTest；专用环境变量固定新证据目录、CLI、包 Runner、repo/base，构建后分别本机/远端执行 | 真实 PTY 活动拒绝；关闭并核实 PTY 消失后，记录的 live root/缺 owner-tree-authentication 仍拒绝；EXE/普通停服均不误报结算，DB/原失败回执不变、服务未启动、原资源清理；证据写本包 evidence 并登记验收 |
| OSV-A05 | SteerLine runnerclient 的重验诊断定向测试和 scripts/test-deployment-stop.ps1 | 当前 SteerLine worktree 隔离夹具，实现后执行 | 逐 Run 诊断不丢失且失败仍阻断；结果登记专属验收 |
| OSV-A06 | L1 `TestCodexStoppedRequiresNativeExitProofBeyondPTY` | 本机真实存活 OS root + 隔离空清单；改后 go test orcatransport -run TestCodexStopped | 原 P1 拒绝；缺 root/缺 readiness/远端同样拒绝，快照不变；结果登记验收 |
| OSV-A07 | L1 `TestTerminalStoppedRequiresCompleteOriginalHost` + `TestManagedRemoteReceiptRecoversMissedStartWithoutResending/restart-absent-close` | 隔离协议夹具；改后 go test protocol/orca、orcatransport | 共用原 host 完整性校验；原 reader 缺席可结算，原 host 省略/重复/换身份不能结算，零重复发送；结果登记验收 |

人工项：无本次必需人工门槛。用户原安装资源的实际停止/部署不在本次现场操作范围；本次验证不能替代它的原资源证明。
