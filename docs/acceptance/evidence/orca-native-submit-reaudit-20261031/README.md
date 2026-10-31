# PR #73 再审计原始证据

对应[再审计报告](../../orca-native-submit-reaudit.md)。本目录`.gitattributes`仅关闭本目录证据的文本转换，保留清单中的原字节；不改变全局配置。

- source-identity/source-file-hashes/binary-identities：本次冻结源、构建时dirty HEAD、隔离agent/driver摘要。未上传或部署EXE。
- final-red/before-overlay/before-lifecycle：当前最终四例在旧生产构造器上的真实失败；overlay仅替换597d669的生产文件，不修改Orca。
- controlled-summary/result/tests：原正式入口239叶各30次＝7170/0/0；嵌入source.passed=false是最终冻结校验前的中间状态，最终result=true另存。
- constructor-original-query-30：含生产构造器及原thin/corroborated原输入回归，15叶各30次＝450/0/0；与正式矩阵中的新四叶有重叠，不能合并成不重复的覆盖数。
- l1-* / framework / extensions：完整项目检查、17项明确opt-in/helper跳过和1无测试包单列；不把跳过算真机通过。
- local/remote-0N-records.jsonl.gz：各行`{path,sha256,original_base64}`，解码为原调用、输入/wire、operation、正式原输入/final和stop字节。只含本批合成任务。
- sessions / local-three-audit / remote-three-audit：原请求/资源身份、helper与WireFormat分类及独立离线核验。managed本机不冒称无helper路径真实通过。
- submitted-input-records/provenance：按本批本机三个精确session_id选择的提交历史原行，无无关全局历史。
- workspaces-records/local-path-absence：本批工作区创建、原host完整空清单、异步移除及后续not-found。本机路径不存在；远端物理路径未独立核验。
- development-records：开发期旧回归及错误展示断言的失败，原日志保留，不计通过。

使用Node离线核验清单、原行摘要及回归实数，再解码到一个不存在的新目录并复核六场：

```powershell
node docs/acceptance/evidence/orca-native-submit-reaudit-20261031/verify-archive.mjs .evidence/reaudit-archive-replay
```

脚本只写指定的新提取目录，不连接Orca、不重投任务。真实条件、历史完整复现、全文原子消费、人工输入排他及远端整机停止的边界见报告。
