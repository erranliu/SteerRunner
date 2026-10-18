# Spec：Orca 本机与远程连接统一

日期：2026-10-18。状态：本机原生查询、远程订阅加补查已实施，三项结构审计发现已修复并整合主分支连接恢复。最终构建验证与历史四项 L3 分开登记；远程 reconnect 前置阻断，V04 新子变体未验，尚未完成全部验收。未修改 Orca，生产改动限两个 adapter。配套 [Plan](orca-local-remote.plan.md)，证据见[专属验收](../acceptance/orca-local-remote.md)。

调查基线：SteerRunner `9f9eca0`。执行语义继续遵守 [Runner Spec](steerrunner.spec.md)和[沙箱认证 Spec](sandbox-authentication.spec.md)；测试按[测试指南](../testing.md)分层。原 [L3 稳定性 Spec](l3-execution-stability.spec.md)的既有 case、三轮资格及历史结果保持原范围，本次不复制或重取整个稳定性矩阵的资格。

## 1. 目标与范围

调查基线的公开契约和 CLI 已区分 Orca `local / orca_environment`，但 `SubscribeState / QueryState / QueryWorktreeAgents / UploadWorkspaceArtifact` 都要求 pair，生命周期又把配对查询当成本机 CLI 前置。本次补齐 Windows/amd64 本机原生 IPC；连接按同一目标选择 IPC 或现有远程配对通路，观察策略按实际原生能力选择查询或订阅加补查。

保留一个 `orca-terminal` 后端，共用 `StateFeed`、工作区、原生回合证明、最终回复、执行主机准备、停止和恢复逻辑。CLI 继续执行工作区/终端/输入操作，不整体改为 RPC。

**生产代码修改仅限 `internal/execution/adapters/orcatransport/` 及其原生协议包 `internal/execution/adapters/protocol/orca/`。** 上层已能表达 local/remote，本次保持 `contracts`、`application`（包括 freeze、engine、environment-check owner）、`backend / reliable / workflow`、storage 和 Codex/agentlaunch 的生产代码及接口不变。测试、现场脚本和说明文档可以同步更新，这不构成上层运行逻辑改造。

不扩展多实例管理、environment 重命名治理、跨 runtime 迁移或其他操作系统；不借此次连接改造新增通用生命周期阶段、存储模型或认证框架。原需求不能满足时说明具体缺口，不以复杂平台建设作为默认解法。

**不修改、升级或自配对 Orca 作为实现前置。** 同版本 Orca 1.4.224 的历史免 pair 两轮查询样本已由当前解码器回放通过；当前原生查询/上传探针另行通过。named pipe 不支持 `session.tabs.subscribe` 是已知协议差异，不再作为本机开发的必要能力。本次修订替代早版“本机必须推送订阅、禁止原生查询观察”的限制；不放宽原请求与原生完成事实的受理条件。

## 2. 最小设计

### 2.1 路由与认证

| 既有目标 | 直接 RPC | CLI | 失败行为 |
| --- | --- | --- | --- |
| `local`，内部 environment 为空 | 由 Orca 本机 metadata 发现 named pipe，使用原生本机认证 | 不加 `--environment` | 不尝试 pair 或远程 environment |
| `orca_environment`，内部 environment 非空 | 按指定 environment 读取现有 endpoint/token/公钥，保留现有加密连接 | 显式传同一个 `--environment` | 不回退本机或进程 pair |

不增加 mode/connection_kind；连接种类由现有目标确定。Orca user-data 来源继续沿用服务环境中的 `ORCA_USER_DATA_PATH` 及平台默认目录，CLI 与直接 RPC 必须一致；本期不把该环境依赖升级成新 profile 字段或用户配置系统。

本机不读取 `ORCA_PAIRING_CODE / ORCA_REMOTE_PAIRING`。两种目标的 CLI 均屏蔽上述变量和 `ORCA_ENVIRONMENT`，避免隐式路由覆盖明确目标；移除 lifecycle 中本机 pairedOnce 门及放行 pair 继承的逻辑。远程选择器的既有 ID/name 语法保持，不在本次另立规范 ID 的持久化流程；歧义不得任选，沿用或补足局部校验即可。

无 pair 不等于无认证。本机认证使用实际 Orca 协议，远程握手、加密和认证保持现有行为；凭据只在连接过程中使用，不进入日志、profile、资源回执或测试证据。

