# Orca 远端审计、多 Run 与可选 App 扩展验收

日期：2026-10-18（上海时间）。实现提交为
`e492ec1167be4168f7a56bd14ece4355efab057d`；之后只补文档与有限事实导出。
本记录补充 [2026-10-17 验收](orca-remote-20261017.md)，不修改其历史结论。

## 当前结果与边界

| 范围 | 当前构建结果 |
| --- | --- |
| L1 | 289 项通过；6 个显式 helper/native skip 不计真机通过 |
| Controlled | 16/16 通过 |
| 选定的远端 L3 | **10/10 通过**，分 2、3、5 项三个先后批次，各批最大并行度 2 |
| 完整远端 L3 | 注册 19 项；**9 项仍未验收**：App 1 项、管理员故障 8 项 |
| 本机 L3 | 本轮未执行，不借用历史通过记录计数 |

当前全 L3 注册 17 场景、24 case（远端 19、本机 5）。本次 scope 为 `selected-cases`，
三个批次无重叠、无重复 case，未使用 `SkipAdminRequired`；`authorized_exclusion_count=0`
不表示执行了整个注册清单。当前状态为
`selected_remote_cases_passed_full_remote_pending_inputs`，交付 PR 保持草稿。

归属、完整 case 清单、批次时间、前置检查及未验收项见
[qualification.json](evidence/orca-remote-extensions-20261018/qualification.json)。
三个原始批次的有限导出分别为
[targeted](evidence/orca-remote-extensions-20261018/phase-targeted/batch.json)、
[failure-regressions](evidence/orca-remote-extensions-20261018/phase-failure-regressions/batch.json)、
[regressions](evidence/orca-remote-extensions-20261018/phase-regressions/batch.json)。

| 远端 case | 结果 | 批次 |
| --- | --- | --- |
| stop-revocation / active | passed | targeted |
| environment-check.orca-sandbox / complete | passed，12/12 工具项 | targeted |
| smoke / default | passed，反馈及精确合入 | failure-regressions |
| environment-check.orca-full-access / complete | passed，12/12 工具项 | failure-regressions |
| multirun / default | passed | failure-regressions |
| audited / default | passed | regressions |
| full-access / default | passed，反馈及精确合入 | regressions |
| caller-exit / accepted-response-lost | passed | regressions |
| service-restart / graceful-active | passed | regressions |
| service-restart / kill-active | passed | regressions |

## 构建与执行归属

三个批次使用相同源码、Runner、native driver、远端 helper、固定 Codex、模型及原 case 预算。
运行前源码干净，结束后逐项核对 [1234 个冻结文件](evidence/orca-remote-extensions-20261018/source-files.json)未变化。

- Runner SHA256：`78d505133234de21ce9e52eab9abe5f1980aec886c3386fe09835837f1b2af89`。
- driver SHA256：`4f0ab768151c14aac424765cf69944856f40df7b0e45abdba35cc302d38e2d4a`。
- 远端 helper：`C:/sr-runtime/2ba0/35/steerrunner-agent.exe`，SHA256
  `4afff003df688a257b705c82a98ff7f5be6b613cf696b9a6282f9bb783736ec0`。
- 固定 Codex `0.159.2`：`C:/sr-runtime/2ba0/01/native/codex.exe`，SHA256
  `52f75c649bebb8001102a1dd129c1ea6d02b0940321e6d7e82ee0526753bd58a`。
