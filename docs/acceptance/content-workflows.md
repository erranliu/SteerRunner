# conversation/1、generate/1 专属验收

日期：2026-10-19。本次要求的专项验收通过。以下分别记录受控、单元和真实 Codex 证据，
不沿用历史开发流程的真机资格。本次未修改 SteerLine、未部署或安装环境、未加入 roadmap。

| ID | 必须验证 | 状态/证据 |
| --- | --- | --- |
| CW01 | 新输入、冻结材料、开发必填及权限边界 | 通过；contracts/resources 和 `TestContentMaterialSnapshot`；[规则](evidence/content-workflows-20261019/rules.log)、[内容单元](evidence/content-workflows-20261019/content-tests.log) |
| CW02 | 文本/JSON 成功、格式失败、原生失败/取消/缺 final、零第二轮 | 受控通过；生成 10 个分支，含缺认证/目录拒绝且零业务轮次；[受控日志](evidence/content-workflows-20261019/controlled-integration.log) |
| CW03 | 两轮同 thread、完成后等待 | 受控与真实通过；[真实摘要](evidence/content-workflows-20261019/native-summary.json) |
| CW04 | 原生问答、重复/过期/并发及未知输入不串线 | 单元/受控通过；原问题回答、幂等重放、活动/并发输入拒绝、未知发送及未知中断不重发 |
| CW05 | interrupt 可继续，close/stop 永久拒绝输入 | 受控与真实通过；[中断](evidence/content-workflows-20261019/native-interrupted.json)、[关闭](evidence/content-workflows-20261019/native-closed.json) |
| CW06 | 重启/未知发送不重发、原身份恢复 | 受控与真实通过；未派发的旧服务消息标为失败，不自动续发；[重启快照](evidence/content-workflows-20261019/native-restarted.json)、[继续](evidence/content-workflows-20261019/native-after-restart.json) |
| CW07 | 结果归属、开发隔离、结果与资源结算独立 | 单元/受控通过；外来 final 拒绝、未知 outcome 阻断；`TestContentCleanupFailureIndependent` 保留成功结果而资源 unknown；[开发回归](evidence/content-workflows-20261019/development-regression.log) |
| CW08 | 真实本机 Codex 结构化生成 | 通过；单个 start_turn、JSON 程序校验成功、资源 stopped；[生成](evidence/content-workflows-20261019/native-generation.json) |
| CW09 | 真实多轮、轮次中断后继续 | 通过；原 thread 内五个轮次，包括一次 cancelled 后继续；[真实日志](evidence/content-workflows-20261019/native-integration.log) |
| CW10 | 真实空闲会话服务重启后同 thread 继续 | 通过；重启前后历史不变，resume 后新输入仍为原 thread；[最终历史](evidence/content-workflows-20261019/native-history-final.json) |

## 职责与复用

新流程只定义 execute → closed 或 execute → input；唯一业务游标仍是 Workflow.StepID，
结果、意图和游标在原 Record/Commit 一次提交。application 接线内容准入、材料快照、历史和控制；
contracts/resources 发布互斥输入及可用操作；workflow 提供薄定义；reliable.Next、operation、
原生问答路由、NormalizeResult、VerifyManualRecovery/BindRecovery 和 owned Job 结算继续复用。
Codex 原权限/初始化 owner 增加 read_only、原生 account/read 与无模型目录探针，
不调用开发 Git/发布检查，不依赖 parser 配置，不新建 Agent 运行时。

Runner 保存执行/输入/结果事实；SteerLine 构造指令、保存业务关联、校验业务内容并保存正式文档。
两个新流程都不进入报告补报、审计、PR 或 TaskDelivery。原 development 准入、恢复和 stop 语义保留。
工作区原有 Orca 改动保留，本次未修改 Orca 实现。

## 实际命令、环境与结果

交付构建专项入口（先规则，再 3 个受控场景，再真实 driver）：

```powershell
./scripts/test-content-workflows.ps1 `
  -NativeExecutable C:/Users/lmas1/.local/bin/node_modules/@openai/codex/node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe `
  -Model gpt-6.1-sol -OutDir .evidence/content-final-receipts
```

Windows/amd64、Go 1.26.4、真实 Codex 0.159.2、gpt-6.1-sol/low。
本次新隔离现场 `C:/srt/cw-1402274103`，不含 Git 或 PR；没有使用 SteerLine 安装环境。
生成 Run `daf3b335be365ec417ebf546da3ac237`；对话 Run `bf21091e74bcdb3fe3683d0e6e68d3c0`。
对话原生 thread 从初始轮、轮次中断、后续轮到服务重启后始终为
`01a1538d-b077-7861-8608-a546694a3c53`，以原会话记忆 token 复验上下文。
显式关闭后新消息拒绝；最终服务已退出，公开
[verify-stopped](evidence/content-workflows-20261019/verify-stopped.json) 再次核实全部 Run 资源结算。

