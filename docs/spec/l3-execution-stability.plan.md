# Plan：L3 执行稳定性验收

日期：2026-10-15。状态：实施完成；2026-10-16 同一构建完整三轮真机资格验收通过（36/36），无排除。用户已授权实施及全部 L3 真机执行，取代最初仅交付文档的范围。结果与失败历史见[专属验收](../acceptance/l3-execution-stability.md)。

依据 [Spec](l3-execution-stability.spec.md)。首次实施输入基线为 SteerRunner `57d6f6f2455b6a748b56aafc75668915bd1d87eb`；实施前核对最新工作区及冻结文档摘要。生产恢复与停止规则继续归 [Runner Spec](steerrunner.spec.md)，测试扩展不另立执行权威。

## 1. 当前实现与改动边界

| 位置 | 现状 | 本批改动 |
| --- | --- | --- |
| `tests/catalog.json` | L3 仅 smoke；每条目录项生成一个现场 | 增加五个场景及固定 case 定义，精确展开十二 case |
| `scripts/test.ps1` | L1/Controlled 前置，逐场景顺序执行 | live case/repeat 展开、准确计数、逐次结果和统计；不加入并发调度 |
| `scripts/new-test-fixture.ps1` | 仅接受 development-smoke | 支持稳定性 driver、case/重复身份、冻结设施配置和预期结果 |
| `scripts/test-native.ps1` | 所有场景硬性要求 merge_commit、run_stopped | 保留 smoke 严格条件，按冻结 case 契约校验新增结果及独立清理 |
| `internal/testkit/livetest` | manifest、产物、PR/final 校验 | 加入版本化故障 manifest、case 契约、证据/结果校验和计数归并 |
| `tests/native/run_test.go` | 一个 driver；waitReady 遇恢复/未知即失败；cleanup 绑定原服务实例 | 提取共享 CLI/PR/证据能力，新增稳定性 driver；维护服务实例历史并准确收尾 |
| `tests/fixtures/evidence-smoke` | 微型产物、checker、publisher | 保留原夹具；新增稳定性夹具和有界测试门、发布调用审计 |
| Orca native state 通路 | CLI 之外直接订阅/查询原生状态 | 注入与证据必须覆盖直接连接，不能仅包装 CLI |
| 生产 stop 对账（真机发现） | 关闭前断连后落入 unsupported terminal reconciliation，不能继续原停止操作 | 原绑定回执持久保存停止进度；明确未尝试关闭才继续首次关闭，结果未知则不重做。适配器、SQLite 重载和错身份反例先行 |

新增代码优先放 `internal/testkit/livetest`、`tests/helpers`、`tests/fixtures/evidence-stability` 和 `tests/native`。生产包不导入 testkit，不新增生产 fault flag 或假 adapter。若真机暴露生产缺陷，先补对应 owner 的受控回归，修复既有行为后重新构建独立现场；不弱化验收来获得通过。

## 2. 实施步骤

### P0 固定 case 契约与技术前置

1. 将 Spec §2 的六场景、十二 case、预期结果、预算、必需能力固化为测试注册表。目录选择与结果判定使用同一经校验的 case 定义；结果中的 expected_outcome 不能反向决定是否通过。
2. 新建 `docs/acceptance/l3-execution-stability.md`，登记 L3-A01–A09、当前待实施状态、命令、来源、必跑 case、工具版本和证据路径。不得迁入历史真机通过结论。
3. 在 Windows 实施并验证本轮专属进程/通信控制设施。网络注入优先使用按绝对程序路径绑定的测试专属 Windows 防火墙规则：仅本轮 Runner EXE 和本轮独占 Orca CLI 安装路径，覆盖到冻结远端 endpoint 的实际连接；明确记录地址解析、连接终止和失败探针。若仅新增规则不能终止已有连接，补充同范围连接关闭能力并证明断开，不能据规则存在即宣称注入成功。
4. Orca CLI 使用独立完整运行文件集，不能直接针对用户共享安装路径加规则。运行文件来源、依赖和版本哈希固定；认证沿用原生解析，不拷贝凭据。只读诊断使用不受注入影响的原生 CLI 通路，严格绑定本轮资源。禁止修改共享 Orca 配置、整机网络或关闭防火墙服务。规则使用随机 fixture ID 命名，创建前登记、恢复按完整规则身份精确删除，不批量清规则。
5. 前置探针验证规则管理权限、独占 CLI 可运行、直接状态连接与 CLI 双通路确实失效和恢复、原 runtime 仍存活、原生调用审计可取得、远端控制门可以按原身份释放。探针使用专属非业务资源并完整清理；前置不满足即所选场景失败。机制必须在受控环境验证后才能用于真实业务 Run，不把选择注入机制留给 live 运行时猜测。
6. 实施阶段如不能按上述路径可靠切断并恢复双通路，保持相应 case 失败/未验，先修订 Spec/Plan 和受控验证，不用 CLI 错误包装冒充断网。无需为了此问题增加生产专用状态注入 API。

