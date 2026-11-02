# Spec：Orca 无 helper 原生可行性核验 B0

日期：2026-11-02。状态：待实施。代码基线：`efa871cfffb950505bf2f0a325ec7751fb4be7f3`。对应 [Plan](orca-native-capability-baseline.plan.md)，依据[能力层长期设计 B0](orca-capability-layer.spec.md#61-b0-的可行性证据与零旧链调用)。

## 1. 目标和交付

在本机和配对远端各用一个全新 Orca 终端，验证无 managed helper 的原生 Codex 能接受原输入、取得完整 final、在同一会话接收第二次输入，并按原终端范围关闭及核实完整清单。本批先取得共同原生能力与现行合同的差距，供 B3 的职责拆分和 B5 候选合同设计使用。

本批只扩展测试探针、测试入口和验收文档，不修改生产算法、准入、profile/state 编码、停止 scope、Orca 安装和全局设置。交付包括双端原始证据、同源构建身份、独立审计结果及能力差距表。探针失败、环境阻断或清理未知都保留，不以旧 managed 通过代替。

本批不运行 development Workflow，不创建业务提交或 PR，不提供实际模型/effort、审计工作区或全进程树停止保证，不实施 B1。后续 B5 的真实 Runner 受理和结算要求不移入本批。

## 2. 当前接线及最小扩展

当前 `scripts/test-native-input.ps1` 的本机前置要求 helper；`runLocalNativeInputProbe` 使用 control/history。原样运行不能证明 B0。复用同一个脚本和 `TestOrcaNativeStateLiveLifecycle`，增加测试专用模式 `capability-baseline`；原 `long-input`、`controlled` 及已有短序列行为保持。

新增探针以真实 `NewLifecycle` 及已有原生查询、输入和 final 核验代码执行，不注入 Started、queryInput、stateFactory 或伪造回执。探针可以构造隔离的内存 Snapshot 和本地证据 journal，它们不是正式 Runner Run，不能当作 B5 业务资格。

本机使用当前可解码的本机 profile，远端使用当前原生远端 profile；两者均无 RuntimeExecutable、RuntimeSHA256、Terminal、RemoteReadiness、Control 或 Recorded history。不得把本机伪装成远端 profile 来绕过构造检查。codec 和 Target 摘要由当前实现生成，不修改 decoder。

若原生 API 或现行核验代码不能证明目标，记录具体缺口，不放宽条件、不修改生产分支补出“通过”。正式本机无 helper 的停止合同仍未由本批实现；探针清理直接消费当前 Orca 关闭及完整清单能力，不调用本机 helper 停止合同后忽略错误。

## 3. 固定目标与资源

目标复用 `nativeInputProbeTarget` 的 Worktree、Runtime、Environment、CLI、Launch 字段。CLI 为确定的绝对路径，Worktree 为明确 `id:` selector，Runtime 必须与实时响应一致。本机 Environment 为空，远端为明确环境；失败不换目标或回退。本机发现和远端环境凭据沿现有机制，凭据不写证据。

Launch 明确 full_access、Codex executable、启动模型和 effort；RuntimeExecutable 与 AuditModel 必须为空。模型/effort只记录冻结启动偏好，本批不验证实际生效。不得沿用原 managed 目标中 helper/审计字段，或改业务仓库 URL 规避本机准备条件。

每个目标使用本轮专属、已由现有 fixture/Orca 机制准备的空 worktree。探针不创建 worktree，不接管历史业务终端。开始前完整清单须证明选定 runtime/worktree 的 local 执行主机范围可见、清单未截断且无终端；随后只创建一个有完整回执的终端。前置资源本身由 fixture 所有者管理，探针不能删除外部分配的 worktree 或声称其物理目录已消失。

每次调用使用全新证据目录，创建不可重复的 attempted 标记。出现未知创建且无法取得确切原身份时，只读记录原清单并保留人工清理信息，不模糊认领终端、不重复创建、不在旧目录补跑。

## 4. 两次原输入与证明

启动命令使用现有 `agentlaunch.Config.Command` 直接运行 Codex。原生初始化 READY 只建立会话，不计业务输入。确认原 terminal/incarnation/worktree/pane/provider，读取真实 READY 和主代理基线。需要 compositor 就绪时只使用已有 `waitOriginalComposer` 的有界等待，不加固定 sleep、补 Enter 或专用提交算法；任何探针增加的等待均记录为测试装配，不能冒称当前正式本机流程已具备该步骤。

随后按顺序执行两次只读 nonce 输入，第一轮成功结算后才准备第二轮：

1. 第一轮生成随机值 A，要求记住 A 并精确回复带 A 的唯一标记，正文包含中文、逻辑换行和空行。
2. 第二轮生成新值 B，正文要求使用上一轮记住的值，精确回复 B 和 A 的组合；第二轮不重复提供 A。两轮必须仍是同一原 provider session 和 terminal/incarnation。

提示词禁止工具、文件修改、业务测试、Git/发布和部署。本批不把提示词约束称为 OS 沙箱或独立工具禁用资格。两轮都有独立 command/operation/request、冻结逻辑正文/摘要/预算、发送前意图和原生状态基线。业务发送每轮一次，总计两次；初始化和创建分开计数，不按所有 terminal send 的总数误计业务输入。

两轮均复用当前通用 text+enter 编译和原生证明：accepted 不能当作 Started；Started/Completed 来自原输入、原主代理新 done 及完整 final。独立 `QueryOriginalInput` 核验每轮唯一正式原文和原进程/回合；第一轮基线不能误用于第二轮。旧 done、重复或改变正文、换 provider、截断 final 均不通过。观察失败只读查询原资源，不补输入、不换请求 ID、不补报。

满足以下证据才签收“零旧链”：实际命令中 helper 启动为零；control terminal 创建和 actor original-turn 请求为零；两个 Turn 均无 Recorded/Extension 和 managed WireFormat，State 无 helper readiness/control；最终来源为原生观察/原输入查询。仅靠计数器默认零不够，须与实际创建/发送日志、State/Turn 及回执相互核对。Orca 的 nativeChat.readSession 是允许的原生读取，不归类为禁止的 managed recorded-history reader。

## 5. 有界终止与完整清单

正常完成和失败路径均进入唯一原终端清理。关闭前核对原 runtime、terminal、incarnation 和 worktree；每个已创建终端最多外发一次 close。响应丢失后只查询原完整清单，退出或确实缺席才记为清理确认；仍活动、缺 host、截断、计数不符或换 incarnation 保持未核实。原终端无法定位时不创建清理 actor、不按模糊 PID 杀进程。

不补造 close 回执，不因重复调用探针而重试旧关闭。即使已证明排队未派发，本轮也不增加自动 mutation 重试；保留首次未派发的事实和原资源清理需求。清理后核对本轮专属 worktree 的完整空清单，记录逐项身份和错误。

本批结论固定为“原 Orca 终端范围”，不写正式 Run stopped/resources_settled，不声称 helper/全部后代退出或宿主凭据撤销。独立能力查询/清理不替代正式 CLI `service verify-stopped`，该接线属于 B5。

场内准备、两轮业务和观察共用原探针 6 分钟预算，不按轮重置；异常收尾最多使用独立 45 秒，只终止原资源，超时不能把业务失败改判成功。测试进程沿现有 10 分钟上限；不新增生产预算或持久游标。

## 6. 验收与证据

| ID | 必需条件 | 证据 |
| --- | --- | --- |
| OCB0-01 | 固定双端目标，helper/审计字段在资源创建前拒绝，非空/不完整清单不创建 | 目标验证单测、真实前置清单、实际调用计数 |
| OCB0-02 | 同一新原生会话两轮输入各仅一次，两个完整原输入、新 done、精确 final 均成立 | 每轮冻结正文、intent、request/accepted、查询页和 final/摘要 |
| OCB0-03 | helper/control/recorded history 为零，未伪造 production callback 或开始事实 | 实际命令、资源清单、State/Turn、构造器接线和来源核验 |
| OCB0-04 | 只关闭原资源一次；响应未知只读对账；完整清单确认终端结束 | 清理反例、close 调用日志、原 host 完整空清单 |
| OCB0-05 | 缺任一轮、错误身份/来源、截断、未知、skip、零匹配或清理失败不得通过 | 新模式结果校验负例和每场汇总 |
| OCB0-06 | 生产与旧模式保持，测试源码/EXE/CLI/目标身份固定，读取凭据不落证据 | 变更白名单、受影响回归、构建摘要及同源检查 |
| OCB0-07 | 双端分别完成并输出能力差距；未验能力不升级承诺 | 独立验收报告、已证/差距/未验三栏，明确 B3/B5 后继条件 |

首次资格为本机一场、远端一场，各一个新终端、两个业务回合，不要求重复 30 次或完整稳定性矩阵。失败保留原目录；修复后仅补跑受影响目标的新现场，不能把旧失败重写为成功。远端未达或清理未知时，本批整体未完成，不能据本机结果放行 B3。

受测源码、测试 EXE、Orca CLI、固定目标及实际启动命令摘要一起记录。证据包括每次调用的阶段/时刻/目标/原请求及脱敏回执，完整业务正文仅为本批非秘密 nonce；不得保存配对 token、认证帧、完整环境或真实业务材料。缺少底层可观察次数时如实注明，不能把 Runner 一次调用解释为宿主内部一次按键。

## 7. 范围与后续

实现仅限新增 `native_capability_probe_test.go`、现有 live 测试模式分派及必要的 `_test.go` 记录/校验支撑、`scripts/test-native-input.ps1` 的新模式和相关文档。新能力不增加正式 Run 设置、后端、生产探针命令或协议版本。旧 managed 模式的前置和结果判断不得放宽；旧受控矩阵不能为了新模式被替换或删除。

如果失败来自 Orca 协议缺口、生产证明限制或当前部署环境，先记录复现和范围；本批不顺带修生产、不修改 Orca、不安装/升级工具、不改变认证。B1/B2a 可按长期路线独立推进，B3 须消费 B0 的双端有效结论，B5 仍需另外完成真实 Runner 和开发资格。
