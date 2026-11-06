# Orca 停止合同与读取夹具：实现复审

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-06，基线 main `458185030c2b43cb93045c546956a79a898cc9ab`。按用户追加的“审计后提 PR”执行；承接 [A 独立验收](orca-stop-contract-boundary.md)、[B 独立验收](orca-reader-fixture-boundary.md)。此前报告中的未提交/未提 PR 为各自初轮状态，本轮授权提交、推送并创建 PR，不自动合并或部署。

## 审计结论与修正

未确认生产缺陷。A 的三个纯查询只解释原标记和诊断；Workflow、已存 scope 比较、具体 Orca 端口/已停止 guard、受理及提交顺序保留。合法的原 profile 没有已存 scope 时仍沿原入口处理，不新增严格解码门槛。严格 decoder/原回执仍承担证明，失败不会退化为 false 分类后报告成功。

B 的 17 个删除项无遗漏的生产消费者，original 的唯一生产调用改为直接使用同一个 frozenRecord.state；未新增生产 reader 副本或状态机。历史测试修改后的 profile/probe/client 仍到达原方法；当前夹具经真实 NewLifecycle 严格构造后在原测试 seam 隔离 bootstrap/CLI/agent port。原终端 RPC/并发、CLI Golden/creation、旧快照的 ShutdownLifecycle 拒绝断言、生产只读白名单及 #123/#125/#126 回归均保留。

确认并修复一处测试断言缺口：TestStopContractMarkersPreserveOriginalClassification 原先只浅拷贝 envelope，Payload 别名会使“未修改输入”的断言漏报。改用 bytes.Clone 独立复制 Payload，同时保留 nil/空切片区别。没有修改产品查询函数或放宽校验。

新增 TestStopContractOnlineAndOfflineKeepScopeAndDecoderOrder，经合法冻结记录和实际 application.port/VerifyStopped 验证：已存 scope 相等；原 profile 无已存 scope；scope 不一致拒绝且不缓存失败端口；坏 codec 的原解码错误先于 scope 错误，在线/离线各自原诊断不变；原记录保持不变。新增测试初跑因缺 execute 提示词、空 payload Clone 表示差异、在线/离线原诊断不相同及改 codec 后未重建封套摘要失败，均只修正新增夹具/断言；原准入、摘要和错误行为未改。原失败 JSONL 位于 `.evidence/orca-stop-reader/reaudit-targeted-first.jsonl`、`reaudit-targeted-second.jsonl`，最终定向通过。

## 最终复验与产物

相对 B 已验证源码清单，仅 application/stop_contract_test.go 和 orcatransport/stop_contract_test.go 改动；所有生产文件原字节相同。[生产声明复核](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/reaudit-declaration-diff.json)仍为 B 的 17 个删除项、一处 original 调用转接，全部格式类型/codec 及其余生产声明保持。

再次独立捕获持久代表编码，与原 main、A、B 的 15,528 字节完全一致，SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`。不迁移或回填记录；原执行、恢复、停止、预算和提交承诺不变。

| 本次执行 | 结果 |
| --- | --- |
| 审计新增/修正定向（adapter/application） | 5 个顶层通过 |
| 完整 orcatransport 包 | 81 个顶层通过，0 失败/skip |
| application 原停止/scope/结构化/消息/恢复/预算/封存定向 | 76 个顶层通过，0 失败/skip |
| 同最终 EXE 公开受控 | ResourceCreationRecovery 七窗口、SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 四窗口全部通过；4 场景，exit_code=0 |

组件命令沿 A/B 原定向入口，`-count=1`；adapter 时限 5 分钟、application 8 分钟。以 `scripts/build.ps1 -OutFile .evidence/orca-stop-reader/reaudit/steerrunner.exe` 正式构建，再经 `scripts/test-foundation.ps1 -RunnerExe <同文件> -Pattern '^(TestSRResourceCreationRecovery|TestSRSingleExecutionColdStop|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir .evidence/orca-stop-reader/reaudit-controlled` 执行。脚本异步，已核对最终 result.json，未将启动命令退出当作测试通过。

[测试/格式明细](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/reaudit-summary.json)、[构建](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/reaudit-build.json)、[公开宿主身份](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/reaudit-controlled-source.json)。最终 source_digest=`18f18053eb68cc3059db3b7196b0e4aaa7d8fd078435a4123a7d9fa6daf4dba3`，source_commit 为上述基线、source_dirty=true。EXE SHA256=`d7a65cfe4e820c3219bf2e945c60b61320d05bcf4fb61bba795c3604b290b42c`，与 B 的 EXE 相同；新摘要差异来自两份测试文件。构建后的生产/测试源码未再修改。

host/storage/contracts/usage/livetest 的 118 项以及 live driver/helpers 编译复用 B 的原结果：本次无相关生产修改，不虚记重复执行。B 首轮 owner-crash fixture 的读取失败根因仍未确立，原记录与同 EXE 复验均保留；本轮四场景通过不改写该历史失败。

没有完整 L1/Controlled、完整 L2/L3、真实模型、合并后或部署资格。A/B 仍为原承诺内的职责/维护面收口；没有扩大停止 scope、删除未知事实、重放原输入或赋予 reader 执行能力。
