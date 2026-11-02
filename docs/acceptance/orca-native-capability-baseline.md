# Orca 无 helper 原生可行性 B0 验收

日期：2026-11-02。状态：实现、确定性回归和双端原生现场通过；实施后独立审计待完成。本报告依据 [Spec](../spec/orca-native-capability-baseline.spec.md) / [Plan](../spec/orca-native-capability-baseline.plan.md)，只证明[长期设计 B0](../spec/orca-capability-layer.spec.md#61-b0-的可行性证据与零旧链调用)，不授予 B5 真实 Runner 开发资格。

## 1. 源码和产物

设计提交 `9761ee0`；实现现场为该提交上的 B0 测试/脚本差异。双端最终现场的 source_digest 均为 `6b9a06d1f5bb52a77943bbaf470bae56c9784ec2df2549d3cb039aae9baf2eea`，测试 EXE SHA256 均为 `4e9c56c040502f8bae6244aa90bb44c1a11d4de213b8485ef3bc65dba1f61c8e`。实际 diff 只涉及新增探针 `_test.go`、原 live 模式分派、同一 opt-in 脚本及文档；生产 Go、配置、codec 和停止合同未变。

本机/远端 Orca 均为 1.4.242，调用方 CLI SHA256 为 `e7a6fa40a59d237cdb7cf32444df6660da850d1b31e4162634287d91fdfc3cba`。实际 Orca shell 中的 Codex 分别报告 0.159.2 / 0.162.1；命令解析均为 Function，函数定义摘要均为 `2b359180ad904e5ad5bdeaf04ecc1455dbba8de5cbdfdb4adf1d2d5bd64076d8`。该摘要描述启动命令定义，不冒称已冻结底层 native EXE。启动模型偏好为 gpt-6.1-sol/high，实际生效模型未独立核验。

原始现场、失败历史、构建身份、命令/回执、原输入查询页、State 和完整清单保存于[证据目录](evidence/orca-native-capability-baseline-20261102/README.md)。证据不含凭据、完整环境、认证帧或真实业务材料。

## 2. 双端结果

| 目标与现场 | 原 runtime | 原终端 / provider session | 结果 |
| --- | --- | --- | --- |
| 本机 local-02 | 8a0d209e-094d-4ec3-a923-5cf6c064df67 | term_3915dd9b-066d-4910-b262-cf6de3753c52 / 01a19bd6-f715-7b70-aaa8-05887f824b32 | 两轮 accepted、原生 started/completed、独立原输入及完整 final 均成立；实际 send=2、close=1；完整空清单 |
| 台式机64G remote-01 | 64bdec68-d3ed-4216-a894-0cc143ed4653 | term_1758a209-4e96-485b-bc9a-b285678298a2 / 01a19bd8-dcb8-77a3-92d5-89635e0e1ab0 | 同上；两轮保持原 provider/terminal/incarnation；同源同 EXE |

每个现场均为一个新业务终端，两次输入之间无 helper/control/history，无 text-only、bare Enter 或正文屏幕读取。第二轮不重给第一轮 nonce，实际 final 精确返回新 nonce 和第一轮记忆值。每轮实际 CLI 的原 terminal/request 与回执、State 及原输入查询相互吻合。

外部分配 fixture 的初始空 shell 和只读 tooling 元数据终端单独记录，均精确关闭并核实专属空清单，不计入两次业务输入；不关闭共享 Orca。worktree 为 fixture 资源，B0 只证明其原终端范围，不声称物理目录删除。

## 3. 确定性检查

- 新测试 `TestOrcaCapabilityProbeTarget/Evidence/Cleanup`：3 个顶层、26 个子例通过，0 fail/skip，原始 test2json 在证据归档中。
- 受影响的原提交、构造器原输入查询、完整清单及取消回归：11 个顶层、132 个子例通过，0 fail/skip。
- PowerShell parser 及 git diff --check 通过。新模式要求两轮原生证据，原 long-input 仍要求一轮且保留本机 helper 前置，原 controlled roots 不变。

清理反例覆盖关闭响应丢失后完整清单缺席可核实、仍活动、替换 incarnation、截断和缺身份，不重复 close。它们是 B0 终端能力测试，不代替后续 B5 的正式 Run 活动停止/持久化/离线核验。

## 4. 失败和未验边界

首场本机 local-01 的两轮模型业务和清理都通过，但脚本在 StrictMode 下读取被省略的空 failure 字段而失败，result.passed=false。保留该失败现场和旧源码/EXE 摘要；修正为显式诊断字段并增加序列化断言后，用 local-02 的全新终端/证据目录补验，未重发旧请求。最终远端使用相同修正源和 EXE。

远端预检中，CLI 不接受环境 ID 的 `id:` 前缀；同一已登记环境名 192.168.1.151 不可达。未在那里创建探针资源或派发模型。随后在任何远端 Run 创建前，选择已配置且可达的台式机64G，固定其明确环境名和 runtime。不存在活动 Run 的目标回退。

| 能力 | B0 已证明 | 差距或后继 |
| --- | --- | --- |
| 原输入、完整 final、同会话后续 | 本机/远端原生双回合通过 | 不等于 Workflow 业务交付 |
| 模型 | 冻结启动偏好 | 实际 model/effort 未独立核验 |
| workspace / audited | 未运行 | 原 managed 工作区/模型审计不能推定原生具备 |
| Git、push、PR | 未运行，未创建业务提交/PR | B5 需要真实开发和独立发布核验 |
| 停止 | 原终端关闭及完整原主机清单 | 不证明 helper/根进程、全部后代或凭据撤销；当前正式本机 scope 不变 |
| Runner 接线 | 复用生产 NewLifecycle 与原生证明，测试装配两次输入 | 无正式 Run 受理、DB 或 service verify-stopped 资格；B5 另验 |

## 5. 后继及审计

本机和远端的底层可行性已取得直接证据；B3 可消费这些已证能力及上述差距，不据此删除 managed 业务链。B1/B2a 仍按独立收益逐批实施。独立审计完成及确认问题修复前，长期路线 B0 保持“验证通过、待审计”，不标全部完成。
