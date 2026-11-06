# Spec A：当前 Orca 真机验收改接结构化协议

日期：2026-11-06。基线 main `d58054a7`（#121 已合入）。状态：设计审计后已实施；受影响受控通过，实际 L3 未执行。配套 [Plan](orca-structured-qualification.plan.md)。长期依据为 [产品契约](steerrunner.spec.md) PC-R01–R09 与 [Orca 设计](../orca-execution.md)。本增量不修改生产执行、公开 DTO 或数据库格式。

## 目的与实际消费者

减少当前开发需要理解的旧 TUI/helper 知识。已注册的 orca-local.codex/pi 的 development-feedback、双端 native-development/native-active-stop，以及通用 smoke/multirun driver 仍在使用旧 session 前缀、readiness/owner/Job、terminal send 日志。不是零消费者；不能删案例或使零匹配/skip 通过。config.Evaluate 与 Manifest.Validate 要求 helper，与当前 Run 受理拒绝 helper 矛盾。CheckPR、remoteDiagnostic、candidateState、restartTerminalService、auditWorkerSandbox、localStopEvidence 都须逐个改接。

实施审计更正：产品支持本机 Orca audited，但目录当前只注册 local-codex.audited，没有 orca-local.audited。auditManagedLocalOrcaModels 和脚本模型守卫只有名义上的 Orca 分支调用，尚无对应注册合法真机入口。本批可使其解释当前结构化证明并保留模型反例，但不新增场景，也不声称该组合取得真机资格。

共享的 execution-stability 专项仍消费旧 receipt/control/network 夹具，本批不声称已迁完它们；不删除 AnalyzeNativeAudit、ReceiptAuditPolicy、LocalPermissionAudit、pre-send 等仍有调用的工具。保留直接 Codex 原始历史、Job/owner、审批和实际命令检查，不套用 Orca scope。

## 改接边界

1. 配置/manifest 沿当前完整 RunConfiguration/RunRequest 准入，要求绝对 transport、原 runtime/环境/仓库；当前 Orca 不要求且不容许旧 Agent helper/launch executable/args。注册项、场景和资格级别不变。
2. 测试侧集中读取本 fixture 只读库中原 envelope、profile、worktree/runtime/host/session、操作与输入；复核公开 RunView。`oas_` 仅作 Run/Attempt 确定性绑定，不能凭前缀证明 runtime；再向冻结 runtime/host 的结构化 History/Item 与原 worktree 查询核对。缺原证据、改 host/runtime/worktree/epoch、缺页/缩略未补齐均失败，不降为成功。
3. 问答/反馈和 PR 保留原 session/Attempt/generation/command/turn/final/真实平台 head 绑定；重启在原授权、工作区、会话及输入历史不变后通过，不创建新业务输入。实际模型审计从结构化 turn 的原模型事实核对，不再读 helper readiness。
4. 本机测试使用测试侧透明 named-pipe 转发器，通过独立 ORCA_USER_DATA_PATH 元数据转发至原认证端点；CLI 转发仍使用原用户配置。只记录请求/响应，不伪造 status/capability/业务事实，凭据不写 evidence。记录实际 create/send/close，支持只丢弃原 create/send 的真实响应，验证原操作只读对账及零重放。独立 Run、original request、完整清理保留。转发器不进入 Runner、不提供生产 hook/新协议。服务重启前后同一转发器存活。
5. 远端保持原配对 host key，不能以解密代理或新测试 host 替换。远端真机在 close 前通过原 durable operation + 完整结构化用户/turn/submission 证明原输入身份；close 后用原已确认输入集与撤销/停止回执核对没有新增业务操作；这不是重复 wire 调用次数证明。重复 RPC 零派发由现有受控协议/生命周期测试覆盖；报告须区分本机 wire 与远端语义计数，不能把 CLI journal 当结构化调用日志。远端缺完整历史仍失败。
6. 原 stop/verify-stopped 采用 `orca_agent_session`，必须有原 close 成功回执或原 create 未派发；close 未知不能通过。保留 stop 后零新派发及服务停止。旧 Job 清场/任意派生进程不存在不属于当前 Orca 承诺：旧活动父子进程用例改为原结构化 active turn + close，不再要求 Pi helper Job，也不将其标为宿主清场资格。直接 Codex 的 Job 检查保持。

## 删除、保留与格式

删除当前 driver 内旧 session 解析、专属 helper/Job/readiness 路由、旧 terminal 输入丢回执探针；替换为上述原结构化证明。仍有 stability/历史读取消费者的共享工具/类型/资产保留，不整文件误删。注册场景、固定产物/manifest 摘要、严格配置、实际 PR/反馈、原操作恢复/停止与失败清理保持。新增数据仅为测试取证，不改任何生产持久声明、tag、codec、schema 或数据，不增加生产入口。

## 最小验收与资格

- A01：无 helper 的当前各组合配置与 manifest 可达；helper/非法组合仍拒绝；CheckPR 拒绝跨 Run/Attempt/session/final/agent。
- A02：测试侧原 envelope/公开身份/完整历史正反例，包含缺证明、改 runtime/worktree/host、缺页、cursor/epoch、重复/外来输入；本机转发真实响应丢失后零重放测试。
- A03：编译全部 live driver/helper，运行 livetest/framework 及受影响公开受控执行、恢复、停止；当前注册项不删除，共享夹具消费者保留。
- A04：实际真机须固定 Runner/driver/Orca 摘要，按 docs/testing.md 正式 focused 入口逐项运行：本机 Codex/Pi feedback、双端 native-development/native-active-stop、受影响 smoke/multirun（本机 Orca audited 未注册，单列资格缺口）；有真实 History/Item、PR/停止回执及无新输入证据才记通过。环境或配置未就绪记未执行，不以编译/合成夹具/远端替代本机资格。实现完成与真机资格分别记账。

收益是当前测试与当前执行共享协议边界，移除开发反馈中的旧 TUI 搜索空间；不宣称执行提速或扩大现行组合。本批不迁移全部 execution-stability driver。
