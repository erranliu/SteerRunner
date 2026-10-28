# 同会话后续输入的完成确认诊断

日期：2026-10-28。复用 `TestOrcaNativeStateLiveLifecycle` 的 `short-sequence` 入口。前半部分保留修复前诊断；末节记录后续授权的合法零值修复及验收。**原发布投递与原超时的历史结论均未重新裁定。**

公开 PR 只包含摘要、脱敏契约核实副本、无副作用输入样本、源码摘要及选定测试日志。完整原始 CLI/observer/session/现场清理记录含账号目录与内网信息，保留本地原路径和原字节，由证据目录的 Git 排除规则保护；脱敏副本单列原始 SHA，不冒充原始发送回执。

修复前确定的可稳定回放阻塞组合是：原输入已受理、没有已核实 started 回执，后续独立查询始终返回带 generation=0 的完整 done 元数据，且没有其他合格证明。旧代码不进入原输入核验。修复前真实探针没有再次出现持续超时；末节新探针虽核实了修复路径，也不能将原来的超时直接归为这个原因。

## 证据与触发链

1. 多行输入走现有生产粘贴、稳定草稿检查、唯一一次 Enter；探针 03 的第二次 submit 回执为 `input_accepted=true`，没有 `turn_started`。只有接受回执不能认定已开始。
2. `NativeProof.signal` 不能仅凭 working 通知把未核实的 started 改成已核实。观察到 working 与核实原输入开始是两件事。
3. `queryCompletedOriginalStart` 要求 `CanQueryCompletionInput()`，之后才会调用有原终端/进程前后围栏的原输入查询。完整元数据分支的 `corroboratedCompletion` 明确要求 `MetadataObservation.Incarnation > 0`。本配置真实返回过完整 generation=0 元数据，因此该分支不合格。
4. 本来就受支持的 thin 投影可以进入另一条原输入查询路径。探针 03 第二次输入在 trace 序号 58 得到 thin 投影，61 完成原输入查询，随后生产 observer 正常确认 final。探针 04 则在单行发送时取得 started 回执，凭既有 observed-root 证明正常完成，即使后续完整元数据仍为 generation=0。

因此，多行、generation=0、输入短或长，单独都不能认定为根因。generation 是 hook 元数据中的数字，与终端/进程 incarnation 身份字符串不同。保留这两个身份检查，不把 generation=0 改成 1，也不将不完整元数据伪装成 thin。

生产位置：`internal/execution/adapters/orcatransport/native_start.go`、`native_state.go` 和 `internal/execution/adapters/protocol/orca/native_input_query.go`。相对首个带诊断探针的源码清单，诊断阶段 232 个生产 Go 文件摘要全部相同，见[脱敏汇总](evidence/native-completion-20261028/public-diagnostic-summary.json)。

## 最小真实探针

所有现场全新、独立，关闭 setup/hooks，经实际 Orca 原生生产 lifecycle 初始化 READY，再发一次仅执行 `Write-Output` 的短任务，最后在同一会话要求只返回唯一标记。第二条不运行工具、不创建 PR。模型 `gpt-6-luna/xhigh`、账号摘要、SDK 环境 launcher、`--no-daemon`、environment/runtime 与原配置一致；逐项证据保存在各自 `target.json`、`source-digests.json`、`native-result.json`，测试 EXE 摘要也已保存。

| 现场 | 实际结果 | 归类 |
| --- | --- | --- |
| probe01 | 首次任务已由生产 observer 确认；探针额外断言误要求 query 专属证明，拒绝了合法 observed-root 证明；未发第二条 | 测试断言失败，已修正测试断言，生产标准不变 |
| probe02-multiline | 首次任务完成；第二条唯一一次 paste 的 CLI 返回远端连接失败，未执行第二条 Enter；没有核实到第二条公开接受记录 | 该次结果不确定，未重发；不计为完成确认阻塞复现 |
| probe03-multiline | 首次任务完成；第二条 144B、多行、无 started 回执，随后通过 thin 投影的原输入核验完成 | 两条均由生产 observer 确认 |
| probe04-single-line | 与 03 相比只将第二条 LF 换成空格，仍为 144B；原发送取得 started 回执，后续完整 generation=0 也完成 | 两条均由生产 observer 确认 |

01 的修正限定在额外验收断言：继续要求精确输入、已有 `recordedInputMatches` 谓词，以及对应来源的原进程证明；observed-root 证明不再被误要求携带另一查询来源专属的 transient 标识。02 后的 03 使用新现场、新标记，不重发 02 的未知输入。03/04 首任务、就绪检查、模型/账号/SDK 均保持相同；未额外改变工具输出量或引入人工就绪延迟。

