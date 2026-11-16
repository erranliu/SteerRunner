# 本机 Windows 环境检测从文件启动

本机 Windows 检测在 Runner 状态目录中为每个 Run 写入 PowerShell 包装脚本和 UTF-8 JSON 输入。命令行通过 `powershell.exe -File` 传递文件路径。Runner 确认原进程树结算后删除启动文件，项目入口、退出码校验及结果封存协议沿用原流程。Orca 继续使用原终端启动方式。

## 验证

- `internal/execution/environmentcheck` 定向包测试通过。检测回归使用会超过原 Windows 命令预算的输入和包含空格的启动文件路径，验证脚本执行、材料漂移拒绝及结果解析。
- `internal/application` 全包测试达到 Go 默认 10 分钟上限，超时发生时正在执行 `TestRuntimeFilesFailureDoesNotGateBusiness`。该用例与 `TestLocalCheckoutUsesSavedPreparationDeadlineAndBlocksInputOnFailure` 单独复跑均通过。
- 使用安装版 Runner 源码基线 `bf0bd191180aa11a3bbaeeb9dbbc0c210b19fac1` 加本次改动构建独立 EXE，SHA-256 为 `e0f2dcd1dfa7c16ddf3182e38d62b0fe143a6391006a1123b94f93efb9ddfbcb`。GitHub main 在验证时仍指向该基线。
- 重放 `merge-h-308-code` 检测记录 `87b2107793174d7e9ba32bf6e50e757b`，原 Run 为 `5b77d4f9d214e528c8898575a2272447`，原 Attempt 为 `d3274b3670a48c337e32175caba01441`。Runner 请求使用新的请求 ID 和隔离 checkout 路径，保留 `AIRefactor@544a8c5cbafe61975740bb734a5d68fbb30883ae`、材料摘要、执行设置及原始结果信封。
- 重放 Run `90e20c235216757a828cd6a42d21eafc` 已完成并停止。公开 `result-file` 封存状态为 `closed`，版本为 1，内容 SHA-256 与回执一致。项目报告显示 `completed=true`、`ready=true`，十项检查通过，包含 Unity batchmode 授权。启动文件已清理，隔离 checkout 干净，`service verify-stopped` 返回 `resources_settled=true`。
- 原始证据位于本次独立构建目录的 `replay-d8912e09/`，其中 `source.json`、`inspect.json`、`result.json`、`report.json` 和 `verify-stopped.json` 分别记录请求来源、Run、封存结果、项目报告和停服结算。

验证在隔离 Runner 中完成。未部署安装环境，未执行完整分级测试或真实 Orca 运行。
