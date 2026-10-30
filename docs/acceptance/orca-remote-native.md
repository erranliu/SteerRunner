# 验收：远端仅通过 Orca 执行

历史构建验收记录：以下 Check、旧状态升级和已删除源码的描述仅说明当时构建，不是当前入口。2026-10-29 已退役旧业务，当前边界见[退役记录](check-retirement.md)；原始失败、资源与结算证据保持。

日期：2026-10-19。归属 [Spec](../spec/orca-remote-native.spec.md) / [Plan](../spec/orca-remote-native.plan.md)。已完成 P0 原生探针、查询修复、helper 删除及 unreported 最小修正；完整闭环、活动停止、结果阻断和上层契约验收通过（按当前 workflow v2 边界）。最终审计另修复持久结果回放的原 outcome 丢失，最新构建及补验见文末；以下历史阶段记录保留。详见首轮 [P0 实测报告](../archive/2026-10/acceptance/orca-remote-native-p0-20261019.md)及[补验报告](../archive/2026-10/acceptance/orca-remote-native-p0-query-fix-20261019.md)。

## 2026-10-24 n308-19 真实远端 L4 最终结果

最终运行时代码 `4e7efd80662fbd4d5996cfe0472450a01b76dff5` 已 rebase 到主线 `aea8abed904c1100b43efc3d5b7ce62584cedd07`。第九、十轮使用同一个 EXE，SHA256 为 `4b1a978e25364e4c1ed948c36f7c3d60c4fb707da4ad8e97390419cc9a1fce1e`，连续两轮完成下述限定范围验收。本文后续出现的 0/2、待复验及失败记录均为历史阶段，不能覆盖本节的最终结果；也不能据本节追认旧构建的失败轮次。

固定材料为 `merge-h-308-code` 的 `SteerlineFlow/spec/n308-19.spec.md` 与 `SteerlineFlow/plans/n308-19.plan.md`，两轮均从 `afe67b539e787fe05ce58876d612a6edb45e25f7` 独立创建工作区、分支和原生会话，PR 目标均为 `AIRefactor`。使用原远端 Orca 1.4.223、原生 Codex、gpt-6-luna xhigh/full_access、development/2 及原业务提示；未修改、构建或部署 Orca。为保持该复跑条件，运行时及资源中的发布提示保留参考运行的原文本，与 rebase 主线新增的详细发布指引存在有意差异。

| 轮次 | Run / request | Runner 核验及独立 Gitea 核验 | 最终用量 |
| --- | --- | --- | --- |
| 9 | `1cbe100a473cd6e23fe93adcd3bc27bd` / `l4-n30819-20261024-r9-submit` | `pr_ready`；[PR415](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/415)，head `7fe8804471b256ff7b382017478a0bffd622cca2`，46 文件 | snapshot365，222 响应，33,535,318 tokens |
| 10 | `11d84e6236ee7c7cac8a15371d4ccfed` / `l4-n30819-20261024-r10-submit` | `pr_ready`；[PR416](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/416)，head `11dff4b806e2439d369cebe6c8b3a383cc84a154`，52 文件 | snapshot381，239 响应，36,654,946 tokens |

两轮分别具有一个非空实现提交，父节点均为固定基线，原 Spec/Plan 与四项测试定义未改。PR 分支与对应 Run 精确匹配，均保持 open/unmerged。两个原生业务回合（implement、publish）各发送一次，唯一原 request、attempt、provider/pane 和启动摘要均核验；公开最新消息与原生正文一致，长正文按既有 1200-byte UTF8 摘要约定展示，没有替换字符。结束和最终正文由独立原 `worktree.ps`、持久原生回执及公开结果交叉核验，不以屏幕或 rollout 日志作为 final。

测试来源需区分：两轮代理报告均产生于实现提交前，原报告和早期失败记录保留，未计作提交候选的当前通过。随后在各自实际远端 PR 提交头，通过原 `tools/run-managed-check.ps1` 独立复跑原四项检查 `structure.asmdef-governance`、`structure.asmdef-source`、`structure.resource-contract`、`structure.diff-check`。实际报告均为 before/after=head、clean=true、Passed/exit0，报告文件 SHA/字节数和原定义逐项核验；固定 base/head 的独立 `git diff --check` 也通过。新证据位于原工作区被忽略的 `.steerrunner/artifacts/l4-r9-postcommit-*`、`l4-r10-postcommit-*`。未手工实现、提交、push 或创建交付 PR，未手写测试通过报告。Windows 长路径设置仅限测试子进程，不改共享 Git 配置。这四项是结构检查，不证明 Unity 编译、玩法或 T92 行为。

用量均 available/finalized，各含初始化 1 响应；分别 6 页和 7 页的唯一事件、求和及同版本不可变复查通过。基于真实快照、已核对官方价格及明确声明的 Standard/no-region token-only API 等价场景，独立 Decimal 复算分别为 USD0.53510436、USD0.57097996，合计 USD1.10608432；合计 tokens 为 70,190,264。实际 native tier 未知，严格策略保留 `PRICING_CONTEXT_UNKNOWN`，坏价表隔离通过。此场景不代表实际订阅、credits、工具费用或账户扣费。

两轮原 Runner stop 均自动精确完成，没有人工关闭兜底；独立核验终端显式关闭，服务 not_running，原 shutdown 与离线 `verify-stopped` 均确认 resources_settled:true，范围为 orca_terminal。临时日志采集许可 inactive，自建 lease 全部 revoked。完成报告后，30 分钟 heartbeat 已 PAUSED；未归档聊天，未关闭或合并交付 PR。

