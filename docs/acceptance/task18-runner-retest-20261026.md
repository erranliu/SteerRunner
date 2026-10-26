# Task18 原远端 Runner 链路复验补录

2026-10-26。结果分开登记：**原 Runner 执行、结果归属、真实发布及资源清理连续两轮通过；调用方测试进度显示未连续通过。** 本记录不接受完整 Task18 MagicPotion 功能，不认定干净发布提交已完成同等测试，也不把本次结果移记为完整本机 L2 或独立 Orca 等待场景资格。

修复 PR [#58](https://github.com/erranliu/SteerRunner/pull/58) 已于 2026-10-26 10:46:31（Asia/Shanghai）合并，merge commit 为 `84c942cd100840ad89b1c730a2d2cceeac671d36`。这是本次补录时发现的外部状态变化，并非本复验执行的合并。用户随后明确选择“另开仅文档补录 PR”；本补录基于 `74d594c5f00e32e97d2ee911db5ac787302022bc`，该文档基线不冒充受测产品提交。

## 固定身份与用户批准的环境例外

| 对象 | 实际身份 |
| --- | --- |
| Runner 产品与正式 EXE | 干净 `f3dd4ec020134e1eb56625ddd670641a33229151`；SHA256 `4a822c99fec734f296890b8d71a4a4013dfef46e3ab4c1395573130025a4e16e`；`0.2.0-dev.3-r2` |
| Runner resource digest | `635c09b807d2a10e484710042c53541e1355dbe8221abde4b600c597509811cd` |
| 干净测试 harness | `58dd5ba44460633ef4b98e84eb4d7437221b788d`；相比 f3 仅三个测试文件和验收文档变化，产品代码相同；不是 EXE 来源 |
| Caller / Web 来源 | `57623186b63c16be2a5748665e63d52ae615adbf`；独立包只提供 Caller/Web，未运行其模板业务 |
| Task18 原 base / 合同摘要 | `afe67b539e787fe05ce58876d612a6edb45e25f7` / `51da15701f56ee129ebf21624ac258764ded1d69437bc6cd32b9e3008fdc5724` |
| 原生远端 Codex | `C:/sr-runtime/2ba0/01/native/codex.exe --no-daemon`，`0.159.2`，SHA256 `52f75c649bebb8001102a1dd129c1ea6d02b0940321e6d7e82ee0526753bd58a`；`gpt-6-luna / xhigh / full_access` |
| 独立环境 profile | `orca-remote-desktop64g-xhigh-unity-env`，digest `125fb06bd49ed2f13787b0cf56bd4624516bf27efd53bd0415bdab611b3eb3aa` |

用户先明确允许远程测试不等待本机 smoke，后答“允许新轮次环境绑定”。原 profile `orca-remote-desktop64g-xhigh` / digest `e6183fae9d42d27f50e9290c86c68e2738754e2c63e78253db3af14fd70e97c5` 和全部原配置保留，新增 profile 的 execution 唯一差异为透明环境启动器，不冒充原 digest。

启动器 `C:/sr-task18-unity-env/20261026-0200/task18-unity-environment-launcher.exe` 的 SHA256 为 `2706913a1cc05f99dc445d8c3edada02b658f1ad76e2207db721c712bed5d412`。它只向原 Codex 子进程注入 `UNITY_EDITOR_PATH=C:/Program Files/Unity 2022.3.62f2/Editor/Unity.exe`，每次先核实原 Codex 与 Unity 字节，保留 argv、console 和退出码，没有模型、传输、proof、状态、重试或重放实现。Unity ProductVersion 为 `2022.3.62f2_7670c08855a9`，SHA256 为 `316507f85aeaedf602966be741e0c2c69d12d44850610fcf3a4ac0a164316ba3`。

Windows argv round-trip、父环境不变、native `--version` 和无模型的原 app-server `command/exec` 实际子进程环境验证通过；这不是 Unity 业务测试。没有改变 User/Machine 环境、全局配置、防护、原 home/pin 或生产安装。首次探针遗漏原 full_access 的失败保留；恢复原权限后的探针通过，不以关闭防护或重建受拦文件绕过问题。

## 两轮原始结果

| 轮次 | 原 Run / Attempt | 实际测试及发布 | 结果 |
| --- | --- | --- | --- |
| 06 | `8b545e5b195c42bd85fb72d20564afec` / `50a180c3b9a1ebedf7f9bd16ef5d7c4d` | L1 XML 11 Passed；L2 两例各 1 Passed、coverage 7/6；Bingo 单例 1 Passed、coverage 2；四结构 exit 0。[PR423](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/423)，head `e067ba86df0ec7ee77185b83a850e547be003248` | Runner 链路及原资源清理通过；页面 7/7 通过 |
| 07 | `596b316d8e877daed36279206054f05a` / `16cc9f19a54d37635c500df8b0057b1a` | L1 XML 11 Passed；L2 两例各 1 Passed、coverage 7/6；四结构 exit 0。[PR426](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/426)，head `dadf856da31e433042dda32dea8cb5c7665b3937` | Runner 链路及原资源清理通过，含用户要求的本机重启恢复；**进度显示未通过，页面仍为 0/6** |

两轮各自系统安排为七项和六项。06 额外选中 `activities.bingo.reward_purchase_restart_duplicate`；07 没有该项，不把它的缺席误记为执行跳过。两轮均保留 `l1.minimum`、`l2.smoke`、`structure.asmdef-governance`、`structure.asmdef-source`、`structure.resource-contract`、`structure.diff-check`，每个实际安排与原冻结 registry 逐字段相同，额外项不删减。

独立复核读取原 managed 定义、分支和时间、原 summary 字节数/SHA、实际退出码、真实 XML、冻结 plans、case receipts、唯一 RunRoot、原输入文件摘要及 cleanup。原 `Test-ArchitectureResults` 只读复核原报告，没有重新运行 Unity。XML 非零且无失败/skip，L2/Bingo 业务回执的 inputValidation/cleanup 为 Passed、externalRequests 为 0。两轮真实 Gitea API 的仓库、base `AIRefactor`、head 与原远端 HEAD 均独立一致；业务 PR 保留 open、未合并。

每轮 Caller 只有原来的一次 submit。06 request `69f162b58dca693520c86a7adffb9b05` / command `338a9d5f701895b9ab3f15f3e8af7ffa`；07 request `6ea745af6b7f15086369a4431e1bca99` / command `62209d01d85a5681b92d6cccb269d852`。早期未知回执只查同一原 request；07 曾在原派发在途时查询到 not_found，后同一 request 被接受，未据此重发。没有新增 Caller submit/reply/resume 来推进复验。

## 重启恢复与停止证明

用户确认本机重启，并要求“正好试试能不能恢复”。OS LastBootUpTime 为 2026-10-26 13:49:24.5（Asia/Shanghai）；原本机 Caller/Web/Runner 退出，远端原 terminal 仍运行，停机时资源未结算。随后只经原 home 的一次公开 `start --no-open` 恢复宿主，没有创建 home/轮次、重发任务或发送 resume。恢复后保留原 candidate、Run、Attempt、submit request、远端 runtime、terminal、incarnation 和 worktree，原 workflow 自动从实现继续到发布。

07 本机 Runner service instance 从 `b9c61d791da1283ac5c33c619163fad6` 换为 `70ecd2b2e2cb8826ff4a734a4c642267`；正式 EXE 与配置 pin 未变。该轮明确包含用户要求的重启恢复变体，不称全程无中断，也不据此推断所有服务故障情形通过。停机现象由用户说明和 OS 启动时间相互印证；没有将其误判为 Defender 阻断。

最终均按公开原 Run stop 受理，等待 stopped / new_inputs=false 后停止原 host，再核实公开 offline `resources_settled=true`、精确 owned-worktree native 完整清单为 0（truncated=false、omittedHosts 为空）和自有本地 home/bin 进程为 0。临时只读诊断 terminal 全部确认 close / ptyKilled。原工作区、未提交审计产物与 PR 保留；不声称撤销任意 detached 进程或外部登录。

## 测试进度显示的独立负例

06 保存的真实页面为计划测试“共 7 项、上次通过 7、未完成 0”，与原七项报告一致。07 的公开 `runs/<candidate>/tests` 返回 total=6、passed=0、pending=6、progress_version=1、collection_status=closed，diagnostic 为“进度字段与本次固定测试安排不一致”，六行均 pending；页面同样显示 0/6，最后观察时间仍为原初始样本。

07 执行器自己的验收记录明确说明未更新进度文件，理由是仓库没有共同写入脚本；这不使已约定的上报义务消失。原测试通过、进度上报完成和 UI 呈现正确是不同结论。当前证据不足以把根因限定为写入器、字段/schema 映射、采集或渲染中的某一层；**不能声明测试进度功能连续两轮通过**。本复验没有补写原进度字段、注入通过证据或重跑原业务来制造显示通过。

## 资格修正与保留边界

- 05 只执行四个结构检查，缺目标指南必跑 Unity L1/L2。安排和旧启动 guard 均未拦住遗漏，已明确向用户认领并从完整轮次计数中剔除；原四项链路和页面 4/4 的历史事实保留。未来新轮门禁强制 L1/L2 + 四结构、原定义深比较和额外项保留，八个无模型反例/正例通过。
- 原失败 01–04、05 的资格修正、各轮编译/发现/输入变动导致的 Incomplete 均不改记成功。没有将旧 PR418/419/420/421/422 混入本次 06/07 连续数。
- 所有本轮测试在发布前各自冻结的 dirty 实现输入上执行。06 的 L1 与 L2/Bingo codeIdentity 不同；07 的 L1 与 L2 也不同，发布后当前重建 identity 又不同。原批次输入和结果复核通过不等于所有检查测试了同一最终代码，更不等于 clean PR-head 资格。原 worktree 长路径删除现象按 `git -c core.longpaths=true` 读取，不据默认 Git 的伪删除清理审计文件。
- Task18 的完整 MagicPotion 专项、PlayMode/真实生产接入及人工验收仍待验，`task18_business_accepted=false`、`published_head_tests_verified=false`。本轮 L1/L2 和 Bingo/结构证据不替代这些条件。
- 本机正式完整 L2 仍失败。原06/07 smoke 的 HTTP429 / native retry exhaustion 保留；用户允许改为 gpt-6-luna/low 的本机08完成三个原生回合后 publication unknown_external、无核实 PR，不能独立归因 GitHub 凭据故障或模型专属限流。其 L1 / 24 Controlled / result acquisition 通过，相关现场均已公开停止及结算；没有启动本机09、换账户或继续循环模型探针。
- 本次自然 Task18 等待没有证明超过旧 15 秒。旧独立 SLW-A06 的 17.514683 秒持锁证据属于另一 scope，不冒充本次自然竞争，也不把重启离线时长算作 Orca CLI 排队。

## 证据定位与复核入口

可审查摘要见[机器记录](evidence/task18-runner-retest-20261026/summary.json)。原始长期 scope 为 `C:/SteerlineDebug/wt-507e-task18-retest`，不把大体积原日志或模型输入提交 Git。

- 06：`round-06/checks-verified.json`、`chain-result.json`、`verification-20261026T0449-reports-02`、`verification-20261026T0452-inputs`、`cleanup-20261026T0500`。原报告传输摘要为 `051d49a27a643e134bd44a41d1fc035c480f4c20ebee8e7eca66ff3582ba4847`，输入复核摘要为 `17da01c9546140e3bc9d3c8bf6b96caa82f1a0ef1798279d826afcdd38a3c72e`。
- 07：`round-07/checks-verified.json`、`chain-result.json`、`verification-20261026T0616-reports`、`verification-20261026T0620-inputs`、`verification-20261026T0623-progress-public`、`cleanup-20261026T0630`。原报告传输摘要为 `0ab0444c5db61952834c057a214c613ae817b471ee0b0a0e1634f430244b5753`，输入复核摘要为 `a714da0f0ac38384f858309905f391173639799f00b9be50c647add77d303fbf`。
- 07 重启：`diagnostics/host-exit-20261026T0555`、`diagnostics/reboot-recovery-20261026T0600/recovery-verified.json`（均在 round-07 下）。初始配置许可、独立 profile 及启动器资格见 scope 根的 admission 文件和 `diagnostics/unity-env-*`。

观察使用原公开 Caller snapshot / tests / activity / last-message 和 Runner CLI；停止使用原 Run stop、原 host stop、offline verify-stopped 与完整 native/local census。父会话只读复核原 Unity 报告，不向原 worker 追加业务输入，不读取或修改 Runner 私有数据库。
