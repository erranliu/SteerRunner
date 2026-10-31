# Plan：Orca 本机与远程连接统一

> 2026-10-31 适用范围更新：认证与存储版本以 [host-native-auth Spec](host-native-auth.spec.md) / [Plan](host-native-auth.plan.md) 为准。新建开发执行使用宿主 Git/gh/SSH 登录；broker、CA、socket、认证租期/撤销证明已移除。仅支持全新 state/v6，不读取或迁移旧 Run；旧版执行由原版本结算。本文的 read_only、原生远端、身份绑定和未知操作对账条款继续适用，旧认证与旧格式兼容条款仅供历史溯源。

2026-10-31 当前开发权限修订：开发 sandbox 已退役，开发必须显式 full_access；read_only 与原认证链路保留。开发权限、迁移、旧 Run 与重复测试矩阵以 [统一权限 Spec](development-full-access.spec.md) / [Plan](development-full-access.plan.md) 为准。下文历史构建与验收事实不改写。

日期：2026-10-18。状态：查询模式已实施，三项结构审计发现已修复，并整合 `origin/main` 的发送前连接恢复。最终构建验证与历史四项 L3 分开登记；远程 reconnect 前置阻断，V04 新子变体未验，尚未完成全部验收。依据 [Spec](orca-local-remote.spec.md)，调查基线 `9f9eca0`，整合主线 `0e34595`。实际验证见[专属验收](../acceptance/orca-local-remote.md)。

## 1. 审计结论与删除项

| 原设计问题 | 结构/验证成本 | 修订 |
| --- | --- | --- |
| 为连接类型新增 `LifecycleBindingPreparationPort` 和中心提交阶段 | Orca 传输细节侵入通用执行中心，形成长期可选端口和排序负担 | 删除；复用已有 workspace operation receipt、ProcessIdentity 和对账 |
| 新 profile v4、binding envelope、target hash 和旧版兼容视图 | 为新持久化制造新迁移；connection_kind 重复现有 environment 语义 | 删除；保持 Profile v3、State v5、target ID 和摘要规则 |
| environment 规范 ID、user-data 持久冻结、多目标 manifest/config v3 | 把环境治理和实例管理混入本机接入 | 删除；沿用现有来源和选择器，局部校验一致性/歧义 |
| 复制本机 11 项可靠性 case，追加路由/混合 driver | 共享生命周期逻辑被重复验证，产生额外 PR、结果分支和维护负担 | 收敛为 3 个新增 L3；路由/恢复/隔离负例进入现有 L1/Controlled |
| IPC relay/OS 重定向作为前置，加 99 执行和强制并行资格 | 测试设施复杂度超过功能本身，且只为复制的矩阵服务 | 删除；真实无模型 IPC 探针 + 5 case 一轮定向真机验证 |
| 要求本机与远程都支持推送订阅 | 现有 Orca 本机只提供 unary IPC；要求改 Orca 才能推进，且比既有完成判定所需能力更强 | 本机查询式 StateFeed、远程订阅加补查；复用 30 秒补查与既有 NativeProof，不新增查询平台或高频采样线程 |

删除项不保留为“后续必补”的隐性债务。保留原生身份校验、认证、取消/资源回收、原生取证、未知零重发、显式恢复和停止撤销，因为它们属于现有正确性约束。

## 2. 改动范围

| 位置 | 必要改动 |
| --- | --- |
| `protocol/orca/state_stream.go / state_config.go` | 一个包内连接接口/打开函数；保留远程 WS；新增本机 metadata、认证与 IPC；删除空 environment 时读 pair 的分支 |
| `protocol/orca/state_subscription.go / artifact.go` | 使用抽象连接，停止直接访问 WS `.conn`；查询、订阅、上传各自有界 |
| `protocol/orca/state_feed.go / state_query_feed.go` | environment 为空时打开查询式 feed；远程保持原订阅分支；保留既有解码与 StateFeed，查询式 feed 不产生通知事件 |
| `orcatransport/lifecycle.go / workspace.go` | 移除本机配对前置；CLI 路由一致；复用现有 runtime 字段补本机创建响应校验 |
| `tests/native`、`internal/testkit/livetest`、fixture/scripts/catalog | 现有 driver 支持本机 Orca；只注册 Spec 的 3 case，调整必要的 manifest/配置校验与准备参数 |

