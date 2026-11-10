# Spec：零消费者 checkpoint／错误声明清尾

日期：2026-11-10。状态：独立设计审计完成并已实施，合并前最小验收完成，见[独立记录](../acceptance/unused-checkpoint-error-cleanup.md)。原设计轮只编写 Spec/Plan 和必要索引，不改生产或测试代码、不运行实施验收；用户先授权创建本设计 PR，随后明确授权独立审计、修正确认问题并实施；实施验收阶段未授权新的提交/PR、合并或部署，用户随后另行授权提交并创建实现 PR。调查主线 `5ccc3f0aa07764657b3346aa6023b73b878d76e0`（#136 已合入）；调查时工作副本 HEAD `f3668b37` 与该主线文件树相同。配套 [Plan](unused-checkpoint-error-cleanup.plan.md)，依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[Orca 能力边界](orca-capability-layer.spec.md)、[原执行与停止范围](../orca-execution.md)及[测试指南](../testing.md)。

## 1. 目标与范围

controlCheckpoint、initialTerminalCheckpoint 和 publication.Error/providerError 不属于当前合法执行、恢复、读取或错误返回路径。前两者只有类型声明；后一组只有未被调用的构造函数、类型及 Error 方法构成的闭包。保留它们使后续开发仍需排查两种 checkpoint 形状和一套错误类型。

本项删除上述零消费者声明及独占内容，减少维护与搜索范围。不退役任何真实 checkpoint、历史读取、创建 owner 或 PR provider，不新增兼容壳或统一错误体系，不以搬文件/改名签收，不承诺任务提速、EXE 缩小或包依赖完全解除。

本项不把“没有直接调用”当作充分证据。删除前须完成引用、构造、注册、编解码、函数值/接口和实际装配核对；发现新增真实消费者先修订范围，不能先删调用者或有效测试制造零引用。

## 2. 实际消费者与合法入口核对

| 对象 | 基线核对结果 | 合法入口及边界 |
| --- | --- | --- |
| controlCheckpoint | orcatransport/control_checkpoint.go 仅有声明，无构造、字段访问、类型注册、编码/解码或测试引用 | 不是当前审计锁或原控制请求读取入口；不能因文件名相同连带退役 inspectionGate |
| initialTerminalCheckpoint | orcatransport/workspace_initial_terminal.go 仅有声明；文件剩余内容为旧阶段注释，无函数、编解码或测试消费者 | 当前工作区创建沿 creationCall/creation owner 的 plan、唯一认领、started/result 和原回执核实，不消费该类型 |
| publication.Error / (*Error).Error / providerError | 包内唯一构造为 providerError，构造函数自身无调用；无测试构造/断言。生产导入者仅 application/freeze.go 与 results.go，均无该类型使用或 errors.As 分支 | freeze 经 NewClient 装配真实 provider；Query/Check、PR 身份/合并事实核验、命令/HTTP 错误及脱敏继续沿现行代码 |
| inspectionGate / inspectionOnce / inspectionSerial | 当前 InspectLifecycleAuditWorkspace → inspectStructuredAuditWorkspace 真实调用，使用原初始化、串行门和 context 取消/释放 | 必须保留方法正文、字段和调用位置；不是零消费者闭包 |
| Codex RemoteControlRequest/Response/Setup、ControlTerminal、State/Profile 及 creationPlan 等 | 原控制协议、历史 frozen reader/停止证明、创建 owner 等仍有实际消费者 | 删除一个独占 import 不等于相关 Codex 类型或整个 Orca→Codex 依赖可删除 |

核对覆盖仓内生产、测试、cmd、scripts、helpers/experiments、各 build tag/平台文件和生成代码，包含符号的函数值、类型断言、errors.As、反射/注册及 go:linkname 等入口。publication.Error 必须按实际 package import/alias 和构造闭包定位，不能把普通 err.Error()、标准库 errors 或其它包 Error 当作本项引用。历史文档/原证据中的符号文本不算运行消费者，原材料不改写。仓外没有本 internal 包的受支持装配入口；不据此推断可删除任何公开产品字段。

## 3. 精确删除与保留清单

| 文件/对象 | 允许删除 | 必须保留 |
| --- | --- | --- |
| orcatransport/control_checkpoint.go | controlCheckpoint 类型及独占注释、codex import；允许 gofmt 对该 import 块作必要收缩 | 原文件路径、context import、inspectionGate 原正文；Lifecycle 两个 inspection 字段、审计调用/取消/释放顺序均不改，不迁址或改名 |
| orcatransport/workspace_initial_terminal.go | 全文件：package/import、initialTerminalCheckpoint 及仅描述旧阶段的尾部注释 | 其它 workspace/creation 文件和真实持久 plan/started/result；不删除运行现场或用户磁盘文件，不新增清理脚本 |
| publication/provider.go | Error 类型、其 Error 方法和 providerError 构造函数，以及直接附属空行 | 所有 imports、CommandRunner/Provider/Client/NewClient、validateKind、Query 的原正文和错误；本基线没有由删除闭包独占的 import |

生产 diff 仅限这三个文件的批准删除及必要 import 格式变化，不新增函数、字段、配置、回调或错误映射。所有测试文件和共享 fixture、driver/helper、JS/Node、脚本均保持原字节。本批没有测试消费者迁移，不为删除声明新增镜像测试或把声明搬进 _test.go。

以下保留项不得顺带清理：真实 creation owner 的配置/命令/摘要/认领文件、原创建未知只读核实及零重发；Codex RemoteControlRequest/Response/Setup 与请求摘要/校验；ControlTerminal、TerminalConfig/owner/receipt、State/Turn/NativeProof/Profile、frozenRecord 的原 decoder/校验与冻结停止 scope；SessionID、当前结构化执行与 close 判据；PR provider 的 command/gitea 支持、ErrPRIdentity、AdapterError、UnsupportedAdapterOperation、HTTP/命令错误和引用凭据脱敏。

