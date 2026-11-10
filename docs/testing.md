# SteerRunner L1–L4 测试指南

核对日期：2026-11-06；代码基线：`dda9a827`。本文件维护运行入口、选择规则、证据与资格判定；场景、数量、独有覆盖和缺口统一见[测试清单](test-catalog.md)。旧计数、耗时样本及已退役命令见[精简前记录](archive/2026-11/maintenance/testing.before-consolidation.md)。

测试源码、夹具和判定归属本仓，运行目标为 Windows/amd64，不依赖 SteerLine checkout、安装或数据库。使用仓库根目录的 PowerShell 7。配置与产物必须属于本次被测构建；文档存在、场景注册和旧构建通过均不等于当前构建资格。

## 分层定义

| 层级 | 测试目的 | 隔离与判定 |
| --- | --- | --- |
| L1 | 快速单元、组件、协议、存储、框架及依赖边界反馈 | 默认不启动真实模型或创建真实 PR；包含框架、`./internal/...`、Node 扩展及原生 driver 编译/零匹配保护 |
| L2 | 最小核心 Smoke 交付 | 当前只有本机 Codex smoke，完整交付、真实 PR 合入与停止 |
| L3 | 不同流程、后端兼容性及核心可靠性 | case 独立且可隔离；重启、断连、未知对账、停止撤销均属于核心保证 |
| L4 | 复杂边界、精确竞态或确实无法并行的测试 | 声明独有故障及唯一预期；当前四项受控边界测试独占调度 |

Controlled 是受控集成前置入口，不是第五级。它使用真实 EXE/CLI/IPC/Git/SQLite 和原生协议夹具，可做隔离本地 push，不创建真实 PR。其内部子场景不重复算作顶层测试。

分类先看独有产品承诺、断言及隔离条件；不能仅因用了故障注入、管理员宿主或暂时未验证并行就归入 L4。只换模型、参数或机器也不直接增加流程覆盖。同一服务多 Run 的 case 内共享服务，各 Run 的请求、会话、结果和停止目标须独立，默认工作区、分支和 PR 独立；显式 worktree 复用仅允许停止结算后的串行接管，并须验证并发互斥、累计成果和旧清理隔离；case 之间独立服务。调度并行通过不能替代产品内部多 Run 隔离验证。

## 当前执行入口

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
New-Item -ItemType Directory -Force .evidence | Out-Null
Copy-Item tests/test-config.example.json .evidence/test-config.json
# 填写已有原生 EXE、宿主登录、配对 environment 和专属测试仓库后运行。
.\scripts\test.ps1 -Level L2 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.smoke -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json -MaxParallel 2
.\scripts\test.ps1 -Level L2,L3 -Config .evidence/test-config.json
.\scripts\test.ps1 -Level L4
```

有可执行 case 的 L2/L3/L4 自动先运行 L1 和完整 Controlled；前置失败不启动依赖它的模型。全缺配置只报告不可用，不启动前置。L2 的原生 Git 检查归其 worker，失败不阻止独立 L3。L4 使用同批 Controlled 的三份冻结二进制并校验摘要、退出及无 skip，不借用历史通过。

`-Scenario` 必须是注册的精确 ID，重复、未匹配或跨级选择拒绝；选择场景执行其全部 case。PowerShell 可传数组，外部命令行须传实际数组参数，不能把逗号串当作一个 ID。`-Repeat` 为 1–10，每个 case/repeat 建新现场。`-MaxParallel` 默认为 1、范围 1–8；L2/L3 有界并行，L4 排空队列后独占。

当前 L1 的 Go 包级聚合预算为 **30 分钟**，完整 Controlled 为 **45 分钟**、另加 30 秒落盘等待；定向 foundation/controlled 默认 **25 分钟**，可用 `-TimeoutMinutes 1..60`。这些聚合预算不延长产品调用或用例内期限。以[前置脚本](../scripts/test-prerequisite-contract.ps1)和[定向入口](../scripts/test-foundation.ps1)为准。裸 `go test ./...` 还包含 L4 资源压力测试，不等同于 L1。

宿主八个 OS 文件锁槽协调使用本调度器的批次；它不限制其它应用。锁随协调器退出释放，后台 worker/服务可能仍在运行，必须先按原现场对账和停止再启动独占批次。每个 case 的 state-dir、工作区、Run/session、base/head、PR、故障设施及证据目录独立；只有冻结且只读的产物可共享。每项只清理自己的资源，不切断共享 runtime 或整机网络。平台容量、并行资格和资源清理分别举证。

## 快速定向验证（P1，2026-11-01）

开发反馈可使用受影响包的 `go test -run <精确模式>` 或 `test-foundation.ps1 -Pattern <精确模式>`。真实 case 的正式定向入口：

```powershell
.\scripts\build.ps1 -OutFile .\.evidence\focused-build\steerrunner.exe
.\scripts\test.ps1 -Focused -Level L2 -Scenario local-codex.smoke `
  -Config .evidence/test-config.json -RunnerExe .\.evidence\focused-build\steerrunner.exe `
  -OutDir .\.evidence\focused-smoke-new
