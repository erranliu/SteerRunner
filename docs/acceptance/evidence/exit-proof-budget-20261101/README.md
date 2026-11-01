# EPBC 2026-11-01 原始证据

结论及范围见[实施报告](../../exit-proof-budget-cleanup.md)。JSONL 原日志使用 gzip 压缩保存原始字节；文件名包含 final 不自动代表通过，以日志、result 和 test-summary 为准。adapters-final 中 protocol 包首轮编译失败，之后 protocol-final 单独整包通过；native-final/foundation-final 为保留的首轮失败，confirmed 为新现场复验。

- source-identity.json：dirty 源码逐文件及测试/Runner/helper 产物 SHA，含删除标记及五个生产文件增删；不以 HEAD 冒充 dirty 源码。
- runner-build.json：官方构建脚本原记录。
- test-summary.json：每次原执行的顶层/子用例/skip/包结果。
- foundation-*：公开 CLI 原运行/原命令日志、结果、源码及夹具身份；不含 EXE。
- native-*：真实 READY、公有资源/CLI 回执、核验和清理结果；不含 state-dir、终端原文或无关资源清单。
- cleanup-confirmation.json：测试清理及最终新查询对两个自有 worktree 的缺席核实。
- SHA256SUMS：除自身以外所有证据文件的原字节摘要。

原产物与完整本次现场仍在工作区 .evidence/exit-proof-budget；不为验收篡改历史目录，未替换安装环境。