### 2.2 只在协议包内抽象连接

在 `internal/execution/adapters/protocol/orca` 内增加一个私有连接接口和一个打开连接的函数。接口仅承载实际需要的请求发送、按请求读取响应及关闭；具体命名不固定。按 environment 分支选择本机 IPC 或现有 `stateSocket`，两者分别负责自己的认证与分帧。相同的请求/runtime 校验可复用，不为形式统一重写已工作的远程协议。

现有 `StateSubscription` 改为持有该连接，保留自己的 `Next/Close`；无需再增加通用 Subscription 接口、可注册 transport 插件、独立凭据 provider 或连接管理器。查询和上传仍使用独立有界连接，不与订阅读取器争抢帧。不得使用可变全局连接或目标缓存。

观察策略不强求相同：本机 `StateFeed` 初始化用 `session.tabs.list` 绑定原终端/pane，之后用现有 `QueryOriginalAgent`（`worktree.ps` 加 tabs 元数据核验）查询；不调用本机订阅，不伪造 snapshot/delta 或 working 事件。远程继续使用订阅加独立补查。复用现有发送前基线、原请求接受/开始回执及 `NativeProof.query` 判定，保留 30 秒补查间隔与每次 40 秒有界查询；本机 feed 不新增轮询线程、定时器、队列或配置，Close 取消在途查询。结束发现可能延迟约一个补查周期。

短回合可以未采到 working；只有原请求明确已开始、原主代理新 done 的时间晚于发送前基线，且 prompt、原 pane/provider、完整原文及排除标志均符合现有规则，才能结算。不要求为了取得一次 working 而加快采样；缺失基线、旧 done、初始化/恢复/工具输出、中断或错误身份仍不通过。该查询模式不承诺捕获每个瞬时状态或检测采样间隔内任意外部并发输入。

`SubscribeState / QueryState / QueryWorktreeAgents / UploadWorkspaceArtifact` 复用既有调用形状；连接分支由 adapter 内部消化。公开请求/配置、LifecyclePort、回执和错误契约保持，上层不感知 IPC/配对握手，也不新增执行步骤。

### 2.3 复用现有资源身份和回执

继续以现有 `ProcessIdentity` 和 workspace operation receipt 保存 runtime/worktree/terminal/incarnation，以 `NativeProof` 核验 pane/provider session/原请求与回合。不增加 `orca.connection-binding` envelope，不变更 Profile v3、State v5、target ID 或配置摘要算法，不建立 v3/v4 兼容视图。

初次连接须核验本机 metadata、CLI status 和直接 RPC 返回的 runtime 一致；已绑定资源的操作使用记录中的 expected runtime。补齐本机工作区当前较弱的检查：复用 `workspaceCreation.ResolvedRuntimeID`，将发现 runtime 与创建响应比较；后续原生查询继续与已记录身份比较。检查失败保留原操作回执和未知事实，停止后续输入，不按名称创建替代资源。

创建前的本机能力核验只需原生 `worktree.ps` 与源项目 `session.tabs.list` 可用、runtime 一致；不创建临时 agent 或要求订阅。环境检查按 URL 与本机 `local_path` 精确选择注册源仓库，避免本机两份同 URL 项目歧义；远程保留原选择规则。

该局部检查不声称解决 CLI 在检查与副作用之间切换 runtime 的所有竞态，也不为此增加通用事务协议。已有 operation intent、creation checkpoint 和未知结果对账规则保持。

旧记录不迁移、不重写。已绑定 runtime 与当前目标不一致时按现有身份冲突阻断；旧本机 profile 曾通过 pair 指向远程的记录不能被悄悄接管成本机资源。服务换代的显式 resume、新输入撤销和未知输入零重发规则继续适用。

## 3. 必须保留的行为

