# Workflow 模型与参考费用用量

2026-11-10。本文是当前模型编排和 SteerLine 估价交接规则；取代历史设计中逐轮独立核验实际模型的要求。历史证据保持原样。

## 模型选择

模型由双方明确约定的配置及冻结的 Workflow 编排决定。每次阶段输入保存模型和 reasoning effort，原生调用只派发该输入指定的选择。Runner 不根据任务、费用、负载、账号权限或失败自主选模型、降级或换模型重试。指定模型不可执行时保留原错误；发送是否接受未知时仍只对账原操作，不能换模型重发。

当前复用现有 execution/audit 角色，不增加外部逐阶段配置接口。development-audited 的实施、实施补报、发布和发布补报使用 execution 模型；审计修复及审计补报使用 audit 模型；发布回切来自明确流程边。普通 development、execute、generate、conversation 沿 execution 配置。恢复沿原已冻结输入，不使用当前配置改写历史模型。

已有允许省略模型的非审计配置继续表示原生默认：Runner 不替它选一个名称，也不把当前原生默认补成已约定模型。需要按模型参考估价的调用方应明确约定并配置模型。

Runner 不为计价增加逐响应供应商模型核验，也不再为本机 Orca audited 额外注册和读取模型日志、等待实际模型证明。原命令、会话、回合、Token 来源、Git 审计及 PR 身份核验仍各负其责。配置选择并不等于供应商实际执行证明。

## 用量事件

公开查询仍是 `run-usage` 和 `run-usage-events`，schema ID 仍为 v1。`capabilities.run_usage.configured_model_attribution=frozen_stage_input` 和 `observed_model_field=model` 标识此增量；事件 schema 发布以下可选字段，旧事件与旧快照仍可读取：

| 字段 | 含义 |
| --- | --- |
| `step_id` | 原始 Workflow 阶段，例如 `implement.execute`、`audit.code.execute`、`publish.execute` |
| `configured_model` | 该次阶段输入冻结的配置模型 |
| `configured_reasoning_effort` | 该次阶段输入冻结的推理强度（如有） |
| `model`、`provider`、`metadata_basis` | 原生日志顺带提供的实际观测元数据；可缺失或冲突，不从配置复制 |
| `tokens` | 本事件已绑定响应的各类 Token；未知分类保持 null |

绑定按 `attempt_id + command_id` 精确对应原 StageInput；不会拿当前阶段、当前角色或 Run 的首个模型填满所有响应。同一阶段多个响应分别保留事件，同模型的不同阶段也保留各自 step_id。同一响应重复采集仍去重；配置归属冲突隔离该响应，实际观测元数据冲突只影响观测字段，保留配置归属及 Token。

缺失原始绑定、模型省略、初始化响应和不认识的旧记录不猜测模型。没有配置模型的 Token 必须单独显示为“未分配模型”，不能按其它阶段或实际观测模型自动归账。已有可用缓存可仅依据冻结输入补齐新快照的配置归属，不重新读原生日志；旧 revision 的事件页不变。

`run-usage.tokens` 是 `scope=main_session` 的 Token 总量，可跨模型相加作数量统计，不能直接乘一个模型单价。这个总量、status 和 coverage 的原含义保持，Pi 等没有可验证来源时仍报告不可用。主会话事件的 scope 可省略；新增内部事件明确为 `runner_internal`，不能将其加入主会话合计。

2026-11-10 起，Runner 自己启动的独立 parser 也进入同一事件账本：`call_source=llm_parser`、`call_id` 标识每次物理调用，`command_id` / `source_result_id` / `step_id` 绑定原业务来源，配置模型来自该次调用的 parser 配置。启动前登记 `coverage_status=pending`；取得原 CLI turn 的用量后为 covered、decision=counted；未取得可信用量为 uncovered、decision=uncovered，Token 保持 null。进程或服务崩溃后未收齐的登记继续 pending，不用零填补。重试各有独立 call_id，不合并其真实消耗；相同事件重采不重复累计，历史 snapshot_revision 不改写。原生调用失败但已取得可信用量时仍计入已覆盖消耗。

`model_call_coverage` 单独给出内部已登记、covered、uncovered、pending 调用数量；只描述已登记调用。subagents、tool_internal_models 和 unobserved_provider_calls 明确未覆盖，不能当作零或声称完整 Run 总成本。Parser 不可用时采用原流程明确允许的一次报告边，是不同编排阶段，不是给 parser 悄悄换模型。新 Run 复用 worktree 时使用新会话和用量身份，只记录来源关系，不继承来源 Token；见[工作区复用](worktree-reuse.md)。

可选 usage 元数据单独解析，字段类型错误只使该调用未覆盖，不改变已验证的业务文本结果。重复或冲突完成事件、流身份错误、截断或扫描错误会使该调用的计数失效，不能沿用较早完成事件的 Token；完整且一致的原 turn 用量才可 covered。

## SteerLine 配套

1. 更新事件解码及能力发现，读取新增字段；先固定 `run-usage.snapshot_revision`，再用同一 revision 翻完事件页，以 `event_id` 去重，按 scope 区分主会话和 runner_internal，再仅对 `decision=counted` 的事件估价。
2. 用明确约定的计价配置解释 `configured_model`，按阶段、配置模型（必要时含推理强度）分别累计 Token、逐组计算再相加。模型的供应商、币种、价表、版本和时间规则由 SteerLine 的约定维护，不依赖每条事件实际观测 provider/model 齐全或相等。
3. 参考费用可在 `model=null`、`metadata_basis=unknown` 或实际观测别名不同的情况下按配置模型计算；实际模型另列为观测信息。结果标注“按约定配置模型估算”，不表述为供应商账单核对。
4. 缺配置模型、Token 分类、覆盖或适用价格时展示已覆盖费用及缺口；不得用另一个模型单价覆盖整次 Run。主会话范围、初始化/未分配用量、内部 parser 的独立 scope、pending/uncovered 调用，以及子 agent/工具内部等未覆盖调用和缺失用量应明确展示。历史快照缺新增字段时保留未知，不能用当前配置倒填。

原模型归属验证见[独立验收记录](acceptance/workflow-model-usage.md)；worktree 复用及内部调用覆盖的本次验证见[复用验收](acceptance/worktree-reuse.md)。