初始化 118B、首任务 227B、第二条 144B。第二条多行传输含 bracketed-paste 控制字节为 156B，完整 Windows argv 含 NUL 为 357 UTF-16 单元；单行 text+Enter 为 144B、423 单元。物理 paste 调用、模型逻辑输入和 bare Enter 分开计数；不将 CLI 的 `accepted` 冒充模型已开始。旧探针字段 `paste_calls` 实际统计全部 `--text` 调用，`enter_calls` 包含 text+Enter 与 bare Enter；实际 bracketed paste/bare Enter 数另在汇总中由 argv 推导。各次请求原始时间线、退出/取消、返回写入字节、稳定草稿读取与公开页见 `actual-cli-*.json`、`actual-public-draft-read-*.json`、`native-result.json`。

`native-observer.jsonl` 记录 feed 建立、初始值、每次 drain/query/error/close，独立 native/input 查询的前后状态，observer fact/diagnostic/公开消息及保存的 State。额外显式记录通常不经 JSON 输出的 `MetadataObservation`、`ThinMetadata`、查询资格、样本来源/接收时间、原输入查询/进程证明标记。失败后使用独立、有限时的只读诊断上下文，在关闭终端前读取原生状态、公开输入/页与屏幕；诊断不反馈为生产回执。

同步写入诊断会影响细微时序，本轮成功不能排除时序问题。封装数据限于可观测的 CLI 参数、控制字节、Windows 转义 argv、stdout 和返回 bytesWritten；未取得第三方内部 WebSocket/RPC 完整封装，不以这些数值冒充其传输大小。

四个现场终端均已核实关闭、全范围终端清点为零；精确路径核对后已回收四个独立 worktree，后续查询均 `selector_not_found`。创建、清点与 `cleanup-*` 回收证据保留在[证据目录](evidence/native-completion-20261028/)。未恢复原 Run，未跑 MagicPotion、Unity 或 SteerLine 业务链。

## 固化的受控回归

扩展现有 `TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput`，未另造运行时协议。`testdata/completion-zero-incarnation.json` 是**组合夹具**：取 04 实测完整 generation=0 done 形状，结合 03 实测多行 accepted/no-start 的发送形状。标识与唯一标记替换、时钟同源平移、revision 相对 baseline 重编号均在 fixture 中声明。它不是某次真实阻塞现场的完整连续回放；generation=1 是明确的单字段受控对照，不是真实捕获。

| 子例 | 预期断言 |
| --- | --- |
| zero-incarnation | 完整 generation=0 + 无 started：不确认，不读原输入，不重发；重复观察仍相同 |
| zero-incarnation-started | 保留原 started 回执：既有 observed-root 证明可完成，额外探针断言也通过 |
| multiline-zero-incarnation | 真实 prepare/submit 粘贴分支 + 捕获的完整 generation=0：稳定不确认，恰好一次 paste 和一次 Enter，无额外投递 |
| multiline-generation-one | 只将上述 metadata generation 改为 1：围栏查询核实唯一原输入，正常确认完成 |

这组测试锁定的是阻塞触发条件和现有安全边界，不是修复验收。以后若修复，需要替换阻塞预期并另证原输入来源，不能直接放宽谓词。

```powershell
go test ./internal/execution/adapters/orcatransport -run '^TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput/(zero-incarnation|zero-incarnation-started|multiline-zero-incarnation|multiline-generation-one)$' -count=30 -v -timeout 90s
```

四个子例各 30 次、共 120 次通过：[最终日志](evidence/native-completion-20261028/stable-captured-final-30.log)。先前一次重复运行暴露 Windows 测试用命名管道退出挂起，另一次唤醒连接过早关闭仍失败；这两次保留为失败记录。最终只修正测试夹具：先保持唤醒连接直到 Accept 退出，再关 listener，不改被测 RPC 内容或完成条件。[完整 adapter 受控包](evidence/native-completion-20261028/adapter-regression-final.log)也通过（93.561s），真实探针开关在该验证时关闭。

原 4 分钟完成确认超时缺少当时逐次 observer 诊断，原 3427 字符发布草稿还属于另一个没有接受回执的现场。新诊断、两次真实成功对照和受控稳定阻塞均不追认它们同根因，也不证明原投递问题已经修复。

## Orca 契约核实及最小修复方案

2026-10-28 只读调查确认：原探针 environment 当前 runtime ID 仍为 `878c2e5f-9602-4f7c-85a9-5239a6116f1b`，报告版本 1.4.242；用户升级后的本机 CLI 和安装包也是 1.4.242。核对的是同一发布版本的实现，没有宣称已取得远端安装包相同 SHA，也没有追溯原故障当时的原始 observation 报文。本轮未创建终端、恢复 Run、发送模型输入或运行业务链。

