# 本次增量原始证据

对应[验收记录](../../orca-native-submit-control-stream.md)。源码/构建身份见 source-identity.json，
每个公开文件的字节数和 SHA256 见 manifest.json。

- before.jsonl.gz / before-exit.txt：新增首批 12 个正式流程叶例在旧生产实现上的失败。
- controlled-summary.json / controlled-tests.jsonl.gz：原正式 controlled 入口的逐叶摘要与完整原事件。
- l1-summary.json / l1-tests.jsonl.gz / framework-result.json / terminal-extensions.tap.gz：项目 L1 及附属检查。
- sessions.json：本机与远端各三个新现场的身份、结果、原文件摘要；失败保持失败。
- local/remote-01/02/03-records.jsonl.gz：每行是一个原始文件的 path、sha256、original_base64。
  解码得到原字节，可再次计算摘要并只读核验原输入、实际 CLI 参数/回执、正式结果与停止证据。
  保留原编码/BOM，不改写 JSON 或把失败报告整理成通过。

归档只选本批明确的公开测试目标、输入/快照/operation intent、回执、调用和清理记录；
不包含 EXE、运行配置正文、完整 owned-journal 或无关工作区清单。私有原现场在本地 .evidence，
该目录不随 PR 上传。受控矩阵、真实普通会话、真实长期累积流是不同证明范围，不能互相替代。
