# 无人执行权限与 Git 环境检查验收

日期：2026-10-16。独立归属 Runner 前置 [Spec](../spec/environment-check.spec.md) / [Plan](../spec/environment-check.plan.md)。已实施；本机 host-gh 两模式最终验收已完成，结论与证据见下方实施记录。SteerLine 接入有其自身验收；本文件不共用调用方验收，也不覆盖 SA01–SA08 历史结果。

## 自动项

| ID | 通过条件 | 场景 | 状态 |
| --- | --- | --- | --- |
| SREC-A01 | access_mode 同源控制检查/普通业务，never/无审核回退；按 Spec §2.1 核对 SA 最终提交/原交付门槛、模式作用域及认证来源；full_access 预检由本功能补齐，不循环依赖 SA；旧执行不静默改模式 | SREC-RULES、权限认证回归、SREC-CLI、A06 原生证据 | 本机 host-gh 两模式及规则/CLI 通过；范围见下 |
| SREC-A02 | 三命令参数/v1 信封/退出码、CheckReceipt 与两种 Inspection（含原 stop 和无 Check 拒绝）、权威 input_digest/单调 revision/必需项清单一致；受理后环境失败仍是可查询报告；公开样例由 Controlled 校验，缺字段/伪造/错身份不通过 | SREC-RULES、SREC-CLI、A06 实际工具来源 | 本机 host-gh 两模式及规则/CLI 通过；范围见下 |
| SREC-A03 | 实际 Agent 创建检查 owned base/head，完成读取/fetch/commit/push、输入 Publication 所要求的 PR 创建/查询/关闭；不对业务 base 建测试 PR；每个外部写动作原意图/目标/核验完整，未知和同轮重复不重复创建，无覆盖/无 merge；缺认证不改不适用 | SREC-RULES、SREC-CLI、A06 实际 Agent/Git/PR | 本机 host-gh 两模式及规则/CLI 通过；范围见下 |
| SREC-A04 | stop/超时/重启/迟到/服务停服包含 Check 资源；无自动模型重建/权限回退，清理按身份/SHA，能力和结算分开，无法清理保留残留 | SREC-RULES、SREC-CLI、A06 原生收尾 | 本机 host-gh 两模式及规则/CLI 通过；范围见下 |
| SREC-A05 | 能力声明与现场就绪区分：远端缺认证/同权限预检/工具机制为 unsupported，已实现但现场缺凭据/401/权限/网络问题为 failed 或 unverified；full_access/模型不绕过 SA07；inspect 零副作用，未知无目标不收猜测 stop | SREC-RULES、SREC-CLI | 本机 host-gh 两模式及规则/CLI 通过；范围见下 |
| SREC-A06 | 声明支持的目标/模式各自真实 Agent 工具与权限/Git/PR 闭环、零审批、原对象及收尾证据齐备；不互相替代 | SREC-NATIVE | 本机 host-gh 两模式通过；Orca/App unsupported |

所有命令、环境、时点和证据位置见 Plan 测试计划。无人工阻塞项；缺凭据、原生接口和真实检查失败属于自动项阻断，不能改记人工后置。

L1/Controlled 只证明规则和公开协议；A01–A04 涉及实际 Agent/权限/Git/资源的条款必须关联 A06 对应目标/模式的原生证据才能完整签收。一个组合未验不记为该组合通过；本机已验组合可独立交付，不借用远端或另一模式结果。SA 自身原生/业务门槛仍由 SA 的固定验收承担，本文件不能覆盖或豁免。

## 目标组合与前置

| 计划场景 | 前置 / 当前结论 |
| --- | --- |
| environment-check.local-sandbox | 真实集成先满足 SA 自身交付门槛并固定合入提交；每次输入前需实际 SA01–SA04 准备有效。SA owner 6818e71 与验收 40b4ccd 已随 177045c 合入；本机原生验收见下方 |
| environment-check.local-full-access | 依赖已发布的共用 SA01/SA03 认证 owner；本功能新增全访问同模式无模型预检/权限证据，不依赖 SA 实现全访问。本功能已实现全访问同模式准备；原生验收见下方 |
| environment-check.orca-sandbox | SA07 执行主机认证及同权限确定性预检、实际工具/操作关联证据；当前待前置，保持 unsupported 设计 |
| environment-check.orca-full-access | 同样要求真实远端身份/认证来源/工具/操作证据，非沙箱专属项按模式适用性；当前待前置，不以放开沙箱豁免 |

## 文档与实施记录