### P1 扩展冻结现场、入口和结果模型

1. 目录保留 `steerrunner-tests/v1`，新增可选 `cases`，缺省只允许既有 smoke 的 default case。新增字段由严格注册表校验；重复/未知 case、driver/target 冲突、缺变体、非法预算在模型启动前失败。旧自定义目录不自动获得稳定性通过范围。
2. 稳定性现场使用 `steerrunner-test/v2` manifest，含 case_id、repeat_index、预期结果、预算、注入/控制/审计工具路径及摘要、目标与规则作用范围。现有 v1 smoke 仍可解析并维持原判定；不对旧 manifest 推断新的故障配置。配置保留既有 v1 smoke 用法，新增稳定性配置版本/示例，未知字段明确失败；秘密只允许原生外部引用。
3. `test.ps1` 新增 `-Repeat`（默认 1、1–10）。按照 repeat→场景→case 顺序生成独立现场并逐次执行；每批先完整执行一次 L1/Controlled，任何前置失败都不准备 live base。非 live 调用使用非默认 Repeat 明确拒绝，避免静默忽略。重复序号不复用 fixture ID、request ID 或 PR。重启和发布缺口 case 冻结总期限 45 分钟，其余 30 分钟；所有阶段/设施期限按 Spec §6 核对可容纳性。
4. `new-test-fixture.ps1` 接收经注册表核对的 `-Case` 与 `-RepeatIndex`，默认值仅服务旧 smoke；无 case 的多变体场景明确拒绝。先做无副作用配置/能力检查，再生成现场、冻结材料、准备真实 base；每次使用独立 `codex/srtest/<ID>/base`。
5. `test-native.ps1` 根据 manifest 版本和 case 注册表验证结构化结果。每个现场只运行一个 case，保留 `scenario_count=1` 门槛并增加 case_id/repeat_index 一致性核对。非交付 case 不检查 merge_commit，但所有 case 都要求完整断言、故障命中和资源/设施清理确认。
6. 新增结果字段区分 test_outcome、business_outcome、assertions、faults、Runner stop 和 driver cleanup。原 smoke 的 v1 passed/run_stopped/service_stopped/merge_commit 路径保留；新结果不能仅给 passed=true 就通过。summary 记录选中/计划/实际 case 执行数、所有现场和首次通过率，失败仍继续其他独立 case，但不得无记录漏跑。

### P2 建设真实透明设施与共享 driver

1. 共享 driver 提供公开 CLI 的有界调用、回执找回、inspect/events 快照、服务实例历史、原资源定位、PR/Git 检查和精确收尾。不复用原 waitReady 的“遇门槛就失败”逻辑判断故障窗口；由 case 状态机决定允许的中间事实。
2. 测试调用方 helper 负责启动真实 submit、保存完整信封、抑制向 driver 交付响应并在接受屏障后等待。driver 从独立 CLI 找回已接受请求后终止 helper/所属 submit；如果无法证明调用方确实仍活着并被结束，case 失败。helper 不传递结束信号给服务。
3. Orca CLI 透明审计 helper 通过已有 transport.executable/args 接入真实 CLI，记录脱敏的输入摘要、原生请求/回执及调用顺序，不改变原协议成功/失败事实。执行与状态查询的真实原生证据一起归并，能区分 start/send/close、诊断和阶段输入；不能将业务发送意图直接当作原生实际收到。
4. 进程控制 helper 使用 service status 返回的 PID、creation time、instance_id、EXE 绝对路径/hash 和 state-dir 核对目标；不按进程名杀进程。正常停服携带当前 expected_instance_id，重启后的 cleanup 必须针对新实例。旧进程退出未确认不启动替代写者。
5. 稳定性夹具沿用单文件 UTC 产物，checker 和 publisher 增加有界门及调用审计；辅助文件放本轮工作区的独立控制目录，避免提交到业务 diff。远端路径由实际 workspace 身份解析，控制命令经独立辅助终端操作；每条操作绑定 runtime/worktree/terminal incarnation，保留实际完成证据。辅助终端关闭确认也进入清理结果。
6. publisher 在实际 PR 查询确认后保存身份并等待；释放后输出真实 URL。PR 查询故障通过本轮 provider query 包装器转发真实查询或制造明确传输失败；不能伪造 PR 内容。它只影响本轮 Runner 查询，不影响独立 driver 核验。发布脚本审计区分调用、push、create/reuse 和错误。
7. 原子写入 checkpoint/result，journal 追加序号和时间。每次有副作用动作先写意图再执行，出现结果未知只查原身份。统一保留计数证据、设施状态及清理缺口。对“没有重复”建立整个观察窗的完整审计，不能仅比较最终 PR 数。

