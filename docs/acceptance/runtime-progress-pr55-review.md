# PR #55 质量审计与合入验收（2026-10-24）

审计范围为运行时单文件汇报实现、公开契约、Orca 启动环境、服务恢复、五项 L3 注册/driver 及结果核验。仅修改 Runner；保留原 R2、主线原生消息/恢复及客户端/Viewer 功能。原 [r1–r12](runtime-progress.md) 与 [r13/r14 整合记录](runtime-progress-integration.md)继续表示各自原构建结果。

## 已确认问题及修复

| 级别 | 问题与影响 | 修复与回归 |
| --- | --- | --- |
| P1 | 已完成最终采样的终态 Run 在服务重启后仍启动观察器，终态后源文件变化可改写保留快照 | terminal + 已持久化 closed 不重开；缺少收尾仍执行有界最终采样，显式恢复活动后可继续。旧 r14 EXE 的公开 Controlled 回归确实失败：版本 3 被改为 4；不是仅凭单元推断 |
| P2 | 终态与服务退出同时发生时可错误保存 detached，重启后产生不必要重采 | 最终状态按当前 Run 终态决定 closed，活动 Run 退出仍 detached；覆盖待收尾、活动及恢复反例 |
| P2 | 禁用 Orca 通道时可能继承父环境的 STEERLINE_PROGRESS_FILE，让工具拿到其它 Run 路径 | 禁用启动包装清除该变量、finally 恢复外层原值；不创建文件/目录或 Git ignore。实际 PowerShell 受控测试核对内外环境及非 Git 空目录 |
| P2 | 并行核验失败时，缺少窗口字段或通过身份列表为空触发 StrictMode，在写诊断前退出 | 无效窗口先记录并跳过后续读取，空身份经管道生成空列表；正式框架新增有效窗口、缺字段、资源冲突、无通过项、零选择五组断言，失败仍必须有结构化报告 |

公开 EXE Controlled 测试同时保留两个 Run：停止一方、修改已收尾源文件、再次换代服务；逐字段保持该快照，活动同伴更新到版本 5，且原业务输入数不增加。合成窗口仅验证核验器，不计原生资格。

## 原始失败与中止保留

修复前的[有效夹具单元复现](evidence/runtime-progress-20261023/pr55-review/pr55-audit-before-corrected-fixture.log)、[公开旧 EXE 回归](evidence/runtime-progress-20261023/pr55-review/pr55-audit-public-before/integration.log)、[旧并行核验器失败](evidence/runtime-progress-20261023/pr55-review/pr55-parallel-before.log)、[修复后针对性结果](evidence/runtime-progress-20261023/pr55-review/pr55-audit-final-targeted.log)与[五组核验反例](evidence/runtime-progress-20261023/pr55-review/pr55-parallel-final/result.json)分别归档。最初[新增单元夹具未设置 cancel 的 panic](evidence/runtime-progress-20261023/pr55-review/pr55-audit-before.log)单独保留，属于测试夹具错误；修正夹具后再取得产品复现，不能将该 panic 当产品根因。公开旧构建失败现场的两个 Run 均 stopped、服务退出、两个原 helper PID 均不存在，[清理证明](evidence/runtime-progress-20261023/pr55-review/pr55-audit-public-before/cleanup-proof.json)及原 Run records 保留。

[r15](evidence/runtime-progress-20261023/r15/summary.json) 在正式 L1 期间发现核验器空身份问题后中止；七项原生场景仍 queued，原 summary 的 running 和部分输出保持原字节。[中止原因](evidence/runtime-progress-20261023/r15/pr55-r15-interruption.json)与[退出证明](evidence/runtime-progress-20261023/r15/pr55-r15-exit-proof.json)单列；原控制进程与独立 L1 子进程均已退出，不推断被中断 L1 的通过结果。r16 使用全新现场从完整正式入口重跑，不复用 r15 资格。

## 正式执行命令

```powershell
./scripts/test.ps1 -Level L1,Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config .evidence/progress-test-config-r16.json -MaxParallel 7 -OutDir .evidence/progress-acceptance-r16
./scripts/verify-progress-parallel.ps1 -Report .evidence/progress-acceptance-r16
```

该入口自动执行正式前置；没有绕过 Controlled 或把受控远端计作原生通过。构建时 source_commit 为 `5020187b50990d39d69550c830de317abf93dbc6`、dirty=true，已包含本次审计修复；main 为 `9740bed6478809922a237b06988d483853250100`。验收冻结 [754 个实现、脚本、测试及夹具文件](evidence/runtime-progress-20261023/r16/build-source.json)，[结束复核零变化](evidence/runtime-progress-20261023/r16/source-immutability.json)。最后 fetch 确认 main 仍为同一提交。

