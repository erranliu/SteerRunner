# Orca B4a 原始证据

`records.json.gz` 保存 5 个原始 UTF-8 文件。JSON files 数组含 path、bytes、sha256、utf8，可编码回原字节复核。归档 SHA256 为 `f2ff64a87d30cec51c98ef6b8989062d2787c77acd57f5cb366e2fbf7229c6bf`。

- source.json、commands.json：源码身份、实际测试入口及受控范围。
- new.jsonl：1 根/17 子节点、15 叶例，0 fail/skip。
- regression.jsonl：24 根/77 子节点、86 叶例，0 fail/skip。
- dependency.jsonl：1 根依赖守卫通过。

没有认证帧、业务正文或真实资源操作；结论和边界见[验收](../../orca-resource-query-capability.md)。
