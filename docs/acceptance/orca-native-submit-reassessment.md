# Orca 提交问题重新核查

后续状态：本文保留当时调查及审计记录，实施方案以当前[Spec](../spec/orca-native-submit.spec.md)和[Plan](../spec/orca-native-submit.plan.md)为准。后续深审已撤回下文当时的“无P0–P2”结论；重试预算已退出方案，正常受理后不查mutation、不追加wait-submit。本机模型/长final等价设计尚未闭合，生产实施尚未开始。以下历史记录及原始证据不改写为通过。

2026-10-30，当前 Runner 生产源码基线仍为 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`，调查分支上一提交 `f57d1f1`。没有修改 Orca、Runner 生产代码、用户全局设置或历史 Run。用户要求重新区分实际 Orca 缺陷与 Runner 用法，不在本次调查中实施重试。

## 修正后的结论

**已证实 Runner 当前多行路径绕开 Orca 通用 prompt 提交能力；尚未证实历史输入问题是 Orca 违反既有契约的缺陷。** Orca 没有承诺 `accepted` 就等于模型开始。先前把“更强完整性目标未获证”直接归结为“必须先由 Orca 修复”过于笼统；主会话及独立审计还漏查了现成的原请求查询。此次明确纠正。

| 项目 | 重新核查事实 | 判断 |
| --- | --- | --- |
| Runner 多行提交 | `lifecycle.go:788` 选择 `submitNativePaste`，手动 bracketed text-only send 后再 bare Enter；两次都没有同一 durable prompt request | 确定的调用路径差异及 Runner 整改项；它主动接管了本可交给 Orca 的提交过程，不能凭此证明历史故障的唯一根因 |
| Orca 通用入口 | CLI `terminal-send.js:15` 只有同次 text+enter、非interrupt 才设 agentPrompt；做 capability preflight，带 durable request；host走 sendTerminalAgentPrompt | 推荐业务输入入口；正文由 Orca 做 framing/settle/submit，Runner不应保留手动框架后再套此入口 |
| accepted | 版本匹配 guide 明确默认 send 观察0秒；只有 input_accepted 是预期，warning明确它不等于failed或started | 返回成功但缺turn_started不能单独判Orca bug；必须继续观察原请求 |
| wait-submit | 观察同一原请求；超时可仍返回input_accepted | timeout不意味着未执行，不能改新ID重投或补Enter |
| 原请求查询 | 已有 `orca orchestration request-show --request <id> --json`，底层 orchestration.requestShow | 先前“没有只读原请求查询”的推断错误。它也适用于terminal.send durable mutation，不必新建API |
| completed | query查询mutation记录；completed表示调用回执保存，receipt可能只有input_accepted | 不是模型任务完成；任务完成仍走原输入/final证明链 |
| pending | 同进程可等待原在途请求；重启后未结算的terminal prompt重放返回operation_unknown | 既有实现防止重启后再投一次；不能把pending当失败未执行 |
| absent | 可能未到达、未记录或记录被清理 | 不授权发送。同ID去重仅对保留记录有效，不是永久保证 |
| 内部render timer/held CR | Orca公开warning承认可能Enter被吞；内部做有界恢复 | 这是实现和保证边界，不单独证明违约。原隔离恶意sink测试也不是实际provider故障复现 |

目前本机CLI、配对远端均能查询同一个全新、不存在的审计ID，返回absent及明确的“不证明无副作用”解释。未查询历史未知Run，未新建终端或投递模型任务。此实测只证明查询入口可用；尚未用新真实提交逐项验证retained terminal.send receipt及六个原生会话。

拆分路径原本就是为了补偿“合并paste/Enter后正文仍留在输入框”的历史问题，不能用今天的绕开行为倒推当时一定调用错误。缺原始dirty源码、完整原回执/警告和同版本现场复现，目前无法在Orca、ConPTY/TUI处理、Runner时序或显示投影之间认定唯一历史根因。

## 调用与恢复细节

正常业务提交形状是一次 `terminal send --terminal <原终端> --text <规范化正文> --enter --retry-request <原ID> --wait-submit <观察秒数> --json`。这是契约形状说明，不是在历史终端执行的命令。切换前仍须保留准备、身份、预算和停止规则；本次没有切换代码。

当前 raw paste 路径**没有双重框架**，因为text-only只原样发送。未来切到通用入口必须移除Runner手动ESC框架；否则Orca的buildAgentPromptPasteBytes会把嵌入ESC转成字面`<ESC>`再加框架，改变输入。Runner现有CRLF→LF和去尾CR/LF规则继续保留；不能把renderer的LF→CR规则误套到CLI，也不能假设通用入口会自动修好历史缺换行。

发送和查询都以authenticated caller fingerprint索引同一mutation表。本机共同使用持久local_authenticated_transport身份，远端要保持同一配对认证principal；换runtime/凭据/caller查询到absent不能推断任务没发生。query不需要当前terminal/worktree/from；只读原mutation，不重试/提交/结算它。整个宿主可能另做身份懒初始化或遥测，不能宣称整个Orca数据库绝对零写入。

retained记录绑定method、正文等params摘要和原进程binding摘要；waitSubmitMs不进入payload hash，便于对同请求继续观察。completed后重放可直接回原receipt，或只调用observeTerminalAgentPrompt补观察；同正文同ID不新投。换正文request_mismatch；换进程回执标incarnation_replaced。pending且无原在途执行者时拒绝重发。completed记录可因30天或10000行容量清理；absent下再次调用send可能被当新请求，所以**不能把任何丢回执都无条件套“同ID重试三次”**。

按用户选定的不重复执行边界，后续Runner应优先只读查询原请求，只有确切未产生输入的操作允许重新尝试；总预算及最多三次重试耗尽返回明确错误，但保留执行结果未知和原资源，不能改报“任务未执行”。该恢复可以利用现有Orca接口，不需要先改Orca；它也不自动补齐原始消费全文/排他强目标。

## 可复核证据

- [核查脚本](evidence/orca-submit-reassessment-20261030/check-usage.cjs)、[13项结果](evidence/orca-submit-reassessment-20261030/usage-check-result.json)：实际安装CLI handler、pure helper和抽取的host mutation runner；13执行、0失败、0跳过。
- [本机只读查询](evidence/orca-submit-reassessment-20261030/local-request-show.json)、[远端只读查询](evidence/orca-submit-reassessment-20261030/remote-request-show.json)：全新不存在ID，原响应保留；0模型输入、0新终端、0补Enter。
- 核查脚本分别确认raw split绕过durable路径、正文框架、默认不请求开始观察、旧host提前拒绝、accepted警告、completed重复三次仍1次effect、带wait只观察、pending重启不重投、正文/进程变化和删除记录后的去重边界。

脚本替换了RPC、DB和PTY依赖，没有启动Orca/模型。它是用法与恢复分支核验，**不是原生历史故障复现、新方案L3或真实模型提交稳定性证明**。故意清掉内存receipt的用例验证缺记录不能推安全，不声称真实运行中发生了该清理。旧240次隔离结果和所有失败文件原字节保留，不改写。

独立只读 `design_audit` 重新核查得到相同结论，并明确纠正其首轮漏查。新的独立只读 `reassessment_review` 核实安装bundle、脚本及五段函数偏移/摘要、两份查询响应原字节、13项隔离结果和Spec/Plan一致性，未发现P0–P2；其措辞建议已采纳，避免将调用路径差异写成历史根因。本次只做调查/文档修正，不重跑未变的生产L1，不冒充新的修复验收。