orcaCLIOutput 恒真回调、Lifecycle.nativeAccess、提示词包装、Record/operation 表示、无关类型/字段及文档大整理不进入本批。不要求通过删除独占 import 改写 package 依赖图，更不能替换仍有消费者的原类型。

## 4. 产品与格式不变量

长期 PC-C01–C07/PC-R01–R10 保持，重点是审计原提交/工作区归属、PR 真实平台事实、原操作未知零重发、原停止 scope 和升级前排空。没有新增运行规则、错误分类、恢复或兼容入口，不把 PR 就绪当作资源结算。

state/v8、SQLite user_version=8、Record/Request/Operation/Receipt、原 Profile/State/ControlTerminal/RemoteControl*、creation plan/started/result 和其它实际持久字段/tag/codec/读写保持。仅三组已批准未消费声明本身从声明集合移除；不能要求删除前后整个类型集合相同，也不能将真实持久类型列作“无引用”一并删除。原磁盘记录与 evidence 不改、不迁移、不物理清理。

公开 CLI/schema/capabilities、resource_digest、Workflow/提示词/JS 资产保持。源码/构建 source_digest、EXE 哈希可能正常变化，须分别记录，不回填原产物身份。验收保存批准删除前的原文件与声明，对其余生产正文逐字比较；保留类型及固定代表编码前后核对，marshal/decode 错误必须检查。

## 5. 最小验收与结束条件

| ID | 必须证明 |
| --- | --- |
| UCE-01 | 三组声明无真实构造、注册、编解码、接口/函数值或测试消费者；删除后无改名副本/兼容壳；没有删除调用者制造零引用 |
| UCE-02 | 仅批准三文件变化；inspectionGate、两个字段与实际审计调用保持，所有测试/共享工具原字节保持 |
| UCE-03 | 实际 historical reader/原 scope、真实 creation owner 的身份/未知/期限和零重发判据保持；被保留的 RemoteControl*、ControlTerminal 等声明/代表编码一致 |
| UCE-04 | 真实 provider 装配/Query/Check 不变，完整 publication 的 PR 身份、合并证据、错误脱敏及 unsupported HTTP 零请求测试通过，原命令早拒绝正文保持；未将错误改成统一新类型 |
| UCE-05 | Plan 的 adapter 十一项和完整 publication 实际执行、正式 EXE 构建成功，固定能力/资源摘要保持；原失败/skip、无测试包与未验范围如实区分 |
| UCE-06 | 实施后在 docs/acceptance/unused-checkpoint-error-cleanup.md 独立登记消费者核对、删除/保留、原文件/摘要/编码、实际测试集合/命令/失败/skip、来源与产物身份，再更新设计状态 |

最小运行范围为 orcatransport 十一项现有定向（当前审计、frozen reader/原 scope、State codec、creation owner）和完整 publication 包，实施前后各执行以保留基线；冻结最终源码后正式构建。具体名字和命令见 Plan。测试集合验证保留入口，不将旧声明不存在当成行为测试，也不减少任何原有效断言。

由于批准变更不触及运行规则，本批不额外要求公开受控、application 全组、protocol 全组、live driver/helper 单独编译或真实后端测试；正式构建覆盖实际生产装配编译，不能冒充这些运行资格。若发现实际消费者或 diff 必须触及保留逻辑/共享测试，先修订范围并按影响补验，不以最小清单继续签收。

原设计轮只交付文档；用户随后授权实施，实际结果见独立验收，不以原设计静态复核代替运行结果。最小结果不代表完整 adapter/application、L1/Controlled、L2–L4、全恢复、真实 Orca/模型/PR 或合并后通过。UCE-01–06 完成即结束，不自动展开下一批、提交/推送/创建 PR/合并/部署。

## 6. 设计 PR 交付复核

2026-11-10，用户在设计完成后授权提 PR，本次交付仅含本 Spec、Plan 和索引，设计保持未实施。创建分支前刷新 main 至 `91507d7e78c4e89598a0f20a967062d888104a3f`（#137），核对新增 owner 读取重试与 Git 核验变更：批准删除的三个文件、inspectionGate/审计调用、publication 装配及原十一项/九项测试声明均未变，实际消费者结论与最小范围保持。交付分支从该主线建立，无内容冲突；仅静态核对引用、名字与文档链接，未运行实施测试或构建，不继承 #137 的运行资格。

## 7. 独立设计审计与实施基线

2026-11-10，独立只读子代理核对实际消费者、删除/保留、最小集合和资格边界，未确认需修订问题；未改文件、未运行测试。设计 PR #138 已合入，实施主线刷新为 `344d3e83eef2c34397d2cae6c02c6c95f805bd2f`，本地 HEAD `13f3500b` 文件树相同。后续修改只按批准闭包进行，不扩大到其它精简。

## 8. 实施状态

2026-11-10，UCE-01–06 完成。按批准三文件删除未消费类型/错误闭包，生产净删 42 行，所有测试及共享工具保持。独立设计审计与实施窄复核均未确认需修正问题；删除前后各十一项 adapter、完整 publication 九项均通过，无失败/skip，正式构建通过。保留声明/代表编码及固定资源摘要前后相同，实际范围与资格见[验收](../acceptance/unused-checkpoint-error-cleanup.md)。验收冻结时尚未提交/推送/创建新 PR/合并/部署；后续实现 PR 授权与交付见验收补记。