原始回执、原生归属、测试报告、用量分页、费用复算和停止证据保留在测试聊天工作区 `.evidence/n308-19-remote-l4/` 的 `round-9-qualification.json`、`round-10-qualification.json`、`final-report.json` 及关联文件；该目录被忽略，本文不声称原始日志已随 Git 提交。受控回归包括初始化包装的修复前 FAIL/修复后 PASS、PS5/7、停止相关回归，35 个登记 internal 包的实际成功输出及 vet exit0。过宽的完整 `go test ./...` 执行曾取消，不能记为整套通过；其自有夹具均已精确收尾。

## 2026-10-23 起的历史长任务调查

2026-10-23 长任务复跑尚未通过：固定 `n308-19` Spec+Plan 的首轮 Run `3d7eb7c6c611f46a3940a4b621698e52`，源码 `ca1a8a3` / EXE SHA256 `d6688642410e9e30ff5672896cfd9992ba81e5f348c6e5f29f160973ddb87474`，远端 Orca 1.4.223、Codex gpt-6-luna xhigh/full_access。原输入仅发送一次；代理在压缩后以 READY 结束，只有未提交的部分实现，没有 PR。压缩后的历史仍含初始化输入和业务输入，不能据此断言业务输入被丢弃；新启动提示明确初始化指令在首个回复后失效，属于待真机复验的修正，不能仅凭提示文本和单测宣称模型问题已解决。

同轮独立 `worktree.ps` 有原主代理结束及 READY，`session.tabs.list` 的 provider 仍匹配，但本轮所需 `mainAgent`/`turnStartedAt` 缺失且状态起点仍是初始化结束；Runner 正确保留 `observation_unverified`，不借屏幕或 rollout 升级为可信 final。rollout 仅作故障调查，不接入结果路径。普通 Git 查询出现的 61 个删除在单次 `git -c core.longpaths=true status` 中全部消失，真实文件未删除，不能批量恢复或带入 PR。Runner stop 首次未确认、后续原终端仍活跃；人工对精确原 handle 清理返回 ptyKilled=true，此人工收尾不计为 Runner 停止通过。详细原始证据保留在本聊天工作区 `.evidence/n308-19-remote-l4/`；连续稳定交付计数仍为 0，30 分钟 heartbeat 继续检查。历史短场景结果不能替代本次长任务资格。

按后续指示 rebase 到 `9740bed` 后复验，复用该主线的 SessionStart 原输入只读补证，不把本轮旧构建的结果归属问题当作其新实现失败。源码审查另修正远端 stop：验证并取消原观察 handle 后再开始关闭；本机 `CLIQueueError` 证明命令未调用时保存 not-attempted，实际调用未知仍禁止重复；保留实际关闭错误。新增两项 stop 回归覆盖队列未调用后的首次关闭、丢回执零重发、观察取消顺序和异源 handle 拒绝。两处修正均待 rebase 后最终构建的真实链路复验，不能以受控回归替代两轮 PR 与自动停止资格。

## P0 删除前能力核验

本轮已修复查询漏项并补验；用户明确参数只承诺按配置传递，不新增实际生效核验。结合首轮原生证据，当时 P0 按收敛契约通过，helper 删除尚未实施。源码调查和本文存在不构成通过。

| 项 | 当前状态 | 证据与结论 |
| --- | --- | --- |
| P0-01 直接启动/身份/参数传递 | 收敛契约通过 | 原生启动/身份已证，冻结参数正确编译并传递；不承诺实际生效核验；查询漏项已修复，原失败保留 |
| P0-02 原回合/短回合/同会话反馈 | 探针通过 | 两个短回合、同 provider、原请求开始、完整 final、query-only；实际 NativeProof 回放通过、旧 done 拒绝 |
| P0-03 原代理实际认证/最小发布 | 探针通过 | 实际 commit/push/PR #578，独立核对；已关闭未合并、远端测试引用已删；首次只读清理校验失败如实保留 |
| P0-04 原连接重开/活动停止 | 限范围通过 | 观察连接重开且零重发；活动 working→close→完整清单缺席，只证明原终端范围 |
| P0-05 能力差异/零 helper/清理 | 已登记 | 零部署/调用 helper，PR/远端引用/终端/工作区收尾；首轮 263 个生产文件哈希相同；本轮仅授权修改原生查询及对应测试；能力差异见两份报告 |

明确收缩项：完整远端 Check、Runner 单仓库 broker/认证撤销和完整 OS 写者树证明。它们不能登记为“现有功能等价”。新缺口若影响保留功能，先修订方案，不静默删功能、不恢复 helper。P0 探针不能替代最终 Runner P5 验收。

## 自动验收登记

