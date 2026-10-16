# L3 执行稳定性专属验收

> 本记录对应当时冻结的远端六场景/十二 case。当前新增本机 L3 和 L4 测试的实现及结果
> 分别见[测试清单](../test-catalog.md)和[测试功能验收](test-levels.md)，不混入本记录的历史 36/36。

日期：2026-10-16。依据 [Spec](../spec/l3-execution-stability.spec.md) 和 [Plan](../spec/l3-execution-stability.plan.md)。状态：已实施，同一最终构建完整三轮真机资格验收通过，36/36，首次通过率 100%，无管理员测试排除。此前失败独立保留，不计入本批通过率。

后续主线 rebase 复测另见[2026-10-16 复测记录](l3-rebase-20261016.md)：`e04d07f` 基线加等待门槛修复 `95d24e5` 的完整轮为 10/12，独立补测为 1/2，不可达反馈版本冲突可复现。本页 36/36 为原归档构建的历史资格。

连续版本冲突的后续修复和独立真机结果见[不可达撤销修复复测](l3-revocation-fix-20261016.md)；rebase 后主线的新远端认证能力门槛阻断另行记录。

## 范围与环境

固定六个 L3 场景、十二个 case；完整资格为同一最终构建连续三轮，共三十六次。新增五个场景包含十一个故障 case，原 smoke 保留反馈及真实合并契约。目录为 `tests/catalog.json`，唯一 case 定义为 `internal/testkit/livetest/cases.json`。

验收运行时来源基线为 `57d6f6f2455b6a748b56aafc75668915bd1d87eb`，分支 `codex/l3-execution-stability`；当时实施为工作树修改，不能将基线 commit 冒充新代码的干净提交。提交前逐一核对 288 个冻结来源文件，均与验收快照相同。各现场保留实际 EXE/driver/helper/native runtime 摘要。

可在 GitHub 直接查看的完整结果存于 [验收证据目录](evidence/l3-execution-stability-20261016/README.md)：包含三轮汇总、36 个现场的测试结果/断言/构建身份、独立最终审计、阶段耗时、来源摘要、L2 回归和设施退出核对。下文的本机路径用于追溯原始现场，远程审阅请使用该目录中的提交副本。

提交范围：上述完整三轮对应归档提交 `9425d2a433d533ae6f3ceee24ec687e60bac6d83`。随后整合最新 main `f156263`，将停止进度持久化接入主线新的 `operation.Intent.SettleStop`；整合版本通过 L1 178 项、脚本检查 12 项、Controlled 12 场景、live driver 编译/零匹配 guard 和 go vet。该版本未重跑完整 L3 三轮，不能将此前 36/36 表述为整合版本的新资格；详见 [主线整合回归](evidence/l3-execution-stability-20261016/main-integration.md)。

真实目标为已配对 environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`，已核实 runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`，注册仓库 `C:/Projects/SteerlineTest`。用户已恢复全部管理员测试范围，不在最终批次使用 SkipAdminRequired。

网络设施通过每现场专属 Runner、helper、Node 程序路径建立防火墙规则，并用 [SetTcpEntry](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-settcpentry) 关闭同范围已有 IPv4 连接。真实 Orca CLI 的公开 JavaScript/依赖复制到现场，由专属 Node 执行；不复制认证，不针对共享 Orca/Electron 程序路径加规则。IPv6 连接不支持时明确失败。业务模型前验证真实订阅断开、双通路失败和恢复，另有独立有界恢复 watchdog。

## 验收登记

