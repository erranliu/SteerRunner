# Spec：Run Token 用量能力与独立费用估算

日期：2026-10-21；实现一致性复审更新：2026-10-22。接口、存储、解析、后台采集及独立估价已接入，实际交付和未交付范围见[验收](../acceptance/run-usage-and-cost.md)。计量不修改 Orca/客户端、不部署 helper，自身不新增模型调用；2026-10-21 本机 Codex 为补齐原执行样本，经授权创建的最小真实 Run 与后续只读计量验证分别登记。2026-10-22 已授权完成复审、修复回归并提交 PR。配套 [Plan](run-usage-and-cost.plan.md)，原文档审计见 §11。

2026-10-24 远端 L4 发现 Runner 自身固定 tea PATH 启动包装使初始化识别失配。识别该包装时必须逐字验证完整固定模板，继续核验完整启动摘要，再按原字面调用语法、原 READY 提示摘要及首个业务回合之前的时间范围归属；不以提示子串识别任意脚本。固定进度启动包装（含关闭通道的环境清理包装）也按完整模板识别；开启通道时路径必须属于当前 Run，先核验原完整摘要，再解开进度和 PATH 层，保留旧模板兼容。此修复的真机资格随新构建重新验收，不将此前仅 PR 交付成功的轮次当作用量完整通过。

## 1. 目标与职责边界

基础能力向调用方提供统一的 Run token 用量查询、逐响应用量及明确的不可取得原因；费用估算是独立可关闭的消费模块。覆盖 `orca-terminal × local/orca_environment × codex/pi/dsh` 六种组合的契约与适用性，不把六种契约的存在等同于六种生产执行均已支持。`codex-app-server` 不在本期矩阵内，保留其现有行为，不由本设计暗示新增支持。

| 模块 | 负责 | 不负责 |
| --- | --- | --- |
| 用量能力 | Run/原生会话绑定、日志取得、格式适配、归一化、去重、覆盖范围、获取状态、读取授权及撤销 | 价目表、币种、单价、费用和订阅额度换算 |
| 费用估算 | 消费用量快照，应用独立版本化价格策略，报告金额、假设、覆盖与未计价原因 | 读取日志、登记目录、解析 agent 文件、修复会话关联、控制执行 |
| 原执行链 | 原输入、执行结果、恢复、停止、业务状态与发布核验 | 用费用或 token 数证明业务成功、认证或资源已停止 |

所有解析、zstd 解压、入账和计价在本机 Runner 完成。远端仅使用已有 Orca 原生文件接口及既有原生客户端；不得部署临时/常驻 EXE、脚本、插件或代理作为远端计量 helper，也不得另开终端执行日志读取/统计命令。生产采集不复制或读取模型 API 凭据，不安装客户端或改变其全局配置。

本增量是观察能力，不创建独立业务状态机、调度服务或新的 agent 执行通道；采集与估价失败均不改 Run 的 status/revision、输入授权、重试、恢复、停止或发布规则。

## 2. 实测基线与支持声明

调查基线：Runner `8261310d5f2d1edb6fa0c81ba05fa4119b561386`；2026-10-21 的本机样本与远端 Orca 1.4.223 实测。下表保留实施前调查事实及当时缺口，不是当前交付清单；现状以本节后续矩阵及验收记录为准。

| 组合 | 调查时已取得证据 | 调查时仍需验证或接入 |
| --- | --- | --- |
| 本机 Codex | 原执行链已有原生 session/工作区信息；远端已验证 rollout 计量格式 | 本机生产 Run 完整关联、接口与回归；本轮没有单独复测本机完整计量链路 |
| 远端 Codex | 生产 Run `7f1f2a2a27be3677f22a21002d4046c2`，7 个响应、3 个 turn，130,011 token；日志完整读取和撤权通过 | 自动绑定、持续采集、异常及重启回收 |
| 本机 Pi | 生产 Run `691dc23163d59c560b734458a1bff3d5`，18 个响应、2 个 command，200,432 token；config/readiness/回执与日志相符 | 生产用量接口与自动采集 |
| 远端 Pi | 原版 0.87.1 原生 CLI 样本，session `06b931a7-8f0c-459f-ab7e-c6ede849f463`；85 token；未授权拒绝、授权读取及撤权通过 | 生产原生 Run 绑定；当前执行 gate 拒绝远端 Pi |
| 本机 DSH | 生产 Run `6dd9e9102f2d197bc419f08881d098c2`，21 个响应、2 个 command，318,722 token；原生 v4 日志及区间相符 | 生产用量接口与自动采集 |
| 远端 DSH | 原版 0.2.1-rc.2 headless 样本，session `session-af523243-7ed2-494f-be58-f87a1a4b80ad`；6,321 token；zstd 读取及撤权通过 | 生产原生 Run 绑定；当前执行 gate 拒绝远端 DSH；headless 不替代生产 TUI 验收 |

远端 Codex 原文件 SHA-256 为 `850e6b03b0786f69f0e82e7de594c211e04772bcbaab87dc878b4fe558cbd989`；远端 Pi 为 `1c7ea1dd9e922e520a36ee41a601e399a8d3e7fe311431ca20db425531c33002`；远端 DSH 压缩文件为 `1b21df2469c11980a9a6ca4ae6ecf19709778e30517f0093744edaef71b898f1`。三个远端样本均验证读取前后 stat 一致、两次读取字节一致。Pi/DSH 远端样本均零缓存命中，不代表非零缓存、fork 或子会话远端统计已通过。

首期适用平台为已调查的 Windows/amd64，解析器按已验格式指纹/版本声明，不能泛化到其他平台或所有客户端版本。六组合的 `collection` 与 `production_run_binding` 能力分别声明，均由选定构建实际实现决定，不从 agent 字符串合法性或文件可读性推导。

当前实现和现场资格分开声明：

