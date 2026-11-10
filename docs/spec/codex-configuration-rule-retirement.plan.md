# Plan：Codex 旧配置规则退役

日期：2026-11-10。状态：按用户后续“实施吧”授权完成下列步骤和合并前定向验收，见[独立记录](../acceptance/codex-configuration-rule-retirement.md)。依据 [Spec](codex-configuration-rule-retirement.spec.md)，调查/实施基线 main `9243a79b847415b29480d6e418dfc76d01307fcd`。此前设计/复审仅文档的历史范围保持；定向验收后按新增“提pr”授权进入提交、推送和 PR 交付，不包含合并或部署。

## P0：固定实施基线与删除闭包

- [x] 实施开始时刷新目标分支，记录 HEAD、目标 SHA 和工作树差异；新增上游差异与本批分别核对。出现真实内容冲突时按 AGENTS.md 在独立 worktree 解决并按影响复验，不改写已验证历史。
- [x] 全仓搜索 NormalizeProfilePayload、NormalizeProfileConfig、LocalReasoningEffortDefault、EnvironmentDiagnostic、EncodeProfileConfig、DecodeProfileConfig，覆盖生产/测试、工具、CLI/脚本入口；把 evidence 的历史文本引用与实际调用分开登记。对照 Spec 表确认没有新生产根。
- [x] 确认合法入口：contracts 纯 gate/ConfigureExecution → application.freeze/freezeContent → 原 profile envelope → NewExecutionLifecycle/权限资源。确认实际诊断发布沿 redactDiagnostic、消息沿 acceptMessage；旧有限 code 分类不是公开入口。
- [x] 固定生产四函数及独占 imports 的预期删除；保存保留的 ProfileConfig/tag/codec、Config/Validate、NativePermissions、encoder/decoder 和两项跨包编码测试的源码摘要。其它批次不进入变更。
- [x] 使用固定身份和配置捕获原 full_access/read_only、模型省略/显式值的 ProfileConfig/Envelope 编码，以及 capabilities 支持声明/resource_digest/静态资源原值；响应中的构建来源/EXE 身份分别记录，不将其正常变化误作静态能力变化。序列化/构建失败明确失败，不以新结果回填旧结果。复用既有工具或一次性证据程序，不新增通用生产审计设施。

## P1：先迁移测试，再删除旧规则

- [x] TestCD63CodecIdentityAndVersion 保留 endpoint、有效 payload、未知字段、unsupported NativePermissions；显式 high 改由原 decoder 验证原值保留。model 空白反例迁至 contracts 合法本机配置 gate，配合法干净 model 正例，不能让无关缺字段/非法目标代替 model 拒绝。
- [x] TestPS2RejectsAllRetiredCodexOverrides 改调用 DecodeProfileConfig，保留七字段×四种值、共 28 例与 unknown field 错误断言；在 contracts 的有效 full_access/development 配置基底上保留同组公开拒绝。测试不调用 must-not-run executable；不增加 decoder 语义校验。
- [x] 新增 application.TestCodexAdmissionPreservesFrozenDefaults（已新增）：复用隔离本地仓库和内容夹具，调用真实 freeze。覆盖 development/full_access、generate/read_only、conversation/read_only，effort 省略→medium/显式 high 保持、model 省略/合法显式值；核对 Request/ProfileConfig/Workflow.ExecutionModel 和有效请求摘要，原输入不变。另覆盖隔离 PATH 中省略 executable→codex.exe 与合法显式 executable 的原解析，不使用用户实际安装或启动模型。分别用原始 JSON 注入 executable/model/reasoning_effort 的显式空串和 null，验证公开 gate 和 freeze 均返回 invalid_input、无 Run 结果，不进入 executable 解析或业务执行；每组先有合法基底正例。负例保持原字节，不经过带 omitempty 的 CodexSettings 解码/再编码，以免被折叠成字段省略。
- [x] 新增 application.TestCodexDiagnosticPublicationUsesExistingRedaction（已新增）：使用现有 Engine/store 夹具，经 block、原事务提交、Run 查询及 store 重读核对 phase/状态、非秘密原文与已知凭据遮盖；不只测 redaction helper，不新增旧 code 分类器。继续运行 TestLastMessageIsBoundedReadOnlyTelemetry 验证未知 Bearer/token 等消息特征过滤。
- [x] 删除旧 remote_diagnostics_test.go 的有限分类断言/测试；在迁移对照中明确这部分随无入口规则退役，不称为完全等价分类覆盖。生产隐私实现不变。EncodeProfileConfig 和两个现有跨包消费者原样保留。
- [x] 在尚未删生产的基线上运行迁移及新增测试，保存命令、逐例结果和原失败/skip。确认 28 例没有遗漏、model 正例真正通过、默认值测试走实际 freeze、空串/null 原始字段没有被省略；发现基线问题先核对范围，不夹带产品修复。

## P2：精确生产删除与静态复核

