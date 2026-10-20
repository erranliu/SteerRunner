# conversation/1、generate/1 专属验收

2026-10-20 的同构建 takeover 冻结配置问题单独记录在[内容接管修复验收](content-takeover.md)。
## 2026-10-20 生成对象接管缺口修复

用户授权继续自动补验。基线 `c122d641677593d4b694e2eda7502001510d4e8b` 加本轮未提交修复；没有部署安装环境。生产 freeze 对内容流程不加载开发 parser 配置，接管却无条件校验该空配置；内容单元夹具误填默认 parser 配置，掩盖了真实问题。修正夹具后 generate/conversation 及真实 admission 两项在原代码上均复现 unsupported；修复仅将该校验限制到使用它的开发流程，保留材料/执行语义/codec/原生身份与原 Record 字节校验。

证据根目录 `C:/Users/lmas1/AppData/Local/Temp/steerline-auto-closeout-20261020-131948/`：

- `runner-before.jsonl`、`runner-regression-before.jsonl` 保留修复前真实失败；`runner-after.jsonl` 中四包定向测试通过，包含 queued/result 两状态、实际 admission、材料/codec 反例与开发配置拒绝；最后将开发空配置和未知 schema 反例独立初始化，`runner-development-guards-final.jsonl` 复验通过。
- `runner-controlled/result.json`：exit_code=0、scenario_count=3；Generate / Conversation / QuestionAndUnknown 三个实际 EXE/IPC/SQLite 受控场景通过，详情 `integration.log`。没有运行全量 Controlled 或原生矩阵。
- 同源码构建 A/B 到该临时目录，version=`0.2.0-content-takeover-a/b`，SHA 分别 `066a182f452ba0e80e51f5091bb5ab6baf31a6cf9085ed7e4cf7ff520234346a` / `ea8ad087931f55fed5da086503b76d01944fe3cbc88c191caae3e481bf901d6d`，source_dirty=true；build.json 和 `runner-final-source.patch` 保留实际来源。只读核对原 SteerLine k 包，原生成/开发两个 Run 现在均 compatible，见 `tm-k-takeover-fixed.json`。
- 原 SteerLine i/j 两失败包已通过 B 接管预检；在 service start 的 pending_stops 中先受理各自原开发 Run 的唯一持久停止请求，随后仅查询原请求，均核实 stop_status=stopped。新服务沿同一停止 ID 退出，公开 verify-stopped 和精确 last_shutdown 均确认 resources_settled=true；原失败 caller shutdown/proof 文件不改写，专属 base 在核验原 commit/lease、无 PR 后删除。两个 `dev-20261020-11*/summary.json`、逐步公开回执及清理日志保留。恢复完成不将原失败测试改记通过。

最终跨构建真机生成子范围通过：SteerLine 新包 `C:/SteerlineDebug/dev-20261020-133842-5612f97d-dirty`，A 生成原 Run `784ed608d78a7e5784c98685e52c7a70` 后经正式预检/verify-restart，由 B 接管，原 request/Run/input/final/content_result 和 saved API 保持；在 B 上原开发 Run `5334c89977c5109cfbee470bf60e9c86` 完成真实程序通过/失败→原报告/API/两轮浏览器，再正式停止，两个原 Run 全部 resources_settled=true。同一包的 base/head 在无 PR、原 commit/lease 核对后均清理。根证据 `qualification.json` passed=true；包内 `evidence/test-management/generation-takeover.json`、`generation-after-takeover.json`、`proof.json`、`verify-stop.log` 和 `evidence/test-management-owned-cleanup/` 保存实况。没有新 generate、替代 Run 或改写原输入；SteerLine 的副作用与 Task 事实由其专属验收维护。

补验中驱动的参数环境丢失、编译变量名冲突、过早检查 ready 和 Windows 回执文件写入失败均分别留失败记录；修正后使用新包。已接受控制仅查询原 ID，零重发。主场景已通过后，收尾发现“无 PR 但已推送 head”的遗漏，补清理并以独立同包后验核实，不改历史 proof。验证脚本的 PowerShell 5.1 编码错误只修只读 UTF-8 解析后复验，不重跑业务。当前两仓修改未提交、未部署。本结果只覆盖本次生成对象及消费者闭环，conversation 的其他原生恢复组合、Orca 和权限/停止 scope 不扩大；本轮与历史 2026-10-19 结果分别记录。

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
