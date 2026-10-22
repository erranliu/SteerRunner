# Run token 用量与独立费用估算实施记录

原验收日期：2026-10-21；提交前一致性复审：2026-10-22。原基线为 `8261310d5f2d1edb6fa0c81ba05fa4119b561386` 加当时工作区增量；本轮已整合 main `4d11148`，新结果单列于末节，不把历史构建结果改写为新构建资格。平台 Windows/amd64、Go 1.26.4。对应 [Spec](../spec/run-usage-and-cost.spec.md) / [Plan](../spec/run-usage-and-cost.plan.md)。本记录区分已交付代码、受控验证和生产现场资格。

## 已交付

基础契约位于 `internal/usage/api`；基础归一化不依赖费用模块。`internal/cost` 只导入这个公开契约，按同一用量 revision 读汇总及事件页，用有理数逐响应计价、汇总后输出八位小数金额字符串。没有内置价格、在线价格抓取、API 凭据或原生 Pi cost 的生产依赖。价表坏、缺失或关闭只影响估价。

SQLite 增加独立事件账本、不可变快照/事件页和私有 checkpoint/授权 journal，不改变 RunRequest、业务 Run revision 或原 Record 编解码版本。事件、裁定和源 checkpoint 在同一事务提交；观察后原业务 revision 已变化时拒绝提交旧观察。历史页保留当时的 token/元数据裁定。暂不自动过期明细与去重身份，因而不会通过清空账本重新计费；没有宣称已交付可配置保留期限。

去重键为主机身份、账户/source namespace、agent、native session 和 response identity。路径、runtime、generation 不进入响应身份。token 冲突隔离；同响应的模型/时间冲突保留 token，费用报告元数据冲突。缺数使用 null，不用零掩盖不可得。费用只在完整、finalized、available 且所有响应均可计价时给总额；其余保留覆盖金额或 null。

宿主在独立 worker 中观察原绑定与生命周期。查询只读已提交快照，既不自动开服务，也不补采、登记目录、发输入或等待远端。已收敛的历史不会因为采集关闭或日志随后删除而降级。初始化源、旧 attempts 和当前 generation 都保留在预期范围内。

## 公开调用

```powershell
steerrunner run-usage --state-dir C:\runner-state --run-id R --json
steerrunner run-usage --state-dir C:\runner-state --run-id R --snapshot-revision 7 --json
steerrunner run-usage-events --state-dir C:\runner-state --run-id R --snapshot-revision 7 --limit 200 --json
steerrunner estimate-run-cost --state-dir C:\runner-state --run-id R --usage-revision 7 --pricing-policy C:\policies\prices.json --json
```

`run-usage-events` 的后续页使用原 revision 和 `next_cursor`。不允许将游标挪到其它 Run/revision。Run 不存在、快照不存在/过期或游标不匹配均退出 5；已存在 Run 的用量/费用不可得属于成功查询，退出 0 并附状态与原因。服务离线保持原退出 4。

价格策略 schema 随 `capabilities.run_pricing_policy_schema` 发布。`--pricing-policy` 是服务所在本机的绝对 JSON 文件路径，最多 1 MiB；价格策略有独立 ID/version/source/currency，UTC 有效区间及模型/provider 匹配规则。按时间段计费需将日历冻结成明确的有效区间；默认拒绝把完成时间当请求时间，只有 `allow_completion_time: true` 才使用并披露该假设。当前不把 requested 模型/tier 当作 observed 元数据。

以下仅为算术示例，**不是供应商价格**：

```json
{
  "schema": "run-pricing-policy/v1",
  "id": "example-only", "version": "1", "source": "synthetic arithmetic example",
  "currency": "USD", "enabled": true, "allow_completion_time": false,
  "rules": [{
    "provider": "fixture-provider", "model": "fixture-model",
    "effective_from": "2026-10-21T00:00:00Z",
    "effective_until": "2026-10-22T00:00:00Z",
    "rates": {
      "input_uncached_per_million": "2", "cache_read_per_million": "0.2",
      "cache_write_per_million": "3", "output_per_million": "4"
    }
  }]
}
```

## 授权与撤销

service start 仍接收裸对象 `pending_stops`，可额外提供 `usage_policy`。默认开启本机采集，远端目录授权默认关闭：

```json
{
  "pending_stops": [],
  "usage_policy": {
    "enabled": true,
    "remote_log_access": {"mode": "disabled", "grants": []}
  }
}
```

启用远端时，使用 `mode: "temporary"`，每个 grant 提供 `environment`、`host_id`、`session_roots`。host_id 为既有配对公钥原始字节的 SHA-256，格式 `sha256:<64位十六进制>`；不能用 IP、名称或 runtime 替代。目录必须为明确的 Codex 日期叶目录、Pi 的 Run/generation sessions 目录或 DSH 单 native session 目录。禁止 home、账户认证根、通配符及路径穿越；源身份和路径逐级链接元数据未验证时不登记。现阶段远端 Pi/DSH 的生产 gate 会先拒绝其绑定，不因配置 grant 放行执行。

有效策略、digest 与实例 revision 可通过 `service status` 获取。已有同构建服务省略策略会保留原策略；显式不一致退出 5 `usage_policy_conflict`。更改使用原 stop/start；stop 被接收后停止本机采集许可，原远端业务仍沿既有 service-stop 契约运行。

每次自建登记先持久化意图，记录精确 group/folder ID、实例、epoch、消费者及策略摘要。登记不建业务终端、不读取 API 凭据、不上传程序。释放先进入 releasing，阻止新 acquire；旧 epoch 不能删除新登记。创建/删除响应未知时只读对账，禁止盲目重放；Orca 没有已证实的远端 operation-settlement 接口，仍未结算的请求保持 pending，不能从空列表推导已撤销。

回收需要原主机认证、原 ID/名称/目录一致、没有其它资源或消费者、终端检查通过；删除后还要证实 ID 消失、原 selector 返回明确 `selector_not_found`、自建 group 为空且消失。断线、认证失败、文件 ENOENT、外部复用或不能证明删除安全时保留 pending，不关闭任何业务终端。停服及重连执行独立有界清理；没有远端 TTL 或离线即时回收保证。folder 登记是通用目录授权，**不是只读 ACL**，也不能原子 compare-and-delete；其它客户端仍可能有自己独立的访问路径。

## 实际验证