2026-10-16 从 SteerLine 跨仓草稿拆出 Runner 前置；本仓只新增 Spec/Plan/本文件及 README 导航。没有修改代码、沙箱认证工作树、服务或外部资源。文档静态检查通过：复用 SteerLine 的 Markdown 检查脚本、切换扫描根到本仓，29 份 Markdown / 133 个本地链接 / 0 错误；本仓 `git diff --check` 和新文档尾空白/冲突标记检查通过。未新增检查脚本；产品项均未执行。

实施后逐批登记源码/dirty、EXE version/SHA/resource_digest、原请求/check/原生身份、执行命令及非零场景数、远端前置是否落实、实际对象与清理、证据目录。秘密不进入仓库；PR 历史保留，真实场景只用 SteerlineTest 本次专属分支。

## 拆分后复审（2026-10-16，仅文档）

本轮核对两仓 Spec/Plan/验收与在途 SA 记录，修正：前置时点与 SA 原交付门槛不明确、全访问预检反向依赖 SA 的风险、unsupported 与缺现场认证混淆、stop 原请求/受理后失败返回不完整、Publication 适用性缺来源、driver 的 base 操作可能冒充 Agent 能力。Spec §2.1/§4.1 为对应权威；Plan 与 SREC-A01–A06 已同步。

前置证明必须记录 SA 固定提交、原验收结论及复用适用性；无效 token 原生负例不证明真实认证可用，单凭 Codex 版本相同不复用已变化的权限证据。driver 的 fixture source/base 与 Worker 的 `steerline-check/<check_id>/base|head` 分开取证。A01–A04 原生条款须关联 A06 对应组合实测，Controlled 不替代真实能力。

未增加产品验收通过记录，未修改在途 SA 工作树；本轮静态复验通过：本仓 29 份 Markdown / 133 个本地链接 / 0 错误、`git diff --check` 与新文档尾空白/冲突标记检查通过。只执行文档检查，没有运行产品/原生测试或外部操作。

## 实施记录（2026-10-16）

本会话用户已授权实施。起点 `c976cc4b5781c2ca65a6a85a47aed3ec44d305dd`，开始时工作树干净；
未修改或复制 `8876` 工作树。SA 最终 owner 为 `6818e711c8ee6874799411a00bdc11c014ea9c56`，
真实有效 gh broker / 本机 L2 smoke 通过记录为 `40b4ccd`，随 `177045c` 合入本次起点。
本次同步 SA Spec 的 sandbox 作用域，保留其历史结论；全访问的非 AppContainer 准备与 canary 单独验证。

已落实：独立 Check 聚合/commands 同事务、异步 start、两种只读 inspect、原 stop 回执、可查询拒绝、
固定输入/资源摘要与清单、单调 revision、冻结与生效权限、逐写操作意图、单次认证写门、独立原对象查询、
正常/取消/停服/重启收尾。未创建 development Run；普通执行和检查共用权限编译及 SA owner。
存储格式改为 v3，旧 v2 明确 unsupported_schema；不迁移或回退旧权限资源。

本机机制范围为 Windows Codex 0.159.2、HTTPS GitHub `.git`、host-gh repository broker，
`sandbox/full_access` 两模式。原有 App installation-token 认证继续用于普通执行；检查所需的该来源逐写门
尚未实现，因此检查中明确 unsupported。Orca 两模式缺 SA07 原生能力，确定性拒绝；未注册为已支持原生组合。
`Publication=command` 当前只支持 Github 固定 PR 契约，不能省略项目 Publication 来绕过 PR 必需项。

公开样例由 `TestEnvironmentCheckPublicFixtures` 使用生产持久化/查询路径生成并校验，作为资源内嵌。
Controlled 的 `TestEnvironmentCheckCLI` 从实际 EXE capabilities 消费同版样例并比对公开形状及固定清单。
13 份样例另经 JSON Schema 校验，包含 start/stop、两种 inspect、准备失败、无 Check 拒绝、unknown 与 stale。
见 [schema-validation.json](evidence/environment-check-20261016/schema-validation.json)。

定向规则/权限回归当前通过：`EnvironmentCheck|NativePermissions|ThreadStart|TurnStart|Authentication|SandboxReadiness|NativeExecutable`。
最终源码上的无模型 canary 两项通过，分别拒绝/允许工作区外写入，使用有效真实 gh broker；原写者均停止。
见 [sandbox](evidence/environment-check-20261016/boundary-sandbox.json) /
[full_access](evidence/environment-check-20261016/boundary-full-access.json)。

