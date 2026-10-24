# PR #55 主线整合验收（2026-10-24）

**后续质量审计及当前构建见 [PR #55 审计验收](runtime-progress-pr55-review.md)**：修复终态重采、停服状态、禁用 Orca 环境继承和失败报告问题；r16 正式前置、五项真实 L3 与实际并行全部通过。下文保留 r13/r14 原构建及其原始结果。

**r14 全部通过**：整合 `9740bed6478809922a237b06988d483853250100` 后，正式入口完成 L1 646、Controlled 23/23、真实 L2 2/2、五项真实 L3 5/5，以及严格实际并行验证。批次北京时间 06:05:44–06:46:54，MaxParallel=7。原 r1–r12 及本轮 r13 失败记录均保留。

本次仅修改 Runner。运行时通道契约、SteerLine 范围及 R2 接口保持原约定；接入见[说明](../runtime-progress.md)、[Spec](../spec/runtime-progress.spec.md)和[Plan](../spec/runtime-progress.plan.md)。

## 整合范围与构建

整合父提交为原 PR `93c6cc358f0ab13422f9de43a457d6171f290c77` 与 main `9740bed`。四处显式冲突的处理：

- `.gitattributes` 合并保留双方证据目录的原字节规则。
- README 保留主线新版结构与内容，补回运行时通道入口。
- `docs/testing.md` 同时保留通道与远端原生消息测试说明。
- `scripts/test.ps1` 保留双方相同的显式 opt-in skip 集，沿主线排版保留说明。

主线独有的 249 个变更文件在整合索引中原样保留，14 个双方修改的文件逐项复核。保留 Codex 最低版本及预发布版支持、Orca 最近消息/空 prompt/SessionStart 恢复、双 skill 客户端和独立 Viewer。没有修改 SteerLine 或用户保留 Run。

构建时 merge 尚未提交，原始身份如实为 source_commit=`93c6cc3`、dirty=true、merge_head=`9740bed`。验收冻结 [753 个实现/脚本/测试及夹具文件](evidence/runtime-progress-20261023/r14/build-source.json)，[复核零变化](evidence/runtime-progress-20261023/r14/source-immutability.json)。

- Runner SHA256：`1014c868c4e6865ec7412aae3151586ecea322cb7332e2ca217fd89895b15f3a`。
- 资源摘要：`4c2e1395f863775023a1c0e83791cfb7d7e9c54ce2d0fd77f6cf8f893dfb7ebb`。
- Native driver SHA256：`81d73959bf41940f8debfee8f4fb852f4adc8acfc4866f1b94f7f240f566d52c`。
- 本机 agent helper SHA256：`c3e56e5bd324340c5dbc608f66db775b37e79f7a2f81df7d2c453c2be81af259`；[Go 构建信息](evidence/runtime-progress-20261023/r14/agent-build-info.txt)核对为 `cmd/steerrunner-agent`。

## 正式结果与清理

[正式 summary](evidence/runtime-progress-20261023/r14/summary.json) 为 passed，实际执行 7/7。L1 646 顶层、框架 21、终端扩展 29、driver 编译及零选择守卫通过；16 个既有辅助/显式 opt-in 默认 skip 单列，不计真机通过。完整 Controlled 23 顶层零 skip，单文件专项 42.44 秒通过，原 R2 和七个资源创建恢复窗口全部通过。

| 场景 ID | 结果 | 准备至收尾时长 | 独立现场 |
| --- | --- | --- | --- |
| `local-codex.result-acquisition` | passed | 328.32 s | `C:/srt/82381395313f` |
| `local-codex.smoke` | passed | 272.32 s | `C:/srt/f60b1d9a1b89` |
| `local-codex.progress-file` | passed | 227.45 s | `C:/srt/0b31f1cbe35a` |
| `orca-local.progress-file` | passed | 512.95 s | `C:/srt/d3fec97d898e` |
| `orca-remote.progress-file` | passed | 381.93 s | `C:/srt/19ab28ead6d2` |
| `local-codex.progress-multirun` | passed | 235.34 s | `C:/srt/902bf618a9dc` |
| `local-codex.progress-recovery` | passed | 233.36 s | `C:/srt/dd658af20591` |

五项沿原目录注册，未减少用例、观察窗口、版本/去重、原子替换、身份/工作区、native final 独立性、服务重启零额外输入或清理断言。远端源文件在原配对主机，本机对应路径不存在；多 Run 核对同服务独立版本及业务 revision，恢复场景核对读取失败保留和原 session/turn/operation 保持。

[逐 Run 清理证明](evidence/runtime-progress-20261023/r14/cleanup-summary.json)：五项 L3 的六个原 Run 全部 stopped、五个服务退出，每个原 Run 的公开离线 `resources_settled=true`。保留源文件、原记录、native 会话/CLI、初始终端准备 journal 和终态快照。L2 创建并合并测试 PR [#667](https://github.com/erranliu/SteerlineTest/pull/667) / [#668](https://github.com/erranliu/SteerlineTest/pull/668)，各自 Run/服务也已停止。

## 实际并行与暂态采集

[v3 并行证明](evidence/runtime-progress-20261023/r14/progress-parallel-verification.json)通过：同一 Runner/driver/source，五项各自至少 120 秒独立原生观察窗口，每项与至少另一通过场景实际重叠，七对重叠、零资源冲突。Orca 本机/远端重叠 **48.541 秒**，多 Run/恢复 **117.812 秒**；不宣称五项窗口同时重叠。

远端窗口实际记录一次 `transport_unavailable`：[异常样本](evidence/runtime-progress-20261023/r14/orca-remote.progress-file/progress-005-collection-gap.json)与[下一健康样本](evidence/runtime-progress-20261023/r14/orca-remote.progress-file/progress-006-collection-healthy.json)相隔 **2.062 秒**，身份、版本 2、原字节、长度、摘要及观察时间完全保留；之后原子替换产生版本 3、4。窗口结束为健康，native final 未核实不阻塞采集。其它四项窗口异常采样数为零。

这次仅修正测试对暂态异常的处理。独立观察循环只允许 `transport_unavailable` / `read_failed`，逐次保留响应，同一异常最多 20 秒、结束前必须恢复；身份、时间、内容等变化，绑定异常、终态、持续故障或超时恢复仍失败。新增四项 L1 回归验证恢复与必须失败的反例。产品单轮采集仍为 2 秒，未扩大权限、重发业务或更换原资源。

主线补充回归在 r13 使用同 SHA 的整合 Runner 完成：客户端 32、独立 Viewer 9、隔离安装包 6、Viewer 单元测试，以及两项公开服务快照/来源隔离/重启测试均通过。结果保留于 [r13 补充汇总](evidence/runtime-progress-20261023/r13/supplemental-result.json)和[服务快照结果](evidence/runtime-progress-20261023/r13/display/result.json)，不将它们重复计入 r14 的正式场景数。

## r13 失败原记录

[r13 summary](evidence/runtime-progress-20261023/r13/summary.json)仍为 failed、6/7；L1 642、Controlled 23、两项 L2 与四项 L3 通过。远端原 Run `49347b012f074fc399e6ad2841c353b6` 在稳定等待中出现 `unavailable/transport_unavailable`，旧测试立即失败。下一次公开查询约 48 秒后已为 observing，版本、内容和时间未变；中间没有采样，不能推断底层原因、实际断连持续时间或满足新 20 秒期限。

原 Run/服务与资源结算已完成，[原始 native 失败](evidence/runtime-progress-20261023/r13/orca-remote.progress-file/native.log)、公开查询、原记录和[并行不合格结果](evidence/runtime-progress-20261023/r13/progress-parallel-verification.json)均保持原值。r14 用全新现场重跑完整前置与全部七项，不把 r13 的恢复查询改记为真机通过。

## 命令与证据保全

```powershell
go build -trimpath -o .evidence/progress-agent-r14.exe ./cmd/steerrunner-agent
./scripts/test.ps1 -Level L1,Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config .evidence/progress-test-config-r14.json -MaxParallel 7 -OutDir .evidence/progress-acceptance-r14
./scripts/verify-progress-parallel.ps1 -Report .evidence/progress-acceptance-r14
```

原配置与完整命令参数见 r14/frozen-config.json；重跑使用新的 helper/config、OutDir 和隔离现场。补充入口为 `scripts/test-client.ps1`、`scripts/test-client-package.ps1`、Viewer `go test ./... -count=1` 和 `scripts/test-foundation.ps1 -Pattern '^TestSRDisplaySnapshot(PublicService|SourceIsolationAndRestart)$'`，均使用任务独立路径。

[r13 files.json](evidence/runtime-progress-20261023/r13/files.json)和[r14 files.json](evidence/runtime-progress-20261023/r14/files.json)保留逐字节摘要。双方原证据的 Git blob 清单覆盖原 PR 4379 项与 main 3379 项；[复核](evidence/runtime-progress-20261023/r14/preserved-evidence.json)全部一致。[4333 个原始失败文件再次复核](evidence/runtime-progress-20261023/r14/original-failures-preserved.json)也全部一致。旧 `.evidence/` 原报告和保留现场未删除，没有重写旧失败/中止/手工清理结论。

仍未实测 progress 专项 Pi/DSH、full_access/audited 矩阵或管理员配对网络封锁。Orca 依赖既有配对与原主机登录，停止证明沿既有 orca_terminal 范围；恶意瞬时链接替换防护不在用户收敛范围内。其它边界沿[原验收](runtime-progress.md)。
