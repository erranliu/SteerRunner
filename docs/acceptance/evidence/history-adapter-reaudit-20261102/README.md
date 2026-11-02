# HAC 实现再审计证据（2026-11-02）

结论和人工入口核对见[再审计报告](../../history-adapter-reaudit.md)。本次未追加生产/测试修改，初轮证据及其失败/skip 保持原字节和原判。

- declarations.go/json：对全部12个变更 Go 文件（含新增文件）按包、方法接收者比较声明。46个保留声明相同（17个类型），6个声明仅迁址，107个声明删除，新增/改写声明为零。工具复跑须以基线 e2bc0a5 为 HEAD，保留本批未提交差异；不将归档脚本接入产品模块。
- baseline-reference-files / references-before/after.txt.gz：全部删除名称的基线文件级盘点及当前全仓文本检查，排除 docs 历史模块；包含定义、字段、其他包方法和注释，不能把文本同名当成实际消费者。manual-audit 记录同名判别与真实入口复核。
- identity：685项源码/构建输入、3个二进制和初轮证据摘要全部匹配；重新生成的140590字节编码与初轮实施前基线匹配。编码工具/原捕获仍在[初轮证据](../history-adapter-cleanup-20261102/README.md)，不复制改写。
- codex/application.jsonl.gz、test-summary、commands：本次原始日志字节、精确命令/数量/顶层名称/退出码，14+30顶层通过、0 skip。重复测试不与初轮累加。
- SHA256SUMS：除自身外本目录所有证据摘要；无 EXE、数据库或用户现场。
