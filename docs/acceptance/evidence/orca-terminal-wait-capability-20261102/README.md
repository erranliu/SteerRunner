# Orca B2d 原始证据

`records.json.gz` 保存 5 个原始 UTF-8 文件，JSON files 数组包含 path、bytes、sha256、utf8，可编码回原字节复核。归档 SHA256 为 `5ca3364466ddeae47beeb36227202db79c16bf1e33cbaa056b07a105c4656a7d`。

- commands.json、source.json：实际命令、范围和源码身份。
- new.jsonl：2 根/22 子例通过。
- regression.jsonl：14 根/103 子例通过。
- dependency.jsonl：1 根依赖守卫通过。

全部 0 fail/skip。未运行真实 Orca/模型，未保存认证帧或业务材料。结果及未验范围见[验收报告](../../orca-terminal-wait-capability.md)。
