# Spec：Orca 资源创建 B4b

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：已完成，受控验证及实施后独立审计通过。依据[长期路线](../../../../spec/orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-resource-create-capability.plan.md)。调查基线 `d949e1c`。B4a 的确认问题须先独立修复。

## 目标和边界

抽出 worktree create、terminal create 的纯原生命令参数及原生回执解码，实际接入 workspace 创建/恢复、initial terminal 和 control terminal。Orca 能力只接受明确 selector、原生选项和 Result，不接收 Run、WorkspaceIntent、creationKey/Plan 或重派资格，不在能力内启动第二套创建 owner。

固定命名、base、是否 bare/agent、setup/no-parent、初始/控制终端标题及 command 由原消费者明确提供；能力保持参数顺序和字节，不添加隐式 --activate、默认 agent 或 fallback。native WorktreeCreation 只表达 worktree/agentTerminalHandle/startupTerminal，TerminalCreation 只表达 terminal。调用错误、不确定性和 runtime 仍在原 Result，解码不把成功 JSON 升级为可信资源或任务开始。

原 creationCall/ServeCreationOwner 继续独占冻结、命令/镜像摘要、排队、marker、派发、deadline、持久回执和对账。它们消费能力生成的原 argv 与 B2a 编译/解码；未知创建从原计划/回执恢复，零重发、零替换、零 inventory 认领。没有新直接 create 方法绕过 owner。

## 迁移和格式

workspaceCreationArgs 保留执行策略，委托 WorktreeCreateArgs。initialWorkspaceTerminalRecovery 与 prepareControl 委托 TerminalCreateArgs/DecodeTerminalCreation，原身份/错误/确认分支保持。workspace 原生创建与恢复共用一份 decodeWorkspaceCreation 映射；原生字段由能力解码，原 checkpoint 的 runtime_id/resolved 身份/selected_terminal 兼容解码仍归执行侧，不能把这些业务字段移进能力包。

workspaceCreation、initialTerminalCheckpoint、creationPlan/Result/Started、Profile/State/Target 的字段、版本、JSON 顺序和摘要保持。原生包不新增执行依赖；删除三处创建 argv 和原生字段专属解析。helper/control 在 B6 之前仍是现行消费者，本批只复用操作，不给它们建立长期框架。

## 验收

| ID | 条件 | 直接证据 |
| --- | --- | --- |
| ORC-01 | 无执行快照的原生参数/回执能力 | 完整旧 argv oracle，bare/agent、空格/引号/中文、独立 slice；原生完整/坏字段/缺失/额外字段和 Result 错误/不确定事实 |
| ORC-02 | 全部实际创建消费者接入且旧构造/解码删除 | 双端 workspace/initial/control 原真实构造，字段范围/身份不增权，原 checkpoint JSON 和命令摘要等价 |
| ORC-03 | 可靠派发保持 | 原 owner caller-loss、delayed/missing receipt、foreign/malformed、deadline、initial/control 恢复；未知和对账零重建 |
| ORC-04 | 验证与审计闭环 | 受控测试实际命中、依赖/持久白名单、直接证据及提交后独立审计 |

本批保持原 wire 和执行合同，采用受控等价验证，不新增真实资源/模型或 B5 资格。

实施与旧实现删除、实际测试命中及范围见[验收](../acceptance/orca-resource-create-capability.md)。