| 目标 | agent | parser / file_transport | collection / production_run_binding | 已取得的公开接口现场资格 |
| --- | --- | --- | --- | --- |
| local | Codex | true / true | true / true | 原 Run 主会话、2 响应/39,667 token、独立估价、采集服务停启/重读 |
| local | Pi | true / true | true / true | 原 pr_ready Run 主会话、18 响应/200,432 token、独立估价、隔离采集服务停启/重读 |
| local | DSH | true / true | true / true | 原 pr_ready Run 主会话、21 响应/318,722 token、独立估价、隔离采集服务停启/重读 |
| orca_environment | Codex | true / true | true / true | 原 Run 主会话、11 响应/225,453 token、临时授权/读取/撤销、独立估价 |
| orca_environment | Pi | true / true | false / false | 仅格式及传输调查；PRODUCTION_BINDING_UNSUPPORTED，生产执行 gate 未开放 |
| orca_environment | DSH | true / true | false / false | 同上；headless 不作为生产 TUI 资格 |

上述为 Windows/amd64 已验格式下的实现及主会话现场边界，见[验收增补](../acceptance/run-usage-and-cost.md)。原 pr_ready Run 的副本只携带已提交身份和回执，不恢复原业务；采集服务重启不等于原 agent 业务恢复资格。fork/子会话完整谱系、零调用证明生产源、明细过期/tombstone 清理、其他包装和 codex-app-server 仍未交付，不通过 matrix=true 推导这些扩展能力。

公开 capability 将 `run_usage` 与 `run_cost` 分开。`run_usage.matrix` 的 target 与执行契约一致，使用 `local|orca_environment`；每行分别声明 parser、file_transport、collection、production_run_binding 和不支持原因。基础能力节点不携带费用策略，费用节点只说明独立的用量契约、估价口径和价表装配方式。实现支持与真实生产 Run 的现场资格分开登记。

远端 Pi/DSH 在可靠生产绑定未就绪时，能力查询明确报告该依赖未支持；原 submit gate 仍拒绝创建其生产 Run。只有目标 Run 确实存在且属于未支持绑定组合时，用量查询才返回 `unavailable / PRODUCTION_BINDING_UNSUPPORTED`；不存在的 Run 保持原目标不存在错误，不能为了展示状态生成虚假 Run。不得借用本机 helper/回执或自动放宽执行 gate。真实生产执行接入是独立依赖，不在本文新增执行范围；未具备该依赖时只能登记四组合生产能力通过、两组合待依赖，不能以阶段交付声称完整六组合已实现。

## 3. 与既有契约的关系

沿用 [产品/CLI](steerrunner.spec.md)、[独立配置](independent-configuration.spec.md)、[通用 Orca Agent](orca-generic-agent.spec.md)及[远端原生执行](orca-remote-native.spec.md)。本设计仅为用量观察允许本机的版本化日志解析；既有“不扫描 transcript 推断执行完成/完整业务正文”的边界继续适用于执行事实。

不得把用量日志、估价、累计 token、EOF 或目录撤销结果输入原 lifecycle/native proof、可靠执行归一化或业务发布判断。本机现有执行 helper/正式扩展保持原行为；计量模块不新增或修改它们。远端仍零 Runner helper，远端原生执行的停止证明范围保持不变。

既有 Run 请求、profile/state codec、请求摘要及业务事件不因用量观察变更。旧记录不回填虚构绑定或零用量，不自动跨机器扫描历史。只能利用已有可验证绑定作只读补采；证据不足明确不可取得。

## 4. 统一用量查询契约

### 4.1 接口及查询副作用

定义 `RunUsageReader.GetRunUsage(runID, snapshotRevision?)` 与 `ListRunUsageEvents(runID, snapshotRevision, cursor, limit)`。Get 的版本省略时原子取得当前最新版本及全部汇总；显式版本时读取该不可变版本的汇总、覆盖范围、状态及元数据。费用模块只依赖该只读契约；基础能力不导入费用模块或价目表。公开入口拟为：

```text
run-usage --state-dir <absolute> --run-id R [--snapshot-revision V] --json
run-usage-events --state-dir <absolute> --run-id R --snapshot-revision V [--after C] [--limit N] --json
```

两者是 `steerrunner-cli/v1` 的拟议增量，复用现有实例/目录绑定及查询错误规则，不新增 HTTP 服务。只读取已提交的用量投影，不自动启动服务、触发现场读取/目录登记、启动 agent、发送输入或恢复 Run。无服务时沿既有宿主查询错误返回；服务在线但数据不可得时返回本文的领域结果。不存在的 Run、非法游标及调用方权限问题保持协议错误，不包装为某 agent 用量不可得。

成功读取领域结果使用退出码 0、`ok:true`，即使其 status 是 `unavailable`。事件 limit 默认 200、范围 1–500；游标绑定 Run、快照版本和查询范围。汇总和各页必须固定同一版本，不能拿最新汇总拼旧事件。尚未分配的版本返回 `snapshot_not_found`，已知但不再保留的版本返回 `snapshot_expired`；沿既有不存在/过期查询语义使用退出码 5。重取新版本必须是调用方明确的新计算，估价过程不能无声切换版本。快照保留包括所引用事件的历史 token 裁定与元数据；只保存一个总量数字不足以承诺可复算。

### 4.2 汇总形状

`schema=run-usage/v1`。下例只展示已有可信记录的部分统计，不是供应商总账：

```json
{
  "schema": "run-usage/v1",
  "run_id": "R",
  "snapshot_revision": 7,
  "status": "partial",
  "scope": "main_session",
  "tokens": {
    "input_total": 6309,
    "input_uncached": 6309,
    "cache_read": 0,
    "cache_write": 0,
    "output": 12,
    "reasoning": null,
    "total": 6321
  },
  "coverage": {
    "expected_sources": 1,
    "verified_sources": 1,
    "counted_responses": 1,
    "missing_usage_responses": 0,
    "through_event_id": "opaque-event-id",
    "finalized": false,
    "source_summaries": [
      {"source_ref": "opaque-source-ref", "status": "partial", "finalized": false, "watermark": "opaque-watermark"}
    ]
  },
  "issues": [{"category": "usage", "code": "LOG_TAIL_INCOMPLETE", "retryable": true}],
  "access_cleanup_status": "active",
  "observed_at": "2026-10-21T03:46:04Z"
}
```

`tokens` 只代表已核实、已计入的覆盖部分。没有可返回的可信用量时整个 tokens 为 null；某字段不可取得时字段为 null。若某字段在已计入事件中有未知值，该字段的汇总也为 null，不把已知子集伪写成完整字段合计。不得把未知、无日志、未关联或未启用映射为 0。