### P3 实现五个场景 driver

按如下顺序完成；每个 driver 均先用隔离协议/provider 夹具覆盖时序与判定，再进入真机。

| 顺序 | 场景与 case | 实施重点 | 关联验收 |
| --- | --- | --- | --- |
| 1 | caller-exit，1 case | 接受屏障、结束调用方、原请求找回与重试、原服务和唯一 Run、原生发送计数 | A03 |
| 2 | connection-reconnect，1 case | active 门、双通路中断/恢复、实例不换代、不新增恢复门槛、原回合零重发 | A04 |
| 3 | service-restart，4 case | graceful/kill × active/boundary；旧写者结束、人工继续门槛、授权前零输入、一次显式 resume | A05 |
| 4 | stop-revocation，3 case | active/boundary 真实停止、迟到结果；不可达 stop 跨重启撤销持久、恢复后原身份收尾 | A06 |
| 5 | publication-unknown，2 case | 真实 PR 已创建、原完成证据缺口、查询失败、原 final/PR 对账和零重复发布 | A07 |

所有成功交付 case 复用 CheckPR、固定材料/真实 final 校验、Git fetch 内容检查、精确 head merge 和 merged 事实校验。仅原 smoke 运行反馈轮。停止场景在实施阶段注入，独立查询专属 base/head 的 PR 列表确认没有意外发布。

`boundary` 必须在下一阶段业务输入尚未发出时命中：使用阶段完成前的测试门及阻断观察安排顺序，并用原生输入审计核实；不能在外部包装器已经接到下一输入后，假装该输入仍未发送。stop 前已送出的输入列为在途事实，但无法满足本 case 屏障则仍判未命中失败。

### P4 补齐反例、超时和异常收尾

1. 扩展现有 `tests/framework` 与 `internal/testkit/livetest` 的有意义反例：缺 case、重复 case、零场景、Repeat 越界、旧 manifest 被误当新场景、driver/预期不匹配、结果伪称 passed、清理缺失和计数缺失。L1 的 live driver 编译/零匹配门槛继续生效。
2. 受控验证注入器错 PID/creation time、错服务实例、错 runtime/incarnation、未命中屏障、只断 CLI、直接查询仍通、故障恢复未确认、规则删除失败、控制门超时。要求明确失败，零越界动作。
3. 受控验证各状态机的关键失败分支：resume 前发新输入、替换原会话、重复 native send、旧回合 final 污染、重复 publisher/create、错误 PR/head、陈旧 UTC、unknown 被记为 stopped、stop 后迟到事实驱动后继。保持预期 outcome 固定，不容许“恢复失败但阻断正确”挽救正向 case。
4. 使用真实 EXE/CLI/IPC/SQLite 的 Controlled 场景验证换代、回执找回、撤销持久及计数。单纯测试结果 JSON 不代替时序测试；不用受控通过宣称真机通过。新增集成 Test 显式注册到 controlled_scenarios，L2/L3 前置覆盖其完整集合。
5. driver 采用 case 总 deadline 和预留收尾 deadline，Go harness timeout 作为最终边界。网络规则/测试门由独立有界 watchdog 持有恢复意图，driver 异常时只撤除本轮设施，不代发业务输入或 resume。硬超时后的修复另记 recovery journal，原 case 永久失败。
6. 清理顺序：保存故障窗口断言和快照→恢复本轮通信/释放或结束控制设施→对原 Run 提交/对账撤销→确认原 owned 活动已停止（必要时单列 driver 原身份清理）→停止当前服务实例并确认退出→确认所有规则/辅助进程/终端无活动。任何未知都登记失败，不删除业务现场或 Git/PR 证据。

### P5 真机验收与文档收口