| 验证 | 本次结果 |
| --- | --- |
| 三种脱敏格式、Codex 累计字段排除、Pi 缓存语义、DSH 串接 zstd frames/嵌套重复 | 通过，夹具及来源摘要在 `internal/usage/parsers/testdata/manifest.json` |
| 不可变汇总/事件页、冲突隔离、跨 Run 归属冲突、事务回滚、业务 revision 不变 | 通过，`internal/storage/usage_test.go` |
| 授权默认值、非法目录、共享引用、旧 epoch、未知创建/删除、离线/外部复用和明确撤权错误码 | 受控测试通过；不是远端现场资格 |
| 独立费用精度、同版本读取、active/partial/缺数据、坏价表/关闭、元数据冲突和时间假设 | 通过，冻结算术策略；未使用实时供应商价格 |
| 真实 EXE、独立 state-dir/服务、三个公开命令、错误退出码、同/省略/冲突策略 | 通过，`TestRunUsagePublicExecutable`；合成投影，无模型/Orca 外部操作 |
| 本机旧生产 Pi 日志，经新绑定、采集器与存储复算 | 通过：18 响应，200,432 token，available/finalized；旧状态库只读，新投影仅进入临时库 |
| 本机旧生产 DSH 日志，经新绑定、采集器与存储复算 | 通过：21 响应，318,722 token，available/finalized；reasoning 保持 null |
| 本机 Codex 完整生产计量路径 | 后续已补齐主会话现场，含原 Run 停止、公开 EXE 计量/费用、服务停启和强制重读去重；见本机增补 |
| 新构建远端 Codex 自动登记/读取/回收的真实生产接口 | 本轮补充通过，原 Run 11 响应/225,453 token；详见后续现场记录 |
| 远端 Pi/DSH 生产自动统计 | 当前不支持生产 Run 绑定，继续 `PRODUCTION_BINDING_UNSUPPORTED`；已有原版 CLI 格式证据不升格为生产支持 |

本机真实日志验证修复了一个格式细节：DSH 的 `source.kind=runtime-context` 记录使用 `user/message`，它不重置原 command 区间。只有真实输入、其全文摘要和回执边界参与归属。该反例已有脱敏回归。

已执行入口：

```powershell
go test ./internal/usage/... ./internal/cost/... ./internal/storage ./internal/contracts ./internal/resources ./internal/host ./internal/cli ./internal/execution/adapters/protocol/orca -count=1
go test ./internal/cli -run TestRunUsagePublicExecutable -count=1 -v
# SR_USAGE_EXISTING_STATE 只在明确指定的旧状态目录做只读验证；不是生产开关。
go test ./internal/usage/collect -run TestUsageExistingNativeEvidence -count=1 -v
```

`TestUsageExistingNativeEvidence` 默认跳过。其本次指定的原日志与 2026-10-21 已有调查一致，无新增模型输入、远端程序、临时凭据或远端目录配置。

首次 `go test ./...` 未通过：整仓 application 测试揭示本次新初始化接线对手工无 context Engine 的兼容问题，已修复且原停止用例重跑通过；旧构建的 CLI 集成测试还揭示缺少临时目录和新增错误码未映射，均已修复并由真实 EXE 重跑通过。整仓 integration 到达默认 10 分钟上限，不能记通过。最终受影响包/受控回归结果在后续记录补充，不以定向成功覆盖这次失败历史。

## 尚未取得的资格

本文已列出的 U/C 定向场景有实现证据；完整设计矩阵尚未全部实现或取得资格，不将 Plan 的 T01–T17 全组或 G01 四种生产路径全部标通过。fork/seeded 会话、没有原始绑定的旧日志、未知客户端格式和未关联子会话均保守报告不可得/缺口，不推测总账完整。没有新的 fork 谱系接入或零调用证明生产源。目录被外部客户端并发复用、终端列表不完整或 RPC 无法证明结算时仍需保留 pending；当前 Orca 的 `id:folder:<id>` 终端选择器已在后续现场确认。

