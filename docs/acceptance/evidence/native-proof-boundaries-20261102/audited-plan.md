# Plan：权限资源与原完成证明的私有知识收口

日期：2026-11-02。状态：P0 审计通过，待实施。依据 [Spec](native-proof-boundaries.spec.md)；基线 `95e53cb`，分支 `codex/native-proof-boundaries`。

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

- [ ] 检查调用者/导入/私有标志，精确核对原函数和未授权文件/持久声明字节；补充业务文件边界守卫，防止私有知识重新渗入。
- [ ] 比较代表编码基线；新测试、受影响组件及 application/storage 回归通过后再冻结源码。
- [ ] live 编译、正式 Runner 构建；同 EXE 三项公开受控场景，记录实际名称/数量、失败/skip 与产物 SHA。仅新失败或新修改引发必要复验，不把专项扩大为全后端真机验收。

## P4：收尾

- [ ] 写验收报告及源/编码/测试摘要；更新长期产品文档、docs 与 acceptance 索引。
- [ ] 说明删除的是私有解释和跨层识别，A/B 不以净删/提速衡量；持久格式、执行/恢复/停止承诺保持。
- [ ] 检查 diff、格式和工作树范围；保留未提交改动供用户审阅，不自行推送或提 PR。
