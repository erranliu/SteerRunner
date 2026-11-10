# 零消费者 checkpoint／错误声明清尾验收

2026-11-10，合并前最小范围验收。依据 [Spec](../spec/unused-checkpoint-error-cleanup.spec.md) / [Plan](../spec/unused-checkpoint-error-cleanup.plan.md)，按用户“独立子代理审计、修正确认问题，然后实施”授权完成。设计 PR #138 已合入，实施前刷新主线为 `344d3e83eef2c34397d2cae6c02c6c95f805bd2f`；工作副本 HEAD `13f3500b8d31ff77d46bf786160582a25120d04c` 与该主线文件树相同。本记录冻结时尚未提交、推送、创建新 PR、合并或部署；用户随后授权提交并创建实现 PR。

## 独立审计与实际消费者

独立只读子代理审计 Spec/Plan，未确认需修订问题；实施后窄复核也未确认新问题。两次审计均核对实际源码，不编辑文件、不运行测试，不作为新增运行资格。

controlCheckpoint、initialTerminalCheckpoint 在生产/测试/cmd/scripts 中均只有声明；没有构造、类型注册、编解码或函数值消费者。publication.Error 的唯一构造点是 providerError，后者没有调用；包内外没有该错误类型断言/返回路径。production 的 publication 导入者仍只有 application/freeze.go 与 results.go，分别消费真实 provider 装配和结果核验，未消费删除类型。原查询与[删除前引用记录](evidence/unused-checkpoint-error-cleanup-20261110/before-references.txt)保留；删除后运行源码无旧符号，见[静态复核](evidence/unused-checkpoint-error-cleanup-20261110/after-references.txt)。

真实审计仍经 InspectLifecycleAuditWorkspace → inspectStructuredAuditWorkspace → inspectionGate；creationCall/creation owner 仍处理原 plan/认领/started/result；历史 frozen reader、控制协议与原停止 scope 仍有消费者。没有为了清理而删除调用者或测试，没有新增兼容壳、类型镜像或重绑入口。

## 精确删除与保留

| 文件 | 实际改动与保留 |
| --- | --- |
| orcatransport/control_checkpoint.go | 删除 controlCheckpoint、独占注释/codex import，import 块收缩为原 context；文件和 inspectionGate 保留，方法正文原字节完全一致 |
| orcatransport/workspace_initial_terminal.go | 删除原 21 行文件，仅含未消费类型、import 和旧注释；本批唯一源码删文件 |
| publication/provider.go | 删除 Error 类型、Error 方法及 providerError 共 9 行；全部 imports 及其余正文原字节相同，包括真实 Query/validateKind 与错误返回 |

合计生产净删 **42 行**，无新增生产函数、字段、接口、回调、配置或错误映射。所有测试、共享 fixture/driver/helper、JS/Node 和脚本均未修改，本批没有测试消费者迁移，也没有新增声明存在性测试。

inspectionOnce/inspectionSerial、审计调用/取消/释放、Codex RemoteControlRequest/Response/Setup、ControlTerminal/TerminalConfig/State/Turn/NativeProof/Profile、frozenRecord decoder、原 scope/close、SessionID、真实 creation owner 及其所有持久读写保持。PR provider 的 command/gitea、ErrPRIdentity/AdapterError/UnsupportedAdapterOperation、HTTP/命令错误和凭据脱敏保持；未扩大为整个 Orca→Codex 依赖退役。Orca CLI 恒真回调/nativeAccess、提示词及 Record/operation 重构不在本批。

state/v8/user_version=8、实际持久字段/tag/codec、公开 CLI/schema/capabilities、Workflow 和资源摘要不变。删除声明上的旧 JSON 形状不是当前持久格式入口；原用户数据、checkpoint 文件和历史 evidence 均未删除或迁移。

## 原字节、声明与编码证据

[一次性捕获程序](evidence/unused-checkpoint-error-cleanup-20261110/capture.go)在删除前后独立执行，明确排除批准删除的三个类型声明；其原定义单独按原文件保留，不伪称整个原类型集合仍存在。捕获的 **299 个保留类型声明**及固定代表编码、schema/capabilities/资源摘要前后完全相同。

导出类型样本包含 Record/Operation、Profile/State/ControlTerminal、RemoteControlSetup/Request/Response 及原 envelope，检查编码/读取错误与 Record JSON 往返。私有 creation 类型以原声明和未改代码对照保留；固定 raw plan/started/result JSON 仅比较代表字节，不冒充真实 typed decoder、合法运行 plan 或创建资格，真实私有编解码/身份核验由原 creation owner 测试举证。