zstd 使用固定的 Go 依赖 `github.com/klauspost/compress v1.18.6`，流式串接帧、本机解压，限制源 64 MiB、解压 256 MiB、记录 4 MiB、远端 chunk 512 KiB；没有 Python/Node/zstd 外部解压运行依赖。依赖接口依据[该版本 Go 文档](https://pkg.go.dev/github.com/klauspost/compress@v1.18.6/zstd)。

## 最终回归补充

- 受影响的新用量/费用、storage、contracts、resources、host、CLI、Orca protocol 与 Orca transport 包测试通过；包含真实 EXE 查询和原有 Orca transport 执行 gate 回归。
- `go test ./internal/application -count=1 -timeout 15m` 通过（465.375 秒），包含已修复的旧停止用例。
- `go test ./tests/integration -run 'TestSR(InputFreeze|StorageLogsEvents|NativeUnknownAndStop|RestartPreservesAuthorization)$' -count=1 -timeout 15m -v` 通过（211.605 秒）；重启包含 graceful、kill、creation_receipt、pending_stop 四子场景。
- `go vet ./internal/usage/... ./internal/cost/... ./internal/storage ./internal/host ./internal/cli` 及 `git diff --check` 通过。全量 `go test ./...` 的默认超时历史保留，未宣称整仓全绿。
- 后续补充反例通过：旧状态投影保留 stop_status/created_at、采集许可/跨主机 folder 重定向检查、同身份空元数据补齐、响应 ID 跨 command 重用隔离、账户目录移动不改变 Orca account namespace，以及远端 Pi/DSH 绑定的公开状态契约。

首次实施批次的新增可执行文件仅由测试创建在独立临时目录，测试服务正常关闭；该批次没有新增远端登记。后续现场登记及撤销记录见下文。各批次均没有部署或更换用户生产实例，没有新增凭据，工作区变更尚未提交/推送。

超时现场清理：首次整仓集成超时遗留的独立临时 Run 已通过原 `stop` 请求结算，再停止原测试服务；`service verify-stopped` 返回 `resources_settled: true`，复核没有剩余临时 Runner/native-fixture 进程。未删除或停止用户生产资源。

## 远端 Codex 现场增补（2026-10-21）

使用已完成 Run `71215281655ecccdc4c5be1b6b91d62d`，原生会话 `01a15c3d-d905-7ef1-9299-562123990385`，既有 environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`、Orca 1.4.223。源数据库用 `mode=ro` 打开；只将原 Profile/Snapshot/Attempts/启动证据复制到独立 state-dir，没有业务授权或意图。真实新构建 EXE 在该隔离目录启动服务，原生产服务、终端与日志保持原状；没有新模型调用、SSH、远端 shell、程序、helper 或凭据。

实际链路：默认关闭策略返回 `unavailable / REMOTE_LOG_ACCESS_NOT_CONFIGURED`，不创建授权意图；明确策略只允许既有账户 `sessions/2026/10/21`。采集器核验主机公钥、session/cwd、逐级链接元数据后，自建 group/folder，分块读取 212,211 字节，逐 response_id 解析并自动撤销。公开 `run-usage`、四页 `run-usage-events --limit 3` 和 `estimate-run-cost` 均通过；完整重复 tick 不生成新事件或快照，同 revision 查询不变，副本业务 revision 保持 1。独立逐行控制和核对了全部 11 个 response ID，未使用 AI Vault 累计数作为计数权威。

| 字段 | 实测 |
| --- | ---: |
| 响应数 | 11，其中初始化 1、三个业务 command 合计 10 |
| input_total | 221,376 |
| input_uncached | 25,536 |
| cache_read / cache_write | 195,840 / 0 |
| output（含 reasoning） | 4,077 |
| reasoning | 1,895 |
| total | 225,453 |
| status / finalized / cleanup | available / true / revoked |

原始字节 SHA-256：`72bb61fdcbdd8393be28f2bf253fca855c3507ed6b4273a7db2d73655183f38c`。现场独立控制和读取副本已删除，仅保留本机计量/绑定摘要、公开 JSON、临时 journal 与算术价表作为证据。生产采集器没有持久化原文。

费用验证使用明确标记的合成价格（input uncached 1、cache read 0.1、cache write 0、output 2 USD/百万 token），并显式允许 completion time：11 个响应均可计价，`0.05327400 USD`。这只是算术回归，**不是供应商现价或实际扣费**。基础 token 不依赖该价表。

本轮现场发现并修复：

- 终端查询要求 `id:folder:<id>`；文件接口仍用 `folder:<id>`。原选择器被拒绝时保守保留 pending，未盲删。修复后通过原 journal 和同一安全检查撤销该登记；后续全自动路径均直接 revoked。终端证明还要求完整、未截断、无遗漏主机的零终端列表。
- 历史无 wire_format 的 Codex TUI 会裁掉尾部换行。只作已验 CRLF/末尾换行规范化，不忽略空格或其它差异；同时核对原输入摘要和原生时间区间。
- 初始化 READY 独立为 `phase=initialization`、空 business command ID；只有原启动摘要、受理/确认、字面参数语法、Run 创建时间和首条业务输入原生基线吻合才归属。不能识别的包装不放宽。
- 目录增加 linkedItems/diffComments 等外部引用时拒绝读取和删除；旧 epoch、未知创建/删除仍使用原保守规则。
- 原生产服务会独立刷新 PR 并推进 revision。现场对照保留全部源绑定/输入事实，单独记录这种 revision 变化；不将其误判为计量写入。只读源库和副本业务 revision 均有独立断言。
- 末次观察预算耗尽与来源完整性分离：缺口在预算耗尽后仍 `coverage.finalized=false`；新许可或新实例可以再观察原来源，已完整取得的历史则无需重读。新增授权后补采、同实例预算停止和换代恢复反例均通过。

`TestUsageRemoteCodexLive` 最终 6.50 秒通过；`TestRunUsageRemotePublicLive` 最终 19.09 秒通过，结束前再通过 stop/start 将隔离服务策略设为 `enabled=false`、远端 mode disabled/grants 空，确认历史查询一致，然后停服。最终公开路径 journal：epoch 1 / revoked，group `b7415ff9-3371-4df4-bfb1-97d89fa2f2f6`、folder `85f8405b-5276-40c7-8106-e03fa070a1c3` 均消失，旧 selector 明确 `selector_not_found`。远端 group/folder 列表回到原空基线。

收尾另查原业务 terminal `term_58de5ec5-e06e-4de5-980b-24e806f4f394`：仍 connected，runtime、worktree 和 incarnation `d9d5b826-5b0f-4006-b4bc-f05669ebe3ad` 与现场前一致。本机 Pi/DSH 原日志在最终关联修复后重跑，仍分别 available/finalized、18 响应/200,432 token 和 21 响应/318,722 token。

三个现场测试默认 skip。显式入口要求 `SR_USAGE_LIVE_STATE`（已有完成 Run 的只读来源）、`SR_USAGE_LIVE_RUN`、全新 `SR_USAGE_LIVE_EVIDENCE`；公开 EXE 测试另要求 `SR_USAGE_LIVE_PUBLIC=1`。`TestUsageRemoteCodexCleanupLive` 只接受明确的原隔离 journal `SR_USAGE_LIVE_CLEANUP_STATE`，不创建新资源。标准 L1 脚本拒绝这些远端 opt-in，防止环境变量遗留触发现场操作。

此次只补齐远端 Codex 的上述场景；本机 Codex、fork/子会话、其它启动包装、完整恢复竞态和远端 Pi/DSH 生产依赖仍按原边界登记，不据此宣称六组合全部通过。

## 整仓超时复核与最终清理

`go test ./... -count=1 -timeout 45m -json` 最终退出 0：29 个 package 通过，记录 1,763 条测试 pass（包含父/子测试，不能当作独立 case 数），没有 fail。application 全包 625.267 秒；integration 全包 2015.891 秒，四个重启子场景全部通过。

本次超时归因：Go 默认 10 分钟限制的是整个 package。`TestSRResultAcquisition` 的 11 个串行子场景合计 738.65 秒，已经超过该上限；裸 `./...` 还包含 L4 的 20 轮资源压力场景（393.01 秒）。压力场景最终句柄数 290、基线 235、增量 55，小于冻结限制 128。各子场景持续结算，没有放宽 CLI、原生准备或场景内超时，也没有减少轮数。此前 nil context 导致的锁死修复已由 application 整包验证，不能与这次聚合预算不足混为一谈。

全量批次在本轮现场修复前启动；现场修复后的 usage/cost/storage/contracts/resources/host/CLI/Orca protocol/transport 包已另行完整重跑通过，最终真实 EXE 现场也再次通过。`go vet ./...`、PowerShell 脚本语法检查与 `git diff --check` 均通过。全量中的 10 个 skip 是 helper 入口、显式原生探针或旧版本 EXE 未提供等条件项；其中 `TestSROperationV2Compatibility` 未提供 baseline，不能记为跨版本通过。新现场测试与既有 Pi/DSH 复算均单独 opt-in 执行，不用默认 skip 替代资格。

最终公开验收 EXE SHA-256：`8b0e37682c248d21dd1ebe73eb215ecdf5a8b3000ea19f29469b261bf03f4141`。本机证据目录为 `C:\Users\lmas1\AppData\Local\Temp\sr-usage-acceptance-20261021-144755`，包含逐项测试 JSONL、最终公开输出、独立控制和、撤回策略、源文件摘要及清理摘要；没有进入仓库。所有 9 份现场授权 journal 均 revoked，无未知创建/删除；临时测试进程复核为 0，原始日志读取副本已删除。未提交、推送或部署任何变更。

## 实现与 Spec/Plan 一致性复审（2026-10-21）

本次直接修复实现中的不一致，并同步 Spec/Plan；没有放宽原执行 gate，没有修改 Orca、部署 helper 或改动生产实例。包依赖实际检查：`internal/usage/...` 不依赖 cost；cost 在仓库内部只依赖 `usage/api`。公开 capability 分为 `run_usage` 和 `run_cost`，基础节点不再混入价表说明。

| 发现 | 修复与回归边界 |
| --- | --- |
| 没有明确等待事实也可能 pending；核心 token 缺失可被当完整 | 首次投影为 unavailable/COLLECTION_NOT_OBSERVED；只有实际正常等待才 pending。缺核心计数保留可信部分并 partial，已知字段仍校验缓存关系/溢出；不可取得不填零 |
| 元数据冲突保留任意旧值，重读可能掩盖冲突 | 冲突字段清为未知，裁定保持，token 仍计入；不同 session/response/command/phase 的同键矛盾隔离 |
| 新输入、回执、profile 变化可命中旧 finalized 缓存；旧观察预算被继承 | 纳入输入摘要并重开窗口；缺口未收敛仍 finalized=false。结束等待耗尽后 unavailable/partial，不永久 pending |
| 本机身份依赖名称/路径，checkpoint 解码失败可按空记录处理 | Windows MachineGuid/SID 摘要；首次来源绑定摘要随 checkpoint 持久化。变更或损坏时拒绝重绑，保留原证据和已计 token |
| stop 与迟到观察提交之间缺少持久屏障 | stop 回执与许可撤销同事务，入账事务重验 active/instance/digest；读取及新创建前核验。撤销不阻止原资源清理，旧实例不能撤销新实例许可 |
| 创建/删除已成功返回，但取消会丢失已知结果 | 最多 5 秒独立本机写入保存结果，取消/撤回后不启动下一次创建；真正丢失响应仍保持未知且不重发 |
| group 删除可能级联子 group/repo，重连写入也可能覆盖新 epoch | 检查所有引用，folder 删除后再次检查；拒绝不完整清单。仅在 epoch/instance 检查后采用重连 runtime，不在锁外覆盖 lease |
| 全局清理状态污染其他 Run，共享与新 epoch 丢失历史归属 | 按该 epoch 关联 Run 集合及撤销证明聚合；pending 优先，共享保留 active/ACCESS_STILL_SHARED，新 epoch 不抹去旧撤销。只有取得实际登记后才发布 active |
| DSH 时间戳未解析、seq 只检查上界 | v4 毫秒时间解析为 completion，下界必须晚于原输入；Codex 新 turn 清除旧 ownership/model，parser 版本推进至 native-usage/2 |
| 缺 tier/context 可选通用价格，unknown 时间可套完成时间开关 | 报 PRICING_CONTEXT_UNKNOWN/PRICE_PERIOD_UNKNOWN；只有明确 completion 才可使用显式代理假设，费用缺口不删除 token |

六组合仍按以下边界交付，不将 reader/parser 证据扩大为生产自动统计资格：

| 目标 | Agent | 实现支持 | 本构建证据/限制 |
| --- | --- | --- | --- |
| 本机 | Codex | collection、生产绑定已实现 | 后续本机主会话现场通过，原 Run/原日志、公开 EXE、费用和停止重启去重见下文 |
| 本机 | Pi | collection、生产绑定已实现 | 公开 EXE、自动关联、逐响应控制和、独立估价及采集服务停启/重读通过；18 响应、200,432 token，见本机 Pi/DSH 增补 |
| 本机 | DSH | collection、生产绑定已实现 | 公开 EXE、自动关联、逐响应控制和、独立估价及采集服务停启/重读通过；21 响应、318,722 token，见本机 Pi/DSH 增补 |
| 远端 | Codex | collection、生产绑定已实现 | 真实 EXE、原 Run 只读证据、自动授权/读取/统计/撤销再次通过 |
| 远端 | Pi | parser/file_transport，生产绑定未支持 | collection=false、production_run_binding=false，明确 PRODUCTION_BINDING_UNSUPPORTED |
| 远端 | DSH | parser/file_transport，生产绑定未支持 | 同上；不以旧调查样本或本机扩展冒充生产接入 |

fork/子会话完整谱系、零调用证明生产源及明细过期/tombstone 清理机制仍未交付；当前没有自动过期删除计量身份的路径。未验格式、无归属或未知谱系只给不可得/缺口。现有 Orca 没有原子条件删除、未知 RPC 结算证明或远端 TTL，仍保留相应 pending 限制。

本轮定向包回归通过，包含 usage 五包、cost、storage、resources、host、CLI、contracts、Orca protocol；Orca transport 整包另行通过。新增测试名称与 P1–P5 映射见 [Plan](../spec/run-usage-and-cost.plan.md)。应用层整包 `go test ./internal/application -count=1 -timeout=20m` 通过，542.749 秒；停止、冻结、日志与 graceful/kill/creation_receipt/pending_stop 重启专项集成通过，226.330 秒。应用/集成批次与后续计量细节修复有重叠执行，最终计量/存储/宿主/接口包另行完整回归，不能把此前整仓批次说成在最终源码上重跑了全部测试。

新现场证据位于本机 `C:\Users\lmas1\AppData\Local\Temp\sr-usage-audit-20261021-154615`。使用同一已完成 Codex Run，未追加任何模型输入；公开接口仍为 11 响应、225,453 token、available/finalized，合成价格仍为 0.05327400 USD（仅算术验证）。每次现场独立目录都执行 stop/start 撤回为 enabled=false、远端 disabled/grants 空；最后停服后额外断言 collection permit inactive。group/folder 回到原基线，原 selector 明确失效，原业务 terminal/incarnation 保持 connected。凭据未使用、未回显，也没有上传任何新远端程序。

最终源码的受影响 12 包回归全部通过，证据为 `affected-final.jsonl`；Orca transport 整包另行通过（51.359 秒）。最终公开现场测试 16.22 秒通过，group `ff680166-1118-4196-8c8f-4f97628d2c2d`、folder `34d56c4d-ed0d-4d0f-af34-eb01070e5eb6` 均已撤销；最终 EXE SHA-256 为 `3a6c3b8508e567201385cb8e70747788d1caa9aaf91bc9cfbe2591eabd5c8268`。四份本轮隔离 state-dir 再以只读数据库连接核对，均为单一 lease/epoch 1/revoked，无 CreateUnresolved/MutationUnresolved，collection permit inactive。最终验收 EXE 进程数为 0，`go vet ./...`、`git diff --check` 通过；HEAD 仍为 `8261310d5f2d1edb6fa0c81ba05fa4119b561386`，没有暂存或提交。

## 本机 Codex 完整现场增补（2026-10-21）

先检查已有生产状态库、临时现场及历史验收记录。现存完整本机 Codex Run `770c2c38beb04bd2f3ee92c567f27d0a` 属于 codex-app-server，未拿它替代本期 Orca 路径；旧 Orca Codex 的历史通过记录已不保留可用的原状态库。没有从日志、UI 标题或 PR 反向伪造 Run。

依照“优先复用”的本轮授权，通过真实 Runner EXE `service start/submit` 创建一个最小 orca-terminal/local/codex Run `85ac625c0c1f6b6b043985cadcc334b9`，native session `01a15db7-b18e-74e1-b815-efe52ca131d5`。实际沿已有本机正式运行时/原版 Codex TUI 执行，复用安装中的 `C:\SteerlineInstall\bin\steerrunner-agent.exe`；没有安装、复制、修改或部署 helper。前后文件 SHA-256 均为 `e7fe84f3aa28afaf24b20a2408e00a495ef6388511a7ca8c17c362c1062ec852`。Orca 源码、安装与全局设置未修改。

本机现有 GitHub 执行契约要求原运行时准备，因此没有绕过该 gate 使用无准备的终端。临时本地 seed Git fixture 未用于实际模型执行，其项目登记已撤销。正式 Run 使用已注册的测试仓库，冻结的最小指令要求不使用工具、不改文件、不提交、不 push/发布，只回复 needs_input。实际完成一个业务回合并进入 waiting_input，然后通过原 stop 正常结算至 stopped/revision 20；原服务随后正常停止，`service verify-stopped` 返回 resources_settled=true。没有创建 PR；本工作区没有提交。

计量验收的后续各轮只复用上述原 Run 和原日志，无新增模型输入。源库只读打开；隔离服务副本保留完整冻结身份及已撤销的业务授权，不改原输入、业务 revision 或日志。生产采集器只接收原 Profile/Snapshot，通过原 session/cwd、原 turn、完整 wire 摘要自动找日志；测试预检所找路径没有传给采集器。

| 字段 | 独立原始记录控制和 / 公开接口 |
| --- | ---: |
| 响应数 | 2：初始化 1、业务 command 1 |
| input_total | 39,583 |
| input_uncached | 31,263 |
| cache_read / cache_write | 8,320 / 0 |
| output（含 reasoning） | 84 |
| reasoning | 35 |
| total | 39,667 |
| status / finalized / cleanup | available / true / not_required |
| snapshot revision | 2 |

原文件为 137,129 字节，SHA-256 `0d184075bcff2c9db2cf7e702e0b3b5694292fff68051cff65d57938d6a999cd`。独立控制和直接逐行读取 token_usage_record.usage，按 response_id 去重；未调用生产 parser/ledger/费用函数，也未累加 event_msg 中的累计 token。公开事件 ID、原 response ID 和每条 token 载荷逐项吻合，原生日志重读哈希不变。初始化公开 phase=initialization、command_id 为空；业务事件保持原 command/turn。

独立费用模块采用合成 USD/百万 token 价格：input_uncached=1、cache_read=0.1、cache_write=0、output=2，明确允许 completion time 代理。2 响应均可计价，总额 `0.03226300 USD`；独立整数算术为 `(31,263×10 + 8,320 + 84×20) / 10,000,000`，没有重复计 reasoning。这是接口及算术验收，**不是供应商价格或实际扣费**。缺失价表及 enabled=false 价表分别返回费用 unavailable，基础用量快照/明细保持一致。

停止重启去重验证均通过公开 EXE：

1. 正常停服后以采集仍开启的新实例重启，后台再次经过采集轮次；同 Run/token/事件 ID、revision 2 和历史事件页不变。
2. 再停服，仅在已停止的隔离副本 checkpoint 中清除 Finalized 缓存，保留 binding、前缀/水位及账本。重启后核对 checkpoint 的新 instance 和 Finalized=true，证明实际重读了原日志；事件数、token、revision 和固定价表估算均不变。此项是明确的受控重读故障窗口，未冒充新业务输入或修改原 Run。
3. 关闭采集后再次启动，只读历史仍不降级；最终全部服务停止，permit inactive，整个本机路径 lease 数始终为 0，没有新增目录授权。

本机现场发现并修复的三项实际适配问题：

- AI Vault 本机条目缺少 codexHome：只向已核验的本机默认/显式/Orca 账户根补全，未知目录继续拒绝；增加 SID namespace 与路径反例，远端不使用该回退。
- 现有正式运行时的固定 READY 原先不在直接 PowerShell 初始化规则内：增加原启动摘要、配置、runtime SHA、已核验准备回执、session/workspace 及时间边界联合证明，任意包装/摘要变化/未验证回执仍拒绝。
- Codex thread/read 的 user item ID 与 rollout ID 不同：原 native turn 与完整 wire digest 联合核对，禁止把两种 ID 直接比较或模糊转换；摘要矛盾及 workspace 变化反例通过。

初次测试失败保留在现场日志中。除上述产品问题，还修正了测试目录创建、停止 Run 副本必须保留完整冻结身份，以及长时间闲置的基线 socket 需对同一 runtime 重新认证；没有降低原事件/金额或清理断言。原生产 Run、其它业务工作区和原安装运行时不受这些失败影响。

`TestRunUsageLocalCodexPublicLive` 最终两次通过：24.48 秒、21.12 秒；后一轮在业务工作树已经清理后再次复用原日志。最终 EXE SHA-256 `842fd4914d5db1b80cfa325adf01d0c4c332de0c182a66704f1d024e0a24abbb`。本机构建涉及的 usage 五包、cost、storage、host、CLI、contracts、resources、Orca transport/protocol 共 13 包完整回归通过；`go vet ./...`、PowerShell 测试入口语法与 `git diff --check` 通过。本轮没有再次声称整仓所有长时集成测试已重跑。

本机证据目录 `C:\srt\usage-local-27ae790d`，保留正常 CLI、脱敏 token/事件、价表、原始文件摘要及错误现场；没有保存原始日志副本或 API 凭据。原 Run 工作树已用精确 ID 正常移除，目录确实消失；未使用 seed 的单一 repo 登记已通过现有 repo.rm RPC 撤销并确认其它项目 ID/path 不变。所有 9 份有许可行的原/验收状态均 inactive，lease=0；验收 Runner 进程为 0，原 owner/native PID 均已退出。本机原生日志仍按用户原保留策略存在，便于只读复算。远端 Pi/DSH gate、fork/子会话与未验包装等限制继续保留。


## 本机 Pi、DSH 完整计量现场增补（2026-10-21）

直接复用 `C:\slr\beret3eg7x3tdkzggckgqkuhmy` 中两个 pr_ready 真实 Run。没有创建新 Run、追加模型输入、使用 API 凭据或启动/部署 helper；原生产服务、业务终端和原生日志保持原状，Orca 源码、安装与设置未改。本轮只增补测试及文档，没有修改生产代码或暂存、提交、推送。

调查时历史 Run 含当时严格业务 codec 不识别的字段；验收不依赖历史业务请求能否重新解码，以 `OpenReader/UsageRuns` 只读提取原 Profile、Snapshot、Attempts、LaunchCommand、CreatedAt 等已提交证据；没有迁移历史请求。保留原 Run ID 的隔离副本不包含原业务授权、待执行 intent 或请求，业务 revision 固定为 1。真实服务后台采集器仅从原冻结事实自动关联日志；独立预检获得的路径、token 和 response ID 均未传给采集器。此资格覆盖现存已结算主会话的公开计量、费用及**隔离采集服务**停止/重启，没有重启原 Pi/DSH 业务，也不代表全部 G01/G02 恢复窗口。

| 项目 | Pi | DSH |
| --- | --- | --- |
| 原 Run | `691dc23163d59c560b734458a1bff3d5` | `6dd9e9102f2d197bc419f08881d098c2` |
| 原生 session | `dd82d24b-60d8-4f7c-4e29-33a96fbe4114` | `ae44dbaa-ee74-4f78-a227-5694f306f869` |
| 格式 | Pi v3 JSONL | DSH v4 串接 zstd 帧 |
| command / 响应数 | 2 / 18（12 + 6） | 2 / 21（6 + 15） |
| input_total | 195,504 | 299,810 |
| input_uncached | 15,024 | 11,810 |
| cache_read / cache_write | 180,480 / 0 | 288,000 / 0 |
| output（含 reasoning） | 4,928 | 18,912 |
| reasoning | 955 | null（原日志未提供） |
| total | 200,432 | 318,722 |
| 独立合成价表估算 USD | 0.04292800 | 0.07843400 |
| status / scope / finalized | available / main_session / true | available / main_session / true |
| snapshot revision / 来源数 / 缺失响应数 | 2 / 1 / 0 | 2 / 1 / 0 |

关联联合核对原配置的 Run/attempt/generation、已接受回执的完整 wire SHA-256、原 input/reply ID。Pi 在冻结 StateDirectory 下找唯一 session 文件，核对 header/cwd，沿 message.parentId 归属响应。DSH 以原 TerminalReadiness.NativeSessionID 定位日志，不能误用 Runner session `b9fa7528-d5c1-9fe8-ba9b-754cef353ffb`；核对 header/cwd 及 input seq < assistant seq <= command end seq。runtime-context user/message 不作为新业务输入；嵌套及流式 usage 不重复计量。

独立控制程序直接读取原文件、解压 DSH、逐行提取 assistant 最终 usage，不调用生产 parser、ledger、归一化或费用函数。核对所有原始输入完整摘要/ID，拒绝无归属响应；逐响应对照公开事件的 Run、agent、attempt、generation、session、command、response 和 token；原 reply ID 必须属于对应 command。未知 reasoning 不补零，不累加原生日志 cost，不重复计 reasoning。

Pi 原文件 72,625 字节，SHA-256 `1024076e06f9d521610629d2ebf86c327443c84ec70b7269a09d638152eced87`；DSH 原压缩文件 128,057 字节，SHA-256 `474a3aee1839f5ac3d66291d72c91b300303419aaea7ab695790ed855dd9e22d`。两组两轮测试前后均未变化。首次 Pi 预检发现测试控制程序把 usage 中非 token 对象按整数读取，已改为仅解码指定 token 字段；未修改产品解析或降低断言。

费用接口固定 usage revision 2，独立加载显式合成价表：USD/百万 token 的 uncached=1、cache_read=0.1、cache_write=0、output=2，明确允许 completion time 代理。所有响应均有 observed provider/model 且可计价。独立整数计算分别为 `(15,024×10 + 180,480 + 4,928×20)/10,000,000` 和 `(11,810×10 + 288,000 + 18,912×20)/10,000,000`。**数额只验证接口和算术，非供应商报价或实际账单**；未更新或内置供应商价格。缺失价表和费用 enabled=false 分别返回 unavailable，原用量快照和事件均不变。

两组均使用实际构建 EXE 的 `service start/stop/status`、`run-usage`、固定版本且每页 2 条的 `run-usage-events`、`estimate-run-cost` 完成以下验证：

1. 正常停服后，以采集仍开启的新实例启动并经过后台采集轮次；快照、revision、事件 ID、逐条 token 和数量完全相同。
2. 再停服，仅清除隔离 checkpoint 的 Finalized 观察缓存，保留 binding、prefix、水位及账本。新实例 checkpoint.Instance 与新服务一致且 Finalized=true，证明重新读取原来源；快照、全部分页和费用仍完全相同。这是受控重读窗口，未改原业务 Run 或日志。
3. 以采集关闭策略再启动，历史查询保持完整；最后全部隔离服务正常停止，持久 permit inactive。

本机 Pi/DSH 直接读取当前账号下的冻结日志目录，无需新增 Orca folder/group 登记。两组 cleanup=not_required、lease=0；Orca 全部 group/folder 清单前后严格相同。不能据此宣称远端免授权或具备服务器 TTL。原业务证据及 revision 前后相同（Pi 821、DSH 814），隔离副本 revision=1、无授权/待执行 intent；原终端 handle/incarnation/connected 另行复核，不关闭或替换用户终端。

最终现场入口 `TestRunUsageLocalPiPublicLive`、`TestRunUsageLocalDSHPublicLive` 分别 18.95 秒、18.70 秒通过；共用服务/分页/费用/停启断言，原文件控制程序独立。入口默认 skip，显式设置 `SR_USAGE_LOCAL_LIVE=1`、`SR_USAGE_LOCAL_STATE`、`SR_USAGE_LOCAL_RUN`、全新绝对 `SR_USAGE_LOCAL_EVIDENCE`，并以精确 `-run` 选择对应 agent。标准 L1 入口已经拒绝这组遗留 opt-in。

最终证据目录为 `C:\srt\usage-local-pi-20261021-final`、`C:\srt\usage-local-dsh-20261021-final`，保存脱敏控制和、定位证据、公开事件/用量/费用、测试价表与停启证明；不保存原日志副本或凭据。同一验收代码复用旧 Codex 原日志再次通过（21.55 秒，2 响应/39,667 token），证据在 `C:\srt\usage-local-codex-20261021-pi-dsh-regression`。相关 usage 五包、cost、storage、host、CLI、contracts、resources、Orca transport/protocol 共 13 包完整回归通过；`go vet ./...`、测试脚本语法通过。本轮未再次运行整仓长时集成；远端 Pi/DSH gate、fork/子会话、零调用证明和保留期限制继续不变。


收尾只读核验覆盖本轮 5 个原/最终隔离状态目录：全部 collection permit inactive、lease=0，验收 Runner 进程数为 0；原 Pi/DSH terminal handle/incarnation 未变且仍 connected。两组最终 EXE SHA-256 均为 `842fd4914d5db1b80cfa325adf01d0c4c332de0c182a66704f1d024e0a24abbb`，与本轮前 Codex 现场构建相同，印证本轮无生产代码变化。附加清理证明为 `C:\srt\usage-local-pi-dsh-final-storage-proof-20261021.json`、`C:\srt\usage-local-pi-dsh-final-native-proof-20261021.json`；13 包回归 JSONL 为 `C:\srt\usage-local-pi-dsh-regression-20261021.jsonl`。`git diff --check` 通过，HEAD 仍为 `8261310d5f2d1edb6fa0c81ba05fa4119b561386`，无暂存或提交。


## 提交前实现、Spec、Plan 与现场记录一致性复审（2026-10-22）

本轮已授权修复、回归、提交和创建 PR，未授权合并或部署。仍不修改 Orca、不部署 helper、不改变原生产服务或追加模型输入。旧调查表、已取得主会话资格、未交付能力分别登记；Spec 增加当前六组合矩阵，Plan 中“先不提交”等历史限制已按新授权更新。文档索引纳入本专项，保留此前失败、未执行和未交付记录。

| 确认的问题 | 修复和回归 |
| --- | --- |
| Pi/DSH 只确认最终 reply ID 在会话中存在，交换两个 command 的 reply 仍可能误报完整 | 最终 response 必须归属原 command；已验证 token 保留为 partial、finalized=false。`TestUsageFinalReplyBelongsToCommand` 两个 agent 均先失败后通过，parser 版本更新为 native-usage/3，原收敛缓存重新核验 |
| token/归属冲突隔离时丢失新观察的来源摘要 | 私有 event-conflict journal 与隔离/快照同事务保存原有及新观察的规范化事件；重放按忽略 observed_at 的证据键幂等，原始日志不入库。`TestUsageConflictEvidenceIsAtomicAndDurable` 验证失败全回滚、双方摘要保留、重放不增快照或证据、历史页不改写 |
| 同实例的新租用 epoch 复用旧 marker，传入 revoked lease 时旧 ID 残留于新创建意图 | marker 包含 epoch；新 epoch 清空 group/folder ID 和未结算标记。`TestUsageLeaseReferencesAndEpoch` 验证新意图、共享释放及旧 epoch 拒绝删除新登记 |
| group/folder/repo 仅识别 truncated，其他明确不完整信号可能被忽略 | 与 session 一致拒绝 hasMore、nextCursor、totalCount 矛盾；完整清单正例保留。`TestUsageIncompleteInventoriesFailClosed` 覆盖四种清单，未知不当撤权证明 |
| 某些输入分类为 null 时，只校验 cacheRead 单项，未覆盖已知分类合计越界 | 对已知 uncached/cacheRead/cacheWrite 合计作下界和溢出检查；任一未知仍保留 null。`TestUsagePartialFieldsStillValidateKnownRelations` 增加单项、已知合计及部分字段溢出反例；最后重新执行相关 13 包及四组合现场 |
| 费用只有 api_equivalent 字段，未在结果明确列出排除收费范围 | assumptions 固定说明 token-only、排除订阅/credits 结算及工具/外部 API 费用，并保留 completion-time 代理假设；`TestCostTokenOnlyScopeAndCompletionAreExplicit` 通过。没有增加基础用量费用字段或供应商现价 |
| 价表可选档位/上下文条件显式 null 被 Go 当作省略，与 schema 不一致 | loader 拒绝三个可选条件的显式 null；`TestCostPolicyOptionalNullCannotErasePredicates` 覆盖 null、省略和显式零，价表错误仅影响费用 |
| 新 main 的完整实例绑定/停止准入需要覆盖新增查询 | 三个查询加入 ServiceBindingCommands，保留 RWMutex 的 handler 准入围栏和 observe 行为；公开 EXE 正例携带完整绑定，原全字段失配/停止/非 ready 反例自动覆盖三查询，`TestSRFullBindingCLI` 增加三查询正例 |

准备 PR 时发现 main 已从原基线前进 14 个提交，整合到 `4d11148` 后人工解决五处重叠：CLI 参数/observe、host 绑定字段、RWMutex 及查询 dispatch、capability 命令集和测试说明。保留上游工作树名称前缀、原子 observe、材料语义与文档归档；新计量不替换这些行为。

整合前的 13 包通过、application 532.657 秒、四项集成 168.772 秒及三个本机复验，只记录为整合前结果。后续全部 internal、专项集成与四组合现场在整合后的固定代码上另行执行；实际结果及收尾证明续记如下，不能拿整合前结果替代。


归一化修复批次（价表 null 校验修复前）的四组合现场再次通过，使用完全相同的 EXE（SHA-256 `3900ea65973d35c181033848a585d499ef4e57081231b035012e62be1d3d146d`）：

| 原 Run 组合 | 响应 / token | 合成估价 USD | 最终测试耗时 / 快照版本 |
| --- | --- | --- | --- |
| 本机 Pi | 18 / 200,432 | 0.04292800 | 21.40 秒 / 2 |
| 本机 DSH | 21 / 318,722 | 0.07843400 | 21.02 秒 / 2 |
| 本机 Codex | 2 / 39,667 | 0.03226300 | 26.71 秒 / 2 |
| 远端 Codex | 11 / 225,453 | 0.05327400 | 17.18 秒 / 4 |

所有费用仍为显式合成价表的 API 等价 token 算术验证，非实际账单。DSH reasoning 保持 null。三个本机现场覆盖正常停启、强制实际重读、关闭后历史及独立费用一致；远端再次覆盖自动临时授权、原日志读取、固定版本费用及撤回策略后的历史。最后一个远端 group `8a6c83a7-c04e-41fa-8d48-1739713603c1`、folder `849d245c-fad6-4072-8602-b203bae1bb1c` 均已验证撤销；前一轮远端登记也已撤销。原独立服务的业务 revision 2744→2745 为 PR 状态观察推进，原身份/输入/日志计量不变，未把 revision 本身当串账或新调用。

本轮累计 11 个隔离 state-dir 另经只读 SQLite 复核：permit 全部 inactive、全部已有 lease/epoch 为 revoked，无 CreateUnresolved/MutationUnresolved，所有副本业务 revision=1。自建 Runner 进程为 0；原 Pi、DSH 和远端 Codex terminal handle/incarnation 保持不变且 connected。本机未新增目录登记，远端 group/folder 恢复原清单，同一 selector 返回明确撤销错误；不称为文件全局不可访问或远端 TTL。

最后补齐 L1 对 `SR_USAGE_EXISTING_STATE`、远端 Run/evidence 等遗留变量的拒绝。以仅设置 `SR_USAGE_EXISTING_STATE=guard-test-only` 的子进程实际运行 L1 入口，确认在日志读取/Go 测试启动前报 `Unexpected live opt-in`，作为预期拒绝通过；没有把这次负例说成完整 L1 执行成功。证据为 `C:\srt\usage-final-audit-l1-guard-20261022.log`。

最终代码另重跑 13 个受影响包全部通过（`usage-final-audit-final-affected-20261022.jsonl`），包含完整绑定 EXE 正例和所有新增反例。`go vet ./...`、测试脚本语法、相对链接与凭据模式检查及 `git diff --check` 通过。源码清单摘要为 `c7c5b80aee8a07fa82eedf2c0d56e58b71f7a8e8e298e89ef5e441928ac3a55b`（按 git 跟踪 Go、go.mod/go.sum、内嵌资源文件的 SHA-256 映射生成），与 EXE 摘要分别保存，不用提交元数据变化冒充源码变化。

现场目录为 `C:\srt\usage-final-audit-{pi,dsh,codex,remote-codex}-final-20261022`；本轮汇总收尾证明为 `C:\srt\usage-final-audit-storage-proof-20261022.json`、`usage-final-audit-native-proof-20261022.json`，源码清单为 `usage-final-audit-source-manifest-final-20261022.json`。不提交原始日志、凭据、原状态库或测试二进制。


整合 main 后 `go test ./internal/... -count=1 -timeout=25m` 的 28 个包全部通过，其中 application 450.360 秒；四项专项 `TestSRInputFreeze`、`TestSRStorageLogsEvents`、`TestSRNativeUnknownAndStop`、`TestSRResourceCreationRecovery` 共 186.080 秒通过。该整包批次在最后的 nullable 输入分类校验和价表显式 null 修复前编译；这些后续修复的 13 个受影响包及四组合现场另行重跑，不冒充最终源码上重跑了整个长时测试全集。

价表加载与公开 schema 的末次核对发现，Go 会将可选 tier/min_context_tokens/max_context_tokens 的显式 null 当作省略，可能消除计价条件；现按 schema 拒绝该价表，不将 null 变成通用条件。`TestCostPolicyOptionalNullCannotErasePredicates` 覆盖三个 null 反例、省略和显式零正例，价格加载错误仍只影响费用。基础 DTO/采集没有引入价格依赖。


代码冻结后的提交资格批次（包含价表 null 修复）已完成：最终 13 包全部通过，四组合公开 EXE 现场也再次通过，Pi/DSH/本机 Codex/远端 Codex 分别 22.66、22.59、30.85、16.75 秒。响应数、token、快照版本与合成金额均保持上表值。四份 EXE SHA-256 均为 `b6326d8aad44189e534519d0260c4cd9839db1920e1fd3ca8ae1a70cf4d7443f`；最终源码摘要 `4c5dcd9733a95df47fb7b4b04659db87b5c9f7efe351c4d65822c7fcb470d008`。

最终远端 group `c0d60d04-1e04-4795-9985-e8609f91eca2`、folder `e7b3db09-c7c0-4af9-be9e-c27f8ce31f94` 已撤销。累计 15 个本轮隔离状态均 permit inactive，全部既有 lease/epoch revoked、无未结算创建/删除；副本业务 revision 均为 1。再次复核原 Pi/DSH/远端 Codex terminal incarnation 不变且 connected，自建 Runner 进程为 0。原客户端安装没有重新部署，其版本未在本轮重新观测；本轮资格基于原冻结 Run 身份/回执和已验格式、来源摘要，不泛化到所有客户端版本/包装/恢复窗口。

提交可审阅的[脱敏机器摘要](evidence/run-usage-and-cost-20261022/final-summary.json)记录上述事实及残余边界；私有现场目录改用 `C:\srt\usage-final-audit-{pi,dsh,codex,remote-codex}-qualified-20261022`。最终受影响包日志为 `usage-final-audit-final-affected-policy-20261022.jsonl`，收尾/源码证明分别为 `usage-final-audit-storage-proof-qualified-20261022.json`、`usage-final-audit-native-proof-qualified-20261022.json`、`usage-final-audit-source-manifest-qualified-20261022.json`。当前二进制离线 capability 实测恰为六行、四组合 collection/production_run_binding=true；cost 依赖检查只含 usage/api，usage 传递依赖不含 cost。
