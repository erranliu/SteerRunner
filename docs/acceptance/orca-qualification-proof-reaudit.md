# Orca 结构化验收与旧证明删除：提 PR 前复审

日期：2026-11-06。复审原实现 `c7a1982d`，整合最新 main `5b5487b5`（#122）；正常 merge 无内容冲突，没有重写已验证历史。原设计和实施见 [A/B 验收](orca-structured-qualification-proof-cleanup.md) 及其 Spec/Plan。本记录描述本次合并前检查，不继承为真实 L3 或发布资格。

## 确认并修复的三项问题

1. **测试读取器错误拒绝合法未创建停止。** 工作区已创建、Agent create 未派发时，原 envelope 保存确定的 session ID，而公开 RunView.session_id 合法为空。原 ReadStructuredRun 在读原状态前强求公开 ID，因而拒绝生产 VerifyStopped 已认可的停止。现保留原 envelope/runtime/worktree 绑定，仅对 stopped、已撤销新输入且原 session_not_created 证明通过生产只读 VerifyStopped 的情况接受空公开 ID；已创建的原 close 成功也沿同一证明核验，未知/失败/缺原证明仍失败。复现先通过生产 VerifyStopped 再在原读取器失败；修复后覆盖未派发、已尝试、仍 pending、仍允许输入、缺 absence/close attempt、已创建 close 成功/未知/失败。夹具显式区分生产 codec 的“缺失 close_receipt”和 JSON null，没有放宽生产 decoder。
2. **空或无操作字段的 CLI 日志能误报无旧调用。** 原 candidateCallAudit 对空文件、`{}` 行或缺 args 行没有拒绝。现由测试支持层集中核对两种真实 helper 日志（intent/receipt 与 dispatch/result）、原参数/结果配对、行完整性及至少一个完整调用，拒绝旧 terminal create/send/helper 控制调用。活动快照允许尚未返回的已记录调用；native candidate 在服务退出后要求原日志完整。该日志只证明 CLI 路由，不拿它计数 IPC/远端结构化输入。正反例覆盖空、截断、无操作、外来/失配回执、未结束与合法失败读取；没有修改共享 stability 日志格式。
3. **实际本机 create 目标复核不足。** 原 VerifyStructuredInputCalls 只比 session、正文和次数；同正文/同 session、有效指纹但错误 worktree/agent 的请求也能通过。复现此形状后，补上原 worktree、agent、当前权限选项、完整 create 指纹，以及 send 的完整指纹/正 fence；原操作 ID、唯一输入及原正文仍核对。修改只在测试取证侧，未扩展产品执行权限或创建新持久字段。

补充真实接线回归：在原 TestSRResourceCreationRecovery 的共享夹具中增加一个读取断言，让上述读取器直接消费公开 EXE 产生的实际 RunRecord/View。七个原窗口全部保留：四个成功恢复/未派发停止的实际记录经新读取器通过，丢创建响应、owner crash、缺原 plan 三个窗口继续保持未知且零新输入。这不是再建一套模拟执行器；共享原字段/故障/原 RPC 计数断言全部保留。

## 当前结构化入口与消费者复核

config/fixture/Manifest/driver/结果守卫贯通当前结构化协议；新 Run 仍由完整生产准入拒绝旧 helper/launch executable/args。registered native-development/native-active-stop、Codex/Pi feedback 和通用 smoke/multirun 的活动/历史/恢复/停止检查不再解析 orca-terminal ID 或读取旧 readiness/owner/Job。本机仅 Runner 子进程读取测试 IPC metadata，原 CLI 继续用原用户配置；IPC 原样转发并保留原认证/运行时响应，凭据不进入 journal。远端保留原配对 host key，以原完整历史/原操作说明输入语义，不伪装实际 wire 次数。

历史原操作/结果对账和 close 成功/未知的边界保持：缺证明仍等待，不重发业务输入/close，不将停止扩大为宿主 Job/任意派生进程退出。新读取器没有执行构造或业务派发；go list -deps 的 Runner 生产依赖不含 livetest，测试 proxy 未进入产品。

B 的删除闭包重新核对：只有 23 个旧计算/不可达 preflight 生产函数、7 个独占 feed 夹具函数与7个非持久类型；NativeProof.agentName、严格 codec/矛盾拒绝、旧 scope/usage/files/inspect 仍有真实消费者并保持。fakeHost/fixtureOn/structuredFixture/creation/local IPC、original/boundTerminal 并发检查和协议共享 StateFeed 均保留，原 adapter 73 个顶层测试未删，增加的 Waiting/unknown 原字节回归使当前为74项。

没有新增或删除注册 case；execution-stability 旧终端故障夹具仍是明确未迁完的边界，本机 Orca audited 仍无注册真机 case。本次不把这些缺口混入当前通过资格。

## 格式与最终验证

- 与独立目标 main `5b5487b5` 源码归档相比，Orca 类型 AST 从37到30，仅少7个上述非持久类型，其余声明/字段/tag/顺序完全一致。数据库/schema/version、backend/runstate/contracts/application/直接 Codex 的本批差异为空。
- 旧 NativeProof/State/Profile 和当前 profile/版本的代表编码1510字节完全相同，SHA256 `9f09b508329a0e2734ff56618371f4fb56ee98c2ec74b9e595ea5b2859c0dbe3`。通用持久 wire/能力/声明174727字节也与同一目标main相同，SHA256 `ac66a49beebf12d71f2e3352a94f91674768b2007bfd5b78dbb175a62d38df6a`。与上一轮较早 main 的摘要不同只因 #122 新增配置能力，未把上游变化误算为本批格式变化。
- 受影响包335个顶层通过、零失败；application 原预算/缺证明/原操作/停止边界定向13项通过。最终 livetest49项全包再验通过（已包含在335项中，不重复累计）。10个既有真实用量/现场/清理 opt-in 为 skip，完整名单保存于 test-summary.json，没有计作通过。
- 正式 build.ps1 固定同一 EXE；正式 test-foundation 定向5顶层全部通过：TestSRResourceCreationRecovery（七窗口）、TestSRSingleExecutionColdStop、TestSRExecutionConfigurationPublicCLI、TestSRNativeCatalogPublicCLI、TestSRRestartPreservesAuthorization（四边界）；exit_code=0、scenario_count=5、零skip。新增实际记录读取断言后，仅对受影响的创建恢复七窗口补验，原同一 EXE 再通过、scenario_count=1、零skip。
- 全部 live driver、保留的 stability helper 编译通过；native-result（Windows PowerShell5.1）及catalog框架通过。未运行完整L1/Controlled、实际模型/Orca L3、合并后测试或部署。首轮回归刻意复现旧错误，以及新夹具 null/缺失字节的校正，不作为通过资格。

最终固定 EXE SHA256 `82c4c9f14821a69e91037f1846e582ff666d52918d856de3df82fad8580ebb74`。所有公开受控复验使用该同一产物。

复算脚本、原始 JSON/日志、公开受控源码身份与结果、原编码/类型对照，保存在 [证据归档](evidence/orca-qualification-proof-reaudit/records.json.gz)；每项含相对路径、原字节数、SHA256、UTF-8内容。不含EXE、用户运行状态、真实认证或整份临时源码库。原始忽略目录为 `.evidence/orca-qualification-reaudit/`。

归档共36项，166484字节，SHA256 `c204f8999fbd0716a8c37b4d5482ee35cc250d61076ff2f839d4030bb6ac91b9`，原字节回放验证通过。最终稳定源重建的 EXE 与上述已测产物完全相同。
