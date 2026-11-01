# 本机 Orca Codex 停止条件收敛：审计、实施与验收

> 后续再审计已发现并修复初轮未覆盖的业务证据门禁、迟到创建、预算和 live 测试消费者。本文保留初轮构建及原始通过/失败事实，最终实现和 PR 验证以[再审计报告](orca-local-codex-stop-reaudit.md)为准。

日期：2026-11-01。结论：按[Spec](../spec/orca-local-codex-stop-simplification.spec.md) / [Plan](../spec/orca-local-codex-stop-simplification.plan.md)完成专项实现与最小验收。未部署、提交、合并或发布，未改外部 SteerLine。长期承诺已更新为现行；本报告不授予完整 L1/Controlled、L2/L3、全 Agent 或任务交付资格。

## 审计后的实际取舍

仅本机 Orca 受管 Codex 使用 `orca_local_codex_resources`：核实原业务终端、控制终端、helper 和 app-server 根身份。签名退出文件与 control Closed/pending reader 响应不再决定停止成功；显式 stop、ShutdownLifecycle 核验及离线 verify-stopped 使用同一读取判据。离线即使已保存 stopped 也查询原资源，不把门禁转移到停服或升级。实际 helper 停止请求、原 Job 取消/清理、精确身份、输入撤销、审计和结果来源保持。

停止成功不再证明所有后代写进程退出；部分清理中断可保持 unknown，交本人处理。保留 15 秒总预算，helper 协作最多占 2 秒；每个原终端首次最多一次 close，之后只读。不同 request_id 不能替换未解决的原 stop。普通 service stop 仍只脱离观察，不默认终止 Run。

审计确认并修正了以下问题：

| 发现 | 实施处理 |
| --- | --- |
| 缺停止回执不能证明动作未调用；原 application 路径会再次 StopLifecycle | 区分本轮新建意图和已持久化意图；丢失回执或服务重启只调用 Reconcile。用真实 SQLite 提交失败与重载验证 |
| CloseReadonlyTerminal 内有 close 重试，不能照搬 | 新路径只复用精确身份核验与单次 CloseTerminalChecked；响应丢失只查，不重发 |
| ExitPublicKey 同时定位原 helper 停止请求 | 保留请求上传与共享签名 writer/key，不删除 helper 真实终止能力；只移除 scoped stop 的验签门禁 |
| control journal 仍承载正常 history/Git/审计查询 | 保留 query/response/reader 来源；删除失去消费者的 closeControl、settleControlQuery、ControlClosure、ReadControlClosureEvidence 及 Closing/Closed 字段，以内存 stopping fence 禁止新查询 |
| 远端旧 profile 版本不能获得当前 scope | 保留 NativeRemoteProfileVersion 检查；首次组件回归捕获并修复误扩范围，完整 adapter 重跑通过 |
| PowerShell 5.1 原客户端无 BOM，按系统编码解析失败 | 基线 23 个 parser 错误；补 UTF-8 BOM 后 0 个，客户端合同通过，正文与派发逻辑不变 |

直接 Codex、Orca 原生远端、Pi/DSH、独立 parser、正常业务及审计采纳标准保持。共享签名机制的消费者为直接 Codex owner/checkpoint 和原 helper，请勿把本次范围收敛理解为全局删除签名或控制通道。

## 版本和构建绑定

state/v8、SQLite user_version=8，默认 `%LOCALAPPDATA%/SteerRunner-v8`；新版本非破坏拒绝 v7 及更旧目录，不回填 scope、不解释旧停止回执。原版本先排空原生资源及未完成操作/控制，停止 writer 后再启用新目录。保存的客户端 state_dir/实例绑定不自动搬迁；外部严格解码客户端需识别可选 `stop_evidence_scope`，外仓未联验。

| 项目 | 实际来源 |
| --- | --- |
| 源码 | HEAD `8bed4c436835d6fef690024969947909e8f7ccd4` 加本次工作副本，`source_dirty=true` |
| Runner | `0.2.0-dev.4`，windows/amd64；SHA256 `140228658e9a73b8cf86fdf1f477d5a64c362e27d07eb4084ede0a466a6641c8` |
| 最终源码摘要 | `5d69697d4dc2d5d159e3df68bbc2a771c9aa57b094de77585885929654562f87`，由正式 build.ps1 生成；含最终测试夹具修订 |
| 原生 helper | 独立构建 `cmd/steerrunner-agent`；SHA256 `2831f8b04af45fe57c14120c4a88c4eb562f880d6496ad02567911e03e498607` |
| 实际工具 | 已安装 Orca EXE；本机 Codex `22cfb1d302613eae/codex.exe`，模型 `gpt-6.1-sol`，沿用户已有登录 |
| 环境 | Go 1.26.4，Windows/amd64，当前用户；Job/IPC 测试使用正常用户权限，未更改系统设置 |

