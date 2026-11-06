# Plan：Orca 终端等待能力 B2d

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：P0–P3 完成，独立审计未发现确认的 P0/P1/P2；见[验收](../acceptance/orca-terminal-wait-capability.md)。依据 [Spec](orca-terminal-wait-capability.spec.md) 和[长期设计](../../../../spec/orca-capability-layer.spec.md)。B2c 独立审计通过，无确认问题。

## P0 固定唯一生产入口

核对 native_start.go/waitOriginalComposer：原 profile 期限、wire 毫秒和外层预算、两次 original、原判断与调用条件。白名单仅新增协议能力、替换该函数的参数/结构，测试/文档另列。不修改同文件的原输入恢复算法。

## P1 能力与接线

1. 定义 TerminalWait(handle/condition/timeout-ms)、TerminalWaitReceipt(satisfied/本地解码完整性)，Client.WaitTerminal 经一次原 MetaCallWithTimeout 调用；缺输入/负 wire 时间拒绝，0ms 原样保留。
2. wire 参数顺序仍为 terminal wait、--terminal、--for、--timeout-ms、--json；进程 timeout 由消费方提供，不在能力内推导。保留 Result 和原本未消费的扩展，不做新的可选字段类型限制。
3. waitOriginalComposer 继续计算 budget.Milliseconds() 和 budget+time.Second，前后 original 及所有原拒绝/取消规则保持，仅替换 wait 构造/JSON 解码。原调用条件及 Started/完成算法不动。

## P2 验证

新增 TestOrcaTerminalWaitCapability：真实 Client Output、纯原生值，精确 argv/调用上限，0ms、satisfied 真/假/缺/null/坏类型、坏包、原生错误/传输丢失、重复字段、额外无关字段及缺输入零调用/零重试。

新增 TestComposerWaitKeepsOriginalFences：真实 NewLifecycle 与原 IPC 夹具，原终端前后两次 RPC，前置替换零 wait、后置替换拒绝、错 runtime/负面/坏等待/取消保持。复用原 TestNativeComposerGatePreventsPrematureInput、TestLocalCodexBootstrapWaitsForOriginalComposer、TestNativeRemoteSubmissionWaitsForOriginalStart、NativePrompt/未开始原输入及 StateCodec 定向测试，原 send 计数保持。

go test -json 记录实际 roots/子例/fail/skip；先新功能再必要回归，workflow 依赖守卫、git diff --check 及唯一旧 wait argv/结构零残留。需要时正常权限运行原进程夹具，不运行真实 Orca/模型。

## P3 收口

按 OTW-01–04 写 docs/acceptance/orca-terminal-wait-capability.md，列唯一生产入口、双端调用条件、删除清单、直接证据和未验范围。同步长期路线及文档索引，提交实现后独立只读审计一次；确认修复另行 commit，不提阶段 PR。全部闭环才标完成。