- 执行模型 `gpt-6-luna/high`，审计模型 `gpt-6.1-sol/high`。
- environment：`e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`；各实际 Orca actor 绑定同一
  runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`。

[收尾执行主机核验](evidence/orca-remote-extensions-20261018/execution-host-runtime.json)再次确认
native 版本、native 与 helper 字节摘要。Orca CLI 使用同一安装路径，但收尾版本为 `1.4.224`，
早先记录为 `1.4.223`；未逐 case 固定其二进制，无法确定更换时点。因此本记录不提供固定
Orca CLI 版本的资格证明。该限制单独记录，不改写实际 case 结果。
稳定性 case 的 `source-identity.json` 中 helper/runtime 摘要属于故障夹具；远端 agent 摘要以
manifest 配置及原 readiness 的 `runtime_sha256` 为准。

## 新增能力与实际闭环

双模型审计在原 Codex TUI/server/thread 上切换冻结模型，原生 `turn_context` 提供实际
model/effort；发布恢复执行模型。桥接要求原输入摘要、原 thread 与内存 bearer capability，
拒绝新建、分叉、foreign resume 或其它业务输入。实际
[模型证明](evidence/orca-remote-extensions-20261018/phase-regressions/orca-remote.audited-default-r1/audit-models.json)
确认 `audit.spec`、`audit.code` 为 `gpt-6.1-sol/high`，`publish` 为 `gpt-6-luna/high`，
均属于 provider session `01a14e04-5353-71d3-994d-cdcfe5bd5b76`。
测试 PR [#491](https://github.com/erranliu/SteerlineTest/pull/491) 按审计 case 契约保留未合入。
finding、修复和预算分支由 Controlled 覆盖，L3 不强迫真实模型制造 finding。

同服务多 Run 使用一个真实服务及 state-dir，各自保留工作区、原生会话、PR 与认证 owner。
第一方 PR [#487](https://github.com/erranliu/SteerlineTest/pull/487)；第二方 PR
[#488](https://github.com/erranliu/SteerlineTest/pull/488)。停止第一方后，第二方原身份继续反馈，
head 从 `97d830c80aa048dd1cf8363c34cc131c6c533aa3` 更新为
`a25b0947e5c9657e3aceb3aad016719413b1ec2e`。
[停止后的原 peer](evidence/orca-remote-extensions-20261018/phase-failure-regressions/orca-remote.multirun-default-r1/peer-after-first-stop-facts.json)、
[反馈后的原 peer](evidence/orca-remote-extensions-20261018/phase-failure-regressions/orca-remote.multirun-default-r1/peer-feedback-after-first-stop-facts.json)
及两个独立认证清理证明均保留。两个 PR 按多 Run case 契约保留未合入。

普通两种权限模式均完成实现、固定 checker、commit/push、PR、同一原会话反馈、新提交、
精确 head 合入独立测试 base，以及原 Run/服务/认证清理：

| 模式 | PR | 反馈前 → 后 head | 实际合入 |
| --- | --- | --- | --- |
| sandbox | [#484](https://github.com/erranliu/SteerlineTest/pull/484) | `77224cdbd0ce` → `a8108acfbe99` | `1375c396bac00e16267230e4daf06ea48f07ebb7` |
| full_access | [#489](https://github.com/erranliu/SteerlineTest/pull/489) | `ba204743a2d6` → `16a4eb58ed61` | `bad1f8789133f39e33c202471dbe0b65ce664b93` |

对应有限 case 与原 native 回合事实位于
[sandbox](evidence/orca-remote-extensions-20261018/phase-failure-regressions/orca-remote.smoke-default-r1/scenario-facts.json)、
[full_access](evidence/orca-remote-extensions-20261018/phase-regressions/orca-remote.full-access-default-r1/scenario-facts.json)。
两种环境检查分别保留实际模式、原 session/turn/tool 事实、12/12 项及引用/PR/writer 清理；
不把确定性准备工具算作业务模型回合。

可选 App 的非秘密引用已贯通冻结配置、remote helper、最小单仓库 grant、broker、就绪检查与
原 owner 撤销。私钥只在执行主机按环境变量名解析，原 grant 留在可信 helper；worker 仅持
受限 capability。撤销、broker/socket、原辅助 actor 与业务 actor 分别核验；撤销未核实时
仍物理关闭 worker，但保留认证/停止未验证。issuer/broker、缺引用、目标/权限/期限、秘密边界、
撤销失败及未知辅助创建均有组件反例。**真实 App 正向闭环尚未验收，不能借用 gh case 通过。**

## 并行、失败修复与历史记录

实际最大并行度为 2，case 的现场、服务、分支和 PR 独立。实际外层 Orca CLI 请求使用同一
Windows 命名互斥；等待可取消，拥有者被杀可恢复，模型业务仍可并行。只有明确尚未调用的
排队观察允许重试。辅助创建取得互斥后、实际调用前才持久化 dispatch 意图；已经调用但未知
的创建结果继续禁止补建。停止后的最终 raw native 审计核对原输入及精确辅助 actor，
不以终端标题或宽泛 namespace 豁免。对应跨进程、PowerShell/Go、取消、强杀、checkpoint、
原 intent 恢复、未知创建及严格回执审计均纳入前置测试。

开发失败保持原结果；以下是前几个不同构建的批次，不能拼入当前 10 项通过：

| 构建 | 原结果 | 主要失败事实 |
| --- | --- | --- |
| `371f91d`（source32） | 7/10 | 多 Run 代理上游 502；强杀及活跃停止排队/收尾未核实 |
| `62aee13`（source33） | 已结算 4 项中 3 通过、1 失败；其余 6 项未派发，批次 aborted | 强杀恢复遇到排队前已写入的辅助创建意图，原观察无法继续 |
| `8b9a98a`（source34） | 5/10 | 活跃停止、smoke 认证未核实；多 Run 产物被冻结 checker 拒绝；两种环境检查只取得 7/12、8/12 |

逐项有限历史见 [source32](evidence/orca-remote-extensions-20261018/historical-source32/batch.json)、
[source33](evidence/orca-remote-extensions-20261018/historical-source33/batch.json)、
[source34](evidence/orca-remote-extensions-20261018/historical-source34/batch.json)。
旧文件锁失败不据此认定为继承句柄；source34 的通用认证错误及缺工具回执也没有精确根因证明。
`e492ec1` 增加有限诊断类别，不把诊断补充或当前成功解释为旧间歇失败已获得根因修复证明。
原失败认证/停止 unknown 保留；后续物理资源清单为空不把旧失败提升为通过。
更早探索批次及原始现场继续保留在任务 `.evidence/orca-remote/`，不覆盖或删除。

## 收尾与剩余验收

[最终原资源复核](evidence/orca-remote-extensions-20261018/remote-resources.json)覆盖 11 个
Run/check actor 范围：开发工作区终端为空；共享主工作区只确认本次两个检查 actor 已不存在。
[当前 10 个调用端服务](evidence/orca-remote-extensions-20261018/caller-services.json)及
[更早 17 个](evidence/orca-remote-extensions-20261018/historical-caller-services26-31.json)、
[随后 24 个](evidence/orca-remote-extensions-20261018/historical-caller-services32-34.json)均为 `not_running`。
旧失败的原资源物理结算单独保留。[自建暂存终端](evidence/orca-remote-extensions-20261018/staging-cleanup.json)
已关闭；Orca 移除成功且原 selector 查无，未另外核验移除后的文件系统路径。固定 native/helper
及历史版本保留在原安装目录，测试工作区/PR 按各自契约保留。

仍需以下前置才能完成剩余 9 项；未把这些项跳过后宣称完整通过：

- `orca-remote.app-auth/default`：需要执行主机 App ID、Installation ID 和私钥环境变量名；
  不需要提供私钥值或 token，不从调用端复制凭据。
- 管理员故障宿主：本机未提升，前一次 UAC 已由用户取消，尚无重试授权或手动 ready 宿主。
  对应 connection-reconnect 的 active/pre-send、service-restart 的 graceful/kill boundary、
  stop-revocation 的 boundary/unreachable、publication-unknown 的 completion-gap/query-unavailable，合计 8 项。

有限导出仅包含身份、摘要、状态、断言和清理事实；不发布模型正文、原始终端、数据库、私钥或 token。
