# 新写入边界实现再审计证据

对应[再审计报告](../../new-intent-boundary-reaudit.md)。最新base66cc0ac，初轮98309f8；audit列明上游8项源/测试/工具差异，产品与新增回归未修订。source-identity.json.gz绑定当前冻结源及最终产物，不复用过期integration夹具。

transforms/declarations为本轮精确源码和AST复核，consumers为当前实际入口，wire-comparison重新编码并与初轮前后逐字比较。初轮全部证据原字节保持，不重写初轮498项资格。

application.jsonl.gz保存本轮既有resultFixture初始化read_failed的首轮失败；budget-recheck为隔离通过，application-final为原完整选集复验通过。file-fixture-review记录相关原代码不变且原因未确证，不伪称已修复未定位的I/O问题。consumers原生/共享夹具定向通过。最终27顶层/43子用例通过，0skip，重复复验不累加。

foundation为当前主线同EXE的三个公开场景；timings仅作FromEvidence计时导入，qualification_eligible=false。commands保留精确命令/退出码及资格边界；未运行全Controlled或真实模型。审计工具.go.txt应复制至仓库.evidence/new-intent-boundary-reaudit并恢复.go后缀，AST工具须在指定基线加本批差异状态运行。

SHA256SUMS覆盖除自身外全部文件；根.gitattributes仅对本目录保护原字节。源码冻结后没有编辑产品或测试，没有EXE或用户已有Run现场入库。
