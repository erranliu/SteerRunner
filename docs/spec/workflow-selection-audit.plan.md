# Plan：内置 Workflow 选择与审计模型

日期：2026-10-15。用户已授权按精简后的 [Spec](workflow-selection-audit.spec.md)实施；实现复用现有 Route、实施/报告步骤与原执行链路。阶段及实际验证结果见第 5 节。

## 1. 基线与本轮审计

原文档调查基线为 `af545563c58171c584bd35d098166cf39012e699`。创建文档前工作区干净；本轮继续修改上一轮未提交的 Spec/Plan，保留 README/原 Spec 的入口。实施开始重新固定真实代码/文档版本和摘要，不把[首版验收](../acceptance/steerrunner.md)的历史结果作为新能力证据。

本轮为当前聊天自审，不宣称独立审计。确认并修订以下问题：

| 原设计问题 | 不必要成本 | 修订 |
| --- | --- | --- |
| 可序列化流程图、typed handler 注册及图校验 | 两条内置流程就要求通用引擎；外部装载尚未需要 | 轻量配置 + 现有 Route，保留代码转移 |
| 临时 index、候选 tree、发布前后与远端 tree 证明 | 将 workflow 选择扩展为新的成果证明/发布协议，还引入过滤器及子模块限制 | 只校验原实施/审计来源及门槛，沿用现有 PR 核验 |
| 预设 state/v3 并拒绝全部旧目录 | 现有 Run 为 JSON 扩展字段，没有证据说明必须破坏存储兼容 | 保留 v2，新字段可选；测试旧记录/原意图兼容 |
| repair.execute/report、新 step instance、按实例重试表、审计补报 | 复制现有实施/报告及 command/turn 机制，放大恢复状态 | 修复复用原步骤/身份；新实施重置现有报告补正计数；坏审计格式阻断 |
| 全步骤公开历史、多个目录/定义版本摘要、必填 finding ID/目标摘要 | 当前调用方只需选流程和模型、看审计结果，增加数据契约而无直接收益 | 最小 workflows/audit 投影，复用资源摘要与原结果字段 |

保留两项有直接用途的约束：修复预算防止自动循环无界；逐轮冻结模型并核验原操作，防止回切和恢复时发送错误参数。新流程要求显式填写已有 execution model，是避免增加“探测默认模型”机制的本期取舍，普通流程不受影响。

## 2. 改动边界

| 位置 | 必要改动 |
| --- | --- |
| `internal/contracts/run.go`、`view.go` | audit_model、流程相关校验、最小 audit 与请求模型投影 |
| `internal/resources`、`assets` | 两流程目录/轻量配置、两审计提示词、既有 schema 的新增字段 |
| `internal/execution/workflow` | Lookup、审计步骤/结果、在原 Route 中加入审计分支和回实施循环 |
| `internal/application/freeze.go`、`results.go`、`control.go`、`worker.go` | 冻结配置/参数，实施来源、计数、审计问题回答与后继输入的原子提交 |
| `internal/runstate/record.go`、workflow Snapshot | 可选审计配置/结果、当前实施报告关联、自动修复计数；复用现有阶段记录 |
| `internal/execution/backend/session.go`、`reliable/intent.go`、`adapters/codex` | TurnInput 的 model/effort、SameInput、prepared input 比较和原 turn/start 映射 |
| `tests/helpers/native`、`tests/integration`、`tests/native` | 逐轮模型断言、修复及恢复场景、最小原生闭环 |

不新增图引擎包，不改 workspace/publication 的业务协议，不更改 storage Format/表结构。不修改 SteerLine UI 或创建新的模型 SDK 调用路径。

## 3. 实施步骤

### P0 固定基线和接口

1. 运行第 4 节 B0/U0，保留实际结果。既有环境失败不伪装成新功能失败或通过；新代码造成的回归必须修复。
2. 固定两流程 ID、audit_model 字段、workflows 目录及最小 inspect.audit；只扩展既有公开 schema。更新严格校验和普通流程兼容测试。
3. 建立纯值轻量配置和 Lookup，包含审计顺序、允许覆盖步骤、修复上限。目录与实际路由共用它；不要在 resources→contracts→workflow 之间形成反向依赖。可由宿主把选中描述传给纯输入校验。

### P1 配置冻结与逐轮模型

1. 新审计 Run 接受时保存配置快照、PromptBodies、规范化 execution/audit 参数。新轮次依据冻结值组装；旧普通 Run 不回填当前默认配置。
2. StageInput/PendingTurn/原 LifecycleInput 携带 model/effort，更新 `reliable.SameInput`、Codex prepared input 比较、克隆及原请求摘要校验。新增字段采用 json omitempty，禁止因读取旧意图而改变其原序列化字节，不为旧 PendingTurn 回填 model。
3. Codex turn/start 使用该轮冻结参数；普通流程没有覆盖时沿用原 Profile 配置。制造审计模型不可用和参数篡改反例，证明零回退、零替代会话、零盲重发。
4. 用旧 v2 Run 和带未核清操作的旧记录夹具验证新构建加载/对账，单独断言 raw request/payload digest 不变；新审计配置缺失拒绝推进。无需先升级数据库格式来回避兼容测试。

