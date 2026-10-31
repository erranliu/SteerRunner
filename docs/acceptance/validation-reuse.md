# 验证链路解耦与 L2 前置复用验收

日期：2026-10-31。状态：已实施，完整前置和正式复用 L2 通过。契约见 [Spec](../spec/validation-reuse.spec.md) / [Plan](../spec/validation-reuse.plan.md)。[机器汇总](evidence/validation-reuse-20261031/qualification.json)关联原始报告和摘要；失败历史保留，不追认通过。

本节为首轮实现的历史资格；随后修复及最终 PR 对应验证见[实现审计](validation-reuse-audit.md)。不将旧证明用于修复后的源码。

## 改造范围与原义务对应

基线 `5c5c1653961ad08a836e82fc84f0898667864a5c`。仅修改测试编排/证明、合同测试和文档；生产 Go、catalog/registry、integration/native 主体、循环和期限未改。

| 原义务 | 实施落点及保留方式 |
| --- | --- |
| framework、Go、Node、opt-in/skip | 共享 Invoke-SRL1Core；默认完整执行；原 16 个允许 skip 不计原生通过 |
| native 编译及零匹配 | Invoke-SRNativePreflight；复用 L2 新编译/guard，worker 核对同一 SHA；默认原编译路线保留 |
| 完整 Controlled 及 foundation 单元前置 | Invoke-SRControlledStage 调原 foundation；25 项及 ResultAcquisition 11 个内部 case 不变；45 分钟及原余量保留 |
| 配置与新现场 | 沿原消费者；选择来源实际 Controlled Runner，当前 helper/配置重新评估冻结，无目标仍早停 |
| L2 Git 边界、发布/权限审计、合并与停止 | 原 worker、fixture、native driver 及结果核验保留；真实 case 不复用 |
| L3/L4、调度/lease、PreparedArtifactsDirectory | 不开放复用；新字段缺省即原路径，L4 仍读本批 controlled 目录 |
| 报告与资格 | 保留集合/旧字段；最终失败不能被 finally 恢复资格，逐 repeat 核对；来源/耗时单列 |

## 合同检查

最终独立检查 66 项通过，并在 GOFLAGS=-mod=readonly 下再次 66 项通过：`.evidence/validation-reuse-contract-05/prerequisites/verified.json`、`.evidence/validation-reuse-contract-readonly/prerequisites/verified.json`。最终完整前置的 framework 也已通过。合成证明不授予真机资格。

覆盖输入增删/未跟踪、native-only/无关文档/docs JSON、工具/不支持输入、包展开差异、证据/产物缺失变化、同数量换名/重复/子测试、L1 意外 skip/零测试、Node skip、失败/运行中/递归来源、不完整 inventory、harness 未退出、失败父批次、失败资格/缺 repeat、部分 driver 字段/错误 SHA/级别、独立入口分派、Windows PowerShell 5.1 摘要一致。

## 完整前置

默认路线 `test.ps1 -Level L1,Controlled -OutDir .evidence/validation-reuse-baseline-01` 已通过：717 Go、29 Node、原编译/零匹配 guard、Controlled 25/25 零跳过。初版严格模式下的空 replace 处理使其阶段证明不可用，未补签或用于 L2。

该问题修复后补入真实严格模式、local replace、vendor 检查；另修正了合同测试在多个不支持原因并存时的判断，并以 readonly GOFLAGS 实测。候选 `.evidence/validation-reuse-final-prerequisites` 在 L1 完成、尚未准备 Controlled 时停止，`cancelled.json` 保留原因，不授予资格。

完整前置 `.evidence/validation-reuse-final-02` 的 framework、717 Go、29 Node 及零匹配通过，l1-core 封存为 passed；Controlled 实际执行 25 项，但 ResourceCreationRecovery/planned-initial-terminal 在读取创建身份时失败，因此 exit=1、批次 failed、Controlled 证明不可用。未补签或作为 L2 来源。失败现场 `C:/Users/lmas1/AppData/Local/Temp/sr-2038322516` 保留。

保留的 worktree-created.json 当前为完整身份，原测试忽略 ReadFile 错误，无法从该错误信息确定当时原因。用同一 Runner 的独立定向 `.evidence/validation-reuse-creation-diagnostic` 复验，ResourceCreationRecovery 原 7 个变体全部通过；未改测试主体、断言、循环或期限，也不以局部通过替代完整 Controlled。

源码保持冻结，最终完整 All 批次 `.evidence/validation-reuse-final-03` 通过：717 Go、29 Node、原生编译/零匹配 guard、全部 framework，以及 Controlled 25/25 零跳过。两阶段均有直接成功[原始证明](evidence/validation-reuse-20261031/prerequisites.json)，同一前置输入摘要为 `3111583c205ebcff8f40cd5f4338dd77c9e96f74803b429df6dcbd6ec2b4c0d9`；来源提交 `5c5c165...`、dirty=true，未把未提交源码报告成 clean。

