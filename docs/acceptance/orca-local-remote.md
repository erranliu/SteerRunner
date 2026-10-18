# Orca 本机/远程连接实施与验收

日期：2026-10-18。状态：已按“不修改 Orca”实施本机查询模式，当前同构建四项可运行 L3 全部通过；远程 reconnect 前置阻断，V04 新子变体未验，未完成全部验收。旧订阅失败与三项 L3 通过均作为历史独立保留。
依据 [Spec](../spec/orca-local-remote.spec.md)及 [Plan](../spec/orca-local-remote.plan.md)。

## 来源与生产范围

基线 `9f9eca01777f4259f34bd5e1449e8fa2a1278069`；分支 `codex/orca-local-remote`，
独立 worktree `C:/Users/lmas1/.codex/worktrees/orca-local-remote/SteerRunner`。
生产改动仅在 `adapters/protocol/orca` 与 `adapters/orcatransport`；上层生产代码、公开契约、
Profile v3、State v5、target hash 和数据库格式不变。测试/现场脚本/文档另列。

## 当前查询构建

Runner SHA256 `4f0fdaac779e2c7f302eedfce7a18815eaa4b898e1d0bf6879d8d24a4460339e`，
当前生产文件冻结于[查询构建源码清单](evidence/orca-local-remote/query-production-source-hashes.json)。
本机用 named pipe/authToken 原生查询，不依赖 pair、不调用订阅、不修改 Orca；远程继续订阅加补查。
现有 StateFeed 接口、30 秒补查、NativeProof 和开始/完成判定保持；本机 feed 无后台采样线程，不合成 working 事件。
查询式 feed Close 会取消在途查询；查询/身份失效仍阻断，未知输入不重发。

| 当前范围 | 结果 |
| --- | --- |
| 定向协议/adapter | 查询 feed、无订阅依赖、关闭取消、身份冲突、查询失败在创建前拒绝及既有完成判定通过 |
| 历史真实免 pair 数据回放 | 两个真实样本与当前解码器兼容；现有完成补查询 29 个子项及恢复首读测试通过，非新真机资格 |
| 当前无模型原生 IPC 探针 | 通过：查询观察真实终端变化、重连后原终端存在、444000 字节上传及独立读取摘要、精确资源清理，见[原生探针](evidence/orca-local-remote/local-query-ipc-probe.json) |
| 完整 L1 / Controlled | L1 273 项、既有 Controlled 16/16 通过，见[前置结果](evidence/orca-local-remote/query-prerequisites.json) |
| 本机开发 L3 | 新现场 `C:/srt/d8e613a93d50` 通过：无 pair、查询式观察，原会话一次反馈、新 head、同一 PR 精确 head 合入、Run/服务停止；见[结果摘要](evidence/orca-local-remote/query-local-smoke-result.json) |
| 本机 sandbox Check L3 | 新现场 `C:/srt/36504a1549f9` 通过，12 项及完整资源/服务收尾；见[结果摘要](evidence/orca-local-remote/query-local-sandbox-result.json) |
| 本机 full_access Check L3 | 新现场 `C:/srt/152a49031439` 通过，12 项及完整资源/服务收尾；见[结果摘要](evidence/orca-local-remote/query-local-full-access-result.json) |
| 远程 smoke | 新现场 `C:/srt/2a85b0663850` 通过：原会话反馈、新 head、同一 PR 精确合入及 Run/服务停止；与本机三项使用相同 Runner/driver，见[结果摘要](evidence/orca-local-remote/query-remote-smoke-result.json) |
| 远程 reconnect | 仍缺管理员网络故障宿主；当前构建[预检记录](evidence/orca-local-remote/query-remote-reconnect-preflight.json)为 blocked，未创建 fixture、未启动模型，不属于本机协议问题 |

当前五项定向 L3 为四项通过、一项前置阻断，见[同构建汇总](evidence/orca-local-remote/query-qualification.json)。
V04 新的同服务双连接 Controlled 子变体仍未完成；既有 16/16 回归不能替代它，因此不声明 Spec 全部通过。

| 当前验收 ID | 结果 |
| --- | --- |
| OLR-A01 | L1 路由/无回退及真实免 pair 本机、明确 environment 远程 smoke 均通过；联合 Controlled 验证的缺口单列 A05 |
| OLR-A02 | 连接抽象、查询式 feed/远程订阅、原生查询/上传及源码边界核验通过，未改上层或 Orca |
| OLR-A03 | 当前本机开发、一次反馈、原生 final、同一 PR 精确合入及停止闭环通过 |
| OLR-A04 | 当前本机 sandbox/full_access Check 的十二项、owned PR/引用/目录、writer 和服务收尾均通过 |
| OLR-A05 | 未完成：远程重连前置阻断，V04 新子变体未验；当前 L1、既有 Controlled 与远程 smoke 的通过事实保留 |

