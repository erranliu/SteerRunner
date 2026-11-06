# 当前 Orca 验收接线与旧证明计算器精简

提 PR 前复审发现并修复三个测试取证问题，最新主线整合、真实受控记录接线和最终验证以[复审记录](orca-qualification-proof-reaudit.md)为准；下方保留原实施点的证据。

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

A 独立点为 `ceca3316`。最终审计补充的 A 修正单独保存为 `d31b7e7c`：本机 Codex reader 使用 Codex 的原 capability 集，不误用 Pi 的附加 capability；通用当前 native audit 同样核对 CLI 中无旧 writer，以及本机实际原输入派发。该修正已重新跑 livetest 全包和全部 live 编译；不混入 B 的生产删除提交。

| 删除闭包 | 保留项与证据 |
| --- | --- |
| NativeProof 的 query/end/attribution/body/preview 等旧计算链、nativeObservation、nativeInput 匹配/补证链 | NativeProof 整个声明和 agentName、state_codec 严格解码/矛盾拒绝、旧 State/Turn/Profile/owner/control/receipt 格式保留 |
| 无合法当前执行根的 !structured hook preflight/verifyLocalObservation、无消费者 stateFeedFactory | 当前 structured bootstrap/runtime 检查、nativeAccess FIFO/准备、原 workspace creation/对账/最佳努力 shell 清理保留 |
| 专属 hostStateFeed/stateFactory/fixtureState/fixtureSample | fakeHost/fixtureOn/structuredFixture/creation/local IPC、original/boundTerminal 并发回归和旧 usage/files/stop 夹具保留 |

共删除 23 个生产函数、7 个专属测试支持函数、7 个明确非持久类型；B 代码净减 497 行（含 codec 测试调整），没有删除顶层测试。原 adapter 73 项全部保留，加 1 项 Waiting/unknown 三 codec 原字节回归，最终 74 项通过、零 skip。roundtrip 原测试继续验证原字段/Conflict 和 usage 绑定，不再调用退役计算器；新测试验证 Waiting/unknown 原字节不变。不是将保存的未知状态升级为完成。

全部 Orca 类型 AST 从 37 到 30，只少上述 7 个进程内类型，其余声明（包括 NativeProof 的字段/tag/顺序）完全相同。独立提取 A 的源码归档，旧 Profile/State/NativeProof/当前 profile/版本代表编码与 B 的 1510 字节完全相同，SHA256 `9f09b508329a0e2734ff56618371f4fb56ee98c2ec74b9e595ea5b2859c0dbe3`。另含 backend/runstate 持久声明及能力的 151520 字节通用 wire 也与 A 完全相同（上述 A 摘要）。无 schema、codec、state/v8 或迁移变更；contracts/application/backend/storage/runstate 源文件无改动。

B adapter 74、host/storage/contracts/usage/livetest 支持包 151、application 定向 13，共 238 个顶层通过、零失败；usage collect 的 TestUsageExistingNativeEvidence、TestUsageRemoteCodexCleanupLive、TestUsageRemoteCodexLive 为原有显式 opt-in skip，未计为通过。application 定向验证缺预算不阻原始结果/读/停止、原证明缺失仍等待、原操作只读对账、新输入围栏与停止优先。完整 live driver/helper 再次编译通过。

正式 B 固定 EXE 的两个公开受控顶层/七创建窗口通过、零 skip。A 收尾修正后再次以最终稳定源正式构建并复验同一范围，结果另记 final-public。所有以上均为合并前定向或受影响包检查，没有冒充完整 L1/Controlled、合并后或真实 L3。未迁移用户数据，没有恢复/重发旧 Run。

## 可复核证据

测试 JSON、公开受控结果/源码身份/日志、AST 与删除消费者清单、A/B 编码比较、复算脚本放在本目录 evidence/orca-structured-qualification-proof-cleanup/records.json.gz。每项含相对 path、原字节数、SHA256 与 UTF-8 内容；解压后按 path 定位。未打包 EXE、完整临时源码库、真实认证或用户状态。工作目录原始复算材料位于忽略目录 `.evidence/orca-qualification-cleanup/`。

实际收益：A 把当前真机开发/验证路径移出旧 TUI/helper 假设；本机精确 wire 取证属于测试设施，会增加测试支持代码，不宣称净删或运行提速。B 把历史读取范围内的旧动态证明状态机移除，后续开发无需再查旧通知/输入归属闭包。原执行、恢复、停止承诺及实际旧数据读取保持。

归档为 51 项、165193 字节，SHA256 `5aacac163982649bda322ba1eb67556ff2d6f089a042006773783d92b361d7e5`；原字节回放核对通过。最终固定 Runner SHA256 `6fd3f8a96429c510ae6771c5130424f02726d544327fd32c52d86ee2e9d7047c`，final-public 的 exit_code=0、scenario_count=2、零 skip。最终目标分支已刷新，仍为 `d58054a7`，无新增上游内容冲突。
