# Orca B2a 原始证据

`records.json.gz` 保存 6 个原始 UTF-8 文件。解压后 JSON files 数组包含 path、bytes、sha256、utf8，编码回原字节可逐项复核。归档 SHA256 为 `0493c4b66bc2720d520dc284e9007290dd0442e3ec0ef754eeef93263194cbbc`。

- commands.json、source.json：实际检查入口、权限范围和源码身份；无真实 Orca 资源或模型场次。
- new.jsonl：初次 3 根/13 子例，两个 fixture 消费者因原冻结 Target 校验拒绝；保留失败。
- new-final.jsonl：修正 fixture 后 3 根/13 子例通过，0 fail/skip。
- regression.jsonl：21 根/35 子例通过，0 fail/skip。
- dependency.jsonl：1 根依赖守卫通过。

结论及边界见[验收报告](../../orca-cli-construction-capability.md)。
