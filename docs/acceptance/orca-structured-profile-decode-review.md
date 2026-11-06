# PR #123 合入前审计与修复

2026-11-06，审计 PR head `5c7c402a3a4c75dfe28d97802b1478846c1e7b35`，目标 main `5b5487b56715dfa3a2bbbd9d4c2d207b6647e555`。本记录为合并前定向复验；实际合入结果以 [PR #123](https://github.com/erranliu/SteerRunner/pull/123) 为准。关联[首次验收](orca-structured-profile-decode.md)、[Spec](../spec/orca-structured-profile-decode.spec.md)及[Plan](../spec/orca-structured-profile-decode.plan.md)。未修改 Orca 或 SteerLine，未部署。

最后合入前 main 新增 #124，已保留历史普通合并到隔离分支，再完成下方「最新主线组合补验」。最终资格以该组合为准，前两轮证据仅保留各自构建的事实。

## 发现及修复

1. 新增构造测试与 #121 的 `TestFrozenReaderHasNoExecutionAndRejectsMutation`、`TestRetiredExecutionFactoryKeepsFrozenProof` 均以 `json.Marshal(setup)` 前后比较且忽略错误。LifecycleSetup 含函数字段，编码失败，比较为空，setup 不变的断言无效。改用独立 `setup.Clone()` 加 `reflect.DeepEqual`，保留原共享 ProcessHost 语义。旧 fixture 的 Snapshot 先沿原 Clone 规范冻结，再捕获比较基线，避免 Clone 对 nil Operations 的规范化被误报为被测代码修改。修复仅涉及三份测试，生产逻辑未改。
2. 最终候选远端首个公开 result-file 查询返回合法 `pending/version=0`，原验收脚本直接索引 sha256，提前中断。原业务、停止及随后的原封存均正常。专项脚本增加最多 10 秒、每次 0.2 秒的只读查询等待，只有 closed/version=1 和正确内容摘要才通过；不延长生产 2 秒封存窗口、不重新采集、不修改共享历史脚本。首次 pending 及脚本中断原日志保留，没有改写为首次通过。

未发现运行逻辑缺陷。原生构造仍由 agent 检查、Clone、ProcessHost、原完整 profile 校验依次失败；NewLifecycle 在它们之前拒绝旧协议。解码器及其内部严格 JSON、封套/codec/归属/host/target/权限、Audited/model/effort 与退役字段校验未修改。五个派生字段均来自同一次完整结果；旧 terminal 分支与字段零值、probe/client/clock/nativeAccess 保持。newFrozenReader 原 list/show 白名单及所有执行拒绝、配对核对、队列和超时未改变，不因共享事实获得写权限。

## 测试与候选绑定

| 合并前命令 | 顶层/叶例通过 | 失败/跳过 | 包耗时 |
| --- | --- | --- | --- |
| 原生产代码 overlay，`go test ... -run '^TestStructuredConstructor\|^TestFrozenReader\|^TestRetiredExecutionFactory' -count=1 -json` | 5 / 35 | 0 / 0 | 2.036 秒 |
| 最终候选 `go test ./internal/execution/adapters/orcatransport -count=1 -json` | 75 / 282 | 0 / 0 | 57.698 秒 |

overlay 生产文件逐字节对应上述 main，新增构造夹具只去掉主线不存在的 frozen audited 字段比较，执行对象原 audited 比较不变；本轮同时运行修复后的两项 #121 测试。最初原样比较未规范冻结的 Snapshot 时，旧 fixture 的 nil Operations 与 Clone 空集合不同；仅修复 fixture 后取得上述最终结果，未放宽生产校验。摘要与日志 SHA 见 [tests.json](evidence/orca-structured-profile-decode-review-20261106/tests.json)，完整 JSONL 留本 worktree `.evidence/profile-decode-review/`。

从最终源码构建 writer（0.2.0-dev.4）和 reader（0.2.0-profile-review-reader）。两者 `source_digest=2bcf4f764f464d8ed225fc88a61302869ce915e8d66d16e7a8fed95afebabf3c`，source_commit 为受审 head、dirty=true，含三项测试修复；验收后重算一致。writer SHA256=`2cb6fcfe81c1dc531b49dab4a790751ba7bc8f34d06cbf14a10ac7d6c932269f`，reader SHA256=`a93ebc77f1e3a2e034fa43d29532387d36947580428cb16f687baf64ab01d821`。[构建记录](evidence/orca-structured-profile-decode-review-20261106/builds.json)。没有用首次候选源码摘要追认最终测试修复。

## 同一最终候选双端及原目录

| 目标 | 本轮新受理的原 Run | 原 prepare_workspace operation |
| --- | --- | --- |
| 本机 Codex execute/1 | `2eb1c6ac48ffdc89892f54a8e8066a53` | `8f97e61c9a7fed67b3701843ee881b47` |
| 配对远端 Codex execute/1 | `8f77ed9fffa1fd83d7df24637bcc7309` | `137f1b21ca4bc5be09a5438089918ee4` |

相同 writer、两端新隔离目录、新请求，先公开 validate，再 service start/submit/inspect。两端各只有原一次 start_turn、原一个会话，取得预期三行可信结果；原多行正文摘要、创建 owner 的 plan/dispatch/output 与 workspace/runtime/instance/selector/HEAD/base 绑定通过。停止为原 close 成功、范围仅 `orca_agent_session`；不证明额外进程清理。未知输入回执只读核实，没有补发。

本机直接通过 result-file/stop/verify-stopped。远端首次文件查询 pending 后，原服务正常退出，数据库已经保存 closed/version=1 的 rpc-seed 快照。为补公开读取证据，仅重启同一个原 writer，查询同一个已完成 Run 的 inspect/result-file，再停服与 verify-stopped。原 intents 数组与整个已封存 result_file 结构在重启前后完全相同，证明没有新增执行意图或刷新封存；未再 submit/reply/resume。原三行结果、停止范围、恢复撤销及正确文件 SHA 全部通过。首次中断见 [formal-live.log](evidence/orca-structured-profile-decode-review-20261106/formal-live.log)，补核对见 [file-recovery.log](evidence/orca-structured-profile-decode-review-20261106/file-recovery.log)与 [file-recovery.json](evidence/orca-structured-profile-decode-review-20261106/file-recovery.json)。该补核对沿同一个原 Run，不宣称新增远端业务通过。

最终两端 result-file 均为 steerrunner-result-file/v1、closed/version=1，内容为预置 rpc-seed，SHA256=`b0343319116e6aad957219dc9c62b4b04f8f96a5c91dc1513404a795566713a5`；未声称模型写入文件。原创建及最终状态见 [runs.jsonl](evidence/orca-structured-profile-decode-review-20261106/runs.jsonl)。保留[专项脚本](evidence/orca-structured-profile-decode-review-20261106/acceptance.py)与[绑定校验脚本](evidence/orca-structured-profile-decode-review-20261106/verify-creation.py)，它们仅用于验收。

原目录升级排空继续调用干净 SteerLine `6d12483213822db44727bdc146ecb2d181bfd326` 的原生产 VerifyUpgradeDrained，未改门禁或启动旧业务：

- 最终候选 writer/reader 对本轮双端原目录全部通过，案例 1.49 秒，[日志](evidence/orca-structured-profile-decode-review-20261106/upgrade-gate.log)。
- 首次验收的原 writer 与最终 reader 对首次双端原目录全部通过，案例 1.21 秒，[日志](evidence/orca-structured-profile-decode-review-20261106/original-upgrade-gate.log)。没有覆盖首次 proof，新增结果独立保存。

每个原目录仍为原同一个 Run/input，数据库与 shutdown.json 冷读前后摘要不变。专项用 `go test -overlay <对应 overlay> ./internal/runnerclient -run '^TestOrcaWorkspaceReceiptOriginalDirectories$' -count=1 -v` 调用；正常与首次原目录调用正文以 `.go.txt` 归档，不修改 SteerLine 源码。首次 EXE/完整测试日志摘要、原 DB/shutdown、原输入/回合/关闭及创建绑定也重新对照过原始文件，记录见 [original-evidence.json](evidence/orca-structured-profile-decode-review-20261106/original-evidence.json)。

资格仅为上述合并前受影响包、双端默认 execute/1 和四项原目录核验；未运行完整 L1/Controlled、完整 L2/L3 或 Pi/audited 真机。未修改旧执行拒绝、非 Orca Codex、原工作区归属、停止、封存与发布合同。测试与文档修复后再次逐行审计、gofmt/diff/链接检查通过，最终合入前刷新主线与实际文件合并状态；不以历史 CI 或首次证据代替最终候选复验。

## 最新主线组合补验（#124 后）

main=`22cc3751f1d618f315ebbda27595b06a0c79a544`，已验证的普通合并候选=`2dccec275634f3c880ea388c8c79187b9299ef79`。#124 清理旧证明计算器并增加状态 codec 保留测试；本 PR 相对该最新 main 的生产差异仍仅原两处构造及字段投影，完整 decoder、旧执行拒绝和冻结读取白名单未改，无内容冲突。未改写已验证分支历史。

最新 main 的相同夹具 overlay 对照 5 顶层/35 叶通过（4.822 秒）；组合候选完整 orcatransport 76 顶层/283 叶通过（58.696 秒），均零失败/零跳过。相较上一轮增加的是 #124 原有状态 codec 测试，不把计数变化误认为本 PR 删除断言。[测试记录](evidence/orca-structured-profile-decode-main-20261106/tests.json)。

组合构建 source_dirty=false，两个 EXE 的 `source_digest=e51b8daba3de4648f256640f21e7e01e0d5fc8cfe2607bafd2da5390e99f432c`，真机与排空后复算相同。writer SHA256=`eb2ce7a580205d9469c23c0ede495b26e441950b849b82eaff852ca110840920`，reader SHA256=`040ce1c90136ae8f4414b539737c07d56f6c0e824d346f6f58a2a8415c475209`。[构建记录](evidence/orca-structured-profile-decode-main-20261106/builds.json)。

相同组合 writer 再完成本机原 Run `20ff4af28efe41dd0e78b3d2187f63fe`、配对远端原 Run `8824b3eceb29bcd1a175736ca95cbcbc`。两端均为全新独立请求/目录，各一次已受理输入/会话及原关闭；可信三行结果、创建归属、恢复撤销、result-file/v1 closed/version=1 与 rpc-seed 摘要全部通过。本轮修复后的只读等待脚本正常结束，无重启或新阻断；不补发任何前轮请求。[状态日志](evidence/orca-structured-profile-decode-main-20261106/formal-live.log)、[Run 与原目录证据](evidence/orca-structured-profile-decode-main-20261106/runs.jsonl)。

原 SteerLine checkout 和生产升级门禁不变。组合 writer/reader 对本轮双端原目录通过（案例 1.43 秒，[日志](evidence/orca-structured-profile-decode-main-20261106/upgrade-gate.log)）；组合 reader 与首次原 writer 对首次双端原目录通过（案例 1.67 秒，[日志](evidence/orca-structured-profile-decode-main-20261106/original-upgrade-gate.log)及[摘要](evidence/orca-structured-profile-decode-main-20261106/original-upgrade.json)）。每个原目录 DB/shutdown 冷读前后不变。停止范围仍为原 Orca 会话，额外进程清理未核实；文件与发布门禁不放宽。后续只补文档及脱敏证据，不改变已测 cmd/internal/build 源码摘要。完整 L1/Controlled、完整 L2/L3、Pi/audited 真机及合入后测试均未执行。
