# Orca 原生初始化期间停止与资源结算

2026-10-19，普通开发，#30 rebase 到 Runner `0b55928`。统一规则见 [Worker 生命周期](https://github.com/erranliu/SteerLine/blob/main/docs/product-design/worker-lifecycle.md) W03/W05–W07/W18、E04/E05、V01/V06/V07；原生边界按 [远端仅通过 Orca 执行 Spec](orca-remote-native.spec.md)。验收归 [初始化停止](../acceptance/initialization-stop.md)。历史 helper 方案与旧构建证据保留在验收，不能作为当前原生方案通过证明。

## 实现范围

1. 沿用主干 profile v4 和既有原终端停止/完整清单核验，只输出 `stop_evidence_scope=orca_terminal`。远端零 helper、固定控制终端、签名文件、认证撤销及 native reader。确有 local 消费者的主干 helper 代码不修改。
2. 初始化未完成且无业务 turn 时，允许原终端 branch 元数据尚空；仍严格核对原 runtime/worktree/path/terminal/incarnation。明确错误分支、工作区或路径继续拒绝；已就绪/已有业务的空 branch 继续拒绝。不将停止资格当输入资格，不新增自动恢复/重发。
3. 保留 `orca-remote.initialization-stop/before-readiness` 与 `restart-before-readiness` 两个 L3 case，改为实际直接启动原生 Codex。测试转发器仅隐藏 caller `terminal show.agentIdentity`，保留真实原响应及其 audit call ID；快照必须是启动已接受但未确认、零业务 turn、零 helper 准备/控制状态。
4. 第二个 case 在上述窗口正常停服换代，核对原绑定、零自动启动/输入后显式停止。两者均要求原完整 Orca 终端清单、离线 scope、零后继输入，不声称全 OS 树消失或登录撤销。审计必须证明一次精确原生启动发送、一个原工作区、零额外终端和文件部署。
5. 保留共享 Orca CLI 冷缓存原子发布修复及其最小并发回归，继续使用既有 registry、driver、协调器、租约和结果校验器。旧 helper 修改和局部测试从当前差异移除；不恢复主干退役的远端 Check case。

## 测试计划

本轮为原生主干后的增量验证，不重复无改动的结果获取/发布整套回归，不沿用旧 helper 真机通过声明。

| 验收项 | 层级/组与命令 | 环境/时点 | 通过条件/证据 |
| --- | --- | --- | --- |
| IS-A01 | L1：`go test -p 1 ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca ./internal/testkit/livetest -count=1 -timeout 180s -json` | 隔离组件夹具，最终代码后 | 原生初始化/换代可停止；已开始空分支、错工作区/分支/path/incarnation 拒绝；local 回归、原生 fault 保真、无 helper，日志登记本验收 |
| IS-A02/03 | L3 两注册 case，既有 `Invoke-SRCaseBatch`，MaxParallel=2 | 先经 SteerLine `scripts/new-debug-release.ps1` 生成新的 C:/SteerlineDebug 包；配置固定 workflow/2 与无 helper 的原生参数，构建后 | 原 native 确已存在而 caller 未确认，实际故障命中；零业务输入/替换/部署、显式停止、完整清单与离线 scope 通过；逐 case 原始证据与本验收 |
| IS-A04 | PowerShell `scripts/test-framework.ps1` 与 live driver 编译；Controlled `scripts/test-foundation.ps1 -Pattern '^(TestSRServiceOwnership|TestSRNativeUnknownAndStop|TestSRRestartRequiresResume)$'` | 各自独立夹具，live 前 | 注册为 L3 21 场景/29 case；保留退役范围；精确原 EXE、原生停止/离线 scope、实例与恢复围栏通过；来源与日志归本验收 |

协调器增量执行脚本固定源码、Runner/build.json、配置/registry 摘要；先核对本轮组件/框架/live 编译与三个 Controlled 项通过，Controlled 与 case 使用同一 EXE。脚本/原始报告保留；不计完整 L1/Controlled/L3，不增加生产通用跳过开关。源/构建/配置变化后重新选择对应验证，新执行方案须新建独立调试包。不运行安装/保留运行环境，不改写历史 Run。人工项无，不运行完整 L2/L3/L4。
