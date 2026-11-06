# Orca 历史读取与共享夹具：B 独立验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-06，合并前定向验证。依据 [Spec](../spec/orca-reader-fixture-boundary.spec.md)/[Plan](../spec/orca-reader-fixture-boundary.plan.md)，[A](orca-stop-contract-boundary.md) 在 main `6dd4360a` 上独立通过并保存源码归档后，才开始 B。B 开始前快进整合 main `458185030c2b43cb93045c546956a79a898cc9ab`（#126），保留新增 ProgressHost/profile 构造测试的实际消费者。

整合前后全部生产 AST 仅有上游 ProgressHost 的函数正文变化，原类型和 A 的五个实施/测试文件原字节均保持；B 基线编码与 A 相同。[上游独立对照](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-base-declaration-diff.json)。B 自身对照这一整合后的 A 点，不把 #126 算成本批收益或格式变化。

## 删除与保留

精确删除 Lifecycle 的 17 个候选方法：resourceState/state/stateVersion/envelope，三个 control 转发，三个 local Codex 转发，五个 receipt/owner 转发，以及 verifyStopped/codexReadiness。所有 frozenRecord 同名生产读取/校验保留。生产仅另改 original 的一处调用，由 l.state 改为 l.frozenRecord().state，仍走同一个验证；无生产函数新增、无类型/tag/codec 变化，其余 2,276 个声明相同。[B 声明差异](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-declaration-diff.json)。

共享测试装配变为：

| 职责 | 实际入口 |
| --- | --- |
| 中性身份与隔离 host | fixtureSetup，仅提供 setup/host/profile 数据 |
| 历史 codec/receipt/control/stop/文件和资格读取 | frozenFixture/frozenFixtureOn → newFrozenReader；单元测试继续修改同一 reader 的原 profile/probe/client |
| 当前 structured/workspace/准备能力 | structuredFixtureOn → 真实 NewLifecycle → 既有测试 seam；合法合成配对 host、完整派生字段，原 nativeAccess=false，隔离 CLI/agent port |
| 旧 State 编码 | frozenRecord.envelope 仅在 `_test.go`；不再有生产写入入口 |
| 原终端 RPC/并发、共享 CLI/creation | capabilityFixtureLifecycle，不能作为通用 reader 或当前执行基座；不放开生产旧协议拒绝 |

保留 original/boundTerminal 两个生产读取例外及原并发回归；另保留旧快照传入 ShutdownLifecycle 不得误报 stopped 的原拒绝断言，显式装配端口，不给 reader 增加执行能力。CLI Golden/creation 原回执、fakeHost/request/bind、owner/IPC 工具均保持。validate/inspectionGate/profileVersion/nativeAccess、当前结构化投影、真实只读 scope/VerifyStopped/usage/files、历史格式和原预算/停止/恢复实现未改。

## 格式与测试保留

对照原代码实际捕获的 Profile/State/NativeProof、Record/操作、新结构化状态及 RuntimeFence：B 的 15,528 字节与 A、整合后的 B 基线以及原 main 捕获全部一致。SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`。旧 encoder 的移出单列为源码删除，不伪称该函数仍在生产；格式/严格解码与实际持久读写不变。原字节和捕获/审计源见[原基线](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/baseline-format.json)、[B 捕获入口](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b_capture_test.go.txt)、[AST 工具](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/declarations.go.txt)。

整合后原 80 个 adapter 顶层测试全部保留，新增一项本机/远端单元夹具隔离测试；静态清单无删除测试或减少直接 t.Fatal/Error 断言。该清单只是补充，不能代替语义审计与实际运行。[保留核对](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-test-retention.json)。缺原摘要/错 owner/坏 receipt、旧 v1/v2、Conflict/Waiting、只读白名单、原终端并发、CLI Golden、#123/#126 构造/host 与 #125 消息/答案/停止窗口通过现有回归；不把局部 fixture 直接装配冒充生产入口验证。

| 范围 | 最终结果 |
| --- | --- |
| 完整 orcatransport 包 | 81 个顶层通过，0 失败/skip |
| application 同 A 定向组 | 75 个顶层通过，0 失败/skip |
| host/storage/contracts/usage/livetest 完整五包 | 118 个顶层通过，0 失败/skip |
| live native driver 和 helpers | 编译通过，未运行真实模型 |
| 同最终 EXE 公开受控复验 | 四项通过：ResourceCreationRecovery（七窗口）、SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization（四窗口） |

组件命令为 `go test ./internal/execution/adapters/orcatransport -count=1 -timeout=5m -json`、application 的明确停止/结构化/消息/重启/预算/封存正则，以及五组件包 `-count=1 -timeout=5m -json`。正式构建 `scripts/build.ps1 -OutFile .evidence/orca-stop-reader/b/steerrunner.exe`，公开入口为 `scripts/test-foundation.ps1 -RunnerExe <同文件> -Pattern '^(TestSRResourceCreationRecovery|TestSRSingleExecutionColdStop|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir <新目录>`。

