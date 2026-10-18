# 按用户要求移除独立 App 认证 L3 case

日期：2026-10-18。用户明确要求删除此前尚未执行的 GitHub App 专项测试。
本次移除 `orca-remote.app-auth/default` 的两份注册、专用 fixture 配置和说明，
同步更新目录计数及既有目录回归检查；不修改可选 App 生产能力及其组件边界测试。

当前 L3 为 16 场景、23 case：远端 18、本机 5；含 L2/L4 的目录总数为 21 场景、28 case。
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
当前清单及八项管理员 case 见 [scope.json](evidence/orca-app-case-removal-20261018/scope.json)。
原受测实现至本次删除提交之间的 `cmd`、application/contracts/execution/resources、
`go.mod/go.sum` 无变更；原 10 项 L3 报告仍归属 `e492ec1`，不改为新 driver 的真机资格。
