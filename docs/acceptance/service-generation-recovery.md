# 服务换代恢复验收

历史构建验收记录：以下 Check、旧状态升级和已删除源码的描述仅说明当时构建，不是当前入口。2026-10-29 已退役旧业务，当前边界见[退役记录](check-retirement.md)；原始失败、资源与结算证据保持。

阅读范围（2026-10-21 整理）：本文保留首次实施及当时未完成矩阵，以下“工作树未提交”“Orca 外部接口阻塞”等均是历史时点。核心、parser 与创建执行器的后续构建和结果见[近期 PR 补测](pr-test-gaps-20261020.md)及[创建恢复复验](pr38-review-20261020.md)；真实 Orca、管理员故障、跨平台与调用方同现场资格继续按各报告范围判断，不把文档整理记作新增通过。

2026-10-20 内容流程同构建 takeover 修复的定向与真实验收见[专属记录](content-takeover.md)，
不以该专项结果替代本文其余恢复/SteerLine 验收条件。

日期：2026-10-19。对应 [Spec](../spec/service-generation-recovery.spec.md) 和 [Plan](../spec/service-generation-recovery.plan.md)。整体状态：**未完成**。Runner 与 `C:/Project/SteerLine` 工作树均有实现，未提交、未部署；不将受控协议夹具算作真实 Codex、Orca 或 SteerLine UpdateRunner 资格。

2026-10-20 责任纠正及追补：下文“Orca 外部阻塞/需要上游接口”的归因不再成立，保留为当时调查和测试历史。创建恢复由 SteerRunner 的有限独立执行器实现，只调用现有公开接口；服务退出后继续保存原响应，新服务按原日志核实原资源，缺证据保持 unknown、零重复创建。本轮实现及实际构建、失败和复验统一见[近期 PR 追补验收](pr-test-gaps-20261020.md)，不将历史未验项自动改写为通过。

## 来源与实现范围

Runner 基线 `fa5e0a82da88f77eba4cb539d17bf0a4a18dce29`，当前工作树增量。Windows/amd64，PowerShell。SteerLine 基线 `9554eb61eefa417f5cd900d335650dacff3d3311`，后续按用户“继续实施”授权修改其源码/文档并运行隔离测试；未修改安装或保留运行环境。其专属结果在 `C:/Project/SteerLine/docs/acceptance/service-generation-recovery.md`。

已实现的 Runner 路径：服务不再统一撤销 Run 授权；冻结 workflow 执行语义比较；state/v4 语义写门及 v3 一致性备份/事务升级；能力版本集合和只读 takeover 报告；启动 pending_stops 原信封排序；本机独立 owner 保留原生 Job/连接/认证，原请求回执留在原 owner，离线结果沿现 journal 补收；新答案按原执行/问题/来源绑定；显式 stop 与服务脱离分开，owner 退出事实签名并绑定原执行。

owner 不读取 runner.db、不推进 workflow、不保存授权副本。格式升版没有新增表、Operation v2、派发租约、通用 spool 或导入游标。原 Request/Profile/Workflow.Config/Materials/PromptBodies/Intents/结果和 PR 仍由原 Record/Store 负责。

## 已有证据

