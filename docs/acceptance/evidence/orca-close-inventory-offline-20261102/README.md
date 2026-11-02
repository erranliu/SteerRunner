# B4d 正式 CLI 离线核验补验

`records.json.gz` 保存实际 command.txt 和 integration-offline.jsonl 两个 UTF-8 文件，含 bytes/sha256/utf8；归档 SHA256 `e52d01565f92662fecfb020ed72837b48acff069ad6a050eb75cc2c3bb18e181`。

在修复提交 `0dfbb53` 上补跑准确的 TestSRServiceOwnership/offline_verification、remote_offline_scope：1 根/9 子节点、8 叶例，0 fail/skip。原消费者正则未包含该根，因此追加而不修改原 49 根统计。仅受控 CLI fixture，不是真实 Orca 资格。
