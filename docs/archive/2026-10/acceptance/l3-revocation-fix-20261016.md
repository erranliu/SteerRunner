# 不可达撤销 revision 饥饿修复与真机复测（2026-10-16）

> 历史归档（2026-10-21）：保留原构建、设计与验收事实；当前规则或汇总结论见[当前入口](../../../test-catalog.md)。归档原因及范围见[归档索引](../../README.md)。

**连续 `stale_revision` 已修复，修复构建 `46f0946` 的不可达撤销真机用例通过；rebase 后 `a778ba7` 的重跑被主线新增认证能力门槛阻断，未通过。** 两个来源都从 clean 工作树开始，分别记录。本次停止修复没有修改五次请求重试上限、拒绝码契约或 240 秒不可达窗口。此前完整一轮 10/12 的历史结果仍见 [rebase 复测记录](l3-rebase-20261016.md)；本次仅复测停止撤销场景，不能记为新的完整 12 项 L3 通过。

## 根因与修复

停止 worker 每秒对账。远端不可达且结果没有变化时，`stopRun` 仍无条件提交 `stop_unknown`，每次都增加 Run revision。调用者读取当前 revision 后，等待 dispatch gate 并发送反馈时版本已再次变化，五次请求都只能取得 `stale_revision`，无法到达撤销后的 `resume_required` 拒绝。

本次先规范重复停止回执的落盘：身份验证通过、状态和 canonical receipt 完全相同时保留原 journal 表示，避免仅因前次回执包装层变化产生伪进展。`stopRun` 再比较完整 Run 聚合，仅在内容发生变化时提交。原资源仍持续对账；诊断变化、owner 的新停止进展和最终停止事实仍正常落盘。CAS 与授权检查的顺序和条件未改变。

SQLite 回归测试在 inspect 与 reply 之间主动插入停止对账：原生产代码明确以 `stale_revision` 失败，修复后连续五次取得 `resume_required`，且没有新增 revision、更新时间或事件；重载后的未知停止仍稳定，新诊断、新 owner 进展及最终停止均能持久化，旧 revision 仍被拒绝，停止操作保持原身份且只派发一次。原有 foreign receipt 回归继续通过。

## 真机不可达结果

现场 `f0b9c9cd4d80`，Run `7ba487f6e7298166326c165713a365c4`。故障实际覆盖约 **249.655 秒**，满足原定 240 秒窗口并跨服务重启。

| 请求 | expected_revision | 响应 |
| --- | --- | --- |
| revoked-feedback-0 | 29 | stale_revision，明确拒绝 |
| revoked-feedback-1 | 30 | resume_required，明确拒绝 |
| stale-revoked-feedback | 29 | stale_revision，明确拒绝 |

之后断连窗口内 **82 次 Run 快照的 revision 均为 30**。撤销、unknown stop、恢复门槛与零新输入检查全部通过；恢复连接后 Runner 确认原执行停止，六个辅助终端按原身份关闭，服务退出、设施恢复，无意外 PR。业务结果 `revoked-unverified` 表示不可达期间无法证实停止；清理结果单独确认恢复后的实际停止，二者没有混写。

native 驱动 SHA-256 为 `A20FCAF8E9CEC5A0A8A56475C6CBB3125FE912FA7BDFFEF736375D392577A308`，与旧失败轮相同。固定 Runner SHA-256 为 `bb480efc2d22995647bf51fb1e8baa783353ec1a61ef90c46d53dbf6ca2fcd39`。324 个来源文件、配置及所有执行工具摘要均独立核对。

## 批次与前置

L1 **192 项**、脚本检查 **12 项**、Controlled **12 场景**、live driver 编译及零匹配 guard、`go vet ./...` 均通过。直接相关的 application、operation、orcatransport 包测试也通过。

首次停止撤销批次实际执行 3 项，无排除；**1/3 通过，批次仍为 failed**。active 与 boundary 都在故障命中前发生原生启动 `account/read ... workspace routing discovery timed out (code -32603)`，未发送业务回合输入。active 收尾另遇远端连接中断，原 close 回执丢失，Runner 停止状态保持 unknown；在重新核对原 runtime、terminal incarnation 和 worktree 后进行单独现场清理，原失败和未确认的 Runner 停止结果保持原样。boundary 与 unreachable 均由 Runner 完成原生停止。

| 批次 / case | fixture | 结果 |
| --- | --- | --- |
| run-01 / active | e7960860cba8 | 原生准备失败，未命中故障；原终端经独立身份核对后单独清理 |
| run-01 / boundary | 862fd351882d | 原生准备失败，未命中故障；Runner 清理完成 |
| run-01 / unreachable | f0b9c9cd4d80 | 通过，完整窗口及严格错误码断言命中，Runner 清理完成 |

同一构建的两个独立新现场补测为 **1/2，failed**，不改写原批次：active `96b316c6113b` 再次因相同启动路由发现超时失败，故障命中 0，Runner 清理完成；boundary `f359daf6938a` 通过，原生停止、三个辅助终端关闭与服务/设施收尾均通过。原批次和独立补测各自保留来源、构建身份与通过率。

## 合入主线后的验证

主线在运行期间合入 PR #12、#13。为解除 PR #11 的脚本冲突，基线更新为 `c976cc4b5781c2ca65a6a85a47aed3ec44d305dd`，修复重放为 `a778ba7b6ac1fed1fafeb5aa7ea331da9f16d499`。旧 9 分钟入口等待修复被主线 15 分钟子宿主、15 分 30 秒入口等待所取代，保留主线版本；生产停止修复和回归不变。

主线认证能力检查新增了远端 GitHub 业务输入门槛：现有 Orca transport 缺少受信执行宿主的凭据准备及确定性的同沙箱命令能力，返回 `remote_sandbox_authentication_unsupported`。本次保留该主线行为，没有补齐或移除认证能力检查。

`a778ba7` 的 L1 **206 项**、脚本检查 **12 项**、Controlled **12 场景**、live driver 编译及零匹配 guard、`go vet ./...` 均通过；撤销回归及原身份/foreign receipt 回归再次通过。停止逻辑及新增回归三个文件与受测 `46f0946` 内容完全相同。

最新主线上的不可达用例新现场为 `c3fa314612bb`，Run `05e466784e7e222e8683e743fcf10273`。实际执行后在 `preparation_blocked` 以 `remote_sandbox_authentication_unsupported` 失败，业务回合尚未启动、原生业务输入为 0、稳定性故障命中 0；**该次没有进入 stop/重启/240 秒窗口，不能作为最新主线 L3 通过**。服务及前置网络探针均已收尾。要在该主线上完成完整 L3，需要先补齐被主线要求的 Orca 远端同沙箱认证能力。

最终核对覆盖六个现场和两个 Controlled 目录：本批执行程序及防火墙规则残留均为 0，四个最后注入记录中的 watchdog 原 PID 均已退出。受限管理员网络宿主于北京时间 15:11:59 协作退出，host_id 与原 PID/创建时间相符，队列为空。首次 active 的历史 `resources_settled=false` 与 operator 清理回执单独保留；原始失败结果没有改写。详见 [最终清理审计](../../../acceptance/evidence/l3-revocation-fix-20261016/task-cleanup.json)。

逐现场结果、断言、构建身份、来源摘要、CAS 诊断、耗时及独立清理审计见 [证据索引](../../../acceptance/evidence/l3-revocation-fix-20261016/README.md)。原始 SQLite、journal、CLI 回执、二进制及失败现场继续保留在测试主机。
