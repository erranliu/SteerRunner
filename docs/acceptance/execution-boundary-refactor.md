# 执行与可靠性边界收敛：实施验收

日期：2026-10-20。状态：五项代码收敛与本轮验收已完成。依据 [Spec](../spec/execution-boundary-refactor.spec.md) / [Plan](../spec/execution-boundary-refactor.plan.md)。本记录不替换已有服务换代、Orca 或跨仓资格结论。

## 来源与范围

基线 `bf3ec581cb0b23a7358d95e5efa8ff5f607c2f55`；实施开始时只有本轮 Spec/Plan 和 README 文档差异。所有代码为该基线上的未提交工作树，不以 HEAD 冒充最终代码版本。测试证据位于仓内忽略目录 `.evidence/ebr-*`；原构建、最终构建及源文件清单随现场保留，不纳入 Git 的二进制交付。

原构建 `.evidence/ebr-baseline-controlled/steerrunner.exe` SHA256=`418f700a968efdadb8fb57577ab63d9b4f1d4323f113228d3f0cef8d542dc88b`。其生产源码在代码修改前构建；source_dirty 包括当时的文档差异。后续受控候选构建会读取工作树，因此原批次不作为原源码全范围通过证明。

本轮不改 CLI/schema/capabilities、state/v4、operation/v1、owner/v1 或 codec 版本，不变更原冻结输入和存储编码，不部署、不发布 PR。

## 实际职责

| 事项 | 实际实现 |
| --- | --- |
| R2 | application/artifact.go 锁内克隆原聚合，外部 Git/文件采集在全局锁外；collect 保持原 request_id/worker gate，提交前复核 revision/身份/服务状态，查询不增加 Allowed 门槛 |
| parser | operation.NewParser/MarkParserDispatch/InvokeParser/ParserReceipt 管原请求、派发模式及回执规则；codexexec.Resources 管原启动/读取与存活身份；宿主保留容量、gate/CAS、原 started 提交、业务来源和 RouteParsed |
| 续执行 | reliable.PlanContinuation 判断原资源续执行/恢复资格，application.qualifyContinuation 协调核验/恢复；resume/feedback/message 保留原诊断、问题/目录/source、授权与业务提交 |
| 模型 | execution/model.Settings 是纯值，workflow.ModelSettings 保留 alias 与原 JSON；workflow.RemoteModelBindings 在组装侧生成原白名单，cmd 显式接入，adapter 不解释业务步骤；桥启动时复制绑定 |
| 接管 | Codex/Orca 的 ValidateRecovery 和 parser.ValidateRecovery 承担私有解码；Orca 构造与只读资格复用 decodeLifecycleProfile，不为预检构造 lifecycle。宿主保留版本、材料、workflow 语义及聚合判定 |

parser 的取消继续进入原资源启动/结算路径，避免 committed in_flight 因预调用取消而失去终态 journal；普通 operation 的 context/资格校验保持。异步资源 accepted 回执携带原 parser generation 作为 resource_id，不持久化第二账本。原生 owner 生命周期和 parser 底层启动/权限/放行实现保持。

## 分阶段验证与失败记录

| 证据 | 结果与范围 |
| --- | --- |
| `.evidence/ebr-baseline/rules.log` | 执行层、adapter、host/storage 等通过；application 全组超过原 6 分钟时限，exit 1，现场保留。不能据此记完整基线通过 |
| `.evidence/ebr-baseline-controlled/result.json` | 7 个顶层场景命中，exit 1。其余场景通过，restart 的 creation_receipt 未在既有 20 秒预算到达原生屏障；保留原失败。该失败现场的 writer lease 已被测试清理移除，按精确 PID/创建时间/冻结 EXE SHA 核实原测试 owner 后结束其 Job，owner 与 child 均退出，见 residue-cleanup.json |
| `.evidence/ebr-p1.log` | 并发/停止/CAS 验证通过；原响应与存储重放的 JSON 字段顺序比较导致测试失败，修正为 JSON 值比较 |
| `.evidence/ebr-p1-p2a.log` | R2、parser 获取/迟到/重复/冲突/原子恢复/容量与来源组通过，326.859s |
| `.evidence/ebr-p2b-p3a.log`、`ebr-p2-p3-targeted.log` | 模型桥/守卫、恢复与换代、parser 派发提交失败等定向通过；后者 application 21.454s |
| 接管只读新增测试 | 早期断言将纯值快照与带 port 的 setup 比较，或用 Clone 的 nil/空切片变化当成状态变更；已改为仅比较原纯值编码，并为反例生成有效摘要。实际生产解码门槛不放宽 |
| `.evidence/ebr-native/model-bridge.log` | 原生探针第一次未按既有 helper 时序 bootstrap/resume 原 thread，等待通知超时，exit 1；原结果保留，探针修正后另建现场 |
| `.evidence/ebr-native-2/` | 修正后的探针证实空 thread 尚无 rollout，另一个 parser 探针发现新资源端口 accepted 回执缺 resource_id；分别补齐测试 bootstrap 和原 generation 回执，再新建现场 |
| `.evidence/ebr-native-3/native.log` | 两个真实 Codex 0.159.2 原生探针通过：模型桥 15.47s，parser 18.39s；均非受控协议夹具 |

原生模型桥使用实际能力目录可用的 `gpt-6.1-sol` / `gpt-6-luna`，原 thread bootstrap 后通过桥完成 implement→audit→publish 三轮；原生 rollout 的 turn_context 核实模型/effort，执行模型恢复，零工具调用，原树退出证明在测试收尾核实。证据 `model-bridge/native-models.json`。

