# Orca B2c 原始证据

`records.json.gz` 保存 5 个 UTF-8 原始文件，JSON files 数组包含 path、bytes、sha256、utf8，可编码回原字节复核。归档 SHA256 为 `0a411c5ec5f0d89347f38f33e068b8ee6ee3032b57cbeb587079779e771eb069`。

- commands.json、source.json：实际检查入口、范围与源码身份。
- new.jsonl：4 根/64 子例通过，其中能力根 15 子例。
- regression.jsonl：22 根/149 子例通过。
- dependency.jsonl：1 根依赖守卫通过。

全部 0 fail/skip，受控组有重叠，不相加为独立场次。未运行真实 Orca/模型，未保存认证帧或业务材料。结论见[验收报告](../../orca-terminal-read-capability.md)。
