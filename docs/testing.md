# SteerRunner L1–L4 测试指南

审计默认提示简化的定向入口与结果见[本轮验收](acceptance/reliable-delivery-simplification.md)。
原生能力对比使用显式选择的[隔离试验](../tests/experiments/README.md)；跨构建冻结验证使用
`scripts/test-foundation.ps1 -Pattern '^TestSRAuditPromptDefaultUpgrade$' -BaselineExe <变更前EXE> -OutDir <新目录>`。
后者是需原 EXE 的补充受控场景，不改默认 22 项目录或以编译/跳过代替资格。

Run token/独立费用增量入口：`go test ./internal/usage/... ./internal/cost/... ./internal/storage ./internal/host ./internal/cli ./internal/contracts ./internal/resources -count=1`。真实 EXE 的用量查询使用隔离合成投影；三 agent 格式夹具均脱敏。原生产日志只读复算为显式 opt-in `TestUsageExistingNativeEvidence`，默认跳过，不启动业务 worker。实际范围、六组合缺口及首次整仓失败记录见[实施验收](acceptance/run-usage-and-cost.md)；这些测试随现有 L1 `./internal/...` 入口运行，不增加模型或 PR case。

授权回收规则的定向回归还需包含 `./internal/execution/adapters/protocol/orca`：验证原有 repo/group/folder 清单的完整性与引用、读取 selector 撤销证明。远端公开验收在正常停服后同时检查持久 collection permit 已 inactive，不能只以 token 已取得或服务退出代替撤权证据。

2026-10-22 提交前复审新增最终回复 command 归属、冲突双方证据同事务/幂等、新 epoch 空资源意图、分页清单完整性及费用排除项反例；三个用量/费用查询沿最新 main 的完整实例绑定和停止准入围栏。实际 EXE 正例带完整绑定；六组合当前边界与重跑结果见同一验收记录。

本机 Codex 只读现场入口为 `TestRunUsageLocalCodexPublicLive`，显式设置 `SR_USAGE_LOCAL_LIVE=1`、`SR_USAGE_LOCAL_STATE`、`SR_USAGE_LOCAL_RUN`、全新绝对 `SR_USAGE_LOCAL_EVIDENCE`；只接受当前可解码且已停止/完成的原 Run，并拒绝仍有业务授权的来源。测试不会创建模型输入，只复制已冻结、已撤权记录到隔离服务；原日志保持只读。普通 L1 入口拒绝上述遗留 opt-in；独立 seed 清理入口的 `SR_USAGE_LOCAL_REPO_CLEANUP`/`SR_USAGE_LOCAL_CLEANUP_ROOT` 同样被拒绝。

本机 Pi/DSH 入口为 `TestRunUsageLocalPiPublicLive` / `TestRunUsageLocalDSHPublicLive`，复用同组 `SR_USAGE_LOCAL_*` 变量，以精确 `-run` 选择与原 Run agent 相符的测试。仅接受现存 pr_ready 主会话，以只读 `UsageRuns` 投影历史状态；隔离副本无业务授权/待执行意图。验证原生日志逐响应 command 归属、公开 EXE 自动统计、独立估价、采集服务停启/强制重读去重；不创建模型输入或目录登记，不停止原生产服务/终端。实际 token、合成价表及清理证据见同一验收记录。

远端已完成 Codex Run 的显式现场入口为 `TestUsageRemoteCodexLive` / `TestRunUsageRemotePublicLive`，所需变量及撤回/清理证明见同一验收记录，默认 skip，标准 L1 拒绝远端 opt-in 遗留。手工整仓 Go 入口应使用 `go test ./... -count=1 -timeout 45m -json`：2026-10-21 实测 application 全包 625.267 秒、`TestSRResultAcquisition` 串行整组 738.65 秒，均可能超过 Go 默认 package 10 分钟上限。该聚合预算不延长产品调用或场景内期限；完整受控入口本已采用 45 分钟预算。

裸 `go test ./...` 还会包含 L4 的 `TestSREdgeResourcePressure`（20 轮完整提交/发布/停止及进程退出/句柄检查），并非 L1 入口。常规分层运行使用下文脚本和 catalog；全仓审计保留此测试，不为缩短耗时削减轮数。

内容流程的最小定向入口为 `./scripts/test-content-workflows.ps1`；附加
`-NativeExecutable <已有原生 Codex 0.159.2 EXE> -Model <可用模型>` 后，在三个内容受控场景
通过后运行真实生成/多轮/中断/重启与结算。原生测试创建新的 `C:/srt/cw-*`，不使用
SteerLine 安装环境、Git 或 PR。此专属 L3 入口独立登记在 `tests/catalog.json.focused_native_scenarios`，
不改变现有开发 manifest driver；命令、构建摘要与实际结果见[专属验收](acceptance/content-workflows.md)。

测试源码、夹具、现场生成、执行和结果判定全部归属本仓；不要求 SteerLine
checkout、EXE、数据库、配置或测试调度器。运行目标为 Windows/amd64。

本文件说明分层定义、设计与维护规则、执行入口和结果判定。
逐项测试内容、预期结果、并行条件及当前缺口见独立的[测试清单](test-catalog.md)。

独立调用配置契约、本仓内部测试配置、本地 Codex 交付基线及按 case 判定 L3 可用性的增量
见 [Spec](spec/independent-configuration.spec.md) / [Plan](spec/independent-configuration.plan.md)。
当前使用 `steerrunner-test-config/v1`；本次构建的实际范围和缺配置项见[验收](acceptance/independent-configuration.md)。

Orca 观察恢复定向回归：`go test ./internal/execution/adapters/orcatransport ./internal/application -run 'TestOrcaNativeIncompleteMetadataRecovers|TestOrcaNativeMissingPromptCannotHideConflict|TestOrcaNativeFinalRetainsVerifiedInputBinding|TestOrcaObserverReportsChangingGapAndRecovery|TestOrcaCompletionQuery|TestOrcaStateCodec|TestTerminalNativeFinalRecoveryNeverReplaysInput|TestSRObservation' -count=1 -timeout 5m`。覆盖工具执行后 prompt 持续缺失仍保留已核实输入绑定、其它最终状态字段补齐、观察器重建、短回合无 working、冲突不可抹除、旧 codec 保守转换、诊断事务/幂等与恢复。使用受控原生响应及真实 SQLite 事务，不创建模型输入或真实 PR；通过不等于该现场历史锁死记录已经修复或新版已部署。

## 分层定义

按测试目的、流程和隔离条件分层，不以是否使用真机、模型或远端环境作为层级定义。

