# GEN-REPO 验证与接入（2026-10-27）

实现范围：扩展 generate/1 的可选 repository；沿用 Codex adapter、Run/SQLite 请求账本、
完整 final、inspect/history、停止与原会话恢复。未修改 SteerLine，未新增 workflow、配置、
执行器、发布/任务机制。业务格式由调用方 Schema 和指令定义。

接入：[接口、大小边界与限制](../workflows/generate.md#固定仓库版本调查)、
[完整请求示例](../workflows/generate-repository.example.json)、
[Spec](../spec/generate-repository.spec.md)、[Plan](../spec/generate-repository.plan.md)。

## 受控验证

- 定向单元通过：固定旧 commit（源 HEAD 已有新提交）、脏/未跟踪文件隔离、材料固定、来源
  blob/SHA-256、恢复不读取移动源目录、快照注入/漂移拒绝、多文档完整正文、缺文档/坏 JSON/
  Schema 不符/缺完整 final/超限非成功、1 MiB 边界、输入与公开 Schema 一致性。
- 仓库资格通过：链接、保留路径、大小写冲突、4096 文件/64 MiB 总量拒绝；环境 GIT_DIR
  无法重定向指定仓库。使用 Git 对象，不运行 checkout/filter。
- 公开 CLI 通过：`TestSRContentRepository` 的 complete、active-restart-stop、
  unknown-restart-stop；同 request_id 重放指向原 Run；重启前后固定来源不变且 turn/start
  仅一次；原 stop 后再次重启无新业务执行。
- 同轮原有 `TestSRContentGenerate|TestSRContentConversation|TestSRContentQuestionAndUnknown`
  全通过，合计 4 个顶层受控场景。包含原生失败/取消、缺 final、人工问答、认证/目录失败。

证据：[受控日志](evidence/generate-repository-20261027/controlled.log)、
[受控结果](evidence/generate-repository-20261027/controlled-result.json)、
[定向单元日志](evidence/generate-repository-20261027/targeted.log)、
[边界日志](evidence/generate-repository-20261027/focused.log)。
首轮受控多文档断言误比较 JSON 字段顺序，结果内容实际完整；改为比较路径与正文后通过，
首轮失败原始记录保留在 `.evidence/generate-repository-controlled`。

额外整包回归中，contracts/resources/workspace/Codex/reliable/resultacquisition/host/storage/CLI
均通过；application 整包累计耗尽 5 分钟预算，当时 `TestRAConcurrencyParserStopResources`
运行 7 秒，无断言失败。该用例随后单独复验通过（8.90 秒）；定向内容/恢复测试通过。
当时未取得 application 整包通过结论。原始超时日志保留在
`.evidence/generate-repository-regression.log`；[独立复验](evidence/generate-repository-20261027/budget-recheck.log)。

2026-10-27 补充整包复验：`go test ./internal/application -count=1 -timeout=30m -json`
在干净源码 `8fc1b0bc79dd89b00ee118de95b649855aa9a27f` 上完整通过，退出码 0。
141 个顶层测试、含子用例共 333 项通过，0 失败、0 跳过；包内耗时 451.112 秒，
完整命令耗时 453.758 秒（上海时间 05:52:33–06:00:07）。上次预算耗尽时正在运行的
`TestRAConcurrencyParserStopResources` 在本轮整包中通过，耗时 9.38 秒。
结论更新为 application 整包通过；未扩展到完整仓库全集。

证据：[汇总与源码/EXE 身份](evidence/generate-repository-20261027/application-full-summary.json)、
[完整测试日志](evidence/generate-repository-20261027/application-full.log)、
[原始 Go JSON 事件](evidence/generate-repository-20261027/application-full.events.jsonl)。
本次没有真实失败，无需修改实现或测试代码；仅补充验证记录。交付 EXE 继续使用下述
`0.2.0-dev.3-r3`，源码仍为 `8fc1b0bc79dd89b00ee118de95b649855aa9a27f`，
SHA-256 为 `F45830F3DE8780EF049F56FA56CA941B426C018FB0826CE33A4142B4D8689A02`。

## 真实 Codex 验证

本机原生 Codex **0.159.2**，模型 **gpt-6.1-sol**，reasoning low，read_only，
`TestSRContentRepositoryNative` 通过。一次业务 turn 真实运行 `Get-Content code.txt -Raw`，
读取 Run 私有快照的 `FIXED_SOURCE_47`；调用方工作树实际为 `DIRTY_SOURCE_99`。
Spec/Plan 两份完整正文均包含固定内容和 commit；返回程序生成的文件 blob/SHA-256。
完成后服务重启、原 request 重放保留原 Run/session/turn；显式停止与离线
`verify-stopped.resources_settled=true`。

证据：[原生日志](evidence/generate-repository-20261027/native.log)、
[请求](evidence/generate-repository-20261027/repository-native-request.json)、
[完整公开结果](evidence/generate-repository-20261027/repository-native-result.json)、
[实际读取命令及输出](evidence/generate-repository-20261027/native-read-command.json)、
[结算](evidence/generate-repository-20261027/repository-native-verify-stopped.json)。
同一 turn 中补充 `rg --files -g AGENTS.md` 因无匹配返回 1；首次读取成功，最终 turn 成功，
不把每条工具命令都描述为成功。首次选择 gpt-5.4 被账号返回 model_not_found，明确失败并
完成关闭；换用本机现有模型配置后另作新测试。该失败记录在 `.evidence/generate-repository-native`，
不作为未知执行重试，也不算通过。

未验：真实 Codex 运行中强杀/未知发送故障（仅受控覆盖）；大仓库最大规模原生生成；
SHA-256 Git 仓库现场；SteerLine 业务文档验收及接线。只读 sandbox 不等于单目录读隔离；
manifest 证明可用代码版本，不能证明模型逐文件阅读或业务结论正确。
合法 JSON 中业务正文是否写完必须由 SteerLine 验收；调用方 Schema 应约束预期相对路径、
文档数量、必填格式和内容。

## 可追溯交付

版本 `0.2.0-dev.3-r3`，Windows/amd64。工作区 `bin/steerrunner.exe` 是接入入口；
相邻 `.build.json` 记录源码 commit、source_dirty 和 SHA-256。
`bin/generate-capabilities.json`、`bin/generate-input.schema.json` 由交付 EXE 实际导出。
交付构建专项 CLI/原生复验记录在 `.evidence/generate-repository-release`；应核对其
source-identity 与交付 EXE 的 SHA-256 相同。此前受控/原生证据是实施中的 dirty 构建，
不将其构建身份冒充交付身份。精确交付身份以构建记录和本次交付结果为准。
