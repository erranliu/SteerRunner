# A/B 独立验收证据

2026-11-06。先 A 后 B，均为合并前定向组件及三项公开受控验证。总入口为 [A](../../orca-state-feed-retirement.md) / [B](../../orca-helper-producer-retirement.md)，不代表完整 L1/Controlled、L2/L3 或真实模型资格。

- `a-summary.json` / `b-summary.json`：分包实际数量、原跳过、同最终 EXE 的三项公开结果、源码/产物/可恢复归档摘要。A 的错误 `internal/livetest` 路径保留在原聚合结果中，正确 testkit/livetest 独立通过。
- `*-declaration-diff.json`：全生产 AST 差异，保留声明正文相同。A 只删11个函数/方法、5个瞬态类型和3个常量；B 只删3个函数/方法、1个非持久类型和1个资产变量。
- `*-file-diff.json` / `*-source-sha256.json`：各批独立源码点和原始订阅、socket/IPC、stability、JS/脚本保护摘要。原源码归档保留在工作副本 `.evidence/retirement-audit/{baseline,a-final,b-final}/source.zip`；原基线可从 main `dda9a827` 还原。
- `baseline-{wire,frozen}.json.gz`、`{a,b}-{wire,frozen}.json.gz`、`format-comparison.json`：独立原字节与比较，未修改实际持久类型/tag/codec。`wire.go.txt` / `frozen-capture_test.go.txt` 为实际捕获入口。
- `b-before-assets.json` / `b-after-assets.json`：旧生产函数和新测试 helper 的实际摘要，非仅用新算法自证。`b-retention.json`：原 Node TAP 摘要/退出码、相对 import、原生产/测试 embed 边界及协议结果复用声明。
- `*.jsonl.gz`、`*-controlled-*`、`b-node-final.tap.gz`：原组件/公开日志、构建和身份；脚本异步启动后等待原最终 result.json，不以启动返回计通过。
- `a-first.jsonl.gz`：原沙箱内 WebSocket 定向失败；`b-first.jsonl.gz`：遗留未用 import 的初次编译失败；`b-node.tap.gz`：原沙箱临时文件原子重命名 EPERM。后续通过不覆盖这些失败。
- `*.py.txt` / `declarations.go.txt`：审计/归档工具，不是生产或新增通用测试入口。生产和正式测试仅按实施文件变更，临时捕获测试运行后删除。

`SHA256SUMS.json` 覆盖本目录其它文件；包含原 opt-in 跳过及失败，不把未执行计为通过。所有模型诊断 probe 均未执行。