正式构建在测试夹具补正后重建，Runner SHA 始终一致；最终构建与真实探针执行的是相同字节。源码摘要随测试源码变化，未追称此前测试运行于最终干净提交。记录见[构建](evidence/orca-local-codex-stop-20261101/runner-build.json)、[工具摘要](evidence/orca-local-codex-stop-20261101/tool-identities.json)及[源码增量](evidence/orca-local-codex-stop-20261101/source-delta.zip)。

## 最小验收与实际结果

以下计数为顶层测试，子用例不重复计数；重跑不累加为独立覆盖。实际测试名、skip 和原始失败均见[机器汇总](evidence/orca-local-codex-stop-20261101/test-summary.json)。

| 验证 | 最终结果 | 证据 |
| --- | --- | --- |
| 完整 orcatransport 包 | 159 通过、3 个既有显式 helper/live skip；92.998 秒 | transport-final.jsonl.gz |
| protocol/orca、直接 codex、storage、contracts、resources、host 完整包 | 分别 41/89/18/16/18/11 通过；共 5 个既有显式 helper/live skip | components-user-01.jsonl.gz 中这六包；同文件 adapter 失败不能算通过 |
| application 定向授权/事务/重启/停止/离线 | 26 通过；16.760 秒 | coordinator-user-01.jsonl.gz |
| scoped stop 最终提交丢失、原意图重启、新 ID 拒绝、其他 Run 隔离 | 4 通过；0.875 秒，前两项与早期定向部分重复 | stop-transaction-final.jsonl.gz |
| 最终证明文件反例定向 | 7 通过；1.254 秒；含实际损坏退出文件与 journal、pending、活动根和错身份 | proof-final.jsonl.gz |
| reliable / operation 完整包 | 14 / 8 通过 | reliable-final.jsonl.gz；早期 coordinator 选择器在这两包零匹配，不记覆盖 |
| 实际 Windows Job 后代终止 | TestOwnedJobReconnectAndStopIncludesStartedDescendant 通过 | coordinator-user-01.jsonl.gz |
| 正式 foundation 公开受控 | 3/3：ServiceOwnership、NativeUnknownAndStop、RestartPreservesAuthorization，含 graceful/kill/creation_receipt/pending_stop | controlled-result.json、controlled-integration.log、caller/native 调用记录 |
| 客户端合同 | result.passed 全项通过，controlled=true、live_execution=false | client-result.json、client-encoding.log |
| 真实本机 Orca Codex 停止探针 | 1/1；52.516 秒；业务输入 0，原资源清理完成 | native-05.jsonl.gz、native-05-result.json |

与 Spec 的对应关系：

| ID | 已证事实与边界 |
| --- | --- |
| A00 | Spec/Plan/长期承诺、当前 v8 操作说明与索引同步；旧 v7 证据不重释 |
| A01 | 缺失/损坏的退出文件和 control journal、pending 查询不阻塞 scoped stop；helper/根仍活不能成功。Stop 不设置 WritersAbsent；新 control 查询被阻止 |
| A02 | PID 复用、错 incarnation、清单不完整、缺 readiness、未知创建拒绝；原完整清单和初始化/迟到创建回归保留；真实 Job 子进程退出通过 |
| A03 | 原操作/停止绑定事务失败零派发、stop 与输入竞争不续投；另一个 Run 的原资源活动状态及持久记录不变。跨 Run 资源为受控夹具，不冒充两个真实模型并行 |
| A04 | helper 请求与 close 响应丢失仍按原资源结算；受控断言请求一次、control close 一次，后续 Reconcile 零新增变更调用；部分仍活保持 unknown；原 stop 回执提交丢失重载后 stops=1、reconciles=1 |
| A05 | 真实活动现场 resources_settled=false；公开 stop 后三入口按同一 scope 成功，删证明文件后仍成功；离线前后 DB/原 shutdown 字节一致、无 endpoint 或隐式服务启动；writer 锁与缺现场回归通过 |
| A06 | v1–v7 的 Open/OpenReader 均 unsupported_schema、原库字节不变、无迁移文件；新库为 v8；公开 inspect 与离线投影同 scope；相邻后端组件回归通过，未重跑全 Agent 真机矩阵 |
| A07 | 下述真实隔离探针完成，真实 readiness 与精确原资源而非伪造进程结果；无完整业务流程或 PR |
| A08 | 下述两类依赖归零及净变化明确；未宣称时间加速 |

关键测试源码：[scoped adapter 断言](../../internal/execution/adapters/orcatransport/local_codex_stop_test.go)、[提交/重启与跨 Run](../../internal/application/local_codex_stop_test.go)、[真实探针](../../internal/host/orca_native_test.go)。

