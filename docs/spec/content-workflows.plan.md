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
| CW10 真实重启 | 同上，独立服务换代 | 与 CW08 同一命令；空闲时停服重启、resume、message、close，收尾另调用公开 `service verify-stopped` | 重启零自动输入、同 thread/context、关闭拒绝新消息、全部资源结算 | `native-restarted.json`、`native-after-restart.json`、`native-closed.json`、`verify-stopped.json` |

不默认运行 full。相关包回归与 build 在定向测试后执行；真实失败保留现场与诊断，不能由受控通过代替。

实施完成。表中证据均在
[content-workflows-20261019](../acceptance/evidence/content-workflows-20261019/README.md)。
实际完整命令、受测版本与未覆盖范围见[验收登记](../acceptance/content-workflows.md)。
最终 `content-final-receipts` 专项通过，当前生产源码重新构建的 SHA256 与真实受测 EXE 相同。
