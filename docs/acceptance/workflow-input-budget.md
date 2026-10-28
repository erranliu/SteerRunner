# Runner 输入预算与提示词收敛验收

后续规则修订：调用方 2000B 与总量 3000B 正文预算已取消，仅保留 Runner 1000B，见[取消调用方预算验收](caller-input-unlimited.md)。下列大小与结论仍对应原冻结版本，不因本次放宽重新判定历史失败或未知投递。

日期：2026-10-28。依据 [Spec](../spec/workflow-input-budget.spec.md) 与 [Plan](../spec/workflow-input-budget.plan.md)。预算和提示词整改已实现，受控功能验证通过；真实最小链路的第二次 Runner 完成确认未通过。本报告不代表发布缺陷已修复、完整原生矩阵合格或业务链重新验收。未部署、创建 PR、恢复原 Run，未运行 MagicPotion/Unity 或 SteerLine 业务测试。

本报告原始验收早于后续零值修复和 PR 提交，后续结果单列在末节及专项报告。公开 PR 只提交可公开的验证摘要、输入样本和选定日志；含账号目录/内网现场信息的原始捕获留在本地原路径、原字节，不随 Git 发布，目录 README 说明公开证据边界。

## 实现与取舍

- 同一纯函数核对最终 UTF-8 的 C≤2000、R≤1000、T≤3000，三者不能借用。冻结正文摘要、来源/归属摘要、片段字节及三项计数，投递前核对。各原 owner 继续装配，没有中央渲染器、通用 DSL 或新运行时协议文件。
- 删除完整 implementation/audit/acquisition 对象、内部台账、重复材料/来源副本及 provider 查询/认证配置。报告只给原命令引用和必要失败事实，完整 session/turn/version/digest 仍由程序核验。发布保留 provider、repository、base URL、remote、base。
- 保留完成标记、结果字段、审计 P0–P2 修复与验证、提交规则、Git 2.55 特例、权限/凭据保护、失败/未知处理。原生标识、模型桥编码、结果解析与来源门槛未放宽。
- 精简默认正文、格式及会话说明。原生问答的人工字符串/正文转义计调用方，问题 ID 与 JSON 外壳计 Runner。发布、report、parse、只读不附加 progress/result 生产者规则；后续实施/审计仅在原生会话已核实收到规则时用短引用。文件变量、原字节容量、收集及封存不变。
- 原内联材料/schema 保留，但最终文字必须满足新容量。未投递 parser 若完整证据超预算，拒绝这个候选并保留已有一次 report；原 final 不截断，已尝试未知 parser 继续对账。旧冻结正文可靠可计量时原样恢复，不能计量只阻止新派发，观察/停止/读取/结算保留。
- `capabilities.model_input_contract` 从执行常量与内嵌来源发布同一份定义。元数据放入原 Run/输入、thread/ready、terminal config 和 parser 记录；Orca 保存逻辑正文及可复算的最终 wire 摘要，不另存一份完整正文。

单次调用方 2000B 是明确的容量收紧：原 10552B 实施输入（调用方 9845B、Runner 707B）不能继续原样投递。开发长材料沿已有 worktree 文件引用使用；路径和多引用的实际长度仍计 Runner，长值可能拒绝。内容 inline/schema 不自动换附件或生成新协议文件。这些容量变化与正常精简影响已被接受，没有关闭 workflow、adapter、Agent、parser 或文件通道。

## 所测 Runner 字节与优化目标

以下来自同源渲染与实际包装检查，单位为 UTF-8 字节。普通阶段使用 `docs/spec.md`、`docs/plan.md`，报告使用 32 字节命令 ID；发布目标使用 `team/project`、`origin/main` 及示例 Gitea base URL；模型桥为 `gpt-6.1-sol/xhigh`。实际其他动态值仍必须逐次检查。

| 步骤/输入 | 无命令头 | 原命令头 | 原模型桥 |
| --- | ---: | ---: | ---: |
| 普通实施（不含文件规则） | 152 | 278 | 553 |
| 实施报告 | 324 | 450 | 724 |
| 发布（含必要目标） | 322 | 448 | 721 |
| 发布报告 | 372 | 498 | 769 |
| 审计（含两项同会话短文件规则） | 557 | 683 | 960 |
| 审计报告 | 524 | 650 | 925 |

带审计 workflow 的实际四种文件组合另测：模型桥实施为 586/714/770/898B，审计为 902/932/930/960B，发布始终 657B；测试用原 `spec`、`plan` 路径及既有受控目标，发布没有文件写入引导。审计零修复/有修复、来源变化和一次后备使用原功能测试验证。

初始化为 118B；附认证规则后为 254B。标准 Windows PowerShell 路径下的会话 worker+ask 说明为 888B；DSH 两个固定工具说明合计 238B；带正常长度身份和有效 P1 修复证据的独立 parser 为 628B。

256/512/768B 是优化目标，1000B 是拒绝线。常规模型桥实施/发布、复杂审计/报告及 worker 会话有目标未达项：原标识/模型桥占约 400B，保留结构化审计字段和必要操作约束后占用较高。本期保留原协议及能力，不另造短标识协议来追求软目标。所测默认有效路径均低于硬上限，不保证任意长模型名、路径、目标、异常文本或证据可接收。

## 受控验证

原始通过日志见[证据目录](evidence/input-budget-20261028/)。范围包括：

