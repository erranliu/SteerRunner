# EAC 2026-11-02 专项证据

结论、原提交边界与资格见[验收](../../event-ack-cleanup.md)。所有 JSONL/gzip 保存原字节，不把首轮夹具失败改判成通过。

- design-audit / consumers-before/after / consumer-audit：生产编辑前的设计/文档摘要、三类确认含义、全部实际夹具、trace/底层问答/旧环境消费者及最终保留实现核对。九份原夹具仅去第二回复，六个关键生产消费者原字节不变（换行规范化比较）。
- wire-audit.go、wire-before/after.json.gz、wire-comparison：固定持久类型/代表性编码、公开能力和资源摘要，145116字节相同；需与持久根审计一起阅读。
- declarations.go/json：基线72b589b和 dirty 实现的生产声明比较，177相同/9授权调整/7删除/0新增；52个保留类型相同。工具以基线为 HEAD 复跑，测试单独审计；allowlist 不代替行为证明。
- ack-application/ack-observer、components/application-final、test-summary、commands：真实确认边界、错误/取消、新回归与整组、精确名称/数量/skip/无测试包及命令/退出码；focused 不与整组重复累加。
- ack-observer-first-failure / first-failure-note：新用例初次使用零快照的失败，保留实际校验，改为合法夹具后通过；无生产规则放宽。
- production-diff：A/B生产分别净减38/67行，LifecycleResolver四行归B单列，避免混算；新增行为测试另计。
- source-identity / runner-build / foundation：680项最终输入、三份产物 SHA；三项公开场景和原命令/原生调用/清理结果。无 EXE、数据库或用户现场。
- SHA256SUMS：除自身外本目录递归摘要。临时完整产物在 .evidence/event-ack-cleanup，历史独立模块和旧资格保持原字节。
