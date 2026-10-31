# Orca 原生输入保真与正式开始凭据验收

后续再审计确认597d669仍有普通本机无helper查询接线P2，旧审计漏掉了这一组合；其中泛称六场模型/effort已核验也超出证据，实际history模型证明仅在本机managed核验，远端只有冻结启动偏好、实际model/effort未独立核验。本页保留原构建已执行的测试与六场结果，原放行结论由[本轮再审计](orca-native-submit-reaudit.md)替代，不代表新源资格；原Spec/Plan可按597d669历史版本核对。

2026-10-31。对应 [Spec §4.1](../spec/orca-native-submit.spec.md)、[Plan P0.1](../spec/orca-native-submit.plan.md) 和 [PR #73](https://github.com/erranliu/SteerRunner/pull/73)。本次冻结源码的稳定回归 **7050 通过、0 失败、0 跳过**；全新本机 **3/3**、远端 **3/3** 原生会话通过。项目L1及最终独立补充复审均通过，无未修P0–P2；按本报告证据边界放行。

本记录追加新构建资格，不改变[上一批本机三场失败](orca-native-submit-control-stream.md)或历史未知 Run。未向旧 Run 重发或补 Enter，未部署安装、修改 Orca、用户全局配置、SteerLine 或业务检测材料。

## 修正及收敛范围

生产增量只有四项，继续使用既有提交、模型桥和 Turn 事实：

1. 模型桥上游 WebSocket 消息上限从 16 MiB 对齐安装版本原生 Codex remote client 的 **128 MiB 有界上限**；下游输入仍为 2 MiB。认证、单活动连接和原请求约束保留，不增加重连或启动等待。
2. 所有原生 Codex 在同一 `terminal send --text --enter --retry-request` 前把逻辑 LF 编译为终端粘贴 CR。编译发生在既有控制字符拒绝之后；本机/远端、单行/多行、既有 command/model wire 格式共用规则。Codex 既有 paste 规范化恢复 LF；冻结正文、逻辑 wire、预算、摘要及正式原输入比较仍保持原规则。没有 LF 回退、目标分支或开关。
3. 模型桥拒绝非法原请求时，向原数字/字符串 JSON-RPC id 返回 `-32602` 结构化错误，不转发该请求；缺失或非法 id 关闭连接。错误不包含正文/凭据，有 id 的拒绝不再以静默断连掩盖原因。没有重试。
4. 原生 Codex 的 Orca `turn_started` 回执只保留为宿主阶段提示，不能单独发布业务 Started。本机继续原 `observeRecorded`，远端继续原 `queryCompletedOriginalStart`。远端可能直到 done 才建立正式开始事实，在此之前不会发布依赖 Started 的 working；这是保守的可见时序变化。其它 agent/terminal extension 规则不变。

原 PR 已删除手动 paste/Enter、屏幕草稿全文/折叠标签匹配、显示换行推断、两次稳定草稿、正文匹配专用视口调整/恢复及其配置、状态与专用测试。此次没有恢复它们；同步清理旧 native-start Spec/Plan 中仍指导使用草稿门禁、分段两次粘贴的活动说明，历史验收原件保留。无诊断探针、实验模式、第二套状态机或通用执行框架进入生产。

## 根因证据与不能扩写的结论

旧三场本机 CLI 正文均为 5093 字节，原 session 的 Codex 提交历史只有 5079/5077/5077 字节，仅缺 14/16/16 个 LF；当时宿主阶段被错误提升为业务 Started，正式业务 turn 没有成立。它证明正文在原生链路中损坏，不能唯一指出 Orca、ConPTY 或 Codex 的具体函数。

临时 Go overlay 诊断捕获 `websocket: message too big: read limited at 16777217 bytes`，随后下游取消、重连。大元数据请求在业务发送前启动，首个限额断连发生在正文处理中。对齐 128 MiB 后，实际 `plugin/list` **23,771,840 字节**回复完整转发；这是 Runner 桥接限额不匹配的直接证据。诊断会话未完成业务，不计真实通过；诊断代码和 overlay 均未进入受测正式 agent 或生产源码。

版本匹配的公开 Codex 源码 tag 为 `rust-v0.166.0-alpha.5.1`，解析到 commit `f79b26ccbb3116b052544666a181012fb45dc49b`。原生上限见 [remote.rs](https://github.com/openai/codex/blob/f79b26ccbb3116b052544666a181012fb45dc49b/codex-rs/app-server-client/src/remote.rs)，CR 规范化见 [paste_input.rs](https://github.com/openai/codex/blob/f79b26ccbb3116b052544666a181012fb45dc49b/codex-rs/tui/src/bottom_pane/chat_composer/paste_input.rs)。tag 解析和源码摘要是来源凭据，不声称安装二进制与源码可复现构建等价。

两个阴性对照均保持失败并完成正式停止：旧 16 MiB 桥接下单独 CR 仍缺 8 个 LF；128 MiB 加 LF 的后续本机三场为 2 通过、1 缺 2 个 LF，后者在同一稳定 TUI 连接中发生。因此限额断连不是全部缺换行的充分解释，CR 单项也没有独立通过资格。最终 128 MiB＋CR 组合才具有下述同源六场资格。不是 Orca 通用提交 API 缺口或 Runner 传 LF 必然误用的完整证明。

## 冻结构建与确定性回归

构建时 HEAD `c5104e0bf43c15b9bb75ebfe821f24a5385a6f65`，dirty=true；正式源摘要 **`c8ead691b0f28ebae4c9bfeb21c2dd90e2966535f8a7873873aa0fa5d7e81395`**。散列范围是正式入口已有的 `internal`、`cmd/steerrunner-agent`、go.mod/go.sum 和两个测试/交付脚本，逐文件清单随证据保存。不得把它改写为在后续干净 commit 上构建。

`source-index-mapping.json`另核验695个源文件与Git暂存内容的对应关系：只有`native_start_windows_test.go`及`test-native-input.ps1`依仓库既有属性做CRLF→LF转换，其余原字节相同；不把原始构建摘要冒称为Git规范化内容摘要，不修改全局Git配置。

隔离构建现有 agent SHA256 `10f90833023caf64c4ec4c6412d7c66cc398983f10202fab1e9aa508104db15e`；六场测试 EXE 均为 `e8fb4781abb74ebed2594bfb05b213667a034ce252e191fa905016ab2816e0d2`。未安装这些二进制。六场与首轮7050使用同一冻结源；最后检查发现的测试夹具修正及新的检查身份另记如下，不声称所有检查全源相同。

`scripts/test-native-input.ps1 -Mode controlled -Repeat 30`：**24 个必需根测试、235 个叶例，各 30 次，实际 7050 次，7050 通过、0 失败、0 跳过、缺失根 0**。NS-T01–T13 与测试入口的对应关系保留在 Plan，逐 case 实数及完整原事件见证据。

- 正文覆盖多行、中文、空行、CRLF、长输入及既有 wire；实际 CR 参数与逻辑 LF 摘要分别核对。屏幕折行、折叠标签、投影缺换行/不可读不进入提交判断。
- 模型桥缺换行/截断/改字节/混入输入、外来线程/模型/方法：真实 WebSocket 计数证明拒绝零转发、保留原 RPC id，错误后同连接只读查询仍通。
- 超过旧 16 MiB 的完整回复核验长度/摘要，同连接后续原提交，连接数 1、请求数 2；旧上限 overlay 红对照失败。
- 延迟/分段/回执故障、仅 accepted 或带 turn_started、身份替换、停止/取消/队列、原输入/结果/模型/认证清理反例按原夹具的 channel/屏障控制，不靠固定等待或复用快照充数。

`controlled-summary.json` 嵌入的 source.passed 是正式脚本在最终源校验前保存的中间 false；最终 `controlled-result.json` 为 passed=true，原事件/计数/源摘要同时成立。不改写原摘要。

此前开发检查中的测试前提错误已修正；之后 application 包出现过12分钟聚合超时，原失败保留，不能算通过。首次c8检查曾仅当前进程设置Go包并行度1；修后最后L1使用原入口及既有Go默认并行，保留全部测试与原12分钟单包时限，没有修改全局配置或增加预算。

本次 c8 源的最后 L1 也保持失败：application 已通过，但 Orca RPC 单测在夹具清理 `terminalQueryIPC → win32PipeListener.Close` 等待，包原12分钟期限耗尽。原栈中的 listenerRoutine 已回外层select，可确认关闭等待，不能唯一断定是 ERROR_NO_DATA。修复沿用仓库 thin-query 夹具已有的 stop/wake/Accept退出再Close顺序，活动连接关闭与worker等待在失败分支也保留，wake失败明确报错。仅修改 `terminal_query_windows_test.go`；不升级依赖、修改生产RPC、延长原时限或跳过测试。

修后测试源摘要 **`b3a64227b6b7af3ab4595b24cd85434fe56d427f2685d3cbc488555e8e635c91`**。逐文件比较证明，相对六场 c8 源只有该单元夹具文件变化，无 init/包级副作用、live驱动依赖或agent构建参与；独立复审认可此界限。重新隔离构建的 agent 与六场原产物 SHA256 **完全相同**，生产文件/正式live驱动未变，因此保留原六场真实资格，不额外投递任务。原IPC/并发回归6根33叶各30次，**990通过、0失败、0跳过**；修后正式controlled30也为**7050通过、0失败、0跳过**，新TestEXE/原事件分别封存，完整L1通过：721顶层/2730含子例通过、0失败，17明确opt-in/helper测试跳过及1无测试包另列；34包、24框架检查、29终端扩展、native driver编译和零匹配门禁通过。默认L1的17项跳过不算真机资格，六场真实结果另记。application包532.881秒、Orca适配器88.844秒，均在原12分钟单包时限内。

## 六个独立真实原生会话

正式入口 `scripts/test-native-input.ps1 -Mode long-input`，实际调用 `TestOrcaNativeStateLiveLifecycle`。每场都是新隔离工作区、终端、incarnation、Run、原请求及 provider session；只要求无副作用的 nonce 回答，不执行项目检测/Unity。

| 目标 | Run | 原 request id | provider session | 结果 |
| --- | --- | --- | --- | --- |
| 本机 1 | `1159dff109238e3203bc8f3f7aafea8c` | `b8cb4b92-1a57-b84b-abe2-25051311b874` | `01a18fb1-c39b-7eb3-9dc2-0a5f9038416c` | 通过 |
| 本机 2 | `16231db3b248a6d0e5d3e90ddfdcfb41` | `131e1ad4-5d9a-827e-bdcb-1209db6e9240` | `01a18fb4-eaa9-7c90-af40-6c14ab0690de` | 通过 |
| 本机 3 | `d61814c984a0895efa0ee64a156e09d0` | `7b356888-21f9-cf5a-cb48-1b943e77ea1b` | `01a18fb6-51af-7850-aacc-eddea7a77198` | 通过 |
| 远端 1 | `f84bbf2f7ce593dd01d4bdf440914ed5` | `c67038f5-69b6-f7e7-e7f1-8fdf4ee0bb4a` | `01a18fb4-b6bb-7e82-8a3a-f96bff450c00` | 通过 |
| 远端 2 | `8d3329b47ad4041a78e799f4826c8aa2` | `004dfee5-1593-7ca9-9ec2-0de384458b5f` | `01a18fb7-0679-7602-a984-b60871fbd57a` | 通过 |
| 远端 3 | `697985961dc4000ea63a9e8c939c0f4b` | `ce3af2bf-36db-bb8f-610c-8d68f5af72d9` | `01a18fb9-747e-7da1-bb2b-3f6d34fe2732` | 通过 |

本机 Orca 1.4.242、runtime `92c410b8-3132-4c48-a07f-2888faa81c53`；远端 `192.168.1.151` Orca 1.4.245、environment `9cf377a1-42de-4f2b-8874-70f63a7bbab2`、runtime `9832f197-1f7a-4c76-b3e7-074fd12b30f0`。另一次旧模板 runtime 不符在准备围栏阻断，业务提交 0 次，现场关闭并移除，不计原生会话、不复用。

每场 Runner 通用业务 send **1 次**、text-only **0**、单独 Enter **0**、业务屏幕读取 **0**、请求不匹配 **0**。宿主内部按键/恢复次数不可观察，不从一次 Runner RPC 推断一次底层按键。本机每场逻辑 wire 5093 字节/201 LF，实际参数 201 CR/0 LF；远端 4694 字节/200 LF，实际参数 200 CR/0 LF。正式原输入均逐字等于原逻辑 wire，未用放宽规范化消除差异。

六场原回执input_accepted成立，初始业务Started=false；随后原正式输入/turn证明Started，并核验完整final nonce和完成归属。本机managed另核验实际模型/effort；远端只有冻结启动偏好，实际模型/effort未独立核验。本机还离线核验原退出签名、writer/reader absent、原认证撤销/broker关闭/socket清理及 control closed。远端保留原 NativeProof/query 与 `orca_terminal` 正式停止范围，不声称远端整机树或全局凭据清理。

停止后每场原 host 的完整 census 为空，紧接公开 worktree rm 后再逐一查询得到原 runtime 的 selector_not_found；本机路径另核实不存在。rm 的原异步回执为 removing=true，不能把它本身改写为同步 removed=true；最终目录不注册由后续只读查询证明。远端未独立查询物理路径不存在，不扩大该结论。本机 census 中 omitted 的其它环境不是被清理对象。

## 独立复审与证据边界

Spec/Plan 增量已由 reassessment_review 审计，未引入额外 ready/gate、等待或目标分支。final_review已独立核对冻结实现、六场、双源边界、990与修后7050及原字节归档，无未修P0–P2；最后完整L1及66文件/861原记录、695源文件对应关系已完成独立补充复审，审计门禁可放行。审计只读，不替本线程运行正式检查。

公开[证据目录](evidence/orca-native-submit-integrity-20261031/README.md)保存逐文件 SHA256、原字节归档、源/二进制身份、原始稳定事件、六场原调用/正式证据及阴性对照。只提取本次指定 session 的五条提交历史原行（最终本机三场＋两份阴性），不上传全局历史、认证秘密、EXE、安装包私有源码或无关工作区目录。

可靠提交的职责仍由 Orca 已发布通用 text+enter 入口承担，Runner 保证冻结完整正文、统一传输编译、原身份单次交付和未知不重复。既有正式原输入与结果核验防止错误归属，**不能阻止错误文本已经执行**；本次没有新增全文原子消费 ACK，也未证明任意未知草稿/人工写者的执行前排他。六场成功和受控回归不扩写为这些保证、历史 dirty 构建完整复现、所有窗口/字符条件或业务完整交付资格。
