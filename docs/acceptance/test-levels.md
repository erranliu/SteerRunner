# L1–L4 测试功能验收

日期：2026-10-16。实现包含十五项受控前置、L3 独立反馈/两模型审计/同服务多 Run、
有界并行与跨批次槽锁、四项 L4 受控边界，以及 L2 精确构建/配置的反馈资格门。
具体定义见[测试指南](../testing.md)，逐项内容见[测试清单](../test-catalog.md)。

本轮结果已归档到[证据目录](evidence/test-levels-20261016/)。验收使用工作树构建，
基线 commit 是 `c976cc4b5781c2ca65a6a85a47aed3ec44d305dd`，不是本次实现的干净提交；
以每批 Runner/driver 摘要及最终源码清单绑定实际受测内容。
构建对照见[build-comparison.json](evidence/test-levels-20261016/build-comparison.json)，
源文件摘要见[source-hashes.json](evidence/test-levels-20261016/source-hashes.json)。

| 项目 | 结果与范围 | 证据 |
| --- | --- | --- |
| L1 | 最终一轮 207 个顶层测试通过；live driver 编译与零匹配 guard 通过 | [l1-summary.json](evidence/test-levels-20261016/l1-summary.json) |
| Controlled | 扩充后的 15 个顶层测试通过，包含审计修复/重审/预算/问题恢复和多 Run 隔离；子变体在日志单列 | [controlled-result.json](evidence/test-levels-20261016/controlled-result.json)；本机/L4 批次也分别执行 |
| 调度器 | 两个 L3 worker 重叠、L4 独占、失联 worker/构建变化/跨 case Run 身份碰撞拒绝；使用独立锁命名空间，避免 L1 框架检查等待真实模型批次 | [scheduler-verification.json](evidence/test-levels-20261016/scheduler-verification.json) |
| L4 | 四个场景全部通过，4/4；原证据永久缺失、停止回执提交故障、接受事务回滚、有界资源增长分别判定 | [l4-summary.json](evidence/test-levels-20261016/l4-summary.json)、[case 记录与日志](evidence/test-levels-20261016/l4-cases/) |
| 本机 L3 | 三个新增 case 分批通过：多 Run 在第二批通过，修正后的反馈/两模型审计第三批 2/2；第三批两个 case 的执行时间重叠、Runner/driver 摘要相同；不声称一次全 L3 通过 | [本机第三批汇总](evidence/test-levels-20261016/local-summary.json)、[各 case 记录](evidence/test-levels-20261016/local-cases/)、[多 Run 原始结果](evidence/test-levels-20261016/multirun-native/scenario.json) |
| L2 core | 正式 1/1 通过，前置 L1 207、框架 16 组、Controlled 15 全通过；冻结第三批反馈资格，实际选择 `development-core`，无反馈轮、真实合入及完整停止 | [core-summary.json](evidence/test-levels-20261016/core-summary.json)、[case](evidence/test-levels-20261016/core-case.json)、[资格绑定](evidence/test-levels-20261016/core-native/qualification-binding.json) |
| 远端 L3 | 本次未重新验收；当前可信认证准备/同沙箱工具检查的生产能力缺口保持 | [沙箱认证记录](sandbox-authentication.md)；不将旧 36/36 计作本版本通过 |

L4 资源检查固定为同一服务 20 轮，首轮后允许句柄增长最多 128；每轮确认原生进程
消失，实际 thread=20、turn=60。它不证明物理磁盘损坏、长期内存稳定或整机资源耗尽。
L4 的故障均限受控夹具/测试数据库，未修改真实 Codex 历史或系统网络/存储配置。

PR 整合后的结果见文末；下列初次真机证据不自动扩展到整合后版本。

## 配置与失败记录

经用户要求，从 `C:/SteerlineInstall/execution.json` 的有效本机/远端 profile 提取非秘密
运行字段，保存为本工作树 `.evidence/testing-implementation/live-config.imported.json`。
执行模型 `gpt-6-luna`、effort high；审计模型由用户指定为 `gpt-6.1-sol`、effort high。
Codex 实际版本 `0.159.2`。远端启动中的沙箱 bypass 参数未复制；没有复制认证秘密。
来源及字段范围见[config-import-source.json](evidence/test-levels-20261016/config-import-source.json)。

首批本机正式入口的 L1/Controlled 通过，但三个 case 在 clone 阶段失败，未启动模型。
保留[原失败汇总](evidence/test-levels-20261016/local-preparation-failed.json)。直接 clone 与
现场生成随后成功；实现为 worker 增加独立 stdin/stdout/stderr、可读取日志及 Git 错误正文，
第二批已越过准备阶段。不从缺失的首批 stderr 推断唯一根因，也不改写失败为通过。

压力诊断还发现旧停止辅助函数对多个 Run 复用同一 request_id，现已把 Run ID 加入
请求身份；初次诊断失败保留，正式 L4 使用修正后的版本四项全通过。