Source manifest SHA256：`cefb6558e250f1bbbae4c1c306386f84cdea73227c4d29be3acd9360a2f6adfb`。L1-core 约 696.63 秒，Controlled 约 1305.95 秒；两个阶段共约 33 分 23 秒。证明仅绑定完成的阶段证据，原始本机路径和输入清单保持，文档中的归档副本不作为跨目录复用入口。

对尚在运行、缺少成功 Controlled 的真实 final-02 来源，正式 L2 入口实测拒绝，exit=2、results=0，且未启动前置或准备 fixture：`.evidence/validation-reuse-partial-rejected/summary.json`。

## 真实 L2

正式执行：

```powershell
.\scripts\test.ps1 -Level L2 -Config .\.evidence\host-native-auth-l2-config.json `
  -PrerequisitesFrom .\.evidence\validation-reuse-final-03 -OutDir .\.evidence\validation-reuse-l2-final
```

[正式 summary](evidence/validation-reuse-20261031/summary.json)为 passed，L2 1/1、complete_level_passed=true、baseline.l2_verified=true。两前置均为 reused，本轮 NativePreflight 为 executed；L2 输出中没有 l1.jsonl、coverage.out、controlled 或 framework/result.json。总耗时 267.00 秒（4 分 27 秒），初次来源核验 5.92 秒；Git 边界 13.48 秒、准备 9.29 秒、native 及其清理 215.86 秒。[case](evidence/validation-reuse-20261031/case.json)。

Runner SHA256 `66fdb60ac6c5b7566b946634e6e01d61baf2890bd59088b4bbd0fed23bcfc6d1`，等于来源实际受测 Controlled Runner；[构建回执](evidence/validation-reuse-20261031/runner-build.json)源码摘要仍为前轮产品的 `bf0c745aba28e40947173d3cfd36718071212add6187edf8eb33d98a71b3db37`。新预检和实际执行的 driver SHA 均为 `9b9c9d108b7467fb38cbd10374fc0dc97cee3de5866ea2566c4ff3ffc10eecae`，没有替换未经 guard 的编译物。[执行身份](evidence/validation-reuse-20261031/source-identity.json)。

新现场 `C:/srt/b160f403d519`，Run `9e3f733f17fe880ab0750e31784bcddb`，真实测试 PR [#723](https://github.com/erranliu/SteerlineTest/pull/723)，head `257d176013167e6af2ff1785f5a67737e97c9b11`，仅合入专用 `codex/srtest/b160f403d519/base`，merge `b92dc5d54a706658adbb929ec648a7f0c365af8e`。[GitHub 独立查询](evidence/validation-reuse-20261031/github-pr.json)确认 merged=true，head/base/merge 均一致。

原反馈拒绝检查、合并后 Git/checker 核验保留；24 条实际命令、权限错误 0；原生 history 2 contexts/15 tools，提权/审批/bypass 均 0，2 个 turn 绑定通过。Run 和服务均已停止。[业务与收尾](evidence/validation-reuse-20261031/scenario.json)、[命令审计](evidence/validation-reuse-20261031/command-audit.json)、[原生权限](evidence/validation-reuse-20261031/permission-audit.json)、[turn](evidence/validation-reuse-20261031/native-turn-audit.json)。没有登录/登出、取 token 或更改宿主凭据设施。

## Spec 验收对应

| 验收 | 证据 |
| --- | --- |
| A1 默认等价 | baseline-01 默认 L1+Controlled 全部通过；最终 All 再次真实完整执行；生产代码、catalog/registry、integration/native 主体 diff 为空 |
| A2 独立阶段 | 66 项合同中的独立入口 L1/Controlled/All 分派；最终 All 实际通过，无模型/PR；合成报告明确无原生资格 |
| A3 正向复用 | native-only 输入变更合同 + 最终正式 1/1 L2；两个 reused，实际 guarded driver 匹配，无前置执行输出 |
| A4 保守失效 | 源码/共享测试/脚本/样例/依赖增删、工具变化、不支持输入等合同；真实 vendor/local replace 检查；readonly flags 不阻断默认合同执行 |
| A5 结果可信 | 部分/伪完整/失败/运行中/递归来源、证据缺失变化合同；真实 final-02 失败未产生成功 Controlled；真实缺失前置入口拒绝 |
| A6 driver 与稳定性 | 实际 driver 等于本轮预检 SHA；输入/证据变化和 harness 未退出合同；正式 L2 停止结算完成 |
| A7 资格不扩大 | 失败最终闸门/缺 repeat/错误级别及半套 driver 字段合同；L3/L4 未开放复用；正式 L2 才置 verified |
| A8 兼容追溯 | 最终完整 framework 和 Windows PowerShell 摘要检查；原配置、目录、scheduler、PreparedArtifactsDirectory、delivery guards；原失败与来源/耗时保留 |

## 耗时口径与未验范围

报告区分前置 executed/reused、原完成时间、校验、本轮 preflight、Git 边界、fixture 准备和 case 总耗时。native_and_cleanup_seconds 包含原 harness 内执行与清理，不伪造独立清理时长；失败时未取得的细分不写成 0。

本轮不开放 L3/L4 复用；实际 Orca local/remote、Pi/DSH 未取得本轮新真机资格。交付包资格沿原入口，本轮前置证明不替代它。