## 初版 P0 原生事实与订阅失败历史

本机 Orca 1.4.224，runtime `456f70a1-9c53-406c-a48a-72833d36abdb`。
从受测安装包的 CLI `runtime/metadata.js`、`runtime/transport.js`、
`shared/runtime-bootstrap.js` 和 main runtime 的实际 dispatch 路径确认：

- 本机 metadata 提供 named pipe endpoint 和原生 authToken；消息为逐行 JSON。
- 本机 unary status/worktree.ps 与 444000 字节分块上传均能在无 pair 环境工作，上传后从真实目标工作区独立读取核对摘要。
- `session.tabs.subscribe` 实际返回 `method_not_supported: Method session.tabs.subscribe requires a streaming transport`。
  main runtime 的本机请求走 unary dispatch，流式方法被显式拒绝；本机 pipe 连接成功不代表支持订阅。
- 当前广播的 WebSocket 使用设备配对认证，不能把本机 authToken 当成免 pair 订阅凭据。

失败历史：第一探针 `C:/srt/olr-ipc-b36a2d9922` 发现订阅拒绝；第二探针
`C:/srt/olr-ipc-411cac9178` 同时保留上传成功与订阅拒绝。两次探针 owned terminal/worktree 均已清理；
未运行模型、未创建 PR，未修改共享 runtime、认证或全局环境。复制的脱敏摘要见
[原生探针记录](evidence/orca-local-remote/local-ipc-probe.json)。

初版实现不以轮询替代订阅，不生成临时 pair。adapter 在创建业务工作区前核验订阅，
不支持时返回 `local_ipc_streaming_unsupported`，保留零业务输入；该门有真实 named pipe 协议夹具测试。
以上是上一构建的能力门行为；用户随后明确不能修改 Orca。当前已改为原生查询能力核验，订阅缺失不再作为本机开发阻断条件；原查询不可用仍在创建前拒绝。

## 本机查询模式复核（后续设计修订依据）

SteerLine 历史提交 `687eb3f21c8c626b87f8af408fca1d8f2c5cf2dc` 的
`docs/acceptance/orca-local-native-channel.md` 记录了同版本 Orca 1.4.224 的免 pair 原生查询：
本机 named pipe 调用 `session.tabs.list` 与 `worktree.ps`，两轮原请求均有接受/开始回执，
主代理 working→done、原 pane/provider/terminal incarnation 和最终原文均已核验。
该历史验证是两轮能力探针，不是免 pair 本机完整业务交付；早先本机真实 PR 验收使用过配对通路，二者不能混算。

本轮在当前工作树回放以上两份真实响应，`TestOrcaLocalPipeNativeEvidence` 的两个子项通过；
`TestOrcaCompletionQuery` 的 29 个子项及恢复首读关闭测试通过，涵盖没有 working 通知时的新 done、
旧结果、错误 prompt、pane/provider、短回合、初始化/额外输入边界及未知零重发等既有规则。
见[复核摘要](evidence/orca-local-remote/historical-query-review.json)。没有新增模型回合或资源。

可行的最小修订是本机用原生有界查询、远程保持订阅加补查，继续使用现有 `StateFeed` 接口、
`QueryOriginalAgent` 和 `NativeProof.query`。本机 feed 的初始状态来自原 `session.tabs.list`；
不伪造订阅事件或 working，完成仍由原请求开始回执、发送前基线及新主代理 done/完整正文共同证明。
优先复用既有 30 秒补查节奏，不新增后台轮询线程或通用观察框架；代价是本机结束发现可能延迟约一个补查周期。
查询模式不承诺捕获每个瞬时状态或检测采样间隔内任意外部并发输入。

原 Spec 中“本机必须订阅、禁止查询轮询”的约束比现有完成判定实际要求更强；现已修订该约束并接入查询式 feed，
不要求修改 Orca，生产范围仍限两个 Orca adapter。之前三项 L3 通过及两项阻断的历史结果不改写；
当前构建单列短回合、旧 done/身份冲突/取消/恢复及本机开发 L3 的实际结果，远程共享改动做相应回归。

## 上一构建测试登记（不借作当前构建资格）