| 范围 | 命令 / 来源 | 实际结果与限度 |
| --- | --- | --- |
| T02 | `go test ./internal/storage -run TestRecoveryUpgrade -count=1` | 原聚合正文及含 WAL 内容的一致性备份保留；元数据更新注入失败回滚；未知持久字段在改格式前拒绝。不是任意历史格式兼容证明 |
| T03/T04 | `go test ./internal/application -run 'TestServiceGeneration\|TestSRResumeWorkflowRetainsOriginalReportSource\|TestWorkflowRecovery' -count=1 -timeout=90s` | 已通过；Engine.New/Shutdown/第二代 New 保留授权、暂停、等待、阻断、PR 终点和 pending resume；描述/目录变化兼容，执行入口变化拒绝。只证明列举的记录与语义边界 |
| T03/T06 | `go test ./tests/integration -run '^TestSRRestartPreservesAuthorization$' -count=1 -timeout=180s -v`（正常/强杀两个原子场景版本） | 已通过，69.272s；原 owner/root 在服务退出后存活，服务离线时原 final 保存，重开无需 resume 自动到 pr_ready；thread/start=1、turn/start=3；再次重启显式 stopped 不变。正常场景实际构建 A/B 的 SHA、资源摘要、产品版本不同。真实 Windows 进程与 EXE，原生/provider 为受控夹具；后续增加了创建回执/启动 stop 子场景，须用下方重验结果判定当前源码 |
| T03/T07 | `go test ./internal/storage ./internal/contracts ./internal/host ./internal/execution/adapters/codex -run 'TestRecoveryUpgrade\|TestPendingStops\|TestExecutionOwner\|TestIndependentOwner' -count=1 -timeout=90s` | 修复后通过（storage 2.418s、contracts 0.841s、host 3.309s、codex 1.094s）；验证 startup 原请求不能丢弃/改正文、未知裸字段拒绝、服务 detach 不杀树、签名/完整原身份/原 payload 校验 |
| 基础路径 | `go test ./tests/integration -run '^TestSRInputFreeze$' -count=1 -timeout=150s -v` | 早期实现通过，38.6s；实际 CLI/IPC/SQLite/owner/三个原生夹具回合。不是最终同构建全量资格 |
| Orca 原创建接口 | `orca worktree create --help`、`orca terminal create --help`，只读 CLI 帮助 | 未发现公开原请求关联/精确创建查询参数；不能据此证明底层不存在接口，也不能宣称创建丢回执正例已通过。未创建 Orca 资源 |

## 失败与修复记录

1. 初次全 application 组中 `TestSRResumeWorkflowRetainsOriginalReportSource` 的人工记录未填实际 adapter；新具体语义比较拒绝。补齐夹具，定向重验通过，未削弱生产校验。
2. 首次恢复集成夹具未启用异步 hold 释放；随后暴露 Engine.Shutdown 仍会撤销全部授权。修正 shutdown 并补入 New/Shutdown/第二代 New 回归。之后多 provider 模式漏写 Query 的原 PR URL 参数，修正夹具；正常/强杀两项最终通过。
3. 新增退出证明测试发现签名前正文带换行，但 RawMessage 写盘被压缩，验签失败。改为直接签 json.Marshal 正文；原绑定正例与篡改/错来源反例通过。
4. 收紧存储读取时误用公开请求 exactFields 解码器，合法持久记录中的 null slice/嵌入字段被拒；当前已改用持久类型的 DisallowUnknownFields 解码器，保留未知字段拒绝。受影响的恢复集成必须重验。该次失败发生 appserver.log 在用、临时目录清理不完整；已按精确夹具路径和原 owner 命令行核对后单独收尾，不把失败清理算作显式 stop 正例。
5. 创建回执子场景第一版在 calls.jsonl 尚未创建时调用计数助手而过早失败；改为等待真实创建调用屏障。不得把这个失败误记为已覆盖创建成功窗口。

## 首次实施的执行与待验矩阵（历史）

| 验收 | 当前状态 | 必需收口 |
| --- | --- | --- |
| A01/T01,T02 | 受控双构建、版本拒绝、冻结材料资格通过 | 原材料字节/摘要、缺失/替换和非重定向路径已核验，不从 checkout 重建；v3 活动旧 Job 不能追溯移交。真实原生兼容资格不由这些 fixture 替代 |
| A02/T03 | Controlled 两退出方式已通过 | 真实 Codex 及工具树、真实认证原期限/原 owner 文件保留尚未验证；不得替代原生资格 |
| A03/T04 | L1 状态表与 pending stop 受控通过 | 原业务等待/阻断、问题 scope、原启动撤销和重复重启不恢复授权；真实原生组合现场仍按待验项处理 |
| A04/T05,T08 | 本机受控正例通过，整体未完成 | 本机 owner 在途创建回执屏障通过；Orca 缺按原请求精确找回的已证接口，必须补原生能力或明确失败，不重建资源/替换会话 |
| A05/T06 | development、离线审计及 parser Controlled 通过 | 原 owner 完成点签名 workspace checkpoint、parser 正常/强杀保留、离线完成/停止均有受控正例；真实 Codex/认证/工具树和更多外部 PR 创建窗口仍待验 |
| A06/T07 | 定向反例与共享场景阶段性通过 | 保留未知输入零重发、CAS/stop/PR 原来源规则；早期完整 16 场景组含清理失败仍记 exit 1，后续定向通过不改写成最终完整 catalog 通过 |
| A07/T09,T10 | SteerLine 已实现，L1 与脚本通过，组合现场未验 | 当前服务 pin、公开 CLI、原停止收集/持久化、兼容部署与共同生命周期文档已接入；真实消费者 A/B 原现场未执行，不以纯消费者 fixture 冒充 |

