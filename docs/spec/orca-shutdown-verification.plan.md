# Orca 停止取证与部署重验

普通开发，2026-10-19。对应 [停服契约](steerrunner.spec.md)和调用方的 worker-lifecycle W18/E04–E05/V06–V07。实现 worktree：`codex/orca-stop-proof`；不修改安装环境、不兼容或迁移旧格式。

## 实施

1. 在原生协议层提供无副作用的原终端停止核验：仅查询原 runtime 的 terminal list，验证完整计数、未截断、执行主机 local 已纳入范围和原 worktree。原终端仍活动、同 handle 换 incarnation、原主机缺席、查询失败均拒绝；原终端已退出或完整范围内缺席才证明 PTY 已结束。其它配对主机的省略不替代或否定当前原 runtime 的 local 事实。
2. 适配器复用冻结 profile/resource 身份核验，覆盖业务终端、各轮辅助 reader 和 caller inspection journal。未知创建而无原身份继续拒绝。通用原生代理额外要求既有 owner/exit/authentication 证明；不以 PTY 停止代替 Job/broker 结算。不创建观察器、不关闭资源、不写 journal。
3. 普通服务退出仍遵循 ContinueExecutionOnServiceStop；完成既有观察清理后，只在原资源可核实停止时保存 stopped。离线 service verify-stopped 在既有 writer lease 和只读库内使用相同原资源读取规则；成功证明另行返回，不补写数据库/历史失败回执。
4. 调用方保留逐 Run 失败诊断，方便部署定位；不放宽聚合资源门槛。

## 测试计划

专属验收：[Orca 停服核验](../acceptance/orca-shutdown-verification.md)。不跑全仓或开发/PR 全流程：本次仅改变停服取证，业务模型、发布和合并不在范围内。

| 验收项 | 级别 / 场景 / 命令 | 环境与时点 | 通过条件与证据 |
| --- | --- | --- | --- |
| OSV-A01 | L1 `TestTerminalStoppedRequiresCompleteOriginalHost` | 独立响应夹具，实现后 `go test ./internal/execution/adapters/protocol/orca -run TestTerminalStopped -count=1` | 缺身份/离线/错 runtime/incarnation/截断/缺 host/计数错误均失败，查询零写动作；结果登记专属验收 |
| OSV-A02 | L1 `TestTerminalOfflineStopIncludesOriginalReaders` | 独立目录，实现后 `go test ./internal/execution/adapters/orcatransport -run 'TestTerminalOffline|TestTerminalStop' -count=1` | reader 活动/未知创建/错 journal/缺 helper exit 拒绝；原身份已消失成功，snapshot/journal 字节不变；结果登记专属验收 |
| OSV-A03 | L1/Controlled `TestSROffline*`、`TestSRShutdown*`、`TestSRServiceOwnership/offline_verification` | 各仓隔离夹具，合入前定向 go test application/host/storage/cli 和 integration | 既有 managed 原身份、写锁、只读、停服与未知停止规则保持；结果登记专属验收 |
| OSV-A04 | 显式原生无模型 `TestOrcaStoppedNativeProbe` | 先由 SteerLine scripts/new-debug-release.ps1 创建 C:/SteerlineDebug 新包，注册该包 SteerlineTest；通过专用环境变量固定新证据目录、CLI、包 Runner、repo/base，构建后执行 | 真实 Orca 原终端活动拒绝、关闭后只读核验通过；真实 Runner EXE 返回原 Run 证明，DB/原失败回执/journal 不变、服务未启动；证据写本包 evidence 并链接专属验收 |
| OSV-A05 | SteerLine runnerclient 的重验诊断定向测试和 scripts/test-deployment-stop.ps1 | 当前 SteerLine worktree 隔离夹具，实现后执行 | 逐 Run 诊断不丢失且失败仍阻断；结果登记专属验收 |

人工项：无本次必需人工门槛。用户原安装资源的实际停止/部署不在本次现场操作范围；本次验证不能替代它的原资源证明。
