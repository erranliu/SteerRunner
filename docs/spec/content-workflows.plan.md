# 实施计划：内容流程

依据 [Spec](content-workflows.spec.md)，验收条目唯一维护于[专属验收](../acceptance/content-workflows.md)。

1. contracts 按 workflow 分离输入校验，发布真实 schema/capabilities/类型。
2. workflow 增加薄定义与无补报结果政策；freeze 复用材料固定和 Profile/Record，目录只读不创建 Git。
3. Codex 权限 owner 增加 read_only；复用原 session 初始化、问答、恢复、对账和 owned Job。
4. application 原 actor 内接线 message/interrupt/close、真实历史、最终结果及独立结算。
5. 增加最小受控反例、公开 CLI 驱动和隔离真实 Codex 验证；修复后记录实际证据。

| 验收 | 入口/环境 | 命令与时点 | 通过条件 | 证据 |
| --- | --- | --- | --- | --- |
| CW01 输入/隔离 | contracts/resources 与快照单元，本机无模型 | `go test ./internal/contracts ./internal/resources -count=1`；`go test ./internal/application -run '^TestContent' -count=1`，契约完成及收尾时 | 新输入有效、材料不随原对象变化、开发必填不松动、schema/capability 一致 | `rules.log`、`content-tests.log`、`owner-tests.log` |
| CW02 单轮结果 | application/resultacquisition 单元及真实 CLI/受控原生夹具 | `scripts/test-content-workflows.ps1`，接线后、真实运行前 | 文本/JSON/错误/失败/取消/缺 final，只有一轮；认证/目录失败零轮 | `controlled-integration.log`，生成 10 个分支 |
| CW03–07 会话可靠性 | application/Codex 单元及 CLI 夹具，独立 state-dir | 同一专项入口自动选 `TestSRContentGenerate|TestSRContentConversation|TestSRContentQuestionAndUnknown`；收尾追加 owner 包回归 | 同 thread、问答身份、互斥、interrupt/close、未知发送/中断零重发、恢复身份和独立结算 | `controlled-integration.log`、`owner-tests.log`、`content-tests.log` |
| CW08 真实结构化生成 | 新 `C:/srt/cw-*`、非 Git 空目录，Codex 0.159.2 | `scripts/test-content-workflows.ps1 -NativeExecutable <原生EXE> -Model gpt-6.1-sol`；受控通过、同源码构建后 | 原生 account/read 和只读目录探针、本次线程初始化、单个业务 turn、JSON 校验成功、资源 stopped | `native-generation.json`、`native-integration.log`、`build-manifest.json` |
| CW09 真实两轮与中断 | 同一 driver，两个 Run 身份独立 | 与 CW08 同一命令，在初始/第二轮后执行 interrupt 和显式继续 | 原 thread 五轮中的一轮明确 cancelled，随后上下文继续 | `native-interrupted.json`、`native-after-interrupt.json`、`native-history-final.json` |
| CW10 真实重启 | 同上，独立服务换代 | 与 CW08 同一命令；空闲时停服重启、message、close，收尾另调用公开 `service verify-stopped` | 保留原授权、root/thread/context，空闲无新输入；无需因重启 resume，关闭拒绝新消息、全部资源结算 | `native-restarted.json`、`native-after-restart.json`、`native-closed.json`、`verify-stopped.json` |

## 补充测试计划：内容接管配置适用性

2026-10-20，CW07/CW10：修正内容测试夹具，保持与生产 freeze 一致的不含开发 parser 配置。Windows 隔离目录执行 `go test ./internal/application ./internal/execution/resultacquisition ./internal/execution/workflow ./internal/execution/adapters/codex -run 'TestContent|TestServiceGeneration|TestWorkflowRecovery|Test.*Recovery|Test.*Config' -json -count=1 -timeout=180s`；修复前先用内容接管两项复现失败，修复后再核验 queued/result 的 generate/conversation、实际 admission、原材料和 codec 反例及开发配置缺失拒绝。原 Record 不改写。

随后由 `scripts/test-foundation.ps1 -Pattern '^TestSRContent(Generate|Conversation|QuestionAndUnknown)$' -OutDir <新目录>` 验证三个真实 EXE/受控流程并核对 result.json 非零命中/退出零。跨仓真机使用 SteerLine `scripts/test-test-management.ps1 -SteerRunnerExe <A> -RunnerCandidate <B> -Out <新目录>`，A/B 为同源码不同版本/SHA，在其统一生成的独立调试包验证原生成 Run 跨构建接管后继续报告/API/浏览器及结算；原生成材料、模型和权限保持，只有传输 pin 更新。证据与失败/补测分别登记在[专属验收](../acceptance/content-workflows.md)，不要求完整 L2、全原生矩阵或 Full。

不默认运行 full。相关包回归与 build 在定向测试后执行；真实失败保留现场与诊断，不能由受控通过代替。

实施完成。表中证据均在
[content-workflows-20261019](../acceptance/evidence/content-workflows-20261019/README.md)。
实际完整命令、受测版本与未覆盖范围见[验收登记](../acceptance/content-workflows.md)。
最终 `content-final-receipts` 专项通过，当前生产源码重新构建的 SHA256 与真实受测 EXE 相同。

PR 阶段在隔离检出合入最新 main，按同一测试计划复验；源码、受测 EXE、公开能力与逐项结果见[PR 复验登记](../acceptance/evidence/content-workflows-pr-20261019/README.md)。

2026-10-20 服务换代合并修订：上述历史证据仍对应原构建及其显式 resume 行为。当前 CW10 driver 已按共同恢复契约调整；新增受控与规则验证、真实原生待验边界另登记于[服务换代验收](../acceptance/service-generation-recovery.md)，不把历史结果改记为新行为通过。
