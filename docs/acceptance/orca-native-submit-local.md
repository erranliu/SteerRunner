# Orca 本机输入补验记录

2026-10-30。关联 [Spec](../spec/orca-native-submit.spec.md)、[Plan](../spec/orca-native-submit.plan.md)、[此前实施记录](orca-native-submit-implementation.md)。本次只补测试适配，生产代码未增加变更。本机三个成功原生会话尚未完成，当前资格仍为 **0/3**。

本页保留授权前的原始批次记录。用户随后已允许两处最小读取修正，实施与新现场结果见[原始流补验](orca-native-submit-local-stream.md)：control READY 已恢复完整读取，新现场仍在既有认证 socket 准备阶段失败，本机资格未提升。

## 测试适配与构建

在 `.evidence/orca-local-input-20261030/bin/steerrunner-agent.exe` 隔离构建现有 `cmd/steerrunner-agent`，SHA256 为 `e80553915e620f4c8bee679a4f46d57c175e6074a29b79620ece83315b453c7e`。没有安装、部署或修改用户全局配置。helper 位于受测 Git 工作区写入范围之外。

本机目标复用 `scripts/test-native-input.ps1 -Mode long-input -Target <target.json> -OutDir <新目录>`。适配使用原 profile v3、真实 GitHub repository URL、原 control/launch/ready、模型 wire/history 和正式 stop/authentication。仅记录实际 CLI、冻结输入、intent、回执和 observer 事实，不替换回调、不预置 readiness、不使用远端完成证据。

本机驱动用 `operation.New/Invoke/Settle`、`reliable.Next/ApplySessionReceipt`。未知资源准备只对原 intent 做现有只读 Reconcile，不重新 Execute，不重新创建；业务提交未知直接失败，无正文重投或补 Enter。业务 terminal 的无 ID 正文、bare Enter、错误 request 及正文读取都会计入异常，设置/control 调用单独计数。失败同样保存实际计数。紧急清理核验原身份，且永远不能冒充正式停止通过。

## 确定性回归与复审

正式 controlled 入口新增本机配置/业务调用计数和原宿主 census 反例，固定 14 个必需 root。最终 130 个叶例各 30 次，共 **3900 执行、3900 通过、0 失败、0 跳过**，退出码 0，缺失 root 0；源码冻结门禁通过。

独立只读复审确认原 intent 恢复方向符合现有 application 流程。发现的无 ID 输入漏计、紧急清理身份约束、失败退出计数三项已修正；随后复审无新增 P0–P2。这只认可测试适配，不能替代真实会话通过。

最终 `orcatransport` 与 `protocol/orca` 整包回归：2 包通过，186 个顶层测试通过，含子例 979 通过、0 失败、6 个明确 opt-in 探针跳过。跳过项不计真实资格。本次无生产代码增量；此前项目 L1 保持其原源码批次边界，没有冒称重跑全项目。

## 真实准备与当前阻断

Orca 1.4.242，本机 runtime `4f073b25-6af0-4b4b-bcb8-d2174d704504`。在现有 `SteerlineTest` 测试仓库创建两个全新 worktree，setup skip；原默认 shell 经确认无 agent/业务输入后精确关闭。未使用历史业务 Run。

| 现场 | 结果 | 业务输入 | 清理 |
| --- | --- | --- | --- |
| session-01 | 测试 census 错误要求无任何离线远端省略项；已按原 local-host 契约修正 | 0；未创建主终端 | 无新增终端 |
| session-01b | 原 control pending 回执缺 ResourceID；测试驱动尚未按现有流程对原 intent 对账 | 0；未启动 agent | 紧急精确关闭，后续完整 local census 为 0；不是正式 stop 通过 |
| session-02 | 已沿原 intent 对账；控制 READY 默认读取被屏幕折行，严格 binding 校验拒收 | 0；未启动 agent | 紧急精确关闭，完整 local census 为 0；不是正式 stop 通过 |

session-01b 保持失败。随后驱动仅补齐生产既有恢复语义，未修饰无效回执或补造 ResourceID。session-02 仍拒绝，证明不能用该恢复绕过 READY 校验。两个实际准备现场均无业务提交请求，不能计为独立原生输入成功会话。

当前 `control_terminal.go:controlLines` 和 `local_authentication.go:readLocalCodexReadiness` 使用默认 `terminal read --limit 1000`。真实控制 READY 被返回为 `source:screen` 的多行投影，原完整编码校验报 `original native control helper binding changed`。这是 Runner 对公开读取入口的用法问题；现有证据不能归因于 Orca 无可靠原始读取能力，也不能证明原历史多行提交问题已消失。

另开仅输出固定字符串的自有 shell 做对照：同一条 316 字符输出，默认读取为 screen、7 个显示行、无完整匹配；显式 `--cursor 0` 为 stream、1 个完整原始行，逐字相等。该探针没有模型输入，随后精确关闭。

最终再次核实两个本次 worktree 的 Git 状态干净、原 local-host 完整终端清单为零，使用精确 worktree ID 删除且不运行项目 hooks。原仓库 `.git/info/exclude` 前后 SHA256 一致；未留下这两个临时工作区，也未改写历史配置。

最小候选为两处读取显式传 `--cursor 0` 并要求 `source=stream`，保留原 runtime/terminal/incarnation、回执解码、绑定和有效性核验；不拼屏幕行、不调整视口、不增加回退。需要覆盖完整 stream、screen/缺 source、截断/缺失回执拒收。它涉及原准备/认证生产读取，超出此前“只替换多行提交、不动其它流程”的范围，已请求用户确认，**尚未应用**。未通过测试包装器注入参数来掩盖生产问题。

## 证据与未验边界

本次原始日志保留在 `.evidence/orca-local-input-20261030`；可审计的回归摘要、失败记录、公开读取对照及清理证据随本 PR 保存到 [evidence/orca-native-submit-local-20261030](evidence/orca-native-submit-local-20261030/manifest.json)。历史证据不改写。

三个成功本机原生会话仍待上述范围确认后用全新现场执行。未验的受理、开始、完成、正式原输入/final 和正式停止证明不得由受控通过、紧急清理或无模型对照探针代替。远端仍沿此前未验证状态，不借本机结果替代。未跑 Unity/业务检测，未修改 SteerLine、业务材料或全局配置。
