# 结构化状态读取收口与多轮文件读取修复

2026-11-09，合并前定向验证。基线 main `827ad83bf0785bbcd32ac5775c311f1a981ae6a5`（已合入 #129）。依据用户确认的收窄建议实施 A；B 的旧终端测试迁移及 C 的无引用声明清理暂缓。本项不扩大历史格式、执行组合或停止 scope。

## 问题与合法入口

当前执行、冻结证明、文件通道和 usage 原先通过完整或手工部分装配的 frozenRecord 读取结构化状态。fileSource 只取出 Profile.Transport，没有带入 Development；agentState 却使用它判断是否允许上一轮状态与下一条 PendingTurn 共存。usage 显式带入该事实，文件路径遗漏，形成语义分叉。

先增加真实隔离 named-pipe 测试 TestStructuredFilesWhileNextInputQueued：使用现有结构化身份及 envelope、合法 development profile，保留原工作区、runtime/session/instance 和上一条命令的原输入绑定，排入新的 PendingTurn；分别打开原 progress/result 文件能力。生产未改时，合法 development-queued 返回 binding_invalid，另外五个拒绝场景通过，见[基线复验](evidence/orca-structured-state-read-20261109/before-recheck.jsonl.gz)。修复后同一反例读取原 RPC 字节成功，execute 的额外命令、同命令改正文、错资源、错 instance 和坏状态仍在 RPC 前拒绝。

测试同时核对实际 NewLifecycle 构造及其状态读取与原绑定一致。两个低层文件能力可读取原字节不代表公开 ResultFile 已取得终态/封存资格；应用的原完成、停止、封存和通道故障不阻业务规则未修改。

## 收口范围与保留项

新增三个包内纯函数：validateSnapshotBinding、validateStructuredSnapshot、readAgentSessionState。原结构化校验和通用绑定校验迁入，无 host、probe、observer、时钟、缓存或持久状态。Development 是显式参数，不能在半初始化 reader 中隐式丢失。

| 原消费者 | 实施结果 |
| --- | --- |
| Lifecycle.validateStructured/agentState | 直接调用纯校验，不再为状态读取转换成 frozenRecord |
| fileSource | 结构化分支只解码一次完整 profile，将 Transport/Development 明确传入；旧协议沿原 decodeLifecycleProfile，外部错误仍为 binding_invalid |
| StructuredUsageBinding | 直接使用同一纯读取函数，保留原 IDENTITY_UNVERIFIED/IDENTITY_CHANGED 等错误与原输入/来源核验 |
| frozenRecord 的状态/停止读取 | 原方法转发到同一纯校验；保留旧 state、owner/receipt/Job、历史严格解码及实际消费者 |

全生产 AST 对照只有七个原函数正文变化、三个纯函数新增，2,246个其它声明相同；无类型/tag/常量/codec 删除或修改。[声明差异](evidence/orca-structured-state-read-20261109/final-declaration-diff.json)与[逻辑等价对照](evidence/orca-structured-state-read-20261109/logic-equivalence.json)证明，三段校验正文经显式参数/调用替换后与原实现逐字相同。业务行为修复来自文件路径终于传入原 Development，而不是删除身份、输入或停止校验。

NewLifecycle/decodeFrozenRecord、decodeAgentSessionProfile、closeSettlesAgentSession/verifyAgentSessionStopped 原正文保持，构造器的错误类型/优先级及旧 profile 版本含义保持。原始查询/订阅、Lifecycle.original/boundTerminal、capabilityFixtureLifecycle、全部既有共享测试、五个 JS、Node/L1 与诊断脚本均保持。[范围审计](evidence/orca-structured-state-read-20261109/scope-audit.json)。生产代码本批为少量净增加，收益是共享校验职责和修复遗漏事实，不宣称净删或提速。

## 格式与可靠性

原 Profile/State/NativeProof、Record/Operation、结构化状态与 RuntimeFence 的独立捕获前后均为15,528字节，SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`。原状态/Observation 缺失与显式零/owner/request/receipt/能力/资源/资产捕获均为158,875字节，SHA256=`f5d69cd15df16ef2ba029b137e4fc6caea774aadf11565963dd0813254d78b62`。两组原字节分别在改生产前后实际运行并比较，序列化错误检查，不使用当前 hash 回填旧证据。

state/v8、user_version=8、全部持久格式、原错误/身份拒绝及 PC-R01–R10 保持。原受理/提交、unknown 对账与零重放、缺预算只阻新派发、同构建恢复、停止撤销和原 scope、结果归属/封存均不变；原 close 缺回执仍未知，不把可选通道读取当作完成或停止证明。

## 实际验证

| 范围 | 结果 |
| --- | --- |
| 完整 orcatransport | 82个顶层、358个执行节点通过，0失败/skip；包含新增6场景及原历史/构造/停止反例 |
| 完整 protocol/orca | 62个顶层、360个执行节点通过；原4个现场 opt-in 跳过，0失败 |
| application 执行/多轮/文件/预算/恢复/停止定向 | 103个顶层、255个执行节点通过，0失败/skip |
| host/storage/contracts/usage collect/testkit livetest 五包 | 135个顶层通过，原3个 usage 现场 opt-in 跳过，0失败 |
| live native driver、native/stability helpers | 分别编译通过；未运行真实模型 |
| 同一最终正式 EXE 的公开受控 | SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 三项通过；四个重启窗口均通过，最终 exit_code=0、scenario_count=3 |

组件命令为 `go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1 -timeout=5m -json` 及上述共享五包。application 使用 `-run '^(Test.*(Stop|Scope|Structured|Message|Recover|Budget|ResultFile|ResultSnapshot|SingleExecution|Generation|ManualReconciliation|Observation|Frozen).*)$' -count=1 -timeout=15m -json`。结果及原跳过名称见[完整明细](evidence/orca-structured-state-read-20261109/summary.json)。

正式构建 `scripts/build.ps1 -OutFile .evidence/structured-state-read/final/steerrunner.exe`；经 `scripts/test-foundation.ps1 -RunnerExe <同文件> -Pattern '^(TestSRSingleExecutionColdStop|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir <全新目录>` 启动独立受控宿主，等待原最终 result.json/integration.log 后判定，不以启动脚本返回代替完成。[原公开结果](evidence/orca-structured-state-read-20261109/controlled-result.json)。

构建 source_digest=`4f7a2d48ff8a4d1a001796886919312a07451b1d9100a447f6a4cc0c951b1275`，EXE SHA256=`76c888a5d27da028c8a91bcb558103bb41fb785b44091db54ecf2634c0d35373`，source_commit 为原基线、source_dirty=true。最终源码/测试清单 SHA256=`3553462ad88589b2e9c096e912b38088f4fbaf0da09ed9834919d6c34ae649f2`；本工作副本 `.evidence/structured-state-read/final/source.zip` 保存可恢复源码归档，其摘要见 summary。

首次新测试夹具直接比较未经规范化的 Setup 与 Clone，因 nil/空表示差异导致五个负例的“不变”断言误报；对输入先做一次既有 Clone 规范化后，生产未改时只有合法多轮读取失败，五个拒绝反例通过。[首次日志](evidence/orca-structured-state-read-20261109/before.jsonl.gz)保留，未改原 Clone 或放宽生产校验。最终测试还增加当前构造/读取一致性核对，完整包在最终冻结源码实际通过。

本项为合并前定向组件与公开受控资格。未执行完整 L1/Controlled、L2/L3、真实模型或 Node（JS/Node入口均未改），不登记相应新增资格；未自动合并、部署或发布。证据目录在暂存前设置原字节保护，直接复核 Git blob 的 SHA 清单。
