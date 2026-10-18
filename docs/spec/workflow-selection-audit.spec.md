# Spec：内置 Workflow 选择与审计模型

日期：2026-10-15。状态：已按收敛方案实施，定向 unit/controlled 与本机最小真机验证通过；具体构建、配置与历史失败见 [Plan](workflow-selection-audit.plan.md)。

本文扩展[首版契约](steerrunner.spec.md)。原有冻结输入、单写者、原会话、未知结果对账、停止授权和 PR 归属规则继续适用。2026-10-18 扩展：审计流程同时支持本机 Codex 与配置可信执行主机 runtime 的 Orca；各自保留独立真机验收记录。

## 1. 目标与范围

调用方通过现有 CLI 查询可选的内置 workflow、提交时选择其中一个，并为审计指定模型。现有及新增审计提示词可以覆盖；审计沿用原 agent、会话和执行链路。

当前 `workflow.id/version/prompt_overrides` 提交字段已存在，但只接受 `development/1`，`capabilities.workflow` 也只描述该流程。本期补齐多选项及实际执行，不新建 UI 或另一套执行接口。

本期交付两种内置流程、步骤提示词覆盖、审计模型选择、结构化审计与有界修复。内部只需要按 ID/version 查找的流程配置和现有路由的扩展。未来增加内置流程仍可修改代码并重新编译。

外部 workflow 装载、通用图解释器、动态 handler/插件注册、表达式、交互选择菜单、并行审计和 SteerLine 界面不在本期。也不新增 Git tree 内容证明、模型目录服务、完整步骤历史查询或存储格式升级。

## 2. 可选流程与步骤

| 标识 | 流程 | 审计模型 | 支持范围 |
| --- | --- | --- | --- |
| `development/1` | 实现→报告→发布→必要时发布补报→PR 核验 | 不接受 | 保留原能力与准入检查 |
| `development-audited/1` | 实现→报告→Spec 审计→代码审计→发布；有阻断问题则回到实施修复并重审 | 必填 | `codex-app-server` / local；受管理的 `orca-terminal` |

原四个模型步骤 `implement.execute`、`implement.report`、`publish`、`publish.report` 保持。新流程只增加：

| step ID | 检查范围 |
| --- | --- |
| `audit.spec` | 冻结 Spec/Plan 的要求、范围、验收与当前实现是否一致 |
| `audit.code` | 正确性、回归、边界、测试，以及可证实的安全问题 |

两个审计共用输入组装、结果校验和执行方式，各有提示正文。修复复用 `implement.execute` 与 `implement.report`，附加原审计 findings，不新增 repair 步骤或另一套报告机制。

流程版本固定审计顺序、通过规则和最多 2 次自动修复。改变这些行为须增加流程版本；措辞调整可沿用版本，但资源摘要改变，已有 Run 沿用冻结正文。普通流程不增加审计门槛或模型必填项。

## 3. 对外接口

### 3.1 发现

复用 `steerrunner capabilities --state-dir <绝对路径> --json`，仍查询目标服务、不隐式启动服务。成功响应的 `data.workflows` 返回两种内置流程，每项包含：

- `id`、`version`、`description`。
- `supported_adapters`、`audit_model_required`。
- `prompt_steps`：允许覆盖的步骤名；`maximum_repair_cycles`：普通流程 0、审计流程 2。

旧 `data.workflow` 字段保留，继续描述 `development/1`。同步更新现有公开配置 schema，说明新增 `workflow.audit_model` 及各流程允许的步骤；不另建目录协议、独立版本体系或新 CLI 命令。资源摘要继续按现有机制覆盖内嵌资源。

调用方可据目录展示选项、再经 submit 提交选择。目录不声明账号可访问的具体模型，原生资格核验仍按 Run 执行。

### 3.2 提交与校验

保持 `steerrunner-cli/v1` 及原 submit 信封。以下仅为 RunRequest 的相关片段；其他必填输入不变，模型名是占位值：

```json
{
  "workflow": {
    "id": "development-audited",
    "version": 1,
    "audit_model": {
      "model": "review-model-id",
      "reasoning_effort": "high"
    },
    "prompt_overrides": {
      "audit.spec": "核对冻结 Spec/Plan，报告有具体依据的偏差。"
    }
  },
  "execution": {
    "adapter": "codex-app-server",
    "target": {"kind": "local"},
    "settings": {
      "model": "execution-model-id",
      "reasoning_effort": "medium"
    }
  }
}
```

