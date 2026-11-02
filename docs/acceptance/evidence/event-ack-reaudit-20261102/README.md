# EAC 实现再审计证据（2026-11-02）

结论及人工消费链核对见[再审计报告](../../event-ack-reaudit.md)。本次没有新增生产/测试修改，原证据及失败/skip 保持原字节和原判。

- boundary-audit：worker/owner 全文只去第二回复；Orca observe 全文只去原通用标签、Processed分配/第二等待，原第一回复/等待和其余循环一致。9份原夹具只去第二回复，关键消费/提交/问答/恢复实现保持；trace分类条件及持久根人工核对单列。
- identity：680项输入、三份产物、初轮摘要匹配，重新生成的145116字节编码等于实施前基线。原工具/捕获仍在[初轮证据](../event-ack-cleanup-20261102/README.md)。
- declarations.go/json：基线72b589b上的8个生产变更文件，177声明相同（52类型）、9授权调整/7删除/0新增；授权变化另经精确差异与行为验证，不仅依赖allowlist。复跑须以基线为HEAD并保留本批未提交实现。
- application/observer.jsonl.gz、commands、test-summary：本次原始字节、精确命令/退出码及实际名称，31+2顶层、25+3子用例通过，0 skip。observer过滤中的PublicMessage前缀未增加匹配，未将其算作通过测试。
- references-before/after.txt.gz：当前模块（排除docs历史模块）基线/当前文本盘点，包含定义和同名字段，不单独当合法入口证明。
- SHA256SUMS：除自身外的目录摘要；不含EXE、数据库或用户现场。未重复执行完整初轮公开场景，不新增真机/完整矩阵资格。