核实结果：**数值字段 `observation.incarnation=0` 是 Orca 的合法初始代次，Runner 要求 `>0` 是适配错误。** 前文的 generation 是对此数值的简称；实际字段名为 incarnation，不是 terminal 的字符串 incarnationId。

- 安装包的 `AgentStatusObservationSequencer` 以 `incarnationFloor=0` 初始化。首次及后续同代 observe 均返回 0，revision 自 1 递增；只有 rebind 才递增 incarnation。authority 每次进程启动重新生成，不能跨 authority 比较 revision/incarnation。
- 实际 main-agent-hooks 的打包实现使用相同的零初始 sequencer，并通过 `stampObservation` 生成 observation，非仅一个未使用的共享工具类。
- Orca 的 IPC codec 要求 observation 显式包含 incarnation，类型必须为非负安全整数。完整显式 0 合法，缺失、null、负数、小数和字符串非法。整个 observation 未提供则属于另一种缺失元数据形状，不能与部分字段缺失混淆。
- 已直接运行安装包的纯 sequencer/codec 模块验证以上行为；没有调用 agent 或状态变更 API。[脱敏执行结果与源码摘要](evidence/native-completion-20261028/orca-contract-check/public-installed-contract-check.json)、[脱敏源码行摘录](evidence/native-completion-20261028/orca-contract-check/public-upstream-observation-excerpts.json)、[脱敏实际 host 调用摘录](evidence/native-completion-20261028/orca-contract-check/public-host-observation-snippets.json)可公开审阅，原件及远端 status 保留本地。

因此无需要求 Orca 将初始值改成 1，也不应等待它偶然返回 thin 投影。Runner 的正常观察路径原来已经可以使用合法 0，漏 started 恢复路径却排除了它。当前 Go 解码同时把字段缺失/null 默认为 0，修复不能只把 `>0` 改成 `>=0`。

最小改动集中在现有 Orca 协议解码、observation 有效性判断及原输入恢复测试，不增加框架、DSL、回执类型或运行时协议文件：

1. **保留缺失语义。** 在现有 observation 类型/解码中区分显式数值与缺失/null；采用能在 JSON 往返中保留该区别的表示，不能用反序列化后即丢失的临时标志。incarnation 只接受 Orca 定义的非负安全整数。完整显式 0 与 1 在证明资格上平等；字段缺失、非法 observation 仍不能获得证明，也不能被降格伪装成 thin。
2. **统一同一事实的校验。** 复用一个轻量 observation 有效性判定，供正常 hook 观察和完整 metadata 恢复使用；保留各自对 transition、时钟、revision 新鲜度及前后完整状态匹配的要求。消除仅恢复路径拥有的 `incarnation>0` 假设，不添加 zero 专用兜底。
3. **沿用原确认链。** 没有 started 回执仍不能仅凭 working/屏幕/accepted 开始或完成。合法完整 0 可以进入已有的原终端/进程前后校验、唯一用户输入全文核对、冻结 baseline、pane/provider/路径及稳定 canonical done/正文核对；只有该链全部通过，才按原设计补证原输入已开始并确认完成。authority/incarnation 变化、状态变化、重复输入、来源冲突、工具输出、边界/中断等继续拒绝，正向替换证据不能被后续错误清掉。
4. **保持存量边界。** 不追认旧未知操作为成功，不改已确认 final。旧未决状态只能用原资源的新的只读查询补证，不能凭旧序列化中可能丢失的字段存在性直接增加确认资格；原 intent、停止和零重发规则保留。

验收修改为：将当前两个合法完整 0 的阻塞子例改为应通过已有围栏查询完成，0/1 对照均通过；增加原始 JSON 缺失/null/负数/小数/字符串/超安全整数、JSON 往返以及部分 observation 不变成 thin 的反例。继续覆盖 authority 和数值 incarnation 的 0→1/1→0 变化、provider/终端/进程/路径变化、revision 回退、重复/变更原输入、正文/时间变化、重建 observer 与正向冲突持久化；所有场景断言没有额外投递。完成受影响包的受控回归后，才按原配置开展有限的新独立最小真实资格探针，原失败不重放、业务链不重跑。

该方案修正已证实的 Runner 契约缺口。它仍不证明此前 4 分钟真实超时或最初草稿未提交由此造成；历史根因结论保持未定。

## 合法零值修复实施与验收

按后续用户授权实施。未增加兜底发送、重试层、框架、DSL、公开回执类型或运行时协议文件。生产改动仅涉及四个文件：

