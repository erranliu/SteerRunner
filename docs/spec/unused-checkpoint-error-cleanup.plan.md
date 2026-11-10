# Plan：零消费者 checkpoint／错误声明清尾

日期：2026-11-10。状态：设计，未实施。依据 [Spec](unused-checkpoint-error-cleanup.spec.md)，调查主线 `5ccc3f0aa07764657b3346aa6023b73b878d76e0`；本地 HEAD `f3668b37` 文件树相同。原设计轮仅写文档，以下代码修改、命令及验收均未执行；用户后续仅授权提交本设计并创建文档 PR，未授权实施、合并或部署。

## P0：基线及实际消费者复核

- [ ] 后续获得实施授权后刷新 main，记录 HEAD/目标提交、工作树和实际差异；上游变化先核对。按 AGENTS.md 处理真实冲突，不为 Rebase 提示改写已验证历史。
- [ ] 全仓核对 controlCheckpoint/initialTerminalCheckpoint/providerError 的直接引用与函数值，以及 publication.Error 的包内构造、导入 alias、类型断言/errors.As/反射注册。包含 cmd/tests/scripts/helpers/experiments、生成代码、各 build tag 与平台源文件；区分方法实现与调用。发现真实消费者先修订 Spec，不迁移/删除消费者来维持零引用结论。
- [ ] 核对 inspectionGate 的真实审计调用与字段、原 context 取消/释放，追踪当前 creation owner、frozen reader/停止、ControlTerminal/RemoteControl*、application 的 provider 装配/结果核验。记录实际入口，不能因文件名或类型名含 checkpoint/Error 就合并清理。
- [ ] 保存三个原文件、引用/入口记录和源码/全部测试/JS/脚本/driver/helper 清单及 SHA；未来独立验收逐项对照原字节。登记本批没有测试消费者需迁移，现有全部测试与共享装配保持；不新增声明存在性或删除镜像测试。
- [ ] 实施前捕获保留声明、固定 capabilities/schema/resource_digest 及原真实持久格式的代表编码：Record/operation、ControlTerminal/State/Profile、RemoteControlRequest/Response/Setup、creation plan/started/result。私有类型可复用现有固定夹具/一次性捕获；不为取证增加生产 API。明确排除批准删除的 controlCheckpoint/initialTerminalCheckpoint/publication.Error，原定义另存，不要求整个类型集合不变；所有编码/读取错误检查，不忽略失败。
- [ ] 在 Windows/amd64 运行 P2 的原十一项 adapter 定向和完整 publication，保存 JSON、stderr、退出码、实际顶层/子例及失败/skip 为原基线。原失败先记录，不改写为通过、不通过修改原断言压过失败。本轮不执行这些命令。

## P1：按批准闭包删除并静态对照

1. control_checkpoint.go：删除 controlCheckpoint 及独占注释、codex import；只对 import 块作必要 gofmt 收缩，原路径/context/inspectionGate 正文保持。不得删除文件或 inspectionOnce/inspectionSerial，不迁址、改名或重新实现锁。
2. workspace_initial_terminal.go：确认全文件仍只含该无消费者类型/独占 import/旧注释后删除文件。保留所有其它 workspace/creation 源码、原持久数据/现场和 owner 认领机制，不新增磁盘清理动作。
3. publication/provider.go：删除 Error 类型、(*Error).Error、providerError 和附属空行；保留现有 imports、其它声明、Query/validateKind 正文。真实命令/HTTP/身份错误和脱敏不映射成其它新类型，不动 application 消费者。
4. 全仓重新核对符号、构造、类型注册/断言和运行入口，无同义副本、兼容壳或为了清理而删除测试。对剩余生产正文逐字比较；允许差异仅三个文件中的批准删除和 import 格式，workspace_initial_terminal.go 是唯一预计删除文件。
5. 对照测试/driver/helper/JS/脚本原摘要，全数保持；对照保留声明和代表编码、公开能力/资源摘要相同，批准删除三组声明的集合变化单独登记。若发生额外生产/测试变更，停止扩展并修订范围；不夹带 Orca CLI/nativeAccess 或错误体系重构。

## P2：最小实施验收

仓库根目录 PowerShell 7、Windows/amd64；以下是未来实施清单，本轮不运行。Go count=1；原基线与删除后的输出分别落到本批新目录，不覆盖历史结果。每条独立检查退出，保存原失败/skip，普通宿主复验不抹去沙箱/环境失败。

### P2.1 Adapter 十一项现有定向