[最终测试/资格明细](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-summary.json)；[构建](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-build.json) source_digest=`0a00ff6bac16ae9b5e2213a7e3dcc626f2258c39c44f92a6344b41f032c79871`，EXE SHA256=`d7a65cfe4e820c3219bf2e945c60b61320d05bcf4fb61bba795c3604b290b42c`，source_dirty=true。[公开复验身份](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-controlled-recheck-source.json)。源码/测试/工具摘要清单 SHA256=`d479d9f29bcfed51acc56b6b33aaedf154574171d86aa5d667434b24bb0e03b4`，实际源码、JSONL 和原日志位于本工作副本 `.evidence/orca-stop-reader/`。

## 初轮失败与修正

1. 首次编译暴露旧停止测试对 ShutdownLifecycle 的实际消费者和移出 fixture 后的未用 import。保留原拒绝断言、显式装配端口并清理 import，未给 reader 添加执行方法；定向通过。
2. 首轮完整 adapter 在当前工作区创建组失败：新夹具经真实构造默认启用 nativeAccess，进入原本未启用的 LocalBootstrap，触发 runtime binding changed。仅在既有单元测试 seam 恢复原 false 设置，增加隔离断言；生产默认/身份/host 校验未改，随后完整 81 项通过。
3. 首轮公开受控四项中三项及六个创建窗口通过；owner-crash 在解码 worktree-created.json 时失败。原测试忽略 os.ReadFile 错误，缺少首失败时的读取诊断；保留现场中的文件后来完整可读。生产写入使用原子替换，不能据此认定半写入、确定共享冲突或称本批已修复根因。没有修改该共享 fixture、生产代码或断言。用同一 EXE/原测试在全新 b-controlled-recheck 目录完整复验四项，报告 exit_code=0，七创建/四重启窗口均通过。原失败不追认为通过，见[首轮结果](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-controlled-first-result.json)和[原结论](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/b-controlled-first-outcome.log)。

正式基础脚本异步启动独立宿主；结果以其最终 result.json/integration.log 为准，不能以启动命令退出码判定通过。复验调用曾在结果生成前报告 Missing controlled result，随后只等待同一已启动宿主，未重复启动该轮；最终正式报告完整保存。

## 资格与结束范围

ORF-B01–B05 最小验收完成，A/B 各有源码及证据点。收益是当前 Lifecycle 少了 17 个旧方法，历史读取、当前执行和共享能力装配各有实际入口；测试辅助代码有所增加，不宣称全局净删或提速。两个原终端方法和明确的能力测试例外仍保留。

持久格式、执行/恢复/停止及原提交承诺保持。首轮公开 fixture 读取失败的根因未确立，单独保留为验证限制。未运行完整 L1/Controlled、完整 L2/L3、真实模型、Orca 真机关闭或合并后回归；未提交、推送、提 PR、合并、部署或发布，不继承全级资格。

同日用户追加审计后提 PR；[实现复审](orca-stop-reader-reaudit.md)确认 B 的生产闭包和保留消费者不变，只有两份停止合同测试修正/补充。完整 adapter 81、应用定向 76 及最终 EXE 四公开场景通过；原失败仍单列。初轮构建和资格不改写，最新源码/产物及提交边界以复审记录为准。
