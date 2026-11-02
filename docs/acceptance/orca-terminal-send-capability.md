# Orca 终端发送能力 B2b 验收

日期：2026-11-02。状态：实现与定向验证通过，待实施后独立审计。依据 [Spec](../spec/orca-terminal-send-capability.spec.md) / [Plan](../spec/orca-terminal-send-capability.plan.md)，本批不改变生产支持或授予 B5 资格。

## 1. 能力与消费

新增 protocol/orca/terminal_send.go 使用 Orca 自有 TerminalSend、TerminalSendReceipt、TerminalPromptReceipt 和 TerminalIdentity。Client.SendTerminal/SendTerminalWithTimeout 只进行一次现有 MetaCall，保留原 Result/payload/runtime/调用错误/不确定性。缺 handle/request 在调用前拒绝。纯协议文件不导入 backend、Codex、可靠执行或工作流。

submit、launchAgent、dispatchControl 均接入新能力，三处手工 send argv、专属 Send JSON 结构和四字段身份重复核对已删除。业务输入仍使用 50 秒，原远端 profile 的 wait-submit=20 仍在 --json 后；agent-start/control 沿原默认上限。正文及 Codex/DSH 编码、预算、Intent/Attempted、原始回执、control journal/恢复、provider/observation/stages 和 Started 判据均留在消费者。

解码分别记录核心身份与完整 prompt 字段的本地完整性。业务原来要求所有阶段字段类型有效，继续同时检查两种完整性；agent-start/control 原来只要求核心身份，继续忽略无关可选字段的类型。完整类型解码保留重复字段先前出现的类型错误，不用最后的 RawMessage 值抹除反证。两个完整性标记及 envelope runtime 均不能从 wire 注入；Accepted 仍保留 nil/false/true。

本批没有改动 profile/state/target/创建持久声明、输入编码算法、完成证明、旧停止合同或现有队列实现。共享 send 不把 accepted 或 turn_started 阶段转为 Codex 业务 Started，不增加补发、补 Enter 或目标回退。

## 2. 验证与证据

源码基线 `441b3b9` 加本批差异，source_digest 为 `5489e9bf5aaca4c85204cf91af635e60f59ce94fbbea50dd1511f4bc1d9ae546`，沿既有 Get-SRSourceIdentity 范围。原始 test2json、实际命令及源码身份见[证据目录](evidence/orca-terminal-send-capability-20261102/README.md)。

| 检查 | 实际结果 |
| --- | --- |
| 最终字段范围 | 4 根/71 子例通过；包括能力 17 子例、双端业务错误字段拒绝、双端 agent-start 只认核心、control 可选坏字段仍按原规则接受 |
| 最终消费方定向 | 37 根/222 子例通过；NativePrompt、原输入、CRLF、未开始回执、Composer、FixedControl、原 agent-start、StateCodec 及 wire budget |
| 基础协议回归 | 4 根/11 子例通过；B2a 构造/解码、未调用 queue 与 FIFO |
| 最终依赖守卫 | 1 根通过；git diff --check 及生产白名单检查通过 |

以上最终记录均为 0 fail/skip，字段范围与消费方集有重叠，不相加成独立案例数。能力夹具捕获完整 argv 和默认/显式 timeout，中文、引号和 CR 文本原样传入；接受缺失/false、四种身份替换、坏包、部分类型错误、响应丢失及原生拒绝保持原事实且实际调用仅一次。缺输入的两个入口各零调用，wire 不能注入解析标记。

业务原故障测试新增阶段/provider/observation 类型错误的双端反例，保留原 unknown、重建观察及仅一次 send 断言。agent-start 新例和 control 原回执恢复新例证明它们没有获得额外的业务阶段要求。原 send 的 turn_started 提示仍不能单独证明 Codex Started。

初次消费方检查因 control_checkpoint.go 漏加 native import 编译失败，未运行测试；补齐后通过。中间能力/消费方通过记录保留，最终资格只使用字段范围和 consumers-qualified 日志。没有修改既有断言来掩盖失败。

未创建真实 Orca 资源或启动模型；本批通过固定原参数与原生字节、实际消费接线及原持久夹具验证等价，不将 B0/B1 的真实现场转记为本批新现场。

## 3. 审计

实现提交后独立只读审计一次；确认修复单独 commit 并补受影响验证，该步骤完成前 B2b 不标完成。
