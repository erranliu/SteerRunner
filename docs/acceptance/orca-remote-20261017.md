# Orca 远端权限、开发闭环与 L3 验收

日期：2026-10-17（上海时间）。本轮实现并实测执行主机 gh 认证路径下的 Orca
sandbox / full_access 普通开发闭环，以及原会话上的恢复、停止和发布对账。

## 结果与构建归属

| 范围 | 完整首轮 | 同构建复测 |
| --- | --- | --- |
| L1 | 249 项通过；登记的显式 native/helper skip 不计真机通过 | 无 |
| Controlled | 16/16 通过 | 无 |
| L3 Orca 远端 | **15/15 通过** | 无 |
| L3 本机 | 4/5 通过 | local-codex.multirun 1/1 通过 |
| L3 合计 | **19/20，首次通过率 95%，无排除** | 唯一失败项复测通过；全部 20 个 case 均有通过记录 |

验收状态为 passed_with_same_build_recheck。首轮从 01:26:05 到 02:26:05，
并行度 2；定向复测从 02:26:28 到 02:31:55，使用新隔离现场。
首轮多 Run 的失败发生在 driver 独立验证 PR 的只读 Git fetch，
原错误为 OpenSSL SSL_ERROR_SYSCALL；原失败现场和结果完整保留。
复测没有调整源码、模型、权限、配置或 case 预算，Runner 与 driver 的 SHA256 均一致。

原始首轮和复测分别导出为
[first-batch.json](evidence/orca-remote-20261017/first-batch.json)、
[targeted-recheck.json](evidence/orca-remote-20261017/targeted-recheck.json)；
汇总归属与逐 case 来源见
[qualification.json](evidence/orca-remote-20261017/qualification.json)。

- 源：072688f145c8917bceb08223aa019b9560d366f6 + 当前工作树；
  [1006 个冻结文件](evidence/orca-remote-20261017/source-files.json)逐项复核未变化。
- Runner SHA256：bf05b0b4ba131174f25094c5fad884a7376e0dd6ce53dcefc2947ca242a47a82。
- driver SHA256：3f16a011ec69a412e921999d955761d7a6ea3150d3ff4cd397796867948c925c。
- 远端 helper：C:/sr-runtime/2ba0/24/steerrunner-agent.exe，
  SHA256 60035e89d301065f4acdba728f7f00d22499a9358f3d408ce7fa64f793a0ff7c。
- 固定 Codex 0.159.2；远端 native SHA256
  52f75c649bebb8001102a1dd129c1ea6d02b0940321e6d7e82ee0526753bd58a。
- 执行模型 gpt-6-luna / high；本机审计模型 gpt-6.1-sol / high。

## 远端完整开发闭环

