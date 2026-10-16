# 无人执行权限与 Git 环境检查验收

日期：2026-10-16。独立归属 Runner 前置 [Spec](../spec/environment-check.spec.md) / [Plan](../spec/environment-check.plan.md)。仅规划，未实施。SteerLine 接入有其自身验收；本文件不共用调用方验收，也不覆盖 SA01–SA08 历史结果。

## 自动项

| ID | 通过条件 | 场景 | 状态 |
| --- | --- | --- | --- |
| SREC-A01 | access_mode 同源控制检查/普通业务，never/无审核回退；按 Spec §2.1 核对 SA 最终提交/原交付门槛、模式作用域及认证来源；full_access 预检由本功能补齐，不循环依赖 SA；旧执行不静默改模式 | SREC-RULES、权限认证回归、SREC-CLI、A06 原生证据 | 未实施，待验 |
| SREC-A02 | 三命令参数/v1 信封/退出码、CheckReceipt 与两种 Inspection（含原 stop 和无 Check 拒绝）、权威 input_digest/单调 revision/必需项清单一致；受理后环境失败仍是可查询报告；公开样例由 Controlled 校验，缺字段/伪造/错身份不通过 | SREC-RULES、SREC-CLI、A06 实际工具来源 | 未实施，待验 |
| SREC-A03 | 实际 Agent 创建检查 owned base/head，完成读取/fetch/commit/push、输入 Publication 所要求的 PR 创建/查询/关闭；不对业务 base 建测试 PR；每个外部写动作原意图/目标/核验完整，未知和同轮重复不重复创建，无覆盖/无 merge；缺认证不改不适用 | SREC-RULES、SREC-CLI、A06 实际 Agent/Git/PR | 未实施，待验 |
| SREC-A04 | stop/超时/重启/迟到/服务停服包含 Check 资源；无自动模型重建/权限回退，清理按身份/SHA，能力和结算分开，无法清理保留残留 | SREC-RULES、SREC-CLI、A06 原生收尾 | 未实施，待验 |
| SREC-A05 | 能力声明与现场就绪区分：远端缺认证/同权限预检/工具机制为 unsupported，已实现但现场缺凭据/401/权限/网络问题为 failed 或 unverified；full_access/模型不绕过 SA07；inspect 零副作用，未知无目标不收猜测 stop | SREC-RULES、SREC-CLI | 未实施，待验 |
| SREC-A06 | 声明支持的目标/模式各自真实 Agent 工具与权限/Git/PR 闭环、零审批、原对象及收尾证据齐备；不互相替代 | SREC-NATIVE | 未实施，待验 |

所有命令、环境、时点和证据位置见 Plan 测试计划。无人工阻塞项；缺凭据、原生接口和真实检查失败属于自动项阻断，不能改记人工后置。

L1/Controlled 只证明规则和公开协议；A01–A04 涉及实际 Agent/权限/Git/资源的条款必须关联 A06 对应目标/模式的原生证据才能完整签收。一个组合未验不记为该组合通过；本机已验组合可独立交付，不借用远端或另一模式结果。SA 自身原生/业务门槛仍由 SA 的固定验收承担，本文件不能覆盖或豁免。

## 目标组合与前置

| 计划场景 | 前置 / 当前结论 |
| --- | --- |
| environment-check.local-sandbox | 真实集成先满足 SA 自身交付门槛并固定合入提交；每次输入前需实际 SA01–SA04 准备有效。当前有效凭据正例/SA 所要求 L2 smoke 阻断，未实施/待验 |
| environment-check.local-full-access | 依赖已发布的共用 SA01/SA03 认证 owner；本功能新增全访问同模式无模型预检/权限证据，不依赖 SA 实现全访问。当前未实施/待验 |
| environment-check.orca-sandbox | SA07 执行主机认证及同权限确定性预检、实际工具/操作关联证据；当前待前置，保持 unsupported 设计 |
| environment-check.orca-full-access | 同样要求真实远端身份/认证来源/工具/操作证据，非沙箱专属项按模式适用性；当前待前置，不以放开沙箱豁免 |

## 文档与实施记录

2026-10-16 从 SteerLine 跨仓草稿拆出 Runner 前置；本仓只新增 Spec/Plan/本文件及 README 导航。没有修改代码、沙箱认证工作树、服务或外部资源。文档静态检查通过：复用 SteerLine 的 Markdown 检查脚本、切换扫描根到本仓，29 份 Markdown / 133 个本地链接 / 0 错误；本仓 `git diff --check` 和新文档尾空白/冲突标记检查通过。未新增检查脚本；产品项均未执行。

实施后逐批登记源码/dirty、EXE version/SHA/resource_digest、原请求/check/原生身份、执行命令及非零场景数、远端前置是否落实、实际对象与清理、证据目录。秘密不进入仓库；PR 历史保留，真实场景只用 SteerlineTest 本次专属分支。

## 拆分后复审（2026-10-16，仅文档）

本轮核对两仓 Spec/Plan/验收与在途 SA 记录，修正：前置时点与 SA 原交付门槛不明确、全访问预检反向依赖 SA 的风险、unsupported 与缺现场认证混淆、stop 原请求/受理后失败返回不完整、Publication 适用性缺来源、driver 的 base 操作可能冒充 Agent 能力。Spec §2.1/§4.1 为对应权威；Plan 与 SREC-A01–A06 已同步。

前置证明必须记录 SA 固定提交、原验收结论及复用适用性；无效 token 原生负例不证明真实认证可用，单凭 Codex 版本相同不复用已变化的权限证据。driver 的 fixture source/base 与 Worker 的 `steerline-check/<check_id>/base|head` 分开取证。A01–A04 原生条款须关联 A06 对应组合实测，Controlled 不替代真实能力。

未增加产品验收通过记录，未修改在途 SA 工作树；本轮静态复验通过：本仓 29 份 Markdown / 133 个本地链接 / 0 错误、`git diff --check` 与新文档尾空白/冲突标记检查通过。只执行文档检查，没有运行产品/原生测试或外部操作。