首个 sandbox 环境检查现场 `C:/srt/f55ba9662a63`、Check
`33f8542bb812fbfc61e28e7e962c860c` 在模型前失败，产品状态为 failed/residual。
原 adapter 只按 remote URL 选择检查项目，本机两份同 URL 注册仓库导致歧义。
已在 `orcatransport/environment_check.go` 增加本机 `local_path` 匹配及尾部分隔符归一化；
本机缺失/重复路径、远程歧义仍在创建 actor 前拒绝，有独立回归测试。
原失败记录不改写，独立复核确认无该 Check 的 actor、helper 目录或远端引用，见
[失败后只读复核](evidence/orca-local-remote/failed-check-independent-cleanup.json)。

修复后已用真实本机 actor 完成无模型 Initialize→READY→Stop，原终端停止且未创建检查目录，
未调用 Prepare，见[actor 探针](evidence/orca-local-remote/check-actor-probe.json)。
三项正式重测使用新现场和同一 Runner SHA256
`876b542ae436763922032cdd8abe5f9c3de9d2252643c29ccb32a9fd4eeeefc0`。
生产修改文件的冻结摘要见[源码清单](evidence/orca-local-remote/production-source-hashes.json)。
实际模型为 `gpt-6-luna / high`；本机 Codex 0.159.2，CLI、agent/helper 的可核验来源及摘要见
[工具清单](evidence/orca-local-remote/tools.json)。本机和远程执行主机各使用其已冻结 helper。

| 范围 | 当前结果 |
| --- | --- |
| 定向协议/adapter/testkit | 初轮通过；新增 IPC 和提前阻断测试纳入完整前置 |
| native driver | 已编译，新增 3 个 case 已注册；注册不计通过 |
| L1 / Controlled | 当前构建 L1 272 项、Controlled 16/16 通过，见[前置结果](evidence/orca-local-remote/prerequisites.json)；首轮及脚本 skip 白名单修复历史保留 |
| 本机 IPC 原生探针 | 失败，原生流式能力缺失；查询/上传及资源清理事实单列，不改写为通过 |
| 本机开发 L3 | 原生订阅能力阻断，未运行模型，不登记通过 |
| 本机 sandbox Check L3 | 新现场 1/1 通过，12 项及 cleanup completed，见[原始结果摘要](evidence/orca-local-remote/local-sandbox-result.json)；前一失败现场独立保留 |
| 本机 full_access Check L3 | 新现场 1/1 通过，12 项及 cleanup completed，服务已停止，见[原始结果摘要](evidence/orca-local-remote/local-full-access-result.json) |
| 远程 smoke | 新现场 1/1 通过，原会话一次反馈、新 head、同一 PR 精确 head 合入、Run/服务停止，见[结果摘要](evidence/orca-local-remote/remote-smoke-result.json) |
| 远程 reconnect | 管理员网络故障预检阻断，未启动模型、未创建 fixture，不计通过 |

远程既有 environment 的只读预检：Orca 1.4.223，runtime
`f8616747-34af-4543-8487-ec6b5d8de4ff`。当前调用进程非管理员，旧管理员网络宿主已退出；
不通过省略故障或更改预期来把 reconnect 报通过。

远程[原生查询](evidence/orca-local-remote/remote-query-proof.json)与
[重连预检阻断](evidence/orca-local-remote/remote-reconnect-preflight.json)分别保存；查询通过不能替代重连测试。
Controlled 16 项是既有共享执行语义回归；本轮路由和真实临时 IPC/WS 夹具在 L1 验证。
尚未增加 Spec V04 所列的同服务双目标真实连接 Controlled 子变体，故不声明完整 V04 通过。

本文件不声明完成 OLR-A01–A05 或正式双模式支持，后续结果与失败历史在原身份下追加。
本轮五项定向 L3 为三项通过、两项前置阻断；三个通过结果的 Runner 和 native driver 摘要一致。
未把失败后的重测算作首次全过，也未执行完整稳定性/并发资格，见[汇总](evidence/orca-local-remote/qualification.json)。

## 上一构建验收项边界

| 验收 ID | 当前证据与未完成项 |
| --- | --- |
| OLR-A01 | L1 路由、pair 干扰与无回退通过；真实本机开发无 pair 闭环被原生订阅阻断 |
| OLR-A02 | 私有连接抽象、IPC 查询/上传、协议夹具通过；真实本机订阅未通过 |
| OLR-A03 | 本机开发 L3 未运行，不通过 |
| OLR-A04 | 本机 sandbox/full_access Check 均通过，12 项及资源/服务收尾已核验 |
| OLR-A05 | 完整 L1、既有 Controlled 回归、远程 smoke 通过；重连前置阻断，V04 新子变体未验，未完成此验收项 |
