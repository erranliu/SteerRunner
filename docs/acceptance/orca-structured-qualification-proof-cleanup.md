# 当前 Orca 验收接线与旧证明计算器精简

日期：2026-11-06。基线 main `d58054a7`（#121 已合入）；分支 `codex/orca-structured-qualification-proof-cleanup`。先写四份 Spec/Plan，审计后按 A→B 独立实施。未部署、未提交真实业务输入、未创建 PR。

## 设计与实际消费者审计

A 对应 [Spec](../spec/orca-structured-qualification.spec.md)/[Plan](../spec/orca-structured-qualification.plan.md)。config/fixture/manifest/current native driver/result 都是实际入口；旧 TUI ID、helper/Job/readiness、terminal send 计数与当前准入及 scope 矛盾。生产结构化调用绕过 CLI，不能将 CLI 零 send 当业务零派发。实现将私有测试读取知识集中于 livetest 的结构化原记录/完整历史核验，本机测试侧 named-pipe 原样转发；远端仍使用原配对 host key，不引入测试解密主机或生产 hook。测试输入/恢复/停止与实际模型源保持区别。

审计更正：当前目录只有 local-codex.audited，没有 orca-local.audited；后者是产品合法组合但尚无注册真机入口。模型 helper/结果守卫按结构化原证明解释并保留反例，不为可编译性制造真实资格。execution-stability 的旧 control/网络/receipt 测试工具仍有真实注册消费者，保留，不声称已整体迁完。直接 Codex 的原始历史、审批、命令及 owner/Job 处理未改。

原 current driver 专属的 terminal ID、Pi owner/readiness/Job、旧 pre-worker executable 探针及丢 terminal-send 回执处理退役；丢回执探针改为原 structured create 的真实响应丢失，并在原历史暂时不可读时保留 unknown，再恢复查询对账，实际调用始终一次。活动停止改为正面观察原 turn running/Working 后正式 stop；停止仅按原 session close，额外派生进程清理不宣称有资格。close 后以冻结输入集/原 close 回执核验，避免新增“关闭后历史必须继续可读”的门禁。

共享检查：fakeHost、fixtureOn、structuredFixture、creation、local IPC、control/native receipt/stability 源夹具保留；当前改接没有删除目录场景。测试侧旧 smokeReceiptAuditPolicy 已无调用，独占函数删除；共享 ReceiptAuditPolicy/AnalyzeNativeAudit/LocalPermissionAudit、协议 StateFeed 和直接 Codex 支持保留。

B 对应 [Spec](../spec/orca-legacy-proof-calculator.spec.md)/[Plan](../spec/orca-legacy-proof-calculator.plan.md)。旧 NativeProof.query 的唯一闭包外调用是 codec roundtrip 测试；已有 Conflict 使其立即拒绝，并未重算后来的 end。保留该回归的编码/冲突/usage 原绑定检查，改查原保存事实。NativeProof.agentName 是 frozen_record 的实际消费者，整个 NativeProof 声明及严格解码保留。workspaceRepository 的 !structured 旧 hook preflight 无合法当前构造根；nativeAccess 仍有当前 FIFO/准备用途，不能整标志删除。

## A 独立验证（B 编辑前）

- livetest 全包通过，包含原配置/manifest/共享审计测试及新增原 envelope/原提交回执/完整分页/变更/缺失反例。本机 pipe 组件通过真实 named-pipe 转发，原认证 token 逐请求核对；只丢实际成功响应，journal 不含 token，原元数据字节未变。此为合成原宿主组件测试，不是真机资格。
- 全部 live driver 和共享 stability helper 编译通过。native-result（Windows PowerShell 5.1）、catalog 框架通过；旧 scope、缺失实际模型来源、异主/异 Turn、零场景/未清理等反例保留并按当前格式更新。
- 正式 build.ps1 构建 Runner；test-foundation.ps1 明确 Pattern 选择 TestSRSingleExecutionColdStop 与 TestSRResourceCreationRecovery，2 顶层/七创建恢复窗口通过，exit_code=0、scenario_count=2、零 skip。包括普通/强杀服务、原回执/丢响应、owner crash、无终端 worktree 回执和缺 creation plan；不等于完整 Controlled。
- A 仅改测试支持、driver、脚本和文档，没有生产执行或持久声明修改。代表生产编码 SHA256 为 `2a31af8009107c3f9e1d06d3193ceb44935ec67220842594c9d2606862b59113`，作为 B 对照。
- 首轮旧 manifest 夹具残留 helper/缺 absolute transport，以及误设未注册 audited 案例的测试假设，均已修正；helper 删除旧分支后未使用 import 的编译失败已修正。只统计最终通过，不将初轮失败计为资格。

## 真机验收入口及当前资格缺口

按 docs/testing.md，用固定当前 RunnerExe、调用方明确提供的独立 Config 与全新 OutDir，依次运行正式 `scripts/test.ps1 -Focused -Level L3 -Scenario <单个已注册 ID> -Config <配置> -RunnerExe <固定 EXE> -OutDir <新目录>`。本批相关 ID 是 orca-local.codex、orca-local.pi、orca-local/remote.native-development、orca-local/remote.native-active-stop；受影响 smoke/multirun 按各自现有注册入口运行。目录、固定产物、lease/清理、结果守卫和完整/定向资格规则不变。

本轮未提供指定真机 Config，未启动实际模型/Orca 业务或真实测试 PR；A 的实际 L3 资格未执行。远端输入计数来自 close 前完整原 History/Item 与持久原操作，close 后核对原冻结输入集；这不是重复 RPC wire 次数证明。本机 recorder 的实际 wire 检查与受控协议零重放反例分别记录，不能相互冒充。

## B 实施与验证

等待 A 独立验证点固定后实施；本节在完成后填入实际删除闭包、格式与受影响测试结果。