生产代码修改白名单只有 `internal/execution/adapters/orcatransport/` 与 `internal/execution/adapters/protocol/orca/`。保持 `contracts`、全部 `application`、`backend / reliable / workflow`、storage、Codex 和 agentlaunch 的生产代码及接口不变；上层已有目标字段足够使用。不新增 production transport registry、凭据 provider、持久绑定层、routing driver 或通用 RPC 审计平台。测试/现场脚本/文档单独列入验证改动，不借测试接线扩张生产范围。

## 3. P0：确认本机协议

1. 固定实际 Orca/CLI/Codex/helper 版本和摘要，读取随版本的原生协议说明或实现，核对 named pipe 发现、认证、分帧、请求/runtime envelope、能力协商和取消；不猜字段。
2. 在隔离调用环境清空 pair 与隐式 environment，验证真实 status、tabs 查询及 worktree.ps。历史 local pipe 样本回放验证字段兼容，不冒充当前原生/业务通过。
3. 复用已有 native 入口或 fixture 准备逻辑，使用本轮辅助资源验证初始查询和资源变化后的查询；独立连接关闭重开均绑定同一 runtime。
4. 在该工作区通过原生文件端口写入小型 nonce，独立读取核对摘要，确认执行 host 参数语义，关闭/清理本轮资源。不上传凭据或 runtime 安装包。

产出脱敏协议/探针证据；本机缺订阅是已确认协议差异，按查询模式消化，不修改 Orca。若缺原请求开始回执、新主代理 done 或原生 final，则仍阻断。无需实现 IPC 故障代理、注入 DLL、进程重定向或停止共享 runtime。

## 4. P1：实现最小连接分支

依赖 P0。

1. 在协议包增加私有连接接口/打开函数。发送、按请求读响应和 Close 足够；继续由现有 `StateSubscription` 承载订阅对象，不增加第二层订阅/凭据抽象。
2. 先把原 `stateSocket` 适配到此接口，保留其加密、认证、错误及超时行为，再实现 Windows IPC。复用 `go-winio`；平台文件仅隔离必要的 named pipe 细节。
3. 按 environment 是否为空选连接；本机只用原生本机发现/认证，远程只用明确 environment。user-data 仍来自现有服务环境，同一调用链不混用来源；认证对象不逃逸到上层。
4. 改订阅、查询、上传调用点，保留现有方法/输入/输出语义和独立连接。共用真正相同的请求/runtime 校验，不为统一结构改写远程不相关逻辑。
   本机 `OpenStateFeed` 通过原 tabs 查询绑定初始 pane；其 Query 复用 `QueryOriginalAgent`，Drain 不生成事件，Close 取消在途查询。保留 lifecycle 的 30 秒补查节奏，不另起轮询线程/定时器，不伪造 working；远程订阅分支保持原样。
5. 复用现有 L1 测试并增加真实临时 pipe 协议夹具，覆盖 Spec V01–V03。远程夹具用明确 environment，不继续用空 environment + pair 冒充本机。

退出条件：双连接协议、状态解码和上传测试通过；本机无订阅依赖且 Close 中断查询，远程有界取消、关闭、队列和订阅/查询无争读；无认证材料泄漏。此阶段不宣布真机通过。

## 5. P2：adapter 接线及现有回执校验

依赖 P1。

