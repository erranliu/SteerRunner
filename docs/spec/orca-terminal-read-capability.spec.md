# Spec：Orca 终端读取能力 B2c

日期：2026-11-02。状态：待实施。依据[长期路线](orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-terminal-read-capability.plan.md)。调查基线 `44a3a2c`，消费 B2a 的既有命令/Result 能力和 B1 原身份查询。

## 1. 目标

提供与链路无关的单页 terminal read 能力，输入自有 handle、明确 cursor 和正 limit，返回原始行、source、oldest/next/latest cursor 字段、truncated/limited 的缺失或值、原 runtime 及解析完整性。能力不自动翻页、trim、丢弃部分行或解释控制前缀，不认领任何 readiness/history/业务回执。

controlLines 和 readLocalCodexReadiness 同批接入，删除各自的 read 参数与 envelope/page 结构。控制消费者保留每页读前/读后的 B1 原 incarnation 围栏、保留窗口/首个 latest 边界、完整行和分页推进规则；readiness 消费方保留从 cursor=0、不可截断/limited、唯一原准备回执及只在原初始化阶段读取的规则。

本批不为 readiness 添加或删除身份查询，不把所有围栏搬进 read。CLI read 只有原 runtime/handle 事实，不伪造 incarnation 字段；原 incarnation 由现有外层查询核对。读操作仍经既有 Client/明确目标/CLI queue，只调用一次，不增加 fallback、重试、锁或回执缓存。

## 2. 原字段范围和责任

两方都消费 handle/source/oldest/tail/truncated/limited；control 另要求 next/latest 的字符串类型有效。共享读取分别记录基础页和完整分页字段的解析完整性，readiness 仍忽略它原先未消费的 next/latest 类型；control 不能用最后一个重复字段覆盖已知类型错误。两个标记均由本地解码生成，不能从 wire 注入。

能力不把缺失的 bool/cursor 补成可信默认值；保留原字段后由消费者按原用途拒绝。cursor 的数值范围、源自 stream、行数/limit、增长/回退以及第一次读取边界规则保持在已有消费算法，不能用共用“读取成功”替代。readiness 中的 helper 校验、前缀解析、失败码、workspace/配置/模型准备及停止 scope 不迁入 Orca 包。

新文件无 backend、Codex、Workflow 或可靠执行依赖。profile/state/target、原操作及创建持久格式不变，旧 helper 路径本批不退役，不改变 B5 资格。

## 3. 验收

| ID | 完成条件 | 证据 |
| --- | --- | --- |
| OTR-01 | 自有单页输入/结果；原始行/cursor/bool/来源完整保留，缺 handle/limit 零调用 | 原生值/字节夹具、精确 argv/timeout、错误和不确定事实 |
| OTR-02 | 两个真实消费者迁入、旧参数/结构删除，原不同字段范围保持 | readiness 可选游标坏类型仍忽略，控制同形态拒绝；缺字段/重复错误不能伪造完整页 |
| OTR-03 | control 每页前后原身份核验及分页完整行规则保持 | B1 双页四处替换与 queue 测试，retained stream/boundaries/faults |
| OTR-04 | 原 readiness、actor history/工作区/停止及未知恢复不退化 | 原初始化/唯一性/截断测试，FixedControl 真实消费者定向回归及发送计数 |
| OTR-05 | 范围/依赖/编码与验收闭环 | 生产白名单、依赖守卫、实际测试和实施后独立审计 |

本批通过原字节及实际消费者的受控验证完成，不重复模型或制造真实终端输出故障；不将历史现场推定为本批真实新资格。
