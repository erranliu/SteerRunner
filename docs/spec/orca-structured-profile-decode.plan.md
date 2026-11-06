# Orca 结构化 profile 单次解码 Plan

2026-11-06：步骤 1–7 已完成实施及合并前验收，PR #123 复审发现的测试空断言已修复并按影响复验；实际合入状态以 [PR #123](https://github.com/erranliu/SteerRunner/pull/123) 为准，未部署。[Spec](orca-structured-profile-decode.spec.md)；[首次验收](../acceptance/orca-structured-profile-decode.md)；[最终候选审计与复验](../acceptance/orca-structured-profile-decode-review.md)。

1. 实施前刷新主线，在隔离分支记录 `NewLifecycle → decodeFrozenRecord → decodeLifecycleProfile/decodeAgentSessionProfile` 的实际调用和首错顺序，保存受影响包基线。区分当前执行、结构化冻结读取及旧 terminal 只读路径，确认成功路径原三次/两次调用，不能把失败时未执行的后续调用计入。
2. 在 `decodeFrozenRecord` 的原前置检查之后分流：结构化入口一次调用原完整解码器，填充原 Transport/version/host/development 及新增私有 audited 事实；旧记录仍调用原 `decodeLifecycleProfile`。NewLifecycle 保留原旧协议拒绝，直接消费这些已验证事实。补齐 `Lifecycle.frozenRecord()` 的字段投影，不改签名、解码器内部、读取白名单或 client/clock/probe 初始化。检查没有新增缓存、通用工厂、策略接口或重复完整 profile 存储。
3. 复用现有配置夹具补构造结果等价断言，独立核对原派生字段，特别是 audited=true 与 development=true/audited=false 的区别；模型/effort 等仍留在原完整解码与调用方合同中。按既有 Clone 语义验证原 setup 未变、已克隆配置不受原配置后续修改污染、多次构造分别校验本次输入，不扩大共享 ProcessHost 的拷贝合同。补结构化多故障首错用例，比较原错误类型/分类及诊断；现有 `TestFrozenReaderKeepsConstructorFaultPriority` 主要覆盖旧 terminal，不能单独当作结构化首错证明。需要时在原主线使用相同测试夹具对照，不放宽预期值。
4. Windows 执行 `go test ./internal/execution/adapters/orcatransport -count=1`。重点复用 `TestFrozenReaderHasNoExecutionAndRejectsMutation`、`TestFrozenReaderKeepsConstructorFaultPriority`、`TestRetiredExecutionFactoryKeepsFrozenProof`，并回归当前结构化创建/恢复、续轮、模型选择、audited 与 close v1/v2 的原成功/未知和零重放断言。旧记录的 scope/VerifyStopped 保持原证明强度，读取或构造不派发 mutation；保留 usage/progress/result 及原严格编码测试。只补已有夹具实际缺失的断言，不新建执行模拟器。
5. 以 diff 和调用图核对每次结构化构造只经过一次 `decodeAgentSessionProfile`，NewLifecycle 不再直接调用它；保留解码器内部全部子校验，旧 terminal 路径不变。该检查不需要可替换的生产 decoder、全局计数器、benchmark 或性能结论。审计派生字段在执行对象与 frozen 视图之间完整传递，没有把配置事实变成新执行权限。
6. 固定同一候选源码和 EXE，复用现有本机/已配对远端 Codex `execute/1` 正式入口，核对原创建、会话、可信结果、原 close、result-file/v1 和原目录双 EXE 排空。历史记录证明使用已冻结原目录或原夹具冷读，旧执行继续明确拒绝；不以启动旧终端/helper 来获得证据。Pi/audited 的原构造与行为回归单列，未运行其真机时不得借双端默认执行声明通过；不安装远端 Pi，不修改 Orca。有环境缺口如实登记，门禁保持。
7. 在独立 `docs/acceptance/orca-structured-profile-decode.md` 回填实际基线、变更范围、字段与首错对照、命令/计数/跳过、同候选双端及旧记录证明，区分定向/完整、合并前/后与真机资格。审计发现问题按影响修复复验，不把历史 #121 证据追认为本候选通过。提 PR 和最终交付前刷新主线，按实际内容冲突处理；完整发布资格沿现有测试与排空要求。

实施回填：基线刷新至 `5b5487b56715dfa3a2bbbd9d4c2d207b6647e555`（#122），隔离分支 `codex/orca-structured-profile-decode`。生产仅改两处构造及私有字段投影（+14/-11）。主线原包 73 顶层/261 叶、候选最终包 75 顶层/282 叶，均零失败/零跳过；同一新增构造夹具在原生产代码 overlay 上 2 顶层/21 叶通过，原版没有的 frozen audited 字段断言只在候选执行，其余预期不变。通过静态调用图确认当前执行由三次、结构化冻结构造由两次完整解码降为一次，旧分支未改。双端实际新 Run、原关闭、result-file/v1 和原目录生产升级排空通过；源码摘要一致，未重发未知输入，未启动旧执行。Pi/audited 仅组件回归，未申报真机资格。审计中的测试封套摘要夹具已在固定候选前纠正，未发现运行逻辑缺陷；完整 L1/Controlled、L2/L3 未执行。