| 项目 | 当前状态 | 证据 |
| --- | --- | --- |
| L3-A01 注册/展开 | 通过：六场景、十二 case，每 case 连续三轮，共 36 次，无排除 | registry、framework、最终 summary/final-audit |
| L3-A02 故障设施 | 通过：真实订阅断开、双通路阻断与恢复；199 个稳定性控制辅助终端完整配对，21 个最后注入记录中的 watchdog 已退出，批次期间创建的 PowerShell 无残留 | 最终 faults/native journal、final-audit、watchdog-cleanup、post-run-shells |
| L3-A03 调用方退出 | 通过：三轮原请求找回、原调用方退出、执行继续及零重复输入 | 最终 accepted-response-lost 三个现场 |
| L3-A04 原实例重连 | 通过：三轮完整 60 秒故障窗、失败后重试、原服务/Attempt 保持及自动恢复 | 最终 active-disconnect 三个现场及物理观察证明 |
| L3-A05 服务换代 | 通过：四种 case 各三轮，原写者退出、恢复门槛、授权前零输入及显式继续原会话 | 最终 service-restart 十二个现场 |
| L3-A06 停止撤销 | 通过：三种 case 各三轮；停止进度生产修复、跨重启持久撤销、两类写请求拒绝、迟到事实不推进 | 最终 stop-revocation 九个现场及受控回归 |
| L3-A07 发布未知 | 通过：两 case 各三轮，原 final/PR/head 对账，实际查询失败不假成功，零重复发布 | 最终 publication-unknown 六个现场 |
| L3-A08 结果与证据 | 通过：反例、原始证据、固定 outcome 与独立最终核对，清理和业务结果分别登记 | L1/Controlled、每现场 helper、最终 final-audit |
| L3-A09 回归与资格 | 通过：L1 163 项、脚本检查 12 项、Controlled 12 场景，L2 同一 Runner 构建真实交付回归，以及完整 L3 三轮 36/36 | 最终 summary、L2 local-regression-04、final-audit |

