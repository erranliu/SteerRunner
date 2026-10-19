# 2026-10-18 完整 L2/L3 复验（未全通过）

用户要求运行当前目录全部 L2/L3，发现问题直接修。SteerLine 保持不改，不部署产品。

最终结果：L2 **2/2 通过**；L3 **4/12 通过、1 失败、7 项环境阻塞**。完整产品 L1 304 个根测试、最后构建的 Controlled 13 项通过；随后测试侧变更的受影响工具包 18 个根测试通过。没有把本次结果标为完整 L3 通过。失败原因与未执行项如下，修复、真实 PR/合入和清理证据均保留。

## 范围与证据

冻结本轮开始时 `tests/catalog.json`：L2 两个 case（结果获取包含三个内部原生场景），L3 六个场景、十二个 case；不增加其他工作树的扩展目录，不使用 `-SkipAdminRequired`。

随仓库保留的[逐项结果及原证据 SHA](case-results.json)、[源码文件清单](source-files.json)、[最后产品 L1](l1-summary.json)、[完整 L2](l2-summary.json)、[Controlled](controlled-result.json)及[完整 L3 入口阻断](l3-full-preflight.json)与下述原始现场对应。case 目录只复制非秘密结果/身份/清理摘要，原生输出和原 Record 留在各自冻结现场。

原始证据保存在本工作树 `.evidence/`，失败批次与成功复测分别保留：

- `full-l2-20261018-r1/`：初始诊断。源码修复期间启动，不能独立作为最终源码的完整通过依据。
- `full-l2-20261018-r2/`：修复后完整 L2 批次。
- `full-l3-diagnostic-20261018-r1/`：不需要管理员网络注入的 smoke/caller-exit 先行诊断，属于子集。
- `full-validation-20261018/`：移植前备份、定向三方合并清单、组件回归、应用回归及远端测试辅助程序 SHA/安装/关闭证据。

## 已处理问题

1. 当前工作树缺少主线已提交的远端可信认证 runtime，GitHub Orca 运行会在业务输入前报 `remote_sandbox_authentication_unsupported`。从已提交实现定向移植认证、同沙箱门槛及生命周期组件，保留当前 workflow/2、独立 parser、单次补报和无 Spec 审计的定义。没有合并其他工作树的工作流与测试矩阵。
2. 受限 Git 写入授权需要每 Run 独立仓库，避免共享缓存 `.git` 被业务执行写入；接入原冻结/恢复边界，原缓存只用于准备。
3. Orca CLI 的环境文件刷新需要跨调用方互斥；复用已提交的可恢复原生锁，覆盖 Runner、准备脚本和真机驱动的外层诊断调用。杀死服务后锁可恢复，不重发未知业务输入。
4. 修复 Markdown `[URL](URL)` 被错误合并成一个 PR 地址的问题；不同候选仍保留歧义，provider 的仓库/base/head/state 核验继续执行。
5. 修复恢复时原输入已消耗、原 final 尚待补读却被过早投影为 blocked 的窗口：继续观察原结果，不重发输入；已结算旧输入仍不得被技术 resume 变成新业务执行。
6. 补齐发布夹具、原生回执计数和远端权限审计接线；修正 MSIX 空别名造成的假权限反例及合法单次补报被拒绝的测试断言。后备必须同源且仅一次，不放宽原生权限、成功行匹配或业务完成核验。

相关三方合并依据为 `f211e316` → `d4969d0` → `62aee13` 的限定模块；具体路径/模式见原始清单。其他工作树只作已提交源码和非秘密环境参考，未修改或停止其资源。

## 环境与阻塞项

Windows/amd64，Go 1.26.4；本机及指定远端 Codex 0.159.2。远端 environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`，runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`。本次辅助程序使用独立的 `C:/sr-runtime/1c3c-full-20261018/` 内容寻址目录，原生主机认证不复制。

完整 L3 的网络注入需要本工作树专用管理员宿主 `C:/srt-admin-1c3c-full-20261018`。当前会话非管理员，已向用户请求按仓库脚本启动；不借用其他工作树的宿主，不将缺失项记为通过。

本轮结束时管理员 ready.json 仍不存在，七项未调用原生 agent；另有一项真实强杀恢复失败。可运行场景已经结束，不保留测试协调器等待后台继续。

## 最后实现的复验

完整 L1：`.evidence/final-l1-20261018/summary.json`，304 个根测试通过；6 个目录约定的 opt-in/helper 跳过不算作原生通过。最后实现的完整 Controlled 13 项已重新通过，见 `.evidence/final-controlled-20261018/result.json` 及 [构建身份](controlled-source-identity.json)、[完整日志](controlled-integration.log)。此前 r5 的 Controlled 使用 `c8638979…` 构建，保留为历史前置，不算最后实现通过；后续以同一个 `69c3f04a…` 构建补齐全量，最后 smoke 在此前置完成后启动。各诊断批次的真实时间顺序及前置引用均保留，没有伪装成一次标准完整入口调用。最终产品 Runner SHA-256 为 `69c3f04a336eae6bcd55962d4e518757ca6e7ddf516e7213cd4142c31e847c93`。

