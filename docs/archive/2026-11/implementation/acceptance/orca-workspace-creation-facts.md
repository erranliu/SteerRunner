# Orca 工作区创建事实组装收敛验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-05，合并前验证。基线 `a78d87da05120b7e6b99981b396d6cecd4d60d92`，隔离分支 `codex/orca-workspace-creation-facts`；依据 [Spec](../spec/orca-workspace-creation-facts.spec.md) 和 [Plan](../spec/orca-workspace-creation-facts.plan.md)。宿主 Windows amd64，Go `go1.26.4`。

生产修改仅 `workspace.go`、`agent_session.go`（+10/-10）：在现有包增加私有 `workspaceCreationFacts`，由 terminal 和结构化收尾各调用一次。项目、Target、worktree、base、时间按值传入，返回原有 WorkspaceInspection/ExecutionTarget。不新增接口、类型、查询、校验、时钟读取或持久数据；Process/State、checkpoint、错误与 confirmed 判定仍由各调用方负责。

## 等价及公开受控恢复

```powershell
go test ./internal/execution/adapters/orcatransport -count=1 -json
.\scripts\build.ps1 -OutFile .evidence/workspace-facts/writer.exe
.\scripts\build.ps1 -Version 0.2.0-workspace-facts-reader -OutFile .evidence/workspace-facts/reader.exe
.\scripts\test-foundation.ps1 -Pattern '^TestSRResourceCreationRecovery$' -RunnerExe .evidence/workspace-facts/writer.exe -OutDir .evidence/workspace-facts/recovery
```

| 执行 | 顶层通过 | 通过事件，含子测试 | 跳过 | 耗时 |
| --- | --- | --- | --- | --- |
| 原主线 orcatransport | 211 | 1225 | 3 | 162.443 秒 |
| 候选 orcatransport | 212 | 1230 | 3 | 120.118 秒 |
| 固定 EXE 公开恢复专项 | 1 | 8 | 0 | 104.66 秒（案例本身） |

均 exit 0、失败 0。网络/IPC/子进程测试在沙箱外执行。原三项显式 opt-in 跳过为 `TestOrcaNativeStateLiveLifecycle`、`TestShortInputProbeInspect`、`TestOrcaWorkspaceStartupLive`，未提供其专属前置，不计作真机通过。摘要见 [tests.json](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/tests.json)，公开恢复原日志见 [recovery.log](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/recovery.log)。完整包 JSONL 保留于隔离 worktree 的 `.evidence/workspace-facts/{baseline,candidate}.jsonl`，未全文提交。

- 新增固定时间、独立预期值对照：Target 全部原非覆盖字段、Workspace 零值、互异的请求 base/native base/HEAD 来源、一次前缀处理、大小写、非 UTC 时间原样保留及入参不变。该纯函数不承担输入准入；真实入口仍经过原核验。
- 在既有双端 terminal 首次创建和结构化原 owner 输出恢复夹具补 Workspace/Target、Process/State 差异断言。原旧 checkpoint、错身份、缺证据与未知结果不重新发现/创建矩阵通过；没有用会话回合恢复测试代替工作区恢复。
- 原公开恢复七个窗口均通过：graceful、kill、receipt-before-commit、wire-loss、owner-crash、worktree-receipt-without-terminal、missing-creation-plan。原故障窗口、归属、一次 worktree 创建、零额外 terminal 创建/业务发送及未知结果不重建要求保留。使用 main 中现有场景，无修改或替换 Runner 集成测试入口。

## 同一候选双端联验

两份 EXE 源码摘要相同，`source_digest=1e6149d28c31ddbd2a2109e29f71f1e3ea08e6c992ebc7fdf7293b12a4c33b98`，source commit 为上述基线、dirty=true，包含最终测试修改。联验后重算源码摘要一致。构建记录见 [builds.json](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/builds.json)。

| 产物 | SHA256 |
| --- | --- |
| 写入 EXE，0.2.0-dev.4 | `f2070fb59a3b3e3bc68f50e45e1e25781cef863a6cabf14ddf4c715f65ecb390` |
| 独立冷读 EXE，0.2.0-workspace-facts-reader | `f7107daaeee3d3dde834a99fb405c8e5cb9e3cc82e9f777ea13f55858c38b11f` |