1. `workflow.id/version` 仍必填。未知流程/版本返回 `unsupported_schema`；审计流程搭配不支持的 adapter/target 返回 `unsupported_capability`，不降级为普通流程。
2. `audit_model` 只允许 `model` 和 `reasoning_effort`。model 必须显式非空且无首尾空白/控制字符；effort 省略时继承规范化执行 effort，枚举沿用现有 settings。null、空字符串、未知字段、重复键按现有严格输入规则拒绝。
3. 审计流程必须提供 `audit_model.model`，并显式填写现有 `execution.settings.model`，使审计后能够确定地切回执行模型；允许两者相同。普通流程保留省略执行 model 的原语义，不接受无效的 `audit_model` 字段。
4. `prompt_overrides` 只接受所选流程的非空正文。普通流程仍为原四步，审计流程再允许 `audit.spec/audit.code`。材料、来源、审计结果格式与发布目标继续由 Runner 附加，不能被覆盖正文删除。
5. 除流程/能力不支持外，以上形状错误返回 `invalid_input`；沿用现有退出码。纯字段和组合检查在接受及业务代理启动前完成。原生模型不可用或参数不兼容时阻断，不回退模型或创建替代会话。

更换 workflow、模型、材料或提示词须新 Run；reply/resume 不修改这些冻结输入。相同 request_id 改变提交内容仍为 `request_conflict`。

### 3.3 查询

`inspect.workflow` 继续返回所选配置；增加可选 `audit` 对象，包含冻结的审计 model/effort、自动修复计数，以及当前实施结果对应的审计列表。列表项包含 step、来源报告 command、审计 command/turn、verdict、findings 或失败原因。

当前实施结果由现有 command/turn 关联标识，不再增加独立 round、generation 或 step instance ID。新实施开始后旧审计不能出现在当前有效列表中；原结果仍保存在现有 StageInputs/StageResults 和事件中，不新增完整历史接口。

原 execution 投影增加可选 `requested_model/requested_reasoning_effort`，来自当前轮次（无待执行轮次时取最近轮次）的冻结输入；旧普通轮次没有这些字段时省略，不回填。只表达请求参数，原生没有证据时不宣称实际使用型号。状态沿用现有集合，通过 step/phase/diagnostic 表达审计、修复和预算耗尽。

## 4. 审计与修复规则

1. 实施和实施报告沿用原行为。报告 done 后，普通流程进入 publish；审计流程依次执行 `audit.spec`、`audit.code`。只有当前实施报告对应的两项审计都通过才允许 publish。
2. 每个审计输入绑定同一份已接受的实施报告 command/turn，并附冻结材料位置、对应实施结果和审计范围。不能靠“上一条消息”或同名 step 选择来源。
3. 任一审计 changes_requested 时停止后续审计。预算允许时，冻结该结果的阻断 findings 并回到 `implement.execute`；本轮实施输入明确要求修复这些问题、完成必要检查，并复用原材料、分支及 PR。
4. 每确定一次自动修复，在保存其 PendingTurn 或待授权 NextStage 的同一事务中将修复计数加一；恢复该意图不再次计数。Run 最多 2 次，重启、resume、重复观察或业务反馈不重置。第 2 次修复后仍有阻断问题则 blocked，reason=`audit_repair_limit`。人工反馈仍可授权再次实施，但不取得新的自动修复预算。
5. 每次新的实施（含自动修复、人工反馈）都使旧审计失效。新的实施报告成功后从 `audit.spec` 重审，不能沿用上一轮 pass。沿用现有 command/turn、输入版本及待发送意图，不增加新的身份系统。
6. 每个新实施 command 的报告保留一次原格式补正预算；新实施排队时重置现有 ReportRetry，报告重试/恢复时不重置。不需要按所有历史实例维护另一张预算表。
7. 审计原生成功且可信 final 可解码时才解释 verdict。缺 final 继续原结果对账；原生失败/中断按原恢复规则阻断；格式错误直接 blocked，不新增自动审计补报/格式重试。needs_input、blocked 和 unknown_external 沿用原报告状态规则，不能升级为 pass。
8. publish 与可选 publish.report 保留原行为，包括发布未知只查询、补报不重复发布、原 PR 复用及 provider 独立归属核验。审计 pass 本身不能产生 pr_ready。

## 5. 最小审计结果契约

复用现有 outcome/summary/question/blocker 语义、JSON 上限和严格解析规则；仅审计结果增加 verdict/findings。例如：

```json
{
  "outcome": "done",
  "summary": "发现一项需要修复的问题",
  "verdict": "changes_requested",
  "findings": [
    {
      "severity": "P1",
      "message": "某恢复路径将修复计数置零，重启后可能重复取得自动修复预算；应沿用持久化值。"
    }
  ]
}
```

done 要求 verdict 为 pass/changes_requested、findings 为数组。每项仅要求 severity（P0–P3）和非空 message；message 应说明具体问题及依据，可包含修复建议。path/line 可选，出现时要求仓库相对路径、正整数行号，line 依赖 path。无需模型生成 finding ID、目标摘要或多份必填说明。

P0/P1/P2 阻断发布，P3 为建议；verdict 必须与 findings 一致，允许 pass 携带 P3。needs_input/blocked 不允许携带 verdict/findings 推进部分审计。缺字段、矛盾、重复键、多对象或无可信来源不得通过。结果仍受现有 128 KiB 上限约束，不截断后放行；普通报告不因新增审计字段改变其通过规则。

