# 按用户要求移除独立 App 认证 L3 case

日期：2026-10-18。用户明确要求删除此前尚未执行的 GitHub App 专项测试。
本次移除 `orca-remote.app-auth/default` 的两份注册、专用 fixture 配置和说明，
同步更新目录计数及既有目录回归检查；不修改可选 App 生产能力及其组件边界测试。

删除提交 `882f2b1` 时，L3 为 16 场景、23 case：远端 18、本机 5；含 L2/L4 的目录总数为 21 场景、28 case。
这属于用户要求的清单变更，不把删除的未执行项记为通过。原
[扩展验收](orca-remote-extensions-20261018.md)及所有历史 JSON 保持不变，
其 19 项远端清单、9 项当时缺口仅描述变更前状态。

原实现提交 `e492ec1` 的 10 项远端真机通过记录仍按原构建归属；本次未更改生产 Go 实现，
也未重跑或覆盖这些 L3 记录。测试清单与 fixture 的删除变更另做 L1/Controlled 回归。

删除 App 项后仍有 **8 项管理员故障未验收**：

- connection-reconnect：active-disconnect、pre-send-disconnect。
- service-restart：graceful-boundary、kill-boundary。
- stop-revocation：boundary、unreachable。
- publication-unknown：completion-gap、query-unavailable。

这些 case 保留注册，尚缺 ready 管理员故障宿主；前次 UAC 由用户取消后未再次发起。
按此前完整远端 L3 的验收目标，PR 仍保持草稿。移除 App 项不等于剩余 18 项全部通过，
也不把无冲突、无 GitHub CI 检查项作为真机通过证明。

## 删除后的回归

受测提交 `882f2b155fdf8e0364b296e9442d2088e3462b27`，开始时工作树干净。
L1 **289 项通过**，包括目录/PowerShell 框架检查、native driver 编译和零匹配保护；
6 个显式 helper/native skip 仍不计真机通过。Controlled **16/16 通过**。
回归后只补本节及有限 JSON 证据，未修改受测测试代码。

原始回归摘要见 [regression.json](evidence/orca-app-case-removal-20261018/regression.json)，
删除提交时的清单及八项管理员 case 见 [scope.json](evidence/orca-app-case-removal-20261018/scope.json)。
原受测实现至本次删除提交之间的 `cmd`、application/contracts/execution/resources、
`go.mod/go.sum` 无变更；原 10 项 L3 报告仍归属 `e492ec1`，不改为新 driver 的真机资格。

## 随后整合主线 PR #24

推送删除后，主线推进到 `7c48969669f7027814f3f30c4e27241b1bb54d7e`（本机 Orca 查询及 L3）。
合并保留主线 3 个本机 Orca case、当前远端审计/多 Run 和 App 专项删除结果。
当前 L3 为 **19 场景、26 case**：远端 18、本机 Codex 5、本机 Orca 3；
含 L2/L4 共 24 场景、31 case。远端八项管理员缺口不变。
准备脚本同时保留本机/远端显式路由校验与 CLI 命名互斥。
上述 `882f2b1` 回归发生在本次主线整合前；整合后的生产代码已变化，原真机记录继续保持原构建归属。

整合提交 `09e8fd43f0440d9acbde1d6e64f45a3d894b0537` 的干净工作树重新完成
L1 **295 项通过**、Controlled **16/16 通过**，包括 native driver 编译、零匹配及
目录/PowerShell 框架检查；7 个显式 helper/native skip 不计真机通过。
原始摘要见 [main-regression.json](evidence/orca-app-case-removal-20261018/main-regression.json)，
整合后的清单见 [main-scope.json](evidence/orca-app-case-removal-20261018/main-scope.json)。
其后仅补本文及有限证据；本次未重跑整合构建的真机 L3，不能将原十项记录提升为新构建完整资格。

## 随后整合主线 PR #26

主线推进到 `59c7597ac8056fc5c29ad622034e10e74e4395de`，新增 Pi/DSH 原生终端能力。
冲突解决提交 `1585a61ef7bd125a4c856cd092d49e573b58f61c` 保留远端双模型审计、App 引用与认证撤销、
CLI 互斥和多 Run，以及主线原生扩展、DSH 粘贴编码和 owned Job/认证清理。
同时避免将 Codex 审计模型或 App 配置应用到尚无对应映射的 Pi/DSH；
多 Run 按各自的 manifest 和 agent 检查证据，Codex 清理查询区分本机/远端路由。
仅在显式指定 Codex 模型时补默认推理强度，保留无模型旧配置的启动兼容性。
相关边界和两种 wire framing 的组件测试已通过。

合并后的 L3 为 **24 场景、31 case**：远端 18、本机 Codex 5、本机 Orca 8；
含 L2/L4 共 29 场景、36 case。独立 App 专项仍已删除，八项管理员远端缺口保持不变。
清单见 [terminal-agents-merge-scope.json](evidence/orca-app-case-removal-20261018/terminal-agents-merge-scope.json)。

受测合并提交工作树干净，正式入口重新完成 L1 **317 项**、终端扩展 **28 项**、Controlled **16/16**，
包括目录/PowerShell 框架、native driver 编译和零匹配保护；7 个显式 helper/native skip 不计真机通过。
摘要见 [terminal-agents-merge-regression.json](evidence/orca-app-case-removal-20261018/terminal-agents-merge-regression.json)。
回归后仅增加本节和有限 JSON 证据。本次未重跑合并构建的远端 L3，原真机报告保持原构建归属，PR 继续保持草稿。