- `protocol/orca/state_feed.go` 的现有 observation 保留 incarnation 缺失标记；新增同包 `state_observation.go` 实现对应 JSON 解码/编码及 `ValidHookTransition`。仍保留数值字段和可比较的值语义；显式 0 保持 0，缺失/null 保存后仍省略 incarnation，不会经 JSON 往返变成合法 0。负数/超安全整数无资格，非整数/非数值解码失败。没有新公开 JSON 字段或持久版本。
- 正常 hook 观察、已有 observed-root 原输入匹配和完整 metadata 恢复复用该基础判断。`native_input_query.go` 不再要求 incarnation 大于 0；`orcatransport/native_state.go` 继续执行各自的身份、输入和时序判断。
- 原 `queryFencedCompletionInput`、`queryCompletedOriginalStart`、终端/进程围栏、唯一原输入及全文匹配、baseline、前后 canonical 状态和正文匹配要求均保留。合法 0 只取得核验资格，不直接提供 started/final；既有 transient 查询证明不能从 JSON 伪造。

受控回归先于真实探针完成，日志在 [zero-fix](evidence/native-completion-20261028/zero-fix/)：

| 验证 | 结果 |
| --- | --- |
| observation 原始 JSON 与两轮保存/恢复、完整 metadata 原输入围栏、漏 started 恢复定向测试 | 通过；缺失/null 不获得资格，也不变成 thin |
| protocol/orca 完整受控包 | 通过，26.700s |
| orcatransport 完整受控包，所有真实开关关闭 | 通过，95.618s |
| 9 个关键场景各重复 30 次 | 270 次通过；5 个恢复/对照场景共 150 次完成，4 个缺失/null 场景共 120 次保持拒绝，无额外投递 |

9 个场景包括显式零、零值 observer 重建、原 started 对照、多行零、正值，以及普通/多行各自缺失和 null。原先两个已知合法零值的阻塞预期已改为必须沿现有围栏查询恢复，不再以“保持阻塞”冒充修复通过。完整 metadata 反例也改为在零代次运行，补充 0→1、1→0、revision 回退，以及字段缺失/null/非法范围；原终端/进程/provider/路径、重复输入、正文和状态变化等反例仍拒绝。第一次普通沙箱调用因 `C:/sr-bgt` 写权限失败、没有开始测试，保留 `directed-initial.log`，后续经批准的普通权限验证单列。

在以上受控验证通过后，只执行一次新的独立 `short-sequence` 真实探针，未重复运行旧现场。新 worktree 为 `sr-zero-incarnation-20261028-01`，同一原配置的 `gpt-6-luna/xhigh`、SDK 环境 launcher、`--no-daemon` 和账号摘要；关闭 setup。输入仍为初始化 118B、首次无副作用工具任务 227B、同会话发布式唯一标记 144B。[实际源码、EXE、请求时间线、封装和公开回执](evidence/native-completion-20261028/zero-fix/probe01/)均已冻结，415 个执行侧 Go 文件摘要在真实探针后核对无变化。

这次真实探针**确实经过完整显式零值的修复路径**：

| trace 序号 | 真实事实 |
| --- | --- |
| 54 | 第二条唯一原 submit 回执 `input_accepted=true`，没有 `turn_started` |
| 65 | 独立查询为完整 done metadata，incarnation=0、revision=406；`ThinMetadata=false`，`CanQueryCompletionInput=true` |
| 67/68 | 通过已有原输入查询；前后 canonical 状态及 metadata 相同，`MatchesOriginalQuery` 和 `MatchesOriginalProcess` 均为 true；唯一用户行全文等于冻结输入 |
| 76 | 原生产 observer 确认第二条 final；原输入的开始由完成证据补证，没有伪造 streamed working |

两条任务均通过（测试 47.87s、包 50.285s）。第二条 baseline 后仅一条用户输入、没有工具调用；首次任务仅一次 `Write-Output` 工具。物理输入为首任务一次 text+Enter、第二条一次 bracketed paste 和一次 bare Enter；没有补 Enter、未知重发或 PR。完整字节/Windows argv 与退出结果按实际 CLI 调用记录，不冒充第三方内部 RPC 封装。

终端已确认关闭，完整 scoped census 为零；核对精确非主路径后移除新 worktree，后续 `worktree show` 返回 `selector_not_found`。结果汇总见 [verification-summary.json](evidence/native-completion-20261028/zero-fix/verification-summary.json)。

**验收结论仅为合法零值契约缺口已修正，并由受控和本次最小真实路径验证。** 原 Run、原 publication operation 与原 4 分钟超时均未恢复或重放，原失败不追认为已修复，也未重跑 MagicPotion、Unity 或 SteerLine 业务链。

后续按授权将 PR67 的必要提交前草稿重绘修复整合到 PR68；本节零值生产实现与确认条件不变，新整合的源码/受控及真实资格另见[整合验收](pr67-integration.md)。不把旧源码的通过记录直接替代整合后验收。