| 模式 | 实际 PR | 反馈前 head → 反馈后 head | 精确合入 |
| --- | --- | --- | --- |
| sandbox | [#410](https://github.com/erranliu/SteerlineTest/pull/410) | b7fb71a577f9 → fd205d0cda84 | 826c42ea16194cd5dbab9466a338b99a9e9e74ff |
| full_access | [#408](https://github.com/erranliu/SteerlineTest/pull/408) | ebff8e236977f → e93a64bacceb | 91d6275c15a312ed914a350b9650320f3b3ffae3 |

两例均完成实际实现、checker、commit、push、创建/复用原 PR、同一会话反馈、
新提交、精确 head 合入、Run 停止和服务退出。合入目标为独立测试 base 分支。
每例 6 个业务回合均以原 Codex session 的完整实际输入摘要、native turn/user item
和 final_answer 原始条目核实，6 个最终查询终端关闭事实均确认。

证据：
[sandbox case](evidence/orca-remote-20261017/orca-remote.smoke-default/case.json) /
[native 回合事实](evidence/orca-remote-20261017/orca-remote.smoke-default/native-turn-facts.json)，
[full_access case](evidence/orca-remote-20261017/orca-remote.full-access-default/case.json) /
[native 回合事实](evidence/orca-remote-20261017/orca-remote.full-access-default/native-turn-facts.json)。

两种远端环境检查各有 12/12 实际工具项通过，均完成临时分支、PR、writer 与检查现场结算：
[sandbox](evidence/orca-remote-20261017/environment-check.orca-sandbox-complete/check-report.json) /
[full_access](evidence/orca-remote-20261017/environment-check.orca-full-access-complete/check-report.json)。
检查报告绑定实际执行主机、runtime、terminal/incarnation、native PID/创建时间、权限模式和原生会话。
确定性 command/exec 检查与业务回合分开记证；TUI 的工具禁用 bootstrap 不计业务回合。

## 本轮修复

1. 远端可信 helper 在执行主机解析 gh 登录，使用有限期、单仓库代理，
   并在实际模式及原 app-server 上完成确定性准备。配置显式绑定 helper 路径与 SHA256。
2. Orca 路径贯通 access_mode、环境检查和执行主机身份，缺能力、摘要/身份/期限不符均阻断。
3. 原生历史回执处理短回合启动事件漏报。查询资源创建意图、返回身份和关闭结果持久记录在调用端；
   优雅关闭等待清理，强杀恢复只清理原资源，未知创建禁止补建。业务停止与查询清理分别保存进度。
4. 工作区创建使用后台方式。已确认的新工作区缺少初始终端时，
   完整空清单与持久创建意图限定一次首次创建；恢复按原身份核对。
5. Git 指引要求精确暂存、核对 staged diff、顺序提交并立即保存 commit 自身退出码。
   Git 2.55 的精确 packed-refs 清理警告仍按原严格条件区分，Git 写根与拒写边界保持。
6. 反馈 driver 在 pr_ready 仍持旧 head 时调用现有只读 refresh-pr，
   并验证原 Run、PR、仓库/分支、Attempt、Session 和工作区不变。
   新 head、服务端 Git 对象、产物时间/checker、精确合入及原 12 分钟预算全部保留。

四种服务重启、调用方退出、断连恢复、三种停止/撤销和两种发布未知结果测试均在本轮远端 15 项内。
缺口、故障命中、零业务重发、原身份及资源清理仍由各 case 独立断言。

## 并行与清理

调度器支持 MaxParallel 1–8，本轮实际并行度为 2。各 case 的服务、state-dir、
工作区、分支、PR 与故障设施独立；断连 case 命中网络阻断期间，另一闭环仍推进发布。

[远端资源复核](evidence/orca-remote-20261017/remote-resources.json)覆盖 15 个 case：
开发工作区终端均为 0；主工作区中的环境检查仅核对本次 actor 已不存在。
[21 个调用端现场服务](evidence/orca-remote-20261017/caller-services.json)均为 not_running。
[故障设施复核](evidence/orca-remote-20261017/network-facilities.json)确认本任务 22 组防火墙规则为 0、
22 个恢复 watchdog 退出、管理员网络宿主退出且队列无待处理请求。
[任务诊断终端](evidence/orca-remote-20261017/diagnostic-terminals.json)已关闭，
[暂存 worktree](evidence/orca-remote-20261017/staging-cleanup.json)已移除。
33 个公共暂存文件归档至 C:/sr-runtime/2ba0/staging-archive-20261017；
固定 native 和 helper 留在任务命名空间供复验。测试 PR/现场按各测试契约保留。

## 覆盖边界

- 远端普通开发闭环、反馈与上述可靠性路径已实测；远端双模型审计、同一服务多 Run 尚无 L3 driver。
  本机对应测试的通过单独归属。
- 本轮远端认证来源为执行主机已有 gh 登录；远端可选 GitHub App settings 尚未接线。
- 初始终端创建、未知资源、身份/摘要/期限、关闭丢回执等确定性分支有组件反例；
  真机 L3 的通过范围按注册 case 及实际断言界定。
- 开发期间的失败/中止批次保留。原生完整日志、模型正文、终端内容、数据库及凭据不进入本公开证据导出；
  含模型正文的原始 completion proof 仅保留现场路径与 SHA256 引用。