字段是非负整数；未知字段与确认的零值不同。`input_total` 含所有输入类别；已确认类别互斥时等于 `input_uncached + cache_read + cache_write`，`total = input_total + output`。reasoning 是 output 的细分，不再加总，缺失不推断为零。溢出、负数或不能解释的字段关系拒绝入账并报告 `USAGE_INVALID`。只有确实可证明的零调用范围才返回全零，不能仅因文件没有 usage 记录就断言零消耗。

scope 首期固定为 `main_session`，指该 Run 各 attempt/generation 的已知主会话及初始化范围，不仅是最后一个会话。未来支持 `main_and_linked_children` 时必须明确选择并报告其已知父子覆盖；不能因某个源不可读就将它移出预期范围或退回较小 scope 来报告完整。范围不声称囊括隐藏调用、未落盘请求或账单附加费用。finalized 只说明该快照声明范围已收敛；后续获授权输入可以产生新快照并重新采集，不追改旧快照。

单次末次观察预算耗尽不等于范围已完整收敛。内部可停止该实例/许可下的有界重试，但缺口仍保留 `coverage.finalized=false`；新许可或新实例可以对原缺口重新观察。已完整收敛的可信历史不因此重新打开日志。

结束后的正常绑定/落盘等待耗尽观察预算时，须显式增加 COVERAGE_INCOMPLETE：没有可信用量返回 unavailable，有可信用量返回 partial，不能永久保持 pending。新获授权输入使用新的末次观察窗口，不继承旧输入耗尽的预算；输入受理/结算、原生回执、待投递输入、session/configuration 或当前 profile 变化均须使旧收敛缓存失效。

coverage 还必须给出 `expected_sources`、`verified_sources` 及 `source_summaries`。每个已知启动意图/会话区间都占一个稳定 source_ref，包括未绑定或不可读的源；摘要带 source_ref、源状态、finalized 和不透明水位，issue 可带同一 source_ref。不知道的源数量用 null 并报告 COVERAGE_INCOMPLETE，不写零。全局 through_event_id 仅是定位提示，不能代替各源水位或被当作跨源可比较序号。聚合 finalized 要求所有预期源身份、归属、已知请求和完整记录边界均已结算，无覆盖缺口；仅一个源 EOF 不够。历史快照记录当时已知范围，后续发现遗漏通过新版本纠正，不覆盖旧版本。

### 4.3 状态与原因

| status | 含义 |
| --- | --- |
| `pending` | 尚无可信用量，采集已启用且存在进行中的绑定/落盘等待，无已确认的失败或不支持 |
| `available` | 当前声明范围/水位内无已知缺口；仍须分别看 scope、finalized 和 nullable 字段 |
| `partial` | 已有可信用量，但存在缺失、冲突、未完成请求或尾部，不能给出完整 Run 总量 |
| `unavailable` | 当前无法提供任何可信用量；保留具体原因及可重试性 |

状态按 token 与覆盖事实确定：有可信用量且有 usage 类缺口时为 partial，没有缺口时为 available；没有可信用量且仍处于上述正常等待时为 pending，已有明确失败/不支持/授权不足时为 unavailable。失败是可重试的并不使它变成 pending。已验证的零调用证明也属于可信用量；只有空文件/零默认字段则不属于。若冲突隔离后最后一条可信事件也被排除，tokens 为 null，状态 unavailable，而不是没有可展示数字的 partial。

对于仍需采集的范围，失联后保留已知数值并转 partial；已 finalized 且无需新读取的历史快照不因宿主后来离线、关闭采集或撤权而失效。关闭采集后若 Run 继续产生尚未覆盖的输入，则新快照显式报告缺口，保留旧数值。重新查询不更新 observed_at；它是原观察时间，不能把旧数据伪装成刚取得。reasoning/model 等可选元数据缺失本身不改变完整 token 的获取状态。快照中的清理/观察状态也是该版本当时的事实；新清理结果产生新 revision，通过最新查询取得，不原地改写历史版本。

issues 至少支持以下稳定 code，message 仅是脱敏说明，不要求调用方匹配文本：

| code | 处理 |
| --- | --- |
| `COLLECTION_DISABLED` / `REMOTE_LOG_ACCESS_NOT_CONFIGURED` | 未授权采集/目录登记；不隐式扩权，配置授权后才可采集 |
| `COLLECTION_NOT_OBSERVED` | Run 查询目标已发布，但首次策略/来源观察尚未提交；返回 unavailable 和未知用量，不能提前假定采集已启用并返回 pending |
| `PRODUCTION_BINDING_UNSUPPORTED` / `FORMAT_UNSUPPORTED` | 该生产绑定/日志版本未实现，不以重试伪装修复 |
| `SESSION_UNBOUND` / `SESSION_AMBIGUOUS` / `IDENTITY_UNVERIFIED` / `IDENTITY_CHANGED` | 等待绑定或对账；分别报告尚未核实和已观察到变化，不按最新文件猜测 |
| `LOG_NOT_FOUND` / `LOG_ACCESS_DENIED` | 分别报告无日志和权限不足；日志尚未生成时可重试，已确认丢失须标记缺口 |
| `HOST_OFFLINE` / `READ_TIMEOUT` / `READ_FAILED` | 原目标失败，不回退本机或其他 environment |
| `LOG_TAIL_INCOMPLETE` / `SOURCE_CHANGED` / `SOURCE_PATH_UNVERIFIED` | 等待完整尾部、重新校验或报告路径能力不足；截断/替换不是新增正常文件 |
| `USAGE_NOT_RECORDED` / `USAGE_INVALID` / `USAGE_CONFLICT` | 请求没有可用计量或记录冲突，不按文本长度估算 token |
| `COVERAGE_INCOMPLETE` / `BUDGET_EXCEEDED` / `RETENTION_EXPIRED` | 子调用/区间未覆盖、读取/解压超限或证据已过保留期，保留仍可信部分，禁止当作新空账本 |

每个 issue 带 `category:usage|metadata|access_cleanup` 和 retryable，必要时带 source_ref 和脱敏下一步。仅 usage 类问题参与 token status 判定；元数据问题和待撤权分别可见，不能使可信 token 数被删除或降级。元数据支持 SOURCE_METADATA_INCOMPLETE/SOURCE_METADATA_CONFLICT，清理支持 ACCESS_CLEANUP_PENDING/ACCESS_CREATE_OUTCOME_UNKNOWN；不得通过查询自动修复配置。metadata 不隐含单价或费用字段。

