# 后续原生输入：证据、最小诊断与资格边界

本专项只运行 Runner 最小探针及受影响回归；没有运行 MagicPotion、Unity 或 SteerLine 业务测试。没有恢复旧 Run、补 Enter、重发未知输入、修改账号/全局环境、防护设置、定时任务或跨会话消息。没有使用子代理。

## 原失败保全

原根：`C:/SteerlineDebug/wt-507e-progress-0of6/entryfixed-01/`，仅只读。
源码 `ffb9c510280d470e064e48770b8eea30174d82c4`；实际 EXE SHA256
`a745ab738152e974850cbaca7851fb13c0d33cd08a915cda555f1ae465556347`。
Run `600d14e87929e5555c74675b56cde86a`，Attempt `219608c2f90445b83698824345efa5fe`，
发布 operation `bbe27895a8d0e57683d4201371a55d48`。

指定六份原文件复制到本次隔离报告根 `.evidence/subsequent-native-input-20261028/original`；
`original-index.json` 保存源路径、长度与 SHA，`audit-original.py` 独立重新读取源文件核对。
四页完整公开会话直接重新计算的 user 正文 SHA 与原清单一致：初始化 327 字节、实施 10552 字节，
没有第三条发布输入；同 provider/main，main done。原九项报告/进度/API/UI 均保留 9/9，
发布未通过、无本轮 PR、连续业务链计数仍为零。原完整停止/零 native/本地进程清理记录保留。

原活动 23:17:02 UTC 出现 CLI deadline/JSON 截断及 outcome_unknown，随后为
`Original input has no verified acceptance receipt; no input was resent`。
23:20 屏幕保留 `[Pasted Content 3427 chars]`。原材料不能区分哪一次 CLI 调用超时；
不能认定漏 Enter、长实施时间或本专项后来发现的零值问题就是原发布失败根因。

另从原服务记录定位到 `C:/slr/kc43sgn3blq4xpqd6s6g67r5ya`，仅复制 DB/WAL/SHM 作为取证副本，
在副本上以只读 SQLite 查询原 Run，源文件前后 SHA 不变，没有启动原服务。
原发布冻结正文为 3428 scalars/3768 bytes，SHA `d6e3f935342eee61a3fabd2ae8cb876b53d4e44d6bf603a5a1923fe6f4a58551`；
沿既有换行/末尾规范化后为 3427 scalars/3767 bytes，SHA
`8de9178bbd2945bc6c862785a532b244779d6eb411d1ea599026d2ff84f6f423`，与屏幕折叠块的计数相符。
因此不能仅根据逻辑正文的末尾换行推断原草稿计数丢失。保存状态仍为 attempted=true、accepted=false、
started=false、原 Run stopped/new_inputs_allowed=false。

## 已有测试审计与新增试验

PR66 的 long-input 是一次生产 Lifecycle paste/Enter，segmented-shape 是显式两次 paste 合成一输入。
它们使用 test EXE；均不包含先工具任务结束、再后续发布输入，也不证明实际 Runner EXE 的开发工作流。
原受控回执丢失、正文变化/重复、provider/pane/进程替换和旧 final 反例保留。

新增 `scripts/test-native-input.ps1` 三个 followup 模式：相同账号、Codex 0.159.2、
SDK launcher、`gpt-6-luna/xhigh/full_access` 和原 environment，使用全新独立工作区/终端。
本次 runtime 为 `878c2e5f-9602-4f7c-85a9-5239a6116f1b`，明确不同于已停止旧现场。
第二输入固定形态为中文 BMP、换行/空行、3427 scalars，仅要求唯一标记。

| 记录 | 实际结论与限制 |
| --- | --- |
| probe-a | 模式登记遗漏，在创建/投递前退出；不是原生失败复现 |
| probe-a-02 | 两条原输入/最终标记及清理通过；执行期间修改了测试源码，外层源码稳定性检查失败，不能标作最终源码资格 |
| probe-b | 额外明确等待 idle 后，两个原输入/最终标记、一次/轮 paste/Enter 和完整清理通过 |
| probe-c | 第一任务已提交，但只读 state 连接超时；未进入第二输入，失败及清理保留 |
| probe-c-02 | 驱动仅对同一请求的只读连接超时增加有界观察；第一任务请求八次 100 行输出，两个输入/最终标记和清理通过。与正式 CLI 诊断有执行时间重叠，不将其解释为纯单变量负载因果对照 |
| formal-cli-01 | 实际候选 EXE，第一任务 plain draft 多出屏幕折行，未授权 Enter；停服和补充 verify-stopped 通过。未进入后续发布，不扩大解释为本次原故障 |
| formal-cli-02 | 实际候选 EXE，第一输入全文与 final 独立相符，但明确 `incarnation:0` 未满足旧补充查询的 `>0` 条件，第一完成未被 Runner 确认，未发送第二输入。四分钟有界结束并完整停止结算 |

