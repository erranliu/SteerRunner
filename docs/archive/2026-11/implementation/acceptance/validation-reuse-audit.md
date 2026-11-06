# 验证复用实现审计与 PR 验证

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-10-31。状态：实现审计修复完成，最终默认完整前置及真实 L2 均通过。[机器汇总](../../../../acceptance/evidence/validation-reuse-20261031/audit-qualification.json)。前序实现证据见[原验收](validation-reuse.md)，不改写原始结果；契约见 [Spec](../spec/validation-reuse.spec.md)。

## 确认问题与修正

| 问题 | 影响与修正 | 新验证 |
| --- | --- | --- |
| P1：只拒绝部分扩展名的 ignored 输入 | Git 忽略的 txt/bin 等数据可能参与测试或 embed，却不进入摘要；现在拒绝输入范围内任意 ignored 文件，native-only 排除仍按阶段区分。目录/gitlink/reparse 输入也拒绝，不猜测外部依赖 | 实际 ignored txt/bin、隔离 gitlink 反例；默认执行仍不受不可复用状态阻断 |
| P1：子进程创建时间二次解析丢失时区 | PowerShell 可将 JSON ISO 时间直接转换为 DateTime，再 Parse 会通过无时区字符串转换，在非 UTC 主机误判活跃 PID 已复用；改为保留 DateTime 类型直接转换 UTC | 真实仍存活子进程先暴露半个 JSON，退出后结果才允许读取；原判断在此测试实际失败，修复后通过 |
| P2：先解析 result，再等 harness 退出 | result.json 的可见性不代表写入或进程结束；现在沿原聚合期限先确认子进程退出，再解析结果，不改变断言、场景或超时预算 | 实际半写结果发布及超期拒绝合同；完整 Controlled 重新验证 |
| P2：构建/运行设置遗漏 | GOAMD64/GODEBUG 改变未改变输入身份；现记录这两项；不支持非空 NODE_OPTIONS 的预加载/附加设置，原默认入口继续允许原环境执行 | 真实 Go 环境切换摘要差异、Node 设置拒绝 |
| P2：合同入口误把不可复用当作默认失败 | 仓库含不支持复用的目录输入时，直接调用严格 snapshot 的合同冒烟会阻断原默认测试；改用默认入口相同的 unavailable 回退，并在普通隔离仓库单独验证严格 snapshot | 不支持输入的默认回退合同；正常严格 snapshot 冒烟仍保留 |
| P1：原始证据 Git 换行转换 | 新证据目录继承 text/eol=lf，原始 manifest 经索引过滤后 Git 对象 SHA 与原始字节不同，破坏归档 SHA；增加目录 -text/-whitespace 保护 | 暂存和提交后的 blob SHA 与原始内容一致；资格证据 SHA 全量重算 |

不修改生产 Go、catalog/registry、integration/native 测试主体、权限/发布/停止逻辑、循环、skip allowlist 或场景期限。L3/L4 不开放复用，原可选字段缺省路径保持。

## 合同检查

75 项通过，源记录 `.evidence/validation-reuse-audit-contract-03/prerequisites/verified.json`。包括前序 66 项、任意 ignored 数据、目录输入及默认回退、构建设置、Node 附加设置、真实子进程结果发布与原 deadline。原失败合同尝试保留在 audit-contract-01，不追认通过。

## 最终验证

最终源码固定为 `8f8fbdfacd154739bb397db709128cc55ce7dd0e`，source_dirty=false。默认 `test.ps1 -Level L1,Controlled -OutDir .evidence/validation-reuse-pr-final-baseline` 通过：75 项合同/完整 framework、717 个 Go 顶层、29 个 Node 扩展、编译/零匹配 guard、Controlled 25/25 零跳过。readonly Go flags 下另行 75 项通过。[默认汇总](../../../../acceptance/evidence/validation-reuse-20261031/audit-baseline-summary.json)、[Controlled 原始结果](../../../../acceptance/evidence/validation-reuse-20261031/audit-controlled-result.json)、[合同](../../../../acceptance/evidence/validation-reuse-20261031/audit-contract-verified.json)、[readonly 合同](../../../../acceptance/evidence/validation-reuse-20261031/audit-readonly-contract-verified.json)。

两个直接成功阶段输入摘要均为 `b43e61f0d58269392ad392ce6fa583fe4a793eb58c52e34fccd64ce73d3ea13a`。新 manifest SHA256 `67b065f3dfa969745dbad240783ff2881645eeba4520d4455f3481f8f8b2f1bf`。[原始证明](../../../../acceptance/evidence/validation-reuse-20261031/audit-prerequisites.json)。旧 manifest 因实现/摘要变化失效，未回填重签；过程中停止的旧候选保留 cancelled.json，不授予资格。

正式 `test.ps1 -Level L2 -PrerequisitesFrom .evidence/validation-reuse-pr-final-baseline -Config .evidence/host-native-auth-l2-config.json -OutDir .evidence/validation-reuse-pr-final-l2` 通过，L2 1/1、complete_level_passed=true、baseline.l2_verified=true。两个阶段 reused，本轮 preflight executed，新目录无 l1.jsonl、coverage.out、controlled、framework/result.json。总耗时 218.62 秒（约 3 分 39 秒），初次来源核验 5.46 秒。[正式报告](../../../../acceptance/evidence/validation-reuse-20261031/audit-summary.json)。

