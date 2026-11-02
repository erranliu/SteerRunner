# HAC 2026-11-02 专项证据

范围和结论见[验收](../../history-adapter-cleanup.md)。gzip 保存原始字节，文件名不替代退出状态；初始审计遗漏、两轮编译失败和范围修订均保留。

- design-audit / consumers-before / consumers-after / implementation-audit：设计先于生产编辑、当前合法入口盘点和最终共享边界。initial design 的文档摘要对应最初版本，不改写后续遗漏。
- wire-audit.go、wire-before/after.json.gz、wire-comparison：含 HistoryBinding 的固定编码/选定类型、公开 capabilities 和资源摘要，前后 140590 字节相同。
- retained-audit.go/json：三个共享文件的 40 个保留声明相同（16 个类型），4 个 phase 声明和 64 KiB 常量原样迁址。复跑时以基线 e2bc0a5 为 HEAD，并将三个基线文件从 git show 恢复到 .evidence/history-adapter-cleanup/base/ 对应路径；工具还从基线读取已退役文件。
- fixture-audit / original-shared-fixture：原 shared helper 和保留 recovery_history_test 的正文一致；夹具不运行旧 mock。
- components/application-first-failure、components-second-failure 及两份 failure-note：首轮 phase 值遗漏和二轮共享夹具遗漏，原 JSON 诊断完整保存。first wire 摘记不冒充完整原始 stdout。
- components/application-final、test-summary、commands：最终命令、精确顶层名称、子用例、skip/无测试包和退出码；重试不累加。
- source-identity / production-diff / runner-build：685 项源码/构建输入、三份二进制 SHA、分项真实生产增删；不把 HEAD 误当本次实现。
- artifact-binding / build-before-fixture-move / foundation：公开两场景使用的 Runner 与迁出测试夹具后正式重建产物 SHA 相同，生产未再变；明确两次 source_digest，不虚称场景重跑。包含原命令/原生调用和成功清理结果，不含 EXE、数据库或用户现场。
- SHA256SUMS：除自身外的本目录递归摘要。完整临时日志和产物位于 .evidence/history-adapter-cleanup；历史独立模块及其证据字节不改。
