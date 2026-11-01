# EPBC 实现再审计证据

范围及最终结论见[报告](../../exit-proof-budget-reaudit.md)。JSONL 使用 gzip 保存原字节；文件名不替代通过/失败事实。

- consumer-audit.json / packages.txt：重新盘点退役符号及当前模块，历史工具保留在独立 evidence 模块；共享清理/请求正文与基线一致。
- source-identity.json / runner-build.json：本轮基线加 dirty 源码及最终产物身份，五个生产文件与初轮逐字节相同。
- application-fixture-failure.jsonl.gz：改为合法 accepted 资源夹具后，遗漏停止前原创建核实阶段的失败；未改判。
- application-final.jsonl.gz / adapters-final.jsonl.gz / test-summary.json：最终精确组件测试与 skip；original-results 为新增九个子用例首轮独立执行，不重复计数。
- foundation-final / native-final：最终 Runner 的公开受控与真实 READY/停止闭环、公开回执；没有 EXE、数据库、终端原文或无关 worktree 清单。
- cleanup-confirmation.json：测试清理及结束后的新查询确认自有 worktree 缺席。
- commands.json / SHA256SUMS：实际选择器与除摘要自身外的全部证据 SHA。

未改变初轮证据字节，不把历史结果升级为本轮全集资格。