| ID | 验收内容 | 状态 | 命令、构建、结果与证据 |
| --- | --- | --- | --- |
| ORN-A01 | v4 配置/直接启动、远端接线删除、零 helper/替代程序 | 通过 | v4 freeze/直接 native 启动、旧 helper 键含空值拒绝、删除与 local 保留清单；最新 #601 完整闭环和 active 的实际 Orca 日志零 helper/reader。 |
| ORN-A02 | 远端已有认证、模式隔离、错误与零凭据转发 | 通过（收敛范围） | 远端已有认证实际 commit/push/PR #601；L1/Controlled 认证/参数隔离和无权限回退；不声明 broker/登录撤销保证。 |
| ORN-A03 | 原输入/会话/回合/完整 final，截断和冲突拒绝 | 通过 | 新缺失 outcome 为 unreported；显式 unknown/失败/取消、缺开始/来源/完整正文及历史 unknown 均保留阻断；实际 #601 原生结果和业务核验成立。 |
| ORN-A04 | Orca 停止、只读重验 scope、不跨层落库及未知零重发 | 通过（Orca 范围） | 最新 active 123.06s、完整闭环 160.00s 后均 offline resources_settled=true / scope=orca_terminal；原先七个 scope/一致性子例及旧记录保护保留。 |
| ORN-A05 | 退出继续运行、换代显式恢复、原请求对账 | 通过 | 统一 Controlled 16/16 历史及最新 TestSRNativeUnknownAndStop 22.14s 增量；重启授权、未知输入零重发及历史 unknown 不晋升。 |
| ORN-A06 | 旧远端只读/精确处理资源，无 helper 兼容引擎；local 回归 | 通过 | 旧远端 Run/Check 只读，无 helper 兼容运行栈；旧 v5 stop 仅原身份；最新 adapter 全包含 local/legacy/14 个 native stop 子例通过。 |
| ORN-A07 | 远端完整 Check unsupported 零资源；普通 Run 不受阻 | 通过 | 公开 schema/capabilities 远端完整 Check unsupported、拒绝可查及 Controlled 零资源；普通 Run 已实际完成发布闭环。 |
| ORN-A08 | SteerLine 配置/scope/部署/Check 消费者 | 通过（单测/公开契约） | 最终 EXE 的 15 项 runnerclient 与 storage 公共读取测试通过，unreported 原值保存；部署仍 resources_settled，无产品部署/live 宣称。 |
| ORN-A09 | full_access 闭环与活动停止、固定构建/零 helper/清理 | 通过（workflow v2） | #601 精确 head 合入专属测试 base，PRready feedback 拒绝且 native input 3→3；active 原终端 working→stop。相同 Runner SHA，driver 身份分别登记，全部 Run/服务/设施收尾。 |

人工项：无独立必需人工门槛。原生环境缺失、skip、协议夹具或历史 helper 结果不能代替真实验收。

以下按发生顺序保留阶段记录；当前结论以上方登记表和最后“最终通过记录”为准。

## 历史与范围

调查基线 `a3dc15dc2f90787ea6e38e78d4dbc8a275470646`。新停止 scope 只证明 Orca 管理的原资源结束，不证明任意后台写者或登录撤销。旧 Run/Check 按原冻结证据结算；PR #18/#27 历史不改写。

## 文档检查

2026-10-19 首轮文档交付：Spec、Plan、验收占位及现有文档入口范围说明。检查相对链接、ORN-A01–A09 映射、Markdown 表格/围栏/换行、脚本示例语法与 diff 空白。原生验证仅复用两个现有 case，不裁剪 catalog 或新增 case。上述首轮仅为文档检查，不计功能通过；随后本轮执行的 P0 原生模型任务和证据回放单独登记在实测报告，不能替代生产实现验收。

## 去 helper 与结构负担审计（2026-10-19）

本轮为主会话文档审计与修订，不声明独立多 agent 审计或实现已通过。

| 发现 | 原稿风险 | 修订 |
| --- | --- | --- |
| P1：只绕开 helper，未要求删除 | 双执行路径和上传/reader/actor/停止签名接线会继续存在 | Spec §2、Plan §1 明确删除清单、剩余 local 调用证明及全流程零 helper；非兼容引擎 |
| P2：另加 execution_mode 且不改 codec | 相同 profile 版本承载不同停止保证，模式/版本重复表达 | 使用现有 envelope codec v4 隔离新远端；local v3 不变，旧远端 v3 不进执行；不加模式字段 |
| P2：scope 跨 backend/RunView/runstate/receipt 持久化 | 一项说明字段扩张为多层事实和授权条件 | 仅 StoppedRun 只读输出；普通停止沿 diagnostic，scope 不落库、不参与恢复/重发 |
| P2：旧远端兼容范围过宽 | 为停用的 helper 留存签名/reader/actor 运行栈 | 旧快照可查、原终端可精确处理；拒绝执行/恢复，未核实不降级，不保留兼容引擎 |
| P2：测试扩大为双模式完整 PR 加新增 stop case | 新 case/模式分支/注册维护超过删除功能本身 | 一个 full_access 闭环加现有 stop/active；sandbox 参数/失败语义由 Controlled 覆盖，未真机范围单列 |
| P2：完整远端 Check 只写 unsupported、未明确退役测试 | 可能继续维护 helper live case 或新增负向 live driver | 移除 helper 正向 live 支持/case 及对应计数，拒绝只在 Controlled 验证；不改历史结果 |
| P1：生产删除前缺真实可行性门槛 | 将静态原生分支误当作当前 Orca 足以保留功能，删除后才发现能力缺口 | 补 P0：原生启动/回合/实际认证发布/重连停止/能力差异与零 helper；未通过不得进入 P1 |

复核保留的必要约束：输入未知零重发、原身份/完整清单、完整 final/截断保护、local 证明不回退、旧资源未知不伪装成功。不以简化为由取消这些条件。实现验收全部仍待完成。

## 查询补验与参数收敛（2026-10-19）

按用户指示，参数只承诺按冻结配置传递，不新增实际生效核验。已修复原生查询前 100 条漏项，真实远端取得 273/273 条完整快照；六个协议场景覆盖目标越界、增长和不完整拒绝。统一 L1 最终通过：310 项 Go 测试、28 项扩展测试、编译及零匹配保护。首次失败和预期 skip 均保留，详见[补验报告](../archive/2026-10/acceptance/orca-remote-native-p0-query-fix-20261019.md)。P0 可行性门槛满足；helper 删除及最终 Runner 验收未执行。

## 删除实现与最终验收（2026-10-19，进行中）

生产实现限定新远端 profile v4；local v3、backend/协议/目标摘要不变。rebase 后沿最新主线既有 state v6（原调查基线为 v5），本次不另增 state codec。仅 contracts.StoppedRun 增加只读 scope，没有新增执行模式、状态表、backend/RunView/receipt scope 或凭据服务。

