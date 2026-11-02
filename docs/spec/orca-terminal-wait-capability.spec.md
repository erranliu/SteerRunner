# Spec：Orca 终端等待能力 B2d

日期：2026-11-02。状态：已实现、定向回归通过，待实施后独立审计；见[验收](../acceptance/orca-terminal-wait-capability.md)。依据[长期路线](orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-terminal-wait-capability.plan.md)。调查基线 `d26be92`。

## 1. 目标及责任

抽取原 terminal wait 的原生参数和等待结果，让已有双端 waitOriginalComposer 消费 Orca 能力。该函数当前已经共用，本批只有这一处生产协议实现需要迁移，不虚构两个重复等待引擎或另建链路接口。

能力输入为自有 handle、明确 condition、原 wire timeout-ms，并由调用者单独提供进程调用上限；一次原 MetaCallWithTimeout 返回原 Result 和原生 satisfied 的类型解码事实。能力不自动等待到业务完成、不解释 Agent、不建立新期限或 retry。缺 handle/condition、负 wire timeout 在调用前拒绝；明确 0 毫秒保留，不能把现行亚毫秒 profile 的转换改成新准入拒绝。

waitOriginalComposer 保留 profile WaitTimeout 解析、Milliseconds 转换、外层 budget+1s、前后两次 original 身份查询、原 runtime/uncertainty/ErrorCode 与 satisfied=true 判据。读取异常或负面结果不进行后查询及业务 send；前后原 incarnation 不符仍拒绝。原 CLI wait 不返回被当前合同核验的 handle/incarnation，不补造该事实。

调用条件仍由原 submit 决定：现行远端原生 Codex 及本机 usesControl 的路径按原准备流程等待；不把未接入的其他组合强行改成等待，也不提前启用本机无 helper 业务路径。就绪、accepted、Started 和完成仍是不同事实。

## 2. 范围和删除

新增能力文件无 backend、Codex、Workflow 或可靠执行依赖。仅删除 native_start.go 中 wait 参数及专属 Wait JSON 结构；原 queryCompletedOriginalStart、Started/原输入证明、profile/state/target、持久类型、正文、队列及停止均不变。

不将未消费的 blockedReason 等原生可选扩展变成新强类型前置；原始 payload 保留供需要时读取。缺失/false/null satisfied 不能证明就绪，原解析和重复字段类型错误保持，wire 不能注入解码成功。

## 3. 验收

| ID | 完成条件 | 证据 |
| --- | --- | --- |
| OTW-01 | 自有值、一次调用，原 args/timeout/原生结果保持 | 无执行快照的协议夹具；0ms、缺输入零调用、坏包/拒绝/丢响应及解码标记 |
| OTW-02 | 实际 waitOriginalComposer 接入，旧构造/结构删除 | 原本机 bootstrap 与远端 composer tests，精确 timeout-ms/budget+1s |
| OTW-03 | 原终端前后围栏、runtime/取消/负面等待后零输入保持 | 新 RPC 围栏夹具，原 busy/permission/timeout/no-send 断言 |
| OTW-04 | readiness 不升级为 Started/完成，无新增重试或持久变化 | 原 NativeRemoteSubmission/NativePrompt/未开始输入/StateCodec 回归、依赖守卫及实施后独立审计 |

受控层直接验证该纯迁移，不新增模型场次，不平移 B0/B1 现场为新资格，B5 边界不变。
