# Plan：本机 Orca Codex 停止条件收敛

日期：2026-11-01。状态：审计后 P0–P5 已实施及专项验收，未部署或发布。依据 [Spec](orca-local-codex-stop-simplification.spec.md)及[长期产品承诺](steerrunner.spec.md#长期产品承诺)。代码基线 `8bed4c4` 加当前未提交内容，构建明确标记 dirty；实际结果见[验收](../acceptance/orca-local-codex-stop-simplification.md)。下文保留阶段要求，完成证据以末节为准。

## 1. 实施原则及交付范围

只改变 Spec §2.1 的本机 Orca 受管 Codex 路径。范围收缩与实际终止分别核对：删除强制证明等待，保留实际取消、原 Job 和精确资源身份。其他后端、业务结果、审计来源及独立 parser 规则保持。

本方案已新增 `orca_local_codex_resources`，切 state/v8，并冻结该 Run 的范围；不兼容读取旧 v7 来改判旧停止结果。格式切断已在源码实施，用户安装与现场升级未执行。主 CLI v1、现有状态枚举和原停止意图继续使用，不建设清理子任务账本。

按以下顺序在一个专项中推进，避免阶段间部署半套新契约。各阶段可单独记录和提交，但只有全套实现及最小验收满足后，才能声明本方案可用。

## 2. P0：文档与基线（原设计阶段）

- 完成 Spec 的新范围、删除/保留、格式成本、收益和 A00–A08；在长期契约和文档索引增加入口，仍标未实施。
- 核对主要消费者：StopLifecycle/reconcileStop、verifyStopped/ShutdownLifecycle、application/host 结算、helper 停止请求及签名、control 查询/关闭、直接 Codex owner/checkpoint。
- 只做文档一致性、链接和 diff 检查；不构建、不运行模型或停止用户资源。

出口：OLCS-A00。该阶段原为仅设计；后续实施与证据单独登记，不能仅因本 Plan 存在记作完成。

## 3. P1：记录实际消费者、收益和受控反例

1. 在实施工作副本记录 HEAD/dirty 和用户改动。冻结本批路径选择谓词及原 profile/readiness/State 夹具，列出原业务终端、control、helper Owner、app-server Process 的来源。未就绪、创建未知和无 helper 身份场景单列。
2. 盘点退出文件公钥的双重用途：既有停止请求定位仍保留；退出文件验签不再决定新范围结算。保留 remote_exit.go 中仍被直接 Codex、checkpoint 或 helper 请求路由使用的内容，不先删除类型/密钥再补兼容层。
3. 盘点 control journal 的查询用途与关闭用途；只删除本路径 stop 的 Closed/pending 响应门禁，不更改原回合或审计来源采纳。原 query 取消与资源释放仍须执行。
4. 以现有夹具构造“登记资源已结束，但 exit 文件缺失/损坏、Closed 文件缺失或 pending reader 响应未保存”的可重复反例，记录当前结果、文件/查询/动作计数。另构造仍存活 helper/根进程反例，不把缺文件等同于资源结束。
5. 记录停止专属基线规则及行数，同时估算 v8、scope 投影和客户端变更成本。若无法删除 Spec OLCS-05 两类规则，先重审范围，不启动大规模抽象或全部后端重构。

出口：可核对的保留/删除消费者表、基线反例及预期收益；没有性能对照时只承诺规则/依赖减少。

## 4. P2：范围投影与一次格式切断

1. 新 Run 受理时按冻结目标/profile 确定 scope，保存为固定 Run 事实；只有本路径使用 `orca_local_codex_resources`。readiness 或资源尚未出现时也不改变此范围。
2. RunView 增加可选 stop_evidence_scope，同步 inspect、observe.run、history.run 及现有 StoppedRun 投影。沿一个保存来源，不为不同查询拼各自的 scope；只读投影零原生调用。
3. 同步公开 schema/capabilities 和本仓严格解码消费者、客户端/helper 及必要 fixture。复用现有响应结构，不新增停止命令或状态枚举，不将辅助观察字段当业务权限。
4. 切 storage.Format/state/v8、user_version=8 及默认目录，更新本仓配置发现、安装/包说明、fixture 和格式拒绝合同。显式旧 state-dir 不暗改，新版本加载 v7 及更旧目录时在任何 worker/owner/远端调用前拒绝；不迁移、不写旧记录、不保留旧停止 decoder/引擎。
5. 新格式中的其他后端继续原 scope/停止规则；用组件/受控合同证明这不是全后端降低保证。外部 SteerLine 仅登记需识别新目录/字段的集成影响，不修改或声称已经联验外仓。

出口：A06 中格式和投影部分。本次验收后已将当前源码的使用说明切为 v8，历史发行与验收按原版本保留。

## 5. P3：替换本路径停止和核验条件

1. 建立只消费固定身份的范围读取函数，复用原完整终端清单校验和本机进程事实；同一 runtime/worktree 一次清单采样验证业务/control，不跨轮缓存。不得传入 Engine/Store/事务回调建立另一状态机。
2. 首次 stop 保持已有持久操作与 gate：先尝试原 helper 的协作请求，最多等待 2 秒；保留 helper 原 Job 的真实 Kill/Close/清理。然后在同一 15 秒总预算内关闭原业务/control 终端并核实 Spec §3.1 对象。不能因上传或查询文件失败跳过其余仍可定位资源的清理，也不能放开身份检查。
3. 新路径不调用 verifyCodexExit 验签门禁，不以 control pending 查询结算、journal.Closed 或退出文件作为成功前提。control 的关闭沿原身份及既有原生关闭能力，停止意图持久化后不再接受该 Run 的新业务查询派发；运行期正常查询采纳规则保持。
4. 停止可能已调用后 Reconcile 仅查原对象。全部结束才结算，部分仍活或未知则保留原操作、授权撤销和诊断，转现有人工处理路径。该路径已有未解决的可能外发 stop 时，新 request_id 以 operation_unverified 拒绝；同 ID 仍回放原命令。不要增加“再试一次剩余 close”的新恢复策略；尚未形成原生调用的首次意图仍按现有规则处理。现有 CloseReadonlyTerminal 含重复 close 循环，新路径只复用必要的身份核验和单次原生关闭，未知后零重发，不能直接继承该重试循环。
5. 不使用内部独立 30 秒 cleanup context 越过本路径 15 秒预算。保留现行离线读取预算，不叠加新的逐阶段超时放大总期限；取消和排队计入原预算。
6. 将显式 stop、CleanupLifecycle、ShutdownLifecycle 核验和 application/host verify-stopped 接到同一新范围。离线核验仍取得原锁、只读库、拒绝未核实启动，不创建资源、不写文件/DB/历史回执；新 scope 即便保存 stopped 仍核实原资源。
7. 删除只为旧本路径证明服务且无消费者的包装/分支及测试要求。共享 stopProgress、签名 writer、control journal 或 reader 字段如仍有其他消费者则保留并说明，不复制第二套机制换取表面净删。
8. 同步诊断：成功表述范围及未证明全树/未撤销登录；失败区分资源活动、不可达、身份冲突和创建未知。不让 scoped WorkerAbsent 生成全树 WritersAbsent 的强事实，也不据此放行新的业务执行或恢复。

出口：A01–A06 相关受控断言，以及两类强制依赖实际归零；不是仅修改报错或跳过测试。

## 6. P4：最小验证

### 6.1 组件与公开受控

复用当前 `go test <受影响包> -run <精确模式>` 和 `scripts/test-foundation.ps1 -Pattern`，不先跑完整 L1/Controlled 或另建测试调度器。新增测试名在实现时登记，不能让下面的选择器零匹配仍报告成功。

| 验证组 | 目标位置/既有入口 | Spec 断言 |
| --- | --- | --- |
| 范围、清单、进程与真实 Job | orcatransport、protocol/orca、process 的原夹具；针对实际改动选择精确测试 | A01/A02；缺证明文件成功与活动对象拒绝成对验证；Job 子进程确实退出 |
| 授权、事务、未知与重启 | application、reliable/operation 相关测试及一个公开停止受控 case | A03/A04；统计所有 create/send/stop-request/close，重启只有只读调用；保留无关 Run |
| 离线与服务边界 | application/host 及原离线 CLI/服务归属受控 case | A05；写锁/缺库/未核实启动拒绝、scope 一致、DB/命令/回执未变、零隐式启动 |
| 格式、投影与相邻回归 | storage/contracts/CLI、配置 helper/framework 合同，以及共享函数受影响的直接 Codex/远端/Pi/DSH 组件 | A06；旧目录零原生调用，当前后端原停止/审计规则保持，完整查询投影一致 |

每组保存实际选择/执行测试、退出码及断言计数；既有 helper/显式 live skip 不算真实验证。提交失败、stop 竞争及资源退出使用屏障、原回执和精确进程事实，不用固定 sleep 代替因果证明。

### 6.2 一条真实本机停止闭环

复用 [Orca 停止核验](orca-shutdown-verification.plan.md)的现有显式 native probe 结构，调整为本仓独立构建和新 v8 隔离现场，不以外部 SteerLine checkout/部署包为前置，不操作用户原安装或活动 Run。

1. 记录 Runner/helper/Orca/Codex 来源、模型和配置，使用现有登录。创建本次拥有的 worktree、业务/control 终端及原 helper，取得真实 readiness 和原根身份；不伪造 ready 或借用生产 Run，不创建真实 PR/合并。
2. 活动期公开离线核验应拒绝（先按 probe 规则脱离服务，原执行可继续）；必要时按原资源重新接回，证明普通停服不等于停止。记录实际资源身份与状态。
3. 经公开 stop 终止原执行，记录范围、实际关闭和原进程结束。可使用同一 Run 的两次离线查询覆盖健康及缺文件条件，不为每个文件反例重开模型。
4. 仅在本次拥有的资源已经独立核实结束后，在本次隔离目录移走退出凭据及关闭检查点，再运行发布 EXE 的 service verify-stopped；新范围仍应成功。此移除属于测试故障注入，不补写证明、不改生产状态；保留前后文件清单和 DB/命令/原失败回执摘要。
5. 实际工具子进程退出由 6.1 的真实 Job 子进程夹具证明；native probe 至少核实原 helper、app-server 根和终端的实际结束，不能仅断言 API 返回成功。若真机恰有可归属的原工具子进程，补录其退出，但不让模型随机行为成为边界测试前置。
6. 清理只作用于本次拥有的资源；记录清理结果及仍存缺口。scope 不能替代对测试自身已知资源的清理责任，也不为凑成功改写首轮失败。

出口：A07。若真实环境不可用，保留未验状态，不用受控结果冒充真机，不要求此时扩大全后端矩阵。

## 7. P5：收益及文档收口

- 完成 A08：对照 P1 基线列出三个结算入口的验签/Closed/pending 响应依赖是否归零、已删规则、实际读写次数及停止/核验耗时。没有可比计时就只报告确定的依赖删除，不宣称百分比提速。
- 统计所有生产增删及 v8/投影/客户端成本，与停止专属净删分开报告；共享保留项列明消费者。若只是把强制等待迁到离线核验/升级，或新框架抵消规则删除，不把目标记为完成。
- 在当前源码及构建上登记 A01–A08 实际结果、限制和未验路径；不把旧历史验收追认为新 scope 资格。无需为文档整理重跑无关模型矩阵。
- 验收后才更新长期契约待实施状态、当前 CLI/升级说明、测试清单和必要支持声明；保留原停止取证 Plan/证据为旧范围，增加取代说明而不改写历史事实。同步 README/docs 索引的新版本操作指引。
- 若停止动作实际终止能力、审计/输入路径或其他后端机制被迫改变，先记录反例及最小 Spec 修订并补受影响验证；不以本计划扩大为完整执行链重构。

完成定义：A00–A08 满足且真实收益可核对。部署、用户任务排空、旧目录/安装清理、外仓修改、产品合并和发布另行安排；本轮未执行这些操作。

## 8. 实施结果与审计修订

| 阶段 | 实际结果 |
| --- | --- |
| P0/P1 | 保留原设计与旧强证明测试基线；确认签名公钥仍用于 helper 停止请求，control journal 仍服务正常查询，不整体删除共享机制 |
| P2 | state/v8、user_version=8、新默认目录；scope 首次受理冻结，在线/离线公开投影和 capabilities/schema 同步；旧 v7 及更早拒绝 |
| P3 | 删除新路径签名退出和 Closed/pending reader 门禁；实际 helper 请求和 Job 保留；原终端各一次 close，未知后只读；新增请求不得替换 pending stop |
| P3 审计修正 | 原操作缺回执并不能证明未调用，已修复为持久意图重启只读 Reconcile；删除失去消费者的关闭字段/函数，保留正常 query 来源规则；恢复远端旧 profile 版本检查，避免范围误扩 |
| P4 | 受影响组件、事务/竞态/跨 Run 隔离、3 项公开受控、客户端合同均通过；一条真实本机 Orca Codex 探针完成公开 stop、普通停服和两次只读离线核验；首轮失败与受限 sandbox 结果保留 |
| P5 | 两类规则删除；adapter 净减 14 行、全局生产净增 15 行，无可比提速结论；长期文档、操作说明和独立验收记录收口 |

附带必要修复：PowerShell 5.1 按系统编码解析原无 BOM 客户端时有 23 个语法错误；补 UTF-8 BOM 后 0 个，客户端合同全部通过。业务正文与派发逻辑未改。构建和逐项断言、未验范围见[专项验收](../acceptance/orca-local-codex-stop-simplification.md)，不继承完整 L1/L2/L3、全 Agent 或外仓资格。