实际删除与保留调用：

| 接线 | 本次处理 | 剩余 local 调用者 |
| --- | --- | --- |
| freeze/agentlaunch runtime 编译 | 仅 local 分配 helper 路径/SHA/config digest；远端旧键含空值在接受前拒绝 | 本机 Codex/Pi/DSH 受管启动 |
| 远端 readiness/auth 门 | 远端准备/输入/启动不调用 helper；共享代码移为 local_authentication.go 并拒绝非 local | 本机 Orca Codex 启动、输入准备/发送 |
| remote_receipt reader/inspection | 从远端完成/重连链删除，v4 状态拒绝 helper 事实；文件改为 local_codex_history.go | 本机 Orca Codex 原生历史读取 |
| 签名退出/停止文件/认证撤销 | 远端 Stop/VerifyStopped 独立走原终端清单；共享函数在 codexReadiness 入口拒绝远端 | 本机 Orca Codex 原 writer/认证退出证明 |
| RemoteEnvironmentSession | 移除远端路由，改为 LocalTerminalEnvironmentSession，构造及 Initialize 均 local-only；新旧远端 actor 入口阻止 | 本机 Orca 完整 Check |
| scripts/remote-runtime-upload | 文件删除 | 无 |
| fixture/config/stability/audit | 删除远端 helper 配置/上传/版本/权限审计；只核对原生冻结启动参数；本机 CLI 记录器不上传远端 | 本机安装 CLI 转发日志工具；local helper 审计保留 |
| live Check 场景 | 退役 environment-check.orca-sandbox / orca-full-access，两份 registry 和框架计数同步 | 4 个本机 Check case 保留；远端拒绝由 Controlled 覆盖 |

旧 v3 远端 Run 只查快照、拒绝执行/resume；显式 stop 只提取原终端身份，PTY 缺席不补记 helper 树/认证结算。旧 Check 包括历史 environment 名称均不得进入 actor。未确认的旧资源仍阻断 resources_settled。

原始实施现场 `.evidence/orca-native-implementation-20261019T042843Z`。统一前置第二轮 `.evidence/orca-native-controlled-20261019T044433Z`：L1 312 项 Go 测试、28 项扩展、编译及零匹配保护通过；8 个显式 opt-in/helper 子进程入口按框架预期 skip，不计真机通过。首次框架失败 `.evidence/orca-native-controlled-20261019T044326Z` 因退役场景的旧数量断言，保留失败并同步修正；未启动业务测试。

整批构建后复查发现旧 Check 的 environment 别名须保持只读，已补保护及定向应用测试（47.425s 通过）；最终构建另以 `.evidence/orca-native-final-delta-20261019T045055Z` 验证 ServiceOwnership 和 EnvironmentCheckCLI，不重复整批已通过范围。后续真实用例与公共契约使用该修正后的最终源码。

Controlled 复用 TestSRServiceOwnership 增加 4 个子例：真实 SQLite/公开 EXE，缺席成功且 scope=orca_terminal，活动/截断保持未结算，旧 v3 PTY 缺席不降级；只允许 terminal list，原 DB 字节不变且 scope 不落库。原生完成反例由既有 adapter/protocol 测试验证；受控 workflow 用原有 native 协议夹具，最终 Orca 生产 workflow 由下述 P5 真实链路验证，不把本机夹具记成远端真机。

SteerLine 公开配置、schema、scope 解码和部署/Check 接入见消费者 `docs/acceptance/orca-remote-native.md`；7 项顶层测试及另 1 项 CLI flag 测试通过。未部署 SteerLine 产品环境，不将调用方单测记为产品 live 流程通过。


## 用户追加 rebase（最新验收进行中）

当前分支 codex/orca-remote-native 已从 a3dc15d rebase 到 origin/main `95ae8a72792d2b11cc59b938257165b472b5b4de`（31 个提交）。此前全部改动保存在 stash `931c8287188e73d095dadcd2d324a4b96571696a`，应用恢复后已清除冲突，未丢失未提交改动或证据。SteerLine 消费者工作区保持其原主线，不改写其历史。

rebase 前 P5：full-access/default，C:/srt/7134be9d1f50，PR #591 的原 head 0639c39de36971ea7e18b23979a200c07560e415、反馈 head 9e7a678cc469e37d247235537498ec0393cf9966，精确 head 合入专属 base，merge c7ad83eb199c1a2a5340cf8cdfa12ddb6beb7ccb；238.79s 通过。stop-revocation/active，C:/srt/9fa2dc57df08，87.70s 通过，无 PR，活动 writer lease 拒绝、stop 后 scope/清单成功且零后继输入。两者 Runner SHA CE1B2B8E40825551BC680810B64EF34777E1A44F09CD598E889222C550556A3A、driver SHA B2421F242BD88BE47F8B975B79ECD5FF1EA16B85EA603D332000970F13B4D6F7；仅证明旧基线改动树，不冒充 rebase 后结果。

最新实施原始证据目录 `.evidence/orca-native-rebase`；最新统一入口路径由 `.evidence/orca-native-rebased-controlled-path.txt` 固定。保留上游 workflow v2/result acquisition、独立配置及 local helper 控制/认证能力；远端不创建固定 control、不读 helper readiness/历史/签名/认证收尾。旧远端只解析原终端 Process，避免把已退役的 reader 字段重新载入运行栈。