### 4.4 逐响应事件

事件包含稳定 `event_id`、source_ref、Run/attempt/generation/command 的已确认归属、原生 session/turn/response 标识、归一化 tokens、provider/model 及其 `observed|requested|unknown` 来源、调用时间及时间来源/不确定性、observed_at、已观察到的 service tier/context 信息、格式/适配器版本、不透明 evidence_ref 及 metadata_issues。缺失元数据用 null，不能用采集时刻冒充调用时刻。快照固定当时的 token、归属和元数据版本；元数据后续补齐也必须产生新 revision。

不公开绝对日志路径、Orca endpoint、凭据、提示词、正文或原始 RPC 返回；原始来源摘要和必要绑定材料仅在本机受保护状态中保存。原生日志中的 cost 不进入统一用量 DTO，也不参与基础 token 正确性判定；调查/离线夹具可以用它核对期望金额，生产费用模块仅消费公开用量与自己的价目表。

## 5. Run、会话与日志的可靠关联

### 5.1 本机绑定记录

Run 创建/提交输入时保存：稳定执行主机身份、冻结 environment、原 runtime/工作区/terminal/incarnation/pane、agent、Runner session、原生 session、配置摘要、实际日志根、command/输入摘要及起始水位。记录来自原执行准备/投递事实，不接受用户随意指定文件作为可信 Run 证据。

本机 Windows 使用机器 MachineGuid 与当前账号 SID 的摘要，远端使用既有配对公钥指纹；名称、home 路径和连接 runtime 不能替代稳定主机身份。首次来源核验将主机、账号 namespace、agent、native session、cwd 的摘要与 checkpoint 一同持久化；后续发生矛盾时报告 IDENTITY_CHANGED。checkpoint 无法解码时保留原证据并报告 SOURCE_CHANGED，不按空水位重新发现来源。上述机器/账号原始标识不进入公开 DTO。

Runner session 与 native session 分开；DSH 不套用 Codex UUID 规则。Windows 路径使用执行主机的路径语义进行规范化和校验，不仅做字符串前缀匹配。会话头、实际 cwd、输入原文按既有 wire 编译规则的全文摘要及原生区间均须一致；200 字符预览、文件名、mtime、prompt 相同或 cwd 相同只能作候选条件。

### 5.2 各 agent 的关联方式

| agent | 优先来源 | 日志核对及归属 |
| --- | --- | --- |
| Codex | 原终端 provider session；本机既有原生会话事实 | rollout `session_meta.id/cwd`；原生 turn 与原 command/输入摘要对应。AI Vault 只发现候选，必须按完整身份再过滤，不假定 scope 过滤或索引完整 |
| Pi | 本机 config/readiness/command 回执；既有原生启动可记录的 session ID/日志目录 | v3 header id/cwd，`native_input_id/native_reply_id` 与 wire_sha256；最终 reply 必须归属同一 command，不能只确认会话内存在；沿消息父链/实际回合边界取全部响应，不凭 entry ID 字典序切区间 |
| DSH | 本机 readiness.native_session_id 与 command 回执；原生客户端明确提供的实际会话身份 | v4 header id/cwd，`native_input_seq/native_reply_seq/native_end_seq` 及原输入摘要，核对 root/child 来源，不将 Runner session_id 当 native session_id |

现有本机回执只作为已有证据消费，计量不修改正式扩展或新增 helper。远端不能要求这些本机私有回执存在；只允许从已有原生启动/终端/会话信息建立绑定。缺少可靠入口即按 §2/§4 返回不支持或未关联。远端 Pi 的原版 `--session-id/--session-dir`、DSH 原版 `DSH_HOME` 和原生会话输出是已调查候选，是否用于生产受原执行契约和独立验收约束。

### 5.3 多会话、恢复与覆盖

1. 一个 Run 可有多个 attempt/generation/native session；计入所有可证明归属的实际请求，包括初始化、失败、取消前和真正重试，不只计算 succeeded command。
2. 同一 native session 被恢复或复用时，以已核实输入/响应区间和持久水位切分。一个事件只归属一个 Run；跨 generation 重读不能再次收费。人工插入输入、共享会话重叠或来源歧义保持未归属。
3. fork 的继承前缀只作上下文，不计为新调用。需有原生谱系/创建水位才能排除继承记录；缺证据就明确覆盖不完整，不把新文件名当新消耗。
   原会话中已产生且属于 Run 的实际请求，即使后来切换显示分支、撤回消息或仅保留另一条最终回复，其 usage 仍计入。消息父链用于证明输入/分支归属，不以“不是当前最终回复祖先”为由丢弃已经发生的调用。
4. 子 agent、自动标题、压缩及其他辅助调用只有取得明确关联且不重叠的 usage 才计入对应范围；未落盘或无谱系的费用不补猜。初始化可没有业务 command，但须有 Run 所有权证据并单列 phase。
   已验历史 Codex 直接 PowerShell 启动，只在原启动字符串与已受理 launch_digest 一致、字面参数语法和固定 READY 输入吻合、时间位于 Run 创建与首条业务输入原生基线之间时计入初始化；公开事件 `phase=initialization`、`command_id=""`。业务事件为 `phase=command`，原生时间回执可用时同时限制输入/响应区间。历史无 wire_format 的 Codex TUI 仅规范化 CRLF 并裁掉尾部换行，保留空格及其余全文；其它格式不作此转换。不能识别的启动包装或缺少时间证据仍明确报告覆盖缺口。
   本机已有正式运行时的固定 READY 也可关联：核对原 launch_digest、唯一字面 runtime executable/--configuration 调用、解码配置摘要、冻结 runtime SHA、已验证准备回执的 contract/native transport、owner/process 创建身份、原 native session/workspace/repository，以及同一初始化时间窗。计量只消费现有事实，不执行、修改或部署运行时；其它包装不按此规则推断。
   Codex 原 thread/read 回执的 user item ID 与 rollout response_item.id 不属于同一个标识空间，不能相互比较或猜测转换。本机已验证回执须以原 native turn ID、原生 session、完整 wire 摘要及 Run/command 绑定交叉核对；摘要失配立即拒绝。Pi/DSH 的既有原生 ID/seq 规则保持不变。