1. **真实原生取证。** 本机支持 status、tabs 查询、worktree.ps 及文件分块写入所需协议，提供既有请求开始回执、原主代理新 done 和 final；远程保留原生订阅。允许本机有界查询观察，不能用屏幕、标题、模型声明或私有数据库替代原生完成事实；不要求修改 Orca 或把本机加入配对环境。
2. **有界连接。** 沿用查询 timeout、消息上限和队列限制；IPC 按实际分帧处理半包/EOF。取消和 Close 中断阻塞读写；建连 timeout 不得连带取消已经返回的长订阅。只读重试不重置 Run 的原总期限。
3. **错误语义。** 沿用现有错误类型。身份冲突不能归入普通断线自动换资源；认证/能力缺失在业务输入前阻断；输入或创建结果未知不重发。无需建立新的错误分类体系。
4. **执行主机准备。** 本机 Orca 仍使用实际执行主机的 helper、认证、同 app-server 检查和权限启动。连接是本机不能成为跳过准备的理由；sandbox/full_access 不互相回退。
5. **文件范围。** 上传仍经选中 Orca 的原生端口，限定原 worktree 的 `.sr-runtime-stage/`，验证 runtime/执行 host；不直接写调用方同名目录，不扩展失败后追加或恢复上传语义。调用端的 local/remote 与 RPC execution-host ID 分属不同含义。

实现前核对受测 Orca 的本机发现、认证、分帧和查询协议，并单列订阅的适用通路。`LocalRuntimeID` 只读身份，历史 pipe 样本也只证明解码，均不能代替实时连通性。未知协议须先通过小型真实探针确认，不要求先建 IPC 故障代理平台。

## 4. 验证分层

| 编号 | 所在层 | 独有验证目标 |
| --- | --- | --- |
| OLR-V01 | L1 路由/连接 | 本机忽略有效及无效 pair；远程失败无本机回退；两目标使用同一 user-data 来源；CLI 隐式路由被屏蔽 |
| OLR-V02 | L1 协议 | 临时真实 named pipe/WS 协议夹具：认证、分帧、keepalive、错误请求/runtime、超限/半包/EOF、取消、关闭和资源回收；远程 snapshot/增量；本机查询 feed 不依赖订阅、不伪造事件/后台采样，Close 中断在途查询 |
| OLR-V03 | L1 状态/文件 | 复用原生状态样本和取证规则；错 pane/子代理/换代拒绝；上传分块、范围、失败和内容摘要；旧 profile/target hash 不变 |
| OLR-V04 | Controlled | 在现有真实 Runner EXE/CLI/SQLite/隔离 Git 夹具中覆盖双连接接线、同服务两目标隔离、临时断线重接原资源、服务换代 resume、停止撤销、结果未知零重发；扩展已有测试，不新建平行 driver |
| OLR-V05 | 无模型原生探针 | 当前真实 Orca 本机认证、status/query、查询可见真实资源变化、独立连接关闭后重开、文件写入及执行主机独立摘要读取；固定 runtime，精确清理本轮辅助资源 |
| OLR-V06 | L3 | §5 的 3 个新增真实流程及 2 个既有远程回归项；验证原生接线、真实权限准备、原生 final、Git/PR 和资源收尾 |

L1/Controlled 使用协议夹具验证确定性负例，不启动模型、不创建平台 PR。双目标隔离、无回退和恢复算法主要在这些层验证，无需为每一条断言创建完整真机 PR。

当前既有 Controlled 16/16 只证明共享执行语义回归，不能直接签收 V04 的同服务双连接新增子变体；该子变体尚未完成。L1 的连接夹具、历史样本回放及 V05 也不替代本机 L3。远程 reconnect 缺管理员故障宿主时明确记前置阻断，不省略故障或更改通过标准。

原生探针接入既有 native 测试入口或 fixture 准备阶段，不新增模型 case，也不新增生产探针命令。使用专属辅助 worktree/terminal 和非秘密 nonce 文件；记录真实通路、原生 envelope、摘要和关闭事实，禁止停止共享 Orca 或改全局环境。主动关闭重开只证明真实连接可重建，不冒充活动 Run 自动重连或完整 IPC 不可达验收；自动重连算法由 Controlled 验证。

## 5. 收敛后的 L3

仅新增以下 3 场景/3 case，现已注册但不表示真机通过；复用现有 development-smoke/environment-check driver、fixture 和结果契约。

