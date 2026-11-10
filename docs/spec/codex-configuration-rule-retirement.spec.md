# Spec：Codex 旧配置规则退役

日期：2026-11-10。状态：已实施并完成合并前定向验收，见[独立记录](../acceptance/codex-configuration-rule-retirement.md)。设计及独立复审阶段仅写文档，随后按用户“实施吧”授权实现和验证。调查/实施基线 main `9243a79b847415b29480d6e418dfc76d01307fcd`。配套 [Plan](codex-configuration-rule-retirement.plan.md)。依据[长期产品承诺](steerrunner.spec.md#长期产品承诺)、[独立配置契约](independent-configuration.spec.md)、[现行模型规则](../model-usage.md)和[测试指南](../testing.md)。历史 Spec 的旧默认、权限及认证描述按当前产品契约的取代关系阅读。

## 1. 问题与目标

当前本机 Codex 请求由 contracts 校验，在 application.freeze 中解析 executable、应用固定默认并冻结 ProfileConfig；执行构造及权限资源读取再核对原 envelope。旧 NormalizeProfilePayload/NormalizeProfileConfig 链没有生产调用根，却仍表达另一套默认、host/agent 分支和展示文案。其 LocalReasoningEffortDefault 将空 effort 变成 high，真实受理默认是 medium。EnvironmentDiagnostic 同样仅被测试调用，仍枚举旧认证阶段。

本项删除这些已脱离合法入口的规则，减少后续开发时需要判断的配置和诊断分支。不是修改默认值，也不是把旧规则搬到新生产接口。收益限于规则及搜索空间减少，不承诺任务提速、EXE 缩小或全局解耦。

## 2. 实际消费者与合法入口

| 对象 | 基线消费者及职责 | 本项结论 |
| --- | --- | --- |
| NormalizeProfilePayload | profile_config_cd63_test.go、ps2_configuration_test.go；内部调用 NormalizeProfileConfig、NativePermissions 和 EncodeProfileConfig | 删除旧规则入口；测试迁移到各自真实职责 |
| NormalizeProfileConfig | 上述旧入口及 TestCD63CodecIdentityAndVersion；无 CLI、受理、执行构造或恢复调用 | 删除；不把语义校验加进历史 decoder |
| LocalReasoningEffortDefault | 仅 NormalizeProfileConfig | 删除；不迁移空值到 high 的旧默认 |
| EnvironmentDiagnostic | 仅 remote_diagnostics_test.go 的 TestEnvironmentObservationDiagnosticsPreserveFiniteOriginalFailure | 删除旧分类器及失效分类断言；保留现行隐私保护的真实测试 |
| EncodeProfileConfig | 旧链，以及 TestPolicyResourceMatchesOriginalProfile、application.TestNativePolicyMismatchBlocksNewDispatch | 保留原薄编码器；这两个真实测试消费者不迁移，不为几行编码新增共享 testkit |
| DecodeProfileConfig / DecodeProfileConfigEnvelope / profileConfigFromEnvelopes | NewExecutionLifecycle、BuildPolicyResourceEnvelope、ValidatePolicyResourceForProfile；严格解码、codec 和原配置身份核对 | 保留实现及原错误顺序 |
| contracts.ValidateExecution / ValidateRunConfiguration / ValidateRun | 新请求的纯校验、Workflow 适用性及完整材料校验 | 保留原接受/拒绝，不借清理收紧或放宽 |
| contracts.ConfigureExecution | 公开 execution-configuration describe/derive/compose；复用现行配置 gate | 保留公开输入、view、identity、默认语义及程序绑定 |
| application.freeze / freezeContent | 新请求的有效参数、默认、材料及 Workflow 冻结 | 保留；本机 executable 缺省 codex.exe、effort 缺省 medium，显式值沿原规则保持 |
| application.redactDiagnostic、acceptMessage、公开配置 Fault | 原已知凭据替换/限长、消息凭据特征遮盖、固定校验错误 | 保留；它们与旧有限诊断分类器不是同一规则 |

实施前复核全仓生产、测试、CLI 路由、脚本、夹具与工具引用。原始 evidence 中保存的旧源码/符号不构成当前运行入口，也不修改它们。若出现新的实际生产调用根，先登记调用链并修订本删除范围，不能删除调用者来制造零引用。

## 3. 精确删除、保留及边界

允许的生产差异仅为：

- 从 profile_config.go 删除 NormalizeProfilePayload、NormalizeProfileConfig 及其独占 errors/strings 导入。
- 从 config.go 删除 LocalReasoningEffortDefault；Config、Config.Validate、ValidReasoningEffort 等保持。
- 删除 diagnostic.go 的 EnvironmentDiagnostic 和独占导入；该文件没有其它声明时可删除文件。

保留 ProfileConfig 的字段、顺序、tag，ProfileCodecKey、ProfileCodecVersion、ReadOnlyProfileCodecVersion、ProfileVersionForMode、EncodeProfileConfig、DecodeProfileConfig；保留 envelope、policy resource、NativePermissions、原 executable 解析、Config.Validate 和所有实际执行/恢复/停止消费者。EncodeProfileConfig 有测试入口且不承担策略，本批明确保留，不要求所有仅测试调用的薄工具都移出生产。

不新增兼容壳、弃用转发、配置服务、归一化接口、诊断注册表、build-tag 旧实现或测试用生产默认。特别不能给 DecodeProfileConfig 加 model/effort/host 语义门槛来代替被删 normalizer；解码边界和新请求校验边界原来不同，仍分别保持。

旧 normalizer 的“缺 executable 拒绝”等前置条件不升级为公开规则：现行新请求允许省略 executable 并在受理时解析默认。省略与显式空串/null 不等价；contracts.optionalStrings/plain 会在 freeze 应用默认前拒绝显式空串/null 的 executable、model、reasoning_effort。非法 effort/权限模式继续由原公开 gate 和实际构造边界判断，不能通过搬迁测试收紧正常请求，也不能从删除无入口校验推导这些输入现在合法。

当前模型可省略的组合继续表示原生默认，不填模型名称、不降级、不换模型；effort 的固定受理默认与模型名称的省略语义分别验证。已有冻结记录中的 high 或其它合法显式值不被改成 medium。开发显式 full_access、合法本机内容 read_only、工具限制和独立 parser 均不变。

本项不实施 Orca reader/测试迁移、无引用类型清尾、Record/operation 重构、公共错误体系统一或旧文档全面整理。其它包只有本节规定的测试接线可以变化，不能趁机修改生产逻辑。

## 4. 测试消费者迁移

| 原测试/断言 | 迁移目标与必须保留的覆盖 |
| --- | --- |
| TestCD63CodecIdentityAndVersion：有效 payload、显式 high | 直接验证 DecodeProfileConfig 保留 executable/model/effort 原值；不再调用 normalizer。原 endpoint、未知字段及 NativePermissions 不支持 orca_supervised 的断言保留 |
| 同测试：带空白 model 的旧 remote normalizer 拒绝 | 改在 contracts 的合法本机 codex-app-server + development/2 配置上调用 ValidateRunConfiguration；先证明对应干净 model 可接受，再只变 model 验证拒绝。不能因使用不支持的 host 或缺 access_mode 而无条件失败 |
| TestPS2RejectsAllRetiredCodexOverrides | 直接调用生产 DecodeProfileConfig，保留 args/profile/windows_sandbox/sandbox/sandbox_mode/approval_policy/approvals_reviewer 七字段 × null、空字符串、空数组、空对象全部 28 例及 unknown field 断言；以有效 full_access 基底另在公开配置 gate 覆盖同组拒绝，不启动 executable |
| TestEnvironmentObservationDiagnosticsPreserveFiniteOriginalFailure | 删除仅属于退役分类器的旧 code 保留/退役 code 归类断言，不新造分类兼容层。未知 Bearer/token 的消息遮盖由现有 TestLastMessageIsBoundedReadOnlyTelemetry 覆盖；已知凭据的业务 diagnostic 保护在真实 block→commit→Run 查询/持久记录路径补一项定向回归 |
| TestPolicyResourceMatchesOriginalProfile / TestNativePolicyMismatchBlocksNewDispatch | EncodeProfileConfig 及调用不变；保留模式匹配、先资源后 profile 错误、输入不变和错权限零新派发断言 |

新增真实冻结默认测试，使用现有 baseRefreshFixture 的隔离本地 Git 仓库、明确测试 EXE 和真实 freeze；内容分支复用现有隔离只读内容装配。不依赖用户安装、登录、网络模型或真实 PR，不启动业务模型。缺省 executable 的解析可以使用隔离 PATH 中的现有测试原生 EXE，不能借机改解析规则。

该测试至少覆盖 development/2 的 full_access 与 generate/1、conversation/1 的 read_only：effort 省略仍冻结 medium，显式 high 仍为 high；模型省略保持省略、显式合法模型保持原值。检查冻结 Request、ProfileConfig、Workflow.ExecutionModel 中相应值及 request input_digest，不能只测一个替代默认函数。省略 executable 仍按原路径解析 codex.exe；显式合法 executable 仍按原解析器冻结。测试原请求不被就地改写。

另以原始 JSON 分别注入 executable、model、reasoning_effort 的显式空串和 null，先有同一合法基底的正例，再验证公开 gate 及真实 freeze 都拒绝，freeze 返回 invalid_input 且没有 Run 结果，不进入 executable 解析或业务执行。负例不能先反序列化成带 omitempty 的 CodexSettings 再编码，否则空串/null 可能变成字段省略，造成与实际请求不同的“默认值通过”。保留原输入字节，不能修改生产 gate 让负例进入默认分支。

新增 diagnostic 回归用测试凭据环境值，走真实业务错误发布与持久提交，不直接只测 strings.ReplaceAll：原 phase、业务状态及非秘密诊断内容仍可见，已知秘密被替换，查询和重新读取记录一致。既有 redactDiagnostic 只替换已声明/已知凭据并限长，acceptMessage 另有凭据特征过滤；不要求所有任意错误文本都变成旧有限 code，不扩大脱敏承诺或修改生产脱敏实现。旧测试中的类别本身已无合法消费者，不伪称迁移后仍覆盖这些分类。

实现测试前先在未删生产的基线上运行上述迁移/新增回归，证明它们验证现行路径。若基线实际失败，单列事实并检查夹具/范围；不能把修复新产品缺陷夹进本次精简。允许删除明确退役的分类测试，不能删除仍有效的解码、权限、隐私或来源断言换取通过。

## 5. 产品、持久格式与公开行为不变量

PC-C01–C07、PC-R01–R10 均不改变：受理及事务、原配置/材料绑定、固定 Workflow 模型编排、unknown 原操作核实且零盲重放、同构建恢复、停止撤销及原 scope、结果与 PR 来源核验、可选观测容错和升级排空保持。

state/v8、user_version=8、Record/Request/Operation/Receipt 和实际 profile/resource 类型、tag、codec、读写及错误优先级保持。不重写历史配置、补默认、迁移状态或重新解释原授权。公开 CLI/schema、静态 capabilities/支持组合、资源摘要、配置 view/identity 和 Workflow/提示词不变；正常 source_digest 与新 EXE 哈希随源码变化，不要求产物相同。capabilities 中的实际构建来源/EXE 身份随产物分别记录，不要求整份含来源身份的响应字节相同，也不以剔除任意差异代替静态契约核对。

删除四个没有生产调用根的函数不代表可删其曾调用的共享权限、编码或验证函数。以批准删除清单对照生产 diff，并用实际保留声明及代表编码核对不变量；不要求全生产函数相同来阻止批准的删除，也不为本批新增通用持久格式审计框架。

## 6. 最小验收与停止条件

以下为验收要求；本次实施的实际结果、沙箱首轮失败、helper skip 与资格边界见[独立验收](../acceptance/codex-configuration-rule-retirement.md)，按[测试指南](../testing.md)记录，不以设计要求代替通过事实。

| ID | 必须取得的结果 |
| --- | --- |
| CCR-01 | 完整引用/合法入口表复核；生产只删除四函数及独占 imports，未复制旧默认/分类规则；保留清单和跨包编码消费者成立 |
| CCR-02 | 28 例未知字段严格拒绝与公开 gate 拒绝、model 正反例、实际冻结的省略默认/合法显式值/模型省略、三个可选字符串的原始 JSON 空串/null 拒绝、现行消息与持久 diagnostic 脱敏、原权限/身份断言通过；基线迁移测试与删除后的结果分别记录 |
| CCR-03 | 保留生产声明/codec 不变；固定 full_access/read_only、模型省略/显式值的 ProfileConfig 及 envelope 代表编码前后相同；公开静态契约、capabilities 支持声明和 resource_digest 不变，源码/产物及响应中的来源身份单列 |
| CCR-04 | 完整 codex adapter、contracts、resources 组件及下述 Plan 指定的 application 定向通过；用正式构建脚本产出 EXE，并以同一 EXE 运行一个公开受控 TestSRSingleExecutionColdStop 通过 |
| CCR-05 | 在 docs/acceptance/codex-configuration-rule-retirement.md 独立登记范围、测试迁移/删除理由、命令、实际测试集合、失败/skip、编码及产物身份、收益和未验项；实现后才更新状态 |

一个公开受控用例只检查本机原执行、服务脱离/接回、停止受理和单次输入/封存没有受清理破坏，不作为模型默认或配置矩阵的替代测试，也不授予完整恢复资格。它包含的实际重启窗口按原用例记，不泛化到完整 RestartPreservesAuthorization。

没有生产调用根变化时，不强制完整 L1/Controlled、L2–L4、真实模型、真实 PR、Orca/Node 或 parser 专项。若实际 diff 超出批准删除闭包或影响原派发/恢复/停止规则，先修订范围和按影响补选，不能只靠本最小集合宣告充分。

CCR-01–05 完成即结束；不自动进入第二批、提交、推送、PR、合并、安装或部署。普通验收按独立文件维护，不向中央 README 添加验收表行。历史 evidence 原字节及资格不改写。

## 7. 独立设计审计修订

2026-11-10，独立只读子代理对照实际源码、长期产品契约及测试指南审计，确认一项 P2：原测试矩阵将显式空串 reasoning_effort 误列为默认 medium 的成功场景。contracts.optionalStrings/plain 在 freeze 默认应用前拒绝该值；已同步修正 §3、§4、CCR-02 和 Plan P1，区分字段省略、合法显式值与原始 JSON 空串/null，并禁止用 omitempty 重编码掩盖负例。

删除四函数的生产调用闭包、薄编码器的保留消费者、CD63/PS2 及诊断测试迁移、三组件/六项应用/一个公开受控的最小范围未发现其它需修订问题。审计及本次修订只涉及文档；未改生产/测试代码，未运行构建或产品测试，不授予任何实施验证资格。

## 8. 实施状态

2026-11-10，在上述独立设计复审后按新增实施授权完成 CCR-01–05。四个旧函数及独占 imports 删除，薄编码器及两个既有消费者、实际 profile/权限/构造保持；迁移/新增测试先在生产未改点通过，再删除生产闭包。原始 JSON 空串/null 反例保持现行拒绝，未改变公开默认或 decoder。

两个独立捕获的 222,022 字节相同，生产净删 87 行；完整三组件 131 顶层通过、1 个原 helper skip，application 六项通过，同一正式 EXE 的一个公开受控用例通过。首轮沙箱 Git 权限失败单列。构建、范围与证据见[验收](../acceptance/codex-configuration-rule-retirement.md)；没有完整 L1/Controlled、L2–L4、真实模型或部署资格，实施验收时尚未提交或创建 PR。后续按用户“提pr”新增授权提交交付，不包含合并或部署。本节是后续实施事实，不改写 §7 文档审计阶段的历史范围。