5. 会话唯一命中、来源核验和区间归属完成后才提交 verified binding。零命中等待/不可得，多命中 ambiguous，任何失配不改选“最新”会话。

## 6. 去重、归一化与幂等入账

| 来源 | 计数权威 | 只作核对/忽略 |
| --- | --- | --- |
| Codex 已验格式 | `token_usage_record.usage`，逐 response_id | `event_msg` token_count、turn/thread 累计快照；禁止和逐响应值相加 |
| Pi v3 | 持久化最终 assistant message.usage，逐 entry ID | 重读、导出副本和其他非计量消息 |
| DSH v4 | 最终 `assistant/message.data.usage`，逐 message ID | 内嵌 stream.chunk.usage、step 输出及同一调用的其他投影 |

适配按已验版本显式选择，不能从缺字段推断另一格式。无逐响应权威记录的 Codex 旧格式、DSH 其他格式须独立适配/验收；当前返回 FORMAT_UNSUPPORTED 或 USAGE_NOT_RECORDED，不直接累加累计快照。

逻辑事件键至少为稳定主机身份、原账号/profile 的 source namespace、agent、原生 session 和原响应标识；provider 按来源元数据保存和核对，不因其缺失、补齐或冲突制造另一个事件。source namespace 在首次核验来源时持久建立，已证明同源的路径移动可保留映射，不能仅因当前路径相同/不同就合并/拆分。同一主机的 Orca runtime 重启、文件移动、副本和 Runner generation 不产生新计费身份。不同 session 的 fork 前缀按 §5 排除；短消息 ID 不跨 session 全局去重。相同 prompt 的真实新响应仍是新事件。

同键、同 token 载荷重读幂等；同键 token 冲突时在私有 event-conflict journal 中同事务保留原有及新观察的规范化载荷/来源摘要，标记 USAGE_CONFLICT，将该事件排除出可信聚合并提升快照版本，不静默覆盖或相加。跨 Run 认领冲突必须在双方持久保留，分别指向各自 source_ref；即使后认领方没有该事件行，缓存观察或服务重启也不能清除其冲突原因、恢复 finalized 或允许完整总费用。来源主机/session/归属本身矛盾时重新核验绑定，不能继续当作同源。最后一条事件被隔离后的状态按 §4.3 推导。

provider/model/服务档位/时间等观察元数据矛盾，不等于 token 载荷冲突：原生事件身份和 token 仍可信时保留计数，单独标记 SOURCE_METADATA_CONFLICT，相关字段保持未知/冲突，交由费用模块决定不可计价。冲突裁定持久保留，重读某一旧值不能自动解除。null→有同源证据的确定值是合法补齐，产生新元数据/快照版本；两个已知值相互矛盾不能伪装成补齐。不能因价格需要模型就从基础聚合中删除事件。单个事件不允许同时分配给两个 Run/command，去重 ID 缺失或复用不能用 prompt/hash/文件偏移临时造出可信响应身份。已提交快照可复核，后续裁定不改写其历史事实。

Codex 当前样本 input 包含缓存命中，Pi/DSH 当前样本 input 不含 cacheRead；由来源适配器拆成互斥类别后校验。非零缓存写入及 TTL 未经该版本字段语义验证时保持未知/不支持相应细分，不把它归零。source event、归属、水位与 usage revision 在同一计量事务提交；故障重放不会漏计或重复。

## 7. 文件取得、授权与撤销

### 7.1 读取路径与边界

本机只读实际绑定文件，校验真实目录/文件身份，拒绝链接越界。远端固定 environment 和本次 runtime，通过已有 `files.stat/readDir/readChunk` 取得原始字节，不使用 nativeChat 解析投影或 TUI 数字替代完整日志。runtime 改变先重验同一稳定主机，计量键保持不变，不回退到其他主机。

本机 AI Vault 的 codexHome 可能为空。仅允许将已匹配 session/cwd 的日期叶目录核对到显式绝对 CODEX_HOME、当前用户默认 .codex，或当前用户应用数据下的 Orca 账户 home；其余路径继续 SOURCE_PATH_UNVERIFIED。随后仍验证原 header、输入和前缀及文件链接，默认本机账户 SID 摘要参与 namespace。远端不能使用本机 home 补全缺失信息。

远端优先使用已合法可读的原工作区范围；需要新登记时，调用已有 `projectGroup.create`（不设置 parentPath）及 `folderWorkspace.create`，后续使用 `folder:<id>` 和相对文件名读取。仅登记必要叶目录：Codex 实际账户 sessions 的必要日期目录；Pi 该 Run 的 sessions；DSH 具体 native session。跨日/多会话逐个核验并登记，不开放整个 home、账号根或凭据目录。

folder workspace 授予通用工作区文件访问，不是只读 ACL。计量客户端白名单只用读取方法；元数据管理仅限自建资源。不得声称其阻止其他获授权调用方写文件。拒绝 relativePath 中的绝对路径、`..`、已知链接/重解析点越界；远端实际路径/链接能力无法核实时不能声称 OS 级隔离，须保守拒绝该候选或报告未验证。

### 7.2 授权策略和持久化租用记录

正常新 Run 可自动采集；远端新增目录登记须有预先配置且明确作用域的宿主授权策略，不能由查询、日志内容或模型输出授权。未配置时返回 REMOTE_LOG_ACCESS_NOT_CONFIGURED，不反复弹确认或默默放宽范围。策略的公开 schema/默认值须在实施前固定：采集启用默认 true；远端新增登记默认 disabled，显式 temporary 后才允许自建；不新增执行 profile 或模型配置。

策略为独立宿主装配值，不是 RunRequest/execution.settings 或模型配置。其最小契约固定为 `enabled:boolean`（默认 true）和 `remote_log_access:{mode:disabled|temporary,grants:[]}`（默认 disabled/空数组）；grant 的字段为 `environment:string`、`host_id:string`、`session_roots:string[]`（非空绝对 sessions 根列表），拒绝未知键、null、空标识、通配主机、账号 home、凭据根及可执行表达式。disabled 时 grants 必须为空；temporary 没有匹配 grant 仍返回 REMOTE_LOG_ACCESS_NOT_CONFIGURED。允许根仅是登记上界，实际登记目录仍按核实后的 Run 叶目录收窄。host_id 必须取自既有配对/设备信息中可验证的稳定身份；只有 IP、名称或 runtime 时不足以确认同一主机，报告 IDENTITY_UNVERIFIED 并停止新登记，不通过新增 Orca 字段来补齐。