| 层级 | 定义 | 设计要求 |
| --- | --- | --- |
| L1 | 快速的单元测试或集成测试 | 快速反馈规则、组件、协议、存储和依赖边界的正确性；现有默认入口不启动真实模型或创建真实 PR |
| L2 | 代表最小核心流程的 Smoke test | 用最少的固定流程判断核心链路是否可用；扩展流程和边界变体放入 L3/L4 |
| L3 | 可并行的多种不同流程及核心可靠性测试 | 覆盖开发、反馈、审计、远端执行，以及可预期故障下的可靠执行；各项独立、互不干扰，主要验证目标尽量正交 |
| L4 | 更复杂的边界情况测试，或确实无法并行的测试 | 覆盖复合故障、精确竞态、极端资源条件等；记录超出常规可靠性流程的条件，或必须串行的原因及互斥范围 |

具体归类先看产品承诺和独有断言，再看故障组合及隔离条件。不能仅按故障数量、次数或
注入点命名判断：暂时不可达期间停止再重启、发布完成证据缺失后查询失败，虽然包含多个
事件，仍在验证核心保证，属于 L3。常规 stop/reply/resume 竞争、有限次重连和正常资源
范围内的多 Run 隔离也应优先放在 L3；只有超出这些基线的组合或极端条件才规划为 L4。

**Agent 可靠运行是 SteerRunner 的核心能力。** 调用方退出、单次断连、服务重启、停止
授权撤销、结果未知后的对账均属于 L3。故障注入本身不是归入 L4 的理由，case 名称中的
`boundary` 指实施回合与下一阶段之间的切换点，也不等于 L4 的边界情况。这些可靠性
测试保留在 L3，包括暂时不可达时停止、跨重启撤销和发布查询失败等核心保证。

L3 的正交性按主要验证目标判断：新增 workflow、交互路径、后端契约或可靠性性质时，
说明它补足的独立覆盖。不同可靠性测试可以复用同一个简单开发任务，独有断言分别核验
请求去重、连接恢复、服务换代授权、停止撤销、发布去重等性质，不必刻意更换业务任务。
只换模型、参数、目标机器或重复相似步骤，不能直接算作新的流程覆盖；平台差异须有独有
兼容性断言。服务换代的正常/异常退出与活动回合/阶段切换是不同恢复窗口，保留四个
变体；只有证实独有断言重复后才合并。共同的准备、产物核验、发布和收尾可以复用。

各测试必须隔离现场、state-dir、Run、工作区、分支、PR 和故障设施，不依赖其它测试的
结果或执行顺序。声明可并行须有资源隔离说明和实际并行验证；当前调度器串行执行、
需要管理员故障设施或尚未验证并行，都不作为把核心可靠性测试移入 L4 的理由，应先完善
隔离。确实无法隔离的独占环境测试归 L4，并明确互斥范围。

区分测试之间的并行与产品内部的并发：普通 case 之间使用独立服务/state-dir；验证
同一服务内多个 Run 时，一个 case 内应共享该服务/state-dir，各 Run 的请求、工作区、
会话、分支和 PR 独立。该 case 仍须与其它 case 隔离。独立服务的测试并行通过，不能替代
同一服务的多 Run 隔离验证；产品允许的资源排队也不能误判为要求所有 Run 同时执行。

Windows 的 Orca CLI 会刷新同一用户的环境配置文件，多个进程同时调用会产生
`orca-environments.json` 的 `EPERM rename`。Runner、真机诊断和 fixture 准备共用
规范化的 `%LOCALAPPDATA%/SteerRunner/orca-cli.lock` 路径摘要构成的 Windows 命名互斥，只串行 CLI 进程，远端
Agent 和 state stream 仍并行。等待占用原调用期限；取消后不启动命令，不重试未知发送。
业务 send 立即捕获受理回执，实际开始/完成继续由原 native history/state 证明，
不在持锁的 CLI 内等待模型回合。原始失败批次保留，不能用重跑改写首轮结果。
命名互斥按拥有线程持有，Go 调用期间固定该 OS 线程；取消通过独立事件结束等待。
持锁进程被杀后按 abandoned-owner 语义取得互斥，但必须重新核验原资源身份。
只有明确未调用 CLI 的排队错误才可重新观察原对象；未核实的创建/发送不据此重试。
只读回执查询在取得 CLI 互斥后、调用前持久化创建意图；排队中被杀不会留下模糊创建。
撤销 journal 先保存原停止意图，取得互斥后再标记实际创建尝试；恢复可执行尚未调用的
原意图。实际尝试已落盘但缺返回身份时仍保持未知，不补建辅助终端。
撤销失败只导出有限错误类别并持久保留；环境检查保留连接、原回合和 CLI 排队的
有限失败类别，不以统一未核实状态吞掉原因，也不发布原始输出、路径或凭据。
真机控制请求只有收到明确 `stale_revision` 拒绝后，才刷新原 Run 并用新请求 ID 提交；
未知或丢失回执仍保留原身份，不能重复授权。

`Controlled` 是现有脚本的受控集成前置入口，不是 L1–L4 之外的第五级。它使用真实
EXE/CLI/IPC/SQLite、隔离 Git 和原生协议/provider 夹具，可在本地隔离 push，不创建真实
PR。目前固定 22 个顶层场景，覆盖独立包、服务归属、材料冻结、权限资格、完整工作流、
存储/日志/事件、PR 身份、工作流完成前的发布事实、原生未知结果与停止、重启后显式恢复、
丢响应找回、停止授权跨重启保持、审计发现/修复/预算、同服务多 Run 隔离及资源创建回执恢复。
审计场景的子变体单独在日志中登记，批次 `scenario_count` 按顶层测试计数。
是否纳入快速 L1 应按耗时和依赖评估。

## 测试清单维护规则

L2/L3/L4 的具体内容统一维护在[测试清单](test-catalog.md)。已实现项以
`场景 ID / Case ID` 标识；增加、删除、拆分或调整可执行测试时，必须在同一变更中更新
测试清单、可执行注册和相关断言。规划项使用
文档编号，明确未注册及实现缺口，落实时再绑定真实场景/case ID。清单至少维护：所属层级、
主要验证目标、执行入口、前置与资源隔离/互斥约束、固定预期结果、实现状态和验收证据。
未实现、前置阻断、受控通过与真机通过分别登记，不能把清单存在或 driver 已实现当作
当前构建通过。测试 outcome、业务 outcome、资源清理和并行资格分别报告。