Runner SHA256 为 `14bd2f0c4dedd1863bb08cf2cd84b1c355c73985c17ccc1bc868517c6c8135a2`，资源摘要为 `4c2e1395f863775023a1c0e83791cfb7d7e9c54ce2d0fd77f6cf8f893dfb7ebb`。Orca 本机 helper 按 `go build -o .evidence/progress-agent-r16.exe ./cmd/steerrunner-agent` 构建，配置冻结其实际路径和 SHA，未修改安装目录。

Native driver SHA256 为 `cb45f2cd2744c322049d347d2e929db5595dbfaf0674ceb45751ea2a08061b65`；helper 为 `4891b189305d663628568d0cf73accdeb2410f6b4f646494a0bbe2ad106584a1`，[构建信息](evidence/runtime-progress-20261023/r16/agent-build-info.txt)确认为 `cmd/steerrunner-agent`。

## r16 实际结果与清理

**全部通过**：[正式 summary](evidence/runtime-progress-20261023/r16/summary.json) 为 passed，7/7 实际执行、零授权排除，时间为北京时间 07:25:11–08:12:01。L1 651 顶层通过、无失败；16 个既有默认辅助/原生 opt-in skip 单列，不算真机通过。正式框架 22、终端扩展 29、driver 编译与零选择守卫通过。完整 Controlled 23/23、零 skip；新增终态保留公开 EXE 断言所在专项 52.35 秒通过，重启前后原生输入均为 2、业务 revision 均为 9。

| 场景 ID | 结果 | 准备至收尾 | 独立现场 |
| --- | --- | --- | --- |
| `local-codex.result-acquisition` | passed | 338.76 s | `C:/srt/51953062360f` |
| `local-codex.smoke` | passed | 316.14 s | `C:/srt/62b48019d10b` |
| `local-codex.progress-file` | passed | 233.30 s | `C:/srt/a9706dd57842` |
| `orca-local.progress-file` | passed | 583.41 s | `C:/srt/9e970fbbb65a` |
| `orca-remote.progress-file` | passed | 426.66 s | `C:/srt/5c8297bc851b` |
| `local-codex.progress-multirun` | passed | 245.86 s | `C:/srt/ad68e93dd130` |
| `local-codex.progress-recovery` | passed | 243.67 s | `C:/srt/15ad2eab933f` |

五项沿原目录注册，不减少用例、故障设施或观察预算；本机 Orca driver 实际 490.79 秒通过，准备和收尾另计入表中时长。真实远端文件位于原配对主机，本机无对应源文件；原子替换继续收到版本 3/4，native final 未核实不阻塞文件。五项各自独立观察至少 120 秒，均结束为健康，异常采样数为零。

[严格并行核验](evidence/runtime-progress-20261023/r16/progress-parallel-verification.json)通过：同一 Runner/driver/source，五项全部 passed，每项窗口均与另一通过项重叠；**4 对实际重叠、零资源冲突**。Orca 本机/远端重叠 **18.214 秒**，多 Run/恢复重叠 **117.530 秒**；不宣称五项同时重叠。

[逐 Run 清理证明](evidence/runtime-progress-20261023/r16/cleanup-summary.json)：五项 L3 的六个原 Run 均 stopped，五个服务退出，每个原 Run 的公开离线 `resources_settled=true`。两项 L2 也停止原 Run/服务，其测试 PR [#669](https://github.com/erranliu/SteerlineTest/pull/669) / [#670](https://github.com/erranliu/SteerlineTest/pull/670)已合入。保留所有原绑定、native 会话/CLI、公开响应、最终快照和初始终端 journal，不触碰用户保留现场。

原 PR/main 的 4,544 个独立证据文件及 4,333 个原始失败现场文件重新核对保持原字节。旧 r1–r14 证据不改写；新 r15、r16 与审计复现分别归档，各自 `files.json` 校验原字节。本轮原始执行和公开接口结果见 [r16 文件索引](evidence/runtime-progress-20261023/r16/files.json)，受控复现见[审计索引](evidence/runtime-progress-20261023/pr55-review/files.json)。

## 资格边界

本轮真实范围为配置中的本机 Codex、本机 Orca Codex 和一个原配对远端 Orca Codex；不据此扩展到其它平台、远端 Pi/DSH 或所有网络故障排列。停服竞态、未启用 Orca 环境隔离及核验器反例分别由 L1/Controlled 证明，不冒充额外真实 L3 场景。延续用户收敛范围：不承诺防御读取瞬间的恶意链接替换，不实现 Plan/测试语义/SteerLine 页面或接线，不删除旧 R2。原始通过、失败、中止各自保留，不把后续恢复查询改记为当时通过。