两端串行使用同一写入 EXE、新建隔离 state-dir 和 Run，通过正式 service start/submit/inspect/result-file/service stop/verify-stopped 入口完成。复用既有 execute/1 验收脚本，只设置本次独立证据根和唯一名称，不改 Orca 或真实配对，不安装远端 Pi。临时出现的 operation_outcome_unknown 仅继续原只读核实，最终原回合得到确认，没有补发业务输入。

| 目标 | 原 Run | 原 prepare_workspace operation | selector |
| --- | --- | --- | --- |
| 本机 Orca Codex execute/1 | `bfa695102804f894838cd328fc13b87a` | `ad11d0e7f8596e6fc2948eaf9df282c3` | path |
| 配对远端 Orca Codex execute/1 | `8a6fad22cf166cdc0fecce3adf80f255` | `1744dd9e3dd7c3b99a73b24cc4d77c3c` | id |

从各自原 owner 计划、派发、输出及持久 Run 回执复核摘要和归属，响应 runtime/worktree/instance、原 selector、base/HEAD 与结构化会话一致。Workspace/Target 完整字段对照原 Snapshot 与创建响应通过，状态仍只有原 runtime/worktree 准备事实，不提前赋予 session available/worker live。两场取得预期三行可信结果、唯一原已受理回合及原会话关闭证明；恢复授权已撤销、无新输入准入。result-file 为 `steerrunner-result-file/v1`、version 1、closed，内容为预置 `rpc-seed`，摘要与实际字节相符；未声称模型写入了该文件。

公开事实、原绑定摘要、最终状态、封存和排空证明见 [runs.jsonl](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/runs.jsonl)。绑定校验脚本见 [verify-creation.py](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/verify-creation.py)，执行入口见 [acceptance.py](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/acceptance.py)；后者按记录的本机历史脚本位置复用原入口，不是新增生产或通用验收框架。完整原 CLI 回执及 state 仍位于本次 `.evidence/workspace-facts/live/` 的两个独立叶目录，不提交数据库、认证或配对正文。

## 原目录排空与审计

两端原写入 EXE 正常停服并完成原 `verify-stopped`。使用 SteerLine `9b2e5d886b8f56eaf07bc697ca78c578c91eec00` 的生产 `VerifyUpgradeDrained`，通过 test overlay 仅加入专项调用，不修改 SteerLine。原目录由固定写入 EXE 与不同 SHA 的同源码冷读 EXE 共同核对；每端仅原一个 Run/input，停止范围均为 `orca_agent_session`，不扩大为额外进程/OS 树清理证明。

[排空原日志](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/upgrade-gate.log) 两场通过，案例 3.30 秒。调用与原始摘要输出见 [排空调用脚本](../../../../acceptance/evidence/orca-workspace-creation-facts-20261105/upgrade_test.go.txt)；每端数据库和 shutdown.json 的冷读前后 SHA256 相同，实际值保存在 runs.jsonl。停服或命令退出本身没有被用来替代生产排空门禁。

实际在上述 SteerLine checkout 执行：设置 `ORCA_WORKSPACE_RECEIPT_EVIDENCE` 为本次 live 证据根，再运行 `go test -overlay <本次 live/upgrade-overlay.json> ./internal/runnerclient -run '^TestOrcaWorkspaceReceiptOriginalDirectories$' -count=1 -v`。overlay 仅增加该专项测试文件，产品代码和生产门禁不变。

逐行审计确认生产只保留一个纯投影及两处替换：base、HEAD、Branch、CheckedAt 的原来源不变，原 Target 按值保留；两处身份检查、所有权、Process/State、Implementation、错误和 disposition 未迁入函数。原创建/恢复/停止/封存/HEAD-tree/审计/发布门禁与 #113/#117/#118 各通道合同未修改。未发现需要修复的运行逻辑缺陷。生产及组件测试文件已 gofmt，差异及文档检查通过。

排空 Go overlay 调用脚本以 `.go.txt` 保存执行正文，原测试执行文件未改；在证据模块内核对该叶目录无 Go 包。证据目录已有独立 go.mod，原本就与根项目构建扫描隔离，不将文本归档处理记作产品编译缺陷。该处理仅涉及证据文件名和文档链接，不影响已验收源码摘要或双端候选资格。

本记录只证明受影响包、一个公开 Controlled 场景、双端 execute/1 和两个原目录排空。未运行完整 L1/Controlled、完整 L2/L3 或真实 Pi，不用历史证据追认为本候选通过，发布资格沿 [测试指南](../../../../testing.md) 保持。