执行目录为 [tests/catalog.json](../tests/catalog.json)，唯一 case 定义为
[cases.json](../internal/testkit/livetest/cases.json)；PowerShell 的
[目录读取](../scripts/live-catalog.ps1)将场景 ID 展开为注册元数据；旧格式自定义目录继续核对层级、driver、target、超时和完整 case 集，批次冻结展开后的完整目录。
[Go 注册测试](../internal/testkit/livetest/cases_test.go)与
[脚本检查](../tests/framework/stability-test.ps1)核对当前 L3 的二十二场景／三十二 case；原 12 个
可靠性 case 保留。四个 L4 场景绑定精确的受控测试名，不以受控结果代替真机资格。
新增场景须注册需求并将 ID 加入执行目录，同步 manifest 校验和结果报告。workflow/2 在 `pr_ready` 后拒绝业务反馈；L2 不执行发布后反馈，也不读取反馈资格文件；L3 feedback 核验拒绝后原 revision、Turn 与 PR head 不变。
耗时和模型使用按实际测试目的安排：快速规则/路由反例放 L1，复杂确定性分支使用受控
集成，真机测试验证真实后端接线与核心行为，不强迫模型制造可重复的 finding。
文档规划不表示入口已经支持新 case、L4 或并行调度。

## 当前执行入口

初始化资源停止新增 L3 `orca-remote.initialization-stop/before-readiness` 与 `restart-before-readiness`。公开 submit 直接启动原生 Codex，测试转发器仅隐藏 caller 的原生 agentIdentity，保留原响应与调用 ID；第二项在该窗口停服换代，验证原终端绑定和零自动输入后显式停止。两者均要求完整原 Orca 清单、`orca_terminal` 离线 scope，零业务输入、额外终端或文件部署，不要求远端 helper/Job/签名/认证撤销。独立现场可并行，无管理员故障设施；新增结果与历史记录见[初始化停止验收](acceptance/initialization-stop.md)。

Orca 本机接入新增 `orca-local.smoke`、`environment-check.orca-local-sandbox` 和
`environment-check.orca-local-full-access`，沿用现有 driver/schema。对应配置项的
`execution.settings.agent_launch.runtime_executable` 指向本机 helper，
`fixture.repository.local_path` 明确选择真实注册项目，不能任选同名项目。
本机 agent/model/effort 取该项 execution.settings 的公开字段，不需要远程
environment。执行主机 helper 安装在业务工作区外。本机 fixture 的 caller clone 与 Orca
已注册项目分别记录，原 runtime 从准备证据核验，不复用 `remote_runtime_id`。
本机通过原生 IPC 查询观察，远程保留订阅加补查；注册不代表已验通过，当前构建结果见[专属验收](acceptance/orca-local-remote.md)。
停服取证探针为 `go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout 280s -v`。
先用调用方 `scripts/new-debug-release.ps1` 生成独立 `C:/SteerlineDebug` 包；显式提供
`STEERRUNNER_ORCA_STOP_PROBE_ROOT`（新的包内 evidence 子目录）、`_CLI`、`_RUNNER`（该包固定 EXE）、
`_REPO`（Orca 注册的测试项目 ID）、`_BASE`（固定完整 commit）、`_REPOSITORY`（本包本机 clone）；
变量名均以 `STEERRUNNER_ORCA_STOP_PROBE` 开头，远端另用 `_ENVIRONMENT` 固定路由。
不设 `_HELPER` 时沿用真实 PTY+存活 root 的拒绝探针；设置 `_HELPER`、`_HELPER_SHA256`、
`_NATIVE`、`_MODEL` 时运行真实 Codex/Orca：原 helper 启动并完成一次无工具 READY bootstrap，
真实原生历史 reader 查询该 bootstrap 并清空原 Job，随后实际适配器显式停止原 Codex，关闭原 PTY。
发布 Runner EXE 的 `service verify-stopped` 必须成功，普通被动 shutdown 使用同一证明；
活动期拒绝，DB/原失败回执不变，不启动服务。只清理本次 owned worktree/PTY。
当前格式 Run 是绑定这些真实身份的隔离契约夹具，零业务输入/PR；不冒充完整 L2/L3 或用户安装现场。
普通 L1 拒绝该 native opt-in，默认 skip 不计真机通过。结果见[本次专属验收](acceptance/orca-shutdown-verification.md)。
无模型探针使用显式 `STEERRUNNER_ORCA_IPC_PROBE_ROOT/CLI/REPO/BASE` 环境变量，运行
`go test ./internal/execution/adapters/protocol/orca -run '^TestOrcaLocalIPCNativeProbe$' -count=1 -timeout=2m -v`。
ROOT 必须为新的绝对证据目录，CLI 是选定原生 Orca EXE，REPO 是真实已注册 repository ID，
BASE 是该项目已有完整 commit；探针只创建/清理自己的 worktree/terminal 和 nonce 文件。
L1 拒绝该 live opt-in，默认 skip 不计真机通过。

在仓库根目录使用 PowerShell 7：

