# Orca B4d 原始证据

`records.json.gz` 保存 12 个 UTF-8 文件；files 含 path、bytes、sha256、utf8。归档 SHA256 `909923a39598ca0f757cd9e3ac8188d97d40b1605a2bbfe2c29bf5155714ebc4`。

- commands.json/source.json：实际命令、先暂存后的源码身份、受控范围。
- compile-initial.jsonl/compile-second.jsonl/new.jsonl：迁移期旧字段/helper 编译失败，分别有 2/1/1 个失败包。
- new-final.jsonl：7 根/63 子节点、64 叶例通过；edge-final.jsonl：最终补验 3 根/26 子节点、27 叶例通过。
- packages.jsonl：229 根/1103 子节点、1177 叶例，0 fail、7 skip；consumers.jsonl：49 根/88 子节点、119 叶例，0 fail、1 skip。
- dependency-final.jsonl：1 根守卫通过；build.log/live-build.log：生产入口构建/live-tag 仅编译通过，空日志不代表场景资格。

EXE、认证帧、业务材料不归档。各集合有重叠、skip 和未验边界见[验收](../../orca-close-inventory-capability.md)。
