# Plan A：Orca 冻结记录证明

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：P0–P3完成。基线 `6940c598`。依据 [Spec](orca-frozen-reader.spec.md)。

1. P0：先澄清长期组合；记录 NewLifecycle/scope/VerifyStopped/usage/files 和只读验证函数依赖，捕获原源码/测试/持久编码，审计 reader 无 mutation 入口。
2. P1：提取 frozenRecord 及原共享读取/校验；Lifecycle 用投影复用，不改现有 fixture 字段/注入合同。停止证明直接构造 reader，CLI 共享原组装和队列语义。
3. P2：新增构造/只读负例，跑现有 Orca reader、结构化、usage/files 和 host 离线回归；A 完成前不删除旧执行分支或测试。
4. P3：比较原编码和声明，写独立验收及 A 源码冻结点，记录实际限制；A 通过后再执行 B 的删除清单审计。

实际：完整adapter213顶层通过，3个原生opt-in跳过；host/storage/usage85顶层通过，4个现场opt-in跳过。原既有测试字节未改，代表编码一致；在`.evidence/orca-reader-boundary/a-source.json`固定A源码，随后才进入B。
