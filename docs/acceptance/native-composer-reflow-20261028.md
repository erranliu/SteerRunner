# 833 字符草稿呈现修复与正式发布输入对照

本报告续接 [原失败及前次六个前置失败](native-followup-input-20261028.md)。原现场仅只读，
不恢复 Run、不补 Enter、不重发未知输入；没有业务测试、真实发布、账号/全局设置变更。

## 已确认的呈现问题与修复

冻结首任务仍为原 833 字符、1083 UTF-8 字节，SHA256
`ddbdf515b767483e34347e17abf27d7b046856d2c6d2188f3517bfabf5ef53de`。
旧窄视口在零基偏移 668 把空格投影为 LF，草稿 SHA256 为
`4bcda39156b57b068cdd346bf2864f3d64be014cba9beb8fb644d59fd4554518`。

Orca 随包 `terminal-composer-draft` 以屏幕行及 wrapped 标记重建草稿。
Codex 编辑框自行折行时，显示续行可以被投影为 LF。首次正式诊断中，同一份原输入只粘贴一次：
临时扩大拥有 PTY 到 1024×80 后，两次独立草稿读取都恢复为原 SHA，随后一次提交并完成首任务；
恢复回执核对回原 177×83。输入内容未修改。

后续真实 903 字符发布提示又出现两处插入 LF 和一处空格→LF，窄视口投影为 905 字符。
首次修复仅识别等长歧义，因此严格拒绝、零 Enter；该失败原件及构建保留。
补充修复允许这两种歧义触发一次重新呈现，**原来的逐字/合法折叠块比较不变**。
扩大视口后真正的换行、任何其他正文差异仍拒绝；不做空白归一化。

只在新建且拥有的远端 Codex 终端使用现有 `terminal.resizeForClient`；调用前后绑定
runtime/terminal/incarnation/worktree，拒绝外部显示策略。唯一 owner ID 只恢复自身原尺寸，
恢复回执丢失也不能判成功。CLI/RPC 意图、结果及原文核验分别留证，视图回执不是输入接受回执。
完整原文唯一性、provider/main、接受/启动/最终回执保持原门禁。详见
[实现边界](../native-composer-reflow.md)。

## 固定构建与回归

最终冻结源码：`0fa92246b60bdafeae1a06ab15c53003b533947b`，构建时 source_dirty=false。

- 产品源码摘要：`22f3419f15ea87dd4d0013d1b34e7f7614a55a0acc2604b12bed14d41605af1f`。
- Runner EXE SHA256：`18372541a5f0cf20b86f60df8a546900d9fdf53bde6cb25451dc7463e6c9cd6e`。
- 独立只读验收器 SHA256：`582a9dab5a3c2f2245a35645c5dc852c7c0ce63c6260eb53ddd21e2e4b9c1af0`。

协议及 adapter 首轮完整回归 185 顶层 PASS、5 原 helper/opt-in skip，两个包正常 exit 0。
补充插入折行处理后，完整 adapter 回归 145 顶层 PASS、2 原 helper skip，正常 exit 0；
协议实现未再修改。反例覆盖真正换行、改字节、provider/terminal 身份变化、fit/restore 回执丢失，
以及原有提交后响应丢失不重发。另一次沙箱内回归因辅助进程 `Access is denied` 失败，原输出保留；
没有改测试断言或防护设置，以允许本机 IPC 的完整重验结果为准。

## 前置现场与计数边界

所有新现场均经实际 Runner EXE `service/submit`、development/2 正式入口。
固定环境 `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`、runtime
`878c2e5f-9602-4f7c-85a9-5239a6116f1b`、gpt-6-luna/xhigh、SDK 启动路径、access_mode、
40s transport 配置不变。与历史现场的已批准偏离保持原记录：新构建、新隔离资源和最小首任务，
共同无真实发布正文；短版只省略可见 Source facts，内部完整事实与严格核验不变。

1. `gate`：源码 2a75e44 / EXE 279ac4c3…bbf060e，首任务完成，903→905 投影使发布未提交。
   原失败及构建保留，完整清理，未计入最终同构建 3+3。