当前新增两项 Controlled 屏障沿原 `TestSRRestartPreservesAuthorization` 入口：`creation_receipt` 在真实 thread/start 成功、响应尚未发回时强杀服务；`pending_stop` 在启动 ready/worker 前提交原 stop，继续重启重试原信封，要求零后继。没有新增生产 fault flag。

共享回归命令：

```powershell
go test ./internal/application ./internal/storage ./internal/host ./internal/cli ./internal/contracts ./internal/resources ./internal/execution/adapters/codex ./internal/execution/reliable/... ./internal/execution/workflow/... ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca ./internal/execution/process -count=1 -timeout=12m
go test ./tests/integration -count=1 -timeout=20m
go test ./tests/integration -run '^TestSRRestartPreservesAuthorization$' -count=1 -timeout=240s -v
```

长组运行期间代码又有修复，其输出只能覆盖启动该组时编译的内容。最终结果须明确修复前失败和修复后命中的子项；本文件不声明“全部目标完成”。

## 实施后回归登记

当时完整输出写入本工作区忽略目录 `.evidence/service-generation-recovery/`，包 SHA/资源摘要/实例/owner/root 的原记录在各组 caller-commands.jsonl 与 integration.log；当时运行索引为该目录的 `run-summary.json`。2026-10-20 文档核对发现这两个临时路径不在当前检出中，故保留文字来源，不再提供失效的仓内导航，也不将其算作本轮已复核证据。下文历史结果与限制保持；本轮内容接管的新证据另见[内容流程验收](content-workflows.md)。测试临时 EXE/现场按原记录收尾，不把不同构建拼为同构建全部通过。

