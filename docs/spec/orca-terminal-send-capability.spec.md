# Spec：Orca 终端发送能力 B2b

日期：2026-11-02。状态：待实施。依据[长期路线](orca-capability-layer.spec.md#6-分批实施路线)，依赖已接入的 B2a 基础构造/解码；对应 [Plan](orca-terminal-send-capability.plan.md)。调查基线 `7240c16`。

## 1. 目标和边界

把现有 text+enter、原 request ID 和可选 wait-submit 的 Orca send 参数及原生接受回执抽为能力，业务输入、原 agent-start 和固定 control 请求共用。能力只发起一次调用，返回原 Result、接受字段、阶段、provider/observation 及解析完整性；不接收 Run、Turn 或 Workflow，不解释正文、不授予重派资格、不判 Started。

能力输入采用自身的 handle/text/request/可选等待秒数。回执身份匹配使用 Orca TerminalIdentity 和原 request，不把缺字段、错 runtime/handle/incarnation/request、坏包或不确定结果升级为接受。解析完整性在本机内存生成，不能从 wire 注入。回执保留 Accepted 的缺失/false/true，接受和阶段的业务解释仍由消费者承担。

保留普通调用原 timeout 与业务显式 50 秒；远端原 profile 的 `--wait-submit 20` 仍位于 `--json` 后，其他消费者不增加等待。所有环境/前缀/屏蔽/queue 行为继续经 B2a 和既有 Client；不增加 send 重试、输入拆分、补 Enter、RPC/CLI fallback 或自动换资源。

## 2. 消费边界

| 位置 | 迁入能力并删除 | 留在执行或 Agent 侧 |
| --- | --- | --- |
| Lifecycle.submit | terminal send 参数、send/prompt 结构解码及原生身份匹配 | 原 intent/Attempted/预算、Codex/DSH 文本编码、远端等待选择、明确拒绝规则、Codex provider/observation/stages、Started/完成和原回执持久化 |
| launchAgent | 同一发送及身份解码 | 原冻结命令/LaunchAttempted、原 readiness/agent、未知不重发及后续查询 |
| dispatchControl | 同一发送及身份解码 | 原 control checkpoint、请求摘要、已有回执先读、固定 actor 同 request 机制、回执解码及用途 |

三处 MetaCall 的 terminal send 手工构造和专属 send JSON 结构必须同批删除。迁出的结构只含原生字段；不能把 Agent 的模型/输入证明、control actor 方法或业务状态写入 Orca send 能力。三方现有 callErr/CallUncertain/ErrorCode 判定分别保留，不借统一改宽或改窄合同。

profile/state/target、operation journal、creation 持久编码、业务正文及 wire 摘要均不变。本批不改变原本机 helper/控制路径，也不将 B0 可行性转为正式业务启用。

## 3. 验收

| ID | 完成条件 | 证据 |
| --- | --- | --- |
| OTS-01 | 自有输入/回执，无执行类型；完整保留缺失接受字段、原生阶段和身份 | 纯原生值/字节夹具，坏包、缺失及替换身份拒绝，wire 无法注入解析成功 |
| OTS-02 | 三个真实消费者接入、旧构造和解码删除，实际命令保持 | local/remote、默认/显式 timeout、wait-submit 顺序、中文/引号/CR 正文与 request 的精确参数捕获 |
| OTS-03 | accepted/Started 区分及原业务证明保持 | 原 NativePrompt/NativeReceipt/原输入和观察测试；含 send 的 turn_started 也不能单独授权 Codex Started |
| OTS-04 | 原拒绝、未知、撤销/队列及 control checkpoint 不退化 | 明确拒绝、缺字段/错 request/换 incarnation、响应丢失零补发、agent-start/control 原请求恢复 |
| OTS-05 | 依赖/格式/范围闭环 | 定向回归、依赖守卫、白名单及生产调用清单；实施后独立审计 |

验收使用实际 Client/ProcessHost 受控装配及原持久机制，不增加真实模型场次；发送能力与参数/接受事实的等价由夹具直接验证，不改变 B5 真 Runner 边界。