2. `gate-r2`：最终冻结 EXE；首任务完成标记中 `。` 被模型改成 `.`，严格契约触发
   implement.report。补报后发布被接受，但两输入验收明确拒绝该偏离；完整清理，不计入 3+3。
   早期派生的 valid_publication_sample 字段只检查实际发布粘贴，原字段保留；正式对照前已加入
   直接 implement.execute→publish.execute 条件，统一清单另行给出严格计数。
3. `gate-r3`：同一最终 EXE，原 833 首任务真实工具输出、契约满足，直接进入 publish.execute；
   903 字符完整唯一原输入、进程/查询和最终 JSON 标记获独立验收。五项清理全部核实。
   仅为前置资格验证，不计入 3+3。

正式对照只计首任务真实完成并直接进入 publish.execute、实际尝试发布粘贴的独立现场。
发布阶段失败/未知仍计失败样本，绝不重放；前置失败单列、完整清理，不占名额。
长短顺序为 L1、S1、S2、L2、L3、S3，使用同一冻结 EXE；每次清理后再启动下次。

## 额外前置失败与观察器边界

`comparison-r3` 在尚无有效发布样本时有三个连续前置失败，按上限停止：
完成标记不一致导致补报；Orca 在创建工作区时关闭连接；正式 submit 的响应超过 30 秒。
全部原失败保留，未计入后续有效配额。

创建响应丢失的 Run 为 `50048278ed6605e526c59ca890118ac9`。Runner 严格拒绝在缺失
原资源绑定时声明清理成功，原 `resources_settled=false`、stop pending 原样保留。
另行只读发现仅属于此 Run 的新工作区，核对唯一 instance、路径、分支及原始创建时间，
其中只有未启动 SDK 的空闲 PowerShell。核对 handle/incarnation/worktree 后一次关闭该 shell，
完整 native census=0（无截断/遗漏主机）后一次删除该工作区，只读确认 selector_not_found，
并确认本构建进程为零。独立资源清理证明与 Runner 原未知记录并列，不篡改数据库或原回执。

第三个前置 Run `03f148fd2cac243a31bd9371ae83314e` 的原 submit 实际已入库，
但观察器把调用超时作为直接终止条件，查到原 Run 后即停止，未等待其原请求继续。
该 Run 已停止，不恢复；准备在只读仓库查询阶段被取消/拒绝，Runner verify-stopped=true，
精确仓库清单证明未创建其原生工作区。

随后仅修正观察器：submit 返回未知时，以只读方式查持久化 commands 的原接受回执，
必须与原 request_id、完整原请求的 Go RequestDigest、command_id、run_id、当前 host instance
和原 Run 绑定逐项相符，才继续观察同一个原请求。改一个正文空格即拒绝；不重发 submit。
旧失败和旧批次保留；新 `comparison-r4` 使用同一冻结 Runner、同一 833 字符输入和相同执行配置。
这里的只读 admission 对账不代替 native 输入接受/启动/完成门禁。

## 最终正式对照结果

`comparison-r4` 完成六个有效发布输入样本，调度器正常 exit 0，最终独立证据一致性核对通过。

| 组别 | 有效现场 | 实际字符 / UTF-8 字节 | 独立原文与最终回执 | 完整清理 |
| --- | --- | --- | --- | --- |
| 长 L1 | 01-long-1 | 3427 / 3715 | 通过 | 通过 |
| 短 S1 | 04-short-1 | 903 / 1169 | 通过 | 通过 |
| 短 S2 | 06-short-2 | 903 / 1169 | 通过 | 通过 |
| 长 L2 | 07-long-2 | 3427 / 3715 | 通过 | 通过 |
| 长 L3 | 08-long-3 | 3427 / 3715 | 通过 | 通过 |
| 短 S3 | 10-short-3 | 903 / 1169 | 通过 | 通过 |

本批另有四个前置失败，不计有效样本：02-short-1 在首任务粘贴后原生状态流超时，
一次 paste、零 Enter；03-short-1、05-short-2、09-short-3 进入 implement.report 补报，
均无 publish.execute 输入。四次完整停止、resources_settled=true、完整 native census=0、
工作区删除确认；原失败全部保留，没有重发未知输入、补 Enter 或覆盖失败。

