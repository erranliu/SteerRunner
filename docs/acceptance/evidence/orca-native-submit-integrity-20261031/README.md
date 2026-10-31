# 原生输入保真修正原始证据

对应[验收报告](../../orca-native-submit-integrity.md)。原始字节与 SHA256 见 manifest.json；`.gitattributes` 仅对本目录关闭文本换行转换，不修改全局 Git 配置。

- source-identity / source-file-hashes / binary-identities：构建时 dirty HEAD、冻结源范围及二进制摘要。没有发布 EXE 或部署。
- fixture-fix-source / source-identity-after-fixture / rebuilt-agent：c8原生资格后仅一个RPC单元夹具关闭修正，新测试源b3a642…5c91；生产/live源未变，agent重建字节相同。新检查与原六场不冒称所有源码相同。
- source-index-mapping：695源文件对应Git内容的核验，单列两个文件既有CRLF→LF转换；原始构建摘要与Git规范化摘要不混称。
- controlled-summary / controlled-result / controlled-*-tests.jsonl.gz：正式稳定入口每叶 30 次的摘要、最终结果和完整原事件。source.passed 的中间 false 保持原样，以最终 result 与原计数共同核验。
- controlled-final-* / rpc-cleanup-30-*：夹具修正后的正式提交矩阵和原IPC/并发单测重复结果，实际执行数独立登记。
- l1-before-fixture-* / l1-timeout-stack：c8源的L1失败及夹具Close阻塞原栈；l1-summary/tests/invocation、framework及扩展日志是修后检查，不能改写旧失败。
- local/remote-0N-records.jsonl.gz：六个独立会话的原调用、原输入/摘要、operation/正式输入/结果/stop 证据。每行 `{path,sha256,original_base64}`；解码后的字节未经重写。
- workspaces-records.jsonl.gz / local-path-absence：只包含本次六个现场的创建、原 host census、关闭、异步移除请求及之后原 runtime 的 not-found 查询。本机路径不存在；未独立证明远端物理路径不存在。
- submitted-input-records / provenance：按精确 session_id 抽取的五条提交历史原行，仅最终本机三场和两个阴性对照。不含无关全局历史。
- negative-*-records / negative-input-comparison：单独 CR＋16MiB 和 LF＋128MiB 的失败原件，不能记作当前组合的通过资格。
- diagnostic-*-records / local-0N-trace-read.json.gz：临时 overlay 诊断会话及原流；前者16MiB限额断连，后者完整转发23,771,840字节回复。诊断业务未完成，不计资格；探针/overlay代码未进入生产。
- cap16-before：大回复正式测试在旧上限下的红对照。
- historical-* / development-*：旧三场损坏的所选历史原行/比较及开发期失败L1日志；均保持失败，不计当前通过。
- upstream-provenance / source-hashes：匹配版本的公开 Codex 源码来源，未上传 Orca 安装包私有源码。

使用 Node 只读核验归档并解码到一个新目录：

```powershell
node docs/acceptance/evidence/orca-native-submit-integrity-20261031/verify-archive.mjs .evidence/integrity-archive-replay
```

目的目录必须不存在；脚本只写该目录，核验所有文件和原行摘要，再运行两个已归档的离线核验器，不连接 Orca、不发送任务。原生任务的底层宿主按键次数、远端整机资源树、任意人工输入排他及历史完整复现不在本档证明范围。
