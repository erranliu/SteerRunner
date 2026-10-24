# 运行时单文件汇报验收（2026-10-23）

**当前审计修复构建见 [PR #55 审计验收](runtime-progress-pr55-review.md)**：r16 L1 651、Controlled 23/23、真实 L2 2/2、五项真实 L3 5/5、实际并行和原资源结算全部通过。以下 r14/r12 及更早记录继续保留各自当时的构建和结论。

**最新主线整合验收（2026-10-24）见 [PR #55 整合记录](runtime-progress-integration.md)**：r14 L1 646、Controlled 23/23、真实 L2 2/2、五项真实 L3 5/5 和严格实际并行全部通过；原六个 Run 资源结算完成。r13 暂态采集断言失败及随后修正、双方功能保留和全部原始证据分别记录。以下 r12 与更早章节保留当时的构建和结论，不代表合入最新 main 后的构建身份。

**r12 全部通过**：同一冻结构建完成 L1 611、完整 Controlled 23/23、两项真实 L2、五项新增真实 L3，实际并行验证通过。正式批次于北京时间 20:53–21:36 执行，MaxParallel=7，零授权排除、零缺配置；没有用受控、旧构建或部分通过替代本轮。本次只修改 Runner，SteerLine 接线、项目生产测试入口和 Plan 自动接入留在范围之外，原 R2 artifacts 保留。

[Spec](../spec/runtime-progress.spec.md)、[Plan](../spec/runtime-progress.plan.md)、[调用方配置与完整返回示例](../runtime-progress.md)。可选 `progress_file.initial_content_base64` 提供最多 65536 原始字节；工具读取绝对路径环境变量 `STEERLINE_PROGRESS_FILE`，调用方使用 `progress-file --run-id R --json` 读取独立持久化 version、时间、原始 base64、长度/摘要及采集状态。内容解释归调用方。

## 当前构建与实现结论

- 源提交 `1cfe2f9a065830ab62d540d3de4c0011ed603b56`，dirty=true，未提交；[冻结实现及测试文件](evidence/runtime-progress-20261023/r12/build-source-files.json)共 68 项，[验收后复核](evidence/runtime-progress-20261023/r12/source-immutability.json)全部未变。
- Runner SHA256 `3f7db530528954bb888729e32349013256e72a2f26f38eceab56a8404ac54f4e`；资源摘要 `8171b167b01ce7e498a49460ff2e2390512411d282d7bd08c6f04bf325cc0340`。[构建原值](evidence/runtime-progress-20261023/r12/runner-build.json)
- Native driver SHA256 `7a3cd2ee92aec04eb7f40e1802b2abf73730c9d5e04cc54b1e4822af8751fafc`；本机独立 agent helper SHA256 `02ab332e9fcbe9927101ccfcbc77bbd6b6354e3fc518bea7af6196d498abcc0f`，[Go 构建信息](evidence/runtime-progress-20261023/r12/agent-build-proof.json)核对为 `cmd/steerrunner-agent`，不是公开 CLI。
- 本机 Codex 0.159.2，gpt-6.1-sol/low/sandbox；Orca 1.4.224，runtime `28da9fea-ca77-48de-8319-decb62914563`。配对远端 台式机64G，environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`，Orca 1.4.223，runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`。使用新任务现场，没有修改 SteerLine 安装或用户保留 Run，也没有远端常驻 Runner/helper。
- 原工作区创建前持久化初始终端准备计划，原回执延迟后继续首次准备，创建未知只读对账原 owner，不能重建终端或重发业务输入。正常原子替换由固定路径重新打开读取，采集独立于 native final。
- 本机停止恢复补齐关闭后的展示卡片对账：`connected=false` 只触发完整原 runtime/worktree 清单查询，证明同一原身份缺席才确认，零重复关闭；身份变化、缺原主机、截断或仍列出均拒绝。首次阶段期限持久化不续期，所有等待和采集有界。

## 当前实际结果与资源证明

[正式 summary](evidence/runtime-progress-20261023/r12/summary.json)为 passed、7/7。L1 611 顶层通过，框架 21、终端扩展 29 通过；16 项既有辅助/显式 opt-in 的默认 skip 单列，不计 native 通过。完整 Controlled 23 顶层零 skip，单文件专项 43.07 秒；[原始日志](evidence/runtime-progress-20261023/r12/controlled/integration.log)、[公开接口专项响应](evidence/runtime-progress-20261023/r12/controlled/progress-public-evidence.json)。两项真实 L2 分别创建并合并 PR #663（smoke）/#664（result-acquisition）。

| 场景 ID | 结果 | 现场准备至收尾总时长 | 独立现场 | 证据 |
| --- | --- | --- | --- | --- |
| `local-codex.result-acquisition` | passed | 321.41 s | `C:\srt\56bf86ce0c8e` | [原记录](evidence/runtime-progress-20261023/r12/local-codex.result-acquisition/scenario.json) |
| `local-codex.smoke` | passed | 284.87 s | `C:\srt\54dcb135787d` | [原记录](evidence/runtime-progress-20261023/r12/local-codex.smoke/scenario.json) |
| `local-codex.progress-file` | passed | 218.86 s | `C:\srt\1b241e91d91a` | [原记录](evidence/runtime-progress-20261023/r12/local-codex.progress-file/scenario.json) |
| `orca-local.progress-file` | passed | 474.85 s | `C:\srt\452815a00fe2` | [原记录](evidence/runtime-progress-20261023/r12/orca-local.progress-file/scenario.json) |
| `orca-remote.progress-file` | passed | 346.20 s | `C:\srt\628ea40f2f6b` | [原记录](evidence/runtime-progress-20261023/r12/orca-remote.progress-file/scenario.json) |
| `local-codex.progress-multirun` | passed | 226.79 s | `C:\srt\f2be0e63b98f` | [原记录](evidence/runtime-progress-20261023/r12/local-codex.progress-multirun/scenario.json) |
| `local-codex.progress-recovery` | passed | 226.72 s | `C:\srt\18aa665faac6` | [原记录](evidence/runtime-progress-20261023/r12/local-codex.progress-recovery/scenario.json) |

本地和两个 Orca 单文件场景均验证实际执行环境注入、任意非 JSON/非 UTF-8 字节、完整快照 2/3/4、重复内容不增版本、原子替换及终态保留。远端写入/读取位于原配对主机，本机没有对应源文件，native final 未核实仍回传。多 Run 场景在同一服务中持有两个独立原生资源，交错更新不串线，停止一项后另一项沿原 session/turn 继续，文件版本不影响业务 revision。恢复场景核对文件读失败保留/恢复、原生工具等待、服务真退出并换实例、原文件/快照和原 session/turn/attempt/operation 保留，零额外业务输入。

[离线结算汇总](evidence/runtime-progress-20261023/r12/cleanup-summary.json)证明本轮五项 L3 的六个原 Run 全部 stopped，五个服务全部停止，各原 Run `resources_settled=true`。每个现场还保存原 CLI 响应、原记录、关闭后查询的最后快照及 native 会话/CLI 审计；Orca 初始终端准备日志位于各 case 的 native-preparation。保留 owned 工作区、分支及记录供复核，不清理用户现场。

## 当前构建的实际并行

[严格并行验证](evidence/runtime-progress-20261023/r12/progress-parallel-verification.json) passed：五项各自 120 秒以上真实观察窗口，Runner/driver/source 一致、资源冲突为零，每项均至少与另一项通过场景的原生观察窗口实际重叠，合计七对窗口重叠。Orca 本机/远端重叠 **63.934 秒**，多 Run/恢复 **120.123 秒**，本地单文件/远端 **29.758 秒**。没有依赖跨场景 gate 或借用旧批次窗口；隔离与资格说明见 [测试目录](../test-catalog.md)。

## 准确执行命令与证据保存

```powershell
# 本机 helper 独立构建，配置中核对其绝对路径和 SHA256。
go build -trimpath -o .evidence/progress-agent-r12.exe ./cmd/steerrunner-agent
./scripts/test.ps1 -Level L1,Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config .evidence/progress-test-config-r12.json -MaxParallel 7 -OutDir .evidence/progress-acceptance-r12
./scripts/verify-progress-parallel.ps1 -Report .evidence/progress-acceptance-r12
```

原始批次仍在 `.evidence/progress-acceptance-r12/` 和上表 `C:/srt/`。可随仓库审阅的归档为 `docs/acceptance/evidence/runtime-progress-20261023/r12/`，含冻结配置/目录、构建、原始执行记录、公开查询、观察窗口、恢复、清理和文件摘要。重跑使用新 OutDir 和新现场。精确选择五项 L3 均为 default case、progress-file driver；其登记在 docs/test-catalog.md、tests/catalog.json、internal/testkit/livetest/cases.json，原有场景均保留。

历史失败未重写：[摘要复核](evidence/runtime-progress-20261023/r12/earlier-failures-preserved.json)同时核对 780、3574、4333 文件基线，全部一致；最后一组覆盖 r1–r11 原始报告和归档。各轮原失败、中止及单独清理记录见下文，未将手工清理或受控诊断改记为 native 通过。

## 验证边界

- 本轮未进行配对网络封锁/管理员故障注入；真实验证包括文件不可读恢复及服务观察断开/重启，受控传输故障另外登记。progress 专项 Pi/DSH、full_access 和 audited 矩阵没有真机资格。
- Orca 远端依赖既有配对、原 runtime/worktree 身份及用户原生登录；文件 RPC 已足够，无新增远端接口缺口。停止证明沿既有 `orca_terminal` 范围，不宣称所有脱离终端的 OS 子进程退出或执行主机凭据撤销。
- 按用户收敛边界固定原主机、工作区和文件，不承诺抵御采集瞬间的恶意链接/目录替换。原 R2 接口及其回归保留；不实现 Plan 解析、测试业务语义、历史登记或 SteerLine 页面。

## 历史 r1–r11 原始记录

历史 r2：实现及测试注册已完成，**该批次验收未全部通过**。修复构建正式批次 `progress-acceptance-r2` 为 failed：L1、Controlled 和两项 L2 通过，五项新增 L3 中四项通过，本机 Orca 被原生工作区初始终端回执缺口阻塞。未将文件探针、受控通过或旧构建并行结果算作本机构建 L3 通过。

只修改 Runner。SteerLine 接线、项目生产测试入口、Plan 自动接入及测试业务语义不在范围内；R2 artifacts 保留。用户收敛后的边界是固定原执行主机/worktree/文件的只读采集，不以防恶意链接瞬时替换作为前置。

契约：[Spec](../spec/runtime-progress.spec.md)、[Plan](../spec/runtime-progress.plan.md)、[调用方接入和完整示例](../runtime-progress.md)。实现为可选 `progress_file.initial_content_base64`，64 KiB 原始字节，`STEERLINE_PROGRESS_FILE`，`progress-file --run-id R --json`，独立持久化 version，不改变业务 revision。

## 历史 r2：固定构建与环境

- 源提交 `1cfe2f9a065830ab62d540d3de4c0011ed603b56`，dirty=true；本次改动未提交。生产源码在正式 r2 前冻结，之后仅加强 native 测试断言及整理文档/证据。
- Runner SHA256：`1629794c9b1122bf8a3a3a5900a95ef466e4c47ed2ecdea86c0a0c3dab732eef`；受控与真实场景均核对同一摘要。资源摘要 `f2d3b154771bc1928a5e2e9b535fffc74e1e4d66e3bd2024f99a3a497d088d00`。
- 最终 native driver SHA256：`cab94c4d6459592a1e06bbf236e6b06e35a9f415c26a291119bb15ab301643ff`。每个正式新现场由 test-native 自动编译、登记；包含逐 Run 离线结算、下一 gate 证明重复写入已发生、确认第一项停止后才写 peer、确认服务离线且实例更换等断言。
- Native Codex 0.159.2，gpt-6.1-sol/low，sandbox；本机 helper 为本任务构建的 `.evidence/progress-agent-final.exe`，SHA256 `49e875d23c7be1af64cc126d5448f6813d96f90a00ec22b3e7acdc4acce79a6d`。未修改 SteerLine 安装文件。
- 本机 Orca 1.4.224、runtime `28da9fea-ca77-48de-8319-decb62914563`。已配对远端 台式机64G，environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`，Orca 1.4.223、runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`；原生已有登录，无新增远端常驻 Runner/helper。

正式总记录及源码清单见 [r2 summary](evidence/runtime-progress-20261023/r2/summary.json)、[构建](evidence/runtime-progress-20261023/r2/runner-build.json)、[源码文件摘要](evidence/runtime-progress-20261023/source-files.json)。原始现场仍保留于 `.evidence/progress-acceptance-r2/` 和下表各独立 `C:/srt/`；可随代码审阅的原始响应、native 日志/会话、原记录及清理摘录位于 [专项证据目录](evidence/runtime-progress-20261023/)。摘录文件摘要见 r2/files.json。

## 历史 r2：分项实际结果

| 范围 | 实际结果 | 原始证据位置 |
| --- | --- | --- |
| L1/框架 | 606 顶层通过，终端扩展 29、框架 21 通过，driver 编译及零选择守卫通过；原有明确 opt-in 的默认 skips 单列，不计真机通过 | `.evidence/progress-acceptance-r2/l1.jsonl`、terminal-extensions.tap、framework/result.json |
| 完整 Controlled | 23/23 通过，无 skip；本次 TestSRProgressFile 50.68 秒，原 R2 场景也通过 | [受控原始日志](evidence/runtime-progress-20261023/r2/controlled/integration.log)、[公开专项响应](evidence/runtime-progress-20261023/r2/controlled/progress-public-evidence.json) |
| L2 smoke | passed，真实实施、发布及合并，PR #653 | `C:/srt/816bcb93565e/evidence`；r2/local-codex.smoke |
| L2 result-acquisition | passed，默认/边界 parser 及完整工作流，PR #654 | `C:/srt/818ed35457da/evidence`；r2/local-codex.result-acquisition |
| 本地真实 A | local-codex.progress-file passed；真实工具环境绝对路径、violet/copper/silver 二进制快照、替换、重复写去重、终态查询和离线结算 | `C:/srt/68faf4c4bd4d/evidence`；r2/local-codex.progress-file |
| Orca 本地 | orca-local.progress-file failed/blocked；业务启动前工作区初始终端回执缺失，未初始化文件、未取得模型写入。不能记本机 Orca L3 通过 | `C:/srt/546347e04018/evidence`；r2/orca-local.progress-file |
| 远端真实 B | orca-remote.progress-file passed，122.40 秒；真实远端工具写入/文件 RPC 读取、本机无源文件、替换后继续、native final 未核实期间回传、终态查询、orca_terminal 范围离线结算 | `C:/srt/0eac72c69462/evidence`；r2/orca-remote.progress-file |
| 同服务多 Run C | local-codex.progress-multirun passed；两个独立原生资源，交错内容/版本/路径/身份不串线；确认第一项 stopped 后第二项原 session/turn 继续写，业务 revision 不变 | `C:/srt/f83de8613330/evidence`；r2/local-codex.progress-multirun |
| 核心恢复 D | local-codex.progress-recovery passed；native 工具等待期间不可读保留、恢复；服务真实退出并换实例、离线原工具继续写，文件/版本和原会话/turn/attempt/operation 数保留，零额外业务输入；终态仍可查 | `C:/srt/0852bf5b2739/evidence`；r2/local-codex.progress-recovery |
| 实际并行 | r2 MaxParallel=2 确实并行调度，资源无跨 case 冲突；但成功 L3 的稳定写入窗口未重叠，新构建完整并行资格未取得。首轮旧构建 C/D 实际写者重叠 1.7607439 秒且两项通过，单列保留 | [r2 窗口及资源核对](evidence/runtime-progress-20261023/r2/progress-parallel-verification.json)、[r1 并行原值](evidence/runtime-progress-20261023/r1/parallel-verification.json) |

Controlled 另证明：文件读取失败时 stop 仍可受理/完成、peer 继续更新；原 native turn 输入数重启前后均为 2，无 resume 或输入重发；原始非 JSON/非 UTF-8 字节、64KiB/超限、缺失、错误绑定、旧业务 clone 不覆盖新快照等快速规则由 L1/Controlled 覆盖。

每项使用新 root/state/service/base/workspace/Run/native 资源；多 Run 场景内部共享服务而每 Run 原生资源独立。前置、独有断言及并行资格设计见 [目录专项](../test-catalog.md)。所有读取、gate、轮询和停止有界；保留 owned 工作区/分支及原记录供审计，不清理用户保留现场。

## 历史 r2：注册及准确执行命令

五项均为 L3、case=default、driver=progress-file、预期 stopped，不创建 PR：`local-codex.progress-file`、`orca-local.progress-file`、`orca-remote.progress-file`、`local-codex.progress-multirun`、`local-codex.progress-recovery`。已同步 test-catalog、tests/catalog.json、唯一 cases.json、driver、配置/目录校验和结果校验。当前注册 33 场景/43 case，L3 27 场景/37 case；Controlled 23 顶层。

本次实际正式命令（没有跳过自动前置）：

```powershell
./scripts/test.ps1 -Level L1,Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config .evidence/progress-test-config-final.json -MaxParallel 2 -OutDir .evidence/progress-acceptance-r2
```

配置复用 local-codex / orca-local-codex / orca-remote-codex，非秘密冻结副本在 r2/frozen-config.json；重跑必须使用新的 OutDir。正式入口自动运行框架、完整 L1 和全部 Controlled，再分配独立真机现场。当前 r2 结论 failed（6/7），不能把已执行、静态 available、默认跳过或探针成功等同真机通过。

## 历史 r1/r2：失败、修正及清理

- 首次公开 Controlled `.evidence/progress-controlled-first` 因 CLI parse 白名单漏新命令而失败；修复后 second/final 和 r2 正式专项均通过，首次失败未覆写。
- `.evidence/progress-foundation-all` 失败于框架旧 unavailable 数；r2 框架已修正。`progress-foundation-all-r2`、`progress-live-all` 失败于既有八个 usage native opt-in 的默认 skip 名单不完整；补齐已发布 opt-in 名单并保留环境拒绝守卫，没有把 skip 算成 native pass。
- `.evidence/progress-acceptance-final` 首轮正式批次 failed（5/7）：L1 604、Controlled 23、L2 两项、本地进度/C/D 通过；Orca 本地/远端独立观察器误用了要求业务 worker 句柄的 setup，空指针崩溃。已改为仅构造冻结路由 progressSetup，回归通过，修复构建 r2 远端真机通过。堆栈、原报告和回归见 r1/。r1 的本地与远端失败服务均已退出；远端原 Run 离线结算 true。本地原终端按本次独有 worktree 精确清空，但 native 绑定不完整，严格结算仍 false，原值保留。
- r2 本机 Orca 创建返回仅有 worktree，没有初始 terminal handle/incarnation；外层工作区创建进入对账后，既有 `initialWorkspaceTerminalRecovery` 拒绝凭 inventory 认领新身份，诊断为 `original initial terminal creation receipt missing; inventory alone cannot prove its identity`。这是业务执行准备/创建恢复接线缺口，**不是进度文件读取 RPC 缺口**。本增量没有猜测绑定、重发输入或重建原业务。需要原 worktree 创建响应提供初始终端身份，或原创建请求可对账的初始 terminal-create 回执，才能沿原资源继续。
- r2 失败现场 Run `c40956c2c0960b359abdb8c4c03d9938`、workspace instance `ff26f630-319a-4dbf-9ff2-572a7b9902dc`：清理前核对原 instance，再只关闭其独有 worktree 的全部终端。清单 totalCount=0、truncated=false，服务 not_running。严格 `resources_settled=false` 因原绑定缺失仍未核实，未修改成通过；原 `manual-cleanup-*` 及停止响应保留。其它已通过 L3 均逐 Run 核对 offline resources_settled=true。
- 独立文件探针 local、remote-r2 通过且原终端停止/worktree 删除；remote 首次替换因空 backup 参数失败，改显式 backup 后通过。这些是原绑定文件接口探针，无模型、不算 L3，见 probes/。

## 历史 r2：当时边界与尚未验证

- 本机 Orca 完整公开 EXE/native 链路及修复构建成功 L3 之间的实际并行写者资格仍未完成，不能宣称本次全部验收通过。
- 远端文件通道已由真机证明，无需新增文件传输接口或远端常驻 helper；依赖既有配对、原 runtime、worktree 身份和原生登录。停止证明沿既有 orca_terminal 范围，不升级为所有脱离终端 OS 子进程退出或凭据撤销。
- 已验证文件不可读恢复、服务观察断开/重启与零重发；没有把这些等同真实配对网络封锁/管理员故障注入。该网络边界及 progress 专项 Pi/DSH、full_access 矩阵本次未验证。
- 固定原主机/工作区/文件、不接受任意路径、不扫描、不本机读取远端源文件；恶意链接/目录瞬时替换不在本次保证内。首次初始化持久意图后的不确定窗口，只读对账原 seed；缺失或不完整不清空/重建，也不启动额外业务输入。

## 续作中间批次 r3（失败原记录）

Runner `6ba0f59d7eeb01cca5edd7d73365216b9d7b46608e54a88fae7c9d00dfdcbcd9` 修复初始终端准备计划后，正式批次 [r3 summary](evidence/runtime-progress-20261023/r3/summary.json) 通过 L1 607、终端扩展 29、框架 21；完整 Controlled 23 个顶层中 22 通过，ResourceCreationRecovery 的两个子项失败，L2/L3 因前置失败未执行，不能计作真机通过。[原始日志](evidence/runtime-progress-20261023/r3/controlled/integration.log) 与两个失败现场的 service/launch 日志保留。

`missing-terminal-receipt` 旧断言期望恢复阻断，实际原终端已正确恢复并停止；新版契约将它拆成 `planned-initial-terminal` 正例与 `missing-initial-plan` 反例，仍保留 wire-loss/owner-crash 拒绝。`receipt-before-commit` 的服务启动失败保留：原停止请求已受理（原 DB 只读逐行导出在同目录），不能直接断言为 revision 冲突；故障门由 2 秒改为 15 秒有界查询，强杀后等待核实的原服务退出，再启动替代服务。定向正式 Controlled 七窗口全部通过（`.evidence/progress-initial-terminal-controlled-r4`），不是完整前置或真机资格。

原始失败现场 `C:/Users/lmas1/AppData/Local/Temp/sr-1520278930`、`sr-2234717503` 仍保留。只读原进程检查 v1 缺少 service 可执行路径，结论不完整也保留；补齐原 EXE 的 [v2 检查](evidence/runtime-progress-20261023/r3/progress-r3-failure-process-proof-v2.json) 确认原 owner/CLI/service 七项检查均为 process absent，不改变原测试失败或未知资源判定。此批无真实 Orca 资源。

## 续作真机 r4 与中止 r5（原记录）

[r4 正式报告](evidence/runtime-progress-20261023/r4/summary.json) 为 failed，3/7：L1 607、Controlled 23 全部通过；本地单文件、多 Run、恢复三项真实 L3 通过（独立现场 `08dc265f1a12`、`a9eb39805fd8`、`4b7a2fecd027`）。Orca 本机现场 `8ffc6693c3cc` 已恢复原初始终端并初始化文件，但原业务发送到三分钟首份内容期限末才受理，未取得 violet；远端现场 `e0efde8aa705` 实际完成原子替换、完整字节、120 秒独立观察及版本断言，最终停止/离线结算未通过，仍是 failed。两项 Orca 的 45 秒测试停止等待会在原原生结算完成前停服；进一步核实应用对每次停止只给 15 秒，并发 Orca 原身份多次 CLI 核验可在首次关闭前耗尽。原始回执及 [并行不合格报告](evidence/runtime-progress-20261023/r4/progress-parallel-verification.json) 保留，不能将功能断言通过或 C/D 重叠算成五项资格。

两项 L2 均 blocked 后失败：现场 `4668599d2a79`、`02b2e5d359fc` 的提交/推送成功，创建 PR 的复合 shell 命令被自动策略返回 `rejected: blocked by policy`，无更详细拒绝原因、无 PR URL。原生会话日志与失败报告完整保留；未重发原失败现场。后续采用更安全的发布提示，分别检查 push/查询/创建，先保存请求文件再单独调用 provider CLI，保留请求供核验，不组合创建和删除文件；原 sandbox/审批规则不变，不将此改动视为已通过。

r4 后续清理与原测试结果分开：按原 runtime/worktree instance/terminal incarnation 核验，只关闭本机原控制终端及远端原业务终端，确认 `ptyKilled=true` 和完整清单原 handle 消失。远端原停止对账随后完成，服务退出，`manual-cleanup-offline-final.json` 为 resources_settled=true、scope=orca_terminal；[本机清理后核验](evidence/runtime-progress-20261023/r4/orca-local.progress-file/manual-cleanup-offline-final.json) 仍为 false，原 helper 已退出且原 PTY 已关闭，但原认证撤销回执缺失，保持 unverified，不补造证明、不改原 failed。各场景目录的 `manual-cleanup-*` 保存全部原响应。旧现场均非用户业务 Run。

[r5 interruption](evidence/runtime-progress-20261023/r5/interruption.json)：此构建尚不含已确认需要的 Orca 停止队列预算修复；等待其 L1 进程自身退出清理后，核验原 controller PID/创建时间及未启动 Controlled/live 再中止。原 summary 保留 running/queued 原样，以独立 interruption 说明中止；启动真机场景为 0，不计通过。后续 Orca 单次停止尝试为 60 秒、其它 adapter 仍为 15 秒，均继承服务取消；新增 L1 验证期限、及时取消、授权不变及不伪造停止。测试首次内容/停止/停服预算为 5 分钟/120 秒/60 秒，产品文件采集仍为 2 秒。r6 在新构建重新执行完整正式入口。

## r6 真机结果与夹具修正（失败原记录）

[r6 summary](evidence/runtime-progress-20261023/r6/summary.json) 为 failed，4/7：Runner `cf10a98cde5ac852674e6a4084fe6c2c6a9e531dae2943ad12afcc6804c6abc5`，L1 608、Controlled 23 全部通过。本地单文件、Orca 本地、Orca 远端和恢复四项真实 L3 完整通过，包含原 Run/服务停止与离线资源结算；首次取得本机 Orca 从提交、原终端恢复、真实环境写入到结算的完整正例。现场分别为 `C:/srt/314f0e88ee87`、`c9f21f9d064f`、`695d369caf4e`、`b9c1c1baf3af`，本机 Orca 实际 430.10 秒。五项/当前并行资格仍未通过，不借用其它构建。

多 Run 现场 `f908d9944416` 的第二个 Run `66898f98e7a1c6fb979d747636da6c1c` 收到正确的 producer 提示，但模型调用了执行环境之外的 node_repl/child_process，原生记录明确返回 `spawn EPERM`，脚本未启动；随后读取了夹具通用示例 Spec/Plan，误做实现/发布，最终首份内容超时。两 Run 的原生会话及公开响应均保留，不算隔离通过。后续为 progress-file driver 配套 producer-only Spec/Plan，并明确直接使用当前执行环境内的原生 shell 工具，禁用 node_repl/child_process 这条夹具调用路径；不放宽进程或文件权限、不向原失败 Run 重发输入。

两项 L2 现场 `f05e1324d8ea`、`a3a09a515f62` 均因模型选择 REST PR 列表查询被 broker 拒绝而失败。原 native 命令为 `gh api --method GET repos/erranliu/SteerlineTest/pulls ...`，响应 `HTTP 403: execution repository or operation denied`，不是凭据过期（原撤销记录有效期为 18:43）。已有 broker 契约只允许受限 GraphQL 的 PR 查询/创建，以及单 PR REST 查询；未扩大权限。发布提示进一步明确使用已支持的 `gh pr list` / `gh pr create --body-file` 路径，正文用独立 Markdown 文件准备并保留，拒绝/未知不重发规则保留。原两项失败不重试或改记。r7 重新构建并完整执行正式入口。

## r7 准备期限失败（原记录）

[r7 正式报告](evidence/runtime-progress-20261023/r7/summary.json) 为 failed，6/7，Runner `393fad8b175469a511129a2f10ba197f7db08a6563a29c14303cbd2f5c8eab1e`：L1 608、完整 Controlled 23 通过，两项 L2 实际创建并合并 PR #657/#658；本地单文件、远端、多 Run 和恢复四项 L3 完整通过。多 Run 已使用原生 shell 运行两个 producer，不再误做示例任务。原记录及[并行不合格报告](evidence/runtime-progress-20261023/r7/progress-parallel-verification.json) 保留，不把四项通过记成五项资格。

本机现场 `C:/srt/8a66a995a3ae`、Run `9f8f6bb78b864b7d5c4db5bd5974b4b9` 的 helper 于 10:45:07 UTC 准备成功，但核心阶段准备期限固定三分钟，后续候选操作被 `preparation_deadline: The original preparation budget is exhausted` 阻断，业务回合没有开始。不能靠扩大文件等待或重发原输入修复这个已过期现场。r8 将新 Orca 阶段的持久期限设为六分钟（含工作区/控制/执行准备与共享 CLI 排队），其它 adapter 三分钟；已有期限在重试、过期或服务重载后均不续期。新增存储重载/过期不续期回归，原立即失败检查保持。

测试等待与产品期限分开：Orca 首份内容八分钟，Codex 五分钟，原 Run 停止 120 秒、服务退出 60 秒，场景沿唯一注册的既有十五分钟总期限；独立持续观察仍为 120 秒、文件每轮采集仍为两秒。本轮此前文档写的十分钟 case 总限已纠正为实际注册的十五分钟，没有缩减测试或更改原失败。r8 再执行新构建的全部正式前置、两项 L2、五项 L3 和真实并行核验。

## r8 前置失败与 Windows 发布争用（原记录）

[r8 正式报告](evidence/runtime-progress-20261023/r8/summary.json) failed：Runner `69aec2a926c4d31447cd3b0db4d18d519427508b5dc1948d82391f5f89d408f2`，L1 609、扩展 29、框架 21 通过；完整 Controlled 23 顶层中 22 通过，单文件专项 42.85 秒通过。ResourceCreationRecovery/kill 的替代服务启动报告 service_start_failed；原错误没有底层诊断，不能断言是 revision 冲突、终端回执或文件采集故障。前置失败，零 L2/L3 执行；[Controlled 日志](evidence/runtime-progress-20261023/r8/controlled/integration.log)、失败现场原 DB/创建回执/日志保留。独立原 owner/CLI/服务身份检查均已不存在，该现场无剩余任务进程；受控虚拟 Orca 未创建真实资源。

先给 service.log 增加底层错误日志；[单次](evidence/runtime-progress-20261023/r8/startup-diagnostics/single/result.json)及[二十次](evidence/runtime-progress-20261023/r8/startup-diagnostics/repeat-20/result.json)相同 kill 窗口均通过，未复现历史失败，不能用它们替代真机资格。另以真实 Windows 文件句柄确定性复现发现文件争用：旧发现文件被只读查询打开时，MoveFileEx 原子替换立即 Access denied，[修复前失败](evidence/runtime-progress-20261023/r8/startup-diagnostics/progress-endpoint-collision-before.log)。同一源/目标只对 sharing/access 错误有限重试约半秒，原文件不预删；短暂占用恢复、持续占用有界失败且旧完整内容保留，[修复后回归通过](evidence/runtime-progress-20261023/r8/startup-diagnostics/progress-endpoint-collision-after.log)。这是已确认的独立启动发布缺陷；尚不能证明它就是 r8 的未诊断原因。r9 新构建重新走全部正式入口及五项真实 L3。

## r9 helper 配置错误（原记录）

[r9 正式报告](evidence/runtime-progress-20261023/r9/summary.json) failed，6/7：Runner `cb41e5ca5d496be98c220dd8ce4df068c78724e136cc816162326a6717964a42`，L1 610、扩展 29、框架 21、完整 Controlled 23 通过。资源创建恢复七窗口和单文件专项均通过；两项真实 L2 创建并合并 PR #659/#660；本地单文件、远端、多 Run、恢复四项 L3 全部通过。并行验证器仍判不合格，不把四项计成五项。

本机现场 `C:/srt/49da718e8396`、Run `843211dc045a57f1a03b73bff896d1d5` 的错误由主会话构建配置造成：将 cmd/steerrunner 公开 CLI 产物填为 agent_launch.runtime_executable，原控制终端立即返回未知选项，未运行模型 producer。[helper 构建信息](evidence/runtime-progress-20261023/r9/helper-build-info.txt)、原始 native CLI 审计、超时、停止未知与离线 resources_settled=false 全部保留。原错误现场不重发输入、不换绑定。

正式收尾后，以原 runtime/worktree instance/terminal handle/incarnation 逐一核验并关闭该 Run 两个准确终端，均得到 ptyKilled=true，最终本机完整清单 totalCount=0；[清理原值](evidence/runtime-progress-20261023/r9/manual-cleanup.log)及各 CLI JSON 保存于 r9/orca-local.progress-file。没有凭此伪造已缺失的 native exit/auth 证明，原失败与严格未结算保持。r10 以同一 Runner 源码构建正确 cmd/steerrunner-agent（摘要 e802f05acef6ed17fbfb8080f5b5f200dade48aa09b481fc458bc382ed70141c），通过 go build info 核对包路径后，用新配置/现场再运行完整正式前置、两项 L2 和五项 L3。

## r10 夹具 push 与本机停止对账失败（原记录）

[r10 正式报告](evidence/runtime-progress-20261023/r10/summary.json) failed，5/7。Runner 仍为 cb41e5ca…7964a42，正确 helper e802f05a…70141c；L1 610、完整 Controlled 23、框架 21、扩展 29 通过。两项 L2 创建并合并 PR #661/#662；本地单文件、多 Run、恢复三项 L3 通过。远端场景在创建 Run 之前，GitHub 拒绝新隔离基线 ref 的 push；原 SHA/ref、失败记录、manifest/preparation 和成功但为空的准确 ref 查询保留。未使用之前远端通过替代本次零执行。

本机 `C:/srt/13486f5f4b91`、Run `001ee3a278f57d400196f5b500a20bfd` 已取得 2/3/4 原始快照、去重和稳定窗口，progress_verified=true；但两分钟内业务 stop 未核实，场景仍 failed。随后原离线 resources_settled=true，原失败不改写。审计明确显示原业务及控制 close 已应答，而 terminal show 仍返回同 incarnation、connected=false 的展示卡片；完整原工作区 inventory 已为空。旧 readonly close 在卡片断连时直接拒绝，导致下一次 business-confirmed 对账不能完成。

[回归修复前失败](evidence/runtime-progress-20261023/r10/progress-stale-control-before.log)、[修复后通过](evidence/runtime-progress-20261023/r10/progress-stale-control-after.log)：断连卡片只触发原 runtime/worktree 完整清单查询，确认同原身份缺席才返回 confirmed，零重复 close；仍列出、截断、缺原主机、异 runtime/incarnation/worktree 均拒绝。Orca 测试停止等待三分钟容纳原有限停止与下一次只读对账，Codex 两分钟，case 十五分钟和 120 秒观察窗口、停止/资源结算断言保持。

本次夹具基线 push 最多三次；每次错误均保存，成功准确 ref 查询发现同 SHA 则只读确认，确认不存在才继续同 SHA 非强推，异 SHA 或查询失败立即退出。真实本地 bare Git 的拒绝恢复、已接受丢回执、异 ref、查询失败四窗口仅计受控诊断，不能替代真实远端。

## r11 中止调度（原记录）

r11 的旧构建 L1 完成后，r10 原始记录确认上述停止缺陷。仅停止核实命令行和创建时间的 controller PID 21836，保留已独立启动的 Controlled child PID 27108 自行执行及清理；所有真机场景仍 queued，零真机派发。[原 summary 与 interruption](evidence/runtime-progress-20261023/r11/qualification.json)不改记，独立 Controlled 最终 23/23 通过，原宿主已不存在，零真机派发；这些旧构建前置不计新构建资格。新源码以 r12 冻结重验。
