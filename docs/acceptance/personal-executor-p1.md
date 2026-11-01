# 个人执行器 P1：快速定向验证独立验收

日期：2026-11-01。状态：P1 已实施并独立验收；P2–P4 未实施，不宣称总计划完成或完整 L2/L3/交付包资格。依据 [Spec PES-04/收益指标](../spec/personal-executor-simplification.spec.md)及 [Plan P1](../spec/personal-executor-simplification.plan.md)。[机器汇总](evidence/personal-executor-p1-20261101/qualification.json)。

源码为 `e2729563562314659a799a5f82debe427fcd7c24` 加本次未提交变更，source_dirty=true。生产 Go、catalog/case 注册、原生测试主体、权限和 state/v6 均未修改。实际脚本/合同文件摘要见 [source-files.json](evidence/personal-executor-p1-20261101/source-files.json)，Runner [构建来源](evidence/personal-executor-p1-20261101/runner-build.json)绑定当前生产输入；不将工作副本报告为 clean 或其他提交已通过。

## 实施边界

- `test.ps1 -Focused` 只执行明确选定的 L2/L3 case；Config、匹配生产输入的固定 RunnerExe、全新显式 OutDir 必填，拒绝通配/重复/未注册/级别不匹配场景及 Catalog/ListAvailability/PrerequisitesFrom 组合。
- 原 L1/Controlled 调用被跳过，native preflight 的新编译和零匹配保护保留。同一已检查 driver 经原 scheduler 传到 L2/L3 worker，执行前后核对 Runner、driver、原生输入及冻结配置/目录摘要，不复制 case、fixture 或清理实现。
- 报告增加 validation_scope、前置实际启动次数和阶段耗时。focused 选定 case 通过可以报告 passed，但所有完整级别及 baseline verified 均为 false。完整并行进度资格消费者也明确拒绝 focused 报告。
- 原默认调用路线、L2 PrerequisitesFrom 范围和完整资格汇总保持；显式固定 driver 传递扩展到 L3，不开放 L3 的前置证明复用。旧 driver 的 partial/stale/级别拒绝回归保留，原“错误级别”负例改为仍不支持的 L4。

## 合同和相邻回归

| 检查 | 结果与证据 | 范围 |
| --- | --- | --- |
| Focused 合同 | [26 项通过](evidence/personal-executor-p1-20261101/focused-contract.json) | 真实 CLI 参数拒绝；实际协调器的进程局部 stage doubles 验证 L2/L3 focused、默认/复用派发、编译/零匹配/清理失败、零 case、driver 变化和完整资格不可恢复；实际 worker 的 L3 driver 准入后冻结配置拒绝 |
| 原前置复用合同 | [75 项通过](evidence/personal-executor-p1-20261101/prerequisite-contract.json) | 原 proof、driver 负例、默认/复用资格、失败/重复/级别和 Windows PowerShell 5.1 摘要规则 |
| 配置回归 | [通过](evidence/personal-executor-p1-20261101/configuration-contract.json) | 静态可用性、旧配置拒绝、未配置目标/不完整范围、独立派发；附带 21 项 Codex 最低版本检查通过 |
| 调度回归 | [通过](evidence/personal-executor-p1-20261101/scheduler-contract.json) | 原有并行/互斥 lease、worker 丢失、构建变化及资源碰撞检查 |
| 并行进度资格读取 | [6 个合成场景通过](evidence/personal-executor-p1-20261101/parallel-consumer-contract.json) | 包含完整样本及 focused 拒绝；不是实际并行原生资格 |
| 脚本静态检查 | 改动 PowerShell 脚本 AST 解析、git diff --check 通过 | 不替代上述运行验证 |

执行入口为 `tests/framework/focused-test.ps1`、`prerequisites-test.ps1`、`configuration-test.ps1`、`scheduler-test.ps1`、`progress-parallel-test.ps1`。配置合同沿原 framework 的调用环境先加载 live-catalog。原始输出位于工作区 `.evidence/personal-executor-p1/` 下对应目录；`test-framework.ps1` 已登记 focused 合同，未为本批重新执行完整 framework/L1/Controlled。

早期 focused 合同的 contracts-01/02/03 失败保留：新合同子启动器未传递退出码、复用 double 错取可变 Config、随后 script 作用域未绑定，均修正于测试替身；contracts-04 最终 26 项通过。首次直接独立运行配置合同缺少其原调用环境的 live-catalog，后按原环境复验通过。未通过的尝试不追认为成功，不以合成 stage double 授予真实资格。