### P2 审计与复用实施修复

1. 添加 audit.spec/code 提示正文，复用材料引用；审计 StageInput 的来源绑定当前成功实施报告 command/turn。扩展结果解析，复用 outcome/blocker 与现有严格 JSON 规则，只新增 verdict/findings。
2. 原 implement.report done 分支按所选配置进入 publish 或首项审计；审计 pass 进入下一审计/发布；changes_requested 携带阻断 findings 回到 implement.execute。发布前检查全部 pass 属于当前实施报告。
3. 保存自动修复 PendingTurn/NextStage 时递增持久计数，恢复该意图不再扣预算；每次新实施都清除当前有效 pass 并重置原 ReportRetry，报告重试或恢复不重置。修复完成仍走原 implement.report；不增加 repair/report 实例类型或审计格式补正分支。
4. 审计格式错误阻断、原生失败保留原失败、缺可信 final 仅对账。普通流程原实现/报告/发布行为不变，原 provider 仍负责 PR 归属。

### P3 反馈、停止、恢复与查询

1. 修正 `control.go` 固定 `Option: implement` 的使用边界：显式反馈重入实施；审计问题答案继续原审计并保留来源；未结束原生问题仍用原精确 route。
2. 复用现有 gate/CAS/NextStage。结果接受、计数、pass 失效、后继输入和事件同事务提交；重复观察不能重复扣预算。stop 与审计结果竞争时，撤销授权后不得发实施或发布。
3. 重启零输入；resume 先对账原未知操作，继续冻结步骤与模型。业务反馈、重启和 resume 不重置 Run 自动修复总预算，也不跳审计直达 publish。
4. 最小 audit 投影只显示当前实施报告的结果和计数，历史沿用现有结果/事件；模型只标请求参数。更新 README 的发现/选择/模型样例，无存储目录强制切换。

### P4 定向集成与最小真机

1. 扩展已有 controlled helper 和实际 EXE 测试：两流程选择、通过、修复后重审、预算耗尽、审计答案、stop/restart/unknown、旧流程回归。用原操作记录证明 A→B→A→B→A 同 thread/workspace。
2. controlled 通过后，用本仓 `new-test-fixture.ps1 -Scenario local-codex.smoke -Config <配置> -AuditModel <审计模型>` 生成新的独立现场，再通过 `test-native.ps1` 运行真实审计流程。配置中显式指定执行模型，两个模型须不同且实际可用；可用 `-AuditReasoningEffort` 覆盖继承值。共用现有 native driver 的准备、实施、PR 核验和清理链路；审计诊断在已验证 PR 后停止，不进入普通 smoke 的反馈/合并阶段。接受业务 Run 前，用 `git ls-remote --heads origin refs/heads/<本包base>` 核对 revision 等于 manifest.base_revision；native 驱动会拒绝未准备的 base。自然出现 finding 时保留实际修复证据；不人为要求模型一定发现问题。旧 SteerlineDebug 证据仅作为历史记录保留。
3. 真实 final、原会话和 provider PR 归属按既有标准核验。不新增 head tree 证明或全模型矩阵，不替换旧运行包。通过公开 stop/inspect 清理原 Run，保留失败现场及清理证据；不自动合并或 Release。

## 4. 最小验证安排

以下新增测试入口已随本次实现提供；执行时仍须核实命中场景数，零测试或 skip 不计通过。所有命令在仓库根执行。

| 编号 | 安排与时点 | Spec 映射 |
| --- | --- | --- |
| U0/B0 | 实施前固定旧回归；相关控制逻辑完成后复验一次 | WA-02/06/07 |
| U1 | 输入/目录、最小审计结果、路由/预算、模型原意图相等性和旧 v2 记录 | WA-01–07 |
| C1 | 真实 EXE + controlled 原生/provider，选择、修复/重审、预算、回答、停止/重启与未知结果 | WA-01–06 |
| N1 | 两不同模型真实同会话审计、切回执行并发布一次测试 PR | WA-08 |
| D1 | 本地链接、JSON 示例、验收映射、git diff --check；代码变更完成后构建及受影响包 go vet | 文档/构建，不替代行为测试 |

