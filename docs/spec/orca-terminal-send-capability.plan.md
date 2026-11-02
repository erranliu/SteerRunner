# Plan：Orca 终端发送能力 B2b

日期：2026-11-02。状态：P0–P2 完成，P3 验收已记录、独立审计待完成；见[验收](../acceptance/orca-terminal-send-capability.md)。依据 [Spec](orca-terminal-send-capability.spec.md) 和[长期路线](orca-capability-layer.spec.md)。B2a 独立审计通过，无确认问题。

## P0 固定原行为

以 `7240c16` 的 submit、launchAgent、dispatchControl 三处 send 为白名单，固定完整 argv、timeout、Accepted 缺失/false/true、原始 payload 和错误处理。编码/预算及 Started 算法、control 重读/持久恢复逻辑不迁移。记录默认 MetaCall 和显式 MetaCallWithTimeout 的不同上限。

## P1 能力及接线

1. protocol/orca 新增窄 TerminalSend 值、原生 SendReceipt/PromptReceipt；编译现有 text+enter/request 参数，可选等待放在 --json 后，不处理逻辑正文。
2. Client 提供默认及显式 timeout 的发送入口，只执行一次原 MetaCall。基础 Result 原样返回；集中解码原生 send 的核心身份与完整可选字段两个既有范围，各自完整性由私有字段表达，身份匹配只接收 TerminalIdentity/原 request。完整字段仍使用原类型解码，保留重复字段先出现类型错误的事实，不用 RawMessage 最后值改写原结果。
3. submit 保留全部正文处理、意图/回执和业务判据；把远端等待选择作为输入参数，替换手工构造/结构，保持 callErr 只在原分支参与错误组合。agent-start 和 control 同批接入，保留各自现有 ErrorCode/uncertainty/错误要求。
4. 删除三处 terminal send 参数数组、三处专属 Send struct 和重复四字段匹配，不增加外层链路接口。

## P2 验证

新增 `TestOrcaTerminalSendCapability`：使用无 backend 的固定原生身份和真实 Client Output 夹具，捕获一次 argv/timeout，覆盖默认/等待20秒、特殊正文及 request；成功、Accepted false/null/missing、错 runtime/handle/incarnation/request、坏包、反向 wire 解析标记和调用失败。断言结果不确定性保留、零重试。

补充核心身份与完整可选字段的原不同范围：能力覆盖可选字段坏类型和先坏后正确的重复字段；TestAgentLaunchReceiptUsesCoreIdentity、TestFixedControlReceiptRecovery 的新例保留核心身份消费；TestNativePromptReceiptFaults 的新增双端坏类型例继续拒绝业务接受。

复用原 TestNativePrompt*、TestNativeCRLFPastePreservesOriginalInputAndCompletion、TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput、TestNativeComposerGatePreventsPrematureInput、TestNativePromptSharedQueueWithStopAndCancellation、TestFixedControl*、agent launch/workspace prepare 及原输入查询测试。先跑新能力，再按实际测试名固定正则跑原消费者与定向协议/错误/queue 回归。统计实际 roots/子例/失败/skip；不把零匹配当通过。

不扩大真实模型/多场稳定性；业务与控制成功/未知/恢复的原计数断言均保留。workflow 依赖守卫与 git diff --check 必须通过。若真实消费者接线无法由既有测试覆盖，补窄接线用例而非仅构造回执测试。

## P3 收口

写 `docs/acceptance/orca-terminal-send-capability.md`，按 OTS-01–05 列实际参数、消费者、删除清单、原回归与未验边界。同步 Spec/Plan、长期路线及索引，确认 profile/state/target/创建持久类型和业务逻辑 diff 为空。提交实现后独立只读审计一次，确认修复独立 commit 并补受影响验证；不提阶段 PR。

能力可单独测试、三个消费者全部接入、对应旧实现删除、原授权/正文/接受与 Started 边界不变且独立审计完成，才标本批完成。