```

`-Focused` 只接受单级 L2/L3、明确 Scenario、现有 Config、匹配当前生产输入的固定 RunnerExe 和全新显式 OutDir。不能与 Catalog、ListAvailability 或 PrerequisitesFrom 合用，不接受通配、重复或级别不匹配的场景。ConfigHelper 仍可显式提供。

它不执行完整 L1/Controlled，仍新编译 native driver 并验证零匹配保护；case、fixture、调度 lease、原生断言和清理不变。报告 `validation_scope=focused`，保存前置启动次数及耗时；即使选中该级全部 case，`complete_level_passed`、`baseline.*_verified` 和完整交付资格均为 false。零匹配保护目录不代表完整 framework/L1。实际收益与原资格见[P1 验收](archive/2026-11/implementation/acceptance/personal-executor-p1.md)。

## Controlled 分组、固定选测与耗时（2026-11-02）

主归属和固定组合维护在 `tests/catalog.json.controlled_selection`；完整执行集合仍只由原 `controlled_scenarios` 决定。
六组必须无遗漏、无重复地覆盖全部注册测试；固定组合只能引用已注册测试，完整名称可用下列命令只读列出：

```powershell
./scripts/test-controlled.ps1 -List
```

| Group ID | 主要内容 |
| --- | --- |
| foundation | 独立包、服务归属、材料冻结、权限、存储/日志/事件、无服务原生目录及执行配置 |
| workflow | 开发闭环、发布身份/提前发布、工作流选择、审计结果获取 |
| recovery | 未知结果、重启、丢响应、授权撤销、多 Run、创建恢复、人工对账 |
| content | generate、固定仓库调查、conversation、问答/unknown、内容恢复 |
| files | R2、progress、result-file、ColdStop |
| orca-cli | Orca CLI 排队/取消/读取重试 |

| Combination ID | 固定测试集合 |
| --- | --- |
| execute-stop | SingleExecutionColdStop、ResultFile 的 final-seal、NativeUnknownAndStop、RestartPreservesAuthorization |
| runtime-files | ProgressFile、ResultFile |
| content-flow | content 组的五项，加 NativeUnknownAndStop、ManualReconciliation |
| acceptance-control | foundation 与 recovery 两组的全部注册项 |
| result-plain | ResultAcquisition/plain-marker |
| result-audit | ResultAcquisition/audit-zero、audit-positive、audit-offline、audit-report |
| result-parser | ResultAcquisition/parser-unsupported、parser-complete、parser-tool |
| restart-continuation | RestartPreservesAuthorization/graceful、kill |
| restart-create-receipt | RestartPreservesAuthorization/creation_receipt |
| restart-pending-stop | RestartPreservesAuthorization/pending_stop |

表中测试名省略 `TestSR` 前缀。组合是常见改动的固定起点；修改共享事务、授权或原生协议时仍需按实际影响补选，不将组合当作所有改动的充分验证。

```powershell
./scripts/test-controlled.ps1 -Group foundation,recovery -OutDir .evidence/control-basic-new
./scripts/test-controlled.ps1 -Combination execute-stop -OutDir .evidence/execute-stop-new
./scripts/test-controlled.ps1 -Combination runtime-files -RunnerExe <固定Runner绝对路径> -OutDir .evidence/files-new
# 只读统计已有批次；输出必须使用原证据目录之外的新目录。
./scripts/test-controlled.ps1 -FromEvidence <已有foundation证据目录> -OutDir .evidence/timing-import-new
```

入口复用 `test-foundation.ps1`，不运行完整 L1；其原有四项 foundation 单元前置、真实 EXE/受控 peer、独立进程、原超时与清理保持。Group 与 Combination 二选一，支持多个精确 ID、拒绝未知/重复/通配 ID；展开后的测试去重并按原完整目录顺序记录；实际执行顺序沿用原 Go 测试执行器，不按分组重新排序。每次使用新 OutDir，可用 `-TimeoutMinutes 1..60` 选择原 foundation 聚合预算，单项调用期限不变。

`summary.json` 固定记录 `validation_scope=controlled-selection`、`complete_controlled_passed=false`、`qualification_eligible=false`，保存组合/精确测试集合、catalog 摘要及入口总耗时。即使选择六组全部测试，成功也只表示这次选择通过；不生成 `prerequisites.json`，不能用于 `-PrerequisitesFrom`。原 `test.ps1 -Level Controlled`、独立完整前置及 L2/L3/L4 的完整 Controlled 门禁继续执行原完整集合，不读取选测资格。

`timings.json` 只累计 Go 顶层 `--- PASS/FAIL/SKIP: Test… (…s)`，不重复累计子测试；记录逐项状态、主分组、分组已观测数量/缺项、已观测耗时合计、最慢十项和原 log/result/source-identity 摘要。未完成项的耗时为 null，零执行、缺结果、重复记录、未知测试、失败/跳过或精确集合不匹配不能报告选测通过。顶层耗时合计不等于构建/准备/清理在内的入口墙钟耗时；日志精度为 Go 输出的百分之一秒，不用它推导细粒度性能收益。

ResultFile 的实际子场景明细另见下节。报告新增 `subtests`（逐条实际运行/结果，不加入顶层合计）与 `result_file_coverage`；要求子场景回执和日志相互吻合，不能用父测试通过或顶层计数代替。

完整 Controlled 阶段也附加 `controlled/timings.json`，并在阶段 record 中记录路径。它是诊断附件，不增加完整资格的证据清单或门禁；统计不可用时记录 `timing_error`，原完整测试及资格判定仍按原规则执行。旧日志可通过 FromEvidence 只读统计，但不重写旧结果、不补签旧资格、不合并不同构建的通过或成本。

### ResultFile 职责与定向粒度（2026-11-02）

`TestSRResultFile` 仍是完整 Controlled 的一个顶层测试，默认执行下列两类子场景；完整集合见[测试清单](test-catalog.md#当前清单与资格边界)。仅调整测试编排和取证，不修改产品实现、等待预算、四种启用组合或原业务/停止断言。

| 子场景 ID | 保留的独有覆盖 |
| --- | --- |
| optional-file-failure-business-loop | enabled-0..3 四种 progress/result 启用组合；普通文件阻断两个通道目录；每项独立现场到达原 pr_ready、原生输入恰为 3、授权不被文件故障阻断、正式停止；启用 result 时闭合为 version=0 且有诊断，不能伪造字节 |
| final-seal | enabled-0..3 四种健康组合；同服务四 Run 的路径/原生环境隔离、禁用通道不创建目录、种子字节、运行中 pending/不刷新、progress 更新、错误实例查询拒绝；原子替换后的任意字节/大小/SHA/版本/时间、停止一次封存、查询及服务重启不刷新、原输入零重发 |

服务、仓库和 Run 按原流程隔离，不在 case 之间共享可变夹具。

`execute-stop` 固定组合现在只选择 ResultFile 的 `final-seal`；其他顶层测试和其 graceful/kill/creation_receipt/pending_stop 等子场景照常执行。`runtime-files`、files 组和完整 Controlled 仍执行 ResultFile 两类流程。修改共享停止、通道初始化或文件故障机制时，应选择完整文件矩阵；组合并集按较宽覆盖合并，不能因同时选择 execute-stop 就删掉故障流程。

```powershell
# execute/1 停止及封存；ResultFile 只选择 final-seal。
./scripts/test-controlled.ps1 -Combination execute-stop -OutDir .evidence/execute-seal-new
# 共享停止/文件改动：并集保留两类 ResultFile 和 ProgressFile。
./scripts/test-controlled.ps1 -Combination execute-stop,runtime-files -OutDir .evidence/shared-stop-new
# 显式覆盖只接受这两个精确 ID；可按现有 PowerShell 数组方式同时指定两个。
./scripts/test-controlled.ps1 -Group files -ResultFileCase optional-file-failure-business-loop -OutDir .evidence/file-faults-new
# 单个父测试的底层定向入口；沿原 foundation 机制等待独立进程与 result.json 后再核验结果。
./scripts/test-foundation.ps1 -Pattern '^TestSRResultFile$' -ResultFileCase final-seal -OutDir .evidence/result-seal-new
```

`-ResultFileCase` 仅作用于 ResultFile，保留为兼容入口；新的选测入口统一解析为按父测试的 Case 计划，foundation 仍兼容旧 `-result-file-cases` 标志。不使用全局 Go 子路径过滤。未知/重复 ID、选择集合没有 ResultFile、与 List/FromEvidence 混用均拒绝。同一父测试不能同时用 Case 与 ResultFileCase。未收窄的 foundation/full 入口仍执行两类。

每次新执行在 `result.json.result_file_cases` 记录实际执行的 ID、成功布尔和包含清理的耗时；`source-identity.json` 保存显式选择，选测 summary 保存解析后的精确集合。Go 端拒绝选中的父场景或 enabled 变体未执行；公开选测和完整前置另外核验对应 RUN/PASS、四个 enabled 叶例、重复/跳过/失败/多余场景及实际回执，完整前置必须具备两类实际覆盖。定向执行全部父测试但缺其中一类仍不能取得完整资格。

历史日志缺新子场景回执时只能只读统计耗时，`result_file_coverage.status=unverified_import`，不补写实际覆盖或完整资格。子场景耗时用于说明成本分布，不与父测试时间重复累计。

### ResultAcquisition 断言对照（2026-11-02）

本轮先以固定 Runner 复测 `TestSRResultAcquisition` 的原八项，再调整定向编排。下表按现有测试源码登记，不要求真实模型制造审计发现，也不将 parser 的接线验证替换为纯文本单元测试。

| 职责 | 子场景 | 原生 turn/start 预算 | 原有独有断言及共同收尾 |
| --- | --- | --- | --- |
| 普通结果获取 | plain-marker | 2 | 普通开发到达 pr_ready；一个原 thread；保留完整工作流和原资源清理 |
| 审计结果 | audit-zero | 3 | 显式 executor/reviewer 模型准备；唯一审计结果，SourceResultID/CommandID/TurnID 与最终 PR head 来源 |
| 审计修复 | audit-positive | 3 | 上述审计来源；pre/post head 不同且 fixed_count=1 |
| 离线审计 | audit-offline | 3 | 审计活动时停服；等待原 owner 至少两个 checkpoint；离线时 turn/start 仍为 2；原服务换代后原流程继续 |
| 一次补报 | audit-report | 4 | 原审计以纯文本返回后补报；审计来源及单 thread/精确 turn 预算 |
| parser 后备 | parser-unsupported | 4 | 保留审计工作流；llm_parser 有 failed attempt，不能隐藏不可用资格；原流程后备完成 |
| parser 成功 | parser-complete | 3 | 保留审计工作流；实际结果 Method=llm_parser、Status=satisfied |
| 工具事件回退 | parser-tool | 4 | 原 PARSER_TOOL 夹具输入、审计来源、补报回退后的精确预算；不增设真实工具调用或模型要求 |

八项共同保留独立现场、公开服务/submit、最终 pr_ready、thread/start=1、原权限/模型夹具、停止及服务清理。各操作和整流程的既有期限不延长。已清理的 parser-restart/parser-kill/parser-stop 条件不在原八项常量列表内，且循环变量未被重写；只删除该不可达测试分支及其专属导入，不删除业务 owner 的恢复/停止义务。

本轮将 ResultFile 的回执/选择机制与 ResultAcquisition 共用，使用 `-Case <父测试/子场景>` 和 Go driver 的 `-controlled-cases`，支持的父测试受登记清单限制，包含 ResultFile、ResultAcquisition 和下节 Restart。每个父测试独立解析，未指定的父测试仍运行自己的完整集合；不会以全局 `-test.run` 子路径过滤其他测试。原四个组合保持，结果获取增量新增三项职责组合；加上下节 Restart 三项，当前共十个固定组合；workflow 组和完整 Controlled 的 ResultAcquisition 仍执行全部八项。

```powershell
./scripts/test-controlled.ps1 -Combination result-plain -OutDir .evidence/result-plain-new
./scripts/test-controlled.ps1 -Combination result-audit -OutDir .evidence/result-audit-new
./scripts/test-controlled.ps1 -Combination result-parser -OutDir .evidence/result-parser-new
# 合并三类，得到原八项；不能据此获取完整 Controlled 资格。
./scripts/test-controlled.ps1 -Combination result-plain,result-audit,result-parser -OutDir .evidence/result-all-new
# 每个父测试单独选择；Restart 等同批测试不受到子路径过滤。
./scripts/test-controlled.ps1 -Combination result-plain,execute-stop -OutDir .evidence/result-mixed-new
# 显式选择只接受已登记的精确 parent/case；父测试须在组合/组中。
./scripts/test-controlled.ps1 -Combination result-parser -Case TestSRResultAcquisition/parser-complete -OutDir .evidence/parser-one-new
```

不同组合对同一父测试取并集；任一所选组/组合要求全量时保留全量。显式 Case 仅覆盖其父测试的选择，未知父测试、未知/重复 case、通配符、额外路径层级、缺父测试和旧参数冲突均拒绝。`summary.selection.cases` 与 `source-identity.controlled_cases` 记录实际请求，`result.json.controlled_cases` 按父测试记录实际完成 ID、成功布尔和包含清理的耗时。ResultFile 的原 result_file_cases 回执保留，存在通用回执时两者必须一致。

`timings.case_coverage` 按父测试核实实际回执与 RUN/PASS 路径，ResultFile 还要求全部选定 enabled 叶例。完整资格须具备 ResultFile 两类、ResultAcquisition 八项及 Restart 四项，不能仅凭全部父测试通过；所有定向入口仍不生成前置证明、完整资格标志始终 false。历史 ResultAcquisition 日志没有新回执时只统计成本、标 unverified_import，不补签覆盖。子场景和父测试时间不重复累计。

### Restart 职责与断言对照（2026-11-02）

`TestSRRestartPreservesAuthorization` 保持一个顶层注册、默认四项，先复测原完整集合，再按职责提供定向选择。原 `execute-stop`、recovery 组及完整 Controlled 保持四项；完整注册数量见[测试清单](test-catalog.md#当前清单与资格边界)，本节只定义选测粒度。

| 职责 | 子场景 ID | 原有独有断言与原生预算 |
| --- | --- | --- |
| 原会话换代继续 | graceful、kill | 活动回合时分别优雅停服/强杀；原 owner/native 仍存活；离线原 final 完成且无后继输入；同构建换代沿原 Session 到达 pr_ready，thread/start=1、turn/start=3；显式停止后再次换代，授权关闭、ResumeRequired、revision 不变且零新输入 |
| 创建回执恢复 | creation_receipt | 原 thread/start 已发生但 SessionID 未提交时强杀；原创建完成后换代，不要求 resume、不换 session、不重建或重发；最终 pr_ready、thread/start=1、turn/start=3 |
| 冷启动停止优先级 | pending_stop | 活动原回合时离线，以原 run/revision/请求 ID 传 PendingStops；原回合完成也不得派发后继；重复携带已受理、revision 已旧的同一请求再换代，原请求仍可查且 turn/start=1 |

四项保留独立现场、公开 EXE/CLI、原进程身份核验与资源清理。graceful/kill 退出方式及创建/停止窗口各有独有断言，保留全部四项，不合并故障窗口，不修改产品等待期限或缩短必要观察。

```powershell
./scripts/test-controlled.ps1 -Combination restart-continuation -OutDir .evidence/restart-session-new
./scripts/test-controlled.ps1 -Combination restart-create-receipt -OutDir .evidence/restart-create-new
./scripts/test-controlled.ps1 -Combination restart-pending-stop -OutDir .evidence/restart-stop-new
# 三类并集恢复默认四项；现有 execute-stop 或 recovery 组也要求四项。
./scripts/test-controlled.ps1 -Combination restart-continuation,restart-create-receipt,restart-pending-stop -OutDir .evidence/restart-all-new
# 显式覆盖仅作用于对应父测试，ResultFile 仍按 execute-stop 运行 final-seal。
./scripts/test-controlled.ps1 -Combination execute-stop -Case TestSRRestartPreservesAuthorization/pending_stop -OutDir .evidence/startup-stop-new
```

沿用按父测试的 Case 计划、实际完成回执和包含清理的子场景耗时；每个选中 ID 都必须在回执和 RUN/PASS 路径中精确出现。三类组合取并集，原 execute-stop、acceptance-control、recovery 及完整前置默认不收窄；显式 Case 可覆盖对应父测试。完整门禁除顶层集合外还要求 Restart 四项，只有父测试通过、顶层数量齐全或部分回执不能代替；单独核验 Restart 时也执行四项门禁。定向结果的完整资格标志保持 false，不生成前置证明。旧无回执日志只统计成本并标 unverified_import，不补签覆盖。



原版/新版耗时、样本波动、固定 Runner 摘要、断言对照和各轮实际资格见[精简前记录](archive/2026-11/maintenance/testing.before-consolidation.md#controlled-分组固定选测与耗时2026-11-02)。历史样本不作为当前性能或完整资格。

## 固定交付包验证（2026-10-26）

`scripts/test-delivery.ps1 -PackageDir <完整包目录> -OutDir <新的包外证据目录>` 固定使用包内 Runner、helper 和 Viewer，不重建替代 Runner。它复用 installation/client/viewer 和 `TestSRProgressFile`，验证清单、隔离安装/篡改、受控调用、原生窗口及原始字节/重启零重发。

`qualification.json` 的 scope 为 `controlled-client-delivery`，绑定完整包清单/内容、harness 提交/源码、四项结果和进度原快照/日志/进程/EXE 身份。失败或缺报告不能通过；报告位于包外，结束后复核包和 harness 未变化。`scripts/verify-client-package.ps1 -PackageDir <目录> -Qualification <报告>` 复验绑定，文件或证据变化拒绝。

此资格不替代 L1、完整 Controlled 或 L2/L3 真机资格，也不验证调用方测试进度字段/页面。[Task18 的历史 0/6](archive/2026-11/implementation/acceptance/task18-runner-retest-20261026.md#测试进度显示的独立负例)仍保留。framework 的 `delivery-test.ps1` 使用合成包验证反例，不启动模型、服务或 Viewer。

`test-foundation.ps1 -RunnerExe` 与 `-PreparedArtifactsDirectory` 为互斥来源，前者不重建 Runner，后者校验复用产物摘要；`test-client.ps1 -ClientHelper` 可固定脚本。清单及源码按 ordinal 排序，宿主与 Windows PowerShell 5.1 必须得到相同包摘要。

## 独立测试配置与迁移

使用[配置示例](../tests/test-config.example.json)和[外层 schema](../tests/test-config.schema.json)，当前为 `steerrunner-test-config/v1`。`configurations` 各项 `{execution, workflow?}` 复用公开生产字段和纯 gate，需求/角色/配置引用由[唯一 case 注册](../internal/testkit/livetest/cases.json)维护。每个后端提供完整配置，不继承另一配置的模型、权限或目标。

`fixture` 声明专属 SteerlineTest repository/publication，当前 case 使用 GitHub command publication；生成器不自动改选 provider。`harness` 只含 admin_host。文件只从显式 Config 读取：入口相对路径按 cwd，harness 相对路径按原文件目录解析。批次冻结规范化配置、摘要和 Runner 构建，worker 不重读原文件。

根结构、版本、未知外层字段或重复键错误退出 2；单项无效仅使依赖 case unavailable。旧 live-config/v1/v2 手动迁移，旧冻结现场仍由原构建结算。静态可用性只判断配置是否足够尝试，不查询登录、程序、模型或连接，不构建、不启动 L1/服务/调度锁/现场：

```powershell
.\scripts\build.ps1
go build -trimpath -o bin/test-config.exe ./tests/helpers/native
.\scripts\test.ps1 -Level L3 -Config .evidence/test-config.json -ListAvailability
```

可用 `-RunnerExe` / `-ConfigHelper` 固定已有构建，Runner 的 `.build.json` 随包提供；列表成功退出 0 不是测试通过，缺产物或损坏退出 2。缺本机 Codex 基线不阻止无依赖的 L1、Controlled 或其他已配置 L3。

开发显式 `full_access`，内容流程及 parser 沿各自 read_only 合同，认证使用执行主机已有登录。新 Orca Agent Run 使用结构化配置，旧 Agent executable/非空 args/helper 路径和摘要拒绝；传输 executable/args 仍属合法传输配置。支持组合和停止限制以[Orca 设计](orca-execution.md)及[配置 Spec](spec/independent-configuration.spec.md)为准，不复制旧 helper 配置示例。不同权限、模型或目标的通过结果分别登记。

## 当前完整 smoke 契约

L2 的 `local-codex.smoke` 验证 workflow/2 最小交付；parser 专项在 L3。固定材料由本仓 `tests/fixtures/evidence-smoke/` 维护，源自 SteerlineTest commit `cebdd5ab552150c49743e40593c836689ecf789b` 的 native-101，只更新 `evidence/native-101.md`，读取实际 UTC、运行 checker 并提交。

1. 新建 `C:/srt/<随机ID>`（可用 FixtureParent），冻结 Runner/Spec/Plan、配置、来源及专属测试 base；业务提交/发布由原 worker 执行。
2. 克隆固定源提交，提交并推送本轮材料到测试 base；远端准备保存原 runtime、fetch/cat-file 事实及准备终端关闭回执。
3. 通过公开 CLI submit，使用生产默认实施/报告/发布提示词，不加载测试发布器；Runner 执行、必要时一次同源 report、发布并核验 `pr_ready`，L2 不启动独立 parser。
4. Driver 从 GitHub 获取实际 PR objects，核验身份、原 final/来源、仅目标文件变化、精确格式及新鲜时间。L2 的无模型 Git 前置检查 loose/packed refs 提交无 stderr 权限错误且 peer canary 不变；HTTP 401/403 或权限拒绝不能被退出码 0 掩盖。
5. 在合入前验证原 pr_ready 后反馈拒绝，revision/Turn/PR head 不变，不创建返工轮；开发后续 `message` 按独立单轮合同验证，不能冒充反馈返工。
6. Driver 按精确 head 合入本轮专属测试 base，保存 `merge-platform.json`、`merge-bound-run.json`；fetch 后核验 merge、祖先关系和实际内容，并在合入 checkout 重跑 checker。
7. 显式 stop 原 Run，核实 `new_inputs_allowed=false`、`stop_status=stopped`，再停止服务至 not_running；保留原工作区、分支、PR 和现场供审计。

普通远端 smoke 同样合入与收尾；audited/multirun 按清单验证真实 PR 并保留未合入。审计实际模型切换须由原 session/turn 的 history 证明，不能只看配置。合并由测试 driver 执行，Runner 生产里程碑仍是 pr_ready，不增加生产自动合并行为。

## 证据与失败

### 独立前置与显式 L2 复用

默认 test.ps1 运行完整前置；显式复用只支持单级 L2、默认 catalog，不能与 Focused、ListAvailability、L3/L4 或混合级别合用。

```powershell
.\scripts\test-prerequisites.ps1 -Stage All -OutDir C:\srt-evidence\prereq-new
.\scripts\test.ps1 -Level L2 -Config .evidence/test-config.json `
  -PrerequisitesFrom C:\srt-evidence\prereq-new -OutDir C:\srt-evidence\l2-new
```

