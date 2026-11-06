# Plan：权限资源与原完成证明的私有知识收口

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：P0–P4 完成，见[验收](../acceptance/native-proof-boundaries.md)；完整应用组的基线失败/时限耗尽不改记通过。依据 [Spec](native-proof-boundaries.spec.md)；基线 `95e53cb`，分支 `codex/native-proof-boundaries`。

## P0：设计审计和保护基线

- [x] 逐项核对 verifyPolicyResources/worker、policy decoder/profile/argv、originalCheckpoint/finish、签名 checkpoint/owner、Orca 工作区后备及共享夹具。
- [x] 捕获原跟踪源码/测试 SHA、持久根/代表 Run/Request/Receipt/能力编码；登记 Spec/Plan SHA 和审计结论，再编辑代码。
- [x] 核对两个边界没有授权退化：A 不省略 profile/资源匹配；B 不以可选端口缺失、文件缺失或缓存代替要求的原证明。

## P1：A——权限匹配唯一解释

1. 在 Codex policy_resources.go 新增纯校验函数，复用原严格 decoder；按包→profile→匹配次序执行，利用已验证 contract 与 mode 对应关系去除第二次 argv 解释。
2. application/policy.go 的已有包分支只调用该函数；原 nil/构造/路径/探测/提交保持。
3. 新测试覆盖两种模式、交叉模式、身份/格式/版本/参数失败及原诊断优先级，不改既有 access_mode/policy/ps2 测试。

## P2：B——原证明能力和业务来源分离

1. 在 Codex owner_checkpoint.go 添加窄只读函数；保留 ExecutionOwner 与 ReadExecutionCheckpoint 正文，明确 required/error。
2. 在现有 engine.go 静态组装区添加纯转接；workspace_source.go 只消费事实，保留原业务校验及所有后备分支。
3. 新测试经真实签名文件和应用入口验证：合同不适用才后备；适用且缺失/篡改/错身份始终等待，即使已有缓存；晚到原文件恢复；错仓库/非后代不推进。
4. 保留 auditEngine、ownerTestRequest、MockAppServer/StagedBusiness/history 等原夹具及全部既有测试字节。

## P3：审计和定向验证

- [x] 检查调用者/导入/私有标志，精确核对原函数和未授权文件/持久声明字节；补充业务文件边界守卫，防止私有知识重新渗入。
- [x] 比较代表编码基线；新增 5 项、组件 448 项及定向应用 57 项通过；完整应用组的原夹具缺 Budget 失败已用基线 overlay 复现，总时限耗尽单列。生产/测试冻结后构建。
- [x] live 编译、正式 Runner 构建；同 EXE 三项公开受控场景通过，记录实际名称/数量、失败/skip 与产物 SHA。未扩大为全后端真机验收。

## P4：收尾

- [x] 写验收报告及源/编码/测试摘要；更新长期产品文档、docs 与 acceptance 索引。
- [x] 说明删除的是私有解释和跨层识别，A/B 不以净删/提速衡量；持久格式、执行/恢复/停止承诺保持。
- [x] 初轮检查 diff、格式和工作树范围；保留未提交改动供用户审阅。后续新增授权见 P5。

## P5：实现再审计与 PR（用户追加授权）

- [x] 重新核对 A/B 私有解释、required/error、结果提交/停止先到、实际消费者/共享夹具及持久格式；未发现产品缺陷，原生产实现保持。
- [x] 同步 main `da8155e` 的测试入口，新增实际 finish 边界回归；33 个定向、live 编译、正式构建及最新 EXE 三公开场景通过，原失败/时限边界保持单列。
- [x] 保存[再审计记录](../acceptance/native-proof-reaudit.md)及原字节证据保护；按本次授权提交、推送、创建 PR，不合并或部署。
- [x] 提交前进一步整合固定 main efa871c，保留 A/B 实现并区分上游 capabilities/夹具变化；38 个定向及同最终 EXE 三公开场景通过，原缺预算测试随上游修复后定向通过。
