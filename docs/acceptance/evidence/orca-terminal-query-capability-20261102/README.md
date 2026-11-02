# Orca B1 原始证据

`records.json.gz` 保存 26 个 UTF-8 原始文件。解压 JSON 的 files 数组包含 path、bytes、sha256、utf8；将 utf8 编码回字节可逐项复核。归档 SHA256 为 `0ca8607b0927a426749f25013a0b1cb62785d2a307d79346da76c0b813a57aba`。未保存测试 EXE、认证帧或环境凭据。

- build.json：源码、查询 EXE、CLI 摘要；源码范围沿既有 Get-SRSourceIdentity，不含验收文档。
- targeted.jsonl、packages.jsonl：首次受控运行，保留沙箱导致的失败及既有 opt-in skip。
- remote-controlled-qualified.jsonl：正常权限下远端 WS 夹具复跑，1 根/9 子例通过。
- failed-roots-qualified.jsonl、failed-roots-pattern.txt：全集 15 个失败根的正常权限复跑，15 根/50 子例通过。
- dependency.jsonl：执行依赖守卫通过。
- local/remote 的 before-verified、create、query-target、query-target.result、query-qualified.log、before-close、close、after：专属无模型 shell 的创建、实际 API 查询与精确收尾。每个原 shell 只 close 一次，终态清单的原主机完整可见且为空。
- local-before.json、local-query.log：首次过严清单预检及参数解析退出的留档，不计资格；前者的其他主机遗漏不能被解读为原 local 不可见。
- orca-guide.json：本轮所选 CLI 的版本匹配操作指南。

结果与未验范围见[验收报告](../../orca-terminal-query-capability.md)。