整合中修复并保存：上游新增 SignedNativeExitEvidence 对 orcatransport 的生产导入与同包 native 探针形成测试循环；改为相同 wire 形状的两字段匿名 closure 载体，不增加共享框架或协议。主线两处框架的旧 29-case 数量同步为退役后的 27，保留首次框架失败。消费者 workflow 的 unknown-version 反例由 2 更新为 3，当前公开 v2 不能误记为未知版本。以上均以最终重验结果为准。


Rebase 后完整 L1 已通过：390 项 Go、29 项扩展、原生 driver 编译及零匹配保护，原始统一目录 `.evidence/orca-native-rebased-controlled-20261019T051704Z`。8 个 opt-in/helper 子进程入口 skip 只表示未执行，不作为真机证明；原生生命周期探针更新为 v4，helper 正向停止探针限定 local，不再能向远端启动 helper。

末轮离线一致性补验 `.evidence/orca-native-rebased-scope-20261019T052854Z`：ServiceOwnership / EnvironmentCheckCLI 两组通过，新增 remote_offline_scope 7 子例全部通过（缺席、活动、截断、旧 v3、合法未创建、未创建却混入 control、非法 v4 profile）。后两种矛盾快照零 Orca/模型调用且不接受 stopped 标签；合法未创建不误阻断。scope 推导返回一致性错误供只读验证处理，不进入授权、持久化或 backend 事实。

最终 EXE version 0.2.0-dev.2，SHA256 `A1BF1A2C46E887532BB11FB627452C99DFEF44C0A58A0AC02CF24F0641063EA4`，resource digest `c7b197e382299922599ae31faf1f69f54db447dee949035a8181332780ac411f`。完整 L1/Controlled 初始构建为 42F759...，最终仅离线快照校验补验不同；实际 P5 与消费者末轮使用 A1BF...，不会混记为旧 CE1B... 构建。


Rebase 后首次 P5 `C:/srt/1853355a3956`（A1BF...）失败：原生首次 send 只有 input_accepted，未等待的开始回执使 Runner 保留 input_accepted_waiting_start；原终端已真实完成，但没有用屏幕结果越过开始门。只读重放原 request `881a666a-f62e-866c-791c-42fc68334a46` 得到 mutation.replayed=true 和 turn_started，确认是回执观察时序，未重发输入。公开 stop 撤销原 Run 后，服务退出、offline-stopped.json resources_settled=true / scope=orca_terminal；失败结果保留。

最小补正仅对新远端 v4 首次业务 send 加 Orca 既有 --wait-submit 20；原请求 ID/文本/身份、50 秒调用预算、Started/完整 final 门保持。没有增加重试、观察服务或持久事实。adapter 全包通过 25.272s，协议/调用审计包分别通过 6.644s/2.664s；新增 TestNativeRemoteSubmissionWaitsForOriginalStart 覆盖同一请求有界等待、accepted 未开始及观察零重发。当前最终 EXE SHA256 EB8AF1DEB2811BB951DE35231FC2B06E21C61C1AB3AE1EB74E339479EC9B96BF，resource digest 不变。消费端三项公开契约复验通过（consumer-start-wait.log）。新 P5 full-access 现场 C:/srt/09c40b6b369f；活动停止现场 C:/srt/54121a91fd31。未执行的旧构建准备现场 C:/srt/4389e6dbb088 不算 case 通过。


### 历史：主线新增的能力冲突（用户随后已确定契约）

`aaf5480db714f7c9315488b318636632631ee324`（2026-10-18，Keep explicit unknown native outcomes in recovery）将 reliable.NormalizeResult 的成功条件改为 evidence.Succeeded && outcome == succeeded，显式 unknown 仍需恢复。新远端原生查询只有 mainAgent done、原 prompt/provider/pane 和完整 final，mainAgent 不提供 outcome；NativeProof 按原 spec 保留 unknown，而非伪造 succeeded。因此 P5 `C:/srt/09c40b6b369f` 已收到原请求 turn_started 和核实的完整 final，却停在 original_session_recovery。退出码 1、120.92s；无 PR。原终端真实提交 9a6f3c8，不能用模型声称的 checker 成功越过新主线路由。driver 已清理原 Run/服务，offline resources_settled=true / orca_terminal。

这是 rebase 引入的功能契约冲突，不能把历史 P0/P5 成功、当前 adapter 结果或完整 final 记为当前完整 workflow 通过。当前保持主线严格规则，未修改 workflow/reliable，也未恢复 helper。已向用户请求决定：保留严格规则并将完整闭环登记为 Orca 能力阻塞，或先修订 spec 区分原生接口未提供 outcome 与显式 unknown，再最小实现并重验。未获决定前不继续创建完整闭环 Run/PR。


### 原生开始修正构建的活动停止

C:/srt/54121a91fd31，Run f35c318008f56127c49fbd14d0d96145，EB8AF1...，136.60s 通过（退出 0、非 skip）。checker gate 下原代理 working，活动重验明确 directory_conflict；显式原终端 stop 后完整 inventory/readonly scope 成功、零后继输入、无 PR、参数仅核对冻结请求值。native_stopped、service_stopped、facilities_restored 全 true；driver_native_cleanup=false 表示没有越过 Runner 执行替代业务终端清理。结果和实际 CLI 调用日志已归档。

持久归档：`docs/acceptance/evidence/orca-remote-native-implementation-20261019/`，含完整 Controlled 摘要、最终离线补验、三次 P5 的成功/失败及停止证明、实际 native-calls、两仓源码逐文件 SHA 与 EXE 身份。原始短 state-dir 和测试 worktree 保留作证据；原模型终端和 Runner 服务已停止，不承诺清除任意远端后台进程。未改安装目录、共享 Orca/登录、原故障 Run，也未卸载历史 helper 文件。


