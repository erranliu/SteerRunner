# 内容接管补验交付证据

- [rebased-directed.json](rebased-directed.json) / [原始 go test 输出](rebased-directed.jsonl)：接上主干 #37 后四包定向 101 项通过，零失败/skip，四包均正常结束。生产 takeover/freeze/workflow 与主干无差异；后续变更仅为本节文档及证据归档。
- [generation-takeover.json](generation-takeover.json)：此前真实 A/B 的原始公开快照，保留原 Run/input/final/content_result 及两个构建 SHA/实例。
- [qualification.json](qualification.json)：同一 SteerLine live 包后验，程序/报告/API/浏览器通过、原 Run 资源结算、所属 base/head 删除且 PR 数为零。

完整上下文及历史失败来源见[专属验收](../../content-workflows.md)。A/B 的来源为 `c122d641` 加当时工作树修复，并非主干 #37 或本 PR 最终提交的干净构建；不扩大其真机资格。二进制、认证及安装环境未纳入仓库或修改。