来源必须是同机、同账号/工作副本、输入及产物/证据匹配的新格式 l1-core 与完整 Controlled 直接成功。共享源码、脚本、依赖、目录/注册及参与 L1 的 docs JSON 改动使其失效；无关文档或 native-only 改动按实际指纹判断。历史不补签，单阶段或 Pattern 不拼装完整前置；失配明确拒绝，不自动补跑。

本轮仍重新评估配置/可用性、新编译/guard driver、创建新现场并完整验证 Git、真实发布/合并/审计与停止。后续 native 失败不改写已封存前置，失败批次仍 failed。边界见[Spec](archive/2026-11/implementation/spec/validation-reuse.spec.md) / [验收](archive/2026-11/implementation/acceptance/validation-reuse.md)。

### 批次报告

`summary.json` 保存原请求范围、各 case 的 availability/原因和每级 registered/requested/excluded/unavailable/executed/passed/missing 集合、repeat、`complete_level_passed`、来源及现场路径。不可用项零派发，独立已配项仍运行；范围内有不可用项则 incomplete、退出 1，实际失败优先为 failed。Scenario 子集通过不等于全级通过，前置失败后未派发项不计执行。

manifest 冻结输入；每个现场保存 result/scenario、CLI journal、Git/GitHub/checker 调用、PR/合并/停止快照和原执行记录。模型 token 以原生来源可提供的真实数据为准。case 数必须非零且精确，无意外 skip；L1 仅允许[脚本逐名登记](../scripts/test-prerequisite-contract.ps1)的默认 skip，遗留 live opt-in 在执行前拒绝，skip 不计真机通过。

