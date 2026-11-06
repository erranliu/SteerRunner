# 原开发 Run 后续消息验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-06。来源主干 `22cc3751f1d618f315ebbda27595b06a0c79a544`，分支 `codex/pr-ready-messages`，独立 worktree。本文记录隔离实现与合并前审计；后续用户授权合并/部署的实际状态见 [Runner PR 125](https://github.com/erranliu/SteerRunner/pull/125)、[SteerLine PR 464](https://github.com/erranliu/SteerLine/pull/464) 及正式部署回执。实现只扩展现有 message/history，保留开发 pr_ready 游标和原 PR/审计，不引入冲突业务知识、Workflow 或结果协议。

调用方正文原样冻结为 caller.message。沿原输入串行和可靠执行；Orca message 复用原结果记录的 fence，跳过 ShowWorktree、恢复探针和发送前完整历史核验，保留 Send 的原生受理及发送后来源核验。无新 worktree/session、业务审计或模型补报。

## 验证

- 原普通/审计开发 Run 两轮 message、同请求重放、并发拒绝、停止拒绝、旧 final 隔离和 unreported 中性结果：`TestDevelopmentRunMessagesRetainOriginalWorkflow`。
- Orca 原输入沿同会话发送与原 fence：扩展 `TestStructuredDevelopmentFollowUp/message`；原审计/未知/活动/外部输入反例保留。
- 实际命令、完整受测源码与报告见下文；未验证的真实模型/远端组合不声明通过。

初次普通沙箱测试受 Git `.git/config` 写入拒绝影响，未计通过；隔离目录下的 owner 复验与正式验证分别记录，不用旧报告替代最终源码验证。

## 实际结果

- 正式 L1：`scripts/test.ps1 -Level L1 -OutDir C:/SteerlineDebug/pr-conflict-5c27/runner-L1-01`，报告 status=passed，806.5 秒。包含内部包与框架/原生零匹配守卫；按正式入口识别的 opt-in live/helper 排除不冒充真实链通过。
- 固定 EXE 五项公开受控：`scripts/test-foundation.ps1 -RunnerExe C:/SteerlineDebug/pr-conflict-5c27/bin/steerrunner.exe -Pattern '^(TestSRRunWorkflow|TestSRContentConversation|TestSRRestartPreservesAuthorization|TestSRPublicationIdentity|TestSRWorkflowDiscoveryAndSelection)$' -OutDir C:/SteerlineDebug/pr-conflict-5c27/runner-controlled-01`，result.json exit_code=0，scenario_count=5；重启含 graceful/kill/creation_receipt/pending_stop 四个分项。
- 新消息与 Orca 发送定向：`runner-message-02.log` 通过。最后补入已受理 message 的同构建重启断言，再执行 `go test ./internal/application -run '^TestDevelopmentRunMessagesRetainOriginalWorkflow$' -count=1`，`runner-message-restart.log` 通过。
- 固定 EXE 版本 `0.2.0-dev.4`，SHA256 `49c30ed6e93171d21008392d8960242ac37a7d149fa1ecf35231646a5645fcc4`，resource_digest `6da5d3f30de3b0a9a4e7614f79e1d86b2db198232964b6d7aaafad2072d5372e`，来源为本分支基础提交加源码变更，构建回执 `C:/SteerlineDebug/pr-conflict-5c27/bin/steerrunner.exe.build.json`。
- 独立只读审核通过：检查原现场直接发送、正文来源、历史/未分类结果、审计隔离、停止/未知边界。Runner 没有新增固定提示词或业务解析。

SteerLine 正式产品的真实 PR 冲突联合验收在其对应记录登记；本记录不宣称所有原生 Agent/目标组合已经真机验证。上述验证均使用隔离资源，未在这些验证中部署安装环境。

## 刷新主干后的复验

提交 `0b09c3d8` 后按仓库约定正常合入 `a12b24dc`（Orca profile 单次解码），无内容冲突；整合提交 `9f0b0872ba6524316e1fcad83b19bcdf1af90fce`。合入后 orcatransport 包全量通过（64.1 秒，`runner-merged-adapter.log`），新 message/同会话对话定向通过（`runner-merged-message.log`，resources/contracts 该模式仅编译，不作新增断言通过声明）。

整合构建 `C:/SteerlineDebug/pr-conflict-5c27/bin/steerrunner-merged.exe`，版本 `0.2.0-dev.4`，SHA256 `0936ce56129e4522a110c67f51ad95b357f35be1de083258af502e5897f4c30b`；`.build.json` 固定来源 commit 和 source digest。SteerLine 联合真实链使用该独立文件，不覆盖此前构建或安装版。

## 联合真实验收与问答补验

