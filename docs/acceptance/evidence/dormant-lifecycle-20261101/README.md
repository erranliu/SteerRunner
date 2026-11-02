# DLC 2026-11-01 原始证据

范围及结论见[实施验收](../../dormant-lifecycle-cleanup.md)。gzip 保存原 JSONL 字节；文件名不代替执行状态。components-final 的 reliable 首轮测试编译失败，之后 reliable-final 单包通过；原失败不改判。

- design-audit / consumers-before / lifecycle-references-before / consumers-after：设计审计与引用分类；生产编辑前完成范围审计。
- wire-before/after、wire-comparison、wire-audit.go：固定编码/类型及公开能力对比，139060 字节相同。样本用于编码，不代表合法外发意图。
- retained-functions / function-audit.go：保留函数 AST 比较。复跑时从 source-identity 的基线，用 git show 恢复本次 .evidence/dormant-lifecycle/base/ 下对应的六个文件，再运行脚本；不将原源码快照接回产品编译。
- components/application/reliable-final、test-summary、commands：精确运行、计数、默认 skip、原编译失败与单包复验。
- foundation-final：最终 Runner 的三项公开场景、原命令/原生调用、结果及夹具身份；不含 EXE 或数据库。
- runner-build / source-identity / production-diff：正式构建、dirty 源逐文件 SHA、产物 SHA 和 A/B 生产增删。
- build-first-failure：第一次 unused imports 的失败摘记，不是完整 stdout；SHA256SUMS 覆盖除自身外所有证据。

本次完整临时产物在工作区 .evidence/dormant-lifecycle；历史验收证据保持原字节。未部署，未运行真实模型。
