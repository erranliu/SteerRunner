# P0 实测：去掉远端 helper 后的 Orca 能力

> 历史归档（2026-10-21）：保留原构建、设计与验收事实；当前规则或汇总结论见[当前入口](../../../acceptance/orca-remote-native.md)。归档原因及范围见[归档索引](../../README.md)。

> 后续状态：查询漏项已修复；用户明确参数只承诺按配置传递。P0 按收敛范围通过，见[补验报告](orca-remote-native-p0-query-fix-20261019.md)。以下保留首轮原始结论、失败与未覆盖范围。

日期：2026-10-19。首轮结论：**无 helper 的核心执行链路可行，但 P0 不记为全部通过，P1 删除尚未开始。** 本轮只执行独立探针和本机证据回放；没有修改生产代码、部署/调用远端 SteerRunner helper，也没有操作原故障 Run。

依据 [Spec](../../../spec/orca-remote-native.spec.md)、[Plan P0](../../../spec/orca-remote-native.plan.md#2-p0先核验真实-orca-能力再删除生产接线)。可复核材料在 [证据目录](../../../acceptance/evidence/orca-remote-native-p0-20261019/identity.json)；完整原始材料路径及 SHA256 在 [manifest](../../../acceptance/evidence/orca-remote-native-p0-20261019/raw-evidence-manifest.json)。

## 1. 固定现场与方法

- 源码：`a3dc15dc2f90787ea6e38e78d4dbc8a275470646`；Orca 远端 runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`，实测版本 `1.4.223`。
- 环境：`e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`；测试仓库 `erranliu/SteerlineTest`，固定 base `cebdd5ab552150c49743e40593c836689ecf789b`。
- 仅创建 `p0-native-b81a5322` 工作区及原终端，原生程序 `C:/sr-runtime/2ba0/01/native/codex.exe`，显示版本 `0.159.2`。请求 `gpt-6-luna` / `high` / `full_access` / `approval_policy=never`。完整绑定见 [resource](../../../acceptance/evidence/orca-remote-native-p0-20261019/resource.json)、[启动命令](../../../acceptance/evidence/orca-remote-native-p0-20261019/launch.json)。
- 本机 Go 探针直接调用仓库现有 `QueryState` / `QueryOriginalAgent` / `OpenStateFeed`，没有复制一套 RPC 实现；CLI 每次指定 environment 并核对返回 runtime。探针 EXE 仅在本机运行，未上传远端。
- 启动、输入、工作区和终端操作见 [调用日志](../../../acceptance/evidence/orca-remote-native-p0-20261019/calls.jsonl)。业务工具由远端原生 Codex 正常调用，不注入本机模型/Git/gh 凭据，不安装 broker、reader、actor 或 verifier。不做全机进程普查；“零 helper”指本次探针没有准备或调用它，不声称清除既有安装。

## 2. 能保留哪些能力

| 能力 | 实测结果 | 证据及实际边界 |
| --- | --- | --- |
| Orca 直接启动原生 Codex | 通过 | 原工作区/终端、pane/provider 已绑定；原生 READY，未经过 helper readiness |
| model 请求与原生反馈 | 部分通过 | 原始 agentStatus 观察到 `gpt-6-luna`；effort/实际 OS 权限未取得权威回读，见[偏好记录](../../../acceptance/evidence/orca-remote-native-p0-20261019/observed-preferences.json) |
| 原请求开始与普通完整 final | 通过 | 输入接受及 `turn_started`，原主代理 working/end 和完整正文；不能把 accepted 或屏幕文字单独当完成 |
| 极短回合、同会话反馈、query-only 补证 | 通过 | 两次分别得到 `P0_FIRST_b81a5322` / `P0_SECOND_b81a5322`；第二轮只查询、未采集 working。实际现有 NativeProof 回放拒绝上一轮 done，保留同一 provider |
| 实际 Git/gh 登录及最小发布 | 通过 | 远端原代理 commit/push/create/query/close PR；独立 provider GET 核对仓库/base/head/SHA/文件，见下文 |
| 原观察连接关闭后重开 | 通过，限连接层 | 同一资源和 provider，原 final/working 可重新查询，无重发；未模拟整机断网、服务崩溃或 Runner 换代恢复 |
| 活动停止 | 通过，限原终端 | 先有权威 working，随后原 handle 的 `ptyKilled=true`，原工作区完整 inventory 为 0、未截断、包含执行主机；不能推出全 OS 写者树退出或凭据撤销 |

原回合取证用实际采集状态回放现有生产 `NativeProof` 判据，而不是探针自行放宽完成条件。执行：

```powershell
$env:STEERRUNNER_P0_EVIDENCE = '<本轮完整原始证据目录>'
go test ./internal/execution/adapters/orcatransport -run '^TestP0ReplayWithoutHelper$' -count=1 -timeout 90s -v
```

临时包内测试已移除，源码副本见 [proof-replay_test.go](../../../acceptance/evidence/orca-remote-native-p0-20261019/proof-replay_test.go.txt)。[日志](../../../acceptance/evidence/orca-remote-native-p0-20261019/native-proof-replay-complete.log)为 1 项测试通过，覆盖两轮、旧 done 拒绝、发布和首次清理回合的原文来源；不等于完整 Runner workflow 或全套测试通过。[回放结果](../../../acceptance/evidence/orca-remote-native-p0-20261019/native-proof-replay.json)中 `outcome=unknown` 如实保留；原生 done 不证明工具成功。发布结果另由独立平台事实核验。

## 3. 实际发布与清理

[临时 PR #578](https://github.com/erranliu/SteerlineTest/pull/578) 为 draft，只有一个 nonce 文件、3 行变更；head 为 `74afbe9a201d20f0e8d0f5ae15acce215f080d68`，base 是本轮单独推送的固定测试引用。没有合入业务分支。见[独立核验](../../../acceptance/evidence/orca-remote-native-p0-20261019/independent-publication.json)。

首次清理回合成功关闭 PR，但模型生成的只读 PowerShell 引用校验将制表符处理错误，退出 1，随后按约束停止、没有尝试删除引用。该失败保留在[原生 final](../../../acceptance/evidence/orca-remote-native-p0-20261019/cleanup-native-proof.json)，不能改记为首次全过，也不是 Orca 认证失败。

确认没有删除尝试、且两个引用仍在后，另发独立清理任务，以精确 SHA 的 force-with-lease 原子删除这两条本轮测试引用；独立 GET 对两者均返回 404。PR 最终 closed、unmerged。该过程没有重试结果未知的写操作。

活动停止独立于发布：发送短暂前台等待任务，观察原主代理 working，再执行一次 Orca close；[结果](../../../acceptance/evidence/orca-remote-native-p0-20261019/active-stop-result.json)保留 close 和完整清单，明确 `scope=orca_terminal`、`full_native_tree_verified=false`、`credentials_revoked=false`。之后移除本轮独立工作区，[移除回执](../../../acceptance/evidence/orca-remote-native-p0-20261019/worktree-remove.json)及[精确查询 selector_not_found](../../../acceptance/evidence/orca-remote-native-p0-20261019/worktree-after-remove.json)一致。没有额外远端清理/核验终端。未独立核验工作区移除后的远端本地 Git 分支是否保留，不能声明该本地引用已删。

## 4. 删除前要处理的缺口

| 类别 | 已知事实 | 对删除方案的影响 |
| --- | --- | --- |
| 新发现：原生查询漏项 | 启动时 `QueryState` 已看到原代理，但 `QueryOriginalAgent` 返回 `original worktree absent from current page`，随后活动刷新后查询成功。现有 `worktree.ps` 固定 `limit=100`，现场约 268 个工作区 | 必须修正现有 adapter 的精确选择/分页覆盖，不能把当前页缺席当资源消失；不需要新后端或 helper |
| 未证：有效偏好与权限 | 本轮测试投影只给出 model；启动参数和 TUI 不构成实际 effort/OS 权限审计 | 可以声明请求参数已设置；若继续承诺“权威核验实际生效”，须先找到并验证 Orca 原生接口，否则明确收缩表述，不能记通过 |
| 已有保证收缩：认证 | 远端已有登录真实可用，但没有 Runner 单仓库 capability、TTL、退出撤销或同 app-server 确定性认证预检 | 采用执行主机已有认证，并如实报实际工具失败；完整认证隔离不能称等价保留 |
| 已有保证收缩：停止 | 原终端已结束，不证明脱离 PTY 的后台进程、全 OS 写者树和登录撤销 | 停止只声明 Orca 原终端范围，保留原方案 scope；需要完整树保证应另核验 Orca 原生能力 |
| 已有保证收缩：完整环境 Check | 本轮没运行原 helper 的完整 Check，也没证明替代能力 | 既定方案仍是远端完整 Check unsupported；普通运行不依赖它 |
| 未覆盖：长 final/恢复/沙箱 | 未现场覆盖 7999 UTF-16 截断、真正服务换代/故障恢复、sandbox 模式 | 保留现有截断拒绝，不扩张本轮结论；实现后的 Controlled/P5 继续负责 |

漏项原始失败见 [launch-state](../../../acceptance/evidence/orca-remote-native-p0-20261019/launch-state.jsonl)，源码定位 `internal/execution/adapters/protocol/orca/state_subscription.go` 的 `QueryWorktreeAgents`。这证明当前 Runner 接法有缺口，不证明 Orca 原生平台没有相应能力。现场 capabilities 还声明了其他原生 agent-session 能力；本轮没有逐个探索，不能仅凭名称宣称它们提供所需证据，也不能宣称平台完全做不到。

## 5. 下一步的最小边界

不恢复 helper，也不立即删除生产接线。先将查询漏项修正列为 P1/P2 的必要小改，并在删除前用原生探针证明精确查询不受工作区数量影响；对有效偏好明确“请求已设置”与“实际已核验”的区别。若产品要求保留后者，需先补 Orca 原生证明，不能以此次 full_access 成功替代。

认证采用远端已有登录、完成复用 NativeProof、停止限定原终端，这三项核心方向已有真实证据支持。Runner 生产实现、配置冻结、旧记录隔离、离线 verify-stopped、消费者改动和最终 P5 验收尚未执行。本报告只交付能力盘点，不放行 P1，不替用户决定旧保证是否可以收缩。

[生产文件核验](../../../acceptance/evidence/orca-remote-native-p0-20261019/production-after-summary.json)：263 个已跟踪非测试生产文件前后 SHA256 相同，临时包内测试已移除。当前交付仅文档与本机探针证据；原故障状态目录、安装目录、共享 Orca 和已有登录没有调整。