宿主入口固定为现有 `service start` stdin input 中的可选 `usage_policy` 对象，沿原信封/参数校验规则发布 schema。省略时，新实例使用上述默认值；复用正在运行的同构建实例时沿用该实例原策略，不把省略解释成重置。显式给出与运行中实例不同的策略返回退出码 5 的 `usage_policy_conflict`，不热更新、不新建第二服务。策略按实例持久化 revision/摘要。更改/撤回通过既有 `service stop` 使采集器关闭并有界回收，随后以新策略 `service start`；stop accepted 不等于远端撤权已经完成，pending 由清理记录继续报告。此入口不增加配置平台或执行 profile，价格策略也不进入 usage_policy。

策略变化不改变原 Run 的冻结执行参数。关闭/撤回先持久化本机采集许可撤销，拒绝后续登记/读取并尽力取消在途 RPC，继续清理此前自建登记；不能宣称已经发出的远端请求被回滚。每次远端操作携带本机 policy revision、实例 fence 和 lease epoch；操作前及结果提交时重验。旧许可下迟到的读取结果不得恢复已撤销采集，迟到的创建结果只允许进入清理路径。已有许可只适用于记录的稳定主机及日志根，重连不扩到新账号/新目录。

stop 受理回执与许可撤销在同一事务提交；观察结果、事件和 checkpoint 的事务须重验许可仍 active 且 instance/digest 一致。异常退出也先撤销许可再取消采集。清理不依赖采集许可仍有效。原 RPC 已成功返回但父上下文同时取消时，用独立且至多 5 秒的本机写入保存已知 ID/操作结果，之后仅继续清理；真正丢失的响应仍保持未知，不能借此补造结算证明。

登记前持久化唯一创建意图、随机实例与 epoch 派生的独立所有权标识、稳定目标、规范化目录及预期 group；新 epoch 清空旧 group/folder ID 和未结算标记；响应后保存精确 group/folder ID。响应丢失先查清单对账，不盲目重复创建；归属有歧义不得删除。创建仍可能在远端执行时，某次清单暂时没有该目录不能证明“从未创建/已撤销”；必须保留 ACCESS_CREATE_OUTCOME_UNKNOWN/pending，直至原操作已结算且其创建结果完成对账。接口不能给出足够证据时允许长期 pending，并明确需要人工核实，不能编造幂等/服务器 fencing 保证。

多个 Run 共享根目录时，只复用采集器自建登记，由同一持久写者维护引用和 epoch。最后释放时先原子进入 releasing 并禁止该 epoch 新引用；新采集者等待回收结算后才可使用新 epoch，不能在旧 delete 仍在途时复用同一 ID。旧实例/旧 epoch 的回执不能删除或激活新登记。引用依据持久消费者记录对账，不仅靠进程内计数，重启不重放 acquire 造成重复引用。以上是本机防护，不能等同于 Orca 已提供原子条件删除。

租用状态至少覆盖 creating、active、releasing、revoked、cleanup_pending。这些仅是计量资源状态，不是业务状态。自建登记不承载业务终端；回收不得影响原执行工作区/终端或用户自有资源。登记元数据、会话原始文件和凭据的生命周期分别管理。

### 7.3 撤销和故障恢复

最后一个使用者完成、取消、失败、超预算或授权撤回后释放引用并回收；服务正常停止先有界释放自身计量登记，不停止仍在运行的远端业务。清理使用独立、有界的上下文，不复用已取消的业务/读取上下文；超时保留 pending。异常路径不能只依赖 finally：启动/重连时优先对账遗留自建资源，再处理仍获授权的采集。group 仅在所有自建 folder 已释放且没有其他资源时删除；被外部复用或无法核实安全时保留 pending，不级联清理他人资源。

撤销完成须在原稳定主机上经认证连接同时验证：自建 ID 从清单消失、同一 selector 返回明确 `selector_not_found`、用户原有登记未被删除，并且不存在尚可能创建/修改该租用的未结算操作。timeout、断线、认证失败、源文件 ENOENT 或任意“读取失败”均不能替代 selector 撤销证明。有其他用户授权时明确其仍保留访问。只撤销本采集器入口，不声称文件对所有入口不可读。删除确认丢失时按同一 ID 只读对账，不借新主机或模糊名称补造成功。

操作前后复核自建 ID 的实际目录和资源关系；用户改变登记路径、附加终端或复用 group 时停止读取/自动删除并报告待处理。当前接口未证明有原子 compare-and-delete，不能以“删除前查过空列表”承诺并发外部修改绝对安全；缺少足够安全证据时保留 pending，而不是级联删除来满足清理计数。

group 的安全检查包含子 group、其他 folder 和 repo 引用；删除 folder 后再次查询 group 及引用，不能沿用删除前的空组事实。声明 truncated/hasMore/nextCursor 或 totalCount 与实际条数矛盾的清单不能证明安全。重连仅在核对原 epoch/instance 后更新清理 runtime，不能先覆盖持久 lease。租用记录保留该 epoch 影响过的 Run 集合及已确认撤销证明；新 epoch 不覆盖旧 Run 的撤销事实，其他 Run 的 pending 也不能污染当前 Run 的清理状态。共享引用仍有效时返回 active 并以 ACCESS_STILL_SHARED 说明。

断网/崩溃后持久标记 cleanup_pending，重连补偿。实测接口没有可用的服务端自动过期能力，不能承诺失联期间即时撤权；本机期限只是清理计划，不是远端 TTL。清理状态必须可查询，不能因 token 已取得就报告全清理成功。

公开 access_cleanup_status 为 `not_required|active|pending|revoked`，聚合所有与本 Run 关联的自建租用：任何创建/释放未知优先 pending，其次任一仍有效/被其他 Run 共享使用的登记为 active，全部已有登记均证实撤销才为 revoked，从未需要自建登记才为 not_required。释放本 Run 的引用不等于共享登记已撤销；issue 必须说明仍共享有效的范围。用户授权或本采集器其他 epoch/其他登记另作来源说明，不能把某个旧 selector 的 revoked 声称为该路径全局不可访问。