```powershell
# U0：现有定向包回归。
go test ./internal/contracts ./internal/execution/workflow ./internal/application -count=1 -timeout=2m
# B0：现有实际 EXE 入口。
./scripts/test-foundation.ps1 -Pattern '^(TestSRRunWorkflow|TestSRPublicationIdentity|TestSRNativeUnknownAndStop|TestSRRestartRequiresResume)$'
# U1：新增入口，每个用例用 subtest 覆盖相关反例。
go test ./internal/contracts ./internal/resources ./internal/execution/workflow ./internal/execution/reliable ./internal/execution/adapters/codex ./internal/application -run '^(TestSRWorkflowSelection|TestSRAuditContractAndRouting|TestSRTurnModelBinding|TestSRAuditLifecycle|TestSRFrozenInputRestoration|TestSRWorkflowLegacyRecord|TestSRWorkflowDevelopment|TestSRExecutionDependencyBoundary)$' -count=1 -timeout=2m
# C1：两个新增实际 EXE 测试入口，分别覆盖选择与审计闭环/控制故障。
./scripts/test-foundation.ps1 -Pattern '^(TestSRWorkflowDiscoveryAndSelection|TestSRAuditedWorkflow)$'
# N1：本仓生成新现场；配置中的执行模型与审计模型须不同。
$auditManifest = ./scripts/new-test-fixture.ps1 -Scenario local-codex.smoke -Config '<配置绝对路径>' -AuditModel '<审计模型>' -AuditReasoningEffort high
./scripts/test-native.ps1 -Manifest $auditManifest
```

B0/C1 异步返回证据目录；等待该目录 result.json，并核对 exit_code/scenario_count，不能将启动成功计作通过。现行 native 脚本等待结果后返回；N1 仅验证原生切模型及最小闭环，故障分支主要由 U1/C1 覆盖，不重复整个矩阵。N1 是本文验证项编号，与统一测试入口的 L1 组件测试层级不同。

## 5. 结果登记与完成条件

