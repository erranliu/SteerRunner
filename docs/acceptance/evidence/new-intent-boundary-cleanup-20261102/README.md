# 新写入边界与展示清尾证据

对应[专项验收](../../new-intent-boundary-cleanup.md)。基线main98309f8加本批差异；source-identity.json.gz保存788项最终冻结源/配置及产物身份，runner-build.json与foundation/source-identity.json绑定同最终EXE。

design-audit/consumers-before/source-before保存生产编辑前设计、文档摘要及消费者；source-comparison/declarations证明仅授权修改、原测试/共享夹具保持。wire-before/after是同一工具捕获的持久根、四类保存输入及公开能力，未把授权变化的瞬时PreparationFacts混作持久格式。

boundary.jsonl.gz是新增夹具直接调用内部invoke而崩溃的首轮；boundary-final.jsonl.gz是未冻结原时间而触发既有预算保护的第二轮；boundary-qualified及最终application均通过。原失败不改判，未修改生产规则或原断言。test-summary区分运行轮次、名称、skip和未完成项，commands给出精确命令与权限。

foundation为精确三个公开场景，timings为主线FromEvidence计时导入；qualification_eligible=false，不授予完整Controlled资格。不包含EXE或用户已有Run数据；原始JSONL和源/编码快照用gzip无损保存。

审计源码以.go.txt保存，复制至仓库.evidence/new-intent-boundary-cleanup并恢复.go后缀后可按commands重跑。声明比较应在原基线加本批差异的检出状态使用，不依赖后续HEAD；工具不是运行时依赖。根.gitattributes仅对本目录设置原字节保护；SHA256SUMS覆盖除自身外的全部证据文件。
