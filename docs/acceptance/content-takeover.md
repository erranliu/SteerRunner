# 内容流程冻结配置接管修复

日期：2026-10-20。基线 c122d641677593d4b694e2eda7502001510d4e8b，与核验时 origin/main 相同。
根因：freeze 对内容流程保留零值 AcquisitionConfig，TakeoverFacts 却对全部 Run 无条件
Validate。旧内容单元夹具手工补 DefaultConfig，未覆盖正式 freeze 的真实形态。

生产修改限于 workflow 能力谓词、freeze 配置加载资格和 takeover 配置/owner 需求判断。
开发结果解释、report/parse 流程及实际 parser 意图继续严格校验；其他接管检查保持。
无公开字段、迁移、默认补写、兼容回退或新运行时。设计见
[内容流程 Spec](../spec/content-workflows.spec.md)及[恢复 Spec](../spec/service-generation-recovery.spec.md)。

验证使用定向 L1、现有 Generate/Conversation 受控场景和单回合真实 Codex 专项。
不运行 Full、不使用 SteerlineInstall/SteerlineRuntime；原 namespace 仅只读 verify-takeover。
构建和最终证据将在实测完成后登记；当前不宣称通过。