认证 socket 的快速回归随 L1 运行（`TestAuthenticationSocket*`），覆盖原安装布局的 124 字节旧路径与 90 字节新路径、冻结资源身份、各 Run/attempt 的隔离、目录 junction/替换、socket 替换、错误 capability、部分准备失败及精确清理。定向原生入口 `TestAuthenticationNativeToolBoundary -sandbox-auth-native <codex.exe> -sandbox-auth-host-broker` 使用独立长路径 Git 夹具和只读 GitHub 查询，检查实际 MXC 工具连接、共享认证父目录写入拒绝及退出清理，不运行模型或发布 PR。

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
# 将 example 复制到本地非秘密配置，填写原生可执行文件与已配对 environment。
New-Item -ItemType Directory -Force .evidence | Out-Null
Copy-Item tests/test-config.example.json .evidence/test-config.json
.\scripts\test.ps1 -Level L2 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.smoke -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Scenario local-codex.feedback,local-codex.audited,local-codex.multirun -MaxParallel 2 -Config .evidence/test-config.json
# 全 L3 保留全部注册 case；缺配置的项目不派发，报告 incomplete，退出 1。
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L2,L3 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L4
```

L2/L3 自动先运行 L1 和完整受控前置。任何前置失败都不会启动模型。
L1 每个 package 上限为 12 分钟：独立仓库冻结回归含多次真实 Git clone/fetch，application 包还包含 owner/parser/checkpoint 受控回归。聚合预算不延长各单次调用期限，实际耗时以当前批次日志为准。
完整 Controlled 独立 Go 宿主聚合上限为 45 分钟，入口额外等待 30 秒以覆盖结果落盘；前置必须完整执行并通过。单场景/定向入口保持原预算，CLI、原生准备、故障和业务轮次期限不随聚合预算延长。
PowerShell 交互调用多个场景时可使用数组；外部命令行请传实际数组参数，不把逗号串作为
单个场景 ID。审计配置 `local-codex-audited` 的 settings.model 与 workflow.audit_model.model
须显式且不同，effort 位于各自的 reasoning_effort。独立两模型场景仍保留原会话，不要求模型产生 finding。

有可执行 case 的 L2/L3/L4 自动先运行 L1 和完整受控前置；全缺配置时只报告不可用，不启动这些前置。共享前置失败阻止所依赖的 case；L2 专属原生 Git 检查归其 worker 准备，失败不阻止独立 L3。L4 不需要真机
配置，执行真实 EXE/服务/Git/SQLite 和受控原生协议，不消耗模型 token 或创建真实 PR。
`-MaxParallel` 默认为 1，范围 1–8；L2/L3 可并行，L4 先排空队列后独占执行。调度器还
用当前用户宿主的八个 OS 文件锁槽协调不同批次的场景；L4 占全部槽，L2/L3 各占一槽。
该约束只协调使用本调度器的场景，不限制其它应用或手动诊断。
锁由协调器持有；协调器异常退出会释放锁，但后台 worker/原生服务可能仍存活。必须先按
原现场对账并停止这些资源，再启动需要独占的批次；锁释放不是原资源已清理的证据。
单一协调器写批次 summary，各 worker 写独立 case 记录；worker 丢失结果、构建摘要变化、
原生资源身份缺失/跨 case 碰撞均失败。启动 worker 不算执行通过，未收尾证据继续保留。
`-Scenario` 为精确场景 ID，未匹配、重复或跨级选择失败。每次生成新现场，
不复用上次 Run、请求授权或已经执行过的 manifest。脚本等待完成后返回退出码；
不能以后台 PID 或启动成功作为通过。

L2 原生资格要求 Codex >= 0.159.2，并保留生产协议与权限检查；`configurations.local-codex` 必须提供本机
codex-app-server、development/2、显式 sandbox 和绝对 native EXE 路径；不接受脚本包装器或提示词覆盖。
未指定模型沿生产契约使用原生默认，可显式固定 model 和 reasoning_effort。
每个远端、Pi、DSH 或审计配置项均填写完整 execution，不继承本机配置。
原生远端不注册有来源证明的审计或完整环境检查正向用例；纯校验、capabilities 和零副作用拒绝由 L1/Controlled 覆盖，不计为真机成功。`orca-remote.multirun` 使用完整的 `configurations.orca-remote-codex-full-access`，同服务两个 Run 独立发布，核验 request/command、会话、工作区、分支、分页事件、last-message 和 PR 不串线；停止一项后另一项原生 owner 保持可用，最后逐一核实原终端停止。远端不校验或伪造 Runner broker/capability 关闭。

L3 远端 Orca 使用指定 CLI、已配对 environment 和远端已有的同名 SteerlineTest Git
项目。两端分别在实际执行主机准备认证，不从调用方复制认证，也不修改全局权限配置。

本机 Orca 的 `orca-local.codex`、`orca-local.pi`、`orca-local.pi-full-access`、`orca-local.dsh`
使用 L3 development-feedback driver，覆盖业务 needs_input/reply、发布后反馈拒绝且原来源不变、独立 PR/head/合入，
每个核心现场还用独立短 Run 验证受管 agent 的启动失败收尾，以及真实扣留一次 Orca business-send 接受响应后的 unknown、零重发和原写者停止。正常 Run 另验 request_id 查询/幂等重放和兼容服务换代后的原授权保持、原会话继续；独立实现 Run 验证实际活动写者停止。
本机 fixture 使用 test-only CLI 转发器记录真实原终端 send；重启零输入指纹必须有非零基线，Codex 不再读取不存在的扩展目录。原 Orca CLI 和 TUI 仍是真实执行来源。
`orca-local.mixed` 复用 multirun driver，在同一服务运行 Pi/DSH，并检查停止一个 Run 后另一个的原生资源、权限及完成来源保持有效。
例如 `./scripts/test.ps1 -Level L3 -Scenario orca-local.pi -Config <配置路径>`；真实结果见[通用接入验收](acceptance/orca-generic-agent.md)。

Orca 配置使用 execution.settings.transport.executable、agent_launch.runtime_executable／runtime_sha256 和 fixture.repository.local_path；
运行期间不依赖 pairing/environment，manifest 冻结 `local_runtime_id` 与 `local_repository_path`。
Pi/DSH 分别读取各自配置的 agent_launch.executable／model／reasoning_effort，
不继承 Codex 参数。本机原 TUI 和 helper 均在所选执行主机运行。

受管版本为 Pi 0.87.1、DSH 0.2.1-rc.2 + 社区 dsh-tui 0.16.1、Node 24；Windows 权限工具使用
native Codex 0.159.2 的 sandbox 子命令，不启动 Codex 模型或会话。两种 access_mode 均无审批升级。
Helper 可用 `go build -o <安装路径>/steerrunner-agent.exe ./cmd/steerrunner-agent` 构建，然后配置绝对路径；
新 Run 要求 helper metadata v3／原生回执 v2，回执核验实际 reasoning effort；旧冻结实例仍按原构建和来源读取。
Pi/DSH 要求原生依赖已安装，Run 不联网安装、不修改全局 profile。独立 DSH 安装示例：

```powershell
npm install --prefix C:/AgentTools/dsh @deepseek-ai/dsh@0.2.1-rc.2 @deepseek-harness-tui/dsh-tui@0.16.1
# 对应配置的 agent_launch.executable 指向 C:/AgentTools/dsh/node_modules/.bin/dsh.cmd
```

原 TUI 的正式扩展提供开始/完成事实，现有 Orca 只负责资源和投递；不修改 Orca。
Node 28 项扩展及共享工具宿主契约测试已纳入统一 L1，可单独运行：

```powershell
node --test internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs internal/execution/adapters/orcatransport/extensions/dsh-managed.test.mjs
```

`scripts/probe-orca-pi.ps1` 仅诊断 Pi 0.87.1 / Orca 1.4.224 的无工具原 TUI，先运行上述测试。
须提供已在 Orca 登记、位于用户临时目录直属 `steerrunner-orca-pi-p0-<32位UUID>` 的独立无 remote Git 仓库，
以及新的 `-EvidenceDirectory`；不能使用业务仓库或旧现场。脚本只请求固定短答，使用已有 Pi 模型访问，
限定探针输出 256 token，关闭工具和发现，只显式装载 Run 私有候选。它不安装依赖、修改全局 profile、
发布 PR 或放行生产。真实证据见[新实施记录](spec/orca-generic-agent.plan.md#7-约束收敛后的实施pi-p0)；
P0 探针不计为三个已注册 L3 case 或 Runner 真实重启/生产权限通过。

本机 GitHub worker 默认使用执行主机已有 gh 登录的受限认证代理。OAuth 只在可信
runtime 内存中，worker 只继承随机单仓库 capability，一小时到期、退出即失效。
Git smart HTTP 经进程级代理及公开 CA，gh 经独立公开配置的 AF_UNIX 接口；原工具
仍在 MXC 内执行，原 GitHub URL 保持。socket 位于已有 Git 写入根，证书不安装到系统。
代理仅允许目标仓库的 Git 上传/下载、身份/仓库只读查询和 PR 查询/创建；禁止其它
仓库、管理接口、未知 GraphQL 及重定向。GitHub App 支持已移除：`execution.settings.github_auth`（含 null）严格拒绝；fixture 的旧
`github_auth`/`remote_github_auth` 同样在环境/仓库准备前拒绝。本机/Orca local 受管认证使用宿主 gh 单仓库 broker；远端使用已有 Orca 原生认证，不承诺相同 capability 或撤销保证。
宿主登录成功不能证明就绪；实际沙箱门槛失败仍零业务输入，不回退审批发布。
参见[本次认证验收](acceptance/sandbox-authentication.md)。

权限模式由配置项的 `execution.settings.access_mode` 冻结，默认 `sandbox`；`full_access` 为显式选择。
普通业务与环境检查复用生产权限编译。`execution.settings.agent_launch.args` 只填写支持的启动参数（示例为
`["--no-daemon"]`），不以额外 `-c` 重复覆盖冻结权限。不能把不同模式/配置的运行
合并为同一通过结果。Orca local 的 `agent_launch.runtime_executable` 指向用户安装的受管 helper，核验摘要后沿同权限准备、原控制回执、broker 与停止证明；这些保证不适用于原生远端。

远端只使用既有 Orca/Codex/Git/gh 和用户登录，不上传、探测或调用 Runner helper，不转发本机 token、OAuth 或 gh 配置。远端 runtime_executable/runtime_sha256 键（包括空值）在接受前拒绝。model/effort/access_mode 仅核验冻结参数及传递命令；native-launch-contract 明记 effective_parameters_verified=false，native-call-audit 要求零 helper/query terminal，不冒充 OS token、权限审计或实际模型回读。完整远端 Check 和有来源证明的双模型审计明确不支持。

新建远端 GitHub sandbox 任务通过有限启动命令校验 linked worktree 的四个 Git 写根，并在执行主机将已有 gh 登录供给该原生进程的工具环境。Git 的 URL header 和 gh token 仅经进程环境传递；沿用原账号权限，未恢复历史 broker 或撤销保证。专项真实沙箱命令和六个管理员故障窗口的结果分别记录；只读身份查询或 push dry-run 不能代替真实 PR 交付。运行时模块为每个故障现场复制独立文件，避免跨现场硬链接与缓存校验争用。参见本轮 [Git 权限修复证据](acceptance/evidence/pr-test-gaps-20261020/git-permissions-fix/)。

原生停止按原 runtime/worktree/terminal/incarnation 的新鲜完整查询判断，缺少执行内结果时保留 unreported/unknown。offline verify-stopped 的 scope 只描述该原终端，不证明全部 OS 后代或远端登录撤销；历史签名/Job 证明不继承到新路径。登录或权限失败按实际操作结果保留，不自动换登录、后端或 full_access。

## 当前完整 smoke 契约

本节以 workflow/2 的 driver 为准，包含实现、发布核验、真实合并与停止；发布后反馈返工已移除，独立 feedback case 验证该输入明确拒绝，不创建新实施轮或更新 PR；[测试清单](test-catalog.md)
中的 L2 最小化为后续调整目标，当前执行步骤或结果判定尚未改变。

固定任务改编自 SteerlineTest 的 native-101 Spec/Plan，源 commit 为
`cebdd5ab552150c49743e40593c836689ecf789b`。自有版本位于
`tests/fixtures/evidence-smoke/`，只更新 `evidence/native-101.md`，实际读取 UTC，
运行检查并提交。执行步骤：

L2 使用生产默认的实施/报告/发布提示词，不覆盖 `prompt_overrides`，也不把
`publish.ps1` 发布器放入 L2 夹具；Git 提交、push 和 gh PR 创建由原 worker 完成。
夹具显式保留 packed refs。L2 case 的准备另通过实际 native `command/exec` 验证 loose/packed
head 提交的 stderr 为空，且另一个仓库仍不可写；不使用模型或远端写操作。
worker 命令日志审计会拒绝权限错误，即使命令退出码为 0、提交及 PR 已成功。
文件系统拒绝及 HTTP 401/403 均在审计范围内；默认 host broker 的可信收尾事实还须
包含拒绝计数且为 0、closed 和 socket cleanup 已确认，不能用隐藏输出掩盖拒绝。
L3 的固定发布器及故障门保持，用于原有反馈/可靠性场景。

1. 生成 `C:/srt/<随机ID>` 短路径现场（可用 `-FixtureParent` 指定父目录），构建 Runner，
   冻结 EXE/Spec/Plan 摘要、配置、来源与专属 `codex/srtest/<ID>/base`。
2. 克隆固定 SteerlineTest 源提交，提交本轮 checker/publisher 和材料作为测试 base，
   推送这个新 base。远端通过独立准备终端 fetch/cat-file，记录固定 runtime 并确认终端关闭。
3. 通过真实 Runner CLI 提交冻结 Spec/Plan，由 Runner 实施、检查、提交、按需报告、发布、
   核验 PR，到达 `pr_ready`。宿主不代替模型提交或发布业务改动。
4. Driver 从 GitHub fetch 实际 PR Git objects，校验原生 final、PR 身份、仅目标文件变更、
   精确格式和新鲜时间戳。远端同样核验实际 Git 内容和固定 runtime 的原会话。
5. workflow/2 的 pr_ready 没有返工边；保持首次已核验的 PR/head，检查冻结原 final 与程序结果来源；原材料不足时允许一次同源 report，parser 始终为 0。
6. 刷新原 PR，精确绑定 head，driver 以 `--match-head-commit` 合入本轮专属测试 base。
   Runner 通过 `inspect --refresh-pr` 保存真实 merged、merge commit 和时间。
7. Fetch 合入后的 base，验证 merge commit、原 head 的祖先关系和文件内容，并在实际合入
   checkout 再运行 checker。
8. 显式 stop 原 Run，确认 `new_inputs_allowed=false` 和 `stop_status=stopped`；再停止服务并确认
   `not_running`。保留 Git 工作区、分支、PR 和原始现场供审计。

L2 和普通远端 smoke 均执行首次交付、合入和收尾；独立 L3 feedback 在合入前
额外验证发布后反馈拒绝。audited/multirun 按各自清单验证真实 PR，保留未合入并停止原资源。
两模型入口额外生成 `audit-models.json`，从原 session/workspace 的原生 history 按审计
turn_id 和发布 turn_id 校验实际 model/effort，不能仅凭请求配置认定发生了模型切换。

Runner 生产契约的交付里程碑仍为 `pr_ready`。合并属于测试 driver，结束判定包含 Runner
观察 merged 后的显式停止；不引入 SteerLine TaskDelivery，也不新增生产自动合并行为。

## 证据与失败

批次 `.evidence/tests-*/summary.json` 包含来源、每级结果及现场路径。
报告保存原请求范围、每 case 的 availability／原因，以及每级 registered／requested／excluded／unavailable／executed／passed／missing
集合和 `complete_level_passed`；repeat 单独计数。缺配置的项为 unavailable，零派发，独立的
已配项仍执行；只要请求范围有不可用项，批次为 incomplete、退出 1，实际失败优先记 failed。
显式 Scenario 子集可通过，但不能代表全级通过。本机基线配置完整性与实际同构建 L2 通过分开报告。
前置失败仍保留全部请求项和 unavailable 原因；尚未派发的可用项为 queued，不计执行。
现场 manifest 记录冻结输入；`evidence/` 保存 `result.json`、`scenario.json`、CLI journal、
driver 的 Git/GitHub/checker 调用、实际 PR、合并与停止快照。Runner 自身业务数据和
原生协议日志在该现场 `s/` 中，模型 token 信息以原生记录可提供的真实数据为准。

smoke 通过要求：非零且精确的场景数、无意外 skip、真实产物正确、检查通过、PR 合入、merged
事实核验、原资源和服务均停止。L1 编译 live driver 并验证零场景失败门槛，同时运行入口
的零匹配/缺配置/重复场景反例、Windows
PowerShell checker 和发布夹具的受控反例，覆盖空 PR 列表、复用原 PR、冲突和查询失败
禁止重复创建。L1 只允许脚本逐名列出的 subprocess helper、Orca 取证入口、MXC 工具环境诊断及用量只读/清理现场入口默认 skip；相应 opt-in 遗留仍在运行前拒绝，任何其它 skip 均失败。这些 skip 不计真机通过。MXC 入口用
`go test ./internal/execution/adapters/codex -run '^TestAuthenticationNativeToolBoundary$' -v -count=1 -args -sandbox-auth-native=<Codex绝对路径>`
单独执行，使用无效受控 canary，不运行模型、不证明真实认证/发布。
同一命令增加 `-sandbox-auth-host-broker=true` 可在全新诊断现场使用宿主登录的受限代理，
验证真实 MXC 的 Git/gh 只读能力；仍零业务输入、不证明 push/PR 创建。此入口保留原进程
身份、准备事实和关闭计数，不导出宿主 token。

未满足冻结 case 预期的失败、超时、unknown、unsupported、缺 final 均不通过；不可达停止
的 `revoked-unverified` 按其故障窗口契约独立判定，不冒充停止确认或业务交付。清理未确认
一律不通过。保留原现场，不自动
重发原生输入或替换会话。`scenario.json` 在外部动作前保存阶段和已知 Run/PR 身份；若
进程异常退出没有结果，批次失败，须用原 state-dir、request_id/Run ID 对账及停止。
修复后创建新现场；旧失败证据不改写。

这里的 unknown 指操作、结果来源或副作用无法对账。原生主代理已经核实当前回合 done、
原正文和来源完整时，可省略的 native outcome 仍按 unreported/unknown 原样保留，不补成
success；它与“原操作结果未知”分别判断，沿用既有 Orca 完成契约。

准备期限只覆盖当前阶段的准备。一个已完成的长业务回合切换报告或发布阶段时，
新输入获得自己的有限准备预算；未决原操作的对账仍沿用原期限，不因重试获得新预算。

单步诊断入口为 `new-test-fixture.ps1 -Scenario ... -Config ...` 和
`test-native.ps1 -Manifest <绝对路径>`。它们供已有前置通过证据的开发诊断使用；正式分级
验收使用 `test.ps1`，确保前置门槛和批次汇总完整。

审计 workflow 的两模型定向诊断复用同一个 native driver。配置中先显式指定执行 `model`，
并填写 local-codex-audited.workflow.audit_model；省略审计 effort 时继承执行 effort。此诊断核验代码审计修复、
执行模型回切及实际 PR 后停止原 Run/服务，保留测试 PR 未合并，不计作普通 L2 完整 smoke。
它不改变默认 L2/L3 场景选择或生产默认发布提示词；该新 Run 冻结测试用发布提示词覆盖。

```powershell
$auditManifest = ./scripts/new-test-fixture.ps1 -Scenario local-codex.audited -Config .evidence/test-config.json
./scripts/test-native.ps1 -Manifest $auditManifest
```

## 独立测试配置与迁移

测试文件使用 [example](../tests/test-config.example.json) 和[外层 schema](../tests/test-config.schema.json)。
`configurations` 中每项为 `{execution, workflow?}`，直接复用公开生产字段和纯 gate；提供
workflow 就必须合法，Run 需要 workflow，环境检查只消费 execution。需求／角色／固定配置项
引用由唯一 [case 注册](../internal/testkit/livetest/cases.json) 维护，用户无需再填写 case 绑定表。
`fixture` 声明完整 repository／publication 测试源，继续只写专属 SteerlineTest；每 case
生成独立分支／材料／查询取证工具。当前 fixture 使用 GitHub command publication；其他
产品合法 provider 会明确标记不符合 case 要求，生成器不自动改选 provider。query 取证工具
由 case 生成，配置中的认证引用等其余设置保留。`harness` 只含 admin_host；
证据目录沿既有 `-OutDir` 管理。测试输入不进入生产 profile，也不影响外部 submit。

文件只从显式 Config 读取，入口相对路径按调用 cwd，harness 相对路径按原文件目录解析。
批次冻结规范化副本／摘要，worker 使用该副本及同一 Runner 构建；不重读原文件选择后端。
根结构、版本、未知外层字段、重复键错误退出 2；单项无效只使依赖它的 case 不可用。
旧 live-config/v1、v2 明确拒绝；按上述层次手动迁移，不自动继承扁平字段。旧冻结现场仍用原构建结算。

静态列表只判配置是否足以尝试，不查询登录、程序存在、模型或连接。它要求预先产出的待测
Runner 和匹配 helper，不触发构建、L1、服务、调度锁或现场：

```powershell
.\scripts\build.ps1
go build -trimpath -o bin/test-config.exe ./tests/helpers/native
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json -ListAvailability
```

可用 `-RunnerExe`／`-ConfigHelper` 指向已有构建；实际测试使用相同参数可冻结选定产物，
Runner 的 `.build.json` 须随包提供。列表成功退出 0 不代表已通过，缺产物／文件损坏退出 2。
本机 Codex 是独立项目交付基线；缺基线不阻止无依赖的 L1、Controlled 或其他已配置 L3。

## 扩展场景与 L3 稳定性入口

场景注册在 `tests/catalog.json`，新增内容先按本文 L1–L4 定义归类并更新
[测试清单](test-catalog.md)。
L3 扩展须说明独有流程/可靠性覆盖和并行隔离；同一流程的新真实目标须有独立的兼容性
断言。常规可靠性及阶段切换变体保留在 L3，更复杂边界或确实无法隔离的测试才归 L4。
当前全跑 `-Level L3` 包含远端 smoke 及全部稳定性 case，不包含清单中未注册的规划项；
也可用 `-Scenario` 选择子集。报告始终记录实际选择，不将子集通过表述为全级通过。

新的流程在 `tests/native/run_test.go` 的 `drivers` 注册表增加 driver，并在
`new-test-fixture.ps1` 增加对应现场构造。Driver 使用公开 Runner CLI，定义可独立核验的
产物和完整停止判据。增加错误 PR/错误 runtime/陈旧产物等受控反例后，再加入真机目录。
每个 case 都要有自己的现场、Run、base/head 和结果；交付 case 必须核验本轮真实 PR，
停止 case 保存专属分支的无 PR 证据。禁止用模拟目标替代真实环境。

L3 执行稳定性契约见 [Spec](spec/l3-execution-stability.spec.md)、
[Plan](spec/l3-execution-stability.plan.md) 和[实际验收登记](acceptance/l3-execution-stability.md)。
主线 rebase 后的一轮复测及未通过项另见[复测记录](archive/2026-10/acceptance/l3-rebase-20261016.md)。
连续版本冲突的修复与后续来源验证见[不可达撤销修复复测](archive/2026-10/acceptance/l3-revocation-fix-20261016.md)；历史远端认证阻断未计为通过。本轮完整范围复验、后续修正与未通过项见[2026-10-18 记录](acceptance/evidence/full-l2-l3-20261018/README.md)。
选择场景即执行其全部变体，每个 case/repeat 独立建现场。稳定性通过还要求故障命中、
原生调用审计、原身份和完整收尾；预期停止/撤销未知不要求 PR 合入，也不能记成业务交付。

完整稳定性套件需要管理员测试宿主、Node.js、已安装的 Windows Orca CLI 公开运行文件及
已配对的真实 environment。使用 `tests/test-config.example.json` 创建本地非秘密
v1 测试配置并补全 registry 引用的配置项；保持原生自身认证。完整资格入口：

主测试保持普通用户身份，管理员终端单独运行
`./scripts/stability-admin-host.ps1 -Directory C:/srt-admin`，并将配置中的
`harness.admin_host` 设置为该目录的绝对路径。该宿主只接受固定现场的网络 block/block-state/restore，
核验宿主 PID/创建时间与原 manifest；每次操作有独立回执和恢复 watchdog。
宿主最多运行十二小时，可在该目录写入 `stop` 文件结束；先确认本轮故障设施均已恢复。
重复验收请使用新的空目录，避免上次的 `stop` 文件使宿主立即结束。

`orca-remote.connection-reconnect` 同时运行 `active-disconnect` 和 `pre-send-disconnect`。
后者在发布输入提交后、首次状态建连前由测试 helper 设门，通过 `block-state` 只阻断
现场 Runner EXE，保留 CLI 通道；核验原发布 operation 仍处于 in-flight、授权未撤销，
至少一次建连失败后重新读取原终端，且阻断期无新 send。恢复后无需 resume，到达同一
原会话/Attempt 的 `pr_ready` 并只发布一次。结果校验独立读取原门、原生调用日志、
Runner-only 网络 scope 和阻断期快照，不接受仅凭 passed 标志的结论。
L1 另覆盖准备/发送前多次断连、独立查询失败、取消/期限耗尽、认证/身份变化不可重试、
恢复后原 provider 绑定及已尝试输入零重发。新 L3 case 仍需有效的对应配置项和管理员故障宿主
做真机验收；注册和受控测试通过不代表该 case 的真机或并行资格通过。

```powershell
.\scripts\test.ps1 -Level L3 -Repeat 3 -Config .evidence/test-config.json
```

当前全 L3 每轮二十二场景三十二 case；原远端六场景十二 case 的历史三轮仍为三十六次，不能
与新增本机流程混用计数。L1/Controlled 每批完整执行一次。
`-Repeat` 默认 1，范围 1–10。`-SkipAdminRequired` 仅供用户明确选择排除管理员场景时使用，
报告保存所有排除项及 `passed_with_exclusions`，不能声明完整 L3 通过。当前最终验收要求全部执行。

## 结果获取重构的当前验证边界

结果获取的定向命令和全部目标在 [Plan](spec/result-acquisition.plan.md#3-最小测试计划)，执行状态见[专属登记](acceptance/result-acquisition.md)。Controlled `TestSRResultAcquisition` 使用真实临时 Git/Runner EXE/SQLite，十一项内部 case 覆盖成功行、零/正数审计修复、离线审计、一次补报、parser 环境失败后备、正常解析、工具事件拒收和解析中停机/重启/显式停止；不创建真实 PR。该场景的终态、阶段准备及停止观察等待已统一为 5 分钟，离线回执观察为 1 分钟；这些是测试等待预算，不改变生产单次操作期限，修正记录见[PR #38 复验](acceptance/pr38-review-20261020.md)。完整 Controlled 按上文 45 分钟聚合预算执行。

`local-codex.result-acquisition` 已注册为 L2 专属 driver；入口先 L1 和完整 Controlled，再运行默认原生认证解析、已有 home/显式模型与权限反例/取消、development 真实 PR 闭环三项内部 case。原生只读权限、行为约定和工具事后检测分别取证，不宣称空工具表、零隐式上下文或工具执行前拦截。2026-10-17 原强隔离阻断记录保持历史原样。实际命令、失败修正及最新通过状态以验收证据为准。
## 环境检查定向验收

规则：`go test ./internal/application ./internal/contracts ./internal/cli ./internal/storage ./internal/execution/... -run '^TestEnvironmentCheck' -count=1`。
`TestEnvironmentCheckPublicFixtures` 通过生产存储/查询路径校验内嵌公开样例；合约有意变化时可用
`STEERRUNNER_UPDATE_CHECK_FIXTURES=1` 生成后复验，样例文件变化会改变 resource_digest。
`TestEnvironmentCheckCLI` 已注册到 Controlled，验证真实 EXE/IPC/SQLite、受理后失败、两种查询、原 stop
回执、无 Check 拒绝、只读查询、重启与 verify-stopped。测试不会访问安装目录。

```powershell
./scripts/test.ps1 -Level Controlled
./scripts/test.ps1 -Level L3 -Scenario @('environment-check.local-sandbox','environment-check.local-full-access') -Config .evidence/test-config.json
```

两种原生场景沿正式分级入口，分别创建新 fixture 和 Check。driver 只准备
`codex/srtest/<fixture>/base`、独立源仓库及只读取证；Worker 通过已保存意图的固定原生工具创建
`steerline-check/<check_id>/base|head`、commit/push 和 PR，driver 不代做这些动作、不合并 PR。
每种模式必须独立核实权限、实际工具、Git/PR 及原对象收尾，不能用 L1/Controlled 或另一模式替代。
历史 helper 构建曾注册远端两种模式 `environment-check.orca-sandbox/complete` 与
`environment-check.orca-full-access/complete`。它们通过原 Orca runtime/terminal incarnation
绑定执行主机的有限检查会话；复用相同的无模型预检、实际 Agent/工具、一次性外部写门、
owner 核验及精确资源收尾。重启只允许结算原会话，不创建替代检查。调用端不探测远端 PID。
登记与实现不等于真机通过；各受测构建、模式和清理结果按本轮证据独立判断。

### 历史 helper 验收说明（不适用于新远端）

下述 inline/独立查询终端描述保留给旧构建记录；当前 local 受管路径以固定 control actor 实现为准，原生远端不使用该历史查询链。

受控 Orca helper 的 inline-v2 输入包含唯一 command ID 和完整原输入摘要。普通开发回合通过
执行主机的只读 Codex `thread/read` 核对原工作区/会话、完整实际输入摘要、native turn/user item
及 `final_answer`，用于处理 Orca 漏报短回合启动事件。只读查询不 resume 或发送业务输入；
独立查询终端必须按 runtime/terminal/incarnation 确认关闭后才能采用回执。重复匹配、后继外来输入、
原身份变化或未知查询资源归属均保持未核实，不能用终端画面、静默或 accepted 回执推断完成。
可靠性日志继续要求唯一业务工作区/终端及零业务重发。新增查询终端只在冻结 helper 路径/摘要、
精确 `--original-turn` 命令、原 ready/session/workspace、实际完整输入摘要、terminal incarnation
和确认关闭全部吻合时单独计数；不能仅凭 title 前缀豁免。取消的只读查询不产生业务证据，
仍需确认其资源关闭；最终审计仅在原 case 预算内等待既有关闭，不发出任何补建/重发动作。
`native-resource-audit.json` 的计数还须由原始 native journal 独立复核，不能自报通过。
服务优雅关闭会取消原观察器，并在既有服务关闭期限内等待其查询清理 goroutine 结束；
不等待 actor 的事件确认，也不停止继续运行的远端业务会话。关闭期限届满仍报未核实。
调用端在创建查询终端前持久保存意图，取得原 runtime/terminal/incarnation 后立即保存身份，
关闭后保存回执；这些文件只位于调用端 Run 目录，不把远端 PID 或路径作为本机对象探测。
强杀重启后先清理记录中的原查询终端，完整原工作区清单可证明其已经不存在；仅原 incarnation
仍存活时可继续关闭。创建结果未知且没有资源身份时继续阻断，不能凭 title 或补建推断成功。
业务终端关闭已确认、查询清理未确认时分别保存进度，恢复只继续原查询资源清理。
已受理回合首次历史查询沿用 30 秒观察间隔；原会话的完成通知可提前触发查询，但通知本身
不作为完成证据。此间隔和后续查询均占用原 case 预算，不延长超时或改变故障命中条件。
工作区创建不切换 Orca 当前视图。新 worktree 已有终端时继续使用原终端；若连续完整清单
确认没有终端，在原准备预算内等待其异步出现后，仅允许一次首个终端创建。调用端先持久保存
创建意图、再保存返回的原 runtime/worktree/terminal/incarnation，重启只核对原身份；未知创建
不补建。返回后仍要求原工作区仅有该终端，才进入原 agent-start 和无模型权限准备步骤。
原生审计将这一可选首次创建单列为 initial_terminal_creations（0 或 1）；business_creations
仍要求只有一个业务工作区。首次终端必须有完整空清单、精确固定参数、原身份及后续 ready
证明，不能按 title 单独豁免；第二个业务终端创建、身份变化或不完整清单均失败。

定向 native canary 使用有效 gh broker、同一原生准备和 `command/exec`，无模型输入：
`go test -tags=live ./internal/execution/adapters/codex -run '^TestEnvironmentCheckNativeBoundary$' -sandbox-auth-native <原生Codex绝对路径> -count=1 -v`。
该测试为 sandbox 记录工作区外拒写、为 full_access 记录允许写；只操作 `C:/srt/ecb-*` 中本次
owned canary，证据目录保留。普通用户检查不重复执行沙箱安全 canary。

已完成 native driver 的 environment-check fixture 可用
`./scripts/test-native.ps1 -Manifest <原 manifest> -ValidateOnly` 只读复验完整结果。
该路径核对原 manifest/Runner/driver SHA、完整 Check、上下文/分项与资源结算，拒绝失败或未核实结果；
不重新启动 Agent、服务或 Git/PR 操作，也不重写原批次记录。正常首次执行仍自动调用相同结果校验逻辑。

### PR 只读刷新与历史反馈证据

当前 inspect 默认返回持久快照；`inspect --refresh-pr` 查询原绑定 provider，不派发 Agent 输入。workflow/2 在 `pr_ready` 后拒绝业务反馈。下列更新 head 的等待规则保留给 workflow/1 的历史 driver 和对应组件回归，不是当前流程的返工入口。

历史反馈闭环的 driver 等待原 PR 更新时，默认 inspect 只是持久快照。若工作流已回到 pr_ready，
快照仍是反馈前的 head，则通过现有 inspect --refresh-pr 只读刷新原绑定 provider。刷新后仍须
保持原 Run、PR/仓库/base/head 分支、Attempt、Session 和工作区；查询失败或身份变化不能通过。
driver 的原 12 分钟等待预算、原 PR 上的新 head、服务端 Git 对象/产物/时间与 checker、
精确 head 合入及停止断言均不变。组件反例包含平台迟到的新 head、运行中不刷新、未绑定 PR、
查询失败和替换 PR/Session；不得用反复读取旧快照来声称反馈未交付。

2026-10-19：远端 helper 接线删除后的本次验收以 [原生方案](spec/orca-remote-native.plan.md)及[专属记录](acceptance/orca-remote-native.md)为准。当前远端完整 Check 返回 unsupported，不创建 actor；local Check 保持。原生参数仅核对冻结值与传递命令，离线停止输出的 scope 只描述原 Orca 终端，不证明全 OS 树或撤销登录。


2026-10-23：远端 Orca Codex 最后消息沿现有 smoke、multirun、connection-reconnect drivers 核验公开 CLI 身份、摘要边界及重复读取时间；不新建 catalog case。`TestOrcaNativeStateLiveLifecycle` 的原 opt-in 入口记录普通通知和 query-only 的实际样本、独立消息/完成事件及原资源关闭。缺 prompt 的查询场景若真实出现，应保持未核实且零重发；若来源保留 prompt，只登记匹配路径通过。Controlled 覆盖空 prompt 连续链、标记变化、断线/重启和诊断重附着。详见[Spec](spec/orca-last-message.spec.md)、[Plan](spec/orca-last-message.plan.md)及[原验收](acceptance/execution-observability.md)。
