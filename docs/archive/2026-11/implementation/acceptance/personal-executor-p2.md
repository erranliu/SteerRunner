# 个人执行器 P2：观测解耦独立交付

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-01。依据[修订 Spec](../spec/personal-executor-simplification.spec.md)和[Plan P2](../spec/personal-executor-simplification.plan.md#4-p2通道故障不再阻塞业务)。补验已完成，P2 按 PES-A01/A02/A08 独立验收通过；原 Orca local 与 L1 缺口均已补齐。P3/P4 未推进；仍为 state/v6，未安装、部署或发布。

基线 HEAD 为 `e2729563562314659a799a5f82debe427fcd7c24`，保留已有 P1 未提交改动继续实施；本次不是 clean commit 构建。最终候选 `.evidence/personal-executor-p2-followup/bin/steerrunner.exe` SHA-256 为 `21983f9e930fafc50f6ffffc7c9ae8f0da4b52a4ad2f01f22b5eca06cd86186f`，source_digest 为 `d6d92844556eb881335d9c208663c3c689afe4fbbfedd7066c9f1c3af14b77a7`，已核实与当前源码匹配。当前证据见[补验 summary](../../../../acceptance/evidence/personal-executor-p2-20261101/followup/summary.json)及[清单](../../../../acceptance/evidence/personal-executor-p2-20261101/followup/inventory.json)；[原部分验收 summary](../../../../acceptance/evidence/personal-executor-p2-20261101/summary.json)保留为历史。摘要仅记录来源，未增加逐 Run 构建门禁。

## 2026-11-01 补验完成

用户追加授权补齐 Orca local 与 L1，P3/P4 继续不推进。以下为本次修复，前一轮部分验收的原始 summary/inventory 保留，不改写失败为通过。

| 缺口 | 已确认的原因 | 修复与保留的断言 |
| --- | --- | --- |
| 审计回执解码 | 默认 terminal.read 是终端画面尾部；一个 base64 回执被折为 120 列多行 | 测试审计器使用 cursor=0 的原始 stream，要求完整、未截断/未限量、有效游标；身份、原 helper、会话和工作区核验及独立 inspector 清理保持 |
| 原控制关闭证明 | cached Lifecycle 的 setup 尚无 workspace，关闭检查点却以它计算路径；最终读取器使用已准备 snapshot，计算出另一个文件名 | 使用原 control/readiness 工作区计算稳定键；只读兼容同一资源原 v6 空 workspace 的精确键；现有 canonical 文件无效时不回退，无目录扫描、重建或重发 |
| 原输入等待超时 | 原生历史中模型已完成，但 1196 字节输入少了头部后的一个换行，SHA 不匹配；本机 helper 先发 bootstrap READY，TUI composer 就绪检查此前只覆盖远端 | 受管本机 Codex 也在原 composer idle 后唯一派发；旧无 control 路径不扩大查询。原字节/完整输入/最终结果核验保持，未知不重发；新现场原输入已核实 1196 字节及 SHA 完全一致 |
| L1 手工 8m 超时 | 三个最慢测试隔离仍耗 53.40、36.24、61.54 秒，均成功；原栈在正常建库，未见死锁证据。此前手工总预算低于既有正式 12m 每包预算，且并行运行多例真机 | 使用原 test-prerequisites.ps1 -Stage L1、原 12m、完整 framework/Go/扩展/driver 守卫；未调高预算、删除断言或排除包 |

审计原流、稳定检查点/旧精确键、foreign canonical 拒绝以及本机 busy/unknown/lost-wait 零输入回归均已通过。首轮完整 L1 发现新就绪检查误扩到旧无 control 路径，已收窄并通过原 TestPreparationAndSubmissionNeverReadScreen；该首轮按失败诊断保存并主动取消，最终 L1 必须另起完整独立轮次。

首个健康真机补验 `C:\srt\9d36d8fb1756` / Run `bae32a6c7fac9c601d5dc568c8b4c67f` / PR 737：288.51 秒完整通过，原生审计、signed exit、control closed、完整零剩余终端清单和 result=1 均通过；该例是修复验证，最终冻结构建另做健康/故障两例。

最终候选与当前源码摘要一致；本次完成记录如下，原始证据与逐文件 SHA 见上述补验目录。

| 最终验收项 | 结果 |
| --- | --- |
| 正式 L1 | `test-prerequisites.ps1 -Stage L1` 完整通过，覆盖全部 37 包，725 个顶层 Go 测试通过；16 个既定 helper/显式 native opt-in 跳过不算原生通过，无异常跳过 |
| L1 其它门禁 | 26 个 framework 组、29 项 Pi/DSH 扩展测试通过；native driver 编译 exit=0，零匹配守卫通过，开始/结束输入 fingerprint 一致 |
| L1 时间 | application 包 658.12 秒，低于既有 720 秒预算；L1 core 总计 899.07 秒（含 framework 等），不将单包时间写成全轮时间；未增加预算、缩减包或拼接失败轮次 |
| Orca local 健康 | `C:\srt\31da6aa6d952` / Run `13d4af6be5180456a37d0765f3180156` / [PR 738](https://github.com/erranliu/SteerlineTest/pull/738)，271.35 秒；progress/result version=1、字节 p2 |
| Orca local 故障 | `C:\srt\2070edf4ce03` / Run `8b1716cca0ae89426d7e13c6acceab04` / [PR 739](https://github.com/erranliu/SteerlineTest/pull/739)，249.96 秒；通道目录故障仍正常发布/停止，result=0/closed/read_failed，无假空快照 |
| 真实原生证明 | 两例均完成原权限审计、原 signed exit、control.closed=true 和 complete_inventory=true/remaining_terminals=0；原 Run/服务均已停止，合并只发生在专用测试 base |
| 原始输入 | 两例各两次业务输入，1196/923 字节均与原生历史 SHA 完全一致；不降低字节匹配、不换资源、不补发输入 |
| 旧 v6 证明 | 两份原失败现场的关闭证明只读核实成功；未改原数据库、未重做关闭或输入。首轮审计/回合失败记录仍保留为失败，不追改为通过 |

实际最终 L1 命令为：

```powershell
./scripts/test-prerequisites.ps1 -Stage L1 -OutDir .evidence/personal-executor-p2-followup/l1-acceptance
```

最终两条 native case 都由 `new-test-fixture.ps1 -Scenario orca-local.smoke` 创建新现场，分别使用 `-ObservationFiles`、`-ObservationFault`，同一最终 Runner/helper/driver 与冻结配置，再由原 `test-native.ps1` 执行，原 case 的审计/PR/合并/停止断言保持。

本轮实际重跑 Orca local；前轮本机 Codex/Orca environment 健康及故障证据继续按其原构建记账，未冒充最终 EXE 的全后端矩阵。结合保持不变的 P2 通道逻辑、当前完整 L1、前轮公开 EXE 5/5 和本轮 Orca local 2/2，P2 独立验收完成；不授予完整 L2/L3 矩阵或安装/发布资格。P3/P4 保持未实施。

---
## 实施与收益

| 指标 | 原行为 | P2 结果与证据 |
| --- | --- | --- |
| 可选文件阻塞业务 | 初始化未确认使 advance 提前返回，并反复重进准备 | 关闭、仅 progress、仅 result、两者启用四组合均到达原 PR 里程碑并确认停止；目录、打开、写回执、保存和超时反例不制造 blocked/ResumeRequired |
| 初始化次数 | Initialized=false 可重复准备/等待 | 两通道先一次保存 InitializationAttempted，写入每通道最多一次；重复唤醒和重启均零补写；保存标记失败时文件 RPC 为 0 |
| 首次等待 | progress/result 分别准备，且可能反复付出等待 | 首次输入候选成立后共用 4 秒 context；无响应反例到下一业务动作实测 4.415 秒，包含 SQLite/派发校验开销；不能把它写成端到端严格 4 秒 |
| 后续等待 | 再次准备可能重新等待 | 同一超时反例下一次 advance 为 53.9 ms，无初始化重试，未再支付准备窗口 |
| stop 受理 | 文件 I/O 占 dispatch gate | 挂住文件 I/O 时 stop 实测 10.7 ms 受理；返回后不派发第二通道写入/业务后继，不发布迟到初始化事实 |
| 未确认初始化后的字节 | 等待种子一致；result 可能直接封存 initialization_unverified | progress 接受原绑定的实际稳定字节，result 在原结束资格后可直接封存这些字节；无补目录/补种子，已封存不刷新 |

这些是本批受控实测，不是跨机器吞吐或整轮开发耗时承诺。没有同条件旧版全流程耗时对照，不宣称固定百分比提速。

实现集中在 application/runtime_files.go；原 Next/QualifyAction 继续负责准备资格。首次业务输入候选成立后，释放 gate，记录唯一通道尝试，再在锁外执行文件 I/O；返回后复核 revision、原绑定、授权及 stop。未将共享业务准备逻辑搬给 adapter。progress 的物理读超时后最多保留一个在途读取，避免每轮另堆 goroutine。停止收尾的 result 读取同样释放 gate，仍受原停止期限和 2 秒采集预算限制。

沿用当前 Record、SaveProgress 和合并保存，文件更新不增加业务 revision 或业务事件；原业务持久化失败继续禁止派发。路径、环境变量、64 KiB/base64、原主机/runtime/worktree、O_EXCL、progress 独立版本和 result 的 version=0/1 保持。Orca Git ignore 前缀改为尽力处理，旧冻结启动命令仍能精确识别。

## 首轮定向验收（历史）

最终组件命令：

```powershell
go test ./internal/application -run '^TestRuntimeFiles|^TestProgress|^TestResultFile|^TestSRStopSerializes|^TestSROperation|^Test.*Preparation' -count=1 -timeout=4m -json
```

34 个顶层测试、72 个测试项通过，67.894 秒。覆盖四启用组合、目录/打开失败、初始化失败及丢失回复、标记保存失败、总窗口超时、重复唤醒/重启零补写、Agent 改写种子、原绑定只读恢复、丢失标记但已有业务意图后的重启、最后快照/独立版本/合并保存、一次封存、stop 竞争和原业务意图提交失败。启动包装/resources 包专项也通过；其余内部包在下面全量尝试中分别记账。

公开 EXE 定向命令：

```powershell
./scripts/test-foundation.ps1 -Pattern '^(TestSRProgressFile|TestSRResultFile|TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -RunnerExe .evidence/personal-executor-p2/steerrunner-final.exe -OutDir .evidence/personal-executor-p2/controlled-final -TimeoutMinutes 10
```

5/5 顶层场景通过、无跳过。新增四组合目录故障完整经过原结果资格回退、发布、pr_ready、停止及失败 result 封存；各 Run 都是原 fixture 预期的 3 次业务输入，无通道诱发的额外输入。到 pr_ready 分别为 17.56、19.45、22.53、25.30 秒；各次明确停止均通过。原 progress/result、Workflow、unknown/stop、正常/强制重启、创建回执及待停止恢复回归通过。

`steerrunner-final.exe` 用于健康真机和公开 EXE 回归；之后仅补充了“初始化标记保存失败且业务已派发后，重启可只读原文件”的 progress 采样准入，交付 EXE 包含该修正。该分支通过最终组件反例，交付 EXE 的真实故障用例另列，不把不同构建混写为一轮完整资格。

补跑 `go test ./internal/... -count=1 -timeout=8m -json`：31 个包通过，application 包到 8 分钟总超时中断；未取得完整 L1 通过，不能把这轮称为全量成功。最终受影响 application 定向回归另行通过。P2 无需等待 P3/P4 或完整后端矩阵，但此结果也不授予完整 L1/L2/L3/发布资格。

## 首轮真实路径（历史）

使用原 `development-smoke` driver，不删原生审计、PR、合并、超时或停止断言。新增 fixture 参数只作用于专用测试现场：`-ObservationFiles` 启用两通道，初始字节 `p2`；`-ObservationFault` 在专用 base 提交两个普通文件阻止通道目录创建，两者互斥。实际源码、Runner/driver/manifest 身份及过程日志均保留。

| 路径 | 健康现场 | 交付构建故障现场 | 当前结论 |
| --- | --- | --- | --- |
| 本机 Codex | `C:\srt\765f4c382028`，Run `5b280a3ac70fb529f4d2ed962305fc44`，PR 732，134.87 s | `C:\srt\3dc815ba5d49`，Run `39797b69ae7f01c53df452543ff5f89b`，PR 734，138.05 s | 两例通过；健康 result=1/字节 p2，故障 result=0/诊断，Run/服务均停止 |
| Orca local | `C:\srt\de83bbe9f761`，Run `de33b4013dd118bbf48d10b6f8295b2f` | `C:\srt\2615416a3eda`，Run `2ca1796afd003f9cbbe346aeb7d6bd86` | 健康 progress/result 均为 version=1/字节 p2，但原回合证据等待 12 分钟超时；故障例到 PR 736，停止后 result=0。两例均缺原控制关闭证明，故障例另有审计回执解析失败；整例均未通过，Run/服务已停止 |
| Orca environment | `C:\srt\fa639a16de91`，Run `9276ba3a6a7afab3a6d99f8d33217cc6`，PR 733，137.98 s | `C:\srt\053cc5cc17cd`，Run `a11de8cb7337daa53b34d3743fcbe275`，PR 735，150.88 s | 两例通过；健康 result=1/字节 p2，故障 result=0/诊断，Run/服务均停止 |

远端环境为 `9cf377a1-42de-4f2b-8874-70f63a7bbab2`，本次发现 runtime 为 `c7911c80-1c69-42bb-ab2e-db16cf81e02f`；使用远端既有 Codex/Git/gh 登录，零远端 helper 部署。所有 PR 只合入本次测试专用的 `codex/srtest/<fixture>/base`，未改业务分支。

## 首轮失败轮次和收尾（历史）

保留失败而不改写历史证据：首轮远端到达 PR 728 后，测试审计仍假定通道禁用而拒绝完整启动命令；已改为精确核验两通道固定包装，新现场 PR 730 通过。首轮 Orca local 到达 PR 729，但原生审计回执解析与辅助控制终端关闭证明失败；该例不能记作通过，独立离线停止核实已确认 resources_settled=true。首轮本机故障 PR 731 通过。

新增受控测试首轮把原 fixture 的 3 次输入误写为 2 次，四组合均已到达 pr_ready，断言失败；修正为原结果资格回退的真实次数，并加入失败时也执行的精确 Run 清理。四个保留现场均已重新按原 Run ID 停止，离线 resources_settled=true，随后最终公开入口 5/5 通过。没有放宽生产结果校验或跳过停止。

上述失败轮次、全量超时和真实路径缺口不作为通过；不触发 P3 恢复删除、v7、人工 resume 改造或 P4 后端职责收口。

Orca local 故障例的审计输出中，SR_REMOTE_AUDIT 回执在终端被换行为 120 字符的多行，而现有测试审计器按单行解码；另有原控制关闭证明未取得。只记录已观察到的证据，不据此声称两个缺口均由换行造成，未扩大本批去重写原生审计或控制机制。

最终 Orca local 健康例在原 waitReady 的 12 分钟期限后失败（整例 760.28 秒），没有换资源或重发输入。停止后 result 成功封存 p2；故障例整例 314.13 秒，到 PR 736 后因审计失败。两例 Run/服务均停止，独立离线结算证明另存；这证明文件接线和原业务整例资格须分别记账。PES-A01/A02 的受控及文件机制证据通过，PES-A08 的 Orca local 完整原生用例仍有缺口，故本批按部分验收交付。

上述首轮缺口已由本文件“补验完成”中的独立轮次补齐；历史失败不追改，当前交付结论为 P2 验收通过。