正常采集不删除原生日志、用户 home 或模型凭据。原生日志和本机证据按各自保留策略管理，过期后保留“已过期”的事实而非零值。

### 7.4 增量读取与结算预算

按固定长度快照读取，分块不超过实测 readChunk 上限 512 KiB；校验返回字节数、边界及来源。活跃文件允许合法追加，只提交完整 JSONL 记录；EOF/尾部/压缩帧未完整时保留已核实前缀。文件缩短、已提交前缀变化、header 变化进入 SOURCE_CHANGED，不以新 offset 直接续读。

DSH 首期本机按完整快照流式解压所有串接 zstd frame，再用响应键去重；不把任意压缩字节偏移当可恢复边界。缺尾帧与损坏区分报告。固定每源单轮上限：压缩/原文件 64 MiB、解压后 256 MiB、单条记录 4 MiB、远端单轮 30 秒；超限保留已提交数据并标记 BUDGET_EXCEEDED，禁止静默截断后声称完整。并发及退避沿本机受控采集预算，不阻塞业务执行 gate。

原生结束/停止只触发有界末次读取：最多 30 秒确认该绑定范围、水位、完整尾部及稳定来源，随后释放登记。未收敛且有可信数据时保留 partial，无可信数据则按 §4.3 返回 pending/unavailable；两次 stat 相同本身不证明所有后台写者已退出或供应商全部计费请求均落盘。后续新授权输入重新开启采集，查询本身不触发补采。

## 8. 独立费用估算模块

定义 `RunCostEstimator.EstimateRunCost(runID, usageSnapshotRevision, pricingPolicyID)`，只通过 §4 接口先取指定版本汇总，再读取同版本事件页。拟议公开命令为 `estimate-run-cost --state-dir <absolute> --run-id R --usage-revision V --pricing-policy P --json`。版本不存在/过期沿 §4 返回查询错误，禁止改算 latest。估算不得请求补采或等待远端。无价目表、价目表损坏/加载失败或费用模块关闭时，本机宿主、执行和用量服务仍正常注册/工作，只将该费用策略标为不可用；基础模块初始化不得读取或校验价格。公共信封/参数本身非法仍按原协议拒绝，不是价表加载失败。

费用模块不导入日志取得器、Orca 客户端、agent parser 或采集写者，不直接读取私有日志/计量表。

价目表由费用模块独立装配，包含 policy ID、版本、摘要、来源、有效时间、币种、provider/model 映射、时段/节假日规则、上下文及服务档位规则。查询不临时联网查价格，单价更新不重写 token 事实或旧估价。相同用量快照和价格摘要应得到相同结果。

价表可选 tier/min_context_tokens/max_context_tokens 可以省略，显式 null 按公开 schema 拒绝，不能静默消除计价条件。若同一模型/时间段存在可能适用的 tier/context 条件规则，而事件缺少该判别字段，报告 PRICING_CONTEXT_UNKNOWN，不能绕过该规则选择通用价。只有显式允许时才使用原生 completion time 代理 request time；unknown 时间来源不能因该开关而变成可计价。DSH v4 原生 time 为毫秒时间戳，按其格式解码为 completion time，不使用采集时间补齐。

按逐响应互斥类别计算 `(未缓存输入×输入价 + 缓存读取×读取价 + 已知缓存写入×写入价 + 输出×输出价) / 1,000,000`，再聚合。reasoning 不重复收费；缓存 TTL/档位不明时不能套用已知零缓存样本的规则。使用十进制/有理精度，明细聚合后才显示舍入，不能逐响应按美分或整数微美元过早舍入。

费用结果使用独立 schema `run-cost-estimate/v1`：status 为 `estimated|partial|unavailable`，带 usage_revision、scope、usage_status、pricing_policy/version/digest、currency、covered_estimated_amount、total_estimated_amount、priced/unpriced event 数、假设及原因。金额使用十进制字符串。一条响应只有所需 token 类别及计价条件齐备时才算一个 priced event；未知类别不能按零补齐。金额为零与没有可计价事件不同。

status=estimated 和非 null 的 total_estimated_amount 要求同版本 usage.status=available、coverage.finalized=true、所有预期源覆盖完整，且全部事件已按有效策略计价；金额仍只限声明的 scope。只取得一个完整前缀、该范围仍有已授权调用执行中或任一预期源未结算时，coverage.finalized=false，已有可计价事件只能给 partial/covered_estimated_amount，total_estimated_amount 为 null。费用模块只依赖此覆盖事实，不另查业务 Run.status 来推断收敛；即使业务仍等待后续人工输入，也可对已经闭合的当前授权范围估价，后续输入进入新快照。没有任何可计价事件时为 unavailable、两种金额为 null；例外是用量明确证明已 finalized 的零调用且策略有效，可返回 estimated/零金额。可信 token 已完整但模型元数据/价格冲突导致估价 unavailable，不改变基础用量的 available。

估价原因至少包含 ESTIMATION_DISABLED、COST_POLICY_INVALID、USAGE_UNAVAILABLE、USAGE_PARTIAL、USAGE_NOT_FINALIZED、PRICE_NOT_FOUND、MODEL_UNKNOWN、PRICING_METADATA_CONFLICT、PRICING_CONTEXT_UNKNOWN、PRICE_PERIOD_UNKNOWN；逐事件保留未计价范围。模型只来自 requested、服务档位缺失或边界时间不明时，必须由显式价格策略选择是否允许假设，并在结果列明；已知值相互冲突默认不选择其一。任何费用失败/假设不能反向修改用量事实、status 或读取授权。

OpenAI 订阅/credits 与 API-key 计费分别解释；首期美元结果标记 `basis=api_equivalent`，不称为实际扣费；公开 assumptions 固定说明 token-only、排除订阅/credits 结算及工具/外部 API 费用，同时单列适用的 completion-time 假设。DeepSeek 高峰/非高峰按请求发生时间及版本化日历选择；只有完成时间且跨边界不确定时报告假设或未定价。工具/API 费用如未采集则排除并明示。Pi 原生日志 cost 仅用于调查/离线验收核对，不替代版本化策略或增加基础接口费用字段。

