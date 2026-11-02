# Orca B3 原始证据

`records.json.gz` 保存 12 个原始 UTF-8 文件。JSON files 数组含 path、bytes、sha256、utf8，可编码回原字节复核。归档 SHA256 为 `6f9f77cae6b0bef64ed0becd937938470e52ced5e92730984847229119557092`。

- source.json、commands.json：最终源码身份、实际入口和范围。
- reader-initial.jsonl：预算 fixture 不推进被拒绝的初次失败；reader-final.jsonl 为修正后 11 子例通过。
- reader-lifecycle-final.jsonl：最终 reader 13 叶例通过，包括原 pipe 的显式/取消关闭。
- targeted-initial.jsonl：16 根/181 子节点通过。
- packages.jsonl：223 根/1024 子节点、1100 叶例，0 fail、7 既有 opt-in skip。
- dependency.jsonl：1 根依赖守卫通过。
- compile-initial.log、compile-second.log、compile-third.log：测试迁移中两次类型编译失败与最终编译通过；build-initial.log 为生产编译通过。编译不记业务测试资格。

定向与全集有重叠，不相加；未运行真实 Orca/模型，未包含认证帧或业务材料。结论及未验边界见[验收报告](../../orca-state-session-capability.md)。