归档中的 source-identity.helper_sha256 指本机 `C:/srt/54121a91fd31/bin/stability.exe` 测试驱动；runtime_sha256 指冻结的本机测试 runtime 文件集。它们不代表向远端部署 SteerRunner helper，实际 native-calls 和 manifest 共同确定执行位置；远端只有既有 native/codex.exe 及 Orca 原生终端。最终 native driver SHA 107FC1C8086827FDCA1502F05DADF065500F5107B359D9291719D5BF0D1FB40C，两项最终 P5 使用同一 driver/Runner。


末轮 legacy codec 复核：上游 state v6 校验曾阻止旧远端 v5 的显式 stop。仅 profile v3 remote 放行 v5/v6 envelope，并只提取已绑定 Process；local 和新远端 v4 不接受 v5，其他旧版本仍拒绝。既有 TestTerminalStoppedNativeRemoteScope 扩为 14 子例，覆盖旧 v5 精确 close 一次但仍不确认 helper 树结算、旧 v4 拒绝、新 native v5 拒绝。全 adapter 包 22.477s 通过；消费者三项实际 EXE 契约再次通过。此次不改结果判据，也不改新 native 停止逻辑。

**当前源码构建**：`.evidence/orca-native-rebase/steerrunner-legacy-state.exe`，SHA256 `87DBF4FC2622171C9E7C2F4E670665A7E331EF6D8F33938090CE65985CCBDAF2`，版本 0.2.0-dev.2 / resource c7b197e382299922599ae31faf1f69f54db447dee949035a8181332780ac411f。当前构建只比 P5 的 EB8A 增加上述 legacy-only codec 处理；P5 原构建及源码保留 runner-start-wait-source.json，不把 P5 记作 87DB 构建执行。因原生 outcome 契约待决，整体仍未交付，未再次运行完整 P5。


补充 Controlled `.evidence/orca-native-rebased-submit-20261019T055005Z` 已结束：TestSRResultAcquisition 474.54s（8 个子例全部通过，包含 parser 重启/工具边界），TestSRNativeUnknownAndStop 21.20s；scenario_count=2、退出 0，构建为 EB8A。同一新主线结果契约回归通过，不代表缺 outcome 的真实 Orca 闭环通过。所有已启动验收均已结束；当前只等待 outcome 契约取舍，未有悬挂验收进程。


## 用户确认的 unreported 修正与重新验收（2026-10-19）

用户明确授权第二种最小方案：新核实的普通原生结束缺 outcome 记 unreported，保留显式 unknown、失败、取消和未知输入阻断，历史记录不自动放行，继续零远端 helper。仅 native_state 的首次结束赋值及 reliable.NormalizeResult 准入有行为变化；未修改 workflow 路由、调度、审计/PR、scope 或部署门，也未增加字段、模式、迁移或上层容错开关。已保存 unknown 不重新归类；原服务重启/恢复授权保持。

当前构建 `.evidence/orca-native-unreported-20261019/steerrunner.exe`，version 0.2.0-dev.2，SHA 29D44DF0315190FDD533E7AB091C9450ABF868E766F0A46985DE2045DEBFEF3A。沿主线 95ae8a7 的 dirty 工作树，profile v4 / state v6、CLI v1 和 resource digest 不变。

本轮验证：adapter 全包 22.507s、reliable/workflow/contracts 全包通过；reliable 最后补验 0.903s。应用 TestRAUnreportedOutcomeBoundary 五子例全部通过（25.177s）：unreported 进入既有审计；unknown/failed/cancelled 无后继；业务 unknown_external 继续 reconciling。第一次应用断言错误地要求排入下一阶段后 r.Result 仍存在，已按既有 queueInput 清理规则改为核对不可变原 StageResult 和公开 Final；生产路由未作补丁，首次失败 application.log 保留。既有审计/反馈用例在首次组运行通过。

消费者 runnerclient 15 项全量顶层测试通过（含实际最终 EXE published contract），storage 的 TestRunnerPublicReadsKeepOriginalFinalAndCallerBinding 通过；unreported/unknown 原样往返，存储不可篡改及原 Run/Attempt 围栏保留。Controlled 增量 TestSRNativeUnknownAndStop 22.14s 通过（退出0、scenario_count=1），最终构建与上述 SHA 一致。此前完整 Controlled 16/16 及 scope 7 子例仍保留，不重跑未变化组。

真实重验现场：full-access/default C:/srt/005b2b1d831f；stop-revocation/active C:/srt/61e65ec23f4c。原始新证据目录 `.evidence/orca-native-unreported-20261019`，结果未完成前不将本轮 A09 记为通过。之前失败或历史结果均不覆盖、不会自动放行。


### workflow v2 反馈验收口径纠正

#600（005b2b1d831f）178.21s、退出0，通过当前实施→发布→PR核验→精确合并→stop；实际只含启动和两个业务输入，未进行正向反馈或新 head。原 head 33b419cd6c082218ad3a4e8c5a9e420c5d5d4cec，merge 9fac2e2136423b4c9cc09d4e0bf49717afe51516，只合入专属 codex/srtest/005b2b1d831f/base。此前进度描述把旧基线反馈能力带入本轮，已纠正。

最新主线 workflow/2 明确拒绝 pr_ready 反馈，driver 也已删除旧正向更新路径；本次不改该产品规则。Spec/Plan 同步为里程碑拒绝且零新输入。复用现有 full-access/default 加入公开 reply invalid_state、revision/PR/head/turn 不变、实际 native input 数不变及外层结果检查，重新生成独立现场补验；不覆盖 #600 的原始结果或冒充其已覆盖此项。最终 Runner EXE 不变，仅 driver/断言增量。


## unreported 首轮通过记录（最终审计前）