官方规则参考：[OpenAI 计费](https://learn.chatgpt.com/docs/pricing)、[模型价格示例](https://developers.openai.com/api/docs/models/gpt-6.1-sol)、[DeepSeek 价目表](https://api-docs.deepseek.com/quick_start/pricing/)。本文不冻结在线现价；验收使用带日期/来源摘要的价目表夹具，历史价格未知不得静默套用当天价格。

## 9. 持久化、兼容与安全

本机持久化分为 Run-log bindings、规范化 usage events/归属/冲突、读取 checkpoints/snapshots、目录授权意图/引用/清理记录；费用策略及可选估价缓存独立管理。复用现有存储写者与宿主生命周期，不给用量模块业务 Record 写权限，不引入可恢复业务队列。事件归属及快照事务不增加 Run revision 或原业务事件。文件/网络/解压/价格加载不持有业务 writer gate；仅以短事务提交计量，不因等待遥测阻断 stop 或业务提交。

原工作区/原生日志清理无需等待计量成功；采集只在既有保留窗口内有界取得来源，源已被原生命周期清理就记录 LOG_NOT_FOUND/COVERAGE_INCOMPLETE，不重建目录、阻断清理或重新启动会话。历史归属/去重身份及必要裁定的保留期不得短于仍可再次采集的 Run 生命周期；精简原文或快照不等于忘记已入账 ID。若规范化明细/去重依据确实过期，保存 tombstone 并报告 RETENTION_EXPIRED，禁止将源文件重放成全新消耗；不能直接把它当空白新账本。

公开能力和 schema 包含用量与费用各自的版本及六组合适用性，不要求配置价格后才能启用基础能力。关闭采集仍保留历史只读查询和未完成撤权清理；关闭估价不影响采集、基础存储或用量状态。已有请求摘要/冻结材料不加入价目表；当前 Run 的实际执行模型仍由原冻结规则决定。

本机诊断只记录不透明来源引用、必要身份及摘要，禁止回显凭据或持久化完整私密日志作为默认证据。读取原文件不可避免接触正文，解析后默认只保留计量/绑定所需字段。若原生日志可被执行者修改，统计仅为来源可追溯的观察值，不声称供应商签名账单或防篡改审计。

## 10. 验收标准

| ID | 必须证明的行为 |
| --- | --- |
| U01 | 费用关闭/无价目表/坏价目表/加载失败时宿主及用量接口、不可得状态、自动采集和撤权仍工作；费用元数据冲突不删除可信 token；依赖方向正确 |
| U02 | 三客户端按已验格式正确归一化；未知≠零，reasoning 不重复；非零缓存细分有版本依据 |
| U03 | Codex 响应/累计快照、Pi 重读、DSH 内嵌 stream 去重；真正重试计入；token 冲突隔离、元数据补齐不重复、去重保留/过期不重放 |
| U04 | Run/session/cwd/原输入/区间多重绑定；DSH 双 session、相同 prompt、人工输入、共享/恢复/fork/显示分支不串账或漏计；多个源的缺口不被缩小 scope 隐藏 |
| U05 | 查询零外部副作用；历史汇总和事件页固定同一快照，状态/零值/最终历史/过期错误明确，六组合能力不夸大 |
| U06 | 远端未登记拒绝、最小目录登记读取、撤销失效；通用文件权限限制明示，不开放凭据根 |
| U07 | 并发共享登记、最后引用/epoch、迟到创建、删除未知、崩溃/失联及权限撤回均显式对账；只有原主机认证查询明确 selector_not_found 才能证实撤权，安全未知保留 pending |
| U08 | JSONL 尾部、串接 zstd/缺帧、增长、替换、截断、读取/解压预算及停止末次采集正确报告 |
| C01 | 固定用量/价表可复算；混合模型、时段、档位、缓存及精度正确；未知价格只影响费用 |
| C02 | 未 finalized、部分覆盖、无可计价事件和已验证零调用分别处理，只有 finalized 完整范围才有总费用；订阅/credits/API 等价说明及工具费范围准确 |
| G01 | 本机 Codex/Pi/DSH 与远端 Codex 生产 Run 经公开接口真实验证；远端 Pi/DSH 仅在生产绑定另行具备并验收后才声明可自动统计 |
| G02 | 零 Orca 改动、零新增 helper/远端解析脚本；计量/估价动作零新增模型输入；原执行/恢复/停止/发布契约不回退 |

不得把调查脚本、一次性 CLI、受控绑定夹具或“目录读取成功”计为远端 Pi/DSH 的生产 Run 自动统计通过。缺现场前提逐组合登记未完成；不得用整体“已支持六种”覆盖缺口。

## 11. 文档审计修订（2026-10-21）

以下为本轮确认并已修正的文档问题，不是实现/现场验收结果。

| ID | 修正的问题 | 契约及计划验证 |
| --- | --- | --- |
| A01 | 估价指定版本，但汇总接口原先只能读 latest，会把旧事件与新覆盖/模型拼接 | §4.1/§4.4/§8；P1/P5、T12 |
| A02 | 模型元数据冲突原先会排除可信 token；坏价目表初始化也缺少故障隔离约束 | §4.3/§6/§8；P1/P2/P5、T13 |
| A03 | 状态推导不能区分可重试失败、正常等待、最终历史离线和冲突后零条可信事件；多源水位不明确 | §4.2–§4.3；P1/P4、T14 |
| A04 | 未 finalized 的完整前缀可能被当作整个 Run 总费用，未计价与真实零金额未区分 | §8；P5、T08/T14 |
| A05 | 按最终消息父链过滤会漏掉已发生的其他分支调用；去重依据过期后可能重放 | §5.3/§6/§9；P2/P4、T15 |
| A06 | 授权策略缺明确宿主加载/撤回入口，也没有在途操作与旧许可结果的围栏 | §7.2；P0/P3/P4、T16 |
| A07 | 创建响应未知时暂时查无目录不足以证明未创建；末引用释放与新 acquire 可发生竞态 | §7.2；P3、T16 |
| A08 | 任意读取失败可被误报撤权；空列表后删除也不等于原子安全，外部复用必须保留未知 | §7.3；P3、T17 |
| A09 | 未支持远端 Pi/DSH 与“不存在 Run”的协议结果原先可能混淆；平台范围未限定 | §2/§4.1；P0/P7、T10 |
| A10 | 末次采集与原日志清理可能冲突，缺少不能阻塞业务及不能重建日志的明确规则 | §9；P4、T11/T15 |