1. 删除 lifecycle 的本机 pairedOnce 与重新继承 pair 的分支。CLI 对两目标屏蔽三个隐式路由变量；远程仍显式传原 environment。本机 CLI 与直接 RPC 使用相同 user-data 环境。相同的少量路由构建如需去重，只放 adapter 内小函数。
2. 本机 preflight 对照本机 metadata、CLI status 与直接查询的 runtime。创建资源时复用现有 `workspaceCreation.ResolvedRuntimeID`，扩展当前仅远程执行的发现/创建响应核验到本机；保留原 creation checkpoint、operation intent 和未知结果对账。后续继续使用原 ProcessIdentity，不新增绑定提交步骤。
   preflight 验证 `worktree.ps` 与源项目 tabs 查询，不以本机订阅支持为门槛；原生查询失败仍在业务资源创建前拒绝。
3. 本机 native 端口从已保存原身份查询，远程保留订阅加补查；错 runtime/终端换代即拒绝后续输入。旧本机 profile 实际绑定远程时不迁移；profile/state/target hash 不改，使用现有记录夹具验证。
5. 扩展现有 Controlled 测试中的协议来源参数/子变体，覆盖同服务两目标隔离、有效 pair 干扰、本机/远程失败无回退、一次断线重接、服务重启显式继续、stop 迟到事实及输入结果未知零重发。复用原 testkit、公开 CLI、真实服务/SQLite/隔离 Git 和审计，不新增模型 driver。

退出条件：完整 L1/Controlled 通过，V04 新子变体另行签收，不能以既有 16 项回归替代。改动范围审查确认无新中心阶段、持久化版本和旧格式迁移；保持既有 Codex 路径。发现原框架能直接复用的行为不重写。

## 6. P3：只扩展必要的测试现场

依赖 P2。

1. 在 catalog/cases 及测试清单保留 `orca-local.smoke/default`。复用现有 development driver/outcome，不新建完整本机 execution-stability 矩阵。
2. manifest 继续使用现有 v1；v2 远程稳定性不变。本机 `local_runtime_id` / `local_repository_path` 在准备成功后写入最终 manifest，受 manifest 摘要保护并与原生回执交叉校验；preparation 仅作证据，不参与运行时目标选择，不把 `RemoteRuntimeID` 改为双重含义，不引入混合目标 manifest。
3. 现场生成器增加 `orca-local` 分支，复用 `codex_executable / model / reasoning_effort`；增加必要的可选 `orca_local_runtime_executable`，多个同名注册项目时用测试配置 `orca_local_repository_path` 精确选择，更新白名单/example，沿用原 schema。后者来自真机发现的两份同名注册仓库，不改变生产配置。远程 `remote_*` 保持原义。本机单目标无需 remote environment。
4. `prepare-native-orca.ps1` 接受明确本机/远程目标，只有远程必需且传 environment。本机对真实注册项目准备专属 base，保存 runtime，确认原准备终端关闭；不复制认证文件。
5. `manifest.go`、case 集校验和 PowerShell 校验允许本机 Orca L3，旧远程和主线 12 项可靠性集合完整保留。整合主线后总 L3 为 17 场景/24 case，不只修改数字而漏查 ID 集。
6. 结果复用既有证据文件，增加必要的本机通路、pair absence 和原生事实证明即可；缺证据/错 runtime/缺清理不能通过。无模型探针可为本机 smoke 准备前置，不另计 L3 case。Close/reopen 证据明确标注仅连接级验证。

退出条件：注册展开、现有框架及新增错误目标/缺证据反例通过；本机 fixture 不再隐含要求远程配置；原生查询/上传探针真实可用。

## 7. P4：5 case 定向真机验收

依赖 P3。以下为本次精确 5 case 的串行入口；主线重连场景另有 `pre-send-disconnect`，整场景选择会多跑一项，因此用现有 fixture 的 `-Case` 参数限定范围。新 ID 已注册；只有原生能力及前置满足后才运行对应模型 case。命令在仓库根目录 PowerShell 7 执行，v2 配置同时保留本机/远程字段并指向存活的管理员故障宿主，不含凭据。完整入口会先跑一次 L1/Controlled，无需人为再分层重复运行。