| 执行 | 结果 | 边界 |
| --- | --- | --- |
| `go test ./internal/... -count=1 -timeout=12m` | 完整通过，application 460.485s | 包含主流程、storage、所有 adapter/protocol、parser、workspace、host/CLI 等；随后修改的 Codex owner 观察与身份检查另跑全包，不能将前者称作最终 EXE 原生资格 |
| Codex 全包 `go test ./internal/execution/adapters/codex -count=1 -timeout=90s` | 23.815s，通过 | 包含 owner 原请求/完整绑定/退出签名/断连/服务 detach，以及原 journal 完成与问题唤醒；补充 watcher 的 question 夹具第一版缺 Header，修正有效请求后通过 |
| `controlled-four-window` | exit 0，四个子场景通过 | A=`4a9c5ca746b382b439067a4d3f1db4eda7d730073afbbebdf9231008e3166d4f`，B=`a93853da2d02dc483a54a1291581f34f2cc152e708be46db7a8bca767284a1ea`；两构建 Version/资源摘要不同，相关契约相同 |
| 首次共享 Controlled，`shared-controlled-before-fixes.log` | 失败并在 20 分钟超时 | 当时误用公开 JSON decoder；另暴露快速 final 活动提示缺失、原连接通知的补收唤醒及旧的重启预期。保留失败，不计通过 |
| `final-controlled` | exit 1 | 四恢复子场景、跨重启撤销、native unknown/stop 通过；唯一功能断言失败为旧“强杀服务即 native settled”预期。新规则应保留 native，已修订并在下组重验 |
| `shared-recheck` | exit 1，部分环境阻断 | Windows 对 SHA=`29f0d2a72e3085fe6f5b4156c7c2439872388515ffa530194a95674c89c515d3` 的测试 EXE 返回“文件包含病毒或潜在不需要的软件”，后续该 EXE 调用失败；未解除/绕过拦截。此前 plain-marker、audit-zero 通过，其它受阻项不算通过。正常源码修复后的新构建可执行，不代表该告警已排除 |
| `monitor-recheck` | exit 0，三个顶层场景通过，107.333s | A=`a333171fe466b08f78845e1a5fe9c5bb5970bc283fe6c07b5ddc387487574850`；完整 workflow/活动消息、多 Run 隔离、服务所有权与离线未结算预期通过。复用原 journal 唤醒，不新增持久游标 |
| `final-recovery` | exit 1，七个顶层场景命中，357.182s | A=`a63914d6ba43522c15e6632fa2c52d4d6ab847eb69d3009d24f558c96c8f08f0`；parser-restart、stop 提交失败跨两代结算、Check、多个 Run、workflow、恢复四窗口通过。失败项为证据缺失场景仍等待重启制造 resume_required 的旧断言 |
| `evidence-guard-recheck` | exit 0，36.452s | 与上一组同一 A SHA；改为核对原 final_unavailable/原授权和无 PR/零新输入，并拒绝不适用的 resume，缺原证据不生成替代来源。原失败组仍保留 exit 1 |
| 测试目录 Controlled 16 场景，`controlled-catalog` | exit 1，869.582s | 16 个顶层场景全部命中；业务断言通过，唯一失败为恢复 kill 分支已核实 owner 退出后临时 runtime EXE 删除返回 Windows Access denied。原输出/exit 1 保留；收尾加入最多 3 秒重试，单独重验，不把第一次失败重写为通过。成功场景的 native 调用审计亦已保存 |

上一实施轮结束时的缺口（历史；parser/checkpoint/P4 后续进展见下节）：

- parser 仍是服务拥有的原生进程，停服会取消。重启测试通过的是原 parser 零重发和原预算内接续，**不是 parser 存活**；A02/A05 的整体目标仍未完成。
- 审计阶段离线完成且尚无 workspace checkpoint 时，只能保留 unverified；尚未实现原完成点事实采集，不能重采 HEAD 或重跑审计补证。
- 接管预检的冻结文件、结果获取包契约等完整资格尚需补齐；当前通过限明确列举的版本/codec/原身份测试。
- Orca 原创建查询、真实 Codex/认证/工具树样本以及 SteerLine P4 未完成。对这些项不得以当前受控成功代替。

新增接口还公开 `execution_record_versions: [1]`，避免把执行 Record 版本与 Operation 版本混为同一兼容判断。收紧后的 application/host/CLI/resources 定向回归通过（124.091s/3.133s/2.981s/0.938s），未知/缺失 Record 版本与未知 codec 均由只读报告拒绝；不反写早期构建的 capabilities。当前实际构建 SHA=`cd216ac00e5ee3e99f2d657155e3b53ddc3f9e53ee38faee8017d0217af2e62e`，其离线 capabilities 已保存到证据目录 current/capabilities.json。


新增收尾检查：显式 stop 后按原 owner PID/creation/executable 验证退出，再删除测试目录；Windows 对刚退出 image 的短暂占用只做有界重试，永久占用或原 owner 未退出仍失败。旧失败或安全软件阻断遗留的少数夹具 owner 均先核对精确 Temp/sr 目录、固定 image SHA 和原 owner 命令行后单独收尾；未按进程名批量处理，也未触碰用户运行现场。