| 文档编号 / 拟注册场景 | Case | 固定预期 | 独有覆盖 |
| --- | --- | --- | --- |
| OLR-L01 `orca-local.smoke` | `default` | `merged`；原 Run/服务/辅助资源停止 | sandbox；服务及子进程无 pair 和隐式 environment；真实 IPC 初始查询、原请求开始回执、新主代理 done 和 final；实施、原会话一次反馈、新 head、同一 PR、精确 head 合入；最终 stop 不能仅结束观察 |
| OLR-L02 `environment-check.orca-local-sandbox` | `complete` | `checked`；owned PR 关闭不合入、引用/工作区删除、writer 停止 | 本机 Orca 的真实 helper、MXC、认证与既有十二项工具检查；绑定原 runtime/session |
| OLR-L03 `environment-check.orca-local-full-access` | `complete` | `checked`；同上 | full_access 实际生效及对应原生检查/收尾；不另跑一遍 full_access 开发闭环 |

此外，使用同一受测构建执行既有 `orca-remote.smoke/default` 和 `orca-remote.connection-reconnect/active-disconnect` 各一次，分别验证远程完整流程与抽象后远程观察重连无回归。复用现有故障设施和判据，不改变其完整双通路断连要求，也不把远程结果描述为本机 IPC 故障通过。

本次必跑真机集合为 **新增 3 + 既有 2 = 5 case，各一次**。本机两种环境检查的主要目的是证明真实执行主机接线；它们不能由共享权限编译的单元测试替代。本机开发 smoke 已有一次反馈与真实停止，故不再复制 caller-exit、四种 restart、三种 stop、两种 publication-unknown 等共享稳定性矩阵。

取消前版 18 个新增 case、99 次资格、强制跨目标并行批次、IPC relay/进程重定向设施及新 routing driver 的要求。这些不是本次的待办项。只有实施暴露了独立的传输缺陷、现有验证确实不能覆盖时，才按具体风险增加最小测试；不得自动恢复整套矩阵。

整合主分支后，既有 L3 14 场景/21 case 保留，本功能只新增 3 case，当前为 17 场景/24 case；实际计数由注册展开校验。只声明本次 5 项验证及各层结果，不宣称重新通过完整 L3、完整稳定性或本机全 IPC 故障/多目标真机并发资格。失败保留原记录，修复后以新现场补跑受影响项，不能覆写历史失败。

## 6. 结果与验收

复用现有 manifest v1/v2、live-config 和结果格式。不增加 manifest/config v3、新 outcome、混合目标数据模型或通用审计事件系统。允许为本机 fixture 添加必要的可选 helper 路径；agent/model/effort 优先复用已有本机配置。不能把远程专用 runtime 字段改成本机含义；本机 `local_runtime_id` 与 `local_repository_path` 冻结进现有 manifest，受其摘要保护，再与原生回执交叉核验。`preparation.json` 只记录准备结果，不能作为运行时路由来源；远程/Codex fixture 拒绝携带这两个本机 Orca 字段。

新增 3 case 各自独立 fixture、state-dir、Run/Check、工作区/会话、分支、PR 和证据目录，可共享只读安装及 Orca runtime。没有本机故障设施，不修改共享端点/ACL。默认串行验收；并行安全由资源隔离设计和 Controlled 验证，本次不外推真机并行资格。

沿用现有 case 期限：开发/远程重连 30 分钟，环境检查 15 分钟；沿用原生 timeout 和收尾预算，不新增一套期限体系。保存受测源码/EXE/工具摘要、pair presence=false、通路与 runtime、必要原生事实、Git/PR 身份及清理结果，不记录凭据、完整环境或认证帧。test/business/cleanup 结论分开；未知、skip、缺结果、错误身份或清理未确认均不得通过。

| 验收 ID | 完成条件 | 覆盖 |
| --- | --- | --- |
| OLR-A01 | 本机无 pair、远程明确 environment，失败无隐式切换 | V01/V04/V05，L01 |
| OLR-A02 | 私有连接抽象支持原生查询/上传及远程订阅；本机查询和远程订阅加补查共用 StateFeed，现有身份和执行语义保持 | V02–V05，源码范围审查 |
| OLR-A03 | 本机开发、反馈、原生 final、真实 PR 和停止闭环成立 | L01 |
| OLR-A04 | 本机两种权限的真实执行主机准备和 Check 完整收尾 | L02/L03 |
| OLR-A05 | 共享可靠性负例及两项远程真机回归通过，证据完整 | V04/V06；完整 L1/Controlled；登记 5 case 原始结果 |

实施后在 `docs/acceptance/orca-local-remote.md` 登记以上结果、失败历史和未验边界。README 等支持声明以真实完成的范围为准，不继承其他构建的历史通过。