## 正式本机定向执行

使用当前执行账号已有 Codex/GitHub 登录，原生 Codex 0.159.2，执行：

```powershell
pwsh -NoProfile -File .\scripts\test.ps1 -Focused -Level L2 -Scenario local-codex.smoke `
  -Config .\.evidence\personal-executor-p1\config.json `
  -RunnerExe .\.evidence\personal-executor-p1\bin\steerrunner.exe `
  -ConfigHelper .\.evidence\personal-executor-p1\bin\test-config.exe `
  -FixtureParent C:\srt -OutDir .\.evidence\personal-executor-p1\live-l2-01
```

[正式 summary](evidence/personal-executor-p1-20261101/live-summary.json)：status=passed、validation_scope=focused、选定 case 1/1，完整 L1/Controlled 启动次数各为 0、NativePreflight 为 1。L2 目录恰好只有这一 case，因此即使 registered/requested/passed 集合相等，complete_level_passed 和 baseline.l2_verified 仍均为 false；没有以该结果拼出完整资格。

| 实测阶段 | 秒 |
| --- | ---: |
| 配置与 helper 处理（本轮显式使用已构建 helper） | 0.76 |
| NativePreflight 新编译与零匹配保护 | 11.33 |
| 原 case batch（含调度和 worker） | 169.86 |
| 总耗时 | **185.85** |

[case](evidence/personal-executor-p1-20261101/live-case.json)另记 Git 边界 12.41 秒、fixture 准备 12.19 秒、原生执行及清理 143.81 秒；这些属于 worker/batch 内部，不能再与 batch 总时长相加。显式 Runner/helper 的事先构建时间不计入本次 test.ps1 时长，也没有伪称其为 0。

现场 `C:/srt/c13e96175937`，Run `77c905679e7c0da45f1770456ff4ceee`。原 case 保留 Git 原生提交/仓库隔离、真实实施/发布、反馈拒绝、PR 身份、合并后核验及停止断言。[scenario](evidence/personal-executor-p1-20261101/scenario.json)确认 run_stopped/service_stopped 均为 true；[原生结果](evidence/personal-executor-p1-20261101/native-result.json)及[日志](evidence/personal-executor-p1-20261101/native.log)为 1/1、无 skip。

专用测试 PR [#727](https://github.com/erranliu/SteerlineTest/pull/727) head=`e121628467bdf78998ce9183ca5058e69d5944ca`，只合入 `codex/srtest/c13e96175937/base`，merge=`4ddf5d72ab126e629415cc1cb3c9af638e493172`。[独立 GitHub 查询](evidence/personal-executor-p1-20261101/github-pr.json)确认 head/base/merge 及 merged=true，一致于原 case。未修改或重新登录账号，未部署 Runner 或处理生产任务。

Runner SHA256 `ee3c392d593ad058942be9b0be393bcc7055dc975244d0cea6ce169d5c8233bd`；preflight 与实际 [native-source-identity](evidence/personal-executor-p1-20261101/native-source-identity.json) 的 driver SHA256 同为 `7c1e4e357ec1a46e6467133728573a166ea7cfabb06f2280a6858fe863ecd46a`。实际执行后仅继续修正合同测试替身和文档，没有改动本次执行经过的生产/协调器/worker 链路；原 summary 的 preflight 输入清单保留其当时字节，不改成最终合同文件摘要。

## 收益及独立结论

PES-A07/A08 和 P1 收益门槛已满足：正式入口确实没有启动完整 L1/Controlled，case 非零且按原断言完整执行/清理，编译/零匹配及固定 driver 有效，完整资格始终 false，阶段耗时可核对。P1 独立交付不等待 P2 观测解耦、P3 恢复/v7 或 P4 职责收口。

没有本批相同源码/主机/配置下完整入口的匹配耗时对照；不宣称具体加速比例或“本轮省了 33 分钟”。历史前置耗时仅作动机，已证实收益是两套完整前置的启动次数降至 0，本轮实际定向总耗时约 3 分 6 秒。

本批未取得真实 L3/Orca focused 资格，L3 接线由合同与原 worker 准入反例覆盖；也未取得当前源码完整 L1/Controlled 或交付包资格。保持 state/v6 和现行恢复/观测行为，后续批次仍待独立实施。原始现场资源已按 case 停止，工作区/evidence 保留用于复核，不进行额外资源自动清理。