最后收尾重验：`go test ./tests/integration -run '^TestSRRestartPreservesAuthorization$' -count=1 -timeout=240s -v -args -runner-exe <current/steerrunner.exe> -evidence-dir <current-recovery>`，exit 0，113.146s。当前 A=`cd216ac00e5ee3e99f2d657155e3b53ddc3f9e53ee38faee8017d0217af2e62e`，B=`b867eeef10dd4ddcf3562f54e41fec08c1a64646a9275da19a08066b1b55fec8`；正常换构建、强杀、创建回执、启动撤销四项及原身份退出/临时文件收尾均通过。原 16 场景组仍记 exit 1，不能将单项重验写成完整组重新通过。后续最终同构建共享资格和未完成项仍按上表收口。

## 继续实施：parser、离线 checkpoint、调用方

新增能力：本机 parser 托管到原独立 owner，原 deadline/stdin/started/final receipt 不变；Runner 退出只断连，显式 Run stop 取消原 parser 并核验树。owner 在发布匹配原 turn 的完成通知前采集 InspectAudit 事实并签名，Runner 仍沿原 WorkspaceSource、源绑定和 ancestry 规则接受，缺证/错签名不采当前 HEAD。预检补 result-acquisition/v1、冻结 parser profile/schema；原 commands 的 request_id/request_digest 仅作为 takeover 的只读关联投影。SteerLine 接线与其 L1/脚本结果独立登记，不把组合现场列为已验。

| 命令/证据目录（`.evidence/service-generation-recovery/`） | 结果 | 解释 |
| --- | --- | --- |
| `parser-checkpoint` | exit 1；5 分钟超时，未正常生成 result.json | 首个 offline 审计已补 checkpoint 并到发布，但 fixture 多 PR 模式仍返回固定 URL；整组等待消耗超时。原测试服务/owner 已按原身份显式收尾；失败保留 |
| `audit-offline-recheck` | exit 1，162.824s | 虽已给 provider query 传 PR URL，SUCCESS_OUTPUT 分支仍固定输出 /84，和其动态 provider artifact 不匹配；修正测试 fixture 返回实际 prURL。没有修改生产 provider 容错 |
| `parser-checkpoint-recheck` | exit 0，346.665s，实际 EXE `6f03b3f0f85e66cefc8d23faa7395dbe37bec0b9849f8118825cf10d5c3dc32e` | `TestSRResultAcquisition/(audit-offline\|parser-unsupported\|parser-restart\|parser-kill\|parser-stop)` 五场景；原 parser root 在服务正常/强杀后存活、离线完成、启动计数 1，无额外报告预算；显式 stop 零后继；审计离线 checkpoint 补收到 PR ready |
| `remaining-final` | exit 0，313.190s；A=`cabcbda3eb3464fab039c860d7eb6f0521cfaf4d9eb7f992d5107d2ccc8bd591` | 新增公开请求投影/结果获取契约资格后，离线审计、parser 强杀/显式 stop，加正常 A/B、强杀、创建无回执、启动撤销四窗口共七项通过；两个顶层场景均命中 |
| `go test ./internal/execution/adapters/codex ./internal/storage ./internal/host ./internal/execution/resultacquisition/... -count=1 -timeout=5m` | 通过，24.371s / 1.682s / 3.249s / 0.419s / 35.586s | 包含最终 owner parser 句柄复用、签名/错身份反例、原接受请求只读关联、现 parser qualification；初次签名测试用非法 command ID，修成真实 32 hex fixture 后通过 |
| `go test ./internal/application ./internal/host ./internal/execution/resultacquisition -run 'TestServiceGeneration\|TestWorkflowRecovery\|TestLegacyRecovery\|TestRAConfig' -count=1 -timeout=4m` | 通过，2.850s / 2.426s / 0.362s | 未知 acquisition schema 拒绝且原事实不改；首次收紧后 unit fixture 缺冻结 config，修复 fixture 后通过 |

最终构建 `verified/steerrunner.exe` SHA=`3d91c6b336bde2588096ab3f26d65c2af89d67c7f1cc21de15bfb7c129f8b933`；在上述 A 后只简化 owner 内存句柄索引、补缺 checkpoint 诊断和保留业务阻断 phase。该精确构建 `verified-smoke` 重验通过，exit 0，147.626s：离线审计、parser 显式 stop、正常 A/B 接管三项通过。application 获取/恢复定向集 `go test ./internal/application -run 'TestRA|TestServiceGeneration|TestWorkflowRecovery' -count=1 -timeout=6m` 通过，286.095s。其余范围按上表，不声明完整 catalog 或真实原生资格通过。

