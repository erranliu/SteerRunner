# 本机 Codex app-server 用量验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-05。Runner 在独立副本中以 `9c36f366` 为基线构建。受测 Runner SHA-256 为 `a71f7756b9dfb5141c0e75509feaa3c09046034b6c238f2bc5790dd0f74804d2`。SteerLine 与 Web 以 `e010373e` 构建。

- Runner 定向回归通过。运行 `go test ./internal/usage/... ./internal/storage ./internal/resources ./internal/host -count=1`。新增测试核对同一候选跨两轮汇总、重复通知去重、外来会话隔离和重复采集的快照稳定性。
- SteerLine 的 App、HTTP、Runner 客户端定向用量测试及 Web 组件 5 项通过。生产 Web 构建通过。
- 独立项目完成正式本机环境检测后，以默认本机同款 Codex 可执行文件和 `gpt-6-luna / high / full_access` 发起真实候选。Git 和 PR 使用回环测试服务。候选 `ad631efaaab04a1697e118fccb5e8d91` 对应 Runner Run `8f7399ae30de99d24e83a38844b9863c`。Edge 实际显示 **37,571 Token**，与该候选用量响应的第 6 版快照一致。两条原生响应合计输入 37,012、输出 559，缓存读取 18,048。快照来自仍在执行的主会话。
- 原始浏览器取证为 `browser-usage.json` 和 `execution-usage.png`。浏览器断言通过后，宿主停止命令报告原生资源尚未结算，因此场景整体返回失败。隔离验收清理流程随后核实 Runner 已清理，`scenario.json` 记录 `cleanup_complete=true`。前两次分别在辅助程序选错和未做环境检测时止于候选发起前。

用量读取沿用原 Run 快照和查询。原生通知的累计位置用于去重和检测缺口，已验证的单响应 Token 进入现有账本。逐响应计价所需的供应商、模型和时间保持未知。