证据使用新的 `.evidence/workflow-audit-<timestamp>` 或脚本生成目录；live 证据在新 debug 包。记录代码/文档版本或摘要、EXE/资源摘要、请求模型/effort、原生版本、state-dir、命令/数量/退出码及原失败。按实际结果报告，不继承首版单模型证据。

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 本轮 Spec/Plan 自审 | 2026-10-15 修订及文档检查通过 | 五类过度设计已移除；4 份变更文档的 19 个本地链接、2 个 JSON 示例、8 项验收映射及新文档表格/围栏/空白检查通过；git diff --check 通过。不是独立多代理审计，不计运行验收 |
| 实施与 unit | 已实现，定向通过 | `.evidence/workflow-audit/unit.jsonl`：8 个有测试的包、192 个 pass 节点、0 fail；1 个 subprocess helper 入口 skip，不计业务验收。`go build ./...` 及受影响包 `go vet` 通过，见 build.log/vet.log。存储仍为 v2，无新增依赖 |
| 实际 EXE controlled | 11 场景通过 | `.evidence/foundation-20261015-093235/result.json`：exit 0，普通流程 4 场景及新流程发现/审计 7 场景。覆盖修复重审、预算、坏 JSON、模型不可用、原问题回答/重启、未知输入零重发和停止；原 thread/PR 复用 |
| 两模型原生闭环 | 最终通过 | `C:/SteerlineDebug/workflow-audit/dev-20261015-175855-ef36a4a1/evidence/native-20261015-100123/result.json` exit 0 / 1 场景、276.41 秒；执行 gpt-6-luna/medium，审计 gpt-6.1-sol/high，同一原 thread 完成实施、报告、两项审计、切回发布及 Runner provider 核验。Run `ae55627a857bb9aea4f3e3874330dfeb` 达到 pr_ready，原生 final.verified=true；[测试 PR #260](https://github.com/erranliu/SteerlineTest/pull/260) 为 OPEN draft，未合并。包内 evidence/runner-native-local-codex-cleanup.json 确认 stop_status=stopped、new_inputs_allowed=false |

基线 `.evidence/foundation-20261015-090943/result.json` exit 0 / 4 场景；U0 见 `.evidence/workflow-audit/baseline-unit.log`。扩展的前两轮受控失败保留在 `foundation-20261015-091934`、`foundation-20261015-092828`：首轮发现空原生日志/旧 revision 的测试夹具问题；修正夹具后确认审计问题在重启后回答会被拒绝。根因是无新输入时提前切换 Attempt，原历史证明随后无法核验。现在延迟至新输入准备好才切换，并复用同一恢复函数，仍先核验原会话空闲、原写者退出及原绑定；未放宽资格条件。最终 11 场景通过不改写这两次失败。

首轮实现构建来自 `af545563c58171c584bd35d098166cf39012e699` 加当时未提交工作树，EXE SHA-256 `c3111a8c9a91b9309bc823eac44fac3ff554c1a661b89f651c21958e3a826dd4`。历史构建来源详见 controlled 的 source-identity.json；修改文件摘要见 `.evidence/workflow-audit/source.json`。这些记录不代表之后审计修复构建的来源。

原生前两轮失败保留：

1. `dev-20261015-173500-0479ebcc/evidence/native-20261015-093745` exit 1 / 1 场景。远端缺专属 base，GitHub 拒绝创建 PR；两项审计及模型回切已完成。原 Run `3986f56a6a2ffc8255addc22e6e69ca9` 已 stopped，推送的专属测试 head 保留。不计完整通过。
2. `dev-20261015-174527-ef36a4a1/evidence/native-20261015-094835` exit 1 / 1 场景。[测试 PR #259](https://github.com/erranliu/SteerlineTest/pull/259) 实际已创建，原生成功命令 stdout 也包含 URL；模型随后把普通 MXC 查询的 401 当作发布未知，Runner 保持 reconciling，未虚报 pr_ready。通过公开 stop 停止原 Run `1eb97d65232a24e2f9111cf29e67d7e6`，cleanup 确认 stopped 且零新输入授权，未合并 PR。

第三轮补齐远端 base 前置，并在该新 Run 冻结的 publish 提示词覆盖中明确：等待原发布命令完整退出、采用退出 0 的完整回执 URL，不以之后未审阅查询的 401 覆盖成功回执，最终仍交给 Runner 独立核验。三个包的 Runner EXE 均为上述同一 SHA，未为测试修改发布状态门槛。最终真机结论限于这两种模型和该冻结提示词配置，不声明任意模型/default 提示词组合都已验证。

最终新增 README 示例后，4 份文档的 19 个本地链接、3 个 JSON 示例及两份资源 JSON 解析通过；格式与 `git diff --check` 通过。构建输出为 `bin/steerrunner.exe`，来源和 SHA 见同目录 steerrunner.exe.build.json。

WA-01–07 的 unit/controlled 通过后，才可交付相应功能；WA-08 真正通过后才能声明本机两模型审计已真机验证。原生环境阻断则保留缺口。不得为简化实现放宽原会话、原 final、停止授权和 PR 归属门槛。

## 6. 实现审计后的修复

2026-10-15 实现审计确认并修复两个 P2 问题：

1. 审计答案已冻结到 PendingTurn、尚未发出时，原进程退出后的同会话恢复会重新渲染并丢掉答案。现在保留原 StageInput 的完整 prompt/model/effort，通过原 queueInput 更新 attempt/command 绑定。新增 `TestSRFrozenInputRestoration` 从 SQLite 重载输入，覆盖审计回答、人工反馈、自动修复、报告补正四个窗口，检查原来源、完整原生输入及两类预算保持。
2. 已知结果字段去重与 encoding/json 的 Unicode 折叠不一致，`findingſ` 可覆盖含 P1 的 findings。现在用 EqualFold 匹配已知字段再去重；新增正反顺序、嵌套 severity 和真实 EXE 阻断反例，不能升级为 pass 或进入发布。

提交前同步到 `57d6f6f` 的 main，保留基线刷新与逐阶段准备期限修复。审计 native 诊断迁移至本仓现有 manifest/driver；没有保留旧 SteerLine 测试驱动或新增生产执行链路。第一轮完整 L1 `.evidence/workflow-audit-fix/summary.json` 因上游新增的长回合测试夹具缺少 workflow ID 失败；补齐夹具的 development/1 标识，保持生产校验不变，随后重跑。首轮失败证据保留。

| 修复后验证 | 结果 | 证据 |
| --- | --- | --- |
| 完整 L1、原有 Controlled | 通过；159 个顶层 pass，5 个入口允许的 helper/显式真机取证 skip；Controlled exit 0 / 10 场景 | `.evidence/workflow-audit-fix-final/summary.json`；包括全部内部包、9 项框架断言、native driver 编译和零匹配拒绝门槛 |
| 审计 Controlled | exit 0 / 8 场景，无 skip | `.evidence/workflow-audit-fix-controlled/result.json`；新增 Unicode 别名阻断，保留修复、预算、坏 JSON、模型不可用、问题重启及未知结果场景 |
| 构建、静态及脚本检查 | 通过 | `go build ./...`、受影响包 go vet（`.evidence/workflow-audit-review/build-fix.log` / `vet-fix.log`）；两份变更 PowerShell 解析、资源 JSON 解析及 git diff --check |

上述最终测试来源为 `142f01c` 加本轮后续测试/文档补丁，构建身份见各 evidence 的 source-identity.json；审计 Controlled EXE SHA-256 为 `8e8f033757860b0a8c30e980d592e164e371de34efc25fcdf358211f821e6d52`。本轮未重跑两模型真机；前述旧包的真实 PR 证据不扩展为审计修复后或迁移后 driver 的真机通过声明。
