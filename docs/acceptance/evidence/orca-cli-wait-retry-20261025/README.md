# Orca CLI wait/retry：2026-10-25 验收证据

本目录固定本仓实现与独立测试结果，不包含 EXE 或认证文件。完整原始批次位于 `C:/SteerlineDebug/runner-cli-wait-retry/`。

- `final-build.json` / `source-files.json` / `source-immutability.json`：最终 F83D... 构建、755 份实现/测试输入及结束复核零变化。
- `final-summary.json`、`l1-package-results.json`、`l1-selected.jsonl`、`framework-result.json`：正式 L1 通过；673 个顶层测试、29 个终端扩展、22 个框架检查。已有允许跳过项在原 summary 单列。
- `directed-*`、`cli-wait-*.json`：同一 Runner SHA 的专属 Controlled 测试，6 变体、100.40 秒、全部通过。
- `baseline-reproduction.log` / `baseline-fixture.go.txt`：指定 origin 分支真实复现 15 秒前置排队截断，实际 CLI 调用 0 次且附带空 JSON 错误。
- `controlled-full*`、`defender-*`：完整 Controlled 执行中 Runner 被 Defender 识别为 Trojan:Win32/Bearfoos.A!ml 并隔离。原失败保持；告警未由人工判断为误报。
- `containment-cleanup.json`、`historical-*`：本任务专属进程收尾与历史夹具的物理资源审计；旧未知数据库保持原样。
- `*-interruption.json`：被替代的旧顶层调度在派发原生前结束；其 running summary 不代表通过。独立受控子进程的后续收尾另列。
- `implementation-audit.json`：实现后的单次静态审计、755 份冻结输入复核和两条 go vet 结果；不替代未完成的功能验收。

当前完整验收未完成：L2 两项与新增 local/remote case 均未执行，没有本轮原生资格。先人工核查并处理安全告警，再以全新 OutDir 重跑正式入口。不得通过关闭防护、排除或更名来绕过检测。
