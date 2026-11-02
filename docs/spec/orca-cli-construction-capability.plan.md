# Plan：Orca 公共命令构造 B2a

日期：2026-11-02。状态：P0–P3 完成，独立审计未发现确认的 P0/P1/P2；见[验收](../acceptance/orca-cli-construction-capability.md)。依据 [Spec](orca-cli-construction-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。B1 的实施后审计通过，无确认问题。

## P0 固定兼容基线

记录 c9097a7 中日常调用、creationPlan 新建/读取及 envelope 解码的参数顺序、nil slice、环境屏蔽、错误及 JSON 字段。只允许新增纯能力、消费方窄值映射、替换这五处重复；测试与文档单列。

保留 creationPlan/creationResult/creationStarted 和 Profile/State/Target 的所有声明。新格式测试用旧实现的字面量编码作 oracle，不用新构造器再生成期望值。

## P1 纯能力和消费方接线

1. 在 protocol/orca 定义最小 CLICommand（executable、args、blocked environment）和 CompileCLICommand。保持前缀→操作→显式 environment 的原顺序，返回新 slice，不引入默认目标、shell 或自动 retry。
2. 把 Result、Envelope 和 ParseEnvelope 移入无执行包依赖的基础解码文件，新增 DecodeCLIResult(raw, uncertain)；此值只表示调用方已知的不确定事实，不推断派发资格。
3. 在 orcatransport 增加小型 LaunchCommand 值映射，将工作目录/timeout 加回执行边界。Lifecycle 的长度校验、准备/普通 queue 分支及回调不动。
4. creationCall 新计划和 readCreationPlan 复用同一构造映射，保持原绝对 executable 和命令字段；普通 Client 和 owner 原结果复用基础解码，保留各自调用错误组合。
5. 删除原三处参数/屏蔽重复、owner 专属 Result 解码。ServeCreationOwner 的 marker、输出、原版本/摘要/期限与一次执行不改。

## P2 定向受控验证

新增 `TestOrcaCLICommandConstruction`、`TestOrcaCLIResultDecoding`、`TestOrcaCLICommandEncodingAndConsumers`。前两项只用 Orca 值和原始字节；第三项捕获真实消费方的 LaunchCommand，并断言 local/remote 固定旧 JSON 和 hash、owner 原持久计划仍可读取，配置/target/state 无变化。

回归入口：全部 TestCreationOwner*、TestOrcaCLIFIFOAndCanceledTicket、TestOrcaPreparationQueueBeyondOldExecutionLimit、TestOrcaQueueErrorDoesNotParseEmptyOutput、两个跨进程 CLI lock tests、TestPreparationRevokedBeforeOwnerClaimSettlesWithoutDispatch、TestNativePromptSharedQueueWithStopAndCancellation、两项 StateCodec、原本机/远端 workspace 创建与恢复 tests。使用 go test -json，保存实际命中数、失败/skip；本机子进程/WS 需要时使用正常权限，沙箱失败单列。

先完成新增定向，再跑上述必要回归及 workflow 的 TestSRExecutionDependencyBoundary。B1 刚跑过的未触及大型真实现场不重复；若出现额外失败或超出白名单，再扩展相应验证，不能因为整包已通过而省略命令/编码 oracle。

## P3 收口

- 记录 OCC-01–05 对应证据、实际消费者、删除清单和未迁 backend 模块，写 `docs/acceptance/orca-cli-construction-capability.md`。
- 检查生产 diff 不含 profile/state/target/创建持久类型、队列实现、回调时机、业务正文或停止逻辑变更；git diff --check。
- 同步长期路线及文档索引。提交实现后独立只读审计一次；确认修复独立 commit 并补相应验证。不提阶段 PR。

只有所有消费方迁入、重复实现已删除、旧格式和派发边界验收齐备且独立审计完成，才标 B2a 完成。