固定 Runner SHA256 `228c530e788097080a5770014636d891246aa4aa9bdedccf6fc200be33c47220` 与来源实际受测产物一致；driver SHA256 `9b9c9d108b7467fb38cbd10374fc0dc97cee3de5866ea2566c4ff3ffc10eecae` 与本轮 guarded driver 一致。[Runner 回执](../../../../acceptance/evidence/validation-reuse-20261031/audit-runner-build.json)、[case](../../../../acceptance/evidence/validation-reuse-20261031/audit-case.json)、[执行身份](../../../../acceptance/evidence/validation-reuse-20261031/audit-source-identity.json)。

新现场 `C:/srt/b082660d2a59`，Run `bbff79eeb3ae2c64918014f4b806e3d1`。真实测试 PR [#724](https://github.com/erranliu/SteerlineTest/pull/724) head `1915b844bcd745c488ef7f666d0e6eaa255b4ec0`，仅合入专用 `codex/srtest/b082660d2a59/base`，merge `8a384a2f58633a28c82643ce0b7335760fa6d7fe`。GitHub 连接器独立确认 merged=true、base/head/merge 一致。[独立查询](../../../../acceptance/evidence/validation-reuse-20261031/audit-github-pr.json)。

原合并后 Git/checker、反馈拒绝、权限/命令审计及停止断言保持：22 条实际命令、权限错误 0；原生 history 2 contexts/15 tools，提权、审批和 bypass 均 0；2 个 turn 绑定通过。Run 与服务均停止。[业务与收尾](../../../../acceptance/evidence/validation-reuse-20261031/audit-scenario.json)、[命令](../../../../acceptance/evidence/validation-reuse-20261031/audit-command.json)、[权限](../../../../acceptance/evidence/validation-reuse-20261031/audit-permission.json)、[turn](../../../../acceptance/evidence/validation-reuse-20261031/audit-turn.json)。

原来一次创建身份场景失败和后续全量通过仍按原验收保留。没有削减任何场景/断言/循环/skip 规则，没有替换真实 PR 验证；L3/L4 复用和相应新真机资格不在本轮范围。

## 主分支合并回归（2026-11-01）

合入 `origin/main` 的 `38becd0`（parser 解耦）。唯一文本冲突是验收索引，保留双方条目。生产实现、catalog 和 native 场景采用主分支内容；旧前置证明在合并后的源码上实际被拒绝，未重签或复用旧资格。

默认 `test.ps1 -Level L1,Controlled -OutDir .evidence/validation-reuse-merge-main-20261101`：完整 framework/75 项合同、716 个 Go 顶层测试、29 个 Node 检查、native 编译及零匹配保护通过，原 16 项 opt-in skip 保持。Controlled 实际执行 25 项，24 通过、1 失败；失败仅为 `TestSRInputFreeze` 的旧 parser 配置受理断言。[默认汇总](../../../../acceptance/evidence/validation-reuse-20261031/merge-baseline-summary.json)、[原始结果](../../../../acceptance/evidence/validation-reuse-20261031/merge-controlled-result.json)、[日志](../../../../acceptance/evidence/validation-reuse-20261031/merge-controlled.log)、[framework](../../../../acceptance/evidence/validation-reuse-20261031/merge-framework-result.json)。该整轮保持 failed，不授予完整前置资格。

主分支 [PD-03](../../../../spec/parser-decoupling.spec.md#5-内存文本任务与结果采纳pd-03) 已要求无解析需求不检查 parser 配置。仅同步该测试：保留原请求冻结、并发幂等和冲突检查；新增损坏 parser 配置下新 development Run 完成 `pr_ready`、精确两会话/六回合断言，再验证非法 Run 配置以 `invalid_input` 拒绝且不增加任何会话/回合。未将 parser 配置重新耦合回受理流程，也未修改生产代码、其它场景、skip 或期限。

修正后 `test-foundation.ps1 -Pattern '^TestSRInputFreeze$' -OutDir .evidence/validation-reuse-merge-input-freeze-20261101 -TimeoutMinutes 8`：harness 退出且 1/1 通过，耗时 41.97 秒。[原始结果](../../../../acceptance/evidence/validation-reuse-20261031/merge-input-freeze-result.json)、[日志](../../../../acceptance/evidence/validation-reuse-20261031/merge-input-freeze.log)。其余 24 项的先前通过与本定向结果分别登记，不拼接为新的整轮通过证明。修正后另行复验 75 项复用合同。

失败现场 `sr-3960411983` 保留；原 Run `da3fb7acfc46adb6b11e23df2922d15b` 曾未停止，使用原记录 revision 的 `PendingStops` 启动后核验 `stop_status=stopped`、revision=20，再按该服务 instance 停止服务；CIM 确认现场进程为零。[Run 收尾](../../../../acceptance/evidence/validation-reuse-20261031/merge-cleanup-run.json)、[服务停止回执](../../../../acceptance/evidence/validation-reuse-20261031/merge-cleanup-service-stop.json)。本次没有重跑真实 L2；上节 L2 仍只证明其固定代码版本。