相关 owner 包完整回归通过：contracts、resources、cli、storage、workflow、reliable/operation、
codex adapter、resultacquisition、workspace，见[日志](evidence/content-workflows-20261019/owner-tests.log)。
开发 CLI 定向回归包含 RunWorkflow、NativeUnknownAndStop、RestartRequiresResume，与三个内容场景
共同通过 6 个顶层场景。application 额外整包 8 分钟和 core 分组 6 分钟触发聚合预算，
不是单例断言失败；保留[原超时](evidence/content-workflows-20261019/application-aggregate-timeout.log)与
[core 已完成用例](evidence/content-workflows-20261019/application-core-partial.log)。
随后耗时 [Git 子集](evidence/content-workflows-20261019/application-git.log)、
[余下 17 项](evidence/content-workflows-20261019/application-tail.log)及最终内容单元均通过。
不把有超时的聚合运行登记成一次完整 package 成功。

初次迭代的 schema 重复 `$id`、session_closed 退出码映射，以及新测试夹具缺失 import 已修复；
最终受测构建的规则、受控、原生均通过。未运行全量 L1–L4；本次真实验收未特意触发人工问答、
坏 JSON 或断网反例，这些由受控/适配器测试覆盖，不宣称其全部真机资格。

## SteerLine 接入与构建

公开命令仍为 v1 信封 submit/inspect/events/reply/resume/stop；新增 message、interrupt、close、history。
输入/控制及示例：

- [generate 最小请求](evidence/content-workflows-20261019/generate.request.json) / [输入 schema](evidence/content-workflows-20261019/generate_input.schema.json)
- [conversation 最小请求](evidence/content-workflows-20261019/conversation.request.json) / [输入 schema](evidence/content-workflows-20261019/conversation_input.schema.json)
- [message schema](evidence/content-workflows-20261019/message_input.schema.json) / [revision 控制 schema](evidence/content-workflows-20261019/revision_input.schema.json)
- [实际离线 capabilities](evidence/content-workflows-20261019/capabilities.json) / [构建 manifest](evidence/content-workflows-20261019/build-manifest.json)

两个请求示例均经该 EXE 的 `validate --kind run` 通过；示例中的 native EXE 和已有目录需由调用方填写，
纯校验不代表环境就绪。运行步骤和查询语义见[generate](../workflows/generate.md)与
[conversation](../workflows/conversation.md)。材料首版只收 inline；schema 固定 2020-12、本资源 fragment 引用。
read_only 禁止写入，读取范围仍为原生只读策略，工作目录绑定不是单目录读取隔离承诺。

Runner `0.2.0-dev.2`，源码 `c65b1936a637e19368039ad09e125e4e8f6d96c1-dirty`。
交付 EXE：`C:/Project/SteerRunner/.evidence/content-build-current/steerrunner.exe`。
重新从当前生产源码构建，SHA256 与真实受测 EXE 完全一致：
`215d7fb619916a4a3cebded4995d82052127ab0fd67f214ce053bf8ee5fd3643`。
资源摘要：`c03261da8704a0e459b69445844baa42fab0e3bab2690f6ce65cc807ccdedd4f`。
原生 EXE 摘要及能力来源在 manifest。只构建本仓隔离产物，未复制到安装目录。

## PR 合并后复验（2026-10-19）

在隔离 worktree 合入 `origin/main` 的 `7d830e7`，生产源码提交为
`46d66d7645f653277dd4b8617a5f632139ea3826`。上文保留首次验收历史；本次接入应使用
[PR 复验登记](evidence/content-workflows-pr-20261019/README.md)、
[capabilities](evidence/content-workflows-pr-20261019/capabilities.json)及其同目录 schema。
新版移除了 main 已退役的认证字段声明，仍拒绝该字段，包括 null。

Runner `0.2.0-dev.2`，EXE SHA256：
`5892bd49ebc37500486924f4f8d0f8e087a0aa4b791bdc1c6cf454c247e4b000`。
资源摘要：`536edcaf2ed199109a1b42875f61fbb27af048a6441c5494d8afe36f4ca1bf56`。
真实 Codex 再次通过结构化生成、同会话多轮、中断继续及重启继续（63.99 秒）；
原生会话 `01a153ca-73ef-7921-94b3-87f62c988691`。公开 verify-stopped 确认两个 Run 资源均已结算。
受控场景与开发回归使用同一摘要的 EXE；未部署安装环境。验证范围和未验证项沿用上述限制。