- C=1999/2000/2001、R=999/1000/1001、T=2999/3000/3001，单方超限而总量未超；中文、非 BMP、组合 Unicode、CRLF/尾换行、JSON 转义和回答的精确字节。冻结后追加文字、变更来源及未声明来源均拒绝。
- 原 app-server、会话工具合并计量、异步回答、问答 JSON、Orca 包装及 parser 的接线；超限在模型发送前拒绝。四种文件开关、覆盖正文与反馈共用预算、同文显式覆盖、旧文字兼容和路由不兼容、长 final 保留与 report 后备。
- model/workflow/resources/resultacquisition/backend/reliable；Orca、codexexec、agentlaunch、process 的完整受控包测试；Codex 与 codexapp 完整受控包测试；内容 conversation/generate、schema/输出、恢复/控制、progress 独立版本和 result 原字节/封存的定向测试。
- `go test ./internal/... ./cmd/... -run '^$' -count=1` 编译接线。最初误用 `go test ./... -run '^$'` 时 integration 的 TestMain 因无 foundation 场景选择报错，没有运行新的业务场景。

首次沙箱运行遇到临时 Git、子进程与回环连接权限限制；普通权限且短临时目录下完成上述验证。初次广泛 application 测试还暴露过长临时路径和 5 分钟总时限；没有把该次整包运行列为通过。整改过程中发现的旧提示词字面断言及新测试对已有 4096B 脱敏视图的误用已修正，原长 final 存储未改；后续用定向功能验证补齐。未重新跑全部资格矩阵。

Windows 完整转义 argv 独立检查 32767 个 UTF-16 单元（含 NUL），有临界值、引号和非 BMP 用例；依据为 [CreateProcessW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw)。Codex 原 WebSocket 对完整 JSON payload 的 16MiB 保护保留。Orca 记录本次实际 CLI 文本参数、返回 bytesWritten 和 Windows 命令行长度，第三方内部 RPC 再封装不冒充可观测大小。

## 唯一真实短探针

使用原 `gpt-6-luna/xhigh`、原 `--no-daemon` 与同一路径的 SDK 环境 launcher、同一个 Orca environment。原生会话来源中的账号标识摘要与原失败证据相同，见[账号比对](evidence/input-budget-20261028/account-comparison.json)。新建独立 worktree，关闭 setup，未运行项目业务测试；初始化沿原生产入口。

| 输入 | C/R/T | 真实结果 |
| --- | --- | --- |
| 初始化 | 0/118/118 | 原生 READY |
| 首次短任务 | 227/0/227 | 一次 `Write-Output` 无副作用工具及结果在公开会话核实，Runner 核实原 final |
| 同会话发布式短指令 | 144/0/144 | 一次粘贴、两次稳定草稿读取、一次原生产提交；公开会话有完整相同输入与唯一助手回复；Runner 完成确认未返回，4 分钟探针期限到达 |

首次文字参数 227B、Orca 返回写入 240B，Windows 完整命令行 506 UTF-16 单元。后续显式 bracketed paste 为 156B（正文 144B 加控制字节 12B）、命令行 357 单元；唯一原生产 Enter 返回写入 1B、命令行 199 单元。两次业务输入各只尝试一次，CLI 没有该次 Orca 退出/超时错误；探针等待完成超时后取消观察，没有人工补 Enter、未知重发或放宽匹配。

后续原生用户输入时间为 `2026-10-28T05:38:11.197Z`，唯一助手回复时间为 `05:38:13.518Z`。这次可归类为**已接受但 Runner 未确认完成**，不属于预算拒绝，也不是仍留草稿。独立公开会话读取在现场关闭后保留为诊断，没有补造生产完成回执。原始时间线/粘贴提交/草稿/公开页/清理在 [short01](evidence/input-budget-20261028/short01/)；探针终端退出与零终端清点已确认，worktree 回收记录单列。

原 Run `600d14e87929e5555c74675b56cde86a` 和发布 operation `bbe27895a8d0e57683d4201371a55d48` 未恢复、重放或重新裁定。原失败只有 3427 字符草稿，没有精确失败字节数和已接受发布输入；此前 3767B 成功发布与 10552B 成功实施仍是原事实。本次只能把新现场问题定位到已接受后的原生开始/完成归属或观察链，不能证明它与原失败同根因。

未进行“立即发送/额外就绪/增大首次工具输出”的无依据重复对照。预算与提示词改动不等于投递问题修复；第二次 Runner 完成确认及原草稿失败的确切根因仍需独立处理。

## 完成确认的现有测试复核

用户追问后，未重跑真实模型或业务链。`TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput` 与 `TestThinOriginalQueryCompletesThroughNativeLifecycle` 各以 `-count=10` 运行，20 次顶层测试全部通过，见 `controlled-tests/completion-repeat.log`。它们验证给定受控元数据的恢复，尚未覆盖本次真实触发条件。现有真实入口的 `short-sequence` 只有一次失败样本，不能称为稳定复现；该样本未保存每次 observer 内部诊断/状态更新，尚不足以直接固化成确定失败的回归序列。

后续按用户授权补做[带原生状态与 observer 诊断的最小探针及受控回归](native-completion-confirmation.md)。新真实多行和单行均完成；捕获数据组合已稳定证明“无 started 回执 + 完整 generation=0 元数据、无其他合格证明”阻塞恢复入口。该轮没有重新复现真实持续超时，不能追认原超时或原发布草稿的根因；生产完成条件未放宽。

其后核实 Orca 1.4.242 的初始 incarnation=0 为合法值，并按授权修正 Runner 的解码和恢复资格判断。两个受影响包的完整受控回归、270 次关键场景及一次新的真实完整零值恢复路径均通过，详见上述报告末节。缺失/null 与零的区别和原输入/完成归属要求保留；这不追认本报告原超时或发布草稿的历史原因。