```powershell
$ErrorActionPreference = 'Stop'
$config = '.evidence/live-orca-stability.json'
.\scripts\test.ps1 -Level Controlled -OutDir .evidence/orca-local-remote-prerequisites
$cases = @(
    @{scenario='orca-local.smoke'; case='default'; timeout=30},
    @{scenario='orca-remote.smoke'; case='default'; timeout=30},
    @{scenario='orca-remote.connection-reconnect'; case='active-disconnect'; timeout=30}
)
foreach ($item in $cases) {
    $manifest = & .\scripts\new-test-fixture.ps1 -Scenario $item.scenario -Case $item.case -Config $config | Select-Object -Last 1
    & .\scripts\test-native.ps1 -Manifest $manifest -TimeoutMinutes $item.timeout
}
```

管理员前置未满足时，不应让该阻断隐藏其他 case 的实际结果。本轮已先用 `scripts/test.ps1 -Level Controlled -OutDir .evidence/olr/query-verified` 完整执行一次 L1/Controlled，再用现有 `new-test-fixture.ps1` 和 `test-native.ps1 -Manifest <本 case 的 manifest.json> -TimeoutMinutes <既有期限>` 串行执行四个可运行 case。每个现场核对相同 Runner/driver 和生产源码摘要，并引用该完整前置结果；直接 native 入口本身不会自动补做前置。第五项单列管理员预检阻断，整批不能报五项全过；管理员宿主就绪后仍须补跑原重连 case。

1. 本机新增 3 case、远程既有 2 case 各一次，使用同一受测源码及构建。配置中的本机/远程工具、模型、权限按各目标冻结，不拼接不同代码版本的结果。
2. 本机 smoke 证明 pair 缺失、真实 IPC 查询、发送前基线、原请求开始回执、新主代理 done/原生 final 和完整开发/一次反馈/合入/停止。
3. 远程重连复用已存在的故障设施、事实屏障和动作计数，仍要求原双通路真实中断及原资源零重发。不新增本机故障注入设施，也不声称验证了本机全链路不可达。
4. 默认串行，不设强制 MaxParallel=2 或三轮门槛。新 case 的现场/资源仍须隔离；同服务双目标并发由 Controlled 覆盖，本次不报告真机并行资格。
5. 前置失败不启动模型，case 失败保留原结果；修复后新现场重跑受影响项。只有源码修改波及另一类连接或框架时才扩大回归，不自动恢复 99 执行计划。

L2 Codex 和无关 L3/L4 不因本次限定在 Orca adapter 的改动而成为新增真机门槛；不修改共享执行逻辑。既有全套稳定性资格规则不改，本次仅声明定向结果。

## 8. 检查与交付

实施时定向验证实际新增/修改的测试，避免未命中测试名也报通过：

```powershell
go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport ./internal/testkit/livetest -count=1
.\scripts\test-framework.ps1 -Evidence .evidence/orca-local-remote-framework-new
git diff --check
```

完整 L1/Controlled 由 P2 及 native 入口的既有前置执行；同构建前置结果按现有框架消费，不额外人为重复多轮。原生探针入口、参数和命中数在实施时固定，明确 live opt-in，不把默认 skip 计为通过。最终按 `git diff --name-only` 审查生产修改白名单，并检查公开接口、profile/state/target hash 无变化；测试和文档变更另列。

更新 README、testing、test-catalog、配置示例中与本机 Orca 直接相关的说明。新增 `docs/acceptance/orca-local-remote.md`，按 OLR-A01–A05 登记实际协议、源码/构建/工具摘要、各层命令/结果、5 个真机 case 原始记录、失败历史、资源收尾及未验边界。历史验收不重写，新文档存在不表示生产已支持。