上述真实探针没有复现原“发布输入停草稿”的完整因果链。最初受限 Orca 查询返回
remote_runtime_unavailable；沙箱外只读查询成功，说明不能把该错误直接归因为远端离线。

## 已确认的 Runner 修正

新的正式现场暴露零值兼容缺陷。独立查询返回原文/最终回复相符，以及明确的
`origin=hook, authorityId=main-agent-hooks:..., incarnation=0, revision=54`。
本机 Orca 随包实现的 observation counter 初始化 `incarnationFloor=0`，首次观察直接返回该值；
源码读证据在 `local-orca-code-observation.txt`。这说明零值可以是明确的初始代次；不能与字段缺失混同。
这不是旧发布失败根因的证明。

用户明确批准只修正显式零值、保留所有严格核验。codec 区分显式 0 与 missing/null；
missing/null 经 JSON 往返仍不制造 0，负值不可信。完整原文唯一性、终端/PTY incarnation、
runtime/provider/pane、稳定 main、前后独立查询及接受/启动/完成凭据要求保留。
受控新回归在修复前只在显式零值正例失败，修复后通过；新双任务回归保留旧 final、正文变化、
仅草稿和提交后回执丢失反例，未知结果仍禁止重新提交。

另新增本地逐请求诊断：prepare/ready/draft/paste/pre-submit fence/submit，
operation/command/Run/Attempt/输入字节与 SHA，CLI 排队/开始/结束、退出码、成功 stderr、
stdout/stderr、原 deadline 与取消时刻、原始回执。日志写入失败会阻止尚未发生的调用；
已经发生的效果保持未知，不因此重试。身份或原始收据不由诊断日志制造。

## 正式 EXE 最小验收入口

`scripts/test-native-followup-cli.py` 仅调用实际 EXE 的 `service start`/`submit`，
使用真实 development/2 的 implement.execute → publish.execute。
第一任务执行无副作用输出命令；第二条发布形态输入只返回唯一 needs_input JSON 标记，
以满足未修改的生产工作流契约并阻止第三条补报输入，不真实发布或创建 PR。
`scripts/native-input-audit` 是独立只读进程，重新取得唯一全文原 user row，
前后核对进程/provider/main 及精确最终标记，不发送输入。

运行前必须构建并固定 EXE、profile JSON、clone/base 与 audit EXE；输出根必须不存在。
相同原请求结果未知时只读对账并停止，不能换 request 或补 Enter。
关闭 Run、停服、verify-stopped 及完整 native census 分别留证；新工作区仅在完整清单为零后移除。
失败现场不复用，原结果不覆盖。最终干净构建/正式入口验收及证据摘要在下方追加。

## 固定构建、独立验收及未通过的交付门禁

产品/测试源码提交 `c47974d040530233787ff3da7ab3c10b1e957952`，干净构建 `source_dirty=false`。
EXE SHA256 `5457a8cac5e653c0903025ae7577342ca4e1a7a283ba83475651259e1bcea0c2`，
产品源码摘要 `130fdc9022f2c504fdcd5957eb26c57748e697bee8dad598576fd6772a808abf`。
原文件位于本报告根 `build-final/steerrunner.exe`，未安装、合并或部署。
随后三个驱动修订仅修正诊断边界/冻结首回复/校准实际输入长度；没有修改该 EXE 的产品源码。

