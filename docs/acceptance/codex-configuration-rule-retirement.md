# Codex 旧配置规则退役验收

2026-11-10，合并前定向验收。调查和实施基线 main `9243a79b847415b29480d6e418dfc76d01307fcd`；实施前已刷新 origin/main，与 HEAD 相同。依据 [Spec](../spec/codex-configuration-rule-retirement.spec.md) / [Plan](../spec/codex-configuration-rule-retirement.plan.md)，在独立设计审计修正空串默认语义后，按用户后续“实施吧”授权实现。此处固定实施验收时点，当时尚未提交、推送、创建 PR、合并或部署；后续交付授权见文末。

## 删除、保留与实际消费者

生产仅改动 Codex adapter 的三处文件：profile_config.go 删除 NormalizeProfilePayload、NormalizeProfileConfig 和独占 imports；config.go 删除 LocalReasoningEffortDefault；删除仅含 EnvironmentDiagnostic 的 diagnostic.go。四函数无生产调用根的结论在实施前复核，未删除调用者来制造零引用。生产净删 **87 行**，无新增生产接口、兼容壳或替代默认/分类器。

ProfileConfig 字段/tag/顺序、codec 常量、ProfileVersionForMode、EncodeProfileConfig、DecodeProfileConfig、Config.Validate、ValidReasoningEffort、权限、原生程序解析及 envelope 核验保持。EncodeProfileConfig 的 TestPolicyResourceMatchesOriginalProfile 和 TestNativePolicyMismatchBlocksNewDispatch 两个现存消费者不变。application/contracts/resources/Orca/parser 等生产代码未改。

当前受理的 medium 默认、模型省略、合法显式值、开发 full_access 和本机内容 read_only 保持；原始 JSON 的 executable/model/reasoning_effort 空串或 null 仍在默认应用前拒绝。没有把旧 normalizer 的语义检查加到历史 decoder，没有改写原冻结记录。PC-C01–C07、PC-R01–R10、state/v8 和原实际读写/codec 边界均保持。

收益为移除一套无入口的配置/展示规则和旧诊断分类，减少后续开发搜索分支。测试迁移和新覆盖增加了测试代码，不宣称全仓净删、任务提速或包体积下降。

## 测试消费者迁移与基线

| 原消费者 | 实施结果 |
| --- | --- |
| TestCD63CodecIdentityAndVersion | 原 endpoint/未知字段/unsupported 权限反例保留；直接验证 decoder 保留完整显式 ProfileConfig。空白 model 的语义反例迁入 contracts.TestCodexConfigurationRetainsModelValidation，同一合法本机配置先正后负 |
| TestPS2RejectsAllRetiredCodexOverrides | 直接调用 DecodeProfileConfig，保留七字段×四种 JSON 值的全部 28 例及 unknown field；新增具名子例。contracts.TestCodexConfigurationRejectsRetiredOverrides 用合法 full_access/development 基底另跑同组 28 例公开拒绝 |
| TestEnvironmentObservationDiagnosticsPreserveFiniteOriginalFailure | 随无入口分类器退役，删除旧有限 code 保留/归类断言；不伪称新测试等价覆盖旧分类 |
| 当前默认与冻结 | 新增 TestCodexAdmissionPreservesFrozenDefaults：development/2、generate/1、conversation/1 的 24 个成功叶子场景覆盖省略/明确 executable、省略/high effort、省略/明确 model；另有 18 个原始 JSON 空串/null 拒绝场景。检查 Request、ProfileConfig、Workflow.ExecutionModel、input_digest、原输入字节和不派发；使用隔离本地 Git/测试 EXE/PATH，不启动模型 |
| 当前隐私与业务诊断 | 原 TestLastMessageIsBoundedReadOnlyTelemetry 保留未知 Bearer/token 等消息过滤；新增 TestCodexDiagnosticPublicationUsesExistingRedaction 经 block→commit→公开 inspect→store 重读，验证 GH_TOKEN/声明的认证引用值遮盖、原 phase/状态/非秘密内容及零原生派发。没有扩大任意错误文本的过滤承诺 |

生产尚未删除四函数时，已先运行迁移及新增测试。首次沙箱执行中，TestCodexAdmissionPreservesFrozenDefaults 和既有 TestNativePolicyMismatchBlocksNewDispatch 均在临时 Git 初始化时失败，原因是 Git 无法写沙箱 Temp 下的 config；其余顶层 8 项通过。保留[原日志](evidence/codex-configuration-rule-retirement-20261110/baseline.jsonl.gz)，不记为通过。

保持测试、断言和生产代码不变，在普通宿主上下文重跑同一集合：10 个顶层、114 个执行节点全部通过，零失败/skip，见[删除前宿主基线](evidence/codex-configuration-rule-retirement-20261110/baseline-host.jsonl.gz)。随后才删除生产闭包。没有为基线失败放宽产品 gate 或夹带其它产品修复。

## 编码、来源与保留范围

删除前/后分别运行同一个[一次性捕获程序](evidence/codex-configuration-rule-retirement-20261110/capture.go)。使用固定 source 身份而非实际 EXE 哈希作可比元数据，捕获 full_access/read_only × 模型省略/明确值的 ProfileConfig、原 envelope 编码、公开 capabilities、配置 schema 和资源摘要。两次均为 **222,022 字节**，SHA256：