当前源码为 origin/main 95ae8a72792d2b11cc59b938257165b472b5b4de 上的未提交改动；本轮没有再次改写主线、提交或部署安装环境。最终 Runner SHA256 29D44DF0315190FDD533E7AB091C9450ABF868E766F0A46985DE2045DEBFEF3A，version 0.2.0-dev.2，resource digest c7b197e382299922599ae31faf1f69f54db447dee949035a8181332780ac411f。新原生结果 unreported 仍如实对外；CLI v1、profile v4、state v6 结构不变。

| 现有 case / 现场 | 实际结果 | 身份与业务事实 | 收尾 |
| --- | --- | --- | --- |
| full-access/default，C:/srt/72376a133a73 | 160.00s，退出0，非 skip | Run f145884cb4f4bc9e6f5cbea14ade73b4；原生 final=unreported 且 verified；PR #601，head 0e1bd296388f29457cd46a294d9657d9cc8d07ba，merge ee7fa43fb68b7bb10a40377edeaa48c25cd97be3，只合入 codex/srtest/72376a133a73/base | Run/服务 stopped；offline scope=orca_terminal、resources_settled=true |
| stop-revocation/active，C:/srt/61e65ec23f4c | 123.06s，退出0，非 skip | Run c83c82feec49d5b6f6c75b0596363bcf；实际 working gate、活动 verifier 拒绝、显式原终端 stop、零后继输入、无 PR | native_stopped/service_stopped/facilities_restored=true；offline scope/settled 成立 |

full-access driver SHA F01737BEC507FC0FCC4EDEC3DC04D8F57FFD92F9E3B7750D7DFEC4073C4BC496；active driver SHA 6206E706C7C925F64E51A081A1271AD21CCF20EF5658230279117B849FFB53D9。差异仅 full-access 后补的反馈拒绝断言；两项 Runner EXE 完全相同。#600 178.21s 的首次正常路径通过保留，不冒充其包含反馈边界。

#601 在合并前经公开 reply 验证 invalid_state，revision/PR head/turn 不变，native input 数 3→3（启动加两个业务输入）。当前主线 workflow v2 不接受 pr_ready 后正向反馈，不宣称产生反馈新 head。最终业务 PR 状态和精确 head/base/merge 另经 gh 查询核对，合入内容由实际固定 checker 再次验证。独立只读 proof 文件逐项断言 native unreported、业务 pr_ready、拒绝边界、一个原业务资源、零 helper inspection、精确 merge 与 scoped offline 收尾。

Orca runtime f8616747-34af-4543-8487-ec6b5d8de4ff、appVersion 1.4.223，本轮实际 status/call 元数据确认；Native Codex 使用同一已配置 C:/sr-runtime/2ba0/01/native/codex.exe，P0 观测版本 0.159.2，本轮不另作版本探针或参数实际生效审计。模型/effort/full_access 只按冻结配置传递。

当前最终证据归档 `docs/acceptance/evidence/orca-remote-native-unreported-20261019/`，含结果/日志、实际 Orca 调用、公开结果、manifest/driver/EXE、PR/provider、scope/cleanup、两仓源码逐文件 SHA 和文件索引；较早的失败及证据目录均保留。历史 unknown 未修改、未自动恢复；原故障目录/Run、C:/SteerlineInstall、C:/SteerlineRuntime、共享登录和已有 helper 安装均未操作。短 state-dir 与测试 worktree 文件保留供复核，所有本轮业务终端及 Runner 服务均已停止；停止保证仅 Orca 原资源范围。

## 最终审计与提交前补验

审计确认并修复一处 P1：历史 StageResult 的 `Succeeded=true` 可能与显式 unknown 并存。RouteParsed 重建结果丢掉原 Outcome，报告来源及发布前置条件也只检查粗粒度成功值，存在被新成功报告/parser 晋升的入口。现将原 Outcome 保留到路由，并在原结果、报告来源和 trustedDone 复用一个私有准入判定；unknown、failed、cancelled 及未知值始终阻断。succeeded/unreported 正例保留，空历史值沿既有解释，不补写 unreported；不变更业务边、恢复授权、schema 或历史记录。

`TestRAHistoricalOutcomeCannotBePromoted` 在现有 workflow 测试中覆盖序列化恢复、直接路由、parser、成功报告和历史 satisfied 发布前置条件。首次新增测试的报告正例因夹具 final 与原 digest 不一致失败，修正夹具后全包通过；生产校验没有放松。另删除 local-only 环境检查器的一条死分支，并格式化消费者两个 Go 文件，未新建兼容引擎、模式、迁移或测试框架。

远端 helper 接线复核：v4 冻结拒绝 helper 键、状态拒绝 helper 证据；control、认证准备、历史 reader 和环境 actor 仅 local 可达；远端上传入口已删除。旧远端 Run 的加载/调度/恢复/PR 刷新被隔离，Check 不重启 actor；显式 stop 仅提取原 Process，不能把 PTY 缺席变成旧 helper 树/认证结算。残留 Remote 类型名和诊断属于共用的本机 helper wire contract 或历史证据，未为清除命名再引入协议改动。

最终构建 SHA256 `819D2079231B8FDD33F96F1264B3279E40896B1FACD3EEE8B761417F85D0D639`，仍为 0.2.0-dev.2、resource c7b197e382299922599ae31faf1f69f54db447dee949035a8181332780ac411f。基线 95ae8a7 上的实际源码逐文件 SHA 与 EXE/driver 身份另存本轮归档，不把上一轮 29D4 构建的验收冒充本轮执行。