原生 parser 使用现只读获取配置与冻结输入，通过 Resources→operation 共同入口完成；原回执确认资源停止/写者退出，unknown 路由不重发，已取消请求生成失败结算且零原生根放行。证据 `parser-resources/native-parser.json`。这只证明所改本机原生机制；不宣称 Orca TUI/远端创建关联或跨仓部署完整验收。

## 最终收口

最终冻结生产源码清单：`.evidence/ebr-final/production-source.json`，共 23 个变更生产文件，收尾逐项复核摘要一致。最终 EXE 为 `.evidence/ebr-qualified-controlled/steerrunner.exe`，SHA256=`ae5ce13afd30d882db9070db1f6a6ec6dc019d8ff4e52c489243fe6eba8c495f`。同源码兼容候选只修改资源空白/产品版本，用于原构建→最终执行语义的接管证明；不能把该候选的不同摘要误写为主 EXE 摘要。

| 最终检查 | 实际命令/证据 | 结果 |
| --- | --- | --- |
| 全部 internal 包 | `go test ./internal/... -p 2 -count=1 -timeout=20m`；`ebr-final/qualified-rules.log` | exit 0；application 657.235s，其余 owner/契约/资源/CLI/testkit 同组通过；不把无测试包当作新增测试命中 |
| 静态检查/构建 | `go vet ./internal/... ./cmd/...`；最终 CLI 构建及独立 agent helper 构建 | 通过；helper SHA=`11328456ddf92821ca92174c4a43af4fc5c1b3baa8736adf2296d5850be2f07e` |
| 受控闭环 | `ebr-qualified-controlled/result.json`、integration.log、caller-commands.jsonl | exit_code=0、scenario_count=8，全部 8 个顶层通过，零失败/skip；获取 11 个子场景、换代 4 个子场景均通过 |
| 原生机制 | `ebr-native-3/native.log`、native-models.json、native-parser.json | 实际 Codex 0.159.2；模型桥及 parser 各 1 项通过，已核实原会话/模型或结果及资源退出 |
| 公开契约差分 | `ebr-final/original-capabilities.json`、qualified-capabilities.json、capability-comparison.json | 原/新离线输出仅 capability_source.executable_sha256 不同；其他能力、schema、workflow、恢复声明相同 |
| 文档/源码 | Spec/Plan/README 及本记录；`git diff --check`、相对链接和冻结源码摘要 | 通过；无格式/协议/版本增量，无提交/部署/发布动作 |

受控命令为：

```powershell
.\scripts\test-foundation.ps1 -OutDir .evidence/ebr-qualified-controlled -BaselineExe .evidence/ebr-baseline-controlled/steerrunner.exe -Pattern '^(TestSRRunWorkflow|TestSRNativeUnknownAndStop|TestSRContentGenerate|TestSRContentConversation|TestSRContentQuestionAndUnknown|TestSRRunEvidenceR2|TestSRRestartPreservesAuthorization|TestSRResultAcquisition)$'
```

实际耗时较长的 application/获取组使用测试进程总时限；生产准备、原生操作、parser 绝对期限、停止和清理预算均未改动。阶段构建的七场景通过结果 `ebr-final-controlled` 仍保留，但最终收口使用以上冻结源码的八场景，避免拼接不同构建。

另补 `TestRARecoveredParserStartedReceiptStaysRunning`：原 started journal 即使含 ParseEvidence，也先按 accepted/running 结算，不能消费成最终获取。直接反例通过（13.949s），最终全 internal 与 parser restart/kill 受控路径共同复验通过。

### 验收映射

| Spec 条件 | 结果与主要依据 |
| --- | --- |
| EBR-A00 | 文档轮检查及本次相对链接/阶段/状态收口完成 |
| EBR-A01 | R2 慢 I/O 的跨 Run stop/事实提交、同 Run gate、CAS/来源冲突、提交失败/原请求重放通过；实际 EXE 原字节读取与跨构建两次重启通过 |
| EBR-A02 | 普通/parser 共用原操作调用规则与回执更新；派发/根身份提交失败、未知零重发、容量/期限、重复/冲突/迟到/来源及 started/final 分流通过；最终获取 11 子场景全过 |
| EBR-A03 | resume/feedback/message 共同资格与协调；原 session/idle/writer/unknown 及目录/问题/source 的正反例通过 |
| EBR-A04 | 原授权、等待/阻断、stop/close/里程碑保持；原 owner/parser 在停服/强杀后继续，原问题/控制/结果不重发，启动 pending stop 不被后继越过 |
| EBR-A05 | 反向依赖守卫与负例、原模型配置字节/摘要、错步骤/模型/会话拒绝通过；实际 rollout 证实同 thread 模型切换和恢复 |
| EBR-A06 | 私有解码归实现；Codex owner/能力/旧认证/codec 与 Orca 构造/只读校验拒绝边界通过，纯值编码保持、零原生调用；实际原构建接管通过 |
| EBR-A07 | 最终同源码全 internal、vet、8 顶层受控及必要原生机制验收完成；原操作、结果、产物的接管/写回/再重启通过 |

### 存量样本与边界

`ebr-parser-restart-original-v4.db` / `ebr-parser-kill-original-v4.db` 为原 writer 退出后的隔离 DB 副本，原 execution owner/parser 保留，不将停服误称资源结算。最终构建沿原请求读取/结算；受控检查原 parser 只启动一次、原会话与来源不替换、后继输入次数不增加，显式 stop 确认原资源退出。R2 用原构建收集产物，最终构建接管并两次重启，artifact ID/原字节/原冻结摘要保持。样本与 caller 原命令审计都保存在本轮证据目录。

仅记录本轮相关机制的资格：不扩为完整 Orca TUI/远端创建关联、任意多组件崩溃、主机迁移或 SteerLine 部署验收；原有缺口保持。本轮未操作用户安装、非测试 state-dir 或真实 PR。