1. 更新 `docs/testing.md`、README 测试入口和专属验收登记：说明场景/case/repeat 数量、子集语义、正向交付与预期停止的差异、依赖与失败恢复方法。更新旧文档中“全部 L3 必须合入”的当前说明，保留历史运行记录，不改写其结果。
2. 完整通过 L1/Controlled 后先逐场景真机各跑一轮，诊断设施和 case 实际命中。失败留原现场，修复后新构建、新批次；不拿不同版本的单项结果拼资格结论。
3. 同一最终构建执行完整 L3 `-Repeat 3`。现场可各自保留 EXE 副本，但必须校验三十六次 Runner/driver/helper 的构建摘要和固定配置一致；中途源码或工具变化立即中止本批资格判定，保留已执行记录。登记三十六次结果、首次通过率、实施/恢复/停止/收尾耗时及全部失败；只有完整连续三轮通过才能完成首批验收。
4. 重验 L2 本机 smoke，证明共享 manifest/结果校验和 driver 提取不破坏原本机真实交付。L2 独立记录，不与 L3 合成通过率。

## 3. 验证入口与证据

以下入口及 Repeat/case 展开已经接通；是否取得真机通过证据以专属验收登记为准，不将注册和编译通过视作完整资格。cwd 均为仓库根目录，终端为 PowerShell 7、Windows/amd64。管理员网络宿主的启动及配置见[测试说明](../testing.md)。

```powershell
# 纯规则/受控前置；不得启动真实模型。
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled

# 新增场景各跑一轮：示例选择恢复场景，其四个 case 全跑。
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.service-restart -Repeat 1 -Config .evidence/live-stability-config.json

# 最终同一构建的完整资格验收：6 场景合计 12 case，每轮全跑，3 轮共 36 次。
.\scripts\test.ps1 -Level L3 -Repeat 3 -Config .evidence/live-stability-config.json

# 原本机交付回归，沿用本机原生配置。
.\scripts\test.ps1 -Level L2 -Config .evidence/live-config.json
```

上方完整资格计数为每轮十二 case、三轮三十六次，六场景不再额外乘入。完整批次持续时间取决于真实模型，每个 case 使用 Spec §6 的独立预算，不将整批限制成一个 30 分钟超时。

| 检查组 | 执行时点与范围 | 通过条件 | 证据登记 |
| --- | --- | --- | --- |
| D1 文档 | 本轮及实现收口；Spec/Plan 链接、case/验收映射、git diff --check | 链接有效、十二 case/三十六执行一致、命令标明目标状态 | 本轮工具输出；实现时写专属验收 |
| U1 规则/脚本 | P1/P2/P4 后，正式 live 前；L1 入口 | 新配置/目录/结果/计数反例通过，native driver 可编译且零匹配失败 | 批次 framework、l1.jsonl、compile/guard 结果 |
| C1 协议与时序 | P2–P4 后，正式 live 前；Controlled 入口 | 各屏障、故障生效、换代/停止/发布时序与异常收尾反例通过；非零且精确完整执行 | controlled 日志、回执、side-effect/fault journal |
| R1 单轮真机 | P3/P4 完成后；分别选择新增五场景、全部变体 | 十一个故障 case 命中并按冻结契约通过；未通过不计资格完成 | 各新 fixture manifest/evidence 与批次 summary |
| R2 完整资格 | 最终同一源码/配置/构建，L3 Repeat=3 | 六场景十二 case 每轮完整，共三十六次全通过；清理全部确认 | 原始 summary、每次 journal/产物/PR/merged 或停止证据 |
| R3 L2 回归 | 共享代码最终版本，L2 入口 | 原本机完整 smoke 保持通过 | 独立 L2 批次及原 smoke 证据 |

summary 位于 `.evidence/tests-*/summary.json`；每次现场位于 `C:/srt/<ID>` 或显式 FixtureParent。每个新 case 的 evidence 至少包含 `source-identity.json`、`faults.jsonl`、`native-calls.jsonl`、`caller-commands.jsonl`、状态/事件快照、`cleanup.json`、`scenario.json` 和 `result.json`；有 PR 的 case 增加原生 final、PR 列表、fetched Git 检查及合并证据。完整秘密不进入任何登记。

## 4. 完成标准与本轮交付

实施完成要求 P0–P5 全部落实，L3-A01–A09 均有对应实测证据，R2 完整三轮及 R3 通过，README/testing 与实际脚本一致。工具权限或真机能力不足如实登记失败/未验，不视为任务完成。

首轮仅交付 Spec/Plan；当前继续实施验收文档、脚本、driver、测试设施及真实运行，最终完成仍须满足上段标准。多轮反馈、竞争请求、多 Run 隔离和长时间稳定性保留为下一批，不混入本批验收数量。