```powershell
go test ./internal/execution/adapters/orcatransport -run '^(TestStructuredWorkspaceAuditReadsOriginalChangedTree|TestFrozenReaderHasNoExecutionAndRejectsMutation|TestFrozenReaderKeepsConstructorFaultPriority|TestRetiredExecutionFactoryKeepsFrozenProof|TestHostNativeStateCodecRejectsRetiredAndAmbiguousFields|TestHostNativeStateCodecRoundTripRetainsOriginalBinding|TestHostNativeStateCodecRetainsWaitingWithoutRecomputing|TestStructuredWorkspaceRecoversOriginalOwnerOutput|TestCreationOwnerUnknownAndForeignEvidenceNeverRedispatch|TestCreationOwnerIncompleteLaunchClaimNeverRetries|TestCreationOwnerStartupUsesOriginalDeadline)$' -count=1 -timeout=5m -json
```

十一项名字在调查基线均存在，要求实际执行、集合精确匹配且保留全部子例；零匹配不能判通过，Windows 审计正例不能因平台跳过冒充通过。不改名、不缩减矩阵，不预报子例数量。

核对当前结构化审计实际经过原 gate，原 worktree 身份/前后读取替换拒绝保持；冻结 reader 无 mutation、原工厂拒绝旧执行但原 scope 可核实、codec 原绑定/等待保持；真实 creation owner 原输出、未知/错来源零重派、不完整认领不重试和原启动期限保持。该集合不是完整 adapter、锁公平性/压力或真实 Orca 资格；锁原正文/字段/调用位置还须与 P0 原字节核对，不能把单个审计正例写作新并发认证。

### P2.2 完整 publication

```powershell
go test ./internal/execution/publication -count=1 -timeout=5m -json
```

调查基线有九个顶层，实际集合按运行日志核对，原全部子例保持。特别检查精确 PR 仓库/base/head/merge 证据、Gitea 原 head 身份、所有错身份诊断、引用凭据脱敏、配置秘密拒绝，以及 unsupported provider 的原 HTTP 零请求断言。命令早拒绝按原 Query/validateKind、Check 的前置返回正文逐字保持核对；该测试的 failingCommandRunner 没有调用计数，不能据此报告 CLI 零调用实测，也不为本次未改行为新增计数测试。测试仅使用原隔离 HTTP/命令装配，不访问真实 PR 或服务；不能用一个替代错误类型的正例取代原错误边界。

### P2.3 正式构建与最终身份

冻结生产删除点及未变测试后构建：

```powershell
./scripts/build.ps1 -OutFile .evidence/unused-checkpoint-error-cleanup/final/steerrunner.exe
```

保存脚本实际退出、构建记录、EXE SHA、source_commit/source_dirty/source_digest 及最终源码/测试清单；确认最终输入与通过点相同。正式构建覆盖 application 与实际 provider/Orca 生产装配的编译，不替代 application 测试或公开运行验收。公开 capabilities/schema/resource_digest 与原值比较，不要求源码/EXE 哈希不变，不回填历史记录。

本项只删除没有消费者的声明，不修改运行规则，因此不额外安排公开受控、完整 adapter/application/protocol、L1/Controlled、L2–L4、live driver/helper 单独编译、Node 或真实模型/Orca/PR。源/依赖复核一旦发现新增实际影响，先修订范围并按 docs/testing.md 补选；不能将最小集合当作扩大 diff 的通行证。通过项仅因新变更、失败或未解问题重跑。

## P3：独立验收与结束

- [ ] UCE-01–06 在 docs/acceptance/unused-checkpoint-error-cleanup.md 独立登记：引用/合法入口证据、删除闭包、保留方法/格式/错误、测试与共享工具未变证明、原/新基线及实际集合、命令/退出/失败/skip/未验、源码和产物身份。需要保留原证据时沿 .gitattributes 保护原字节，不替换原验收。
- [ ] 复核生产只改批准三文件、仅一个预计删文件；没有测试迁移、无新旧声明镜像，没有删除历史磁盘数据或误删 inspectionGate/RemoteControl*/creation owner。固定保留编码/声明和公开资源摘要相同；声明集合的批准减少单列，不伪称所有原声明均存在。
- [ ] 实际验收后才更新 Spec/Plan 状态、索引及必要长期实施说明；本轮仅设计，不预写实现完成，不给中央 README 验收表插行，不开展目录/归档大整理。
- [ ] UCE-01–06 完成即结束，不自动进入 CLI 恒真参数等其它批次，不提交/推送/创建 PR/合并/部署。若后续另获 PR 授权，创建和最终交付前刷新目标并核对 GitHub 实际文件合并状态，优先 Squash and merge，不因 Rebase 提示改写已验证历史。

## P4：设计 PR 交付说明

2026-11-10，按用户后续提 PR 授权，仅提交 Spec/Plan 与索引。已刷新并核对 main `91507d7e78c4e89598a0f20a967062d888104a3f` 的 #137 变更，消费者和测试声明保持，详情见 Spec §6；全部实施复选框仍未执行，不将静态复核或上游验收记为本批通过。