第二批三个 native driver 的真实断言与资源收尾均通过，其中反馈 PR #335 已合入，
两模型审计 PR #337 和多 Run PR #338/#339 按契约保留未合入。外层 PowerShell 对审计
结果直接读取被 `omitempty` 省略的 `merge_commit`，导致审计 case 校验失败；原批次仍为
failed，2/3，不补写通过。已拆出纯结果校验器并增加可选字段、零执行、错误身份、布尔
类型、反馈/合入缺失、多 Run 身份碰撞反例；对旧原始结果的只读复核通过不改变旧批次。
第三批用新现场重跑反馈及审计，正式 2/2 通过，PR #340 已合入、#341 保留未合入，均停止
Run/服务并撤销 capability。多 Run 保留第二批独立通过证据，不声称同批 3/3。

最终框架回归共 16 组，通过单 Run 的 session/workspace/branch 身份登记反例、跨 case
重复 Run/session/workspace 拒绝，以及 v2 示例新增审计字段的兼容性检查。对第二批三项
保留现场执行[只读资源复核](evidence/test-levels-20261016/native-resource-recheck.json)，
确认真实 Run/session/workspace/branch/PR 均不同；它不改变原失败汇总。
第三批反馈/审计与已通过的多 Run 也完成[最终本机身份复核](evidence/test-levels-20261016/local-qualified-resource-recheck.json)。

两模型还从原生 history 核验实际 `turn_context`：Spec/代码审计为 `gpt-6.1-sol/high`，
最终发布为 `gpt-6-luna/high`，同 session/workspace，按原审计 turn_id 和最终执行 turn_id
精确绑定，见[原生模型复核](evidence/test-levels-20261016/audited-native/audit-models-recheck.json)。
自动入口现在保存 `audit-models.json`，框架反例拒绝错误 session、错误审计模型或未切回执行
模型；本批启动时外层入口尚未加入这一调用，因此额外复核独立登记，不改写原批次记录。

L2 正式批次使用第三批通过的反馈 case 资格，Runner/driver 与三个本机 L3 case 一致。
现场 `C:/srt/f0db28b15b91` 的 driver 为 `development-core`，结果不含 `feedback_head`；
PR #342 已精确绑定 head 合入，merge `552b45f29ce3927d08819784e22aacacb29067ff`。
Run、原生资源/capability 和服务收尾均确认，最终 case 还登记 workspace/session/branch
身份。配置中的 `feedback_qualification` 已指向本轮正式反馈 case；后续构建或执行配置变化
时须重新取得资格，不能复用本轮记录删除反馈覆盖。

## PR 主线整合回归（2026-10-16）

为提交 PR 整合 `origin/main` 的 `ea1bfd9`，保留停止结算修复和独立环境检查能力。
测试 registry 现为十六场景、二十二 case：L2 1、L3 17、L4 4；Controlled 默认 16 项，
包括原三项新增受控覆盖和主线的 `TestEnvironmentCheckCLI`。原 11 个核心可靠性 case
仍在 L3。两种 Check 场景沿用原完整判据、ValidateOnly 及资源身份登记，不创建 development Run。

- `scripts/test.ps1 -Level Controlled`：L1 224 个顶层测试、框架 16 组、Controlled 16 项全部通过；live driver 编译和零匹配保护通过。[前置汇总](evidence/test-levels-20261016/pr-main-integration/prerequisite-summary.json)、[框架](evidence/test-levels-20261016/pr-main-integration/framework-result.json)、[受控结果](evidence/test-levels-20261016/pr-main-integration/controlled-result.json)。
- 前置通过后，持有全宿主槽独占锁，通过 `test-foundation.ps1` 精确选择四项 `TestSREdge*` 做定向回归，4/4、零 skip；未重跑相同前置。[结果](evidence/test-levels-20261016/pr-main-integration/l4-result.json)、[原始日志](evidence/test-levels-20261016/pr-main-integration/l4-integration.log)、[构建身份](evidence/test-levels-20261016/pr-main-integration/l4-source-identity.json)。资源项 20/20 轮完成，handles 223→272，增长 49≤128。
- 主线的 sandbox/full_access 两份原生环境检查报告均通过整合后结果校验器和资源身份的只读复核；新增反例拒绝模式不符、PR 未关闭和原生 evidence 身份错误。两种模式的注册数量保护也单独复验通过。这些不是新一轮模型运行。

整合后的受测源码清单见[摘要](evidence/test-levels-20261016/pr-main-integration/source-hashes.json)。
最初批次是在 merge 前 dirty 工作树启动，原 summary 中的来源字段保留；merge 后代码纳入
`8e5e128`，后补的环境模式数量保护有独立复验，最终以源码清单和各实际 EXE/driver 摘要
区分受测范围。没有改写初次真机、历史失败或主线环境检查的原报告。

本次 PR 整合后未重跑模型/真实 PR，不能复用旧 Runner/driver/执行配置的反馈资格；L2 和
本机 L3 的整合后真机资格仍须新现场取得，远端 L3 仍受认证/同权限预检能力缺口阻断。