通过须满足冻结预期、故障命中、原身份/来源、实际产物和完整收尾。操作 unknown、超时、缺 final 或未核实清理不能报告成功；`revoked-unverified` 仅按其冻结故障预期判定，不能算停止确认或业务交付。Orca 已核实原回合 done 且完整 final 可归属时，可省略的 native outcome 按原值保留，与副作用未对账的 unknown 分开判断。

动作前保存阶段及 Run/PR 身份；异常退出缺结果为失败，按原 state-dir/request_id/Run 对账并停止。不得重发原生输入或换会话弥补缺证据。修复后使用新现场，旧失败保持。阶段切换可用自己的准备预算；未决原操作不因重试/重启延长期限。

单步诊断可用 `new-test-fixture.ps1` 和 `test-native.ps1 -Manifest`，应已有前置通过证据；正式分级使用 test.ps1。审计诊断按配置指定不同执行/审计模型，原实际 PR 保留未合入，停止原资源；诊断不计普通 L2 完整 smoke。

## 扩展场景与 L3 稳定性入口

新增/删除/拆分 case 时在同一变更更新[目录](../tests/catalog.json)、[case 注册](../internal/testkit/livetest/cases.json)、driver、fixture、清单和断言。每项记录独有目标、固定预期、层级、入口、资源/互斥范围、实现和缺口；规划不计注册或通过。来源/PR/身份错误须有受控反例，不能用模拟目标代替真实环境资格。

稳定性注册的故障窗口和独有断言保留。部分旧终端故障夹具尚未迁完，现行适用边界见[测试清单](test-catalog.md#当前接线与资格缺口)，旧 36/36 或已注册不代表当前全 L3 通过。

需管理员网络故障的 case 使用独立管理员终端运行 `scripts/stability-admin-host.ps1 -Directory C:/srt-admin`，主测试保持普通用户，配置 `harness.admin_host` 指向该全新目录。宿主只接受固定现场的 block/block-state/restore，核验身份、回执及 watchdog，最长十二小时；恢复全部故障后写 stop 文件结束。专属故障只能作用于自己的 Runner/设施。

`-Repeat 3 -Level L3` 对当前完整注册重复运行，前置每批完整一次。`-SkipAdminRequired` 仅供明确选择排除管理员 case；保存排除项和 passed_with_exclusions，不能声称完整 L3。故障命中、原操作调用数、零重发及实际停止均须证据；更细窗口见[稳定性 Spec](archive/2026-11/implementation/spec/l3-execution-stability.spec.md)及清单。