证据入口：各受控目录保留 integration.log、caller-commands.jsonl、native-calls、result.json；`remaining-source-manifest.json` 记录两仓基线和本轮源码 hash，`verified/capabilities.json` 属于最终精确构建。current/ 等构建目录本轮曾复用，历史 EXE 不保证还在原路径，历史身份以该轮登记的 SHA 与报告为准，不把旧 capabilities/manifest 改成新构建。

Orca 外部阻塞：2026-10-19 实际 CLI **1.4.224**，`orca worktree create --help` 与 `orca terminal create --help` 不提供调用方持久原创建请求键/重试或按原请求找回创建结果的入口。只读核查不等于证明底层不存在此能力；需要上游公开具体调用和跨连接持久语义后才能接线和验证。未创建试验资源、未重发 create、未凭名称/列表认领。该路径继续 unknown/unverified；原生接口缺口不阻止本机和 SteerLine 工作，但整体 A04/原生资格保持未完成。

最后补齐冻结材料资格：`material-qualified/steerrunner.exe` SHA=`03bcc58120b391ee15da3ec1e57c344c30236a0bb138d2e468d435db092876b6` 的 `material-qualified-smoke` exit 0，166.974s，离线审计/parser 强杀/正常 A/B 三项通过。`go test ./internal/application ./internal/host -run 'TestRecoveryFrozen|TestRA|TestServiceGeneration|TestWorkflowRecovery|TestLegacyRecovery' -count=1 -timeout=6m` 通过，268.593s / 2.575s；含真实原材料文件、替换/缺失拒绝及零修复反例。审计 unit fixture 原用虚构 Spec/Plan 路径，已改为真实冻结字节/文件，未放宽生产校验。

收尾发现兼容比较仍含 State.Prompt 的默认文本，会与“保留原提示词、同契约新默认可更新”相冲突；已只排除默认 Prompt 文本，kind/contract/转移/模型角色/权限/预算和成功标记仍精确比较，保留原 frozen States/PromptBodies。相应 `TestWorkflowRecovery` 加入旧默认文本差异，连同冻结材料和 generation 定向测试通过，2.648s。最终 EXE `final/steerrunner.exe` SHA=`1ca788946183b7e29927c5f1ac25d57c952a224b5ea9450bddfbda9d90e8d36a` 的 A/B 精确构建重验 `final-compatible` exit 0，40.499s，命中正常 A/B 接管，原 owner/native 存活、离线结果补收、原 thread=1/turn=3、显式 stopped 和收尾通过。本轮其它已验/未验边界不变；两仓文档链接、diff 空白与部署脚本语法检查通过。最终能力文件为 `final/capabilities.json`，最终源码与结果索引为 `final-source-manifest.json` / `final-run-summary.json`。

## PR #35 主干合并复验（2026-10-20）

从 `b0e66b284e8f2ab3c5dbfe294ba7ace8f9989bd0` 合入 `origin/main` 的 `a259c3ad4723c09f6fc5d3b1cf40b6a4cf028092`，解决六个文件的文本冲突并核对自动合并边界。保留主干 conversation/1、generate/1、run-evidence 查询及 gh-only 认证退役边界；不恢复已移除的 GitHub App 配置。当前支持的内容流程沿同一 Run、原执行范围和 owner 恢复，不新增队列/授权状态机。

已接受但未冻结的 message 绑定原执行范围，换代后核验并只冻结一次；已冻结后的第二次重启不重建输入。旧 epoch 或错会话仍阻断。内容流程没有原 WorkspacePlan 时不采 Git checkpoint，结果仍沿原 native journal；开发/审计的签名 checkpoint 保持。独立 owner 补齐主干 interrupt 所用 stop_observation 分发；未知 interrupt 沿原结果只读对账，不再次发送。