初轮真实测试暴露并修复 PowerShell 原生命令 stderr 处理及未知副作用错误结算；两个旧检查的原 base 引用
仍存在，不能借用中心宽权限删除。已用修复后的 Runner 对原请求作无模型对账，两份报告均为 residual，
原写者已停，本地 owned 资源已删；精确引用/SHA 及 offline 反例见
[initial-failure-reconciliation.json](evidence/environment-check-20261016/initial-failure-reconciliation.json)。
这些是失败现场，不能记为通过或全部清理完成。

下一轮本机两模式实际完成全部 12 项和 PR 创建/查询/关闭、按 SHA 删除两条引用、停止原写者及本地清理；
但通知解码错误把无 RPC ID 的 `turn/completed` 拒绝了，整体正确保留 unverified，没有借分项补记通过。
该缺陷已修复，并新增无 ID 完成通知及错 thread/turn/status 反例。
旧轮结果保留在 [development-rounds.json](evidence/environment-check-20261016/development-rounds.json)。
最终原生闭环使用新 fixture/check/PR 重新运行，不拼接旧轮分项。

当前本地构建：`0.1.0-dev.1`，EXE SHA256
`9049008c47b4ef11ba66ff0b0cb13330823b61fc409a387c6377e20fa679d021`，resource_digest
`efc0cf54a81cdb2be672ba6a7e5e6657c715e60b0b37756840cd25d012c6629e`。
来源仍为上述 HEAD-dirty；该身份记录验收时的本地构建。后续提交源码不改变此 EXE 的内嵌身份；当前尚未合入或发布，不能作为 SteerLine 已发布前置。
见 [构建](evidence/environment-check-20261016/build.json)；最终实现文件清单在交付时核对。

## 最终本机资格结果

最终同一 Runner SHA（`9049008c…`）和同一 native driver SHA（`e5034425…`）的两个独立原生 case 均通过：

| 模式 | 原 Check | 原生结果与对象 | 收尾 |
| --- | --- | --- | --- |
| sandbox | `2c02420a63633458a8570ca1ecb80681` | 12/12 必需项 passed，原模型回合 completed；[PR #334](https://github.com/erranliu/SteerlineTest/pull/334) 由实际 Worker 创建/查询/关闭 | 原两条引用按 SHA 删除、原 writer tree 已停、owned 本地目录已删；项目 source/base 未改 |
| full_access | `d5cca89aa844d2229cb517072b1b8f45` | 12/12 必需项 passed，原模型回合 completed；[PR #336](https://github.com/erranliu/SteerlineTest/pull/336) 由实际 Worker 创建/查询/关闭 | 同上；未合并 PR、未借用 sandbox/MXC 边界结果 |

资格汇总、固定源/EXE/driver 身份与两个完整报告：
[qualification.json](evidence/environment-check-20261016/qualification.json)、
[sandbox 报告](evidence/environment-check-20261016/sandbox-report.json) /
[身份](evidence/environment-check-20261016/sandbox-identity.json) /
[原生审计](evidence/environment-check-20261016/sandbox-native-audit.json)，
[full_access 报告](evidence/environment-check-20261016/full_access-report.json) /
[身份](evidence/environment-check-20261016/full_access-identity.json) /
[原生审计](evidence/environment-check-20261016/full_access-native-audit.json)。
两份原生历史均核实原 session/turn、12 次实际 model tool call、原 task_complete、零审批/权限申请事件。
历史只保存路径/SHA 与有限审计摘要，不复制含完整上下文的原生历史。

正式入口 `.evidence/environment-check-native-3` 的 L1 和完整 Controlled 前置均 exit 0，Controlled 精确 13 场景。
两个 native driver 原结果均 exit 0 / scenario_count=1 / passed=true。
原批次的包装层因访问 Go `omitempty` 省略的 `merge_commit` 字段而失败；
[原汇总](evidence/environment-check-20261016/original-final-batch.json) 原样保留，不改为 passed。
包装层已修复，使用同一 `test-native.ps1 -ValidateOnly` 逻辑只读复验两份完整原结果，
核对原 manifest/EXE/driver SHA、原 Check/上下文/全部分项与收尾，两项均通过；两个旧失败 case 反例均拒绝。
此复验没有重新运行模型/Git/PR，也没有拼接不同轮次分项。后续新现场默认调用同一修复后的结果校验。

本次本机范围的 SREC-A01–A06 由规则、CLI 与上述对应原生证据联合签收；Orca 和 App 检查不记通过。
[实现来源清单](evidence/environment-check-20261016/source-files.json) 固定了 360 个实现/测试/脚本/资源输入；
交付前重新核对，并在清单中包含包装脚本的修复版本。代码保持待评审工作树状态，未合入或发布。
首轮两个原 base 引用仍按前述残留登记保留；新资格现场均已完成自身资源结算。