SteerLine `smoke.local-codex` 正式入口：`./scripts/test-live-delivery.ps1 -SteerRunnerExe C:/SteerlineDebug/pr-conflict-5c27/bin/steerrunner-merged.exe -OutDir C:/SteerlineDebug/pr-conflict-5c27/live-06`，summary.json passed=true、cleanup_complete=true，336.5 秒。本地 codex-app-server / gpt-6-luna medium / full_access / development/2。正式产品命令两次向原 Run 投递，第一次由 Agent 解决真实文本冲突并推送更新原 PR，第二次无冲突正常结束；Run/session/worktree/分支/PR 均保持。驱动只准备自有 base 和查证结果，随后沿既有显式测试合并/交付/停止路径清理。

最后在原消息 owner 中补齐普通/审计开发两种流程的原生问题→正式 reply→原路答案一次发送→本轮结束→第二条 message，断言原 question route/turn、串行门禁与 question/answer 历史。`go test ./internal/application -run '^TestDevelopmentRunMessagesRetainOriginalWorkflow$' -count=1` 通过（0.554 秒）；差异只在测试，不改变以上固定 EXE。独立只读 reviewer 复审通过。真实冲突 Agent 未提问，实机问答不据此宣称通过。

SteerLine 最终 smoke 夹具修订后再次运行相同命令，OutDir=`C:/SteerlineDebug/pr-conflict-5c27/live-07`，316.9 秒通过且清理完成；同一固定 Runner EXE，真实 PR 779 两轮均沿原资源完成，第二轮未新增提交。仅此本地组合有真实联验声明。

## 合并前再审与修复

2026-11-06 用户授权再次审计、修复后合并部署。独立审计发现停止窗口 P2：message 已受理或已冻结但尚未提交原生输入时，stop 未清理 pending，原控制会永久 pending/running；消息内答案有同一窗口。本次仅在开发 pr_ready 的 stop 受理事务中撤销未发送/已明确拒绝的输入并标 failed，已提交的 in-flight/unknown 意图保留原身份和对账，completed 回执不降级，保留原 pr_ready 与历史。

`TestDevelopmentRunMessageStopCancelsOnlyUnsentInput` 覆盖普通/审计两流程的 saved、frozen、input-intent、input-rejected、input-accepted、answer、answer-intent、answer-rejected，完整停止后重启查询原 request；原两轮/原生问答测试保留。修复前复现 `runner-stop-audit-before.log`；最终定向 `go test ./internal/application -run '^TestDevelopmentRunMessage' -count=1`，`runner-stop-audit-final.log` 通过（2.251 秒），独立只读复审通过。

中间 `runner-stop-audit-after-01.log` 的 audited 夹具缺原 ExecutionModel，补齐后 after-02、after-03 和 final 通过。正式 L1 首轮 `runner-audit-L1-01` 在 focused 框架阶段因测试期间源码继续修订被生产输入摘要守卫拒绝（150.7 秒，尚未执行内部包）；不放宽守卫，不计通过。冻结修复后另起报告复验。

冻结修复提交 `d37cadfadc01677de505989d0f7b7f864df395bd` 后，`scripts/test.ps1 -Level L1 -OutDir C:/SteerlineDebug/pr-conflict-5c27/runner-audit-L1-02` 完整通过（885.3 秒，source_dirty=false；含框架、内部包、原生零匹配守卫）。新固定 `bin/steerrunner-audit.exe`，SHA256 `ed531ccfbdd3682a5e0c0ffaab740ebf0ab285972239eff9103e9676cf2625a8`，来源同一干净提交，构建回执记录 source_digest。

真实复验 `live-08-audit` 的独立宿主中途消失，缺最终报告，不计通过；原因未确立。公开查询确认其 Runner 停止、零 Run，清理登记的自有隔离 base 分支；事实另存该目录 `interruption.json`。不改正式测试宿主或结果口径；完整 L1 后用同一固定 EXE 串行重跑 `live-09-audit-serial`，其正式报告与本轮 PR 登记实际结论。

串行复验正式命令：`./scripts/test-live-delivery.ps1 -SteerRunnerExe C:/SteerlineDebug/pr-conflict-5c27/bin/steerrunner-audit.exe -OutDir C:/SteerlineDebug/pr-conflict-5c27/live-09-audit-serial`（在 SteerLine 仓库）。summary.json passed=true、cleanup_complete=true，393.6 秒；同一原 Run `b8b837b1c8b46f889151ae1af89bc482`、session/worktree、PR 780，两轮均保持。首轮 head `db8c9f29b7fc943c0000d536b46541cea65fa6c3`→`9e403ded9f119616623489908ee111f799952049`，二轮保持该 head，消息结束时 PR 均未合并；随后原驱动显式测试合并、交付和停止均通过。验证范围仍是本地 Codex 组合，未扩大实机资格声明。