| 批次 | 命令/实际结果 | 证据 |
| --- | --- | --- |
| controlled-01 | `test.ps1 -Level Controlled`：L1 153 个顶层测试、脚本 guard 和 live 编译/零匹配门槛通过；Controlled 12 个场景全部通过 | `.evidence/l3-stability-controlled-01/summary.json`、`controlled/result.json` |
| framework-02 | 12 项脚本检查通过，含新增 Repeat 拒绝、case 完整性、有界门真实 checker、网络作用范围拒绝 | `.evidence/l3-framework-02/result.json`、`stability-framework.log` |
| controlled-final-01 | L1 157 项、12 项脚本检查、live 编译/零匹配及 Controlled 12 场景均通过；后续仅对发布查询故障证据增加严格断言，待最终完整批次重新执行所有前置 | `.evidence/l3-stability-controlled-final-01/summary.json` |
| caller-diagnostic-01 | 失败：启动模型前发现 runtime index 根数组不满足严格对象解码；零业务 Run | `.evidence/l3-caller-diagnostic-01/summary.json`；`C:/srt/0d2899240453` |
| caller-diagnostic-02 | 失败：原生沙箱 GitHub DNS 解析失败，未发布；独立宿主 DNS/Git 查询正常。已公开 stop 原 Run 并确认服务退出，已知失败后终止诊断 harness，缺 result 不算通过 | `C:/srt/f2d95e47da0d/evidence/`；`.evidence/remote-network-read.json` |
| caller-diagnostic-03 | 整项失败：实际 PR #261 已合入，收尾确认；计数器错误要求显式 terminal.create，而实际 worktree.create 已创建原终端。原审计有 1 次工作区创建、4 次不同发送且全部原 runtime 回执完整；已补规则回归，不覆盖旧失败 | `C:/srt/ddc4d838eb05/evidence/scenario.json`；[测试 PR #261](https://github.com/erranliu/SteerlineTest/pull/261) |
| caller-diagnostic-04 | 单项通过：原请求找回、原调用方退出、零重复输入、真实 Git/PR/精确 head 合并、原资源停止、服务退出；192.82 秒 | `C:/srt/e36e032d3ce4/evidence/result.json`、`scenario.json`；[测试 PR #262](https://github.com/erranliu/SteerlineTest/pull/262)；merge `0a0dd84bf78e4e83b7e88d262201461cf6b3b71c` |
| stop-active-diagnostic-01 | 失败：原生准备查询连接不可用，业务输入未发送、未命中故障；原资源和服务均停止。后续五次独立只读状态连接均成功，保留这次失败，不推断其根因 | `C:/srt/8be0b0f12429/evidence/`；`.evidence/state-connectivity-probes.json` |
| stop-active-diagnostic-02 | 通过：真实 checker 控制门命中后接受 stop；授权撤销、原资源停止、零后继输入、无 PR、完整收尾，98.63 秒 | `C:/srt/6ea4053506b4/evidence/result.json`、`scenario.json` |
| restart-graceful-active-01 | 失败：新旧服务换代与继续门槛已命中，原生当前回合已完成，但测试错误要求可省略的 mainAgent.outcome 必为 success。对照既有 r7 native_outcome=unknown 且 verified=true 的通过证据后修正；未改生产完成规则，旧失败保留 | `C:/srt/be74b437c1f4/evidence/`；新增 completion 单元反例 |
| restart-graceful-active-02 | 通过：正常停服重启、授权前零输入、原会话显式恢复、真实合入与收尾，213.51 秒 | `C:/srt/5ba634d842d7/evidence/`；[测试 PR #263](https://github.com/erranliu/SteerlineTest/pull/263)，merge `d56a485b8b9b57518040468802aa572fadf73f50` |
| restart-kill-active-01 | 通过：核验 PID/创建时间后终止原服务、确认原写者结束、新实例只读恢复、显式 resume 继续原会话、真实合入与收尾，205.64 秒 | `C:/srt/ae19ae47f7fc/evidence/`；[测试 PR #264](https://github.com/erranliu/SteerlineTest/pull/264)，merge `f07b35912fc441a2163bc9144ad3493006e037c5` |
| remote-smoke-01 | 通过：原完整 smoke 含原会话反馈、真实合入、Run/服务停止，300.97 秒 | `C:/srt/eee473de1e6a/evidence/`；[测试 PR #265](https://github.com/erranliu/SteerlineTest/pull/265)，merge `a3ac664ad83e6785e7ad53f2deb3122bf1202336` |
| local-regression-01 | 失败：固定发布器已创建 PR #266，模型在其他沙箱认证上下文的额外查询得到 404；宿主随后核实 PR 真实存在。收尾另遇 stale_revision，已补充仅对明确拒绝的版本冲突刷新重试；旧现场通过原公开 CLI 重新建立观察、stop，确认授权撤销、原执行停止与服务退出。未重发业务输入，未改写失败 | `C:/srt/f06fda3656e4/evidence/`，含 post-failure-* 记录；[保留的失败测试 PR #266](https://github.com/erranliu/SteerlineTest/pull/266) |
| local-regression-02 | 通过：首次交付、原会话反馈、同一 PR 复用、真实精确 head 合入、Run/服务停止。测试材料明确由固定发布器输出真实源事实，Runner 负责独立平台核验，避免不同认证上下文的额外查询混淆源结果 | `C:/srt/981bb99a864b/evidence/`；[测试 PR #267](https://github.com/erranliu/SteerlineTest/pull/267)，merge `7234cdce89dbcce10730b449bd06ce67f2c6a287` |
| full-qualification-preflight-01 | 完整三轮入口已尝试：计划 36 次，执行 0 次，整体 failed。管理员网络宿主 ready.json 尚不存在，前置阻断，不启动真实网络故障 case | `.evidence/l3-full-qualification-preflight-01/summary.json` |
| admin-network-host | 用户确认 UAC 后，受限管理员网络宿主启动，PID/创建时间及 elevated 身份核验通过；主测试保持普通身份。真实 block/restore 已执行 | `.evidence/admin-network/ready.json`、队列动作回执；`C:/srt/217e6ae877dc/evidence/network-blocked.json`、`network-restored.json` |
| connection-diagnostic-01 | 通过：真实双通路预检、活动回合断连、原实例自动恢复、真实合入及完整收尾，220.95 秒。此诊断 driver 冻结早于新增 60 秒窗口及独立失败重试判定，不能代替最终构建资格 | `C:/srt/217e6ae877dc/evidence/`；[测试 PR #268](https://github.com/erranliu/SteerlineTest/pull/268)，merge `0b4fed7eda98241e48eeb00d442db421c95e6dc6` |
| controlled-final-02 | L1 158 项、脚本检查 12 项、live 编译/零匹配和 Controlled 12 场景通过。后续补强失败观察意图配对及网络关闭逐条审计，最终批次仍重跑前置 | `.evidence/l3-stability-controlled-final-02/summary.json` |
| restart-graceful-boundary-01 | 通过：真实断连 60.66 秒、原回合完成但未交付给 Runner、正常服务换代、授权前零输入、显式恢复原会话、真实合并及收尾；273.33 秒 | `C:/srt/2b3c8eabae79/evidence/`；[测试 PR #269](https://github.com/erranliu/SteerlineTest/pull/269)，merge `c591ce543c3da483a5c53f67a3dd7b4c7eefeb21` |
| restart-kill-boundary-01 | 失败：活动门命中后网络注入 API 返回 317，未命中服务终止；原执行、服务和规则均已清理，101.05 秒。宿主已验证 elevated，旧证据缺少错误瞬间连接重新查询，不能据此断言权限失效或连接消失。注入器已增加逐次原始返回码/身份/同一连接重查；非零码只有在确认原连接消失时才继续，双通路探针契约不变 | `C:/srt/3e1108645e32/evidence/`；`.evidence/l3-admin-diagnostics-01.json`；新现场 `C:/srt/decdf154e061` 重跑 |
| restart-kill-boundary-02 | 通过：异常终止原服务、原写者退出、新实例授权门槛与零新输入、显式继续原会话、真实合并及完整收尾，270.26 秒 | `C:/srt/decdf154e061/evidence/`；[测试 PR #270](https://github.com/erranliu/SteerlineTest/pull/270)，merge `c76186ae6ece94337bffa47281c4889e730e96a0` |
| stop-boundary-01 | 失败：真实断连后 stop 在原生关闭前失败，恢复后生产适配器未实现该停止进度的对账，持续 `unknown / unsupported terminal reconciliation`，307.40 秒。失败后的独立 cleanup stop 在恢复链路上确认原执行停止，服务与设施亦收尾；不把 cleanup 成功改记为 case 通过 | `C:/srt/c29230c16566/evidence/`；`.evidence/l3-admin-diagnostics-02.json` |
| stop-progress-fix | 已先复现受控失败再修复生产 owner：停止回执区分 not-attempted/attempted/confirmed；中心在原操作绑定下持久保存进度。只允许明确尚未尝试关闭的原操作继续首次关闭；已尝试且丢回执、缺失/损坏/外来进度保持未知且不重复关闭。真实 SQLite 重载及适配器反例通过，当时等待完整前置与真机；后续 stop-boundary-02 及最终三轮均通过 | `internal/execution/adapters/orcatransport/stop_test.go`；`internal/application/stop_progress_test.go`；`.evidence/l3-stability-controlled-final-03/` |
| controlled-final-03 | 整体失败：L1 160 项通过，Controlled 实际执行 12 场景，其中跨重启撤销测试 stop 返回退出码 5。旧日志未展开具体错误码，不断言精确根因。测试已补完整错误信封和仅限明确 stale_revision 拒绝后的有界刷新重试；未知结果绝不重发。新增原生 incarnation 变化及已确认 stop 重放不重复关闭的回归亦通过，另建完整前置批次 | `.evidence/l3-stability-controlled-final-03/summary.json`、`controlled/integration.log`；新批次 `.evidence/l3-stability-controlled-final-04/` |
| controlled-final-04 | L1 161 项、脚本检查 12 项、live 编译/零匹配和 Controlled 12 场景全部通过；包含生产 stop 进度修复及原生身份反例。最终批次会再次执行完整前置 | `.evidence/l3-stability-controlled-final-04/summary.json` |
| stop-boundary-02 | 通过：原回合完成但未交付时接受 stop，恢复通信后由 Runner 继续同一未尝试关闭的停止操作并确认原生停止；撤销后零新输入、无 PR、完整收尾，未由 driver 代清理，185.28 秒 | `C:/srt/b0cb0f1e98fc/evidence/`；`.evidence/l3-admin-diagnostics-04.json` |
| stop-unreachable-01 | 通过：真实 240 秒不可达窗口跨服务重启，保持 unknown、撤销和恢复门槛，零新输入；恢复后 Runner 确认原生停止，设施与服务收尾，336.19 秒。此诊断冻结早于新增反馈 resume_required 与旧 revision stale_revision 两种错误码的独立严格断言，最终资格使用补强版本 | `C:/srt/8c66fc33133b/evidence/`；`.evidence/l3-admin-diagnostics-04.json` |
| local-regression-03 | 失败：原生回合在 Windows PowerShell 5 用默认 Get-Content 读取无 BOM 的 UTF-8 Spec，已显示乱码，随后将错误文本写入产物；真实 checker 报错后仍提交，真实发布器再次检查并阻断。未创建 PR，原 Run 与服务停止。已在冻结 Spec/Plan 明确 UTF-8 解码及 checker 非零时不得提交/报成功，新现场重跑，旧失败不改写 | `C:/srt/b0ac088cc30b/evidence/`，含 post-failure-source-read.json、post-failure-pr-list.json；`.evidence/l3-local-regression-03/summary.json` |
| local-regression-04 | 通过：明确 UTF-8 读取指引后，首次交付、原会话反馈、同一 PR 复用、精确 head 合入及 Run/服务停止完成；L1 161 项、Controlled 12 场景前置亦通过 | `C:/srt/96158bdc5ce7/evidence/`；`.evidence/l3-local-regression-04/summary.json`；[测试 PR #272](https://github.com/erranliu/SteerlineTest/pull/272)，merge `3fd1ec602a5b7700f1d0ceae6f3c55f2c0e440a5` |
| publication-completion-gap-01 | 通过：真实 PR 创建后切断双通路，原发布回合完成且 Runner 未取得 final；恢复只读对账同一 PR/head，发布器一次、真实 PR 一个，精确合入与完整收尾，262.12 秒 | `C:/srt/2e081bc3f289/evidence/`；[测试 PR #271](https://github.com/erranliu/SteerlineTest/pull/271)，merge `a3e242531eb0a9328c31c626d991101936bbfcad` |
| publication-query-unavailable-01 | 通过：同样的真实发布完成缺口后，实际 provider 查询故障命中，Runner 处于 pr_unverified；恢复查询后才核验原 PR/head并合入，零重复发布、完整收尾，279.27 秒 | `C:/srt/1e3fd1278d88/evidence/`；[测试 PR #273](https://github.com/erranliu/SteerlineTest/pull/273)，merge `a66d84c0a3564bb913dd233ea6eb1ae9e8bdf7c2` |
| full-qualification-02 | 失败并停止后续派发：计划 36，实际 7，通过 6，失败 1，无排除，首次通过率 6/7。第 1 轮 graceful-boundary 在辅助终端 close 遇 remote_runtime_unavailable，业务 Run/服务/网络已停止，但辅助终端尚活；随后按原 handle/incarnation 独立收尾并取得 ptyKilled=true。派发停止时第 7 个独立 harness 已启动，允许其用原冻结构建完成并另经冻结 helper 验证通过（PR #279）；其余 29 次未执行，不拼接成资格 | `.evidence/l3-full-qualification-02/summary.json`、`summary-before-dispatch-finalize.json`、`dispatch-stop.json`；失败现场 `C:/srt/6637c859afd2/evidence/`，含 post-failure-aux-*；第 7 项 `C:/srt/2a5672744269/evidence/`，PR [#279](https://github.com/erranliu/SteerlineTest/pull/279) |
| auxiliary-facility-fix | 只读显式连接错误有界重试，通用包装器绝不重放 create/send/close。测试辅助终端关闭单独按原 runtime/handle/incarnation/worktree 对账：完整清单证明不存在才确认清理，原实例仍活才继续同一关闭意图；身份变化/清单不完整/持续失败均不通过。所有辅助终端及未知创建纳入设施清理登记，不能再仅凭网络恢复声称设施收尾。反例、live 编译和 vet 通过，原失败保留 | `internal/testkit/livetest/diagnostic_test.go`；`tests/native/stability_test.go` |
| full-qualification-03 | 通过：三轮各 12/12，实际 36/36，首次通过率 100%，无排除。北京时间 05:45:36–08:33:35，约 2 小时 48 分钟。L1 163 项、脚本检查 12 项、Controlled 12 场景完整前置通过；288 个来源文件与配置均未变化，36 次 Runner/driver 及 33 次稳定性工具摘要一致。36 个独立 Run、27 个真实精确 head 合入 PR，6 个 stopped、3 个 revoked-unverified；未用 driver 代替原生停止。199 个稳定性控制辅助终端配对收尾，36 个服务已退出，本批规则为零，21 个最后注入记录中的 watchdog 及管理员网络宿主均退出 | `.evidence/l3-full-qualification-03/summary.json`、`final-audit.json`、`stage-timings.json`、`source-snapshot.json`、`config-identity.json`、`source-verification-r1.json`、`source-verification-r2.json`、`watchdog-cleanup.json`、`post-run-shells.json`、`admin-host-cleanup.json` |

故障、权限不足、未命中、超时、身份未知、无结果和清理未确认均不通过。诊断过程中源码调整后的旧现场仍保持冻结；最终资格另建批次，不混用不同构建。完整原始运行记录保留在现场与批次目录；本页仅登记事实，不重写历史结果。

## 最终结论与收尾

本版本首批 L3 稳定性验收完成。结论仅来自 full-qualification-03 的同一构建完整 36 次，不拼接诊断或旧失败批次。生产停止进度修复、UTF-8 材料指引及辅助设施对账均已验证；此前失败、独立后续清理与原始证据保留。

主测试全程使用普通 token；用户确认 UAC 后，受限管理员网络宿主仅执行本轮按专属程序路径隔离的 block/restore。最终独立核对确认全部 36 个服务 not_running、199 个稳定性控制辅助终端创建/关闭身份一一对应、本批防火墙规则为零、21 个最后注入记录中的 watchdog 已退出；批次期间创建的 PowerShell 进程无残留（post-run-shells.json）。管理员网络宿主也已在 08:35:58 退出，队列无待处理请求。

最终命令为 `test.ps1 -Level L3 -Repeat 3 -Config .evidence/live-stability-config.json -OutDir .evidence/l3-full-qualification-03`。60 秒断连及 240 秒不可达窗口、原身份、授权隔离和真实 PR 契约均未放宽。阶段耗时来自原始 driver 时间点与 Go test 输出，另存 stage-timings.json；原 smoke 无故障 journal，未虚构更细业务阶段耗时。L2 最新通过现场的 Runner SHA256 与最终 L3 构建一致。

## 最终三轮明细

`merged` 表示真实 PR 精确 head 合入；`stopped` 表示原执行停止；`revoked-unverified` 表示不可达故障窗口中正确保留未知，最终原资源仍已停止。

| 场景 / case | 第 1 轮 | 第 2 轮 | 第 3 轮 | 业务结果 |
| --- | --- | --- | --- | --- |
| orca-remote.caller-exit/accepted-response-lost | 通过 [#281](https://github.com/erranliu/SteerlineTest/pull/281) | 通过 [#290](https://github.com/erranliu/SteerlineTest/pull/290) | 通过 [#299](https://github.com/erranliu/SteerlineTest/pull/299) | merged |
| orca-remote.connection-reconnect/active-disconnect | 通过 [#282](https://github.com/erranliu/SteerlineTest/pull/282) | 通过 [#291](https://github.com/erranliu/SteerlineTest/pull/291) | 通过 [#300](https://github.com/erranliu/SteerlineTest/pull/300) | merged |
| orca-remote.publication-unknown/completion-gap | 通过 [#287](https://github.com/erranliu/SteerlineTest/pull/287) | 通过 [#296](https://github.com/erranliu/SteerlineTest/pull/296) | 通过 [#305](https://github.com/erranliu/SteerlineTest/pull/305) | merged |
| orca-remote.publication-unknown/query-unavailable | 通过 [#288](https://github.com/erranliu/SteerlineTest/pull/288) | 通过 [#297](https://github.com/erranliu/SteerlineTest/pull/297) | 通过 [#306](https://github.com/erranliu/SteerlineTest/pull/306) | merged |
| orca-remote.service-restart/graceful-active | 通过 [#283](https://github.com/erranliu/SteerlineTest/pull/283) | 通过 [#292](https://github.com/erranliu/SteerlineTest/pull/292) | 通过 [#301](https://github.com/erranliu/SteerlineTest/pull/301) | merged |
| orca-remote.service-restart/graceful-boundary | 通过 [#285](https://github.com/erranliu/SteerlineTest/pull/285) | 通过 [#294](https://github.com/erranliu/SteerlineTest/pull/294) | 通过 [#303](https://github.com/erranliu/SteerlineTest/pull/303) | merged |
| orca-remote.service-restart/kill-active | 通过 [#284](https://github.com/erranliu/SteerlineTest/pull/284) | 通过 [#293](https://github.com/erranliu/SteerlineTest/pull/293) | 通过 [#302](https://github.com/erranliu/SteerlineTest/pull/302) | merged |
| orca-remote.service-restart/kill-boundary | 通过 [#286](https://github.com/erranliu/SteerlineTest/pull/286) | 通过 [#295](https://github.com/erranliu/SteerlineTest/pull/295) | 通过 [#304](https://github.com/erranliu/SteerlineTest/pull/304) | merged |
| orca-remote.smoke/default | 通过 [#280](https://github.com/erranliu/SteerlineTest/pull/280) | 通过 [#289](https://github.com/erranliu/SteerlineTest/pull/289) | 通过 [#298](https://github.com/erranliu/SteerlineTest/pull/298) | merged |
| orca-remote.stop-revocation/active | 通过 [现场](C:/srt/4a998ce12213/evidence/scenario.json) | 通过 [现场](C:/srt/d8f8e8ffd1c7/evidence/scenario.json) | 通过 [现场](C:/srt/024474849fad/evidence/scenario.json) | stopped |
| orca-remote.stop-revocation/boundary | 通过 [现场](C:/srt/adb892f77012/evidence/scenario.json) | 通过 [现场](C:/srt/833226eddf42/evidence/scenario.json) | 通过 [现场](C:/srt/8ca340f2bae7/evidence/scenario.json) | stopped |
| orca-remote.stop-revocation/unreachable | 通过 [现场](C:/srt/4c92a234918b/evidence/scenario.json) | 通过 [现场](C:/srt/ff82e4d1cbca/evidence/scenario.json) | 通过 [现场](C:/srt/48a31b7798ea/evidence/scenario.json) | revoked-unverified |