| 本轮补验 | 实际结果 |
| --- | --- |
| workflow/reliable/orcatransport/protocol-orca/contracts/resources/livetest 全包 | 全通过；workflow 1.396s、adapter 23.673s；新历史反例和正例通过 |
| 应用层 `TestRA\|TestSRFreezeOrcaAgent\|TestEnvironmentCheckLegacyRemote` | 25 顶层测试、零 skip，493.415s，通过；首次 180s 测试预算不足的日志保留，20m 预算重跑退出0 |
| SteerLine runnerclient/storage 定向 | 24 顶层测试、零 skip，分别 8.352s/6.475s；实际最终 EXE 契约、原 native outcome 往返与不可变存储通过 |
| Controlled ResultAcquisition / NativeUnknownAndStop | scenario_count=2、退出0；8 个结果获取子例 679.16s、未知输入/停止 30.55s，全通过 |
| 真实 full-access/default，C:/srt/c8446c20ecbc | 296.63s、退出0、非 skip；Run 3c871871de142d9b0684a5bab9d8e4d3；实施→原会话报告→发布→核验→测试 base 合入→停止通过 |

本轮真实测试 PR [#610](https://github.com/erranliu/SteerlineTest/pull/610)，head `03da1ac74ee4eaed28a827a9f95a11c7c6872234`，merge `d5596d4e4f713d53fb1f42f6b51ff41dfe026838`，仅合入 `codex/srtest/c8446c20ecbc/base`。最终原文 unreported/verified 与业务 pr_ready 分开核验；本次确实走到 implement.report，并未由报告覆盖原 outcome。pr_ready feedback invalid_state、原 revision/head 不变、native inputs 4→4；一个原业务资源、零 helper inspection/control。Run/服务均 stopped，offline resources_settled=true / scope=orca_terminal。driver SHA `4D061C5DCA14BF4B1C9C0DA5B2EA81951DF2226460091BF224CC53E7D39FE4A2`。

活动停止沿用审计前 29D4 构建的 active 123.06s 真实通过记录，本次停止实现未改；819D 构建另通过完整闭环停止和 Controlled unknown/stop。原生参数仍仅按配置传递，未新增实际生效核验；不声明 sandbox 真机闭环或全进程树/凭据撤销保证。

最终归档 `docs/acceptance/evidence/orca-remote-native-final-audit-20261019/`，保存首次超时/完成重跑、Controlled、消费者、真实调用及 provider/离线证明、两仓源码和逐文件索引。准备阶段只保存命令及原文件 SHA，避免收入无关工作区清单；提交前敏感信息扫描无命中。消费者分支从 origin/main d12779733a21cfe4397baf1d51e73881a1ba15ee 建立；其原本机 main 的额外合并历史文件树与远端相同，未带入无关提交。未部署产品或远端 helper，未改原故障记录和安装环境。


## 最后消息与原生归属增量（2026-10-23，未部署生产）

本次增量的实现、自审及共享契约结果统一登记在[执行可见性验收](execution-observability.md#orca-最后消息实施与自审2026-10-23)，原始证据唯一归档到 [orca-last-message-20261023](evidence/orca-last-message-20261023/)；不新增同义账本。仍只扩展远端原生 Codex，未新增远端 helper、完整环境 Check、远端 Pi/DSH 或参数实际生效保证。原生产 Run 的泛化冲突、unknown/outcome 和冻结输入均未重写。

主线整合前的真机冻结构建为 0.2.0-dev.3-r2，SHA256 `61C8B89110DAB644D03B5AE74A77304CF98FCA0859709D92CA9AFBB7D9B38DE4`。原生定向通知及 query-only 各两个真实回合通过，均为独立 SteerlineTest fixture，工作区和测试终端已清理；本次实际结束 prompt 保留。真实空 prompt 和 working 期间字段发布仍不具备现场资格，不能用模拟或结算后的消息替代。正式 L1 与22项 Controlled 已通过；四个远端 case 最新现场均通过，详见同一可见性记录的最终结果。首次 smoke 的停止 unknown 失败保留，经原身份收尾后在新 fixture 串行复验通过；首批 batch 仍记录 failed，未覆盖。


最终资格现场：smoke `08cb8fae5e48` 200.81s / PR #650；活动断线 `b2b0990dc19e` 548.42s / #646；发送前断线 `d3e7db32713f` 451.98s / #647；multirun `bbc3990ab2a0` 197.17s / #648、#649。四项均使用上述冻结 EXE；双通道故障真实命中、原服务与输入授权保持、无原输入重发，并确认原资源/服务及网络设施收尾。multirun 的两个 PR 在隔离验证后关闭，其它 PR 仅合入对应测试 base。首次失败 #645 的资源已另行原身份对账并离线结算，不计首次通过。

[资格索引](evidence/orca-last-message-20261023/qualification.json)、原始结果和 provider 最终状态只保存于该唯一归档。用户启动的管理员代理已正常结束；测试 state-dir/业务工作区保留复核，生产安装、原业务记录与共享登录未操作。实际空 prompt 及原生 working 期间字段发布的现场缺口保持明确，不把受控或匹配 prompt 的成功冒充这些资格。

后续 [PR #50](https://github.com/erranliu/SteerRunner/pull/50) 已整合主线 #49，复用 state v7 与唯一 v6 解码边界；远端空预览继续保持专项 Spec 的连续观察要求，本机沿主线输入绑定行为。整合构建 `3C1F9531…2AA338` 的 L1 609 项顶层 / 2111 项含子测试、29 项扩展及 Controlled 22/22 通过；其身份和完整结果见[整合资格索引](evidence/orca-last-message-20261023/pr-integration/qualification.json)。该构建未重跑远端真机资格，上述 `61C8B891…B38DE4` 结果仍属于原冻结构建；未部署生产。