文档检查包括相对链接、范围一致性、3+2 case 计数及文本格式；实际实施/测试以专属验收为准，不将文档检查写作功能或真机通过。

## 9. 查询模式构建的历史执行状态

以下为结构审计修复前的查询模式构建记录；Runner SHA256 为
`4f0fdaac779e2c7f302eedfce7a18815eaa4b898e1d0bf6879d8d24a4460339e`。
生产修改仍限两个 Orca adapter；[当前源码摘要](../acceptance/evidence/orca-local-remote/query-production-source-hashes.json)
与[验证登记](../acceptance/orca-local-remote.md)独立保存，旧构建结果不拼接为新构建资格。

| 阶段 | 当前事实 | 未完成部分 |
| --- | --- | --- |
| P0 协议 | 当前免 pair 查询、资源变化/重开、444000 字节上传及独立读取、精确清理通过；历史两轮数据回放通过 | 原生探针不是完整业务验收 |
| P1 连接与观察 | 私有 IPC/WS 连接、查询式 feed、取消/关闭及身份负例已实现并通过 L1 | 不宣称所有瞬时事件可见或本机全 IPC 故障资格 |
| P2 adapter | 本机能力门改为查询；完整 L1 273 项、既有 Controlled 16/16 通过 | V04 同服务双连接子变体仍待补验 |
| P3 现场 | 新增 3 case 已注册，17 场景/23 case 的守卫通过；本机 3 项及远程 smoke 的同构建新现场已准备 | 注册/准备不计 L3 通过 |
| P4 真机 | 历史构建的免 pair 本机开发 smoke、权限检查、远程 smoke 已通过；Check 现已退役，均确认完整收尾，Runner/driver 摘要一致 | 远程 reconnect 等管理员故障宿主，未启动模型，不计通过 |

执行采用同构建串行顺序；失败保留原现场并停止后续自动链，先核对原资源及清理，不重发未知输入。
本机开发必须验证一次反馈、新 head、同一 PR 精确 head 合入和实际停止。
远程重连的管理员前置与本机查询能力分开登记。完整验收仍要求 V04 与五项 L3 的明确结果，不能因当前部分通过就标记全部完成。
当前四项通过与一项前置阻断的同构建记录见[验证汇总](../acceptance/evidence/orca-local-remote/query-qualification.json)。
剩余工作为补验 V04 的现有 Controlled 子变体，并在管理员宿主就绪后补跑远程重连；不要求用户修改 Orca。

## 10. 结构审计修复与 PR 验证

1. 删除 testkit 每次读取 preparation 选择目标的逻辑，将本机 runtime/源路径冻结到现有 manifest；回归测试证明 preparation 改写不改变目标，目标改写会改变 manifest 摘要，缺项及跨目标混入被拒绝。
2. 删除本机订阅成功夹具，保留真实 pipe 查询、上传、取消和关闭测试；远程订阅覆盖不变。
3. 删除无人调用的 pair 链接解析函数及其专用字段，不引入替代层。
4. 合入主线发送前恢复，本机只读 IPC 的 I/O 失败使用既有连接错误类型及 5 秒连接上限；认证、身份和协议错误保持不可重试。
5. 最终构建执行完整 L1/Controlled、native driver 编译、框架校验及受影响的原生验证。历史 L3 的源码/构建摘要不改写；远程故障与 V04 缺口单列，PR 在验收缺口关闭前保留 draft。

最终审计修复验证：干净生产提交 `0aa7f2d`、Runner `49104e8a725718ddb3f974c85678d2b22e3d5890b955cffea05e2926d3f1dc8c`。L1 280 项、Controlled 16/16、真实无模型 IPC 探针及修复后的本机 smoke 均通过；本机反馈、新 head、精确合入及 Run/服务停止已确认。[汇总](../acceptance/evidence/orca-local-remote/pr-verification.json)单列历史 L3 和剩余缺口，后续提交仅追加文档/证据，不修改受测生产源码。
