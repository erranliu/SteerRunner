# Orca B0 原始证据

`records.json.gz` 是本批的原始文本文件归档。解压后的 JSON 包含 files 数组，各项保存相对路径、原始字节数、SHA256 和原 UTF-8 文本；用 utf8 编码回字节后可复核摘要。未包含测试 EXE，EXE/source/CLI 摘要保存在各现场 result.json 中。

- local-01：修正前现场。业务两轮及清理通过，汇总脚本失败，passed=false；保留失败，不作为最终资格。
- local-02、remote-01：最终同源、同 EXE 的双端新终端现场，passed=true。
- controlled-final.jsonl / controlled-summary.json：3 个新根、26 个子例，0 fail/skip。
- regression-final.jsonl：11 个原回归根、132 个子例，0 fail/skip。
- 根目录 fixture/tooling 记录：专属 worktree 分配、初始 shell 精确关闭，以及 Codex 版本/命令定义摘要的补采与终端清理；不混入业务发送计数。

证据为本批非秘密 nonce 和公开原生事实，未保存配对凭据、认证帧或完整环境。读取正文证明只能使用原输入查询和正式 final，tooling 的终端流只用于版本元数据。

资格与限制见[验收报告](../../orca-native-capability-baseline.md)。
