# A/B 清尾证据

对应[专项验收](../../orphan-entry-cleanup.md)。基线main `111bcd3`，最终源码/产物见source-identity.json.gz和runner-build.json；最终公开场景的source-identity/result另存于foundation/。不包含EXE或用户已有Run现场。

design-audit、consumers-before、source-before保存实施前盘点及文档摘要；source-comparison、declarations、wire-comparison证明授权删除边界及保留源/编码。wire-before/after和原始测试JSONL使用gzip无损保存。test-summary区分首轮沙箱失败与最终正常权限复验、skip和超时未完成项，commands保存精确命令。两轮之间产品及测试源码未修改。

审计源码以.go.txt保存，避免给docs增加Go包。复核时复制wire-audit.go.txt、declarations.go.txt到仓库.evidence/orphan-entry-cleanup/并恢复.go后缀，按commands运行；它们不是运行时依赖。声明工具以HEAD为基线，已提交后复核须在原基线加本批差异的检出状态使用。完整源文件身份使记录不依赖后续HEAD。

SHA256SUMS覆盖本目录除清单自身的全部文件。公开链路的caller/native调用原字节压缩保存；其通过资格仍以隔离场景实际断言和最终result为准，不从日志缺项推导成功。
