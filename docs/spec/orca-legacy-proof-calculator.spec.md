# Spec B：退役旧 Orca 证明计算器

日期：2026-11-06。基线为 A 独立通过点。状态：设计，实施前审计。配套 [Plan](orca-legacy-proof-calculator.plan.md)。不修改长期能力/可靠性承诺。

## 入口与消费者审计

NewLifecycle、host 启动、application.port 均不接受旧执行协议。旧 NativeProof.query/end/attribution/nativeInput 链没有当前生产执行根；但 state_codec_test 的 roundtrip 仍调用 query，native_fixture_test.fixtureState 支持这一调用，不能称完全无消费者。该测试实际验证已经保存的 Conflict 不可丢失：query 在 Conflict 非空时立即返回，未走重算。应保留编码、Conflict/Waiting、usage 原绑定断言，改为直接核验已存事实，不能为删函数而删此回归。

## 删除与保留闭包

删除 native_state.go 中除 NativeProof 声明和 agentName 之外无生产消费者的旧计算方法/preview；删除 native_observation.go 的 provenance/attribution/nativeObservation；删除 original_input.go 的进程内 nativeInput/originalInputQuery/匹配器；删除 orca_state.go 的无消费者 stateFeedFactory；删除仅互相引用的 hostStateFeed/stateFactory/fixtureSample，保留仍被测试使用的函数。workspaceRepository 的不可达 !structured hook preflight 与独占 verifyLocalObservation 可在证明闭包后删除；nativeAccess 仍用于 FIFO/当前准备，不得整标志删除。

保留 NativeProof 全部字段/type/tag/顺序，agentName 的 frozen_record 实际消费者；validateNativeProofFlags、validateNativeTerminalState、历史严格解码、原 Conflict/Waiting、unknown/原 scope/usage/files/progress/inspect。保留 original/boundTerminal 与并发回归、fakeHost/fixtureOn/structuredFixture/creation/local IPC 等共享夹具。协议层 StateFeed/AgentState/RPC 的其他实际消费者不在本批删除范围；嵌入资产、TerminalRuntimeMetadata 和旧 owner/receipt 类型也不连带删除。

## 收益与最小验收

收益是 historical reader 不再带着旧通知/补证/输入归属状态机供开发搜索；不是降低实时开销，也不是减少旧格式兼容性。state/v8、user_version、全部生产持久声明/tag/codec、请求与回执原字节保持，不迁移、不回填、不把 unknown 当 success，不制造第二旧执行器。

- B01：逐函数/类型全仓消费者清单可审；删除闭包不包含任何实际 reader/当前调用或共享夹具。
- B02：原 codec roundtrip、Conflict 与 Waiting、usage 原绑定及缺失/篡改拒绝，原 original/boundTerminal 并发回归保留；编码前后摘要相同。
- B03：adapter/host/storage/contracts/usage 相关回归与正式公开受控执行/恢复/停止通过；原缺证明仍等待，预算缺失只阻新派发、stop 后零新派发、unknown 不重放均保持。
- B04：A 通过点到 B 无生产持久类型差异；记录实际删除数及所有失败/skip。未执行完整 L1/Controlled 或真机不得宣称相应资格。
