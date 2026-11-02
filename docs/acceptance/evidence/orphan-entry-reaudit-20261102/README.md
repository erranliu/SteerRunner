# A/B 实现再审计证据

对应[再审计报告](../../orphan-entry-reaudit.md)。evidence-audit重新核对初轮完整SHA清单、全部源身份、18个关键消费者与原始测试计数；consumer-audit从基线源码重扫全部删除符号。declarations及wire-comparison为本轮重新运行，当前编码与原前后字节相同。

25个定向顶层通过、无skip，原始JSONL无损gzip保存。commands记录精确命令、退出码、权限和辅助审计工具纠正；没有生产或测试源码修复。初轮全部证据保持原字节，且未借本轮扩大为真实模型或完整L1/L2/L3资格。

审计源码以.go.txt存档，复制到仓库.evidence/orphan-entry-reaudit并恢复.go后缀可运行；声明工具应在原基线加本批差异的状态运行，后续提交须选定该基线。SHA256SUMS覆盖目录中除自身外全部文件。