| 命令 / 本地证据目录 | 结果 | 证据边界 |
| --- | --- | --- |
| `go test ./internal/application -run '^TestContent|TestServiceGeneration|TestWorkflowRecovery|TestRecoveryFrozen' -count=1 -timeout=3m`；`merge-content-rules.log` | 通过，4.033s | 内容原消息跨代/仅冻结一次、旧 scope 拒绝、内容 workflow 只读兼容、原冻结材料反例 |
| `go test ./internal/... -count=1 -timeout=12m`；`merge-internal.log` | 全部通过，application 407.790s | 合并后共同回归；启动该组后又补无 Git checkpoint 和 interrupt 白名单，最后两项另按下列全包/受控重验，不能把本行称作最终源码全量重新通过 |
| 首次 `merge-controlled` | 编译失败，未启动受控场景 | 新增取证断言误用不存在的 ExecutionView.Process；改读原 process.json 并用 OS 身份核验存活，不改公共 DTO |
| `merge-controlled-recheck`，六个顶层场景 | exit 1 | 全部 11 个获取子场景及 R2 取证通过；会话和 unknown-interrupt 暴露 owner 白名单缺 stop_observation，A/B 又暴露独立 harness 当前目录假设。失败保留，不记整组通过 |
| `merge-final-controlled`，五个顶层场景 | exit 1 | 无 Git checkpoint 修订后，离线审计/parser-kill/parser-stop 三项通过（158.40s）、生成十分支通过；中断白名单和 A/B 夹具目录故障仍在。此构建 SHA=`4f150f5ac6b983ef601240976ef728aee90c8cf571ac16cca5329d73d623873d` |
| `go test ./internal/execution/adapters/codex -count=1 -timeout=3m`；`merge-interrupt-codex.log` | 通过，48.223s | 最后生产源码，含无 Git 完成依赖测试及全部原 owner/绑定/签名/停止等单元规则 |
| `merge-interrupt-recheck`，四个顶层场景 | exit 1；三个内容顶层均通过 | Generate 十分支 65.02s、Conversation 10.07s、QuestionAndUnknown 24.20s（含丢 interrupt 回执、原问题、未知 start 零重发）；原进程回执/存活及原 session 保留、无需重启 resume。唯一剩余失败是 A/B helper 固定相对目录，下面单独修复重验 |
| `merge-candidate-recheck`；`scripts/test-foundation.ps1 -Pattern '^TestSRRestartPreservesAuthorization$'` | exit 0，112.09s，命中 1 个顶层/4 个子场景 | 修复 test helper 同时支持 go test 的 tests/integration cwd 和独立 harness 的仓库根 cwd。graceful、kill、creation_receipt、pending_stop 全部通过；无生产恢复逻辑变更 |
| `go test -tags live -c -o <merge-interrupt-live-compile.test.exe> ./tests/integration` | 编译通过 | 只验证已同步的真实 driver 可编译，没有运行真实 Codex |

以上目录均在 `.evidence/service-generation-recovery/`，保留原 integration.log、caller-commands、source-identity 和 result.json。最终生产 A SHA=`bb155e89240f19f784e244704ab447cc96cf9551a2ceb158b795a7d62aea2939`，在 `merge-interrupt-recheck` 与 `merge-candidate-recheck` 两轮构建逐字节相同；A/B 的 B SHA=`d40e8e9bb25e4b81e2d91508bd9827730c0259817ee8311102e38f402ee632c9`。最终源码/夹具/脚本身份登记为 `merge-interrupt-source-manifest.json`；此前 checkpoint 阶段的 `merge-final-source-manifest.json` 保留原内容。构建发生于合并未提交工作树，SourceCommit 为原分支基线且 dirty=true，不能将后续提交号假写成实测包 metadata，也不承诺提交后任意重建字节相同。

未运行最终源码的完整 Controlled catalog，不把失败组拼成“全部通过”。真实 Codex/认证/工具树、SteerLine 同现场 UpdateRunner、Orca 原创建查询的外部缺口仍按原记录待验。SteerLine PR #360 源码本轮未改；此次未部署或合并 PR。
