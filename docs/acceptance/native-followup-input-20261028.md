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