| 最终验证 | 结果 |
| --- | --- |
| 三个受影响完整 owner 包 | 189 个顶层测试通过、零失败；6 个原有 helper/opt-in skip 不计原生资格，skip allowlist 未改 |
| catalogue guards | expansion、冻结重载、兼容及冲突守卫通过 |
| 显式零值失败反例 | `zero-incarnation-before-fix.log` 中显式 0 失败；修复后显式 0/正值通过，missing/null/负值及其往返拒绝，双任务严格反例通过 |
| `formal-cli-final` | 两条生产输入完成，但驱动错误地把 publication needs_input 对应的合法 blocked 状态当异常，关闭前未执行独立审计；该外层结果仍失败 |
| `formal-cli-qualified` | 首 final 末尾为 ASCII 句号，未命中固定中文成功声明，转入 implement.report。952 字符补报草稿观测为 957 字符，差异是 5 个额外 LF；draft-settle 预算耗尽，没有 submit CLI 调用。未知结果没有重发。失败保留 |
| `formal-cli-structured` | 实际 EXE 的 implement.execute → publish.execute 完成；独立 native 查询、另一 Python 只读逐文件核验均通过。恰好两条生产输入，每条一个 paste/一个 Enter，工具输出已观测，精确唯一原文/进程/provider/main/最终标记及完整停止结算通过。实际发布输入为 6010 scalars/12018 bytes，SHA `ce9815c86e0e0ca877060105527444c24ecb6b72735ff1774316ef583862b445`，不是原 3427 字符形态资格 |
| `formal-cli-3427` | 按已测 Source facts 追加长度校准占位正文，但首 final 的 JSON 后又附 ASCII 句号的声明，进入补报并再次遇到 952→957 的可见草稿差异，未抵达目标发布输入；该结果失败，不统计为目标形态通过 |

测试/驱动的所有错误和首次失败保留。没有删除不利样本、把失败改记通过、归一化额外 LF 或宽松匹配
ASCII/中文句号；没有用更长输入通过推断原长度通过。原折叠草稿3427故障仍未定位到确切调用阶段；
本次新的 plain draft 可见差异也不能冒充原折叠块故障的因果证明。

**本场景的合格构建交付门禁仍未通过。** 上述 EXE 是已修正零值并具备逐阶段留证的诊断候选，
仅已通过明确记载的6000字符级正式双阶段最小路径。原长度的正式发布链及短可见草稿边界没有通过，
不能据此恢复 SteerLine 两轮正式业务复验。Runner 探针不计业务轮次，SteerLine 仍为0/2。
下一步需要能区分原文换行与可见草稿折行的只读证据，或新的可定位原折叠草稿失败证据；
不通过扩大正文/身份/回执接受条件绕过它。PR保持草稿，不自动合并或部署。

全部9个本次新建 Orca 工作区均在完整 native 清单为0、无截断/omitted hosts 后移除，
逐一 `worktree show` 返回 selector_not_found；所有新 Run/服务和本地 EXE 进程独立结算。
原现场六文件和原 store 文件的 SHA 再次核对未变。机器摘要、原始文件索引和完整清理记录见
[专项证据](evidence/native-followup-input-20261028/summary.json)，
原始完整日志根为 `.evidence/subsequent-native-input-20261028/`。

## 追加：实际800字符对照

按用户后续要求，沿用相同环境、账号、SDK launcher、模型和执行配置，在新工作区运行一次
`followup-immediate -FollowupScalars 800`。只把原生产 Lifecycle 双任务探针的第二输入从3427缩到800；
第一任务、中文/换行模板、正文及身份/回执核验不变。没有更改产品发送逻辑或原始失败现场。

**本次通过**：第一任务的真实工具输出已观测；同一 provider 的第二条公开原输入恰为800 scalars、
2206 UTF-8 bytes，SHA `acb26e2645edd157e1477a6511577ce50f4c1cea0107653b3cd8b6e664c8a391`。
原草稿自然显示完整明文，连续两次逐字相同并精确匹配；每个任务一次paste、一次Enter，
两个原任务均完成且唯一最终标记匹配。关闭后另一个只读进程恢复唯一完整user row，与冻结正文逐字相符；
该补充读取不制造关闭后的进程/完成回执，现场运行时的严格前后进程核验仍由原探针执行。

干净探针源码 `d48fb76ee7ff3a7b4c308eed72d1b829457c5876`，test EXE SHA256
`4933f5e634dfe55753e7f6d57c714cdef8bc713d373a8eed814a3dfef11aa5a5`。
47.22秒完成，无重发。完整native清单为0、无截断/omitted hosts，新工作区移除后
selector_not_found，本地进程0；累计本专项10个新工作区均已清理。

此项使用test EXE调用生产Lifecycle；正式publish.execute会自动追加Source facts，不能通过只缩override
使其实际正文低于1000，因此没有把该结果记作交付Runner EXE的完整发布链通过。
800明文成功也不证明原3427折叠草稿失败只由总长度导致；上方交付门禁及SteerLine 0/2不变。
[机器摘要及原文](evidence/native-followup-input-20261028/under1000-summary.json)，
原始完整日志根 `.evidence/native-followup-under1000-20261028/`。