- [x] 删除 profile_config.go 的 NormalizeProfilePayload、NormalizeProfileConfig 及独占 imports；删除 config.go 的 LocalReasoningEffortDefault；删除 diagnostic.go 的唯一函数 EnvironmentDiagnostic 及文件。
- [x] 复核保留的 ProfileConfig/codec/EncodeProfileConfig/DecodeProfileConfig、Config.Validate/ValidReasoningEffort、permissions/executable/envelope 实现原字节；application/contracts/resources/Orca/parser 等生产代码本批不得变化。
- [x] 生产无四个旧符号及替代兼容壳；测试不再调用它们，不复制旧 high 默认或旧分类表。只处理上述测试迁移所需差异，不顺带清理其它无引用声明。
- [x] 对照 P0 捕获，确认代表编码、公开静态契约、capabilities 支持声明/resource_digest 相同；源码摘要/EXE 哈希及响应来源身份变化正常记录。检查所有批准保留的测试断言和错误优先级。

## P3：最小实施验证

遵循 docs/testing.md；下列命令已在实施阶段执行，原退出码和日志见独立验收。首次沙箱 Git 写权限失败与普通宿主重跑通过分别保存，命令返回与测试通过分别判断。

1. 完整受影响组件：

   ```powershell
   go test ./internal/execution/adapters/codex ./internal/contracts ./internal/resources -count=1 -timeout=10m -json
   ```

   必须包含迁移后的 CD63/PS2、原 policy/profile/permissions、公开配置构造/校验和原静态契约测试。记录现场 opt-in 的实际 skip，不能把 skip 当作通过；这不是完整 L1。

2. application 定向集合：

   ```powershell
   go test ./internal/application -run '^(TestCodexAdmissionPreservesFrozenDefaults|TestCodexDiagnosticPublicationUsesExistingRedaction|TestLastMessageIsBoundedReadOnlyTelemetry|TestNativePolicyMismatchBlocksNewDispatch|TestNativeProofBusinessKnowledgeBoundary|TestContentRecoveryKeepsContractsAndRejectsForeignMessageScope)$' -count=1 -timeout=10m -json
   ```

   先核对六个顶层测试都实际存在且执行，新测试不能因名称未落地而零匹配漏跑；保存每个默认/权限/内容矩阵子例的结果。新测试改名时同步本 Plan 和实际集合，原断言不因改名消失。编码消费者保持原测试装配；无需新增重复生命周期框架。

3. 正式 EXE 与一个公开受控用例：

   ```powershell
   ./scripts/build.ps1 -OutFile .evidence/codex-configuration-rule-retirement/final/steerrunner.exe
   ./scripts/test-foundation.ps1 -RunnerExe .evidence/codex-configuration-rule-retirement/final/steerrunner.exe -Pattern '^TestSRSingleExecutionColdStop$' -OutDir .evidence/codex-configuration-rule-retirement/controlled-new
   ```

   实施时必须使用全新输出目录，RunnerExe 固定到同一最终文件。保留该入口原有单元前置、真实 EXE/受控 peer、超时和清理，不自行裸跑集成包绕过装配。等待宿主最终 result.json/integration.log，核对退出码、恰好一项顶层匹配、原输入恰一次、原停止受理/服务接回和结果封存；不能把启动返回算完成。

   保存源码/测试摘要、build sidecar、EXE SHA 与该用例结果。只登记本项定向公开受控和实际内含窗口，不授予完整 Controlled、RestartPreservesAuthorization、L1 或 L2–L4 资格。

以上范围已经包含严格解码、公开 gate、实际默认冻结、权限派发边界、消息/持久 diagnostic 保护和公开原执行/停止。未知操作、Orca、完整恢复、真实模型/PR、Node、parser 不因本项默认全跑；实际 diff 如触及这些机制，先修订范围再按影响补验。只有新改动、失败或未解决问题才重跑或扩展，不机械重复通过项。

## P4：独立验收与交付边界

- [x] 按 CCR-01–05 在 docs/acceptance/codex-configuration-rule-retirement.md 记录基线/最终源码、精确删除/保留、每个测试消费者去向、原/新编码对照、命令及实际顶层/子例集合、失败/skip、EXE 身份和收益。原分类覆盖删除与仍有效断言迁移分开登记。
- [x] 明确全部结果是合并前还是合并后、定向还是完整；未运行项目明确列出，不借旧报告授予新资格。普通记录不向中央 README 表头加行，只有导航需要才更新索引。
- [x] 实际通过后才将 Spec/Plan 标为实施完成并链接验收；长期承诺未变，不为凑交付改写 PC-C/PC-R。若要改变公开行为，另修对应契约/说明/测试清单，不混入本次退役。
- [x] CCR-01–05 完成后停止实施，不开展 Orca/无引用清尾后续批次。实施授权当时未包含提交/PR；随后按用户“提pr”新增授权交付，仍不包含合并或部署。创建 PR 和最终交付前刷新目标并核对 GitHub 实际内容合并状态，优先 Squash and merge，不因 Rebase 提示改写已验证历史。

## 设计审计记录

2026-11-10，独立只读子代理审计确认一项 P2 并已修正：P1 默认值测试只将字段省略视为默认成功；显式空串/null 用原始 JSON 验证现行拒绝，不经 omitempty 重编码。Spec 的边界、矩阵及 CCR-02 已同步，详见 [设计审计修订](codex-configuration-rule-retirement.spec.md#7-独立设计审计修订)。其余删除/保留、测试消费者迁移和最小验收范围保持。此段记录设计复审时点，当时未实施、没有产品测试或构建资格；后续实施以本文件首部和独立验收为准。
