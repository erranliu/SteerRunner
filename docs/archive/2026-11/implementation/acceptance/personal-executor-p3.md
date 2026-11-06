# 个人执行器简化 P3：恢复/兼容删除与人工对账

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-01。依据[修订 Spec](../spec/personal-executor-simplification.spec.md)及 [Plan P3](../spec/personal-executor-simplification.plan.md#5-p3删除恢复兼容并补齐人工出口)。P3 按 PES-A03/A04/A05/A08 独立验收通过。P4 未推进，未安装、部署或发布。

调查提交为 `e2729563562314659a799a5f82debe427fcd7c24`，交付为该提交加现有 P1/P2 与本批未提交变更，source_dirty=true。P3 的删除收益相对开始实施前、已经独立验收的 P2 工作副本计算，不把 P1/P2 的删除计入 P3。

## 实施与边界

- 启用 state/v7、SQLite user_version=7，默认 `%LOCALAPPDATA%/SteerRunner-v7`。v6 及更早格式在 worker/owner/远端调用前非破坏拒绝；无迁移器、旧 Run 只读投影、旧执行器选择或逐 Run SHA 构建门禁。CLI 外层及 Workflow 版本保持。升级前用原版本排空全部原生资源和未完成操作/控制，pr_ready 不等于排空；未来契约兼容且已排空时可继续查询原目录历史。
- 删除 `restoreOriginalAttempt`、`BindRecovery`、`SessionRebindable`、恢复 Attempt 历史及其 usage 聚合。资源丢失不新建 owner/session/终端；保留正常首次准备、独立 owner、同构建原资源重启、原创建回执和离线 final。
- 未知创建/业务输入沿用原准备期限；没有共享期限时最多 30 秒。耗尽转 `blocked/manual_intervention_required`、ResumeRequired=true。accepted/ResourcePending 使用原准备预算，活动长回合及停止证明保持各自原契约。已证明尚未跨过创建边界的首次准备队列仍可沿原意图/原预算重试；人工 resume 不获得该权限。
- 显式 resume 保留 request_id/expected_revision，最多 30 秒只读查询原操作/观察；期限保存在现有人工暂停记录，数据库故障或服务重启不续期。查询释放 actor/dispatch 锁；保存原事实后再核对当前绑定、stop/close、材料和原资源资格。成功只解除技术暂停，活动回合只接回观察，业务等待/阻断保持；仍未知、过期、冲突或资源丢失记 failed。新 ID/当前 revision 可再查；同 ID 重放结果，不重发原输入。
- 删除 CLI/host/application/storage 的 `service verify-takeover --candidate`、候选 EXE 执行和私有兼容矩阵。旧命令普通 exit 2，不隐式启动服务。删除退役开发权限分类、旧停止适配器、v5/旧空 workspace 检查点读取、旧 parser 识别及无消费者元数据。保留当前内容 read_only、原资源停止/签名/checkpoint/model bridge 和日志访问租约。
- Linked-worktree 身份改为直接路径；删除仅充当路径载体的 `--add-dir` 编解码，保留 worktree/common-dir 和私有仓库一致性检查。当前 Orca profile 解码保留在 profile_decode.go，未搬迁 Next/QualifyAction 或共享准备逻辑，未推进 P4。
- 更新本仓 CLI/capabilities、PowerShell helper 默认目录、两份 bundled skill 和当前指南。外部 SteerLine 需更新默认目录、去掉接管/旧矩阵依赖并展示人工对账及原控制结果；本批未改动或联验外仓。

## 收益核对

| 指标 | P2 基线 → P3 |
| --- | --- |
| 退役规则族 | 资源重建、候选构建兼容、退役开发权限执行兼容，3 类实际删除 |
| 生产 Go 文件 | 344 → 337，净减少 7 个 |
| 生产实现行数 | 43,970 → 43,412 |
| 本批生产增删 | 新增 465、删除 1,023，净减少 558 行；测试/文档另列，当前 decoder 搬迁不计净收益 |
| 退役入口 | restoreOriginalAttempt、BindRecovery、SessionRebindable、TakeoverFacts、VerifyTakeover、RecoverySupport、retiredDevelopment、LegacyRemoteProfile 和两个旧 stop 构造器，生产引用均归零 |
| 未知操作自动等待 | 原窗口耗尽后不重开；人工控制也不因重启/写入失败续期 |
| 人工对账副作用 | 验收要求原 session/Attempt/操作不变、原输入零重发、零资源重建，未核实不误授权 |

这批收益是减少恢复/兼容规则及跨层联动，并提供可达的人工出口；不宣称完整 L1 整体提速。完整 Controlled 新增一个公开人工对账场景，其实际成本单独记录。P1 的日常定向验证入口保持。

## 冻结构建与验收

最终 Runner SHA256：`8e45be489227e8619d0d2732db1f633a87f9db61eef8488a4e74de793420f552`；构建 source_digest：`d853b20010c8bb9201156015a547ed6b9c205ed9de5e1a4b4eb73b5079c491a1`。完整 L1、Controlled 及三条真实链路均已通过。最后只修订 orca_agent_test.go 和 resource_test.go 中两个已退役行为断言；重建后 EXE 逐字节一致，故本机和 Orca local 实际执行的仍为最终生产 EXE。两份实际构建来源摘要分别保留，不改写旧记录；远端使用更新后的来源记录。

人工出口的定向组件检查已覆盖仍未知/新请求再查、资源丢失、幂等重放、数据库提交失败后重载、过期控制重载不再查询、stop 竞争及业务等待保持。公开 CLI 场景使用独立原 owner：原输入已接受、响应暂不可见，服务退出后无 observer，自动超时转人工；一次 resume 超时失败，原响应可见后新请求对账成功，原 session/输入保持，唯一下一条消息仅派发一次。本轮完整 Controlled 已重新执行并通过该场景，耗时 64.73 秒。

完整验收按两个独立默认阶段执行：`scripts/test-prerequisites.ps1 -Stage L1 -OutDir .evidence/personal-executor-p3/acceptance-06-l1`；`scripts/test-prerequisites.ps1 -Stage Controlled -OutDir .evidence/personal-executor-p3/acceptance-06-controlled`。二者均要求当前来源指纹稳定且完整，不拼接失败轮次。真实最小接线使用同一冻结构建及新 v7 fixture 的 local-codex.smoke、orca-local.smoke、orca-remote.smoke；Focused 不冒充完整 L2/L3 资格。

| 最终生产 EXE 的真实最小链路 | 原生驱动耗时 | 新 v7 现场 / Run | 专用测试 PR |
| --- | --- | --- | --- |
| local-codex.smoke | 126.70 秒 | C:/srt/aa9e0705b6df / 67c0dd99fd42fbcb9ed2ecc89d3edece | [745](https://github.com/erranliu/SteerlineTest/pull/745) |
| orca-local.smoke | 255.54 秒 | C:/srt/8db8393be9b7 / 780595e6d15d92802c7424d2f49db0fc | [746](https://github.com/erranliu/SteerlineTest/pull/746) |
| orca-remote.smoke | 111.33 秒 | C:/srt/d71611fa0a36 / f068d14abccb4786d17eeb955c65e3f6 | [747](https://github.com/erranliu/SteerlineTest/pull/747) |

三个现场均只读核实 metadata=steerrunner-state/v7、user_version=7，公开 cleanup-run 的 stop_status=stopped；驱动完整停止/结算断言通过。原生驱动耗时不含外围准备，整体报告耗时分别为 162.81、315.33、149.25 秒。合并仅发生在各自专用测试 base。完整 L2/L3 资格仍为 false，不扩大为全 agent、外仓或发布资格。

## 诊断轮次

首轮完整 L1 中两项旧断言仍要求 v6 空 workspace 检查点及旧 broker 分类，已改为拒绝断言并取消该轮。live adapter 编译发现 native Git 探针仍引用已删除路径解码器，已改为直接路径；live integration 编译同时清理失效 import 和未定义序列化 helper。随后复核纠正了对“已证明未创建资源的首次准备”的误限制，7 个队列/重启/撤销子场景完整通过；再补齐人工控制的持久期限及资源丢失出口。

acceptance-01 至 -04 均为诊断/取消轮次，不计验收通过。acceptance-05 完整执行后仅两项旧行为断言失败（旧远端 profile 只读投影、过期准备继续自动对账）；application 637.12 秒，未超原 720 秒预算。两项已按 v7 新契约修订并定向通过，随后在最终测试内容上独立重跑两个完整阶段。生产源码发生变化的早期 smoke 曾完成，但后续 Runner guard 正确拒绝其最终资格；这些记录保留原失败/取消状态，不与最终结果拼接。最后两项仅测试断言变更的字节一致复核与此区分，见上述最终构建说明。P1/P2 历史证据未改写。

## 最终验收与交付边界

| 最终检查 | 结果 |
| --- | --- |
| 完整 L1 | 全部 37 包、720 个顶层 Go 测试通过；16 个既定 helper/native opt-in 跳过，不算原生通过，无异常跳过 |
| L1 其他门禁 | 26 组 framework、29 项扩展通过；native driver 编译 exit=0，零匹配守卫通过 |
| L1 耗时 | application 593.98 秒，低于原 720 秒预算；L1 core 807.93 秒，不把单包耗时写成整轮耗时 |
| 完整 Controlled | 默认 catalog 的 26/26 顶层场景通过，零跳过、零失败，1457.76 秒；保留原完整 45 分钟预算，未调高或缩减场景 |
| 人工出口 | 公开无 observer、自动超时、首次 resume 失败、新请求再查成功、幂等回放、原输入仅一次、唯一后继及关闭通过；组件另覆盖提交失败/重载、期限不续期、丢失及 stop 竞争 |
| 同构建重启 | graceful/kill、原创建回执及启动 pending stop 均通过，原 owner/session 保持，未知不重发 |
| 实际来源 | 两个完整阶段开始/结束来源指纹完全一致；均为独立完整执行，不拼接失败轮次 |
| 真实最小链路 | 上表 3/3；新 v7/user_version=7，最终生产 EXE 相同，原 Run/服务已停止 |

最终证据见[汇总](../../../../acceptance/evidence/personal-executor-p3-20261101/verification-summary.json)、[完整 L1](../../../../acceptance/evidence/personal-executor-p3-20261101/complete/l1/stages.json)、[完整 Controlled](../../../../acceptance/evidence/personal-executor-p3-20261101/complete/controlled/stages.json)、[v7 原生现场](../../../../acceptance/evidence/personal-executor-p3-20261101/native-fixtures.json)、[生产增删指标](../../../../acceptance/evidence/personal-executor-p3-20261101/metrics.json)及[P3 专属生产 diff](../../../../acceptance/evidence/personal-executor-p3-20261101/production.diff)。[文件清单](../../../../acceptance/evidence/personal-executor-p3-20261101/files.json)核对归档字节；大型 JSONL/coverage 无损 gzip，解压后摘要见[原始流摘要](../../../../acceptance/evidence/personal-executor-p3-20261101/raw-files.json)。原 .evidence、P1/P2 历史证据及诊断轮次均未改写。

本批交付为共享工作副本和独立验收文档，未提交、安装、部署或发布。P4 未推进，总计划未完成。旧 v6 执行须由原版本排空；外部 SteerLine 联验、全 agent/完整 L2/L3 资格及产品发布不在本批完成范围。
