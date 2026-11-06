# Orca 结构化 profile 单次解码 Spec

2026-11-06：已按本 Spec 实施并完成合并前受影响包、同候选双端 execute/1 及原目录排空验收，见[首次验收记录](../acceptance/orca-structured-profile-decode.md)和[PR #123 审计修复复验](../acceptance/orca-structured-profile-decode-review.md)。实际实施基线为 `5b5487b56715dfa3a2bbbd9d4c2d207b6647e555`（含 #122）；两处原构造及严格解码器与下述调查基线一致，#122 配置准入和公共接口保持不变。实际合入状态以 [PR #123](https://github.com/erranliu/SteerRunner/pull/123) 为准，未部署，不代表完整发布资格。

以 `main=d58054a78471211c1eeb86c0eb1396ae870d6ca1` 为基线，承接 [#121 冻结读取边界](https://github.com/erranliu/SteerRunner/blob/d58054a78471211c1eeb86c0eb1396ae870d6ca1/docs/spec/orca-frozen-reader.spec.md) 和 [旧执行退役合同](https://github.com/erranliu/SteerRunner/blob/d58054a78471211c1eeb86c0eb1396ae870d6ca1/docs/spec/orca-legacy-execution-retirement.spec.md)。当前成功构造结构化 Lifecycle 时，`decodeLifecycleProfile` 内部、`decodeFrozenRecord` 及 `NewLifecycle` 共调用三次 `decodeAgentSessionProfile`；冻结 reader 的结构化构造调用两次。本 PR 只消除这两个构造入口对同一克隆 setup 的重复解码。

`decodeFrozenRecord` 保留 agent 检查、setup.Clone、ProcessHost 检查。随后按原协议选择：结构化配置直接调用一次完整 `decodeAgentSessionProfile`，从结果填充现有记录视图；旧 terminal 配置仍走原 `decodeLifecycleProfile`。`NewLifecycle` 保留旧协议前置拒绝，再从同一次解码产生的 frozenRecord 取得所需事实，不再解码 profile。

沿用现有私有类型与函数签名，在 `frozenRecord` 补一个私有 `agentAudited` 布尔字段，直接来自已验证的 `AgentSessionProfile.Audited`。既有执行视图 `Lifecycle.frozenRecord()` 同步投影该字段，保证视图一致；不新增结果类型或保留另一份完整 profile。旧 terminal 读取沿原流程，其结构化派生字段保持零值。

| 边界 | 等价要求 |
| --- | --- |
| 严格解码 | 单次指一次完整 `decodeAgentSessionProfile` 调用，不删除其内部封套归属、重复配置、codec、严格 JSON、Transport、host/target、agent/access、Audited/model/effort 与已退役字段检查。原解码器及 `decodeLifecycleProfile` 本身不改。失败不能返回可用记录或执行端口。 |
| 错误顺序 | `NewLifecycle` 先拒绝非结构化协议，再进入 frozen 构造；不能先解码或启用旧执行。frozen 构造依次保留 agent、克隆、ProcessHost、原 profile/身份校验；多个故障同时存在时仍返回原首个错误及类型/分类。解码器内部错误优先级不重排。 |
| 派生事实 | profile=`p.Transport`、profileVersion=`p.version`、agentHostID=`p.PairingHostID`、agentDevelopment=`p.Development`、agentAudited=`p.Audited`，均来自同一完整结果。Audited 不从版本、model 或 development 推断；本机空 host、远端原 host 和所有原合法/非法组合沿基线。ProcessHost、probe、克隆 setup、执行 clock/nativeAccess 与 client 初始化保持。 |
| #121 读写边界 | frozenRecord/newFrozenReader 不取得执行、输入、observer、feed、Job、上传或停止派发能力。保留原只读 CLI 白名单、队列、超时与配对 host 校验；旧记录仍能原样严格读取和重核验，不能信任 saved stopped 代替证明。旧执行入口拒绝、旧 close 成功/未知及当前停止 scope 均保持。 |

每次构造独立处理本次克隆输入；不跨调用缓存或共享可变解码结果，不假设前一次成功可替代后一次校验。生产修改限 `frozen_record.go`、`lifecycle.go` 的上述构造、必要字段及视图投影。保留所有调用方原行为；不顺带收敛 ProgressHost、usage、文件、目录、会话请求或停止版本解释。不修改 Orca、非 Orca Codex、持久字段/tag/codec、公开 wire、权限、工作流、停止、封存或发布门禁。

验收对照基线的构造结果、首个错误、源输入不变与零原生调用，而不是只断言构造成功。用本机 Codex/Pi、配对远端 Codex、development/audited 和旧 terminal 记录的原合法代表值核对全部派生字段；缺失、重复、非法 codec、坏 JSON、错身份/target/host 及多故障仍按原顺序失败。复用现有 frozen reader、旧执行拒绝、当前原子首消息/续轮/模型/停止回归；单次调用通过生产调用图与 diff 核对，不增加生产计数器或测试开关。

同一候选沿现有本机与配对远端当前 Codex 流程做正式执行及原目录冷读证明；旧协议仅使用已有记录/夹具读取，不能为验收重新启动旧执行。记录范围与跳过/阻断，真实双端默认执行不推导 Pi/audited 真机或全部组合资格。完整资格继续遵循 [测试指南](../testing.md)。