## 真实探针的范围

使用本次拥有的隔离 worktree、业务/control 终端和真实 helper，固定测试仓库基线 `91371fda325f04ce18592da67412a7f8af98ce52`。Codex 仅执行已有 READY bootstrap，不派发任务正文。Run 是当前格式的注入夹具，绑定实际创建回执、真实 readiness、PID/创建时间和终端 incarnation；不通过公开 submit/material/业务/PR 全链路。

同一现场依次证明：

1. 原资源活动时，公开离线核验 `resources_settled=false`；ShutdownLifecycle 也不能把普通观察脱离认作停止。
2. 启动真实 Runner 服务，经公开 inspect/stop 得到新 scope 和 stopped；service stop 受理后等待 writer 真正退出。
3. 离线核验原业务/control 终端和 helper/app-server 根均结束，`resources_settled=true`。
4. 仅删除本次隔离现场的退出文件和 query journal，再次离线核验仍为 true；前后 DB 和 shutdown 回执逐字节一致，无服务启动。两次核验没有增加模型会话。
5. 测试拥有的终端关闭与 worktree 删除完成；后续新鲜 worktree 清单确认五轮探针的全部自有 worktree 均缺席，见[清理确认](evidence/orca-local-codex-stop-20261101/cleanup-confirmation.json)。未操作用户活动 Run。

原始公开结果：[活动未结算](evidence/orca-local-codex-stop-20261101/native-05-active-rejected.json)、[停止后核验](evidence/orca-local-codex-stop-20261101/native-05-stopped-verified.json)、[无证明文件核验](evidence/orca-local-codex-stop-20261101/native-05-stopped-without-proof-files.json)。实际 native 工具后代并非本探针随机生成，Job 子进程终止由独立真实进程夹具证明；本报告不把根身份缺席推导为全部后代消失。

## 收益和保留成本

| 新范围的停止/结算依赖 | 实施前 | 实施后 |
| --- | --- | --- |
| 签名退出文件读取/验签作为成功前置 | 必需，缺失/错误拒绝 | 0；相应 verifier/等待包装删除 |
| control Closed 与 pending reader 响应作为成功前置 | 必需 | 0；停止专属关闭字段/消费者删除 |
| 实际 helper 请求及 Job 清理 | 保留 | 保留 |
| 原终端、helper/root 精确身份 | 原证明的一部分 | 当前 scope 的直接依据 |
| 运行期 history/Git/审计 reader 来源、直接 Codex 签名 | 必需 | 保留原规则，不计删除收益 |

实施前运行 `TestCodexStoppedRequiresSignedExitAndFixedControl`，原测试证明缺控制/未结算 reader 或错误退出证明会拒绝；日志为 baseline.jsonl.gz。实施后损坏文件夹具和真实缺文件核验成功；不是基线与新代码的等条件耗时试验。成功路径不再读取这两类证明的结论由源码调用删除和反例共同支持，不对共享 writer 的产出次数作归零宣称。

相对 HEAD 的生产 Go/JSON/client 增加 **259** 行、删除 **244** 行，净 **+15**；其中 orcatransport 增加 213、删除 227，净 **−14**。包含新增 scope 读取、一次终止约束、格式/公开投影/客户端成本，不含测试、文档和 skill 文字。详见[逐文件计数](evidence/orca-local-codex-stop-20261101/production-delta.json)。实际收益是两类规则和外部证明门禁删除，不能宣称全局净删、显著提速或故障率下降。

## 保留的失败与限制

- 初期编译/定向失败包括删除旧函数后的测试消费者、未准备 control/损坏文件父目录等夹具遗漏；修复后重跑，原日志保留。
- sandbox 中 Windows Job 创建和 Orca IPC 被限制；这些失败不作为产品通过。换正常用户权限执行后，真实 Job、组件、受控及原生探针通过，未重启或改装 Orca。
- 首次正常用户组件回归发现远端退役 profile scope 误扩，修复后 adapter 完整重跑 159 通过；同一首次日志中其余六包各自通过有效，不能将失败 package 改判成功。
- native-01 的操作/资源版本夹具过时；native-02 的保存 StateDir 未归一化；native-03 解析 inspect 时漏了 data.run；native-04 未等待 service stop 的异步 writer 退出。四轮保留失败结论并清理，native-05 才取得完整通过。Runner 字节未因这些测试修订改变。
- 本次无真实任务实施、审计 finding、PR、远端模型或 Pi/DSH 真机闭环；不继承旧全量资格。未排空用户旧现场、安装新 EXE、改旧库或联验外仓。

原始证据限定于公开测试输出、固定源码增量和必要身份；私有 endpoint 凭据、数据库及用户现场未归档。文件完整性见 [SHA256 清单](evidence/orca-local-codex-stop-20261101/sha256.json)。