审计以读取代码、冻结材料和已有检查结果为主，不实施修复、commit 或发布。该限制是业务提示策略，沿用既有原生权限，不宣称强只读或上下文隔离。publish 提示词要求只发布已经完成审计的当前成果，不在发布阶段继续修复。

本期审计门槛基于 Runner 的实施/审计来源与顺序，不新增临时 Git index、工作区内容摘要或远端 head tree 对比。PR 仍沿用既有归属核验；本期不提供并发外部改文件的内容一致性证明。发现需继续修改时须回到实施并重新审计。

## 6. 模型传递、冻结与恢复

审计及其原阶段继续使用 audit model/effort，实施、报告、发布使用 execution 参数。新流程每轮显式发送具体参数，避免修复或发布继承上一轮审计模型。模型差异不改变 agent、原 thread/session、workspace、分支或 native 权限。合法恢复仍按原规则重连原 thread，不为换模型新建会话。

接受新审计 Run 时冻结轻量配置（ID/version、审计顺序、修复上限）、合并后的提示正文及两组模型参数。每轮最终 prompt、model/effort 随原 StageInput/PendingTurn/操作请求保存，纳入原意图相等性与摘要校验。恢复、对账不读取当前默认值重新生成原输入，也不静默回退模型。

停止继续先撤销后续输入授权；审计完成不能绕过 gate 排队实施或发布。服务重启零自动输入，显式 resume 先核验原会话及未知原操作，修复计数和审计来源保留。

显式业务反馈重入实施并使旧 pass 失效；审计问题的答案留在原审计步骤、保留原实施报告来源，不能被现有固定 `Option: implement` 逻辑误导。原生未结束问题仍经既有精确 question route。预算耗尽后的 resume 不能自动发起第三次修复或跳过审计发布。

## 7. 实现与兼容边界

按 ID/version 查找一份轻量内置配置，最少包含提示词步骤、审计顺序与修复上限。目录、准入和 workflow.Route 使用同一配置；新增内置审计通常增加配置/提示词并扩展测试。保留 Go 中明确的实施/报告/审计/发布转移，不要求实现任意图、可执行 JSON 或通用步骤类型框架。

Workflow 管业务转移，Application 负责冻结/提交/调度，ReliableExecution 和 Adapter 继续管原操作及原生机制。未来外部装载可以提供同类配置，本期只保留该边界，不提前设计装载协议。

维持 `steerrunner-state/v2`，新字段以可选 JSON 值加入现有 Run，不改变表结构或元数据版本。新构建读取旧 development/1 时，没有审计字段即走原行为，沿用原冻结提示和原操作，不能补入当前默认值。新增 TurnInput 字段采用省略零值的编码，旧原请求的序列化字节与摘要必须保持。

新审计 Run 缺少自身冻结配置是错误，不能按旧普通流程解释。未知流程/配置在任何新业务输入前明确拒绝。这里要求新版本读取旧普通 Run，不承诺旧 EXE 能读取新审计 Run；升级后不得把同一状态目录回退交给旧 EXE。

## 8. 验收要求

| ID | 必须证明的行为 | 最小层级 |
| --- | --- | --- |
| WA-01 | capabilities 发现两流程，submit 选择实际生效，旧 workflow 字段/schema 保持一致 | unit + 实际 EXE controlled |
| WA-02 | 普通流程行为不变；非法流程、audit_model、覆盖步骤或 adapter 组合在接受前拒绝 | unit + controlled |
| WA-03 | 两项审计依序执行、修复复用实施/报告、重审不复用旧 pass；两次自动修复上限有界 | unit + controlled |
| WA-04 | findings/verdict 一致性、无可信 final、原生失败、坏 JSON、错误来源均不放行 | unit + controlled |
| WA-05 | 实施 A→审计 B→修复 A→重审 B→发布 A，同 thread/workspace；参数改变拒绝，无回退/盲重发 | adapter/reliable unit + controlled |
| WA-06 | 提示词/模型/配置冻结、幂等冲突、重启零输入、resume 不重置预算、stop 后零后继输入、原阶段回答 | application/storage + controlled |
| WA-07 | 新版本可读取旧 v2 普通 Run，旧原操作字节/摘要不变；新审计配置缺失明确拒绝；原依赖边界有效 | unit + 旧记录夹具 |
| WA-08 | 两个实际不同模型在本机原会话完成审计与切回发布，原生 final 和真实测试 PR 可核验 | 最小 opt-in live |

controlled 覆盖修复和故障分支；live 只验证两模型、同会话及最小真实 PR 闭环，不扩展模型矩阵，也不强迫真实模型制造 finding。原生环境失败保留缺口，不能用受控结果代替真机通过。结果登记在 Plan，不改写首版验收历史。