[删除前](evidence/unused-checkpoint-error-cleanup-20261110/before-capture.json.gz)/[删除后](evidence/unused-checkpoint-error-cleanup-20261110/after-capture.json.gz)均为 **338,087 字节**，SHA256：

`a22bcab27292aa398b1e0306a746b1fad4b8f482e7bd3fdf82a9ae2594b684ee`

resource_digest=`910c7fa06b933be3632f862062e966200fd581b34873a395268826d44c1bbbe7`。原 811 个 Go 源码/测试、JS、PowerShell 文件中，仅批准的两个文件修改、一个文件删除，其余 **808 文件原字节相同**，见[范围核对](evidence/unused-checkpoint-error-cleanup-20261110/scope-audit.json)。最终 810 项清单 SHA256=`8630b68aff3fa90372546fcdd343adedd37501bf866f83f5711435bd1b58f6be`。gate 原字节与 provider 扣除批准闭包后的原字节分别精确对照通过。

## 实际最小验收

Windows/amd64，PowerShell 7，普通宿主的隔离 Git/进程、HTTP/命令装配；Go count=1。测试文件和断言均未修改。删除前和删除后各运行同一集合，实际顶层名字、子例数量与结果一致，均无失败/skip；未发生需要修正的基线或最终测试失败。见[命令](evidence/unused-checkpoint-error-cleanup-20261110/commands.txt)、[完整汇总](evidence/unused-checkpoint-error-cleanup-20261110/summary.json)及独立 JSON/stderr/退出文件。

| 范围 | 删除前 | 删除后 |
| --- | --- | --- |
| orcatransport 计划十一项 | 11 顶层、39 执行节点通过 | 11 顶层、39 执行节点通过 |
| 完整 publication 包 | 9 顶层、19 执行节点通过 | 9 顶层、19 执行节点通过 |
| 正式 EXE | 本批不要求前置 EXE 构建 | scripts/build.ps1 成功，退出 0，产物 SHA 与构建记录一致 |

adapter 集合实际覆盖当前审计 worktree 身份及前后替换拒绝、frozen reader 无 mutation、原 scope/旧执行拒绝、codec 绑定/等待，以及 creation owner 原输出、未知/错来源零重派、不完整认领和原期限。原 gate 正文未改，加上原审计测试通过，不解释为新增锁公平性或并发压力资格。

publication 覆盖原 PR 身份/合并证据、Gitea head、错身份诊断、引用凭据脱敏、配置秘密拒绝及 unsupported HTTP 零请求。命令早拒绝由原 Query/validateKind/Check 正文逐字不变证明；failingCommandRunner 没有调用计数，不报告 CLI 零调用实测，也未为未改行为新增测试。

正式构建 source_commit=`13f3500b8d31ff77d46bf786160582a25120d04c`、source_dirty=true、source_digest=`c909c2cc00bc36b3d580dd1d6b3990e0442612449f83a3f647ee761bb7d86c39`。EXE SHA256：

`8d26b5dbd1c1f508032f74a7d97163da62d201fe40f34e1b32c54ee48ff1cb31`

[构建记录](evidence/unused-checkpoint-error-cleanup-20261110/build.json)与最终输入核对一致，源码和产物身份正常变化单列，不回填历史记录。完整 EXE 位于本工作副本 `.evidence/unused-checkpoint-error-cleanup/final/steerrunner.exe`，未替换已安装程序。

## 结束与资格

UCE-01–06 完成。资格限于合并前 adapter 十一项、完整 publication 和正式生产装配编译；按照已审计最小范围，未运行公开受控、完整 adapter/application/protocol、L1/Controlled、L2–L4、全恢复、独立 live driver/helper 编译、Node 或真实 Orca/模型/PR，不扩大既有资格。生产只删除未消费声明，收益是减少维护/搜索范围，不声称提速或包依赖整体解除。

Spec/Plan、索引和长期实施状态已同步，原证据按 .gitattributes 保持原字节，见 [SHA 清单](evidence/unused-checkpoint-error-cleanup-20261110/SHA256SUMS.json)。本批实现未提交/推送/创建新 PR/合并/部署，不自动进入其它清理。

## 实现 PR 交付补记

2026-11-10，用户在最小验收完成后授权提 PR。实现分支从刷新后的 main 建立；810 项最终源码/测试/工具清单及 30 份证据哈希复核一致，沿用原定向与构建结果，不因提交包装重跑。原构建与一致性记录保留捕获时的 source_commit/source_dirty/交付状态，不回填为提交后构建或合并后资格。本次授权不包括合并或部署。