完整 L2 最后批次 `.evidence/full-l2-20261018-r6/summary.json` 两项通过、零排除：

| 场景 | 原生现场 | 结果 |
| --- | --- | --- |
| local-codex.result-acquisition/default | `C:/srt/130e2f841585` | 三个内部原生场景及真实发布闭环通过；PR [479](https://github.com/erranliu/SteerlineTest/pull/479) 已核验合并 |
| local-codex.smoke/default | `C:/srt/1077862978d8` | 实施、发布、实际 head 合并和内容核验通过；PR [480](https://github.com/erranliu/SteerlineTest/pull/480) 已核验合并 |

本次修复 parser 权限反例的测试环境选择：WindowsApps 中零长度 MSIX `pwsh` 别名会使命令根本没有执行。反例现在只在子进程 PATH 排除该别名，并显式选取系统 Windows PowerShell；要求原生命令真实运行、原生拒绝写入、canary 不变及进程树停止。原生 `read-only/mxc/never` 不变，产品 parser 仍关闭 shell。r6 的 `permission-probe.json` 五项证据均为 true；没有把启动失败当作权限拒绝。

远端 smoke 的测试断言曾将合法的一次 report 误判为失败：原 final 实际写成 `實施及必要驗證已完成.`，不等于阶段声明 `实施及必要验证已完成。`，产品正确进入一次补报并成功核验 PR 481。保留该失败批次，不改成通过。新断言读取原 Record 的冻结输入、原 final 和来源：完整机器结果/精确声明/唯一 PR 线索可满足程序提取时不得额外补报，否则仅接受一次同源补报；development 不得调用 parser，原生失败和补报业务工作不得被覆盖。测试工具包的正反例、两份原现场只读复核、native 编译和零匹配拒绝证据在 `.evidence/full-validation-20261018/native-acquisition-assertions.jsonl`、`check-acquisition-*.log`、`native-assertions-final-compile.log`、`native-assertions-final-zero/result.json`。该项仅改测试代码；最后产品 L1 后重跑整个受影响测试工具包。

最后 L3 按原 case 分别核验，不把被中断的协调批次整体记为通过。已通过 caller-exit/accepted-response-lost（`C:/srt/7c060e67863f`，PR 477）、stop-revocation/active（`C:/srt/b0a772b169f6`，无 PR）和 service-restart/graceful-active（`C:/srt/646c11159dbb`，PR 483）。每项独立驱动完成后都用该现场冻结的 `stability.exe validate-result -manifest <manifest>` 复核成功；原身份、必要断言、原 Run/服务停止及设施恢复均确认。协调器为串行化或测试代码更新而终止时，原生 harness 继续完成；只补记其真实 case 结果，批次根状态仍保留 interrupted。`kill-active` 最后现场结果见下述原生缺口；远端 smoke 补齐测试入口后的最终新现场在 `.evidence/full-l3-smoke-20261018-r7/`。

| L3 场景 / case | 最终状态 | 现场 / 原因 |
| --- | --- | --- |
| smoke / default | 通过 | `7db0698e1e63`，PR [498](https://github.com/erranliu/SteerlineTest/pull/498) 已核验合并 |
| caller-exit / accepted-response-lost | 通过 | `7c060e67863f`，PR 477 |
| connection-reconnect / active-disconnect | 阻塞（1） | 缺本工作树管理员网络宿主 |
| service-restart / graceful-active | 通过 | `646c11159dbb`，PR 483 |
| service-restart / kill-active | 失败（1） | `20550476584f`，原只读 inspector 创建身份丢失，未恢复交付 |
| service-restart / graceful-boundary、kill-boundary | 阻塞（2） | 缺本工作树管理员网络宿主 |
| stop-revocation / active | 通过 | `b0a772b169f6`，无 PR |
| stop-revocation / boundary、unreachable | 阻塞（2） | 缺本工作树管理员网络宿主 |
| publication-unknown / completion-gap、query-unavailable | 阻塞（2） | 缺本工作树管理员网络宿主 |

最终 smoke 的驱动结果 exit=0、scenario_count=1，Run/服务停止确认。它实际走过一次同源实施补报，发布程序提取并经 provider 核验；远端原生权限审计取得 4 个 context、9 次工具调用、0 escalation/approval/bypass，inspector_closed=true，runtime SHA 与冻结的 `7a6d2d56…` 一致。实际 PR head、合入对象和内容均核验通过。驱动 SHA `d57b0842dd30903e0bb497c0ce6eaaec965caca56edc95033ff37fb1091d55ed4` 与最后编译/零匹配检查相同；没有用旧驱动结果替代新测试接线。

r5 的 smoke（`C:/srt/725225653e5a`，PR 486）通过新获取断言并到达 pr_ready，但旧测试仍硬编码“远端原生权限审计不支持”，因此仍记失败、未合并。补接已提交 `62aee13` 的远端权限历史核验工具和 native 入口，不引入主线的其他本机审计扩展；CLI 调用沿本轮串行锁，原会话/工作区/provider 关联、权限合同/access mode、冻结 runtime SHA 和独立诊断 terminal 的确认关闭都必须通过。新增工具包反例拒绝异源会话/工作区、未验证回执、错误 runtime/坏回执，并验证只关闭 inspector、不发送业务输入；证据为 `.evidence/full-validation-20261018/native-remote-audit-regression.jsonl`、`native-remote-audit-final-compile.log`、`native-remote-audit-final-zero/result.json`。该修正仅在测试侧；完整受影响工具包再次通过，r6 因上游 kill-active 未通过而在原生调用前停止；最终新 smoke 在 `.evidence/full-l3-smoke-20261018-r7/` 验证，不将未执行 r6 计入通过。

`kill-active` 新现场 `C:/srt/20550476584f` 命中了另一窗口：只读回执 inspector 的创建意图已保存，服务在收到并保存 `terminal.create` 的资源身份前被杀，留下无 Process 的原未知创建。原业务执行已由独立原生查询证明结束，Runner 没有重发输入，但不能丢弃未知创建或制造一个替代 inspector 来宣称恢复。原 CLI help 和固定 CLI 源码证实 `terminal.create` 没有调用方幂等键/按调用 ID 对账，且不属于原生 durable mutation；当前完整 inventory 只有业务 terminal，也不证明不存在晚到创建。详见[原生缺口证据](native-create-gap.json)及[CLI 帮助](orca-terminal-create-help.txt)。这是远端执行回执资源恢复缺口，独立 parser 的真实 L2 已通过，不能混为 parser 不可用。

该缺口没有在本轮以清空 journal、放宽来源、改变 case 的 merged 预期或重试创建来“修复”。要自动恢复，需先补齐该创建的可恢复原生身份/回执能力，或另行设计有持久派发握手的 CLI 资源 owner；后者会扩展现有进程恢复边界，本轮不临时引入第二套状态权威。原失败现场已按原期限结束，驱动结果 exit=1；独立认证撤销在原 owner 仍可验证时执行并确认 broker/socket 关闭，manual inspector 已确认关闭。原服务已退出，后续完整 inventory 为空，物理资源清理通过独立核验；原 Run 仍 stop_status=unknown，原服务 last_shutdown.resources_settled=false。这些结论在[独立撤销](kill-active-final-manual-authentication-cleanup.json)、[清理复核](kill-active-final-manual-cleanup-complete.json)中分开记录，不改变原测试或原 Run 的 stop 结论。完整 L3 因此不能宣称通过。

最后完整 L3 入口 `.evidence/full-l3-20261018-final-preflight/summary.json`（08:05:55 UTC）已展开全部 12 case、零排除，在管理员宿主 preflight 失败，未调用原生 agent。七个缺宿主 case 为 connection-reconnect/active-disconnect、service-restart/{graceful-boundary,kill-boundary}、stop-revocation/{boundary,unreachable}、publication-unknown/{completion-gap,query-unavailable}；不以无网络断连的近似场景替代。

复现入口：

```powershell
./scripts/test.ps1 -Level L2 -Config .evidence/full-live-config-20261018.json -OutDir .evidence/full-l2-next
./scripts/test.ps1 -Level L3 -Config .evidence/full-live-config-20261018.json -OutDir .evidence/full-l3-next
```

完整 L3 前须在管理员 PowerShell 启动本工作树宿主：

```powershell
& 'C:/Users/lmas1/.codex/worktrees/1c3c/SteerRunner/scripts/stability-admin-host.ps1' -Directory 'C:/srt-admin-1c3c-full-20261018'
```

该宿主只执行仓库固定网络注入脚本并核对夹具与脚本 SHA。已有其他工作树的管理员宿主不属于本次资源，不能借用其身份或停止其服务。

## 收尾

成功现场的原 Run、服务、认证撤销及原工作区空 inventory 均按各自驱动核验。失败强杀现场另有上述独立物理清理证明，原聚合仍保留未知；不得将其标成原 Runner 自动停止通过。所有本轮测试协调器与 native harness 已结束；本机按本轮现场白名单复核无活动 Runner/native harness。SteerLine 仍为干净的 `bfe392447591137e6d762f19daf90cdc828aa428`，未修改或部署产品、未提交/推送本分支。测试 PR 及分支保留作为证据。

## 迭代记录

- 初始应用回归发现 warm-cache 测试仍要求两个 Run 共用 Git 管理目录。改为核验缓存刷新、冻结输入不变、独立 Git common directory 和无共享 objects alternates；定向回归通过（69.915 秒）。独立 Git 授权反例回归通过（20.929 秒）。
- 初始真实结果获取场景：parser-default、parser-boundaries 通过，workflow-smoke 在业务输入前因 authentication_api_binding_mismatch 失败。补齐权限构建请求中的 RunDirectory/RepositoryURL 和独立仓库根锁文件授权；失败现场 C:/srt/653c551d783d 保留。
- 完整应用包实测 345.973 秒，超过旧 5 分钟上限；入口每 package 上限改为 8 分钟。r2 L2 / r1 L3 子集在旧前置失败后均未调用模型。r3 L2 / r2 L3 子集使用新上限重新验证。

- 最新源码 L1 完整通过 299 个根测试（full-l3-diagnostic-20261018-r2），native 驱动编译及零匹配拒绝门槛通过；六个 catalog 允许的 opt-in/helper 跳过保留，不计入 L2/L3 通过。L2 r3 的 L1 298 项先通过，新增授权反例另有定向通过。
- 本次 staging worktree 自动建立的空 shell 与安装终端均按原 handle 关闭，ptyKilled=true；只停止本次资源，不影响其他工作树。

- L3 旧原生计数器会把新版 runtime 的只读回执查询终端算作额外业务 agent。接入已提交的严格 NativeReceiptAuditPolicy：固定 helper/SHA/config digest、原就绪回执、完整输入、原 terminal incarnation、实际只读回执及确认关闭必须一致。标题或任意附加命令不能豁免。对应审计包回归（含错误身份/输入/关闭等反例）通过，2.043 秒；未引入其他分支的新测试场景。

- L2 r3 新发现夹具移植遗漏：当前驱动要求固定发布脚本，而导入片段仅为 L3 复制 publish/pr-list/publication 元数据。本轮恢复所有 smoke 的同一发布夹具，不改已冻结现场；框架正反例复验通过，r4 两个新 L2 现场复测。
- 真实 L3 smoke 创建 PR 466 后暴露产品缺陷：`[URL](URL)` 被宽泛 URL 正则合并成单个地址，导致 provider 持续拒绝错误地址。修正 Markdown/括号边界并去重，相异 label/destination 继续保留歧义；provider 仓库/base/head/state 核验保持。新增实测 final 回归及发布来源回归，r5 从完整 L1/Controlled 开始重跑 L2；旧现场/批次继续保留真实失败与源码身份，不跨二进制合并通过结论。

- r4 夹具修正后的完整 L2 两个 case 均通过，PR [468](https://github.com/erranliu/SteerlineTest/pull/468)、[470](https://github.com/erranliu/SteerlineTest/pull/470) 已由驱动核验并合入，原 Run/服务已停止。r4 完成于 URL 产品修正前，因此最终源码仍以 r5 新批次为准。
- URL 修正后的结果获取包回归通过（1.122 秒），发布来源漂移/错误 PR head 应用回归通过（56.531 秒）。旧远端失败现场 PR 466、469、471 保留各自独立 base/head 证据，已附到本任务；不把这些旧二进制运行记为最终通过。PR 466 失败后的原 broker 撤销、原工作区空 inventory、Run/服务停止均已独立确认。

- 新版 kill-active 复验发现接受后的异步结果窗口：原输入已从 PendingTurn 消耗，CurrentTurnID 尚未成为 LastStoppedTurnID，native 已结束但原 final 的只读补读尚未提交。resumeWorkflow 过早投影 blocked，驱动据此停止。修正为 reconciling/original_result_pending，仅继续原来源观察；不新增 Workflow Step、PendingTurn、意图或业务输入。已结算旧输入仍保留 consumed-input 阻塞规则。新增端到端 actor 单测验证恢复后原 final 到达且发送次数为零；与旧 consumed-input、原报告来源、观察提交门槛合计四项定向回归通过（4.191 秒）。旧故障现场 C:/srt/8360478f92e0 已确认原生/服务停止及设施恢复。
- L1 303 根测试在该恢复投影修正前通过；final-l1-20261018 在修正后完整重跑，不把定向通过冒充最后全量通过。r5 的 13 项受控前置期间修改了该恢复投影，原生现场在前置完成后使用新构建；批次与差异通过最终源码清单及独立完整 L1 关联。
