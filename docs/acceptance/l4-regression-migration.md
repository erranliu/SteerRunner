# 五项低频回归迁入 L4

日期：2026-11-20（Asia/Shanghai）。基线 `8ebbc717776eb2ad85371060566776c4402773fa`，验证对象为本工作区未提交的测试分层改动。用户确认仅迁移下列五项，保留 L2，并按每周、发布前及相关改动执行 L4。

## 执行范围

| 原归属 | 原测试 | 新执行位置 |
| --- | --- | --- |
| L1 | `TestRunNamePrefixFreeze` | L4 `component.low-frequency/name-prefix` |
| L1 | `TestDisplayRunsIgnoreRolledBackChanges` | L4 `component.low-frequency/display-rollback` |
| Controlled | `TestSRResultAcquisition/audit-report` | L4 `controlled.result-acquisition-fallback/audit-report` |
| Controlled | `TestSRResultAcquisition/parser-unsupported` | L4 `controlled.result-acquisition-fallback/parser-unsupported` |
| Controlled | `TestSRResultAcquisition/parser-tool` | L4 `controlled.result-acquisition-fallback/parser-tool` |

两个组件测试改由 `l4` build tag 编译；原函数正文逐项与基线核对，断言保持一致。结果获取原八项测试正文未修改，日常默认保留 plain-marker、audit-zero、audit-positive、audit-offline、parser-complete，另外三项通过独立 case 计划调用。旧 ID 仍支持显式 `-Case` 定向反馈。

Controlled 的 30 个顶层测试、L2 的唯一核心 smoke、全部 L3 注册及原四个 L4 场景保持原样。L4 现为 6 场景/9 case，仍独占调度。每个受控 case 使用独立证据目录、同批校验摘要的三份产物，并核验实际子场景回执、完整 RUN/PASS、无 skip 和原 harness 退出。组件 driver 核验精确测试实际运行一次、前后输入指纹一致；正式批次另核对本批完整前置证明。迁移后的源码及清单进入既有前置指纹，旧证明不能沿用。

本聊天已创建每周五 16:00（Asia/Shanghai）自动任务 `steerrunner-l4`。发布前对拟发布构建运行完整 L4；命名/展示或结果获取/parser 相关改动按[测试指南](../testing.md)定向运行。每周通过只属于当次构建。

## 验证结果

原始证据根为 `.evidence/l4-migration/`。可纳入版本管理的日志、结果、压缩后的来源身份及摘要位于 [evidence/l4-regression-migration-20261120/](evidence/l4-regression-migration-20261120/)，总览为 [verification.json](evidence/l4-regression-migration-20261120/verification.json)。来源身份保留原完整文件路径及 SHA-256，组件的完整输入库存仍在原证据根；未将缺失证明补签为完整资格。

| 检查 | 结果 |
| --- | --- |
| 编译后的 application 测试清单 | 日常 220 项、带 l4 tag 为 222 项；差集恰为指定两个函数，没有移除其他测试；[清单证明](evidence/l4-regression-migration-20261120/inventory-proof.json) |
| 注册及原断言对照 | L2、L3、原四个 L4、Controlled 父测试清单保持一致；两个组件函数及原结果获取测试正文保持一致；[注册证明](evidence/l4-regression-migration-20261120/registry-proof.json) |
| `go test ./internal/testkit/livetest -count=1` | 通过；包括核心可靠性仍为 L3、L4 绑定及五项新增 case 的唯一性检查 |
| 完整 `scripts/test-framework.ps1` | 最终 27 项全部通过，exit=0；包括配置/冻结产物、目录、调度、前置复用及 Focused；[结果](evidence/l4-regression-migration-20261120/framework-result.json) |
| Controlled 选测/回执框架 | 87 项通过；新增日常五项与 L4 三项无重叠、并集等于原八项的检查，继续拒绝父测试通过但实际子场景缺失、重复、失败、skip 或回执不符；[检查列表](evidence/l4-regression-migration-20261120/selection-verified.json) |
| L4 组件调度分支 | 合成 worker 的两个稳定来源通过，跨 case 漂移、case 内漂移和空指纹三种反例拒绝；无并行重叠。此项不冒充真实组件运行；[结果](evidence/l4-regression-migration-20261120/component-scheduler-verified.json) |
| 最终源码的两个组件 driver | 两项各实际运行一次且通过，exit=0、无 skip，前后输入指纹一致；见组件 result/identity/go-test 文件 |
| 日常结果获取默认入口 | 同一冻结 Runner 在短临时根下五项全部通过，约 318.86 秒；未显式传 Case，实际回执只有五项；[结果](evidence/l4-regression-migration-20261120/daily-result.json) |
| L4 的三个结果获取 case | 真实调度器及正式 worker 逐项通过，分别约 82.56、89.48、81.34 秒；独立证据目录、精确 ID、实际回执及固定 Runner SHA 一致；[定向报告](evidence/l4-regression-migration-20261120/l4-worker-summary.json) |

五项迁入测试均已实际通过。目录差集、注册对照以及日常五项/L4 三项的实际回执共同证明本次迁移没有丢失原测试覆盖。上述成本来自不同现场，不据此宣称日常入口总墙钟提速比例。

## 保留的失败与资格边界

- 首轮完整框架暴露 worker 状态判断依赖旧模拟请求不存在的 target 字段；已改为按 driver 判断，完整框架重验通过。保留 [首轮失败](evidence/l4-regression-migration-20261120/initial-framework-worker-failure.json)。
- 默认临时根下的首轮日常五项为两项通过、三项失败，涉及原 clean checkpoint、提交回执或 parser 流程。原失败、原 Run 及现场未改写；在同一冻结 Runner、原测试正文和原期限下，以独立短临时根 `C:/srt/l4mig-20261120` 重验五项全部通过。这不证明路径是唯一根因，也不将首轮失败计为成功；[首轮日志](evidence/l4-regression-migration-20261120/initial-daily-integration.log)及[结果](evidence/l4-regression-migration-20261120/initial-daily-result.json)保留。
- 定向准备产物时，StandalonePackage 的既有 rejected-submit 受理回执断言失败，未修改或豁免该断言；[原日志](evidence/l4-regression-migration-20261120/standalone-failure-integration.log)保留。三个 L4 worker 的定向接线验证采用另一独立批次中实际通过的 NativeCatalogPublicCLI 产物，未制造完整前置证明。正式 `test.ps1 -Level L4` 仍须通过本批完整 L1 和 Controlled，不能由该定向准备代替。
- 组件调度诊断脚本首版比较混合 DateTime/字符串时误报重叠，改为统一 UTC 后重验；首版证据保留在原证据根。此项只修正诊断脚本，没有改动调度互斥规则。清单对照脚本曾因默认 GBK 读取源码失败，改为 UTF-8 后重验通过。

本次取得测试迁移的结构、框架及定向实际运行验证；没有执行完整 L1 Go 集合、完整 Controlled、完整 L4 或真实 L2/L3，不声明这些完整级别、真实模型/PR或合并后资格。上述验证阶段未创建或合并 PR。提交 PR 前为本证据目录补充 `.gitattributes` 的原始字节保留规则，并核对 Git 索引中的 SHA-256 与原清单一致；未修改测试实现。交付前刷新 `origin/main` 并由 GitHub compare 核对远端 main 与上述基线一致，未将本地差集检查冒充平台 PR 可合并状态。
