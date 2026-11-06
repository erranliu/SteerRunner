# Orca 终端等待能力 B2d 验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：实现、定向回归及实施后独立审计通过。依据 [Spec](../spec/orca-terminal-wait-capability.spec.md) / [Plan](../spec/orca-terminal-wait-capability.plan.md)。本批不改变调用条件或正式业务资格。

## 1. 能力与唯一生产入口

protocol/orca/terminal_wait.go 使用自有 TerminalWait/TerminalWaitReceipt 和 Client.WaitTerminal，一次原 MetaCallWithTimeout 返回 Result、satisfied 及本地类型解码完整性。输入明确 handle/condition/非负 wire 毫秒，进程调用上限另由消费方提供；0ms 原样保留，无重试/回退或新期限。

唯一生产协议位置 native_start.go/waitOriginalComposer 已接入，原参数及 Wait 结构已删除。该函数在迁移前已经双端共用；本批没有新增链路接口。它仍解析原 profile WaitTimeout，传入 budget.Milliseconds() 和 budget+1s，并保持前后 original 查询、原 runtime/uncertainty/ErrorCode/satisfied 判据。

调用仍限定于现有 submit 的远端原生 Codex 与本机 usesControl 路径；不把其他组合改成等待。原就绪算法、queryCompletedOriginalStart、业务 Started/完成、profile/state/target、持久类型、正文、队列及停止均未改。原 wait 回执不提供本合同核验的 handle/incarnation，未补造该事实；未消费的可选扩展继续忽略类型，原 payload 保留。

## 2. 验证

源码基线 `39bc670` 加本批差异，source_digest 为 `e65ef919680d225513497d55401c680496b60597cab527edfea6a2528f22d135`，沿原 Get-SRSourceIdentity 范围。原始 test2json、实际命令及源码身份见[证据目录](../../../../acceptance/evidence/orca-terminal-wait-capability-20261102/README.md)。

| 验收 | 直接证据 |
| --- | --- |
| OTW-01 | TestOrcaTerminalWaitCapability 14 子例：完整 argv/调用上限、0ms、真/假/缺/null/坏类型、重复字段类型错误、无关扩展、原生拒绝/传输丢失/取消、缺输入零调用及 wire 不能注入解码 |
| OTW-02–03 | TestComposerWaitKeepsOriginalFences 8 子例：真实 NewLifecycle 和 IPC，原前后两次查询；前置替换零 wait，后置替换拒绝；runtime/负面/坏等待及取消；原 budget+1s 和 timeout-ms 精确匹配 |
| OTW-04 | 本机 bootstrap、远端 composer/Started、NativePrompt、未开始原输入、StateCodec 及准备期限回归，原 send 次数及就绪前零输入断言保持 |

新增 2 根/22 子例、原回归 14 根/103 子例、依赖守卫 1 根全部通过，0 fail/skip；git diff --check 及唯一旧 wait argv/结构零残留检查通过。测试仅使用原生字节、IPC 和原隔离进程夹具，未启动模型或操作真实 Orca 资源，未扩张 B5 资格。

## 3. 独立审计

对实现提交 `3fd71fd` 相对 `39bc670` 的独立只读审计通过，未发现确认的 P0/P1/P2。审计者复核唯一消费者、参数/0ms/预算/原围栏/调用条件、无 Started 升级与持久格式变化，以及全部 5 个证据文件 bytes/hash 和源码摘要。

新增检查、原定向回归、依赖守卫和固定提交 diff 检查均独立复跑通过。未启动模型或操作真实 Orca 资源，不扩大资格。无确认问题，因此无代码修复 commit。
