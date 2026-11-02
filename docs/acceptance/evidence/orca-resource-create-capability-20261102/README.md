# Orca B4b 原始证据

`records.json.gz` 保存 6 个原始 UTF-8 文件，files 数组含 path、bytes、sha256、utf8。归档 SHA256 为 `39b2824c01062d1a6dbd33472d9f7b194a71f806664ee8e1a9e48075e9ca9216`。

- source.json、commands.json：源码身份、实际命令及受控范围。
- new.jsonl：2 根/25 子节点、25 叶例，0 fail/skip。
- regression.jsonl：34 根/141 子节点、147 叶例，0 fail/skip。
- dependency.jsonl：1 根守卫通过；compile.log：两包编译通过。

无真实 Orca/模型、认证帧或业务材料。结论及边界见[验收](../../orca-resource-create-capability.md)。
