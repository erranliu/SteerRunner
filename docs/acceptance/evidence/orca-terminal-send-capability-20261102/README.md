# Orca B2b 原始证据

`records.json.gz` 保存 11 个原始 UTF-8 文件，JSON files 数组包含 path、bytes、sha256、utf8，可编码回原字节复核。归档 SHA256 为 `a4094cc96dc49610c97bc76e17a829f588fe9d9469060067e8c6f87554a4a743`。

- source.json、commands.json：源码身份、最终命令和范围。
- field-scopes-final.jsonl：最终字段范围 4 根/71 子例通过，其中能力根 17 子例。
- consumers-qualified.jsonl：最终实际消费方集 37 根/222 子例通过。
- protocol-regression.jsonl：4 根/11 子例通过。
- dependency-final.jsonl：最终 1 根依赖守卫通过。
- new.jsonl、new-final.jsonl、consumers-final.jsonl、dependency.jsonl：中间检查，最终字段分层补齐前通过；不替代最终范围。
- consumers.jsonl：首次漏 import 的 build-fail，0 测试根；保留失败，不计通过。

全部最终检查均为 0 fail/skip。未包含 EXE、认证帧或真实业务材料，未运行真实 Orca/模型。结论见[验收报告](../../orca-terminal-send-capability.md)。