`b17f4fd2915862332e6e99ef3be4c0678116195364d4bce789e8c4f6919eb190`

[删除前](evidence/codex-configuration-rule-retirement-20261110/before-capture.json.gz)与[删除后](evidence/codex-configuration-rule-retirement-20261110/after-capture.json.gz)原字节相同，resource_digest=`910c7fa06b933be3632f862062e966200fd581b34873a395268826d44c1bbbe7`。这不是用新值补签旧结果，也不要求真实 EXE 的来源身份相同。

[范围核对](evidence/codex-configuration-rule-retirement-20261110/scope-audit.json)比较迁移测试完成、生产未改点的 692 个 Go 文件：只有批准的三个生产文件变化，另外 689 个文件原字节相同；该数字的基线已经包含本次测试迁移，不冒充所有原测试均未改。对 profile_config/config 的保留正文另按 HEAD 原文扣除批准删除块逐字比较一致，diagnostic 整文件按批准范围删除。最终 Go 源码/测试 SHA 清单的 SHA256=`70965d12ecc3d4673890512c6266f2c98f0d3849de4aad975b30acbd04367bf6`；原始、最终清单均保存于证据目录。

## 最小验收实际结果

命令采用 Plan P3 的原入口，所有 Go 命令均带 count=1，不复用测试缓存结果；涉及临时 Git 的测试运行在普通宿主上下文。实际范围见[汇总](evidence/codex-configuration-rule-retirement-20261110/summary.json)和逐条原日志。

| 范围 | 实际结果 |
| --- | --- |
| 完整 codex adapter | 82 顶层通过，255 执行节点通过；1 个 helper 入口按原设计 skip，零失败 |
| 完整 contracts | 27 顶层、149 执行节点通过，零失败/skip |
| 完整 resources | 22 顶层、113 执行节点通过，零失败/skip |
| application 精确六项 | 6 顶层、53 执行节点通过，零失败/skip；实际集合与 Plan 完全匹配，包含两个新增测试 |
| 正式 EXE | scripts/build.ps1 构建成功，build sidecar 与受控现场 Runner SHA 一致 |
| test-foundation 原有单元前置 | 四个包、5 个顶层通过，零失败/skip；含两个包各自的 TestSRCLIContract，不能仅按测试名称去重 |
| 同 EXE 公开受控 | TestSRSingleExecutionColdStop 通过，12.30 秒；最终 result.json 的 exit_code=0、scenario_count=1，无 skip |

codex 包的 skip 是 TestServiceStopWorkerHelper：它在没有 STEERRUNNER_TEST_STOP_HELPER=1 时跳过，专供 TestServiceStopWorkerFacts 启动受管子进程。父测试通过；不把这个 helper 入口的 skip 改记为独立通过，也不误称现场 opt-in 缺配置。

正式构建 source_commit=`9243a79b847415b29480d6e418dfc76d01307fcd`，source_dirty=true，source_digest=`fb37852bf05a0b7004a6f2515b28cac070ef202d4500dc610228d96d10344074`。EXE SHA256：

`d4ea3bf140caed87f7de9730f0a3994c606cdde27fa6d71d24a42768c4dc39ab`

[构建记录](evidence/codex-configuration-rule-retirement-20261110/build.json)、[受控源码/产物身份](evidence/codex-configuration-rule-retirement-20261110/controlled-source-identity.json)、[最终结果](evidence/codex-configuration-rule-retirement-20261110/controlled-result.json)及[公开日志](evidence/codex-configuration-rule-retirement-20261110/controlled-integration.log)固定到同一个正式文件。完整 EXE 和受控现场保存在本工作副本 `.evidence/codex-configuration-rule-retirement/`；没有运行或替换用户已安装服务。

受控用例按原断言核对原输入恰一次、服务脱离/接回后持久停止、原命令结果和 result-file 封存。等待最终 result.json/integration.log 后才登记成功，没有以脚本的“Independent tests running”输出代替完成。

证据汇总首次将 foundation 日志中的同名/子例误作四项顶层，汇总断言失败；修正为按行首统计四包共五个顶层后完成，产品日志/产物未变，也没有重跑或改写原测试结果。

## 资格与结束范围

CCR-01–05 已完成，仅取得本次合并前的组件、application 定向和一个公开受控路径资格。未执行完整 L1/Controlled、L2–L4、真实模型/PR、Orca/Node 或独立 parser 专项；单个受控窗口不等于完整恢复资格。原沙箱失败和 helper skip 保持原记录，不回填历史资格。

证据目录通过 .gitattributes 保留原字节并保存 SHA 清单。Spec/Plan 已同步实施状态；不自动开展第二批、提交、推送、创建 PR、合并、安装或部署。

## 后续 PR 交付授权

2026-11-10，用户追加“提pr”，授权将本批 Spec/Plan、代码、测试及独立验收提交、推送并创建 PR。交付前再次刷新 origin/main，仍与原基线相同；源码与最终测试/构建清单一致，证据 SHA 全部通过，未发生需要重验的生产或测试变化。以上测试仍是原产物的合并前结果，不改记为提交后重跑或合并后资格。优先 Squash and merge；本次交付不包含合并或部署。