六个有效现场各自只有 implement.execute、publish.execute 两条工作流输入；首任务真实工具输出、
规范完成和直接发布分支均核实。每条输入各一次 paste、一次 Enter，无重发。
六个独立 provider 会话各有完整唯一的发布原输入，逐字匹配冻结实际正文，原进程/查询绑定和
最终 JSON 标记全部通过。六个首任务仍为原 833 字符及同一 SHA，宽视口原文至少两次精确稳定，
fit/restore 同一 owner，原尺寸恢复回执全部核实。

长短差异恰为可见 Source facts 标签及 JSON：2524 字符、2546 字节。内部完整 results、
attempts/fields/source、实施结果来源和渲染原文均从实际记录重建核对，其他请求条件保持一致
（新身份/序号及配对标记按计划单列）。调用方报告原始字节保持初始化原件，没有补填业务通过。
全批 Runner EXE、独立验收器、输入文件、基线配置、Orca EXE 与 app.asar 摘要保持不变。

本阶段合计 16 个新诊断现场（包括三个前置门禁、三个已中止批次前置现场及最终批次十个现场）
均已无所属活动资源；其中前述创建回执丢失 Run 的独立清理证明与 Runner 原未证实状态并列保留。
本次两个构建及验收器本地进程为零；原六文件、原数据库三文件和原 EXE 共十项 SHA 再核对未变。

本结果证明修复后的最小真实后续发布输入场景在两种长度各三次通过，**没有证明缩短提示词是
历史3427失败的原因或必要修复，也不证明实施时长是原因**。旧现场无法补造阶段回执，历史故障仍
不作单一因果归因。未运行 MagicPotion/Unity/SteerLine 业务测试；原连续两轮仍为 0/2，
本探针不计入两轮。未合并、部署或热换旧包；由 SteerLine 另行生成正式新包与新现场复验。

## 交付与可复核证据

实际验收的 EXE 位于 `.evidence/draft833-fix-20261028/build-r2/steerrunner.exe`，
相邻 `.build.json` 绑定上述源码及构建 SHA。后续提交仅修改观察器、独立审计与报告，
没有重建、热换或改变六次验收的 Runner 二进制。

- [构建身份](evidence/native-followup-input-20261028/reflow-build.json)
- [最终调度原结算](evidence/native-followup-input-20261028/reflow-comparison-summary.json)
- [独立完整核对](evidence/native-followup-input-20261028/reflow-independent-audit.json)
- [六条完整发布输入](evidence/native-followup-input-20261028/reflow-full-publication-inputs.json)
- [833 原文与重新呈现观测](evidence/native-followup-input-20261028/reflow-draft-observations.json)
- [前置门禁与中止批次原记录](evidence/native-followup-input-20261028/reflow-gates.json)
- [全部新现场清理及原异常状态](evidence/native-followup-input-20261028/reflow-cleanup.json)
- [回归和沙箱失败保留](evidence/native-followup-input-20261028/reflow-regressions.json)
- [原现场身份未变](evidence/native-followup-input-20261028/reflow-original-unchanged.json)
- [原报告归档与统一时间线索引](evidence/native-followup-input-20261028/reflow-raw-index.json)

完整原报告及 CLI/RPC stdout/stderr、本轮失败、数据库原记录、原生回执、统一时间线和清理过程
保存在 `.evidence/draft833-fix-20261028-raw.zip`；
[归档摘要](evidence/native-followup-input-20261028/reflow-archive.json) 记录 SHA256 与成员数。
本次归档为 2909 个成员、统一时间线 1191 条记录，SHA256
`8e0617561a39ed9f413dc4bba187d1ec2660db5e39bd38762972398e4ff2394d`；
[逐成员字节及摘要验收](evidence/native-followup-input-20261028/reflow-archive-verification.json) 全部通过。
统一时间线区分真实调用时间、producer 时间和无独立时间戳的记录快照时间，不推造缺失回执。
早先六个前置失败及其归档仍按原报告保留，没有覆盖。
